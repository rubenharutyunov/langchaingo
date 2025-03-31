package indexing

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/vectorstores"
)

type Result struct {
	NumAdded   int
	NumSkipped int
	NumUpdated int
	NumDeleted int
}

type CleanupMode int

const (
	None CleanupMode = iota
	Incremental
	Full
	ScopedFull
)

func (d CleanupMode) String() string {
	return [...]string{"None", "Incremental", "Full", "ScopedFull"}[d]
}

type Options struct {
	batchSize        int
	cleanup          CleanupMode
	sourceIdKey      string
	cleanupBatchSize int
	forceUpdate      bool
}

type Option func(*Options)

func WithBatchSize(size int) Option {
	return func(opts *Options) {
		opts.batchSize = size
	}
}

func WithCleanup(cleanup CleanupMode) Option {
	return func(opts *Options) {
		opts.cleanup = cleanup
	}
}

func WithSourceIdKey(key string) Option {
	return func(opts *Options) {
		opts.sourceIdKey = key
	}
}

func WithCleanupBatchSize(size int) Option {
	return func(opts *Options) {
		opts.cleanupBatchSize = size
	}
}

func WithForceUpdate(force bool) Option {
	return func(opts *Options) {
		opts.forceUpdate = force
	}
}

func Index(
	docsSource []schema.Document,
	recordManager RecordManager,
	destination vectorstores.IndexableVectorStore,
	namespace vectorstores.Option,
	opts ...Option,
) (Result, error) {
	// Set defaults
	options := &Options{
		batchSize:        100,
		cleanup:          None,
		sourceIdKey:      "",
		cleanupBatchSize: 1000,
		forceUpdate:      false,
	}

	for _, opt := range opts {
		opt(options)
	}

	indexingResult := Result{
		NumAdded:   0,
		NumSkipped: 0,
		NumUpdated: 0,
		NumDeleted: 0,
	}
	// Mark when the update started.
	indexStartTime, _ := recordManager.GetTime()

	scopedFullCleanupSourceIds := make(map[string]bool)

	// Process documents in batches
	for _, docBatch := range batch(options.batchSize, docsSource) {
		var allHashedDocuments []*hashedDocument
		for _, doc := range docBatch {
			hashedDocument, err := fromDocument(doc)
			allHashedDocuments = append(allHashedDocuments, hashedDocument)
			if err != nil {
				log.Printf("Error getting hashed document: %v", err)
				continue
			}

		}

		// Deduplicate hashed documents
		hashedDocuments := DeduplicateInOrder(allHashedDocuments)

		// Find source IDs for the batch
		// Some documents might not have a source ID, but they still can be used for Full cleanup mode
		var sourceIds []*string
		// UIDs for the batch
		var hashedDocumentUIDs []string

		for _, doc := range hashedDocuments {
			sourceIdFromMetadata, ok := doc.Metadata[options.sourceIdKey].(string)
			if ok {
				sourceIds = append(sourceIds, &sourceIdFromMetadata)
			} else {
				sourceIds = append(sourceIds, nil)
			}
			hashedDocumentUIDs = append(hashedDocumentUIDs, doc.UID)
		}

		if options.cleanup == Incremental || options.cleanup == ScopedFull {
			for _, sourceId := range sourceIds {
				if sourceId == nil {
					return indexingResult, errors.New("sourceId can't be nil when cleanup is Incremental or ScopedFull")
				}
				scopedFullCleanupSourceIds[*sourceId] = true
			}
		}

		// Check if documents with current batch UIDs exist in recordManager
		existsBatch, err := recordManager.Exists(hashedDocumentUIDs)
		if err != nil {
			return indexingResult, fmt.Errorf("failed to check if documents exist in record manager: %w", err)
		}

		// Filter out documents that already exist in the record store.
		var docsToIndex []schema.Document     // Documents to index (both additions and force-updates)
		var uidsToRefresh []string            // UIDs of documents that need to be refreshed (skipped)
		uidsToUpdate := make(map[string]bool) // UIDs of documents that need to be updated (force-updated)

		for i, hashedDocument := range hashedDocuments {
			docExists := existsBatch[i]
			if docExists {
				if options.forceUpdate {
					// Update anyway, even if it's already in the record store.
					uidsToUpdate[hashedDocument.UID] = true
				} else {
					// Refresh the timestamp for the UID but don't index
					uidsToRefresh = append(uidsToRefresh, hashedDocument.UID)
					continue
				}
			}
			docsToIndex = append(docsToIndex, hashedDocument.toDocument())
		}
		// Refresh timestamps for UIDs of existing documents
		if len(uidsToRefresh) > 0 {
			err := recordManager.Update(uidsToRefresh, nil, &indexStartTime)
			if err != nil {
				return indexingResult, errors.New("failed to update timestamps in records manager")
			}
			indexingResult.NumSkipped += len(uidsToRefresh)
		}
		// Index documents
		if len(docsToIndex) > 0 {
			ids, err := destination.AddDocuments(
				context.Background(),
				docsToIndex,
				namespace)
			if err != nil {
				return indexingResult, fmt.Errorf("failed to add documents with IDs %v", ids)
			}
			indexingResult.NumAdded += len(docsToIndex) - len(uidsToUpdate)
			indexingResult.NumUpdated += len(uidsToUpdate)
		}

		// Update ALL records, even if they already exist since we want to refresh their timestamp.
		recordManager.Update(hashedDocumentUIDs, sourceIds, &indexStartTime)

		if options.cleanup == Incremental {
			for _, sourceId := range sourceIds {
				if sourceId == nil {
					return indexingResult, errors.New("sourceId can't be nil")
				}
			}
			uidsToDelete, err := recordManager.ListKeys(&indexStartTime, nil, sourceIds, nil)
			if err != nil {
				return indexingResult, errors.New("failed to list keys in record manager")
			}

			// Delete from vector store
			if len(uidsToDelete) > 0 {
				ids, err := deleteDocuments(destination, uidsToDelete, namespace)
				if err != nil {
					return indexingResult, errors.New("failed to delete documents from vector store")
				}
				err = recordManager.DeleteKeys(uidsToDelete)
				if err != nil {
					return Result{}, err
				}
				indexingResult.NumDeleted += len(*ids)
			}
		}
	}
	if options.cleanup == Full || options.cleanup == ScopedFull {
		var deleteGroupIds []*string
		if options.cleanup == ScopedFull {
			deleteGroupIds = scopedFullCleanupSourceIdsToSlice(scopedFullCleanupSourceIds)
		} else {
			deleteGroupIds = nil
		}

		for {
			uidsToDelete, err := recordManager.ListKeys(&indexStartTime, nil, deleteGroupIds, &options.cleanupBatchSize)
			if err != nil {
				return indexingResult, fmt.Errorf("failed to list keys in record manager: %w", err)
			}

			if len(uidsToDelete) == 0 {
				break
			}

			ids, err := deleteDocuments(destination, uidsToDelete, namespace)
			if err != nil {
				return indexingResult, fmt.Errorf("failed to delete documents from vector store: %w", err)
			}
			recordManager.DeleteKeys(uidsToDelete)
			indexingResult.NumDeleted += len(*ids)
		}
	}
	return indexingResult, nil
}

func deleteDocuments(destination vectorstores.IndexableVectorStore, uidsToDelete []*string, namespace vectorstores.Option) (*[]string, error) {
	ids := make([]string, len(uidsToDelete))
	for i, uid := range uidsToDelete {
		ids[i] = *uid
	}
	ids, err := destination.DeleteDocuments(context.Background(), ids, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to delete documents (%v) from vector store: %w", ids, err)
	}
	return &ids, nil
}

func scopedFullCleanupSourceIdsToSlice(scopedFullCleanupSourceIds map[string]bool) []*string {
	var sourceIds []*string
	for sourceId := range scopedFullCleanupSourceIds {
		sourceIds = append(sourceIds, &sourceId)
	}
	return sourceIds
}

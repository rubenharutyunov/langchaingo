package indexing

import (
	"context"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/vectorstores"
	"testing"
	"time"
)

// MockVectorStore implements vectorstores.VectorStore interface for testing
type MockVectorStore struct {
	docs map[string]schema.Document
}

func NewMockVectorStore() *MockVectorStore {
	return &MockVectorStore{
		docs: make(map[string]schema.Document),
	}
}

func (m *MockVectorStore) AddDocuments(_ context.Context, docs []schema.Document, _ ...vectorstores.Option) ([]string, error) {
	for _, doc := range docs {
		id := doc.Metadata["id"].(string)
		m.docs[id] = doc
	}
	return nil, nil
}

func (m *MockVectorStore) DeleteDocuments(_ context.Context, ids []string, _ ...vectorstores.Option) ([]string, error) {
	var deletedIds []string
	for _, id := range ids {
		delete(m.docs, id)
		deletedIds = append(deletedIds, id)
	}
	return deletedIds, nil
}

func (m *MockVectorStore) SimilaritySearch(_ context.Context, _ string, _ int, _ ...vectorstores.Option) ([]schema.Document, error) {
	return nil, nil
}

func TestIndex(t *testing.T) {
	tests := []struct {
		name               string
		docs               []schema.Document
		existingDocs       []schema.Document
		opts               []Option
		expectedNumAdded   int
		expectedNumSkipped int
		expectedNumUpdated int
		expectedNumDeleted int
		expectedErr        bool
		expectedFinalDocs  []schema.Document
	}{
		{
			name: "basic indexing",
			docs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc2", Metadata: map[string]any{"source": "test2"}},
			},
			expectedNumAdded: 2,
			expectedFinalDocs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc2", Metadata: map[string]any{"source": "test2"}},
			},
		},
		{
			name: "deduplication",
			docs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}}, // Duplicate
			},
			expectedNumAdded: 1,
			expectedFinalDocs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
			},
		},
		{
			name: "force update existing document",
			docs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
			},
			existingDocs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
			},
			opts:               []Option{WithForceUpdate(true)},
			expectedNumUpdated: 1,
			expectedFinalDocs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
			},
		},
		{
			name: "skip existing document",
			docs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
			},
			existingDocs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
			},
			expectedNumSkipped: 1,
			expectedFinalDocs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
			},
		},
		{
			name: "incremental cleanup requires sourceIdKey",
			docs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{}},
			},
			opts:              []Option{WithCleanup(Incremental)},
			expectedErr:       true,
			expectedFinalDocs: []schema.Document{}, // Error case, no docs should be added
		},
		{
			name: "batch processing",
			docs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc2", Metadata: map[string]any{"source": "test2"}},
				{PageContent: "doc3", Metadata: map[string]any{"source": "test3"}},
			},
			opts:             []Option{WithBatchSize(2)},
			expectedNumAdded: 3,
			expectedFinalDocs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc2", Metadata: map[string]any{"source": "test2"}},
				{PageContent: "doc3", Metadata: map[string]any{"source": "test3"}},
			},
		},
		{
			name: "incremental cleanup with sourceIdKey",
			docs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
			},
			opts: []Option{
				WithCleanup(Incremental),
				WithSourceIdKey("source"),
			},
			expectedNumAdded: 1,
			expectedFinalDocs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
			},
		},
		{
			name: "duplicates in different batches",
			docs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc2", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc3", Metadata: map[string]any{"source": "test2"}},
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}}, // Duplicate
			},
			existingDocs: []schema.Document{
				{PageContent: "doc2", Metadata: map[string]any{"source": "test1"}},
			},
			opts: []Option{
				WithBatchSize(2),
				WithForceUpdate(true),
				WithSourceIdKey("source"),
			},
			expectedNumAdded:   2,
			expectedNumUpdated: 2,
			expectedFinalDocs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc2", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc3", Metadata: map[string]any{"source": "test2"}},
			},
		},
		{
			name: "duplicates in the same batch",
			docs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc2", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}}, // Duplicate
				{PageContent: "doc3", Metadata: map[string]any{"source": "test2"}},
			},
			existingDocs: []schema.Document{
				{PageContent: "doc2", Metadata: map[string]any{"source": "test1"}},
			},
			opts: []Option{
				WithBatchSize(3),
				WithForceUpdate(true),
				WithSourceIdKey("source"),
			},
			expectedNumAdded:   2,
			expectedNumUpdated: 1,
			expectedFinalDocs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc2", Metadata: map[string]any{"source": "test1"}},
				{PageContent: "doc3", Metadata: map[string]any{"source": "test2"}},
			},
		},
		{
			name: "full cleanup with multiple sources",
			docs: []schema.Document{
				{PageContent: "new1", Metadata: map[string]any{"source": "source1"}},
			},
			existingDocs: []schema.Document{
				{PageContent: "old1", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "old2", Metadata: map[string]any{"source": "source2"}},
			},
			opts: []Option{
				WithCleanup(Full),
				WithSourceIdKey("source"),
				WithCleanupBatchSize(1),
			},
			expectedNumAdded:   1,
			expectedNumDeleted: 2,
			expectedFinalDocs: []schema.Document{
				{PageContent: "new1", Metadata: map[string]any{"source": "source1"}},
			},
		},
		{
			name: "full cleanup with single source",
			docs: []schema.Document{
				{PageContent: "doc3", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "doc4", Metadata: map[string]any{"source": "source1"}},
			},
			existingDocs: []schema.Document{
				{PageContent: "doc1", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "doc2", Metadata: map[string]any{"source": "source1"}},
			},
			opts: []Option{
				WithCleanup(Full),
				WithSourceIdKey("source"),
				WithCleanupBatchSize(1),
			},
			expectedNumAdded:   2,
			expectedNumDeleted: 2,
			expectedFinalDocs: []schema.Document{
				{PageContent: "doc3", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "doc4", Metadata: map[string]any{"source": "source1"}},
			},
		},
		{
			name: "scoped full cleanup",
			docs: []schema.Document{
				{PageContent: "new1", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "new2", Metadata: map[string]any{"source": "source1"}},
			},
			existingDocs: []schema.Document{
				{PageContent: "old1", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "old2", Metadata: map[string]any{"source": "source2"}},
			},
			opts: []Option{
				WithCleanup(ScopedFull),
				WithSourceIdKey("source"),
			},
			expectedNumAdded:   2,
			expectedNumDeleted: 1,
			expectedFinalDocs: []schema.Document{
				{PageContent: "new1", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "new2", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "old2", Metadata: map[string]any{"source": "source2"}},
			},
		},
		{
			name: "incremental cleanup with multiple batches",
			docs: []schema.Document{
				{PageContent: "new1", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "new2", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "new3", Metadata: map[string]any{"source": "source2"}},
			},
			existingDocs: []schema.Document{
				{PageContent: "old1", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "old2", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "old3", Metadata: map[string]any{"source": "source2"}},
			},
			opts: []Option{
				WithBatchSize(3), //TODO: Change to 2
				WithCleanup(Incremental),
				WithSourceIdKey("source"),
			},
			expectedNumAdded:   3,
			expectedNumDeleted: 3,
			expectedFinalDocs: []schema.Document{
				{PageContent: "new1", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "new2", Metadata: map[string]any{"source": "source1"}},
				{PageContent: "new3", Metadata: map[string]any{"source": "source2"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := NewInMemoryRecordManager("test")
			vs := NewMockVectorStore()

			// Setup existing docs
			if len(tt.existingDocs) > 0 {
				past := time.Now().UnixNano() - int64(100*time.Millisecond)
				for _, doc := range tt.existingDocs {
					hashedDoc, err := fromDocument(doc)
					if err != nil {
						t.Fatalf("Failed to hash document: %v", err)
					}
					vs.docs[hashedDoc.UID] = doc
					sourceStr := doc.Metadata["source"].(string)
					if err := rm.Update([]string{hashedDoc.UID}, []*string{&sourceStr}, nil); err != nil {
						t.Fatalf("Failed to setup existing docs: %v", err)
					}
					// Artificially deduct UpdatedAt timestamps in records for predictable testing
					record := rm.records[hashedDoc.UID]
					record.UpdatedAt = past
					rm.records[hashedDoc.UID] = record
				}
			}
			// Run the index operation
			result, err := Index(tt.docs, rm, vs, nil, tt.opts...)

			// Verify error expectations
			if (err != nil) != tt.expectedErr {
				t.Errorf("Index() error = %v, expectedErr %v", err, tt.expectedErr)
				return
			}

			if err != nil {
				return
			}

			// Verify operation counts
			if result.NumAdded != tt.expectedNumAdded {
				t.Errorf("NumAdded = %v, expected %v", result.NumAdded, tt.expectedNumAdded)
			}
			if result.NumSkipped != tt.expectedNumSkipped {
				t.Errorf("NumSkipped = %v, expected %v", result.NumSkipped, tt.expectedNumSkipped)
			}
			if result.NumUpdated != tt.expectedNumUpdated {
				t.Errorf("NumUpdated = %v, expected %v", result.NumUpdated, tt.expectedNumUpdated)
			}
			if result.NumDeleted != tt.expectedNumDeleted {
				t.Errorf("NumDeleted = %v, expected %v", result.NumDeleted, tt.expectedNumDeleted)
			}

			// Verify final state of documents
			if len(vs.docs) != len(tt.expectedFinalDocs) {
				t.Errorf("Expected %d documents in store, got %d", len(tt.expectedFinalDocs), len(vs.docs))
			}

			// Create maps for easier comparison
			expectedDocs := make(map[string]map[string]any)
			for _, doc := range tt.expectedFinalDocs {
				expectedDocs[doc.PageContent] = doc.Metadata
			}

			actualDocs := make(map[string]map[string]any)
			for _, doc := range vs.docs {
				actualDocs[doc.PageContent] = doc.Metadata
			}

			// Verify each expected document exists with correct metadata
			for content, expectedMeta := range expectedDocs {
				actualMeta, exists := actualDocs[content]
				if !exists {
					t.Errorf("Expected document with content %q not found in store", content)
					continue
				}

				expectedSource := expectedMeta["source"].(string)
				actualSource, ok := actualMeta["source"].(string)
				if !ok {
					t.Errorf("Document %q: missing source in metadata", content)
					continue
				}
				if actualSource != expectedSource {
					t.Errorf("Document %q: expected source %q, got %q",
						content, expectedSource, actualSource)
				}
			}

			// Verify no unexpected documents
			for content := range actualDocs {
				if _, exists := expectedDocs[content]; !exists {
					t.Errorf("Unexpected document found in store: %q", content)
				}
			}
		})
	}
}

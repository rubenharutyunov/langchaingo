package indexing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/tmc/langchaingo/schema"
)

// hashedDocument represents a document with unique hashes for content and metadata.
type hashedDocument struct {
	UID          string
	Hash         string
	ContentHash  string
	MetadataHash string
	PageContent  string
	Metadata     map[string]any
}

// newHashedDocument creates a new hashedDocument and computes its hashes.
func newHashedDocument(pageContent string, metadata map[string]any) (*hashedDocument, error) {
	// Reserved metadata keys for validation
	forbiddenKeys := []string{"hash", "content_hash", "metadata_hash"}
	for _, key := range forbiddenKeys {
		if _, exists := metadata[key]; exists {
			return nil, fmt.Errorf("metadata cannot contain key '%s' as it is reserved for internal use", key)
		}
	}

	// Compute content hash
	contentHash := hashString(pageContent)

	// Compute metadata hash
	metadataHash, err := hashMetadata(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to hash metadata: %w", err)
	}

	// Compute overall document hash
	documentHash := hashString(contentHash + metadataHash)

	// Create the document with computed hashes
	// TODO: Simplify this
	doc := &hashedDocument{
		UID:          documentHash,
		Hash:         documentHash,
		ContentHash:  contentHash,
		MetadataHash: metadataHash,
		PageContent:  pageContent,
		Metadata:     metadata,
	}

	return doc, nil
}

// hashString computes an SHA-256 hash of the given string.
func hashString(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

// hashMetadata computes a hash for the given metadata map.
func hashMetadata(metadata map[string]any) (string, error) {
	// Create a sorted list of keys
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		if key != "id" { // Exclude the "id" field
			keys = append(keys, key)
		}
	}
	sort.Strings(keys) // Sort keys alphabetically

	// Create a new map in sorted order (to guarantee consistent JSON serialization)
	orderedMetadata := make(map[string]any, len(keys))
	for _, key := range keys {
		orderedMetadata[key] = metadata[key]
	}

	// Serialize ordered metadata to JSON
	data, err := json.Marshal(orderedMetadata)
	if err != nil {
		return "", errors.New("metadata must be JSON serializable")
	}
	return hashString(string(data)), nil
}

// toDocument returns a simplified Document struct from the hashedDocument.
func (doc *hashedDocument) toDocument() schema.Document {
	schemaDoc := schema.Document{
		PageContent: doc.PageContent,
		Metadata:    doc.Metadata,
	}
	// Set document id in document metadata to match record manager uid as addDocuments doesn't support `ids`
	schemaDoc.Metadata["id"] = doc.UID
	return schemaDoc
}

// fromDocument creates a hashedDocument from a given Document.
func fromDocument(document schema.Document) (*hashedDocument, error) {
	hashedDoc, err := newHashedDocument(document.PageContent, document.Metadata)
	if err != nil {
		return nil, err
	}
	return hashedDoc, nil
}

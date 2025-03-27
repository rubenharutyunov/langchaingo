package indexing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHashedDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pageContent string
		metadata    map[string]any
		expectError bool
	}{
		{
			name:        "valid document",
			pageContent: "test content",
			metadata:    map[string]any{"key": "value"},
			expectError: false,
		},
		{
			name:        "with forbidden metadata key",
			pageContent: "test content",
			metadata:    map[string]any{"hash": "reserved"},
			expectError: true,
		},
		{
			name:        "empty content and metadata",
			pageContent: "",
			metadata:    map[string]any{},
			expectError: false,
		},
		{
			name:        "non-serializable metadata",
			pageContent: "test content",
			metadata:    map[string]any{"key": make(chan int)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			doc, err := newHashedDocument(tt.pageContent, tt.metadata)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, doc.ContentHash)
			assert.NotEmpty(t, doc.MetadataHash)
		})
	}
}

func TestHashMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		metadata     map[string]any
		expectedHash string
		expectError  bool
	}{
		{
			name:         "valid metadata",
			metadata:     map[string]any{"key": "value"},
			expectedHash: "e43abcf3375244839c012f9633f95862d232a95b00d5bc7348b3098b9fed7f32",
			expectError:  false,
		},
		{
			name:         "valid metadata, multiple keys",
			metadata:     map[string]any{"firstKey": "value1", "secondKey": "value2"},
			expectedHash: "390a8edde1ba004696e2513fe95820c822b98da82b187c5b7da8884c4daf0ba7",
			expectError:  false,
		},
		{
			name:         "empty metadata",
			metadata:     map[string]any{},
			expectedHash: "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
			expectError:  false,
		},
		{
			name:        "non-serializable metadata",
			metadata:    map[string]any{"key": make(chan int)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hash, err := hashMetadata(tt.metadata)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedHash, hash)
		})
	}
}

func TestToDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		metadata    map[string]any
		expectError bool
	}{
		{
			name:        "with content and metadata",
			content:     "test content",
			metadata:    map[string]any{"key": "value"},
			expectError: false,
		},
		{
			name:        "with content and metadata with id",
			content:     "test content",
			metadata:    map[string]any{"id": "Will be ignored", "key": "value"},
			expectError: false,
		},
		{
			name:        "with content only",
			content:     "test content",
			metadata:    map[string]any{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hashedDoc, err := newHashedDocument(tt.content, tt.metadata)
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			doc := hashedDoc.toDocument()

			assert.Equal(t, hashedDoc.UID, doc.Metadata["id"])
			assert.Equal(t, tt.content, doc.PageContent)

			// Check metadata was properly transferred
			for k, v := range tt.metadata {
				if k != "id" { // ignore id as it's handled specially
					assert.Equal(t, v, doc.Metadata[k])
				}
			}
		})
	}
}

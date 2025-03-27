package indexing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		size        int
		input       []int
		expected    [][]int
		expectPanic bool
	}{
		{
			name:        "valid batch",
			size:        2,
			input:       []int{1, 2, 3, 4, 5},
			expected:    [][]int{{1, 2}, {3, 4}, {5}},
			expectPanic: false,
		},
		{
			name:        "exact size batch",
			size:        3,
			input:       []int{1, 2, 3},
			expected:    [][]int{{1, 2, 3}},
			expectPanic: false,
		},
		{
			name:        "batch size zero",
			size:        0,
			input:       []int{1, 2, 3},
			expectPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.expectPanic {
				assert.Panics(t, func() {
					batch(tt.size, tt.input)
				})
				return
			}

			result := batch(tt.size, tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeduplicateInOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []*hashedDocument
		expected []*hashedDocument
	}{
		{
			name:     "empty slice",
			input:    []*hashedDocument{},
			expected: []*hashedDocument{},
		},
		{
			name: "no duplicates",
			input: []*hashedDocument{
				{Hash: "hash1"},
				{Hash: "hash2"},
				{Hash: "hash3"},
			},
			expected: []*hashedDocument{
				{Hash: "hash1"},
				{Hash: "hash2"},
				{Hash: "hash3"},
			},
		},
		{
			name: "with duplicates",
			input: []*hashedDocument{
				{Hash: "hash1"},
				{Hash: "hash2"},
				{Hash: "hash1"}, // duplicate
				{Hash: "hash3"},
				{Hash: "hash2"}, // duplicate
			},
			expected: []*hashedDocument{
				{Hash: "hash1"},
				{Hash: "hash2"},
				{Hash: "hash3"},
			},
		},
		{
			name: "all duplicates",
			input: []*hashedDocument{
				{Hash: "hash1"},
				{Hash: "hash1"},
				{Hash: "hash1"},
			},
			expected: []*hashedDocument{
				{Hash: "hash1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := DeduplicateInOrder(tt.input)

			// Check length and content
			require.Equal(t, len(tt.expected), len(result))

			// Check each element's hash
			for i := range result {
				assert.Equal(t, tt.expected[i].Hash, result[i].Hash)
			}

			// Verify no duplicates in result
			seen := make(map[string]bool)
			for _, doc := range result {
				assert.False(t, seen[doc.Hash], "duplicate hash found: %s", doc.Hash)
				seen[doc.Hash] = true
			}
		})
	}
}

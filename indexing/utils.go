package indexing

// batch splits a slice into smaller chunks of the given size.
// Returns a slice of slices.
func batch[T any](size int, input []T) [][]T {
	if size <= 0 {
		panic("batch size must be greater than 0")
	}

	var result [][]T
	for i := 0; i < len(input); i += size {
		end := i + size
		if end > len(input) {
			end = len(input)
		}
		result = append(result, input[i:end])
	}
	return result
}

// DeduplicateInOrder Deduplicate a slice of hashed documents while preserving order.
func DeduplicateInOrder(hashedDocs []*hashedDocument) []*hashedDocument {
	seen := make(map[string]bool)
	var result []*hashedDocument
	for _, hashedDoc := range hashedDocs {
		if _, ok := seen[hashedDoc.Hash]; !ok {
			seen[hashedDoc.Hash] = true
			result = append(result, hashedDoc)
		}
	}
	return result
}

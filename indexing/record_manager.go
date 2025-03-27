package indexing

import (
	"errors"
	"fmt"
	"time"
)

// RecordManager defines a common interface for managing records.
type RecordManager interface {
	// GetTime returns the current server time as a high-resolution timestamp (in seconds).
	GetTime() (int64, error)

	// Update adds or updates records in memory. It updates the given keys with corresponding group IDs.
	// It enforces time constraints if `timeAtLeast` is provided.
	Update(keys []string, groupIDs []*string, timeAtLeast *int64) error

	// Exists checks if the given keys exist in the records.
	Exists(keys []string) ([]bool, error)

	// ListKeys filters and returns a list of keys matching the given filters (timestamps, group IDs, limit).
	ListKeys(before, after *int64, groupIDs []*string, limit *int) ([]*string, error)

	// DeleteKeys removes records with the specified keys.
	DeleteKeys(keys []*string) error
}

type Record struct {
	GroupID   *string // GroupID is optional and can be nil.
	UpdatedAt int64   // UpdatedAt holds the timestamp of when the record was last updated.
}

// InMemoryRecordManager holds all the records in memory and provides methods to manage them.
type InMemoryRecordManager struct {
	namespace string            // The namespace for the record manager.
	records   map[string]Record // A map to store records, keyed by their unique keys (strings).
}

// NewInMemoryRecordManager initializes a new in-memory record manager with a given namespace.
func NewInMemoryRecordManager(namespace string) *InMemoryRecordManager {
	return &InMemoryRecordManager{
		namespace: namespace,
		records:   make(map[string]Record), // Initialize the records map.
	}
}

// GetTime returns the current server time as a high-resolution timestamp (in microseconds).
func (rm *InMemoryRecordManager) GetTime() (int64, error) {
	return time.Now().UnixNano(), nil
}

// Update adds or updates records in memory. It upserts the given keys with corresponding group IDs.
func (rm *InMemoryRecordManager) Update(keys []string, groupIDs []*string, timeAtLeast *int64) error {
	// Input validation
	if keys == nil {
		return errors.New("keys cannot be nil")
	}

	if len(keys) == 0 {
		return errors.New("keys cannot be empty")
	}

	// Check if the lengths of keys and groupIDs match
	if groupIDs != nil && len(keys) != len(groupIDs) {
		return errors.New("length of keys must match length of group_ids")
	}

	// Check timestamp validity
	currentTime, _ := rm.GetTime()
	if timeAtLeast != nil {
		if *timeAtLeast > currentTime {
			return fmt.Errorf("time_at_least (%d) must be in the past (current time: %d)", *timeAtLeast, currentTime)
		}
	}

	// Validate individual keys
	for _, key := range keys {
		if key == "" {
			return errors.New("empty keys are not allowed")
		}
	}

	// Update records
	for i, key := range keys {
		var groupID *string
		if groupIDs != nil {
			groupID = groupIDs[i]
		}

		updateTime := currentTime
		if timeAtLeast != nil && updateTime < *timeAtLeast {
			updateTime = *timeAtLeast
		}

		rm.records[key] = Record{
			GroupID:   groupID,
			UpdatedAt: updateTime,
		}
	}

	return nil
}

// Exists checks if the given keys exist in the records map.
func (rm *InMemoryRecordManager) Exists(keys []string) ([]bool, error) {
	if keys == nil {
		return nil, errors.New("keys cannot be nil")
	}

	existence := make([]bool, len(keys))
	for i, key := range keys {
		_, exists := rm.records[key]
		existence[i] = exists
	}

	return existence, nil
}

// ListKeys returns a list of keys that match the given filters based on timestamps and group IDs.
func (rm *InMemoryRecordManager) ListKeys(before, after *int64, groupIDs []*string, limit *int) ([]*string, error) {
	var result []*string

	if before != nil && after != nil && *before <= *after {
		return nil, errors.New("'before' timestamp must be greater than 'after' timestamp")
	}

	// Convert groupIDs to a map for faster lookup
	groupIDMap := make(map[string]bool)
	if groupIDs != nil {
		for _, gID := range groupIDs {
			if gID != nil {
				groupIDMap[*gID] = true
			}
		}
	}

	for key, data := range rm.records {
		// Apply filters
		if before != nil && data.UpdatedAt >= *before {
			continue
		}
		if after != nil && data.UpdatedAt <= *after {
			continue
		}
		if groupIDs != nil && (data.GroupID == nil || !groupIDMap[*data.GroupID]) {
			continue
		}

		result = append(result, &key)

		// Apply limit if specified
		if limit != nil && len(result) >= *limit {
			break
		}
	}

	return result, nil
}

// DeleteKeys removes records with the specified keys.
func (rm *InMemoryRecordManager) DeleteKeys(keys []*string) error {
	if keys == nil {
		return errors.New("keys cannot be nil")
	}

	for _, key := range keys {
		if key == nil {
			return errors.New("key cannot be nil")
		}
		delete(rm.records, *key)
	}

	return nil
}

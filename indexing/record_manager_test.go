package indexing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryRecordManager_Update(t *testing.T) {
	t.Parallel()

	group1 := "group1"
	group2 := "group2"
	futureTime := time.Now().Add(time.Millisecond).UnixNano()

	tests := []struct {
		name         string
		keys         []string
		groupIDs     []*string
		timeAtLeast  *int64
		errorPattern string
	}{
		{
			name:         "keys_mismatch_groupIDs",
			keys:         []string{"key1"},
			groupIDs:     []*string{&group1, &group2},
			timeAtLeast:  nil,
			errorPattern: "length of keys must match length of group_ids",
		},
		{
			name:         "time_at_least_in_the_future",
			keys:         []string{"key1"},
			groupIDs:     []*string{&group1},
			timeAtLeast:  &futureTime,
			errorPattern: `time_at_least \(\d+\) must be in the past \(current time: \d+\)`,
		},
		{
			name:         "successful_update",
			keys:         []string{"key1", "key2"},
			groupIDs:     []*string{&group1, &group2},
			timeAtLeast:  nil,
			errorPattern: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rm := NewInMemoryRecordManager("testNamespace")

			err := rm.Update(tt.keys, tt.groupIDs, tt.timeAtLeast)
			if tt.errorPattern != "" {
				require.Error(t, err)
				assert.Regexp(t, tt.errorPattern, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInMemoryRecordManager_Exists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupKeys []string
		checkKeys []string
		expected  []bool
	}{
		{
			name:      "all_keys_exist",
			setupKeys: []string{"key1", "key2"},
			checkKeys: []string{"key1", "key2"},
			expected:  []bool{true, true},
		},
		{
			name:      "some_keys_exist",
			setupKeys: []string{"key1", "key2"},
			checkKeys: []string{"key1", "key3"},
			expected:  []bool{true, false},
		},
		{
			name:      "no_keys_exist",
			setupKeys: []string{"key1", "key2"},
			checkKeys: []string{"key3", "key4"},
			expected:  []bool{false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rm := NewInMemoryRecordManager("testNamespace")
			group1 := "group1"
			groups := make([]*string, len(tt.setupKeys))
			for i := range groups {
				groups[i] = &group1
			}

			require.NoError(t, rm.Update(tt.setupKeys, groups, nil))

			result, err := rm.Exists(tt.checkKeys)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInMemoryRecordManager_ListKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFunc func(*InMemoryRecordManager)
		before    *int64
		after     *int64
		groupIDs  []*string
		limit     *int
		expected  int // expected number of keys
	}{
		{
			name: "all_keys",
			setupFunc: func(rm *InMemoryRecordManager) {
				group1, group2 := "group1", "group2"
				require.NoError(t, rm.Update(
					[]string{"key1", "key2"},
					[]*string{&group1, &group2},
					nil,
				))
			},
			expected: 2,
		},
		{
			name: "filtered_by_group",
			setupFunc: func(rm *InMemoryRecordManager) {
				group1, group2 := "group1", "group2"
				require.NoError(t, rm.Update(
					[]string{"key1", "key2"},
					[]*string{&group1, &group2},
					nil,
				))
			},
			groupIDs: []*string{func() *string { s := "group1"; return &s }()},
			expected: 1,
		},
		{
			name: "with_limit",
			setupFunc: func(rm *InMemoryRecordManager) {
				group1, group2 := "group1", "group2"
				require.NoError(t, rm.Update(
					[]string{"key1", "key2"},
					[]*string{&group1, &group2},
					nil,
				))
			},
			limit:    func() *int { l := 1; return &l }(),
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rm := NewInMemoryRecordManager("testNamespace")
			tt.setupFunc(rm)

			result, err := rm.ListKeys(tt.before, tt.after, tt.groupIDs, tt.limit)
			if err != nil {
				t.Fatal(err)
			}
			assert.Len(t, result, tt.expected)
		})
	}
}

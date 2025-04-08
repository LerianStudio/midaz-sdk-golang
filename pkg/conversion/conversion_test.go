package conversion_test

import (
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/pkg/conversion"
	"github.com/stretchr/testify/assert"
)

func TestConvertToISODate(t *testing.T) {
	// Create a fixed time for testing
	testTime := time.Date(2025, 4, 2, 15, 4, 5, 0, time.UTC)
	expected := "2025-04-02"

	result := conversion.ConvertToISODate(testTime)
	assert.Equal(t, expected, result)
}

func TestConvertToISODateTime(t *testing.T) {
	// Create a fixed time for testing
	testTime := time.Date(2025, 4, 2, 15, 4, 5, 0, time.UTC)
	expected := "2025-04-02T15:04:05Z"

	result := conversion.ConvertToISODateTime(testTime)
	assert.Equal(t, expected, result)
}

func TestConvertMetadataToTags(t *testing.T) {
	testCases := []struct {
		name     string
		metadata map[string]any
		expected []string
	}{
		{
			name:     "Nil metadata",
			metadata: nil,
			expected: nil,
		},
		{
			name:     "Empty metadata",
			metadata: map[string]any{},
			expected: nil,
		},
		{
			name: "Metadata with empty tags",
			metadata: map[string]any{
				"tags": "",
			},
			expected: []string{},
		},
		{
			name: "Metadata with single tag",
			metadata: map[string]any{
				"tags": "payment",
			},
			expected: []string{"payment"},
		},
		{
			name: "Metadata with multiple tags",
			metadata: map[string]any{
				"tags": "payment,recurring,subscription",
			},
			expected: []string{"payment", "recurring", "subscription"},
		},
		{
			name: "Metadata with tags containing whitespace",
			metadata: map[string]any{
				"tags": " payment , recurring , subscription ",
			},
			expected: []string{"payment", "recurring", "subscription"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := conversion.ConvertMetadataToTags(tc.metadata)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestConvertTagsToMetadata(t *testing.T) {
	testCases := []struct {
		name     string
		metadata map[string]any
		tags     []string
		expected map[string]any
	}{
		{
			name:     "Empty tags with nil metadata",
			metadata: nil,
			tags:     []string{},
			expected: nil,
		},
		{
			name: "Empty tags with existing metadata",
			metadata: map[string]any{
				"reference": "INV-123",
			},
			tags: []string{},
			expected: map[string]any{
				"reference": "INV-123",
			},
		},
		{
			name:     "Single tag with nil metadata",
			metadata: nil,
			tags:     []string{"payment"},
			expected: map[string]any{"tags": "payment"},
		},
		{
			name: "Multiple tags with existing metadata",
			metadata: map[string]any{
				"reference": "INV-123",
				"amount":    100.50,
			},
			tags: []string{"payment", "recurring", "subscription"},
			expected: map[string]any{
				"reference": "INV-123",
				"amount":    100.50,
				"tags":      "payment,recurring,subscription",
			},
		},
		{
			name: "Tags with whitespace",
			metadata: map[string]any{
				"reference": "INV-123",
			},
			tags: []string{" payment ", " recurring ", " subscription "},
			expected: map[string]any{
				"reference": "INV-123",
				"tags":      "payment,recurring,subscription",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := conversion.ConvertTagsToMetadata(tc.metadata, tc.tags)
			assert.Equal(t, tc.expected, result)
		})
	}
}

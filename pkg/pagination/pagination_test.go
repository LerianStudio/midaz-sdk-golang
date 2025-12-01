package pagination

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPaginationObserver(_ *testing.T) {
	// Create a new observer
	observer := NewObserver()

	// Create a pagination event
	event := &Event{
		Operation:      "ListAccounts",
		EntityType:     "accounts",
		Limit:          10,
		Offset:         0,
		Page:           1,
		CursorUsed:     false,
		TotalItems:     100,
		ProcessedItems: 10,
		Duration:       time.Millisecond * 100,
		HasNextPage:    true,
		Error:          nil,
	}

	// This should not panic
	observer.RecordEvent(context.Background(), event)

	// Test with error
	event.Error = errors.New("test error")
	observer.RecordEvent(context.Background(), event)
}

func TestGetEntityTypeFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "API URL with v1",
			url:      "https://api.example.com/v1/accounts",
			expected: "accounts",
		},
		{
			name:     "API URL with v2",
			url:      "https://api.example.com/v2/transactions",
			expected: "transactions",
		},
		{
			name:     "API URL with version number and path",
			url:      "https://api.example.com/v1/organizations/123/accounts",
			expected: "organizations",
		},
		{
			name:     "No version in URL",
			url:      "https://api.example.com/accounts",
			expected: "unknown",
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetEntityTypeFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("GetEntityTypeFromURL(%s) = %s, want %s", tt.url, result, tt.expected)
			}
		})
	}
}

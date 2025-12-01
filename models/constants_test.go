package models

import (
	"testing"
)

func TestTransactionStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "TransactionStatusPending",
			constant: TransactionStatusPending,
			expected: "pending",
		},
		{
			name:     "TransactionStatusCompleted",
			constant: TransactionStatusCompleted,
			expected: "completed",
		},
		{
			name:     "TransactionStatusFailed",
			constant: TransactionStatusFailed,
			expected: "failed",
		},
		{
			name:     "TransactionStatusCancelled",
			constant: TransactionStatusCancelled,
			expected: "cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s to be %s, got %s", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

func TestAccountStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "StatusActive",
			constant: StatusActive,
			expected: "ACTIVE",
		},
		{
			name:     "StatusInactive",
			constant: StatusInactive,
			expected: "INACTIVE",
		},
		{
			name:     "StatusPending",
			constant: StatusPending,
			expected: "PENDING",
		},
		{
			name:     "StatusClosed",
			constant: StatusClosed,
			expected: "CLOSED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s to be %s, got %s", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

func TestSortDirectionConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant SortDirection
		expected string
	}{
		{
			name:     "SortAscending",
			constant: SortAscending,
			expected: "asc",
		},
		{
			name:     "SortDescending",
			constant: SortDescending,
			expected: "desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("Expected %s to be %s, got %s", tt.name, tt.expected, string(tt.constant))
			}
		})
	}
}

func TestPaginationDefaults(t *testing.T) {
	tests := []struct {
		name     string
		constant int
		expected int
	}{
		{
			name:     "DefaultLimit",
			constant: DefaultLimit,
			expected: 10,
		},
		{
			name:     "MaxLimit",
			constant: MaxLimit,
			expected: 100,
		},
		{
			name:     "DefaultOffset",
			constant: DefaultOffset,
			expected: 0,
		},
		{
			name:     "DefaultPage",
			constant: DefaultPage,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s to be %d, got %d", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

func TestDefaultSortDirection(t *testing.T) {
	if DefaultSortDirection != string(SortDescending) {
		t.Errorf("Expected DefaultSortDirection to be %s, got %s", string(SortDescending), DefaultSortDirection)
	}
}

func TestQueryParamNames(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "QueryParamLimit",
			constant: QueryParamLimit,
			expected: "limit",
		},
		{
			name:     "QueryParamOffset",
			constant: QueryParamOffset,
			expected: "offset",
		},
		{
			name:     "QueryParamPage",
			constant: QueryParamPage,
			expected: "page",
		},
		{
			name:     "QueryParamCursor",
			constant: QueryParamCursor,
			expected: "cursor",
		},
		{
			name:     "QueryParamOrderBy",
			constant: QueryParamOrderBy,
			expected: "orderBy",
		},
		{
			name:     "QueryParamOrderDirection",
			constant: QueryParamOrderDirection,
			expected: "orderDirection",
		},
		{
			name:     "QueryParamStartDate",
			constant: QueryParamStartDate,
			expected: "startDate",
		},
		{
			name:     "QueryParamEndDate",
			constant: QueryParamEndDate,
			expected: "endDate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s to be %s, got %s", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

func TestStatusConstantsUniqueness(t *testing.T) {
	// Ensure that all status constants are unique
	statuses := []string{
		StatusActive,
		StatusInactive,
		StatusPending,
		StatusClosed,
	}

	seen := make(map[string]bool)
	for _, status := range statuses {
		if seen[status] {
			t.Errorf("Duplicate status constant found: %s", status)
		}

		seen[status] = true
	}

	if len(seen) != len(statuses) {
		t.Errorf("Expected %d unique status constants, got %d", len(statuses), len(seen))
	}
}

func TestTransactionStatusConstantsUniqueness(t *testing.T) {
	// Ensure that all transaction status constants are unique
	statuses := []string{
		TransactionStatusPending,
		TransactionStatusCompleted,
		TransactionStatusFailed,
		TransactionStatusCancelled,
	}

	seen := make(map[string]bool)
	for _, status := range statuses {
		if seen[status] {
			t.Errorf("Duplicate transaction status constant found: %s", status)
		}

		seen[status] = true
	}

	if len(seen) != len(statuses) {
		t.Errorf("Expected %d unique transaction status constants, got %d", len(statuses), len(seen))
	}
}

func TestQueryParamConstantsUniqueness(t *testing.T) {
	// Ensure that all query param constants are unique
	params := []string{
		QueryParamLimit,
		QueryParamOffset,
		QueryParamPage,
		QueryParamCursor,
		QueryParamOrderBy,
		QueryParamOrderDirection,
		QueryParamStartDate,
		QueryParamEndDate,
	}

	seen := make(map[string]bool)
	for _, param := range params {
		if seen[param] {
			t.Errorf("Duplicate query param constant found: %s", param)
		}

		seen[param] = true
	}

	if len(seen) != len(params) {
		t.Errorf("Expected %d unique query param constants, got %d", len(params), len(seen))
	}
}

func TestPaginationDefaultsConsistency(t *testing.T) {
	// Ensure pagination defaults are consistent and logical
	if DefaultLimit <= 0 {
		t.Error("DefaultLimit should be greater than 0")
	}

	if MaxLimit <= DefaultLimit {
		t.Error("MaxLimit should be greater than DefaultLimit")
	}

	if DefaultOffset < 0 {
		t.Error("DefaultOffset should be non-negative")
	}

	if DefaultPage < 1 {
		t.Error("DefaultPage should be at least 1")
	}

	// DefaultSortDirection should be either ascending or descending
	if DefaultSortDirection != string(SortAscending) && DefaultSortDirection != string(SortDescending) {
		t.Errorf("DefaultSortDirection should be either %s or %s, got %s",
			string(SortAscending), string(SortDescending), DefaultSortDirection)
	}
}

func TestSortDirectionStringConversion(t *testing.T) {
	// Test that SortDirection type can be converted to string properly
	asc := SortAscending
	desc := SortDescending

	if string(asc) != "asc" {
		t.Errorf("Expected SortAscending to convert to 'asc', got '%s'", string(asc))
	}

	if string(desc) != "desc" {
		t.Errorf("Expected SortDescending to convert to 'desc', got '%s'", string(desc))
	}
}

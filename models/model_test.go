package models

import (
	"testing"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

func TestNewStatus(t *testing.T) {
	tests := []struct {
		name string
		code string
		want Status
	}{
		{
			name: "active status",
			code: "ACTIVE",
			want: Status{Code: "ACTIVE"},
		},
		{
			name: "pending status",
			code: "PENDING",
			want: Status{Code: "PENDING"},
		},
		{
			name: "empty status",
			code: "",
			want: Status{Code: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewStatus(tt.code)
			if got.Code != tt.want.Code {
				t.Errorf("NewStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithStatusDescription(t *testing.T) {
	status := NewStatus("ACTIVE")
	description := "This is an active status"

	result := WithStatusDescription(status, description)

	if result.Description == nil {
		t.Error("Expected description to be set, but it was nil")
		return
	}

	if *result.Description != description {
		t.Errorf("Expected description to be %s, got %s", description, *result.Description)
	}
}

func TestIsStatusEmpty(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "empty status",
			status: Status{},
			want:   true,
		},
		{
			name:   "status with code only",
			status: Status{Code: "ACTIVE"},
			want:   false,
		},
		{
			name:   "status with description only",
			status: Status{Description: stringPtr("Test")},
			want:   false,
		},
		{
			name: "status with both code and description",
			status: Status{
				Code:        "ACTIVE",
				Description: stringPtr("Test"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStatusEmpty(tt.status); got != tt.want {
				t.Errorf("IsStatusEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewAddress(t *testing.T) {
	line1 := "123 Main St"
	zipCode := "12345"
	city := "New York"
	state := "NY"
	country := "US"

	addr := NewAddress(line1, zipCode, city, state, country)

	if addr.Line1 != line1 {
		t.Errorf("Expected Line1 to be %s, got %s", line1, addr.Line1)
	}
	if addr.ZipCode != zipCode {
		t.Errorf("Expected ZipCode to be %s, got %s", zipCode, addr.ZipCode)
	}
	if addr.City != city {
		t.Errorf("Expected City to be %s, got %s", city, addr.City)
	}
	if addr.State != state {
		t.Errorf("Expected State to be %s, got %s", state, addr.State)
	}
	if addr.Country != country {
		t.Errorf("Expected Country to be %s, got %s", country, addr.Country)
	}
	if addr.Line2 != nil {
		t.Errorf("Expected Line2 to be nil, got %v", addr.Line2)
	}
}

func TestAddressWithLine2(t *testing.T) {
	addr := NewAddress("123 Main St", "12345", "New York", "NY", "US")
	line2 := "Apt 4B"

	result := addr.WithLine2(line2)

	if result.Line2 == nil {
		t.Error("Expected Line2 to be set, but it was nil")
		return
	}

	if *result.Line2 != line2 {
		t.Errorf("Expected Line2 to be %s, got %s", line2, *result.Line2)
	}
}

func TestAddressToMmodelAddress(t *testing.T) {
	line2 := "Apt 4B"
	addr := Address{
		Line1:   "123 Main St",
		Line2:   &line2,
		ZipCode: "12345",
		City:    "New York",
		State:   "NY",
		Country: "US",
	}

	mAddr := addr.ToMmodelAddress()

	if mAddr.Line1 != addr.Line1 {
		t.Errorf("Expected Line1 to be %s, got %s", addr.Line1, mAddr.Line1)
	}
	if mAddr.Line2 == nil || *mAddr.Line2 != *addr.Line2 {
		t.Errorf("Expected Line2 to be %v, got %v", addr.Line2, mAddr.Line2)
	}
	if mAddr.ZipCode != addr.ZipCode {
		t.Errorf("Expected ZipCode to be %s, got %s", addr.ZipCode, mAddr.ZipCode)
	}
	if mAddr.City != addr.City {
		t.Errorf("Expected City to be %s, got %s", addr.City, mAddr.City)
	}
	if mAddr.State != addr.State {
		t.Errorf("Expected State to be %s, got %s", addr.State, mAddr.State)
	}
	if mAddr.Country != addr.Country {
		t.Errorf("Expected Country to be %s, got %s", addr.Country, mAddr.Country)
	}
}

func TestFromMmodelAddress(t *testing.T) {
	line2 := "Apt 4B"
	mAddr := mmodel.Address{
		Line1:   "123 Main St",
		Line2:   &line2,
		ZipCode: "12345",
		City:    "New York",
		State:   "NY",
		Country: "US",
	}

	addr := FromMmodelAddress(mAddr)

	if addr.Line1 != mAddr.Line1 {
		t.Errorf("Expected Line1 to be %s, got %s", mAddr.Line1, addr.Line1)
	}
	if addr.Line2 == nil || *addr.Line2 != *mAddr.Line2 {
		t.Errorf("Expected Line2 to be %v, got %v", mAddr.Line2, addr.Line2)
	}
	if addr.ZipCode != mAddr.ZipCode {
		t.Errorf("Expected ZipCode to be %s, got %s", mAddr.ZipCode, addr.ZipCode)
	}
	if addr.City != mAddr.City {
		t.Errorf("Expected City to be %s, got %s", mAddr.City, addr.City)
	}
	if addr.State != mAddr.State {
		t.Errorf("Expected State to be %s, got %s", mAddr.State, addr.State)
	}
	if addr.Country != mAddr.Country {
		t.Errorf("Expected Country to be %s, got %s", mAddr.Country, addr.Country)
	}
}

func TestPaginationHasMethods(t *testing.T) {
	tests := []struct {
		name       string
		pagination Pagination
		hasMore    bool
		hasPrev    bool
		hasNext    bool
	}{
		{
			name: "first page with more pages",
			pagination: Pagination{
				Limit:  10,
				Offset: 0,
				Total:  100,
			},
			hasMore: true,
			hasPrev: false,
			hasNext: true,
		},
		{
			name: "middle page",
			pagination: Pagination{
				Limit:  10,
				Offset: 20,
				Total:  100,
			},
			hasMore: true,
			hasPrev: true,
			hasNext: true,
		},
		{
			name: "last page",
			pagination: Pagination{
				Limit:  10,
				Offset: 90,
				Total:  100,
			},
			hasMore: false,
			hasPrev: true,
			hasNext: false,
		},
		{
			name: "single page",
			pagination: Pagination{
				Limit:  10,
				Offset: 0,
				Total:  5,
			},
			hasMore: false,
			hasPrev: false,
			hasNext: false,
		},
		{
			name: "with cursors",
			pagination: Pagination{
				Limit:      10,
				Offset:     0,
				Total:      5,
				PrevCursor: "prev",
				NextCursor: "next",
			},
			hasMore: false,
			hasPrev: true,
			hasNext: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pagination.HasMorePages(); got != tt.hasMore {
				t.Errorf("HasMorePages() = %v, want %v", got, tt.hasMore)
			}
			if got := tt.pagination.HasPrevPage(); got != tt.hasPrev {
				t.Errorf("HasPrevPage() = %v, want %v", got, tt.hasPrev)
			}
			if got := tt.pagination.HasNextPage(); got != tt.hasNext {
				t.Errorf("HasNextPage() = %v, want %v", got, tt.hasNext)
			}
		})
	}
}

func TestPaginationCurrentPageTotalPages(t *testing.T) {
	tests := []struct {
		name        string
		pagination  Pagination
		currentPage int
		totalPages  int
	}{
		{
			name: "first page",
			pagination: Pagination{
				Limit:  10,
				Offset: 0,
				Total:  100,
			},
			currentPage: 1,
			totalPages:  10,
		},
		{
			name: "middle page",
			pagination: Pagination{
				Limit:  10,
				Offset: 25,
				Total:  100,
			},
			currentPage: 3,
			totalPages:  10,
		},
		{
			name: "with remainder",
			pagination: Pagination{
				Limit:  10,
				Offset: 0,
				Total:  95,
			},
			currentPage: 1,
			totalPages:  10,
		},
		{
			name: "zero limit",
			pagination: Pagination{
				Limit:  0,
				Offset: 0,
				Total:  100,
			},
			currentPage: 1,
			totalPages:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pagination.CurrentPage(); got != tt.currentPage {
				t.Errorf("CurrentPage() = %v, want %v", got, tt.currentPage)
			}
			if got := tt.pagination.TotalPages(); got != tt.totalPages {
				t.Errorf("TotalPages() = %v, want %v", got, tt.totalPages)
			}
		})
	}
}

func TestPaginationNextPageOptions(t *testing.T) {
	tests := []struct {
		name       string
		pagination Pagination
		wantNil    bool
		wantOffset int
		wantCursor string
		wantLimit  int
	}{
		{
			name: "has next page with offset",
			pagination: Pagination{
				Limit:  10,
				Offset: 0,
				Total:  100,
			},
			wantNil:    false,
			wantOffset: 10,
			wantLimit:  10,
		},
		{
			name: "has next page with cursor",
			pagination: Pagination{
				Limit:      10,
				Offset:     0,
				Total:      5,
				NextCursor: "next-cursor",
			},
			wantNil:    false,
			wantCursor: "next-cursor",
			wantLimit:  10,
		},
		{
			name: "no next page",
			pagination: Pagination{
				Limit:  10,
				Offset: 90,
				Total:  100,
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := tt.pagination.NextPageOptions()

			if tt.wantNil {
				if options != nil {
					t.Error("Expected nil but got options")
				}
				return
			}

			if options == nil {
				t.Error("Expected options but got nil")
				return
			}

			if options.Limit != tt.wantLimit {
				t.Errorf("Expected Limit to be %d, got %d", tt.wantLimit, options.Limit)
			}

			if tt.wantCursor != "" {
				if options.Cursor != tt.wantCursor {
					t.Errorf("Expected Cursor to be %s, got %s", tt.wantCursor, options.Cursor)
				}
			} else {
				if options.Offset != tt.wantOffset {
					t.Errorf("Expected Offset to be %d, got %d", tt.wantOffset, options.Offset)
				}
			}
		})
	}
}

func TestNewListOptions(t *testing.T) {
	options := NewListOptions()

	if options.Limit != DefaultLimit {
		t.Errorf("Expected Limit to be %d, got %d", DefaultLimit, options.Limit)
	}
	if options.Offset != DefaultOffset {
		t.Errorf("Expected Offset to be %d, got %d", DefaultOffset, options.Offset)
	}
	if options.OrderDirection != DefaultSortDirection {
		t.Errorf("Expected OrderDirection to be %s, got %s", DefaultSortDirection, options.OrderDirection)
	}
}

func TestListOptionsWithLimit(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		wantLimit int
	}{
		{
			name:      "valid limit",
			limit:     25,
			wantLimit: 25,
		},
		{
			name:      "zero limit defaults to default",
			limit:     0,
			wantLimit: DefaultLimit,
		},
		{
			name:      "negative limit defaults to default",
			limit:     -5,
			wantLimit: DefaultLimit,
		},
		{
			name:      "limit exceeding max gets capped",
			limit:     150,
			wantLimit: MaxLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewListOptions().WithLimit(tt.limit)
			if options.Limit != tt.wantLimit {
				t.Errorf("Expected Limit to be %d, got %d", tt.wantLimit, options.Limit)
			}
		})
	}
}

func TestListOptionsWithOffset(t *testing.T) {
	tests := []struct {
		name       string
		offset     int
		wantOffset int
	}{
		{
			name:       "valid offset",
			offset:     25,
			wantOffset: 25,
		},
		{
			name:       "zero offset",
			offset:     0,
			wantOffset: 0,
		},
		{
			name:       "negative offset defaults to default",
			offset:     -5,
			wantOffset: DefaultOffset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewListOptions().WithOffset(tt.offset)
			if options.Offset != tt.wantOffset {
				t.Errorf("Expected Offset to be %d, got %d", tt.wantOffset, options.Offset)
			}
		})
	}
}

func TestListOptionsWithPage(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		wantPage int
	}{
		{
			name:     "valid page",
			page:     5,
			wantPage: 5,
		},
		{
			name:     "zero page defaults to default",
			page:     0,
			wantPage: DefaultPage,
		},
		{
			name:     "negative page defaults to default",
			page:     -1,
			wantPage: DefaultPage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewListOptions().WithPage(tt.page)
			if options.Page != tt.wantPage {
				t.Errorf("Expected Page to be %d, got %d", tt.wantPage, options.Page)
			}
		})
	}
}

func TestListOptionsWithFilter(t *testing.T) {
	options := NewListOptions()

	// Add a filter
	options.WithFilter("status", "active")

	if options.Filters == nil {
		t.Error("Expected Filters map to be initialized")
		return
	}

	if value, exists := options.Filters["status"]; !exists || value != "active" {
		t.Errorf("Expected filter status=active, got %v", options.Filters)
	}

	// Add another filter
	options.WithFilter("type", "user")

	if len(options.Filters) != 2 {
		t.Errorf("Expected 2 filters, got %d", len(options.Filters))
	}
}

func TestListOptionsWithFilters(t *testing.T) {
	options := NewListOptions()
	filters := map[string]string{
		"status": "active",
		"type":   "user",
	}

	options.WithFilters(filters)

	if len(options.Filters) != 2 {
		t.Errorf("Expected 2 filters, got %d", len(options.Filters))
	}

	if options.Filters["status"] != "active" {
		t.Errorf("Expected status filter to be active, got %s", options.Filters["status"])
	}
}

func TestListOptionsToQueryParams(t *testing.T) {
	options := NewListOptions().
		WithLimit(25).
		WithOffset(10).
		WithOrderBy("name").
		WithOrderDirection(SortAscending).
		WithFilter("status", "active").
		WithDateRange("2023-01-01", "2023-12-31").
		WithAdditionalParam("custom", "value")

	params := options.ToQueryParams()

	expectedParams := map[string]string{
		QueryParamLimit:          "25",
		QueryParamOffset:         "10",
		QueryParamOrderBy:        "name",
		QueryParamOrderDirection: string(SortAscending),
		QueryParamStartDate:      "2023-01-01",
		QueryParamEndDate:        "2023-12-31",
		"status":                 "active",
		"custom":                 "value",
	}

	for key, expectedValue := range expectedParams {
		if actualValue, exists := params[key]; !exists || actualValue != expectedValue {
			t.Errorf("Expected %s=%s, got %s=%s", key, expectedValue, key, actualValue)
		}
	}
}

func TestObjectWithMetadataHasMetadata(t *testing.T) {
	tests := []struct {
		name     string
		obj      ObjectWithMetadata
		expected bool
	}{
		{
			name:     "no metadata",
			obj:      ObjectWithMetadata{},
			expected: false,
		},
		{
			name: "empty metadata map",
			obj: ObjectWithMetadata{
				Metadata: map[string]any{},
			},
			expected: false,
		},
		{
			name: "has metadata",
			obj: ObjectWithMetadata{
				Metadata: map[string]any{
					"key": "value",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.obj.HasMetadata(); got != tt.expected {
				t.Errorf("HasMetadata() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}

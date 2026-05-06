package models

import (
	"encoding/json"
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
		},
		{
			name: "with next cursor (cursor pagination has more)",
			pagination: Pagination{
				Limit:      10,
				Offset:     0,
				Total:      5,
				PrevCursor: "prev",
				NextCursor: "next",
			},
			hasMore: true, // v3: NextCursor != "" → HasMore() is true (cursor pagination is definitive)
			hasPrev: true,
		},
		{
			name: "cursor with no next",
			pagination: Pagination{
				Limit:      10,
				PrevCursor: "prev",
			},
			hasMore: false,
			hasPrev: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pagination.HasMore(); got != tt.hasMore {
				t.Errorf("HasMore() = %v, want %v", got, tt.hasMore)
			}

			if got := tt.pagination.HasPrev(); got != tt.hasPrev {
				t.Errorf("HasPrev() = %v, want %v", got, tt.hasPrev)
			}
		})
	}
}

func TestPaginationTotalKnown(t *testing.T) {
	tests := []struct {
		name       string
		pagination Pagination
		want       bool
	}{
		{name: "total populated", pagination: Pagination{Total: 100}, want: true},
		{name: "zero total", pagination: Pagination{Total: 0}, want: false},
		{name: "cursor without total", pagination: Pagination{NextCursor: "c"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pagination.TotalKnown(); got != tt.want {
				t.Errorf("TotalKnown() = %v, want %v", got, tt.want)
			}
		})
	}

	var nilPagination *Pagination
	if nilPagination.TotalKnown() {
		t.Error("nil receiver TotalKnown() returned true, want false")
	}
}

func TestPaginationNilReceiverMethodsAreSafe(t *testing.T) {
	var pagination *Pagination

	if pagination.HasMore() || pagination.HasPrev() {
		t.Fatal("nil pagination receiver should report no available pages")
	}

	if pagination.TotalKnown() {
		t.Fatal("nil pagination receiver should report total as unknown")
	}
}

func TestListResponseZeroValueMarshalUsesEmptyItems(t *testing.T) {
	data, err := json.Marshal(ListResponse[string]{})
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	items, ok := payload["items"].([]any)
	if !ok {
		t.Fatalf("expected items to marshal as an empty array, got %s", string(data))
	}

	if len(items) != 0 {
		t.Fatalf("expected empty items array, got %v", items)
	}
}

func TestListResponseUnmarshalJSONTopLevelPagination(t *testing.T) {
	data := []byte(`{
		"items": [{"id": "org-1"}],
		"limit": 10,
		"page": 2,
		"next_cursor": "next-123",
		"prev_cursor": "prev-456"
	}`)

	var response ListResponse[map[string]any]
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if len(response.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(response.Items))
	}

	if response.Pagination.Limit != 10 {
		t.Fatalf("expected limit 10, got %d", response.Pagination.Limit)
	}

	if response.Pagination.Page != 2 {
		t.Fatalf("expected page 2, got %d", response.Pagination.Page)
	}

	if response.Pagination.NextCursor != "next-123" {
		t.Fatalf("expected next cursor next-123, got %s", response.Pagination.NextCursor)
	}

	if response.Pagination.PrevCursor != "prev-456" {
		t.Fatalf("expected prev cursor prev-456, got %s", response.Pagination.PrevCursor)
	}

	if response.Pagination.ItemCount != 1 {
		t.Fatalf("expected item count 1, got %d", response.Pagination.ItemCount)
	}
}

func TestListResponseUnmarshalJSONLegacyNestedPagination(t *testing.T) {
	data := []byte(`{
		"items": [{"id": "org-1"}],
		"pagination": {
			"limit": 10,
			"offset": 20,
			"total": 55,
			"next_cursor": "next-legacy",
			"prev_cursor": "prev-legacy"
		}
	}`)

	var response ListResponse[map[string]any]
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if len(response.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(response.Items))
	}

	if response.Pagination.Limit != 10 {
		t.Fatalf("expected limit 10, got %d", response.Pagination.Limit)
	}

	if response.Pagination.Offset != 20 {
		t.Fatalf("expected offset 20, got %d", response.Pagination.Offset)
	}

	if response.Pagination.Total != 55 {
		t.Fatalf("expected total 55, got %d", response.Pagination.Total)
	}

	if response.Pagination.NextCursor != "next-legacy" {
		t.Fatalf("expected next cursor next-legacy, got %s", response.Pagination.NextCursor)
	}

	if response.Pagination.PrevCursor != "prev-legacy" {
		t.Fatalf("expected prev cursor prev-legacy, got %s", response.Pagination.PrevCursor)
	}

	if response.Pagination.ItemCount != 1 {
		t.Fatalf("expected item count 1, got %d", response.Pagination.ItemCount)
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

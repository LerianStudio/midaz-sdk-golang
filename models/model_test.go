package models

import (
	"encoding/json"
	"testing"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
	"github.com/stretchr/testify/assert"
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

	if _, exists := params[QueryParamOffset]; exists {
		t.Errorf("Did not expect unsupported offset query parameter")
	}

	if _, exists := params[QueryParamOrderBy]; exists {
		t.Errorf("Did not expect legacy orderBy query parameter")
	}
}

func TestListOptionsToQueryParamsNilReceiverSafe(t *testing.T) {
	var options *ListOptions

	params := options.ToQueryParams()

	if params[QueryParamLimit] != "10" {
		t.Fatalf("expected default limit 10, got %q", params[QueryParamLimit])
	}

	if _, exists := params[QueryParamOffset]; exists {
		t.Fatalf("nil receiver must not emit unsupported offset query parameter")
	}
}

func TestListOptionsWithFiltersDeepCopiesInput(t *testing.T) {
	filters := map[string]string{"status": "active"}
	options := NewListOptions().WithFilters(filters)

	filters["status"] = "mutated"
	filters["new"] = "value"

	if options.Filters["status"] != "active" {
		t.Fatalf("expected filters to be deep-copied, got status=%q", options.Filters["status"])
	}

	if _, exists := options.Filters["new"]; exists {
		t.Fatalf("expected filters copy to be isolated from source map mutation")
	}
}

func TestListOptionsNextPagePreservesStateAndDeepCopiesMaps(t *testing.T) {
	options := NewListOptions().
		WithLimit(25).
		WithPage(2).
		WithFilter("status", "active").
		WithDateRange("2024-01-01", "2024-12-31").
		WithAdditionalParam("include_deleted", "true").
		WithOrderBy("name").
		WithOrderDirection(SortDescending)

	next := options.NextPage()

	if next.Page != 3 {
		t.Fatalf("expected next page 3, got %d", next.Page)
	}

	if next.Filters["status"] != "active" || next.AdditionalParams["include_deleted"] != "true" {
		t.Fatalf("expected filters/additional params to be preserved, got %#v %#v", next.Filters, next.AdditionalParams)
	}

	if next.StartDate != "2024-01-01" || next.EndDate != "2024-12-31" || next.OrderBy != "name" || next.OrderDirection != string(SortDescending) {
		t.Fatalf("expected date and sort state to be preserved, got %#v", next)
	}

	next.Filters["status"] = "mutated"
	next.AdditionalParams["include_deleted"] = "false"

	if options.Filters["status"] != "active" || options.AdditionalParams["include_deleted"] != "true" {
		t.Fatalf("expected NextPage to deep-copy maps without mutating original")
	}
}

func TestNextPageOptionsFromPreservesState(t *testing.T) {
	current := NewListOptions().
		WithLimit(10).
		WithFilter("status", "active").
		WithAdditionalParam("holder_id", "holder-123").
		WithDateRange("2024-01-01", "2024-12-31")

	pagination := Pagination{Limit: 10, Page: 1, Total: 30}
	next := NextPageOptionsFrom(current, &pagination)

	if next == nil {
		t.Fatal("expected next page options")
	}

	if next.Page != 2 || next.Filters["status"] != "active" || next.AdditionalParams["holder_id"] != "holder-123" {
		t.Fatalf("expected state-preserving next options, got %#v", next)
	}

	if next.StartDate != "2024-01-01" || next.EndDate != "2024-12-31" {
		t.Fatalf("expected date range to be preserved, got %#v", next)
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

func TestListOptionsCRMFilters(t *testing.T) {
	params := NewListOptions().
		WithIncludeDeleted(true).
		WithHolderID("holder-123").
		WithExternalID("external-123").
		WithDocument("12345678900").
		WithAccountID("account-123").
		WithLedgerID("ledger-123").
		WithParticipantDocument("11222333000199").
		WithRelatedPartyDocument("99988877766").
		WithBankingDetailsBranch("0001").
		WithBankingDetailsAccount("123450").
		WithBankingDetailsIBAN("US12345678901234567890").
		WithRelatedPartyRole(RelatedPartyRolePrimaryHolder).
		ToQueryParams()

	expected := map[string]string{
		"include_deleted":                        "true",
		"holder_id":                              "holder-123",
		"external_id":                            "external-123",
		"document":                               "12345678900",
		"account_id":                             "account-123",
		"ledger_id":                              "ledger-123",
		"regulatory_fields_participant_document": "11222333000199",
		"related_party_document":                 "99988877766",
		"banking_details_branch":                 "0001",
		"banking_details_account":                "123450",
		"banking_details_iban":                   "US12345678901234567890",
		"related_party_role":                     RelatedPartyRolePrimaryHolder,
	}
	for key, value := range expected {
		if params[key] != value {
			t.Errorf("Expected %s=%s, got %s", key, value, params[key])
		}
	}
}

func TestListOptionsAdditionalParamsSkipEmptyAndDoNotOverrideReservedParams(t *testing.T) {
	params := NewListOptions().
		WithLimit(25).
		WithPage(2).
		WithAdditionalParam(QueryParamLimit, "999").
		WithAdditionalParam(QueryParamPage, "99").
		WithAdditionalParam("holder_id", "").
		WithAdditionalParam("document", "12345678900").
		ToQueryParams()

	assert.Equal(t, "25", params[QueryParamLimit])
	assert.Equal(t, "2", params[QueryParamPage])
	assert.NotContains(t, params, "holder_id")
	assert.Equal(t, "12345678900", params["document"])
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

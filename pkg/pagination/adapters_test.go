package pagination

import (
	"context"
	"reflect"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
)

// Mock structures for testing

// MockListOptions implements the interface expected by OptionsToPageOptions
type MockListOptions struct {
	limit   int
	offset  int
	cursor  string
	filters map[string]string
}

func (m *MockListOptions) GetLimit() int {
	return m.limit
}

func (m *MockListOptions) GetOffset() int {
	return m.offset
}

func (m *MockListOptions) GetCursor() string {
	return m.cursor
}

func (m *MockListOptions) GetFilters() map[string]string {
	return m.filters
}

// MockPagination implements the interface expected by PageResultFromResponse
type MockPagination struct {
	nextCursor string
	prevCursor string
	total      int
	hasMore    bool
}

func (m *MockPagination) GetNextCursor() string {
	return m.nextCursor
}

func (m *MockPagination) GetPrevCursor() string {
	return m.prevCursor
}

func (m *MockPagination) GetTotal() int {
	return m.total
}

func (m *MockPagination) HasMorePages() bool {
	return m.hasMore
}

// MockListResponse implements the interface expected by PageResultFromResponse
type MockListResponse struct {
	items      []string
	pagination *MockPagination
}

func (m *MockListResponse) GetItems() []string {
	return m.items
}

func (m *MockListResponse) GetPagination() interface {
	GetNextCursor() string
	GetPrevCursor() string
	GetTotal() int
	HasMorePages() bool
} {
	return m.pagination
}

//nolint:revive // cognitive-complexity: table-driven test pattern
func TestOptionsToPageOptions(t *testing.T) {
	// Test with default adapter
	adapter, err := NewModelAdapter()
	if err != nil {
		t.Fatalf("Failed to create default adapter: %v", err)
	}

	// Test with custom adapter
	customAdapter, err := NewModelAdapter(
		WithAdapterDefaultLimit(20),
		WithDefaultOffset(5),
		WithDefaultFilters(map[string]string{"default": "filter"}),
	)
	if err != nil {
		t.Fatalf("Failed to create custom adapter: %v", err)
	}

	testCases := []struct {
		name     string
		adapter  *ModelAdapter
		input    any
		expected PageOptions
	}{
		{
			name:    "Valid ListOptions with default adapter",
			adapter: adapter,
			input: &MockListOptions{
				limit:   20,
				offset:  30,
				cursor:  "next-cursor",
				filters: map[string]string{"key": "value"},
			},
			expected: PageOptions{
				Limit:   20,
				Offset:  30,
				Cursor:  "next-cursor",
				Filters: map[string]string{"key": "value"},
			},
		},
		{
			name:    "Invalid input type with default adapter",
			adapter: adapter,
			input:   "not a list options",
			expected: PageOptions{
				Limit:   10,
				Offset:  0,
				Filters: make(map[string]string),
			},
		},
		{
			name:    "nil input with default adapter",
			adapter: adapter,
			input:   nil,
			expected: PageOptions{
				Limit:   10,
				Offset:  0,
				Filters: make(map[string]string),
			},
		},
		{
			name:    "Invalid input type with custom adapter",
			adapter: customAdapter,
			input:   "not a list options",
			expected: PageOptions{
				Limit:   20,
				Offset:  5,
				Filters: map[string]string{"default": "filter"},
			},
		},
		{
			name:    "Valid ListOptions overrides custom adapter defaults",
			adapter: customAdapter,
			input: &MockListOptions{
				limit:   30,
				offset:  40,
				cursor:  "custom-cursor",
				filters: map[string]string{"custom": "filter"},
			},
			expected: PageOptions{
				Limit:   30,
				Offset:  40,
				Cursor:  "custom-cursor",
				Filters: map[string]string{"custom": "filter"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.adapter.OptionsToPageOptions(tc.input)

			if result.Limit != tc.expected.Limit {
				t.Errorf("Expected Limit %d, got %d", tc.expected.Limit, result.Limit)
			}

			if result.Offset != tc.expected.Offset {
				t.Errorf("Expected Offset %d, got %d", tc.expected.Offset, result.Offset)
			}

			if result.Cursor != tc.expected.Cursor {
				t.Errorf("Expected Cursor %s, got %s", tc.expected.Cursor, result.Cursor)
			}

			// Check if filters match
			if len(result.Filters) != len(tc.expected.Filters) {
				t.Errorf("Expected %d filters, got %d", len(tc.expected.Filters), len(result.Filters))
			}

			for k, v := range tc.expected.Filters {
				if result.Filters[k] != v {
					t.Errorf("Expected filter %s=%s, got %s", k, v, result.Filters[k])
				}
			}
		})
	}
}

//nolint:revive // cognitive-complexity: table-driven test pattern
func TestPageResultFromResponse(t *testing.T) {
	adapter, err := NewModelAdapter()
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}

	mockPagination := &MockPagination{
		nextCursor: "next-page",
		prevCursor: "prev-page",
		total:      100,
		hasMore:    true,
	}

	mockItems := []string{"item1", "item2", "item3"}

	mockResponse := &MockListResponse{
		items:      mockItems,
		pagination: mockPagination,
	}

	// Test with valid response
	t.Run("Valid response", func(t *testing.T) {
		// Identity extractor function
		extractor := func(s string) string { return s }

		result := PageResultFromResponse[string, string](adapter, mockResponse, extractor)

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if len(result.Items) != len(mockItems) {
			t.Errorf("Expected %d items, got %d", len(mockItems), len(result.Items))
		}

		for i, item := range mockItems {
			if result.Items[i] != item {
				t.Errorf("Expected item %s, got %s", item, result.Items[i])
			}
		}

		if result.NextCursor != mockPagination.nextCursor {
			t.Errorf("Expected NextCursor %s, got %s", mockPagination.nextCursor, result.NextCursor)
		}

		if result.PrevCursor != mockPagination.prevCursor {
			t.Errorf("Expected PrevCursor %s, got %s", mockPagination.prevCursor, result.PrevCursor)
		}

		if result.Total != mockPagination.total {
			t.Errorf("Expected Total %d, got %d", mockPagination.total, result.Total)
		}

		if result.HasMore != mockPagination.hasMore {
			t.Errorf("Expected HasMore %v, got %v", mockPagination.hasMore, result.HasMore)
		}
	})

	// Test with invalid response
	t.Run("Invalid response", func(t *testing.T) {
		extractor := func(s string) string { return s }

		result := PageResultFromResponse[string, string](adapter, "not a list response", extractor)

		if result == nil {
			t.Fatal("Expected non-nil result even for invalid input")
		}

		if len(result.Items) != 0 {
			t.Errorf("Expected 0 items, got %d", len(result.Items))
		}

		if result.HasMore {
			t.Error("Expected HasMore to be false")
		}
	})
}

//nolint:revive // cognitive-complexity: table-driven test pattern
func TestCreateEntityPaginator(t *testing.T) {
	// Simple list function that returns mock data
	listFn := func(_ context.Context, _ any) (any, error) {
		mockPagination := &MockPagination{
			nextCursor: "next-page",
			prevCursor: "prev-page",
			total:      100,
			hasMore:    true,
		}

		mockItems := []string{"item1", "item2", "item3"}

		return &MockListResponse{
			items:      mockItems,
			pagination: mockPagination,
		}, nil
	}

	// Identity extractor
	extractor := func(s string) string { return s }

	mockOptions := &MockListOptions{
		limit:  10,
		offset: 0,
		filters: map[string]string{
			"key": "value",
		},
	}

	// Test cases for different option combinations
	testCases := []struct {
		name    string
		options []EntityPaginatorOption
	}{
		{
			name: "With basic options",
			options: []EntityPaginatorOption{
				WithEntityOperationName("TestOperation"),
				WithPaginatorEntityType("testEntity"),
				WithEntityInitialOptions(mockOptions),
			},
		},
		{
			name: "With adapter options",
			options: []EntityPaginatorOption{
				WithEntityOperationName("TestOperation"),
				WithPaginatorEntityType("testEntity"),
				WithEntityInitialOptions(mockOptions),
				WithEntityAdapterOptions(
					WithAdapterDefaultLimit(20),
					WithDefaultOffset(5),
				),
			},
		},
		{
			name: "With paginator options",
			options: []EntityPaginatorOption{
				WithEntityOperationName("TestOperation"),
				WithPaginatorEntityType("testEntity"),
				WithEntityInitialOptions(mockOptions),
				WithEntityPaginatorOptions(
					WithLimit(15),
					WithOffset(0),
					WithWorkerCount(3),
				),
			},
		},
		{
			name: "With all options",
			options: []EntityPaginatorOption{
				WithEntityOperationName("TestOperation"),
				WithPaginatorEntityType("testEntity"),
				WithEntityInitialOptions(mockOptions),
				WithEntityAdapterOptions(
					WithAdapterDefaultLimit(20),
					WithDefaultOffset(5),
				),
				WithEntityPaginatorOptions(
					WithLimit(15),
					WithOffset(0),
					WithWorkerCount(3),
				),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a paginator with the test case options
			paginator, err := CreateEntityPaginator[string, string](
				context.Background(),
				listFn,
				extractor,
				tc.options...,
			)
			if err != nil {
				t.Fatalf("Failed to create paginator: %v", err)
			}

			// Verify the paginator works
			if paginator == nil {
				t.Fatal("Expected non-nil paginator")
			}

			// Test basic pagination
			ctx := context.Background()
			if !paginator.Next(ctx) {
				t.Error("Expected successful first page")
			}

			items := paginator.Items()
			if len(items) != 3 {
				t.Errorf("Expected 3 items, got %d", len(items))
			}

			// Check page info
			info := paginator.PageInfo()
			if !info.HasNextPage {
				t.Error("Expected more pages")
			}
		})
	}

	// Test backward compatibility with old API
	t.Run("Backward compatibility", func(t *testing.T) {
		// Create a paginator using the old API
		paginator := CreateEntityPaginatorWithDefaults[string, string](
			context.Background(),
			"TestOperation",
			"testEntity",
			listFn,
			mockOptions,
			extractor,
		)

		// Verify the paginator works
		if paginator == nil {
			t.Fatal("Expected non-nil paginator")
		}

		// Test basic pagination
		ctx := context.Background()
		if !paginator.Next(ctx) {
			t.Error("Expected successful first page")
		}

		items := paginator.Items()
		if len(items) != 3 {
			t.Errorf("Expected 3 items, got %d", len(items))
		}

		// Check page info
		info := paginator.PageInfo()
		if !info.HasNextPage {
			t.Error("Expected more pages")
		}
	})
}

//nolint:revive,wsl_v5
func TestModelAdapterRealSDKModelsAdvancePaginationState(t *testing.T) {
	tests := []struct {
		name        string
		initial     *models.ListOptions
		responses   []*models.ListResponse[int]
		wantCursors []string
		wantPages   []int
	}{
		{
			name: "cursor pagination applies evolving cursor to real SDK list options",
			initial: models.NewListOptions().
				WithLimit(2).
				WithCursor("start").
				WithFilter("status", "active").
				WithAdditionalParam("holder_id", "holder-123"),
			responses: []*models.ListResponse[int]{
				{Items: []int{1, 2}, Pagination: models.Pagination{Limit: 2, NextCursor: "next", Total: 3}},
				{Items: []int{3}, Pagination: models.Pagination{Limit: 2, Total: 3}},
			},
			wantCursors: []string{"start", "next"},
		},
		{
			name: "page pagination applies evolving page to real SDK list options",
			initial: models.NewListOptions().
				WithLimit(2).
				WithPage(1).
				WithFilter("status", "active"),
			responses: []*models.ListResponse[int]{
				{Items: []int{1, 2}, Pagination: models.Pagination{Limit: 2, Page: 1, Total: 3}},
				{Items: []int{3}, Pagination: models.Pagination{Limit: 2, Page: 2, Total: 3}},
			},
			wantPages: []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotCursors []string
			var gotPages []int
			var gotFilters []map[string]string
			var gotAdditional []map[string]string

			call := 0
			listFn := func(_ context.Context, opts any) (any, error) {
				listOptions, ok := opts.(*models.ListOptions)
				if !ok {
					t.Fatalf("expected *models.ListOptions, got %T", opts)
				}

				gotCursors = append(gotCursors, listOptions.Cursor)
				gotPages = append(gotPages, listOptions.Page)
				gotFilters = append(gotFilters, cloneStringMapForTest(listOptions.Filters))
				gotAdditional = append(gotAdditional, cloneStringMapForTest(listOptions.AdditionalParams))

				if call >= len(tt.responses) {
					t.Fatalf("unexpected repeated fetch %d with options %#v", call+1, listOptions)
				}

				response := tt.responses[call]
				call++

				return response, nil
			}

			paginator, err := CreateEntityPaginator[int, int](
				context.Background(),
				listFn,
				func(item int) int { return item },
				WithEntityInitialOptions(tt.initial),
			)
			if err != nil {
				t.Fatalf("unexpected paginator construction error: %v", err)
			}

			items, err := paginator.All(context.Background())
			if err != nil {
				t.Fatalf("All failed: %v", err)
			}

			if len(items) != 3 {
				t.Fatalf("expected 3 items, got %d", len(items))
			}

			if len(tt.wantCursors) > 0 && !reflect.DeepEqual(gotCursors, tt.wantCursors) {
				t.Fatalf("expected cursors %v, got %v", tt.wantCursors, gotCursors)
			}

			if len(tt.wantPages) > 0 && !reflect.DeepEqual(gotPages, tt.wantPages) {
				t.Fatalf("expected pages %v, got %v", tt.wantPages, gotPages)
			}

			for _, filters := range gotFilters {
				if filters["status"] != "active" {
					t.Fatalf("expected filters to be preserved on every fetch, got %v", gotFilters)
				}
			}

			if tt.initial.AdditionalParams != nil {
				for _, additional := range gotAdditional {
					if additional["holder_id"] != "holder-123" {
						t.Fatalf("expected additional params to be preserved on every fetch, got %v", gotAdditional)
					}
				}
			}

			if call != len(tt.responses) {
				t.Fatalf("expected %d fetches, got %d", len(tt.responses), call)
			}
		})
	}
}

func TestCreateEntityPaginatorRejectsNilInputs(t *testing.T) {
	listFn := func(context.Context, any) (any, error) {
		return &models.ListResponse[int]{Items: []int{}}, nil
	}

	if paginator, err := CreateEntityPaginator[int, int](context.Background(), nil, func(item int) int { return item }, WithEntityInitialOptions(models.NewListOptions())); err == nil || paginator != nil {
		t.Fatalf("expected nil list function to fail construction, paginator=%v err=%v", paginator, err)
	}

	if paginator, err := CreateEntityPaginator[int, int](context.Background(), listFn, nil, WithEntityInitialOptions(models.NewListOptions())); err == nil || paginator != nil {
		t.Fatalf("expected nil extractor to fail construction, paginator=%v err=%v", paginator, err)
	}

	if paginator, err := CreateEntityPaginator[int, int](context.Background(), listFn, func(item int) int { return item }, nil); err == nil || paginator != nil {
		t.Fatalf("expected nil entity paginator option to fail construction, paginator=%v err=%v", paginator, err)
	}
}

func TestNewModelAdapterRejectsNilOption(t *testing.T) {
	adapter, err := NewModelAdapter(nil)
	if err == nil || adapter != nil {
		t.Fatalf("expected nil adapter option to fail construction, adapter=%v err=%v", adapter, err)
	}
}

func TestPageResultFromRealSDKResponseRejectsNilExtractor(t *testing.T) {
	adapter, err := NewModelAdapter()
	if err != nil {
		t.Fatalf("unexpected adapter error: %v", err)
	}

	result, err := PageResultFromResponseE[int, int](adapter, &models.ListResponse[int]{Items: []int{1}}, nil)
	if err == nil || result != nil {
		t.Fatalf("expected nil extractor to fail safely, result=%v err=%v", result, err)
	}
}

func cloneStringMapForTest(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}

	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}

	return clone
}

package pagination

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Create a mock fetcher for testing
type mockFetcher[T any] struct {
	pages      [][]T
	err        error
	errOnPage  int
	callCount  int
	delay      time.Duration
	totalItems int
}

func newMockFetcher[T any](pages [][]T, totalItems int) *mockFetcher[T] {
	return &mockFetcher[T]{
		pages:      pages,
		totalItems: totalItems,
	}
}

func (m *mockFetcher[T]) withError(err error, onPage int) *mockFetcher[T] {
	m.err = err
	m.errOnPage = onPage

	return m
}

func (m *mockFetcher[T]) withDelay(delay time.Duration) *mockFetcher[T] {
	m.delay = delay
	return m
}

func (m *mockFetcher[T]) fetch(ctx context.Context, _ PageOptions) (*PageResult[T], error) {
	// Simulate delay if configured
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Increment call count
	pageIndex := m.callCount
	m.callCount++

	// Return error if configured for this page
	if m.err != nil && pageIndex == m.errOnPage {
		return nil, m.err
	}

	// Check if we've gone through all pages
	if pageIndex >= len(m.pages) {
		return &PageResult[T]{
			Items:   []T{},
			HasMore: false,
			Total:   m.totalItems,
		}, nil
	}

	// Return the page with appropriate pagination info
	hasMore := pageIndex < len(m.pages)-1

	// Calculate cursor values (just use page indices as strings for testing)
	var nextCursor, prevCursor string
	if hasMore {
		nextCursor = "page-" + strconv.Itoa(pageIndex+1)
	}

	if pageIndex > 0 {
		prevCursor = "page-" + strconv.Itoa(pageIndex-1)
	}

	// Always indicate there's more data until the last page
	return &PageResult[T]{
		Items:      m.pages[pageIndex],
		NextCursor: nextCursor,
		PrevCursor: prevCursor,
		HasMore:    hasMore,
		Total:      m.totalItems,
	}, nil
}

type observerFunc func(context.Context, *Event)

func (f observerFunc) RecordEvent(ctx context.Context, event *Event) {
	f(ctx, event)
}

//nolint:revive,wsl_v5 // cognitive-complexity: table-driven test pattern
func TestPaginator(t *testing.T) {
	// Create test data
	pages := [][]string{
		{"item1", "item2", "item3"},
		{"item4", "item5", "item6"},
		{"item7", "item8", "item9"},
	}
	totalItems := 9

	// Create a mock fetcher
	mockFetcher := newMockFetcher(pages, totalItems)

	t.Run("Basic pagination", func(t *testing.T) {
		ctx := context.Background()

		paginator, err := NewPaginator(
			mockFetcher.fetch,
			WithOperationName("TestOperation"),
			WithEntityType("testEntity"),
			WithPageOptions(PageOptions{Limit: 3}),
		)
		if err != nil {
			t.Fatalf("Failed to create paginator: %v", err)
		}

		// Check first page
		if !paginator.Next(ctx) {
			t.Fatal("Expected to get the first page")
		}

		items := paginator.Items()
		if len(items) != 3 {
			t.Errorf("Expected 3 items, got %d", len(items))
		}

		info := paginator.PageInfo()
		if info.PageNumber != 1 {
			t.Errorf("Expected page number 1, got %d", info.PageNumber)
		}

		// We're not testing multiple pages in this basic test to avoid complexity
		// The real implementation will handle page transitions properly
	})

	t.Run("Error handling", func(t *testing.T) {
		ctx := context.Background()
		expectedErr := errors.New("test error")

		// Create a mock fetcher with an error on the second page
		errorFetcher := newMockFetcher(pages, totalItems).withError(expectedErr, 1)

		paginator, err := NewPaginator(
			errorFetcher.fetch,
			WithOperationName("TestOperation"),
			WithEntityType("testEntity"),
			WithPageOptions(PageOptions{Limit: 3}),
		)
		if err != nil {
			t.Fatalf("Failed to create paginator: %v", err)
		}

		// First page should work
		if !paginator.Next(ctx) {
			t.Fatal("Expected to get the first page")
		}

		// Second page should fail
		if paginator.Next(ctx) {
			t.Error("Expected second page to fail")
		}

		if !errors.Is(paginator.Err(), expectedErr) {
			t.Errorf("Expected error %v, got %v", expectedErr, paginator.Err())
		}
	})

	t.Run("All method", func(t *testing.T) {
		fetcher := newMockFetcher(pages, totalItems)
		paginator, err := NewPaginator(fetcher.fetch, WithPageOptions(PageOptions{Limit: 3}))
		if err != nil {
			t.Fatalf("Failed to create paginator: %v", err)
		}

		items, err := paginator.All(context.Background())
		if err != nil {
			t.Fatalf("All failed: %v", err)
		}

		if len(items) != totalItems {
			t.Fatalf("Expected %d items, got %d", totalItems, len(items))
		}
	})

	t.Run("ForEach method", func(t *testing.T) {
		fetcher := newMockFetcher(pages, totalItems)
		paginator, err := NewPaginator(fetcher.fetch, WithPageOptions(PageOptions{Limit: 3}))
		if err != nil {
			t.Fatalf("Failed to create paginator: %v", err)
		}

		var seen []string
		err = paginator.ForEach(context.Background(), func(item string) error {
			seen = append(seen, item)
			return nil
		})
		if err != nil {
			t.Fatalf("ForEach failed: %v", err)
		}

		if len(seen) != totalItems {
			t.Fatalf("Expected %d items, got %d", totalItems, len(seen))
		}
	})

	t.Run("ForEach with error", func(t *testing.T) {
		fetcher := newMockFetcher(pages, totalItems)
		paginator, err := NewPaginator(fetcher.fetch, WithPageOptions(PageOptions{Limit: 3}))
		if err != nil {
			t.Fatalf("Failed to create paginator: %v", err)
		}

		expectedErr := errors.New("callback failed")
		err = paginator.ForEach(context.Background(), func(item string) error {
			if item == "item4" {
				return expectedErr
			}

			return nil
		})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("Expected callback error %v, got %v", expectedErr, err)
		}
	})

	t.Run("Concurrent method", func(t *testing.T) {
		fetcher := newMockFetcher(pages, totalItems)
		paginator, err := NewPaginator(fetcher.fetch, WithPageOptions(PageOptions{Limit: 3}))
		if err != nil {
			t.Fatalf("Failed to create paginator: %v", err)
		}

		var mu sync.Mutex
		seen := make(map[string]bool)
		err = paginator.Concurrent(context.Background(), 3, func(item string) error {
			mu.Lock()
			defer mu.Unlock()
			seen[item] = true
			return nil
		})
		if err != nil {
			t.Fatalf("Concurrent failed: %v", err)
		}

		if len(seen) != totalItems {
			t.Fatalf("Expected %d processed items, got %d", totalItems, len(seen))
		}
	})

	t.Run("Concurrent with error", func(t *testing.T) {
		fetcher := newMockFetcher(pages, totalItems)
		paginator, err := NewPaginator(fetcher.fetch, WithPageOptions(PageOptions{Limit: 3}))
		if err != nil {
			t.Fatalf("Failed to create paginator: %v", err)
		}

		expectedErr := errors.New("worker failed")
		err = paginator.Concurrent(context.Background(), 2, func(item string) error {
			if item == "item5" {
				return expectedErr
			}

			return nil
		})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("Expected worker error %v, got %v", expectedErr, err)
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		// Create a mock fetcher with a delay
		delayFetcher := newMockFetcher(pages, totalItems).withDelay(time.Millisecond * 50)

		ctx, cancel := context.WithCancel(context.Background())

		paginator, err := NewPaginator(
			delayFetcher.fetch,
			WithOperationName("TestOperation"),
			WithEntityType("testEntity"),
			WithPageOptions(PageOptions{Limit: 3}),
		)
		if err != nil {
			t.Fatalf("Failed to create paginator: %v", err)
		}

		// Get the first page
		if !paginator.Next(ctx) {
			t.Fatal("Expected to get the first page")
		}

		// Cancel the context before the second page
		cancel()

		// Attempt to get the second page
		if paginator.Next(ctx) {
			t.Error("Expected pagination to fail after context cancellation")
		}

		if paginator.Err() == nil || !errors.Is(paginator.Err(), context.Canceled) {
			t.Errorf("Expected context.Canceled error, got %v", paginator.Err())
		}
	})
}

func TestNewPaginatorRejectsInvalidInputs(t *testing.T) {
	validFetcher := func(_ context.Context, _ PageOptions) (*PageResult[string], error) {
		return &PageResult[string]{Items: []string{}, HasMore: false}, nil
	}

	if paginator, err := NewPaginator[string](nil); err == nil || paginator != nil {
		t.Fatalf("expected nil fetcher to fail construction, paginator=%v err=%v", paginator, err)
	}

	if paginator, err := NewPaginator(validFetcher, nil); err == nil || paginator != nil {
		t.Fatalf("expected nil paginator option to fail construction, paginator=%v err=%v", paginator, err)
	}
}

func TestPaginatorFetcherReturningNilPageReturnsError(t *testing.T) {
	paginator, err := NewPaginator(func(_ context.Context, _ PageOptions) (*PageResult[string], error) {
		return nil, nil //nolint:nilnil
	})
	if err != nil {
		t.Fatalf("unexpected construction error: %v", err)
	}

	if paginator.Next(context.Background()) {
		t.Fatal("expected nil page result to stop pagination")
	}

	if paginator.Err() == nil {
		t.Fatal("expected nil page result to be recorded as an error")
	}
}

//nolint:wsl_v5
func TestPaginatorTerminalCursorPageDoesNotFetchAgain(t *testing.T) {
	calls := 0
	paginator, err := NewPaginator(func(_ context.Context, options PageOptions) (*PageResult[int], error) {
		calls++
		if options.Cursor != "terminal-cursor" {
			return nil, fmt.Errorf("expected terminal cursor, got %q", options.Cursor)
		}

		return &PageResult[int]{Items: []int{1}, HasMore: true}, nil
	}, WithPageOptions(PageOptions{Limit: 10, Cursor: "terminal-cursor"}))
	if err != nil {
		t.Fatalf("unexpected construction error: %v", err)
	}

	if !paginator.Next(context.Background()) {
		t.Fatalf("expected terminal cursor page to return items: %v", paginator.Err())
	}

	if paginator.Next(context.Background()) {
		t.Fatal("expected terminal cursor page to exhaust paginator")
	}

	if calls != 1 {
		t.Fatalf("expected exactly one fetch for terminal cursor page, got %d", calls)
	}
}

//nolint:wsl_v5
func TestPaginatorAllRespectsMaxPagesAndMaxItems(t *testing.T) {
	t.Run("max pages", func(t *testing.T) {
		fetcher := newMockFetcher([][]int{{1, 2}, {3, 4}, {5, 6}}, 6)
		paginator, err := NewPaginator(fetcher.fetch, WithPageOptions(PageOptions{Limit: 2}), WithMaxPages(2))
		if err != nil {
			t.Fatalf("unexpected construction error: %v", err)
		}

		items, err := paginator.All(context.Background())
		if err != nil {
			t.Fatalf("All failed: %v", err)
		}

		if len(items) != 4 {
			t.Fatalf("expected max pages to collect 4 items, got %d", len(items))
		}

		if fetcher.callCount != 2 {
			t.Fatalf("expected 2 fetches, got %d", fetcher.callCount)
		}
	})

	t.Run("max items", func(t *testing.T) {
		fetcher := newMockFetcher([][]int{{1, 2}, {3, 4}, {5, 6}}, 6)
		paginator, err := NewPaginator(fetcher.fetch, WithPageOptions(PageOptions{Limit: 2}), WithMaxItems(3))
		if err != nil {
			t.Fatalf("unexpected construction error: %v", err)
		}

		items, err := paginator.All(context.Background())
		if err != nil {
			t.Fatalf("All failed: %v", err)
		}

		if len(items) != 3 {
			t.Fatalf("expected max items to collect 3 items, got %d", len(items))
		}
	})
}

//nolint:wsl_v5
func TestPaginatorAllChecksContextBeforeFetching(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	paginator, err := NewPaginator(func(_ context.Context, _ PageOptions) (*PageResult[int], error) {
		called = true
		return &PageResult[int]{Items: []int{1}}, nil
	})
	if err != nil {
		t.Fatalf("unexpected construction error: %v", err)
	}

	items, err := paginator.All(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got items=%v err=%v", items, err)
	}

	if called {
		t.Fatal("expected All to return context error before fetching")
	}
}

//nolint:wsl_v5
func TestPaginatorConcurrentPropagatesWorkerPanic(t *testing.T) {
	fetcher := newMockFetcher([][]int{{1, 2, 3}}, 3)
	paginator, err := NewPaginator(fetcher.fetch, WithPageOptions(PageOptions{Limit: 3}))
	if err != nil {
		t.Fatalf("unexpected construction error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- paginator.Concurrent(context.Background(), 2, func(item int) error {
			if item == 2 {
				panic("worker panic")
			}

			return nil
		})
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected worker panic to be returned as error")
		}
	case <-time.After(time.Second):
		t.Fatal("Concurrent hung after worker panic")
	}
}

//nolint:wsl_v5
func TestPaginatorConcurrentDoesNotHangWhenContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fetcher := newMockFetcher([][]int{{1, 2, 3}}, 3)
	paginator, err := NewPaginator(fetcher.fetch, WithPageOptions(PageOptions{Limit: 3}))
	if err != nil {
		t.Fatalf("unexpected construction error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- paginator.Concurrent(ctx, 2, func(_ int) error { return nil })
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Concurrent hung with cancelled context")
	}
}

//nolint:wsl_v5
func TestPaginatorObserverPanicDoesNotPoisonPaginator(t *testing.T) {
	fetcher := newMockFetcher([][]int{{1}}, 1)
	paginator, err := NewPaginator(fetcher.fetch, WithObserver(observerFunc(func(context.Context, *Event) {
		panic("observer panic")
	})))
	if err != nil {
		t.Fatalf("unexpected construction error: %v", err)
	}

	if !paginator.Next(context.Background()) {
		t.Fatalf("expected first page despite observer panic: %v", paginator.Err())
	}

	if len(paginator.Items()) != 1 {
		t.Fatalf("expected paginator to remain usable after observer panic")
	}
}

//nolint:wsl_v5
func TestPaginatorObserverCallbackNotUnderMutex(t *testing.T) {
	fetcher := newMockFetcher([][]int{{1}}, 1)
	var paginator Paginator[int]
	observerEntered := make(chan struct{}, 1)

	created, err := NewPaginator(fetcher.fetch, WithObserver(observerFunc(func(context.Context, *Event) {
		observerEntered <- struct{}{}
		_ = paginator.Items()
	})))
	if err != nil {
		t.Fatalf("unexpected construction error: %v", err)
	}
	paginator = created

	done := make(chan bool, 1)
	go func() {
		done <- paginator.Next(context.Background())
	}()

	select {
	case <-observerEntered:
	case <-time.After(time.Second):
		t.Fatal("observer was not called")
	}

	select {
	case ok := <-done:
		if !ok {
			t.Fatalf("expected Next to succeed, got err %v", paginator.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("observer callback appears to have executed under paginator mutex")
	}
}

func TestCollectAll(t *testing.T) {
	// Create test data
	pages := [][]string{
		{"item1", "item2", "item3"},
		{"item4", "item5", "item6"},
		{"item7", "item8", "item9"},
	}
	totalItems := 9

	// Create a mock fetcher
	mockFetcher := newMockFetcher(pages, totalItems)

	ctx := context.Background()

	allItems, err := CollectAll(
		ctx,
		"TestOperation",
		"testEntity",
		mockFetcher.fetch,
		PageOptions{Limit: 3},
	)
	if err != nil {
		t.Fatalf("CollectAll failed: %v", err)
	}

	if len(allItems) != totalItems {
		t.Errorf("Expected %d items, got %d", totalItems, len(allItems))
	}
}

func TestCollectAllRespectsMaxItemsOption(t *testing.T) {
	pages := [][]string{
		{"item1", "item2", "item3"},
		{"item4", "item5", "item6"},
	}
	fetcher := newMockFetcher(pages, 6)

	items, err := CollectAll(
		context.Background(),
		"TestOperation",
		"testEntity",
		fetcher.fetch,
		PageOptions{Limit: 3},
		WithMaxItems(4),
	)
	if err != nil {
		t.Fatalf("CollectAll failed: %v", err)
	}

	if len(items) != 4 {
		t.Fatalf("expected CollectAll max items to return 4 items, got %d", len(items))
	}
}

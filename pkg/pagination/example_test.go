package pagination_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/pagination"
)

// This example demonstrates how to use the pagination package with custom types
func Example() {
	// Create a sample page fetcher with mock data
	fetcher := func(_ context.Context, _ pagination.PageOptions) (*pagination.PageResult[string], error) {
		// Simulate an API call that returns paginated results
		time.Sleep(10 * time.Millisecond)

		// Return a page of results
		return &pagination.PageResult[string]{
			Items:      []string{"item1", "item2", "item3"},
			NextCursor: "next-page-cursor",
			PrevCursor: "",
			Total:      100,
			HasMore:    true,
		}, nil
	}

	// Create a paginator
	paginator, err := pagination.NewPaginator(
		fetcher,
		pagination.WithOperationName("ListItems"),
		pagination.WithEntityType("item"),
		pagination.WithPageOptions(pagination.PageOptions{
			Limit: 10,
		}),
	)
	if err != nil {
		fmt.Printf("Error creating paginator: %v\n", err)
		return
	}

	// Use the paginator to fetch the first page
	ctx := context.Background()
	if paginator.Next(ctx) {
		// Access the items in the current page
		items := paginator.Items()
		for _, item := range items {
			fmt.Println(item)
		}

		// Get pagination information
		info := paginator.PageInfo()
		fmt.Printf("Page %d of %d, Items: %d, HasNext: %v\n",
			info.PageNumber, info.TotalPages, info.TotalItems, info.HasNextPage)
	}

	// Check for errors
	if err := paginator.Err(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// Output:
	// item1
	// item2
	// item3
	// Page 1 of 10, Items: 100, HasNext: true
}

// This example demonstrates how to use the ForEach method to process all items
func ExamplePaginator_ForEach() {
	// Create a mock page fetcher with three pages
	pageCount := 0
	fetcher := func(_ context.Context, _ pagination.PageOptions) (*pagination.PageResult[int], error) {
		pageCount++

		// Return empty page after 3 pages
		if pageCount > 3 {
			return &pagination.PageResult[int]{
				Items:   []int{},
				HasMore: false,
				Total:   12,
			}, nil
		}

		// Calculate items for this page
		startItem := (pageCount - 1) * 4
		items := []int{startItem + 1, startItem + 2, startItem + 3, startItem + 4}

		return &pagination.PageResult[int]{
			Items:   items,
			HasMore: pageCount < 3,
			Total:   12,
		}, nil
	}

	// Create the paginator
	paginator, err := pagination.NewPaginator(
		fetcher,
		pagination.WithOperationName("ListNumbers"),
		pagination.WithEntityType("number"),
		pagination.WithPageOptions(pagination.PageOptions{Limit: 4}),
	)
	if err != nil {
		fmt.Printf("Error creating paginator: %v\n", err)
		return
	}

	// Process all items with ForEach
	ctx := context.Background()
	sum := 0

	err = paginator.ForEach(ctx, func(item int) error {
		sum += item
		return nil
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Sum of all items: %d\n", sum)
	}

	// Output:
	// Sum of all items: 78
}

// This example demonstrates how to use the Concurrent method to process items in parallel
func ExamplePaginator_Concurrent() {
	// Create a mock page fetcher with slow operations
	pageCount := 0
	fetcher := func(_ context.Context, _ pagination.PageOptions) (*pagination.PageResult[string], error) {
		pageCount++

		// Return empty page after 2 pages
		if pageCount > 2 {
			return &pagination.PageResult[string]{
				Items:   []string{},
				HasMore: false,
				Total:   6,
			}, nil
		}

		// Return different items based on the page
		var items []string
		if pageCount == 1 {
			items = []string{"A", "B", "C"}
		} else {
			items = []string{"D", "E", "F"}
		}

		return &pagination.PageResult[string]{
			Items:   items,
			HasMore: pageCount < 2,
			Total:   6,
		}, nil
	}

	// Create the paginator
	paginator, err := pagination.NewPaginator(
		fetcher,
		pagination.WithOperationName("ProcessLetters"),
		pagination.WithEntityType("letter"),
		pagination.WithPageOptions(pagination.PageOptions{Limit: 3}),
	)
	if err != nil {
		fmt.Printf("Error creating paginator: %v\n", err)
		return
	}

	// Process items concurrently
	ctx := context.Background()

	var count atomic.Int32

	_ = paginator.Concurrent(ctx, 3, func(_ string) error {
		// This would normally be a more complex operation
		count.Add(1)
		return nil
	})

	fmt.Printf("Processed %d items\n", count.Load())

	// Output:
	// Processed 6 items
}

// This example demonstrates how to collect all items with CollectAll helper
func ExampleCollectAll() {
	// Create a mock page fetcher with multiple pages
	pageCount := 0
	fetcher := func(_ context.Context, _ pagination.PageOptions) (*pagination.PageResult[int], error) {
		pageCount++

		// Return empty page after 3 pages
		if pageCount > 3 {
			return &pagination.PageResult[int]{
				Items:   []int{},
				HasMore: false,
				Total:   9,
			}, nil
		}

		// Different page sizes to test pagination logic
		var items []int

		switch pageCount {
		case 1:
			items = []int{1, 2, 3}
		case 2:
			items = []int{4, 5, 6}
		case 3:
			items = []int{7, 8, 9}
		}

		return &pagination.PageResult[int]{
			Items:   items,
			HasMore: pageCount < 3,
			Total:   9,
		}, nil
	}

	// Collect all items
	ctx := context.Background()

	allItems, err := pagination.CollectAll(
		ctx,
		"CollectNumbers",
		"number",
		fetcher,
		pagination.PageOptions{Limit: 3},
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Collected %d items\n", len(allItems))
	}

	// Output:
	// Collected 9 items
}

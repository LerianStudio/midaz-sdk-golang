package pagination

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PageFetcher is a generic function type for fetching a page of results
type PageFetcher[T any] func(ctx context.Context, options PageOptions) (*PageResult[T], error)

// PageOptions represents options for fetching a page
type PageOptions struct {
	Limit  int
	Offset int
	Cursor string
	// Other filter parameters can be added as needed
	Filters map[string]string
}

// PaginatorOption defines a function that configures a PaginatorOptions object
type PaginatorOption func(*PaginatorOptions) error

// PaginatorOptions holds all configuration options for a paginator
type PaginatorOptions struct {
	// Initial page options
	PageOptions PageOptions

	// Observer for monitoring pagination operations
	Observer Observer

	// Operation name for metrics and logging
	OperationName string

	// Entity type for metrics and logging
	EntityType string

	// Number of concurrent workers for Concurrent method
	WorkerCount int

	// Default limit when not specified
	DefaultLimit int
}

// PageResult represents a single page of results
type PageResult[T any] struct {
	Items      []T
	NextCursor string
	PrevCursor string
	Total      int
	HasMore    bool
}

// Paginator provides an interface for paginating through results
type Paginator[T any] interface {
	// Next advances to the next page of results
	Next(ctx context.Context) bool

	// Items returns the items in the current page
	Items() []T

	// Err returns any error that occurred during pagination
	Err() error

	// PageInfo returns information about the current page
	PageInfo() PageInfo

	// All retrieves all remaining items across multiple pages
	All(ctx context.Context) ([]T, error)

	// ForEach iterates through all items across pages
	ForEach(ctx context.Context, fn func(item T) error) error

	// Concurrent processes items concurrently with the specified number of workers
	Concurrent(ctx context.Context, workers int, fn func(item T) error) error
}

// PageInfo contains information about the current page
type PageInfo struct {
	PageNumber   int
	TotalPages   int
	TotalItems   int
	ItemsPerPage int
	HasNextPage  bool
	HasPrevPage  bool
}

// defaultPaginator is the default implementation of Paginator
type defaultPaginator[T any] struct {
	fetcher       PageFetcher[T]
	currentPage   *PageResult[T]
	options       PageOptions
	pageNumber    int
	totalItems    int
	err           error
	observer      Observer
	operationName string
	entityType    string
	mu            sync.Mutex
}

// DefaultPaginatorOptions returns the default options for a paginator
func DefaultPaginatorOptions() *PaginatorOptions {
	return &PaginatorOptions{
		PageOptions: PageOptions{
			Limit:  10,
			Offset: 0,
		},
		Observer:     NewObserver(),
		WorkerCount:  5,
		DefaultLimit: 10,
	}
}

// WithLimit sets the initial page limit
func WithLimit(limit int) PaginatorOption {
	return func(o *PaginatorOptions) error {
		if limit <= 0 {
			return fmt.Errorf("limit must be positive, got %d", limit)
		}
		o.PageOptions.Limit = limit
		return nil
	}
}

// WithOffset sets the initial page offset
func WithOffset(offset int) PaginatorOption {
	return func(o *PaginatorOptions) error {
		if offset < 0 {
			return fmt.Errorf("offset must be non-negative, got %d", offset)
		}
		o.PageOptions.Offset = offset
		return nil
	}
}

// WithCursor sets the initial cursor
func WithCursor(cursor string) PaginatorOption {
	return func(o *PaginatorOptions) error {
		o.PageOptions.Cursor = cursor
		return nil
	}
}

// WithFilters sets the initial filters
func WithFilters(filters map[string]string) PaginatorOption {
	return func(o *PaginatorOptions) error {
		o.PageOptions.Filters = filters
		return nil
	}
}

// WithPageOptions sets all initial page options at once
func WithPageOptions(options PageOptions) PaginatorOption {
	return func(o *PaginatorOptions) error {
		o.PageOptions = options
		return nil
	}
}

// WithObserver sets the observer for monitoring pagination operations
func WithObserver(observer Observer) PaginatorOption {
	return func(o *PaginatorOptions) error {
		if observer == nil {
			return fmt.Errorf("observer cannot be nil")
		}
		o.Observer = observer
		return nil
	}
}

// WithOperationName sets the operation name for metrics and logging
func WithOperationName(operationName string) PaginatorOption {
	return func(o *PaginatorOptions) error {
		if operationName == "" {
			return fmt.Errorf("operation name cannot be empty")
		}
		o.OperationName = operationName
		return nil
	}
}

// WithEntityType sets the entity type for metrics and logging
func WithEntityType(entityType string) PaginatorOption {
	return func(o *PaginatorOptions) error {
		if entityType == "" {
			return fmt.Errorf("entity type cannot be empty")
		}
		o.EntityType = entityType
		return nil
	}
}

// WithWorkerCount sets the number of concurrent workers
func WithWorkerCount(workerCount int) PaginatorOption {
	return func(o *PaginatorOptions) error {
		if workerCount <= 0 {
			return fmt.Errorf("worker count must be positive, got %d", workerCount)
		}
		o.WorkerCount = workerCount
		return nil
	}
}

// WithDefaultLimit sets the default page limit
func WithDefaultLimit(defaultLimit int) PaginatorOption {
	return func(o *PaginatorOptions) error {
		if defaultLimit <= 0 {
			return fmt.Errorf("default limit must be positive, got %d", defaultLimit)
		}
		o.DefaultLimit = defaultLimit
		return nil
	}
}

// NewPaginator creates a new Paginator instance with options
func NewPaginator[T any](
	fetcher PageFetcher[T],
	options ...PaginatorOption,
) (Paginator[T], error) {
	// Start with default options
	opts := DefaultPaginatorOptions()

	// Apply all provided options
	for _, option := range options {
		if err := option(opts); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Ensure we have an observer
	if opts.Observer == nil {
		opts.Observer = NewObserver()
	}

	// Ensure we have a valid limit
	if opts.PageOptions.Limit <= 0 {
		opts.PageOptions.Limit = opts.DefaultLimit
	}

	// Create and return the paginator
	return &defaultPaginator[T]{
		fetcher:       fetcher,
		options:       opts.PageOptions,
		pageNumber:    0,
		observer:      opts.Observer,
		operationName: opts.OperationName,
		entityType:    opts.EntityType,
	}, nil
}

// NewPaginatorWithDefaults creates a new Paginator instance with minimal required parameters
// This function is provided for backward compatibility
func NewPaginatorWithDefaults[T any](
	operationName string,
	entityType string,
	fetcher PageFetcher[T],
	initialOptions PageOptions,
	observer Observer,
) Paginator[T] {
	// Create options list
	var optionsList []PaginatorOption

	// Set required options
	optionsList = append(optionsList, WithOperationName(operationName))
	optionsList = append(optionsList, WithEntityType(entityType))

	// Set page options if provided
	optionsList = append(optionsList, WithPageOptions(initialOptions))

	// Set observer if provided
	if observer != nil {
		optionsList = append(optionsList, WithObserver(observer))
	}

	// Create paginator with options
	paginator, err := NewPaginator(fetcher, optionsList...)
	if err != nil {
		// Return a default paginator in case of error, for backward compatibility
		var observerToUse Observer
		if observer != nil {
			observerToUse = observer
		} else {
			observerToUse = NewObserver()
		}

		return &defaultPaginator[T]{
			fetcher:       fetcher,
			options:       initialOptions,
			pageNumber:    0,
			observer:      observerToUse,
			operationName: operationName,
			entityType:    entityType,
		}
	}

	return paginator
}

// Next advances to the next page of results
func (p *defaultPaginator[T]) Next(ctx context.Context) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	event := &Event{
		Operation:  p.operationName,
		EntityType: p.entityType,
		Limit:      p.options.Limit,
		Offset:     p.options.Offset,
		Page:       p.pageNumber + 1,
		CursorUsed: p.options.Cursor != "",
	}

	// Fetch the next page
	var err error
	p.currentPage, err = p.fetcher(ctx, p.options)

	// Record pagination metrics
	duration := time.Since(start)

	if err != nil {
		p.err = err
		event.Error = err
		event.Duration = duration
		p.observer.RecordEvent(ctx, event)
		return false
	}

	// Update page information
	p.pageNumber++

	if p.currentPage.Total > 0 {
		p.totalItems = p.currentPage.Total
	}

	// Update options for the next page
	if p.currentPage.NextCursor != "" {
		// Use cursor-based pagination if available
		p.options.Cursor = p.currentPage.NextCursor
		p.options.Offset = 0 // Reset offset when using cursor
	} else {
		// Fall back to offset-based pagination
		p.options.Offset += p.options.Limit
	}

	event.ProcessedItems = len(p.currentPage.Items)
	event.TotalItems = p.totalItems
	event.HasNextPage = p.currentPage.HasMore
	event.Duration = duration
	p.observer.RecordEvent(ctx, event)

	// Return false if we've reached the end or got an empty page
	return len(p.currentPage.Items) > 0
}

// Items returns the items in the current page
func (p *defaultPaginator[T]) Items() []T {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.currentPage == nil {
		return []T{}
	}
	return p.currentPage.Items
}

// Err returns any error that occurred during pagination
func (p *defaultPaginator[T]) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.err
}

// PageInfo returns information about the current page
func (p *defaultPaginator[T]) PageInfo() PageInfo {
	p.mu.Lock()
	defer p.mu.Unlock()

	info := PageInfo{
		PageNumber:   p.pageNumber,
		ItemsPerPage: p.options.Limit,
		TotalItems:   p.totalItems,
	}

	if p.currentPage != nil {
		info.HasNextPage = p.currentPage.HasMore
		info.HasPrevPage = p.pageNumber > 1 || p.currentPage.PrevCursor != ""

		// Calculate total pages if we know the total items
		if p.totalItems > 0 && p.options.Limit > 0 {
			info.TotalPages = (p.totalItems + p.options.Limit - 1) / p.options.Limit
		}
	}

	return info
}

// All retrieves all remaining items across multiple pages
func (p *defaultPaginator[T]) All(ctx context.Context) ([]T, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var allItems []T

	// Add current page items if we already have a page
	if p.currentPage != nil {
		allItems = append(allItems, p.currentPage.Items...)
	}

	// Fetch all remaining pages directly
	// We're already under lock so we can use our own fetcher
	for {
		pageResult, err := p.fetcher(ctx, p.options)
		if err != nil {
			return allItems, err
		}

		if len(pageResult.Items) == 0 {
			break
		}

		allItems = append(allItems, pageResult.Items...)

		// Update options for next page
		if pageResult.NextCursor != "" {
			p.options.Cursor = pageResult.NextCursor
			p.options.Offset = 0
		} else {
			p.options.Offset += p.options.Limit
		}

		// Check if we should stop
		if !pageResult.HasMore {
			break
		}
	}

	return allItems, nil
}

// ForEach iterates through all items across pages
func (p *defaultPaginator[T]) ForEach(ctx context.Context, fn func(item T) error) error {
	// Process current page items if we already have a page
	if p.currentPage != nil {
		for _, item := range p.currentPage.Items {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			if err := fn(item); err != nil {
				return err
			}
		}
	}

	// Process all remaining pages
	for p.Next(ctx) {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		for _, item := range p.Items() {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			if err := fn(item); err != nil {
				return err
			}
		}
	}

	return p.Err()
}

// Concurrent processes items concurrently with the specified number of workers
func (p *defaultPaginator[T]) Concurrent(ctx context.Context, workers int, fn func(item T) error) error {
	if workers <= 0 {
		workers = 5 // Default to 5 workers
	}

	// Create a buffered channel for items to process
	itemCh := make(chan T, workers*2)

	// Create a channel for errors
	errCh := make(chan error, 1)

	// Create a WaitGroup to wait for all workers to finish
	var wg sync.WaitGroup

	// Context with cancellation for terminating workers
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for item := range itemCh {
				// Check if the context was cancelled
				if ctx.Err() != nil {
					return
				}

				// Process the item
				if err := fn(item); err != nil {
					// Send the error and return
					select {
					case errCh <- err:
					default:
						// An error is already in the channel
					}
					cancel() // Cancel other workers
					return
				}
			}
		}()
	}

	// Function to feed items to workers
	processCurrent := func() {
		for _, item := range p.Items() {
			// Check for errors or cancellation
			select {
			case <-ctx.Done():
				return
			case err := <-errCh:
				errCh <- err // Put it back for the main goroutine to find
				return
			default:
				// No error, continue
			}

			itemCh <- item
		}
	}

	// Process current page if we already have one
	if p.currentPage != nil && len(p.currentPage.Items) > 0 {
		processCurrent()
	}

	// Process all remaining pages
	for p.Next(ctx) {
		processCurrent()

		// Check for cancellation
		if ctx.Err() != nil {
			break
		}
	}

	// Close the item channel and wait for workers to finish
	close(itemCh)
	wg.Wait()

	// Check if there was an error from the fetcher
	if err := p.Err(); err != nil {
		return err
	}

	// Check if there was an error from a worker
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// CollectAll is a shortcut function to create a Paginator and collect all items
func CollectAll[T any](
	ctx context.Context,
	operationName string,
	entityType string,
	fetcher PageFetcher[T],
	options PageOptions,
) ([]T, error) {
	paginator, err := NewPaginator(fetcher,
		WithOperationName(operationName),
		WithEntityType(entityType),
		WithPageOptions(options),
	)
	if err != nil {
		return []T{}, err
	}

	if !paginator.Next(ctx) {
		return []T{}, paginator.Err()
	}

	return paginator.All(ctx)
}

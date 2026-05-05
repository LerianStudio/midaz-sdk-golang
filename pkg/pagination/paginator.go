package pagination

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
)

// MaxPaginationLimit is the maximum allowed limit for pagination requests.
// This prevents excessive memory usage from overly large page requests.
const MaxPaginationLimit = models.MaxLimit

// PageFetcher is a generic function type for fetching a page of results
type PageFetcher[T any] func(ctx context.Context, options PageOptions) (*PageResult[T], error)

// PageOptions represents options for fetching a page
type PageOptions struct {
	Limit            int
	Offset           int
	Page             int
	Cursor           string
	Filters          map[string]string
	AdditionalParams map[string]string
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

	// MaxPages limits how many pages bulk operations may fetch. Zero means unlimited.
	MaxPages int

	// MaxItems limits how many items bulk operations may return/process. Zero means unlimited.
	MaxItems int
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
	maxPages      int
	maxItems      int
	exhausted     bool
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

		if limit > MaxPaginationLimit {
			return fmt.Errorf("limit must not exceed %d, got %d", MaxPaginationLimit, limit)
		}

		o.PageOptions.Limit = limit

		return nil
	}
}

//nolint:wsl_v5
func normalizePageOptions(options PageOptions, defaultLimit int) PageOptions {
	if defaultLimit <= 0 {
		defaultLimit = models.DefaultLimit
	}
	if defaultLimit > MaxPaginationLimit {
		defaultLimit = MaxPaginationLimit
	}

	if options.Limit <= 0 {
		options.Limit = defaultLimit
	} else if options.Limit > MaxPaginationLimit {
		options.Limit = MaxPaginationLimit
	}

	options.Filters = maps.Clone(options.Filters)
	options.AdditionalParams = maps.Clone(options.AdditionalParams)

	return options
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
		o.PageOptions.Filters = maps.Clone(filters)
		return nil
	}
}

// WithPageOptions sets all initial page options at once
func WithPageOptions(options PageOptions) PaginatorOption {
	return func(o *PaginatorOptions) error {
		o.PageOptions = normalizePageOptions(options, o.DefaultLimit)
		return nil
	}
}

// WithMaxPages sets the maximum number of pages fetched by All, ForEach, and Concurrent.
func WithMaxPages(maxPages int) PaginatorOption {
	return func(o *PaginatorOptions) error {
		if maxPages < 0 {
			return fmt.Errorf("max pages must be non-negative, got %d", maxPages)
		}

		o.MaxPages = maxPages

		return nil
	}
}

// WithMaxItems sets the maximum number of items returned or processed by bulk operations.
func WithMaxItems(maxItems int) PaginatorOption {
	return func(o *PaginatorOptions) error {
		if maxItems < 0 {
			return fmt.Errorf("max items must be non-negative, got %d", maxItems)
		}

		o.MaxItems = maxItems

		return nil
	}
}

// WithObserver sets the observer for monitoring pagination operations
func WithObserver(observer Observer) PaginatorOption {
	return func(o *PaginatorOptions) error {
		if observer == nil {
			return errors.New("observer cannot be nil")
		}

		o.Observer = observer

		return nil
	}
}

// WithOperationName sets the operation name for metrics and logging
func WithOperationName(operationName string) PaginatorOption {
	return func(o *PaginatorOptions) error {
		if operationName == "" {
			return errors.New("operation name cannot be empty")
		}

		o.OperationName = operationName

		return nil
	}
}

// WithEntityType sets the entity type for metrics and logging
func WithEntityType(entityType string) PaginatorOption {
	return func(o *PaginatorOptions) error {
		if entityType == "" {
			return errors.New("entity type cannot be empty")
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

		if defaultLimit > MaxPaginationLimit {
			return fmt.Errorf("default limit must not exceed %d, got %d", MaxPaginationLimit, defaultLimit)
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
	if fetcher == nil {
		return nil, errors.New("page fetcher cannot be nil")
	}

	// Start with default options
	opts := DefaultPaginatorOptions()

	// Apply all provided options
	for _, option := range options {
		if option == nil {
			return nil, errors.New("paginator option cannot be nil")
		}

		if err := option(opts); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Ensure we have an observer
	if opts.Observer == nil {
		opts.Observer = NewObserver()
	}

	opts.PageOptions = normalizePageOptions(opts.PageOptions, opts.DefaultLimit)

	// Create and return the paginator
	return &defaultPaginator[T]{
		fetcher:       fetcher,
		options:       opts.PageOptions,
		pageNumber:    0,
		observer:      opts.Observer,
		operationName: opts.OperationName,
		entityType:    opts.EntityType,
		maxPages:      opts.MaxPages,
		maxItems:      opts.MaxItems,
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
			options:       normalizePageOptions(initialOptions, 10),
			pageNumber:    0,
			observer:      observerToUse,
			operationName: operationName,
			entityType:    entityType,
		}
	}

	return paginator
}

// Next advances to the next page of results.
//
//nolint:funlen,nestif,wsl_v5
func (p *defaultPaginator[T]) Next(ctx context.Context) bool {
	if p == nil {
		return false
	}

	if err := ctx.Err(); err != nil {
		p.setErr(err)
		return false
	}

	p.mu.Lock()
	if p.err != nil || p.exhausted {
		p.mu.Unlock()
		return false
	}

	options := p.options
	nextPageNumber := p.pageNumber + 1
	operationName := p.operationName
	entityType := p.entityType
	fetcher := p.fetcher
	p.mu.Unlock()

	start := time.Now()
	event := &Event{
		Operation:  operationName,
		EntityType: entityType,
		Limit:      options.Limit,
		Offset:     options.Offset,
		Page:       nextPageNumber,
		CursorUsed: options.Cursor != "",
	}

	// Fetch the next page
	pageResult, err := fetcher(ctx, options)

	// Record pagination metrics
	duration := time.Since(start)

	if err != nil {
		p.setErr(err)
		event.Error = err
		event.Duration = duration
		p.recordEvent(ctx, event)

		return false
	}
	if pageResult == nil {
		err = errors.New("page fetcher returned nil page result")
		p.setErr(err)
		event.Error = err
		event.Duration = duration
		p.recordEvent(ctx, event)

		return false
	}
	if pageResult.Items == nil {
		pageResult.Items = make([]T, 0)
	}

	p.mu.Lock()
	p.currentPage = pageResult

	// Update page information
	p.pageNumber++

	if pageResult.Total > 0 {
		p.totalItems = pageResult.Total
	}

	// Update options for the next page
	if pageResult.NextCursor != "" {
		// Use cursor-based pagination if available
		p.options.Cursor = pageResult.NextCursor
		p.options.Offset = 0 // Reset offset when using cursor
		p.options.Page = 0
	} else if options.Cursor != "" {
		p.exhausted = true
	} else if pageResult.HasMore {
		if p.options.Page > 0 {
			p.options.Page++
		} else {
			p.options.Offset += p.options.Limit
		}
	} else {
		p.exhausted = true
	}
	if len(pageResult.Items) == 0 {
		p.exhausted = true
	}
	totalItems := p.totalItems
	p.mu.Unlock()

	event.ProcessedItems = len(pageResult.Items)
	event.TotalItems = totalItems
	event.HasNextPage = pageResult.HasMore && !p.isExhausted()
	event.Duration = duration
	p.recordEvent(ctx, event)

	// Return false if we've reached the end or got an empty page
	return len(pageResult.Items) > 0
}

//nolint:wsl_v5
func (p *defaultPaginator[T]) setErr(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
}

//nolint:wsl_v5
func (p *defaultPaginator[T]) isExhausted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exhausted
}

//nolint:wsl_v5
func (p *defaultPaginator[T]) recordEvent(ctx context.Context, event *Event) {
	p.mu.Lock()
	observer := p.observer
	p.mu.Unlock()

	if observer == nil {
		return
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			return
		}
	}()
	observer.RecordEvent(ctx, event)
}

// Items returns the items in the current page
func (p *defaultPaginator[T]) Items() []T {
	if p == nil {
		return []T{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.currentPage == nil {
		return []T{}
	}

	if p.currentPage.Items == nil {
		return []T{}
	}

	return p.currentPage.Items
}

// Err returns any error that occurred during pagination
func (p *defaultPaginator[T]) Err() error {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	return p.err
}

// PageInfo returns information about the current page
func (p *defaultPaginator[T]) PageInfo() PageInfo {
	if p == nil {
		return PageInfo{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	info := PageInfo{
		PageNumber:   p.pageNumber,
		ItemsPerPage: p.options.Limit,
		TotalItems:   p.totalItems,
	}

	if p.currentPage != nil {
		info.HasNextPage = p.currentPage.HasMore && !p.exhausted
		info.HasPrevPage = p.pageNumber > 1 || p.currentPage.PrevCursor != ""

		// Calculate total pages if we know the total items
		if p.totalItems > 0 && p.options.Limit > 0 {
			info.TotalPages = (p.totalItems + p.options.Limit - 1) / p.options.Limit
		}
	}

	return info
}

// All retrieves all remaining items across multiple pages.
//
//nolint:wsl_v5
func (p *defaultPaginator[T]) All(ctx context.Context) ([]T, error) {
	if p == nil {
		return []T{}, nil
	}
	if err := ctx.Err(); err != nil {
		return []T{}, err
	}

	allItems := make([]T, 0)
	pagesFetched := 0

	// Add current page items if we already have a page
	p.mu.Lock()
	if p.currentPage != nil {
		allItems = append(allItems, p.currentPage.Items...)
		pagesFetched = 1
	}
	p.mu.Unlock()
	if p.maxItems > 0 && len(allItems) >= p.maxItems {
		return allItems[:p.maxItems], nil
	}

	for p.maxPages == 0 || pagesFetched < p.maxPages {
		if err := ctx.Err(); err != nil {
			return allItems, err
		}
		if !p.Next(ctx) {
			break
		}

		pagesFetched++
		allItems = append(allItems, p.Items()...)

		if p.maxItems > 0 && len(allItems) >= p.maxItems {
			return allItems[:p.maxItems], nil
		}
	}

	if err := p.Err(); err != nil {
		return allItems, err
	}

	return allItems, nil
}

// ForEach iterates through all items across pages.
//
//nolint:cyclop,gocognit,gocyclo,revive,wsl_v5
func (p *defaultPaginator[T]) ForEach(ctx context.Context, fn func(item T) error) error {
	if p == nil {
		return nil
	}
	if fn == nil {
		return errors.New("item callback cannot be nil")
	}

	processed := 0
	pagesFetched := 0

	// Process current page items if we already have a page
	p.mu.Lock()
	currentPage := p.currentPage
	p.mu.Unlock()
	if currentPage != nil {
		pagesFetched = 1
		for _, item := range currentPage.Items {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			if err := fn(item); err != nil {
				return err
			}
			processed++
			if p.maxItems > 0 && processed >= p.maxItems {
				return nil
			}
		}
	}

	// Process all remaining pages
	for (p.maxPages == 0 || pagesFetched < p.maxPages) && p.Next(ctx) {
		pagesFetched++
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
			processed++
			if p.maxItems > 0 && processed >= p.maxItems {
				return nil
			}
		}
	}

	return p.Err()
}

// concurrentWorkerConfig holds the configuration for concurrent processing.
type concurrentWorkerConfig[T any] struct {
	itemCh chan T
	errCh  chan error
	cancel context.CancelFunc
	fn     func(item T) error
}

// startWorker runs a single worker that processes items from the channel.
//
//nolint:wsl_v5
func startWorker[T any](ctx context.Context, wg *sync.WaitGroup, cfg *concurrentWorkerConfig[T]) {
	defer wg.Done()
	defer func() {
		if recovered := recover(); recovered != nil {
			select {
			case cfg.errCh <- fmt.Errorf("pagination worker panic: %v", recovered):
			default:
			}
			cfg.cancel()
		}
	}()

	for item := range cfg.itemCh {
		if ctx.Err() != nil {
			return
		}

		if err := cfg.fn(item); err != nil {
			select {
			case cfg.errCh <- err:
			default:
			}

			cfg.cancel()

			return
		}
	}
}

// feedItemsToChannel sends items to the worker channel, respecting context cancellation.
func feedItemsToChannel[T any](ctx context.Context, items []T, itemCh chan<- T, errCh <-chan error) error {
	for _, item := range items {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case itemCh <- item:
		}
	}

	return nil
}

// collectWorkerError checks for errors from workers after processing completes.
func collectWorkerError(errCh <-chan error) error {
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// Concurrent processes items concurrently with the specified number of workers.
//
//nolint:cyclop,funlen,gocognit,gocyclo,revive,wsl_v5
func (p *defaultPaginator[T]) Concurrent(ctx context.Context, workers int, fn func(item T) error) error {
	if p == nil {
		return nil
	}
	if fn == nil {
		return errors.New("item callback cannot be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if workers <= 0 {
		workers = 5 // Default to 5 workers
	}

	itemCh := make(chan T, workers*2)
	errCh := make(chan error, 1)

	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cfg := &concurrentWorkerConfig[T]{
		itemCh: itemCh,
		errCh:  errCh,
		cancel: cancel,
		fn:     fn,
	}

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)

		go startWorker(ctx, &wg, cfg)
	}

	// Process current page if we already have one
	pagesFetched := 0
	p.mu.Lock()
	currentPage := p.currentPage
	p.mu.Unlock()
	if currentPage != nil && len(currentPage.Items) > 0 {
		pagesFetched = 1
		if err := feedItemsToChannel(ctx, p.Items(), itemCh, errCh); err != nil {
			close(itemCh)
			wg.Wait()
			if collectedErr := collectWorkerError(errCh); collectedErr != nil {
				return collectedErr
			}

			return err
		}
	}

	// Process all remaining pages
	var workerErr error
	for p.maxPages == 0 || pagesFetched < p.maxPages {
		if err := collectWorkerError(errCh); err != nil {
			workerErr = err
			break
		}
		if !p.Next(ctx) {
			break
		}
		pagesFetched++
		if err := feedItemsToChannel(ctx, p.Items(), itemCh, errCh); err != nil {
			close(itemCh)
			wg.Wait()
			if collectedErr := collectWorkerError(errCh); collectedErr != nil {
				return collectedErr
			}

			return err
		}

		if ctx.Err() != nil {
			break
		}
	}

	close(itemCh)
	wg.Wait()
	if workerErr != nil {
		return workerErr
	}
	if err := collectWorkerError(errCh); err != nil {
		return err
	}

	return p.Err()
}

// CollectAll is a shortcut function to create a Paginator and collect all items
func CollectAll[T any](
	ctx context.Context,
	operationName string,
	entityType string,
	fetcher PageFetcher[T],
	options PageOptions,
	additionalOptions ...PaginatorOption,
) ([]T, error) {
	paginatorOptions := make([]PaginatorOption, 0, 3+len(additionalOptions))
	paginatorOptions = append(paginatorOptions,
		WithOperationName(operationName),
		WithEntityType(entityType),
		WithPageOptions(options),
	)
	paginatorOptions = append(paginatorOptions, additionalOptions...)

	paginator, err := NewPaginator(fetcher, paginatorOptions...)
	if err != nil {
		return []T{}, err
	}

	if !paginator.Next(ctx) {
		return []T{}, paginator.Err()
	}

	return paginator.All(ctx)
}

package pagination

import (
	"context"
	"fmt"
)

// ModelAdapter provides adapter functions to convert between SDK models and pagination utilities
type ModelAdapter struct {
	// Configuration fields
	defaultLimit   int
	defaultOffset  int
	defaultFilters map[string]string
}

// ModelAdapterOption defines a function type for configuring a ModelAdapter
type ModelAdapterOption func(*ModelAdapter) error

// DefaultModelAdapter returns a ModelAdapter with default settings
func DefaultModelAdapter() *ModelAdapter {
	return &ModelAdapter{
		defaultLimit:   10,
		defaultOffset:  0,
		defaultFilters: make(map[string]string),
	}
}

// WithAdapterDefaultLimit sets the default page limit for the adapter
func WithAdapterDefaultLimit(limit int) ModelAdapterOption {
	return func(a *ModelAdapter) error {
		if limit <= 0 {
			return fmt.Errorf("default limit must be positive, got %d", limit)
		}
		a.defaultLimit = limit
		return nil
	}
}

// WithDefaultOffset sets the default page offset for the adapter
func WithDefaultOffset(offset int) ModelAdapterOption {
	return func(a *ModelAdapter) error {
		if offset < 0 {
			return fmt.Errorf("default offset must be non-negative, got %d", offset)
		}
		a.defaultOffset = offset
		return nil
	}
}

// WithDefaultFilters sets the default filters for the adapter
func WithDefaultFilters(filters map[string]string) ModelAdapterOption {
	return func(a *ModelAdapter) error {
		if filters == nil {
			return fmt.Errorf("default filters map cannot be nil")
		}
		a.defaultFilters = filters
		return nil
	}
}

// NewModelAdapter creates a new model adapter with the provided options
func NewModelAdapter(options ...ModelAdapterOption) (*ModelAdapter, error) {
	adapter := DefaultModelAdapter()

	for _, option := range options {
		if err := option(adapter); err != nil {
			return nil, fmt.Errorf("failed to apply adapter option: %w", err)
		}
	}

	return adapter, nil
}

// OptionsToPageOptions converts SDK ListOptions to PageOptions
func (a *ModelAdapter) OptionsToPageOptions(opts any) PageOptions {
	// This is just a type assertion example; adjust based on actual SDK types
	if listOpts, ok := opts.(interface {
		GetLimit() int
		GetOffset() int
		GetCursor() string
		GetFilters() map[string]string
	}); ok {
		return PageOptions{
			Limit:   listOpts.GetLimit(),
			Offset:  listOpts.GetOffset(),
			Cursor:  listOpts.GetCursor(),
			Filters: listOpts.GetFilters(),
		}
	}

	// Default values if conversion fails
	return PageOptions{
		Limit:   a.defaultLimit,
		Offset:  a.defaultOffset,
		Filters: a.defaultFilters,
	}
}

// PageResultFromResponse converts a ListResponse to PageResult
// The T and R type parameters represent the target item type and response item type respectively
func PageResultFromResponse[T any, R any](adapter *ModelAdapter, response any, itemsExtractor func(R) T) *PageResult[T] {
	// This is just a type assertion example; adjust based on actual SDK types
	if listResp, ok := response.(interface {
		GetItems() []R
		GetPagination() interface {
			GetNextCursor() string
			GetPrevCursor() string
			GetTotal() int
			HasMorePages() bool
		}
	}); ok {
		pagination := listResp.GetPagination()
		items := listResp.GetItems()

		var extractedItems []T
		for _, item := range items {
			extractedItems = append(extractedItems, itemsExtractor(item))
		}

		return &PageResult[T]{
			Items:      extractedItems,
			NextCursor: pagination.GetNextCursor(),
			PrevCursor: pagination.GetPrevCursor(),
			Total:      pagination.GetTotal(),
			HasMore:    pagination.HasMorePages(),
		}
	}

	// Return empty result if conversion fails
	return &PageResult[T]{
		Items:   []T{},
		HasMore: false,
	}
}

// EntityPaginatorOption defines a function that configures a EntityPaginatorOptions object
type EntityPaginatorOption func(*EntityPaginatorOptions) error

// EntityPaginatorOptions holds all configuration options for an entity paginator
type EntityPaginatorOptions struct {
	// Initial page options
	InitialOptions any

	// Operation name for metrics and logging
	OperationName string

	// Entity type for metrics and logging
	EntityType string

	// Model adapter options
	AdapterOptions []ModelAdapterOption

	// Paginator options
	PaginatorOptions []PaginatorOption
}

// WithEntityInitialOptions sets the initial options for entity pagination
func WithEntityInitialOptions(options any) EntityPaginatorOption {
	return func(o *EntityPaginatorOptions) error {
		if options == nil {
			return fmt.Errorf("initial options cannot be nil")
		}
		o.InitialOptions = options
		return nil
	}
}

// WithEntityOperationName sets the operation name for entity pagination
func WithEntityOperationName(name string) EntityPaginatorOption {
	return func(o *EntityPaginatorOptions) error {
		if name == "" {
			return fmt.Errorf("operation name cannot be empty")
		}
		o.OperationName = name
		return nil
	}
}

// WithPaginatorEntityType sets the entity type for entity pagination
func WithPaginatorEntityType(entityType string) EntityPaginatorOption {
	return func(o *EntityPaginatorOptions) error {
		if entityType == "" {
			return fmt.Errorf("entity type cannot be empty")
		}
		o.EntityType = entityType
		return nil
	}
}

// WithEntityAdapterOptions sets the model adapter options for entity pagination
func WithEntityAdapterOptions(options ...ModelAdapterOption) EntityPaginatorOption {
	return func(o *EntityPaginatorOptions) error {
		o.AdapterOptions = append(o.AdapterOptions, options...)
		return nil
	}
}

// WithEntityPaginatorOptions sets the paginator options for entity pagination
func WithEntityPaginatorOptions(options ...PaginatorOption) EntityPaginatorOption {
	return func(o *EntityPaginatorOptions) error {
		o.PaginatorOptions = append(o.PaginatorOptions, options...)
		return nil
	}
}

// DefaultEntityPaginatorOptions returns the default options for an entity paginator
func DefaultEntityPaginatorOptions() *EntityPaginatorOptions {
	return &EntityPaginatorOptions{
		OperationName:    "list",
		EntityType:       "entity",
		AdapterOptions:   []ModelAdapterOption{},
		PaginatorOptions: []PaginatorOption{},
	}
}

// CreateEntityPaginator creates a Paginator for entity list operations
func CreateEntityPaginator[T any, R any](
	ctx context.Context,
	listFn func(context.Context, any) (any, error),
	itemsExtractor func(R) T,
	options ...EntityPaginatorOption,
) (Paginator[T], error) {
	// Start with default options
	opts := DefaultEntityPaginatorOptions()

	// Apply all provided options
	for _, option := range options {
		if err := option(opts); err != nil {
			return nil, fmt.Errorf("failed to apply entity paginator option: %w", err)
		}
	}

	// Create adapter with options
	adapter, err := NewModelAdapter(opts.AdapterOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create model adapter: %w", err)
	}

	// Convert the initial options
	pageOptions := adapter.OptionsToPageOptions(opts.InitialOptions)

	// Add page options to paginator options
	paginatorOpts := append([]PaginatorOption{
		WithOperationName(opts.OperationName),
		WithEntityType(opts.EntityType),
		WithPageOptions(pageOptions),
	}, opts.PaginatorOptions...)

	// Create a fetcher function that adapts the SDK list function
	fetcher := func(ctx context.Context, options PageOptions) (*PageResult[T], error) {
		// Convert options back to SDK format (this would depend on the actual SDK)
		sdkOptions := opts.InitialOptions // A shallow copy would be made here

		// Call the list function
		response, err := listFn(ctx, sdkOptions)
		if err != nil {
			return nil, err
		}

		// Convert the response
		return PageResultFromResponse[T, R](adapter, response, itemsExtractor), nil
	}

	// Create and return the paginator
	return NewPaginator(fetcher, paginatorOpts...)
}

// CreateEntityPaginatorWithDefaults creates a Paginator for entity list operations with simplified parameters
// This function is provided for backward compatibility
func CreateEntityPaginatorWithDefaults[T any, R any](
	ctx context.Context,
	operationName string,
	entityType string,
	listFn func(context.Context, any) (any, error),
	initialOptions any,
	itemsExtractor func(R) T,
) Paginator[T] {
	paginator, err := CreateEntityPaginator[T, R](
		ctx,
		listFn,
		itemsExtractor,
		WithEntityOperationName(operationName),
		WithPaginatorEntityType(entityType),
		WithEntityInitialOptions(initialOptions),
	)

	if err != nil {
		// For backward compatibility, create a simple paginator directly
		adapter, _ := NewModelAdapter() // Ignore error, using default adapter
		pageOptions := adapter.OptionsToPageOptions(initialOptions)

		// Create a fetcher function that adapts the SDK list function
		fetcher := func(ctx context.Context, options PageOptions) (*PageResult[T], error) {
			// Call the list function
			response, err := listFn(ctx, initialOptions)
			if err != nil {
				return nil, err
			}

			// Convert the response
			return PageResultFromResponse[T, R](adapter, response, itemsExtractor), nil
		}

		return NewPaginatorWithDefaults[T](operationName, entityType, fetcher, pageOptions, nil)
	}

	return paginator
}

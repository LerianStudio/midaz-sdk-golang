package pagination

import (
	"context"
	"errors"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
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

		if limit > MaxPaginationLimit {
			return fmt.Errorf("default limit must not exceed %d, got %d", MaxPaginationLimit, limit)
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
			return errors.New("default filters map cannot be nil")
		}

		a.defaultFilters = copyStringMap(filters)

		return nil
	}
}

func copyStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}

	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}

	return clone
}

// NewModelAdapter creates a new model adapter with the provided options
func NewModelAdapter(options ...ModelAdapterOption) (*ModelAdapter, error) {
	adapter := DefaultModelAdapter()

	for _, option := range options {
		if option == nil {
			return nil, errors.New("model adapter option cannot be nil")
		}

		if err := option(adapter); err != nil {
			return nil, fmt.Errorf("failed to apply adapter option: %w", err)
		}
	}

	return adapter, nil
}

// OptionsToPageOptions converts SDK ListOptions to PageOptions
func (a *ModelAdapter) OptionsToPageOptions(opts any) PageOptions {
	if a == nil {
		a = DefaultModelAdapter()
	}

	if listOpts, ok := opts.(*models.ListOptions); ok && listOpts != nil {
		return PageOptions{
			Limit:            normalizeLimit(listOpts.Limit, a.defaultLimit),
			Offset:           listOpts.Offset,
			Page:             listOpts.Page,
			Cursor:           listOpts.Cursor,
			Filters:          copyStringMap(listOpts.Filters),
			AdditionalParams: copyStringMap(listOpts.AdditionalParams),
		}
	}

	// This is just a type assertion example; adjust based on actual SDK types
	if listOpts, ok := opts.(interface {
		GetLimit() int
		GetOffset() int
		GetCursor() string
		GetFilters() map[string]string
	}); ok {
		return PageOptions{
			Limit:   normalizeLimit(listOpts.GetLimit(), a.defaultLimit),
			Offset:  listOpts.GetOffset(),
			Cursor:  listOpts.GetCursor(),
			Filters: copyStringMap(listOpts.GetFilters()),
		}
	}

	// Default values if conversion fails
	return PageOptions{
		Limit:   a.defaultLimit,
		Offset:  a.defaultOffset,
		Filters: copyStringMap(a.defaultFilters),
	}
}

func normalizeLimit(limit, defaultLimit int) int {
	if limit <= 0 {
		if defaultLimit <= 0 {
			return models.DefaultLimit
		}

		return defaultLimit
	}

	if limit > MaxPaginationLimit {
		return MaxPaginationLimit
	}

	return limit
}

//nolint:wsl_v5
func (a *ModelAdapter) applyPageOptions(initialOptions any, pageOptions PageOptions) any {
	if listOptions, ok := initialOptions.(*models.ListOptions); ok {
		updated := listOptions.Clone()
		updated.WithLimit(normalizeLimit(pageOptions.Limit, a.defaultLimit))
		updated.Offset = pageOptions.Offset
		updated.Page = pageOptions.Page
		updated.Cursor = pageOptions.Cursor
		if pageOptions.Filters != nil {
			updated.Filters = copyStringMap(pageOptions.Filters)
		}
		if pageOptions.AdditionalParams != nil {
			updated.AdditionalParams = copyStringMap(pageOptions.AdditionalParams)
		}

		return updated
	}

	return initialOptions
}

// PageResultFromResponse converts a ListResponse to PageResult
// The T and R type parameters represent the target item type and response item type respectively
func PageResultFromResponse[T any, R any](_ *ModelAdapter, response any, itemsExtractor func(R) T) *PageResult[T] {
	result, err := PageResultFromResponseE[T, R](nil, response, itemsExtractor)
	if err != nil {
		return &PageResult[T]{Items: []T{}, HasMore: false}
	}

	return result
}

// PageResultFromResponseE converts a ListResponse to PageResult and reports unsafe conversions.
func PageResultFromResponseE[T any, R any](_ *ModelAdapter, response any, itemsExtractor func(R) T) (*PageResult[T], error) {
	if itemsExtractor == nil {
		return nil, errors.New("items extractor cannot be nil")
	}

	if listResp, ok := response.(*models.ListResponse[R]); ok && listResp != nil {
		extractedItems := make([]T, 0, len(listResp.Items))
		for _, item := range listResp.Items {
			extractedItems = append(extractedItems, itemsExtractor(item))
		}

		return &PageResult[T]{
			Items:      extractedItems,
			NextCursor: listResp.Pagination.NextCursor,
			PrevCursor: listResp.Pagination.PrevCursor,
			Total:      listResp.Pagination.Total,
			HasMore:    listResp.Pagination.HasNextPage(),
		}, nil
	}

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

		extractedItems := make([]T, 0, len(items))
		for _, item := range items {
			extractedItems = append(extractedItems, itemsExtractor(item))
		}

		if pagination == nil {
			return &PageResult[T]{Items: extractedItems, HasMore: false}, nil
		}

		return &PageResult[T]{
			Items:      extractedItems,
			NextCursor: pagination.GetNextCursor(),
			PrevCursor: pagination.GetPrevCursor(),
			Total:      pagination.GetTotal(),
			HasMore:    pagination.HasMorePages(),
		}, nil
	}

	// Return empty result if conversion fails
	return &PageResult[T]{
		Items:   []T{},
		HasMore: false,
	}, nil
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
			return errors.New("initial options cannot be nil")
		}

		o.InitialOptions = options

		return nil
	}
}

// WithEntityOperationName sets the operation name for entity pagination
func WithEntityOperationName(name string) EntityPaginatorOption {
	return func(o *EntityPaginatorOptions) error {
		if name == "" {
			return errors.New("operation name cannot be empty")
		}

		o.OperationName = name

		return nil
	}
}

// WithPaginatorEntityType sets the entity type for entity pagination
func WithPaginatorEntityType(entityType string) EntityPaginatorOption {
	return func(o *EntityPaginatorOptions) error {
		if entityType == "" {
			return errors.New("entity type cannot be empty")
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
	_ context.Context,
	listFn func(context.Context, any) (any, error),
	itemsExtractor func(R) T,
	options ...EntityPaginatorOption,
) (Paginator[T], error) {
	// Start with default options
	opts := DefaultEntityPaginatorOptions()

	// Apply all provided options
	for _, option := range options {
		if option == nil {
			return nil, errors.New("entity paginator option cannot be nil")
		}

		if err := option(opts); err != nil {
			return nil, fmt.Errorf("failed to apply entity paginator option: %w", err)
		}
	}

	if listFn == nil {
		return nil, errors.New("list function cannot be nil")
	}

	if itemsExtractor == nil {
		return nil, errors.New("items extractor cannot be nil")
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
	fetcher := func(ctx context.Context, pageOptions PageOptions) (*PageResult[T], error) {
		sdkOptions := adapter.applyPageOptions(opts.InitialOptions, pageOptions)

		// Call the list function
		response, err := listFn(ctx, sdkOptions)
		if err != nil {
			return nil, err
		}

		// Convert the response
		return PageResultFromResponseE[T, R](adapter, response, itemsExtractor)
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
		// NewModelAdapter with no options always succeeds
		adapter, _ := NewModelAdapter() //nolint:errcheck // default options never fail
		pageOptions := adapter.OptionsToPageOptions(initialOptions)

		// Create a fetcher function that adapts the SDK list function
		fetcher := func(ctx context.Context, _ PageOptions) (*PageResult[T], error) {
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

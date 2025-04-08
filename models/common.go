package models

import (
	"fmt"
	"time"

	"github.com/LerianStudio/midaz/pkg/mmodel"
)

// Status represents the status of an entity in the Midaz system.
// It contains a status code and an optional description providing additional context.
// Status is used across various models to indicate the current state of resources.
type Status struct {
	// Code is the status code identifier (e.g., "active", "pending", "closed")
	Code string `json:"code"`

	// Description provides optional additional context about the status
	Description *string `json:"description,omitempty"`
}

// NewStatus creates a new Status with the given code.
// This is a convenience constructor for creating Status objects.
//
// Parameters:
//   - code: The status code to set (e.g., "active", "pending", "closed")
//
// Returns:
//   - A new Status instance with the specified code
func NewStatus(code string) Status {
	return Status{
		Code: code,
	}
}

// WithDescription adds a description to the status.
// This is a fluent-style method that returns the modified Status.
//
// Parameters:
//   - description: The description text to add to the status
//
// Returns:
//   - The modified Status instance with the added description
func (s Status) WithDescription(description string) Status {
	s.Description = &description
	return s
}

// IsEmpty returns true if the status is empty.
// A status is considered empty if it has no code and no description.
//
// Returns:
//   - true if the status is empty, false otherwise
func (s Status) IsEmpty() bool {
	return s.Code == "" && s.Description == nil
}

// ToMmodelStatus converts an SDK Status to an mmodel Status (internal use only).
// This method is used for internal SDK operations when interfacing with the backend.
//
// Returns:
//   - An mmodel.Status instance with the same values as this Status
func (s Status) ToMmodelStatus() mmodel.Status {
	return mmodel.Status{
		Code:        s.Code,
		Description: s.Description,
	}
}

// FromMmodelStatus converts an mmodel Status to an SDK Status (internal use only).
// This function is used for internal SDK operations when processing responses from the backend.
//
// Parameters:
//   - modelStatus: The mmodel.Status to convert
//
// Returns:
//   - A models.Status instance with the same values as the input mmodel.Status
func FromMmodelStatus(modelStatus mmodel.Status) Status {
	return Status{
		Code:        modelStatus.Code,
		Description: modelStatus.Description,
	}
}

// Address represents a physical address.
// This structure is used across various models where address information is required,
// such as for organizations or account holders.
type Address struct {
	// Line1 is the primary address line (e.g., street number and name)
	Line1 string `json:"line1"`

	// Line2 is an optional secondary address line (e.g., apartment or suite number)
	Line2 *string `json:"line2,omitempty"`

	// ZipCode is the postal or ZIP code
	ZipCode string `json:"zipCode"`

	// City is the city or locality name
	City string `json:"city"`

	// State is the state, province, or region
	State string `json:"state"`

	// Country is the country, typically using ISO country codes
	Country string `json:"country"`
}

// NewAddress creates a new Address with the given parameters.
// This is a convenience constructor for creating Address objects with required fields.
//
// Parameters:
//   - line1: The primary address line
//   - zipCode: The postal or ZIP code
//   - city: The city or locality name
//   - state: The state, province, or region
//   - country: The country code
//
// Returns:
//   - A new Address instance with the specified fields
func NewAddress(line1, zipCode, city, state, country string) Address {
	return Address{
		Line1:   line1,
		ZipCode: zipCode,
		City:    city,
		State:   state,
		Country: country,
	}
}

// WithLine2 adds the optional Line2 field to the address.
// This is a fluent-style method that returns the modified Address.
//
// Parameters:
//   - line2: The secondary address line to add
//
// Returns:
//   - The modified Address instance with the added Line2
func (a Address) WithLine2(line2 string) Address {
	a.Line2 = &line2
	return a
}

// ToMmodelAddress converts an SDK Address to an mmodel Address (internal use only).
// This method is used for internal SDK operations when interfacing with the backend.
//
// Returns:
//   - An mmodel.Address instance with the same values as this Address
func (a Address) ToMmodelAddress() mmodel.Address {
	return mmodel.Address{
		Line1:   a.Line1,
		Line2:   a.Line2,
		ZipCode: a.ZipCode,
		City:    a.City,
		State:   a.State,
		Country: a.Country,
	}
}

// FromMmodelAddress converts an mmodel Address to an SDK Address (internal use only).
// This function is used for internal SDK operations when processing responses from the backend.
//
// Parameters:
//   - modelAddress: The mmodel.Address to convert
//
// Returns:
//   - A models.Address instance with the same values as the input mmodel.Address
func FromMmodelAddress(modelAddress mmodel.Address) Address {
	return Address{
		Line1:   modelAddress.Line1,
		Line2:   modelAddress.Line2,
		ZipCode: modelAddress.ZipCode,
		City:    modelAddress.City,
		State:   modelAddress.State,
		Country: modelAddress.Country,
	}
}

// Pagination represents pagination information for list operations.
// This structure is used in list responses to provide context about the pagination state
// and to help with navigating through paginated results.
type Pagination struct {
	// Limit is the number of items per page
	Limit int `json:"limit"`

	// Offset is the starting position for the current page
	Offset int `json:"offset"`

	// Total is the total number of items available across all pages
	Total int `json:"total"`

	// PrevCursor is the cursor for the previous page (for cursor-based pagination)
	PrevCursor string `json:"prevCursor,omitempty"`

	// NextCursor is the cursor for the next page (for cursor-based pagination)
	NextCursor string `json:"nextCursor,omitempty"`
}

// HasMorePages returns true if there are more pages available.
// This is determined by checking if the offset plus limit is less than the total.
//
// Returns:
//   - true if there are more pages available, false otherwise
func (p *Pagination) HasMorePages() bool {
	return p.Offset+p.Limit < p.Total
}

// HasPrevPage returns true if there is a previous page available.
// This is determined by checking if the offset is greater than 0 or if a previous cursor is available.
//
// Returns:
//   - true if there is a previous page available, false otherwise
func (p *Pagination) HasPrevPage() bool {
	return p.Offset > 0 || p.PrevCursor != ""
}

// HasNextPage returns true if there is a next page available.
// This is determined by checking if there are more pages or if a next cursor is available.
//
// Returns:
//   - true if there is a next page available, false otherwise
func (p *Pagination) HasNextPage() bool {
	return p.HasMorePages() || p.NextCursor != ""
}

// NextPageOptions returns options for fetching the next page.
// This method uses the most appropriate pagination method (offset or cursor-based)
// based on what information is available.
//
// Returns:
//   - A new ListOptions instance configured for the next page
//   - nil if there is no next page available
func (p *Pagination) NextPageOptions() *ListOptions {
	if !p.HasNextPage() {
		return nil
	}

	options := NewListOptions().WithLimit(p.Limit)

	// Prefer cursor-based pagination if available
	if p.NextCursor != "" {
		return options.WithCursor(p.NextCursor)
	}

	// Fall back to offset-based pagination
	return options.WithOffset(p.Offset + p.Limit)
}

// PrevPageOptions returns options for fetching the previous page.
// This method uses the most appropriate pagination method (offset or cursor-based)
// based on what information is available.
//
// Returns:
//   - A new ListOptions instance configured for the previous page
//   - nil if there is no previous page available
func (p *Pagination) PrevPageOptions() *ListOptions {
	if !p.HasPrevPage() {
		return nil
	}

	options := NewListOptions().WithLimit(p.Limit)

	// Prefer cursor-based pagination if available
	if p.PrevCursor != "" {
		return options.WithCursor(p.PrevCursor)
	}

	// Fall back to offset-based pagination
	newOffset := p.Offset - p.Limit
	if newOffset < 0 {
		newOffset = 0
	}
	return options.WithOffset(newOffset)
}

// CurrentPage returns the current page number (1-based).
// This is calculated based on the limit and offset values.
//
// Returns:
//   - The current page number (starts from 1)
func (p *Pagination) CurrentPage() int {
	if p.Limit <= 0 {
		return 1
	}
	return (p.Offset / p.Limit) + 1
}

// TotalPages returns the total number of pages available.
// This is calculated based on the total items and limit values.
//
// Returns:
//   - The total number of pages
func (p *Pagination) TotalPages() int {
	if p.Limit <= 0 {
		return 1
	}
	pages := p.Total / p.Limit
	if p.Total%p.Limit > 0 {
		pages++
	}
	return pages
}

// ListOptions represents the common options for list operations.
// This structure is used to specify filtering, pagination, and sorting parameters
// when retrieving lists of resources from the Midaz API.
type ListOptions struct {
	// Limit is the maximum number of items to return per page
	Limit int `json:"limit,omitempty"`

	// Offset is the starting position for pagination
	Offset int `json:"offset,omitempty"`

	// Filters are additional filters to apply to the query
	// The map keys are filter names and values are the filter criteria
	Filters map[string]string `json:"filters,omitempty"`

	// OrderBy specifies the field to order results by
	OrderBy string `json:"orderBy,omitempty"`

	// OrderDirection is the order direction ("asc" for ascending or "desc" for descending)
	OrderDirection string `json:"orderDirection,omitempty"`

	// Page is the page number to return (when using page-based pagination)
	// This is kept for backward compatibility
	Page int `json:"page,omitempty"`

	// Cursor is the cursor for pagination (when using cursor-based pagination)
	// This is kept for backward compatibility
	Cursor string `json:"cursor,omitempty"`

	// StartDate and EndDate for filtering by date range
	// These should be in ISO 8601 format (YYYY-MM-DD)
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`

	// AdditionalParams contains additional parameters that are specific to certain endpoints
	// These parameters are not serialized to JSON but are used when making API requests
	AdditionalParams map[string]string `json:"-"`
}

// NewListOptions creates a new ListOptions with default values.
// This constructor ensures that the default pagination values are applied consistently.
//
// Returns:
//   - A new ListOptions instance with default values
func NewListOptions() *ListOptions {
	return &ListOptions{
		Limit:          DefaultLimit,
		Offset:         DefaultOffset,
		OrderDirection: DefaultSortDirection,
	}
}

// WithLimit sets the maximum number of items to return per page.
// This method validates that the limit is within acceptable bounds.
//
// Parameters:
//   - limit: The maximum number of items to return (will be capped at MaxLimit)
//
// Returns:
//   - The modified ListOptions instance for method chaining
func (o *ListOptions) WithLimit(limit int) *ListOptions {
	if limit <= 0 {
		o.Limit = DefaultLimit
	} else if limit > MaxLimit {
		o.Limit = MaxLimit
	} else {
		o.Limit = limit
	}
	return o
}

// WithOffset sets the starting position for pagination.
//
// Parameters:
//   - offset: The starting position (must be >= 0)
//
// Returns:
//   - The modified ListOptions instance for method chaining
func (o *ListOptions) WithOffset(offset int) *ListOptions {
	if offset < 0 {
		o.Offset = DefaultOffset
	} else {
		o.Offset = offset
	}
	return o
}

// WithPage sets the page number for backward compatibility.
// Note: Using offset-based pagination (WithOffset) is recommended over page-based pagination.
//
// Parameters:
//   - page: The page number (must be >= 1)
//
// Returns:
//   - The modified ListOptions instance for method chaining
func (o *ListOptions) WithPage(page int) *ListOptions {
	if page < 1 {
		o.Page = DefaultPage
	} else {
		o.Page = page
	}
	return o
}

// WithCursor sets the cursor for cursor-based pagination.
//
// Parameters:
//   - cursor: The pagination cursor
//
// Returns:
//   - The modified ListOptions instance for method chaining
func (o *ListOptions) WithCursor(cursor string) *ListOptions {
	o.Cursor = cursor
	return o
}

// WithOrderBy sets the field to order results by.
//
// Parameters:
//   - field: The field name to sort by
//
// Returns:
//   - The modified ListOptions instance for method chaining
func (o *ListOptions) WithOrderBy(field string) *ListOptions {
	o.OrderBy = field
	return o
}

// WithOrderDirection sets the sort direction.
//
// Parameters:
//   - direction: The sort direction (use models.SortAscending or models.SortDescending)
//
// Returns:
//   - The modified ListOptions instance for method chaining
func (o *ListOptions) WithOrderDirection(direction SortDirection) *ListOptions {
	o.OrderDirection = string(direction)
	return o
}

// WithFilter adds a filter criterion.
//
// Parameters:
//   - key: The filter name
//   - value: The filter value
//
// Returns:
//   - The modified ListOptions instance for method chaining
func (o *ListOptions) WithFilter(key, value string) *ListOptions {
	if o.Filters == nil {
		o.Filters = make(map[string]string)
	}
	o.Filters[key] = value
	return o
}

// WithFilters sets multiple filters at once.
//
// Parameters:
//   - filters: A map of filter names to values
//
// Returns:
//   - The modified ListOptions instance for method chaining
func (o *ListOptions) WithFilters(filters map[string]string) *ListOptions {
	o.Filters = filters
	return o
}

// WithDateRange sets the date range for filtering.
//
// Parameters:
//   - startDate: The start date in ISO 8601 format (YYYY-MM-DD)
//   - endDate: The end date in ISO 8601 format (YYYY-MM-DD)
//
// Returns:
//   - The modified ListOptions instance for method chaining
func (o *ListOptions) WithDateRange(startDate, endDate string) *ListOptions {
	o.StartDate = startDate
	o.EndDate = endDate
	return o
}

// WithAdditionalParam adds an additional query parameter.
//
// Parameters:
//   - key: The parameter name
//   - value: The parameter value
//
// Returns:
//   - The modified ListOptions instance for method chaining
func (o *ListOptions) WithAdditionalParam(key, value string) *ListOptions {
	if o.AdditionalParams == nil {
		o.AdditionalParams = make(map[string]string)
	}
	o.AdditionalParams[key] = value
	return o
}

// NextPage returns a copy of the ListOptions configured for the next page.
// This method is useful for implementing pagination in client code.
//
// Returns:
//   - A new ListOptions instance configured for the next page
func (o *ListOptions) NextPage() *ListOptions {
	// Make a shallow copy of the current options
	next := *o

	// If using offset-based pagination
	if o.Offset >= 0 && o.Limit > 0 {
		next.Offset = o.Offset + o.Limit
	}

	// If using page-based pagination (backward compatibility)
	if o.Page > 0 {
		next.Page = o.Page + 1
	}

	// Clear cursor to avoid conflicts
	next.Cursor = ""

	return &next
}

// ToQueryParams converts ListOptions to a map of query parameters.
// This method transforms the ListOptions structure into a format suitable
// for use as URL query parameters in API requests.
//
// Returns:
//   - A map of string key-value pairs representing the query parameters
func (o *ListOptions) ToQueryParams() map[string]string {
	params := make(map[string]string)

	// Add pagination parameters
	o.addPaginationParams(params)

	// Add filtering parameters
	o.addFilteringParams(params)

	// Add sorting parameters
	o.addSortingParams(params)

	// Add date range parameters
	o.addDateRangeParams(params)

	// Add additional parameters
	o.addAdditionalParams(params)

	return params
}

// addPaginationParams adds pagination-related parameters to the query parameters map.
// This is an internal helper method used by ToQueryParams.
//
// Parameters:
//   - params: The map to add the pagination parameters to
func (o *ListOptions) addPaginationParams(params map[string]string) {
	// Always include limit parameter with at least the default
	if o.Limit <= 0 {
		params[QueryParamLimit] = fmt.Sprintf("%d", DefaultLimit)
	} else if o.Limit > MaxLimit {
		params[QueryParamLimit] = fmt.Sprintf("%d", MaxLimit)
	} else {
		params[QueryParamLimit] = fmt.Sprintf("%d", o.Limit)
	}

	// Add offset if specified
	if o.Offset > 0 {
		params[QueryParamOffset] = fmt.Sprintf("%d", o.Offset)
	}

	// These are kept for backward compatibility
	if o.Page > 0 {
		params[QueryParamPage] = fmt.Sprintf("%d", o.Page)
	}

	if o.Cursor != "" {
		params[QueryParamCursor] = o.Cursor
	}
}

// addFilteringParams adds filter-related parameters to the query parameters map.
// This is an internal helper method used by ToQueryParams.
//
// Parameters:
//   - params: The map to add the filter parameters to
func (o *ListOptions) addFilteringParams(params map[string]string) {
	if o.Filters != nil {
		for k, v := range o.Filters {
			// If the filter value is empty, skip it
			if v == "" {
				continue
			}

			params[k] = v
		}
	}
}

// addSortingParams adds sorting-related parameters to the query parameters map.
// This is an internal helper method used by ToQueryParams.
//
// Parameters:
//   - params: The map to add the sorting parameters to
func (o *ListOptions) addSortingParams(params map[string]string) {
	if o.OrderBy != "" {
		params[QueryParamOrderBy] = o.OrderBy
	}

	// Always include order direction with at least the default
	if o.OrderDirection == "" {
		params[QueryParamOrderDirection] = DefaultSortDirection
	} else {
		params[QueryParamOrderDirection] = o.OrderDirection
	}
}

// addDateRangeParams adds date range parameters to the query parameters map.
// This is an internal helper method used by ToQueryParams.
//
// Parameters:
//   - params: The map to add the date range parameters to
func (o *ListOptions) addDateRangeParams(params map[string]string) {
	if o.StartDate != "" {
		params[QueryParamStartDate] = o.StartDate
	}

	if o.EndDate != "" {
		params[QueryParamEndDate] = o.EndDate
	}
}

// addAdditionalParams adds additional parameters to the query parameters map.
// This is an internal helper method used by ToQueryParams.
//
// Parameters:
//   - params: The map to add the additional parameters to
func (o *ListOptions) addAdditionalParams(params map[string]string) {
	if o.AdditionalParams != nil {
		for k, v := range o.AdditionalParams {
			params[k] = v
		}
	}
}

// Metadata is a map of key-value pairs that can be attached to resources.
// It allows for storing arbitrary data with resources in a flexible way.
type Metadata map[string]any

// Timestamps represents common timestamp fields for resources.
// This structure is embedded in many models to provide standard
// creation, update, and deletion timestamps.
type Timestamps struct {
	// CreatedAt is the timestamp when the resource was created
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is the timestamp when the resource was last updated
	UpdatedAt time.Time `json:"updatedAt"`

	// DeletedAt is the timestamp when the resource was deleted (if applicable)
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// BaseResponse represents the common fields in all API responses.
// This structure is embedded in response models to provide standard
// fields that are present in all API responses.
type BaseResponse struct {
	// RequestID is a unique identifier for the API request
	// This can be used for troubleshooting and support
	RequestID string `json:"requestId,omitempty"`
}

// ListResponse is a generic response for list operations.
// It contains a collection of items along with pagination information.
type ListResponse[T any] struct {
	// Embedding BaseResponse to include common response fields
	BaseResponse

	// Items is the collection of resources returned by the list operation
	Items []T `json:"items"`

	// Pagination contains information about the pagination state
	Pagination Pagination `json:"pagination,omitempty"`
}

// ErrorResponse represents an error response from the API.
// This structure is used to parse and represent error responses
// returned by the Midaz API.
type ErrorResponse struct {
	// Error is the error message
	Error string `json:"error"`

	// Code is the error code for programmatic handling
	Code string `json:"code,omitempty"`

	// Details contains additional information about the error
	Details map[string]any `json:"details,omitempty"`
}

// ObjectWithMetadata is an object that has metadata.
// This interface is implemented by resources that support
// attaching arbitrary metadata.
type ObjectWithMetadata struct {
	// Metadata is a map of key-value pairs associated with the object
	Metadata map[string]any `json:"metadata,omitempty"`
}

// HasMetadata checks if the object has metadata.
//
// Returns:
//   - true if the object has metadata, false otherwise
func (o *ObjectWithMetadata) HasMetadata() bool {
	return len(o.Metadata) > 0
}

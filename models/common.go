package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"time"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

// Status represents the status of an entity in the Midaz system.
// This is now an alias to mmodel.Status to avoid duplication while maintaining
// SDK-specific documentation and examples.
// Status is used across various models to indicate the current state of resources.
type Status = mmodel.Status

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

// StatusHelpers provides utility functions for working with Status.
// Since Status is now an alias to mmodel.Status, we provide helper functions instead of methods.

// WithStatusDescription creates a new Status with a description.
func WithStatusDescription(status Status, description string) Status {
	status.Description = &description
	return status
}

// IsStatusEmpty returns true if the status is empty.
func IsStatusEmpty(status Status) bool {
	return status.Code == "" && status.Description == nil
}

func addStringField(fields map[string]any, name, value string) {
	if value != "" {
		fields[name] = value
	}
}

func addStringPtrField(fields map[string]any, name string, value *string) {
	if value != nil {
		fields[name] = value
	}
}

func addStatusField(fields map[string]any, value Status) {
	if !IsStatusEmpty(value) {
		status := map[string]any{"code": value.Code}
		if value.Description != nil {
			status["description"] = *value.Description
		}

		fields["status"] = status
	}
}

func addMetadataField(fields map[string]any, metadata map[string]any) {
	if metadata != nil {
		fields["metadata"] = metadata
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

	// Description is an optional label for the address (e.g., "Home", "Office", "Billing")
	Description *string `json:"description,omitempty"`
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
		Line1:       a.Line1,
		Line2:       a.Line2,
		ZipCode:     a.ZipCode,
		City:        a.City,
		State:       a.State,
		Country:     a.Country,
		Description: a.Description,
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
		Line1:       modelAddress.Line1,
		Line2:       modelAddress.Line2,
		ZipCode:     modelAddress.ZipCode,
		City:        modelAddress.City,
		State:       modelAddress.State,
		Country:     modelAddress.Country,
		Description: modelAddress.Description,
	}
}

// Pagination represents pagination information for list operations.
// This structure is used in list responses to provide context about the pagination state
// and to help with navigating through paginated results.
type Pagination struct {
	// Limit is the number of items per page
	Limit int `json:"limit"`

	// Page is the current page number for page-based endpoints.
	Page int `json:"page,omitempty"`

	// Offset is the starting position for legacy offset-based pagination.
	Offset int `json:"offset,omitempty"`

	// Total is the total number of items available across all pages when provided.
	Total int `json:"total,omitempty"`

	// PrevCursor is the cursor for the previous page (for cursor-based pagination)
	PrevCursor string `json:"prev_cursor,omitempty"`

	// NextCursor is the cursor for the next page (for cursor-based pagination)
	NextCursor string `json:"next_cursor,omitempty"`

	// ItemCount tracks the number of decoded items for cursor/page heuristics.
	ItemCount int `json:"-"`
}

// UnmarshalJSON supports both current snake_case and legacy camelCase cursor keys.
func (p *Pagination) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.New("pagination receiver cannot be nil")
	}

	type alias Pagination

	aux := struct {
		alias
		PrevCursorLegacy string `json:"prevCursor,omitempty"`
		NextCursorLegacy string `json:"nextCursor,omitempty"`
	}{}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*p = Pagination(aux.alias)
	if p.PrevCursor == "" {
		p.PrevCursor = aux.PrevCursorLegacy
	}

	if p.NextCursor == "" {
		p.NextCursor = aux.NextCursorLegacy
	}

	return nil
}

// HasMorePages returns true if there are more pages available.
// This is determined by checking if the offset plus limit is less than the total.
//
// Returns:
//   - true if there are more pages available, false otherwise
func (p *Pagination) HasMorePages() bool {
	if p == nil {
		return false
	}

	if p.Total > 0 && p.Limit > 0 {
		if p.Page > 0 {
			return p.Page*p.Limit < p.Total
		}

		return p.Offset+p.Limit < p.Total
	}

	return p.Page > 0 && p.Limit > 0 && p.ItemCount >= p.Limit
}

// HasPrevPage returns true if there is a previous page available.
// This is determined by checking if the offset is greater than 0 or if a previous cursor is available.
//
// Returns:
//   - true if there is a previous page available, false otherwise
func (p *Pagination) HasPrevPage() bool {
	if p == nil {
		return false
	}

	return p.Page > 1 || p.Offset > 0 || p.PrevCursor != ""
}

// HasNextPage returns true if there is a next page available.
// This is determined by checking if there are more pages or if a next cursor is available.
//
// Returns:
//   - true if there is a next page available, false otherwise
func (p *Pagination) HasNextPage() bool {
	if p == nil {
		return false
	}

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
	if p == nil {
		return nil
	}

	if !p.HasNextPage() {
		return nil
	}

	options := NewListOptions().WithLimit(p.Limit)

	// Prefer cursor-based pagination if available
	if p.NextCursor != "" {
		return options.WithCursor(p.NextCursor)
	}

	if p.Page > 0 {
		options.Page = p.Page + 1
		options.Offset = DefaultOffset

		return options
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
	if p == nil {
		return nil
	}

	if !p.HasPrevPage() {
		return nil
	}

	options := NewListOptions().WithLimit(p.Limit)

	// Prefer cursor-based pagination if available
	if p.PrevCursor != "" {
		return options.WithCursor(p.PrevCursor)
	}

	if p.Page > 1 {
		options.Page = p.Page - 1
		options.Offset = DefaultOffset

		return options
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
	if p == nil {
		return DefaultPage
	}

	if p.Page > 0 {
		return p.Page
	}

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
	if p == nil {
		return 1
	}

	if p.Limit <= 0 || p.Total <= 0 {
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

	// Offset is a legacy local compatibility field. Midaz APIs do not accept
	// offset on the wire; aligned offsets may be converted to page numbers.
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

func ensureListOptions(options *ListOptions) *ListOptions {
	if options == nil {
		return NewListOptions()
	}

	return options
}

func cloneAnyMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}

	clone := make(map[string]any, len(values))
	for key, value := range values {
		clone[key] = value
	}

	return clone
}

// Clone returns a deep copy of the list options.
func (o *ListOptions) Clone() *ListOptions {
	if o == nil {
		return NewListOptions()
	}

	clone := *o
	clone.Filters = maps.Clone(o.Filters)
	clone.AdditionalParams = maps.Clone(o.AdditionalParams)

	return &clone
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
	o = ensureListOptions(o)

	if limit <= 0 {
		o.Limit = DefaultLimit
	} else if limit > MaxLimit {
		o.Limit = MaxLimit
	} else {
		o.Limit = limit
	}

	return o
}

// WithOffset sets the legacy starting position for pagination.
// Midaz APIs do not accept offset on the wire; ToQueryParams only converts
// aligned offsets to deterministic page values and never emits offset.
//
// Parameters:
//   - offset: The starting position (must be >= 0)
//
// Returns:
//   - The modified ListOptions instance for method chaining
func (o *ListOptions) WithOffset(offset int) *ListOptions {
	o = ensureListOptions(o)

	if offset < 0 {
		o.Offset = DefaultOffset
	} else {
		o.Offset = offset
	}

	return o
}

// WithPage sets the page number for Midaz page-based pagination.
// Page-based pagination is the supported wire format for current list endpoints.
//
// Parameters:
//   - page: The page number (must be >= 1)
//
// Returns:
//   - The modified ListOptions instance for method chaining
func (o *ListOptions) WithPage(page int) *ListOptions {
	o = ensureListOptions(o)

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
//
//nolint:wsl_v5
func (o *ListOptions) WithCursor(cursor string) *ListOptions {
	o = ensureListOptions(o)

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
//
//nolint:wsl_v5
func (o *ListOptions) WithOrderBy(field string) *ListOptions {
	o = ensureListOptions(o)

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
//
//nolint:wsl_v5
func (o *ListOptions) WithOrderDirection(direction SortDirection) *ListOptions {
	o = ensureListOptions(o)

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
	o = ensureListOptions(o)

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
//
//nolint:wsl_v5
func (o *ListOptions) WithFilters(filters map[string]string) *ListOptions {
	o = ensureListOptions(o)
	o.Filters = maps.Clone(filters)
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
	o = ensureListOptions(o)

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
	o = ensureListOptions(o)

	if o.AdditionalParams == nil {
		o.AdditionalParams = make(map[string]string)
	}

	o.AdditionalParams[key] = value

	return o
}

// WithIncludeDeleted adds the CRM include_deleted filter.
func (o *ListOptions) WithIncludeDeleted(includeDeleted bool) *ListOptions {
	return o.WithAdditionalParam("include_deleted", strconv.FormatBool(includeDeleted))
}

// WithHolderID adds the CRM holder_id filter.
func (o *ListOptions) WithHolderID(holderID string) *ListOptions {
	return o.WithAdditionalParam("holder_id", holderID)
}

// WithExternalID adds the CRM external_id filter.
func (o *ListOptions) WithExternalID(externalID string) *ListOptions {
	return o.WithAdditionalParam("external_id", externalID)
}

// WithDocument adds the CRM document filter.
func (o *ListOptions) WithDocument(document string) *ListOptions {
	return o.WithAdditionalParam("document", document)
}

// WithAccountID adds the CRM account_id filter.
func (o *ListOptions) WithAccountID(accountID string) *ListOptions {
	return o.WithAdditionalParam("account_id", accountID)
}

// WithPortfolioID adds the onboarding portfolio_id account filter.
func (o *ListOptions) WithPortfolioID(portfolioID string) *ListOptions {
	return o.WithAdditionalParam("portfolio_id", portfolioID)
}

// WithSegmentID adds the onboarding segment_id account filter.
func (o *ListOptions) WithSegmentID(segmentID string) *ListOptions {
	return o.WithAdditionalParam("segment_id", segmentID)
}

// WithStatusFilter adds the onboarding status filter.
func (o *ListOptions) WithStatusFilter(status string) *ListOptions {
	return o.WithAdditionalParam("status", status)
}

// WithTypeFilter adds the onboarding type account filter.
func (o *ListOptions) WithTypeFilter(accountType string) *ListOptions {
	return o.WithAdditionalParam("type", accountType)
}

// WithAssetCode adds the onboarding asset_code account filter.
func (o *ListOptions) WithAssetCode(assetCode string) *ListOptions {
	return o.WithAdditionalParam("asset_code", assetCode)
}

// WithEntityID adds the onboarding entity_id account filter.
func (o *ListOptions) WithEntityID(entityID string) *ListOptions {
	return o.WithAdditionalParam("entity_id", entityID)
}

// WithBlocked adds the onboarding blocked account filter.
func (o *ListOptions) WithBlocked(blocked bool) *ListOptions {
	return o.WithAdditionalParam("blocked", strconv.FormatBool(blocked))
}

// WithParentAccountID adds the onboarding parent_account_id account filter.
func (o *ListOptions) WithParentAccountID(parentAccountID string) *ListOptions {
	return o.WithAdditionalParam("parent_account_id", parentAccountID)
}

// WithNameFilter adds the onboarding name filter.
func (o *ListOptions) WithNameFilter(name string) *ListOptions {
	return o.WithAdditionalParam("name", name)
}

// WithAlias adds the onboarding alias account filter.
func (o *ListOptions) WithAlias(alias string) *ListOptions {
	return o.WithAdditionalParam("alias", alias)
}

// WithLedgerID adds the CRM ledger_id filter.
func (o *ListOptions) WithLedgerID(ledgerID string) *ListOptions {
	return o.WithAdditionalParam("ledger_id", ledgerID)
}

// WithParticipantDocument adds the CRM regulatory_fields_participant_document filter.
func (o *ListOptions) WithParticipantDocument(document string) *ListOptions {
	return o.WithAdditionalParam("regulatory_fields_participant_document", document)
}

// WithRelatedPartyDocument adds the CRM related_party_document filter.
func (o *ListOptions) WithRelatedPartyDocument(document string) *ListOptions {
	return o.WithAdditionalParam("related_party_document", document)
}

// WithBankingDetailsBranch adds the CRM banking_details_branch alias filter.
func (o *ListOptions) WithBankingDetailsBranch(branch string) *ListOptions {
	return o.WithAdditionalParam("banking_details_branch", branch)
}

// WithBankingDetailsAccount adds the CRM banking_details_account alias filter.
func (o *ListOptions) WithBankingDetailsAccount(account string) *ListOptions {
	return o.WithAdditionalParam("banking_details_account", account)
}

// WithBankingDetailsIBAN adds the CRM banking_details_iban alias filter.
func (o *ListOptions) WithBankingDetailsIBAN(iban string) *ListOptions {
	return o.WithAdditionalParam("banking_details_iban", iban)
}

// WithRelatedPartyRole adds the CRM related_party_role alias filter.
func (o *ListOptions) WithRelatedPartyRole(role string) *ListOptions {
	return o.WithAdditionalParam("related_party_role", role)
}

// NextPage returns a copy of the ListOptions configured for the next page.
// This method is useful for implementing pagination in client code.
//
// Returns:
//   - A new ListOptions instance configured for the next page
func (o *ListOptions) NextPage() *ListOptions {
	o = ensureListOptions(o)
	next := o.Clone()

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

	return next
}

// NextPageOptionsFrom returns next-page options while preserving all state from current.
//
//nolint:wsl_v5
func NextPageOptionsFrom(current *ListOptions, pagination *Pagination) *ListOptions {
	if pagination == nil || !pagination.HasNextPage() {
		return nil
	}

	next := ensureListOptions(current).Clone()
	limit := pagination.Limit
	if limit <= 0 {
		limit = next.Limit
	}
	if limit <= 0 {
		limit = DefaultLimit
	}
	next.WithLimit(limit)

	if pagination.NextCursor != "" {
		next.Page = 0
		next.Cursor = pagination.NextCursor
		next.Offset = DefaultOffset
		return next
	}

	if pagination.Page > 0 {
		next.Page = pagination.Page + 1
		next.Cursor = ""
		next.Offset = DefaultOffset
		return next
	}

	next.Page = 0
	next.Offset = pagination.Offset + limit
	next.Cursor = ""

	return next
}

// ToQueryParams converts ListOptions to a map of query parameters.
// This method transforms the ListOptions structure into a format suitable
// for use as URL query parameters in API requests.
//
// Returns:
//   - A map of string key-value pairs representing the query parameters
func (o *ListOptions) ToQueryParams() map[string]string {
	o = ensureListOptions(o)

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

// Validate enforces SDK-side preconditions on the ListOptions shape.
//
// The most important rule enforced here: a non-zero Offset must be a
// multiple of Limit. Midaz only speaks page-based pagination on the wire,
// and the SDK converts an aligned offset (e.g. 50 with limit 25 → page 3)
// for backward compatibility. An UNALIGNED offset cannot be expressed as
// a page number, and the previous addPaginationParams silently dropped
// it — leaving callers stuck on page 1 wondering why their offset
// "didn't work". Validate surfaces the mismatch as an error so consumers
// can fix it instead of debugging missing rows.
//
// Validate is safe to call on a nil or zero-value ListOptions.
func (o *ListOptions) Validate() error {
	if o == nil {
		return nil
	}

	limit := o.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}

	if o.Offset > 0 && o.Offset%limit != 0 {
		return fmt.Errorf("offset (%d) must be a multiple of limit (%d) — Midaz pagination only supports page boundaries", o.Offset, limit)
	}

	return nil
}

// addPaginationParams adds pagination-related parameters to the query parameters map.
// This is an internal helper method used by ToQueryParams.
//
// Parameters:
//   - params: The map to add the pagination parameters to
func (o *ListOptions) addPaginationParams(params map[string]string) {
	limit := o.Limit

	// Always include limit parameter with at least the default
	if o.Limit <= 0 {
		limit = DefaultLimit
		params[QueryParamLimit] = fmt.Sprintf("%d", DefaultLimit)
	} else if o.Limit > MaxLimit {
		limit = MaxLimit
		params[QueryParamLimit] = fmt.Sprintf("%d", MaxLimit)
	} else {
		params[QueryParamLimit] = fmt.Sprintf("%d", o.Limit)
	}

	if o.Page > 0 {
		params[QueryParamPage] = fmt.Sprintf("%d", o.Page)
		return
	}

	if o.Offset > 0 && o.Offset%limit == 0 {
		params[QueryParamPage] = fmt.Sprintf("%d", (o.Offset/limit)+1)
	}
	// Unaligned offsets are silently dropped here. v3 contract: callers
	// should run ListOptions.Validate() before list calls. Track 5 of the
	// v3-dx plan replaces this whole shape with per-service typed
	// ListOpts so the misuse becomes uncompilable.

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
	// Midaz list endpoints expose sort direction, but not a generic order-by field.
	// Keep OrderBy as an SDK-side compatibility field without serializing it.
	switch o.OrderDirection {
	case "":
		params[QueryParamOrderDirection] = DefaultSortDirection
	case string(SortAscending), string(SortDescending):
		params[QueryParamOrderDirection] = o.OrderDirection
	default:
		params[QueryParamOrderDirection] = DefaultSortDirection
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
			if v == "" {
				continue
			}

			if isReservedListQueryParam(k) {
				continue
			}

			params[k] = v
		}
	}
}

func isReservedListQueryParam(key string) bool {
	switch key {
	case QueryParamLimit, QueryParamPage, QueryParamOffset, QueryParamCursor, QueryParamOrderBy, QueryParamOrderDirection, QueryParamStartDate, QueryParamEndDate:
		return true
	default:
		return false
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

// MarshalJSON ensures zero-value list responses encode items as an empty array.
//
//nolint:wsl_v5
func (r ListResponse[T]) MarshalJSON() ([]byte, error) {
	type alias ListResponse[T]
	response := alias(r)
	if response.Items == nil {
		response.Items = make([]T, 0)
	}

	return json.Marshal(response)
}

// UnmarshalJSON supports both the legacy nested pagination envelope and the
// current Midaz top-level pagination fields.
func (r *ListResponse[T]) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.New("list response receiver cannot be nil")
	}

	type alias ListResponse[T]

	aux := struct {
		alias
		Limit              int         `json:"limit,omitempty"`
		Page               int         `json:"page,omitempty"`
		Offset             int         `json:"offset,omitempty"`
		Total              int         `json:"total,omitempty"`
		NextCursor         string      `json:"next_cursor,omitempty"`
		PrevCursor         string      `json:"prev_cursor,omitempty"`
		NextCursorLegacy   string      `json:"nextCursor,omitempty"`
		PrevCursorLegacy   string      `json:"prevCursor,omitempty"`
		Pagination         *Pagination `json:"pagination,omitempty"`
		HTTPPagination     *Pagination `json:"http.Pagination,omitempty"`
		HTTPPaginationFlat *Pagination `json:"httpPagination,omitempty"`
	}{}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*r = ListResponse[T](aux.alias)
	if r.Items == nil {
		r.Items = make([]T, 0)
	}

	r.Pagination = firstPagination(aux.Pagination, aux.HTTPPagination, aux.HTTPPaginationFlat)
	if isEmptyPagination(r.Pagination) {
		r.Pagination = topLevelPagination(aux.Limit, aux.Page, aux.Offset, aux.Total, aux.NextCursor, aux.NextCursorLegacy, aux.PrevCursor, aux.PrevCursorLegacy)
	}

	r.Pagination.ItemCount = len(r.Items)

	return nil
}

func firstPagination(paginations ...*Pagination) Pagination {
	for _, pagination := range paginations {
		if pagination != nil {
			return *pagination
		}
	}

	return Pagination{}
}

func topLevelPagination(limit, page, offset, total int, nextCursor, nextCursorLegacy, prevCursor, prevCursorLegacy string) Pagination {
	if nextCursor == "" {
		nextCursor = nextCursorLegacy
	}

	if prevCursor == "" {
		prevCursor = prevCursorLegacy
	}

	return Pagination{Limit: limit, Page: page, Offset: offset, Total: total, NextCursor: nextCursor, PrevCursor: prevCursor}
}

func isEmptyPagination(p Pagination) bool {
	return p.Limit == 0 && p.Page == 0 && p.Offset == 0 && p.Total == 0 && p.NextCursor == "" && p.PrevCursor == ""
}

// ErrorResponse represents an error response from the API.
// This structure is used to parse and represent error responses
// returned by the Midaz API.
type ErrorResponse struct {
	// Error is the error message
	Error string `json:"error,omitempty"`

	// Code is the error code for programmatic handling
	Code string `json:"code,omitempty"`

	// Title is the short human-readable API error title.
	Title string `json:"title,omitempty"`

	// Message is the detailed Midaz API error message.
	Message string `json:"message,omitempty"`

	// EntityType identifies the resource type associated with the error.
	EntityType string `json:"entityType,omitempty"`

	// Fields contains field-level validation errors returned by Midaz.
	Fields map[string]string `json:"fields,omitempty"`

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
	if o == nil {
		return false
	}

	return len(o.Metadata) > 0
}

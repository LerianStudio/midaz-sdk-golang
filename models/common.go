package models

import (
	"encoding/json"
	"errors"
	"time"
)

// Status represents the status of an entity in the Midaz system.
//
// Status is hand-written and SDK-owned (audit 7.1, 7.2 — Track 7E).
// The fields and JSON tags MUST stay aligned with the Midaz wire format
// (mmodel.Status today). The SDK gains the freedom to evolve its public
// surface independently of mmodel without forcing a server-package
// import on every caller of the SDK.
//
// Wire format alignment:
//
//	{"code": "ACTIVE", "description": "Active status"}
type Status struct {
	// Code is the canonical status code (e.g. "ACTIVE", "INACTIVE", "PENDING").
	Code string `json:"code" validate:"max=100" example:"ACTIVE" maxLength:"100" enum:"ACTIVE,INACTIVE,PENDING,SUSPENDED,DELETED"`

	// Description is an optional human-readable description of the status.
	Description *string `json:"description" validate:"omitempty,max=256" example:"Active status" maxLength:"256"`
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

// StatusHelpers provides utility functions for working with Status.
// Since Status is hand-written and SDK-owned (Track 7E), we provide helper
// functions instead of methods that would require a separate Status receiver
// type.

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

// IsEmpty reports whether all fields of the address are empty.
//
// Used by Organization marshal/validate to decide whether to emit the
// `address` JSON key. Mirrors mmodel.Address.IsEmpty semantics.
func (a Address) IsEmpty() bool {
	return a.Line1 == "" &&
		a.Line2 == nil &&
		a.ZipCode == "" &&
		a.City == "" &&
		a.State == "" &&
		a.Country == "" &&
		a.Description == nil
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

// NOTE: ToMmodelAddress / FromMmodelAddress were retired in Track 7E.
// Address is now SDK-owned with the same JSON tags as mmodel.Address, so
// the wire format is identical and the conversions are unnecessary.
// Internal callers that historically reached for these adapters should
// pass / receive models.Address directly.

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

// HasMore reports whether more results are available after the current
// page. It is the primary signal callers should use to decide whether to
// fetch another page.
//
// Decision logic, in priority order:
//
//  1. NextCursor present → cursor pagination has more (definitive).
//  2. Total + Limit known → arithmetic determines whether more pages
//     remain (page-based with known total).
//  3. ItemCount equals Limit → page is full, server probably has more
//     (heuristic for endpoints that don't report Total).
//  4. Otherwise → no more.
//
// Nil-receiver-safe: returns false on a nil *Pagination.
func (p *Pagination) HasMore() bool {
	if p == nil {
		return false
	}

	if p.NextCursor != "" {
		return true
	}

	if p.Total > 0 && p.Limit > 0 {
		if p.Page > 0 {
			return p.Page*p.Limit < p.Total
		}

		return p.Offset+p.Limit < p.Total
	}

	return p.Page > 0 && p.Limit > 0 && p.ItemCount >= p.Limit
}

// HasPrev reports whether a page exists before the current one.
//
// True when any of:
//   - a server-issued PrevCursor is present (cursor pagination)
//   - Page > 1 (page-based pagination, not on first page)
//   - Offset > 0 (legacy offset pagination, not at start)
//
// Nil-receiver-safe: returns false on a nil *Pagination.
func (p *Pagination) HasPrev() bool {
	if p == nil {
		return false
	}

	return p.Page > 1 || p.Offset > 0 || p.PrevCursor != ""
}

// TotalKnown reports whether the server populated Pagination.Total with
// a non-zero value. Use this to decide whether arithmetic on Total is
// meaningful — Midaz cursor endpoints generally omit Total, leaving the
// field zero.
//
// Replaces the v2 TotalPages() method, which silently returned 1 when
// Total was unknown and produced misleading "Page N of 1" UIs. Callers
// who want a page count should compute it explicitly:
//
//	if p.TotalKnown() && p.Limit > 0 {
//	    totalPages := (p.Total + p.Limit - 1) / p.Limit
//	}
//
// Nil-receiver-safe: returns false on a nil *Pagination.
func (p *Pagination) TotalKnown() bool {
	if p == nil {
		return false
	}

	return p.Total > 0
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

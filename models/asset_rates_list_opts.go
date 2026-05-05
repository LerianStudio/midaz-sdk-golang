// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// AssetRatesListOpts is the typed options struct for
// ListAssetRatesByAssetCode and the ListAll/ListPages iterators.
//
// AssetRatesListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
//
// The asset-rates endpoint is cursor-paginated: callers iterate by
// passing back the NextCursor surfaced in the previous response's
// Pagination shape. Page-based fields are intentionally absent — the
// endpoint does not honor them on the wire.
//
// Use Validate before calling the entity method to surface limit-cap
// violations as typed errors instead of having them silently capped
// (the v2 footgun, audit finding 5.7).
type AssetRatesListOpts struct {
	// Limit is the maximum number of items per page. Zero falls back
	// to the server default (currently 10). Values above MaxLimit
	// (currently 100) cause Validate to return a validation error.
	Limit int

	// Cursor is the server-issued opaque pagination token. Empty
	// string means "first page". Read NextCursor from the previous
	// response's Pagination to fetch the next page.
	Cursor string

	// SortDirection orders results by createdAt. Empty string falls
	// back to the server default (descending). Use SortAscending or
	// SortDescending; other values are rejected by Validate.
	SortDirection SortDirection

	// Filters narrows the result set. Zero value means no narrowing.
	Filters AssetRatesFilters
}

// AssetRatesFilters carries the filter fields valid for the
// ListAssetRatesByAssetCode endpoint. Narrower than the v2 mega-struct
// ListOptions, which exposed 30+ filter setters regardless of which
// one the endpoint actually honored (audit finding 5.12).
type AssetRatesFilters struct {
	// To restricts results to rates whose target asset matches one
	// of the supplied codes. Empty slice means "no restriction".
	// The wire encoding is a single comma-separated query parameter.
	To []string

	// StartDate filters rates created on or after this date in
	// YYYY-MM-DD format. Empty string means "no lower bound".
	StartDate string

	// EndDate filters rates created on or before this date in
	// YYYY-MM-DD format. Empty string means "no upper bound".
	EndDate string
}

// Validate enforces SDK-side preconditions on AssetRatesListOpts.
//
// Returns a typed validation error when:
//   - Limit is negative
//   - Limit exceeds MaxLimit
//   - SortDirection is non-empty and not one of the recognized values
//
// Validate is safe to call on a zero-value AssetRatesListOpts; the
// entity method calls it automatically before issuing the HTTP request.
//
// This replaces the v2 silent-cap behavior, where a Limit of 5000
// was rounded down to 100 with no error returned (audit finding 5.7).
func (o AssetRatesListOpts) Validate() error {
	const operation = "AssetRatesListOpts.Validate"

	if o.Limit < 0 {
		return errors.NewValidationError(operation,
			"limit must be non-negative",
			fmt.Errorf("got %d", o.Limit))
	}

	if o.Limit > MaxLimit {
		return errors.NewValidationError(operation,
			"limit exceeds maximum",
			fmt.Errorf("got %d, max %d", o.Limit, MaxLimit))
	}

	switch o.SortDirection {
	case "", SortAscending, SortDescending:
	default:
		return errors.NewValidationError(operation,
			"sort direction must be empty, asc, or desc",
			fmt.Errorf("got %q", o.SortDirection))
	}

	return nil
}

// ToQueryParams renders an AssetRatesListOpts into the wire query
// parameter map consumed by the asset-rates endpoint.
//
// Empty/zero fields are omitted so the server applies its own
// defaults rather than the SDK forcing a value.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o AssetRatesListOpts) ToQueryParams() map[string]string {
	params := make(map[string]string)

	if len(o.Filters.To) > 0 {
		params["to"] = strings.Join(o.Filters.To, ",")
	}

	if o.Limit > 0 {
		params["limit"] = strconv.Itoa(o.Limit)
	}

	if o.Filters.StartDate != "" {
		params["start_date"] = o.Filters.StartDate
	}

	if o.Filters.EndDate != "" {
		params["end_date"] = o.Filters.EndDate
	}

	if o.SortDirection != "" {
		params["sort_order"] = string(o.SortDirection)
	}

	if o.Cursor != "" {
		params["cursor"] = o.Cursor
	}

	return params
}

// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"strings"
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
// Embeds CursorListOpts for the shared cursor/sort/date-range fields,
// matching the shape of TransactionsListOpts, OperationsListOpts,
// OperationRoutesListOpts, and TransactionRoutesListOpts. The asset-rates
// specific filter (To) lives in AssetRatesFilters.
//
// Use Validate before calling the entity method to surface limit-cap
// violations and malformed/inverted date ranges as typed errors instead
// of having them silently capped (the v2 footgun, audit finding 5.7).
type AssetRatesListOpts struct {
	CursorListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters AssetRatesFilters
}

// AssetRatesFilters carries the filter fields valid for the
// ListAssetRatesByAssetCode endpoint. Narrower than the v2 mega-struct
// ListOptions, which exposed 30+ filter setters regardless of which
// one the endpoint actually honored (audit finding 5.12).
//
// Date-range filtering (StartDate, EndDate) moved to the embedded
// CursorListOpts to match the shape of every other cursor-based opts.
// The wire still encodes them as start_date/end_date — only the SDK
// shape changed.
type AssetRatesFilters struct {
	// To restricts results to rates whose target asset matches one
	// of the supplied codes. Empty slice means "no restriction".
	// The wire encoding is a single comma-separated query parameter.
	To []string
}

// Validate enforces SDK-side preconditions on AssetRatesListOpts.
//
// Returns a typed validation error when:
//   - Limit is negative
//   - Limit exceeds MaxLimit
//   - SortDirection is non-empty and not one of the recognized values
//   - StartDate or EndDate is non-empty and not a valid YYYY-MM-DD date
//   - StartDate is after EndDate (when both are set)
//
// Validate is safe to call on a zero-value AssetRatesListOpts; the
// entity method calls it automatically before issuing the HTTP request.
//
// This replaces the v2 silent-cap behavior, where a Limit of 5000
// was rounded down to 100 with no error returned (audit finding 5.7).
func (o AssetRatesListOpts) Validate() error {
	return ValidateCursorListOpts("AssetRatesListOpts.Validate", o.CursorListOpts)
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
	params := CursorQueryParams(o.CursorListOpts)

	if len(o.Filters.To) > 0 {
		params["to"] = strings.Join(o.Filters.To, ",")
	}

	return params
}

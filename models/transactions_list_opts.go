// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Apache-2.0

package models

// TransactionsListOpts is the typed options struct for ListTransactions and
// the ListTransactionsAll / ListTransactionsPages iterators.
//
// Embeds CursorListOpts for the shared cursor/sort/date-range fields;
// attaches a TransactionsFilters sub-struct carrying only the filter
// fields the transactions endpoint actually honors.
//
// TransactionsListOpts is a value type. Concurrent-safe by construction.
//
// IMPORTANT: This is a CURSOR-PAGINATED endpoint. The struct does NOT
// expose Page or Offset fields. Iterate by passing back the NextCursor
// from the previous response's Pagination. v3 compile-time prevents
// the v2 footgun where setting WithPage on a cursor endpoint silently
// dropped the value and emitted a stderr warning (audit finding 5.5).
type TransactionsListOpts struct {
	CursorListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters TransactionsFilters
}

// TransactionsFilters is the typed filter set for the transactions endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type TransactionsFilters struct {
	// AssetCode narrows by asset code (e.g. "USD").
	AssetCode string

	// Status narrows by transaction status (e.g. "COMPLETED").
	Status string

	// Reference narrows by external transaction reference.
	Reference string

	// DestinationAccount narrows to transactions targeting a specific account.
	DestinationAccount string

	// SourceAccount narrows to transactions originating from a specific account.
	SourceAccount string

	// Route narrows by transaction route name (e.g. "cashin", "cashout").
	// Honored on both List and the metrics-count endpoint.
	Route string
}

// Validate enforces SDK-side preconditions on TransactionsListOpts.
func (o TransactionsListOpts) Validate() error {
	return ValidateCursorListOpts("TransactionsListOpts.Validate", o.CursorListOpts)
}

// ToQueryParams renders an TransactionsListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o TransactionsListOpts) ToQueryParams() map[string]string {
	params := CursorQueryParams(o.CursorListOpts)

	if o.Filters.AssetCode != "" {
		params["asset_code"] = o.Filters.AssetCode
	}

	if o.Filters.Status != "" {
		params["status"] = o.Filters.Status
	}

	if o.Filters.Reference != "" {
		params["reference"] = o.Filters.Reference
	}

	if o.Filters.DestinationAccount != "" {
		params["destination_account"] = o.Filters.DestinationAccount
	}

	if o.Filters.SourceAccount != "" {
		params["source_account"] = o.Filters.SourceAccount
	}

	if o.Filters.Route != "" {
		params["route"] = o.Filters.Route
	}

	return params
}

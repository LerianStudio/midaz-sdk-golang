// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Apache-2.0

package models

// OperationsListOpts is the typed options struct for ListOperations and
// the ListOperationsAll / ListOperationsPages iterators.
//
// Embeds CursorListOpts for the shared cursor/sort/date-range fields;
// attaches a OperationsFilters sub-struct carrying only the filter
// fields the operations endpoint actually honors.
//
// OperationsListOpts is a value type. Concurrent-safe by construction.
//
// IMPORTANT: This is a CURSOR-PAGINATED endpoint. The struct does NOT
// expose Page or Offset fields. Iterate by passing back the NextCursor
// from the previous response's Pagination. v3 compile-time prevents
// the v2 footgun where setting WithPage on a cursor endpoint silently
// dropped the value and emitted a stderr warning (audit finding 5.5).
type OperationsListOpts struct {
	CursorListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters OperationsFilters
}

// OperationsFilters is the typed filter set for the operations endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type OperationsFilters struct {
	// Type narrows by operation type (e.g. "DEBIT", "CREDIT").
	Type string

	// Status narrows by operation status.
	Status string

	// AssetCode narrows by asset code.
	AssetCode string
}

// Validate enforces SDK-side preconditions on OperationsListOpts.
func (o OperationsListOpts) Validate() error {
	return ValidateCursorListOpts("OperationsListOpts.Validate", o.CursorListOpts)
}

// ToQueryParams renders an OperationsListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o OperationsListOpts) ToQueryParams() map[string]string {
	params := CursorQueryParams(o.CursorListOpts)

	if o.Filters.Type != "" {
		params["type"] = o.Filters.Type
	}

	if o.Filters.Status != "" {
		params["status"] = o.Filters.Status
	}

	if o.Filters.AssetCode != "" {
		params["asset_code"] = o.Filters.AssetCode
	}

	return params
}

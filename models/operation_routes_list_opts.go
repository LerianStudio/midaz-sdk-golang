// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// OperationRoutesListOpts is the typed options struct for ListOperationRoutes and
// the ListOperationRoutesAll / ListOperationRoutesPages iterators.
//
// Embeds CursorListOpts for the shared cursor/sort/date-range fields;
// attaches a OperationRoutesFilters sub-struct carrying only the filter
// fields the operation-routes endpoint actually honors.
//
// OperationRoutesListOpts is a value type. Concurrent-safe by construction.
//
// IMPORTANT: This is a CURSOR-PAGINATED endpoint. The struct does NOT
// expose Page or Offset fields. Iterate by passing back the NextCursor
// from the previous response's Pagination. v3 compile-time prevents
// the v2 footgun where setting WithPage on a cursor endpoint silently
// dropped the value and emitted a stderr warning (audit finding 5.5).
type OperationRoutesListOpts struct {
	CursorListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters OperationRoutesFilters
}

// OperationRoutesFilters is the typed filter set for the operation-routes endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type OperationRoutesFilters struct {
	// Name narrows by operation-route name.
	Name string

	// Status narrows by status.
	Status string

	// OperationType narrows by operation type (e.g. "DEBIT", "CREDIT").
	OperationType string
}

// Validate enforces SDK-side preconditions on OperationRoutesListOpts.
func (o OperationRoutesListOpts) Validate() error {
	return ValidateCursorListOpts("OperationRoutesListOpts.Validate", o.CursorListOpts)
}

// ToQueryParams renders an OperationRoutesListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o OperationRoutesListOpts) ToQueryParams() map[string]string {
	params := CursorQueryParams(o.CursorListOpts)

	if o.Filters.Name != "" {
		params["name"] = o.Filters.Name
	}

	if o.Filters.Status != "" {
		params["status"] = o.Filters.Status
	}

	if o.Filters.OperationType != "" {
		params["operation_type"] = o.Filters.OperationType
	}

	return params
}

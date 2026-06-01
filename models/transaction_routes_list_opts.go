// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// TransactionRoutesListOpts is the typed options struct for ListTransactionRoutes and
// the ListTransactionRoutesAll / ListTransactionRoutesPages iterators.
//
// Embeds CursorListOpts for the shared cursor/sort/date-range fields;
// attaches a TransactionRoutesFilters sub-struct carrying only the filter
// fields the transaction-routes endpoint actually honors.
//
// TransactionRoutesListOpts is a value type. Concurrent-safe by construction.
//
// IMPORTANT: This is a CURSOR-PAGINATED endpoint. The struct does NOT
// expose Page or Offset fields. Iterate by passing back the NextCursor
// from the previous response's Pagination. v3 compile-time prevents
// the v2 footgun where setting WithPage on a cursor endpoint silently
// dropped the value and emitted a stderr warning (audit finding 5.5).
type TransactionRoutesListOpts struct {
	CursorListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters TransactionRoutesFilters
}

// TransactionRoutesFilters is the typed filter set for the transaction-routes endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type TransactionRoutesFilters struct {
	// Name narrows by transaction-route name.
	Name string

	// Status narrows by status.
	Status string

	// OperationRouteID narrows by linked operation-route ID.
	OperationRouteID string
}

// Validate enforces SDK-side preconditions on TransactionRoutesListOpts.
func (o TransactionRoutesListOpts) Validate() error {
	return ValidateCursorListOpts("TransactionRoutesListOpts.Validate", o.CursorListOpts)
}

// ToQueryParams renders an TransactionRoutesListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o TransactionRoutesListOpts) ToQueryParams() map[string]string {
	params := CursorQueryParams(o.CursorListOpts)

	if o.Filters.Name != "" {
		params["name"] = o.Filters.Name
	}

	if o.Filters.Status != "" {
		params["status"] = o.Filters.Status
	}

	if o.Filters.OperationRouteID != "" {
		params["operation_route_id"] = o.Filters.OperationRouteID
	}

	return params
}

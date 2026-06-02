// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"fmt"
	"strconv"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

// CursorListOpts is the shared base struct for cursor-based list
// endpoints (transactions, operations, operation_routes,
// transaction_routes). Per-entity ListOpts (e.g.
// TransactionsListOpts, OperationsListOpts) embed CursorListOpts to
// inherit the cursor pagination + sort + date-range fields, then
// attach a typed Filters sub-struct carrying only the fields that
// endpoint actually honors.
//
// CursorListOpts is a value type. Concurrent-safe by construction.
//
// # Why this is separate from PageListOpts
//
// Cursor endpoints DO NOT honor Page or Offset on the wire. Putting
// Page/Offset on a cursor endpoint's typed Opts would let callers
// write code that compiles but silently no-ops (the v2 footgun
// audit finding 5.5 — v2 wrote a stderr warning when this happened).
// v3 makes the wrong shape uncompilable: cursor opts have Cursor,
// page opts have Page, and the type system rejects mixing them.
//
// PageListOpts and CursorListOpts intentionally share NO fields by
// composition. They share SortDirection, StartDate, EndDate only
// because every list endpoint accepts those — but they don't inherit
// from a common base.
type CursorListOpts struct {
	// Limit is the maximum number of items per page. Zero falls
	// back to the server default. Values above MaxLimit cause
	// the entity-level Validate to return a typed validation error.
	Limit int

	// Cursor is the server-issued opaque pagination token. Empty
	// string means "first page". Read NextCursor from the previous
	// response's Pagination to fetch the next page.
	Cursor string

	// SortDirection orders results by createdAt. Empty string falls
	// back to the server default. Validate rejects any value other
	// than "", asc, or desc.
	SortDirection SortDirection

	// StartDate filters results created on or after this date in
	// YYYY-MM-DD format. Empty string means "no lower bound".
	StartDate string

	// EndDate filters results created on or before this date in
	// YYYY-MM-DD format. Empty string means "no upper bound".
	EndDate string
}

// ValidateCursorListOpts enforces SDK-side preconditions shared by
// every cursor-based list endpoint. Per-entity Validate methods call
// this first, then layer in their own filter-specific checks. Returns
// a typed *errors.Error of category validation on failure.
//
// Exported because per-entity ListOpts in package entities call it.
// Treat as an internal contract; not part of the user-facing API.
func ValidateCursorListOpts(operation string, o CursorListOpts) error {
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

	return validateDateRange(operation, o.StartDate, o.EndDate)
}

// CursorQueryParams renders the shared cursor/sort/date-range fields
// of CursorListOpts into the wire query parameter map. Per-entity
// ToQueryParams methods call this first, then layer in their own
// filter-specific keys.
//
// Empty/zero fields are omitted so the server applies its own
// defaults rather than the SDK forcing a value. This matters for
// limit (server default differs from SDK default) and sort_order
// (server may pick a non-descending default for some endpoints).
//
// Exported because per-entity ListOpts in package entities call it.
// Treat as an internal contract; not part of the user-facing API.
func CursorQueryParams(o CursorListOpts) map[string]string {
	params := make(map[string]string)

	if o.Limit > 0 {
		params[QueryParamLimit] = strconv.Itoa(o.Limit)
	}

	if o.Cursor != "" {
		params[QueryParamCursor] = o.Cursor
	}

	if o.SortDirection != "" {
		params[QueryParamOrderDirection] = string(o.SortDirection)
	}

	if o.StartDate != "" {
		params[QueryParamStartDate] = o.StartDate
	}

	if o.EndDate != "" {
		params[QueryParamEndDate] = o.EndDate
	}

	return params
}

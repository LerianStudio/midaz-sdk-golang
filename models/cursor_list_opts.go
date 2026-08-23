// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"fmt"
	"strconv"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
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
	if err := validateCursorLimitSort(operation, o); err != nil {
		return err
	}

	return validateDateRange(operation, o.StartDate, o.EndDate)
}

// validateCursorLimitSort enforces the limit-bounds and sort-direction checks
// shared by every cursor-list validator, independent of the date format. The
// date step differs per plane — validateDateRange for the ledger's YYYY-MM-DD,
// validateDateRangeRFC3339 for the tracer's RFC3339 — and is applied by the
// caller after this returns. Extracted so ValidateCursorListOpts and its
// RFC3339 sibling share ONE copy of the non-date checks with identical order,
// messages, and behavior.
func validateCursorLimitSort(operation string, o CursorListOpts) error {
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

// ValidateCursorListOptsRFC3339Dates is the tracer-plane sibling of
// ValidateCursorListOpts. The tracer's cursor list endpoints (validations now,
// audit-events next) strict-parse start_date/end_date as RFC3339 server-side
// (midaz components/tracer/internal/adapters/http/in/transaction_validation_handler.go:355,
// time.Parse(time.RFC3339, input.StartDate) returning a 400 on failure),
// diverging from the ledger plane's YYYY-MM-DD. So it runs the shared limit/sort
// checks and then enforces RFC3339 on the date range, rejecting a YYYY-MM-DD
// value the ledger siblings would accept — catching the wrong format on the SDK
// side instead of shipping it to a server that rejects it.
//
// Exported because per-entity ListOpts in package entities call it. Treat as an
// internal contract; not part of the user-facing API.
func ValidateCursorListOptsRFC3339Dates(operation string, o CursorListOpts) error {
	if err := validateCursorLimitSort(operation, o); err != nil {
		return err
	}

	return validateDateRangeRFC3339(operation, o.StartDate, o.EndDate)
}

// ValidateCursorListOptsNoDates is the sibling of ValidateCursorListOpts for
// cursor endpoints whose generated params carry NO start_date/end_date slot
// (the tracer rules and limits lists). On those endpoints a date filter is a
// silent-wrong-result footgun: a well-formed range PASSES ValidateCursorListOpts
// (its last step, validateDateRange, only rejects malformed/inverted ranges),
// then gets DROPPED at param-mapping time because there is no wire slot to map
// it to — so the server returns the FULL unfiltered set while the caller
// believes the filter took effect. This validator fails loud instead: any
// StartDate/EndDate is rejected up front. With no dates set it defers to
// ValidateCursorListOpts for the shared limit/sort/(vacuous)date checks.
//
// Exported because per-entity ListOpts in package entities call it. Treat as
// an internal contract; not part of the user-facing API.
func ValidateCursorListOptsNoDates(operation string, o CursorListOpts) error {
	if o.StartDate != "" || o.EndDate != "" {
		return errors.NewValidationError(operation,
			"date filtering is not supported by this endpoint",
			fmt.Errorf("startDate=%q endDate=%q", o.StartDate, o.EndDate))
	}

	return ValidateCursorListOpts(operation, o)
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

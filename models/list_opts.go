// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"fmt"
	"strconv"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// PageListOpts is the shared base struct for page-based list endpoints.
// Per-entity ListOpts (e.g. AccountsListOpts, PortfoliosListOpts) embed
// PageListOpts to inherit the standard pagination + sort + date-range
// fields, then attach a typed Filters sub-struct carrying only the
// fields that endpoint actually honors.
//
// PageListOpts is a value type. Sharing across goroutines is safe by
// construction because the entity layer never mutates a caller's opts.
//
// # When to use the embedded base
//
// Every page-based list endpoint accepts the same pagination shape
// (Limit + Page), the same sort shape (SortDirection), and the same
// date-range shape (StartDate + EndDate). Pulling these into a shared
// base eliminates boilerplate across 11 entities while preserving the
// per-entity Filters typing that prevents WithStatus from compiling
// on an endpoint that doesn't honor status.
//
// Cursor-based endpoints (transactions, operations) DO NOT use
// PageListOpts — they use their own typed shape with Cursor instead
// of Page. See models.TransactionsListOpts and models.OperationsListOpts.
type PageListOpts struct {
	// Limit is the maximum number of items per page. Zero falls
	// back to the server default. Values above MaxLimit cause
	// the entity-level Validate to return a typed validation error.
	Limit int

	// Page is the 1-indexed page number. Zero means "first page"
	// (the server treats it as page 1). Used for offset-style
	// pagination supported by every page-based endpoint.
	Page int

	// SortDirection orders results by the server's default sort
	// field. Empty string means "server default direction".
	// Validate rejects any value other than "", asc, or desc.
	// Midaz list endpoints do not expose a generic order-by
	// query parameter — sort field selection lives server-side.
	SortDirection SortDirection

	// StartDate filters results created on or after this date in
	// YYYY-MM-DD format. Empty string means "no lower bound".
	StartDate string

	// EndDate filters results created on or before this date in
	// YYYY-MM-DD format. Empty string means "no upper bound".
	EndDate string
}

// ValidatePageListOpts enforces SDK-side preconditions shared by every
// page-based list endpoint. Per-entity Validate methods call this
// first, then layer in their own filter-specific checks. Returns a
// typed *errors.Error of category validation on failure.
//
// Exported because per-entity ListOpts in package entities call it.
// Treat as an internal contract; not part of the user-facing API.
func ValidatePageListOpts(operation string, o PageListOpts) error {
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

	if o.Page < 0 {
		return errors.NewValidationError(operation,
			"page must be non-negative",
			fmt.Errorf("got %d", o.Page))
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

// PageQueryParams renders the shared pagination/sort/date-range fields
// of PageListOpts into the wire query parameter map. Per-entity
// toQueryParams methods call this first, then layer in their own
// filter-specific keys.
//
// Empty/zero fields are omitted so the server applies its own defaults
// rather than the SDK forcing a value.
//
// Exported because per-entity ListOpts in package entities call it.
// Treat as an internal contract; not part of the user-facing API.
func PageQueryParams(o PageListOpts) map[string]string {
	params := make(map[string]string)

	if o.Limit > 0 {
		params[QueryParamLimit] = strconv.Itoa(o.Limit)
	}

	if o.Page > 0 {
		params[QueryParamPage] = strconv.Itoa(o.Page)
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

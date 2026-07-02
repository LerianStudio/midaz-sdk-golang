// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import "strconv"

// PackagesListOpts is the typed options struct for listing fee packages and
// the All / Pages iterators.
//
// Fee packages are PAGE-paginated (Page + Limit, no cursor): the generated
// GetAllPackagesParams exposes Page/Limit, and the response carries a top-level
// total, so the iterator advances Page++ and stops on !Pagination.HasMore().
//
// Embeds PageListOpts for the shared pagination/sort/date-range fields; attaches
// a PackagesFilters sub-struct for the filters this endpoint honors.
//
// PackagesListOpts is a value type. Concurrent-safe by construction — the entity
// layer never mutates a caller's opts.
type PackagesListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters PackagesFilters
}

// PackagesFilters is the typed filter set for the fee-packages endpoint. Each
// field has a native slot on the generated GetAllPackagesParams.
type PackagesFilters struct {
	// SegmentID narrows by segment UUID.
	SegmentID string

	// LedgerID narrows by ledger UUID.
	LedgerID string

	// TransactionRoute narrows by transaction route.
	TransactionRoute string

	// Enable narrows by the enabled flag. Nil means no narrowing.
	Enable *bool
}

// Validate enforces SDK-side preconditions on PackagesListOpts.
func (o PackagesListOpts) Validate() error {
	return ValidatePageListOpts("PackagesListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders a PackagesListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it. Treat as an
// internal contract; not part of the user-facing API.
func (o PackagesListOpts) ToQueryParams() map[string]string {
	params := PageQueryParams(o.PageListOpts)

	if o.Filters.SegmentID != "" {
		params["segmentId"] = o.Filters.SegmentID
	}

	if o.Filters.LedgerID != "" {
		params["ledgerId"] = o.Filters.LedgerID
	}

	if o.Filters.TransactionRoute != "" {
		params["transactionRoute"] = o.Filters.TransactionRoute
	}

	if o.Filters.Enable != nil {
		params["enable"] = strconv.FormatBool(*o.Filters.Enable)
	}

	return params
}

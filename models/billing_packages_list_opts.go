// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// BillingPackagesListOpts is the typed options struct for listing billing
// packages and the ListAll / ListPages iterators.
//
// Billing packages are PAGE-paginated (Page + Limit, no cursor): the generated
// GetAllBillingPackagesParams exposes Page/Limit, and the response carries a
// top-level total, so the iterator advances Page++ and stops on
// !Pagination.HasMore().
//
// Embeds PageListOpts for the shared pagination/sort/date-range fields; attaches
// a BillingPackagesFilters sub-struct for the filters this endpoint honors.
//
// BillingPackagesListOpts is a value type. Concurrent-safe by construction — the
// entity layer never mutates a caller's opts.
type BillingPackagesListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters BillingPackagesFilters
}

// BillingPackagesFilters is the typed filter set for the billing-packages
// endpoint. Each field has a native slot on the generated
// GetAllBillingPackagesParams.
type BillingPackagesFilters struct {
	// Type narrows by billing package type (volume, maintenance).
	Type string

	// LedgerID narrows by ledger UUID.
	LedgerID string
}

// Validate enforces SDK-side preconditions on BillingPackagesListOpts.
func (o BillingPackagesListOpts) Validate() error {
	return ValidatePageListOpts("BillingPackagesListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders a BillingPackagesListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it. Treat as an
// internal contract; not part of the user-facing API.
func (o BillingPackagesListOpts) ToQueryParams() map[string]string {
	params := PageQueryParams(o.PageListOpts)

	if o.Filters.Type != "" {
		params["type"] = o.Filters.Type
	}

	if o.Filters.LedgerID != "" {
		params["ledgerId"] = o.Filters.LedgerID
	}

	return params
}

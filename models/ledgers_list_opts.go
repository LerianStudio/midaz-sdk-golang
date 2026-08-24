// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// LedgersListOpts is the typed options struct for the ledger list surface:
// V1.Ledgers.List and the All / Pages iterators.
//
// Embeds PageListOpts for the shared pagination/sort/date-range
// fields; attaches a LedgersFilters sub-struct carrying only the
// filter fields the ledgers endpoint actually honors.
//
// LedgersListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
type LedgersListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters LedgersFilters
}

// LedgersFilters is the typed filter set for the ledgers endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type LedgersFilters struct {
	// Name narrows by ledger name (substring match per API).
	Name string

	// Status narrows by ledger status.
	Status string

	// IncludeDeleted, when true, when true, includes soft-deleted ledgers.
	IncludeDeleted bool
}

// Validate enforces SDK-side preconditions on LedgersListOpts.
func (o LedgersListOpts) Validate() error {
	return ValidatePageListOpts("LedgersListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an LedgersListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o LedgersListOpts) ToQueryParams() map[string]string {
	params := PageQueryParams(o.PageListOpts)

	if o.Filters.Name != "" {
		params["name"] = o.Filters.Name
	}

	if o.Filters.Status != "" {
		params["status"] = o.Filters.Status
	}

	if o.Filters.IncludeDeleted {
		params["include_deleted"] = "true"
	}

	return params
}

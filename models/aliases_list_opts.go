// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Apache-2.0

package models

// AliasesListOpts is the typed options struct for ListAliases and
// the ListAliasesAll / ListAliasesPages iterators.
//
// Embeds PageListOpts for the shared pagination/sort/date-range
// fields; attaches a AliasesFilters sub-struct carrying only the
// filter fields the aliases endpoint actually honors.
//
// AliasesListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
type AliasesListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters AliasesFilters
}

// AliasesFilters is the typed filter set for the aliases endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type AliasesFilters struct {
	// HolderID narrows to aliases owned by a specific holder.
	HolderID string

	// AccountID narrows to aliases tied to a specific account.
	AccountID string
}

// Validate enforces SDK-side preconditions on AliasesListOpts.
func (o AliasesListOpts) Validate() error {
	return ValidatePageListOpts("AliasesListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an AliasesListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o AliasesListOpts) ToQueryParams() map[string]string {
	params := PageQueryParams(o.PageListOpts)

	if o.Filters.HolderID != "" {
		params["holder_id"] = o.Filters.HolderID
	}

	if o.Filters.AccountID != "" {
		params["account_id"] = o.Filters.AccountID
	}

	return params
}

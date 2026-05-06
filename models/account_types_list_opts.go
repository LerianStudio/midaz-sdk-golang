// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Apache-2.0

package models

// AccountTypesListOpts is the typed options struct for ListAccountTypes and
// the ListAccountTypesAll / ListAccountTypesPages iterators.
//
// Embeds PageListOpts for the shared pagination/sort/date-range
// fields; attaches a AccountTypesFilters sub-struct carrying only the
// filter fields the account-types endpoint actually honors.
//
// AccountTypesListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
type AccountTypesListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters AccountTypesFilters
}

// AccountTypesFilters is the typed filter set for the account-types endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type AccountTypesFilters struct {
	// Name narrows by account type name (substring match per API).
	Name string

	// KeyValue narrows by the key_value identifier of the account type.
	KeyValue string

	// IncludeDeleted, when true, when true, includes soft-deleted account types.
	IncludeDeleted bool
}

// Validate enforces SDK-side preconditions on AccountTypesListOpts.
func (o AccountTypesListOpts) Validate() error {
	return ValidatePageListOpts("AccountTypesListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an AccountTypesListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o AccountTypesListOpts) ToQueryParams() map[string]string {
	params := PageQueryParams(o.PageListOpts)

	if o.Filters.Name != "" {
		params["name"] = o.Filters.Name
	}

	if o.Filters.KeyValue != "" {
		params["key_value"] = o.Filters.KeyValue
	}

	if o.Filters.IncludeDeleted {
		params["include_deleted"] = "true"
	}

	return params
}

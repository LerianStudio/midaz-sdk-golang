// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// HoldersListOpts is the typed options struct for ListHolders and
// the ListHoldersAll / ListHoldersPages iterators.
//
// Embeds PageListOpts for the shared pagination/sort/date-range
// fields; attaches a HoldersFilters sub-struct carrying only the
// filter fields the holders endpoint actually honors.
//
// HoldersListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
type HoldersListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters HoldersFilters
}

// HoldersFilters is the typed filter set for the holders endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type HoldersFilters struct {
	// Name narrows by holder name (substring match per API).
	Name string

	// Document narrows by holder document (CPF/CNPJ).
	Document string

	// Status narrows by holder status.
	Status string

	// ExternalID narrows by external system identifier.
	ExternalID string

	// IncludeDeleted, when true, includes soft-deleted holders.
	IncludeDeleted bool
}

// Validate enforces SDK-side preconditions on HoldersListOpts.
func (o HoldersListOpts) Validate() error {
	return ValidatePageListOpts("HoldersListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an HoldersListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o HoldersListOpts) ToQueryParams() map[string]string {
	params := PageQueryParams(o.PageListOpts)

	if o.Filters.Name != "" {
		params["name"] = o.Filters.Name
	}

	if o.Filters.Document != "" {
		params["document"] = o.Filters.Document
	}

	if o.Filters.Status != "" {
		params["status"] = o.Filters.Status
	}

	if o.Filters.ExternalID != "" {
		params["external_id"] = o.Filters.ExternalID
	}

	if o.Filters.IncludeDeleted {
		params["include_deleted"] = "true"
	}

	return params
}

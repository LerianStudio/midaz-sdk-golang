// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// PortfoliosListOpts is the typed options struct for ListPortfolios and
// the ListPortfoliosAll / ListPortfoliosPages iterators.
//
// Embeds PageListOpts for the shared pagination/sort/date-range
// fields; attaches a PortfoliosFilters sub-struct carrying only the
// filter fields the portfolios endpoint actually honors.
//
// PortfoliosListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
type PortfoliosListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters PortfoliosFilters
}

// PortfoliosFilters is the typed filter set for the portfolios endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type PortfoliosFilters struct {
	// Name narrows by portfolio name.
	Name string

	// EntityID narrows by external entity ID.
	EntityID string

	// Status narrows by status.
	Status string

	// IncludeDeleted, when true, when true, includes soft-deleted portfolios.
	IncludeDeleted bool
}

// Validate enforces SDK-side preconditions on PortfoliosListOpts.
func (o PortfoliosListOpts) Validate() error {
	return ValidatePageListOpts("PortfoliosListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an PortfoliosListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o PortfoliosListOpts) ToQueryParams() map[string]string {
	params := PageQueryParams(o.PageListOpts)

	if o.Filters.Name != "" {
		params["name"] = o.Filters.Name
	}

	if o.Filters.EntityID != "" {
		params["entity_id"] = o.Filters.EntityID
	}

	if o.Filters.Status != "" {
		params["status"] = o.Filters.Status
	}

	if o.Filters.IncludeDeleted {
		params["include_deleted"] = "true"
	}

	return params
}

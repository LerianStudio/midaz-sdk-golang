// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// HoldersListOpts is the typed options struct for ListHolders and
// the ListHoldersAll / ListHoldersPages iterators.
//
// Embeds CursorListOpts for the shared cursor/sort fields; attaches a
// HoldersFilters sub-struct carrying only the filter fields the holders
// endpoint actually honors.
//
// HoldersListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
//
// IMPORTANT: This is a CURSOR-PAGINATED endpoint. The struct does NOT
// expose Page or Offset fields. Seed Cursor to resume a mid-stream page,
// or read the NextCursor from the previous response's Pagination to fetch
// the next page. v3 compile-time prevents the v2 footgun where setting
// WithPage on a cursor endpoint silently dropped the value (audit finding 5.5).
// The endpoint has no start_date/end_date slot, so Validate REJECTS any date
// filter rather than shipping a silently-ignored one.
type HoldersListOpts struct {
	CursorListOpts

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

// Validate enforces the shared cursor-list preconditions (limit bounds, sort
// direction) and REJECTS any date filter: the generated ListHoldersParams has no
// start_date/end_date slot, so a date would validate then silently drop,
// returning the full unfiltered set.
func (o HoldersListOpts) Validate() error {
	return ValidateCursorListOptsNoDates("HoldersListOpts.Validate", o.CursorListOpts)
}

// ToQueryParams renders an HoldersListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o HoldersListOpts) ToQueryParams() map[string]string {
	params := CursorQueryParams(o.CursorListOpts)

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

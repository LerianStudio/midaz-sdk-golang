// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Apache-2.0

package models

// SegmentsListOpts is the typed options struct for ListSegments and
// the ListSegmentsAll / ListSegmentsPages iterators.
//
// Embeds PageListOpts for the shared pagination/sort/date-range
// fields; attaches a SegmentsFilters sub-struct carrying only the
// filter fields the segments endpoint actually honors.
//
// SegmentsListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
type SegmentsListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters SegmentsFilters
}

// SegmentsFilters is the typed filter set for the segments endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type SegmentsFilters struct {
	// Name narrows by segment name.
	Name string

	// Status narrows by status.
	Status string

	// IncludeDeleted, when true, when true, includes soft-deleted segments.
	IncludeDeleted bool
}

// Validate enforces SDK-side preconditions on SegmentsListOpts.
func (o SegmentsListOpts) Validate() error {
	return ValidatePageListOpts("SegmentsListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an SegmentsListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o SegmentsListOpts) ToQueryParams() map[string]string {
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

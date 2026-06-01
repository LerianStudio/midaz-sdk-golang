// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// AssetsListOpts is the typed options struct for ListAssets and
// the ListAssetsAll / ListAssetsPages iterators.
//
// Embeds PageListOpts for the shared pagination/sort/date-range
// fields; attaches a AssetsFilters sub-struct carrying only the
// filter fields the assets endpoint actually honors.
//
// AssetsListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
type AssetsListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters AssetsFilters
}

// AssetsFilters is the typed filter set for the assets endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type AssetsFilters struct {
	// Code narrows by asset code (e.g. "USD", "BRL").
	Code string

	// Type narrows by asset type (e.g. "currency", "security").
	Type string

	// Status narrows by status (e.g. "ACTIVE", "INACTIVE").
	Status string
}

// Validate enforces SDK-side preconditions on AssetsListOpts.
func (o AssetsListOpts) Validate() error {
	return ValidatePageListOpts("AssetsListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an AssetsListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o AssetsListOpts) ToQueryParams() map[string]string {
	params := PageQueryParams(o.PageListOpts)

	if o.Filters.Code != "" {
		params["code"] = o.Filters.Code
	}

	if o.Filters.Type != "" {
		params["type"] = o.Filters.Type
	}

	if o.Filters.Status != "" {
		params["status"] = o.Filters.Status
	}

	return params
}

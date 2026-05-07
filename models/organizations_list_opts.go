// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Apache-2.0

package models

// OrganizationsListOpts is the typed options struct for ListOrganizations and
// the ListOrganizationsAll / ListOrganizationsPages iterators.
//
// Embeds PageListOpts for the shared pagination/sort/date-range
// fields; attaches a OrganizationsFilters sub-struct carrying only the
// filter fields the organizations endpoint actually honors.
//
// OrganizationsListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
type OrganizationsListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters OrganizationsFilters
}

// OrganizationsFilters is the typed filter set for the organizations endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type OrganizationsFilters struct {
	// LegalName narrows by legal name (substring match per API).
	LegalName string

	// Status narrows by status.
	Status string

	// IncludeDeleted, when true, when true, includes soft-deleted organizations.
	IncludeDeleted bool
}

// Validate enforces SDK-side preconditions on OrganizationsListOpts.
func (o OrganizationsListOpts) Validate() error {
	return ValidatePageListOpts("OrganizationsListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an OrganizationsListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o OrganizationsListOpts) ToQueryParams() map[string]string {
	params := PageQueryParams(o.PageListOpts)

	if o.Filters.LegalName != "" {
		params["legal_name"] = o.Filters.LegalName
	}

	if o.Filters.Status != "" {
		params["status"] = o.Filters.Status
	}

	if o.Filters.IncludeDeleted {
		params["include_deleted"] = "true"
	}

	return params
}

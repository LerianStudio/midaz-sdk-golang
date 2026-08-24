// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// AccountsListOpts is the typed options struct for the account list surface:
// V1.Accounts.List and the All / Pages iterators.
//
// Embeds PageListOpts for the shared pagination/sort/date-range
// fields; attaches an AccountsFilters sub-struct carrying only the
// filter fields the accounts endpoint actually honors.
//
// AccountsListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
type AccountsListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters AccountsFilters
}

// AccountsFilters is the typed filter set for the accounts endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type AccountsFilters struct {
	// Type narrows by account type (e.g. "CUSTOMER", "DEPOSIT").
	Type string

	// Status narrows by account status (e.g. "ACTIVE", "BLOCKED").
	Status string

	// AssetCode narrows by the asset associated with the account.
	AssetCode string

	// HolderID narrows to accounts owned by a specific holder.
	HolderID string

	// PortfolioID narrows to accounts within a specific portfolio.
	PortfolioID string

	// SegmentID narrows to accounts within a specific segment.
	SegmentID string

	// Alias narrows to accounts with a specific alias.
	Alias string

	// ParentAccountID narrows to child accounts of a specific parent.
	ParentAccountID string

	// Name narrows by account name (substring match per API).
	Name string

	// EntityID narrows by external entity ID.
	EntityID string

	// IncludeDeleted, when true, includes soft-deleted accounts in
	// the result set.
	IncludeDeleted bool

	// Blocked, when true, narrows to accounts in the BLOCKED state.
	Blocked bool
}

// Validate enforces SDK-side preconditions on AccountsListOpts.
func (o AccountsListOpts) Validate() error {
	return ValidatePageListOpts("AccountsListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an AccountsListOpts into the wire query map.
//
// Exported (capitalized) because the entity layer in package entities
// calls it. Treat as an internal contract; not part of the user-facing
// API. Customers reach the wire path through ListAccounts/-All/-Pages.
func (o AccountsListOpts) ToQueryParams() map[string]string {
	params := PageQueryParams(o.PageListOpts)

	if o.Filters.Type != "" {
		params["type"] = o.Filters.Type
	}

	if o.Filters.Status != "" {
		params["status"] = o.Filters.Status
	}

	if o.Filters.AssetCode != "" {
		params["asset_code"] = o.Filters.AssetCode
	}

	if o.Filters.HolderID != "" {
		params["holder_id"] = o.Filters.HolderID
	}

	if o.Filters.PortfolioID != "" {
		params["portfolio_id"] = o.Filters.PortfolioID
	}

	if o.Filters.SegmentID != "" {
		params["segment_id"] = o.Filters.SegmentID
	}

	if o.Filters.Alias != "" {
		params["alias"] = o.Filters.Alias
	}

	if o.Filters.ParentAccountID != "" {
		params["parent_account_id"] = o.Filters.ParentAccountID
	}

	if o.Filters.Name != "" {
		params["name"] = o.Filters.Name
	}

	if o.Filters.EntityID != "" {
		params["entity_id"] = o.Filters.EntityID
	}

	if o.Filters.IncludeDeleted {
		params["include_deleted"] = "true"
	}

	if o.Filters.Blocked {
		params["blocked"] = "true"
	}

	return params
}

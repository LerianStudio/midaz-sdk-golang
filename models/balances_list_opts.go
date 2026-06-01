// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// BalancesListOpts is the typed options struct for ListBalances and
// the ListBalancesAll / ListBalancesPages iterators.
//
// Embeds PageListOpts for the shared pagination/sort/date-range
// fields; attaches a BalancesFilters sub-struct carrying only the
// filter fields the balances endpoint actually honors.
//
// BalancesListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
type BalancesListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters BalancesFilters
}

// BalancesFilters is the typed filter set for the balances endpoint.
// Only fields the endpoint actually honors are exposed (replaces the
// v2 mega-struct ListOptions with its 30+ fluent setters that mostly
// no-op'd on this endpoint — audit finding 5.12).
type BalancesFilters struct {
	// AccountID narrows to balances on a specific account.
	AccountID string

	// AssetCode narrows by asset code.
	AssetCode string

	// Status narrows by balance status.
	Status string
}

// Validate enforces SDK-side preconditions on BalancesListOpts.
func (o BalancesListOpts) Validate() error {
	return ValidatePageListOpts("BalancesListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an BalancesListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o BalancesListOpts) ToQueryParams() map[string]string {
	params := PageQueryParams(o.PageListOpts)

	if o.Filters.AccountID != "" {
		params["account_id"] = o.Filters.AccountID
	}

	if o.Filters.AssetCode != "" {
		params["asset_code"] = o.Filters.AssetCode
	}

	if o.Filters.Status != "" {
		params["status"] = o.Filters.Status
	}

	return params
}

// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// BalancesListOpts is the typed options struct for the balance lists —
// ListBalances / ListAccountBalances and their All / Pages iterators.
//
// It embeds CursorListOpts, not PageListOpts, because the server paginates
// these lists by opaque cursor: it builds each response from
// limit/sort_order/start_date/end_date plus a cursor, and drops "page" on the
// floor. An opts shape carrying Page would compile, send a parameter with no
// wire slot, and leave an iterator that increments it re-requesting the first
// page forever.
//
// It carries no Filters for the same reason: the balance lists honour none. The
// account is expressed as a path segment on the account-scoped list, and there
// is no asset-code or status predicate to narrow by.
//
// BalancesListOpts is a value type. Concurrent-safe by construction —
// the entity layer never mutates a caller's opts.
type BalancesListOpts struct {
	CursorListOpts
}

// Validate enforces SDK-side preconditions on BalancesListOpts.
func (o BalancesListOpts) Validate() error {
	return ValidateCursorListOpts("BalancesListOpts.Validate", o.CursorListOpts)
}

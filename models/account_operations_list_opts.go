// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// AccountOperationsListOpts is the typed options struct for the
// account-scoped operations sub-list (GetAllOperationsByAccount).
//
// This is a CURSOR-PAGINATED endpoint, so it embeds CursorListOpts
// (Limit/Cursor/SortDirection/StartDate/EndDate) — never Page.
//
// OperationsListOpts is an alias of this type: the server has exactly one
// operations list and it is account-scoped, so both names address the same
// endpoint and honour the same filters.
//
// AccountOperationsListOpts is a value type. Concurrent-safe by construction.
type AccountOperationsListOpts struct {
	CursorListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters AccountOperationsFilters
}

// AccountOperationsFilters is the typed filter set the account-operations
// sub-list actually honors, mirroring the generated
// GetAllOperationsByAccountParams filter slots.
type AccountOperationsFilters struct {
	// Type narrows by operation type (e.g. "DEBIT", "CREDIT").
	Type string

	// Direction narrows by accounting direction (e.g. "debit", "credit").
	Direction string

	// RouteID narrows by operation route ID (UUID).
	RouteID string

	// RouteCode narrows by operation route code.
	RouteCode string
}

// Validate enforces SDK-side preconditions on AccountOperationsListOpts.
func (o AccountOperationsListOpts) Validate() error {
	return ValidateCursorListOpts("AccountOperationsListOpts.Validate", o.CursorListOpts)
}

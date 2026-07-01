// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// AccountOperationsListOpts is the typed options struct for the
// account-scoped operations sub-list (GetAllOperationsByAccount).
//
// This is a CURSOR-PAGINATED endpoint, so it embeds CursorListOpts
// (Limit/Cursor/SortDirection/StartDate/EndDate) — never Page.
//
// Its filter set differs from the top-level OperationsListOpts: the
// account-operations endpoint honors type/direction/route_id/route_code
// (the account is already pinned by the path), not the status/asset_code
// filters the ledger-wide operations list exposes. Modeling that
// difference explicitly is deliberate — reusing OperationsFilters here
// would silently drop direction/route filters and expose two filters this
// endpoint ignores.
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

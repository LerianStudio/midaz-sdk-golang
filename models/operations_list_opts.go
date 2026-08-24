// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

// OperationsListOpts is the typed options struct for ListOperations and
// the ListOperationsAll / ListOperationsPages iterators.
//
// There is exactly one operations list on the server and it is account-scoped,
// so this name and AccountOperationsListOpts address the same endpoint and
// therefore denote the same type. Keeping them as one shape is what stops the
// two from drifting into disagreement about which filters the endpoint honours.
type OperationsListOpts = AccountOperationsListOpts

// OperationsFilters is the typed filter set for the operations list. Alias of
// AccountOperationsFilters — see OperationsListOpts.
type OperationsFilters = AccountOperationsFilters

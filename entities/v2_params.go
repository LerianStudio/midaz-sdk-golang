// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// Every V2 list operation declares exactly the same query set as its V1 twin —
// verified field by field against the generated params structs, not assumed.
// oapi-codegen still emits a distinct Go type per operation, so the identical
// sets are not assignable to each other and something has to bridge them.
//
// The bridge reads the V1 mapper's output and copies it across field by field,
// rather than re-deriving each query param from the opts a second time. Two
// mappers for one query set would drift the moment one of them gained a filter,
// and the drift would be silent: a filter present on V1 and absent on V2 reads
// as a narrower result set that was never narrowed. One mapper, one place a
// filter can be added.
//
// A whole-struct conversion would be shorter and is deliberately NOT used: Go
// permits it between structs with identical field types, so two same-typed
// fields swapping order in a regenerated client would compile and silently map
// start_date onto end_date. Named fields make that a compile error.

// listOrganizationsV2Params renders the V2 query set for this list.
func listOrganizationsV2Params(opts models.OrganizationsListOpts) *genledger.ListOrganizationsV2Params {
	v1 := listOrganizationsParams(opts)

	return &genledger.ListOrganizationsV2Params{
		Metadata:        v1.Metadata,
		Limit:           v1.Limit,
		Page:            v1.Page,
		StartDate:       v1.StartDate,
		EndDate:         v1.EndDate,
		SortOrder:       v1.SortOrder,
		LegalName:       v1.LegalName,
		DoingBusinessAs: v1.DoingBusinessAs,
		Status:          v1.Status,
		LegalDocument:   v1.LegalDocument,
	}
}

// listLedgersV2Params renders the V2 query set for this list.
func listLedgersV2Params(opts models.LedgersListOpts) *genledger.ListLedgersV2Params {
	v1 := listLedgersParams(opts)

	return &genledger.ListLedgersV2Params{
		Metadata:  v1.Metadata,
		Limit:     v1.Limit,
		Page:      v1.Page,
		StartDate: v1.StartDate,
		EndDate:   v1.EndDate,
		SortOrder: v1.SortOrder,
		Name:      v1.Name,
		Status:    v1.Status,
	}
}

// listAccountsV2Params renders the V2 query set for this list.
func listAccountsV2Params(opts models.AccountsListOpts) *genledger.ListAccountsV2Params {
	v1 := listAccountsParams(opts)

	return &genledger.ListAccountsV2Params{
		Metadata:        v1.Metadata,
		Limit:           v1.Limit,
		Page:            v1.Page,
		StartDate:       v1.StartDate,
		EndDate:         v1.EndDate,
		SortOrder:       v1.SortOrder,
		PortfolioId:     v1.PortfolioId,
		SegmentId:       v1.SegmentId,
		Status:          v1.Status,
		Type:            v1.Type,
		AssetCode:       v1.AssetCode,
		EntityId:        v1.EntityId,
		Blocked:         v1.Blocked,
		ParentAccountId: v1.ParentAccountId,
		Name:            v1.Name,
		Alias:           v1.Alias,
	}
}

// listAccountTypesV2Params renders the V2 query set for this list.
func listAccountTypesV2Params(opts models.AccountTypesListOpts) *genledger.ListAccountTypesV2Params {
	v1 := listAccountTypesParams(opts)

	return &genledger.ListAccountTypesV2Params{
		Metadata:  v1.Metadata,
		KeyValue:  v1.KeyValue,
		Limit:     v1.Limit,
		Page:      v1.Page,
		Cursor:    v1.Cursor,
		StartDate: v1.StartDate,
		EndDate:   v1.EndDate,
		SortOrder: v1.SortOrder,
	}
}

// listAssetsV2Params renders the V2 query set for this list.
func listAssetsV2Params(opts models.AssetsListOpts) *genledger.ListAssetsV2Params {
	v1 := listAssetsParams(opts)

	return &genledger.ListAssetsV2Params{
		Metadata:  v1.Metadata,
		Limit:     v1.Limit,
		Page:      v1.Page,
		StartDate: v1.StartDate,
		EndDate:   v1.EndDate,
		SortOrder: v1.SortOrder,
	}
}

// balancesListV2Params renders the V2 query set for this list.
func balancesListV2Params(opts models.BalancesListOpts) *genledger.GetAllBalancesV2Params {
	v1 := balancesListParams(opts)

	return &genledger.GetAllBalancesV2Params{
		Limit:     v1.Limit,
		StartDate: v1.StartDate,
		EndDate:   v1.EndDate,
		SortOrder: v1.SortOrder,
		Cursor:    v1.Cursor,
	}
}

// accountBalancesListV2Params renders the V2 query set for this list.
func accountBalancesListV2Params(opts models.BalancesListOpts) *genledger.GetAllBalancesByAccountIDV2Params {
	v1 := accountBalancesListParams(opts)

	return &genledger.GetAllBalancesByAccountIDV2Params{
		Limit:     v1.Limit,
		StartDate: v1.StartDate,
		EndDate:   v1.EndDate,
		SortOrder: v1.SortOrder,
		Cursor:    v1.Cursor,
	}
}

// operationsByAccountV2Params renders the V2 query set for this list.
func operationsByAccountV2Params(opts models.AccountOperationsListOpts) *genledger.GetAllOperationsByAccountV2Params {
	v1 := operationsByAccountParams(opts)

	return &genledger.GetAllOperationsByAccountV2Params{
		Metadata:  v1.Metadata,
		Limit:     v1.Limit,
		StartDate: v1.StartDate,
		EndDate:   v1.EndDate,
		SortOrder: v1.SortOrder,
		Cursor:    v1.Cursor,
		Type:      v1.Type,
		Direction: v1.Direction,
		RouteId:   v1.RouteId,
		RouteCode: v1.RouteCode,
	}
}

// listPortfoliosV2Params renders the V2 query set for this list.
func listPortfoliosV2Params(opts models.PortfoliosListOpts) *genledger.ListPortfoliosV2Params {
	v1 := listPortfoliosParams(opts)

	return &genledger.ListPortfoliosV2Params{
		Metadata:  v1.Metadata,
		EntityId:  v1.EntityId,
		Status:    v1.Status,
		Limit:     v1.Limit,
		Page:      v1.Page,
		StartDate: v1.StartDate,
		EndDate:   v1.EndDate,
		SortOrder: v1.SortOrder,
	}
}

// listSegmentsV2Params renders the V2 query set for this list.
func listSegmentsV2Params(opts models.SegmentsListOpts) *genledger.ListSegmentsV2Params {
	v1 := listSegmentsParams(opts)

	return &genledger.ListSegmentsV2Params{
		Metadata:  v1.Metadata,
		Limit:     v1.Limit,
		Page:      v1.Page,
		StartDate: v1.StartDate,
		EndDate:   v1.EndDate,
		SortOrder: v1.SortOrder,
	}
}

// listOperationRoutesV2Params renders the V2 query set for this list.
func listOperationRoutesV2Params(opts models.OperationRoutesListOpts) *genledger.ListOperationRoutesV2Params {
	v1 := listOperationRoutesParams(opts)

	return &genledger.ListOperationRoutesV2Params{
		Limit:     v1.Limit,
		StartDate: v1.StartDate,
		EndDate:   v1.EndDate,
		SortOrder: v1.SortOrder,
		Cursor:    v1.Cursor,
	}
}

// listTransactionRoutesV2Params renders the V2 query set for this list.
func listTransactionRoutesV2Params(opts models.TransactionRoutesListOpts) *genledger.ListTransactionRoutesV2Params {
	v1 := listTransactionRoutesParams(opts)

	return &genledger.ListTransactionRoutesV2Params{
		Limit:     v1.Limit,
		StartDate: v1.StartDate,
		EndDate:   v1.EndDate,
		SortOrder: v1.SortOrder,
		Cursor:    v1.Cursor,
	}
}

// listTransactionsV2Params renders the V2 query set for this list.
func listTransactionsV2Params(opts models.TransactionsListOpts) *genledger.GetAllTransactionsV2Params {
	v1 := listTransactionsParams(opts)

	return &genledger.GetAllTransactionsV2Params{
		Metadata:  v1.Metadata,
		Limit:     v1.Limit,
		StartDate: v1.StartDate,
		EndDate:   v1.EndDate,
		SortOrder: v1.SortOrder,
		Cursor:    v1.Cursor,
	}
}

// countTransactionsV2Params renders the V2 query set for this list.
func countTransactionsV2Params(opts models.TransactionsListOpts) *genledger.CountTransactionsByFiltersV2Params {
	v1 := countTransactionsParams(opts)

	return &genledger.CountTransactionsByFiltersV2Params{
		Route:     v1.Route,
		Status:    v1.Status,
		StartDate: v1.StartDate,
		EndDate:   v1.EndDate,
	}
}

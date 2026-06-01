// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPageListOpts_OverLimit_ValidatesBeforeRequest covers H29: ALL ListXxx
// entity methods must short-circuit on Validate() before issuing any HTTP
// request when opts.Limit > MaxLimit. This is the entity-side regression
// pinning the contract that backs ValidatePageListOpts and
// ValidateCursorListOpts (already covered at the model level).
//
// The contract under test:
//   - opts.Limit = MaxLimit + 1 → entity returns "limit exceeds maximum"
//   - The httptest server's request counter stays at 0 (no wire traffic)
//
// The asset_rates entity is already covered by
// TestListAssetRatesByAssetCode_ValidatesOptsBeforeRequest. The portfolios
// entity is partially covered by
// TestPortfoliosEntity_ListPortfolios_ValidationBeforeRequest (org/ledger
// validation). This test fills in the limit-exceeds path for the
// remaining entities — the gap H29 was opened to close.
//
// Cursor entities (transactions, operations, operation_routes,
// transaction_routes) get the same treatment in
// TestCursorListOpts_OverLimit_ValidatesBeforeRequest below.
func TestPageListOpts_OverLimit_ValidatesBeforeRequest(t *testing.T) {
	tests := []struct {
		name string
		// run drives the entity's ListXxx method against the test server.
		// Returns the entity-layer error.
		run func(t *testing.T, baseURL string) error
	}{
		{
			name: "ListAccounts",
			run: func(_ *testing.T, baseURL string) error {
				e := newAccountsEntity(http.DefaultClient, "tok", map[string]string{"onboarding": baseURL})
				_, err := e.ListAccounts(context.Background(), "org", "ledger",
					models.AccountsListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListAccountTypes",
			run: func(_ *testing.T, baseURL string) error {
				e := newAccountTypesEntity(http.DefaultClient, "tok", map[string]string{"onboarding": baseURL})
				_, err := e.ListAccountTypes(context.Background(), "org", "ledger",
					models.AccountTypesListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListAliases",
			run: func(_ *testing.T, baseURL string) error {
				e := newAliasesEntity(http.DefaultClient, map[string]string{"crm": baseURL})
				_, err := e.ListAliases(context.Background(), "org",
					models.AliasesListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListAssets",
			run: func(_ *testing.T, baseURL string) error {
				e := newAssetsEntity(http.DefaultClient, "tok", map[string]string{"onboarding": baseURL})
				_, err := e.ListAssets(context.Background(), "org", "ledger",
					models.AssetsListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListBalances",
			run: func(_ *testing.T, baseURL string) error {
				e := newBalancesEntity(http.DefaultClient, "tok", map[string]string{"transaction": baseURL})
				_, err := e.ListBalances(context.Background(), "org", "ledger",
					models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListHolders",
			run: func(_ *testing.T, baseURL string) error {
				e := newHoldersEntity(http.DefaultClient, "token", map[string]string{"crm": baseURL})
				_, err := e.ListHolders(context.Background(), "org",
					models.HoldersListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListLedgers",
			run: func(_ *testing.T, baseURL string) error {
				e := newLedgersEntity(http.DefaultClient, "token", map[string]string{"onboarding": baseURL})
				_, err := e.ListLedgers(context.Background(), "org",
					models.LedgersListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListOrganizations",
			run: func(_ *testing.T, baseURL string) error {
				e := newOrganizationsEntity(http.DefaultClient, "tok", map[string]string{"onboarding": baseURL})
				_, err := e.ListOrganizations(context.Background(),
					models.OrganizationsListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListPortfolios",
			run: func(_ *testing.T, baseURL string) error {
				e := newPortfoliosEntity(http.DefaultClient, "tok", map[string]string{"onboarding": baseURL})
				_, err := e.ListPortfolios(context.Background(), "org", "ledger",
					models.PortfoliosListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListSegments",
			run: func(_ *testing.T, baseURL string) error {
				e := newSegmentsEntity(http.DefaultClient, "tok", map[string]string{"onboarding": baseURL})
				_, err := e.ListSegments(context.Background(), "org", "ledger",
					models.SegmentsListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			err := tt.run(t, server.URL)
			require.Error(t, err, "Limit > MaxLimit must be rejected before sending request")
			assert.Contains(t, err.Error(), "limit exceeds maximum")
			assert.Equal(t, int32(0), hits.Load(), "validation failure must short-circuit before any HTTP request")
		})
	}
}

// TestCursorListOpts_OverLimit_ValidatesBeforeRequest mirrors
// TestPageListOpts_OverLimit_ValidatesBeforeRequest for the cursor-paginated
// entities. asset_rates is intentionally omitted here — it has its own
// dedicated test (TestListAssetRatesByAssetCode_ValidatesOptsBeforeRequest).
//
// Coverage history: the four cursor entities (transactions, operations,
// operation_routes, transaction_routes) all sit on the same opts.Validate()
// pre-flight contract. ListTransactions adopted it first; the other three
// were brought into line by the M23 fix that added the missing call to each
// entity method. This test pins the post-M23 behavior across all four.
func TestCursorListOpts_OverLimit_ValidatesBeforeRequest(t *testing.T) {
	tests := []struct {
		name string
		run  func(_ *testing.T, baseURL string) error
	}{
		{
			name: "ListTransactions",
			run: func(_ *testing.T, baseURL string) error {
				e := newTransactionsEntity(http.DefaultClient, map[string]string{"transaction": baseURL})
				_, err := e.ListTransactions(context.Background(), "org", "ledger",
					models.TransactionsListOpts{CursorListOpts: models.CursorListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListOperations",
			run: func(_ *testing.T, baseURL string) error {
				e := newOperationsEntity(http.DefaultClient, "tok", map[string]string{"transaction": baseURL})
				_, err := e.ListOperations(context.Background(), "org", "ledger", "acc",
					models.OperationsListOpts{CursorListOpts: models.CursorListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListOperationRoutes",
			run: func(_ *testing.T, baseURL string) error {
				e := newOperationRoutesEntity(http.DefaultClient, "tok", map[string]string{"transaction": baseURL})
				_, err := e.ListOperationRoutes(context.Background(), "org", "ledger",
					models.OperationRoutesListOpts{CursorListOpts: models.CursorListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListTransactionRoutes",
			run: func(_ *testing.T, baseURL string) error {
				e := newTransactionRoutesEntity(http.DefaultClient, "tok", map[string]string{"transaction": baseURL})
				_, err := e.ListTransactionRoutes(context.Background(), "org", "ledger",
					models.TransactionRoutesListOpts{CursorListOpts: models.CursorListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			err := tt.run(t, server.URL)
			require.Error(t, err, "Limit > MaxLimit must be rejected before sending request")
			assert.Contains(t, err.Error(), "limit exceeds maximum")
			assert.Equal(t, int32(0), hits.Load(), "validation failure must short-circuit before any HTTP request")
		})
	}
}

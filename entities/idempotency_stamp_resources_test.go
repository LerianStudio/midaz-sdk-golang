// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
)

const (
	resOrg    = "11111111-1111-1111-1111-111111111111"
	resLedger = "22222222-2222-2222-2222-222222222222"
)

// TestLedgerResourceWritesStampIdempotency proves Task 5.3.1: every write on the
// 12 non-transaction ledger facades stamps an auto-generated X-Idempotency with
// the gate ON and stays header-free with the gate OFF (no ctx key). One row per
// write op — a missing editor append, or a hardcoded gate, fails here.
func TestLedgerResourceWritesStampIdempotency(t *testing.T) {
	cases := []struct {
		name string
		fire func(t *testing.T, srv *httptest.Server, gate bool)
	}{
		{"organizations.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newOrganizationsFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(),
				&models.CreateOrganizationInput{LegalName: "Acme", LegalDocument: "doc-1"})
		}},
		{"organizations.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newOrganizationsFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), stampID,
				&models.UpdateOrganizationInput{LegalName: "Acme Renamed"})
		}},
		{"organizations.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newOrganizationsFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), stampID)
		}},
		{"ledgers.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newLedgersFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), resOrg, &models.CreateLedgerInput{Name: "Treasury"})
		}},
		{"ledgers.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newLedgersFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), resOrg, stampID, &models.UpdateLedgerInput{Name: "Renamed"})
		}},
		{"ledgers.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newLedgersFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), resOrg, stampID)
		}},
		{"ledgers.UpdateSettings", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newLedgersFacade(newTestLedgerClient(t, srv), gate).UpdateSettings(context.Background(), resOrg, stampID,
				models.NewUpdateLedgerSettingsInput().WithRequireHolder(true))
		}},
		{"accounts.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newAccountsFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), resOrg, resLedger,
				&models.CreateAccountInput{Name: "Checking", AssetCode: "USD", Type: "deposit"})
		}},
		{"accounts.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newAccountsFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), resOrg, resLedger, stampID, &models.UpdateAccountInput{Name: "Renamed"})
		}},
		{"accounts.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newAccountsFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), resOrg, resLedger, stampID)
		}},
		{"assets.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newAssetsFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), resOrg, resLedger,
				&models.CreateAssetInput{Name: "US Dollar", Code: "USD", Type: "currency"})
		}},
		{"assets.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newAssetsFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), resOrg, resLedger, stampID, &models.UpdateAssetInput{Name: "Renamed"})
		}},
		{"assets.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newAssetsFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), resOrg, resLedger, stampID)
		}},
		{"portfolios.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newPortfoliosFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), resOrg, resLedger,
				&models.CreatePortfolioInput{Name: "Alpha", EntityID: "ent-1"})
		}},
		{"portfolios.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newPortfoliosFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), resOrg, resLedger, stampID, &models.UpdatePortfolioInput{Name: "Renamed"})
		}},
		{"portfolios.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newPortfoliosFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), resOrg, resLedger, stampID)
		}},
		{"segments.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newSegmentsFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), resOrg, resLedger, &models.CreateSegmentInput{Name: "North"})
		}},
		{"segments.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newSegmentsFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), resOrg, resLedger, stampID, &models.UpdateSegmentInput{Name: "Renamed"})
		}},
		{"segments.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newSegmentsFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), resOrg, resLedger, stampID)
		}},
		{"accountTypes.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newAccountTypesFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), resOrg, resLedger,
				&models.CreateAccountTypeInput{Name: "Cash", KeyValue: "CASH"})
		}},
		{"accountTypes.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newAccountTypesFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), resOrg, resLedger, stampID,
				models.NewUpdateAccountTypeInput().WithName("Renamed"))
		}},
		{"accountTypes.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newAccountTypesFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), resOrg, resLedger, stampID)
		}},
		{"metadataIndexes.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newMetadataIndexesFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), "account",
				models.NewCreateMetadataIndexInput("customer_id").WithUnique(true))
		}},
		{"metadataIndexes.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newMetadataIndexesFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), "account", "customer_id")
		}},
		{"operationRoutes.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newOperationRoutesFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), resOrg, resLedger,
				models.NewCreateOperationRouteInput("Cashin", "cash-in route", "source"))
		}},
		{"operationRoutes.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newOperationRoutesFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), resOrg, resLedger, stampID,
				models.NewUpdateOperationRouteInput().WithTitle("Renamed"))
		}},
		{"operationRoutes.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newOperationRoutesFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), resOrg, resLedger, stampID)
		}},
		{"transactionRoutes.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newTransactionRoutesFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), resOrg, resLedger,
				models.NewCreateTransactionRouteInput("Settlement", "settlement route", []string{"66666666-6666-6666-6666-666666666666"}))
		}},
		{"transactionRoutes.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newTransactionRoutesFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), resOrg, resLedger, stampID,
				models.NewUpdateTransactionRouteInput().WithTitle("Renamed"))
		}},
		{"transactionRoutes.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newTransactionRoutesFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), resOrg, resLedger, stampID)
		}},
		{"holders.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newHoldersFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), resOrg,
				models.NewCreateHolderInput(models.HolderTypeNaturalPerson, "Alice", "123"))
		}},
		{"holders.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newHoldersFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), resOrg, stampID,
				models.NewUpdateHolderInput().WithName("Renamed"))
		}},
		{"holders.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newHoldersFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), resOrg, stampID)
		}},
		{"assetRates.CreateOrUpdate", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newAssetRatesFacade(newTestLedgerClient(t, srv), gate).CreateOrUpdateAssetRate(context.Background(), resOrg, resLedger,
				models.NewCreateAssetRateInput("USD", "BRL", 525).WithScale(2))
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertGatedStamp(t, tc.name, tc.fire)
		})
	}
}

// TestLedgerResourceExplicitKeyWins proves the explicit-key override on a
// representative resource write: gate off + sdkctx.WithIdempotencyKey → the key
// still stamps (the gate never touches an explicit/ctx key).
func TestLedgerResourceExplicitKeyWins(t *testing.T) {
	srv, key := idempotencyCaptureServer()
	defer srv.Close()

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "explicit-key")
	_, _ = newAccountsFacade(newTestLedgerClient(t, srv), false).Create(ctx, resOrg, resLedger,
		&models.CreateAccountInput{Name: "Checking", AssetCode: "USD", Type: "deposit"})

	if got := key(); got != "explicit-key" {
		t.Fatalf("gate off + ctx key: got %q, want explicit-key", got)
	}
}

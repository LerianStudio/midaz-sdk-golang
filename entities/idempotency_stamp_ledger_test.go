// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
)

const stampID = "44444444-4444-4444-4444-444444444444"

// idempotencyCaptureServer returns a server that records the X-Idempotency
// header of the (single) request it serves and always answers 200 {}. The
// header is captured BEFORE the response, so callers may ignore the facade's
// return (a decode mismatch does not affect the assertion). Mutex-guarded so
// the read is race-clean.
func idempotencyCaptureServer() (*httptest.Server, func() string) {
	var (
		mu  sync.Mutex
		key string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		key = r.Header.Get(idempotencyHeader)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))

	return srv, func() string {
		mu.Lock()
		defer mu.Unlock()

		return key
	}
}

// TestLedgerWritesStampIdempotency proves 5.2.4: every wired ledger write op
// stamps an auto-generated X-Idempotency header (gate on, the default). One row
// per stamped op — a missing wire on any op fails here.
func TestLedgerWritesStampIdempotency(t *testing.T) {
	cases := []struct {
		name string
		fire func(t *testing.T, srv *httptest.Server)
	}{
		{"encryption.Provision", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestEncryptionFacade(t, srv).Provision(context.Background(), encryptionOrgID,
				models.NewProvisionEncryptionInput("svc-account", "p"))
		}},
		{"instruments.Create", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestInstrumentsFacade(t, srv).Create(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID,
				models.NewCreateInstrumentInput("CHECKING").WithDocument("DOC-1"))
		}},
		{"instruments.Update", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestInstrumentsFacade(t, srv).Update(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, stampID,
				models.NewUpdateInstrumentInput().WithDocument("DOC-9"))
		}},
		{"instruments.Delete", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_ = newTestInstrumentsFacade(t, srv).Delete(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, stampID)
		}},
		{"composition.CreateHolderAccount", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestCompositionFacade(t, srv).CreateHolderAccount(context.Background(), compositionFacadeOrgID, compositionFacadeLedgerID, compositionFacadeHolderID,
				&models.CreateHolderAccountInput{Name: "Ops Cash", AssetCode: "USD", Type: "deposit"})
		}},
		{"feePackages.Create", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestFeePackagesFacade(t, srv).Create(context.Background(), feePackagesOrgID,
				models.NewCreatePackageInput("Std", feePackagesLedgerID, "100.00", "1000.00", map[string]models.Fee{"admin": validFee()}).WithEnable(true))
		}},
		{"feePackages.Update", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestFeePackagesFacade(t, srv).Update(context.Background(), feePackagesOrgID, stampID,
				models.NewUpdatePackageInput().WithMaxAmount("5000.00"))
		}},
		{"feePackages.Delete", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_ = newTestFeePackagesFacade(t, srv).Delete(context.Background(), feePackagesOrgID, stampID)
		}},
		{"billingPackages.Create", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestBillingPackagesFacade(t, srv).Create(context.Background(), billingPkgOrgID,
				models.NewCreateVolumeBillingPackageInput("Vol", billingPkgLedgerID, "BRL", "@d", "@c").
					WithEventFilter("route-1", "APPROVED").
					WithPricingModel("tiered").
					WithPricingTiers(models.BillingPricingTier{MinQuantity: 0, UnitPrice: "1.50"}).
					WithEnable(true))
		}},
		{"billingPackages.Update", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestBillingPackagesFacade(t, srv).Update(context.Background(), billingPkgOrgID, stampID,
				models.NewUpdateBillingPackageInput().WithLabel("Renamed"))
		}},
		{"billingPackages.Delete", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_ = newTestBillingPackagesFacade(t, srv).Delete(context.Background(), billingPkgOrgID, stampID)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, key := idempotencyCaptureServer()
			defer srv.Close()

			tc.fire(t, srv)

			if key() == "" {
				t.Fatalf("%s: no X-Idempotency stamped (gate on, auto-gen expected)", tc.name)
			}
		})
	}
}

// TestLedgerWriteIdempotencyGate proves 5.2.4 gate semantics on a representative
// ledger write (instruments.Create): gate on → auto-gen; gate off → suppressed;
// gate off + explicit ctx key → the explicit key still stamps (the gate never
// touches an explicit/ctx key).
func TestLedgerWriteIdempotencyGate(t *testing.T) {
	input := models.NewCreateInstrumentInput("CHECKING").WithDocument("DOC-1")

	create := func(gate bool, ctx context.Context) string {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		f := newInstrumentsFacade(newTestLedgerClient(t, srv), gate)
		_, _ = f.Create(ctx, instrumentsFacadeOrgID, instrumentsFacadeHolderID, input)

		return key()
	}

	if got := create(true, context.Background()); got == "" {
		t.Fatal("gate on: want auto-generated X-Idempotency")
	}

	if got := create(false, context.Background()); got != "" {
		t.Fatalf("gate off: want no auto-gen, got %q", got)
	}

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "explicit-key")
	if got := create(false, ctx); got != "explicit-key" {
		t.Fatalf("gate off + ctx key: got %q, want explicit-key", got)
	}
}

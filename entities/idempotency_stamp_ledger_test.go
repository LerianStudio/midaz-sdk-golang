// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/sdkctx"
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

// assertGatedStamp fires an auto-gen write twice: gate ON must stamp an
// X-Idempotency header, gate OFF (no ctx key) must stay header-free. This is
// the shared check for every parametrized stamp table — it catches both a
// missing wire and a hardcoded gate (a literal true instead of
// f.enableIdempotency).
func assertGatedStamp(t *testing.T, name string, fire func(t *testing.T, srv *httptest.Server, gate bool)) {
	t.Helper()

	srvOn, keyOn := idempotencyCaptureServer()
	defer srvOn.Close()

	fire(t, srvOn, true)

	if keyOn() == "" {
		t.Fatalf("%s gate on: no X-Idempotency (auto-gen expected)", name)
	}

	srvOff, keyOff := idempotencyCaptureServer()
	defer srvOff.Close()

	fire(t, srvOff, false)

	if got := keyOff(); got != "" {
		t.Fatalf("%s gate off: X-Idempotency=%q, want headerless", name, got)
	}
}

// TestLedgerWritesStampIdempotency proves 5.2.4 + FW2: every wired ledger write
// op stamps an auto-generated X-Idempotency header with the gate ON, and emits
// nothing (no ctx key) with the gate OFF. Facades are built with the gate under
// test, not the always-on helper.
func TestLedgerWritesStampIdempotency(t *testing.T) {
	cases := []struct {
		name string
		fire func(t *testing.T, srv *httptest.Server, gate bool)
	}{
		{"encryption.Provision", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newEncryptionFacade(newTestLedgerClient(t, srv), gate).Provision(context.Background(), encryptionOrgID,
				models.NewProvisionEncryptionInput("svc-account", "p"))
		}},
		{"instruments.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newInstrumentsFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID,
				models.NewCreateInstrumentInput("CHECKING").WithDocument("DOC-1"))
		}},
		{"instruments.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newInstrumentsFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, stampID,
				models.NewUpdateInstrumentInput().WithDocument("DOC-9"))
		}},
		{"instruments.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newInstrumentsFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, stampID)
		}},
		{"instruments.DeleteRelatedParty", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newInstrumentsFacade(newTestLedgerClient(t, srv), gate).DeleteRelatedParty(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, stampID, stampID)
		}},
		{"composition.CreateHolderAccount", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newCompositionFacade(newTestLedgerClient(t, srv), gate).CreateHolderAccount(context.Background(), compositionFacadeOrgID, compositionFacadeLedgerID, compositionFacadeHolderID,
				&models.CreateHolderAccountInput{Name: "Ops Cash", AssetCode: "USD", Type: "deposit"})
		}},
		{"feePackages.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newFeePackagesFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), feePackagesOrgID,
				models.NewCreatePackageInput("Std", feePackagesLedgerID, "100.00", "1000.00", map[string]models.Fee{"admin": validFee()}).WithEnable(true))
		}},
		{"feePackages.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newFeePackagesFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), feePackagesOrgID, stampID,
				models.NewUpdatePackageInput().WithMaxAmount("5000.00"))
		}},
		{"feePackages.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newFeePackagesFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), feePackagesOrgID, stampID)
		}},
		{"billingPackages.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newBillingPackagesFacade(newTestLedgerClient(t, srv), gate).Create(context.Background(), billingPkgOrgID,
				models.NewCreateVolumeBillingPackageInput("Vol", billingPkgLedgerID, "BRL", "@d", "@c").
					WithEventFilter("route-1", "APPROVED").
					WithPricingModel("tiered").
					WithPricingTiers(models.BillingPricingTier{MinQuantity: 0, UnitPrice: "1.50"}).
					WithEnable(true))
		}},
		{"billingPackages.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newBillingPackagesFacade(newTestLedgerClient(t, srv), gate).Update(context.Background(), billingPkgOrgID, stampID,
				models.NewUpdateBillingPackageInput().WithLabel("Renamed"))
		}},
		{"billingPackages.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newBillingPackagesFacade(newTestLedgerClient(t, srv), gate).Delete(context.Background(), billingPkgOrgID, stampID)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertGatedStamp(t, tc.name, tc.fire)
		})
	}
}

// TestLedgerWriteIdempotencyGate proves the explicit-key override on a
// representative ledger write: gate off + sdkctx.WithIdempotencyKey → the key
// still stamps (the gate never touches an explicit/ctx key). On/off auto-gen is
// covered by the parametrized table above.
func TestLedgerWriteIdempotencyGate(t *testing.T) {
	input := models.NewCreateInstrumentInput("CHECKING").WithDocument("DOC-1")

	srv, key := idempotencyCaptureServer()
	defer srv.Close()

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "explicit-key")
	_, _ = newInstrumentsFacade(newTestLedgerClient(t, srv), false).Create(ctx, instrumentsFacadeOrgID, instrumentsFacadeHolderID, input)

	if got := key(); got != "explicit-key" {
		t.Fatalf("gate off + ctx key: got %q, want explicit-key", got)
	}
}

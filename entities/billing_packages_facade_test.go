// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
)

const (
	billingPkgOrgID    = "11111111-1111-1111-1111-111111111111"
	billingPkgLedgerID = "22222222-2222-2222-2222-222222222222"
)

func billingPkgBase() string {
	return "/v2/organizations/" + billingPkgOrgID + "/ledgers/" + billingPkgLedgerID + "/billing-packages"
}

// TestBillingPackagesFacade_ListAndPaginate exercises PAGE-mode pagination: the
// loop advances Page while HasMore() is true (Total arithmetic) and must NOT stop
// after page 1 because the page-mode envelope emits no next_cursor. A cursor-style
// "stop on empty cursor" would silently drop page 2.
func TestBillingPackagesFacade_ListAndPaginate(t *testing.T) {
	page1 := `{"items":[{"id":"aaaa","label":"P1","type":"volume","ledgerId":"led","feeAmount":"1.00"}],"limit":1,"page":1,"total":2}`
	page2 := `{"items":[{"id":"bbbb","label":"P2","type":"maintenance","ledgerId":"led","feeAmount":"2.00"}],"limit":1,"page":2,"total":2}`

	var gotPath string
	var seenPages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		page := r.URL.Query().Get("page")
		seenPages = append(seenPages, page)
		w.Header().Set("Content-Type", "application/json")
		if page == "2" {
			_, _ = w.Write([]byte(page2))
		} else {
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer srv.Close()

	facade := newTestBillingPackagesFacade(t, srv)

	first, err := facade.List(context.Background(), billingPkgOrgID, billingPkgLedgerID, models.BillingPackagesListOpts{PageListOpts: models.PageListOpts{Limit: 1}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotPath != billingPkgBase() {
		t.Fatalf("path = %q, want %q", gotPath, billingPkgBase())
	}
	if len(first.Items) != 1 || first.Items[0].Label != "P1" {
		t.Fatalf("List page 1 = %+v", first.Items)
	}

	seenPages = nil // discard the standalone List call above; assert only the iterator's page walk.
	all, err := CollectAll(facade.ListAll(context.Background(), billingPkgOrgID, billingPkgLedgerID, models.BillingPackagesListOpts{PageListOpts: models.PageListOpts{Limit: 1}}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].Label != "P1" || all[1].Label != "P2" {
		t.Fatalf("ListAll = %+v (want 2 items across 2 pages; a page-1-only stop is data loss)", all)
	}
	if len(seenPages) != 2 || seenPages[0] != "1" || seenPages[1] != "2" {
		t.Fatalf("seenPages = %v, want page walk [1 2] (a page-1-only stop is data loss)", seenPages)
	}
}

// TestBillingPackagesFacade_StringAmountRoundTrip is the money-adjacent third-rail
// guard: feeAmount, pricing-tier unitPrice, and discount-tier percentage must ride
// the wire as JSON strings and survive unchanged. 0.333333333333333333 is
// unrepresentable in float64 and would visibly drift through a float hop.
func TestBillingPackagesFacade_StringAmountRoundTrip(t *testing.T) {
	const precise = "0.333333333333333333"
	resp := `{"id":"cccc","label":"Precise","type":"volume","ledgerId":"led",` +
		`"tiers":[{"minQuantity":0,"unitPrice":"` + precise + `"}],` +
		`"discountTiers":[{"minQuantity":100,"discountPercentage":"` + precise + `"}],` +
		`"feeAmount":"` + precise + `"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	bp, err := newTestBillingPackagesFacade(t, srv).Get(context.Background(), billingPkgOrgID, billingPkgLedgerID, "cccc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if bp.FeeAmount == nil || *bp.FeeAmount != precise {
		t.Fatalf("FeeAmount = %v, want %q (no float hop)", bp.FeeAmount, precise)
	}
	if bp.Tiers == nil || (*bp.Tiers)[0].UnitPrice != precise {
		t.Fatalf("Tiers[0].UnitPrice = %+v, want %q", bp.Tiers, precise)
	}
	if bp.DiscountTiers == nil || (*bp.DiscountTiers)[0].DiscountPercentage != precise {
		t.Fatalf("DiscountTiers[0].DiscountPercentage = %+v, want %q", bp.DiscountTiers, precise)
	}
}

// TestBillingPackagesFacade_CRUD round-trips Create/Get/Update/Delete. Create must
// succeed on the server's 201 (spec says 200) — routing through RAW WithBody +
// isSuccess(2xx) is what makes that hold.
//
//nolint:revive // cognitive-complexity: four CRUD subtests each with their own server closure; matches the repo's per-test convention.
func TestBillingPackagesFacade_CRUD(t *testing.T) {
	const id = "33333333-3333-3333-3333-333333333333"

	t.Run("create-201", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated) // server returns 201, spec says 200
			_, _ = w.Write([]byte(`{"id":"` + id + `","label":"Vol","type":"volume","ledgerId":"led"}`))
		}))
		defer srv.Close()

		input := models.NewCreateVolumeBillingPackageInput("Vol", "BRL", "@d", "@c").
			WithEventFilter("route-1", "APPROVED").
			WithPricingModel("tiered").
			WithPricingTiers(models.BillingPricingTier{MinQuantity: 0, UnitPrice: "1.50"}).
			WithEnable(true)

		bp, err := newTestBillingPackagesFacade(t, srv).Create(context.Background(), billingPkgOrgID, billingPkgLedgerID, input)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m != http.MethodPost || p != billingPkgBase() {
			t.Fatalf("create req = %s %s", m, p)
		}
		if !strings.Contains(body, `"label":"Vol"`) || !strings.Contains(body, `"unitPrice":"1.50"`) {
			t.Fatalf("body = %q, want create input wire shape", body)
		}
		requireNoLedgerIDInRequestBody(t, body)
		if bp.ID != id || bp.Label != "Vol" {
			t.Fatalf("Create returned %+v", bp)
		}
	})

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","label":"Vol"}`))
		}))
		defer srv.Close()

		bp, err := newTestBillingPackagesFacade(t, srv).Get(context.Background(), billingPkgOrgID, billingPkgLedgerID, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if m != http.MethodGet || p != billingPkgBase()+"/"+id {
			t.Fatalf("get req = %s %s", m, p)
		}
		if bp.ID != id {
			t.Fatalf("Get returned %+v", bp)
		}
	})

	t.Run("update", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","label":"Renamed"}`))
		}))
		defer srv.Close()

		bp, err := newTestBillingPackagesFacade(t, srv).Update(context.Background(), billingPkgOrgID, billingPkgLedgerID, id, models.NewUpdateBillingPackageInput().WithLabel("Renamed"))
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if m != http.MethodPatch || p != billingPkgBase()+"/"+id {
			t.Fatalf("update req = %s %s", m, p)
		}
		if !strings.Contains(body, `"label":"Renamed"`) {
			t.Fatalf("body = %q, want marshaled UpdateBillingPackageInput", body)
		}
		if strings.Contains(body, "description") || strings.Contains(body, "enable") {
			t.Fatalf("body = %q, unset fields must be omitted", body)
		}
		if bp.Label != "Renamed" {
			t.Fatalf("Update returned %+v", bp)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		if err := newTestBillingPackagesFacade(t, srv).Delete(context.Background(), billingPkgOrgID, billingPkgLedgerID, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if m != http.MethodDelete || p != billingPkgBase()+"/"+id {
			t.Fatalf("delete req = %s %s", m, p)
		}
	})
}

// TestBillingPackagesFacade_Filters proves the native type filter slot reaches
// the wire, and that the ledger now travels in the PATH rather than as a
// ledgerId query filter (the v2 route is ledger-scoped).
func TestBillingPackagesFacade_Filters(t *testing.T) {
	var q url.Values

	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":3,"total":0}`))
	}))
	defer srv.Close()

	_, err := newTestBillingPackagesFacade(t, srv).List(context.Background(), billingPkgOrgID, billingPkgLedgerID, models.BillingPackagesListOpts{
		PageListOpts: models.PageListOpts{Limit: 3},
		Filters:      models.BillingPackagesFilters{Type: "volume"},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if gotPath != billingPkgBase() {
		t.Fatalf("path = %q, want %q", gotPath, billingPkgBase())
	}

	if got := q.Get("ledgerId"); got != "" {
		t.Fatalf("ledgerId query = %q, want empty (ledger is a path segment on v2)", got)
	}

	if got := q.Get("type"); got != "volume" {
		t.Fatalf("type = %q, want volume", got)
	}

	if got := q.Get("limit"); got != "3" {
		t.Fatalf("limit = %q, want 3", got)
	}
}

// TestBillingPackagesFacade_ErrorDecodes asserts RFC 9457 decode with request-ID.
func TestBillingPackagesFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-bill-409")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0084","title":"Conflict","status":409}`))
	}))
	defer srv.Close()

	_, err := newTestBillingPackagesFacade(t, srv).Get(context.Background(), billingPkgOrgID, billingPkgLedgerID, "33333333-3333-3333-3333-333333333333")
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0084" || sdkErr.RequestID != "req-bill-409" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestBillingPackagesFacade_WriteReplaySafe is the money-path 401-replay guard:
// the create body must survive a token-refresh replay intact.
func TestBillingPackagesFacade_WriteReplaySafe(t *testing.T) {
	var attempts int
	var replayed string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"LEDGER-0001","title":"Unauthorized","status":401}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		replayed = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"33333333-3333-3333-3333-333333333333","label":"Vol"}`))
	}))
	defer srv.Close()

	input := models.NewCreateVolumeBillingPackageInput("Vol", "BRL", "@d", "@c").
		WithEventFilter("route-1", "APPROVED").
		WithPricingModel("tiered").
		WithPricingTiers(models.BillingPricingTier{MinQuantity: 0, UnitPrice: "1.50"}).
		WithEnable(true)

	_, err := newTestBillingPackagesFacade(t, srv).Create(context.Background(), billingPkgOrgID, billingPkgLedgerID, input)
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"label":"Vol"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
	// The replayed body must satisfy the same contract as the first attempt.
	requireNoLedgerIDInRequestBody(t, replayed)
}

// TestBillingPackagesFacade_Validation rejects bad input before any request leaves.
func TestBillingPackagesFacade_Validation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("no request should reach the server on validation failure")
	}))
	defer srv.Close()

	facade := newTestBillingPackagesFacade(t, srv)

	if _, err := facade.Create(context.Background(), billingPkgOrgID, billingPkgLedgerID, &models.CreateBillingPackageInput{}); err == nil {
		t.Fatal("Create with empty input must fail validation")
	}
	if _, err := facade.Update(context.Background(), billingPkgOrgID, billingPkgLedgerID, "id", models.NewUpdateBillingPackageInput()); err == nil {
		t.Fatal("Update with empty payload must fail validation")
	}
}

// TestBillingPackagesFacade_LedgerIsPathOnly is the midaz v4 contract rail on the
// billing-package create: the ledger scopes the request through the URL alone and
// the body must carry NO ledgerId. The server removed the field and closed the
// schema (additionalProperties: false), so a body still carrying it is rejected
// with 400 — no billing package (which prices money movement) could be registered
// at all.
func TestBillingPackagesFacade_LedgerIsPathOnly(t *testing.T) {
	var gotPath, body string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"aaaa","label":"Vol","type":"volume"}`))
	}))
	defer srv.Close()

	input := models.NewCreateVolumeBillingPackageInput("Vol", "BRL", "@d", "@c").
		WithEventFilter("route-1", "APPROVED").
		WithPricingModel("tiered").
		WithPricingTiers(models.BillingPricingTier{MinQuantity: 0, UnitPrice: "1.50"}).
		WithEnable(true)

	if _, err := newTestBillingPackagesFacade(t, srv).Create(context.Background(), billingPkgOrgID, billingPkgLedgerID, input); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if gotPath != billingPkgBase() {
		t.Fatalf("path = %q, want %q (the ledger scopes the request through the URL)", gotPath, billingPkgBase())
	}

	requireNoLedgerIDInRequestBody(t, body)
}

func newTestBillingPackagesFacade(t *testing.T, srv *httptest.Server) *billingPackagesFacade {
	t.Helper()
	return newBillingPackagesFacade(newTestLedgerClient(t, srv), true)
}

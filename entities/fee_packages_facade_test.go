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

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

const (
	feePackagesOrgID    = "11111111-1111-1111-1111-111111111111"
	feePackagesLedgerID = "22222222-2222-2222-2222-222222222222"
)

func feePackagesBase() string {
	return "/v1/organizations/" + feePackagesOrgID + "/packages"
}

// TestFeePackagesFacade_ListAndPaginate exercises PAGE-mode pagination: the loop
// must advance Page and keep fetching while HasMore() is true (Total arithmetic),
// and must NOT stop after page 1 just because the page-mode envelope emits no
// next_cursor. A cursor-style "stop on empty cursor" would lose page 2 entirely.
func TestFeePackagesFacade_ListAndPaginate(t *testing.T) {
	// total=2, limit=1 → page 1 has_more (1*1 < 2), page 2 terminal (2*1 !< 2).
	page1 := `{"items":[{"id":"aaaa","feeGroupLabel":"P1","minimumAmount":"100.00","maximumAmount":"1000.00"}],"limit":1,"page":1,"total":2}`
	page2 := `{"items":[{"id":"bbbb","feeGroupLabel":"P2","minimumAmount":"200.00","maximumAmount":"2000.00"}],"limit":1,"page":2,"total":2}`

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

	facade := newTestFeePackagesFacade(t, srv)

	first, err := facade.List(context.Background(), feePackagesOrgID, models.PackagesListOpts{PageListOpts: models.PageListOpts{Limit: 1}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotPath != feePackagesBase() {
		t.Fatalf("path = %q, want %q", gotPath, feePackagesBase())
	}
	if len(first.Items) != 1 || first.Items[0].FeeGroupLabel != "P1" {
		t.Fatalf("List page 1 = %+v", first.Items)
	}

	all, err := CollectAll(facade.All(context.Background(), feePackagesOrgID, models.PackagesListOpts{PageListOpts: models.PageListOpts{Limit: 1}}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].FeeGroupLabel != "P1" || all[1].FeeGroupLabel != "P2" {
		t.Fatalf("All = %+v (want 2 items across 2 pages; a page-1-only stop is data loss)", all)
	}
}

// TestFeePackagesFacade_StringAmountRoundTrip is the money-adjacent third-rail
// guard: minimum/maximum amounts and fee calculation values must ride the wire
// as JSON strings and survive unchanged — no float hop, no reformat. The value
// 0.333333333333333333 is unrepresentable in float64 and would visibly drift.
func TestFeePackagesFacade_StringAmountRoundTrip(t *testing.T) {
	const precise = "0.333333333333333333"
	resp := `{"id":"cccc","feeGroupLabel":"Precise","minimumAmount":"` + precise + `","maximumAmount":"1000.00",` +
		`"fees":{"admin":{"calculationModel":{"applicationRule":"flat","calculations":[{"type":"percentage","value":"` + precise + `"}]},"creditAccount":"@fees","feeLabel":"Admin","referenceAmount":"originalAmount"}}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	pkg, err := newTestFeePackagesFacade(t, srv).Get(context.Background(), feePackagesOrgID, "cccc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if pkg.MinimumAmount != precise {
		t.Fatalf("MinimumAmount = %q, want %q (no float hop)", pkg.MinimumAmount, precise)
	}
	fee, ok := pkg.Fees["admin"]
	if !ok {
		t.Fatalf("fees[admin] missing: %+v", pkg.Fees)
	}
	if len(fee.CalculationModel.Calculations) != 1 || fee.CalculationModel.Calculations[0].Value != precise {
		t.Fatalf("calculation value = %+v, want %q", fee.CalculationModel.Calculations, precise)
	}
}

// TestFeePackagesFacade_CRUD round-trips Create/Get/Update/Delete.
//
//nolint:revive // cognitive-complexity: four CRUD subtests each with their own server closure; matches the repo's per-test convention.
func TestFeePackagesFacade_CRUD(t *testing.T) {
	const id = "33333333-3333-3333-3333-333333333333"

	t.Run("create", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"` + id + `","feeGroupLabel":"Std","minimumAmount":"100.00","maximumAmount":"1000.00"}`))
		}))
		defer srv.Close()

		enable := true
		input := models.NewCreatePackageInput("Std", feePackagesLedgerID, "100.00", "1000.00", map[string]models.Fee{
			"admin": validFee(),
		}).WithEnable(enable)

		pkg, err := newTestFeePackagesFacade(t, srv).Create(context.Background(), feePackagesOrgID, input)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m != http.MethodPost || p != feePackagesBase() {
			t.Fatalf("create req = %s %s", m, p)
		}
		if !strings.Contains(body, `"feeGroupLabel":"Std"`) || !strings.Contains(body, `"minimumAmount":"100.00"`) {
			t.Fatalf("body = %q, want flat CreatePackageInput wire shape", body)
		}
		if pkg.ID != id || pkg.FeeGroupLabel != "Std" {
			t.Fatalf("Create returned %+v", pkg)
		}
	})

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","feeGroupLabel":"Std"}`))
		}))
		defer srv.Close()

		pkg, err := newTestFeePackagesFacade(t, srv).Get(context.Background(), feePackagesOrgID, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if m != http.MethodGet || p != feePackagesBase()+"/"+id {
			t.Fatalf("get req = %s %s", m, p)
		}
		if pkg.ID != id {
			t.Fatalf("Get returned %+v", pkg)
		}
	})

	t.Run("update", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","feeGroupLabel":"Std","maximumAmount":"5000.00"}`))
		}))
		defer srv.Close()

		pkg, err := newTestFeePackagesFacade(t, srv).Update(context.Background(), feePackagesOrgID, id, models.NewUpdatePackageInput().WithMaxAmount("5000.00"))
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if m != http.MethodPatch || p != feePackagesBase()+"/"+id {
			t.Fatalf("update req = %s %s", m, p)
		}
		if !strings.Contains(body, `"maximumAmount":"5000.00"`) {
			t.Fatalf("body = %q, want marshaled UpdatePackageInput", body)
		}
		if strings.Contains(body, "minimumAmount") {
			t.Fatalf("body = %q, unset fields must be omitted", body)
		}
		if pkg.MaximumAmount != "5000.00" {
			t.Fatalf("Update returned %+v", pkg)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		if err := newTestFeePackagesFacade(t, srv).Delete(context.Background(), feePackagesOrgID, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if m != http.MethodDelete || p != feePackagesBase()+"/"+id {
			t.Fatalf("delete req = %s %s", m, p)
		}
	})
}

// TestFeePackagesFacade_Filters proves the native filter slots reach the wire.
func TestFeePackagesFacade_Filters(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":3,"total":0}`))
	}))
	defer srv.Close()

	enable := true
	_, err := newTestFeePackagesFacade(t, srv).List(context.Background(), feePackagesOrgID, models.PackagesListOpts{
		PageListOpts: models.PageListOpts{Limit: 3},
		Filters: models.PackagesFilters{
			SegmentID:        "seg-1",
			LedgerID:         feePackagesLedgerID,
			TransactionRoute: "route-1",
			Enable:           &enable,
		},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := q.Get("segmentId"); got != "seg-1" {
		t.Fatalf("segmentId = %q, want seg-1", got)
	}
	if got := q.Get("ledgerId"); got != feePackagesLedgerID {
		t.Fatalf("ledgerId = %q, want %q", got, feePackagesLedgerID)
	}
	if got := q.Get("transactionRoute"); got != "route-1" {
		t.Fatalf("transactionRoute = %q, want route-1", got)
	}
	if got := q.Get("enable"); got != "true" {
		t.Fatalf("enable = %q, want true", got)
	}
	if got := q.Get("limit"); got != "3" {
		t.Fatalf("limit = %q, want 3", got)
	}
}

// TestFeePackagesFacade_ErrorDecodes asserts RFC 9457 decode with request-ID.
func TestFeePackagesFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-fee-409")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0084","title":"Conflict","status":409}`))
	}))
	defer srv.Close()

	_, err := newTestFeePackagesFacade(t, srv).Get(context.Background(), feePackagesOrgID, "33333333-3333-3333-3333-333333333333")
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0084" || sdkErr.RequestID != "req-fee-409" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestFeePackagesFacade_WriteReplaySafe is the money-path 401-replay guard: the
// create body must survive a token-refresh replay intact.
func TestFeePackagesFacade_WriteReplaySafe(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"33333333-3333-3333-3333-333333333333","feeGroupLabel":"Std"}`))
	}))
	defer srv.Close()

	input := models.NewCreatePackageInput("Std", feePackagesLedgerID, "100.00", "1000.00", map[string]models.Fee{
		"admin": validFee(),
	}).WithEnable(true)

	_, err := newTestFeePackagesFacade(t, srv).Create(context.Background(), feePackagesOrgID, input)
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"feeGroupLabel":"Std"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

// TestFeePackagesFacade_Validation rejects bad input before any request leaves.
func TestFeePackagesFacade_Validation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("no request should reach the server on validation failure")
	}))
	defer srv.Close()

	facade := newTestFeePackagesFacade(t, srv)

	// Missing required fields + empty fees map.
	_, err := facade.Create(context.Background(), feePackagesOrgID, &models.CreatePackageInput{})
	if err == nil {
		t.Fatal("Create with empty input must fail validation")
	}

	// Empty PATCH is a no-op round trip.
	_, err = facade.Update(context.Background(), feePackagesOrgID, "id", models.NewUpdatePackageInput())
	if err == nil {
		t.Fatal("Update with empty payload must fail validation")
	}
}

// TestFeePackagesFacade_UpdateMinAmountStringRail is the money-string rail on the
// UPDATE write path: a precise minimum-amount set via WithMinAmount must ride the
// PATCH body as the exact JSON string, no float hop, no reformat.
// 0.333333333333333333 is unrepresentable in float64 and would visibly drift.
func TestFeePackagesFacade_UpdateMinAmountStringRail(t *testing.T) {
	const precise = "0.333333333333333333"
	const id = "33333333-3333-3333-3333-333333333333"

	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + id + `","feeGroupLabel":"Std","minimumAmount":"` + precise + `"}`))
	}))
	defer srv.Close()

	_, err := newTestFeePackagesFacade(t, srv).Update(context.Background(), feePackagesOrgID, id, models.NewUpdatePackageInput().WithMinAmount(precise))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !strings.Contains(body, `"minimumAmount":"`+precise+`"`) {
		t.Fatalf("PATCH body = %q, want exact minimumAmount string %q (no float hop)", body, precise)
	}
}

// validFee returns an inner Fee that satisfies CreatePackageInput.Validate's dive
// (server package.go tags: feeLabel/calculationModel/referenceAmount/creditAccount
// required, isDeductibleFrom non-nil, calculation type/value present).
func validFee() models.Fee {
	deductible := false
	return models.Fee{
		CreditAccount:    "@fees",
		FeeLabel:         "Admin",
		ReferenceAmount:  "originalAmount",
		IsDeductibleFrom: &deductible,
		CalculationModel: models.FeeCalculationModel{
			ApplicationRule: "flatFee",
			Calculations:    []models.Calculation{{Type: "flat", Value: "10.00"}},
		},
	}
}

func newTestFeePackagesFacade(t *testing.T, srv *httptest.Server) *feePackagesFacade {
	t.Helper()
	return newFeePackagesFacade(newTestLedgerClient(t, srv))
}

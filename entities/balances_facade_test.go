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
	"github.com/shopspring/decimal"
)

const (
	balancesOrgID     = "11111111-1111-1111-1111-111111111111"
	balancesLedgerID  = "22222222-2222-2222-2222-222222222222"
	balancesAcctID    = "33333333-3333-3333-3333-333333333333"
	balancesBalanceID = "44444444-4444-4444-4444-444444444444"
	balancesInstant   = "2026-01-02 03:04:05"
)

func balancesLedgerScope() string {
	return "/v1/organizations/" + balancesOrgID + "/ledgers/" + balancesLedgerID
}

func newTestBalancesFacade(t *testing.T, srv *httptest.Server) *balancesFacade {
	t.Helper()
	return newBalancesFacade(newTestLedgerClient(t, srv), true)
}

// TestBalancesFacade_ListBalances pins the ledger-wide list: the cursor/sort/date
// fields reach the wire, "page" does not (the endpoint has no page slot), and the
// money amounts survive as decimals.
func TestBalancesFacade_ListBalances(t *testing.T) {
	var gotPath string
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"b-1","assetCode":"USD","available":"1500.00000001","onHold":"0.5"}],"limit":7}`))
	}))
	defer srv.Close()

	page, err := newTestBalancesFacade(t, srv).ListBalances(context.Background(), balancesOrgID, balancesLedgerID,
		models.BalancesListOpts{CursorListOpts: models.CursorListOpts{
			Limit:         7,
			Cursor:        "cur-1",
			SortDirection: models.SortDescending,
			StartDate:     "2026-01-01",
			EndDate:       "2026-01-31",
		}})
	if err != nil {
		t.Fatalf("ListBalances: %v", err)
	}

	if want := balancesLedgerScope() + "/balances"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}

	for key, want := range map[string]string{
		"limit": "7", "cursor": "cur-1", "sort_order": "desc",
		"start_date": "2026-01-01", "end_date": "2026-01-31",
	} {
		if got := gotQuery.Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}

	if got := gotQuery.Get("page"); got != "" {
		t.Fatalf("page = %q, want absent: this list is cursor-paginated", got)
	}

	// Money-path decode: full precision, no float rounding.
	if len(page.Items) != 1 {
		t.Fatalf("ListBalances items = %+v", page.Items)
	}

	if !page.Items[0].Available.Equal(decimal.RequireFromString("1500.00000001")) {
		t.Fatalf("available = %s, want 1500.00000001", page.Items[0].Available)
	}

	if !page.Items[0].OnHold.Equal(decimal.RequireFromString("0.5")) {
		t.Fatalf("onHold = %s, want 0.5", page.Items[0].OnHold)
	}
}

// TestBalancesFacade_ListBalancesAll_CursorTerminatesOnFullPage is the money-path
// infinite-loop guard. The terminal page comes back FULL (ItemCount == Limit) and
// carries a page field but NO next_cursor, which is exactly the shape where
// Pagination.HasMore() reports true. An iterator gated on HasMore() would reset
// the cursor to "" and re-request the first page forever; the request cap turns
// that into a fast failure instead of a hang.
func TestBalancesFacade_ListBalancesAll_CursorTerminatesOnFullPage(t *testing.T) {
	var cursors []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		if len(cursors) > 4 {
			t.Fatalf("infinite loop: iterator did not terminate, cursors=%v", cursors)
		}

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("cursor") == "cur-2" {
			_, _ = w.Write([]byte(`{"items":[{"id":"b-3","assetCode":"USD","available":"30"},{"id":"b-4","assetCode":"USD","available":"40"}],"limit":2,"page":1}`))
			return
		}

		_, _ = w.Write([]byte(`{"items":[{"id":"b-1","assetCode":"USD","available":"10"},{"id":"b-2","assetCode":"USD","available":"20"}],"limit":2,"page":1,"next_cursor":"cur-2"}`))
	}))
	defer srv.Close()

	all, err := CollectAll(newTestBalancesFacade(t, srv).ListBalancesAll(context.Background(), balancesOrgID, balancesLedgerID,
		models.BalancesListOpts{CursorListOpts: models.CursorListOpts{Limit: 2}}))
	if err != nil {
		t.Fatalf("ListBalancesAll: %v", err)
	}

	if len(all) != 4 || all[0].ID != "b-1" || all[3].ID != "b-4" {
		t.Fatalf("ListBalancesAll = %+v", all)
	}

	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "cur-2" {
		t.Fatalf("cursor chain = %v, want ['', 'cur-2']", cursors)
	}
}

// TestBalancesFacade_ListAccountBalances pins the account-scoped list path and
// chains its cursor pages.
func TestBalancesFacade_ListAccountBalances(t *testing.T) {
	var paths []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("cursor") == "cur-2" {
			_, _ = w.Write([]byte(`{"items":[{"id":"b-2","assetCode":"BRL","available":"20"}],"limit":1}`))
			return
		}

		_, _ = w.Write([]byte(`{"items":[{"id":"b-1","assetCode":"USD","available":"10"}],"limit":1,"next_cursor":"cur-2"}`))
	}))
	defer srv.Close()

	facade := newTestBalancesFacade(t, srv)
	opts := models.BalancesListOpts{CursorListOpts: models.CursorListOpts{Limit: 1}}

	first, err := facade.ListAccountBalances(context.Background(), balancesOrgID, balancesLedgerID, balancesAcctID, opts)
	if err != nil {
		t.Fatalf("ListAccountBalances: %v", err)
	}

	want := balancesLedgerScope() + "/accounts/" + balancesAcctID + "/balances"
	if paths[0] != want {
		t.Fatalf("path = %q, want %q", paths[0], want)
	}

	if len(first.Items) != 1 || first.Items[0].ID != "b-1" {
		t.Fatalf("page 1 = %+v", first.Items)
	}

	all, err := CollectAll(facade.ListAccountBalancesAll(context.Background(), balancesOrgID, balancesLedgerID, balancesAcctID, opts))
	if err != nil {
		t.Fatalf("ListAccountBalancesAll: %v", err)
	}

	if len(all) != 2 || all[0].ID != "b-1" || all[1].ID != "b-2" {
		t.Fatalf("ListAccountBalancesAll = %+v", all)
	}
}

// TestBalancesFacade_CRUD round-trips get/update/delete/create on their
// respective paths, asserting method and body on the wire.
//
//nolint:revive // cognitive-complexity: four CRUD subtests, each with its own httptest server closure and assertions; the complexity is the subtest count, not branching logic. Matches the repo's per-test convention.
func TestBalancesFacade_CRUD(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + balancesBalanceID + `","assetCode":"USD","available":"10.25"}`))
		}))
		defer srv.Close()

		b, err := newTestBalancesFacade(t, srv).GetBalance(context.Background(), balancesOrgID, balancesLedgerID, balancesBalanceID)
		if err != nil {
			t.Fatalf("GetBalance: %v", err)
		}
		if want := balancesLedgerScope() + "/balances/" + balancesBalanceID; m != http.MethodGet || p != want {
			t.Fatalf("get req = %s %s, want GET %s", m, p, want)
		}
		if !b.Available.Equal(decimal.RequireFromString("10.25")) {
			t.Fatalf("available = %s, want 10.25", b.Available)
		}
	})

	t.Run("update", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			body = string(raw)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + balancesBalanceID + `","assetCode":"USD","allowSending":false}`))
		}))
		defer srv.Close()

		allow := false
		b, err := newTestBalancesFacade(t, srv).UpdateBalance(context.Background(), balancesOrgID, balancesLedgerID, balancesBalanceID,
			&models.UpdateBalanceInput{AllowSending: &allow})
		if err != nil {
			t.Fatalf("UpdateBalance: %v", err)
		}
		if want := balancesLedgerScope() + "/balances/" + balancesBalanceID; m != http.MethodPatch || p != want {
			t.Fatalf("update req = %s %s, want PATCH %s", m, p, want)
		}
		if !strings.Contains(body, `"allowSending":false`) {
			t.Fatalf("body = %q, want marshaled UpdateBalanceInput", body)
		}
		if b.AllowSending {
			t.Fatalf("UpdateBalance returned %+v", b)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		if err := newTestBalancesFacade(t, srv).DeleteBalance(context.Background(), balancesOrgID, balancesLedgerID, balancesBalanceID); err != nil {
			t.Fatalf("DeleteBalance: %v", err)
		}
		if want := balancesLedgerScope() + "/balances/" + balancesBalanceID; m != http.MethodDelete || p != want {
			t.Fatalf("delete req = %s %s, want DELETE %s", m, p, want)
		}
	})

	t.Run("create", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			body = string(raw)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"` + balancesBalanceID + `","key":"asset-freeze","assetCode":"USD","available":"0"}`))
		}))
		defer srv.Close()

		b, err := newTestBalancesFacade(t, srv).CreateBalance(context.Background(), balancesOrgID, balancesLedgerID, balancesAcctID,
			&models.CreateBalanceInput{Key: "asset-freeze"})
		if err != nil {
			t.Fatalf("CreateBalance: %v", err)
		}
		if want := balancesLedgerScope() + "/accounts/" + balancesAcctID + "/balances"; m != http.MethodPost || p != want {
			t.Fatalf("create req = %s %s, want POST %s", m, p, want)
		}
		if !strings.Contains(body, `"key":"asset-freeze"`) {
			t.Fatalf("body = %q, want marshaled CreateBalanceInput", body)
		}
		if b.Key != "asset-freeze" {
			t.Fatalf("CreateBalance returned %+v", b)
		}
	})
}

// TestBalancesFacade_PointInTime pins both history endpoints: the path, the date
// query param, the response shape (one object vs a bare array) and the decimal
// decode.
//
//nolint:revive // cognitive-complexity: four independent subtests, each with its own httptest server closure and assertions; the complexity is the subtest count, not branching logic. Matches the repo's per-test convention.
func TestBalancesFacade_PointInTime(t *testing.T) {
	t.Run("one balance", func(t *testing.T) {
		var p, date string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, date = r.URL.Path, r.URL.Query().Get("date")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + balancesBalanceID + `","accountId":"` + balancesAcctID + `","assetCode":"USD","available":"12.34"}`))
		}))
		defer srv.Close()

		got, err := newTestBalancesFacade(t, srv).GetBalanceHistory(context.Background(), balancesOrgID, balancesLedgerID, balancesBalanceID, balancesInstant)
		if err != nil {
			t.Fatalf("GetBalanceHistory: %v", err)
		}
		if want := balancesLedgerScope() + "/balances/" + balancesBalanceID + "/history"; p != want {
			t.Fatalf("path = %q, want %q", p, want)
		}
		if date != balancesInstant {
			t.Fatalf("date = %q, want %q", date, balancesInstant)
		}
		if !got.Available.Equal(decimal.RequireFromString("12.34")) {
			t.Fatalf("available = %s, want 12.34", got.Available)
		}
	})

	t.Run("every account balance", func(t *testing.T) {
		var p, date string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, date = r.URL.Path, r.URL.Query().Get("date")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"` + balancesBalanceID + `","accountId":"` + balancesAcctID + `","assetCode":"USD","available":"5"}]`))
		}))
		defer srv.Close()

		got, err := newTestBalancesFacade(t, srv).GetAccountBalancesHistory(context.Background(), balancesOrgID, balancesLedgerID, balancesAcctID, balancesInstant)
		if err != nil {
			t.Fatalf("GetAccountBalancesHistory: %v", err)
		}
		if want := balancesLedgerScope() + "/accounts/" + balancesAcctID + "/balances/history"; p != want {
			t.Fatalf("path = %q, want %q", p, want)
		}
		if date != balancesInstant {
			t.Fatalf("date = %q, want %q", date, balancesInstant)
		}
		if len(got) != 1 || !got[0].Available.Equal(decimal.NewFromInt(5)) {
			t.Fatalf("GetAccountBalancesHistory = %+v", got)
		}
	})

	t.Run("date contract", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			t.Error("server must not be hit for an invalid date")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		facade := newTestBalancesFacade(t, srv)

		// A day with no time component: the server rejects it, so the SDK does too.
		if _, err := facade.GetBalanceHistory(context.Background(), balancesOrgID, balancesLedgerID, balancesBalanceID, "2026-01-02"); err == nil {
			t.Fatal("expected a date-only value to be rejected")
		}

		if _, err := facade.GetAccountBalancesHistory(context.Background(), balancesOrgID, balancesLedgerID, balancesAcctID, ""); err == nil {
			t.Fatal("expected an empty date to be rejected")
		}
	})

	t.Run("accepts the RFC3339 spelling of the same instant", func(t *testing.T) {
		var date string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			date = r.URL.Query().Get("date")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + balancesBalanceID + `","accountId":"` + balancesAcctID + `","assetCode":"USD","available":"1"}`))
		}))
		defer srv.Close()

		if _, err := newTestBalancesFacade(t, srv).GetBalanceHistory(context.Background(), balancesOrgID, balancesLedgerID, balancesBalanceID, "2026-01-02T03:04:05Z"); err != nil {
			t.Fatalf("GetBalanceHistory with an RFC3339 date: %v", err)
		}
		if date != "2026-01-02T03:04:05Z" {
			t.Fatalf("date = %q", date)
		}
	})
}

// TestBalancesFacade_AccountLookups pins the two one-shot lookups: the alias and
// external-code paths, and that each answers the caller's full balance set in a
// single request.
func TestBalancesFacade_AccountLookups(t *testing.T) {
	body := `{"items":[{"id":"b-1","assetCode":"USD","available":"10"}],"limit":10}`

	t.Run("by account alias", func(t *testing.T) {
		var p string
		var requests int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			p = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		defer srv.Close()

		facade := newTestBalancesFacade(t, srv)

		got, err := facade.ListBalancesByAccountAlias(context.Background(), balancesOrgID, balancesLedgerID, "@cash")
		if err != nil {
			t.Fatalf("ListBalancesByAccountAlias: %v", err)
		}
		if want := balancesLedgerScope() + "/accounts/alias/@cash/balances"; p != want {
			t.Fatalf("path = %q, want %q", p, want)
		}
		if len(got.Items) != 1 {
			t.Fatalf("items = %+v", got.Items)
		}
		if requests != 1 {
			t.Fatalf("requests = %d, want 1 (the endpoint answers in one shot)", requests)
		}
	})

	t.Run("by external code", func(t *testing.T) {
		var p string
		var requests int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			p = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		defer srv.Close()

		facade := newTestBalancesFacade(t, srv)

		got, err := facade.ListBalancesByExternalCode(context.Background(), balancesOrgID, balancesLedgerID, "USD")
		if err != nil {
			t.Fatalf("ListBalancesByExternalCode: %v", err)
		}
		if want := balancesLedgerScope() + "/accounts/external/USD/balances"; p != want {
			t.Fatalf("path = %q, want %q", p, want)
		}
		if len(got.Items) != 1 {
			t.Fatalf("items = %+v", got.Items)
		}
		if requests != 1 {
			t.Fatalf("requests = %d, want 1 (the endpoint answers in one shot)", requests)
		}
	})
}

// TestBalancesFacade_ValidatesBeforeRequest proves the pre-flight
// short-circuits: an over-limit list and an empty write payload fail locally, so
// the server is never hit.
func TestBalancesFacade_ValidatesBeforeRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("server must not be hit on invalid input")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	facade := newTestBalancesFacade(t, srv)
	overLimit := models.BalancesListOpts{CursorListOpts: models.CursorListOpts{Limit: models.MaxLimit + 1}}

	_, err := facade.ListBalances(context.Background(), balancesOrgID, balancesLedgerID, overLimit)
	if err == nil || !strings.Contains(err.Error(), "limit exceeds maximum") {
		t.Fatalf("ListBalances over limit = %v, want a limit validation error", err)
	}

	if _, err := facade.ListAccountBalances(context.Background(), balancesOrgID, balancesLedgerID, balancesAcctID, overLimit); err == nil {
		t.Fatal("expected ListAccountBalances to reject an over-limit request")
	}

	if _, err := facade.CreateBalance(context.Background(), balancesOrgID, balancesLedgerID, balancesAcctID, &models.CreateBalanceInput{}); err == nil {
		t.Fatal("expected CreateBalance to reject an input with no key")
	}

	if _, err := facade.UpdateBalance(context.Background(), balancesOrgID, balancesLedgerID, balancesBalanceID, &models.UpdateBalanceInput{}); err == nil {
		t.Fatal("expected UpdateBalance to reject an empty payload")
	}
}

// TestBalancesFacade_ErrorDecodes asserts the RFC 9457 decode with request-ID
// correlation on the read, write and delete paths.
func TestBalancesFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-bal-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0090","title":"Invalid balance","status":422}`))
	}))
	defer srv.Close()

	facade := newTestBalancesFacade(t, srv)

	assertDecoded := func(t *testing.T, err error) {
		t.Helper()

		var sdkErr *sdkerrors.Error
		if !errors.As(err, &sdkErr) {
			t.Fatalf("error type = %T, want *errors.Error", err)
		}

		if sdkErr.APICode != "LEDGER-0090" || sdkErr.RequestID != "req-bal-422" {
			t.Fatalf("decoded error = %+v", sdkErr)
		}
	}

	_, err := facade.ListBalances(context.Background(), balancesOrgID, balancesLedgerID, models.BalancesListOpts{})
	assertDecoded(t, err)

	_, err = facade.GetBalance(context.Background(), balancesOrgID, balancesLedgerID, balancesBalanceID)
	assertDecoded(t, err)

	allow := true
	_, err = facade.UpdateBalance(context.Background(), balancesOrgID, balancesLedgerID, balancesBalanceID, &models.UpdateBalanceInput{AllowSending: &allow})
	assertDecoded(t, err)

	assertDecoded(t, facade.DeleteBalance(context.Background(), balancesOrgID, balancesLedgerID, balancesBalanceID))

	_, err = facade.GetBalanceHistory(context.Background(), balancesOrgID, balancesLedgerID, balancesBalanceID, balancesInstant)
	assertDecoded(t, err)

	_, err = facade.GetAccountBalancesHistory(context.Background(), balancesOrgID, balancesLedgerID, balancesAcctID, balancesInstant)
	assertDecoded(t, err)

	_, err = facade.ListBalancesByAccountAlias(context.Background(), balancesOrgID, balancesLedgerID, "@cash")
	assertDecoded(t, err)

	_, err = facade.ListBalancesByExternalCode(context.Background(), balancesOrgID, balancesLedgerID, "USD")
	assertDecoded(t, err)
}

// TestBalancesFacade_WriteReplaySafe is the money-path 401-replay guard: the
// marshaled body must survive a token refresh and replay intact.
func TestBalancesFacade_WriteReplaySafe(t *testing.T) {
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

		raw, _ := io.ReadAll(r.Body)
		replayed = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + balancesBalanceID + `","key":"asset-freeze","assetCode":"USD","available":"0"}`))
	}))
	defer srv.Close()

	if _, err := newTestBalancesFacade(t, srv).CreateBalance(context.Background(), balancesOrgID, balancesLedgerID, balancesAcctID,
		&models.CreateBalanceInput{Key: "asset-freeze"}); err != nil {
		t.Fatalf("CreateBalance with one 401 refresh: %v", err)
	}

	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}

	if !strings.Contains(replayed, `"key":"asset-freeze"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

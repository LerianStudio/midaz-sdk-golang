// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	accountsOrgID    = "11111111-1111-1111-1111-111111111111"
	accountsLedgerID = "22222222-2222-2222-2222-222222222222"
	accountsAcctID   = "33333333-3333-3333-3333-333333333333"
)

func accountsBase() string {
	return "/v1/organizations/" + accountsOrgID + "/ledgers/" + accountsLedgerID + "/accounts"
}

func newTestAccountsFacade(t *testing.T, srv *httptest.Server) *accountsFacade {
	t.Helper()
	return &accountsFacade{ledger: newTestLedgerClient(t, srv)}
}

// TestAccountsFacade_ListAndPaginate exercises the page-based List/All trinaldo,
// chaining two org+ledger-scoped pages.
func TestAccountsFacade_ListAndPaginate(t *testing.T) {
	page1 := `{"items":[{"id":"a1","name":"Checking","type":"deposit","assetCode":"USD"}],"limit":1,"next_cursor":"c2"}`
	page2 := `{"items":[{"id":"a2","name":"Savings","type":"savings","assetCode":"USD"}],"limit":1}`

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(page2))
		} else {
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer srv.Close()

	facade := newTestAccountsFacade(t, srv)

	first, err := facade.List(context.Background(), accountsOrgID, accountsLedgerID, models.AccountsListOpts{PageListOpts: models.PageListOpts{Limit: 1}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotPath != accountsBase() {
		t.Fatalf("path = %q, want %q", gotPath, accountsBase())
	}
	if len(first.Items) != 1 || first.Items[0].Name != "Checking" {
		t.Fatalf("List page 1 = %+v", first.Items)
	}

	all, err := CollectAll(facade.All(context.Background(), accountsOrgID, accountsLedgerID, models.AccountsListOpts{PageListOpts: models.PageListOpts{Limit: 1}}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].Name != "Checking" || all[1].Name != "Savings" {
		t.Fatalf("All = %+v", all)
	}
}

// TestAccountsFacade_CRUD round-trips Create/Get/Update/Delete on the
// org+ledger-scoped path.
func TestAccountsFacade_CRUD(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"` + accountsAcctID + `","name":"Checking","type":"deposit","assetCode":"USD"}`))
		}))
		defer srv.Close()

		a, err := newTestAccountsFacade(t, srv).Create(context.Background(), accountsOrgID, accountsLedgerID, &models.CreateAccountInput{
			Name: "Checking", AssetCode: "USD", Type: "deposit",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m != http.MethodPost || p != accountsBase() {
			t.Fatalf("create req = %s %s, want POST %s", m, p, accountsBase())
		}
		if !strings.Contains(body, `"assetCode":"USD"`) || !strings.Contains(body, `"type":"deposit"`) {
			t.Fatalf("body = %q, want marshaled CreateAccountInput", body)
		}
		if a.ID != accountsAcctID || a.AssetCode != "USD" {
			t.Fatalf("Create returned %+v", a)
		}
	})

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + accountsAcctID + `","name":"Checking"}`))
		}))
		defer srv.Close()

		a, err := newTestAccountsFacade(t, srv).Get(context.Background(), accountsOrgID, accountsLedgerID, accountsAcctID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if m != http.MethodGet || p != accountsBase()+"/"+accountsAcctID {
			t.Fatalf("get req = %s %s", m, p)
		}
		if a.ID != accountsAcctID {
			t.Fatalf("Get returned %+v", a)
		}
	})

	t.Run("update", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + accountsAcctID + `","name":"Renamed"}`))
		}))
		defer srv.Close()

		a, err := newTestAccountsFacade(t, srv).Update(context.Background(), accountsOrgID, accountsLedgerID, accountsAcctID, &models.UpdateAccountInput{Name: "Renamed"})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if m != http.MethodPatch || p != accountsBase()+"/"+accountsAcctID {
			t.Fatalf("update req = %s %s", m, p)
		}
		if !strings.Contains(body, `"name":"Renamed"`) {
			t.Fatalf("body = %q, want marshaled UpdateAccountInput", body)
		}
		if a.Name != "Renamed" {
			t.Fatalf("Update returned %+v", a)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		if err := newTestAccountsFacade(t, srv).Delete(context.Background(), accountsOrgID, accountsLedgerID, accountsAcctID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if m != http.MethodDelete || p != accountsBase()+"/"+accountsAcctID {
			t.Fatalf("delete req = %s %s", m, p)
		}
	})
}

// TestAccountsFacade_GetByAlias resolves an account by its path-segment alias.
func TestAccountsFacade_GetByAlias(t *testing.T) {
	const alias = "@treasury"
	var p string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + accountsAcctID + `","name":"Treasury","alias":"` + alias + `"}`))
	}))
	defer srv.Close()

	a, err := newTestAccountsFacade(t, srv).GetByAlias(context.Background(), accountsOrgID, accountsLedgerID, alias)
	if err != nil {
		t.Fatalf("GetByAlias: %v", err)
	}
	wantPath := accountsBase() + "/alias/" + alias
	if p != wantPath {
		t.Fatalf("alias path = %q, want %q", p, wantPath)
	}
	if a.ID != accountsAcctID || a.Alias == nil || *a.Alias != alias {
		t.Fatalf("GetByAlias returned %+v", a)
	}
}

// TestAccountsFacade_Filters is the no-silent-drop guard: holder_id and
// include_deleted have no slot in ListAccountsParams, so they must be injected
// via request editors; the natively-slotted filters must land as query params.
func TestAccountsFacade_Filters(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":7}`))
	}))
	defer srv.Close()

	_, err := newTestAccountsFacade(t, srv).List(context.Background(), accountsOrgID, accountsLedgerID, models.AccountsListOpts{
		PageListOpts: models.PageListOpts{Limit: 7},
		Filters: models.AccountsFilters{
			Type: "deposit", Status: "ACTIVE", AssetCode: "USD",
			HolderID: "h-1", PortfolioID: "pf-1", SegmentID: "sg-1",
			Alias: "@x", ParentAccountID: "pa-1", Name: "chk", EntityID: "ext-1",
			IncludeDeleted: true, Blocked: true,
		},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// Natively-slotted filters.
	for k, want := range map[string]string{
		"type": "deposit", "status": "ACTIVE", "asset_code": "USD",
		"portfolio_id": "pf-1", "segment_id": "sg-1", "alias": "@x",
		"parent_account_id": "pa-1", "name": "chk", "entity_id": "ext-1",
		"blocked": "true", "limit": "7",
	} {
		if got := q.Get(k); got != want {
			t.Fatalf("%s = %q, want %q", k, got, want)
		}
	}

	// Editor-injected filters (no slot in ListAccountsParams).
	if got := q.Get("holder_id"); got != "h-1" {
		t.Fatalf("holder_id = %q, want h-1 (editor must inject it)", got)
	}
	if got := q.Get("include_deleted"); got != "true" {
		t.Fatalf("include_deleted = %q, want true (editor must inject it)", got)
	}
}

// TestAccountsFacade_ListBalances chains the cursor-paginated balances sub-list:
// page 1 issues no cursor and returns next_cursor, page 2 must echo it back.
func TestAccountsFacade_ListBalances(t *testing.T) {
	var cursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "cur-2" {
			_, _ = w.Write([]byte(`{"items":[{"id":"b2","accountId":"` + accountsAcctID + `","assetCode":"USD","available":"20"}],"limit":1}`))
		} else {
			_, _ = w.Write([]byte(`{"items":[{"id":"b1","accountId":"` + accountsAcctID + `","assetCode":"USD","available":"10"}],"limit":1,"next_cursor":"cur-2"}`))
		}
	}))
	defer srv.Close()

	facade := newTestAccountsFacade(t, srv)

	first, err := facade.ListBalances(context.Background(), accountsOrgID, accountsLedgerID, accountsAcctID, models.CursorListOpts{Limit: 1})
	if err != nil {
		t.Fatalf("ListBalances: %v", err)
	}
	if len(first.Items) != 1 || first.Items[0].ID != "b1" {
		t.Fatalf("ListBalances page 1 = %+v", first.Items)
	}
	// Money-path decode: the available amount must round-trip to a decimal.
	if !first.Items[0].Available.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("ListBalances page 1 available = %s, want 10", first.Items[0].Available)
	}

	all, err := CollectAll(facade.ListBalancesAll(context.Background(), accountsOrgID, accountsLedgerID, accountsAcctID, models.CursorListOpts{Limit: 1}))
	if err != nil {
		t.Fatalf("ListBalancesAll: %v", err)
	}
	if len(all) != 2 || all[0].ID != "b1" || all[1].ID != "b2" {
		t.Fatalf("ListBalancesAll = %+v", all)
	}
	// Chained page's amount must decode too.
	if !all[0].Available.Equal(decimal.NewFromInt(10)) || !all[1].Available.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("ListBalancesAll available = [%s %s], want [10 20]", all[0].Available, all[1].Available)
	}
	// The cursor must advance from empty -> next_cursor, not Page++.
	if len(cursors) != 3 || cursors[0] != "" || cursors[1] != "" || cursors[2] != "cur-2" {
		t.Fatalf("cursor chain = %v, want ['', '', 'cur-2']", cursors)
	}
}

// TestAccountsFacade_ListOperations chains the cursor-paginated operations
// sub-list and asserts its endpoint-specific filters land on the wire.
func TestAccountsFacade_ListOperations(t *testing.T) {
	var cursors []string
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "op-2" {
			_, _ = w.Write([]byte(`{"items":[{"id":"o2","type":"CREDIT"}],"limit":1}`))
		} else {
			_, _ = w.Write([]byte(`{"items":[{"id":"o1","type":"DEBIT"}],"limit":1,"next_cursor":"op-2"}`))
		}
	}))
	defer srv.Close()

	facade := newTestAccountsFacade(t, srv)

	opts := models.AccountOperationsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 1},
		Filters:        models.AccountOperationsFilters{Type: "DEBIT", Direction: "debit", RouteID: "rt-1", RouteCode: "RC1"},
	}

	first, err := facade.ListOperations(context.Background(), accountsOrgID, accountsLedgerID, accountsAcctID, opts)
	if err != nil {
		t.Fatalf("ListOperations: %v", err)
	}
	if len(first.Items) != 1 || first.Items[0].ID != "o1" {
		t.Fatalf("ListOperations page 1 = %+v", first.Items)
	}

	// Endpoint-specific filters must reach the wire (no silent drop).
	for k, want := range map[string]string{"type": "DEBIT", "direction": "debit", "route_id": "rt-1", "route_code": "RC1"} {
		if got := q.Get(k); got != want {
			t.Fatalf("%s = %q, want %q", k, got, want)
		}
	}

	all, err := CollectAll(facade.ListOperationsAll(context.Background(), accountsOrgID, accountsLedgerID, accountsAcctID, opts))
	if err != nil {
		t.Fatalf("ListOperationsAll: %v", err)
	}
	if len(all) != 2 || all[0].ID != "o1" || all[1].ID != "o2" {
		t.Fatalf("ListOperationsAll = %+v", all)
	}
	if len(cursors) != 3 || cursors[0] != "" || cursors[1] != "" || cursors[2] != "op-2" {
		t.Fatalf("cursor chain = %v, want ['', '', 'op-2']", cursors)
	}
}

// TestAccountsFacade_ListBalancesAll_CursorTerminatesOnFullPage is the
// money-path infinite-loop guard for the cursor-paginated balances sub-list.
// The terminal page comes back FULL (ItemCount==Limit) and carries a page
// field but NO next_cursor. HasMore()'s page-based heuristic (branch 4) then
// returns true, yet NextCursor is "" — so a HasMore()-gated loop would set
// current.Cursor = "" and refetch page 1 forever. A cursor loop must stop on
// an empty next_cursor, not on HasMore(). The kill-switch bounds the RED
// failure so it fails fast instead of hanging.
func TestAccountsFacade_ListBalancesAll_CursorTerminatesOnFullPage(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 5 {
			t.Fatalf("infinite loop: cursor sub-list did not terminate (request #%d, cursor=%q)", requests, r.URL.Query().Get("cursor"))
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "cur-2" {
			// Terminal page: FULL (2 items == limit 2), carries a page field,
			// NO next_cursor. This is the HasMore() branch-4 trap.
			_, _ = w.Write([]byte(`{"items":[{"id":"b3","accountId":"` + accountsAcctID + `","assetCode":"USD","available":"30"},{"id":"b4","accountId":"` + accountsAcctID + `","assetCode":"USD","available":"40"}],"limit":2,"page":1}`))
		} else {
			_, _ = w.Write([]byte(`{"items":[{"id":"b1","accountId":"` + accountsAcctID + `","assetCode":"USD","available":"10"},{"id":"b2","accountId":"` + accountsAcctID + `","assetCode":"USD","available":"20"}],"limit":2,"page":1,"next_cursor":"cur-2"}`))
		}
	}))
	defer srv.Close()

	facade := newTestAccountsFacade(t, srv)

	all, err := CollectAll(facade.ListBalancesAll(context.Background(), accountsOrgID, accountsLedgerID, accountsAcctID, models.CursorListOpts{Limit: 2}))
	if err != nil {
		t.Fatalf("ListBalancesAll: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2 (page 1 + terminal page)", requests)
	}
	if len(all) != 4 {
		t.Fatalf("ListBalancesAll = %d items, want 4", len(all))
	}
	if all[0].ID != "b1" || all[3].ID != "b4" {
		t.Fatalf("ListBalancesAll IDs = %v", []string{all[0].ID, all[1].ID, all[2].ID, all[3].ID})
	}
}

// TestAccountsFacade_ListOperationsAll_CursorTerminatesOnFullPage is the same
// money-path infinite-loop guard for the cursor-paginated operations sub-list.
func TestAccountsFacade_ListOperationsAll_CursorTerminatesOnFullPage(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 5 {
			t.Fatalf("infinite loop: cursor sub-list did not terminate (request #%d, cursor=%q)", requests, r.URL.Query().Get("cursor"))
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "op-2" {
			// Terminal page: FULL (2 items == limit 2), carries a page field,
			// NO next_cursor.
			_, _ = w.Write([]byte(`{"items":[{"id":"o3","type":"CREDIT"},{"id":"o4","type":"DEBIT"}],"limit":2,"page":1}`))
		} else {
			_, _ = w.Write([]byte(`{"items":[{"id":"o1","type":"DEBIT"},{"id":"o2","type":"CREDIT"}],"limit":2,"page":1,"next_cursor":"op-2"}`))
		}
	}))
	defer srv.Close()

	facade := newTestAccountsFacade(t, srv)

	opts := models.AccountOperationsListOpts{CursorListOpts: models.CursorListOpts{Limit: 2}}

	all, err := CollectAll(facade.ListOperationsAll(context.Background(), accountsOrgID, accountsLedgerID, accountsAcctID, opts))
	if err != nil {
		t.Fatalf("ListOperationsAll: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2 (page 1 + terminal page)", requests)
	}
	if len(all) != 4 {
		t.Fatalf("ListOperationsAll = %d items, want 4", len(all))
	}
	if all[0].ID != "o1" || all[3].ID != "o4" {
		t.Fatalf("ListOperationsAll IDs = %v", []string{all[0].ID, all[1].ID, all[2].ID, all[3].ID})
	}
}

// TestAccountsFacade_BalancesAtTimestamp decodes the non-paginated slice
// response and threads the date query param.
func TestAccountsFacade_BalancesAtTimestamp(t *testing.T) {
	var date string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		date = r.URL.Query().Get("date")
		w.Header().Set("Content-Type", "application/json")
		// The generated point-in-time Parse decodes into typed openapi_types.UUID
		// id/accountId fields (it validates before the facade re-decodes into the
		// SDK model), so these must be valid UUIDs — a real server sends UUIDs here.
		_, _ = w.Write([]byte(`[{"id":"44444444-4444-4444-4444-444444444444","accountId":"` + accountsAcctID + `","assetCode":"USD","available":"5"}]`))
	}))
	defer srv.Close()

	got, err := newTestAccountsFacade(t, srv).BalancesAtTimestamp(context.Background(), accountsOrgID, accountsLedgerID, accountsAcctID, "2026-01-01 00:00:00")
	if err != nil {
		t.Fatalf("BalancesAtTimestamp: %v", err)
	}
	if date != "2026-01-01 00:00:00" {
		t.Fatalf("date param = %q", date)
	}
	if len(got) != 1 || got[0].ID != "44444444-4444-4444-4444-444444444444" {
		t.Fatalf("BalancesAtTimestamp = %+v", got)
	}
	// Money-path decode: the point-in-time available amount must round-trip.
	if !got[0].Available.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("BalancesAtTimestamp available = %s, want 5", got[0].Available)
	}
}

// TestAccountsFacade_ErrorDecodes asserts RFC 9457 decode with request-ID on
// the money-path-adjacent error path.
func TestAccountsFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-acct-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0009","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestAccountsFacade(t, srv).Create(context.Background(), accountsOrgID, accountsLedgerID, &models.CreateAccountInput{
		Name: "Checking", AssetCode: "USD", Type: "deposit",
	})
	sdkErr, ok := err.(*sdkerrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0009" || sdkErr.RequestID != "req-acct-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestAccountsFacade_WriteReplaySafe is the money-path 401-replay guard: the
// marshaled body must survive a 401 refresh and replay intact.
func TestAccountsFacade_WriteReplaySafe(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"` + accountsAcctID + `","name":"Checking","assetCode":"USD"}`))
	}))
	defer srv.Close()

	_, err := newTestAccountsFacade(t, srv).Create(context.Background(), accountsOrgID, accountsLedgerID, &models.CreateAccountInput{
		Name: "Checking", AssetCode: "USD", Type: "deposit",
	})
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"assetCode":"USD"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

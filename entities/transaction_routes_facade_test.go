// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
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
	transactionRoutesOrgID    = "11111111-1111-1111-1111-111111111111"
	transactionRoutesLedgerID = "22222222-2222-2222-2222-222222222222"
)

func transactionRoutesBase() string {
	return "/v1/organizations/" + transactionRoutesOrgID + "/ledgers/" + transactionRoutesLedgerID + "/transaction-routes"
}

// TestTransactionRoutesFacade_ListAndPaginate exercises the cursor List/Pages/All
// trinaldo end-to-end, chaining two cursor pages then stopping on an empty
// next_cursor. A HasMore()-based stop would loop forever on the second
// (terminal) page; this asserts the cursor-pure stop.
func TestTransactionRoutesFacade_ListAndPaginate(t *testing.T) {
	page1 := `{"items":[{"id":"33333333-3333-3333-3333-333333333333","title":"Settlement"}],"limit":1,"next_cursor":"c2"}`
	page2 := `{"items":[{"id":"44444444-4444-4444-4444-444444444444","title":"Refund"}],"limit":1}`

	var seenCursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCursors = append(seenCursors, r.URL.Query().Get("cursor"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "c2" {
			_, _ = w.Write([]byte(page2))
		} else {
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer srv.Close()

	facade := newTestTransactionRoutesFacade(t, srv)

	all, err := CollectAll(facade.All(context.Background(), transactionRoutesOrgID, transactionRoutesLedgerID, models.TransactionRoutesListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 1},
	}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].Title != "Settlement" || all[1].Title != "Refund" {
		t.Fatalf("All = %+v", all)
	}
	if len(seenCursors) != 2 || seenCursors[0] != "" || seenCursors[1] != "c2" {
		t.Fatalf("cursor chain = %v, want ['' 'c2'] and exactly 2 requests", seenCursors)
	}
}

// TestTransactionRoutesFacade_CRUD round-trips Create/Get/Update/Delete over the
// generated client on the org+ledger-scoped path, asserting verb+path+body match
// the legacy transactionRoutesEntity wire. The create subtest is the money-path
// assert: operationRoutes must serialize as a flat array of UUID strings, not
// objects (mirroring the legacy json.Marshal of []uuid.UUID).
//
//nolint:revive // cognitive-complexity: four CRUD subtests, each with its own httptest server closure and assertions; matches the repo's per-test convention.
func TestTransactionRoutesFacade_CRUD(t *testing.T) {
	const id = "55555555-5555-5555-5555-555555555555"
	const opRoute1 = "66666666-6666-6666-6666-666666666666"
	const opRoute2 = "77777777-7777-7777-7777-777777777777"

	t.Run("create", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"` + id + `","title":"Settlement"}`))
		}))
		defer srv.Close()

		route, err := newTestTransactionRoutesFacade(t, srv).Create(context.Background(), transactionRoutesOrgID, transactionRoutesLedgerID,
			models.NewCreateTransactionRouteInput("Settlement", "settlement route", []string{opRoute1, opRoute2}))
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m != http.MethodPost || p != transactionRoutesBase() {
			t.Fatalf("create req = %s %s, want POST %s", m, p, transactionRoutesBase())
		}
		if !strings.Contains(body, `"title":"Settlement"`) {
			t.Fatalf("body = %q, want marshaled CreateTransactionRouteInput", body)
		}

		// Money-path wire assert: operationRoutes must be a flat array of UUID
		// strings on the wire (["<uuid>","<uuid>"]), NEVER an array of objects.
		// Decode the sent body and compare against the legacy []uuid.UUID shape.
		var sent struct {
			OperationRoutes []string `json:"operationRoutes"`
		}
		if err := json.Unmarshal([]byte(body), &sent); err != nil {
			t.Fatalf("sent body not decodable as UUID array: %v (body=%q)", err, body)
		}
		if len(sent.OperationRoutes) != 2 || sent.OperationRoutes[0] != opRoute1 || sent.OperationRoutes[1] != opRoute2 {
			t.Fatalf("operationRoutes = %v, want flat UUID array [%s %s]", sent.OperationRoutes, opRoute1, opRoute2)
		}
		if strings.Contains(body, `"operationRoutes":[{`) {
			t.Fatalf("operationRoutes serialized as objects, want flat UUID array: %q", body)
		}
		if route.ID.String() != id || route.Title != "Settlement" {
			t.Fatalf("Create returned %+v", route)
		}
	})

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","title":"Settlement"}`))
		}))
		defer srv.Close()

		route, err := newTestTransactionRoutesFacade(t, srv).Get(context.Background(), transactionRoutesOrgID, transactionRoutesLedgerID, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if m != http.MethodGet || p != transactionRoutesBase()+"/"+id {
			t.Fatalf("get req = %s %s", m, p)
		}
		if route.ID.String() != id {
			t.Fatalf("Get returned %+v", route)
		}
	})

	t.Run("update", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","title":"Renamed"}`))
		}))
		defer srv.Close()

		route, err := newTestTransactionRoutesFacade(t, srv).Update(context.Background(), transactionRoutesOrgID, transactionRoutesLedgerID, id,
			models.NewUpdateTransactionRouteInput().WithTitle("Renamed"))
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if m != http.MethodPatch || p != transactionRoutesBase()+"/"+id {
			t.Fatalf("update req = %s %s", m, p)
		}
		if !strings.Contains(body, `"title":"Renamed"`) {
			t.Fatalf("body = %q, want marshaled UpdateTransactionRouteInput", body)
		}
		if route.Title != "Renamed" {
			t.Fatalf("Update returned %+v", route)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		if err := newTestTransactionRoutesFacade(t, srv).Delete(context.Background(), transactionRoutesOrgID, transactionRoutesLedgerID, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if m != http.MethodDelete || p != transactionRoutesBase()+"/"+id {
			t.Fatalf("delete req = %s %s", m, p)
		}
	})
}

// TestTransactionRoutesFacade_Filters is the per-resource differentiator. The
// cursor/sort/date fields map to generated param slots; name, status, and
// operation_route_id have no slot and must be injected via request editors,
// mirroring the legacy ToQueryParams wire keys.
func TestTransactionRoutesFacade_Filters(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":7}`))
	}))
	defer srv.Close()

	_, err := newTestTransactionRoutesFacade(t, srv).List(context.Background(), transactionRoutesOrgID, transactionRoutesLedgerID, models.TransactionRoutesListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 7, SortDirection: models.SortAscending},
		Filters:        models.TransactionRoutesFilters{Name: "Settlement", Status: "ACTIVE", OperationRouteID: "66666666-6666-6666-6666-666666666666"},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := q.Get("limit"); got != "7" {
		t.Fatalf("limit = %q, want 7 (param slot)", got)
	}
	if got := q.Get("sort_order"); got != "asc" {
		t.Fatalf("sort_order = %q, want asc (param slot)", got)
	}
	if got := q.Get("name"); got != "Settlement" {
		t.Fatalf("name = %q, want Settlement (editor must inject it)", got)
	}
	if got := q.Get("status"); got != "ACTIVE" {
		t.Fatalf("status = %q, want ACTIVE (editor must inject it)", got)
	}
	if got := q.Get("operation_route_id"); got != "66666666-6666-6666-6666-666666666666" {
		t.Fatalf("operation_route_id = %q, want the UUID (editor must inject it)", got)
	}
}

// TestTransactionRoutesFacade_ErrorDecodes asserts RFC 9457 decode with request-ID.
func TestTransactionRoutesFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-tr-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0009","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestTransactionRoutesFacade(t, srv).Create(context.Background(), transactionRoutesOrgID, transactionRoutesLedgerID,
		models.NewCreateTransactionRouteInput("Settlement", "settlement route", []string{"66666666-6666-6666-6666-666666666666"}))
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0009" || sdkErr.RequestID != "req-tr-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestTransactionRoutesFacade_WriteReplaySafe is the money-path 401-replay guard:
// the write body must survive the auth round tripper's post-401 replay, and the
// operationRoutes UUID array must be intact on the replayed request.
func TestTransactionRoutesFacade_WriteReplaySafe(t *testing.T) {
	const opRoute = "66666666-6666-6666-6666-666666666666"
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
		_, _ = w.Write([]byte(`{"id":"55555555-5555-5555-5555-555555555555","title":"Settlement"}`))
	}))
	defer srv.Close()

	_, err := newTestTransactionRoutesFacade(t, srv).Create(context.Background(), transactionRoutesOrgID, transactionRoutesLedgerID,
		models.NewCreateTransactionRouteInput("Settlement", "settlement route", []string{opRoute}))
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"title":"Settlement"`) || !strings.Contains(replayed, `"`+opRoute+`"`) {
		t.Fatalf("replayed body = %q, want full JSON with UUID array intact", replayed)
	}
}

func newTestTransactionRoutesFacade(t *testing.T, srv *httptest.Server) *transactionRoutesFacade {
	t.Helper()
	return newTransactionRoutesFacade(newTestLedgerClient(t, srv), true)
}

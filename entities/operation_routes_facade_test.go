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
	operationRoutesOrgID    = "11111111-1111-1111-1111-111111111111"
	operationRoutesLedgerID = "22222222-2222-2222-2222-222222222222"
)

func operationRoutesBase() string {
	return "/v1/organizations/" + operationRoutesOrgID + "/ledgers/" + operationRoutesLedgerID + "/operation-routes"
}

// TestOperationRoutesFacade_ListAndPaginate exercises the cursor List/Pages/All
// trinaldo end-to-end over the generated client, chaining two cursor pages then
// stopping on an empty next_cursor. A HasMore()-based stop would loop forever on
// the second (terminal) page; this asserts the cursor-pure stop.
func TestOperationRoutesFacade_ListAndPaginate(t *testing.T) {
	page1 := `{"items":[{"id":"33333333-3333-3333-3333-333333333333","title":"Cashin","operationType":"source"}],"limit":1,"next_cursor":"c2"}`
	page2 := `{"items":[{"id":"44444444-4444-4444-4444-444444444444","title":"Cashout","operationType":"destination"}],"limit":1}`

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

	facade := newTestOperationRoutesFacade(t, srv)

	all, err := CollectAll(facade.All(context.Background(), operationRoutesOrgID, operationRoutesLedgerID, models.OperationRoutesListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 1},
	}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].Title != "Cashin" || all[1].Title != "Cashout" {
		t.Fatalf("All = %+v", all)
	}
	// Cursor chain: first request has no cursor, second echoes next_cursor "c2",
	// then the terminal page's empty next_cursor stops the loop (exactly 2 calls).
	if len(seenCursors) != 2 || seenCursors[0] != "" || seenCursors[1] != "c2" {
		t.Fatalf("cursor chain = %v, want ['' 'c2'] and exactly 2 requests", seenCursors)
	}
}

// TestOperationRoutesFacade_CRUD round-trips Create/Get/Update/Delete over the
// generated client on the org+ledger-scoped path, asserting verb+path+body match
// the legacy operationRoutesEntity wire.
//
//nolint:revive // cognitive-complexity: four CRUD subtests, each with its own httptest server closure and assertions; matches the repo's per-test convention.
func TestOperationRoutesFacade_CRUD(t *testing.T) {
	const id = "55555555-5555-5555-5555-555555555555"

	t.Run("create", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"` + id + `","title":"Cashin","operationType":"source"}`))
		}))
		defer srv.Close()

		route, err := newTestOperationRoutesFacade(t, srv).Create(context.Background(), operationRoutesOrgID, operationRoutesLedgerID,
			models.NewCreateOperationRouteInput("Cashin", "cash-in route", "source"))
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m != http.MethodPost || p != operationRoutesBase() {
			t.Fatalf("create req = %s %s, want POST %s", m, p, operationRoutesBase())
		}
		if !strings.Contains(body, `"title":"Cashin"`) || !strings.Contains(body, `"operationType":"source"`) {
			t.Fatalf("body = %q, want marshaled CreateOperationRouteInput", body)
		}
		if route.ID.String() != id || route.Title != "Cashin" {
			t.Fatalf("Create returned %+v", route)
		}
	})

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","title":"Cashin","operationType":"source"}`))
		}))
		defer srv.Close()

		route, err := newTestOperationRoutesFacade(t, srv).Get(context.Background(), operationRoutesOrgID, operationRoutesLedgerID, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if m != http.MethodGet || p != operationRoutesBase()+"/"+id {
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
			_, _ = w.Write([]byte(`{"id":"` + id + `","title":"Renamed","operationType":"source"}`))
		}))
		defer srv.Close()

		route, err := newTestOperationRoutesFacade(t, srv).Update(context.Background(), operationRoutesOrgID, operationRoutesLedgerID, id,
			models.NewUpdateOperationRouteInput().WithTitle("Renamed"))
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if m != http.MethodPatch || p != operationRoutesBase()+"/"+id {
			t.Fatalf("update req = %s %s", m, p)
		}
		if !strings.Contains(body, `"title":"Renamed"`) {
			t.Fatalf("body = %q, want marshaled UpdateOperationRouteInput", body)
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

		if err := newTestOperationRoutesFacade(t, srv).Delete(context.Background(), operationRoutesOrgID, operationRoutesLedgerID, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if m != http.MethodDelete || p != operationRoutesBase()+"/"+id {
			t.Fatalf("delete req = %s %s", m, p)
		}
	})
}

// TestOperationRoutesFacade_Filters is the per-resource differentiator. The
// cursor/sort/date fields map to generated param slots; name, status, and
// operation_type have no slot and must be injected via request editors. A
// regression that drops any filter surfaces here.
func TestOperationRoutesFacade_Filters(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":7}`))
	}))
	defer srv.Close()

	_, err := newTestOperationRoutesFacade(t, srv).List(context.Background(), operationRoutesOrgID, operationRoutesLedgerID, models.OperationRoutesListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 7, SortDirection: models.SortAscending},
		Filters:        models.OperationRoutesFilters{Name: "Cashin", Status: "ACTIVE", OperationType: "source"},
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
	if got := q.Get("name"); got != "Cashin" {
		t.Fatalf("name = %q, want Cashin (editor must inject it)", got)
	}
	if got := q.Get("status"); got != "ACTIVE" {
		t.Fatalf("status = %q, want ACTIVE (editor must inject it)", got)
	}
	if got := q.Get("operation_type"); got != "source" {
		t.Fatalf("operation_type = %q, want source (editor must inject it)", got)
	}
}

// TestOperationRoutesFacade_ErrorDecodes asserts RFC 9457 decode with request-ID.
func TestOperationRoutesFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-or-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0009","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestOperationRoutesFacade(t, srv).Create(context.Background(), operationRoutesOrgID, operationRoutesLedgerID,
		models.NewCreateOperationRouteInput("Cashin", "cash-in route", "source"))
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0009" || sdkErr.RequestID != "req-or-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestOperationRoutesFacade_WriteReplaySafe is the money-path 401-replay guard:
// the write body must survive the auth round tripper's post-401 replay.
func TestOperationRoutesFacade_WriteReplaySafe(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"55555555-5555-5555-5555-555555555555","title":"Cashin","operationType":"source"}`))
	}))
	defer srv.Close()

	_, err := newTestOperationRoutesFacade(t, srv).Create(context.Background(), operationRoutesOrgID, operationRoutesLedgerID,
		models.NewCreateOperationRouteInput("Cashin", "cash-in route", "source"))
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"title":"Cashin"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

func newTestOperationRoutesFacade(t *testing.T, srv *httptest.Server) *operationRoutesFacade {
	t.Helper()
	return newOperationRoutesFacade(newTestLedgerClient(t, srv), true)
}

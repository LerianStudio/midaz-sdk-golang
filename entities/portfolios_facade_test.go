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
	portfoliosOrgID    = "11111111-1111-1111-1111-111111111111"
	portfoliosLedgerID = "22222222-2222-2222-2222-222222222222"
)

func portfoliosBase() string {
	return "/v1/organizations/" + portfoliosOrgID + "/ledgers/" + portfoliosLedgerID + "/portfolios"
}

// TestPortfoliosFacade_ListAndPaginate exercises the List/All trinaldo end-to-end.
func TestPortfoliosFacade_ListAndPaginate(t *testing.T) {
	page1 := `{"items":[{"id":"44444444-4444-4444-4444-444444444444","name":"Alpha"}],"limit":1,"next_cursor":"c2"}`
	page2 := `{"items":[{"id":"55555555-5555-5555-5555-555555555555","name":"Beta"}],"limit":1}`

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

	facade := newTestPortfoliosFacade(t, srv)

	first, err := facade.List(context.Background(), portfoliosOrgID, portfoliosLedgerID, models.PortfoliosListOpts{PageListOpts: models.PageListOpts{Limit: 1}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotPath != portfoliosBase() {
		t.Fatalf("path = %q, want %q", gotPath, portfoliosBase())
	}
	if len(first.Items) != 1 || first.Items[0].Name != "Alpha" {
		t.Fatalf("List page 1 = %+v", first.Items)
	}

	all, err := CollectAll(facade.All(context.Background(), portfoliosOrgID, portfoliosLedgerID, models.PortfoliosListOpts{PageListOpts: models.PageListOpts{Limit: 1}}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].Name != "Alpha" || all[1].Name != "Beta" {
		t.Fatalf("All = %+v", all)
	}
}

// TestPortfoliosFacade_CRUD round-trips Create/Get/Update/Delete.
//
//nolint:revive // cognitive-complexity: four CRUD subtests, each with its own httptest server closure and assertions; the complexity is the subtest count, not branching logic. Matches the repo's per-test convention.
func TestPortfoliosFacade_CRUD(t *testing.T) {
	const id = "33333333-3333-3333-3333-333333333333"

	t.Run("create", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Alpha","entityId":"ent-1"}`))
		}))
		defer srv.Close()

		pf, err := newTestPortfoliosFacade(t, srv).Create(context.Background(), portfoliosOrgID, portfoliosLedgerID, &models.CreatePortfolioInput{
			Name: "Alpha", EntityID: "ent-1",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m != http.MethodPost || p != portfoliosBase() {
			t.Fatalf("create req = %s %s", m, p)
		}
		if !strings.Contains(body, `"name":"Alpha"`) || !strings.Contains(body, `"entityId":"ent-1"`) {
			t.Fatalf("body = %q, want marshaled CreatePortfolioInput", body)
		}
		if pf.ID != id || pf.Name != "Alpha" {
			t.Fatalf("Create returned %+v", pf)
		}
	})

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Alpha"}`))
		}))
		defer srv.Close()

		pf, err := newTestPortfoliosFacade(t, srv).Get(context.Background(), portfoliosOrgID, portfoliosLedgerID, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if m != http.MethodGet || p != portfoliosBase()+"/"+id {
			t.Fatalf("get req = %s %s", m, p)
		}
		if pf.ID != id {
			t.Fatalf("Get returned %+v", pf)
		}
	})

	t.Run("update", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Renamed"}`))
		}))
		defer srv.Close()

		pf, err := newTestPortfoliosFacade(t, srv).Update(context.Background(), portfoliosOrgID, portfoliosLedgerID, id, &models.UpdatePortfolioInput{Name: "Renamed"})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if m != http.MethodPatch || p != portfoliosBase()+"/"+id {
			t.Fatalf("update req = %s %s", m, p)
		}
		if !strings.Contains(body, `"name":"Renamed"`) {
			t.Fatalf("body = %q, want marshaled UpdatePortfolioInput", body)
		}
		if pf.Name != "Renamed" {
			t.Fatalf("Update returned %+v", pf)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		if err := newTestPortfoliosFacade(t, srv).Delete(context.Background(), portfoliosOrgID, portfoliosLedgerID, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if m != http.MethodDelete || p != portfoliosBase()+"/"+id {
			t.Fatalf("delete req = %s %s", m, p)
		}
	})
}

// TestPortfoliosFacade_ErrorDecodes asserts RFC 9457 decode with request-ID.
func TestPortfoliosFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-pf-503")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0178","title":"Service Unavailable","status":503}`))
	}))
	defer srv.Close()

	_, err := newTestPortfoliosFacade(t, srv).List(context.Background(), portfoliosOrgID, portfoliosLedgerID, models.PortfoliosListOpts{})
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0178" || sdkErr.RequestID != "req-pf-503" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestPortfoliosFacade_Filters guards the mixed split: EntityID+Status map to
// generated slots; Name and IncludeDeleted have no slot and must be injected via
// request editors. The OAS omits name despite the endpoint honoring it, so a
// naive params-only mapping would drop it — this test catches that.
func TestPortfoliosFacade_Filters(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":9}`))
	}))
	defer srv.Close()

	_, err := newTestPortfoliosFacade(t, srv).List(context.Background(), portfoliosOrgID, portfoliosLedgerID, models.PortfoliosListOpts{
		PageListOpts: models.PageListOpts{Limit: 9},
		Filters:      models.PortfoliosFilters{Name: "Alpha", EntityID: "ent-1", Status: "ACTIVE", IncludeDeleted: true},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := q.Get("entity_id"); got != "ent-1" {
		t.Fatalf("entity_id = %q, want ent-1 (generated slot)", got)
	}
	if got := q.Get("status"); got != "ACTIVE" {
		t.Fatalf("status = %q, want ACTIVE (generated slot)", got)
	}
	if got := q.Get("name"); got != "Alpha" {
		t.Fatalf("name = %q, want Alpha (editor must inject it — no param slot)", got)
	}
	if got := q.Get("include_deleted"); got != "true" {
		t.Fatalf("include_deleted = %q, want true (editor must inject it)", got)
	}
	if got := q.Get("limit"); got != "9" {
		t.Fatalf("limit = %q, want 9 (editor must preserve params)", got)
	}
}

// TestPortfoliosFacade_WriteReplaySafe is the money-path 401-replay guard.
func TestPortfoliosFacade_WriteReplaySafe(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"33333333-3333-3333-3333-333333333333","name":"Alpha","entityId":"ent-1"}`))
	}))
	defer srv.Close()

	_, err := newTestPortfoliosFacade(t, srv).Create(context.Background(), portfoliosOrgID, portfoliosLedgerID, &models.CreatePortfolioInput{
		Name: "Alpha", EntityID: "ent-1",
	})
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"name":"Alpha"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

// TestPortfoliosFacade_Count HEADs the metrics/count endpoint and reads the
// total from X-Total-Count.
func TestPortfoliosFacade_Count(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set(HeaderTotalCount, "11")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n, err := newTestPortfoliosFacade(t, srv).Count(context.Background(), portfoliosOrgID, portfoliosLedgerID)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if gotMethod != http.MethodHead {
		t.Fatalf("method = %s, want HEAD", gotMethod)
	}
	if want := portfoliosBase() + "/metrics/count"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
	if n != 11 {
		t.Fatalf("count = %d, want 11", n)
	}
}

// TestPortfoliosFacade_CountErrorEmptyBody proves the readCount error path maps
// a headers-only 403 (JSON content-type, empty body) to authorization, not
// internal.
func TestPortfoliosFacade_CountErrorEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := newTestPortfoliosFacade(t, srv).Count(context.Background(), portfoliosOrgID, portfoliosLedgerID)
	if err == nil {
		t.Fatal("expected error on 403 count")
	}
	if sdkerrors.IsInternalError(err) {
		t.Fatalf("403 empty-body count must not map to internal error, got: %v", err)
	}
	if !sdkerrors.IsAuthorizationError(err) {
		t.Fatalf("403 empty-body count must map to authorization error, got: %v", err)
	}
}

func newTestPortfoliosFacade(t *testing.T, srv *httptest.Server) *portfoliosFacade {
	t.Helper()
	return newPortfoliosFacade(newTestLedgerClient(t, srv), true)
}

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
	assetsOrgID    = "11111111-1111-1111-1111-111111111111"
	assetsLedgerID = "22222222-2222-2222-2222-222222222222"
)

func assetsBase() string {
	return "/v1/organizations/" + assetsOrgID + "/ledgers/" + assetsLedgerID + "/assets"
}

// TestAssetsFacade_ListAndPaginate exercises the List/All trinaldo end-to-end
// over the generated client, chaining two org+ledger-scoped pages.
func TestAssetsFacade_ListAndPaginate(t *testing.T) {
	page1 := `{"items":[{"id":"44444444-4444-4444-4444-444444444444","name":"US Dollar","code":"USD"}],"limit":1,"next_cursor":"c2"}`
	page2 := `{"items":[{"id":"55555555-5555-5555-5555-555555555555","name":"Bitcoin","code":"BTC"}],"limit":1}`

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

	facade := newTestAssetsFacade(t, srv)

	first, err := facade.List(context.Background(), assetsOrgID, assetsLedgerID, models.AssetsListOpts{PageListOpts: models.PageListOpts{Limit: 1}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotPath != assetsBase() {
		t.Fatalf("path = %q, want %q", gotPath, assetsBase())
	}
	if len(first.Items) != 1 || first.Items[0].Code != "USD" {
		t.Fatalf("List page 1 = %+v", first.Items)
	}

	all, err := CollectAll(facade.All(context.Background(), assetsOrgID, assetsLedgerID, models.AssetsListOpts{PageListOpts: models.PageListOpts{Limit: 1}}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].Code != "USD" || all[1].Code != "BTC" {
		t.Fatalf("All = %+v", all)
	}
}

// TestAssetsFacade_CRUD round-trips Create/Get/Update/Delete over the generated
// client on the org+ledger-scoped path.
//
//nolint:revive // cognitive-complexity: four CRUD subtests, each with its own httptest server closure and assertions; the complexity is the subtest count, not branching logic. Matches the repo's per-test convention.
func TestAssetsFacade_CRUD(t *testing.T) {
	const id = "33333333-3333-3333-3333-333333333333"

	t.Run("create", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"US Dollar","code":"USD","type":"currency"}`))
		}))
		defer srv.Close()

		a, err := newTestAssetsFacade(t, srv).Create(context.Background(), assetsOrgID, assetsLedgerID, &models.CreateAssetInput{
			Name: "US Dollar", Code: "USD", Type: "currency",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m != http.MethodPost || p != assetsBase() {
			t.Fatalf("create req = %s %s, want POST %s", m, p, assetsBase())
		}
		if !strings.Contains(body, `"code":"USD"`) || !strings.Contains(body, `"type":"currency"`) {
			t.Fatalf("body = %q, want marshaled CreateAssetInput", body)
		}
		if a.ID != id || a.Code != "USD" {
			t.Fatalf("Create returned %+v", a)
		}
	})

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"US Dollar","code":"USD"}`))
		}))
		defer srv.Close()

		a, err := newTestAssetsFacade(t, srv).Get(context.Background(), assetsOrgID, assetsLedgerID, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if m != http.MethodGet || p != assetsBase()+"/"+id {
			t.Fatalf("get req = %s %s", m, p)
		}
		if a.ID != id {
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
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Renamed","code":"USD"}`))
		}))
		defer srv.Close()

		a, err := newTestAssetsFacade(t, srv).Update(context.Background(), assetsOrgID, assetsLedgerID, id, &models.UpdateAssetInput{Name: "Renamed"})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if m != http.MethodPatch || p != assetsBase()+"/"+id {
			t.Fatalf("update req = %s %s", m, p)
		}
		if !strings.Contains(body, `"name":"Renamed"`) {
			t.Fatalf("body = %q, want marshaled UpdateAssetInput", body)
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

		if err := newTestAssetsFacade(t, srv).Delete(context.Background(), assetsOrgID, assetsLedgerID, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if m != http.MethodDelete || p != assetsBase()+"/"+id {
			t.Fatalf("delete req = %s %s", m, p)
		}
	})
}

// TestAssetsFacade_ErrorDecodes asserts RFC 9457 decode with request-ID.
func TestAssetsFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-asset-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0009","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestAssetsFacade(t, srv).Create(context.Background(), assetsOrgID, assetsLedgerID, &models.CreateAssetInput{
		Name: "US Dollar", Code: "USD", Type: "currency",
	})
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0009" || sdkErr.RequestID != "req-asset-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestAssetsFacade_Filters is the per-resource differentiator: the ledger OAS
// omits code/type/status from ListAssetsParams, so all three must be injected
// via request editors. A regression that drops any filter (or fails to inject
// it) surfaces here.
func TestAssetsFacade_Filters(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":7}`))
	}))
	defer srv.Close()

	_, err := newTestAssetsFacade(t, srv).List(context.Background(), assetsOrgID, assetsLedgerID, models.AssetsListOpts{
		PageListOpts: models.PageListOpts{Limit: 7},
		Filters:      models.AssetsFilters{Code: "USD", Type: "currency", Status: "ACTIVE"},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := q.Get("code"); got != "USD" {
		t.Fatalf("code = %q, want USD (editor must inject it)", got)
	}
	if got := q.Get("type"); got != "currency" {
		t.Fatalf("type = %q, want currency (editor must inject it)", got)
	}
	if got := q.Get("status"); got != "ACTIVE" {
		t.Fatalf("status = %q, want ACTIVE (editor must inject it)", got)
	}
	if got := q.Get("limit"); got != "7" {
		t.Fatalf("limit = %q, want 7 (editor must preserve params)", got)
	}
}

// TestAssetsFacade_WriteReplaySafe is the money-path 401-replay guard.
func TestAssetsFacade_WriteReplaySafe(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"33333333-3333-3333-3333-333333333333","name":"US Dollar","code":"USD"}`))
	}))
	defer srv.Close()

	_, err := newTestAssetsFacade(t, srv).Create(context.Background(), assetsOrgID, assetsLedgerID, &models.CreateAssetInput{
		Name: "US Dollar", Code: "USD", Type: "currency",
	})
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"code":"USD"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

// TestAssetsFacade_Count HEADs the metrics/count endpoint and reads the total
// from X-Total-Count.
func TestAssetsFacade_Count(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set(HeaderTotalCount, "5")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n, err := newTestAssetsFacade(t, srv).Count(context.Background(), assetsOrgID, assetsLedgerID)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if gotMethod != http.MethodHead {
		t.Fatalf("method = %s, want HEAD", gotMethod)
	}
	if want := assetsBase() + "/metrics/count"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
	if n != 5 {
		t.Fatalf("count = %d, want 5", n)
	}
}

// TestAssetsFacade_CountErrorEmptyBody proves the readCount error path maps a
// headers-only 403 (JSON content-type, empty body) to authorization, not
// internal.
func TestAssetsFacade_CountErrorEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := newTestAssetsFacade(t, srv).Count(context.Background(), assetsOrgID, assetsLedgerID)
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

// TestAssetsFacade_CreateValidation proves Create runs input.Validate() before
// touching the wire: an empty input (missing name/code/type) must fail locally,
// so the server is never hit.
func TestAssetsFacade_CreateValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("server must not be hit on invalid input")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if _, err := newTestAssetsFacade(t, srv).Create(context.Background(), assetsOrgID, assetsLedgerID, &models.CreateAssetInput{}); err == nil {
		t.Fatal("expected validation error for empty input")
	}
}

func newTestAssetsFacade(t *testing.T, srv *httptest.Server) *assetsFacade {
	t.Helper()
	return newAssetsFacade(newTestLedgerClient(t, srv), true)
}

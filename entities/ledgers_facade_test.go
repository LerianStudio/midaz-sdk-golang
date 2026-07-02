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

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

const ledgersOrgID = "11111111-1111-1111-1111-111111111111"

// TestLedgersFacade_ListAndPaginate exercises the List/All trinaldo end-to-end
// over the generated client, chaining two org-scoped pages via next_cursor.
func TestLedgersFacade_ListAndPaginate(t *testing.T) {
	page1 := `{"items":[{"id":"44444444-4444-4444-4444-444444444444","name":"Ledger One"}],"limit":1,"next_cursor":"cursor-2"}`
	page2 := `{"items":[{"id":"55555555-5555-5555-5555-555555555555","name":"Ledger Two"}],"limit":1}`

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(page2))
		} else {
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer srv.Close()

	facade := newTestLedgersFacade(t, srv)

	first, err := facade.List(context.Background(), ledgersOrgID, models.LedgersListOpts{PageListOpts: models.PageListOpts{Limit: 1}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotPath != "/v1/organizations/"+ledgersOrgID+"/ledgers" {
		t.Fatalf("path = %q, want org-scoped ledgers path", gotPath)
	}
	if len(first.Items) != 1 || first.Items[0].Name != "Ledger One" {
		t.Fatalf("List page 1 = %+v, want single Ledger One", first.Items)
	}
	if !first.Pagination.HasMore() {
		t.Fatalf("List page 1 must report HasMore via next_cursor")
	}

	all, err := CollectAll(facade.All(context.Background(), ledgersOrgID, models.LedgersListOpts{PageListOpts: models.PageListOpts{Limit: 1}}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].Name != "Ledger One" || all[1].Name != "Ledger Two" {
		t.Fatalf("All = %+v, want [Ledger One, Ledger Two]", all)
	}
}

// TestLedgersFacade_CRUD round-trips Create/Get/Update/Delete over the generated
// client, asserting method, org-scoped path, and body, and normalizing into the
// public model without leaking generated types.
func TestLedgersFacade_CRUD(t *testing.T) {
	const id = "33333333-3333-3333-3333-333333333333"
	base := "/v1/organizations/" + ledgersOrgID + "/ledgers"

	t.Run("create", func(t *testing.T) {
		var m, p, ct, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p, ct = r.Method, r.URL.Path, r.Header.Get("Content-Type")
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Treasury"}`))
		}))
		defer srv.Close()

		l, err := newTestLedgersFacade(t, srv).Create(context.Background(), ledgersOrgID, &models.CreateLedgerInput{Name: "Treasury"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m != http.MethodPost || p != base || ct != "application/json" {
			t.Fatalf("create req = %s %s (%s), want POST %s json", m, p, ct, base)
		}
		if !strings.Contains(body, `"name":"Treasury"`) {
			t.Fatalf("body = %q, want marshaled CreateLedgerInput", body)
		}
		if l.ID != id || l.Name != "Treasury" {
			t.Fatalf("Create returned %+v", l)
		}
	})

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Treasury"}`))
		}))
		defer srv.Close()

		l, err := newTestLedgersFacade(t, srv).Get(context.Background(), ledgersOrgID, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if m != http.MethodGet || p != base+"/"+id {
			t.Fatalf("get req = %s %s, want GET %s/%s", m, p, base, id)
		}
		if l.ID != id {
			t.Fatalf("Get returned %+v", l)
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

		l, err := newTestLedgersFacade(t, srv).Update(context.Background(), ledgersOrgID, id, &models.UpdateLedgerInput{Name: "Renamed"})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if m != http.MethodPatch || p != base+"/"+id {
			t.Fatalf("update req = %s %s, want PATCH %s/%s", m, p, base, id)
		}
		if !strings.Contains(body, `"name":"Renamed"`) {
			t.Fatalf("body = %q, want marshaled UpdateLedgerInput", body)
		}
		if l.Name != "Renamed" {
			t.Fatalf("Update returned %+v", l)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		if err := newTestLedgersFacade(t, srv).Delete(context.Background(), ledgersOrgID, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if m != http.MethodDelete || p != base+"/"+id {
			t.Fatalf("delete req = %s %s, want DELETE %s/%s", m, p, base, id)
		}
	})
}

// TestLedgersFacade_ErrorDecodes asserts an RFC 9457 problem body decodes into
// *errors.Error with code, status, and request-ID, never leaking generated types.
func TestLedgersFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-led-503")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0178","title":"Service Unavailable","status":503}`))
	}))
	defer srv.Close()

	_, err := newTestLedgersFacade(t, srv).List(context.Background(), ledgersOrgID, models.LedgersListOpts{})
	sdkErr, ok := err.(*sdkerrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0178" || sdkErr.StatusCode != 503 || sdkErr.RequestID != "req-led-503" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestLedgersFacade_Filters guards the params/editor split: Name+Status map to
// generated query slots, IncludeDeleted is injected via a request editor
// (no param slot), and none clobber pagination.
func TestLedgersFacade_Filters(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":5}`))
	}))
	defer srv.Close()

	_, err := newTestLedgersFacade(t, srv).List(context.Background(), ledgersOrgID, models.LedgersListOpts{
		PageListOpts: models.PageListOpts{Limit: 5},
		Filters:      models.LedgersFilters{Name: "Treasury", Status: "ACTIVE", IncludeDeleted: true},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := q.Get("name"); got != "Treasury" {
		t.Fatalf("name = %q, want Treasury", got)
	}
	if got := q.Get("status"); got != "ACTIVE" {
		t.Fatalf("status = %q, want ACTIVE", got)
	}
	if got := q.Get("include_deleted"); got != "true" {
		t.Fatalf("include_deleted = %q, want true (editor must inject it)", got)
	}
	if got := q.Get("limit"); got != "5" {
		t.Fatalf("limit = %q, want 5 (editor must preserve params)", got)
	}
}

// TestLedgersFacade_WriteReplaySafe is the money-path guard: after a 401 the
// auth round tripper must rewind and replay the JSON body (GetBody set by the
// concrete reader), so the replayed write is not empty.
func TestLedgersFacade_WriteReplaySafe(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"33333333-3333-3333-3333-333333333333","name":"Treasury"}`))
	}))
	defer srv.Close()

	_, err := newTestLedgersFacade(t, srv).Create(context.Background(), ledgersOrgID, &models.CreateLedgerInput{Name: "Treasury"})
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2 (401 must trigger replay)", attempts)
	}
	if !strings.Contains(replayed, `"name":"Treasury"`) {
		t.Fatalf("replayed body = %q, want full JSON (non-rewindable body dropped it)", replayed)
	}
}

// TestLedgersFacade_Settings exercises the tri-block ledger settings surface
// end-to-end. GetSettings must expose all three blocks (accounting including
// requireHolder, overrides, tracer); UpdateSettings must send all three blocks
// on the PATCH body and normalize the response into the public model.
func TestLedgersFacade_Settings(t *testing.T) {
	const id = "33333333-3333-3333-3333-333333333333"
	path := "/v1/organizations/" + ledgersOrgID + "/ledgers/" + id + "/settings"

	full := `{"accounting":{"requireHolder":true,"validateAccountType":true,"validateRoutes":false},` +
		`"overrides":{"allowFeeSkip":true,"allowHolderSkip":false,"allowTracerSkip":true},` +
		`"tracer":{"failPosture":"strict","mode":"sync","timeoutMs":1500}}`

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(full))
		}))
		defer srv.Close()

		s, err := newTestLedgersFacade(t, srv).GetSettings(context.Background(), ledgersOrgID, id)
		if err != nil {
			t.Fatalf("GetSettings: %v", err)
		}
		if m != http.MethodGet || p != path {
			t.Fatalf("get req = %s %s, want GET %s", m, p, path)
		}
		if !s.Accounting.RequireHolder || !s.Accounting.ValidateAccountType || s.Accounting.ValidateRoutes {
			t.Fatalf("accounting block = %+v, want requireHolder+validateAccountType, not validateRoutes", s.Accounting)
		}
		if !s.Overrides.AllowFeeSkip || s.Overrides.AllowHolderSkip || !s.Overrides.AllowTracerSkip {
			t.Fatalf("overrides block = %+v, want fee+tracer skip, not holder skip", s.Overrides)
		}
		if s.Tracer.FailPosture != "strict" || s.Tracer.Mode != "sync" || s.Tracer.TimeoutMs != 1500 {
			t.Fatalf("tracer block = %+v, want strict/sync/1500", s.Tracer)
		}
	})

	t.Run("update", func(t *testing.T) {
		var m, p, ct, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p, ct = r.Method, r.URL.Path, r.Header.Get("Content-Type")
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(full))
		}))
		defer srv.Close()

		input := models.NewUpdateLedgerSettingsInput().
			WithRequireHolder(true).
			WithValidateAccountType(true).
			WithAllowFeeSkip(true).
			WithAllowTracerSkip(true).
			WithTracerMode("sync").
			WithTracerFailPosture("strict").
			WithTracerTimeoutMs(1500)

		s, err := newTestLedgersFacade(t, srv).UpdateSettings(context.Background(), ledgersOrgID, id, input)
		if err != nil {
			t.Fatalf("UpdateSettings: %v", err)
		}
		if m != http.MethodPatch || p != path || ct != "application/json" {
			t.Fatalf("update req = %s %s (%s), want PATCH %s json", m, p, ct, path)
		}
		if !strings.Contains(body, `"accounting"`) {
			t.Fatalf("body = %q, want accounting block", body)
		}
		if !strings.Contains(body, `"overrides"`) {
			t.Fatalf("body = %q, want overrides block", body)
		}
		if !strings.Contains(body, `"tracer"`) {
			t.Fatalf("body = %q, want tracer block", body)
		}
		if s.Tracer.Mode != "sync" || !s.Overrides.AllowFeeSkip {
			t.Fatalf("UpdateSettings returned %+v", s)
		}
	})
}

func newTestLedgersFacade(t *testing.T, srv *httptest.Server) *ledgersFacade {
	t.Helper()
	return newLedgersFacade(newTestLedgerClient(t, srv))
}

// newTestLedgerClient builds a ledger plane client pointed at the test server
// with a static Bearer token. Shared by the Phase 2 facade tests.
func newTestLedgerClient(t *testing.T, srv *httptest.Server) *genledger.ClientWithResponses {
	t.Helper()

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: srv.URL + "/v1",
		tracerURL: srv.URL + "/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-1", nil },
		},
		httpClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("newPlaneClients: %v", err)
	}

	return planes.Ledger
}

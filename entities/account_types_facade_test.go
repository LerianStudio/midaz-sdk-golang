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
	accountTypesOrgID    = "11111111-1111-1111-1111-111111111111"
	accountTypesLedgerID = "22222222-2222-2222-2222-222222222222"
)

func accountTypesBase() string {
	return "/v1/organizations/" + accountTypesOrgID + "/ledgers/" + accountTypesLedgerID + "/account-types"
}

// TestAccountTypesFacade_ListAndPaginate exercises the List/All trinaldo
// end-to-end over the generated client, chaining two org+ledger-scoped pages.
func TestAccountTypesFacade_ListAndPaginate(t *testing.T) {
	page1 := `{"items":[{"id":"33333333-3333-3333-3333-333333333333","name":"Cash","keyValue":"CASH"}],"limit":1,"next_cursor":"c2"}`
	page2 := `{"items":[{"id":"44444444-4444-4444-4444-444444444444","name":"Receivables","keyValue":"AR"}],"limit":1}`

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

	facade := newTestAccountTypesFacade(t, srv)

	first, err := facade.List(context.Background(), accountTypesOrgID, accountTypesLedgerID, models.AccountTypesListOpts{PageListOpts: models.PageListOpts{Limit: 1}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotPath != accountTypesBase() {
		t.Fatalf("path = %q, want %q", gotPath, accountTypesBase())
	}
	if len(first.Items) != 1 || first.Items[0].KeyValue != "CASH" {
		t.Fatalf("List page 1 = %+v", first.Items)
	}

	all, err := CollectAll(facade.All(context.Background(), accountTypesOrgID, accountTypesLedgerID, models.AccountTypesListOpts{PageListOpts: models.PageListOpts{Limit: 1}}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].KeyValue != "CASH" || all[1].KeyValue != "AR" {
		t.Fatalf("All = %+v", all)
	}
}

// TestAccountTypesFacade_CRUD round-trips Create/Get/Update/Delete over the
// generated client on the org+ledger-scoped path. Get also asserts the
// uuid.UUID response fields decode straight into the public model.
//
//nolint:revive // cognitive-complexity: four CRUD subtests, each with its own httptest server closure and assertions; the complexity is the subtest count, not branching logic. Matches the repo's per-test convention.
func TestAccountTypesFacade_CRUD(t *testing.T) {
	const id = "55555555-5555-5555-5555-555555555555"

	t.Run("create", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Cash","keyValue":"CASH"}`))
		}))
		defer srv.Close()

		at, err := newTestAccountTypesFacade(t, srv).Create(context.Background(), accountTypesOrgID, accountTypesLedgerID, &models.CreateAccountTypeInput{
			Name: "Cash", KeyValue: "CASH",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m != http.MethodPost || p != accountTypesBase() {
			t.Fatalf("create req = %s %s, want POST %s", m, p, accountTypesBase())
		}
		if !strings.Contains(body, `"keyValue":"CASH"`) || !strings.Contains(body, `"name":"Cash"`) {
			t.Fatalf("body = %q, want marshaled CreateAccountTypeInput", body)
		}
		if at.ID.String() != id || at.KeyValue != "CASH" {
			t.Fatalf("Create returned %+v", at)
		}
	})

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Cash","keyValue":"CASH"}`))
		}))
		defer srv.Close()

		at, err := newTestAccountTypesFacade(t, srv).Get(context.Background(), accountTypesOrgID, accountTypesLedgerID, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if m != http.MethodGet || p != accountTypesBase()+"/"+id {
			t.Fatalf("get req = %s %s", m, p)
		}
		if at.ID.String() != id {
			t.Fatalf("Get returned %+v", at)
		}
	})

	t.Run("update", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Renamed","keyValue":"CASH"}`))
		}))
		defer srv.Close()

		at, err := newTestAccountTypesFacade(t, srv).Update(context.Background(), accountTypesOrgID, accountTypesLedgerID, id, models.NewUpdateAccountTypeInput().WithName("Renamed"))
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if m != http.MethodPatch || p != accountTypesBase()+"/"+id {
			t.Fatalf("update req = %s %s", m, p)
		}
		if !strings.Contains(body, `"name":"Renamed"`) {
			t.Fatalf("body = %q, want marshaled UpdateAccountTypeInput", body)
		}
		if at.Name != "Renamed" {
			t.Fatalf("Update returned %+v", at)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		if err := newTestAccountTypesFacade(t, srv).Delete(context.Background(), accountTypesOrgID, accountTypesLedgerID, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if m != http.MethodDelete || p != accountTypesBase()+"/"+id {
			t.Fatalf("delete req = %s %s", m, p)
		}
	})
}

// TestAccountTypesFacade_ErrorDecodes asserts RFC 9457 decode with request-ID.
func TestAccountTypesFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-at-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0009","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestAccountTypesFacade(t, srv).Create(context.Background(), accountTypesOrgID, accountTypesLedgerID, &models.CreateAccountTypeInput{
		Name: "Cash", KeyValue: "CASH",
	})
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0009" || sdkErr.RequestID != "req-at-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestAccountTypesFacade_Filters is the per-resource differentiator. KeyValue
// maps to a generated param slot; name and include_deleted have no slot and
// must be injected via request editors. A regression that drops any filter
// surfaces here.
func TestAccountTypesFacade_Filters(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":7}`))
	}))
	defer srv.Close()

	_, err := newTestAccountTypesFacade(t, srv).List(context.Background(), accountTypesOrgID, accountTypesLedgerID, models.AccountTypesListOpts{
		PageListOpts: models.PageListOpts{Limit: 7},
		Filters:      models.AccountTypesFilters{Name: "Cash", KeyValue: "CASH", IncludeDeleted: true},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := q.Get("key_value"); got != "CASH" {
		t.Fatalf("key_value = %q, want CASH (param slot)", got)
	}
	if got := q.Get("name"); got != "Cash" {
		t.Fatalf("name = %q, want Cash (editor must inject it)", got)
	}
	if got := q.Get("include_deleted"); got != "true" {
		t.Fatalf("include_deleted = %q, want true (editor must inject it)", got)
	}
	if got := q.Get("limit"); got != "7" {
		t.Fatalf("limit = %q, want 7 (editor must preserve params)", got)
	}
}

// TestAccountTypesFacade_WriteReplaySafe is the money-path 401-replay guard:
// the write body must survive the auth round tripper's post-401 replay.
func TestAccountTypesFacade_WriteReplaySafe(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"55555555-5555-5555-5555-555555555555","name":"Cash","keyValue":"CASH"}`))
	}))
	defer srv.Close()

	_, err := newTestAccountTypesFacade(t, srv).Create(context.Background(), accountTypesOrgID, accountTypesLedgerID, &models.CreateAccountTypeInput{
		Name: "Cash", KeyValue: "CASH",
	})
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"keyValue":"CASH"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

func newTestAccountTypesFacade(t *testing.T, srv *httptest.Server) *accountTypesFacade {
	t.Helper()
	return newAccountTypesFacade(newTestLedgerClient(t, srv), true)
}

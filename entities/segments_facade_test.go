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
	segmentsOrgID    = "11111111-1111-1111-1111-111111111111"
	segmentsLedgerID = "22222222-2222-2222-2222-222222222222"
)

func segmentsBase() string {
	return "/v1/organizations/" + segmentsOrgID + "/ledgers/" + segmentsLedgerID + "/segments"
}

// TestSegmentsFacade_ListAndPaginate exercises the List/All trinaldo end-to-end.
func TestSegmentsFacade_ListAndPaginate(t *testing.T) {
	page1 := `{"items":[{"id":"44444444-4444-4444-4444-444444444444","name":"North"}],"limit":1,"next_cursor":"c2"}`
	page2 := `{"items":[{"id":"55555555-5555-5555-5555-555555555555","name":"South"}],"limit":1}`

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

	facade := newTestSegmentsFacade(t, srv)

	first, err := facade.List(context.Background(), segmentsOrgID, segmentsLedgerID, models.SegmentsListOpts{PageListOpts: models.PageListOpts{Limit: 1}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotPath != segmentsBase() {
		t.Fatalf("path = %q, want %q", gotPath, segmentsBase())
	}
	if len(first.Items) != 1 || first.Items[0].Name != "North" {
		t.Fatalf("List page 1 = %+v", first.Items)
	}

	all, err := CollectAll(facade.All(context.Background(), segmentsOrgID, segmentsLedgerID, models.SegmentsListOpts{PageListOpts: models.PageListOpts{Limit: 1}}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].Name != "North" || all[1].Name != "South" {
		t.Fatalf("All = %+v", all)
	}
}

// TestSegmentsFacade_CRUD round-trips Create/Get/Update/Delete.
func TestSegmentsFacade_CRUD(t *testing.T) {
	const id = "33333333-3333-3333-3333-333333333333"

	t.Run("create", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"North"}`))
		}))
		defer srv.Close()

		s, err := newTestSegmentsFacade(t, srv).Create(context.Background(), segmentsOrgID, segmentsLedgerID, &models.CreateSegmentInput{Name: "North"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m != http.MethodPost || p != segmentsBase() {
			t.Fatalf("create req = %s %s", m, p)
		}
		if !strings.Contains(body, `"name":"North"`) {
			t.Fatalf("body = %q, want marshaled CreateSegmentInput", body)
		}
		if s.ID != id || s.Name != "North" {
			t.Fatalf("Create returned %+v", s)
		}
	})

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"North"}`))
		}))
		defer srv.Close()

		s, err := newTestSegmentsFacade(t, srv).Get(context.Background(), segmentsOrgID, segmentsLedgerID, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if m != http.MethodGet || p != segmentsBase()+"/"+id {
			t.Fatalf("get req = %s %s", m, p)
		}
		if s.ID != id {
			t.Fatalf("Get returned %+v", s)
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

		s, err := newTestSegmentsFacade(t, srv).Update(context.Background(), segmentsOrgID, segmentsLedgerID, id, &models.UpdateSegmentInput{Name: "Renamed"})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if m != http.MethodPatch || p != segmentsBase()+"/"+id {
			t.Fatalf("update req = %s %s", m, p)
		}
		if !strings.Contains(body, `"name":"Renamed"`) {
			t.Fatalf("body = %q, want marshaled UpdateSegmentInput", body)
		}
		if s.Name != "Renamed" {
			t.Fatalf("Update returned %+v", s)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		if err := newTestSegmentsFacade(t, srv).Delete(context.Background(), segmentsOrgID, segmentsLedgerID, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if m != http.MethodDelete || p != segmentsBase()+"/"+id {
			t.Fatalf("delete req = %s %s", m, p)
		}
	})
}

// TestSegmentsFacade_ErrorDecodes asserts RFC 9457 decode with request-ID.
func TestSegmentsFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-seg-409")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0084","title":"Conflict","status":409}`))
	}))
	defer srv.Close()

	_, err := newTestSegmentsFacade(t, srv).Get(context.Background(), segmentsOrgID, segmentsLedgerID, "33333333-3333-3333-3333-333333333333")
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0084" || sdkErr.RequestID != "req-seg-409" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestSegmentsFacade_Filters guards the all-editor case: the ledger OAS omits
// name/status/include_deleted from ListSegmentsParams, so all three filters must
// be injected via request editors. A naive params-only mapping drops every
// filter — this test catches that.
func TestSegmentsFacade_Filters(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":3}`))
	}))
	defer srv.Close()

	_, err := newTestSegmentsFacade(t, srv).List(context.Background(), segmentsOrgID, segmentsLedgerID, models.SegmentsListOpts{
		PageListOpts: models.PageListOpts{Limit: 3},
		Filters:      models.SegmentsFilters{Name: "North", Status: "ACTIVE", IncludeDeleted: true},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := q.Get("name"); got != "North" {
		t.Fatalf("name = %q, want North (editor must inject it)", got)
	}
	if got := q.Get("status"); got != "ACTIVE" {
		t.Fatalf("status = %q, want ACTIVE (editor must inject it)", got)
	}
	if got := q.Get("include_deleted"); got != "true" {
		t.Fatalf("include_deleted = %q, want true (editor must inject it)", got)
	}
	if got := q.Get("limit"); got != "3" {
		t.Fatalf("limit = %q, want 3 (editor must preserve params)", got)
	}
}

// TestSegmentsFacade_WriteReplaySafe is the money-path 401-replay guard.
func TestSegmentsFacade_WriteReplaySafe(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"33333333-3333-3333-3333-333333333333","name":"North"}`))
	}))
	defer srv.Close()

	_, err := newTestSegmentsFacade(t, srv).Create(context.Background(), segmentsOrgID, segmentsLedgerID, &models.CreateSegmentInput{Name: "North"})
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"name":"North"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

func newTestSegmentsFacade(t *testing.T, srv *httptest.Server) *segmentsFacade {
	t.Helper()
	return newSegmentsFacade(newTestLedgerClient(t, srv))
}

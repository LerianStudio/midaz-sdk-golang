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
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

const holdersFacadeOrgID = "11111111-1111-1111-1111-111111111111"

func holdersFacadeBase() string {
	return "/v1/organizations/" + holdersFacadeOrgID + "/holders"
}

// TestHoldersFacade_ListAndPaginate exercises cursor List/Pages/All end-to-end
// over the generated ledger-plane client: two cursor pages chained by echoing
// next_cursor, stopping on the terminal page's empty cursor. A HasMore()-based
// stop would loop forever on a full terminal page carrying no cursor.
func TestHoldersFacade_ListAndPaginate(t *testing.T) {
	page1 := `{"items":[{"id":"33333333-3333-3333-3333-333333333333","name":"Alice"}],"limit":1,"next_cursor":"c2"}`
	page2 := `{"items":[{"id":"44444444-4444-4444-4444-444444444444","name":"Bob"}],"limit":1}`

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

	facade := newTestHoldersFacade(t, srv)

	all, err := CollectAll(facade.ListAll(context.Background(), holdersFacadeOrgID, models.HoldersListOpts{
		PageListOpts: models.PageListOpts{Limit: 1},
	}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].Name == nil || *all[0].Name != "Alice" || all[1].Name == nil || *all[1].Name != "Bob" {
		t.Fatalf("All = %+v", all)
	}
	if len(seenCursors) != 2 || seenCursors[0] != "" || seenCursors[1] != "c2" {
		t.Fatalf("cursor chain = %v, want ['' 'c2'] and exactly 2 requests", seenCursors)
	}
}

// TestHoldersFacade_CRUD round-trips Create/Get/Update/Delete over the generated
// client on the org-in-path ledger-plane route.
//
//nolint:revive // cognitive-complexity: four CRUD subtests, each with its own httptest server closure and assertions; matches the repo's per-test convention.
func TestHoldersFacade_CRUD(t *testing.T) {
	const id = "55555555-5555-5555-5555-555555555555"

	t.Run("create", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Alice","document":"123"}`))
		}))
		defer srv.Close()

		holder, err := newTestHoldersFacade(t, srv).Create(context.Background(), holdersFacadeOrgID,
			models.NewCreateHolderInput(models.HolderTypeNaturalPerson, "Alice", "123"))
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m != http.MethodPost || p != holdersFacadeBase() {
			t.Fatalf("create req = %s %s, want POST %s", m, p, holdersFacadeBase())
		}
		if !strings.Contains(body, `"name":"Alice"`) || !strings.Contains(body, `"document":"123"`) {
			t.Fatalf("body = %q, want marshaled CreateHolderInput", body)
		}
		if holder.ID.String() != id || holder.Name == nil || *holder.Name != "Alice" {
			t.Fatalf("Create returned %+v", holder)
		}
	})

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Alice"}`))
		}))
		defer srv.Close()

		holder, err := newTestHoldersFacade(t, srv).Get(context.Background(), holdersFacadeOrgID, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if m != http.MethodGet || p != holdersFacadeBase()+"/"+id {
			t.Fatalf("get req = %s %s", m, p)
		}
		if holder.ID.String() != id {
			t.Fatalf("Get returned %+v", holder)
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

		holder, err := newTestHoldersFacade(t, srv).Update(context.Background(), holdersFacadeOrgID, id,
			models.NewUpdateHolderInput().WithName("Renamed"))
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if m != http.MethodPatch || p != holdersFacadeBase()+"/"+id {
			t.Fatalf("update req = %s %s", m, p)
		}
		if !strings.Contains(body, `"name":"Renamed"`) {
			t.Fatalf("body = %q, want marshaled UpdateHolderInput", body)
		}
		if holder.Name == nil || *holder.Name != "Renamed" {
			t.Fatalf("Update returned %+v", holder)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		if err := newTestHoldersFacade(t, srv).Delete(context.Background(), holdersFacadeOrgID, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if m != http.MethodDelete || p != holdersFacadeBase()+"/"+id {
			t.Fatalf("delete req = %s %s", m, p)
		}
	})
}

// TestHoldersFacade_Filters asserts native param slots (limit, sort_order,
// external_id, document) and the editor-injected filters (name, status) that
// the generated ListHoldersParams has no slot for.
func TestHoldersFacade_Filters(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":7}`))
	}))
	defer srv.Close()

	_, err := newTestHoldersFacade(t, srv).List(context.Background(), holdersFacadeOrgID, models.HoldersListOpts{
		PageListOpts: models.PageListOpts{Limit: 7, SortDirection: models.SortAscending},
		Filters: models.HoldersFilters{
			Name:       "Alice",
			Document:   "123",
			Status:     "ACTIVE",
			ExternalID: "ext-1",
		},
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
	if got := q.Get("document"); got != "123" {
		t.Fatalf("document = %q, want 123 (param slot)", got)
	}
	if got := q.Get("external_id"); got != "ext-1" {
		t.Fatalf("external_id = %q, want ext-1 (param slot)", got)
	}
	if got := q.Get("name"); got != "Alice" {
		t.Fatalf("name = %q, want Alice (editor must inject it)", got)
	}
	if got := q.Get("status"); got != "ACTIVE" {
		t.Fatalf("status = %q, want ACTIVE (editor must inject it)", got)
	}
}

// TestHoldersFacade_IncludeDeletedAndHardDelete asserts the context-driven query
// flags: include_deleted=true on Get when the context is tagged, and
// hard_delete=true on Delete when the context is tagged.
func TestHoldersFacade_IncludeDeletedAndHardDelete(t *testing.T) {
	const id = "55555555-5555-5555-5555-555555555555"

	t.Run("get include_deleted", func(t *testing.T) {
		var q url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Alice"}`))
		}))
		defer srv.Close()

		ctx := sdkctx.WithIncludeDeleted(context.Background(), true)
		if _, err := newTestHoldersFacade(t, srv).Get(ctx, holdersFacadeOrgID, id); err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got := q.Get("include_deleted"); got != "true" {
			t.Fatalf("include_deleted = %q, want true", got)
		}
	})

	t.Run("delete hard_delete", func(t *testing.T) {
		var q url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q = r.URL.Query()
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		ctx := sdkctx.WithHardDelete(context.Background(), true)
		if err := newTestHoldersFacade(t, srv).Delete(ctx, holdersFacadeOrgID, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if got := q.Get("hard_delete"); got != "true" {
			t.Fatalf("hard_delete = %q, want true", got)
		}
	})
}

// TestHoldersFacade_ErrorDecodes asserts RFC 9457 decode with request-ID
// correlation and that a non-2xx surfaces as *errors.Error.
func TestHoldersFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-hd-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0009","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestHoldersFacade(t, srv).Create(context.Background(), holdersFacadeOrgID,
		models.NewCreateHolderInput(models.HolderTypeNaturalPerson, "Alice", "123"))
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0009" || sdkErr.RequestID != "req-hd-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestHoldersFacade_WriteReplaySafe is the 401-replay guard: the write body must
// survive the auth round tripper's post-401 replay (rewindable *bytes.Reader).
func TestHoldersFacade_WriteReplaySafe(t *testing.T) {
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
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"55555555-5555-5555-5555-555555555555","name":"Alice"}`))
	}))
	defer srv.Close()

	_, err := newTestHoldersFacade(t, srv).Create(context.Background(), holdersFacadeOrgID,
		models.NewCreateHolderInput(models.HolderTypeNaturalPerson, "Alice", "123"))
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"name":"Alice"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

// TestHoldersFacade_ListPagesContextCanceled covers the ctx.Err() escape hatch
// (holders_facade.go ~L96-99): an already-cancelled context makes ListPages
// yield context.Canceled and stop before touching the wire — no request reaches
// the server.
func TestHoldersFacade_ListPagesContextCanceled(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":1}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var yielded error
	var pages int
	for _, err := range newTestHoldersFacade(t, srv).ListPages(ctx, holdersFacadeOrgID, models.HoldersListOpts{
		PageListOpts: models.PageListOpts{Limit: 1},
	}) {
		pages++
		yielded = err
	}

	if pages != 1 {
		t.Fatalf("iterations = %d, want exactly 1 (the ctx.Err() yield)", pages)
	}
	if !errors.Is(yielded, context.Canceled) {
		t.Fatalf("yielded err = %v, want context.Canceled", yielded)
	}
	if requests != 0 {
		t.Fatalf("server requests = %d, want 0 (cancelled ctx must short-circuit before the wire)", requests)
	}
}

// TestHoldersFacade_ListPagesConsumerStops covers the `if !yield(page,nil)
// { return }` branch (holders_facade.go ~L107): a consumer that returns false
// after the first page must stop the iterator immediately. The first page
// carries a next_cursor, so a regression ignoring yield's return would fetch a
// second page — asserted absent by counting server requests.
func TestHoldersFacade_ListPagesConsumerStops(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		// Non-empty next_cursor: the iterator would fetch again if the consumer
		// did not stop it.
		_, _ = w.Write([]byte(`{"items":[{"id":"33333333-3333-3333-3333-333333333333","name":"Alice"}],"limit":1,"next_cursor":"c2"}`))
	}))
	defer srv.Close()

	var pages int
	for page, err := range newTestHoldersFacade(t, srv).ListPages(context.Background(), holdersFacadeOrgID, models.HoldersListOpts{
		PageListOpts: models.PageListOpts{Limit: 1},
	}) {
		if err != nil {
			t.Fatalf("unexpected err on first page: %v", err)
		}
		if page == nil || len(page.Items) != 1 {
			t.Fatalf("first page = %+v", page)
		}
		pages++
		break // yield false -> iterator must return without fetching page 2
	}

	if pages != 1 {
		t.Fatalf("consumed pages = %d, want 1", pages)
	}
	if requests != 1 {
		t.Fatalf("server requests = %d, want exactly 1 (consumer stop must not leak a second fetch)", requests)
	}
}

func newTestHoldersFacade(t *testing.T, srv *httptest.Server) *holdersFacade {
	t.Helper()
	return newHoldersFacade(newTestLedgerClient(t, srv))
}

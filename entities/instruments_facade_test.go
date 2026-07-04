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

const (
	instrumentsFacadeOrgID    = "11111111-1111-1111-1111-111111111111"
	instrumentsFacadeHolderID = "22222222-2222-2222-2222-222222222222"
)

// instrumentsListBase is the org-scoped list endpoint. The generated
// ListInstruments hits /organizations/{org}/instruments and scopes to a holder
// via the holder_id query param, NOT a holder-in-path segment.
func instrumentsListBase() string {
	return "/v1/organizations/" + instrumentsFacadeOrgID + "/instruments"
}

// instrumentsHolderBase is the holder-scoped write/read-by-id endpoint.
func instrumentsHolderBase() string {
	return "/v1/organizations/" + instrumentsFacadeOrgID + "/holders/" + instrumentsFacadeHolderID + "/instruments"
}

// TestInstrumentsFacade_ListAndPaginate exercises cursor List/ListPages/ListAll
// over the generated org-scoped list endpoint: two cursor pages chained by
// echoing next_cursor, stopping on the terminal page's empty cursor. A
// HasMore()-based stop would loop forever on a full terminal page carrying no
// cursor. Every request must carry holder_id (the list is org-scoped and the
// holder is a query filter, not a path segment).
func TestInstrumentsFacade_ListAndPaginate(t *testing.T) {
	page1 := `{"items":[{"id":"33333333-3333-3333-3333-333333333333","holderId":"` + instrumentsFacadeHolderID + `","type":"CHECKING"}],"limit":1,"next_cursor":"c2"}`
	page2 := `{"items":[{"id":"44444444-4444-4444-4444-444444444444","holderId":"` + instrumentsFacadeHolderID + `","type":"SAVINGS"}],"limit":1}`

	var seenCursors, seenHolderIDs, seenPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCursors = append(seenCursors, r.URL.Query().Get("cursor"))
		seenHolderIDs = append(seenHolderIDs, r.URL.Query().Get("holder_id"))
		seenPaths = append(seenPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "c2" {
			_, _ = w.Write([]byte(page2))
		} else {
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer srv.Close()

	facade := newTestInstrumentsFacade(t, srv)

	all, err := CollectAll(facade.ListAll(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, models.InstrumentsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 1},
	}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].Type == nil || *all[0].Type != "CHECKING" || all[1].Type == nil || *all[1].Type != "SAVINGS" {
		t.Fatalf("All = %+v", all)
	}
	if len(seenCursors) != 2 || seenCursors[0] != "" || seenCursors[1] != "c2" {
		t.Fatalf("cursor chain = %v, want ['' 'c2'] and exactly 2 requests", seenCursors)
	}
	for i, hid := range seenHolderIDs {
		if hid != instrumentsFacadeHolderID {
			t.Fatalf("request %d holder_id = %q, want %q", i, hid, instrumentsFacadeHolderID)
		}
	}
	for i, p := range seenPaths {
		if p != instrumentsListBase() {
			t.Fatalf("request %d path = %q, want %q (org-scoped list)", i, p, instrumentsListBase())
		}
	}
}

// TestInstrumentsFacade_ListSeedsCursorAndDates locks the two knobs the cursor
// migration restored. Before the migration InstrumentsListOpts embedded
// PageListOpts and the facade always seeded the cursor with "", so a
// caller-supplied cursor was ignored (no way to resume a mid-stream page) and
// StartDate/EndDate never reached the wire. Now opts.Cursor seeds the first
// request and start_date/end_date are injected as query params (the generated
// ListInstrumentsParams has no slot for any of the three). holder_id must still
// ride along on the same request.
func TestInstrumentsFacade_ListSeedsCursorRejectsDates(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":1}`))
	}))
	defer srv.Close()

	facade := newTestInstrumentsFacade(t, srv)

	// Cursor seeds the first request; no dates set, so Validate passes.
	_, err := facade.List(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, models.InstrumentsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 1, Cursor: "seed-cursor"},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := q.Get("cursor"); got != "seed-cursor" {
		t.Fatalf("cursor = %q, want seed-cursor (opts.Cursor must seed the first request)", got)
	}
	if got := q.Get("holder_id"); got != instrumentsFacadeHolderID {
		t.Fatalf("holder_id = %q, want %q (must ride along on the seeded request)", got, instrumentsFacadeHolderID)
	}
	if q.Has("start_date") || q.Has("end_date") {
		t.Fatalf("date params leaked to the wire: start_date=%q end_date=%q (instruments endpoint has no date slot)", q.Get("start_date"), q.Get("end_date"))
	}

	// A set date is rejected by Validate (NoDates) before any request.
	_, err = facade.List(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, models.InstrumentsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 1, StartDate: "2026-01-01"},
	})
	if err == nil {
		t.Fatal("List with StartDate: want a validation error (date filtering unsupported), got nil")
	}
}

// TestInstrumentsFacade_CRUD round-trips Create/Get/Update/Delete over the
// generated client on the holder-in-path ledger-plane route.
//
//nolint:revive // cognitive-complexity: four CRUD subtests, each with its own httptest server closure and assertions; matches the repo's per-test convention.
func TestInstrumentsFacade_CRUD(t *testing.T) {
	const id = "55555555-5555-5555-5555-555555555555"

	t.Run("create", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"` + id + `","holderId":"` + instrumentsFacadeHolderID + `","type":"CHECKING"}`))
		}))
		defer srv.Close()

		inst, err := newTestInstrumentsFacade(t, srv).Create(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID,
			models.NewCreateInstrumentInput("CHECKING").WithDocument("DOC-1"))
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m != http.MethodPost || p != instrumentsHolderBase() {
			t.Fatalf("create req = %s %s, want POST %s", m, p, instrumentsHolderBase())
		}
		if !strings.Contains(body, `"type":"CHECKING"`) || !strings.Contains(body, `"document":"DOC-1"`) {
			t.Fatalf("body = %q, want marshaled CreateInstrumentInput", body)
		}
		if inst.ID == nil || inst.ID.String() != id || inst.Type == nil || *inst.Type != "CHECKING" {
			t.Fatalf("Create returned %+v", inst)
		}
	})

	t.Run("get", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","holderId":"` + instrumentsFacadeHolderID + `","type":"CHECKING"}`))
		}))
		defer srv.Close()

		inst, err := newTestInstrumentsFacade(t, srv).Get(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if m != http.MethodGet || p != instrumentsHolderBase()+"/"+id {
			t.Fatalf("get req = %s %s", m, p)
		}
		if inst.ID == nil || inst.ID.String() != id {
			t.Fatalf("Get returned %+v", inst)
		}
	})

	t.Run("update", func(t *testing.T) {
		var m, p, body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + id + `","holderId":"` + instrumentsFacadeHolderID + `","type":"CHECKING","document":"DOC-9"}`))
		}))
		defer srv.Close()

		inst, err := newTestInstrumentsFacade(t, srv).Update(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, id,
			models.NewUpdateInstrumentInput().WithDocument("DOC-9"))
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if m != http.MethodPatch || p != instrumentsHolderBase()+"/"+id {
			t.Fatalf("update req = %s %s", m, p)
		}
		if !strings.Contains(body, `"document":"DOC-9"`) {
			t.Fatalf("body = %q, want marshaled UpdateInstrumentInput", body)
		}
		if inst.Document == nil || *inst.Document != "DOC-9" {
			t.Fatalf("Update returned %+v", inst)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var m, p string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, p = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		if err := newTestInstrumentsFacade(t, srv).Delete(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if m != http.MethodDelete || p != instrumentsHolderBase()+"/"+id {
			t.Fatalf("delete req = %s %s", m, p)
		}
	})
}

// TestInstrumentsFacade_DeleteHardDelete asserts the context-driven hard_delete
// query flag on Delete when the context is tagged.
func TestInstrumentsFacade_DeleteHardDelete(t *testing.T) {
	const id = "55555555-5555-5555-5555-555555555555"
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ctx := sdkctx.WithHardDelete(context.Background(), true)
	if err := newTestInstrumentsFacade(t, srv).Delete(ctx, instrumentsFacadeOrgID, instrumentsFacadeHolderID, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := q.Get("hard_delete"); got != "true" {
		t.Fatalf("hard_delete = %q, want true", got)
	}
}

// TestInstrumentsFacade_DeleteRelatedParty round-trips the nested
// related-parties delete on the holder-in-path route.
func TestInstrumentsFacade_DeleteRelatedParty(t *testing.T) {
	const (
		instrumentID   = "55555555-5555-5555-5555-555555555555"
		relatedPartyID = "66666666-6666-6666-6666-666666666666"
	)
	var m, p string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, p = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := newTestInstrumentsFacade(t, srv).DeleteRelatedParty(context.Background(),
		instrumentsFacadeOrgID, instrumentsFacadeHolderID, instrumentID, relatedPartyID)
	if err != nil {
		t.Fatalf("DeleteRelatedParty: %v", err)
	}
	want := instrumentsHolderBase() + "/" + instrumentID + "/related-parties/" + relatedPartyID
	if m != http.MethodDelete || p != want {
		t.Fatalf("delete related-party req = %s %s, want DELETE %s", m, p, want)
	}
}

// TestInstrumentsFacade_ListAccountsByHolder exercises cursor pagination over the
// holder-in-path accounts endpoint, chaining next_cursor and stopping on the
// terminal empty cursor, returning models.Account.
func TestInstrumentsFacade_ListAccountsByHolder(t *testing.T) {
	acctID1 := "77777777-7777-7777-7777-777777777777"
	acctID2 := "88888888-8888-8888-8888-888888888888"
	page1 := `{"items":[{"id":"` + acctID1 + `","name":"Checking","assetCode":"USD"}],"limit":1,"next_cursor":"a2"}`
	page2 := `{"items":[{"id":"` + acctID2 + `","name":"Savings","assetCode":"USD"}],"limit":1}`

	var seenCursors, seenPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCursors = append(seenCursors, r.URL.Query().Get("cursor"))
		seenPaths = append(seenPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "a2" {
			_, _ = w.Write([]byte(page2))
		} else {
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer srv.Close()

	facade := newTestInstrumentsFacade(t, srv)

	// First page directly.
	first, err := facade.ListAccountsByHolder(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, models.AccountsListOpts{
		PageListOpts: models.PageListOpts{Limit: 1},
	})
	if err != nil {
		t.Fatalf("ListAccountsByHolder: %v", err)
	}
	if len(first.Items) != 1 || first.Items[0].ID != acctID1 {
		t.Fatalf("first page = %+v", first.Items)
	}

	all, err := CollectAll(facade.ListAccountsByHolderAll(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, models.AccountsListOpts{
		PageListOpts: models.PageListOpts{Limit: 1},
	}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].ID != acctID1 || all[1].ID != acctID2 {
		t.Fatalf("All = %+v", all)
	}
	wantPath := "/v1/organizations/" + instrumentsFacadeOrgID + "/holders/" + instrumentsFacadeHolderID + "/accounts"
	for i, p := range seenPaths {
		if p != wantPath {
			t.Fatalf("request %d path = %q, want %q", i, p, wantPath)
		}
	}
	// seenCursors: [""] from the direct first call, then ["", "a2"] from ListAll.
	if seenCursors[len(seenCursors)-2] != "" || seenCursors[len(seenCursors)-1] != "a2" {
		t.Fatalf("ListAll cursor chain tail = %v, want ['' 'a2']", seenCursors)
	}
}

// TestInstrumentsFacade_Filters asserts native param slots (limit, sort_order,
// document, include_deleted) plus the always-injected holder_id and the
// type filter (which maps to the type param slot).
func TestInstrumentsFacade_Filters(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":7}`))
	}))
	defer srv.Close()

	_, err := newTestInstrumentsFacade(t, srv).List(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, models.InstrumentsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 7, SortDirection: models.SortAscending},
		Filters: models.InstrumentFilters{
			Type:           "CHECKING",
			Document:       "DOC-1",
			IncludeDeleted: true,
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
	if got := q.Get("holder_id"); got != instrumentsFacadeHolderID {
		t.Fatalf("holder_id = %q, want %q (always injected)", got, instrumentsFacadeHolderID)
	}
	if got := q.Get("document"); got != "DOC-1" {
		t.Fatalf("document = %q, want DOC-1 (param slot)", got)
	}
	if got := q.Get("include_deleted"); got != "true" {
		t.Fatalf("include_deleted = %q, want true (param slot)", got)
	}
	if got := q.Get("type"); got != "CHECKING" {
		t.Fatalf("type = %q, want CHECKING (editor must inject it)", got)
	}
}

// TestInstrumentsFacade_ErrorDecodes asserts RFC 9457 decode with request-ID
// correlation and that a non-2xx surfaces as *errors.Error.
func TestInstrumentsFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-inst-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0009","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestInstrumentsFacade(t, srv).Create(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID,
		models.NewCreateInstrumentInput("CHECKING"))
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0009" || sdkErr.RequestID != "req-inst-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestInstrumentsFacade_WriteReplaySafe is the 401-replay guard: the write body
// must survive the auth round tripper's post-401 replay (rewindable
// *bytes.Reader).
func TestInstrumentsFacade_WriteReplaySafe(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"55555555-5555-5555-5555-555555555555","holderId":"` + instrumentsFacadeHolderID + `","type":"CHECKING"}`))
	}))
	defer srv.Close()

	_, err := newTestInstrumentsFacade(t, srv).Create(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID,
		models.NewCreateInstrumentInput("CHECKING"))
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"type":"CHECKING"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

// TestInstrumentsFacade_ListPagesConsumerStops covers the `if !yield(page,nil)
// { return }` branch (instruments_facade.go ~L111) on the org-scoped instruments
// list iterator: a consumer returning false after the first page stops the
// iterator immediately. The first page carries a next_cursor, so a regression
// ignoring yield's return would leak a second fetch — asserted absent by
// counting server requests.
func TestInstrumentsFacade_ListPagesConsumerStops(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"33333333-3333-3333-3333-333333333333","holderId":"` + instrumentsFacadeHolderID + `","type":"CHECKING"}],"limit":1,"next_cursor":"c2"}`))
	}))
	defer srv.Close()

	var pages int
	for page, err := range newTestInstrumentsFacade(t, srv).ListPages(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, models.InstrumentsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 1},
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

// TestInstrumentsFacade_ListAccountsByHolderPagesConsumerStops covers the same
// `if !yield(page,nil) { return }` branch (instruments_facade.go ~L281) on the
// holder-in-path accounts iterator: a consumer returning false after the first
// page stops it before the second next_cursor-driven fetch.
func TestInstrumentsFacade_ListAccountsByHolderPagesConsumerStops(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"77777777-7777-7777-7777-777777777777","name":"Checking","assetCode":"USD"}],"limit":1,"next_cursor":"a2"}`))
	}))
	defer srv.Close()

	var pages int
	for page, err := range newTestInstrumentsFacade(t, srv).ListAccountsByHolderPages(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, models.AccountsListOpts{
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

// TestInstrumentsFacade_UpdateNullFieldThroughWire locks the end-to-end
// PATCH-null contract: WithNullField("document") must marshal through
// UpdateInstrumentInput.MarshalJSON and writeJSON as an explicit "document":null
// on the wire — the RFC 7396 field-clear signal. A regression dropping the field
// (omitempty) instead of emitting null would be caught here.
func TestInstrumentsFacade_UpdateNullFieldThroughWire(t *testing.T) {
	const id = "55555555-5555-5555-5555-555555555555"
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + id + `","holderId":"` + instrumentsFacadeHolderID + `","type":"CHECKING"}`))
	}))
	defer srv.Close()

	_, err := newTestInstrumentsFacade(t, srv).Update(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID, id,
		models.NewUpdateInstrumentInput().WithNullField("document"))
	if err != nil {
		t.Fatalf("Update with null field: %v", err)
	}
	if !strings.Contains(body, `"document":null`) {
		t.Fatalf("body = %q, want explicit \"document\":null on the wire", body)
	}
}

func newTestInstrumentsFacade(t *testing.T, srv *httptest.Server) *instrumentsFacade {
	t.Helper()
	return newInstrumentsFacade(newTestLedgerClient(t, srv), true)
}

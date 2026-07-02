// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

const auditOrgID = "11111111-1111-1111-1111-111111111111"

func auditPath() string {
	return "/v1/organizations/" + auditOrgID + "/protection/audit"
}

func newTestAuditFacade(t *testing.T, srv *httptest.Server) *auditFacade {
	t.Helper()
	return newAuditFacade(newTestLedgerClient(t, srv))
}

// TestAuditFacade_ListDecodes covers case (a): one page decodes items and the
// next_cursor from the flat server envelope, and the request lands on the
// audit path with the filter params on the wire.
func TestAuditFacade_ListDecodes(t *testing.T) {
	var q url.Values
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"organization_id":"` + auditOrgID + `","items":[{"id":"e1","action":"provision","actor":"svc","outcome":"success","timestamp":"2026-01-01T00:00:00Z","request_id":"r1"}],"limit":20,"next_cursor":"c2"}`))
	}))
	defer srv.Close()

	list, err := newTestAuditFacade(t, srv).ListAuditEvents(context.Background(), auditOrgID, models.AuditEventsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 20, SortDirection: models.SortDescending, StartDate: "2026-01-01", EndDate: "2026-12-31"},
		Action:         "provision",
		Actor:          "svc",
		Outcome:        "success",
	})
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}

	if path != auditPath() {
		t.Fatalf("path = %q, want %q", path, auditPath())
	}
	if len(list.Items) != 1 || list.Items[0].ID != "e1" || list.Items[0].Outcome != "success" {
		t.Fatalf("Items = %+v", list.Items)
	}
	if list.Pagination.NextCursor != "c2" {
		t.Fatalf("NextCursor = %q, want c2", list.Pagination.NextCursor)
	}

	// Native param slots reach the wire.
	for key, want := range map[string]string{
		"limit": "20", "sort_order": "desc", "start_date": "2026-01-01",
		"end_date": "2026-12-31", "action": "provision", "actor": "svc", "outcome": "success",
	} {
		if got := q.Get(key); got != want {
			t.Fatalf("query %s = %q, want %q", key, got, want)
		}
	}
}

// TestAuditFacade_ListMalformed2xxBody covers the malformed-2xx-body branch:
// the server returns 200 but the body cannot unmarshal into
// ListResponse[AuditEvent] (items is a string, not an array). The facade must
// surface a non-nil *errors.Error (internal), NOT a silent empty page and NOT a
// panic.
func TestAuditFacade_ListMalformed2xxBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":"not-an-array"}`))
	}))
	defer srv.Close()

	got, err := newTestAuditFacade(t, srv).ListAuditEvents(context.Background(), auditOrgID, models.AuditEventsListOpts{})
	if got != nil {
		t.Fatalf("got = %+v, want nil on malformed 2xx body", got)
	}

	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.Category != sdkerrors.CategoryInternal {
		t.Fatalf("Category = %q, want internal for a malformed 2xx body", sdkErr.Category)
	}
}

// TestAuditFacade_CursorStopOnFullTerminalPage is THE KEY RED (case b). The
// terminal page is FULL (ItemCount == Limit) but carries NO next_cursor. A
// HasMore()-driven loop would treat a full page as "probably more" and re-fetch
// forever; the cursor-pure stop must end after the single yield. This test
// counts server hits: it asserts EXACTLY 2 requests (page1 with cursor="c2",
// then page2 which is full-but-terminal and MUST stop). Under a HasMore()
// implementation the second request's full page would trigger a third fetch and
// the hit count would exceed 2 (the httptest server would loop).
func TestAuditFacade_CursorStopOnFullTerminalPage(t *testing.T) {
	const limit = 2
	// page2 is FULL (2 items == limit 2) yet has no next_cursor: the trap. It
	// also carries page:2 — the ONLY input under which HasMore()'s page-based
	// heuristic branch (Page>0 && Limit>0 && ItemCount>=Limit) fires true and
	// re-fetches forever. A cursor-pure stop (NextCursor=="") ends here; a
	// HasMore()-driven stop would issue a third request. This page field makes
	// the two implementations observably diverge.
	page1 := `{"organization_id":"` + auditOrgID + `","items":[{"id":"a"},{"id":"b"}],"limit":2,"page":1,"next_cursor":"c2"}`
	page2 := `{"organization_id":"` + auditOrgID + `","items":[{"id":"c"},{"id":"d"}],"limit":2,"page":2}`

	var seenCursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		seenCursors = append(seenCursors, cursor)
		// Fail hard if a third request ever arrives — that is the HasMore() bug.
		if len(seenCursors) > 2 {
			t.Errorf("iterator re-fetched after a full terminal page (HasMore bug): cursors=%v", seenCursors)
		}
		w.Header().Set("Content-Type", "application/json")
		if cursor == "c2" {
			_, _ = w.Write([]byte(page2))
		} else {
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer srv.Close()

	all, err := CollectAll(newTestAuditFacade(t, srv).ListAuditEventsAll(context.Background(), auditOrgID, models.AuditEventsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: limit},
	}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}

	if len(all) != 4 {
		t.Fatalf("collected %d events, want 4", len(all))
	}
	if len(seenCursors) != 2 || seenCursors[0] != "" || seenCursors[1] != "c2" {
		t.Fatalf("cursor chain = %v, want ['' 'c2'] and exactly 2 requests", seenCursors)
	}
}

// TestAuditFacade_PagesMidStreamHTTPError covers the mid-stream HTTP-error
// branch of ListAuditEventsPages: page 1 is a full page with next_cursor="c2",
// page 2 returns 500 with an RFC-9457 body. The iterator must yield the first
// page successfully, then yield (nil, *errors.Error{500}) on the second pull,
// and STOP (no third request).
func TestAuditFacade_PagesMidStreamHTTPError(t *testing.T) {
	var seenCursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		seenCursors = append(seenCursors, cursor)
		w.Header().Set("Content-Type", "application/json")
		if cursor == "c2" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"type":"about:blank","title":"Internal Server Error","status":500,"detail":"boom"}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"a"}],"limit":1,"next_cursor":"c2"}`))
	}))
	defer srv.Close()

	type yielded struct {
		page *models.ListResponse[models.AuditEvent]
		err  error
	}
	var got []yielded
	for page, err := range newTestAuditFacade(t, srv).ListAuditEventsPages(context.Background(), auditOrgID, models.AuditEventsListOpts{}) {
		got = append(got, yielded{page, err})
	}

	if len(got) != 2 {
		t.Fatalf("iterator yielded %d times, want 2 (page then error)", len(got))
	}
	if got[0].err != nil || got[0].page == nil || len(got[0].page.Items) != 1 || got[0].page.Items[0].ID != "a" {
		t.Fatalf("first yield = %+v, want the first page with no error", got[0])
	}
	if got[1].page != nil {
		t.Fatalf("second yield page = %+v, want nil on error", got[1].page)
	}

	var sdkErr *sdkerrors.Error
	if !errors.As(got[1].err, &sdkErr) {
		t.Fatalf("second yield error type = %T, want *errors.Error", got[1].err)
	}
	if sdkErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want 500", sdkErr.StatusCode)
	}
	if len(seenCursors) != 2 || seenCursors[0] != "" || seenCursors[1] != "c2" {
		t.Fatalf("cursor chain = %v, want ['' 'c2'] and exactly 2 requests (must stop after error)", seenCursors)
	}
}

// TestAuditFacade_CtxCancel covers case (c): a cancelled context ends the
// iterator with the ctx error at the top of an iteration.
func TestAuditFacade_CtxCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"a"}],"limit":1,"next_cursor":"c2"}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectAll(newTestAuditFacade(t, srv).ListAuditEventsAll(ctx, auditOrgID, models.AuditEventsListOpts{}))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// TestAuditFacade_LegacyMode404 covers case (e): a 404 with a bare
// non-RFC-9457 body (legacy mode — envelope encryption disabled, route
// unregistered) maps cleanly to *errors.Error{404}, not an internal error.
func TestAuditFacade_LegacyMode404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "req-audit-404")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Cannot GET /v1/organizations/x/protection/audit"}`))
	}))
	defer srv.Close()

	got, err := newTestAuditFacade(t, srv).ListAuditEvents(context.Background(), auditOrgID, models.AuditEventsListOpts{})
	if got != nil {
		t.Fatalf("got = %+v, want nil on 404", got)
	}

	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want 404 (legacy mode)", sdkErr.StatusCode)
	}
	if sdkErr.Category == sdkerrors.CategoryInternal {
		t.Fatalf("Category = internal, want a status-mapped category for a clean 404")
	}
}

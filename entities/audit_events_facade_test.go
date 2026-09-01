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

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
)

const (
	auditEventID  = "a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1"
	auditEventID2 = "b2b2b2b2-b2b2-b2b2-b2b2-b2b2b2b2b2b2"
	auditHash     = "sha256:cafe"
	auditPrevHash = "sha256:beef"
)

// auditEventJSON is a canonical tracer AuditEvent wire body — Actor is a STRUCT
// (not a string, unlike the ledger protection audit) and the hash-chain fields
// are present.
func auditEventJSON(id string) string {
	return `{"eventId":"` + id + `","eventType":"TRANSACTION_VALIDATED","action":"VALIDATE",` +
		`"result":"ALLOW","resourceType":"transaction","resourceId":"` + valLimitID + `",` +
		`"actor":{"actorType":"system","id":"svc-1","ipAddress":"10.0.0.1","name":"tracer","role":"engine"},` +
		`"hash":"` + auditHash + `","previousHash":"` + auditPrevHash + `",` +
		`"context":{"ruleId":"r1"},"metadata":{"k":"v"},"createdAt":"2026-01-01T00:00:00Z"}`
}

// TestAuditEventsFacade_ListFlatEnvelope is the load-bearing envelope red. The
// tracer serializes the list as {auditEvents:[...],hasMore,nextCursor}; a straight
// Unmarshal into models.ListResponse (which reads Items only from the "items" key)
// would yield EMPTY Items. The facade MUST map the "auditEvents" key into Items.
func TestAuditEventsFacade_ListFlatEnvelope(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"auditEvents":[` + auditEventJSON(auditEventID) + `],"hasMore":true,"nextCursor":"cur-2"}`))
	}))
	defer srv.Close()

	page, err := newTestAuditEventsFacade(t, srv).List(context.Background(), models.AuditEventRecordsListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if method != http.MethodGet || path != "/v1/audit-events" {
		t.Fatalf("list req = %s %s, want GET /v1/audit-events", method, path)
	}
	if len(page.Items) != 1 {
		t.Fatalf("Items = %d, want 1 (the flat auditEvents envelope must map into Items)", len(page.Items))
	}
	ev := page.Items[0]
	if ev.EventID != auditEventID || ev.EventType != "TRANSACTION_VALIDATED" || ev.Result != "ALLOW" {
		t.Fatalf("event decoded wrong: %+v", ev)
	}
	if ev.Actor.ActorType != "system" || ev.Actor.ID != "svc-1" || ev.Actor.Name != "tracer" {
		t.Fatalf("actor struct decoded wrong: %+v", ev.Actor)
	}
	if ev.Hash == nil || *ev.Hash != auditHash || ev.PreviousHash == nil || *ev.PreviousHash != auditPrevHash {
		t.Fatalf("hash-chain fields decoded wrong: %+v", ev)
	}
	if page.Pagination.NextCursor != "cur-2" {
		t.Fatalf("NextCursor = %q, want cur-2", page.Pagination.NextCursor)
	}
}

// TestAuditEventsFacade_PagesCursorStop chains two cursor pages, stops on an empty
// nextCursor, and asserts the cursor advances across exactly two requests.
func TestAuditEventsFacade_PagesCursorStop(t *testing.T) {
	var cursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := r.URL.Query().Get("cursor")
		cursors = append(cursors, c)
		w.Header().Set("Content-Type", "application/json")
		if c == "" {
			_, _ = w.Write([]byte(`{"auditEvents":[` + auditEventJSON(auditEventID) + `],"hasMore":true,"nextCursor":"cur-2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"auditEvents":[` + auditEventJSON(auditEventID2) + `],"hasMore":false,"nextCursor":""}`))
	}))
	defer srv.Close()

	var seen int
	for page, err := range newTestAuditEventsFacade(t, srv).ListPages(context.Background(), models.AuditEventRecordsListOpts{}) {
		if err != nil {
			t.Fatalf("ListPages: %v", err)
		}
		seen += len(page.Items)
	}
	if seen != 2 {
		t.Fatalf("seen %d events, want 2 across two pages", seen)
	}
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "cur-2" {
		t.Fatalf("cursor advance = %v, want [\"\" \"cur-2\"]", cursors)
	}
}

// TestAuditEventsFacade_PagesCtxCancel proves a cancelled context stops the
// iterator with the ctx error before any further request.
func TestAuditEventsFacade_PagesCtxCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"auditEvents":[],"hasMore":false,"nextCursor":""}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var gotErr error
	for _, err := range newTestAuditEventsFacade(t, srv).ListPages(ctx, models.AuditEventRecordsListOpts{}) {
		gotErr = err
		break
	}
	if !errors.Is(gotErr, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", gotErr)
	}
}

// TestAuditEventsFacade_Get decodes one record by id (Actor struct + hash chain).
func TestAuditEventsFacade_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/audit-events/"+auditEventID {
			t.Errorf("get req = %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(auditEventJSON(auditEventID)))
	}))
	defer srv.Close()

	ev, err := newTestAuditEventsFacade(t, srv).Get(context.Background(), auditEventID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ev.EventID != auditEventID || ev.Actor.ID != "svc-1" || ev.PreviousHash == nil || *ev.PreviousHash != auditPrevHash {
		t.Fatalf("Get decoded wrong: %+v", ev)
	}
}

// TestAuditEventsFacade_Verify proves the server's hash-chain verdict decodes
// (the SDK does no crypto — it only reads the verdict). An invalid chain sets
// FirstInvalidID to the tamper locator.
func TestAuditEventsFacade_Verify(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/audit-events/"+auditEventID+"/verify" {
			t.Errorf("verify req = %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"isValid":false,"firstInvalidId":42,"totalChecked":100,"message":"tampered at 42"}`))
	}))
	defer srv.Close()

	res, err := newTestAuditEventsFacade(t, srv).Verify(context.Background(), auditEventID)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if res.IsValid {
		t.Fatalf("IsValid = true, want false")
	}
	if res.FirstInvalidID == nil || *res.FirstInvalidID != 42 {
		t.Fatalf("FirstInvalidID = %v, want 42 (the tamper locator)", res.FirstInvalidID)
	}
	if res.TotalChecked != 100 || res.Message != "tampered at 42" {
		t.Fatalf("verdict decoded wrong: %+v", res)
	}
}

// TestAuditEventsFacade_ListParamMapping proves every field listAuditEventRecordsParams
// maps reaches the wire under the correct query key. Guards against a copy-paste
// mis-map silently returning the wrong audit records. Dates are RFC3339.
func TestAuditEventsFacade_ListParamMapping(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"auditEvents":[],"hasMore":false,"nextCursor":""}`))
	}))
	defer srv.Close()

	opts := models.AuditEventRecordsListOpts{
		CursorListOpts: models.CursorListOpts{
			Limit:         25,
			Cursor:        "cur-1",
			SortDirection: models.SortDescending,
			StartDate:     "2026-01-01T00:00:00Z",
			EndDate:       "2026-01-31T23:59:59Z",
		},
		SortBy: "event_type",
		Filters: models.AuditEventRecordFilters{
			EventType:       "TRANSACTION_VALIDATED",
			Action:          "VALIDATE",
			Result:          "ALLOW",
			ResourceType:    "transaction",
			ResourceID:      valLimitID,
			ActorType:       "system",
			ActorID:         "svc-1",
			AccountID:       valAccountID,
			SegmentID:       valSegmentID,
			PortfolioID:     valPortfolioX,
			TransactionType: "CARD",
			MatchedRuleID:   valMatchedID,
		},
	}

	if _, err := newTestAuditEventsFacade(t, srv).List(context.Background(), opts); err != nil {
		t.Fatalf("List: %v", err)
	}

	for key, want := range map[string]string{
		"limit":            "25",
		"cursor":           "cur-1",
		"sort_order":       "desc",
		"sort_by":          "event_type",
		"start_date":       "2026-01-01T00:00:00Z",
		"end_date":         "2026-01-31T23:59:59Z",
		"event_type":       "TRANSACTION_VALIDATED",
		"action":           "VALIDATE",
		"result":           "ALLOW",
		"resource_type":    "transaction",
		"resource_id":      valLimitID,
		"actor_type":       "system",
		"actor_id":         "svc-1",
		"account_id":       valAccountID,
		"segment_id":       valSegmentID,
		"portfolio_id":     valPortfolioX,
		"transaction_type": "CARD",
		"matched_rule_id":  valMatchedID,
	} {
		if got := q.Get(key); got != want {
			t.Fatalf("query %s = %q, want %q", key, got, want)
		}
	}
}

// TestAuditEventsFacade_ListError proves a non-2xx maps into the unified typed
// error with the APICode + RequestID preserved.
func TestAuditEventsFacade_ListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-audit-list-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"TRACER-0090","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestAuditEventsFacade(t, srv).List(context.Background(), models.AuditEventRecordsListOpts{})
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "TRACER-0090" || sdkErr.RequestID != "req-audit-list-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

func newTestAuditEventsFacade(t *testing.T, srv *httptest.Server) *auditEventsFacade {
	t.Helper()
	return newAuditEventsFacade(newTestTracerClient(t, srv))
}

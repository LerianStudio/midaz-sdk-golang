// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// The three tests below pin the second stop condition on every cursor iterator
// in the SDK.
//
// An empty next_cursor is the stop a well-behaved server gives. A server — or a
// gateway cache, or a handler that copies the request cursor into the response
// envelope — that echoes back the cursor it was HANDED asks for the same page
// again, and the loop obliges forever: the only other bound is ctx.Err(), so a
// caller with no deadline issues unbounded identical requests and never returns.
// That is the same unbounded-iterator shape Epic 2 found on the balance list,
// reached through a different door.
//
// The guard lives in cursorSeq, so it covers every cursor accessor at once. Two
// of them are driven here — a /v2 page-level iterator and the /v1 balance list
// that was migrated onto the shared helper — plus the boundary case, that a real
// advance still runs.

// stallingCursorServer answers every request with the SAME next_cursor it
// received, which is the stall. The request count is capped so a regression
// fails the test instead of hanging the suite.
func stallingCursorServer(t *testing.T, envelope func(cursor string) string) (*httptest.Server, *int) {
	t.Helper()

	const maxRequests = 20

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > maxRequests {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		cursor := r.URL.Query().Get("cursor")
		if cursor == "" {
			cursor = "stuck"
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(envelope(cursor)))
	}))

	t.Cleanup(srv.Close)

	return srv, &requests
}

func TestCursorPagesStopOnARepeatedCursor(t *testing.T) {
	srv, requests := stallingCursorServer(t, func(cursor string) string {
		return `{"items":[{"id":"t-1","amount":"10"}],"limit":1,"next_cursor":"` + cursor + `"}`
	})

	pages := 0

	for _, err := range newTestTransactionsV2Facade(t, srv).Pages(context.Background(), txOrgID, txLedgerID,
		models.TransactionsListOpts{CursorListOpts: models.CursorListOpts{Limit: 1, Cursor: "stuck"}}) {
		if err != nil {
			t.Fatalf("Pages: %v", err)
		}

		pages++

		if pages > 2 {
			t.Fatalf("iterator yielded %d pages; a repeated cursor must stop it after the first", pages)
		}
	}

	if pages != 1 {
		t.Fatalf("pages = %d, want the stalled page yielded exactly once", pages)
	}

	if *requests != 1 {
		t.Fatalf("issued %d requests, want 1; the cursor never advanced so there was nothing to fetch again", *requests)
	}
}

func TestBalanceIteratorStopsOnARepeatedCursor(t *testing.T) {
	srv, requests := stallingCursorServer(t, func(cursor string) string {
		return `{"items":[{"id":"bal-1","assetCode":"USD","available":"1","onHold":"0"}],"limit":1,"next_cursor":"` + cursor + `"}`
	})

	items := 0

	for _, err := range newBalancesFacade(newTestLedgerClient(t, srv), true).
		ListBalancesAll(context.Background(), v2Org, v2Ledger,
			models.BalancesListOpts{CursorListOpts: models.CursorListOpts{Limit: 1, Cursor: "stuck"}}) {
		if err != nil {
			t.Fatalf("ListBalancesAll: %v", err)
		}

		items++

		if items > 2 {
			t.Fatalf("iterator yielded %d balances; a repeated cursor must stop it after the first page", items)
		}
	}

	if items != 1 || *requests != 1 {
		t.Fatalf("items = %d over %d requests, want 1 and 1", items, *requests)
	}
}

// TestCursorIteratorStillAdvancesOnANewCursor is the boundary: the stall guard
// must not have shortened a REAL advance. A next_cursor different from the one
// sent still moves the iterator forward.
func TestCursorIteratorStillAdvancesOnANewCursor(t *testing.T) {
	var seen []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		seen = append(seen, cursor)

		w.Header().Set("Content-Type", "application/json")
		if cursor == "c2" {
			_, _ = w.Write([]byte(`{"items":[{"id":"t-2","amount":"20"}],"limit":1}`))

			return
		}

		_, _ = w.Write([]byte(`{"items":[{"id":"t-1","amount":"10"}],"limit":1,"next_cursor":"c2"}`))
	}))
	defer srv.Close()

	var ids []string

	for tx, err := range newTestTransactionsV2Facade(t, srv).All(context.Background(), txOrgID, txLedgerID,
		models.TransactionsListOpts{CursorListOpts: models.CursorListOpts{Limit: 1}}) {
		if err != nil {
			t.Fatalf("All: %v", err)
		}

		ids = append(ids, tx.ID)
	}

	if len(ids) != 2 || ids[0] != "t-1" || ids[1] != "t-2" {
		t.Fatalf("ids = %v, want both pages consumed", ids)
	}

	if len(seen) != 2 || seen[0] != "" || seen[1] != "c2" {
		t.Fatalf("cursors = %v, want the iterator to advance", seen)
	}
}

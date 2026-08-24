// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// The routing table at the repository root pins method and path for every V2
// endpoint through the public client. This file covers what routing cannot: what
// the facades DO with a response — decode it into the public model, keep money
// exact, advance a cursor, and classify a refusal.

// TestV2FacadesDecodeIntoThePublicModel walks one read per V2 family and checks
// the value that comes back, not just that no error did.
//
// A facade that decoded into nothing would pass a routing table: the request
// still goes to the right place, the call still returns nil error, and the
// caller reads a zero-valued resource. That is the exact shape of the silent
// zero Epic 2 found on the balance reads, so the V2 twins get the assertion on
// arrival.
func TestV2FacadesDecodeIntoThePublicModel(t *testing.T) {
	const id = "44444444-4444-4444-4444-444444444444"

	tests := []struct {
		name string
		body string
		read func(t *testing.T, srv *httptest.Server) (string, error)
	}{
		{
			name: "organization",
			body: `{"id":"` + id + `","legalName":"Acme"}`,
			read: func(t *testing.T, srv *httptest.Server) (string, error) {
				t.Helper()
				org, err := newOrganizationsV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), "org-1")

				return idOf(org, err, func() string { return org.ID })
			},
		},
		{
			name: "ledger",
			body: `{"id":"` + id + `","name":"Main"}`,
			read: func(t *testing.T, srv *httptest.Server) (string, error) {
				t.Helper()
				led, err := newLedgersV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), "org-1", "led-1")

				return idOf(led, err, func() string { return led.ID })
			},
		},
		{
			name: "account",
			body: `{"id":"` + id + `","name":"Cash","assetCode":"USD"}`,
			read: func(t *testing.T, srv *httptest.Server) (string, error) {
				t.Helper()
				acc, err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), "org-1", "led-1", "acc-1")

				return idOf(acc, err, func() string { return acc.ID })
			},
		},
		{
			name: "account type",
			body: `{"id":"` + id + `","name":"Deposit"}`,
			read: func(t *testing.T, srv *httptest.Server) (string, error) {
				t.Helper()
				at, err := newAccountTypesV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), "org-1", "led-1", "at-1")

				return idOf(at, err, func() string { return at.ID.String() })
			},
		},
		{
			name: "asset",
			body: `{"id":"` + id + `","code":"USD"}`,
			read: func(t *testing.T, srv *httptest.Server) (string, error) {
				t.Helper()
				a, err := newAssetsV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), "org-1", "led-1", "ast-1")

				return idOf(a, err, func() string { return a.ID })
			},
		},
		{
			name: "portfolio",
			body: `{"id":"` + id + `","name":"Retail"}`,
			read: func(t *testing.T, srv *httptest.Server) (string, error) {
				t.Helper()
				p, err := newPortfoliosV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), "org-1", "led-1", "pf-1")

				return idOf(p, err, func() string { return p.ID })
			},
		},
		{
			name: "segment",
			body: `{"id":"` + id + `","name":"Premium"}`,
			read: func(t *testing.T, srv *httptest.Server) (string, error) {
				t.Helper()
				s, err := newSegmentsV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), "org-1", "led-1", "sg-1")

				return idOf(s, err, func() string { return s.ID })
			},
		},
		{
			name: "operation route",
			body: `{"id":"` + id + `","title":"Settle"}`,
			read: func(t *testing.T, srv *httptest.Server) (string, error) {
				t.Helper()
				r, err := newOperationRoutesV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), "org-1", "led-1", "or-1")

				return idOf(r, err, func() string { return r.ID.String() })
			},
		},
		{
			name: "transaction route",
			body: `{"id":"` + id + `","title":"Cash in"}`,
			read: func(t *testing.T, srv *httptest.Server) (string, error) {
				t.Helper()
				r, err := newTransactionRoutesV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), "org-1", "led-1", "tr-1")

				return idOf(r, err, func() string { return r.ID.String() })
			},
		},
		{
			name: "operation",
			body: `{"id":"` + id + `","amount":{"value":"10"}}`,
			read: func(t *testing.T, srv *httptest.Server) (string, error) {
				t.Helper()
				op, err := newOperationsV2Facade(newTestLedgerClient(t, srv), true).
					GetOperation(context.Background(), "org-1", "led-1", "acc-1", "op-1")

				return idOf(op, err, func() string { return op.ID })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			got, err := tt.read(t, srv)
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			if got != id {
				t.Fatalf("decoded id = %q, want %q — the response reached the caller as a zero value", got, id)
			}
		})
	}
}

// idOf collapses the read/err/extract dance every row above repeats, and guards
// the nil dereference a failed read would otherwise cause inside the extractor.
func idOf(value any, err error, extract func() string) (string, error) {
	if err != nil {
		return "", err
	}

	if value == nil {
		return "", nil
	}

	return extract(), nil
}

// TestBalancesV2Facade_KeepsMoneyExact is the money-path decode guard for the
// V2 balance reads.
//
// A balance whose available amount round-trips through a float loses precision
// silently: "1500.00000001" becomes 1500.0000000099999 and a reconciliation run
// reports a discrepancy that does not exist. The SDK model holds shopspring
// decimals for exactly this reason, and this asserts the V2 facade keeps them.
func TestBalancesV2Facade_KeepsMoneyExact(t *testing.T) {
	const exact = "1500.00000001"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"bal-1","assetCode":"USD","available":"` + exact + `","onHold":"0.00000002"}`))
	}))
	defer srv.Close()

	balance, err := newBalancesV2Facade(newTestLedgerClient(t, srv), true).
		GetBalance(context.Background(), "org-1", "led-1", "bal-1")
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}

	if balance.Available.String() != exact {
		t.Fatalf("available = %s, want the exact decimal %s", balance.Available, exact)
	}

	if balance.OnHold.String() != "0.00000002" {
		t.Fatalf("onHold = %s, want 0.00000002", balance.OnHold)
	}
}

// TestV2CursorIteratorsAdvanceByCursor is the infinite-loop guard for the V2
// cursor iterators.
//
// Each of these endpoints advances by next_cursor and drops "page" on the floor,
// so an iterator that incremented a page number re-requests the FIRST page for
// as long as the server reports more results — yielding the same rows forever.
// That is not hypothetical: it is the defect Epic 2 found on the /v1 balance
// iterator. The request cap turns a regression into a fast failure instead of a
// hang.
func TestV2CursorIteratorsAdvanceByCursor(t *testing.T) {
	tests := []struct {
		name    string
		collect func(*testing.T, *httptest.Server) ([]string, error)
	}{
		{"ledger balances", func(t *testing.T, srv *httptest.Server) ([]string, error) {
			t.Helper()

			return collectV2IDs(newBalancesV2Facade(newTestLedgerClient(t, srv), true).
				ListBalancesAll(context.Background(), "org-1", "led-1",
					models.BalancesListOpts{CursorListOpts: models.CursorListOpts{Limit: 1}}),
				func(b models.Balance) string { return b.ID })
		}},
		{"account balances", func(t *testing.T, srv *httptest.Server) ([]string, error) {
			t.Helper()

			return collectV2IDs(newBalancesV2Facade(newTestLedgerClient(t, srv), true).
				ListAccountBalancesAll(context.Background(), "org-1", "led-1", "acc-1",
					models.BalancesListOpts{CursorListOpts: models.CursorListOpts{Limit: 1}}),
				func(b models.Balance) string { return b.ID })
		}},
		{"account operations", func(t *testing.T, srv *httptest.Server) ([]string, error) {
			t.Helper()

			return collectV2IDs(newOperationsV2Facade(newTestLedgerClient(t, srv), true).
				ListOperationsAll(context.Background(), "org-1", "led-1", "acc-1",
					models.OperationsListOpts{CursorListOpts: models.CursorListOpts{Limit: 1}}),
				func(o models.Operation) string { return o.ID })
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seenCursors []string

			srv := httptest.NewServer(twoCursorPageHandlerV2(t, &seenCursors))
			defer srv.Close()

			ids, err := tt.collect(t, srv)
			if err != nil {
				t.Fatalf("iterate: %v", err)
			}

			if len(ids) != 2 || ids[0] != "x-1" || ids[1] != "x-2" {
				t.Fatalf("ids = %v, want [x-1 x-2]", ids)
			}

			if len(seenCursors) != 2 || seenCursors[0] != "" || seenCursors[1] != "cur-2" {
				t.Fatalf("cursors = %v, want the iterator to advance by next_cursor", seenCursors)
			}
		})
	}
}

// TestV2RouteIteratorsAdvanceByCursor is the same guard for the two route
// families. They get their own test because their ids decode into uuid.UUID, so
// the fixture has to carry real UUIDs — a fake id fails the decode before the
// cursor logic is ever reached.
func TestV2RouteIteratorsAdvanceByCursor(t *testing.T) {
	const (
		firstID  = "11111111-1111-1111-1111-111111111111"
		secondID = "22222222-2222-2222-2222-222222222222"
	)

	tests := []struct {
		name    string
		collect func(*testing.T, *httptest.Server) ([]string, error)
	}{
		{"operation routes", func(t *testing.T, srv *httptest.Server) ([]string, error) {
			t.Helper()

			return collectV2IDs(newOperationRoutesV2Facade(newTestLedgerClient(t, srv), true).
				All(context.Background(), "org-1", "led-1",
					models.OperationRoutesListOpts{CursorListOpts: models.CursorListOpts{Limit: 1}}),
				func(r models.OperationRoute) string { return r.ID.String() })
		}},
		{"transaction routes", func(t *testing.T, srv *httptest.Server) ([]string, error) {
			t.Helper()

			return collectV2IDs(newTransactionRoutesV2Facade(newTestLedgerClient(t, srv), true).
				All(context.Background(), "org-1", "led-1",
					models.TransactionRoutesListOpts{CursorListOpts: models.CursorListOpts{Limit: 1}}),
				func(r models.TransactionRoute) string { return r.ID.String() })
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seenCursors []string

			srv := httptest.NewServer(twoCursorRoutePageHandler(t, &seenCursors, firstID, secondID))
			defer srv.Close()

			ids, err := tt.collect(t, srv)
			if err != nil {
				t.Fatalf("iterate: %v", err)
			}

			if len(ids) != 2 || ids[0] != firstID || ids[1] != secondID {
				t.Fatalf("ids = %v, want [%s %s]", ids, firstID, secondID)
			}

			if len(seenCursors) != 2 || seenCursors[0] != "" || seenCursors[1] != "cur-2" {
				t.Fatalf("cursors = %v, want the iterator to advance by next_cursor", seenCursors)
			}
		})
	}
}

// TestV2PageIteratorsAdvanceByPage is the sibling guard for the page-numbered V2
// lists. Their failure mode is the mirror image: an iterator that echoed a
// cursor these endpoints do not issue would stop after one page and report a
// partial result set as complete.
func TestV2PageIteratorsAdvanceByPage(t *testing.T) {
	var seenPages []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		seenPages = append(seenPages, page)

		if len(seenPages) > 4 {
			t.Fatalf("iterator did not terminate: pages=%v", seenPages)
		}

		w.Header().Set("Content-Type", "application/json")

		// The terminal page must be PARTIAL (fewer items than the limit) and
		// carry no cursor: those are the only two signals HasMore reads, and a
		// full terminal page really does mean "there may be more".
		if page == "2" {
			_, _ = w.Write([]byte(`{"items":[{"id":"o-2"}],"limit":2,"page":2}`))
			return
		}

		_, _ = w.Write([]byte(`{"items":[{"id":"o-1a"},{"id":"o-1b"}],"limit":2,"page":1}`))
	}))
	defer srv.Close()

	var ids []string

	for org, err := range newOrganizationsV2Facade(newTestLedgerClient(t, srv), true).
		All(context.Background(), models.OrganizationsListOpts{PageListOpts: models.PageListOpts{Limit: 2}}) {
		if err != nil {
			t.Fatalf("All: %v", err)
		}

		ids = append(ids, org.ID)
	}

	if len(ids) != 3 || ids[0] != "o-1a" || ids[2] != "o-2" {
		t.Fatalf("ids = %v, want the items of both pages in order", ids)
	}

	if len(seenPages) != 2 || seenPages[0] != "1" || seenPages[1] != "2" {
		t.Fatalf("pages = %v, want the iterator to advance by page number", seenPages)
	}
}

// TestV2FacadesMapServerRefusals pins that a server refusal reaches the caller
// as the category the status means, with the server's own detail intact — not as
// an SDK-internal fault. The distinction decides whether a caller retries.
func TestV2FacadesMapServerRefusals(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"title":"Not Found","detail":"ledger not found","code":"0052"}`))
	}))
	defer srv.Close()

	_, err := newLedgersV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), "org-1", "led-1")
	if err == nil {
		t.Fatal("expected the server refusal to surface")
	}

	if !sdkerrors.IsNotFoundError(err) {
		t.Fatalf("err = %v, want a not-found error", err)
	}

	if !sdkerrors.IsNotFoundError(err) || err.Error() == "" {
		t.Fatalf("err = %v, want the server detail preserved", err)
	}
}

// twoCursorRoutePageHandler is twoCursorPageHandlerV2 for the route families,
// whose ids decode into uuid.UUID and therefore need real UUIDs on the wire.
func twoCursorRoutePageHandler(t *testing.T, seenCursors *[]string, firstID, secondID string) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		*seenCursors = append(*seenCursors, cursor)

		if len(*seenCursors) > 4 {
			t.Fatalf("iterator did not terminate: cursors=%v", *seenCursors)
		}

		w.Header().Set("Content-Type", "application/json")

		if cursor == "cur-2" {
			_, _ = w.Write([]byte(`{"items":[{"id":"` + secondID + `","title":"b"}],"limit":1}`))
			return
		}

		_, _ = w.Write([]byte(`{"items":[{"id":"` + firstID + `","title":"a"}],"limit":1,"next_cursor":"cur-2"}`))
	}
}

// twoCursorPageHandlerV2 serves exactly two cursor pages and fails the test if it
// is asked for more than a handful, so a page-number iterator hits a fast failure
// instead of looping forever.
func twoCursorPageHandlerV2(t *testing.T, seenCursors *[]string) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		*seenCursors = append(*seenCursors, cursor)

		if len(*seenCursors) > 4 {
			t.Fatalf("iterator did not terminate: cursors=%v", *seenCursors)
		}

		w.Header().Set("Content-Type", "application/json")

		if cursor == "cur-2" {
			_, _ = w.Write([]byte(`{"items":[{"id":"x-2","assetCode":"USD","available":"20"}],"limit":1}`))
			return
		}

		_, _ = w.Write([]byte(`{"items":[{"id":"x-1","assetCode":"USD","available":"10"}],"limit":1,"next_cursor":"cur-2"}`))
	}
}

// collectV2IDs drains an item iterator into the ids the assertions compare.
func collectV2IDs[T any](seq func(func(T, error) bool), id func(T) string) ([]string, error) {
	var ids []string

	for item, err := range seq {
		if err != nil {
			return ids, err
		}

		ids = append(ids, id(item))
	}

	return ids, nil
}

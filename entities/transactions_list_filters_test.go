// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// listFilterSurface is one of the two transaction list surfaces.
type listFilterSurface struct {
	name  string
	list  func(t *testing.T, srv *httptest.Server, opts models.TransactionsListOpts) error
	count func(t *testing.T, srv *httptest.Server, opts models.TransactionsListOpts) (int, error)
}

// listFilterSurfaces must behave identically here because they are ONE server
// function: transaction_handler.go:500 and transaction_v2_mirror_handler.go:148
// both call handler.getAllTransactions with the raw query values. Anything that
// holds for one and not the other would be an SDK-invented difference.
var listFilterSurfaces = []listFilterSurface{
	{
		name: "V1",
		list: func(t *testing.T, srv *httptest.Server, opts models.TransactionsListOpts) error {
			t.Helper()

			_, err := newTestTransactionsFacade(t, srv).List(context.Background(), txOrgID, txLedgerID, opts)

			return err
		},
		count: func(t *testing.T, srv *httptest.Server, opts models.TransactionsListOpts) (int, error) {
			t.Helper()

			return newTestTransactionsFacade(t, srv).Count(context.Background(), txOrgID, txLedgerID, opts)
		},
	},
	{
		name: "V2",
		list: func(t *testing.T, srv *httptest.Server, opts models.TransactionsListOpts) error {
			t.Helper()

			_, err := newTestTransactionsV2Facade(t, srv).List(context.Background(), txOrgID, txLedgerID, opts)

			return err
		},
		count: func(t *testing.T, srv *httptest.Server, opts models.TransactionsListOpts) (int, error) {
			t.Helper()

			return newTestTransactionsV2Facade(t, srv).Count(context.Background(), txOrgID, txLedgerID, opts)
		},
	},
}

// undeclaredFilterCases are the six filters neither list narrows by, plus the
// whitespace spellings that used to slip the refusal.
var undeclaredFilterCases = []struct {
	name string
	set  func(*models.TransactionsFilters)
	want string
}{
	{"asset code", func(f *models.TransactionsFilters) { f.AssetCode = "USD" }, "AssetCode"},
	{"status", func(f *models.TransactionsFilters) { f.Status = "APPROVED" }, "Status"},
	{"reference", func(f *models.TransactionsFilters) { f.Reference = "ref-1" }, "Reference"},
	{"source account", func(f *models.TransactionsFilters) { f.SourceAccount = "@src" }, "SourceAccount"},
	{"destination account", func(f *models.TransactionsFilters) { f.DestinationAccount = "@dst" }, "DestinationAccount"},
	{"route", func(f *models.TransactionsFilters) { f.Route = "route-1" }, "Route"},
	// A value that is only whitespace is still a value the caller SET. It
	// slipped both nets: the refusal trimmed before testing, so it was not
	// named, and no editor emitted it either — leaving the same silent full
	// result set at a nonsense input.
	{"whitespace-only status", func(f *models.TransactionsFilters) { f.Status = "   " }, "Status"},
	{"whitespace-only route", func(f *models.TransactionsFilters) { f.Route = "\t\n" }, "Route"},
}

// pageServer answers any list request with an empty page and records whether it
// was reached at all.
func pageServer(t *testing.T, reached *bool, query *url.Values) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reached != nil {
			*reached = true
		}

		if query != nil {
			*query = r.URL.Query()
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))

	t.Cleanup(srv.Close)

	return srv
}

// assertFilterRefused runs one undeclared filter against one surface.
func assertFilterRefused(t *testing.T, surface listFilterSurface, set func(*models.TransactionsFilters), want string) {
	t.Helper()

	var reached bool

	srv := pageServer(t, &reached, nil)

	var opts models.TransactionsListOpts
	set(&opts.Filters)

	err := surface.list(t, srv, opts)
	if err == nil {
		t.Fatal("a filter the list cannot express must be refused, not sent and ignored")
	}

	if !sdkerrors.IsValidationError(err) {
		t.Fatalf("want a validation error, got %v", err)
	}

	if !strings.Contains(err.Error(), want) {
		t.Fatalf("the refusal must name %s so the caller knows what to remove, got %v", want, err)
	}

	if reached {
		t.Fatal("the request must not leave the SDK")
	}
}

// TestTransactionListRefusesUndeclaredFilters pins the money-data silence the
// refusal closes, on BOTH surfaces.
//
// Neither list narrows by any of these six. Two of them (status, asset_code) are
// parsed by the server and then dropped — ToCursorPagination
// (pkg/net/http/httputils.go:533-539) hands the repository only limit, cursor,
// sort_order and the date range. The other four are never parsed at all: the
// query switch (httputils.go:150-252) has no case for reference,
// source_account, destination_account or a bare route.
//
// So a caller who set Status expected APPROVED transactions and received EVERY
// transaction in the ledger with a nil error — a reconciliation that reads the
// whole book and believes it read one status. Refusing locally is the only place
// the caller's intent still exists; the server cannot report a filter it never
// declared.
func TestTransactionListRefusesUndeclaredFilters(t *testing.T) {
	for _, surface := range listFilterSurfaces {
		t.Run(surface.name, func(t *testing.T) {
			for _, tt := range undeclaredFilterCases {
				t.Run(tt.name, func(t *testing.T) {
					assertFilterRefused(t, surface, tt.set, tt.want)
				})
			}
		})
	}
}

// TestTransactionListNamesEveryUndeclaredFilter: a caller with three of them
// set should fix three at once, not discover them one round trip at a time.
func TestTransactionListNamesEveryUndeclaredFilter(t *testing.T) {
	for _, surface := range listFilterSurfaces {
		t.Run(surface.name, func(t *testing.T) {
			opts := models.TransactionsListOpts{Filters: models.TransactionsFilters{
				Status: "APPROVED", Route: "route-1", AssetCode: "USD",
			}}

			err := surface.list(t, pageServer(t, nil, nil), opts)
			if err == nil {
				t.Fatal("expected a refusal")
			}

			for _, want := range []string{"AssetCode", "Status", "Route"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal must name %s; got %v", want, err)
				}
			}
		})
	}
}

// expressibleQuery is what a transaction list can still narrow by, and deadQuery
// is what the refusal must have removed from the wire entirely.
var (
	expressibleQuery = map[string]string{
		"metadata.orderId": "abc-123",
		"limit":            "25",
		"cursor":           "CUR",
		"sort_order":       "desc",
		"start_date":       "2025-01-01",
		"end_date":         "2025-02-01",
	}
	deadQuery = []string{"asset_code", "status", "reference", "source_account", "destination_account", "route"}
)

// TestTransactionListSendsWhatItCanExpress is the other half: a guard that
// refused everything would also pass the tests above. The metadata predicate,
// the date range and the paging fields must still reach the wire — on both
// surfaces, which is also what proves /v1 kept its cursor/limit/sort/date
// params when its six legacy filter editors were deleted.
func TestTransactionListSendsWhatItCanExpress(t *testing.T) {
	for _, surface := range listFilterSurfaces {
		t.Run(surface.name, func(t *testing.T) {
			var got url.Values

			opts := models.TransactionsListOpts{Filters: models.TransactionsFilters{
				MetadataKey: "orderId", MetadataValue: "abc-123",
			}}
			opts.Limit = 25
			opts.Cursor = "CUR"
			opts.SortDirection = "desc"
			opts.StartDate = "2025-01-01"
			opts.EndDate = "2025-02-01"

			if err := surface.list(t, pageServer(t, nil, &got), opts); err != nil {
				t.Fatalf("an expressible query must not be refused: %v", err)
			}

			assertQuery(t, got)
		})
	}
}

// assertQuery checks both halves of the wire contract in one place.
func assertQuery(t *testing.T, got url.Values) {
	t.Helper()

	for key, want := range expressibleQuery {
		if got.Get(key) != want {
			t.Errorf("%s = %q, want %q", key, got.Get(key), want)
		}
	}

	// The six deleted editors must be gone from the wire, not merely unnamed in
	// the refusal.
	for _, dead := range deadQuery {
		if got.Has(dead) {
			t.Errorf("%s must no longer reach the wire; it never narrowed anything", dead)
		}
	}
}

// TestTransactionCountStillHonoursStatusAndRoute guards the boundary of the
// refusal. The COUNT endpoint declares status and route, so refusing them there
// would remove the only server-side narrowing the surface offers.
func TestTransactionCountStillHonoursStatusAndRoute(t *testing.T) {
	for _, surface := range listFilterSurfaces {
		t.Run(surface.name, func(t *testing.T) {
			var got url.Values

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Query()

				w.Header().Set("X-Total-Count", "7")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			opts := models.TransactionsListOpts{Filters: models.TransactionsFilters{
				Status: "APPROVED", Route: "route-1",
			}}

			count, err := surface.count(t, srv, opts)
			if err != nil {
				t.Fatalf("count declares status and route; it must not refuse them: %v", err)
			}

			if count != 7 {
				t.Fatalf("count = %d, want 7", count)
			}

			if got.Get("status") != "APPROVED" || got.Get("route") != "route-1" {
				t.Fatalf("count must send status and route, got %v", got)
			}
		})
	}
}

// countServer answers a HEAD count and records the query it was asked with.
func countServer(t *testing.T, query *url.Values) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if query != nil {
			*query = r.URL.Query()
		}

		w.Header().Set("X-Total-Count", "3")
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Cleanup(srv.Close)

	return srv
}

// TestTransactionCountTakesADateRange is the pin behind the promise the Count
// docs make, on BOTH surfaces.
//
// Before this, NO spelling of a Count date range worked. The opts struct is
// shared with List, so its validator enforces the list format (YYYY-MM-DD) and
// refused an RFC3339 value locally; a YYYY-MM-DD value passed the SDK and the
// count endpoint then rejected it with ErrInvalidDatetimeFormat, because that
// endpoint strict-parses RFC3339 (count_transactions_by_filters.go:57 and 69).
// A caller asking "how many transactions in January" could not be answered at
// all while three doc sites said they could — and Count is where the refused
// list filters steer them.
//
// One format for the caller (YYYY-MM-DD, as List takes), widened here to the day
// boundaries. END-of-day on the upper bound is the load-bearing half: the query
// compares created_at <= end_date (transaction.postgresql.go:1595), so a naive
// midnight bound would silently drop every transaction on the last day the
// caller named.
func TestTransactionCountTakesADateRange(t *testing.T) {
	for _, surface := range listFilterSurfaces {
		t.Run(surface.name, func(t *testing.T) {
			var got url.Values

			opts := models.TransactionsListOpts{
				CursorListOpts: models.CursorListOpts{StartDate: "2026-01-01", EndDate: "2026-01-31"},
			}

			if _, err := surface.count(t, countServer(t, &got), opts); err != nil {
				t.Fatalf("a YYYY-MM-DD range must reach the count endpoint: %v", err)
			}

			// Literals, deliberately not the countStartOfDayUTC /
			// countEndOfDayUTC constants: asserting against the same constant
			// the code interpolates pins nothing — it passes for any value,
			// including a midnight upper bound that loses the last day.
			for key, want := range map[string]string{
				"start_date": "2026-01-01T00:00:00Z",
				"end_date":   "2026-01-31T23:59:59.999999999Z",
			} {
				if got.Get(key) != want {
					t.Fatalf("%s = %q, want %q", key, got.Get(key), want)
				}
			}

			// Both bounds must parse as RFC3339 or the server answers 400 —
			// the exact failure this fix removes.
			for _, key := range []string{"start_date", "end_date"} {
				if _, err := time.Parse(time.RFC3339, got.Get(key)); err != nil {
					t.Fatalf("%s = %q does not parse as RFC3339: %v", key, got.Get(key), err)
				}
			}
		})
	}
}

// TestTransactionCountOmitsAnUnsetDateRange is the other half: widening must not
// invent a window. With no dates set the SDK sends none, which is what lets the
// server apply its documented today-only default rather than a range the caller
// never asked for.
func TestTransactionCountOmitsAnUnsetDateRange(t *testing.T) {
	for _, surface := range listFilterSurfaces {
		t.Run(surface.name, func(t *testing.T) {
			var got url.Values

			if _, err := surface.count(t, countServer(t, &got), models.TransactionsListOpts{}); err != nil {
				t.Fatalf("count without dates: %v", err)
			}

			for _, key := range []string{"start_date", "end_date"} {
				if got.Has(key) {
					t.Fatalf("%s must be omitted when unset, got %q", key, got.Get(key))
				}
			}
		})
	}
}

// TestTransactionCountStillRefusesAMalformedDate keeps the local refusal: the
// widening appends a suffix and trusts the validator that ran first, so a value
// that is not a calendar day must never reach it.
func TestTransactionCountStillRefusesAMalformedDate(t *testing.T) {
	for _, surface := range listFilterSurfaces {
		t.Run(surface.name, func(t *testing.T) {
			for _, bad := range []string{"01/2026", "2026-01-01T00:00:00Z", "yesterday"} {
				opts := models.TransactionsListOpts{
					CursorListOpts: models.CursorListOpts{StartDate: bad},
				}

				_, err := surface.count(t, countServer(t, nil), opts)
				if err == nil {
					t.Fatalf("%q is not a day and must be refused locally", bad)
				}

				if !sdkerrors.IsValidationError(err) {
					t.Fatalf("%q must be a validation error, got %v", bad, err)
				}
			}
		})
	}
}

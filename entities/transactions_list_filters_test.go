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

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// listFilterSurfaces are the two transaction list surfaces, which must behave
// identically here because they are ONE server function: transaction_handler.go
// :500 and transaction_v2_mirror_handler.go:148 both call
// handler.getAllTransactions with the raw query values. Anything that holds for
// one and not the other would be an SDK-invented difference.
var listFilterSurfaces = []struct {
	name string
	list func(t *testing.T, srv *httptest.Server, opts models.TransactionsListOpts) error
	//nolint:unparam // both surfaces return the count; the v1 row would read as broken without it.
	count func(t *testing.T, srv *httptest.Server, opts models.TransactionsListOpts) (int, error)
}{
	{
		name: "V1",
		list: func(t *testing.T, srv *httptest.Server, opts models.TransactionsListOpts) error {
			_, err := newTestTransactionsFacade(t, srv).List(context.Background(), txOrgID, txLedgerID, opts)

			return err
		},
		count: func(t *testing.T, srv *httptest.Server, opts models.TransactionsListOpts) (int, error) {
			return newTestTransactionsFacade(t, srv).Count(context.Background(), txOrgID, txLedgerID, opts)
		},
	},
	{
		name: "V2",
		list: func(t *testing.T, srv *httptest.Server, opts models.TransactionsListOpts) error {
			_, err := newTestTransactionsV2Facade(t, srv).List(context.Background(), txOrgID, txLedgerID, opts)

			return err
		},
		count: func(t *testing.T, srv *httptest.Server, opts models.TransactionsListOpts) (int, error) {
			return newTestTransactionsV2Facade(t, srv).Count(context.Background(), txOrgID, txLedgerID, opts)
		},
	},
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
	tests := []struct {
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

	for _, surface := range listFilterSurfaces {
		t.Run(surface.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					var reached bool

					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						reached = true

						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`{"items":[]}`))
					}))
					defer srv.Close()

					var opts models.TransactionsListOpts
					tt.set(&opts.Filters)

					err := surface.list(t, srv, opts)
					if err == nil {
						t.Fatal("a filter the list cannot express must be refused, not sent and ignored")
					}

					if !sdkerrors.IsValidationError(err) {
						t.Fatalf("want a validation error, got %v", err)
					}

					if !strings.Contains(err.Error(), tt.want) {
						t.Fatalf("the refusal must name %s so the caller knows what to remove, got %v", tt.want, err)
					}

					if reached {
						t.Fatal("the request must not leave the SDK")
					}
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
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"items":[]}`))
			}))
			defer srv.Close()

			opts := models.TransactionsListOpts{Filters: models.TransactionsFilters{
				Status: "APPROVED", Route: "route-1", AssetCode: "USD",
			}}

			err := surface.list(t, srv, opts)
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

// TestTransactionListSendsWhatItCanExpress is the other half: a guard that
// refused everything would also pass the tests above. The metadata predicate,
// the date range and the paging fields must still reach the wire — on both
// surfaces, which is also what proves /v1 kept its cursor/limit/sort/date
// params when its six legacy filter editors were deleted.
func TestTransactionListSendsWhatItCanExpress(t *testing.T) {
	for _, surface := range listFilterSurfaces {
		t.Run(surface.name, func(t *testing.T) {
			var got url.Values

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Query()

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"items":[]}`))
			}))
			defer srv.Close()

			opts := models.TransactionsListOpts{Filters: models.TransactionsFilters{
				MetadataKey: "orderId", MetadataValue: "abc-123",
			}}
			opts.Limit = 25
			opts.Cursor = "CUR"
			opts.SortDirection = "desc"
			opts.StartDate = "2025-01-01"
			opts.EndDate = "2025-02-01"

			if err := surface.list(t, srv, opts); err != nil {
				t.Fatalf("an expressible query must not be refused: %v", err)
			}

			for key, want := range map[string]string{
				"metadata.orderId": "abc-123",
				"limit":            "25",
				"cursor":           "CUR",
				"sort_order":       "desc",
				"start_date":       "2025-01-01",
				"end_date":         "2025-02-01",
			} {
				if got.Get(key) != want {
					t.Errorf("%s = %q, want %q", key, got.Get(key), want)
				}
			}

			// The six deleted editors must be gone from the wire, not merely
			// unnamed in the refusal.
			for _, dead := range []string{"asset_code", "status", "reference", "source_account", "destination_account", "route"} {
				if got.Has(dead) {
					t.Errorf("%s must no longer reach the wire; it never narrowed anything", dead)
				}
			}
		})
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

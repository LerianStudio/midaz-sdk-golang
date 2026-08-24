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

// TestV2TransactionListRefusesUndeclaredFilters pins the money-data silence the
// refusal closes.
//
// The /v2 transaction list declares exactly metadata, limit, start_date,
// end_date, sort_order and cursor. A caller who sets Status expects APPROVED
// transactions and, before this, received EVERY transaction in the ledger with a
// nil error — a reconciliation that reads the whole book and believes it read
// one status. Refusing locally is the only place the caller's intent still
// exists; the server cannot report a filter it never declared.
func TestV2TransactionListRefusesUndeclaredFilters(t *testing.T) {
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
	}

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

			_, err := newTestTransactionsV2Facade(t, srv).List(context.Background(), txOrgID, txLedgerID, opts)
			if err == nil {
				t.Fatal("a filter the /v2 list cannot express must be refused, not sent and ignored")
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
}

// TestV2TransactionListNamesEveryUndeclaredFilter: a caller with three of them
// set should fix three at once, not discover them one round trip at a time.
func TestV2TransactionListNamesEveryUndeclaredFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	opts := models.TransactionsListOpts{Filters: models.TransactionsFilters{
		Status: "APPROVED", Route: "route-1", AssetCode: "USD",
	}}

	_, err := newTestTransactionsV2Facade(t, srv).List(context.Background(), txOrgID, txLedgerID, opts)
	if err == nil {
		t.Fatal("expected a refusal")
	}

	for _, want := range []string{"AssetCode", "Status", "Route"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %s; got %v", want, err)
		}
	}
}

// TestV2TransactionListSendsWhatItCanExpress is the other half: a guard that
// refused everything would also pass the tests above. The metadata predicate,
// the date range and the paging fields must still reach the wire.
func TestV2TransactionListSendsWhatItCanExpress(t *testing.T) {
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
	opts.StartDate = "2025-01-01"
	opts.EndDate = "2025-02-01"

	if _, err := newTestTransactionsV2Facade(t, srv).
		List(context.Background(), txOrgID, txLedgerID, opts); err != nil {
		t.Fatalf("an expressible query must not be refused: %v", err)
	}

	for key, want := range map[string]string{
		"metadata.orderId": "abc-123",
		"limit":            "25",
		"start_date":       "2025-01-01",
		"end_date":         "2025-02-01",
	} {
		if got.Get(key) != want {
			t.Errorf("%s = %q, want %q", key, got.Get(key), want)
		}
	}
}

// TestV2TransactionCountStillHonoursStatusAndRoute guards the boundary of the
// refusal. The COUNT endpoint declares status and route, so refusing them there
// would remove the only server-side narrowing the surface offers.
func TestV2TransactionCountStillHonoursStatusAndRoute(t *testing.T) {
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

	count, err := newTestTransactionsV2Facade(t, srv).Count(context.Background(), txOrgID, txLedgerID, opts)
	if err != nil {
		t.Fatalf("count declares status and route; it must not refuse them: %v", err)
	}

	if count != 7 {
		t.Fatalf("count = %d, want 7", count)
	}

	if got.Get("status") != "APPROVED" || got.Get("route") != "route-1" {
		t.Fatalf("count must send status and route, got %v", got)
	}
}

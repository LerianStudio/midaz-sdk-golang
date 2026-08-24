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

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

const (
	operationsOrgID    = "11111111-1111-1111-1111-111111111111"
	operationsLedgerID = "22222222-2222-2222-2222-222222222222"
	operationsAcctID   = "33333333-3333-3333-3333-333333333333"
	operationsOpID     = "66666666-6666-6666-6666-666666666666"
	operationsTxID     = "77777777-7777-7777-7777-777777777777"
)

func operationsLedgerScope() string {
	return "/v1/organizations/" + operationsOrgID + "/ledgers/" + operationsLedgerID
}

func newTestOperationsFacade(t *testing.T, srv *httptest.Server) *operationsFacade {
	t.Helper()
	return newOperationsFacade(newTestLedgerClient(t, srv), true)
}

// TestOperationsFacade_ListOperations pins the account-scoped list: its path, the
// cursor/sort/date fields, and every filter the server actually applies. A
// filter that reached no wire slot would return the full unnarrowed set while the
// caller believed it had been narrowed, so each one is asserted.
func TestOperationsFacade_ListOperations(t *testing.T) {
	var gotPath string
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"op-1","type":"DEBIT"}],"limit":9}`))
	}))
	defer srv.Close()

	page, err := newTestOperationsFacade(t, srv).ListOperations(context.Background(), operationsOrgID, operationsLedgerID, operationsAcctID,
		models.OperationsListOpts{
			CursorListOpts: models.CursorListOpts{
				Limit:         9,
				Cursor:        "cur-1",
				SortDirection: models.SortDescending,
				StartDate:     "2026-01-01",
				EndDate:       "2026-01-31",
			},
			Filters: models.OperationsFilters{
				Type:      "DEBIT",
				Direction: "debit",
				RouteID:   "route-1",
				RouteCode: "RC1",
			},
		})
	if err != nil {
		t.Fatalf("ListOperations: %v", err)
	}

	if want := operationsLedgerScope() + "/accounts/" + operationsAcctID + "/operations"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}

	for key, want := range map[string]string{
		"limit": "9", "cursor": "cur-1", "sort_order": "desc",
		"start_date": "2026-01-01", "end_date": "2026-01-31",
		"type": "DEBIT", "direction": "debit", "route_id": "route-1", "route_code": "RC1",
	} {
		if got := gotQuery.Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}

	if len(page.Items) != 1 || page.Items[0].ID != "op-1" {
		t.Fatalf("ListOperations items = %+v", page.Items)
	}
}

// TestOperationsFacade_ListOperationsEmptyItems pins the non-nil items contract:
// a server response with no items array still yields an empty slice, so callers
// can range over it without a nil check.
func TestOperationsFacade_ListOperationsEmptyItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"limit":10}`))
	}))
	defer srv.Close()

	page, err := newTestOperationsFacade(t, srv).ListOperations(context.Background(), operationsOrgID, operationsLedgerID, operationsAcctID, models.OperationsListOpts{})
	if err != nil {
		t.Fatalf("ListOperations: %v", err)
	}

	if page.Items == nil {
		t.Fatal("Items = nil, want an empty slice")
	}
}

// TestOperationsFacade_ListOperationsAll_CursorTerminatesOnFullPage is the
// infinite-loop guard. The terminal page is FULL (ItemCount == Limit) and carries
// a page field but NO next_cursor — the shape where Pagination.HasMore() reports
// true. An iterator gated on HasMore() would reset the cursor and re-request the
// first page forever; the request cap turns that into a fast failure.
func TestOperationsFacade_ListOperationsAll_CursorTerminatesOnFullPage(t *testing.T) {
	var cursors []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		if len(cursors) > 4 {
			t.Fatalf("infinite loop: iterator did not terminate, cursors=%v", cursors)
		}

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("cursor") == "op-2" {
			_, _ = w.Write([]byte(`{"items":[{"id":"op-3","type":"CREDIT"},{"id":"op-4","type":"DEBIT"}],"limit":2,"page":1}`))
			return
		}

		_, _ = w.Write([]byte(`{"items":[{"id":"op-1","type":"DEBIT"},{"id":"op-2","type":"CREDIT"}],"limit":2,"page":1,"next_cursor":"op-2"}`))
	}))
	defer srv.Close()

	all, err := CollectAll(newTestOperationsFacade(t, srv).ListOperationsAll(context.Background(), operationsOrgID, operationsLedgerID, operationsAcctID,
		models.OperationsListOpts{CursorListOpts: models.CursorListOpts{Limit: 2}}))
	if err != nil {
		t.Fatalf("ListOperationsAll: %v", err)
	}

	if len(all) != 4 || all[0].ID != "op-1" || all[3].ID != "op-4" {
		t.Fatalf("ListOperationsAll = %+v", all)
	}

	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "op-2" {
		t.Fatalf("cursor chain = %v, want ['', 'op-2']", cursors)
	}
}

// TestOperationsFacade_GetOperation resolves one operation through the account
// that observed it.
func TestOperationsFacade_GetOperation(t *testing.T) {
	var m, p string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, p = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + operationsOpID + `","type":"DEBIT","assetCode":"USD"}`))
	}))
	defer srv.Close()

	op, err := newTestOperationsFacade(t, srv).GetOperation(context.Background(), operationsOrgID, operationsLedgerID, operationsAcctID, operationsOpID)
	if err != nil {
		t.Fatalf("GetOperation: %v", err)
	}

	want := operationsLedgerScope() + "/accounts/" + operationsAcctID + "/operations/" + operationsOpID
	if m != http.MethodGet || p != want {
		t.Fatalf("get req = %s %s, want GET %s", m, p, want)
	}

	if op.ID != operationsOpID {
		t.Fatalf("GetOperation returned %+v", op)
	}
}

// TestOperationsFacade_UpdateTransactionOperation pins the write to the
// TRANSACTION scope. The read surface is account-scoped and the write is not:
// an operation is owned by the transaction that produced it, and patching it
// through an account path is not a route the server serves.
func TestOperationsFacade_UpdateTransactionOperation(t *testing.T) {
	var m, p, body string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, p = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + operationsOpID + `","description":"updated","type":"DEBIT"}`))
	}))
	defer srv.Close()

	op, err := newTestOperationsFacade(t, srv).UpdateTransactionOperation(context.Background(), operationsOrgID, operationsLedgerID, operationsTxID, operationsOpID,
		&models.UpdateOperationInput{Description: "updated"})
	if err != nil {
		t.Fatalf("UpdateTransactionOperation: %v", err)
	}

	want := operationsLedgerScope() + "/transactions/" + operationsTxID + "/operations/" + operationsOpID
	if m != http.MethodPatch || p != want {
		t.Fatalf("update req = %s %s, want PATCH %s", m, p, want)
	}

	if !strings.Contains(body, `"description":"updated"`) {
		t.Fatalf("body = %q, want marshaled UpdateOperationInput", body)
	}

	if op.Description != "updated" {
		t.Fatalf("UpdateTransactionOperation returned %+v", op)
	}
}

// TestOperationsFacade_ValidatesBeforeRequest proves the pre-flight
// short-circuits: an over-limit list, an empty update payload and a typed-nil
// input all fail locally, so the server is never hit.
func TestOperationsFacade_ValidatesBeforeRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("server must not be hit on invalid input")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	facade := newTestOperationsFacade(t, srv)

	_, err := facade.ListOperations(context.Background(), operationsOrgID, operationsLedgerID, operationsAcctID,
		models.OperationsListOpts{CursorListOpts: models.CursorListOpts{Limit: models.MaxLimit + 1}})
	if err == nil || !strings.Contains(err.Error(), "limit exceeds maximum") {
		t.Fatalf("ListOperations over limit = %v, want a limit validation error", err)
	}

	if _, err := facade.UpdateTransactionOperation(context.Background(), operationsOrgID, operationsLedgerID, operationsTxID, operationsOpID,
		&models.UpdateOperationInput{}); err == nil {
		t.Fatal("expected an empty update payload to be rejected")
	}

	var typedNil *models.UpdateOperationInput
	if _, err := facade.UpdateTransactionOperation(context.Background(), operationsOrgID, operationsLedgerID, operationsTxID, operationsOpID,
		typedNil); err == nil {
		t.Fatal("expected a typed-nil update payload to be rejected")
	}
}

// TestOperationsFacade_ErrorDecodes asserts the RFC 9457 decode with request-ID
// correlation on the read and write paths.
func TestOperationsFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-op-404")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0056","title":"Operation not found","status":404}`))
	}))
	defer srv.Close()

	facade := newTestOperationsFacade(t, srv)

	assertDecoded := func(t *testing.T, err error) {
		t.Helper()

		var sdkErr *sdkerrors.Error
		if !errors.As(err, &sdkErr) {
			t.Fatalf("error type = %T, want *errors.Error", err)
		}

		if sdkErr.APICode != "LEDGER-0056" || sdkErr.RequestID != "req-op-404" {
			t.Fatalf("decoded error = %+v", sdkErr)
		}
	}

	_, err := facade.ListOperations(context.Background(), operationsOrgID, operationsLedgerID, operationsAcctID, models.OperationsListOpts{})
	assertDecoded(t, err)

	_, err = facade.GetOperation(context.Background(), operationsOrgID, operationsLedgerID, operationsAcctID, operationsOpID)
	assertDecoded(t, err)

	_, err = facade.UpdateTransactionOperation(context.Background(), operationsOrgID, operationsLedgerID, operationsTxID, operationsOpID,
		&models.UpdateOperationInput{Description: "updated"})
	assertDecoded(t, err)
}

// TestOperationsFacade_WriteReplaySafe is the 401-replay guard: the marshaled
// body must survive a token refresh and replay intact.
func TestOperationsFacade_WriteReplaySafe(t *testing.T) {
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

		raw, _ := io.ReadAll(r.Body)
		replayed = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + operationsOpID + `","description":"updated"}`))
	}))
	defer srv.Close()

	if _, err := newTestOperationsFacade(t, srv).UpdateTransactionOperation(context.Background(), operationsOrgID, operationsLedgerID, operationsTxID, operationsOpID,
		&models.UpdateOperationInput{Description: "updated"}); err != nil {
		t.Fatalf("UpdateTransactionOperation with one 401 refresh: %v", err)
	}

	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}

	if !strings.Contains(replayed, `"description":"updated"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// Two refusals, one table, because they are the same question asked at the two
// ends of a call:
//
//   - Going OUT, a path id the caller cannot have meant must be refused HERE,
//     before a request exists. An empty trailing id builds ".../accounts/",
//     which Fiber routes to the COLLECTION; a ".." id pops a segment, so a
//     delete addressed at a ledger is issued against the ORGANIZATION. By the
//     time the URL is resolved the caller's intent is gone, and a
//     scope-escalated delete is indistinguishable from the one they asked for.
//     So the assertion is not "an error came back" — it is that NOTHING WAS
//     SENT.
//
//   - Coming BACK, a gateway's 403 or 404 with an empty body must keep its
//     status. "You are not allowed" and "it is not there" are different next
//     actions, and both arriving as an SDK-internal 500 is retryable-looking,
//     unattributable, and wrong about whose fault it is.
//
// The V2 facades are immune to the second by construction — they read raw
// responses rather than through the generated status-exact parser. That is a
// property of how they are written, which is exactly the kind of thing that
// survives until someone rewrites one.

// v2Calls samples the call sites on the THIRTEEN dual-served V2 facades that
// format a caller-supplied value into their URL path: every by-id read and
// delete, the counts, and the transaction lifecycle actions. The nine V2-only
// facades — Holders, Instruments, Encryption, Composition, ProtectionAudit and
// the four billing/fee accessors — are pinned by their own per-facade tests and
// are deliberately not repeated here.
//
// It is a SAMPLE, not the enumeration, and it does not need to be one. The
// enumeration is TestEveryPathParameterOperationIsGuarded
// (path_id_guard_structural_test.go), which reads every facade function in the
// package and fails when one forwards a path value it never handed to
// requirePathIDs — so a guard missing from V2.Ledgers.Update or
// V2.Transactions.Revert fails there, with no row needed here. What this table
// adds is the part a structural scan cannot assert: that the refusal happens
// BEFORE a request exists, proven by a server that counts what reaches it.
// Rows earn their place by covering a distinct shape of that behaviour, so the
// writes carrying a body (the Updates, UpdateBalance) are represented by
// V2.Organizations.Update, V2.Operations.UpdateTransactionOperation and
// V2.Transactions.Update rather than repeated per family.
//
// pathID is substituted into the LAST path parameter of each call, which is the
// position where an empty value silently reaches the collection endpoint rather
// than 404ing. Every other id in the row stays valid, so a refusal here is
// attributable to the value under test.
var v2Calls = []struct {
	name string
	call func(t *testing.T, srv *httptest.Server, pathID string) error
}{
	{"V2.Organizations.Get", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newOrganizationsV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), id)

		return err
	}},
	{"V2.Organizations.Update", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newOrganizationsV2Facade(newTestLedgerClient(t, srv), true).
			Update(context.Background(), id, &models.UpdateOrganizationInput{LegalName: "Acme"})

		return err
	}},
	{"V2.Organizations.Delete", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()

		return newOrganizationsV2Facade(newTestLedgerClient(t, srv), true).Delete(context.Background(), id)
	}},
	{"V2.Ledgers.Get", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newLedgersV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), v2Org, id)

		return err
	}},
	{"V2.Ledgers.Delete", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()

		return newLedgersV2Facade(newTestLedgerClient(t, srv), true).Delete(context.Background(), v2Org, id)
	}},
	{"V2.Ledgers.GetSettings", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newLedgersV2Facade(newTestLedgerClient(t, srv), true).GetSettings(context.Background(), v2Org, id)

		return err
	}},
	{"V2.Ledgers.Count", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newLedgersV2Facade(newTestLedgerClient(t, srv), true).Count(context.Background(), id)

		return err
	}},
	{"V2.Accounts.Get", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.Accounts.GetByAlias", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).
			GetByAlias(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.Accounts.GetByExternalCode", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).
			GetByExternalCode(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.Accounts.Delete", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()

		return newAccountsV2Facade(newTestLedgerClient(t, srv), true).Delete(context.Background(), v2Org, v2Ledger, id)
	}},
	{"V2.Accounts.List", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).
			List(context.Background(), v2Org, id, models.AccountsListOpts{})

		return err
	}},
	{"V2.Accounts.Count", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).Count(context.Background(), v2Org, id)

		return err
	}},
	{"V2.AccountTypes.Get", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newAccountTypesV2Facade(newTestLedgerClient(t, srv), true).
			Get(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.AccountTypes.Delete", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()

		return newAccountTypesV2Facade(newTestLedgerClient(t, srv), true).
			Delete(context.Background(), v2Org, v2Ledger, id)
	}},
	{"V2.Assets.Get", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newAssetsV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.Assets.Delete", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()

		return newAssetsV2Facade(newTestLedgerClient(t, srv), true).Delete(context.Background(), v2Org, v2Ledger, id)
	}},
	{"V2.Portfolios.Get", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newPortfoliosV2Facade(newTestLedgerClient(t, srv), true).
			Get(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.Portfolios.Delete", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()

		return newPortfoliosV2Facade(newTestLedgerClient(t, srv), true).
			Delete(context.Background(), v2Org, v2Ledger, id)
	}},
	{"V2.Segments.Get", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newSegmentsV2Facade(newTestLedgerClient(t, srv), true).Get(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.Segments.Delete", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()

		return newSegmentsV2Facade(newTestLedgerClient(t, srv), true).
			Delete(context.Background(), v2Org, v2Ledger, id)
	}},
	{"V2.OperationRoutes.Get", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newOperationRoutesV2Facade(newTestLedgerClient(t, srv), true).
			Get(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.OperationRoutes.Delete", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()

		return newOperationRoutesV2Facade(newTestLedgerClient(t, srv), true).
			Delete(context.Background(), v2Org, v2Ledger, id)
	}},
	{"V2.TransactionRoutes.Get", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newTransactionRoutesV2Facade(newTestLedgerClient(t, srv), true).
			Get(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.TransactionRoutes.Delete", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()

		return newTransactionRoutesV2Facade(newTestLedgerClient(t, srv), true).
			Delete(context.Background(), v2Org, v2Ledger, id)
	}},
	{"V2.Balances.GetBalance", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newBalancesV2Facade(newTestLedgerClient(t, srv), true).
			GetBalance(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.Balances.GetBalanceHistory", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newBalancesV2Facade(newTestLedgerClient(t, srv), true).
			GetBalanceHistory(context.Background(), v2Org, v2Ledger, id, balanceInstant)

		return err
	}},
	{"V2.Balances.DeleteBalance", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()

		return newBalancesV2Facade(newTestLedgerClient(t, srv), true).
			DeleteBalance(context.Background(), v2Org, v2Ledger, id)
	}},
	{"V2.Balances.ListBalancesByAccountAlias", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newBalancesV2Facade(newTestLedgerClient(t, srv), true).
			ListBalancesByAccountAlias(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.Operations.GetOperation", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newOperationsV2Facade(newTestLedgerClient(t, srv), true).
			GetOperation(context.Background(), v2Org, v2Ledger, v2Account, id)

		return err
	}},
	{"V2.Operations.UpdateTransactionOperation", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newOperationsV2Facade(newTestLedgerClient(t, srv), true).
			UpdateTransactionOperation(context.Background(), v2Org, v2Ledger, v2UUIDA, id,
				&models.UpdateOperationInput{Description: "updated"})

		return err
	}},
	{"V2.MetadataIndexes.Delete", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()

		return newMetadataIndexesV2Facade(newTestLedgerClient(t, srv), true).
			Delete(context.Background(), "account", id)
	}},
	{"V2.Transactions.Get", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newTestTransactionsV2Facade(t, srv).Get(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.Transactions.Commit", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newTestTransactionsV2Facade(t, srv).Commit(context.Background(), v2Org, v2Ledger, id)

		return err
	}},
	{"V2.Transactions.Update", func(t *testing.T, srv *httptest.Server, id string) error {
		t.Helper()
		_, err := newTestTransactionsV2Facade(t, srv).Update(context.Background(), v2Org, v2Ledger, id,
			&models.UpdateTransactionV2Input{Description: "corrected"})

		return err
	}},
}

// dangerousPathIDs are the values that must never become a URL path segment.
//
// "" and "   " both trim to nothing, and the trailing-empty case is the one
// that reaches the collection endpoint and reads as a zero-valued resource with
// a nil error. ".." pops a segment, which turns a scoped delete into one level
// up. "a/b" injects a segment outright.
var dangerousPathIDs = []struct {
	name string
	id   string
}{
	{name: "empty", id: ""},
	{name: "whitespace only", id: "   "},
	{name: "dot segment", id: "."},
	{name: "parent dot segment", id: ".."},
	{name: "path separator", id: "acc/../../ledgers"},
	{name: "percent encoded", id: "%2e%2e"},
}

// TestV2RefusesADangerousPathIDWithoutReachingTheWire is the outbound guard.
//
// The counting server is the assertion. An error alone would also come back
// from a server that 404'd, and a 404 means the request was SENT — which for a
// dot-segment delete means it was sent somewhere other than where the caller
// pointed it.
func TestV2RefusesADangerousPathIDWithoutReachingTheWire(t *testing.T) {
	for _, call := range v2Calls {
		t.Run(call.name, func(t *testing.T) {
			for _, bad := range dangerousPathIDs {
				t.Run(bad.name, func(t *testing.T) {
					var requests atomic.Int32

					srv := countingServer(t, &requests, http.StatusOK, `{"id":"never"}`)

					err := call.call(t, srv, bad.id)
					if err == nil {
						t.Fatalf("a path id of %q must be refused", bad.id)
					}

					if got := requests.Load(); got != 0 {
						t.Fatalf("%d request(s) reached the server; a refused id must never be formatted into a URL", got)
					}
				})
			}
		})
	}
}

// TestV2RefusalNamesTheProblemLocally is the other half: the refusal has to be
// a LOCAL, typed error the caller can branch on, not an opaque failure. An
// SDK-internal error here would tell the caller their client is broken when
// what happened is that they passed a bad id.
func TestV2RefusalNamesTheProblemLocally(t *testing.T) {
	var requests atomic.Int32

	srv := countingServer(t, &requests, http.StatusOK, `{"id":"never"}`)

	t.Run("an empty id is a missing parameter", func(t *testing.T) {
		err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).
			Delete(context.Background(), v2Org, v2Ledger, "")

		if !sdkerrors.IsValidationError(err) {
			t.Fatalf("err = %v, want a validation-category error naming the parameter", err)
		}

		if sdkerrors.IsInternalError(err) {
			t.Fatalf("err = %v, want the caller told what they got wrong, not that the SDK broke", err)
		}
	})

	t.Run("a dot segment is a validation error", func(t *testing.T) {
		err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).
			Delete(context.Background(), v2Org, v2Ledger, "..")

		if !sdkerrors.IsValidationError(err) {
			t.Fatalf("err = %v, want a validation error", err)
		}
	})
}

// TestV2KeepsTheGatewayStatusOnAnEmptyErrorBody is the inbound guard.
//
// A gateway in front of Midaz answers 403 or 404 with "Content-Type:
// application/json" and NO body. Through the generated status-exact parser both
// arrived as an internal error carrying 500, because the parser fails on the
// unreadable body before any facade logic runs. Every row here reads the raw
// response instead — this is what keeps that true.
func TestV2KeepsTheGatewayStatusOnAnEmptyErrorBody(t *testing.T) {
	statuses := []struct {
		name   string
		status int
		is     func(error) bool
	}{
		{name: "403 stays an authorization error", status: http.StatusForbidden, is: sdkerrors.IsAuthorizationError},
		{name: "404 stays a not-found error", status: http.StatusNotFound, is: sdkerrors.IsNotFoundError},
	}

	for _, call := range v2Calls {
		t.Run(call.name, func(t *testing.T) {
			for _, s := range statuses {
				t.Run(s.name, func(t *testing.T) {
					err := call.call(t, emptyBodyServer(t, s.status, ""), v2UUIDA)

					assertStatusSurvived(t, err, s.status, s.is)
				})
			}
		})
	}
}

// countingServer answers every request with status and body, and counts how many
// arrived.
func countingServer(t *testing.T, requests *atomic.Int32, status int, body string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(srv.Close)

	return srv
}

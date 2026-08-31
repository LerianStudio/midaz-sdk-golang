// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
)

// retrofittedReads samples the single-object read path after the last 45 facade
// sites moved off the generated *WithResponse parser.
//
// # Why a SAMPLE is honest here, when it was not in earlier rounds
//
// Four consecutive rounds of this epic shipped a table that sampled examples of
// a property and called the property proven. Each time, a site the table did not
// name was left defective with the whole suite green. The rule that came out of
// it: enumerate the decode paths, and let a test enforce the enumeration.
//
// This table does not try to BE the enumeration, because it cannot be — a
// behavioural row per site is forty-five servers proving one helper forty-five
// times. TestNoFacadeCallsAGeneratedParser is the enumeration: it reads every
// non-test file in this package and refuses any facade that names a generated
// parser spelling, so after the retrofit there is exactly ONE decode path for a
// single-object read and no facade can quietly open a second.
//
// Sample plus scan is what covers every site. The scan proves the sites all
// arrive at the same helper; these rows prove what the helper does when it gets
// there. Neither half is sufficient alone, and the scan is the half that fails
// when someone adds a forty-sixth site.
//
// The rows span the SHAPES the one path is reached through, not the resources it
// returns: a plain retrofitted read, a money-path retrofitted read, the tracer
// plane's separate client, both SURFACES of a dual family (V1 and V2
// Accounts.Get, shown to agree, which they did not before), both SPELLINGS of a
// single endpoint, and the two v1 lifecycle transitions — which the retrofit did
// NOT touch. Those last two are the point of the rule this epic learned in fix
// round 2: a central fix is proven from a site the fix did not touch. Commit and
// Revert reach decodeOne through readRawResponse directly rather than through
// readOne, and they hold the property just the same.
//
// # Dual SURFACE and dual SPELLING are not the same row
//
// The retrofit's commit message and the plan claimed this table covered the dual
// spelling — the transaction-scoped operation update, which /v1 reaches under
// two facade names over ONE generated operation, and which answered differently
// depending on the name until the retrofit. It did not: it carried the dual
// FAMILY, V1 and V2 Accounts.Get, which is two endpoints on two surfaces. A
// review caught the substitution. The real pair is here now —
// V1.Transactions.UpdateOperation and V1.Operations.UpdateTransactionOperation —
// so the claim is true rather than reworded.
//
// It also buys a shape nothing else in the table had: those two are WRITES.
// Every other row reaches decodeOne through readOne; a write reaches it through
// writeJSON, which marshals a payload, sends it through the same
// readRawResponse and then decodes. Same three properties, different half of the
// funnel.
//
// Each row carries its OWN success body. The tracer's Limit keys its identity on
// "limitId" where every ledger resource uses "id", so a shared ledger-shaped
// stand-in would have decoded into a zero id on that row and proved nothing —
// the same stand-in mistake a fix round already had to correct on the list
// fixtures.
var retrofittedReads = []struct {
	name   string
	okBody string
	call   func(t *testing.T, srv *httptest.Server) (string, error)
}{
	{
		name:   "V1.Organizations.Get",
		okBody: `{"id":"` + txID + `"}`,
		call: func(t *testing.T, srv *httptest.Server) (string, error) {
			t.Helper()

			got, err := newOrganizationsFacade(newTestLedgerClient(t, srv), true).
				Get(context.Background(), txOrgID)

			return idOrEmpty(err, func() string { return got.ID })
		},
	},
	{
		name:   "V1.Balances.GetBalance",
		okBody: `{"id":"` + txID + `"}`,
		call: func(t *testing.T, srv *httptest.Server) (string, error) {
			t.Helper()

			got, err := newBalancesFacade(newTestLedgerClient(t, srv), true).
				GetBalance(context.Background(), txOrgID, txLedgerID, txID)

			return idOrEmpty(err, func() string { return got.ID })
		},
	},
	{
		name:   "V1.Accounts.Get",
		okBody: `{"id":"` + txID + `"}`,
		call: func(t *testing.T, srv *httptest.Server) (string, error) {
			t.Helper()

			got, err := newTestAccountsFacade(t, srv).Get(context.Background(), txOrgID, txLedgerID, txID)

			return idOrEmpty(err, func() string { return got.ID })
		},
	},
	{
		name:   "V2.Accounts.Get",
		okBody: `{"id":"` + txID + `"}`,
		call: func(t *testing.T, srv *httptest.Server) (string, error) {
			t.Helper()

			got, err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).
				Get(context.Background(), txOrgID, txLedgerID, txID)

			return idOrEmpty(err, func() string { return got.ID })
		},
	},
	{
		name:   "V1.Limits.Get",
		okBody: `{"limitId":"` + txID + `"}`,
		call: func(t *testing.T, srv *httptest.Server) (string, error) {
			t.Helper()

			got, err := newLimitsFacade(newTestTracerClient(t, srv), true).Get(context.Background(), txID)

			return idOrEmpty(err, func() string { return got.ID })
		},
	},
	{
		// One generated operation, UpdateOperation, spelled twice on /v1. Until
		// the retrofit these two answered differently: V1.Transactions read it
		// raw, V1.Operations read it through the parser. Both rows exist so a
		// regression on either name is visible on its own.
		name:   "V1.Transactions.UpdateOperation",
		okBody: `{"id":"` + txID + `"}`,
		call: func(t *testing.T, srv *httptest.Server) (string, error) {
			t.Helper()

			got, err := newTestTransactionsFacade(t, srv).UpdateOperation(
				context.Background(), txOrgID, txLedgerID, txID, txID, &models.UpdateOperationInput{
					Description: "retrofit pin",
				})

			return idOrEmpty(err, func() string { return got.ID })
		},
	},
	{
		name:   "V1.Operations.UpdateTransactionOperation",
		okBody: `{"id":"` + txID + `"}`,
		call: func(t *testing.T, srv *httptest.Server) (string, error) {
			t.Helper()

			got, err := newOperationsFacade(newTestLedgerClient(t, srv), true).UpdateTransactionOperation(
				context.Background(), txOrgID, txLedgerID, txID, txID, &models.UpdateOperationInput{
					Description: "retrofit pin",
				})

			return idOrEmpty(err, func() string { return got.ID })
		},
	},
	{
		name:   "V1.Transactions.Commit",
		okBody: `{"id":"` + txID + `"}`,
		call: func(t *testing.T, srv *httptest.Server) (string, error) {
			t.Helper()

			got, err := newTestTransactionsFacade(t, srv).Commit(context.Background(), txOrgID, txLedgerID, txID)

			return idOrEmpty(err, func() string { return got.ID })
		},
	},
	{
		name:   "V1.Transactions.Revert",
		okBody: `{"id":"` + txID + `"}`,
		call: func(t *testing.T, srv *httptest.Server) (string, error) {
			t.Helper()

			got, err := newTestTransactionsFacade(t, srv).Revert(context.Background(), txOrgID, txLedgerID, txID)

			return idOrEmpty(err, func() string { return got.ID })
		},
	},
}

// idOrEmpty reads the decoded id only when the facade reported success, so a row
// never dereferences the nil model an error path returns.
func idOrEmpty(err error, id func() string) (string, error) {
	if err != nil {
		return "", err
	}

	return id(), nil
}

// TestReadRawResponseRefusesANilResponse pins the one input that would panic
// rather than return.
//
// readRawResponse is the funnel every retrofitted read and every write now goes
// through, and it dereferenced resp.Body on the strength of err being nil. A
// caller who discards the transport error — `resp, _ := f.ledger.GetX(...)` —
// hands it (nil, nil), and a connection failure becomes a nil dereference on the
// money path. This SDK does not panic in library code.
func TestReadRawResponseRefusesANilResponse(t *testing.T) {
	//nolint:bodyclose // there is no response to close; that is the case under test.
	resp, body, err := readRawResponse(nil, nil)
	if err == nil {
		t.Fatal("a nil response with no transport error must return an error, not dereference it")
	}

	if resp != nil || body != nil {
		t.Fatalf("want no response and no body alongside the error, got resp=%v body=%q", resp, body)
	}
}

// TestRetrofittedReadKeepsTheRealErrorStatus is the expensive half of what the
// generated parser was destroying.
//
// A gateway answers 403 or 404 with "Content-Type: application/json" and NO
// body — the shape a proxy, a service mesh or an auth sidecar produces without
// ever reaching Midaz. The parser unmarshalled that empty body as an error
// document, failed, and the facade reported errors.NewInternalError: status 500,
// retryable-looking, and pointing the operator at their own SDK.
//
// The caller needs the two statuses kept apart: 403 means fix the credentials,
// 404 means the resource is not there. Both now survive an empty body.
func TestRetrofittedReadKeepsTheRealErrorStatus(t *testing.T) {
	statuses := []struct {
		status int
		is     func(error) bool
		name   string
	}{
		{http.StatusForbidden, sdkerrors.IsAuthorizationError, "403 stays an authorization error"},
		{http.StatusNotFound, sdkerrors.IsNotFoundError, "404 stays a not-found error"},
	}

	for _, read := range retrofittedReads {
		t.Run(read.name, func(t *testing.T) {
			for _, s := range statuses {
				t.Run(s.name, func(t *testing.T) {
					_, err := read.call(t, emptyBodyServer(t, s.status, ""))

					assertStatusSurvived(t, err, s.status, s.is)
				})
			}
		})
	}
}

// TestRetrofittedReadReportsAnUnreadableBodyAsADecodeError is the other half of
// the same swap, and on a write it decides what the caller does next.
//
// A 2xx whose body cannot be parsed is a fact about the RESPONSE: the server
// answered, the answer is unreadable, and the operation may well have taken
// effect. errors.IsResponseDecodeError says exactly that. The generated parser
// reported the identical situation as an SDK-internal fault, which says the
// opposite — that nothing happened and the bug is local.
//
// The body is TRUNCATED rather than absent: an empty, "null" or "{}" 2xx is
// refused earlier by the separate empty-body guard (TestEmptySuccessBodyIsRefused),
// so a cut stream is what isolates the parser swap itself.
func TestRetrofittedReadReportsAnUnreadableBodyAsADecodeError(t *testing.T) {
	for _, read := range retrofittedReads {
		t.Run(read.name, func(t *testing.T) {
			_, err := read.call(t, emptyBodyServer(t, http.StatusOK, `{"id":"a`))
			if err == nil {
				t.Fatal("a 200 carrying a truncated body is not a readable resource")
			}

			if sdkerrors.IsInternalError(err) {
				t.Fatalf("an unreadable 2xx must not read as an SDK-internal fault: %v", err)
			}

			if !sdkerrors.IsResponseDecodeError(err) {
				t.Fatalf("want a response-decode error (the server answered, the SDK could not read it), got %v", err)
			}
		})
	}
}

// TestRetrofittedReadStillDecodesARealBody is the positive half. The retrofit
// changed which code reads the response, so every refusal above would also pass
// on a facade that had simply stopped reading anything — asserting the id landed
// is what separates "refuses the bad shapes" from "refuses everything".
func TestRetrofittedReadStillDecodesARealBody(t *testing.T) {
	for _, read := range retrofittedReads {
		t.Run(read.name, func(t *testing.T) {
			got, err := read.call(t, emptyBodyServer(t, http.StatusOK, read.okBody))
			if err != nil {
				t.Fatalf("a well-formed 200 body must decode, got %v", err)
			}

			if got != txID {
				t.Fatalf("want the decoded id %q, got %q", txID, got)
			}
		})
	}
}

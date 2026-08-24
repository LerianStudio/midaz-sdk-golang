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

// emptySuccessBodies are the five 2xx shapes that carry no resource. Each one
// unmarshals into a zero-valued model with a NIL error, which is the whole
// defect: a caller who branches on err != nil books a settled transfer whose id
// is "" and whose status is "".
//
//   - ""       a proxy or gateway that dropped the body
//   - "  "     the same, with whitespace surviving
//   - null     the JSON literal; json.Unmarshal on it is a documented no-op
//   - "null\n" the same, as most JSON writers actually emit it — a trailing
//     newline is the default from json.Encoder and from a shell pipe, so this is
//     the spelling a real proxy is likeliest to produce. It is here because the
//     guard's TrimSpace was pinned by exactly ONE row (the Cancel whitespace
//     case), which left every other endpoint free to lose the trim unnoticed.
//   - {}       a well-formed object that sets nothing
var emptySuccessBodies = []struct {
	name string
	body string
}{
	{name: "empty body", body: ""},
	{name: "whitespace body", body: "  \n\t "},
	{name: "null literal", body: "null"},
	{name: "null literal with trailing newline", body: "null\n"},
	{name: "empty object", body: "{}"},
}

// emptyBodyServer answers every request with status and body.
func emptyBodyServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(srv.Close)

	return srv
}

// TestEmptySuccessBodyIsRefused is the money-path guard behind the central
// decodeOne empty-body check.
//
// The failure it prevents is the worst shape a money SDK has: a 201 whose body
// the server (or any gateway in front of it) dropped, decoded into a
// zero-valued transaction and returned with a nil error. Nothing downstream can
// tell that apart from a real settlement — the id is "", the status is "", and
// err is nil. Refusing it as a RESPONSE DECODE error is the honest answer,
// because on a create the transaction may well exist server-side: the caller
// must reconcile, not retry blindly.
//
// One create, one lifecycle transition and one plain read, on BOTH surfaces,
// because the guard is central and any of the twenty single-object reads would
// prove it — these six are the ones where being wrong costs money.
func TestEmptySuccessBodyIsRefused(t *testing.T) {
	ctx := context.Background()

	calls := []struct {
		name   string
		status int
		call   func(v1 *transactionsFacade, v2 *transactionsV2Facade) error
	}{
		{
			name:   "V1.Transactions.CreateJSON",
			status: http.StatusCreated,
			call: func(v1 *transactionsFacade, _ *transactionsV2Facade) error {
				_, err := v1.CreateJSON(ctx, txOrgID, txLedgerID, sampleTransactionInput())

				return err
			},
		},
		{
			name:   "V1.Transactions.Commit",
			status: http.StatusCreated,
			call: func(v1 *transactionsFacade, _ *transactionsV2Facade) error {
				_, err := v1.Commit(ctx, txOrgID, txLedgerID, txID)

				return err
			},
		},
		{
			name:   "V1.Transactions.Get",
			status: http.StatusOK,
			call: func(v1 *transactionsFacade, _ *transactionsV2Facade) error {
				_, err := v1.Get(ctx, txOrgID, txLedgerID, txID)

				return err
			},
		},
		{
			name:   "V2.Transactions.CreateDirect",
			status: http.StatusCreated,
			call: func(_ *transactionsFacade, v2 *transactionsV2Facade) error {
				_, err := v2.CreateDirect(ctx, txOrgID, txLedgerID, sampleV2Input())

				return err
			},
		},
		{
			name:   "V2.Transactions.Revert",
			status: http.StatusCreated,
			call: func(_ *transactionsFacade, v2 *transactionsV2Facade) error {
				_, err := v2.Revert(ctx, txOrgID, txLedgerID, txID)

				return err
			},
		},
		{
			name:   "V2.Transactions.Get",
			status: http.StatusOK,
			call: func(_ *transactionsFacade, v2 *transactionsV2Facade) error {
				_, err := v2.Get(ctx, txOrgID, txLedgerID, txID)

				return err
			},
		},
	}

	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			for _, shape := range emptySuccessBodies {
				t.Run(shape.name, func(t *testing.T) {
					srv := emptyBodyServer(t, c.status, shape.body)

					err := c.call(newTestTransactionsFacade(t, srv), newTestTransactionsV2Facade(t, srv))
					if err == nil {
						t.Fatalf("a %d carrying %q must not read as a successful %s",
							c.status, shape.body, c.name)
					}

					if !sdkerrors.IsResponseDecodeError(err) {
						t.Fatalf("want a response-decode error (outcome unknown), got %v", err)
					}
				})
			}
		})
	}
}

// TestEmptySuccessBodyGuardLetsRealBodiesThrough is the other half: a guard
// that refused everything would also pass the test above. These are the bodies
// the guard must NOT touch.
func TestEmptySuccessBodyGuardLetsRealBodiesThrough(t *testing.T) {
	ctx := context.Background()

	t.Run("V1 transaction body decodes", func(t *testing.T) {
		srv := emptyBodyServer(t, http.StatusOK, txResponseBody())

		got, err := newTestTransactionsFacade(t, srv).Get(ctx, txOrgID, txLedgerID, txID)
		if err != nil {
			t.Fatalf("a populated body must decode: %v", err)
		}

		if got.ID != txID {
			t.Fatalf("id = %q, want %q", got.ID, txID)
		}
	})

	t.Run("V2 transaction body decodes", func(t *testing.T) {
		srv := emptyBodyServer(t, http.StatusOK, `{"id":"`+txID+`","status":{"code":"APPROVED"}}`)

		got, err := newTestTransactionsV2Facade(t, srv).Get(ctx, txOrgID, txLedgerID, txID)
		if err != nil {
			t.Fatalf("a populated body must decode: %v", err)
		}

		if got.ID != txID {
			t.Fatalf("id = %q, want %q", got.ID, txID)
		}
	})
}

// listReads are the list surfaces the empty-body guard is exercised on, one per
// DECODE PATH rather than one per endpoint.
//
// BOTH versions AND BOTH PLANES are here on purpose. The guard lived on the v2
// helper alone for one fix round while all twenty-six v1 lists decoded their
// envelope inline, so a v1 caller walking a ledger still read a dropped body as
// an empty page. Any v1 row would have caught that.
//
// The four tracer rows are here for the sibling reason: the tracer lists do NOT
// route through readList. Each one unmarshals its own domain-keyed envelope and
// calls guardListBody directly, so the ledger rows below prove nothing about
// them — reverting all four tracer call sites to a bare non-2xx check left this
// whole suite green while the silent-empty-page defect was live on every tracer
// list. Four distinct envelopes, four rows.
var listReads = []struct {
	name string
	// emptyPage is what a REAL empty page looks like on this endpoint. It is
	// the other half of the guard: one that refused everything would pass the
	// no-page cases too. The ledger envelope marks items and limit REQUIRED;
	// each tracer list names its own collection field instead, so the shape is
	// per-row and a shared body would exercise none of them.
	emptyPage string
	call      func(t *testing.T, srv *httptest.Server) error
}{
	{
		name:      "V1.Transactions.List",
		emptyPage: `{"items":[],"limit":10}`,
		call: func(t *testing.T, srv *httptest.Server) error {
			t.Helper()

			_, err := newTestTransactionsFacade(t, srv).
				List(context.Background(), txOrgID, txLedgerID, models.TransactionsListOpts{})

			return err
		},
	},
	{
		name:      "V1.Accounts.List",
		emptyPage: `{"items":[],"limit":10}`,
		call: func(t *testing.T, srv *httptest.Server) error {
			t.Helper()

			_, err := newTestAccountsFacade(t, srv).
				List(context.Background(), txOrgID, txLedgerID, models.AccountsListOpts{})

			return err
		},
	},
	{
		name:      "V1.Balances.ListBalances",
		emptyPage: `{"items":[],"limit":10}`,
		call: func(t *testing.T, srv *httptest.Server) error {
			t.Helper()

			_, err := newBalancesFacade(newTestLedgerClient(t, srv), true).
				ListBalances(context.Background(), txOrgID, txLedgerID, models.BalancesListOpts{})

			return err
		},
	},
	{
		name:      "V2.Transactions.List",
		emptyPage: `{"items":[],"limit":10}`,
		call: func(t *testing.T, srv *httptest.Server) error {
			t.Helper()

			_, err := newTestTransactionsV2Facade(t, srv).
				List(context.Background(), txOrgID, txLedgerID, models.TransactionsListOpts{})

			return err
		},
	},
	{
		name:      "Tracer.Rules.List",
		emptyPage: `{"rules":[],"hasMore":false,"nextCursor":""}`,
		call: func(t *testing.T, srv *httptest.Server) error {
			t.Helper()

			_, err := newTestRulesFacade(t, srv).List(context.Background(), models.RulesListOpts{})

			return err
		},
	},
	{
		name:      "Tracer.Limits.List",
		emptyPage: `{"limits":[],"hasMore":false,"nextCursor":""}`,
		call: func(t *testing.T, srv *httptest.Server) error {
			t.Helper()

			_, err := newTestLimitsFacade(t, srv).List(context.Background(), models.LimitsListOpts{})

			return err
		},
	},
	{
		name:      "Tracer.Validations.List",
		emptyPage: `{"transactionValidations":[],"hasMore":false,"nextCursor":""}`,
		call: func(t *testing.T, srv *httptest.Server) error {
			t.Helper()

			_, err := newTestValidationsFacade(t, srv).List(context.Background(), models.ValidationsListOpts{})

			return err
		},
	},
	{
		name:      "Tracer.AuditEvents.List",
		emptyPage: `{"auditEvents":[],"hasMore":false,"nextCursor":""}`,
		call: func(t *testing.T, srv *httptest.Server) error {
			t.Helper()

			_, err := newTestAuditEventsFacade(t, srv).
				List(context.Background(), models.AuditEventRecordsListOpts{})

			return err
		},
	},
}

// assertEmptyPageRefused runs one list read against one no-page body shape.
func assertEmptyPageRefused(t *testing.T, call func(*testing.T, *httptest.Server) error, body string) {
	t.Helper()

	err := call(t, emptyBodyServer(t, http.StatusOK, body))
	if err == nil {
		t.Fatalf("a 200 carrying %q must not read as an empty page", body)
	}

	if !sdkerrors.IsResponseDecodeError(err) {
		t.Fatalf("want a response-decode error, got %v", err)
	}
}

// TestEmptySuccessBodyIsRefusedOnAList is the same defect one shape over, and on
// a reconciliation path it is arguably the worse one: "null" and "{}" decode
// into a zero-valued envelope, which reads as an EMPTY PAGE with no next cursor.
// A caller walking a ledger's transactions stops on the first dropped body and
// concludes the ledger is empty — with a nil error.
//
// No legitimate list body is any of these shapes: every list envelope the ledger
// declares marks items and limit REQUIRED, so an empty page is
// {"items":[],"limit":N}.
func TestEmptySuccessBodyIsRefusedOnAList(t *testing.T) {
	for _, read := range listReads {
		t.Run(read.name, func(t *testing.T) {
			for _, shape := range emptySuccessBodies {
				t.Run(shape.name, func(t *testing.T) {
					assertEmptyPageRefused(t, read.call, shape.body)
				})
			}

			t.Run("a real empty page still decodes", func(t *testing.T) {
				srv := emptyBodyServer(t, http.StatusOK, read.emptyPage)

				if err := read.call(t, srv); err != nil {
					t.Fatalf("an empty page is a real answer and must decode: %v", err)
				}
			})
		})
	}

	t.Run("a real page decodes with its items", func(t *testing.T) {
		srv := emptyBodyServer(t, http.StatusOK, `{"items":[],"limit":10}`)

		page, err := newTestTransactionsV2Facade(t, srv).
			List(context.Background(), txOrgID, txLedgerID, models.TransactionsListOpts{})
		if err != nil {
			t.Fatalf("an empty page is a real answer and must decode: %v", err)
		}

		if len(page.Items) != 0 {
			t.Fatalf("items = %d, want 0", len(page.Items))
		}
	})
}

// bareArrayReads are the endpoints that answer with a bare JSON ARRAY instead of
// a paginated envelope: the two point-in-time balance reads (reachable through
// three facades) and the metadata-index list, on BOTH surfaces.
//
// The v1 half is here because it was the last corner still reading through the
// generated status-exact parser after the rest of the SDK moved off it. That
// parser unmarshals the body itself whenever the content type says json, so it
// failed BEFORE any facade logic ran, and the facade could only report the
// parser's failure: an unreadable 2xx became internal_error rather than a decode
// error, and — the expensive one — a gateway 403 or 404 with an empty body
// became internal_error/500, destroying the status the caller needed to act on.
// The v2 twins had none of that, so the same endpoint answered differently
// depending on which surface reached it.
var bareArrayReads = []struct {
	name string
	call func(t *testing.T, srv *httptest.Server) (int, error)
}{
	{
		name: "V1.Balances.GetAccountBalancesHistory",
		call: func(t *testing.T, srv *httptest.Server) (int, error) {
			t.Helper()

			got, err := newBalancesFacade(newTestLedgerClient(t, srv), true).
				GetAccountBalancesHistory(context.Background(), txOrgID, txLedgerID, txID, balanceInstant)

			return len(got), err
		},
	},
	{
		name: "V1.Accounts.BalancesAtTimestamp",
		call: func(t *testing.T, srv *httptest.Server) (int, error) {
			t.Helper()

			got, err := newTestAccountsFacade(t, srv).
				BalancesAtTimestamp(context.Background(), txOrgID, txLedgerID, txID, balanceInstant)

			return len(got), err
		},
	},
	{
		name: "V1.MetadataIndexes.List",
		call: func(t *testing.T, srv *httptest.Server) (int, error) {
			t.Helper()

			got, err := newTestMetadataIndexesFacade(t, srv).List(context.Background(), "transaction")

			return len(got), err
		},
	},
	{
		name: "V2.Balances.GetAccountBalancesHistory",
		call: func(t *testing.T, srv *httptest.Server) (int, error) {
			t.Helper()

			got, err := newBalancesV2Facade(newTestLedgerClient(t, srv), true).
				GetAccountBalancesHistory(context.Background(), txOrgID, txLedgerID, txID, balanceInstant)

			return len(got), err
		},
	},
}

// balanceInstant is a point in time the balance-history date contract accepts.
const balanceInstant = "2025-01-01T00:00:00Z"

// TestBareArrayReadStillAcceptsNull pins the deliberate exception to the
// empty-body guard.
//
// These endpoints answer with a bare JSON ARRAY, and Go's encoding/json marshals
// a nil slice as the literal "null". So on that shape "null" is what a handler
// with no results actually emits, and applying the object guard there would turn
// "no balances at that instant" into an error.
func TestBareArrayReadStillAcceptsNull(t *testing.T) {
	for _, read := range bareArrayReads {
		t.Run(read.name, func(t *testing.T) {
			n, err := read.call(t, emptyBodyServer(t, http.StatusOK, "null"))
			if err != nil {
				t.Fatalf("a bare array read must treat null as an empty result set, got %v", err)
			}

			if n != 0 {
				t.Fatalf("want an empty result set, got %d", n)
			}
		})
	}
}

// TestBareArrayReadKeepsTheRealErrorStatus is the failure the generated parser
// was hiding, and the one that costs an operator the most.
//
// A gateway answers 403 or 404 with "Content-Type: application/json" and NO
// body. The caller needs "you are not allowed" or "it is not there" — two
// different next actions. Through the generated parser both arrived as an
// internal error carrying status 500: retryable-looking, unattributable, and
// wrong about whose fault it is.
func TestBareArrayReadKeepsTheRealErrorStatus(t *testing.T) {
	statuses := []struct {
		status int
		is     func(error) bool
		name   string
	}{
		{http.StatusForbidden, sdkerrors.IsAuthorizationError, "403 stays an authorization error"},
		{http.StatusNotFound, sdkerrors.IsNotFoundError, "404 stays a not-found error"},
	}

	for _, read := range bareArrayReads {
		t.Run(read.name, func(t *testing.T) {
			for _, s := range statuses {
				t.Run(s.name, func(t *testing.T) {
					_, err := read.call(t, emptyBodyServer(t, s.status, ""))
					if err == nil {
						t.Fatalf("a %d must not read as a successful empty result set", s.status)
					}

					if sdkerrors.IsInternalError(err) {
						t.Fatalf("a %d with an empty body must not become an SDK-internal fault: %v", s.status, err)
					}

					if !s.is(err) {
						t.Fatalf("want the %d preserved, got %v", s.status, err)
					}
				})
			}
		})
	}
}

// TestBareArrayReadReportsAnUnreadableBodyAsADecodeError is the other half of the
// same swap: a 2xx whose body cannot be read is a fact about the RESPONSE, not
// about the SDK. Reporting it as internal sends the caller looking for a bug in
// their own client.
func TestBareArrayReadReportsAnUnreadableBodyAsADecodeError(t *testing.T) {
	// Truncated (a proxy that cut the stream) and dropped entirely — the two
	// shapes a bare-array read can receive that "null" does not cover.
	for _, body := range []string{`[{"id":"a"`, ""} {
		for _, read := range bareArrayReads {
			t.Run(read.name+"/"+body, func(t *testing.T) {
				_, err := read.call(t, emptyBodyServer(t, http.StatusOK, body))
				if err == nil {
					t.Fatalf("a 200 carrying %q is not a readable result set", body)
				}

				if !sdkerrors.IsResponseDecodeError(err) {
					t.Fatalf("want a response-decode error (the server answered, the SDK could not read it), got %v", err)
				}
			})
		}
	}
}

// TestCancelStillSynthesizesOnAnEmptyBody pins the ONE endpoint that is allowed
// to answer a single-object request with nothing.
//
// Cancel is exempt because its outcome is fully determined by the request: the
// transaction the caller named is CANCELED. Commit and Revert are not — a
// revert's answer is a NEW child transaction whose id the caller cannot know,
// and a commit's answer carries the settled state the caller called to observe.
// Synthesizing either would invent money data; failing loudly does not.
func TestCancelStillSynthesizesOnAnEmptyBody(t *testing.T) {
	ctx := context.Background()

	for _, shape := range emptySuccessBodies {
		t.Run(shape.name, func(t *testing.T) {
			srv := emptyBodyServer(t, http.StatusCreated, shape.body)

			v1, err := newTestTransactionsFacade(t, srv).Cancel(ctx, txOrgID, txLedgerID, txID)
			if err != nil {
				t.Fatalf("V1 cancel must synthesize, got %v", err)
			}

			if v1.ID != txID || v1.Status.Code != string(models.TransactionStatusCanceled) {
				t.Fatalf("V1 cancel = {%q,%q}, want {%q,CANCELED}", v1.ID, v1.Status.Code, txID)
			}

			v2, err := newTestTransactionsV2Facade(t, srv).Cancel(ctx, txOrgID, txLedgerID, txID)
			if err != nil {
				t.Fatalf("V2 cancel must synthesize, got %v", err)
			}

			if v2.ID != txID || v2.Status.Code != string(models.TransactionStatusCanceled) {
				t.Fatalf("V2 cancel = {%q,%q}, want {%q,CANCELED}", v2.ID, v2.Status.Code, txID)
			}
		})
	}
}

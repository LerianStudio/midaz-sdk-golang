// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

func newTestTransactionsV2Facade(t *testing.T, srv *httptest.Server) *transactionsV2Facade {
	t.Helper()

	return newTransactionsV2Facade(newTestLedgerClient(t, srv), true)
}

// sampleV2Input builds a minimal, valid two-leg transaction with the leg scope
// left EMPTY, which is what a caller writes when they let the facade fill it.
func sampleV2Input() *models.CreateTransactionV2Input {
	return &models.CreateTransactionV2Input{
		Asset:   "USD",
		Amount:  "100",
		Debits:  []models.TransactionV2Leg{{Alias: "@src", Amount: "100"}},
		Credits: []models.TransactionV2Leg{{Alias: "@dst", Amount: "100"}},
	}
}

// TestTransactionsV2Facade_RefusesContradictingLegScope is the money-path guard
// on the scope reconciliation.
//
// /v2 resolves which ledger a transaction is created in from the BODY, and it
// refuses a body whose legs disagree. So the failure this prevents is not a
// rejected request — the server would catch that — it is the one where the
// caller addresses ledger A, one leg out of twelve says ledger B, and nobody
// notices which ledger the SDK meant. Refusing locally names the side and the
// index, which the server's own rejection does not.
func TestTransactionsV2Facade_RefusesContradictingLegScope(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*models.CreateTransactionV2Input)
		want string
	}{
		{
			name: "debit leg names another ledger",
			mut:  func(in *models.CreateTransactionV2Input) { in.Debits[0].LedgerID = "other-ledger" },
			want: "debits[0].ledgerId",
		},
		{
			name: "credit leg names another organization",
			mut:  func(in *models.CreateTransactionV2Input) { in.Credits[0].OrganizationID = "other-org" },
			want: "credits[0].organizationId",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reached bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached = true

				w.WriteHeader(http.StatusCreated)
			}))
			defer srv.Close()

			input := sampleV2Input()
			tt.mut(input)

			_, err := newTestTransactionsV2Facade(t, srv).CreateDirect(context.Background(), txOrgID, txLedgerID, input)
			if err == nil {
				t.Fatal("expected a refusal for a leg naming a different scope")
			}

			if !sdkerrors.IsValidationError(err) {
				t.Fatalf("err = %v, want a validation error", err)
			}

			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to name %q so the caller knows which leg to fix", err, tt.want)
			}

			if reached {
				t.Fatal("a contradicting scope must not reach the wire")
			}
		})
	}
}

// TestTransactionsV2Facade_AcceptsMatchingLegScopeInAnyCase pins that a leg
// spelling the addressed pair in a different letter case is ACCEPTED.
//
// A UUID's text spelling is case-insensitive, and the server compares the two
// that way. Being stricter here would refuse a body the ledger accepts — a
// transaction rejected by the SDK for a reason the server does not recognise.
func TestTransactionsV2Facade_AcceptsMatchingLegScopeInAnyCase(t *testing.T) {
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"` + txID + `","status":{"code":"APPROVED"}}`))
	}))
	defer srv.Close()

	input := sampleV2Input()
	input.Debits[0].OrganizationID = strings.ToUpper(txOrgID)
	input.Debits[0].LedgerID = strings.ToUpper(txLedgerID)

	if _, err := newTestTransactionsV2Facade(t, srv).CreateDirect(context.Background(), txOrgID, txLedgerID, input); err != nil {
		t.Fatalf("CreateDirect: %v", err)
	}

	// The leg keeps its own spelling — the facade fills empties, it does not
	// rewrite what the caller already said.
	var wire struct {
		Debits []map[string]any `json:"debits"`
	}

	if err := json.Unmarshal(gotBody, &wire); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, gotBody)
	}

	if wire.Debits[0]["organizationId"] != strings.ToUpper(txOrgID) {
		t.Fatalf("debit leg organizationId = %v, want the caller's own spelling %q",
			wire.Debits[0]["organizationId"], strings.ToUpper(txOrgID))
	}
}

// TestTransactionsV2Facade_DecodesTheV2Shape pins the response divergence that
// makes TransactionV2 a separate type rather than an alias.
//
// /v2 dropped four /v1 fields and kept two /v1 dropped. The two it kept are the
// ones worth asserting: feesSkipped and tracerSkipped tell a caller whether the
// fee engine and Tracer ran at all, and a model that silently discarded them
// would leave a reconciliation client unable to explain a transaction that
// charged no fee.
func TestTransactionsV2Facade_DecodesTheV2Shape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"` + txID + `",
			"amount":"1500.00000001",
			"assetCode":"USD",
			"debit":["@src"],
			"credit":["@dst"],
			"feesSkipped":true,
			"tracerSkipped":true,
			"status":{"code":"APPROVED"},
			"operations":[{"id":"op-1","amount":{"value":"1500.00000001"},"type":"DEBIT"}]
		}`))
	}))
	defer srv.Close()

	tx, err := newTestTransactionsV2Facade(t, srv).Get(context.Background(), txOrgID, txLedgerID, txID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if !tx.FeesSkipped || !tx.TracerSkipped {
		t.Fatalf("feesSkipped=%v tracerSkipped=%v, want both true: /v2 serves these and /v1 does not",
			tx.FeesSkipped, tx.TracerSkipped)
	}

	if tx.Amount != "1500.00000001" {
		t.Fatalf("amount = %q, want the exact decimal the server sent", tx.Amount)
	}

	if len(tx.Debit) != 1 || tx.Debit[0] != "@src" {
		t.Fatalf("debit = %v, want the alias list /v2 spells debit (not source)", tx.Debit)
	}

	if len(tx.Operations) != 1 || tx.Operations[0].Amount.Value == nil ||
		tx.Operations[0].Amount.Value.String() != "1500.00000001" {
		t.Fatalf("operation amount did not survive as a decimal: %+v", tx.Operations)
	}
}

// TestTransactionsV2Facade_CancelSynthesizesOnEmptyBody pins the one lifecycle
// action that can answer with nothing.
//
// The server projects the canonical transaction onto the /v2 shape and that
// projection is nil-preserving, so a cancel can come back as an empty body or
// the literal "null". Failing the decode there would report a CANCELLED
// transaction as an error and invite the caller to cancel it again.
func TestTransactionsV2Facade_CancelSynthesizesOnEmptyBody(t *testing.T) {
	for _, body := range []string{"", "null"} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(body))
		}))

		tx, err := newTestTransactionsV2Facade(t, srv).Cancel(context.Background(), txOrgID, txLedgerID, txID)
		srv.Close()

		if err != nil {
			t.Fatalf("Cancel with body %q: %v", body, err)
		}

		if tx.ID != txID || tx.Status.Code != string(models.TransactionStatusCanceled) {
			t.Fatalf("Cancel with body %q returned %+v, want the cancelled transaction's id and status", body, tx)
		}
	}
}

// TestTransactionsV2Facade_CreateRefusesInvalidPayloadLocally pins that a payload
// the SDK can see is wrong is classified as a validation failure and never
// reaches the wire. On a create, an unclassified failure is the one a caller
// retries — against a request that never left.
func TestTransactionsV2Facade_CreateRefusesInvalidPayloadLocally(t *testing.T) {
	tests := []struct {
		name  string
		input *models.CreateTransactionV2Input
	}{
		{
			name: "no credit side",
			input: &models.CreateTransactionV2Input{
				Asset: "USD", Amount: "100",
				Debits: []models.TransactionV2Leg{{Alias: "@src", Amount: "100"}},
			},
		},
		{
			name: "leg carries both an amount and a share",
			input: &models.CreateTransactionV2Input{
				Asset: "USD", Amount: "100",
				Debits: []models.TransactionV2Leg{{
					Alias: "@src", Amount: "100", Share: &models.TransactionV2Share{Percentage: 100},
				}},
				Credits: []models.TransactionV2Leg{{Alias: "@dst", Amount: "100"}},
			},
		},
		{
			name: "leg carries neither an amount nor a share",
			input: &models.CreateTransactionV2Input{
				Asset: "USD", Amount: "100",
				Debits:  []models.TransactionV2Leg{{Alias: "@src"}},
				Credits: []models.TransactionV2Leg{{Alias: "@dst", Amount: "100"}},
			},
		},
		{
			name: "zero share moves nothing while the transaction commits",
			input: &models.CreateTransactionV2Input{
				Asset: "USD", Amount: "100",
				Debits:  []models.TransactionV2Leg{{Alias: "@src", Share: &models.TransactionV2Share{Percentage: 0}}},
				Credits: []models.TransactionV2Leg{{Alias: "@dst", Amount: "100"}},
			},
		},
		{
			name: "non-positive total",
			input: &models.CreateTransactionV2Input{
				Asset: "USD", Amount: "0",
				Debits:  []models.TransactionV2Leg{{Alias: "@src", Amount: "0"}},
				Credits: []models.TransactionV2Leg{{Alias: "@dst", Amount: "0"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reached bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached = true

				w.WriteHeader(http.StatusCreated)
			}))
			defer srv.Close()

			_, err := newTestTransactionsV2Facade(t, srv).CreateDirect(context.Background(), txOrgID, txLedgerID, tt.input)
			if err == nil {
				t.Fatal("expected a local refusal")
			}

			if !sdkerrors.IsValidationError(err) {
				t.Fatalf("err = %v, want a validation error", err)
			}

			if reached {
				t.Fatal("a locally refused payload must not reach the wire")
			}
		})
	}
}

// TestTransactionsV2Facade_CreateRequiresAnAddressedScope pins that the two scope
// arguments are mandatory. They are not path segments, so the shared path-id
// guard does not cover them — without this check an empty pair would be stamped
// onto every leg and the server would answer 400 for a mistake the SDK could see.
func TestTransactionsV2Facade_CreateRequiresAnAddressedScope(t *testing.T) {
	tests := []struct {
		name     string
		orgID    string
		ledgerID string
	}{
		{"no organization", "", txLedgerID},
		{"no ledger", txOrgID, ""},
		{"blank organization", "   ", txLedgerID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reached bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached = true

				w.WriteHeader(http.StatusCreated)
			}))
			defer srv.Close()

			_, err := newTestTransactionsV2Facade(t, srv).CreateDirect(context.Background(), tt.orgID, tt.ledgerID, sampleV2Input())
			if err == nil {
				t.Fatal("expected a refusal for a missing scope")
			}

			if reached {
				t.Fatal("a missing scope must not reach the wire")
			}
		})
	}
}

// TestTransactionsV2Facade_ListAdvancesByCursor is the infinite-loop guard for the
// /v2 transaction iterator. The endpoint advances by next_cursor, so an iterator
// that incremented a page number would re-request the FIRST page for as long as
// the server reported more results, yielding the same transactions forever. The
// request cap turns that into a fast failure instead of a hang.
func TestTransactionsV2Facade_ListAdvancesByCursor(t *testing.T) {
	var seenCursors []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		seenCursors = append(seenCursors, cursor)

		if len(seenCursors) > 4 {
			t.Fatalf("iterator did not terminate: cursors=%v", seenCursors)
		}

		w.Header().Set("Content-Type", "application/json")

		if cursor == "cur-2" {
			_, _ = w.Write([]byte(`{"items":[{"id":"t-2","amount":"20"}],"limit":1}`))
			return
		}

		_, _ = w.Write([]byte(`{"items":[{"id":"t-1","amount":"10"}],"limit":1,"next_cursor":"cur-2"}`))
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
		t.Fatalf("ids = %v, want [t-1 t-2]", ids)
	}

	if len(seenCursors) != 2 || seenCursors[0] != "" || seenCursors[1] != "cur-2" {
		t.Fatalf("cursors = %v, want the iterator to advance by next_cursor", seenCursors)
	}
}

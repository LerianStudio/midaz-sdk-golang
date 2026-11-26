// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"
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

// TestTransactionsV2Facade_CommitAndRevertStayBodiless pins the wire commit and
// revert shipped with: no body. The server's lifecycle body is optional and only
// carries an account-block grant the SDK never sends.
func TestTransactionsV2Facade_CommitAndRevertStayBodiless(t *testing.T) {
	actions := map[string]func(*transactionsV2Facade) (*models.TransactionV2, error){
		"commit": func(f *transactionsV2Facade) (*models.TransactionV2, error) {
			return f.Commit(context.Background(), txOrgID, txLedgerID, txID)
		},
		"revert": func(f *transactionsV2Facade) (*models.TransactionV2, error) {
			return f.Revert(context.Background(), txOrgID, txLedgerID, txID)
		},
	}

	for name, action := range actions {
		t.Run(name, func(t *testing.T) {
			var body []byte

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ = io.ReadAll(r.Body)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":"tx-1","status":{"code":"APPROVED"}}`))
			}))
			defer srv.Close()

			if _, err := action(newTestTransactionsV2Facade(t, srv)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}

			if len(body) != 0 {
				t.Fatalf("%s sent body %q, want none", name, body)
			}
		})
	}
}

// TestTransactionsV2Facade_LifecycleReadsCrossLedgerGroup pins commit, cancel and
// revert on a transaction that belongs to a cross-ledger group: the server answers with
// the whole group, and the call returns the member it was about. A group with no
// such member is a decode error naming the group, never an empty transaction.
func TestTransactionsV2Facade_LifecycleReadsCrossLedgerGroup(t *testing.T) {
	const (
		groupID = "4d5e6f70-4444-4444-8444-4d5e6f708192"
		otherID = "5e6f7081-5555-4555-8555-5e6f708192a3"
	)

	commit := func(f *transactionsV2Facade) (*models.TransactionV2, error) {
		return f.Commit(context.Background(), txOrgID, txLedgerID, txID)
	}
	revert := func(f *transactionsV2Facade) (*models.TransactionV2, error) {
		return f.Revert(context.Background(), txOrgID, txLedgerID, txID)
	}
	cancel := func(f *transactionsV2Facade) (*models.TransactionV2, error) {
		return f.Cancel(context.Background(), txOrgID, txLedgerID, txID)
	}

	tests := []struct {
		name   string
		call   func(*transactionsV2Facade) (*models.TransactionV2, error)
		body   string
		wantID string // "" means a decode error naming the group
	}{
		{
			name: "commit returns the committed member",
			call: commit,
			body: `{"groupId":"` + groupID + `","transactions":[` +
				`{"id":"` + otherID + `","status":{"code":"APPROVED"},"order":1},` +
				`{"id":"` + txID + `","status":{"code":"APPROVED"},"order":2}]}`,
			wantID: txID,
		},
		{
			name: "cancel returns the cancelled member",
			call: cancel,
			body: `{"groupId":"` + groupID + `","transactions":[` +
				`{"id":"` + otherID + `","status":{"code":"CANCELED"},"order":1},` +
				`{"id":"` + txID + `","status":{"code":"CANCELED"},"order":2}]}`,
			wantID: txID,
		},
		{
			name: "cancel group without the addressed member",
			call: cancel,
			body: `{"groupId":"` + groupID + `","transactions":[{"id":"` + otherID + `","status":{"code":"CANCELED"}}]}`,
		},
		{
			name: "revert returns the reversal of the addressed transaction",
			call: revert,
			body: `{"groupId":"` + groupID + `","revertedGroupId":"` + otherID + `","transactions":[` +
				`{"id":"rev-other","parentTransactionId":"` + otherID + `","status":{"code":"APPROVED"},"order":1},` +
				`{"id":"rev-addressed","parentTransactionId":"` + txID + `","status":{"code":"APPROVED"},"order":2}]}`,
			wantID: "rev-addressed",
		},
		{
			name: "commit group without the addressed member",
			call: commit,
			body: `{"groupId":"` + groupID + `","transactions":[{"id":"` + otherID + `","status":{"code":"APPROVED"}}]}`,
		},
		{
			name: "revert group without the addressed reversal",
			call: revert,
			body: `{"groupId":"` + groupID + `","transactions":[{"id":"rev-other","parentTransactionId":"` + otherID + `"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			tx, err := tt.call(newTestTransactionsV2Facade(t, srv))

			if tt.wantID == "" {
				if !sdkerrors.IsResponseDecodeError(err) || !strings.Contains(err.Error(), groupID) {
					t.Fatalf("got (%+v, %v), want a response-decode error naming group %s", tx, err, groupID)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tx.ID != tt.wantID {
				t.Fatalf("returned transaction %q, want %q", tx.ID, tt.wantID)
			}
		})
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

// TestTransactionsCountRefusesUndeclaredFilters is the count-side twin of the
// list refusal, on BOTH surfaces.
//
// The count endpoint declares status, route and the date range and nothing else,
// so AssetCode, Reference, SourceAccount, DestinationAccount and the metadata
// predicate had no wire slot and were dropped in silence. That is worse than the
// same drop on List: a count answers with a NUMBER, so an unnarrowed total is
// plausible, unattributable, and read as the narrowed one. The assertion is that
// nothing was sent.
func TestTransactionsCountRefusesUndeclaredFilters(t *testing.T) {
	refused := []struct {
		name    string
		filters models.TransactionsFilters
		named   string
	}{
		{"an asset code", models.TransactionsFilters{AssetCode: "USD"}, "AssetCode"},
		{"a reference", models.TransactionsFilters{Reference: "ref-1"}, "Reference"},
		{"a source account", models.TransactionsFilters{SourceAccount: "@src"}, "SourceAccount"},
		{"a destination account", models.TransactionsFilters{DestinationAccount: "@dst"}, "DestinationAccount"},
		{"a metadata predicate", models.TransactionsFilters{MetadataKey: "transferId", MetadataValue: "t-1"}, "MetadataKey"},
	}

	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			var requests int

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests++

				w.Header().Set("X-Total-Count", "42")
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			opts := models.TransactionsListOpts{Filters: tt.filters}

			assertCountRefuses(t, "Transactions.Count", tt.named, func() (int, error) {
				return newTestTransactionsFacade(t, srv).Count(context.Background(), txOrgID, txLedgerID, opts)
			})
			assertCountRefuses(t, "V2.Transactions.Count", tt.named, func() (int, error) {
				return newTestTransactionsV2Facade(t, srv).Count(context.Background(), txOrgID, txLedgerID, opts)
			})

			if requests != 0 {
				t.Fatalf("issued %d requests; a filter the count cannot express must be refused before the wire", requests)
			}
		})
	}
}

// assertCountRefuses checks one Count surface refuses one undeclared filter, as
// a validation error naming the field.
func assertCountRefuses(t *testing.T, surface, field string, count func() (int, error)) {
	t.Helper()

	got, err := count()
	if err == nil {
		t.Fatalf("%s returned %d; %s has no wire slot and must be refused, not dropped", surface, got, field)
	}

	if !sdkerrors.IsValidationError(err) {
		t.Fatalf("%s err = %v, want a validation error the caller can act on", surface, err)
	}

	if !strings.Contains(err.Error(), field) {
		t.Fatalf("%s err = %v, want the refusal to name %s", surface, err, field)
	}
}

// TestTransactionsCountStillSendsItsDeclaredFilters is the boundary on the
// refusal above: Status, Route and the date range ARE declared on the count
// endpoint, so the new guard must not have swallowed the two filters Count
// exists to honour.
func TestTransactionsCountStillSendsItsDeclaredFilters(t *testing.T) {
	var query string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery

		w.Header().Set("X-Total-Count", "7")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	honoured := models.TransactionsListOpts{
		CursorListOpts: models.CursorListOpts{StartDate: "2026-01-01", EndDate: "2026-01-31"},
		Filters:        models.TransactionsFilters{Status: "APPROVED", Route: "cashin"},
	}

	got, err := newTestTransactionsV2Facade(t, srv).Count(context.Background(), txOrgID, txLedgerID, honoured)
	if err != nil {
		t.Fatalf("Count with only declared filters: %v", err)
	}

	if got != 7 {
		t.Fatalf("count = %d, want 7", got)
	}

	for _, want := range []string{"status=APPROVED", "route=cashin", "start_date=", "end_date="} {
		if !strings.Contains(query, want) {
			t.Fatalf("query = %q, want it to carry %q", query, want)
		}
	}
}

// TestTransactionsV2Facade_CreateBatch_PostsOneOrderedRequestUnderOneKey pins the
// atomic batch wire shape: one POST, items flat beside action and order, every
// leg scoped, one key for the whole batch, and the wrapped answer in order.
func TestTransactionsV2Facade_CreateBatch_PostsOneOrderedRequestUnderOneKey(t *testing.T) {
	var (
		requests     int
		method, path string
		key, ttl     string
		body         []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		method, path = r.Method, r.URL.Path
		key, ttl = r.Header.Get("X-Idempotency"), r.Header.Get("X-TTL")
		body, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"transactions":[
			{"id":"tx-direct","order":1,"amount":"100","status":{"code":"APPROVED"}},
			{"id":"tx-hold","order":2,"amount":"40","status":{"code":"PENDING"}}]}`))
	}))
	defer srv.Close()

	hold := sampleV2Input()
	hold.Amount, hold.Debits[0].Amount, hold.Credits[0].Amount = "40", "40", "40"

	ctx := sdkctx.WithIdempotencyTTL(sdkctx.WithIdempotencyKey(context.Background(), "cashin:E123"), 3600)

	got, err := newTestTransactionsV2Facade(t, srv).CreateBatch(ctx, txOrgID, txLedgerID, []models.TransactionBatchItemV2{
		{Action: models.TransactionBatchActionDirect, Order: 1, CreateTransactionV2Input: sampleV2Input()},
		{Action: models.TransactionBatchActionHold, Order: 2, CreateTransactionV2Input: hold},
	})
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	if requests != 1 || method != http.MethodPost || path != "/v2/transactions/batch" {
		t.Fatalf("requests=%d %s %s, want one POST /v2/transactions/batch", requests, method, path)
	}

	if key != "cashin:E123" || ttl != "3600" {
		t.Fatalf("X-Idempotency=%q X-TTL=%q, want the sdkctx key and TTL for the whole batch", key, ttl)
	}

	var wire struct {
		Transactions []struct {
			Action  string                    `json:"action"`
			Order   int                       `json:"order"`
			Amount  string                    `json:"amount"`
			Asset   string                    `json:"asset"`
			Debits  []models.TransactionV2Leg `json:"debits"`
			Credits []models.TransactionV2Leg `json:"credits"`
		} `json:"transactions"`
	}

	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, body)
	}

	want := []struct {
		action, amount string
		order          int
	}{{"direct", "100", 1}, {"hold", "40", 2}}

	if len(wire.Transactions) != len(want) {
		t.Fatalf("items on the wire = %d, want %d (%s)", len(wire.Transactions), len(want), body)
	}

	for i, w := range want {
		item := wire.Transactions[i]
		if item.Action != w.action || item.Order != w.order || item.Amount != w.amount || item.Asset != "USD" {
			t.Fatalf("item %d = %+v, want action %s order %d amount %s flat beside them", i, item, w.action, w.order, w.amount)
		}

		for _, leg := range append(item.Debits, item.Credits...) {
			if leg.OrganizationID != txOrgID || leg.LedgerID != txLedgerID {
				t.Fatalf("item %d leg %+v is not scoped to the addressed org and ledger", i, leg)
			}
		}
	}

	if len(got) != 2 || got[0].ID != "tx-direct" || got[1].ID != "tx-hold" || got[1].Status.Code != "PENDING" {
		t.Fatalf("decoded = %+v, want tx-direct then tx-hold in order", got)
	}
}

// TestTransactionsV2Facade_CreateBatch_RefusesABadItemLocallyNamingIt pins that a
// batch item is scoped and validated like a single create, and that the refusal
// names the item so a caller with fifty items knows which one to fix.
func TestTransactionsV2Facade_CreateBatch_RefusesABadItemLocallyNamingIt(t *testing.T) {
	invalid := sampleV2Input()
	invalid.Credits = nil

	ownKey := sampleV2Input()
	ownKey.IdempotencyKey = "cashin:E1"

	tests := []struct {
		name  string
		input *models.CreateTransactionV2Input
		want  string
	}{
		{"invalid payload", invalid, "transactions[1]"},
		{"no create body", nil, "transactions[1]"},
		{"item carries its own key", ownKey, "transactions[1]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reached bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached = true

				w.WriteHeader(http.StatusCreated)
			}))
			defer srv.Close()

			_, err := newTestTransactionsV2Facade(t, srv).CreateBatch(context.Background(), txOrgID, txLedgerID, []models.TransactionBatchItemV2{
				{Action: models.TransactionBatchActionDirect, Order: 1, CreateTransactionV2Input: sampleV2Input()},
				{Action: models.TransactionBatchActionHold, Order: 2, CreateTransactionV2Input: tt.input},
			})

			if !sdkerrors.IsValidationError(err) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want a validation error naming %s", err, tt.want)
			}

			if reached {
				t.Fatal("a locally refused batch must not reach the wire")
			}
		})
	}
}

// Cross-ledger 201 bodies, captured verbatim from a Midaz ledger on 2026-11-26
// for a direct and a hold create whose debit leg named one ledger and whose
// credit leg named another ledger of the same organization. The direct group
// carries one CREATED transaction per ledger; the hold group carries only the
// PENDING part the hold reserved.
const (
	crossLedgerDirectGroupJSON = `{"groupId":"01a2193f-9540-799f-8c22-3a988eef1aff","transactions":[{"id":"01a2193f-9542-7ebb-94b2-4193580ca9c3","groupId":"01a2193f-9540-799f-8c22-3a988eef1aff","description":"probe 9271","status":{"code":"CREATED","description":"CREATED"},"amount":"10","assetCode":"BRL","debit":["@alice"],"credit":["@external/BRL"],"ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","feesSkipped":false,"tracerSkipped":false,"createdAt":"2026-11-26T19:05:27.107013548Z","updatedAt":"2026-11-26T19:05:27.107013686Z","deletedAt":null,"operations":[{"id":"01a2193f-9566-7c10-b8d5-3db52aeecc29","transactionId":"01a2193f-9542-7ebb-94b2-4193580ca9c3","description":"probe 9271","type":"DEBIT","assetCode":"BRL","amount":{"value":"10"},"balance":{"available":"100","onHold":"0","version":1,"overdraftUsed":"0"},"balanceAfter":{"available":"90","onHold":"0","version":2,"overdraftUsed":"0"},"status":{"code":"","description":null},"accountId":"01a2193f-6e84-70ee-92e6-fe2ec819a38b","accountAlias":"@alice","balanceKey":"default","balanceId":"01a2193f-6e93-79a1-a512-80e5f0d1d841","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","balanceAffected":true,"direction":"debit","createdAt":"2026-11-26T19:05:27.107013548Z","updatedAt":"2026-11-26T19:05:27.107013839Z","deletedAt":null,"metadata":null},{"id":"01a2193f-9566-7c13-aa31-dcd5afed82a8","transactionId":"01a2193f-9542-7ebb-94b2-4193580ca9c3","description":"probe 9271","type":"CREDIT","assetCode":"BRL","amount":{"value":"10"},"balance":{"available":"100","onHold":"0","version":1,"overdraftUsed":"0"},"balanceAfter":{"available":"90","onHold":"0","version":2,"overdraftUsed":"0"},"status":{"code":"","description":null},"accountId":"01a2193f-6e34-7a8c-9235-fc2988f68047","accountAlias":"@external/BRL","balanceKey":"default","balanceId":"01a2193f-6e4a-73e4-b01b-768e60623f63","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","balanceAffected":true,"direction":"credit","createdAt":"2026-11-26T19:05:27.107013548Z","updatedAt":"2026-11-26T19:05:27.107013839Z","deletedAt":null,"metadata":null}],"order":1,"fees":{"transactionId":"01a2193f-9542-7ebb-94b2-4193580ca9c3","recorded":false,"available":true,"items":[]}},{"id":"01a2193f-9543-703d-8df0-70a1341b5fab","groupId":"01a2193f-9540-799f-8c22-3a988eef1aff","description":"probe 9271","status":{"code":"CREATED","description":"CREATED"},"amount":"10","assetCode":"BRL","debit":["@external/BRL"],"credit":["@bob"],"ledgerId":"01a2193f-4d26-721b-b604-464fa60c7e43","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","feesSkipped":false,"tracerSkipped":false,"createdAt":"2026-11-26T19:05:27.107066874Z","updatedAt":"2026-11-26T19:05:27.107066968Z","deletedAt":null,"operations":[{"id":"01a2193f-9566-7c2a-8a84-caba3300971f","transactionId":"01a2193f-9543-703d-8df0-70a1341b5fab","description":"probe 9271","type":"DEBIT","assetCode":"BRL","amount":{"value":"10"},"balance":{"available":"0","onHold":"0","version":0,"overdraftUsed":"0"},"balanceAfter":{"available":"10","onHold":"0","version":1,"overdraftUsed":"0"},"status":{"code":"","description":null},"accountId":"01a2193f-6e67-7d00-896e-5f34df0b065b","accountAlias":"@external/BRL","balanceKey":"default","balanceId":"01a2193f-6e6b-7e0e-9a77-f732a31ae9bb","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","ledgerId":"01a2193f-4d26-721b-b604-464fa60c7e43","balanceAffected":true,"direction":"debit","createdAt":"2026-11-26T19:05:27.107066874Z","updatedAt":"2026-11-26T19:05:27.107067063Z","deletedAt":null,"metadata":null},{"id":"01a2193f-9566-7c2e-a923-01a83bd89040","transactionId":"01a2193f-9543-703d-8df0-70a1341b5fab","description":"probe 9271","type":"CREDIT","assetCode":"BRL","amount":{"value":"10"},"balance":{"available":"0","onHold":"0","version":0,"overdraftUsed":"0"},"balanceAfter":{"available":"10","onHold":"0","version":1,"overdraftUsed":"0"},"status":{"code":"","description":null},"accountId":"01a2193f-6eab-7b0e-9920-f573ee27b927","accountAlias":"@bob","balanceKey":"default","balanceId":"01a2193f-6eb6-7226-8565-513257b19816","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","ledgerId":"01a2193f-4d26-721b-b604-464fa60c7e43","balanceAffected":true,"direction":"credit","createdAt":"2026-11-26T19:05:27.107066874Z","updatedAt":"2026-11-26T19:05:27.107067063Z","deletedAt":null,"metadata":null}],"order":2,"fees":{"transactionId":"01a2193f-9543-703d-8df0-70a1341b5fab","recorded":false,"available":true,"items":[]}}]}`

	crossLedgerHoldGroupJSON = `{"groupId":"01a2193f-95db-7225-ae05-ec34be8f8cec","transactions":[{"id":"01a2193f-95e3-7bd4-8e69-bfd96d929c35","groupId":"01a2193f-95db-7225-ae05-ec34be8f8cec","description":"probe 9271","status":{"code":"PENDING","description":"PENDING"},"amount":"7","assetCode":"BRL","debit":["@alice"],"credit":["@external/BRL"],"ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","feesSkipped":false,"tracerSkipped":false,"createdAt":"2026-11-26T19:05:27.267881135Z","updatedAt":"2026-11-26T19:05:27.267881568Z","deletedAt":null,"operations":[{"id":"01a2193f-95ec-781c-aed3-4763b49c91c5","transactionId":"01a2193f-95e3-7bd4-8e69-bfd96d929c35","description":"probe 9271","type":"ON_HOLD","assetCode":"BRL","amount":{"value":"7"},"balance":{"available":"90","onHold":"0","version":2,"overdraftUsed":"0"},"balanceAfter":{"available":"83","onHold":"7","version":3,"overdraftUsed":"0"},"status":{"code":"","description":null},"accountId":"01a2193f-6e84-70ee-92e6-fe2ec819a38b","accountAlias":"@alice","balanceKey":"default","balanceId":"01a2193f-6e93-79a1-a512-80e5f0d1d841","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","balanceAffected":true,"direction":"debit","createdAt":"2026-11-26T19:05:27.267881135Z","updatedAt":"2026-11-26T19:05:27.267881928Z","deletedAt":null,"metadata":null}],"order":1,"fees":{"transactionId":"01a2193f-95e3-7bd4-8e69-bfd96d929c35","recorded":false,"available":true,"items":[]}}]}`
)

// The organization and the two ledgers the captured groups were created in.
const (
	crossLedgerOrgID   = "01a2193f-4ce6-753d-8673-2712abdbfe5b"
	crossLedgerLedgerA = "01a2193f-4d0c-7ba5-bb53-42d79cf1701d"
	crossLedgerLedgerB = "01a2193f-4d26-721b-b604-464fa60c7e43"
)

// sampleCrossLedgerV2Input builds the request the captured groups answered: a
// debit of @alice in ledger A and a credit of @bob in ledger B, each leg naming
// its own pair. Every call returns a fresh input, so a test can build a second
// one to compare against after the call.
func sampleCrossLedgerV2Input() *models.CreateTransactionV2Input {
	return &models.CreateTransactionV2Input{
		Asset:       "BRL",
		Amount:      "10",
		Description: "cross-ledger transfer",
		Debits: []models.TransactionV2Leg{
			{Alias: "@alice", OrganizationID: crossLedgerOrgID, LedgerID: crossLedgerLedgerA, Amount: "10"},
		},
		Credits: []models.TransactionV2Leg{
			{Alias: "@bob", OrganizationID: crossLedgerOrgID, LedgerID: crossLedgerLedgerB, Amount: "10"},
		},
	}
}

// crossLedgerCreates lists the two cross-ledger creates with the endpoint each
// posts to and the single-ledger method its single-pair refusal points at, so
// every behaviour the two share is pinned on both.
var crossLedgerCreates = []struct {
	name   string
	path   string
	single string
	create func(*transactionsV2Facade, context.Context, *models.CreateTransactionV2Input) (*models.CrossLedgerTransactionGroupV2, error)
}{
	{
		name:   "CreateDirectCrossLedger",
		path:   "/v2/transactions/direct",
		single: "CreateDirect",
		create: (*transactionsV2Facade).CreateDirectCrossLedger,
	},
	{
		name:   "CreateHoldCrossLedger",
		path:   "/v2/transactions/hold",
		single: "CreateHold",
		create: (*transactionsV2Facade).CreateHoldCrossLedger,
	},
}

// crossLedgerRequest is what a cross-ledger create sent.
type crossLedgerRequest struct {
	requests     int
	method, path string
	body         []byte
}

// crossLedgerServer answers every request with status and response, recording
// the request it got.
func crossLedgerServer(t *testing.T, status int, contentType, response string) (*httptest.Server, *crossLedgerRequest) {
	t.Helper()

	got := &crossLedgerRequest{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.requests++
		got.method, got.path = r.Method, r.URL.Path
		got.body, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))

	t.Cleanup(srv.Close)

	return srv, got
}

// TestTransactionsV2Facade_CreateCrossLedgerReturnsTheServedGroup covers the
// scenarios criacao-direct-cross-ledger and criacao-hold-cross-ledger: one POST
// to the action's endpoint, and the group exactly as served. The hold answer has
// ONE transaction for two ledgers, and the SDK must not invent the second.
func TestTransactionsV2Facade_CreateCrossLedgerReturnsTheServedGroup(t *testing.T) {
	type wantTransaction struct {
		id, ledgerID, status string
	}

	tests := []struct {
		scenario string
		create   int
		response string
		groupID  string
		want     []wantTransaction
	}{
		{
			scenario: "criacao-direct-cross-ledger",
			create:   0,
			response: crossLedgerDirectGroupJSON,
			groupID:  "01a2193f-9540-799f-8c22-3a988eef1aff",
			want: []wantTransaction{
				{id: "01a2193f-9542-7ebb-94b2-4193580ca9c3", ledgerID: crossLedgerLedgerA, status: "CREATED"},
				{id: "01a2193f-9543-703d-8df0-70a1341b5fab", ledgerID: crossLedgerLedgerB, status: "CREATED"},
			},
		},
		{
			scenario: "criacao-hold-cross-ledger",
			create:   1,
			response: crossLedgerHoldGroupJSON,
			groupID:  "01a2193f-95db-7225-ae05-ec34be8f8cec",
			want: []wantTransaction{
				{id: "01a2193f-95e3-7bd4-8e69-bfd96d929c35", ledgerID: crossLedgerLedgerA, status: "PENDING"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			c := crossLedgerCreates[tt.create]
			srv, got := crossLedgerServer(t, http.StatusCreated, "application/json", tt.response)

			group, err := c.create(newTestTransactionsV2Facade(t, srv), context.Background(), sampleCrossLedgerV2Input())
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}

			if got.requests != 1 || got.method != http.MethodPost || got.path != c.path {
				t.Fatalf("requests=%d %s %s, want one POST %s", got.requests, got.method, got.path, c.path)
			}

			if group.GroupID != tt.groupID {
				t.Fatalf("GroupID = %q, want the served %q", group.GroupID, tt.groupID)
			}

			if len(group.Transactions) != len(tt.want) {
				t.Fatalf("transactions = %d, want the %d served", len(group.Transactions), len(tt.want))
			}

			for i, want := range tt.want {
				tx := group.Transactions[i]
				if tx.ID != want.id || tx.LedgerID != want.ledgerID || tx.Status.Code != want.status {
					t.Fatalf("transactions[%d] = {%s %s %s}, want {%s %s %s} in served order",
						i, tx.ID, tx.LedgerID, tx.Status.Code, want.id, want.ledgerID, want.status)
				}
			}
		})
	}
}

// TestTransactionsV2Facade_CrossLedgerSendsTheCallersLegs covers the scenarios
// corpo-cross-ledger-enviado and espacos-nos-identificadores-da-perna on both
// creates: the body is the /v2 create body with each leg's own pair, trimmed and
// never filled, and the caller's input is left as it was.
func TestTransactionsV2Facade_CrossLedgerSendsTheCallersLegs(t *testing.T) {
	// Written by hand from the input below, not produced by the SDK.
	const wantBody = `{"asset":"BRL","amount":"10",` +
		`"debits":[{"alias":"@alice","organizationId":"` + crossLedgerOrgID + `","ledgerId":"` + crossLedgerLedgerA + `","amount":"10"}],` +
		`"credits":[{"alias":"@bob","organizationId":"` + crossLedgerOrgID + `","ledgerId":"` + crossLedgerLedgerB + `","amount":"10"}],` +
		`"description":"cross-ledger transfer"}`

	tests := []struct {
		scenario string
		mut      func(*models.CreateTransactionV2Input)
	}{
		{
			scenario: "corpo-cross-ledger-enviado",
			mut:      func(*models.CreateTransactionV2Input) {},
		},
		{
			scenario: "espacos-nos-identificadores-da-perna",
			mut: func(in *models.CreateTransactionV2Input) {
				in.Debits[0].OrganizationID = " " + crossLedgerOrgID + " "
				in.Debits[0].LedgerID = "\t" + crossLedgerLedgerA
				in.Credits[0].LedgerID = crossLedgerLedgerB + "  "
			},
		},
	}

	for _, c := range crossLedgerCreates {
		for _, tt := range tests {
			t.Run(c.name+"/"+tt.scenario, func(t *testing.T) {
				srv, got := crossLedgerServer(t, http.StatusCreated, "application/json", crossLedgerDirectGroupJSON)

				input := sampleCrossLedgerV2Input()
				tt.mut(input)

				if _, err := c.create(newTestTransactionsV2Facade(t, srv), context.Background(), input); err != nil {
					t.Fatalf("%s: %v", c.name, err)
				}

				if string(got.body) != wantBody {
					t.Fatalf("body = %s\nwant   %s", got.body, wantBody)
				}

				want := sampleCrossLedgerV2Input()
				tt.mut(want)

				if !reflect.DeepEqual(input, want) {
					t.Fatalf("the caller's input changed: %+v, want %+v", input, want)
				}
			})
		}
	}
}

// TestTransactionsV2Facade_CrossLedgerRefusesLocally covers, on both creates, the
// scenarios pernas-em-mesmo-ledger-recusadas, mesmo-ledger-em-caixas-diferentes,
// perna-sem-escopo-recusada and input-nulo-ou-invalido-recusado: each is a
// validation error naming the operation, and nothing reaches the wire.
//
// A single-pair refusal also names the single-ledger method to use, because the
// fix is a different call, not a different body.
func TestTransactionsV2Facade_CrossLedgerRefusesLocally(t *testing.T) {
	sameLedger := func(mut func(*models.TransactionV2Leg)) func() *models.CreateTransactionV2Input {
		return func() *models.CreateTransactionV2Input {
			in := sampleCrossLedgerV2Input()
			in.Credits[0].LedgerID = crossLedgerLedgerA
			mut(&in.Credits[0])

			return in
		}
	}

	mutated := func(mut func(*models.CreateTransactionV2Input)) func() *models.CreateTransactionV2Input {
		return func() *models.CreateTransactionV2Input {
			in := sampleCrossLedgerV2Input()
			mut(in)

			return in
		}
	}

	tests := []struct {
		scenario   string
		input      func() *models.CreateTransactionV2Input
		want       string // "" when the refusal only has to name the operation
		singlePair bool   // the refusal must point at the single-ledger method
	}{
		{
			scenario:   "pernas-em-mesmo-ledger-recusadas",
			input:      sameLedger(func(*models.TransactionV2Leg) {}),
			singlePair: true,
		},
		{
			scenario: "pernas-em-mesmo-ledger-recusadas/spelled with surrounding spaces",
			input: sameLedger(func(leg *models.TransactionV2Leg) {
				leg.OrganizationID, leg.LedgerID = " "+leg.OrganizationID, leg.LedgerID+" "
			}),
			singlePair: true,
		},
		{
			scenario: "mesmo-ledger-em-caixas-diferentes",
			input: sameLedger(func(leg *models.TransactionV2Leg) {
				leg.OrganizationID, leg.LedgerID = strings.ToUpper(leg.OrganizationID), strings.ToUpper(leg.LedgerID)
			}),
			singlePair: true,
		},
		{
			scenario: "perna-sem-escopo-recusada/debit leg without organization",
			input:    mutated(func(in *models.CreateTransactionV2Input) { in.Debits[0].OrganizationID = "" }),
			want:     "debits[0]",
		},
		{
			scenario: "perna-sem-escopo-recusada/credit leg without ledger",
			input:    mutated(func(in *models.CreateTransactionV2Input) { in.Credits[0].LedgerID = "" }),
			want:     "credits[0]",
		},
		{
			scenario: "perna-sem-escopo-recusada/credit leg with a blank ledger",
			input:    mutated(func(in *models.CreateTransactionV2Input) { in.Credits[0].LedgerID = "   " }),
			want:     "credits[0]",
		},
		{
			scenario: "input-nulo-ou-invalido-recusado/nil input",
			input:    func() *models.CreateTransactionV2Input { return nil },
			want:     "input cannot be nil",
		},
		{
			scenario: "input-nulo-ou-invalido-recusado/no asset",
			input:    mutated(func(in *models.CreateTransactionV2Input) { in.Asset = "" }),
			want:     "asset",
		},
		{
			scenario: "input-nulo-ou-invalido-recusado/zero amount",
			input:    mutated(func(in *models.CreateTransactionV2Input) { in.Amount = "0" }),
			want:     "amount",
		},
		{
			scenario: "input-nulo-ou-invalido-recusado/negative amount",
			input:    mutated(func(in *models.CreateTransactionV2Input) { in.Amount = "-10" }),
			want:     "amount",
		},
		{
			scenario: "input-nulo-ou-invalido-recusado/no credit side",
			input:    mutated(func(in *models.CreateTransactionV2Input) { in.Credits = nil }),
			want:     "credits",
		},
	}

	for _, c := range crossLedgerCreates {
		for _, tt := range tests {
			t.Run(c.name+"/"+tt.scenario, func(t *testing.T) {
				srv, got := crossLedgerServer(t, http.StatusCreated, "application/json", crossLedgerDirectGroupJSON)

				group, err := c.create(newTestTransactionsV2Facade(t, srv), context.Background(), tt.input())

				if !sdkerrors.IsValidationError(err) || group != nil {
					t.Fatalf("got (%+v, %v), want a validation error and no group", group, err)
				}

				if operation := "V2.Transactions." + c.name; !strings.Contains(err.Error(), operation) {
					t.Fatalf("err = %v, want it to name %s", err, operation)
				}

				if tt.want != "" && !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("err = %v, want it to name %q", err, tt.want)
				}

				if pointer := "use " + c.single + " for"; tt.singlePair && !strings.Contains(err.Error(), pointer) {
					t.Fatalf("err = %v, want it to point at %s", err, c.single)
				}

				if got.requests != 0 {
					t.Fatalf("requests = %d, want none: a locally refused create must not reach the wire", got.requests)
				}
			})
		}
	}
}

// TestTransactionsV2Facade_CrossLedgerCountsDifferentOrganizationsAsDistinctPairs
// covers the scenario organizacoes-diferentes-contam-como-pares-distintos: legs
// in the same ledger id under two organizations are two pairs, so the request
// leaves and the server decides.
func TestTransactionsV2Facade_CrossLedgerCountsDifferentOrganizationsAsDistinctPairs(t *testing.T) {
	const otherOrgID = "5f607182-5555-4555-8555-5f6071829304"

	for _, c := range crossLedgerCreates {
		t.Run(c.name, func(t *testing.T) {
			srv, got := crossLedgerServer(t, http.StatusCreated, "application/json", crossLedgerDirectGroupJSON)

			input := sampleCrossLedgerV2Input()
			input.Credits[0].OrganizationID, input.Credits[0].LedgerID = otherOrgID, crossLedgerLedgerA

			if _, err := c.create(newTestTransactionsV2Facade(t, srv), context.Background(), input); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}

			if got.requests != 1 {
				t.Fatalf("requests = %d, want exactly one", got.requests)
			}
		})
	}
}

// crossLedgerSingleTransactionJSON is the 201 the same ledger served, on
// 2026-11-26, to a direct create whose legs named ONE ledger: a transaction, not
// a group. Kept verbatim, because its shape is what the group guard must refuse.
const crossLedgerSingleTransactionJSON = `{"id":"01a2193f-6ecc-7ae1-b06b-86721b2bf2e7","description":"","status":{"code":"CREATED","description":"CREATED"},"amount":"100","assetCode":"BRL","debit":["@external/BRL"],"credit":["@alice"],"ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","feesSkipped":false,"tracerSkipped":false,"createdAt":"2026-11-26T19:05:17.260713844Z","updatedAt":"2026-11-26T19:05:17.306428447Z","deletedAt":null,"operations":[{"id":"01a2193f-6f1d-7835-9890-78cd543e89c5","transactionId":"01a2193f-6ecc-7ae1-b06b-86721b2bf2e7","description":"","type":"DEBIT","assetCode":"BRL","amount":{"value":"100"},"balance":{"available":"0","onHold":"0","version":0,"overdraftUsed":"0"},"balanceAfter":{"available":"100","onHold":"0","version":1,"overdraftUsed":"0"},"status":{"code":"","description":null},"accountId":"01a2193f-6e34-7a8c-9235-fc2988f68047","accountAlias":"@external/BRL","balanceKey":"default","balanceId":"01a2193f-6e4a-73e4-b01b-768e60623f63","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","balanceAffected":true,"direction":"debit","createdAt":"2026-11-26T19:05:17.260713844Z","updatedAt":"2026-11-26T19:05:17.306428581Z","deletedAt":null,"metadata":null},{"id":"01a2193f-6f1d-7839-8a21-46965de97904","transactionId":"01a2193f-6ecc-7ae1-b06b-86721b2bf2e7","description":"","type":"CREDIT","assetCode":"BRL","amount":{"value":"100"},"balance":{"available":"0","onHold":"0","version":0,"overdraftUsed":"0"},"balanceAfter":{"available":"100","onHold":"0","version":1,"overdraftUsed":"0"},"status":{"code":"","description":null},"accountId":"01a2193f-6e84-70ee-92e6-fe2ec819a38b","accountAlias":"@alice","balanceKey":"default","balanceId":"01a2193f-6e93-79a1-a512-80e5f0d1d841","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","balanceAffected":true,"direction":"credit","createdAt":"2026-11-26T19:05:17.260713844Z","updatedAt":"2026-11-26T19:05:17.306428581Z","deletedAt":null,"metadata":null}],"fees":{"transactionId":"01a2193f-6ecc-7ae1-b06b-86721b2bf2e7","recorded":false,"available":true,"items":[]}}`

// TestTransactionsV2Facade_CrossLedgerRefusesAResponseThatIsNotAGroup covers the
// scenarios resposta-de-transacao-unica-recusada and grupo-sem-transacoes-recusado
// on both creates. Each body decodes cleanly into the group type, so only the
// group guard stands between it and a zero-valued group returned as success; the
// refusal is a decode error because the create may have taken effect.
func TestTransactionsV2Facade_CrossLedgerRefusesAResponseThatIsNotAGroup(t *testing.T) {
	const groupID = "01a2193f-9540-799f-8c22-3a988eef1aff"

	tests := []struct {
		scenario string
		body     string
	}{
		{scenario: "resposta-de-transacao-unica-recusada", body: crossLedgerSingleTransactionJSON},
		{scenario: "grupo-sem-transacoes-recusado/empty transactions", body: `{"groupId":"` + groupID + `","transactions":[]}`},
		{scenario: "grupo-sem-transacoes-recusado/absent transactions", body: `{"groupId":"` + groupID + `"}`},
		{scenario: "transactions without a groupId", body: `{"transactions":[{"id":"` + txID + `","status":{"code":"CREATED"}}]}`},
	}

	for _, c := range crossLedgerCreates {
		for _, tt := range tests {
			t.Run(c.name+"/"+tt.scenario, func(t *testing.T) {
				srv, got := crossLedgerServer(t, http.StatusCreated, "application/json", tt.body)

				group, err := c.create(newTestTransactionsV2Facade(t, srv), context.Background(), sampleCrossLedgerV2Input())

				if !sdkerrors.IsResponseDecodeError(err) || group != nil {
					t.Fatalf("got (%+v, %v), want a response-decode error and no group", group, err)
				}

				if !strings.Contains(err.Error(), "HTTP 201") {
					t.Fatalf("err = %v, want it to carry the status the server answered", err)
				}

				if got.requests != 1 {
					t.Fatalf("requests = %d, want exactly one", got.requests)
				}
			})
		}
	}
}

// Cross-ledger refusals, captured verbatim from the same ledger on 2026-11-26:
// 0249 answered a direct create across two ledgers with crossLedger.enabled off,
// and 0499 answered a block create across two ledgers, the refusal a server
// that takes legs in one pair only gives every action.
const (
	crossLedgerNotEnabledJSON = `{"type":"https://errors.lerian.studio/v1/0249","title":"Cross-Ledger Not Enabled Error","status":422,"detail":"Ledger 01a2193f-4d0c-7ba5-bb53-42d79cf1701d is not enabled for cross-ledger transactions. Enable crossLedger.enabled on every ledger involved.","code":"0249","entityType":"Ledger"}`

	crossLedgerScopeMismatchJSON = `{"type":"https://errors.lerian.studio/v1/0499","title":"Transaction Scope Mismatch","status":422,"detail":"Every leg of the transaction must name the same organization and ledger. Please review your input so that all legs share one scope and try again.","code":"0499","entityType":"Transaction"}`
)

// TestTransactionsV2Facade_CrossLedgerSurfacesServerRefusals covers the scenarios
// servidor-recusa-cross-ledger-desligado and servidor-recusa-escopos-diferentes
// on both creates: the server's refusal comes back as the facade's API error
// with its status and code, and no group.
func TestTransactionsV2Facade_CrossLedgerSurfacesServerRefusals(t *testing.T) {
	tests := []struct {
		scenario string
		body     string
		code     string
	}{
		{scenario: "servidor-recusa-cross-ledger-desligado", body: crossLedgerNotEnabledJSON, code: "0249"},
		{scenario: "servidor-recusa-escopos-diferentes", body: crossLedgerScopeMismatchJSON, code: "0499"},
	}

	for _, c := range crossLedgerCreates {
		for _, tt := range tests {
			t.Run(c.name+"/"+tt.scenario, func(t *testing.T) {
				srv, got := crossLedgerServer(t, http.StatusUnprocessableEntity, "application/problem+json", tt.body)

				group, err := c.create(newTestTransactionsV2Facade(t, srv), context.Background(), sampleCrossLedgerV2Input())
				if group != nil {
					t.Fatalf("group = %+v, want none on a refusal", group)
				}

				var sdkErr *sdkerrors.Error
				if !errors.As(err, &sdkErr) {
					t.Fatalf("error type = %T (%v), want *errors.Error", err, err)
				}

				if sdkErr.StatusCode != http.StatusUnprocessableEntity || sdkErr.APICode != tt.code {
					t.Fatalf("decoded error = %+v, want status 422 and code %s", sdkErr, tt.code)
				}

				if got.requests != 1 {
					t.Fatalf("requests = %d, want exactly one: a 422 is not retried", got.requests)
				}
			})
		}
	}
}

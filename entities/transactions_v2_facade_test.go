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
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry"
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
	hold.IdempotencyKey = "ignored-inside-a-batch"

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

// TestTransactionsV2Facade_CreateBatch_MapsRefusalsAfterARetryUnderTheSameKey pins
// CreateDirect's error mapping, and that a retry re-sends the batch key and fires
// the retry.WithAttemptHook a caller reads to mark the failure as possibly applied.
func TestTransactionsV2Facade_CreateBatch_MapsRefusalsAfterARetryUnderTheSameKey(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		is     func(error) bool
	}{
		{"0018 insufficient funds", http.StatusUnprocessableEntity, `{"code":"0018","title":"Insufficient Funds","status":422}`, sdkerrors.IsInsufficientBalanceError},
		{"0084 batch key in use", http.StatusConflict, `{"code":"0084","title":"Idempotency Conflict","status":409}`, sdkerrors.IsIdempotencyError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var keys []string

			srv := unavailableThenRefusingServer(t, &keys, tt.status, tt.body)

			planes, err := newPlaneClients(planeClientsConfig{
				ledgerURL:    srv.URL,
				tracerURL:    srv.URL + "/v1",
				auth:         authRoundTripperConfig{tokenProvider: func(context.Context) (string, error) { return "tok", nil }},
				httpClient:   srv.Client(),
				retryOptions: retryEnabledOptions(1),
			})
			if err != nil {
				t.Fatalf("newPlaneClients: %v", err)
			}

			var retried bool

			ctx := retry.WithAttemptHook(sdkctx.WithIdempotencyKey(context.Background(), "cashin:E123"),
				func(context.Context, int, error, time.Duration) { retried = true })

			_, err = newTransactionsV2Facade(planes.Ledger, true).CreateBatch(ctx, txOrgID, txLedgerID,
				[]models.TransactionBatchItemV2{{Action: models.TransactionBatchActionDirect, Order: 1, CreateTransactionV2Input: sampleV2Input()}})

			if !tt.is(err) {
				t.Fatalf("err = %v, want it mapped like CreateDirect maps %s", err, tt.name)
			}

			if !retried || len(keys) != 2 || keys[0] != "cashin:E123" || keys[1] != keys[0] {
				t.Fatalf("retried=%v keys=%v, want one retry re-sending the batch key", retried, keys)
			}
		})
	}
}

// unavailableThenRefusingServer answers 503 once, then status/body, recording
// the X-Idempotency of every request.
func unavailableThenRefusingServer(t *testing.T, keys *[]string, status int, body string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*keys = append(*keys, r.Header.Get("X-Idempotency"))

		w.Header().Set("Content-Type", "application/problem+json")

		if len(*keys) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"title":"Service Unavailable","status":503}`))

			return
		}

		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(srv.Close)

	return srv
}

// TestTransactionsV2Facade_CreateBatch_RefusesABadItemLocallyNamingIt pins that a
// batch item is scoped and validated like a single create, and that the refusal
// names the item so a caller with fifty items knows which one to fix.
func TestTransactionsV2Facade_CreateBatch_RefusesABadItemLocallyNamingIt(t *testing.T) {
	foreign := sampleV2Input()
	foreign.Debits[0].LedgerID = "other-ledger"

	invalid := sampleV2Input()
	invalid.Credits = nil

	tests := []struct {
		name  string
		input *models.CreateTransactionV2Input
		want  string
	}{
		{"leg names another ledger", foreign, "transactions[1]"},
		{"invalid payload", invalid, "transactions[1]"},
		{"no create body", nil, "transactions[1]"},
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

// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
)

// Real UUIDs: the /json and /inflow etc. responses decode into
// models.Transaction, but the org/ledger path segments and the response id are
// exercised end-to-end, so keep them valid.
const (
	txOrgID    = "11111111-1111-1111-1111-111111111111"
	txLedgerID = "22222222-2222-2222-2222-222222222222"
	txID       = "33333333-3333-3333-3333-333333333333"
)

func txBase() string {
	return "/v1/organizations/" + txOrgID + "/ledgers/" + txLedgerID + "/transactions"
}

func newTestTransactionsFacade(t *testing.T, srv *httptest.Server) *transactionsFacade {
	t.Helper()
	return &transactionsFacade{ledger: newTestLedgerClient(t, srv)}
}

// txResponseBody is a 200 create response carrying the two skip flags the SDK
// model must now surface, plus a real id.
func txResponseBody() string {
	return `{"id":"` + txID + `","assetCode":"USD","amount":"100","status":{"code":"APPROVED"},"feesSkipped":true,"tracerSkipped":true}`
}

func sampleTransactionInput() *models.CreateTransactionInput {
	return models.NewCreateTransactionInput("USD", "100").WithSend(&models.SendInput{
		Asset: "USD",
		Value: "100",
		Source: &models.SourceInput{From: []models.FromToInput{
			{Account: "@src", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
		}},
		Distribute: &models.DistributeInput{To: []models.FromToInput{
			{Account: "@dst", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
		}},
	})
}

// TestTransactionsFacade_Create exercises each of the four create paths: correct
// method + path + content type, the endpoint-specific wire body, the
// X-Idempotency header stamped from the params, and the skip flags decoded into
// the model.
func TestTransactionsFacade_Create(t *testing.T) {
	tests := []struct {
		name    string
		suffix  string
		call    func(f *transactionsFacade, ctx context.Context) (*models.Transaction, error)
		hasSend bool // whether a send envelope is expected in the wire body
		hasSrc  bool
		hasDst  bool
	}{
		{
			name:   "json",
			suffix: "/json",
			call: func(f *transactionsFacade, ctx context.Context) (*models.Transaction, error) {
				return f.CreateJSON(ctx, txOrgID, txLedgerID, sampleTransactionInput())
			},
			hasSend: true,
			hasSrc:  true,
			hasDst:  true,
		},
		{
			name:   "inflow",
			suffix: "/inflow",
			call: func(f *transactionsFacade, ctx context.Context) (*models.Transaction, error) {
				return f.CreateInflow(ctx, txOrgID, txLedgerID, models.NewCreateInflowInput("USD", "100",
					&models.DistributeInput{To: []models.FromToInput{
						{Account: "@dst", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
					}}))
			},
			hasSend: true,
			hasDst:  true,
		},
		{
			name:   "outflow",
			suffix: "/outflow",
			call: func(f *transactionsFacade, ctx context.Context) (*models.Transaction, error) {
				return f.CreateOutflow(ctx, txOrgID, txLedgerID, models.NewCreateOutflowInput("USD", "100",
					&models.SourceInput{From: []models.FromToInput{
						{Account: "@src", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
					}}))
			},
			hasSend: true,
			hasSrc:  true,
		},
		{
			// Annotations are metadata-only: no send envelope, no balance impact.
			name:   "annotation",
			suffix: "/annotation",
			call: func(f *transactionsFacade, ctx context.Context) (*models.Transaction, error) {
				return f.CreateAnnotation(ctx, txOrgID, txLedgerID, models.NewCreateAnnotationInput("audit note"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				gotMethod, gotPath, gotCT, gotIdem string
				gotBody                            []byte
			)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				gotCT = r.Header.Get("Content-Type")
				gotIdem = r.Header.Get("X-Idempotency")
				gotBody, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK) // creates succeed with 200, not 201
				_, _ = w.Write([]byte(txResponseBody()))
			}))
			defer srv.Close()

			tx, err := tt.call(newTestTransactionsFacade(t, srv), context.Background())
			if err != nil {
				t.Fatalf("create %s: %v", tt.name, err)
			}

			if gotMethod != http.MethodPost || gotPath != txBase()+tt.suffix {
				t.Fatalf("req = %s %s, want POST %s", gotMethod, gotPath, txBase()+tt.suffix)
			}
			if gotCT != jsonContentType {
				t.Fatalf("Content-Type = %q, want %q", gotCT, jsonContentType)
			}
			// autoGen=true: every create must leave with an idempotency key.
			if gotIdem == "" {
				t.Fatal("X-Idempotency header missing on create (money-path: unsafe writes must be idempotent)")
			}

			// Wire shape: the body is the mapper output (send envelope), not a
			// flat json.Marshal(input). Assert the source/distribute structure
			// each endpoint owns.
			var wire map[string]any
			if err := json.Unmarshal(gotBody, &wire); err != nil {
				t.Fatalf("body not JSON object: %v (%s)", err, gotBody)
			}
			send, hasSend := wire["send"].(map[string]any)
			if hasSend != tt.hasSend {
				t.Fatalf("send envelope present = %v, want %v: %s", hasSend, tt.hasSend, gotBody)
			}
			if tt.hasSend {
				if _, ok := send["source"]; ok != tt.hasSrc {
					t.Fatalf("send.source present = %v, want %v: %s", ok, tt.hasSrc, gotBody)
				}
				if _, ok := send["distribute"]; ok != tt.hasDst {
					t.Fatalf("send.distribute present = %v, want %v: %s", ok, tt.hasDst, gotBody)
				}
			}

			// Skip flags the SDK model previously dropped must decode.
			if !tx.FeesSkipped || !tx.TracerSkipped {
				t.Fatalf("skip flags not decoded: FeesSkipped=%v TracerSkipped=%v", tx.FeesSkipped, tx.TracerSkipped)
			}
			if tx.ID != txID {
				t.Fatalf("tx.ID = %q, want %q", tx.ID, txID)
			}
		})
	}
}

// TestTransactionsFacade_CreateExplicitKey proves the caller-supplied key on the
// input struct wins and reaches the wire (CreateJSON is the only path whose
// input carries IdempotencyKey).
func TestTransactionsFacade_CreateExplicitKey(t *testing.T) {
	const key = "payment-inv123"
	var gotIdem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIdem = r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(txResponseBody()))
	}))
	defer srv.Close()

	input := sampleTransactionInput()
	input.IdempotencyKey = key

	if _, err := newTestTransactionsFacade(t, srv).CreateJSON(context.Background(), txOrgID, txLedgerID, input); err != nil {
		t.Fatalf("CreateJSON: %v", err)
	}
	if gotIdem != key {
		t.Fatalf("X-Idempotency = %q, want explicit %q", gotIdem, key)
	}
}

// TestTransactionsFacade_CreateTTL proves the ctx TTL knob reaches X-TTL.
func TestTransactionsFacade_CreateTTL(t *testing.T) {
	var gotTTL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTTL = r.Header.Get("X-TTL")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(txResponseBody()))
	}))
	defer srv.Close()

	ctx := sdkctx.WithIdempotencyTTL(context.Background(), 600)
	if _, err := newTestTransactionsFacade(t, srv).CreateJSON(ctx, txOrgID, txLedgerID, sampleTransactionInput()); err != nil {
		t.Fatalf("CreateJSON: %v", err)
	}
	if gotTTL != "600" {
		t.Fatalf("X-TTL = %q, want %q", gotTTL, "600")
	}
}

// TestTransactionsFacade_CreateReplaySafe drives the 401 -> refresh -> replay
// path through the real auth round tripper: a create must reach the server
// twice (original + one replay), carry the SAME idempotency key both times, and
// replay an identical body — otherwise a refresh would post a second balance
// mutation under a different key.
func TestTransactionsFacade_CreateReplaySafe(t *testing.T) {
	var (
		attempts atomic.Int32
		keys     []string
		bodies   []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		b, _ := io.ReadAll(r.Body)
		keys = append(keys, r.Header.Get("X-Idempotency"))
		bodies = append(bodies, string(b))
		if n == 1 {
			w.WriteHeader(http.StatusUnauthorized) // stale token
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(txResponseBody()))
	}))
	defer srv.Close()

	tx, err := newTestTransactionsFacade(t, srv).CreateJSON(context.Background(), txOrgID, txLedgerID, sampleTransactionInput())
	if err != nil {
		t.Fatalf("CreateJSON: %v", err)
	}
	if tx.ID != txID {
		t.Fatalf("tx.ID = %q, want %q", tx.ID, txID)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2 (original + one replay)", got)
	}
	if len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("X-Idempotency across attempts = %v, want both equal and non-empty (money-path: stable key across replay)", keys)
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] {
		t.Fatalf("bodies across attempts = %v, want identical replay", bodies)
	}
}

// TestTransactionsFacade_CreateError maps a non-2xx into the unified RFC 9457
// error rather than decoding a transaction.
func TestTransactionsFacade_CreateError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0490","title":"Insufficient funds","status":422}`))
	}))
	defer srv.Close()

	tx, err := newTestTransactionsFacade(t, srv).CreateJSON(context.Background(), txOrgID, txLedgerID, sampleTransactionInput())
	if err == nil {
		t.Fatalf("expected error, got tx %+v", tx)
	}
}

// TestTransactionsFacade_CreateValidation rejects an invalid input before any
// request leaves the process.
func TestTransactionsFacade_CreateValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("server must not be hit on invalid input")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Missing send -> Validate fails.
	if _, err := newTestTransactionsFacade(t, srv).CreateJSON(context.Background(), txOrgID, txLedgerID, &models.CreateTransactionInput{}); err == nil {
		t.Fatal("expected validation error for empty input")
	}
}

// TestTransactionsFacade_Lifecycle exercises commit/cancel/revert: correct
// method (POST) + path (.../{id}/{action}), success on 201, and the decoded
// transaction returned to the caller.
func TestTransactionsFacade_Lifecycle(t *testing.T) {
	tests := []struct {
		name   string
		action string
		call   func(f *transactionsFacade, ctx context.Context) (*models.Transaction, error)
	}{
		{
			name:   "commit",
			action: "/commit",
			call: func(f *transactionsFacade, ctx context.Context) (*models.Transaction, error) {
				return f.Commit(ctx, txOrgID, txLedgerID, txID)
			},
		},
		{
			name:   "cancel",
			action: "/cancel",
			call: func(f *transactionsFacade, ctx context.Context) (*models.Transaction, error) {
				return f.Cancel(ctx, txOrgID, txLedgerID, txID)
			},
		},
		{
			name:   "revert",
			action: "/revert",
			call: func(f *transactionsFacade, ctx context.Context) (*models.Transaction, error) {
				return f.Revert(ctx, txOrgID, txLedgerID, txID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath, gotIdem string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				gotIdem = r.Header.Get("X-Idempotency")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated) // lifecycle actions succeed with 201
				_, _ = w.Write([]byte(txResponseBody()))
			}))
			defer srv.Close()

			tx, err := tt.call(newTestTransactionsFacade(t, srv), context.Background())
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}

			if gotMethod != http.MethodPost || gotPath != txBase()+"/"+txID+tt.action {
				t.Fatalf("req = %s %s, want POST %s", gotMethod, gotPath, txBase()+"/"+txID+tt.action)
			}
			// autoGen=false: no key unless the caller supplies one.
			if gotIdem != "" {
				t.Fatalf("X-Idempotency = %q, want empty (actions are not auto-idempotent)", gotIdem)
			}
			if tx.ID != txID {
				t.Fatalf("tx.ID = %q, want %q", tx.ID, txID)
			}
		})
	}
}

// TestTransactionsFacade_RevertReturnsChild proves revert returns the child
// (reversal) transaction with ParentTransactionID pointing at the original,
// without mutating the original.
func TestTransactionsFacade_RevertReturnsChild(t *testing.T) {
	const childID = "44444444-4444-4444-4444-444444444444"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"` + childID + `","parentTransactionId":"` + txID + `","status":{"code":"APPROVED"}}`))
	}))
	defer srv.Close()

	tx, err := newTestTransactionsFacade(t, srv).Revert(context.Background(), txOrgID, txLedgerID, txID)
	if err != nil {
		t.Fatalf("Revert: %v", err)
	}
	if tx.ID != childID {
		t.Fatalf("tx.ID = %q, want child %q", tx.ID, childID)
	}
	if tx.ParentTransactionID != txID {
		t.Fatalf("tx.ParentTransactionID = %q, want original %q", tx.ParentTransactionID, txID)
	}
}

// TestTransactionsFacade_CancelEmptyBody proves cancel synthesizes a CANCELED
// transaction when the 201 carries an empty (or "null") body, rather than
// failing the decode.
func TestTransactionsFacade_CancelEmptyBody(t *testing.T) {
	for _, body := range []string{"", "null"} {
		t.Run("body="+body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			tx, err := newTestTransactionsFacade(t, srv).Cancel(context.Background(), txOrgID, txLedgerID, txID)
			if err != nil {
				t.Fatalf("Cancel: %v", err)
			}
			if tx.ID != txID {
				t.Fatalf("tx.ID = %q, want %q", tx.ID, txID)
			}
			if tx.Status.Code != string(models.TransactionStatusCanceled) {
				t.Fatalf("tx.Status.Code = %q, want %q", tx.Status.Code, models.TransactionStatusCanceled)
			}
		})
	}
}

// TestTransactionsFacade_LifecycleIdempotencyKey proves a caller-supplied ctx
// key reaches the wire as X-Idempotency on an action (autoGen=false path).
func TestTransactionsFacade_LifecycleIdempotencyKey(t *testing.T) {
	const key = "commit-once-abc"
	var gotIdem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIdem = r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(txResponseBody()))
	}))
	defer srv.Close()

	ctx := sdkctx.WithIdempotencyKey(context.Background(), key)
	if _, err := newTestTransactionsFacade(t, srv).Commit(ctx, txOrgID, txLedgerID, txID); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if gotIdem != key {
		t.Fatalf("X-Idempotency = %q, want ctx key %q", gotIdem, key)
	}
}

// TestTransactionsFacade_LifecycleError maps a non-2xx into the unified error.
func TestTransactionsFacade_LifecycleError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0088","title":"Transaction not pending","status":409}`))
	}))
	defer srv.Close()

	if _, err := newTestTransactionsFacade(t, srv).Commit(context.Background(), txOrgID, txLedgerID, txID); err == nil {
		t.Fatal("expected error on 409 commit")
	}
}

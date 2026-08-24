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
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/sdkctx"
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
	return newTransactionsFacade(newTestLedgerClient(t, srv), true)
}

// txResponseBody is a 200 create response with a real id.
func txResponseBody() string {
	return `{"id":"` + txID + `","assetCode":"USD","amount":"100","status":{"code":"APPROVED"}}`
}

func sampleTransactionInput() *models.CreateTransactionInput {
	return models.NewCreateTransactionInput("USD", "100").WithSend(&models.SendInput{
		Asset: "USD",
		Value: "100",
		Source: &models.SourceInput{From: []models.FromToInput{
			{AccountAlias: "@src", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
		}},
		Distribute: &models.DistributeInput{To: []models.FromToInput{
			{AccountAlias: "@dst", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
		}},
	})
}

// TestTransactionsFacade_Create exercises each of the four create paths: correct
// method + path + content type, the endpoint-specific wire body, the
// X-Idempotency header stamped from the params, and the skip flags decoded into
// the model.
//
//nolint:revive // cognitive-complexity: the four create paths (json/inflow/outflow/annotation) as subtests, each with its own httptest server closure and assertions; the complexity is the subtest count, not branching logic. Matches the repo's per-test convention.
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
						{AccountAlias: "@dst", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
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
						{AccountAlias: "@src", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
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
				// Money assert: the amount must reach the wire intact — a
				// regression that zeroed send.value must fail here.
				if send["value"] != "100" {
					t.Fatalf("send.value = %v, want %q: %s", send["value"], "100", gotBody)
				}
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
// error rather than decoding a transaction, threading APICode/StatusCode/
// RequestID so a regression that masked a 422 as an internal error fails here.
func TestTransactionsFacade_CreateError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-tx-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0490","title":"Insufficient funds","status":422}`))
	}))
	defer srv.Close()

	tx, err := newTestTransactionsFacade(t, srv).CreateJSON(context.Background(), txOrgID, txLedgerID, sampleTransactionInput())
	if err == nil {
		t.Fatalf("expected error, got tx %+v", tx)
	}

	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0490" || sdkErr.StatusCode != http.StatusUnprocessableEntity || sdkErr.RequestID != "req-tx-422" {
		t.Fatalf("decoded error = %+v, want APICode=LEDGER-0490 StatusCode=422 RequestID=req-tx-422", sdkErr)
	}
}

// txValueResponseBody echoes the send.value the caller posted, plus a real id
// and an object status, so a create test can assert the monetary amount survived
// the round trip.
func txValueResponseBody(value string) string {
	return `{"id":"` + txID + `","assetCode":"USD","amount":"` + value + `","status":{"code":"APPROVED"}}`
}

// TestTransactionsFacade_CreateOffStatusSucceeds is the money-path RED: a create
// CONFIRMED by the server with a 2xx that the OAS did not declare (async 202, or
// a 201 divergence) carries a real Transaction body whose status is an OBJECT.
// The generated status-exact parser would try to unmarshal that object into
// Error.status (*int64) and fail, turning a confirmed write into a spurious
// internal error. Any 2xx must decode as success.
//
//nolint:revive // cognitive-complexity: several off-status create subtests, each with its own httptest server closure and assertions; the complexity is the subtest count, not branching logic. Matches the repo's per-test convention.
func TestTransactionsFacade_CreateOffStatusSucceeds(t *testing.T) {
	for _, status := range []int{http.StatusCreated, http.StatusAccepted} {
		t.Run("status="+strconv.Itoa(status), func(t *testing.T) {
			var gotBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				// Server echoes an amount distinct from the request's send.value so
				// the response assert below proves the decoded body carried it, not
				// the request value.
				_, _ = w.Write([]byte(txValueResponseBody("250")))
			}))
			defer srv.Close()

			tx, err := newTestTransactionsFacade(t, srv).CreateJSON(context.Background(), txOrgID, txLedgerID, sampleTransactionInput())
			if err != nil {
				t.Fatalf("CreateJSON on %d: %v", status, err)
			}
			if tx.ID != txID {
				t.Fatalf("tx.ID = %q, want %q", tx.ID, txID)
			}
			if tx.Status.Code != "APPROVED" {
				t.Fatalf("tx.Status.Code = %q, want APPROVED", tx.Status.Code)
			}
			// Response money assert: the amount the server returned must survive
			// the raw-body decode into models.Transaction.
			if tx.Amount != "250" {
				t.Fatalf("tx.Amount = %q, want %q (response amount must survive decode)", tx.Amount, "250")
			}

			// Money assert: the posted send.value must be the amount we sent.
			var wire map[string]any
			if err := json.Unmarshal(gotBody, &wire); err != nil {
				t.Fatalf("body not JSON object: %v (%s)", err, gotBody)
			}
			send, _ := wire["send"].(map[string]any)
			if send["value"] != "100" {
				t.Fatalf("send.value = %v, want %q: %s", send["value"], "100", gotBody)
			}
		})
	}
}

// txBodyOfSize builds a valid transaction JSON of exactly size bytes by padding
// a description field, so a test can pin the response-body cap boundary (accept
// == cap, reject > cap) without depending on the production cap value or reading
// an unbounded body into the test.
func txBodyOfSize(t *testing.T, size int) string {
	t.Helper()

	const (
		prefix = `{"id":"` + txID + `","status":{"code":"APPROVED"},"description":"`
		suffix = `"}`
	)

	pad := size - len(prefix) - len(suffix)
	if pad < 0 {
		t.Fatalf("size %d below minimum envelope %d", size, len(prefix)+len(suffix))
	}

	return prefix + strings.Repeat("A", pad) + suffix
}

// TestTransactionsFacade_CreateResponseBodyCap proves the money-path write drain
// (readRawResponse) bounds the response read: a hostile/broken server returning
// a 2xx body larger than maxHTTPResponseBodyBytes is rejected rather than read
// into unbounded memory (the legacy path caps at http_retry_response.go:186; the
// plane write path must match), while a body exactly at the cap still decodes.
func TestTransactionsFacade_CreateResponseBodyCap(t *testing.T) {
	writeBody := func(body string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, body)
		}))
	}

	t.Run("over-cap rejected", func(t *testing.T) {
		srv := writeBody(txBodyOfSize(t, int(maxHTTPResponseBodyBytes)+1))
		defer srv.Close()

		tx, err := newTestTransactionsFacade(t, srv).CreateJSON(context.Background(), txOrgID, txLedgerID, sampleTransactionInput())
		if err == nil {
			t.Fatalf("over-cap response accepted (memory-exhaustion risk); got tx id %q", tx.ID)
		}
		if !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("error = %v, want over-limit rejection mentioning 'exceeds'", err)
		}
	})

	t.Run("at-cap decodes", func(t *testing.T) {
		srv := writeBody(txBodyOfSize(t, int(maxHTTPResponseBodyBytes)))
		defer srv.Close()

		tx, err := newTestTransactionsFacade(t, srv).CreateJSON(context.Background(), txOrgID, txLedgerID, sampleTransactionInput())
		if err != nil {
			t.Fatalf("at-cap response rejected: %v", err)
		}
		if tx.ID != txID {
			t.Fatalf("tx.ID = %q, want %q", tx.ID, txID)
		}
	})
}

// TestTransactionsFacade_CommitOffStatusSucceeds proves a commit CONFIRMED by
// the server with a 200 (instead of the OAS-declared 201) carrying a real
// Transaction body decodes as success, not a spurious internal error.
func TestTransactionsFacade_CommitOffStatusSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // server returns 200, OAS declares 201
		_, _ = w.Write([]byte(txResponseBody()))
	}))
	defer srv.Close()

	tx, err := newTestTransactionsFacade(t, srv).Commit(context.Background(), txOrgID, txLedgerID, txID)
	if err != nil {
		t.Fatalf("Commit on 200: %v", err)
	}
	if tx.ID != txID || tx.Status.Code != "APPROVED" {
		t.Fatalf("tx = %+v, want id %q status APPROVED", tx, txID)
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

// TestTransactionsFacade_LifecycleError maps a non-2xx into the unified error,
// threading APICode/StatusCode/RequestID (a regression masking the 409 as an
// internal error must fail here).
func TestTransactionsFacade_LifecycleError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-commit-409")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0088","title":"Transaction not pending","status":409}`))
	}))
	defer srv.Close()

	_, err := newTestTransactionsFacade(t, srv).Commit(context.Background(), txOrgID, txLedgerID, txID)
	if err == nil {
		t.Fatal("expected error on 409 commit")
	}

	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0088" || sdkErr.StatusCode != http.StatusConflict || sdkErr.RequestID != "req-commit-409" {
		t.Fatalf("decoded error = %+v, want APICode=LEDGER-0088 StatusCode=409 RequestID=req-commit-409", sdkErr)
	}
}

// TestTransactionsFacade_UpdateTransaction proves an update sends PATCH to
// .../transactions/{id} with a plain application/json body carrying the WHOLE
// input object (metadata + description) — parity with the legacy PATCH, NOT a
// merge-patch content type — and decodes the 200 response into the public model.
func TestTransactionsFacade_UpdateTransaction(t *testing.T) {
	var (
		gotMethod, gotPath, gotCT string
		gotBody                   []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotCT = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(txResponseBody()))
	}))
	defer srv.Close()

	input := models.NewUpdateTransactionInput().
		WithDescription("adjusted").
		WithMetadata(map[string]any{"reviewed": "yes"})

	tx, err := newTestTransactionsFacade(t, srv).UpdateTransaction(context.Background(), txOrgID, txLedgerID, txID, input)
	if err != nil {
		t.Fatalf("UpdateTransaction: %v", err)
	}

	if gotMethod != http.MethodPatch || gotPath != txBase()+"/"+txID {
		t.Fatalf("req = %s %s, want PATCH %s", gotMethod, gotPath, txBase()+"/"+txID)
	}
	if gotCT != jsonContentType {
		t.Fatalf("Content-Type = %q, want %q (parity: plain JSON, not merge-patch)", gotCT, jsonContentType)
	}

	var wire map[string]any
	if err := json.Unmarshal(gotBody, &wire); err != nil {
		t.Fatalf("body not JSON object: %v (%s)", err, gotBody)
	}
	if wire["description"] != "adjusted" {
		t.Fatalf("body.description = %v, want %q: %s", wire["description"], "adjusted", gotBody)
	}
	if _, ok := wire["metadata"].(map[string]any); !ok {
		t.Fatalf("body.metadata missing/not object: %s", gotBody)
	}
	if tx.ID != txID {
		t.Fatalf("tx.ID = %q, want %q", tx.ID, txID)
	}
}

// TestTransactionsFacade_UpdateTransactionEmptyRejected proves an empty update
// payload is rejected before any request leaves the process (parity: legacy
// requires a non-empty change set).
func TestTransactionsFacade_UpdateTransactionEmptyRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("server must not be hit on empty update payload")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if _, err := newTestTransactionsFacade(t, srv).UpdateTransaction(context.Background(), txOrgID, txLedgerID, txID, models.NewUpdateTransactionInput()); err == nil {
		t.Fatal("expected validation error for empty update payload")
	}
}

// TestTransactionsFacade_UpdateTransactionError maps a non-2xx into the unified
// error rather than decoding a transaction, threading APICode/StatusCode/
// RequestID.
func TestTransactionsFacade_UpdateTransactionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-upd-404")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0007","title":"Transaction not found","status":404}`))
	}))
	defer srv.Close()

	input := models.NewUpdateTransactionInput().WithDescription("x")
	_, err := newTestTransactionsFacade(t, srv).UpdateTransaction(context.Background(), txOrgID, txLedgerID, txID, input)
	if err == nil {
		t.Fatal("expected error on 404 update")
	}

	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0007" || sdkErr.StatusCode != http.StatusNotFound || sdkErr.RequestID != "req-upd-404" {
		t.Fatalf("decoded error = %+v, want APICode=LEDGER-0007 StatusCode=404 RequestID=req-upd-404", sdkErr)
	}
}

// TestTransactionsFacade_UpdateOperation proves an operation update sends PATCH
// to .../transactions/{txID}/operations/{opID} and decodes the 200 response into
// models.Operation (NOT models.Transaction) — the endpoint returns an operation.
func TestTransactionsFacade_UpdateOperation(t *testing.T) {
	const opID = "77777777-7777-7777-7777-777777777777"
	var gotMethod, gotPath, gotCT string
	opBody := `{"id":"` + opID + `","transactionId":"` + txID + `","type":"DEBIT","description":"adjusted","assetCode":"USD"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotCT = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(opBody))
	}))
	defer srv.Close()

	input := models.NewUpdateOperationInput().WithDescription("adjusted")

	op, err := newTestTransactionsFacade(t, srv).UpdateOperation(context.Background(), txOrgID, txLedgerID, txID, opID, input)
	if err != nil {
		t.Fatalf("UpdateOperation: %v", err)
	}

	wantPath := txBase() + "/" + txID + "/operations/" + opID
	if gotMethod != http.MethodPatch || gotPath != wantPath {
		t.Fatalf("req = %s %s, want PATCH %s", gotMethod, gotPath, wantPath)
	}
	if gotCT != jsonContentType {
		t.Fatalf("Content-Type = %q, want %q", gotCT, jsonContentType)
	}
	if op.ID != opID {
		t.Fatalf("op.ID = %q, want %q", op.ID, opID)
	}
	if op.TransactionID != txID {
		t.Fatalf("op.TransactionID = %q, want %q", op.TransactionID, txID)
	}
	if op.Type != "DEBIT" {
		t.Fatalf("op.Type = %q, want DEBIT", op.Type)
	}
}

// TestTransactionsFacade_UpdateOperationEmptyRejected proves an empty operation
// update payload is rejected before any request leaves the process.
func TestTransactionsFacade_UpdateOperationEmptyRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("server must not be hit on empty operation update payload")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	const opID = "77777777-7777-7777-7777-777777777777"
	if _, err := newTestTransactionsFacade(t, srv).UpdateOperation(context.Background(), txOrgID, txLedgerID, txID, opID, models.NewUpdateOperationInput()); err == nil {
		t.Fatal("expected validation error for empty operation update payload")
	}
}

// TestTransactionsFacade_Get decodes a single transaction from raw bytes into
// the public model (never the generated UUID-eager type) and returns it.
func TestTransactionsFacade_Get(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(txResponseBody()))
	}))
	defer srv.Close()

	tx, err := newTestTransactionsFacade(t, srv).Get(context.Background(), txOrgID, txLedgerID, txID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != txBase()+"/"+txID {
		t.Fatalf("req = %s %s, want GET %s", gotMethod, gotPath, txBase()+"/"+txID)
	}
	if tx.ID != txID {
		t.Fatalf("tx.ID = %q, want %q", tx.ID, txID)
	}
}

// TestTransactionsFacade_GetError maps a non-2xx into the unified error,
// threading APICode/StatusCode/RequestID.
func TestTransactionsFacade_GetError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-get-404")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0007","title":"Transaction not found","status":404}`))
	}))
	defer srv.Close()

	_, err := newTestTransactionsFacade(t, srv).Get(context.Background(), txOrgID, txLedgerID, txID)
	if err == nil {
		t.Fatal("expected error on 404 get")
	}

	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0007" || sdkErr.StatusCode != http.StatusNotFound || sdkErr.RequestID != "req-get-404" {
		t.Fatalf("decoded error = %+v, want APICode=LEDGER-0007 StatusCode=404 RequestID=req-get-404", sdkErr)
	}
}

// txPageWithCursor is a one-item cursor page carrying next_cursor (more to
// come). Real UUID for the item id: this decodes into models.Transaction.
func txPageWithCursor(id, next string) string {
	return `{"items":[{"id":"` + id + `","status":{"code":"APPROVED"}}],"next_cursor":"` + next + `"}`
}

// txTerminalFullPage is the 2.1 footgun fixture: a FULL terminal page (ItemCount
// == limit) that ALSO carries a page field but NO next_cursor. HasMore()'s
// branch 3 (page>0 && limit>0 && ItemCount>=limit) returns true here, so a
// cursor loop keyed on !HasMore() would refetch forever. A cursor-pure stop
// (next_cursor=="") terminates correctly.
func txTerminalFullPage(id string, limit int) string {
	return `{"items":[{"id":"` + id + `","status":{"code":"APPROVED"}}],"page":2,"limit":` +
		strconv.Itoa(limit) + `,"next_cursor":""}`
}

// TestTransactionsFacade_ListCursorChainsAndTerminates proves the cursor
// iterator chains >=2 pages AND terminates on a full terminal page whose only
// terminal signal is next_cursor=="" — the exact shape that loops forever under
// a page-based !HasMore() stop.
func TestTransactionsFacade_ListCursorChainsAndTerminates(t *testing.T) {
	const (
		id1 = "55555555-5555-5555-5555-555555555555"
		id2 = "66666666-6666-6666-6666-666666666666"
	)
	var cursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "" {
			_, _ = w.Write([]byte(txPageWithCursor(id1, "CURSOR2")))
			return
		}
		// Terminal page is FULL and carries a page field: only next_cursor=""
		// can safely stop the loop.
		_, _ = w.Write([]byte(txTerminalFullPage(id2, 1)))
	}))
	defer srv.Close()

	f := newTestTransactionsFacade(t, srv)
	opts := models.TransactionsListOpts{CursorListOpts: models.CursorListOpts{Limit: 1}}

	var got []string
	pages := 0
	for tx, err := range f.All(context.Background(), txOrgID, txLedgerID, opts) {
		if err != nil {
			t.Fatalf("All: %v", err)
		}
		got = append(got, tx.ID)
		if pages++; pages > 10 {
			t.Fatal("cursor iterator did not terminate (infinite loop) — page-based HasMore footgun")
		}
	}

	if len(got) != 2 || got[0] != id1 || got[1] != id2 {
		t.Fatalf("collected ids = %v, want [%s %s]", got, id1, id2)
	}
	// Two requests: first with empty cursor, second echoing the server cursor.
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "CURSOR2" {
		t.Fatalf("cursors seen = %v, want [\"\" \"CURSOR2\"]", cursors)
	}
}

// TestTransactionsFacade_Count parses X-Total-Count from the HEAD response and
// sends only the filters the count endpoint honors (status/route/dates).
func TestTransactionsFacade_Count(t *testing.T) {
	var gotMethod string
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		q = r.URL.Query()
		w.Header().Set(HeaderTotalCount, "42")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	opts := models.TransactionsListOpts{
		CursorListOpts: models.CursorListOpts{StartDate: "2026-01-01", EndDate: "2026-02-01"},
		Filters:        models.TransactionsFilters{Status: "APPROVED", Route: "cashin"},
	}

	n, err := newTestTransactionsFacade(t, srv).Count(context.Background(), txOrgID, txLedgerID, opts)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if gotMethod != http.MethodHead {
		t.Fatalf("method = %s, want HEAD", gotMethod)
	}
	if n != 42 {
		t.Fatalf("count = %d, want 42", n)
	}
	for k, v := range map[string]string{
		"status": "APPROVED", "route": "cashin",
		// Days in, day boundaries out — see countTransactionsParams.
		"start_date": "2026-01-01T00:00:00Z",
		"end_date":   "2026-02-01T23:59:59.999999999Z",
	} {
		if got := q.Get(k); got != v {
			t.Fatalf("count query[%q] = %q, want %q", k, got, v)
		}
	}
}

// TestTransactionsFacade_CountMissingHeader surfaces a missing/blank
// X-Total-Count as an error rather than silently returning 0.
func TestTransactionsFacade_CountMissingHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK) // no X-Total-Count header
	}))
	defer srv.Close()

	if _, err := newTestTransactionsFacade(t, srv).Count(context.Background(), txOrgID, txLedgerID, models.TransactionsListOpts{}); err == nil {
		t.Fatal("expected error on missing X-Total-Count header")
	}
}

// TestTransactionsFacade_CountError maps a non-2xx into the unified error. Count
// is a HEAD read (out of scope of the write-path fix), so this only asserts an
// error surfaces — not the decoded APICode, which a HEAD never carries a body
// for.
func TestTransactionsFacade_CountError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0403","title":"Forbidden","status":403}`))
	}))
	defer srv.Close()

	if _, err := newTestTransactionsFacade(t, srv).Count(context.Background(), txOrgID, txLedgerID, models.TransactionsListOpts{}); err == nil {
		t.Fatal("expected error on 403 count")
	}
}

// TestTransactionsFacade_CountErrorEmptyBody is the harden case (Task 2.3.4): a
// HEAD count is a headers-only response, so an error status carries a
// Content-Type: application/problem+json header with an EMPTY body. The generated
// ParseCountTransactionsByFiltersResp gates on "json" in the content type and
// json.Unmarshals the (empty) body, which errors — the WithResponse path then
// returns (nil, err) and the facade misclassifies a real 403 as an INTERNAL
// error. Routing through the raw method + readCount decodes the status directly:
// DecodeProblemJSON handles the empty body and the 403 surfaces as an
// authorization error, never internal.
func TestTransactionsFacade_CountErrorEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusForbidden) // headers-only: no body, as a real HEAD 403 sends
	}))
	defer srv.Close()

	_, err := newTestTransactionsFacade(t, srv).Count(context.Background(), txOrgID, txLedgerID, models.TransactionsListOpts{})
	if err == nil {
		t.Fatal("expected error on 403 count with empty body")
	}
	if sdkerrors.IsInternalError(err) {
		t.Fatalf("403 empty-body count must not map to internal error, got: %v", err)
	}
	if !sdkerrors.IsAuthorizationError(err) {
		t.Fatalf("403 empty-body count must map to authorization error, got: %v", err)
	}
}

// TestTransactionsFacade_CreateJSON_DecodesOperationFinancialFields is the
// money-path wire-decode regression retargeted from the deleted
// transactions_http_test.go (Epic 5.4): a hand-written /transactions/json
// response whose operations[] carry STRING decimals must decode into the typed
// operation money fields. It REDs on a json-tag or type regression on
// Operation.Amount/Balance/BalanceAfter — a struct round-trip would hide that;
// the hand-written body pins the real wire shape.
func TestTransactionsFacade_CreateJSON_DecodesOperationFinancialFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != txBase()+"/json" {
			t.Errorf("path = %q, want %q", got, txBase()+"/json")
		}

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"id":"` + txID + `",
			"amount":"100",
			"assetCode":"USD",
			"operations":[{
				"id":"op-1",
				"type":"DEBIT",
				"assetCode":"USD",
				"amount":{"value":"100"},
				"balance":{"available":"900","onHold":"0"},
				"balanceAfter":{"available":"800","onHold":"0"},
				"status":{"code":"APPROVED"}
			}]
		}`))
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	result, err := newTestTransactionsFacade(t, srv).CreateJSON(context.Background(), txOrgID, txLedgerID, sampleTransactionInput())
	if err != nil {
		t.Fatalf("CreateJSON: %v", err)
	}

	if len(result.Operations) != 1 {
		t.Fatalf("operations = %d, want 1", len(result.Operations))
	}

	op := result.Operations[0]
	if op.Amount.Value == nil || op.Balance.Available == nil || op.BalanceAfter.Available == nil {
		t.Fatalf("nil money field(s): amount=%v balance=%v after=%v",
			op.Amount.Value, op.Balance.Available, op.BalanceAfter.Available)
	}

	if got := op.Amount.Value.String(); got != "100" {
		t.Errorf("operation amount.value = %q, want 100", got)
	}

	if got := op.Balance.Available.String(); got != "900" {
		t.Errorf("operation balance.available = %q, want 900", got)
	}

	if got := op.BalanceAfter.Available.String(); got != "800" {
		t.Errorf("operation balanceAfter.available = %q, want 800", got)
	}

	if op.Status.Code != "APPROVED" {
		t.Errorf("operation status.code = %q, want APPROVED", op.Status.Code)
	}
}

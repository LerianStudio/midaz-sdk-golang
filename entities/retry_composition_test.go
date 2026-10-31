// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/internal/genledger"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"
)

// retryEnabledOptions is the retry-enabled variant used ONLY by the composition
// / end-to-end money-write tests. The shared facade harness
// (planeTestRetryOptions) keeps retries OFF for determinism; these tests need
// the engine actually looping.
func retryEnabledOptions(maxRetries int) retry.Options {
	return retry.Options{
		MaxRetries:         maxRetries,
		InitialDelay:       time.Millisecond,
		MaxDelay:           5 * time.Millisecond,
		BackoffFactor:      2.0,
		RetryableHTTPCodes: []int{http.StatusServiceUnavailable, http.StatusGatewayTimeout},
		RetryableErrors:    []string{"i/o timeout"},
	}
}

// TestRetryComposesWithAuthReplay is F2(a): the retry engine and a LIVE auth
// round tripper coexist. Server serves 401 → 503 → 200 to a POST carrying a
// body + X-Idempotency. This locks the money-path invariant three reviewers
// verified by construction but nobody tested: across the auth 401-replay AND
// the engine retry, EVERY wire request carries identical body bytes AND an
// identical X-Idempotency key, and the two layers compose to
// 1 initial + 1 auth-replay + 1 engine-retry = 3 wire requests.
func TestRetryComposesWithAuthReplay(t *testing.T) {
	var (
		mu     sync.Mutex
		bodies [][]byte
		keys   []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)

		mu.Lock()
		bodies = append(bodies, b)
		keys = append(keys, r.Header.Get(idempotencyHeader))
		n := len(bodies)
		mu.Unlock()

		switch n {
		case 1:
			w.WriteHeader(http.StatusUnauthorized) // triggers auth refresh+replay
		case 2:
			w.WriteHeader(http.StatusServiceUnavailable) // triggers engine retry
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()

	authRT := newAuthRefreshRoundTripper(http.DefaultTransport, authRoundTripperConfig{
		tokenProvider: func(context.Context) (string, error) { return "tok-live", nil },
	})
	rt := newRetryRoundTripper(authRT, retryEnabledOptions(3), nil)

	payload := []byte(`{"amount":100,"asset":"BRL"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(payload))
	req.Header.Set(idempotencyHeader, "key-compose")

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()

	if len(bodies) != 3 {
		t.Fatalf("wire requests = %d, want 3 (1 initial + 1 auth-replay + 1 engine-retry)", len(bodies))
	}

	for i := range bodies {
		if len(bodies[i]) == 0 {
			t.Fatalf("wire %d body is empty (body did not survive retry x auth)", i+1)
		}

		if !bytes.Equal(bodies[i], payload) {
			t.Fatalf("wire %d body = %q, want %q", i+1, bodies[i], payload)
		}

		if keys[i] != "key-compose" {
			t.Fatalf("wire %d X-Idempotency = %q, want key-compose (key mutated across replay/retry)", i+1, keys[i])
		}
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("final status = %d, want 200", resp.StatusCode)
	}
}

// newRetryEnabledTxFacade builds a transactions facade over the REAL plane
// chain (retry RT -> live auth RT -> generated ledger client) with retries
// enabled — the end-to-end money-write harness for F2(b).
func newRetryEnabledTxFacade(t *testing.T, srv *httptest.Server, maxRetries int) *transactionsFacade {
	t.Helper()

	return newTransactionsFacade(newRetryEnabledPlanes(t, srv, maxRetries).Ledger, true)
}

// newRetryEnabledPlanes builds the real plane chain (retry RT -> live auth RT -> generated clients).
func newRetryEnabledPlanes(t *testing.T, srv *httptest.Server, maxRetries int) *PlaneClients {
	t.Helper()

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: srv.URL,
		tracerURL: srv.URL + "/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-live", nil },
		},
		httpClient:   srv.Client(),
		retryOptions: retryEnabledOptions(maxRetries),
	})
	if err != nil {
		t.Fatalf("newPlaneClients: %v", err)
	}

	return planes
}

// TestFacadeWriteRetriesWithStableAutoIdempotency is F2(b): a real facade WRITE
// (Transactions.CreateJSON, the auto-idempotent money-write) through the full
// plane chain with retries enabled. Server 503,503,200 → exactly 3 attempts,
// all carrying the SAME auto-GENERATED X-Idempotency key (not an explicit one)
// and identical body. A per-attempt-regenerated key would be a double-charge.
// Each retry fires retry.WithAttemptHook, which callers read to mark a retried
// failure as possibly applied.
func TestFacadeWriteRetriesWithStableAutoIdempotency(t *testing.T) {
	var (
		mu     sync.Mutex
		bodies [][]byte
		keys   []string
		hooks  int
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)

		mu.Lock()
		bodies = append(bodies, b)
		keys = append(keys, r.Header.Get(idempotencyHeader))
		n := len(bodies)
		mu.Unlock()

		if n < 3 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"type":"about:blank","title":"Service Unavailable","status":503}`))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(txResponseBody()))
	}))
	defer srv.Close()

	ctx := retry.WithAttemptHook(context.Background(), func(context.Context, int, error, time.Duration) { hooks++ })

	tx, err := newRetryEnabledTxFacade(t, srv, 3).
		CreateJSON(ctx, txOrgID, txLedgerID, sampleTransactionInput())
	if err != nil {
		t.Fatalf("CreateJSON: %v", err)
	}

	if hooks != 2 {
		t.Fatalf("attempt hook fired %d times, want 2 (once per retry)", hooks)
	}

	if tx == nil || tx.ID != txID {
		t.Fatalf("decoded tx = %+v, want id %s", tx, txID)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(bodies) != 3 {
		t.Fatalf("attempts = %d, want 3 (1 + 2 retries)", len(bodies))
	}

	if keys[0] == "" {
		t.Fatalf("attempt 1 carried no auto-generated X-Idempotency key")
	}

	for i := range bodies {
		if keys[i] != keys[0] {
			t.Fatalf("attempt %d X-Idempotency = %q, want %q (auto-gen key must be stable across retries)", i+1, keys[i], keys[0])
		}

		if len(bodies[i]) == 0 || !bytes.Equal(bodies[i], bodies[0]) {
			t.Fatalf("attempt %d body = %q, want identical non-empty %q", i+1, bodies[i], bodies[0])
		}
	}
}

// TestFacadeWritePersistentServerErrorDecodesProblem is F2(b), second half: on
// a persistent 503 the FACADE returns a DECODED problem error (via the real
// decodeOne / readRawResponse path), not a garbled internal error, after the
// retry budget is spent.
func TestFacadeWritePersistentServerErrorDecodesProblem(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"type":"about:blank","title":"Service Unavailable","status":503,"detail":"boom"}`))
	}))
	defer srv.Close()

	_, err := newRetryEnabledTxFacade(t, srv, 2).
		CreateJSON(context.Background(), txOrgID, txLedgerID, sampleTransactionInput())
	if err == nil {
		t.Fatalf("CreateJSON: want a decoded 503 error after exhaustion")
	}

	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error (decoded problem, not garbled internal)", err)
	}

	if sdkErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", sdkErr.StatusCode)
	}

	if got := atomic.LoadInt32(&count); got != 3 {
		t.Fatalf("attempts = %d, want 3 (1 + 2 retries)", got)
	}
}

// timeoutFirst makes the first wire attempt fail with a non-dial transport timeout when on,
// and counts every attempt that leaves the retry layer.
type timeoutFirst struct {
	base     http.RoundTripper
	on       bool
	attempts atomic.Int32
}

func (t *timeoutFirst) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.attempts.Add(1) == 1 && t.on {
		return nil, &net.OpError{Op: "read", Err: timeoutErr{}}
	}

	return t.base.RoundTrip(req)
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

// TestFacadeWriteRetriesIdempotencyInFlight: a 409/0084 is retried under the same key only when the
// SDK generated the call's key, so the 0084 can only be this call's own earlier attempt. A key the
// caller supplied (ctx or input field) may be held by another call's older claim, so its 0084
// surfaces as an error. Any other 409 is never retried.
func TestFacadeWriteRetriesIdempotencyInFlight(t *testing.T) {
	for _, tc := range []struct {
		name     string
		replies  []string // per request that reaches the server: "503", "504", a 409 code, or "201"
		ctxKey   string
		inputKey string
		timeout  bool // the first attempt fails with a transport timeout before reaching the server
		attempts int32
		wantErr  bool
	}{
		{name: "sdk key 503 then 0084 retried", replies: []string{"503", "0084", "201"}, attempts: 3},
		{name: "sdk key timeout then 0084 retried", replies: []string{"0084", "201"}, timeout: true, attempts: 3},
		{name: "caller ctx key 504 then 0084 surfaces", replies: []string{"504", "0084", "201"}, ctxKey: "caller-key", attempts: 2, wantErr: true},
		{name: "caller ctx key 503 then 0084 surfaces", replies: []string{"503", "0084", "201"}, ctxKey: "caller-key", attempts: 2, wantErr: true},
		{name: "input key 504 then 0084 surfaces", replies: []string{"504", "0084", "201"}, inputKey: "input-key", attempts: 2, wantErr: true},
		{name: "other 409 not retried", replies: []string{"503", "0101", "201"}, attempts: 2, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				mu   sync.Mutex
				keys []string
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				keys = append(keys, r.Header.Get(idempotencyHeader))
				reply := tc.replies[len(keys)-1]
				mu.Unlock()

				writeReply(w, reply)
			}))
			defer srv.Close()

			wire := &timeoutFirst{base: srv.Client().Transport, on: tc.timeout}
			srv.Client().Transport = wire

			input := sampleTransactionInput()
			input.IdempotencyKey = tc.inputKey

			ctx := sdkctx.WithIdempotencyKey(context.Background(), tc.ctxKey)

			tx, err := newRetryEnabledTxFacade(t, srv, 3).CreateJSON(ctx, txOrgID, txLedgerID, input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}

			if !tc.wantErr && (tx == nil || tx.ID != txID) {
				t.Fatalf("decoded tx = %+v, want id %s", tx, txID)
			}

			if got := wire.attempts.Load(); got != tc.attempts {
				t.Fatalf("attempts = %d, want %d", got, tc.attempts)
			}

			mu.Lock()
			defer mu.Unlock()

			if !oneKey(keys, tc.ctxKey+tc.inputKey) {
				t.Fatalf("keys = %v, want one stable key (the caller's when supplied)", keys)
			}
		})
	}
}

// TestPlaneWriteSurfacesCallerStamped0084: a key the caller stamps itself through the exported
// plane client carries no SDK mark, so its 0084 surfaces instead of being retried.
func TestPlaneWriteSurfacesCallerStamped0084(t *testing.T) {
	var attempts atomic.Int32

	replies := []string{"504", "0084", "201"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeReply(w, replies[attempts.Add(1)-1])
	}))
	defer srv.Close()

	stamp := func(_ context.Context, req *http.Request) error {
		req.Header.Set(idempotencyHeader, "their-key")

		return nil
	}

	resp, err := newRetryEnabledPlanes(t, srv, 3).Ledger.CreateTransactionJSONWithBody(context.Background(),
		txOrgID, txLedgerID, &genledger.CreateTransactionJSONParams{}, jsonContentType, strings.NewReader("{}"), stamp)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict || attempts.Load() != 2 {
		t.Fatalf("status = %d after %d attempts, want 409 after 2", resp.StatusCode, attempts.Load())
	}
}

// oneKey reports whether every key is the same non-empty key, equal to want when want is set.
func oneKey(keys []string, want string) bool {
	for _, key := range keys {
		if key == "" || key != keys[0] || (want != "" && key != want) {
			return false
		}
	}

	return true
}

// writeReply answers "503", "504", "201" with the sample transaction, or a 409 carrying reply as its code.
func writeReply(w http.ResponseWriter, reply string) {
	w.Header().Set("Content-Type", "application/json")

	switch reply {
	case "503":
		w.WriteHeader(http.StatusServiceUnavailable)
	case "504":
		w.WriteHeader(http.StatusGatewayTimeout)
	case "201":
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(txResponseBody()))
	default:
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"` + reply + `","title":"Conflict","status":409}`))
	}
}

// TestFacadeWriteStopsRetryingAfterSlotTTL: a write whose X-TTL has passed since its
// first send is not retried, since Midaz may no longer hold the key and would run it again.
// The first attempt answers 503 after the slot's 1 s, so the retry would land past it.
func TestFacadeWriteStopsRetryingAfterSlotTTL(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if attempts.Add(1) == 1 {
			time.Sleep(1100 * time.Millisecond)
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(txResponseBody()))
	}))
	defer srv.Close()

	ctx := sdkctx.WithIdempotencyTTL(context.Background(), 1)

	_, err := newRetryEnabledTxFacade(t, srv, 3).CreateJSON(ctx, txOrgID, txLedgerID, sampleTransactionInput())
	if err == nil {
		t.Fatal("err = nil, want the 503 the retry was refused after")
	}

	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}

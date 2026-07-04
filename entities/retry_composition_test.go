// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/retry"
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
		RetryableHTTPCodes: []int{http.StatusServiceUnavailable},
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

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: srv.URL + "/v1",
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

	return newTransactionsFacade(planes.Ledger, true)
}

// TestFacadeWriteRetriesWithStableAutoIdempotency is F2(b): a real facade WRITE
// (Transactions.CreateJSON, the auto-idempotent money-write) through the full
// plane chain with retries enabled. Server 503,503,200 → exactly 3 attempts,
// all carrying the SAME auto-GENERATED X-Idempotency key (not an explicit one)
// and identical body. A per-attempt-regenerated key would be a double-charge.
func TestFacadeWriteRetriesWithStableAutoIdempotency(t *testing.T) {
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

	tx, err := newRetryEnabledTxFacade(t, srv, 3).
		CreateJSON(context.Background(), txOrgID, txLedgerID, sampleTransactionInput())
	if err != nil {
		t.Fatalf("CreateJSON: %v", err)
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

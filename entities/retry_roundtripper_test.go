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

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
)

// erroringRoundTripper is a base transport that always fails with a fixed
// error, counting invocations. Used to exercise the transport-error branch
// deterministically (no real sockets).
type erroringRoundTripper struct {
	calls int32
	err   error
}

func (e *erroringRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	atomic.AddInt32(&e.calls, 1)

	return nil, e.err
}

// fastRetryOpts returns the default retry policy with the backoff delays
// squeezed down so the tests exercise the retry loop without sleeping for the
// production 100ms → 10s schedule.
func fastRetryOpts() retry.Options {
	o := retry.DefaultOptions()
	o.InitialDelay = time.Millisecond
	o.MaxDelay = 5 * time.Millisecond

	return *o
}

// TestRetryRoundTripper_RetriesThenSucceeds proves invariant (a): a 503,503,200
// sequence on a request carrying an idempotency key produces EXACTLY three wire
// requests, every one of them carrying the IDENTICAL X-Idempotency value, and
// the final result is the 200. This is the no-double-charge core: retries never
// mutate the idempotency key.
func TestRetryRoundTripper_RetriesThenSucceeds(t *testing.T) {
	var (
		mu       sync.Mutex
		count    int
		seenKeys []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		n := count
		seenKeys = append(seenKeys, r.Header.Get(idempotencyHeader))
		mu.Unlock()

		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"title":"unavailable"}`))

			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), nil)

	req, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader([]byte(`{"x":1}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set(idempotencyHeader, "key-123")

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	mu.Lock()
	defer mu.Unlock()

	if count != 3 {
		t.Fatalf("request count = %d, want 3", count)
	}

	for i, k := range seenKeys {
		if k != "key-123" {
			t.Fatalf("attempt %d idempotency key = %q, want key-123", i+1, k)
		}
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("final status = %d, want 200", resp.StatusCode)
	}

	if string(body) != `{"ok":true}` {
		t.Fatalf("final body = %q, want {\"ok\":true}", body)
	}
}

// TestRetryRoundTripper_UnsafeWithoutKeyNotRetried proves invariant (b): a POST
// with NO idempotency key must be tried exactly once even on a retryable 503 —
// retrying a keyless write risks a double balance mutation.
func TestRetryRoundTripper_UnsafeWithoutKeyNotRetried(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"title":"unavailable"}`))
	}))
	defer srv.Close()

	rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), nil)

	req, _ := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader([]byte(`{"x":1}`)))

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("request count = %d, want 1 (unsafe method without idempotency key must not retry)", got)
	}

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

// TestRetryRoundTripper_SuppressedByContext proves invariant (c): a context
// tagged with WithoutHTTPRetries forces a single attempt even when a key is
// present and the status is retryable.
func TestRetryRoundTripper_SuppressedByContext(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), nil)

	req, _ := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader([]byte(`{"x":1}`)))
	req.Header.Set(idempotencyHeader, "key-xyz")
	req = req.WithContext(sdkctx.WithoutHTTPRetries(req.Context()))

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("request count = %d, want 1 (retries suppressed via context)", got)
	}
}

// TestRetryRoundTripper_RewindsBody proves invariant (d): on a retried attempt
// the server receives the SAME non-empty body it received on the first attempt.
// A retry that dropped or truncated the body would be a corrupted money-path
// replay.
func TestRetryRoundTripper_RewindsBody(t *testing.T) {
	var (
		mu     sync.Mutex
		bodies [][]byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)

		mu.Lock()
		bodies = append(bodies, b)
		n := len(bodies)
		mu.Unlock()

		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), nil)

	payload := []byte(`{"amount":100,"asset":"BRL"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(payload))
	req.Header.Set(idempotencyHeader, "key-body")

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()

	if len(bodies) < 2 {
		t.Fatalf("server saw %d requests, want at least 2 (retry expected)", len(bodies))
	}

	if len(bodies[0]) == 0 {
		t.Fatalf("attempt 1 body is empty")
	}

	if !bytes.Equal(bodies[0], bodies[1]) {
		t.Fatalf("attempt 2 body = %q, want identical to attempt 1 body %q", bodies[1], bodies[0])
	}

	if !bytes.Equal(bodies[0], payload) {
		t.Fatalf("attempt 1 body = %q, want %q", bodies[0], payload)
	}
}

// TestRetryRoundTripper_CustomPolicyHonored proves invariant (e): a custom
// policy retrying on a status OUTSIDE the default retryable set (418) is
// honored. A GET is used so the unsafe-method gate does not interfere.
func TestRetryRoundTripper_CustomPolicyHonored(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&count, 1) < 2 {
			w.WriteHeader(http.StatusTeapot)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	policy := func(resp *http.Response, _ error) bool {
		return resp != nil && resp.StatusCode == http.StatusTeapot
	}

	rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), policy)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if got := atomic.LoadInt32(&count); got != 2 {
		t.Fatalf("request count = %d, want 2 (custom policy should retry the 418)", got)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("final status = %d, want 200", resp.StatusCode)
	}
}

// TestRetryRoundTripper_NilCustomPolicySafe proves invariant (e, negative
// half): a nil custom policy is safe and a status outside the default retryable
// set (418) is NOT retried.
func TestRetryRoundTripper_NilCustomPolicySafe(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()

	rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), nil)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("request count = %d, want 1 (418 not retryable, nil policy)", got)
	}

	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", resp.StatusCode)
	}
}

// TestRetryRoundTripper_CustomPolicyFalseOn5xxSuppresses proves F1 parity: a
// custom policy is SUBSTITUTIVE. When present and it returns false on a 503,
// the default RetryableHTTPCodes set is IGNORED — exactly 1 attempt (matches
// legacy handleRetryAttemptResponse: policy false → retry.AsNonRetryable).
func TestRetryRoundTripper_CustomPolicyFalseOn5xxSuppresses(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"title":"unavailable"}`))
	}))
	defer srv.Close()

	policy := func(*http.Response, error) bool { return false }

	rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), policy)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("request count = %d, want 1 (custom policy false is SUBSTITUTIVE: default 503-retryable must be ignored)", got)
	}

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

// TestRetryRoundTripper_CustomPolicyFalseOnTransportErrorSuppresses proves F1
// parity for the transport-error branch: a custom policy returning false on an
// otherwise-retryable transport error (its message matches DefaultRetryableErrors)
// suppresses the retry — exactly 1 attempt (matches legacy
// handleRequestExecutionError: policy false → retry.AsNonRetryable). This is
// the "I don't know if the write landed, don't replay" defensive move.
func TestRetryRoundTripper_CustomPolicyFalseOnTransportErrorSuppresses(t *testing.T) {
	base := &erroringRoundTripper{err: errors.New("dial tcp: connection refused")}
	policy := func(*http.Response, error) bool { return false }

	rt := newRetryRoundTripper(base, fastRetryOpts(), policy)

	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid", nil)

	//nolint:bodyclose // transport error path: no response/body is produced.
	_, err := rt.RoundTrip(req)
	if err == nil {
		t.Fatalf("RoundTrip: want the transport error surfaced")
	}

	if got := atomic.LoadInt32(&base.calls); got != 1 {
		t.Fatalf("base calls = %d, want 1 (custom policy false must suppress the retryable transport error)", got)
	}
}

// TestRetryRoundTripper_CustomPolicyNotConsultedOnSuccess proves F1 parity: a
// 2xx returns BEFORE the policy is consulted, so a policy that would retry
// everything can NEVER replay a success (matches legacy: success <400 returns
// nil before reaching the policy).
func TestRetryRoundTripper_CustomPolicyNotConsultedOnSuccess(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	policy := func(*http.Response, error) bool { return true } // would retry anything

	rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), policy)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("request count = %d, want 1 (a 2xx must never be replayed by a custom policy)", got)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// TestRetryRoundTripper_ExhaustionReturnsResponse proves invariant (f): when
// retries exhaust on a persistent 503 the RoundTripper returns a DECODABLE 503
// response (non-nil, StatusCode 503, body intact), NOT a bare transport error —
// so the facade parses the problem-JSON exactly as it would with no retry.
func TestRetryRoundTripper_ExhaustionReturnsResponse(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"title":"down"}`))
	}))
	defer srv.Close()

	rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), nil)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip returned an error, want a decodable response: %v", err)
	}

	if resp == nil {
		t.Fatalf("RoundTrip returned nil response on exhaustion")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"title":"down"}` {
		t.Fatalf("final body = %q, want {\"title\":\"down\"} (must remain decodable)", body)
	}

	// 1 initial attempt + 3 retries (DefaultOptions MaxRetries).
	if got := atomic.LoadInt32(&count); got != 4 {
		t.Fatalf("request count = %d, want 4 (1 + 3 retries)", got)
	}
}

// TestRetryRoundTripper_Bare401NotRetried proves invariant (g): 401 is OWNED by
// the inner auth round tripper and is NOT one of the retry RT's retryable
// codes. A bare 401 (no auth provider below) is returned as-is without extra
// attempts.
func TestRetryRoundTripper_Bare401NotRetried(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), nil)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("request count = %d, want 1 (401 is not retryable at the retry RT)", got)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// TestRetryRoundTripper_ContextDeadlineDuringBackoffSurfaces proves F3: when a
// context deadline fires DURING backoff (after a retryable 503), RoundTrip must
// surface the context error, not the stale buffered 503 with a nil error.
func TestRetryRoundTripper_ContextDeadlineDuringBackoffSurfaces(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	// InitialDelay far exceeds the ctx deadline, so the deadline fires while the
	// engine waits before attempt 2.
	opts := retry.Options{
		MaxRetries:         3,
		InitialDelay:       200 * time.Millisecond,
		MaxDelay:           400 * time.Millisecond,
		BackoffFactor:      2.0,
		RetryableHTTPCodes: []int{http.StatusServiceUnavailable},
	}
	rt := newRetryRoundTripper(http.DefaultTransport, opts, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req = req.WithContext(ctx)

	resp, err := rt.RoundTrip(req)
	if err == nil {
		t.Fatalf("RoundTrip: want the context error, got a response")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}

	if resp != nil {
		_ = resp.Body.Close()
		t.Fatalf("resp = %v, want nil (context error must not be masked by the buffered 503)", resp)
	}

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("request count = %d, want 1 (deadline fired during the first backoff)", got)
	}
}

// TestRetryRoundTripper_ContextDeadline is a guard against a context that
// carries retry options being ignored: the retry RT installs its OWN resolved
// options on the ctx, so a caller-set deadline must still cancel the loop.
func TestRetryRoundTripper_ContextCancelStops(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), nil)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req = req.WithContext(ctx)

	//nolint:bodyclose // cancelled before the first attempt: no response/body is produced.
	_, err := rt.RoundTrip(req)
	if err == nil {
		t.Fatalf("RoundTrip: want error on a cancelled context")
	}

	if got := atomic.LoadInt32(&count); got != 0 {
		t.Fatalf("request count = %d, want 0 (cancelled before first attempt)", got)
	}
}

// TestPlaneWrite_RequestCarriesGetBody proves the money-path rewind
// precondition: a generated plane WRITE (Holders.Create -> CreateHolderWithBody)
// leaves the wire with req.GetBody != nil. That is exactly what lets the inner
// auth-refresh round tripper replay after a 401, and the retry round tripper
// rewind after a retryable 5xx, resending the identical JSON body. A nil GetBody
// would replay an empty body — a corrupted balance-affecting write. The request
// is captured at the base transport (below auth + retry) because GetBody is a
// client-side field, invisible to an httptest handler.
func TestPlaneWrite_RequestCarriesGetBody(t *testing.T) {
	var sawRequest, sawGetBody bool

	base := &countingRoundTripper{
		response: func(req *http.Request) *http.Response {
			sawRequest = true
			sawGetBody = req.GetBody != nil

			return &http.Response{
				StatusCode: http.StatusCreated,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"id":"11111111-1111-1111-1111-111111111111","name":"Alice"}`))),
			}
		},
	}

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: "http://ledger.invalid/v1",
		tracerURL: "http://tracer.invalid/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-1", nil },
		},
		httpClient:   &http.Client{Transport: base},
		retryOptions: fastRetryOpts(),
	})
	if err != nil {
		t.Fatalf("newPlaneClients: %v", err)
	}

	_, err = newHoldersFacade(planes.Ledger, true).Create(
		context.Background(),
		holdersFacadeOrgID,
		models.NewCreateHolderInput(models.HolderTypeNaturalPerson, "Alice", "123"),
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !sawRequest {
		t.Fatal("capturing transport never observed the outbound write request")
	}
	if !sawGetBody {
		t.Fatal("plane write request had GetBody == nil; a 401/5xx replay would resend an empty body")
	}
}

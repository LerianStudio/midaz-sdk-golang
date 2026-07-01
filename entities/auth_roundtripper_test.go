package entities

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestAuthRefreshRoundTripper_BearerInjection verifies the Bearer branch: when
// no API key is configured, the round tripper injects the provider's token as
// "Authorization: Bearer <tok>".
func TestAuthRefreshRoundTripper_BearerInjection(t *testing.T) {
	var gotAuth, gotAPIKey string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rt := newAuthRefreshRoundTripper(http.DefaultTransport, authRoundTripperConfig{
		tokenProvider: func(context.Context) (string, error) { return "tok-abc", nil },
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if gotAuth != "Bearer tok-abc" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer tok-abc")
	}
	if gotAPIKey != "" {
		t.Fatalf("X-API-Key = %q, want empty (Bearer branch)", gotAPIKey)
	}
}

// TestAuthRefreshRoundTripper_APIKeyInjection verifies the tracer X-API-Key
// branch: when an API key is configured, the round tripper injects
// "X-API-Key: <key>" and does NOT attach a Bearer token.
func TestAuthRefreshRoundTripper_APIKeyInjection(t *testing.T) {
	var gotAuth, gotAPIKey string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rt := newAuthRefreshRoundTripper(http.DefaultTransport, authRoundTripperConfig{
		apiKey: "key-xyz",
		// A provider is still wired (shared across planes), but the API-key
		// branch must ignore it.
		tokenProvider: func(context.Context) (string, error) { return "tok-abc", nil },
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if gotAPIKey != "key-xyz" {
		t.Fatalf("X-API-Key = %q, want %q", gotAPIKey, "key-xyz")
	}
	if gotAuth != "" {
		t.Fatalf("Authorization = %q, want empty (API-key branch)", gotAuth)
	}
}

// TestAuthRefreshRoundTripper_RefreshReplayOnce is the money-path guard. On a
// 401, the round tripper must reauthenticate ONCE and replay the SAME request
// exactly once — with the X-Idempotency header byte-identical across both
// attempts. A second 401 must not trigger a second refresh; it surfaces as-is.
func TestAuthRefreshRoundTripper_RefreshReplayOnce(t *testing.T) {
	const idemKey = "caller-supplied-idempotency-key"

	var (
		attempts   int32
		seenAuths  []string
		seenIdem   []string
		seenBodies []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		seenAuths = append(seenAuths, r.Header.Get("Authorization"))
		seenIdem = append(seenIdem, r.Header.Get("X-Idempotency"))
		body, _ := io.ReadAll(r.Body)
		seenBodies = append(seenBodies, string(body))

		if n == 1 {
			// First attempt: stale token → 401.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var refreshCalls int32
	rt := newAuthRefreshRoundTripper(http.DefaultTransport, authRoundTripperConfig{
		tokenProvider: func(context.Context) (string, error) {
			n := atomic.AddInt32(&refreshCalls, 1)
			if n == 1 {
				return "stale-token", nil
			}
			return "fresh-token", nil
		},
	})

	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"amount":100}`))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("X-Idempotency", idemKey)

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("final status = %d, want 200", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("server attempts = %d, want 2 (original + one replay)", got)
	}
	if len(seenIdem) != 2 || seenIdem[0] != idemKey || seenIdem[1] != idemKey {
		t.Fatalf("X-Idempotency across attempts = %v, want both %q (money-path: key must survive replay unchanged)", seenIdem, idemKey)
	}
	if len(seenBodies) != 2 || seenBodies[0] != seenBodies[1] {
		t.Fatalf("request bodies across attempts = %v, want identical replay", seenBodies)
	}
	if seenAuths[0] != "Bearer stale-token" || seenAuths[1] != "Bearer fresh-token" {
		t.Fatalf("auth headers = %v, want [Bearer stale-token, Bearer fresh-token]", seenAuths)
	}
}

// TestAuthRefreshRoundTripper_RefreshOnlyOnceOnPersistent401 verifies the latch:
// if refresh succeeds but the replay still 401s, the round tripper does NOT
// loop — it returns the second 401 and refreshes exactly once.
func TestAuthRefreshRoundTripper_RefreshOnlyOnceOnPersistent401(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	// The invalidator fires only on the refresh path (not on the initial
	// seed injection), so it is the clean signal for "how many refreshes".
	var refreshes int32
	rt := newAuthRefreshRoundTripper(http.DefaultTransport, authRoundTripperConfig{
		tokenProvider:    func(context.Context) (string, error) { return "any-token", nil },
		tokenInvalidator: func() { atomic.AddInt32(&refreshes, 1) },
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("final status = %d, want 401", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("server attempts = %d, want 2 (original + one replay, no more)", got)
	}
	if got := atomic.LoadInt32(&refreshes); got != 1 {
		t.Fatalf("refreshes = %d, want exactly 1", got)
	}
}

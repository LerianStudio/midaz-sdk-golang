package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
)

func TestIdempotencyHeaderInjectionSkipsSafeGET(t *testing.T) {
	var seen string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	hc := srv.Client()
	c := NewHTTPClient(hc, "", nil)

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "abc123")

	var out map[string]any

	err := c.doRequest(ctx, http.MethodGet, srv.URL, nil, nil, &out)
	require.NoError(t, err)

	assert.Empty(t, seen)
}

// TestRedactDebugURLStripsEntireQueryString verifies that redactDebugURL
// wipes the entire query string for debug logging. v3 contract: we do not
// attempt per-key query redaction because cardinality + correctness are
// hard to get right; stripping the whole thing is a simple, auditable
// guarantee. Per-key redaction is a future opt-in via
// [WithDebugIncludeQueryParams] (see TODO).
func TestRedactDebugURLStripsEntireQueryString(t *testing.T) {
	redacted := redactDebugURL("https://api.example.com/holders?document=12345678900&external_id=ext-1&limit=10&metadata.email=a@example.com")

	require.NotEmpty(t, redacted)
	// The exact rendered form: scheme://host/path with no query, no
	// fragment, no userinfo. Direct equality is the strongest assertion.
	assert.Equal(t, "https://api.example.com/holders", redacted)

	// Defensive: even if someone changes the implementation, the rendered
	// URL must parse and must NOT carry a query component.
	redactedURL, err := url.Parse(redacted)
	require.NoError(t, err)
	assert.Empty(t, redactedURL.RawQuery)
	assert.Empty(t, redactedURL.Fragment)
	assert.Nil(t, redactedURL.User)
}

func TestNormalizeTelemetryURLRedactsUnparseableInput(t *testing.T) {
	redacted := normalizeTelemetryURL("https://api.example.com/%zz?accessToken=raw-token&clientSecret=raw-secret#frag")

	assert.NotContains(t, redacted, "raw-token")
	assert.NotContains(t, redacted, "raw-secret")
	assert.NotContains(t, redacted, "accessToken")
	assert.NotContains(t, redacted, "clientSecret")
	assert.NotContains(t, redacted, "frag")
	assert.Contains(t, redacted, "https://api.example.com/%zz")
}

func TestDefaultHTTPClientRejectsAuthenticatedCrossOriginRedirect(t *testing.T) {
	client := defaultHTTPClient()

	previous, err := http.NewRequest(http.MethodGet, "https://api.example.com/v1/accounts", nil)
	require.NoError(t, err)
	previous.Header.Set("Authorization", "Bearer raw-token")

	next, err := http.NewRequest(http.MethodGet, "https://evil.example.net/v1/accounts", nil)
	require.NoError(t, err)

	err = client.CheckRedirect(next, []*http.Request{previous})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authenticated redirect")
}

func TestDefaultHTTPClientAllowsAuthenticatedSameOriginRedirect(t *testing.T) {
	client := defaultHTTPClient()

	previous, err := http.NewRequest(http.MethodGet, "https://api.example.com/v1/accounts", nil)
	require.NoError(t, err)
	previous.Header.Set("Authorization", "Bearer raw-token")

	next, err := http.NewRequest(http.MethodGet, "https://api.example.com/v1/accounts?page=2", nil)
	require.NoError(t, err)

	require.NoError(t, client.CheckRedirect(next, []*http.Request{previous}))
}

func TestNewHTTPClientInstallsRedirectGuardOnCallerClientWithoutPolicy(t *testing.T) {
	callerClient := &http.Client{}
	c := NewHTTPClient(callerClient, "", nil)
	require.NotNil(t, c.client.CheckRedirect)
	require.Nil(t, callerClient.CheckRedirect, "caller-owned client must not be mutated in place")

	previous, err := http.NewRequest(http.MethodGet, "https://api.example.com/v1/accounts", nil)
	require.NoError(t, err)
	previous.Header.Set("X-Idempotency", "raw-idempotency-key")

	next, err := http.NewRequest(http.MethodGet, "https://evil.example.net/v1/accounts", nil)
	require.NoError(t, err)

	err = c.client.CheckRedirect(next, []*http.Request{previous})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authenticated redirect")
}

func TestNewHTTPClientPreservesCallerRedirectPolicy(t *testing.T) {
	callerClient := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return assert.AnError
		},
	}
	c := NewHTTPClient(callerClient, "", nil)

	require.Same(t, callerClient, c.client)
	require.ErrorIs(t, c.client.CheckRedirect(nil, nil), assert.AnError)
}

func TestAutomaticIdempotencyHeaderForUnsafeMethods(t *testing.T) {
	var seen string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodPost, srv.URL, nil, map[string]string{"ok": "true"}, &out)
	require.NoError(t, err)

	assert.NotEmpty(t, seen)
}

// TestWithoutAutoIdempotencySuppressesAutoKey verifies that the per-call
// suppression escape hatch keeps the X-Idempotency header off the wire
// even when client-level idempotency is enabled.
func TestWithoutAutoIdempotencySuppressesAutoKey(t *testing.T) {
	var seenIdem string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenIdem = r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)

	var out map[string]any

	ctx := sdkctx.WithoutAutoIdempotency(context.Background())
	err := c.doRequest(ctx, http.MethodPost, srv.URL, nil, map[string]string{"ok": "true"}, &out)
	require.NoError(t, err)

	assert.Empty(t, seenIdem, "WithoutAutoIdempotency should suppress the auto-generated key")
}

// TestExplicitIdempotencyKeyWinsOverSuppression verifies the documented
// ordering rule: explicit caller key > suppression > auto-generation.
func TestExplicitIdempotencyKeyWinsOverSuppression(t *testing.T) {
	var seenIdem string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenIdem = r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)

	var out map[string]any

	ctx := sdkctx.WithoutAutoIdempotency(sdkctx.WithIdempotencyKey(context.Background(), "explicit-key"))
	err := c.doRequest(ctx, http.MethodPost, srv.URL, nil, map[string]string{"ok": "true"}, &out)
	require.NoError(t, err)

	assert.Equal(t, "explicit-key", seenIdem, "explicit key must win over suppression")
}

func TestUnsafeMethodRetriesOnlyWithIdempotency(t *testing.T) {
	// As of the cluster-D rework, an SDK-generated idempotency key (the
	// MIDAZ_IDEMPOTENCY=on path that auto-attaches X-Idempotency to every
	// unsafe method) DOES enable retries — otherwise idempotency would be
	// useful for at-most-once delivery but useless for transient failure
	// recovery, defeating the entire point of opting in.
	//
	// The rows below assert the behavior:
	//   1. headers=nil → ensureIdempotencyHeader auto-generates a key →
	//      retry on 5xx is allowed → after 1 transient failure we succeed
	//      on the 2nd attempt.
	//   2. sdkctx.WithoutAutoIdempotency suppresses the key and disables
	//      unsafe retries.
	tests := []struct {
		name            string
		ctx             context.Context
		headers         map[string]string
		expectedCalls   int32
		expectedSuccess bool
	}{
		{
			name:            "auto idempotency enables unsafe retry",
			ctx:             context.Background(),
			headers:         nil,
			expectedCalls:   2,
			expectedSuccess: true,
		},
		{
			name:            "suppressed auto idempotency disables unsafe retry",
			ctx:             sdkctx.WithoutAutoIdempotency(context.Background()),
			headers:         nil,
			expectedCalls:   1,
			expectedSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32

			var generatedKey string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.expectedSuccess {
					key := r.Header.Get("X-Idempotency")
					assert.NotEmpty(t, key)

					if generatedKey == "" {
						generatedKey = key
					} else {
						assert.Equal(t, generatedKey, key)
					}
				}

				if calls.Add(1) == 1 {
					w.WriteHeader(http.StatusInternalServerError)

					_, err := w.Write([]byte(`{"error":"temporary"}`))
					assert.NoError(t, err)

					return
				}

				w.Header().Set("Content-Type", "application/json")

				_, err := w.Write([]byte(`{}`))
				assert.NoError(t, err)
			}))
			defer srv.Close()

			c := NewHTTPClient(srv.Client(), "", nil)
			require.NoError(t, c.WithRetryOptions(
				retry.WithMaxRetries(1),
				retry.WithInitialDelay(time.Millisecond),
				retry.WithMaxDelay(time.Millisecond),
			))

			var out map[string]any

			err := c.doRequest(tt.ctx, http.MethodPost, srv.URL, tt.headers, map[string]string{"ok": "true"}, &out)

			if tt.expectedSuccess {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}

			assert.Equal(t, tt.expectedCalls, calls.Load())
		})
	}
}

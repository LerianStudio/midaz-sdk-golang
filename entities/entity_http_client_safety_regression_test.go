package entities

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/auth"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
	"github.com/stretchr/testify/require"
)

type typedNilConfig struct{}

func (*typedNilConfig) GetHTTPClient() *http.Client                      { return nil }
func (*typedNilConfig) GetBaseURLs() map[string]string                   { return nil }
func (*typedNilConfig) GetObservabilityProvider() observability.Provider { return nil }
func (*typedNilConfig) GetPluginAuth() auth.AccessManager                { return auth.AccessManager{} }

func nilContext() context.Context { return nil }

func requireHandlerNoError(t *testing.T, errs <-chan error) {
	t.Helper()

	select {
	case err := <-errs:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("handler did not report result")
	}
}

// Note: nil-option handling for the public midaz.New() entry point is exercised
// by validation_contract_test.go in the root package; the entities-level helper
// constructors used to live here but were removed in v3 (Batch 1E).

func TestEntityContextHelpers_WithNilContext_AreSafe(t *testing.T) {
	ctx := sdkctx.WithIdempotencyKey(nilContext(), "idem-1")
	require.NotNil(t, ctx)
	require.Equal(t, "idem-1", getIdempotencyKeyFromContext(ctx))
}

func TestNewEntityWithConfig_WithTypedNilConfig_ReturnsError(t *testing.T) {
	var cfg *typedNilConfig

	_, err := NewEntityWithConfig(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "config cannot be nil")
}

func TestZeroValueEntityExportedMethods_AreSafe(t *testing.T) {
	var e Entity

	require.NotPanics(t, func() { e.InitServices() })
	require.Nil(t, e.GetEntityHTTPClient())
	require.Nil(t, e.GetHTTPClient())
	require.Nil(t, e.GetObservabilityProvider())
	require.NotPanics(t, func() { e.SetHTTPClient(&http.Client{Timeout: time.Second}) })
}

// Default-CRM-to-onboarding behavior is covered in entity_test.go via
// TestNormalizeBaseURLs_DefaultsMissingCRMURLToOnboarding (Batch 1E refactor).

func TestEntityURLs_NormalizeV1AndRejectUnsafeDirectURLs(t *testing.T) {
	normalized, err := normalizeBaseURLs(map[string]string{
		"onboarding":  "http://localhost:3002///",
		"transaction": "http://localhost:3002/api",
		"crm":         "http://localhost:4003",
	}, false)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:3002/v1", normalized["onboarding"])
	require.Equal(t, "http://localhost:3002/api/v1", normalized["transaction"])
	require.Equal(t, "http://localhost:4003/v1", normalized["crm"])

	_, err = normalizeServiceURL("https://user:pass@example.com", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "user information")

	_, err = normalizeServiceURL("http://api.example.com", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insecure HTTP")
}

func TestEntitySetHTTPClient_PreservesProtocolConfiguration(t *testing.T) {
	entity := newTestEntity(t, &http.Client{Timeout: 30 * time.Second}, "", map[string]string{
		"onboarding":  "http://localhost:3002",
		"transaction": "http://localhost:3002",
		"crm":         "http://localhost:3002",
	}, nil)
	entity.GetEntityHTTPClient().SetDebug(true)
	entity.GetEntityHTTPClient().SetUserAgent("entity-http-client-agent")
	entity.GetEntityHTTPClient().SetEnableIdempotency(false)
	entity.GetEntityHTTPClient().SetCustomRetryPolicy(func(*http.Response, error) bool { return true })
	require.NoError(t, entity.GetEntityHTTPClient().WithRetryOptions(retry.WithMaxRetries(7)))

	entity.SetHTTPClient(&http.Client{Timeout: 2 * time.Second})

	hc := entity.GetEntityHTTPClient()
	require.Equal(t, "entity-http-client-agent", hc.loadUserAgent())
	require.True(t, hc.debug.Load())
	require.False(t, hc.enableIdempotency.Load())
	require.NotNil(t, hc.customRetryPolicy)
	require.Equal(t, 7, hc.retryOptions.MaxRetries)
}

// TestHTTPClient_DebugErrorPathRedactsURL captures debug output via a
// *slog.Logger that writes to a bytes.Buffer. v3 contract: debug output
// flows through the configured *slog.Logger; the v2 WithDebugWriter
// stderr-bypass is gone. Constructing a per-test slog handler avoids
// racing against os.Stderr (which any parallel test could also touch).
func TestHTTPClient_DebugErrorPathRedactsURL(t *testing.T) {
	writeErrs := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"message":"bad"}`))
		writeErrs <- err
	}))
	defer srv.Close()

	var debugBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&debugBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := NewHTTPClient(srv.Client(), "", nil)
	c.SetDebug(true)
	c.SetLogger(logger)

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"?document=12345678900&limit=10", nil, nil, &out)
	require.Error(t, err)
	requireHandlerNoError(t, writeErrs)

	require.NotContains(t, debugBuf.String(), "12345678900")
	require.NotContains(t, debugBuf.String(), "limit=10")
}

func TestHTTPClient_ResponseBodyLimit(t *testing.T) {
	// 5xx bodies are now capped at the tighter maxExposed5xxBodyBytes
	// (4 KiB) rather than the legacy unconditional 64 KiB. Rationale
	// lives next to the constant in entities/http.go: 5xx payloads
	// historically carry server diagnostics (stack traces, SQL) that
	// the regex redactor can miss, so the smaller exposure window is
	// defense in depth. 4xx bodies (validation envelopes the caller
	// must inspect) keep the 64 KiB cap — see the sibling test below.
	writeErrs := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write(bytes.Repeat([]byte("x"), int(maxHTTPResponseBodyBytes+1)))
		writeErrs <- err
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)
	c.SetExposeErrorBody(true)
	require.NoError(t, c.WithRetryOptions(retry.WithMaxRetries(0)))

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
	require.Error(t, err)
	requireHandlerNoError(t, writeErrs)

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	require.Equal(t, http.StatusInternalServerError, sdkErr.StatusCode)
	require.True(t, sdkErr.IsUpstreamBodyTruncated())
	require.Len(t, sdkErr.GetUpstreamBody(), maxExposed5xxBodyBytes)
	require.Contains(t, err.Error(), "upstream body (truncated")
}

// TestHTTPClient_ResponseBodyLimit_4xxKeepsGenerousCap pins the 4xx side
// of the differentiated exposure caps: validation envelopes need to be
// inspectable, so they keep the 64 KiB ceiling.
func TestHTTPClient_ResponseBodyLimit_4xxKeepsGenerousCap(t *testing.T) {
	writeErrs := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write(bytes.Repeat([]byte("x"), int(maxHTTPResponseBodyBytes+1)))
		writeErrs <- err
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)
	c.SetExposeErrorBody(true)
	require.NoError(t, c.WithRetryOptions(retry.WithMaxRetries(0)))

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
	require.Error(t, err)
	requireHandlerNoError(t, writeErrs)

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	require.Equal(t, http.StatusBadRequest, sdkErr.StatusCode)
	require.True(t, sdkErr.IsUpstreamBodyTruncated())
	require.Len(t, sdkErr.GetUpstreamBody(), maxExposed4xxBodyBytes)
	require.Contains(t, err.Error(), "upstream body (truncated")
}

func TestHTTPClient_ResponseBodyLimitStillFailsForSuccessfulResponses(t *testing.T) {
	writeErrs := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)

		_, err := w.Write(bytes.Repeat([]byte("x"), int(maxHTTPResponseBodyBytes+1)))
		writeErrs <- err
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
	require.Error(t, err)
	requireHandlerNoError(t, writeErrs)
	require.Contains(t, err.Error(), "response body exceeds")
}

func TestHTTPClient_AutoIdempotencyForAllUnsafeMethods(t *testing.T) {
	unsafeMethods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, method := range unsafeMethods {
		t.Run(method, func(t *testing.T) {
			var seen string

			writeErrs := make(chan error, 1)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = r.Header.Get("X-Idempotency")
				w.Header().Set("Content-Type", "application/json")

				_, err := w.Write([]byte(`{}`))
				writeErrs <- err
			}))
			defer srv.Close()

			c := NewHTTPClient(srv.Client(), "", nil)

			var out map[string]any

			err := c.doRequest(context.Background(), method, srv.URL, nil, map[string]string{"ok": "true"}, &out)
			require.NoError(t, err)
			requireHandlerNoError(t, writeErrs)
			require.NotEmpty(t, seen)
		})
	}
}

func TestHTTPClient_CustomRetryPolicyCanForceRetryForNonDefaultStatus(t *testing.T) {
	var calls atomic.Int32

	writeErrs := make(chan error, 2)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusUnprocessableEntity)

			_, err := w.Write([]byte(`{"message":"try again"}`))
			writeErrs <- err

			return
		}

		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{}`))
		writeErrs <- err
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)
	require.NoError(t, c.WithRetryOptions(retry.WithMaxRetries(1), retry.WithInitialDelay(time.Millisecond), retry.WithMaxDelay(time.Millisecond)))
	c.SetCustomRetryPolicy(func(resp *http.Response, _ error) bool {
		return resp != nil && resp.StatusCode == http.StatusUnprocessableEntity
	})

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
	require.NoError(t, err)
	requireHandlerNoError(t, writeErrs)
	requireHandlerNoError(t, writeErrs)
	require.Equal(t, int32(2), calls.Load())
}

func TestHTTPClient_AccessManagerTokenInvalidatedAndRefetchedOnceOnUnauthorized(t *testing.T) {
	var calls atomic.Int32

	// Keep buffers above expected calls so unexpected retries fail assertions,
	// not by blocking the handler goroutine.
	authHeaders := make(chan string, 8)
	writeErrs := make(chan error, 8)
	c := NewHTTPClient(http.DefaultClient, "expired", nil)
	c.setAuthTokenProvider(func(context.Context) (string, error) { return "fresh", nil }, func() {})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)

		authHeaders <- r.Header.Get("Authorization")

		if call == 1 {
			w.WriteHeader(http.StatusUnauthorized)

			_, err := w.Write([]byte(`{"message":"unauthorized"}`))
			writeErrs <- err

			return
		}

		w.Header().Set("Content-Type", "application/json")

		writeErrs <- json.NewEncoder(w).Encode(models.Organization{ID: "org-1"})
	}))
	defer srv.Close()

	c.client = srv.Client()

	var out models.Organization

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
	require.NoError(t, err)
	require.Equal(t, "Bearer expired", <-authHeaders)
	require.Equal(t, "Bearer fresh", <-authHeaders)
	requireHandlerNoError(t, writeErrs)
	requireHandlerNoError(t, writeErrs)
	require.Equal(t, int32(2), calls.Load())
}

// TestHTTPClient_AuthRefreshIndependentOfMaxRetries pins the architectural
// guarantee that 401 → token-refresh → one-shot re-execution is independent
// of the generic transient-failure retry mechanism.
//
// Background: callers (e.g. plugin-br-bank-transfer) wire their own
// adapter-level retry layer and configure the SDK with WithoutRetries()
// (MaxRetries=0). Before this guarantee was enforced, the SDK coupled
// auth-refresh-on-401 to the generic retry loop: when MaxRetries=0 the loop
// fetched a new token, mutated the request header, then returned the
// pre-refresh 401 to the caller without ever re-executing the request.
//
// The desired behaviour: MaxRetries governs only transient-failure retries.
// One-shot auth-refresh-retry fires whenever a tokenProvider is wired and
// refresh succeeds, regardless of MaxRetries. The existing refreshedAuth
// flag guarantees no infinite loop.
func TestHTTPClient_AuthRefreshRetryIndependentOfMaxRetries(t *testing.T) {
	var calls atomic.Int32

	authHeaders := make(chan string, 8)
	writeErrs := make(chan error, 8)

	c := NewHTTPClient(http.DefaultClient, "expired", nil)
	c.setAuthTokenProvider(func(context.Context) (string, error) { return "fresh", nil }, func() {})

	// Pin MaxRetries to 0 — the documented effect of midaz.WithoutRetries().
	// The one-shot auth-refresh must still recover the 401.
	require.NoError(t, c.WithRetryOptions(retry.WithMaxRetries(0)))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)

		authHeaders <- r.Header.Get("Authorization")

		if call == 1 {
			w.WriteHeader(http.StatusUnauthorized)

			_, err := w.Write([]byte(`{"message":"unauthorized"}`))
			writeErrs <- err

			return
		}

		w.Header().Set("Content-Type", "application/json")

		writeErrs <- json.NewEncoder(w).Encode(models.Organization{ID: "org-1"})
	}))
	defer srv.Close()

	c.client = srv.Client()

	var out models.Organization

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
	require.NoError(t, err)
	require.Equal(t, "org-1", out.ID)
	require.Equal(t, "Bearer expired", <-authHeaders)
	require.Equal(t, "Bearer fresh", <-authHeaders)
	requireHandlerNoError(t, writeErrs)
	requireHandlerNoError(t, writeErrs)
	require.Equal(t, int32(2), calls.Load())
}

func TestHTTPClient_AuthRefreshStopsAfterSecondUnauthorized(t *testing.T) {
	var calls atomic.Int32
	var refreshes atomic.Int32

	authHeaders := make(chan string, 2)
	writeErrs := make(chan error, 2)

	c := NewHTTPClient(http.DefaultClient, "expired", nil)
	c.setAuthTokenProvider(func(context.Context) (string, error) {
		refreshes.Add(1)
		return "fresh", nil
	}, func() {})
	require.NoError(t, c.WithRetryOptions(retry.WithMaxRetries(0)))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		authHeaders <- r.Header.Get("Authorization")
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{"message":"unauthorized"}`))
		writeErrs <- err
	}))
	defer srv.Close()

	c.client = srv.Client()

	var out models.Organization
	err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)

	require.Error(t, err)
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	require.Equal(t, sdkerrors.CategoryAuthentication, sdkErr.Category)
	require.Equal(t, http.StatusUnauthorized, sdkErr.GetStatusCode())
	require.Equal(t, sdkerrors.StatusCodeSourceUpstream, sdkErr.GetStatusCodeSource())
	require.Equal(t, "Bearer expired", <-authHeaders)
	require.Equal(t, "Bearer fresh", <-authHeaders)
	requireHandlerNoError(t, writeErrs)
	requireHandlerNoError(t, writeErrs)
	require.Equal(t, int32(2), calls.Load(), "second 401 must not trigger an infinite auth-refresh loop")
	require.Equal(t, int32(1), refreshes.Load(), "token provider must be called only once per request")
}

func TestHTTPClient_RetryableHTTPCodesOverridesTypedNonRetryableStatus(t *testing.T) {
	var calls atomic.Int32
	writeErrs := make(chan error, 2)

	c := NewHTTPClient(http.DefaultClient, "", nil)
	require.NoError(t, c.WithRetryOptions(
		retry.WithMaxRetries(1),
		retry.WithRetryableHTTPCodes([]int{http.StatusConflict}),
		retry.WithInitialDelay(time.Millisecond),
		retry.WithMaxDelay(time.Millisecond),
	))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusConflict)
			_, err := w.Write([]byte(`{"message":"transient conflict"}`))
			writeErrs <- err
			return
		}

		w.Header().Set("Content-Type", "application/json")
		writeErrs <- json.NewEncoder(w).Encode(models.Organization{ID: "org-1"})
	}))
	defer srv.Close()

	c.client = srv.Client()

	var out models.Organization
	err := c.doRequest(
		context.Background(),
		http.MethodPost,
		srv.URL,
		map[string]string{"X-Idempotency": "custom-retry-409"},
		map[string]string{"ok": "true"},
		&out,
	)

	require.NoError(t, err)
	require.Equal(t, "org-1", out.ID)
	requireHandlerNoError(t, writeErrs)
	requireHandlerNoError(t, writeErrs)
	require.Equal(t, int32(2), calls.Load())
}

func TestHTTPClient_UserRetryPredicateComposesWithInternalPredicate(t *testing.T) {
	var calls atomic.Int32
	writeErrs := make(chan error, 2)

	c := NewHTTPClient(http.DefaultClient, "", nil)
	require.NoError(t, c.WithRetryOptions(
		retry.WithMaxRetries(1),
		retry.WithInitialDelay(time.Millisecond),
		retry.WithMaxDelay(time.Millisecond),
		retry.WithErrorPredicate(func(err error) bool {
			var statusErr interface{ StatusCode() int }
			return stderrors.As(err, &statusErr) && statusErr.StatusCode() == http.StatusUnprocessableEntity
		}),
	))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, err := w.Write([]byte(`{"message":"retry via custom predicate"}`))
			writeErrs <- err
			return
		}

		w.Header().Set("Content-Type", "application/json")
		writeErrs <- json.NewEncoder(w).Encode(models.Organization{ID: "org-1"})
	}))
	defer srv.Close()

	c.client = srv.Client()

	var out models.Organization
	err := c.doRequest(
		context.Background(),
		http.MethodPost,
		srv.URL,
		map[string]string{"X-Idempotency": "custom-predicate-422"},
		map[string]string{"ok": "true"},
		&out,
	)

	require.NoError(t, err)
	require.Equal(t, "org-1", out.ID)
	requireHandlerNoError(t, writeErrs)
	requireHandlerNoError(t, writeErrs)
	require.Equal(t, int32(2), calls.Load())
}

// TestHTTPClient_AuthRefreshRetryIndependentOfMaxRetries_UnsafePOSTNoIdempotency
// pins the precise contract guarded by commit 400652e: an unsafe POST
// without a caller-supplied idempotency key, with MaxRetries=0, and with
// auto-idempotency turned OFF, still recovers ONCE from a 401 when a
// tokenProvider is wired.
//
// This is the trickiest corner because executeRequestWithRetry coerces
// effectiveRetryOptions.MaxRetries=0 for unsafe-no-key methods to prevent
// blind retries on a non-idempotent call. The auth-refresh-retry loop
// must remain orthogonal — driven by execution.refreshedAuth, not by the
// retry budget.
func TestHTTPClient_AuthRefreshRetryIndependentOfMaxRetries_UnsafePOSTNoIdempotency(t *testing.T) {
	var calls atomic.Int32

	authHeaders := make(chan string, 8)
	writeErrs := make(chan error, 8)

	c := NewHTTPClient(http.DefaultClient, "expired", nil)
	c.setAuthTokenProvider(func(context.Context) (string, error) { return "fresh", nil }, func() {})
	c.SetEnableIdempotency(false) // turn off auto-idempotency so X-Idempotency is absent on the POST

	require.NoError(t, c.WithRetryOptions(retry.WithMaxRetries(0)))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)

		authHeaders <- r.Header.Get("Authorization")

		// The contract: unsafe POST + no idempotency key MUST NOT have
		// X-Idempotency on the wire (the client coerced retries off and
		// auto-idempotency is disabled).
		if r.Header.Get("X-Idempotency") != "" {
			t.Errorf("unexpected X-Idempotency header on unsafe-no-key POST: %q", r.Header.Get("X-Idempotency"))
		}

		if call == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte(`{"message":"unauthorized"}`))
			writeErrs <- err
			return
		}

		w.Header().Set("Content-Type", "application/json")
		writeErrs <- json.NewEncoder(w).Encode(models.Organization{ID: "org-1"})
	}))
	defer srv.Close()

	c.client = srv.Client()

	var out models.Organization

	err := c.doRequest(context.Background(), http.MethodPost, srv.URL, nil, map[string]string{"name": "Acme"}, &out)
	require.NoError(t, err)
	require.Equal(t, "org-1", out.ID)
	require.Equal(t, "Bearer expired", <-authHeaders)
	require.Equal(t, "Bearer fresh", <-authHeaders)
	requireHandlerNoError(t, writeErrs)
	requireHandlerNoError(t, writeErrs)
	require.Equal(t, int32(2), calls.Load())
}

// TestHTTPClient_AuthRefreshFailedLogsWarn verifies that the auth-refresh
// "failed" branch lands a Warn-level structured log. We wire a
// tokenProvider that errors out and assert the log captures the failure
// state alongside a redacted error.
func TestHTTPClient_AuthRefreshFailedLogsWarn(t *testing.T) {
	var logs bytes.Buffer
	writeErrs := make(chan error, 4)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-ID", "req-fail")
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{"message":"unauthorized"}`))
		writeErrs <- err
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "expired", nil)
	// Capture Warn-and-above only — the "failed" branch must emit at Warn.
	c.SetLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	c.setAuthTokenProvider(func(context.Context) (string, error) {
		return "", &refreshFailureError{cause: "token=secret-token-value invalid"}
	}, nil)
	require.NoError(t, c.WithRetryOptions(retry.WithMaxRetries(0)))

	var out map[string]any
	err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/v1/accounts", nil, nil, &out)
	require.Error(t, err)

	logText := logs.String()
	require.Contains(t, logText, "token refresh failed")
	require.Contains(t, logText, "auth_refresh.state=failed")
	// Embedded credential value must be redacted in the log line.
	require.NotContains(t, logText, "secret-token-value")
	requireHandlerNoError(t, writeErrs)
}

// refreshFailureError is a test-only typed error used to drive the
// tokenProvider-fail branch.
type refreshFailureError struct {
	cause string
}

func (e *refreshFailureError) Error() string { return "token refresh: " + e.cause }

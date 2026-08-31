package entities

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry"
	"github.com/stretchr/testify/require"
)

type diagnosticStringer string

func (s diagnosticStringer) String() string { return string(s) }

type diagnosticTypedNilError struct{ message string }

func (e *diagnosticTypedNilError) Error() string { return e.message }

type diagnosticTypedNilStringer struct{ message string }

func (s *diagnosticTypedNilStringer) String() string { return s.message }

func TestHTTPDiagnostics_RedactsURLQueryUserinfoPathIDsHeadersAndLogArgs(t *testing.T) {
	redactedURL := redactDebugURL("https://user:pass@api.example.test/v1/accounts?token=tok_123&access_token=acc_123&refresh_token=ref_123&api_key=key_123&x-api-key=xkey_123&client_secret=sec_123&authorization=Bearer+abc&password=pwd_123&jwt=jwt_123&limit=10")

	for _, secret := range []string{"user:pass", "tok_123", "acc_123", "ref_123", "key_123", "xkey_123", "sec_123", "Bearer+abc", "pwd_123", "jwt_123"} {
		require.NotContains(t, redactedURL, secret)
	}
	require.NotContains(t, redactedURL, "limit=10")
	require.NotContains(t, redactedURL, "?")

	host, path := safeURLHostPath("https://user:pass@api.example.test:8443/v1/accounts/550e8400-e29b-41d4-a716-446655440000/balances/12345?token=tok_123#frag")
	require.Equal(t, "api.example.test:8443", host)
	require.Equal(t, "/v1/accounts/:id/balances/:id", path)
	require.NotContains(t, host, "user")
	require.NotContains(t, path, "550e8400")
	require.NotContains(t, path, "12345")

	headers := redactHeaders(http.Header{
		"Authorization": []string{"Bearer token-value"},
		"Cookie":        []string{"session=secret"},
		"Set-Cookie":    []string{"session=secret"},
		"X-Idempotency": []string{"idem-secret"},
		"X-API-Key":     []string{"api-secret"},
		"Client-Secret": []string{"client-secret"},
		"X-Request-ID":  []string{"req-123"},
		"Content-Type":  []string{"application/json"},
	})

	require.Equal(t, []string{"[REDACTED]"}, headers["Authorization"])
	require.Equal(t, []string{"[REDACTED]"}, headers["Cookie"])
	require.Equal(t, []string{"[REDACTED]"}, headers["Set-Cookie"])
	require.Equal(t, []string{"[REDACTED]"}, headers["X-Idempotency"])
	require.Equal(t, []string{"[REDACTED]"}, headers["X-API-Key"])
	require.Equal(t, []string{"[REDACTED]"}, headers["Client-Secret"])
	require.Equal(t, []string{"req-123"}, headers["X-Request-ID"])
	require.Equal(t, []string{"application/json"}, headers["Content-Type"])

	sanitized := sanitizeLogArgs([]any{
		"token=tok_123\nnext",
		errors.New("password=pwd_123\rnext"),
		diagnosticStringer("client_secret=sec_123\tnext"),
	})

	require.NotContains(t, fmt.Sprint(sanitized...), "tok_123")
	require.NotContains(t, fmt.Sprint(sanitized...), "pwd_123")
	require.NotContains(t, fmt.Sprint(sanitized...), "sec_123")
	require.Contains(t, sanitized[0], `\n`)
	require.Contains(t, sanitized[1], `\r`)
	require.Contains(t, sanitized[2], `\t`)
}

func TestHTTPDiagnostics_TypedNilErrorAndStringerAreSafe(t *testing.T) {
	var typedNilErr *diagnosticTypedNilError
	var typedNilStringer *diagnosticTypedNilStringer

	require.Empty(t, safeLogError(typedNilErr))
	require.NotPanics(t, func() {
		sanitized := sanitizeLogArgs([]any{typedNilErr, typedNilStringer})
		require.Equal(t, []any{"", ""}, sanitized)
	})
}

func TestHTTPDiagnostics_LocalBuildFailureLogsPhaseAndSentFlag(t *testing.T) {
	var logs bytes.Buffer
	c := NewHTTPClient(nil, "", nil)
	c.SetLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))

	var out map[string]any
	err := c.doRequest(context.Background(), http.MethodGet, "://bad?token=tok_123", nil, nil, &out)
	require.Error(t, err)

	logText := logs.String()
	require.Contains(t, logText, "HTTP request phase failed")
	require.Contains(t, logText, "phase=request_build")
	require.Contains(t, logText, "http.request_sent=false")
	require.NotContains(t, logText, "tok_123")
}

func TestHTTPDiagnostics_AuthRefreshReplayLogsSafeMetadata(t *testing.T) {
	var logs bytes.Buffer
	var mu sync.Mutex
	observedAuthHeaders := make([]string, 0, 2)
	writeErrors := make([]error, 0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		observedAuthHeaders = append(observedAuthHeaders, r.Header.Get("Authorization"))
		requestCount := len(observedAuthHeaders)
		mu.Unlock()

		w.Header().Set("X-Request-ID", "req-refresh")

		if requestCount == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte(`{"message":"expired"}`)); err != nil {
				mu.Lock()
				writeErrors = append(writeErrors, err)
				mu.Unlock()
			}

			return
		}

		if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
			mu.Lock()
			writeErrors = append(writeErrors, err)
			mu.Unlock()
		}
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "old-token", nil)
	c.SetLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	c.setAuthTokenProvider(func(context.Context) (string, error) {
		return "new-token", nil
	}, nil)
	require.NoError(t, c.WithRetryOptions(retry.WithMaxRetries(0)))

	var out map[string]any
	err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/v1/accounts?access_token=secret", nil, nil, &out)
	require.NoError(t, err)

	mu.Lock()
	observedAuthHeadersCopy := append([]string(nil), observedAuthHeaders...)
	writeErrorsCopy := append([]error(nil), writeErrors...)
	mu.Unlock()

	require.Empty(t, writeErrorsCopy)
	require.Equal(t, []string{"Bearer old-token", "Bearer new-token"}, observedAuthHeadersCopy)

	logText := logs.String()
	require.Contains(t, logText, "token refresh started")
	require.Contains(t, logText, "token refresh succeeded")
	require.Contains(t, logText, "url.host=")
	require.Contains(t, logText, "url.path=/v1/accounts")
	require.Contains(t, logText, "http.status_code=401")
	require.Contains(t, logText, "request_id=req-refresh")
	require.NotContains(t, logText, "old-token")
	require.NotContains(t, logText, "new-token")
	require.NotContains(t, logText, "secret")
}

func TestHTTPDiagnostics_HTTPResponseErrorLogRetainsRequestContext(t *testing.T) {
	ctxKey := diagnosticContextKey("request-context")
	handler := &contextCapturingSlogHandler{key: ctxKey}
	var writeErr error
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"message":"unavailable"}`))
		if err != nil {
			mu.Lock()
			writeErr = err
			mu.Unlock()
		}
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)
	c.SetLogger(slog.New(handler))
	require.NoError(t, c.WithRetryOptions(retry.WithMaxRetries(0)))

	ctx := context.WithValue(context.Background(), ctxKey, "retained")
	var out map[string]any
	err := c.doRequest(ctx, http.MethodGet, srv.URL+"/v1/accounts", nil, nil, &out)
	require.Error(t, err)
	mu.Lock()
	observedWriteErr := writeErr
	mu.Unlock()
	require.NoError(t, observedWriteErr)
	require.Contains(t, handler.values(), "retained")
}

type diagnosticContextKey string

type contextCapturingSlogHandler struct {
	mu   sync.Mutex
	key  diagnosticContextKey
	seen []string
}

func (*contextCapturingSlogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *contextCapturingSlogHandler) Handle(ctx context.Context, _ slog.Record) error {
	if value, ok := ctx.Value(h.key).(string); ok {
		h.mu.Lock()
		h.seen = append(h.seen, value)
		h.mu.Unlock()
	}

	return nil
}

func (h *contextCapturingSlogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *contextCapturingSlogHandler) WithGroup(string) slog.Handler { return h }

func (h *contextCapturingSlogHandler) values() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]string(nil), h.seen...)
}

func TestHTTPDiagnostics_RetryExhaustionLogsSafeMetadata(t *testing.T) {
	var logs bytes.Buffer
	var mu sync.Mutex
	requestCount := 0
	writeErrors := make([]error, 0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("X-Request-ID", "req-retry")
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err := w.Write([]byte(`{"message":"unavailable"}`)); err != nil {
			mu.Lock()
			writeErrors = append(writeErrors, err)
			mu.Unlock()
		}
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)
	c.SetLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	require.NoError(t, c.WithRetryOptions(
		retry.WithMaxRetries(1),
		retry.WithInitialDelay(time.Millisecond),
		retry.WithMaxDelay(time.Millisecond),
		retry.WithJitterFactor(0),
	))

	var out map[string]any
	err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/v1/accounts?api_key=secret", nil, nil, &out)
	require.Error(t, err)

	mu.Lock()
	observedRequestCount := requestCount
	writeErrorsCopy := append([]error(nil), writeErrors...)
	mu.Unlock()

	require.Empty(t, writeErrorsCopy)
	require.Equal(t, 2, observedRequestCount)

	logText := logs.String()
	require.Contains(t, logText, "retry exhausted")
	require.Contains(t, logText, "attempts=2")
	require.Contains(t, logText, "max_retries=1")
	require.Contains(t, logText, "url.host=")
	require.Contains(t, logText, "url.path=/v1/accounts")
	require.Contains(t, logText, "http.status_code=503")
	require.Contains(t, logText, "request_id=req-retry")
	require.NotContains(t, logText, "secret")
}

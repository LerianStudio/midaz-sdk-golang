package retry

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWarnLegacyIdempotencyKeyEmitsOnce(t *testing.T) {
	t.Cleanup(func() {
		legacyIdempotencyKeyWarningOnce = sync.Once{}
	})

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	legacyIdempotencyKeyWarningOnce = sync.Once{}
	canonical := newIdempotencyHeaderRequest(t)
	canonical.Header.Set("X-Idempotency", "canonical-key")
	canonical.Header.Set("Idempotency-Key", "legacy-key")
	require.True(t, hasIdempotencyHeader(canonical, logger))
	assert.Empty(t, logs.String())

	legacyIdempotencyKeyWarningOnce = sync.Once{}
	legacy := newIdempotencyHeaderRequest(t)
	legacy.Header.Set("Idempotency-Key", "legacy-key")
	require.False(t, hasIdempotencyHeader(legacy, logger))
	require.False(t, hasIdempotencyHeader(legacy, logger))

	assert.Equal(t, 1, strings.Count(logs.String(), "retry: request carries Idempotency-Key"))
}

func TestDoHTTPRequestWithLoggerWarnsOnceForLegacyIdempotencyKey(t *testing.T) {
	t.Cleanup(func() {
		legacyIdempotencyKeyWarningOnce = sync.Once{}
	})

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	legacyIdempotencyKeyWarningOnce = sync.Once{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Idempotency-Key", "legacy-key")

	resp, err := DoHTTPRequest(context.Background(), server.Client(), req, WithHTTPLogger(logger), WithHTTPMaxRetries(1))
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 0, resp.Attempt)
	assert.Equal(t, 1, strings.Count(logs.String(), "retry: request carries Idempotency-Key"))

	legacyIdempotencyKeyWarningOnce = sync.Once{}
	logs.Reset()
	req, err = http.NewRequest(http.MethodPost, server.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Idempotency-Key", "legacy-key")
	_, _ = DoHTTPRequest(context.Background(), server.Client(), req, WithHTTPMaxRetries(1))
	assert.Empty(t, logs.String())
}

func TestWarnLegacyIdempotencyKeyDoesNotUseGlobalLogger(t *testing.T) {
	previousLogger := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
		legacyIdempotencyKeyWarningOnce = sync.Once{}
	})

	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	legacyIdempotencyKeyWarningOnce = sync.Once{}

	legacy := newIdempotencyHeaderRequest(t)
	legacy.Header.Set("Idempotency-Key", "legacy-key")
	require.False(t, hasIdempotencyHeader(legacy, nil))

	assert.Empty(t, logs.String())
}

func newIdempotencyHeaderRequest(t *testing.T) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, "https://api.example.com/v1/transactions", nil)
	require.NoError(t, err)

	return req
}

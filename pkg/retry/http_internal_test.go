package retry

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWarnLegacyIdempotencyKeyEmitsOnce(t *testing.T) {
	previousLogger := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
		legacyIdempotencyKeyWarningOnce = sync.Once{}
	})

	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))

	legacyIdempotencyKeyWarningOnce = sync.Once{}
	canonical := newIdempotencyHeaderRequest(t)
	canonical.Header.Set("X-Idempotency", "canonical-key")
	canonical.Header.Set("Idempotency-Key", "legacy-key")
	require.True(t, hasIdempotencyHeader(canonical))
	assert.Empty(t, logs.String())

	legacyIdempotencyKeyWarningOnce = sync.Once{}
	legacy := newIdempotencyHeaderRequest(t)
	legacy.Header.Set("Idempotency-Key", "legacy-key")
	require.False(t, hasIdempotencyHeader(legacy))
	require.False(t, hasIdempotencyHeader(legacy))

	assert.Equal(t, 1, strings.Count(logs.String(), "retry: request carries Idempotency-Key"))
}

func newIdempotencyHeaderRequest(t *testing.T) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, "https://api.example.com/v1/transactions", nil)
	require.NoError(t, err)

	return req
}

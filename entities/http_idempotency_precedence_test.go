package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
)

// TestIdempotencyPrecedence_InputKeyBeatsCtxKey verifies the documented
// precedence rule: when a caller supplies BOTH an input-level
// IdempotencyKey via X-Idempotency in the headers map AND a ctx-supplied
// key via sdkctx.WithIdempotencyKey, the input-level value wins.
//
// Bug guard: prior to this fix, injectContextHeaders unconditionally
// overwrote X-Idempotency with the ctx key, silently dropping the
// caller's input-level key — a dedup-vs-double-bookkeeping hazard for a
// ledger SDK.
func TestIdempotencyPrecedence_InputKeyBeatsCtxKey(t *testing.T) {
	seenCh := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCh <- r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "ctx-key")

	// Mirror the call shape used by transactionsEntity.sendCreateTransactionRequest:
	// the input-level key is plumbed in by setting X-Idempotency directly.
	headers := map[string]string{
		"X-Idempotency": "input-key",
	}

	var out map[string]any

	err := c.doRequest(ctx, http.MethodPost, srv.URL, headers, map[string]string{"ok": "true"}, &out)
	require.NoError(t, err)

	seen := <-seenCh
	assert.Equal(t, "input-key", seen, "input-level IdempotencyKey must win over ctx-supplied key")
}

// TestIdempotencyPrecedence_CtxKeyUsedWhenInputAbsent verifies that when
// only the ctx-supplied key is set (no input-level header / marker), the
// ctx key is the one emitted on the wire.
func TestIdempotencyPrecedence_CtxKeyUsedWhenInputAbsent(t *testing.T) {
	seenCh := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCh <- r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "ctx-key")

	var out map[string]any

	err := c.doRequest(ctx, http.MethodPost, srv.URL, nil, map[string]string{"ok": "true"}, &out)
	require.NoError(t, err)

	seen := <-seenCh
	assert.Equal(t, "ctx-key", seen, "ctx key must be emitted when no input-level key is set")
}

// TestIdempotencyPrecedence_AutoGenWhenBothAbsent verifies that when
// neither an input-level header nor a ctx-supplied key is present, the
// SDK auto-generates a UUIDv4 idempotency key (the default with
// client-level idempotency enabled).
func TestIdempotencyPrecedence_AutoGenWhenBothAbsent(t *testing.T) {
	seenCh := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCh <- r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodPost, srv.URL, nil, map[string]string{"ok": "true"}, &out)
	require.NoError(t, err)

	seen := <-seenCh
	require.NotEmpty(t, seen, "auto-generated key must be emitted when neither input nor ctx set one")
	// UUIDv4 string form is 36 chars with 4 hyphens.
	assert.Len(t, seen, 36)
}

// TestIdempotencyPrecedence_NoAutoWhenWithoutAutoIdempotency verifies that
// sdkctx.WithoutAutoIdempotency suppresses auto-generation: with no input
// header and no ctx-supplied key, the request goes on the wire with an
// EMPTY X-Idempotency header.
func TestIdempotencyPrecedence_NoAutoWhenWithoutAutoIdempotency(t *testing.T) {
	seenCh := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCh <- r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)

	ctx := sdkctx.WithoutAutoIdempotency(context.Background())

	var out map[string]any

	err := c.doRequest(ctx, http.MethodPost, srv.URL, nil, map[string]string{"ok": "true"}, &out)
	require.NoError(t, err)

	seen := <-seenCh
	assert.Empty(t, seen, "WithoutAutoIdempotency must suppress auto-generation when neither input nor ctx set a key")
}

// TestIdempotencyPrecedence_DirectHeaderWithoutMarkerPreserved verifies
// the regression fix: when a caller sets X-Idempotency directly via the
// headers map (without the internal marker) and a ctx-supplied key is
// also present, the directly-set header value MUST be preserved on the
// wire. The previous implementation gated caller-supplied detection on
// the marker, so a direct header was silently overwritten by the ctx key.
//
// This also implies the request remains retryable on 5xx (the retry gate
// now keys off the on-wire X-Idempotency value, not the marker).
func TestIdempotencyPrecedence_DirectHeaderWithoutMarkerPreserved(t *testing.T) {
	seenCh := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCh <- r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "ctx-key")

	// Caller sets X-Idempotency directly via headers map — no internal
	// marker. Pre-fix this was overwritten by the ctx key.
	headers := map[string]string{
		"X-Idempotency": "direct-key",
	}

	var out map[string]any

	err := c.doRequest(ctx, http.MethodPost, srv.URL, headers, map[string]string{"ok": "true"}, &out)
	require.NoError(t, err)

	seen := <-seenCh
	assert.Equal(t, "direct-key", seen, "directly-set X-Idempotency header must be preserved over ctx-supplied key")
}

func TestIdempotencyPrecedence_DirectHeaderCaseInsensitivePreserved(t *testing.T) {
	seenCh := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCh <- r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)
	ctx := sdkctx.WithIdempotencyKey(context.Background(), "ctx-key")

	var out map[string]any
	err := c.doRequest(
		ctx,
		http.MethodPost,
		srv.URL,
		map[string]string{"x-idempotency": "lowercase-direct-key"},
		map[string]string{"ok": "true"},
		&out,
	)
	require.NoError(t, err)

	seen := <-seenCh
	assert.Equal(t, "lowercase-direct-key", seen, "idempotency header detection must be case-insensitive")
}

func TestIdempotencyPrecedence_BlankCaseVariantDoesNotEraseContextKey(t *testing.T) {
	seenCh := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCh <- r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)
	ctx := sdkctx.WithIdempotencyKey(context.Background(), "ctx-key")

	var out map[string]any
	err := c.doRequest(
		ctx,
		http.MethodPost,
		srv.URL,
		map[string]string{"x-idempotency": ""},
		map[string]string{"ok": "true"},
		&out,
	)
	require.NoError(t, err)

	seen := <-seenCh
	assert.Equal(t, "ctx-key", seen, "blank case-variant idempotency header must not overwrite the context key")
}

func TestIdempotencyPrecedence_DirectHeaderWithoutMarkerEnablesRetry(t *testing.T) {
	var calls atomic.Int32

	seen := make(chan string, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Get("X-Idempotency")
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
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
	err := c.doRequest(
		context.Background(),
		http.MethodPost,
		srv.URL,
		map[string]string{"X-Idempotency": "direct-key"},
		map[string]string{"ok": "true"},
		&out,
	)

	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load())
	assert.Equal(t, "direct-key", <-seen)
	assert.Equal(t, "direct-key", <-seen)
}

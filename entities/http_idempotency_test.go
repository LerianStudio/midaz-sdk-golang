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

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
)

func TestIdempotencyHeaderInjection(t *testing.T) {
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

	ctx := WithIdempotencyKey(context.Background(), "abc123")

	var out map[string]any

	err := c.doRequest(ctx, http.MethodGet, srv.URL, nil, nil, &out)
	require.NoError(t, err)

	assert.Equal(t, "abc123", seen)
}

func TestRedactDebugURLMasksSensitiveQueryValues(t *testing.T) {
	redacted := redactDebugURL("https://api.example.com/holders?document=12345678900&external_id=ext-1&limit=10&metadata.email=a@example.com")

	require.NotEmpty(t, redacted)
	redactedURL, err := url.Parse(redacted)
	require.NoError(t, err)

	query := redactedURL.Query()
	for key, original := range map[string]string{
		"document":       "12345678900",
		"external_id":    "ext-1",
		"metadata.email": "a@example.com",
	} {
		assert.NotEqual(t, original, query.Get(key))
		assert.NotEqual(t, url.QueryEscape(original), query.Get(key))
	}

	assert.Equal(t, "10", query.Get("limit"))
}

func TestAutomaticIdempotencyHeaderForUnsafeMethods(t *testing.T) {
	var seen, autoHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("X-Idempotency")
		autoHeader = r.Header.Get("X-Midaz-Auto-Idempotency")
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodPost, srv.URL, map[string]string{"X-Midaz-Auto-Idempotency": "true"}, map[string]string{"ok": "true"}, &out)
	require.NoError(t, err)

	assert.NotEmpty(t, seen)
	assert.Empty(t, autoHeader)
}

func TestUnsafeMethodRetriesOnlyWithIdempotency(t *testing.T) {
	tests := []struct {
		name            string
		headers         map[string]string
		expectedCalls   int32
		expectedSuccess bool
	}{
		{
			name:            "no idempotency disables retry",
			headers:         nil,
			expectedCalls:   1,
			expectedSuccess: false,
		},
		{
			name:            "generated idempotency allows retry",
			headers:         map[string]string{"X-Midaz-Auto-Idempotency": "true"},
			expectedCalls:   2,
			expectedSuccess: true,
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
			c.WithRetryOptions(
				retry.WithMaxRetries(1),
				retry.WithInitialDelay(time.Millisecond),
				retry.WithMaxDelay(time.Millisecond),
			)

			var out map[string]any

			err := c.doRequest(context.Background(), http.MethodPost, srv.URL, tt.headers, map[string]string{"ok": "true"}, &out)

			if tt.expectedSuccess {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}

			assert.Equal(t, tt.expectedCalls, calls.Load())
		})
	}
}

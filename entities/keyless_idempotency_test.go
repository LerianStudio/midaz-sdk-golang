package entities

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"
)

// A request without an integrator key leaves the SDK with no X-Idempotency, so the ledger
// dedupes a keyless transaction create by its body. The integrator key and the explicit
// opt-in keep working.
func TestKeylessIdempotencyDefaults(t *testing.T) {
	assert.False(t, config.DefaultEnableIdempotency)
	assert.False(t, configEnableIdempotency(nil), "a Config without the gate must not auto-generate")

	post := func(hc *HTTPClient, ctx context.Context) string {
		var seen string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = r.Header.Get(idempotencyHeader)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		var out map[string]any

		require.NoError(t, hc.doRequest(ctx, http.MethodPost, srv.URL, nil, map[string]string{"a": "b"}, &out))

		return seen
	}

	assert.Empty(t, post(NewHTTPClient(nil, "", nil), context.Background()), "default sends no key")
	assert.Equal(t, "mine", post(NewHTTPClient(nil, "", nil), sdkctx.WithIdempotencyKey(context.Background(), "mine")))

	optIn := NewHTTPClient(nil, "", nil)
	optIn.SetEnableIdempotency(true)
	assert.NotEmpty(t, post(optIn, context.Background()), "explicit opt-in still generates")
}

// A keyless unsafe request retries only where the ledger dedupes it by body.
func TestKeylessRetryOnlyWhereLedgerDedupesByBody(t *testing.T) {
	cases := []struct {
		path        string
		wantRetries bool
	}{
		{"/v1/organizations/o/ledgers/l/transactions/json", true},
		{"/v1/organizations/o/ledgers/l/transactions/inflow", true},
		{"/v1/organizations/o/ledgers/l/transactions/outflow", true},
		{"/v1/organizations/o/ledgers/l/transactions/annotation", true},
		{"/v1/organizations/o/ledgers/l/transactions/block", true},
		{"/v1/organizations/o/ledgers/l/transactions/unblock", true},
		{"/v2/transactions/direct", true},
		{"/v2/transactions/hold", true},
		{"/v2/transactions/block", true},
		{"/v2/transactions/unblock", true},
		{"/v2/transactions/batch", true},
		{"/v2/organizations/o/holders", true},
		{"/v2/organizations/o/holders/h/instruments", true},
		{"/v1/organizations/o/ledgers/l/accounts", false},
		{"/v2/organizations/o/ledgers/l/transactions/t/commit", false},
		{"/v2/organizations/o/ledgers/l/transactions/t/revert", false},
		{"/v2/organizations/o/ledgers/l/accounts/a/fee-debts/collect", false},
		{"/v2/organizations/o/holders/h/instruments/i", false},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			var count int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&count, 1)
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer srv.Close()

			rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), nil)
			req, _ := http.NewRequest(http.MethodPost, srv.URL+tc.path, bytes.NewReader([]byte(`{"x":1}`)))

			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			resp.Body.Close()

			if tc.wantRetries {
				assert.Greater(t, atomic.LoadInt32(&count), int32(1))
			} else {
				assert.EqualValues(t, 1, atomic.LoadInt32(&count))
			}
		})
	}

	// The legacy path applies the same rule.
	for path, wantRetries := range map[string]bool{"/v2/transactions/direct": true, "/v1/organizations/o/ledgers/l/accounts": false} {
		var count int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&count, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))

		opts := fastRetryOpts()
		hc := NewHTTPClient(nil, "", nil)
		hc.retryOptions = &opts

		var out map[string]any

		_ = hc.doRequest(context.Background(), http.MethodPost, srv.URL+path, nil, map[string]string{"a": "b"}, &out)
		srv.Close()

		assert.Equal(t, wantRetries, atomic.LoadInt32(&count) > 1, path)
	}
}

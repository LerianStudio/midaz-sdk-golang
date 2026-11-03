package entities

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	// A keyless 409/0084 is retried only on a body-deduped route.
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		writeReply(w, "0084")
	}))
	defer srv.Close()

	rt := newRetryRoundTripper(http.DefaultTransport, fastRetryOpts(), nil)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/organizations/o/ledgers/l/accounts", bytes.NewReader([]byte(`{"x":1}`)))

	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.EqualValues(t, 1, atomic.LoadInt32(&count))
}

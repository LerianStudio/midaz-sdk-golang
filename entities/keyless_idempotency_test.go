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

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"
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
}

// A keyless /v1 annotation, block or unblock create sends a key derived from the request: a resend
// reuses it, and block and unblock with one input never share a slot (the ledger's /v1 slot ignores
// the action). A keyless json create sends no key, the bare-body slot v6.4.0 used.
func TestKeylessV1CreateDerivesKeyFromRequest(t *testing.T) {
	var keys []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("X-Idempotency"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(txResponseBody()))
	}))
	defer srv.Close()

	ctx := context.Background()
	opted := newTransactionsFacade(newTestLedgerClient(t, srv), true)
	keyless := newTransactionsFacade(newTestLedgerClient(t, srv), false)

	for _, send := range []func() error{
		func() error {
			_, err := keyless.CreateBlock(ctx, txOrgID, txLedgerID, sampleTransactionInput())
			return err
		},
		func() error {
			_, err := keyless.CreateUnblock(ctx, txOrgID, txLedgerID, sampleTransactionInput())
			return err
		},
		func() error {
			_, err := keyless.CreateBlock(ctx, txOrgID, txLedgerID, sampleTransactionInput())
			return err
		},
		func() error {
			_, err := keyless.CreateJSON(ctx, txOrgID, txLedgerID, withKey(sampleTransactionInput(), "mine"))
			return err
		},
		func() error {
			_, err := opted.CreateJSON(ctx, txOrgID, txLedgerID, sampleTransactionInput())
			return err
		},
		func() error {
			_, err := keyless.CreateJSON(sdkctx.WithoutAutoIdempotency(ctx), txOrgID, txLedgerID, sampleTransactionInput())
			return err
		},
		func() error {
			_, err := keyless.CreateJSON(ctx, txOrgID, txLedgerID, sampleTransactionInput())
			return err
		},
		func() error {
			_, err := keyless.CreateUnblock(sdkctx.WithoutAutoIdempotency(ctx), txOrgID, txLedgerID, sampleTransactionInput())
			return err
		},
	} {
		require.NoError(t, send())
	}

	block, unblock, resend, own, random, suppressed, bare, suppressedUnblock := keys[0], keys[1], keys[2], keys[3], keys[4], keys[5], keys[6], keys[7]
	assert.Regexp(t, `^[0-9a-f]{64}$`, block)
	assert.NotEqual(t, block, unblock)
	assert.Equal(t, block, resend)
	assert.Equal(t, "mine", own)
	assert.NotEmpty(t, random)
	assert.NotEqual(t, block, random)
	assert.Empty(t, suppressed) // json is keyless; suppression stops only the random key
	assert.Empty(t, bare)
	assert.Regexp(t, `^[0-9a-f]{64}$`, suppressedUnblock)
	assert.NotEqual(t, block, suppressedUnblock)
}

func withKey(in *models.CreateTransactionInput, key string) *models.CreateTransactionInput {
	in.IdempotencyKey = key

	return in
}

// A /v2 keyless create still sends no key: the ledger's /v2 slot already separates actions.
func TestKeylessV2CreateSendsNoKey(t *testing.T) {
	var (
		key  string
		seen *http.Request
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		key = r.Header.Get("X-Idempotency")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, _ = newTransactionsV2Facade(newTestLedgerClient(t, srv), false).CreateDirect(context.Background(), txOrgID, txLedgerID, sampleV2Input())
	require.NotNil(t, seen)
	assert.Empty(t, key)
}

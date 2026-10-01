package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	midaz "github.com/LerianStudio/midaz-sdk-golang/v6"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/config"
)

// TestFeeModeFollowsLedger pins the pattern: boot resolves once, the refresh
// loop follows a ledger upgrade, and a /version failure falls back to legacy.
func TestFeeModeFollowsLedger(t *testing.T) {
	var body atomic.Value // string; "" answers 503

	body.Store(`{"version":"v3.8.4","requestDate":"2026-09-30T12:00:00Z"}`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		b, _ := body.Load().(string)
		if b == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		_, _ = w.Write([]byte(b))
	}))
	defer srv.Close()

	c, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
		midaz.WithAnonymous(),
		midaz.WithLedgerURL(srv.URL),
	)
	require.NoError(t, err)

	fees := newFeeMode(t.Context(), c)
	assert.Equal(t, midaz.FeeModeLegacy, fees.Mode(), "a v3 ledger has no fee engine")

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		fees.refresh(ctx, 10*time.Millisecond)
		close(done)
	}()

	defer func() {
		cancel()
		<-done
	}()

	body.Store(`{"schemaVersion":"v1","service":"midaz-ledger","version":"4.1.3","revision":"b6506518f"}`)
	require.Eventually(t, func() bool { return fees.Mode() == midaz.FeeModeNative },
		2*time.Second, 10*time.Millisecond, "the refresh must pick up the v4.1 upgrade")

	body.Store("")
	require.Eventually(t, func() bool { return fees.Mode() == midaz.FeeModeLegacy },
		2*time.Second, 10*time.Millisecond, "an unavailable /version must fall back to legacy")
}

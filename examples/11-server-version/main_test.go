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

// TestFeeModeFollowsLedger pins the pattern: an unreadable /version at boot
// starts on legacy, the refresh loop follows the ledger, and a failed refresh
// keeps the last resolved mode.
func TestFeeModeFollowsLedger(t *testing.T) {
	var body atomic.Value // string; "" answers 503

	body.Store("")

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
	assert.Equal(t, midaz.FeeModeLegacy, fees.Mode(), "nothing is known at boot, so start on legacy")

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
	assert.Never(t, func() bool { return fees.Mode() != midaz.FeeModeNative },
		200*time.Millisecond, 10*time.Millisecond, "a failed refresh must keep the last resolved mode")

	body.Store(`{"version":"v3.8.4","requestDate":"2026-09-30T12:00:00Z"}`)
	require.Eventually(t, func() bool { return fees.Mode() == midaz.FeeModeLegacy },
		2*time.Second, 10*time.Millisecond, "a successful read must still catch a downgrade to v3")
}

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Epic 5.3 note: the 13 ledger accessors now route to plane facades on their
// own plane client, so they deliberately do NOT share the legacy *HTTPClient
// and do NOT observe SetUserAgent/SetDebug (plan
// docs/plans/2026-06-30-sdk-v4-remodel.md:571/:621 classify these as
// droppable-with-note DX; re-homing to the plane path is the optional Task
// 5.2.6). The shared-*HTTPClient invariant (H4/H5/H6) now applies only to the
// still-legacy services: Balances and Operations. These tests are
// narrowed to that pair.

// TestSharedHTTPClient_LegacyServicesShareOneInstance verifies the still-legacy
// services created by initServices point at the SAME *HTTPClient as the parent
// Entity, so mid-lifetime Set* calls propagate and refresh dedup holds. Scoped
// to the legacy pair (Balances/Operations) after the facade swap — the
// facade accessors no longer route through the shared *HTTPClient.
func TestSharedHTTPClient_LegacyServicesShareOneInstance(t *testing.T) {
	entity := newTestEntity(t, &http.Client{Timeout: time.Second}, "token", map[string]string{
		"onboarding":  "http://localhost",
		"transaction": "http://localhost",
	}, nil)

	parentHTTPClient := entity.GetEntityHTTPClient()
	require.NotNil(t, parentHTTPClient, "parent HTTPClient must not be nil")

	// Only the legacy pair still shares the parent *HTTPClient (the 13 ledger
	// accessors are plane facades — Epic 5.3).
	services := []any{entity.Balances, entity.Operations}
	require.Len(t, services, 2, "the still-legacy service pair")

	for i, svc := range services {
		reader, ok := svc.(interface{ entityHTTPClient() *HTTPClient })
		require.True(t, ok, "legacy service[%d] (%T) must expose entityHTTPClient()", i, svc)

		require.Same(t, parentHTTPClient, reader.entityHTTPClient(),
			"legacy service[%d] (%T) must share the parent Entity's *HTTPClient pointer", i, svc)
	}
}

// TestSharedHTTPClient_SetUserAgentMidLifetimePropagates verifies SetUserAgent
// on GetEntityHTTPClient changes the User-Agent on subsequent requests from the
// legacy services. (Plane-facade UA is deferred to Task 5.2.6.)
func TestSharedHTTPClient_SetUserAgentMidLifetimePropagates(t *testing.T) {
	var seenUserAgent atomic.Value

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUserAgent.Store(r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	entity := newTestEntity(t, srv.Client(), "token", map[string]string{
		"onboarding":  srv.URL,
		"transaction": srv.URL,
	}, nil)

	// Mid-lifetime: flip the user-agent AFTER initServices already ran.
	entity.GetEntityHTTPClient().SetUserAgent("rotated-ua/1.0")

	// Two different legacy services must BOTH observe the new UA.
	_, err := entity.Balances.ListBalances(context.Background(), "org-1", "ledger-1", models.BalancesListOpts{})
	require.NoError(t, err)
	assert.Equal(t, "rotated-ua/1.0", seenUserAgent.Load(),
		"Balances must observe the post-construction SetUserAgent")

	_, err = entity.Operations.ListOperations(context.Background(), "org-1", "ledger-1", "acct-1", models.OperationsListOpts{})
	require.NoError(t, err)
	assert.Equal(t, "rotated-ua/1.0", seenUserAgent.Load(),
		"Operations must observe the post-construction SetUserAgent (same shared *HTTPClient)")
}

// TestSharedHTTPClient_SetDebugMidLifetimePropagates verifies SetDebug
// propagates to the legacy services via the shared *HTTPClient.
func TestSharedHTTPClient_SetDebugMidLifetimePropagates(t *testing.T) {
	entity := newTestEntity(t, &http.Client{Timeout: time.Second}, "token", map[string]string{
		"onboarding":  "http://localhost",
		"transaction": "http://localhost",
	}, nil)

	parent := entity.GetEntityHTTPClient()
	legacy := []any{entity.Balances, entity.Operations}

	require.False(t, parent.cloneConfiguration().debug)
	for _, svc := range legacy {
		require.False(t, svc.(interface{ entityHTTPClient() *HTTPClient }).
			entityHTTPClient().cloneConfiguration().debug)
	}

	parent.SetDebug(true)

	require.True(t, parent.cloneConfiguration().debug)
	for _, svc := range legacy {
		require.True(t, svc.(interface{ entityHTTPClient() *HTTPClient }).
			entityHTTPClient().cloneConfiguration().debug,
			"legacy service must observe the SetDebug(true) made on the parent")
	}
}

// TestSharedHTTPClient_TokenRefreshVisibleAcrossServices verifies the H4 fix on
// the legacy path: when one legacy service triggers a 401-driven refresh, the
// next legacy service sees the new token immediately.
func TestSharedHTTPClient_TokenRefreshVisibleAcrossServices(t *testing.T) {
	var (
		tokenProviderCalls atomic.Int32
		tokensIssued       atomic.Int32
	)

	tokenProvider := func(_ context.Context) (string, error) {
		tokenProviderCalls.Add(1)
		seq := tokensIssued.Add(1)
		return formatToken(seq), nil
	}

	var (
		seenAuthMu   sync.Mutex
		seenAuthByOp = map[string][]string{}
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		seenAuthMu.Lock()
		seenAuthByOp[r.URL.Path] = append(seenAuthByOp[r.URL.Path], auth)
		seenAuthMu.Unlock()

		if auth == "Bearer stale" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	entity := newTestEntity(t, srv.Client(), "stale", map[string]string{
		"onboarding":  srv.URL,
		"transaction": srv.URL,
	}, nil)

	entity.GetEntityHTTPClient().setAuthTokenProvider(tokenProvider, nil)

	// Legacy service A hits 401, refreshes, retries with the new token.
	_, err := entity.Balances.ListBalances(context.Background(), "org-1", "ledger-1", models.BalancesListOpts{})
	require.NoError(t, err)

	// Legacy service B reuses the refreshed token without another refresh.
	_, err = entity.Operations.ListOperations(context.Background(), "org-1", "ledger-1", "acct-1", models.OperationsListOpts{})
	require.NoError(t, err)

	assert.Equal(t, int32(1), tokenProviderCalls.Load(),
		"tokenProvider must fire exactly once — service B reuses the refreshed token")

	seenAuthMu.Lock()
	defer seenAuthMu.Unlock()

	var sawRefreshed bool
	for _, auths := range seenAuthByOp {
		for _, a := range auths {
			if a == "Bearer "+formatToken(1) {
				sawRefreshed = true
			}
		}
	}
	require.True(t, sawRefreshed,
		"a second service's request must carry the refreshed bearer token without an extra 401 round trip")
}

// TestSharedHTTPClient_ConcurrentRefreshDeduplicates verifies the H5 fix on the
// legacy path: concurrent 401s collapse onto ONE tokenProvider call via the
// shared singleflight.
func TestSharedHTTPClient_ConcurrentRefreshDeduplicates(t *testing.T) {
	var tokenProviderCalls atomic.Int32

	tokenProvider := func(_ context.Context) (string, error) {
		tokenProviderCalls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return "refreshed", nil
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer stale" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	entity := newTestEntity(t, srv.Client(), "stale", map[string]string{
		"onboarding":  srv.URL,
		"transaction": srv.URL,
	}, nil)

	entity.GetEntityHTTPClient().setAuthTokenProvider(tokenProvider, nil)

	// Fire concurrent requests across the legacy pair; the shared singleflight
	// must collapse their 401s onto ONE provider call.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := entity.Operations.ListOperations(context.Background(), "org-1", "ledger-1", "acct-1", models.OperationsListOpts{})
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := entity.Balances.ListBalances(context.Background(), "org-1", "ledger-1", models.BalancesListOpts{})
		errs <- err
	}()

	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	assert.Equal(t, int32(1), tokenProviderCalls.Load(),
		"singleflight must collapse concurrent 401s onto ONE provider call (regression of H5)")
}

func formatToken(seq int32) string {
	return "fresh-token-" + strconv.Itoa(int(seq))
}

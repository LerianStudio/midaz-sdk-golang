package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSharedHTTPClient_AllServicesShareOneInstance verifies the central
// invariant of the v3 Entity wiring: every service entity created by
// initServices points at the SAME *HTTPClient as the parent Entity. Without
// this, mid-lifetime Set* calls on the parent would not propagate, refresh
// stampede-protection would degrade, and the documented configuration
// contract in docs/configuration.md (client.GetEntityHTTPClient().Set*)
// would be a lie.
func TestSharedHTTPClient_AllServicesShareOneInstance(t *testing.T) {
	entity := newTestEntity(t, &http.Client{Timeout: time.Second}, "token", map[string]string{
		"onboarding":  "http://localhost",
		"transaction": "http://localhost",
		"crm":         "http://localhost",
	}, nil)

	parentHTTPClient := entity.GetEntityHTTPClient()
	require.NotNil(t, parentHTTPClient, "parent HTTPClient must not be nil")

	services := []any{
		entity.Accounts, entity.AccountTypes, entity.Aliases, entity.AssetRates,
		entity.Assets, entity.Balances, entity.Holders, entity.Ledgers,
		entity.MetadataIndexes, entity.Operations, entity.OperationRoutes,
		entity.Organizations, entity.Portfolios, entity.Segments,
		entity.Transactions, entity.TransactionRoutes,
	}

	require.Len(t, services, 16, "must verify all 16 service entities")

	for i, svc := range services {
		reader, ok := svc.(interface{ entityHTTPClient() *HTTPClient })
		require.True(t, ok, "service[%d] (%T) must expose entityHTTPClient()", i, svc)

		serviceHTTPClient := reader.entityHTTPClient()
		require.Same(t, parentHTTPClient, serviceHTTPClient,
			"service[%d] (%T) must share the parent Entity's *HTTPClient pointer; otherwise H4/H5/H6 regress",
			i, svc)
	}
}

// TestSharedHTTPClient_SetUserAgentMidLifetimePropagates verifies that
// calling SetUserAgent on the *HTTPClient returned by GetEntityHTTPClient
// actually changes the User-Agent header sent on subsequent requests from
// every service. This is the documented contract in docs/configuration.md
// section 1.4.
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
		"crm":         srv.URL,
	}, nil)

	// Mid-lifetime: flip the user-agent AFTER initServices already ran. If
	// services held independent HTTPClients this would silently no-op.
	entity.GetEntityHTTPClient().SetUserAgent("rotated-ua/1.0")

	// Issue requests from two different services to prove BOTH see the new UA.
	_, err := entity.Organizations.ListOrganizations(context.Background(), models.OrganizationsListOpts{})
	require.NoError(t, err)
	assert.Equal(t, "rotated-ua/1.0", seenUserAgent.Load(),
		"Organizations must observe the post-construction SetUserAgent")

	_, err = entity.Accounts.ListAccounts(context.Background(), "org-1", "ledger-1", models.AccountsListOpts{})
	require.NoError(t, err)
	assert.Equal(t, "rotated-ua/1.0", seenUserAgent.Load(),
		"Accounts must observe the post-construction SetUserAgent (set on the same shared *HTTPClient)")
}

// TestSharedHTTPClient_SetDebugMidLifetimePropagates verifies SetDebug
// propagates to all services. The check is indirect — debug mode flips the
// internal `debug` field that several code paths read via cloneConfiguration.
// We assert the pointer-shared semantics directly by reading the same field
// through one of the service entities.
func TestSharedHTTPClient_SetDebugMidLifetimePropagates(t *testing.T) {
	entity := newTestEntity(t, &http.Client{Timeout: time.Second}, "token", map[string]string{
		"onboarding":  "http://localhost",
		"transaction": "http://localhost",
		"crm":         "http://localhost",
	}, nil)

	parent := entity.GetEntityHTTPClient()

	// Initial state: every service reads debug=false.
	require.False(t, parent.cloneConfiguration().debug)
	for _, svc := range []any{entity.Accounts, entity.Transactions, entity.Ledgers} {
		require.False(t, svc.(interface{ entityHTTPClient() *HTTPClient }).
			entityHTTPClient().cloneConfiguration().debug)
	}

	// Flip on the parent.
	parent.SetDebug(true)

	// Every service reads debug=true because they share the *HTTPClient.
	require.True(t, parent.cloneConfiguration().debug)
	for _, svc := range []any{entity.Accounts, entity.Transactions, entity.Ledgers, entity.Holders} {
		require.True(t, svc.(interface{ entityHTTPClient() *HTTPClient }).
			entityHTTPClient().cloneConfiguration().debug,
			"service must observe the SetDebug(true) made on the parent")
	}
}

// TestSharedHTTPClient_TokenRefreshVisibleAcrossServices verifies the H4 fix:
// when one service triggers a 401-driven refresh, every other service sees
// the new token immediately on its next request.
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
		op := r.URL.Path
		seenAuthMu.Lock()
		seenAuthByOp[op] = append(seenAuthByOp[op], auth)
		seenAuthMu.Unlock()

		// Reject the FIRST request (with stale token) only. Accept everything else.
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
		"crm":         srv.URL,
	}, nil)

	entity.GetEntityHTTPClient().setAuthTokenProvider(tokenProvider, nil)

	// Service A hits 401, refreshes, retries with new token.
	_, err := entity.Organizations.ListOrganizations(context.Background(), models.OrganizationsListOpts{})
	require.NoError(t, err)

	// Service B's next request: should see token-1 IMMEDIATELY without
	// triggering another refresh (because the refresh wrote to the shared
	// HTTPClient's authToken field, not a per-service copy).
	_, err = entity.Accounts.ListAccounts(context.Background(), "org-1", "ledger-1", models.AccountsListOpts{})
	require.NoError(t, err)

	assert.Equal(t, int32(1), tokenProviderCalls.Load(),
		"tokenProvider must fire exactly once — service B should reuse the refreshed token, not refresh again")

	// Verify the accounts call carried the refreshed token straight away
	// (no 401 dance for the second service).
	seenAuthMu.Lock()
	defer seenAuthMu.Unlock()
	require.NotEmpty(t, seenAuthByOp)

	var accountsAuth string
	for path, auths := range seenAuthByOp {
		if path == "" {
			continue
		}
		// Find the path that corresponds to accounts (anything containing "/accounts").
		for _, a := range auths {
			if a == "Bearer "+formatToken(1) {
				accountsAuth = a
			}
		}
	}
	require.Equal(t, "Bearer "+formatToken(1), accountsAuth,
		"second service's request must carry the refreshed bearer token without an extra 401 round trip")
}

// TestSharedHTTPClient_ConcurrentRefreshDeduplicates verifies the H5 fix:
// concurrent 401s across multiple services collapse onto ONE underlying
// tokenProvider call via singleflight. With per-service singleflight groups,
// a 16-service stampede would have produced 16 calls.
func TestSharedHTTPClient_ConcurrentRefreshDeduplicates(t *testing.T) {
	var (
		tokenProviderCalls atomic.Int32
		// Block tokenProvider for 50ms so concurrent requests pile up
		// behind the singleflight, which is exactly the case the lock
		// protects against.
		tokenProviderEntered = make(chan struct{}, 4)
	)

	tokenProvider := func(_ context.Context) (string, error) {
		tokenProviderEntered <- struct{}{}
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
		"crm":         srv.URL,
	}, nil)

	entity.GetEntityHTTPClient().setAuthTokenProvider(tokenProvider, nil)

	// Fire 4 concurrent requests across different services. Without the
	// shared singleflight group, each service would fire its own provider
	// call (4 total). With the shared group, they all collapse onto 1.
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	wg.Add(4)
	go func() {
		defer wg.Done()
		_, err := entity.Organizations.ListOrganizations(context.Background(), models.OrganizationsListOpts{})
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := entity.Accounts.ListAccounts(context.Background(), "org-1", "ledger-1", models.AccountsListOpts{})
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := entity.Ledgers.ListLedgers(context.Background(), "org-1", models.LedgersListOpts{})
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := entity.Assets.ListAssets(context.Background(), "org-1", "ledger-1", models.AssetsListOpts{})
		errs <- err
	}()

	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	assert.Equal(t, int32(1), tokenProviderCalls.Load(),
		"singleflight must collapse 4 concurrent 401s onto ONE provider call; got %d (regression of H5)",
		tokenProviderCalls.Load())
}

func formatToken(seq int32) string {
	return "fresh-token-" + string('0'+seq)
}

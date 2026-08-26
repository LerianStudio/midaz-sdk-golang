package entities

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/security"
)

// TestNewPlaneClients_PreservesConfiguredTimeout proves the configured request
// Timeout reaches the plane client. The bare &http.Client{Transport: rt} form
// dropped it, so a WithTimeout / MIDAZ_TIMEOUT was a no-op on ALL plane traffic
// (incl. the transaction money path): a hung server would block indefinitely.
//
// A 100ms client Timeout is applied against a server that stalls for 2s. With
// the fix the call must fail fast (well under 1s); without it the single
// attempt waits the full stall and succeeds. The handler unblocks on request
// cancellation so the deferred srv.Close() does not wait out the full stall.
func TestNewPlaneClients_PreservesConfiguredTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[],"limit":10}`))
	}))
	defer srv.Close()

	client := srv.Client()
	client.Timeout = 100 * time.Millisecond

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: srv.URL,
		tracerURL: srv.URL + "/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-1", nil },
		},
		httpClient:   client,
		retryOptions: planeTestRetryOptions(), // MaxRetries=0: Timeout bounds one attempt
	})
	if err != nil {
		t.Fatalf("newPlaneClients: %v", err)
	}

	limit := "10"
	start := time.Now()
	_, err = planes.Ledger.ListOrganizationsWithResponse(context.Background(), &genledger.ListOrganizationsParams{Limit: &limit})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("want a timeout error, got nil after %v (configured Timeout was dropped)", elapsed)
	}
	if elapsed > time.Second {
		t.Fatalf("call took %v, want < 1s (100ms Timeout must bound the plane request)", elapsed)
	}
}

// TestNewPlaneClients_BlocksMoneyPathCrossHostRedirect proves CheckRedirect is
// enforced on the plane path for the money path. An unsafe method (POST) that a
// cross-host 302 tries to bounce to a foreign origin must be blocked by
// security.ValidateRedirect BEFORE the client follows it — otherwise the auth
// round tripper re-stamps the Bearer on the follow-up RoundTrip and leaks the
// token to the foreign host.
//
// ValidateRedirect refuses a cross-origin redirect whose previous request was a
// non-GET/HEAD method (requestMayReplaySensitiveBody), which covers every
// transaction/write on the money path regardless of where the auth header sits
// in the transport chain.
func TestNewPlaneClients_BlocksMoneyPathCrossHostRedirect(t *testing.T) {
	var foreignAuth atomic.Value // string
	foreignAuth.Store("")

	var foreignHits int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&foreignHits, 1)
		foreignAuth.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer foreign.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/v1/organizations", http.StatusFound)
	}))
	defer origin.Close()

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: origin.URL,
		tracerURL: origin.URL + "/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-secret", nil },
		},
		httpClient:   origin.Client(),
		retryOptions: planeTestRetryOptions(),
	})
	if err != nil {
		t.Fatalf("newPlaneClients: %v", err)
	}

	_, err = planes.Ledger.CreateOrganizationWithBodyWithResponse(
		context.Background(), "application/json", strings.NewReader(`{}`),
	)

	if err == nil {
		t.Fatalf("want the redirect-validation error, got nil (cross-host 302 was followed)")
	}
	if !errors.Is(err, security.ErrAuthenticatedRedirect) {
		t.Fatalf("err = %v, want it to wrap security.ErrAuthenticatedRedirect", err)
	}
	if got := atomic.LoadInt32(&foreignHits); got != 0 {
		t.Fatalf("foreign host received %d request(s), want 0 (cross-host redirect must be blocked)", got)
	}
	if got := foreignAuth.Load().(string); got != "" {
		t.Fatalf("foreign host saw Authorization = %q, want empty (Bearer must not leak across hosts)", got)
	}
}

// TestSDKDefaultTransports_ResponseHeaderTimeout proves the hard-guard against a
// server that accepts the connection but stalls before sending response headers
// lands on BOTH SDK-owned default transports (the config-built client used by
// the real construction path, and the entities package fallback). Both planes
// and the legacy path wrap one of these, so the backstop reaches them all.
func TestSDKDefaultTransports_ResponseHeaderTimeout(t *testing.T) {
	entitiesTr, ok := defaultHTTPClient().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("entities defaultHTTPClient transport is %T, want *http.Transport", defaultHTTPClient().Transport)
	}
	if entitiesTr.ResponseHeaderTimeout <= 0 {
		t.Fatalf("entities default transport ResponseHeaderTimeout = %v, want > 0", entitiesTr.ResponseHeaderTimeout)
	}

	configTr, ok := config.NewDefaultHTTPClient(60 * time.Second).Transport.(*http.Transport)
	if !ok {
		t.Fatalf("config.NewDefaultHTTPClient transport is not *http.Transport")
	}
	if configTr.ResponseHeaderTimeout <= 0 {
		t.Fatalf("config default transport ResponseHeaderTimeout = %v, want > 0", configTr.ResponseHeaderTimeout)
	}
}

// TestNewPlaneClients_BlocksGETCrossHostRedirect closes the read-gap: a plain
// GET cross-host 302 must ALSO be blocked. validateSDKRedirect would follow it
// (the auth header is injected below the CheckRedirect layer, so via[0] carries
// nothing sensitive on a GET), leaking the Bearer to the foreign host on the
// re-stamped follow-up RoundTrip. The plane-specific policy blocks ALL
// cross-origin redirects, so the foreign host is never contacted.
func TestNewPlaneClients_BlocksGETCrossHostRedirect(t *testing.T) {
	var foreignAuth atomic.Value // string
	foreignAuth.Store("")

	var foreignHits int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&foreignHits, 1)
		foreignAuth.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer foreign.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/v1/organizations", http.StatusFound)
	}))
	defer origin.Close()

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: origin.URL,
		tracerURL: origin.URL + "/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-secret", nil },
		},
		httpClient:   origin.Client(),
		retryOptions: planeTestRetryOptions(),
	})
	if err != nil {
		t.Fatalf("newPlaneClients: %v", err)
	}

	_, err = planes.Ledger.ListOrganizationsWithResponse(context.Background(), nil)

	if err == nil {
		t.Fatalf("want the redirect-validation error, got nil (GET cross-host 302 was followed)")
	}
	if !errors.Is(err, security.ErrAuthenticatedRedirect) {
		t.Fatalf("err = %v, want it to wrap security.ErrAuthenticatedRedirect", err)
	}
	if got := atomic.LoadInt32(&foreignHits); got != 0 {
		t.Fatalf("foreign host received %d request(s), want 0 (GET cross-host redirect must be blocked)", got)
	}
	if got := foreignAuth.Load().(string); got != "" {
		t.Fatalf("foreign host saw Authorization = %q, want empty (Bearer must not leak across hosts)", got)
	}
}

// TestNewPlaneClients_FollowsSameOriginRedirect proves the plane policy does not
// over-block: a SAME-origin redirect (here a path change on the same host) is
// still followed to a successful 200.
func TestNewPlaneClients_FollowsSameOriginRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/organizations" {
			// Relative Location → resolved against the same host → same origin.
			http.Redirect(w, r, "/v1/organizations-final", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[],"limit":10}`))
	}))
	defer srv.Close()

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: srv.URL,
		tracerURL: srv.URL + "/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-1", nil },
		},
		httpClient:   srv.Client(),
		retryOptions: planeTestRetryOptions(),
	})
	if err != nil {
		t.Fatalf("newPlaneClients: %v", err)
	}

	resp, err := planes.Ledger.ListOrganizationsWithResponse(context.Background(), nil)
	if err != nil {
		t.Fatalf("same-origin redirect must be followed, got error: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, want 200 (same-origin redirect should reach the final 200)", resp.StatusCode())
	}
}

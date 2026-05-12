package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTokenFromAccessManager_ValidatesEnabledCredentialsAndClient(t *testing.T) {
	tests := []struct {
		name       string
		mgr        AccessManager
		client     *http.Client
		wantError  string
		wantReason string
		wantScheme string
		wantHost   string
	}{
		{name: "disabled auth", mgr: AccessManager{Enabled: false}, client: http.DefaultClient, wantError: "plugin authentication is not enabled", wantReason: "auth_disabled"},
		{name: "missing address", mgr: AccessManager{Enabled: true, ClientID: "client", ClientSecret: "secret"}, client: http.DefaultClient, wantError: "plugin auth address is required", wantReason: "missing_address"},
		{name: "missing client id", mgr: AccessManager{Enabled: true, Address: "http://localhost:4000", ClientSecret: "secret"}, client: http.DefaultClient, wantError: "client id is required", wantReason: "missing_client_id"},
		{name: "missing client secret", mgr: AccessManager{Enabled: true, Address: "http://localhost:4000", ClientID: "client"}, client: http.DefaultClient, wantError: "client secret is required", wantReason: "missing_client_secret"},
		{name: "nil http client", mgr: AccessManager{Enabled: true, Address: "http://localhost:4000", ClientID: "client", ClientSecret: "secret"}, client: nil, wantError: "HTTP client cannot be nil", wantReason: "nil_http_client", wantScheme: "http", wantHost: "localhost:4000"},
		{name: "malformed endpoint", mgr: AccessManager{Enabled: true, Address: "https://auth.example.com/%zz", ClientID: "client", ClientSecret: "secret"}, client: http.DefaultClient, wantError: "invalid plugin auth address", wantReason: "malformed_endpoint"},
		{name: "missing scheme", mgr: AccessManager{Enabled: true, Address: "auth.example.com", ClientID: "client", ClientSecret: "secret"}, client: http.DefaultClient, wantError: "must include scheme", wantReason: "missing_scheme"},
		{name: "missing host", mgr: AccessManager{Enabled: true, Address: "https:///auth", ClientID: "client", ClientSecret: "secret"}, client: http.DefaultClient, wantError: "must include host", wantReason: "missing_host", wantScheme: "https"},
		{name: "userinfo", mgr: AccessManager{Enabled: true, Address: "https://user:pass@auth.example.com", ClientID: "client", ClientSecret: "secret"}, client: http.DefaultClient, wantError: "must not include user information", wantReason: "userinfo_not_allowed", wantScheme: "https", wantHost: "auth.example.com"},
		{name: "invalid scheme", mgr: AccessManager{Enabled: true, Address: "ftp://auth.example.com", ClientID: "client", ClientSecret: "secret"}, client: http.DefaultClient, wantError: "unsupported URL scheme", wantReason: "invalid_scheme", wantScheme: "ftp", wantHost: "auth.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetTokenFromAccessManager(context.Background(), tt.mgr, tt.client)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantError)

			var tokenErr *AccessManagerTokenRequestError
			require.ErrorAs(t, err, &tokenErr)
			require.Equal(t, accessManagerTokenRequestOperation, tokenErr.Operation)
			require.Equal(t, accessManagerTokenFetchPhase, tokenErr.Phase)
			require.True(t, tokenErr.LocalValidationFailed)
			require.False(t, tokenErr.HTTPRequestSent)
			require.Equal(t, tt.wantReason, tokenErr.ValidationReason)
			require.Equal(t, tt.wantScheme, tokenErr.EndpointScheme)
			require.Equal(t, tt.wantHost, tokenErr.EndpointHost)
			require.NotContains(t, err.Error(), "super-secret-value")
			require.NotContains(t, err.Error(), "user:pass")
			require.NotContains(t, err.Error(), "Authorization")
		})
	}
}

func TestGetTokenFromAccessManager_UsesCallerContextAndNormalizesTrailingSlash(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mgr := AccessManager{Enabled: true, Address: "http://localhost:4000/", ClientID: "client", ClientSecret: "secret"}
	_, err := GetTokenFromAccessManager(ctx, mgr, http.DefaultClient)
	require.Error(t, err)
	require.Contains(t, err.Error(), "context canceled")

	var seenPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(TokenResponse{AccessToken: "token", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}); err != nil {
			t.Errorf("encode token response: %v", err)
		}
	}))
	defer srv.Close()

	mgr.Address = srv.URL + "/"
	InvalidateAccessManagerToken(mgr)
	token, err := GetTokenFromAccessManager(context.Background(), mgr, srv.Client())
	require.NoError(t, err)
	require.Equal(t, "token", token)
	require.Equal(t, "/v1/login/oauth/access_token", seenPath)
}

func TestGetTokenFromAccessManager_BoundsResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)

		_, err := w.Write(make([]byte, maxAccessManagerResponseBodyBytes+1))
		if err != nil {
			t.Errorf("write response body: %v", err)
		}
	}))
	defer srv.Close()

	mgr := AccessManager{Enabled: true, Address: srv.URL, ClientID: "client", ClientSecret: "secret"}
	InvalidateAccessManagerToken(mgr)
	_, err := GetTokenFromAccessManager(context.Background(), mgr, srv.Client())
	require.Error(t, err)
	require.Contains(t, err.Error(), "response body exceeds")
}

func TestGetTokenFromAccessManager_CachesTokenUntilRefreshWindow(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := calls.Add(1)

		w.Header().Set("Content-Type", "application/json")

		// Use fmt.Sprintf so call counts above 9 still produce a sensible
		// token string. The previous expression — "token-" + string('0'+call)
		// — relied on rune arithmetic and only worked for single-digit
		// values, which silently corrupted assertions on higher counts.
		if err := json.NewEncoder(w).Encode(TokenResponse{AccessToken: fmt.Sprintf("token-%d", call), ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}); err != nil {
			t.Errorf("encode token response: %v", err)
		}
	}))
	defer srv.Close()

	mgr := AccessManager{Enabled: true, Address: srv.URL, ClientID: "client", ClientSecret: "secret"}
	InvalidateAccessManagerToken(mgr)
	first, err := GetTokenFromAccessManager(context.Background(), mgr, srv.Client())
	require.NoError(t, err)
	second, err := GetTokenFromAccessManager(context.Background(), mgr, srv.Client())
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, int32(1), calls.Load())
}

func TestGetTokenFromAccessManager_BoundsLongCallerDeadlineForSingleflightRequest(t *testing.T) {
	deadline := time.Now().Add(time.Hour)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	var gotDeadline time.Time
	var hasDeadline bool
	client := &http.Client{Transport: accessManagerRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotDeadline, hasDeadline = req.Context().Deadline()
		return accessManagerTokenResponse(req), nil
	})}

	mgr := AccessManager{Enabled: true, Address: "http://localhost:4000", ClientID: "deadline-client", ClientSecret: "deadline-secret"}
	InvalidateAccessManagerToken(mgr)

	token, err := GetTokenFromAccessManager(ctx, mgr, client)

	require.NoError(t, err)
	require.Equal(t, "token", token)
	require.True(t, hasDeadline)
	require.WithinDuration(t, time.Now().Add(accessManagerTokenRequestTimeout), gotDeadline, time.Second)
	require.True(t, gotDeadline.Before(deadline))
}

func TestGetTokenFromAccessManager_RejectsInternalHTTPAccessManagerBeforeOutbound(t *testing.T) {
	var called atomic.Bool

	client := &http.Client{Transport: accessManagerRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		called.Store(true)

		return accessManagerTokenResponse(req), nil
	})}

	mgr := AccessManager{
		Enabled:      true,
		Address:      "http://plugin-access-manager-auth.midaz-plugins.svc.cluster.local:4000",
		ClientID:     "internal-client",
		ClientSecret: "internal-secret",
	}
	InvalidateAccessManagerToken(mgr)

	token, err := GetTokenFromAccessManager(context.Background(), mgr, client)

	require.Error(t, err)
	require.Empty(t, token)
	require.False(t, called.Load(), "local URL validation must fail before the caller HTTP client is invoked")

	var tokenErr *AccessManagerTokenRequestError
	require.ErrorAs(t, err, &tokenErr)
	require.Equal(t, "access_manager.token_request", tokenErr.Operation)
	require.Equal(t, "token_fetch", tokenErr.Phase)
	require.Equal(t, "http", tokenErr.EndpointScheme)
	require.Equal(t, "plugin-access-manager-auth.midaz-plugins.svc.cluster.local:4000", tokenErr.EndpointHost)
	require.Equal(t, "/v1/login/oauth/access_token", tokenErr.EndpointPath)
	require.True(t, tokenErr.LocalValidationFailed)
	require.False(t, tokenErr.HTTPRequestSent)
	require.Equal(t, "insecure_scheme", tokenErr.ValidationReason)
	require.Zero(t, tokenErr.StatusCode())

	rendered := err.Error()
	require.Contains(t, rendered, "endpoint=http://plugin-access-manager-auth.midaz-plugins.svc.cluster.local:4000/v1/login/oauth/access_token")
	require.Contains(t, rendered, "httpRequestSent=false")
	require.Contains(t, rendered, "localValidationFailed=true")
	require.Contains(t, rendered, "validationReason=insecure_scheme")
	require.NotContains(t, rendered, "internal-secret")
	require.NotContains(t, rendered, "Bearer")
}

func TestGetTokenFromAccessManager_AllowsInternalHTTPWhenExplicitlyEnabled(t *testing.T) {
	var called atomic.Bool
	var seenScheme, seenHost, seenPath string

	client := &http.Client{Transport: accessManagerRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		called.Store(true)
		seenScheme = req.URL.Scheme
		seenHost = req.URL.Host
		seenPath = req.URL.Path

		return accessManagerTokenResponse(req), nil
	})}

	mgr := AccessManager{
		Enabled:           true,
		Address:           "http://plugin-access-manager-auth.midaz-plugins.svc.cluster.local:4000",
		ClientID:          "internal-client",
		ClientSecret:      "internal-secret",
		AllowInsecureHTTP: true,
	}
	InvalidateAccessManagerToken(mgr)

	token, err := GetTokenFromAccessManager(context.Background(), mgr, client)

	require.NoError(t, err)
	require.Equal(t, "token", token)
	require.True(t, called.Load())
	require.Equal(t, "http", seenScheme)
	require.Equal(t, "plugin-access-manager-auth.midaz-plugins.svc.cluster.local:4000", seenHost)
	require.Equal(t, accessManagerOAuthLoginPath, seenPath)
}

func TestGetTokenFromAccessManager_BlocksCrossOriginRedirectWithCustomPolicy(t *testing.T) {
	var redirected atomic.Bool

	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected.Store(true)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"accessToken":"stolen"}`))
		assert.NoError(t, err)
	}))
	defer destination.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", destination.URL+accessManagerOAuthLoginPath)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	client := source.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return nil
	}

	mgr := AccessManager{
		Enabled:      true,
		Address:      source.URL,
		ClientID:     "redirect-client",
		ClientSecret: "redirect-secret",
	}
	InvalidateAccessManagerToken(mgr)

	token, err := GetTokenFromAccessManager(context.Background(), mgr, client)

	require.Error(t, err)
	require.Empty(t, token)
	require.False(t, redirected.Load(), "cross-origin redirect must not receive Access Manager credentials")
	require.Contains(t, err.Error(), "authenticated redirect")
	require.NotContains(t, err.Error(), "redirect-secret")
}

func TestGetTokenFromAccessManager_AllowsHTTPSAccessManagerWithCallerClient(t *testing.T) {
	var called atomic.Bool
	var seenPath string

	client := &http.Client{Transport: accessManagerRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		called.Store(true)
		seenPath = req.URL.Path

		return accessManagerTokenResponse(req), nil
	})}

	mgr := AccessManager{
		Enabled:      true,
		Address:      "https://auth.stg.lerian.io",
		ClientID:     "internal-client",
		ClientSecret: "internal-secret",
	}
	InvalidateAccessManagerToken(mgr)

	token, err := GetTokenFromAccessManager(context.Background(), mgr, client)

	require.NoError(t, err)
	require.True(t, called.Load())
	require.Equal(t, "/v1/login/oauth/access_token", seenPath)
	require.Equal(t, "token", token)
}

type accessManagerRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn accessManagerRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func accessManagerTokenResponse(req *http.Request) *http.Response {
	body := `{"accessToken":"token","expiresAt":"` + time.Now().Add(time.Hour).Format(time.RFC3339) + `"}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetTokenFromAccessManager_ValidatesEnabledCredentialsAndClient(t *testing.T) {
	tests := []struct {
		name      string
		mgr       AccessManager
		client    *http.Client
		wantError string
	}{
		{name: "missing client id", mgr: AccessManager{Enabled: true, Address: "http://localhost:4000", ClientSecret: "secret"}, client: http.DefaultClient, wantError: "client id is required"},
		{name: "missing client secret", mgr: AccessManager{Enabled: true, Address: "http://localhost:4000", ClientID: "client"}, client: http.DefaultClient, wantError: "client secret is required"},
		{name: "nil http client", mgr: AccessManager{Enabled: true, Address: "http://localhost:4000", ClientID: "client", ClientSecret: "secret"}, client: nil, wantError: "HTTP client cannot be nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetTokenFromAccessManager(context.Background(), tt.mgr, tt.client)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantError)
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

		if err := json.NewEncoder(w).Encode(TokenResponse{AccessToken: "token-" + string('0'+call), ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}); err != nil {
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

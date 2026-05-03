package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	auth "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
	"github.com/stretchr/testify/require"
)

type typedNilConfig struct{}

func (*typedNilConfig) GetHTTPClient() *http.Client                      { return nil }
func (*typedNilConfig) GetBaseURLs() map[string]string                   { return nil }
func (*typedNilConfig) GetObservabilityProvider() observability.Provider { return nil }
func (*typedNilConfig) GetPluginAuth() auth.AccessManager                { return auth.AccessManager{} }

func nilContext() context.Context { return nil }

func requireHandlerNoError(t *testing.T, errs <-chan error) {
	t.Helper()

	select {
	case err := <-errs:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("handler did not report result")
	}
}

func TestEntityConstructors_WithNilOption_ReturnError(t *testing.T) {
	_, err := New("http://localhost:3002", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "option cannot be nil")

	_, err = NewWithServiceURLs(map[string]string{"onboarding": "http://localhost:3002", "transaction": "http://localhost:3002", "crm": "http://localhost:4003"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "option cannot be nil")
}

func TestEntityContextHelpers_WithNilContext_AreSafe(t *testing.T) {
	ctx := WithIdempotencyKey(nilContext(), "idem-1")
	require.NotNil(t, ctx)
	require.Equal(t, "idem-1", getIdempotencyKeyFromContext(ctx))

	ctx = WithTenantID(nilContext(), "tenant-1")
	require.NotNil(t, ctx)
	require.Equal(t, "tenant-1", TenantIDFromContext(ctx))
	require.Empty(t, TenantIDFromContext(nilContext()))
}

func TestNewEntityWithConfig_WithTypedNilConfig_ReturnsError(t *testing.T) {
	var cfg *typedNilConfig

	_, err := NewEntityWithConfig(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "config cannot be nil")
}

func TestZeroValueEntityExportedMethods_AreSafe(t *testing.T) {
	var e Entity

	require.NotPanics(t, func() { e.InitServices() })
	require.Nil(t, e.GetEntityHTTPClient())
	require.Nil(t, e.GetHTTPClient())
	require.Nil(t, e.GetObservabilityProvider())
	require.NotPanics(t, func() { e.SetHTTPClient(&http.Client{Timeout: time.Second}) })
	require.NotPanics(t, func() { e.SetAuthToken("token") })
}

func TestSlice2NewWithServiceURLs_DefaultsMissingCRMURLToOnboarding(t *testing.T) {
	entity, err := NewWithServiceURLs(map[string]string{
		"onboarding":  "https://api.example.com/onboarding",
		"transaction": "https://api.example.com/transaction",
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/onboarding/v1", entity.baseURLs["crm"])
}

func TestEntityURLs_NormalizeV1AndRejectUnsafeDirectURLs(t *testing.T) {
	entity, err := NewWithServiceURLs(map[string]string{
		"onboarding":  "http://localhost:3002///",
		"transaction": "http://localhost:3002/api",
		"crm":         "http://localhost:4003",
	})
	require.NoError(t, err)
	require.Equal(t, "http://localhost:3002/v1", entity.baseURLs["onboarding"])
	require.Equal(t, "http://localhost:3002/api/v1", entity.baseURLs["transaction"])
	require.Equal(t, "http://localhost:4003/v1", entity.baseURLs["crm"])

	_, err = New("https://user:pass@example.com")
	require.Error(t, err)
	require.Contains(t, err.Error(), "user information")

	_, err = New("http://api.example.com")
	require.Error(t, err)
	require.Contains(t, err.Error(), "insecure HTTP")
}

func TestEntitySetHTTPClient_PreservesProtocolConfiguration(t *testing.T) {
	entity, err := New("http://localhost:3002", WithDebug(true), WithUserAgent("slice2-agent"), WithDefaultTenantID("tenant-1"))
	require.NoError(t, err)
	entity.GetEntityHTTPClient().SetEnableIdempotency(false)
	entity.GetEntityHTTPClient().SetCustomRetryPolicy(func(*http.Response, error) bool { return true })
	entity.GetEntityHTTPClient().WithRetryOptions(retry.WithMaxRetries(7))

	entity.SetHTTPClient(&http.Client{Timeout: 2 * time.Second})

	hc := entity.GetEntityHTTPClient()
	require.Equal(t, "slice2-agent", hc.userAgent)
	require.Equal(t, "tenant-1", hc.GetTenantID())
	require.True(t, hc.debug)
	require.False(t, hc.enableIdempotency)
	require.NotNil(t, hc.customRetryPolicy)
	require.Equal(t, 7, hc.retryOptions.MaxRetries)
}

// TestHTTPClient_DebugErrorPathRedactsURL captures debug output via the
// public WithDebugWriter option instead of swapping os.Stderr globally.
// Mutating os.Stderr races against any parallel test that also touches
// it, so the bytes.Buffer + WithDebugWriter pattern is the correct
// goroutine-safe shape.
func TestHTTPClient_DebugErrorPathRedactsURL(t *testing.T) {
	writeErrs := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"message":"bad"}`))
		writeErrs <- err
	}))
	defer srv.Close()

	var debugBuf bytes.Buffer

	c := NewHTTPClient(srv.Client(), "", nil).
		WithDebug(true).
		WithDebugWriter(&debugBuf)

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"?document=12345678900&limit=10", nil, nil, &out)
	require.Error(t, err)
	requireHandlerNoError(t, writeErrs)

	require.NotContains(t, debugBuf.String(), "12345678900")
	require.Contains(t, debugBuf.String(), "%5BREDACTED%5D")
}

func TestHTTPClient_ResponseBodyLimit(t *testing.T) {
	writeErrs := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write(make([]byte, maxHTTPResponseBodyBytes+1))
		writeErrs <- err
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
	require.Error(t, err)
	requireHandlerNoError(t, writeErrs)
	require.Contains(t, err.Error(), "response body exceeds")
}

func TestHTTPClient_AutoIdempotencyForAllUnsafeMethods(t *testing.T) {
	unsafeMethods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, method := range unsafeMethods {
		t.Run(method, func(t *testing.T) {
			var seen string

			writeErrs := make(chan error, 1)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = r.Header.Get("X-Idempotency")
				w.Header().Set("Content-Type", "application/json")

				_, err := w.Write([]byte(`{}`))
				writeErrs <- err
			}))
			defer srv.Close()

			c := NewHTTPClient(srv.Client(), "", nil)

			var out map[string]any

			err := c.doRequest(context.Background(), method, srv.URL, nil, map[string]string{"ok": "true"}, &out)
			require.NoError(t, err)
			requireHandlerNoError(t, writeErrs)
			require.NotEmpty(t, seen)
		})
	}
}

func TestHTTPClient_CustomRetryPolicyCanForceRetryForNonDefaultStatus(t *testing.T) {
	var calls atomic.Int32

	writeErrs := make(chan error, 2)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusUnprocessableEntity)

			_, err := w.Write([]byte(`{"message":"try again"}`))
			writeErrs <- err

			return
		}

		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{}`))
		writeErrs <- err
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.Client(), "", nil)
	c.WithRetryOptions(retry.WithMaxRetries(1), retry.WithInitialDelay(time.Millisecond), retry.WithMaxDelay(time.Millisecond))
	c.SetCustomRetryPolicy(func(resp *http.Response, _ error) bool {
		return resp != nil && resp.StatusCode == http.StatusUnprocessableEntity
	})

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
	require.NoError(t, err)
	requireHandlerNoError(t, writeErrs)
	requireHandlerNoError(t, writeErrs)
	require.Equal(t, int32(2), calls.Load())
}

func TestHTTPClient_AccessManagerTokenInvalidatedAndRefetchedOnceOnUnauthorized(t *testing.T) {
	var calls atomic.Int32

	authHeaders := make(chan string, 2)
	writeErrs := make(chan error, 2)
	c := NewHTTPClient(http.DefaultClient, "expired", nil)
	c.setAuthTokenProvider(func(context.Context) (string, error) { return "fresh", nil }, func() {})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)

		authHeaders <- r.Header.Get("Authorization")

		if call == 1 {
			w.WriteHeader(http.StatusUnauthorized)

			_, err := w.Write([]byte(`{"message":"unauthorized"}`))
			writeErrs <- err

			return
		}

		w.Header().Set("Content-Type", "application/json")

		writeErrs <- json.NewEncoder(w).Encode(models.Organization{ID: "org-1"})
	}))
	defer srv.Close()

	c.client = srv.Client()

	var out models.Organization

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
	require.NoError(t, err)
	require.Equal(t, "Bearer expired", <-authHeaders)
	require.Equal(t, "Bearer fresh", <-authHeaders)
	requireHandlerNoError(t, writeErrs)
	requireHandlerNoError(t, writeErrs)
	require.Equal(t, int32(2), calls.Load())
}

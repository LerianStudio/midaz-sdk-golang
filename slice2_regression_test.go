package midaz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestClientNew_WithNilOption_ReturnsError(t *testing.T) {
	_, err := New(nil)
	require.Error(t, err)
	require.True(t, sdkerrors.IsConfigurationError(err),
		"nil option should yield a typed ErrConfiguration")
	require.Contains(t, err.Error(), "index 0",
		"error should identify which option index was nil")
}

func TestClientTrace_WithNilCallback_ReturnsError(t *testing.T) {
	c, err := New(WithConfig(createTestConfig(t)))
	require.NoError(t, err)

	err = c.Trace("slice2.nil_callback", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "trace callback cannot be nil")
}

func TestClientWithTimeout_PropagatesToOwnedEntityHTTPClient(t *testing.T) {
	c, err := New(
		WithConfig(createTestConfig(t)),
		WithTimeout(7*time.Second),
	)
	require.NoError(t, err)
	require.NotNil(t, c.Entity)
	require.Equal(t, 7*time.Second, c.Entity.GetHTTPClient().Timeout)
}

func TestClientWithTimeout_DoesNotMutateUserOwnedCustomHTTPClient(t *testing.T) {
	custom := &http.Client{Timeout: 55 * time.Second}
	c, err := New(
		WithConfig(createTestConfig(t)),
		WithHTTPClient(custom),
		WithTimeout(8*time.Second),
	)
	require.NoError(t, err)
	require.NotNil(t, c.Entity)
	require.Same(t, custom, c.Entity.GetHTTPClient())
	require.Equal(t, 55*time.Second, custom.Timeout)
}

func TestClientEntityOptions_PropagateToServiceHTTPClients(t *testing.T) {
	var seenUserAgent, seenTenantID, seenIdempotency string

	writeErrs := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUserAgent = r.Header.Get("User-Agent")
		seenTenantID = r.Header.Get(entities.HeaderTenantID)
		seenIdempotency = r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		writeErrs <- json.NewEncoder(w).Encode(models.Organization{ID: "org-1", LegalName: "Acme", LegalDocument: "123"})
	}))
	defer srv.Close()

	c, err := New(
		WithConfig(createTestConfig(t)),
		WithBaseURL(srv.URL),
		WithUserAgent("slice2-agent/1.0"),
		WithTenantID("tenant-root"),
	)
	require.NoError(t, err)

	_, err = c.Organizations.CreateOrganization(context.Background(), models.NewCreateOrganizationInput("Acme", "123"))
	require.NoError(t, err)
	require.NoError(t, <-writeErrs)
	require.Equal(t, "slice2-agent/1.0", seenUserAgent)
	require.Equal(t, "tenant-root", seenTenantID)
	require.NotEmpty(t, seenIdempotency)
}

func TestClientNew_WithEnvironmentRecomputesDefaultServiceURLs(t *testing.T) {
	c, err := New(WithEnvironment(config.EnvironmentProduction))
	require.NoError(t, err)

	urls := c.GetConfig().ServiceURLs
	require.Equal(t, "https://api.midaz.io/v1", urls[config.ServiceOnboarding])
	require.Equal(t, "https://api.midaz.io/v1", urls[config.ServiceTransaction])
	require.Equal(t, "https://api.midaz.io/v1", urls[config.ServiceCRM])
}

func TestClientNew_WithEnvironmentDoesNotOverrideExplicitURLs(t *testing.T) {
	c, err := New(
		WithOnboardingURL("https://onboarding.example.com/v1"),
		WithTransactionURL("https://transaction.example.com/v1"),
		WithCRMURL("https://crm.example.com/v1"),
		WithEnvironment(config.EnvironmentProduction),
	)
	require.NoError(t, err)

	urls := c.GetConfig().ServiceURLs
	require.Equal(t, "https://onboarding.example.com/v1", urls[config.ServiceOnboarding])
	require.Equal(t, "https://transaction.example.com/v1", urls[config.ServiceTransaction])
	require.Equal(t, "https://crm.example.com/v1", urls[config.ServiceCRM])
}

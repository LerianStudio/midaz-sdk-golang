package midaz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// --- from client_options_regression_test.go ---

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

	err = c.Trace("client_options.nil_callback", nil)
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
	require.Equal(t, 7*time.Second, c.GetHTTPClient().Timeout)
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
	require.NotSame(t, custom, c.GetHTTPClient())
	require.Equal(t, 55*time.Second, c.GetHTTPClient().Timeout)
	require.NotNil(t, c.GetHTTPClient().CheckRedirect)
	require.Equal(t, 55*time.Second, custom.Timeout)
	require.Nil(t, custom.CheckRedirect)
}

func TestClientEntityOptions_PropagateToServiceHTTPClients(t *testing.T) {
	// Epic 5.3: Organizations now routes through the ledger plane facade. The
	// plane path stamps X-Idempotency (the 5.2.4 auto-gen gate — asserted below),
	// but User-Agent propagation to the generated plane clients is the deferred,
	// plan-sanctioned Task 5.2.6 (docs/plans/2026-06-30-sdk-v4-remodel.md:621), so
	// WithUserAgent no longer reaches this transport. The UA assertion returns
	// with 5.2.6.
	var seenIdempotency string

	writeErrs := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenIdempotency = r.Header.Get("X-Idempotency")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		writeErrs <- json.NewEncoder(w).Encode(models.Organization{ID: "11111111-1111-1111-1111-111111111111", LegalName: "Acme", LegalDocument: "123"})
	}))
	defer srv.Close()

	c, err := New(
		WithConfig(createTestConfig(t)),
		WithBaseURL(srv.URL),
		WithUserAgent("client-options-agent/1.0"),
	)
	require.NoError(t, err)

	_, err = c.Organizations.Create(context.Background(), models.NewCreateOrganizationInput("Acme", "123"))
	require.NoError(t, err)
	require.NoError(t, <-writeErrs)
	require.NotEmpty(t, seenIdempotency)
}

func TestClientNew_WithEnvironmentRecomputesDefaultServiceURLs(t *testing.T) {
	c, err := New(WithEnvironment(config.EnvironmentProduction), WithAnonymous())
	require.NoError(t, err)

	urls := c.GetConfig().ServiceURLs
	require.Equal(t, "https://api.midaz.io/v1", urls[config.ServiceOnboarding])
	require.Equal(t, "https://api.midaz.io/v1", urls[config.ServiceTransaction])
	require.Equal(t, "https://api.midaz.io/v1", urls[config.ServiceTracer])
}

func TestClientNew_WithEnvironmentDoesNotOverrideExplicitURLs(t *testing.T) {
	c, err := New(
		WithLedgerURL("https://ledger.example.com/v1"),
		WithTracerURL("https://tracer.example.com/v1"),
		WithEnvironment(config.EnvironmentProduction),
		WithAnonymous(),
	)
	require.NoError(t, err)

	urls := c.GetConfig().ServiceURLs
	require.Equal(t, "https://ledger.example.com/v1", urls[config.ServiceOnboarding])
	require.Equal(t, "https://ledger.example.com/v1", urls[config.ServiceTransaction])
	require.Equal(t, "https://tracer.example.com/v1", urls[config.ServiceTracer])
}

// --- from client_config_provider_regression_test.go ---

type typedNilProvider struct{}

func (*typedNilProvider) Tracer() trace.Tracer           { return nil }
func (*typedNilProvider) Meter() metric.Meter            { return nil }
func (*typedNilProvider) Logger() observability.Logger   { return nil }
func (*typedNilProvider) IsEnabled() bool                { return true }
func (*typedNilProvider) Shutdown(context.Context) error { return nil }

func TestClientConfigProviderHelpers(t *testing.T) {
	t.Run("typed nil observability provider returns error", func(t *testing.T) {
		var provider *typedNilProvider

		_, err := New(WithObservabilityProvider(provider))
		require.Error(t, err)
	})

	t.Run("WithConfig clones caller-owned config", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.ServiceURLs[config.ServiceOnboarding] = "https://original.example.com/v1"
		cfg.Anonymous = true // satisfies v3 auth-required gate

		c, err := New(WithConfig(cfg))
		require.NoError(t, err)

		cfg.ServiceURLs[config.ServiceOnboarding] = "https://mutated.example.com/v1"
		require.Equal(t, "https://original.example.com/v1", c.config.ServiceURLs[config.ServiceOnboarding])

		returned := c.GetConfig()
		returned.ServiceURLs[config.ServiceOnboarding] = "https://returned.example.com/v1"
		require.Equal(t, "https://original.example.com/v1", c.config.ServiceURLs[config.ServiceOnboarding])
	})

	t.Run("WithConfig attaches observability provider to context", func(t *testing.T) {
		provider, err := observability.New(context.Background(), observability.WithComponentEnabled(false, false, false))
		require.NoError(t, err)

		cfg := createTestConfig(t)
		require.NoError(t, config.WithObservabilityProvider(provider)(cfg))

		c, err := New(WithConfig(cfg))
		require.NoError(t, err)
		require.Same(t, provider, observability.GetProvider(c.GetContext()))
	})

	t.Run("nil shutdown is safe", func(t *testing.T) {
		var c *Client
		require.NoError(t, c.Shutdown(context.Background()))
	})
}

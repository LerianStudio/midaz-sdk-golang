package midaz

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientOptionsAccessorsAndConstructors(t *testing.T) {
	ctx := context.WithValue(context.Background(), testContextKey{}, "value")
	customHTTPClient := &http.Client{Timeout: 5 * time.Second}

	// v3: build a Config with retry knobs at the config layer using the
	// individual single-concern Options. The deleted v2 WithRetries(int,dur,dur)
	// macro was a 3-positional-arg shortcut; the v3 expression is more verbose
	// but every Option has exactly one concern. WithRetryOptions at the client
	// layer is the override path for retry-package knobs that don't have a
	// Config counterpart (BackoffFactor, JitterFactor, etc.).
	cfg := createTestConfig(t)
	require.NoError(t, config.WithMaxRetries(2)(cfg))
	require.NoError(t, config.WithRetryWaitMin(10*time.Millisecond)(cfg))
	require.NoError(t, config.WithRetryWaitMax(20*time.Millisecond)(cfg))

	c, err := New(
		WithConfig(cfg),
		WithContext(ctx),
		WithHTTPClient(customHTTPClient),
		WithBaseURL("https://api.example.com"),
		WithUserAgent("midaz-test/coverage"),
		WithCustomRetryPolicy(func(resp *http.Response, err error) bool {
			return err != nil || (resp != nil && resp.StatusCode == http.StatusTooManyRequests)
		}),
		// v3: WithObservability(t,m,l bool) was deleted. The replacement is
		// WithObservabilityOptions(observability.WithComponentEnabled(t,m,l)).
		// All-disabled here matches the New()-installed default; this call
		// exists to exercise the WithObservabilityOptions path without
		// changing the effective enabled state.
		WithObservabilityOptions(observability.WithComponentEnabled(false, false, false)),
	)
	require.NoError(t, err)
	require.NotNil(t, c.Entity)

	assert.Same(t, customHTTPClient, c.config.HTTPClient)
	assert.Equal(t, "midaz-test/coverage", c.config.UserAgent)
	// In v3, retries are off iff MaxRetries == 0; here we set 2 above.
	assert.Equal(t, 2, c.config.MaxRetries)
	assert.Equal(t, 10*time.Millisecond, c.config.RetryWaitMin)
	assert.Equal(t, 20*time.Millisecond, c.config.RetryWaitMax)
	assert.Equal(t, "value", c.GetContext().Value(testContextKey{}))
	assert.NotNil(t, c.GetObservabilityProvider())
	assert.NotNil(t, c.Logger())
	// In v3, WithObservabilityOptions builds a MetricsCollector whenever
	// provider.IsEnabled() returns true, regardless of which OTel components
	// (tracing/metrics/logging) are individually toggled. The collector
	// emits noop counters when the metrics component is off, so its
	// presence is harmless. The deleted v2 WithObservability(t,m,l)
	// macro short-circuited via its closure bool — that asymmetry is now
	// gone in favor of uniform construction.
	assert.NotNil(t, c.GetMetricsCollector())
	// v3: the six factory-trap methods (NewAccount/NewLedger/NewOrganization/
	// NewTransaction/NewOperation/NewAsset on *Client) were deleted —
	// they returned bare zero-value structs without engaging the API and had
	// zero production callers. Use the models package directly when you need
	// a zero-value request shape:
	//   in := &models.CreateAccountInput{...}
	assert.Equal(t, Version, c.GetVersion())

	called := false

	require.NoError(t, c.Trace("disabled-span", func(traceCtx context.Context) error {
		called = true

		assert.Equal(t, "value", traceCtx.Value(testContextKey{}))

		return nil
	}))
	assert.True(t, called)

	returnedConfig := c.GetConfiguration()
	require.NotNil(t, returnedConfig)
	returnedConfig.ServiceURLs[config.ServiceOnboarding] = "https://mutated.example.com"
	assert.NotEqual(t, "https://mutated.example.com", c.GetConfig().ServiceURLs[config.ServiceOnboarding])

	require.NoError(t, c.Shutdown(context.Background()))
}

func TestClientOptionErrorsAndNilReceivers(t *testing.T) {
	// v3: every construction error is a typed *errors.Error with
	// Category=CategoryConfiguration. The wrapped underlying cause is
	// reachable via errors.Unwrap.

	_, err := New(nil)
	require.Error(t, err)
	assert.True(t, sdkerrors.IsConfigurationError(err), "nil option should yield ErrConfiguration")
	assert.Contains(t, err.Error(), "index 0", "error should identify which option index was nil")

	_, err = New(WithConfig(nil))
	require.Error(t, err)
	assert.True(t, sdkerrors.IsConfigurationError(err))
	assert.Contains(t, errors.Unwrap(err).Error(), "config cannot be nil",
		"underlying option error should be reachable via Unwrap")

	var nilContext context.Context

	_, err = New(WithConfig(createTestConfig(t)), WithContext(nilContext))
	require.Error(t, err)
	assert.True(t, sdkerrors.IsConfigurationError(err))
	assert.Contains(t, errors.Unwrap(err).Error(), "context cannot be nil")

	_, err = New(WithConfig(createTestConfig(t)), WithBaseURL("://bad-url"))
	require.Error(t, err)
	assert.True(t, sdkerrors.IsConfigurationError(err))
	assert.Contains(t, errors.Unwrap(err).Error(), "invalid base URL")

	// v3: DisableRetries() was renamed to WithoutRetries(). Sets MaxRetries=0;
	// the EnableRetries field was deleted (one source of truth).
	c, err := New(WithConfig(createTestConfig(t)), WithoutRetries())
	require.NoError(t, err)
	assert.Equal(t, 0, c.config.MaxRetries, "WithoutRetries should set MaxRetries to 0")
	require.NotNil(t, c.Entity)

	assert.Nil(t, (*Client)(nil).GetConfiguration())
	require.NoError(t, (*Client)(nil).Shutdown(context.Background()))
	require.Error(t, (*Client)(nil).Trace("nil", func(context.Context) error { return nil }))
	require.Error(t, c.Trace("nil-callback", nil))

	expectedErr := errors.New("boom")
	assert.ErrorIs(t, c.Trace("callback-error", func(context.Context) error { return expectedErr }), expectedErr)
}

func TestClientObservabilityOptionVariants(t *testing.T) {
	provider, err := observability.New(context.Background(), observability.WithServiceName("coverage-provider"), observability.WithComponentEnabled(false, false, false))
	require.NoError(t, err)

	c, err := New(
		WithConfig(createTestConfig(t)),
		WithObservabilityProvider(provider),
		WithObservabilityOptions(observability.WithServiceName("coverage-options"), observability.WithComponentEnabled(false, false, false)),
	)
	require.NoError(t, err)
	assert.NotNil(t, c.GetObservabilityProvider())
	assert.NotNil(t, c.GetMetricsCollector())

	require.NoError(t, c.Shutdown(context.Background()))
}

func TestClientCollectorEndpointOptionCreatesProvider(t *testing.T) {
	// v3: midaz.WithCollectorEndpoint was deleted. It was sugar for the
	// equivalent observability.Option chain; users now compose explicitly.
	// This test ensures the canonical path produces a working provider +
	// metrics collector for the same input shape (collector endpoint +
	// all components enabled).
	c, err := New(
		WithConfig(createTestConfig(t)),
		WithObservabilityOptions(
			observability.WithServiceName("midaz-go-sdk"),
			observability.WithCollectorEndpoint("localhost:4317"),
			observability.WithComponentEnabled(true, true, true),
		),
	)
	require.NoError(t, err)
	assert.NotNil(t, c.GetObservabilityProvider())
	assert.NotNil(t, c.GetMetricsCollector())
}

type testContextKey struct{}

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
		WithObservability(false, false, false),
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
	assert.Nil(t, c.GetMetricsCollector())
	assert.NotNil(t, c.NewAccount())
	assert.NotNil(t, c.NewLedger())
	assert.NotNil(t, c.NewOrganization())
	assert.NotNil(t, c.NewTransaction())
	assert.NotNil(t, c.NewOperation())
	assert.NotNil(t, c.NewAsset())
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
	c, err := New(WithConfig(createTestConfig(t)), WithCollectorEndpoint("localhost:4317"))
	require.NoError(t, err)
	assert.NotNil(t, c.GetObservabilityProvider())
	assert.NotNil(t, c.GetMetricsCollector())
}

type testContextKey struct{}

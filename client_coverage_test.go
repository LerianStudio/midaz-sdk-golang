package client

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientOptionsAccessorsAndConstructors(t *testing.T) {
	ctx := context.WithValue(context.Background(), testContextKey{}, "value")
	customHTTPClient := &http.Client{Timeout: 5 * time.Second}

	c, err := New(
		WithConfig(createTestConfig(t)),
		WithContext(ctx),
		WithHTTPClient(customHTTPClient),
		WithBaseURL("https://api.example.com"),
		WithUserAgent("midaz-test/coverage"),
		WithRetries(2, 10*time.Millisecond, 20*time.Millisecond),
		WithCustomRetryPolicy(func(resp *http.Response, err error) bool {
			return err != nil || (resp != nil && resp.StatusCode == http.StatusTooManyRequests)
		}),
		WithObservability(false, false, false),
		UseAllAPIs(),
	)
	require.NoError(t, err)
	require.NotNil(t, c.Entity)

	assert.Same(t, customHTTPClient, c.config.HTTPClient)
	assert.Equal(t, "midaz-test/coverage", c.config.UserAgent)
	assert.True(t, c.config.EnableRetries)
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
	_, err := New(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "option cannot be nil")

	_, err = New(WithConfig(nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config cannot be nil")

	var nilContext context.Context

	_, err = New(WithConfig(createTestConfig(t)), WithContext(nilContext))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context cannot be nil")

	_, err = New(WithConfig(createTestConfig(t)), WithBaseURL("://bad-url"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base URL")

	c, err := New(WithConfig(createTestConfig(t)), DisableRetries(), UseEntityAPI())
	require.NoError(t, err)
	assert.False(t, c.config.EnableRetries)
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

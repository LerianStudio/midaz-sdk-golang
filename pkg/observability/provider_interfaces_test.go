package observability

import (
	"context"
	"net/http"
	"testing"

	obsmetrics "github.com/LerianStudio/lib-observability/v3/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
)

// wrappedProvider is a host Provider that embeds the SDK's own provider, the
// shape that hid *MidazProvider from the concrete-type fast paths (#255).
type wrappedProvider struct {
	*MidazProvider
}

func TestWrappedProviderReusesMetricsFactory(t *testing.T) {
	// Stands in for New with a collector endpoint, without dialing one.
	factory, err := obsmetrics.NewMetricsFactory(metricnoop.NewMeterProvider().Meter(""), nil)
	require.NoError(t, err)

	config := DefaultConfig()
	config.EnabledComponents = EnabledComponents{Metrics: true}
	wrapper := wrappedProvider{&MidazProvider{config: config, enabled: true, metricsFactory: factory}}

	first, err := metricsFactoryForProvider(wrapper)
	require.NoError(t, err)
	second, err := metricsFactoryForProvider(wrapper)
	require.NoError(t, err)

	assert.Same(t, factory, first)
	assert.Same(t, first, second)
}

func TestWrappedProviderKeepsPropagationAllowList(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithPropagationHeaders("traceparent"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, provider.Shutdown(context.Background())) })

	midazProvider, ok := provider.(*MidazProvider)
	require.True(t, ok)

	headers := http.Header{}
	headers.Set("baggage", "tenant=blocked")
	ctx := ExtractHTTPContext(WithProvider(context.Background(), wrappedProvider{midazProvider}), headers)

	assert.Empty(t, GetBaggageItem(ctx, "tenant"))
}

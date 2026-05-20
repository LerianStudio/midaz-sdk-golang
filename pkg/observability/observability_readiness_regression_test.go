package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

func TestWithCollectorInsecureDefaultsToTLS(t *testing.T) {
	cfg := DefaultConfig()
	assert.False(t, cfg.CollectorInsecure)

	require.NoError(t, WithCollectorInsecure(true)(cfg))
	assert.True(t, cfg.CollectorInsecure)
}

func TestNewRejectsInsecureCollectorInProduction(t *testing.T) {
	t.Setenv("ALLOW_INSECURE_OTEL", "")

	provider, err := New(context.Background(),
		WithEnvironment("production"),
		WithCollectorEndpoint("localhost:4317"),
		WithCollectorInsecure(true),
		WithRegisterGlobally(false),
	)
	require.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "insecure exporter")
}

func TestCreateResourceMergesOTelDefaultsAndCustomAttributes(t *testing.T) {
	provider := &MidazProvider{config: DefaultConfig()}
	provider.config.ServiceName = "custom-service"
	provider.config.Attributes = []attribute.KeyValue{
		attribute.String("custom.key", "custom-value"),
	}

	res, err := provider.createResource()
	require.NoError(t, err)

	attrs := resourceAttributes(res)
	defaultAttrs := resourceAttributes(sdkresource.Default())

	assert.Equal(t, defaultAttrs[string(semconv.TelemetrySDKLanguageKey)], attrs[string(semconv.TelemetrySDKLanguageKey)])
	assert.Equal(t, defaultAttrs[string(semconv.TelemetrySDKNameKey)], attrs[string(semconv.TelemetrySDKNameKey)])
	assert.Equal(t, "custom-service", attrs[string(semconv.ServiceNameKey)])
	assert.Equal(t, "custom-value", attrs["custom.key"])
}

func TestNewWithoutCollectorDoesNotApplyGlobalsWhenGlobalRegistrationDisabled(t *testing.T) {
	original := otel.GetTextMapPropagator()
	t.Cleanup(func() { otel.SetTextMapPropagator(original) })

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())

	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithPropagators(markerPropagator{}),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, provider.Shutdown(context.Background())) })

	assert.Empty(t, otel.GetTextMapPropagator().Fields(), "provider-local propagation must not mutate global OTel state")
	assert.Equal(t, []string{"x-marker"}, provider.(*MidazProvider).TextMapPropagator().Fields())
}

func TestNewWithoutCollectorUsesLibNoopTelemetry(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(false, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, provider.Shutdown(context.Background())) })

	midazProvider := provider.(*MidazProvider)
	assert.NotNil(t, midazProvider.telemetry)
	assert.NotNil(t, midazProvider.Meter())
}

type markerPropagator struct{}

func (markerPropagator) Inject(context.Context, propagation.TextMapCarrier) {}

func (markerPropagator) Extract(ctx context.Context, _ propagation.TextMapCarrier) context.Context {
	return ctx
}

func (markerPropagator) Fields() []string { return []string{"x-marker"} }

func resourceAttributes(res *sdkresource.Resource) map[string]string {
	attrs := make(map[string]string)
	for _, kv := range res.Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}

	return attrs
}

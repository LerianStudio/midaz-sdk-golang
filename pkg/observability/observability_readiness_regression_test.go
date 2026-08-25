package observability

import (
	"context"
	"testing"
	"time"

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

// TestSchemelessCollectorInProductionNamesTheFix covers the message quality
// layered on top of lib-observability's refusal. A scheme-less endpoint is
// exported as plaintext, so it is refused in production; the library says so
// but does not say how to satisfy the policy from the SDK's options. The error
// must name both remedies: an https:// endpoint, or a non-production
// environment.
func TestSchemelessCollectorInProductionNamesTheFix(t *testing.T) {
	t.Setenv("ALLOW_INSECURE_OTEL", "")

	provider, err := New(context.Background(),
		WithEnvironment("production"),
		WithCollectorEndpoint("otel-collector:4317"),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.Error(t, err)
	assert.Nil(t, provider)

	msg := err.Error()
	assert.Contains(t, msg, "insecure exporter", "must preserve the library's own refusal")
	assert.Contains(t, msg, "https://otel-collector:4317", "must name the TLS fix with the concrete endpoint")
	assert.Contains(t, msg, `observability.WithEnvironment("development")`, "must name the local-plaintext fix")
}

// TestHTTPSCollectorEndpointIsAcceptedInProduction is the other half: an
// explicit https:// endpoint is the documented way to get TLS, so it must
// survive the policy untouched even in production.
func TestHTTPSCollectorEndpointIsAcceptedInProduction(t *testing.T) {
	t.Setenv("ALLOW_INSECURE_OTEL", "")

	provider, err := New(context.Background(),
		WithEnvironment("production"),
		WithCollectorEndpoint("https://otel-collector:4317"),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)
	require.NotNil(t, provider)
	shutdownQuietly(t, provider)
}

// TestExplicitHTTPCollectorErrorIsNotAnnotated pins the deliberate carve-out:
// a caller who wrote http:// asked for plaintext explicitly, so the library's
// own wording stands and the scheme-less guidance must not be appended.
func TestExplicitHTTPCollectorErrorIsNotAnnotated(t *testing.T) {
	t.Setenv("ALLOW_INSECURE_OTEL", "")

	_, err := New(context.Background(),
		WithEnvironment("production"),
		WithCollectorEndpoint("http://otel-collector:4317"),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insecure exporter")
	assert.NotContains(t, err.Error(), "carries no scheme")
}

// TestAllowInsecureOTelEscapeStillWorks proves the annotation did not take
// over the policy decision: with the library's documented escape hatch set,
// a scheme-less production endpoint is accepted and nothing is annotated.
func TestAllowInsecureOTelEscapeStillWorks(t *testing.T) {
	t.Setenv("ALLOW_INSECURE_OTEL", "local collector behind a trusted mesh")

	provider, err := New(context.Background(),
		WithEnvironment("production"),
		WithCollectorEndpoint("otel-collector:4317"),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)
	require.NotNil(t, provider)
	shutdownQuietly(t, provider)
}

// shutdownQuietly releases a provider that was pointed at a collector nobody
// is listening on. Shutdown flushes pending telemetry, so it blocks for the
// exporter's full timeout and then reports the upload failure — expected here,
// and unrelated to what these tests assert. The short deadline keeps that
// flush from adding ten seconds per test.
func shutdownQuietly(t *testing.T, provider Provider) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_ = provider.Shutdown(ctx)
	})
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

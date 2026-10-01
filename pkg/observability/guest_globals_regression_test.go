package observability_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	midaz "github.com/LerianStudio/midaz-sdk-golang/v6"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	otellogglobal "go.opentelemetry.io/otel/log/global"
	lognoop "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// hostPropagator and hostLoggerProvider carry an identity of their own so the
// test can tell the host's globals apart from anything the SDK might install.
type hostPropagator struct {
	propagation.TraceContext
	owner string
}

type hostLoggerProvider struct {
	lognoop.LoggerProvider
	owner string
}

// TestGuestClientWithoutCollectorLeavesHostGlobalsAlone reproduces #253: a Midaz
// client with tracing and metrics on but no collector endpoint must leave the
// host's OTel globals in place, still receiving the host's spans and metrics.
func TestGuestClientWithoutCollectorLeavesHostGlobalsAlone(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	hostTracers := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	reader := sdkmetric.NewManualReader()
	hostMeters := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	hostProp := &hostPropagator{owner: "host"}
	hostLoggers := &hostLoggerProvider{owner: "host"}

	previousTracers, previousMeters := otel.GetTracerProvider(), otel.GetMeterProvider()
	previousProp, previousLoggers := otel.GetTextMapPropagator(), otellogglobal.GetLoggerProvider()
	t.Cleanup(func() {
		otel.SetTracerProvider(previousTracers)
		otel.SetMeterProvider(previousMeters)
		otel.SetTextMapPropagator(previousProp)
		otellogglobal.SetLoggerProvider(previousLoggers)
	})
	otel.SetTracerProvider(hostTracers)
	otel.SetMeterProvider(hostMeters)
	otel.SetTextMapPropagator(hostProp)
	otellogglobal.SetLoggerProvider(hostLoggers)

	var logs bytes.Buffer
	client, err := midaz.New(
		midaz.WithAnonymous(),
		midaz.WithObservabilityOptions(
			observability.WithComponentEnabled(true, true, true),
			observability.WithLogOutput(&logs),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, client.Shutdown(context.Background())) })

	assert.Same(t, hostTracers, otel.GetTracerProvider(), "SDK replaced the host TracerProvider")
	assert.Same(t, hostMeters, otel.GetMeterProvider(), "SDK replaced the host MeterProvider")
	assert.Same(t, hostProp, otel.GetTextMapPropagator(), "SDK replaced the host TextMapPropagator")
	assert.Same(t, hostLoggers, otellogglobal.GetLoggerProvider(), "SDK replaced the host LoggerProvider")

	ctx := context.Background()
	_, span := otel.Tracer("host").Start(ctx, "host-operation")
	span.End()

	counter, err := otel.Meter("host").Int64Counter("host.requests")
	require.NoError(t, err)
	counter.Add(ctx, 1)

	ended := spans.Ended()
	require.Len(t, ended, 1, "host span must reach the host TracerProvider")
	assert.Equal(t, "host-operation", ended[0].Name())

	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &collected))
	require.Len(t, collected.ScopeMetrics, 1, "host metric must reach the host MeterProvider")
	assert.Equal(t, "host.requests", collected.ScopeMetrics[0].Metrics[0].Name)

	assert.Equal(t, 1, strings.Count(logs.String(), `"level":"WARN"`), "exactly one warning about the missing endpoint: %s", logs.String())
}

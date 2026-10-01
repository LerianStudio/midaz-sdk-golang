package observability_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

const hostPropagatorHeader = "x-host-propagator"

// hostPropagator and hostLoggerProvider carry an identity of their own so the
// test can tell the host's globals apart from anything the SDK might install.
type hostPropagator struct {
	propagation.TraceContext
	owner string
}

// Inject marks the carrier, proving the host's propagator wrote the headers.
func (p *hostPropagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	p.TraceContext.Inject(ctx, carrier)
	carrier.Set(hostPropagatorHeader, p.owner)
}

type hostLoggerProvider struct {
	lognoop.LoggerProvider
	owner string
}

// TestGuestClientWithoutCollectorUsesHostProviders reproduces #253: a Midaz
// client with tracing and metrics on but no collector endpoint must leave the
// host's OTel globals in place and send its own spans, metrics and trace
// headers through them.
func TestGuestClientWithoutCollectorUsesHostProviders(t *testing.T) {
	tests := []struct {
		name    string
		options []observability.Option
	}{
		{name: "no endpoint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				midaz.WithObservabilityOptions(append([]observability.Option{
					observability.WithComponentEnabled(true, true, true),
					observability.WithLogOutput(&logs),
				}, tt.options...)...),
			)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, client.Shutdown(context.Background())) })

			assert.Same(t, hostTracers, otel.GetTracerProvider(), "SDK replaced the host TracerProvider")
			assert.Same(t, hostMeters, otel.GetMeterProvider(), "SDK replaced the host MeterProvider")
			assert.Same(t, hostProp, otel.GetTextMapPropagator(), "SDK replaced the host TextMapPropagator")
			assert.Same(t, hostLoggers, otellogglobal.GetLoggerProvider(), "SDK replaced the host LoggerProvider")

			var outbound http.Header
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				outbound = r.Header.Clone()
			}))
			t.Cleanup(server.Close)

			ctx, hostSpan := otel.Tracer("host").Start(context.Background(), "host-operation")
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/v1/organizations", nil)
			require.NoError(t, err)
			resp, err := observability.NewHTTPMiddleware(client.GetObservabilityProvider())(http.DefaultTransport).RoundTrip(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			hostSpan.End()

			ended := spans.Ended()
			require.Len(t, ended, 2, "the SDK span and the host span must both reach the host TracerProvider")
			sdkSpan := ended[0].SpanContext()
			assert.Equal(t, hostSpan.SpanContext().SpanID(), ended[0].Parent().SpanID(), "the SDK span must be a child of the host span")
			assert.Equal(t, fmt.Sprintf("00-%s-%s-01", sdkSpan.TraceID(), sdkSpan.SpanID()), outbound.Get("traceparent"))
			assert.Equal(t, "host", outbound.Get(hostPropagatorHeader), "the host propagator must write the outbound trace headers")

			var collected metricdata.ResourceMetrics
			require.NoError(t, reader.Collect(ctx, &collected))
			var names []string
			for _, scope := range collected.ScopeMetrics {
				for _, m := range scope.Metrics {
					names = append(names, m.Name)
				}
			}
			assert.Contains(t, names, observability.MetricRequestTotal, "SDK metrics must reach the host MeterProvider")

			assert.Equal(t, 1, strings.Count(logs.String(), `"level":"WARN"`), "exactly one warning about the missing endpoint: %s", logs.String())
		})
	}
}

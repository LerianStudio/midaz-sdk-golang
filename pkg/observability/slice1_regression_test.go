package observability

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

type regressionProvider struct {
	tracer trace.Tracer
	meter  metric.Meter
	logger Logger
}

func newRegressionProvider(recorder *tracetest.SpanRecorder) *regressionProvider {
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

	return &regressionProvider{
		tracer: tracerProvider.Tracer("slice1-regression"),
		meter:  metricnoop.NewMeterProvider().Meter("slice1-regression"),
		logger: NewNoopLogger(),
	}
}

func (p *regressionProvider) Tracer() trace.Tracer { return p.tracer }

func (p *regressionProvider) Meter() metric.Meter { return p.meter }

func (p *regressionProvider) Logger() Logger { return p.logger }

func (*regressionProvider) Shutdown(context.Context) error { return nil }

func (*regressionProvider) IsEnabled() bool { return true }

type regressionRoundTripper struct {
	response *http.Response
	err      error
}

func (r regressionRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return r.response, r.err
}

type sentinelPropagator struct{}

func (sentinelPropagator) Inject(context.Context, propagation.TextMapCarrier) {}

func (sentinelPropagator) Extract(ctx context.Context, _ propagation.TextMapCarrier) context.Context {
	return ctx
}

func (sentinelPropagator) Fields() []string { return []string{"sentinel"} }

func TestHTTPMiddleware_SanitizesSensitiveURLAndHeaders(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := newRegressionProvider(recorder)
	transport := NewHTTPMiddleware(provider)(regressionRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"X-Idempotency": []string{"response-idempotency-secret"},
			},
			Body: io.NopCloser(strings.NewReader("ok")),
		},
	})

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"https://api.example.test/v1/accounts?access_token=token-secret&api_key=api-secret&password=password-secret&document=12345678900&safe=value",
		nil,
	)
	require.NoError(t, err)
	req.Header.Set("X-Idempotency", "request-idempotency-secret")
	req.Header.Set("Idempotency-Key", "request-idempotency-key-secret")
	req.Header.Set("X-Midaz-Auto-Idempotency", "auto-idempotency-secret")
	req.Header.Set("X-Tenant-Id", "tenant-secret")
	req.Header.Set("X-Organization-Id", "organization-secret")
	req.Header.Set("Baggage", "access_token=baggage-secret")

	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NoError(t, resp.Body.Close())
	require.Len(t, recorder.Ended(), 1)

	attrs := recorder.Ended()[0].Attributes()
	observed := make([]string, 0, len(attrs)*2)

	for _, attr := range attrs {
		observed = append(observed, string(attr.Key), attr.Value.AsString())
	}

	joined := strings.Join(observed, "\n")

	assert.NotContains(t, joined, "token-secret")
	assert.NotContains(t, joined, "api-secret")
	assert.NotContains(t, joined, "password-secret")
	assert.NotContains(t, joined, "12345678900")
	assert.NotContains(t, joined, "request-idempotency-secret")
	assert.NotContains(t, joined, "request-idempotency-key-secret")
	assert.NotContains(t, joined, "auto-idempotency-secret")
	assert.NotContains(t, joined, "tenant-secret")
	assert.NotContains(t, joined, "organization-secret")
	assert.NotContains(t, joined, "baggage-secret")
	assert.Contains(t, joined, "safe=value")

	attrMap := spanAttributesToMap(recorder.Ended()[0].Attributes())
	assert.Equal(t, http.MethodGet, attrMap[KeyHTTPRequestMethod])
	assert.Equal(t, "https://api.example.test/v1/accounts?access_token=%5BREDACTED%5D&api_key=%5BREDACTED%5D&document=%5BREDACTED%5D&password=%5BREDACTED%5D&safe=value", attrMap[KeyURLFull])
	assert.Equal(t, "/v1/accounts", attrMap[KeyURLPath])
	assert.Equal(t, "https", attrMap[KeyURLScheme])
	assert.Equal(t, "api.example.test", attrMap[KeyServerAddress])
	assert.Equal(t, int64(443), attrMap[KeyServerPort])
	assert.Equal(t, int64(http.StatusOK), attrMap[KeyHTTPResponseStatusCode])
	assert.NotContains(t, attrMap, "http.method")
	assert.NotContains(t, attrMap, "http.url")
	assert.NotContains(t, attrMap, "http.host")
	assert.NotContains(t, attrMap, "http.path")
	assert.NotContains(t, attrMap, "http.status_code")
}

func TestHTTPMiddleware_SemconvErrorTypeForHTTPStatus(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := newRegressionProvider(recorder)
	transport := NewHTTPMiddleware(provider)(regressionRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("error")),
		},
	})

	req := mustRequest(t, "https://api.example.test/v1/accounts")
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NoError(t, resp.Body.Close())

	require.Len(t, recorder.Ended(), 1)
	attrs := spanAttributesToMap(recorder.Ended()[0].Attributes())
	assert.Equal(t, int64(http.StatusInternalServerError), attrs[KeyHTTPResponseStatusCode])
	assert.Equal(t, "500", attrs[KeyErrorType])
	assert.NotContains(t, attrs, "error")
}

func TestHTTPMiddleware_SemconvErrorTypeForTransportError(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := newRegressionProvider(recorder)
	transport := NewHTTPMiddleware(provider)(regressionRoundTripper{
		err: errors.New("password=hunter2 token=secret"),
	})

	_, err := transport.RoundTrip(mustRequest(t, "https://api.example.test/v1/accounts"))
	require.Error(t, err)

	require.Len(t, recorder.Ended(), 1)
	attrs := spanAttributesToMap(recorder.Ended()[0].Attributes())
	assert.Equal(t, "*errors.errorString", attrs[KeyErrorType])
	assert.NotContains(t, attrs[KeyErrorType], "hunter2")
	assert.NotContains(t, attrs[KeyErrorType], "secret")
}

func TestHTTPMiddleware_DNSDoneWithEmptyAddrs_DoesNotPanic(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := newRegressionProvider(recorder)
	_, span := provider.Tracer().Start(context.Background(), "dns-test")

	traceHooks := (&httpMiddleware{}).createClientTrace(span)
	require.NotNil(t, traceHooks.DNSDone)

	require.NotPanics(t, func() {
		traceHooks.DNSDone(httptrace.DNSDoneInfo{Err: errors.New("dns failed")})
	})
	span.End()
}

func TestHTTPMiddleware_NilSafety(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		next     http.RoundTripper
		req      *http.Request
	}{
		{
			name:     "nil provider and nil next returns safe transport",
			provider: nil,
			next:     nil,
			req:      nil,
		},
		{
			name:     "enabled provider with nil request",
			provider: newRegressionProvider(tracetest.NewSpanRecorder()),
			next:     regressionRoundTripper{response: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}},
			req:      nil,
		},
		{
			name:     "enabled provider with nil next",
			provider: newRegressionProvider(tracetest.NewSpanRecorder()),
			next:     nil,
			req:      mustRequest(t, "https://api.example.test/v1/accounts"),
		},
		{
			name:     "nil response with nil error",
			provider: newRegressionProvider(tracetest.NewSpanRecorder()),
			next:     regressionRoundTripper{},
			req:      mustRequest(t, "https://api.example.test/v1/accounts"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewHTTPMiddleware(tt.provider)(tt.next)
			require.NotNil(t, transport)
			require.NotPanics(t, func() {
				resp, roundTripErr := transport.RoundTrip(tt.req)
				if roundTripErr != nil || resp == nil || resp.Body == nil {
					return
				}

				require.NoError(t, resp.Body.Close())
			})
		})
	}
}

func TestObservabilityHelpers_NilSafety(t *testing.T) {
	require.NotPanics(t, func() {
		collector, err := NewMetricsCollector(nil)
		require.NoError(t, err)
		collector.RecordRequest(context.Background(), "op", "resource", http.StatusOK, 0)
		collector.RecordBatchRequest(context.Background(), "op", "resource", 1, 0)
		collector.RecordRetry(context.Background(), "op", "resource", 1)
		collector.NewTimer(context.Background(), "op", "resource").Stop(http.StatusOK)
	})

	require.NotPanics(t, func() {
		err := WithSpan(context.Background(), nil, "nil-provider", nil)
		require.NoError(t, err)
	})

	require.NotPanics(t, func() {
		RecordMetric(context.Background(), nil, MetricRequestTotal, 1)
		RecordDuration(context.Background(), nil, MetricRequestDuration, timeNowForRegression())
		InjectContext(context.Background(), nil)
	})
}

func TestMidazProvider_DisabledOrNilPathsReturnNoopComponents(t *testing.T) {
	provider := &MidazProvider{
		config:  &Config{EnabledComponents: EnabledComponents{Tracing: true, Metrics: true, Logging: true}},
		enabled: true,
	}

	require.NotPanics(t, func() {
		_, span := provider.Tracer().Start(context.Background(), "noop-safe")
		span.End()
		provider.Meter().Float64Counter("noop.safe.metric")
		provider.Logger().Info("noop-safe")
	})
}

func TestObservability_DefaultHelpersDoNotMutateGlobalOpenTelemetryState(t *testing.T) {
	previousTracerProvider := otel.GetTracerProvider()
	previousMeterProvider := otel.GetMeterProvider()
	previousPropagator := otel.GetTextMapPropagator()

	t.Cleanup(func() {
		otel.SetTracerProvider(previousTracerProvider)
		otel.SetMeterProvider(previousMeterProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})

	tracerProvider := tracenoop.NewTracerProvider()
	meterProvider := metricnoop.NewMeterProvider()
	propagator := sentinelPropagator{}

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagator)

	ctx, span := StartSpan(context.Background(), "default-helper")
	span.End()
	RecordSpanMetric(ctx, "default-helper.metric", 1)

	_, err := New(context.Background(), WithComponentEnabled(false, false, false))
	require.NoError(t, err)

	assert.Equal(t, tracerProvider, otel.GetTracerProvider())
	assert.Equal(t, meterProvider, otel.GetMeterProvider())
	assert.Equal(t, propagator, otel.GetTextMapPropagator())
}

func TestRecordError_SanitizesSensitiveErrorValues(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := newRegressionProvider(recorder)
	ctx, span := provider.Tracer().Start(context.Background(), "error-sanitize")

	RecordError(ctx, errors.New("request failed password=hunter2 api_key=secret-key X-Idempotency=idem-secret"), "request_failed")
	span.End()

	require.Len(t, recorder.Ended(), 1)
	ended := recorder.Ended()[0]
	assert.NotContains(t, ended.Status().Description, "hunter2")
	assert.NotContains(t, ended.Status().Description, "secret-key")
	assert.NotContains(t, ended.Status().Description, "idem-secret")

	var attrs []attribute.KeyValue
	for _, event := range ended.Events() {
		attrs = append(attrs, event.Attributes...)
	}

	joined := attrsToString(attrs)
	assert.NotContains(t, joined, "hunter2")
	assert.NotContains(t, joined, "secret-key")
	assert.NotContains(t, joined, "idem-secret")
}

// TestRecordSpanMetric_DoesNotEmitTraceOrSpanIDMetricAttributes is the
// regression guard for the metric-cardinality leak: trace.id and span.id
// must NEVER appear as attribute keys on emitted metric data points.
// Metrics are pre-aggregated; treating high-cardinality identifiers as
// metric attributes blows up the storage backend.
//
// NOTE: this test mutates the package-level defaultProvider and is NOT
// safe to run under t.Parallel(). It guards against a behaviour that
// affects every consumer of this package, so we leave it as a
// process-wide test by design.
func TestRecordSpanMetric_DoesNotEmitTraceOrSpanIDMetricAttributes(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	t.Cleanup(func() {
		_ = meterProvider.Shutdown(context.Background())
	})

	recorder := tracetest.NewSpanRecorder()
	provider := newRegressionProvider(recorder)
	provider.meter = meterProvider.Meter("slice1-regression-metrics")

	previousDefault := defaultProvider
	defaultProvider = provider

	t.Cleanup(func() { defaultProvider = previousDefault })

	ctx, span := provider.Tracer().Start(context.Background(), "metric-cardinality")
	RecordSpanMetric(ctx, "metric.cardinality", 1)
	span.End()

	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &collected))

	// Walk every emitted metric stream and assert that neither trace.id nor
	// span.id (in any spelling — dot, underscore, hyphen) appears as an
	// attribute key. The implementation strips them via filterMetricAttributes;
	// the test ensures that filter remains in place.
	forbidden := []string{"trace.id", "trace_id", "traceid", "span.id", "span_id", "spanid"}

	for _, scope := range collected.ScopeMetrics {
		for _, metricStream := range scope.Metrics {
			switch data := metricStream.Data.(type) {
			case metricdata.Sum[float64]:
				for _, dp := range data.DataPoints {
					assertNoForbiddenAttributes(t, metricStream.Name, dp.Attributes, forbidden)
				}
			case metricdata.Sum[int64]:
				for _, dp := range data.DataPoints {
					assertNoForbiddenAttributes(t, metricStream.Name, dp.Attributes, forbidden)
				}
			case metricdata.Gauge[float64]:
				for _, dp := range data.DataPoints {
					assertNoForbiddenAttributes(t, metricStream.Name, dp.Attributes, forbidden)
				}
			case metricdata.Gauge[int64]:
				for _, dp := range data.DataPoints {
					assertNoForbiddenAttributes(t, metricStream.Name, dp.Attributes, forbidden)
				}
			case metricdata.Histogram[float64]:
				for _, dp := range data.DataPoints {
					assertNoForbiddenAttributes(t, metricStream.Name, dp.Attributes, forbidden)
				}
			}
		}
	}
}

func assertNoForbiddenAttributes(t *testing.T, metricName string, attrs attribute.Set, forbidden []string) {
	t.Helper()

	iter := attrs.Iter()
	for iter.Next() {
		key := strings.ToLower(string(iter.Attribute().Key))
		for _, banned := range forbidden {
			require.NotEqualf(t, banned, key,
				"metric %q must not carry attribute %q (high-cardinality)", metricName, key)
		}
	}
}

func mustRequest(t *testing.T, rawURL string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	require.NoError(t, err)

	return req
}

func attrsToString(attrs []attribute.KeyValue) string {
	values := make([]string, 0, len(attrs)*2)
	for _, attr := range attrs {
		values = append(values, string(attr.Key), attr.Value.AsString())
	}

	return strings.Join(values, "\n")
}

func spanAttributesToMap(attrs []attribute.KeyValue) map[string]any {
	values := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		switch attr.Value.Type().String() {
		case "INT64":
			values[string(attr.Key)] = attr.Value.AsInt64()
		case "STRINGSLICE":
			values[string(attr.Key)] = attr.Value.AsStringSlice()
		default:
			values[string(attr.Key)] = attr.Value.AsString()
		}
	}

	return values
}

func timeNowForRegression() time.Time {
	return time.Now()
}

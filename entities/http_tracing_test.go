package entities

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// TestHTTPClientTracingIntegration tests tracing integration with the HTTP client
func TestHTTPClientTracingIntegration(t *testing.T) {
	t.Run("HTTPClientInjectsTraceHeaders", func(t *testing.T) {
		testHTTPClientInjectsTraceHeaders(t)
	})

	t.Run("HTTPClientTracingDisabled", func(t *testing.T) {
		testHTTPClientTracingDisabled(t)
	})

	t.Run("HTTPClientWithCustomHeaders", func(t *testing.T) {
		testHTTPClientWithCustomHeaders(t)
	})

	t.Run("HTTPClientErrorHandlingWithTracing", func(t *testing.T) {
		testHTTPClientErrorHandlingWithTracing(t)
	})
}

// testHTTPClientInjectsTraceHeaders verifies that the HTTP client automatically injects trace headers
func testHTTPClientInjectsTraceHeaders(t *testing.T) {
	t.Helper()

	// Create observability provider
	provider, err := observability.New(context.Background(),
		observability.WithServiceName("test-service"),
		observability.WithComponentEnabled(true, false, false),
		observability.WithFullTracingSampling(),
	)
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, provider.Shutdown(context.Background()))
	}()

	// Track received headers
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()

		// Return a simple JSON response
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	// Create HTTP client with observability
	httpClient := NewHTTPClient(&http.Client{
		Timeout: 10 * time.Second,
	}, "Bearer test-token", provider)

	// Start a trace
	tracer := provider.Tracer()

	ctx, span := tracer.Start(context.Background(), "test_http_request")
	defer span.End()

	// Make a request
	var result map[string]string

	err = httpClient.doRequest(ctx, "GET", server.URL+"/test", nil, nil, &result)
	require.NoError(t, err)

	// Verify trace headers were injected
	assert.NotNil(t, receivedHeaders, "Should have received headers")
	assert.NotEmpty(t, receivedHeaders.Get("traceparent"), "Should have traceparent header")
	assert.Equal(t, "application/json", receivedHeaders.Get("Accept"), "Should have Accept header")
	assert.Equal(t, "Bearer test-token", receivedHeaders.Get("Authorization"), "Should have Authorization header")

	// Verify the response was parsed correctly
	assert.Equal(t, "ok", result["status"])
}

// testHTTPClientTracingDisabled verifies behavior when tracing is disabled
func testHTTPClientTracingDisabled(t *testing.T) {
	t.Helper()

	// Create observability provider with tracing disabled
	provider, err := observability.New(context.Background(),
		observability.WithComponentEnabled(false, false, true), // Only logging enabled
	)
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, provider.Shutdown(context.Background()))
	}()

	// Track received headers
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	// Create HTTP client with observability (tracing disabled)
	httpClient := NewHTTPClient(&http.Client{
		Timeout: 10 * time.Second,
	}, "Bearer test-token", provider)

	// Make a request
	var result map[string]string

	err = httpClient.doRequest(context.Background(), "GET", server.URL+"/test", nil, nil, &result)
	require.NoError(t, err)

	// Verify no trace headers were injected when tracing is disabled
	assert.NotNil(t, receivedHeaders, "Should have received headers")
	assert.Empty(t, receivedHeaders.Get("traceparent"), "Should not have traceparent header when tracing disabled")
	assert.Equal(t, "application/json", receivedHeaders.Get("Accept"), "Should still have standard headers")
}

// testHTTPClientWithCustomHeaders verifies that custom headers work alongside tracing
func testHTTPClientWithCustomHeaders(t *testing.T) {
	t.Helper()

	// Create observability provider
	provider, err := observability.New(context.Background(),
		observability.WithComponentEnabled(true, false, false),
		observability.WithFullTracingSampling(),
	)
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, provider.Shutdown(context.Background()))
	}()

	// Track received headers
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	// Create HTTP client
	httpClient := NewHTTPClient(&http.Client{
		Timeout: 10 * time.Second,
	}, "Bearer test-token", provider)

	// Start a trace
	tracer := provider.Tracer()

	ctx, span := tracer.Start(context.Background(), "test_http_request_with_headers")
	defer span.End()

	// Custom headers
	customHeaders := map[string]string{
		"X-Request-ID":   "req-123",
		"X-Custom-Value": "custom-data",
	}

	// Make a request with custom headers
	var result map[string]string

	err = httpClient.doRequest(ctx, "POST", server.URL+"/test", customHeaders,
		map[string]string{"data": "test"}, &result)
	require.NoError(t, err)

	// Verify both trace and custom headers are present
	assert.NotNil(t, receivedHeaders, "Should have received headers")
	assert.NotEmpty(t, receivedHeaders.Get("traceparent"), "Should have traceparent header")
	assert.Equal(t, "req-123", receivedHeaders.Get("X-Request-ID"), "Should have custom header")
	assert.Equal(t, "custom-data", receivedHeaders.Get("X-Custom-Value"), "Should have custom header")
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"), "Should have content type for POST")
}

// testHTTPClientErrorHandlingWithTracing verifies error handling works with tracing
func testHTTPClientErrorHandlingWithTracing(t *testing.T) {
	t.Helper()

	// Create observability provider
	provider, err := observability.New(context.Background(),
		observability.WithComponentEnabled(true, false, false),
		observability.WithFullTracingSampling(),
	)
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, provider.Shutdown(context.Background()))
	}()

	// Server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_request",
			"message": "Test error message",
		})
	}))
	defer server.Close()

	// Create HTTP client
	httpClient := NewHTTPClient(&http.Client{
		Timeout: 10 * time.Second,
	}, "Bearer test-token", provider)

	// Start a trace
	tracer := provider.Tracer()

	ctx, span := tracer.Start(context.Background(), "test_http_request_error")
	defer span.End()

	// Make a request that will result in an error
	var result map[string]string

	err = httpClient.doRequest(ctx, "GET", server.URL+"/error", nil, nil, &result)

	// Should return an error
	require.Error(t, err, "Should return an error for 400 status code")

	// The span should still be valid and have recorded the error
	spanCtx := trace.SpanFromContext(ctx)
	assert.True(t, spanCtx.SpanContext().IsValid(), "Span context should still be valid after error")
}

// TestHTTPClientDistributedTracing tests distributed tracing across multiple HTTP calls
func TestHTTPClientDistributedTracing(t *testing.T) {
	// Create two observability providers (simulating different services)
	provider1, err := observability.New(context.Background(),
		observability.WithServiceName("service-1"),
		observability.WithComponentEnabled(true, false, false),
		observability.WithFullTracingSampling(),
	)
	require.NoError(t, err)

	defer provider1.Shutdown(context.Background())

	provider2, err := observability.New(context.Background(),
		observability.WithServiceName("service-2"),
		observability.WithComponentEnabled(true, false, false),
		observability.WithFullTracingSampling(),
	)
	require.NoError(t, err)

	defer provider2.Shutdown(context.Background())

	// Track traceparent headers received by service 2
	var receivedTraceparents []string

	service2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if traceparent header exists
		traceparent := r.Header.Get("traceparent")
		if traceparent != "" {
			receivedTraceparents = append(receivedTraceparents, traceparent)

			// Start span in service 2 with extracted context
			headers := make(map[string]string)

			for name, values := range r.Header {
				if len(values) > 0 {
					headers[name] = values[0]
				}
			}

			extractedCtx := observability.ExtractContext(r.Context(), headers)
			tracer2 := provider2.Tracer()

			_, span2 := tracer2.Start(extractedCtx, "service_2_operation")
			defer span2.End()
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"service": "2", "status": "ok"})
	}))
	defer service2.Close()

	// Service 1 HTTP client
	httpClient1 := NewHTTPClient(&http.Client{
		Timeout: 10 * time.Second,
	}, "Bearer service1-token", provider1)

	// Start trace in service 1
	tracer1 := provider1.Tracer()

	ctx, span1 := tracer1.Start(context.Background(), "service_1_operation")
	defer span1.End()

	originalTraceID := trace.SpanFromContext(ctx).SpanContext().TraceID()

	// Service 1 makes request to service 2
	var result map[string]string

	err = httpClient1.doRequest(ctx, "GET", service2.URL+"/process", nil, nil, &result)
	require.NoError(t, err)

	// Verify response
	assert.Equal(t, "2", result["service"])
	assert.Equal(t, "ok", result["status"])

	// Verify trace propagation
	require.Len(t, receivedTraceparents, 1, "Service 2 should have received one traceparent header")

	// Verify the traceparent contains the original trace ID
	traceparent := receivedTraceparents[0]
	assert.Contains(t, traceparent, originalTraceID.String(), "traceparent should contain original trace ID")

	// Verify traceparent format (should be version-traceId-spanId-flags)
	parts := strings.Split(traceparent, "-")
	require.Len(t, parts, 4, "traceparent should have 4 parts")
	assert.Equal(t, "00", parts[0], "version should be 00")
	assert.Equal(t, originalTraceID.String(), parts[1], "trace ID should match")
}

func TestHTTPClientPropagatesExtractedIncomingTraceWithRegisterGloballyFalse(t *testing.T) {
	provider, err := observability.New(context.Background(),
		observability.WithServiceName("sdk-service"),
		observability.WithComponentEnabled(true, false, false),
		observability.WithFullTracingSampling(),
		observability.WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { assert.NoError(t, provider.Shutdown(context.Background())) }()

	tracer := provider.Tracer()

	incomingCtx, incomingSpan := tracer.Start(observability.WithProvider(context.Background(), provider), "client-app-request")
	defer incomingSpan.End()

	incomingHeaders := http.Header{}
	observability.InjectHTTPContext(incomingCtx, incomingHeaders)
	extractedCtx := observability.ExtractHTTPContext(observability.WithProvider(context.Background(), provider), incomingHeaders)
	originalTraceID := trace.SpanContextFromContext(incomingCtx).TraceID().String()

	type capturedTraceHeaders struct {
		baggage     string
		traceparent string
	}

	receivedHeaders := make(chan capturedTraceHeaders, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders <- capturedTraceHeaders{
			baggage:     r.Header.Get("baggage"),
			traceparent: r.Header.Get("traceparent"),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	extractedCtx, err = observability.WithBaggageItem(extractedCtx, "tenant", "acme")
	require.NoError(t, err)

	httpClient := NewHTTPClient(server.Client(), "token", provider)

	var result map[string]string

	err = httpClient.doRequest(extractedCtx, http.MethodGet, server.URL+"/unified", nil, nil, &result)
	require.NoError(t, err)

	headers := <-receivedHeaders
	assert.Contains(t, headers.traceparent, originalTraceID)
	assert.Contains(t, headers.baggage, "tenant=acme")
}

func TestHTTPClientSDKInstrumentationCreatesSingleClientSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := newHTTPErrorResponseProvider(recorder)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "req-123")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	httpClient := NewHTTPClient(server.Client(), "token", provider)

	var result map[string]string

	err := httpClient.doRequest(context.Background(), http.MethodGet, server.URL+"/single", nil, nil, &result)
	require.NoError(t, err)

	ended := recorder.Ended()
	require.Len(t, ended, 1)
	assert.Equal(t, trace.SpanKindClient, ended[0].SpanKind())
	assert.Equal(t, "ok", result["status"])

	serverURL, parseErr := url.Parse(server.URL)
	require.NoError(t, parseErr)
	host, port, splitErr := net.SplitHostPort(serverURL.Host)
	require.NoError(t, splitErr)

	attrs := spanAttributeMap(ended[0])
	assert.Equal(t, http.MethodGet, attrs[observability.KeyHTTPRequestMethod])
	assert.Equal(t, "/single", attrs[observability.KeyURLPath])
	assert.Equal(t, server.URL+"/single", attrs[observability.KeyURLFull])
	assert.Equal(t, "http", attrs[observability.KeyURLScheme])
	assert.Equal(t, host, attrs[observability.KeyServerAddress])
	assert.Equal(t, port, attrIntString(attrs[observability.KeyServerPort]))
	assert.Equal(t, "1.1", attrs[observability.KeyNetworkProtocolVersion])
	assert.Equal(t, int64(http.StatusOK), attrs[observability.KeyHTTPResponseStatusCode])
	assert.Equal(t, []string{"req-123"}, attrs["http.response.header.x-request-id"])

	assert.NotContains(t, attrs, "http.method")
	assert.NotContains(t, attrs, "http.path")
	assert.NotContains(t, attrs, "http.url")
	assert.NotContains(t, attrs, "http.host")
	assert.NotContains(t, attrs, "http.status_code")
}

func TestHTTPClientSDKInstrumentationUsesOtherForNonStandardMethod(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := newHTTPErrorResponseProvider(recorder)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	httpClient := NewHTTPClient(server.Client(), "token", provider)
	var result map[string]string

	err := httpClient.doRequest(context.Background(), "BREW", server.URL+"/coffee", nil, nil, &result)
	require.NoError(t, err)

	ended := recorder.Ended()
	require.Len(t, ended, 1)
	attrs := spanAttributeMap(ended[0])
	assert.Equal(t, "_OTHER", attrs[observability.KeyHTTPRequestMethod])
	assert.Equal(t, "BREW", attrs["http.request.method_original"])
}

func TestHTTPClientErrorSpanRedactsSensitiveValues(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := newHTTPErrorResponseProvider(recorder)
	httpClient := NewHTTPClient(&http.Client{Transport: failingRoundTripper{}}, "token", provider)

	var result map[string]string

	err := httpClient.doRequest(context.Background(), http.MethodGet, "https://example.com/error", nil, nil, &result)
	require.Error(t, err)

	ended := recorder.Ended()
	require.Len(t, ended, 1)

	statusDescription := ended[0].Status().Description
	assert.NotContains(t, statusDescription, "super-secret")
	assert.NotContains(t, statusDescription, "idem-secret")
	assert.NotContains(t, statusDescription, "token-secret")
	assert.Contains(t, statusDescription, "[REDACTED]")
}

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, stdErrors.New("Authorization: Bearer super-secret X-Idempotency=idem-secret token=token-secret")
}

func spanAttributeMap(span sdktrace.ReadOnlySpan) map[string]any {
	attrs := map[string]any{}

	for _, attr := range span.Attributes() {
		switch attr.Value.Type().String() {
		case "INT64":
			attrs[string(attr.Key)] = attr.Value.AsInt64()
		case "STRINGSLICE":
			attrs[string(attr.Key)] = attr.Value.AsStringSlice()
		default:
			attrs[string(attr.Key)] = attr.Value.AsString()
		}
	}

	return attrs
}

func attrIntString(value any) string {
	switch v := value.(type) {
	case int64:
		return fmt.Sprintf("%d", v)
	case string:
		return v
	default:
		return ""
	}
}

// BenchmarkHTTPClientWithTracing benchmarks HTTP client performance with tracing enabled
func BenchmarkHTTPClientWithTracing(b *testing.B) {
	// Create observability provider
	provider, err := observability.New(context.Background(),
		observability.WithComponentEnabled(true, false, false),
		observability.WithTraceSampleRate(0.1), // Low sampling for benchmarks
	)
	require.NoError(b, err)

	defer provider.Shutdown(context.Background())

	// Simple test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	b.Run("WithTracing", func(b *testing.B) {
		httpClient := NewHTTPClient(&http.Client{
			Timeout: 10 * time.Second,
		}, "Bearer test-token", provider)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var result map[string]string

			err := httpClient.doRequest(context.Background(), "GET", server.URL, nil, nil, &result)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("WithoutTracing", func(b *testing.B) {
		// Disable observability
		httpClient := NewHTTPClient(&http.Client{
			Timeout: 10 * time.Second,
		}, "Bearer test-token", nil)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var result map[string]string

			err := httpClient.doRequest(context.Background(), "GET", server.URL, nil, nil, &result)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

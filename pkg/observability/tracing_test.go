package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TestTracingPropagation tests comprehensive tracing propagation scenarios
func TestTracingPropagation(t *testing.T) {
	t.Run("InjectAndExtractTraceContext", func(t *testing.T) {
		testInjectAndExtractTraceContext(t)
	})

	t.Run("HTTPMiddlewareTracePropagation", func(t *testing.T) {
		testHTTPMiddlewareTracePropagation(t)
	})

	t.Run("DistributedTracingAcrossServices", func(t *testing.T) {
		testDistributedTracingAcrossServices(t)
	})

	t.Run("TraceContextWithBaggage", func(t *testing.T) {
		testTraceContextWithBaggage(t)
	})

	t.Run("TraceContextPersistenceAcrossRequests", func(t *testing.T) {
		testTraceContextPersistenceAcrossRequests(t)
	})
}

// testInjectAndExtractTraceContext tests basic inject/extract functionality
func testInjectAndExtractTraceContext(t *testing.T) {
	// Create a provider with tracing enabled
	ctx := context.Background()
	provider, err := New(ctx,
		WithComponentEnabled(true, false, false), // Only tracing
		WithFullTracingSampling(),                // Sample all traces for testing
		WithPropagators(propagation.TraceContext{}, propagation.Baggage{}),
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, provider.Shutdown(ctx))
	}()

	// Start a parent span
	tracer := provider.Tracer()
	parentCtx, parentSpan := tracer.Start(ctx, "parent_operation")
	defer parentSpan.End()

	// Get the trace ID before injection
	originalTraceID := trace.SpanFromContext(parentCtx).SpanContext().TraceID()
	require.True(t, originalTraceID.IsValid(), "Original trace ID should be valid")

	// Inject context into headers
	headers := make(map[string]string)
	InjectContext(parentCtx, headers)

	// Verify headers are not empty
	assert.NotEmpty(t, headers, "Headers should contain trace context")

	// Should contain traceparent header
	assert.Contains(t, headers, "traceparent", "Should contain traceparent header")

	// Extract context from headers
	extractedCtx := ExtractContext(context.Background(), headers)

	// Create a child span from extracted context
	childCtx, childSpan := tracer.Start(extractedCtx, "child_operation")
	defer childSpan.End()

	// Verify trace IDs match
	extractedTraceID := trace.SpanFromContext(childCtx).SpanContext().TraceID()
	assert.Equal(t, originalTraceID, extractedTraceID, "Trace IDs should match after extract")
}

// testHTTPMiddlewareTracePropagation tests HTTP middleware tracing propagation
func testHTTPMiddlewareTracePropagation(t *testing.T) {
	// Create provider
	ctx := context.Background()
	provider, err := New(ctx,
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, provider.Shutdown(ctx))
	}()

	// Create a test server that captures request headers
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create HTTP client with middleware
	client := &http.Client{
		Transport: NewHTTPMiddleware(provider)(http.DefaultTransport),
	}

	// Start a parent span
	tracer := provider.Tracer()
	requestCtx, span := tracer.Start(ctx, "http_request_test")
	defer span.End()

	// Make HTTP request
	req, err := http.NewRequestWithContext(requestCtx, "GET", server.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify trace headers were injected
	assert.NotNil(t, capturedHeaders, "Headers should be captured")
	assert.NotEmpty(t, capturedHeaders.Get("traceparent"), "Should have traceparent header")

	// Parse the traceparent header to verify it contains valid trace info
	traceparent := capturedHeaders.Get("traceparent")
	parts := strings.Split(traceparent, "-")
	assert.Len(t, parts, 4, "traceparent should have 4 parts")
	assert.Equal(t, "00", parts[0], "Should use version 00")
}

// testDistributedTracingAcrossServices simulates distributed tracing across services
func testDistributedTracingAcrossServices(t *testing.T) {
	// Service A provider
	ctxA := context.Background()
	providerA, err := New(ctxA,
		WithServiceName("service-a"),
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, providerA.Shutdown(ctxA))
	}()

	// Service B provider
	ctxB := context.Background()
	providerB, err := New(ctxB,
		WithServiceName("service-b"),
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, providerB.Shutdown(ctxB))
	}()

	// Service A starts operation
	tracerA := providerA.Tracer()
	ctxA, spanA := tracerA.Start(ctxA, "service_a_operation")
	defer spanA.End()

	// Simulate service A making request to service B
	headers := make(map[string]string)
	InjectContext(ctxA, headers)

	// Service B receives request and extracts context
	incomingCtx := ExtractContext(ctxB, headers)

	// Service B starts its own operation with extracted context
	tracerB := providerB.Tracer()
	ctxBWithTrace, spanB := tracerB.Start(incomingCtx, "service_b_operation")
	defer spanB.End()

	// Verify both spans belong to same trace
	traceIdA := trace.SpanFromContext(ctxA).SpanContext().TraceID()
	traceIdB := trace.SpanFromContext(ctxBWithTrace).SpanContext().TraceID()

	assert.Equal(t, traceIdA, traceIdB, "Both services should share the same trace ID")
	assert.True(t, traceIdA.IsValid(), "Trace ID should be valid")
}

// testTraceContextWithBaggage tests baggage propagation alongside trace context
func testTraceContextWithBaggage(t *testing.T) {
	ctx := context.Background()
	provider, err := New(ctx,
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
		WithPropagators(propagation.TraceContext{}, propagation.Baggage{}),
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, provider.Shutdown(ctx))
	}()

	// Start span and add baggage
	tracer := provider.Tracer()
	ctx, span := tracer.Start(ctx, "baggage_test")
	defer span.End()

	// Add baggage item
	ctx, err = WithBaggageItem(ctx, "user-id", "123")
	require.NoError(t, err)

	ctx, err = WithBaggageItem(ctx, "request-id", "req-456")
	require.NoError(t, err)

	// Inject context into headers
	headers := make(map[string]string)
	InjectContext(ctx, headers)

	// Should have both trace and baggage headers
	assert.Contains(t, headers, "traceparent", "Should have traceparent header")
	assert.Contains(t, headers, "baggage", "Should have baggage header")

	// Extract context and verify baggage
	extractedCtx := ExtractContext(context.Background(), headers)

	userID := GetBaggageItem(extractedCtx, "user-id")
	requestID := GetBaggageItem(extractedCtx, "request-id")

	assert.Equal(t, "123", userID, "User ID should be preserved in baggage")
	assert.Equal(t, "req-456", requestID, "Request ID should be preserved in baggage")
}

// testTraceContextPersistenceAcrossRequests tests that trace context persists across multiple HTTP requests
func testTraceContextPersistenceAcrossRequests(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, provider.Shutdown(context.Background()))
	}()

	// Track traceparent headers received by the server
	var receivedTraceparents []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for traceparent header directly
		traceparent := r.Header.Get("traceparent")
		if traceparent != "" {
			receivedTraceparents = append(receivedTraceparents, traceparent)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create HTTP client with middleware
	client := &http.Client{
		Transport: NewHTTPMiddleware(provider)(http.DefaultTransport),
	}

	// Start a parent trace
	tracer := provider.Tracer()
	ctx, parentSpan := tracer.Start(context.Background(), "multiple_requests_test")
	defer parentSpan.End()

	originalTraceID := trace.SpanFromContext(ctx).SpanContext().TraceID()

	// Make multiple HTTP requests with the same context
	for i := 0; i < 3; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
	}

	// Verify all requests had traceparent headers with the same trace ID
	assert.Len(t, receivedTraceparents, 3, "Should have received 3 traceparent headers")

	for i, traceparent := range receivedTraceparents {
		assert.NotEmpty(t, traceparent, "traceparent %d should not be empty", i)
		assert.Contains(t, traceparent, originalTraceID.String(), "traceparent %d should contain original trace ID", i)
	}
}

// BenchmarkTracePropagation benchmarks the performance of trace propagation
func BenchmarkTracePropagation(b *testing.B) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithTraceSampleRate(0.1), // Low sampling for benchmarks
	)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		provider.Shutdown(context.Background())
	}()

	tracer := provider.Tracer()
	ctx, span := tracer.Start(context.Background(), "benchmark_operation")
	defer span.End()

	b.Run("InjectContext", func(b *testing.B) {
		headers := make(map[string]string)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// Clear headers for each iteration
			for k := range headers {
				delete(headers, k)
			}
			InjectContext(ctx, headers)
		}
	})

	b.Run("ExtractContext", func(b *testing.B) {
		// Pre-populate headers
		headers := make(map[string]string)
		InjectContext(ctx, headers)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			ExtractContext(context.Background(), headers)
		}
	})

	b.Run("InjectAndExtract", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			headers := make(map[string]string)
			InjectContext(ctx, headers)
			ExtractContext(context.Background(), headers)
		}
	})
}

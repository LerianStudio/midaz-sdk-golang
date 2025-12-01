package observability

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TestSimple is a simple test to ensure the package builds properly.
// This is just a placeholder test to ensure the package compiles.
func TestSimple(t *testing.T) {
	t.Log("Simple test passed")
}

// TestObservabilityIntegration demonstrates the basic integration of the observability package.
//
// This test shows the minimal setup required to use the observability package:
// 1. Create a provider with default options
// 2. Get a logger and write logs
// 3. Get a tracer and create spans
// 4. Properly shut down the provider
//
// In a real application, you would typically create a provider during initialization
// and use it throughout your application's lifecycle.
func TestObservabilityIntegration(t *testing.T) {
	// Create a provider with default options (no options means use defaults)
	// The default provider enables all components (tracing, metrics, logging)
	provider, err := New(context.Background())
	if err != nil {
		t.Fatalf("Failed to create provider with default options: %v", err)
	}

	if !provider.IsEnabled() {
		t.Fatal("Provider should be enabled by default")
	}

	// Get a logger and make sure it doesn't crash
	// In a real application, you would use this logger for all logging
	logger := provider.Logger()
	logger.Info("Test log message")

	// Get a tracer and make sure it doesn't crash
	// In a real application, you would use this tracer to create spans
	// for tracking operations and their timing
	tracer := provider.Tracer()
	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	// Make sure we can shut down cleanly
	// Always shut down the provider when your application exits
	err = provider.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Failed to shutdown provider: %v", err)
	}

	if provider.IsEnabled() {
		t.Fatal("Provider should be disabled after shutdown")
	}
}

// TestObservabilityWithAllOptions demonstrates configuring the observability package
// with all available options.
//
// This test shows how to:
// 1. Configure the provider with custom options for each component
// 2. Customize logging behavior (output, level, format)
// 3. Configure tracing with custom options
// 4. Set up metrics with custom options
//
// Use this pattern when you need fine-grained control over your observability setup,
// such as in production environments where you want to send telemetry to specific backends.
func TestObservabilityWithAllOptions(t *testing.T) {
	// Create a buffer to capture log output for testing
	var logBuffer bytes.Buffer

	// Create a provider with all options configured
	// In a real application, you would configure these based on your requirements
	provider, err := New(context.Background(),
		// Configure service information
		WithServiceName("test-service"),
		WithServiceVersion("1.0.0"),
		WithSDKVersion("1.0.0"),
		WithEnvironment("test"),

		// Configure component enablement
		WithComponentEnabled(true, true, true), // tracing, metrics, logging

		// Configure logging options
		WithLogLevel(DebugLevel),
		WithLogOutput(&logBuffer),

		// Configure tracing options
		WithHighTracingSampling(),
		WithPropagators(propagation.TraceContext{}),

		// Add custom attributes to all telemetry
		WithAttributes(
			attribute.String("test.attribute", "value"),
			attribute.Int("test.version", 1),
		),
	)
	if err != nil {
		t.Fatalf("Failed to create provider with all options: %v", err)
	}

	// Verify the provider is enabled
	if !provider.IsEnabled() {
		t.Fatal("Provider should be enabled")
	}

	// Use the logger and verify output format
	logger := provider.Logger()
	logger.Info("Test log message with all options")

	// Verify log output contains expected data
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "Test log message with all options") {
		t.Errorf("Log output does not contain expected message: %s", logOutput)
	}

	// Clean up
	err = provider.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Failed to shutdown provider: %v", err)
	}
}

// TestMetricsCollectorFunctionality demonstrates how to use the metrics functionality
// to record various metrics about your application.
//
// This test shows how to:
// 1. Record API request metrics (counts, durations, status codes)
// 2. Record custom metrics
//
// In a real application, you would use these metrics to monitor:
// - API performance and error rates
// - Custom business metrics
func TestMetricsCollectorFunctionality(t *testing.T) {
	// Create a context and provider
	ctx := context.Background()

	provider, err := New(ctx, WithComponentEnabled(true, true, true))
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Get the meter
	meter := provider.Meter()
	if meter == nil {
		t.Fatal("Meter should not be nil when metrics are enabled")
	}

	// Record metrics using the utility functions
	// In a real application, you would call these after handling operations
	RecordMetric(
		ctx,
		provider,
		MetricRequestTotal,
		1.0,
		attribute.String("operation", "GET"),
		attribute.String("resource", "/users"),
		attribute.Int("status", 200),
	)

	// Record duration metrics
	start := time.Now().Add(-100 * time.Millisecond) // Simulate 100ms duration
	RecordDuration(
		ctx,
		provider,
		MetricRequestDuration,
		start,
		attribute.String("operation", "POST"),
		attribute.String("resource", "/orders"),
	)

	// Clean up
	err = provider.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Failed to shutdown provider: %v", err)
	}
}

// TestSpanUtilities demonstrates how to use the span utility functions
// to create and manage spans for tracing operations.
//
// This test shows how to:
// 1. Start spans and create a trace context
// 2. Add attributes to spans for additional context
// 3. Record errors in spans
// 4. Add events to spans for detailed timing information
//
// In a real application, you would use these functions to trace
// the execution flow and performance of critical operations.
func TestSpanUtilities(t *testing.T) {
	// Create a context
	ctx := context.Background()

	// Start a new span
	// In a real application, you would wrap operations with spans
	// to track their execution and timing
	ctx, span := StartSpan(ctx, "parent_operation")

	// Add attributes to the span
	// This provides additional context for analysis
	AddAttribute(ctx, "service", "payment")
	AddAttribute(ctx, "environment", "test")

	// Start a child span for a sub-operation
	// This creates a hierarchy of operations for better tracing
	childCtx, childSpan := StartSpan(ctx, "child_operation")

	// Simulate an error in the child operation
	// This helps identify where failures occur
	err := errors.New("simulated error")
	RecordError(childCtx, err, "operation_failed", map[string]string{
		"operation": "payment_processing",
		"attempt":   "1",
	})

	// End the child span
	childSpan.End()

	// Add an event to the parent span
	AddEvent(ctx, "child_operation_completed", map[string]string{
		"status": "error",
		"reason": "payment_failed",
	})

	// End the parent span
	span.End()

	// Verify the context contains a span
	if !trace.SpanContextFromContext(childCtx).IsValid() {
		t.Fatal("Expected valid span context")
	}
}

// TestLoggingConfiguration demonstrates how to configure and use
// the logging component of the observability package.
//
// This test shows how to:
// 1. Configure logging with different outputs
// 2. Use different log levels
// 3. Log structured data
//
// In a real application, you would configure logging based on
// your environment (development vs. production) and requirements.
func TestLoggingConfiguration(t *testing.T) {
	// Create a buffer to capture log output
	var logBuffer bytes.Buffer

	// Create a provider with custom logging configuration
	ctx := context.Background()

	provider, err := New(ctx,
		WithComponentEnabled(true, false, true), // tracing, metrics, logging
		WithLogLevel(DebugLevel),
		WithLogOutput(&logBuffer),
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Get the logger
	logger := provider.Logger()

	// Log messages at different levels
	// In a real application, use appropriate levels based on importance
	logger.Debug("Debug message with details", "key", "value")
	logger.Info("Informational message about normal operation")
	logger.Warn("Warning about potential issue", "problem", "slow response")
	logger.Error("Error occurred", "error", "connection failed")

	// Verify log output contains expected data
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "Debug message") {
		t.Errorf("Log output missing debug message: %s", logOutput)
	}

	// Clean up
	err = provider.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Failed to shutdown provider: %v", err)
	}
}

// TestContextPropagation demonstrates how to propagate trace context
// across service boundaries, such as in microservice architectures.
//
// This test shows how to:
// 1. Extract trace context from incoming requests
// 2. Inject trace context into outgoing requests
// 3. Maintain trace continuity across service boundaries
//
// In a real application, this ensures distributed traces remain
// connected as requests flow through your system.
func TestContextPropagation(t *testing.T) {
	// Create a provider with propagation configured
	ctx := context.Background()

	provider, err := New(ctx,
		WithComponentEnabled(true, false, false), // Only enable tracing
		WithPropagators(propagation.TraceContext{}),
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Start a span to create some trace context
	tracer := provider.Tracer()

	ctx, span := tracer.Start(ctx, "parent_service_operation")
	defer span.End()

	// Simulate extracting context from an incoming request
	// In a real application, this would come from HTTP headers
	headers := make(map[string]string)

	// Inject the current context into headers
	// This is what you would do before making an outgoing request
	InjectContext(ctx, headers)

	// Verify headers contain trace information
	if len(headers) == 0 {
		t.Fatal("Expected trace context in headers")
	}

	// Extract the context from headers
	// This is what a receiving service would do
	extractedCtx := ExtractContext(context.Background(), headers)

	// Start a span in the "receiving" service
	// This will be connected to the original trace
	_, childSpan := tracer.Start(extractedCtx, "child_service_operation")
	childSpan.End()

	// Clean up
	err = provider.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Failed to shutdown provider: %v", err)
	}
}

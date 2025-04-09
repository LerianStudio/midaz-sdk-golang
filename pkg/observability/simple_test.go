package observability

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
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
	// to ensure all data is flushed to exporters
	err = provider.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Failed to shutdown provider: %v", err)
	}
	if provider.IsEnabled() {
		t.Fatal("Provider should be disabled after shutdown")
	}
}

// TestObservabilityWithAllOptions demonstrates how to configure the observability provider
// with all available options.
//
// This test shows:
// 1. How to create a fully configured provider with custom settings
// 2. How to use different log levels
// 3. How to capture log output for verification
// 4. How to create and use spans with attributes
// 5. How to use the helper functions for metrics and spans
//
// In a real application, you would select the options that make sense for your
// specific requirements and environment.
func TestObservabilityWithAllOptions(t *testing.T) {
	// Create a buffer to capture log output
	// This is useful for testing, but in a real application
	// you would typically log to a file or standard output
	var logBuffer bytes.Buffer

	// Create a context with timeout for safety
	// In a real application, you might use a context from your
	// application's main context or a request context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a provider with all available options
	// This demonstrates the full range of configuration options available
	provider, err := New(ctx,
		// Basic service information
		WithServiceName("test-service"),          // Name of your service
		WithServiceVersion("1.0.0"),              // Version of your service
		WithSDKVersion("2.0.0"),                  // Version of the SDK
		WithEnvironment("test"),                  // Environment (test, dev, prod)
		
		// Collector configuration
		WithCollectorEndpoint("localhost:4317"),  // OpenTelemetry collector endpoint
		
		// Logging configuration
		WithLogLevel(DebugLevel),                 // Set minimum log level
		WithLogOutput(&logBuffer),                // Where to write logs
		
		// Component enablement
		WithComponentEnabled(true, true, true),   // Enable tracing, metrics, logging
		
		// Additional attributes for all telemetry
		WithAttributes(
			attribute.String("test.attribute", "value"),
			attribute.Int("test.count", 42),
		),
		
		// Context propagation configuration
		WithPropagators(propagation.TraceContext{}),
		WithPropagationHeaders("X-Request-ID", "X-Correlation-ID"),
		
		// Sampling configuration
		WithFullTracingSampling(),                // Sample 100% of traces
	)

	if err != nil {
		t.Fatalf("Failed to create provider with all options: %v", err)
	}

	// Verify the provider is enabled
	if !provider.IsEnabled() {
		t.Fatal("Provider should be enabled")
	}

	// Test logger with different log levels
	// In a real application, use the appropriate level based on importance:
	// - Debug: Detailed information for debugging
	// - Info: General operational information
	// - Warn: Warning conditions that don't cause errors
	// - Error: Error conditions that should be investigated
	// - Fatal: Severe errors that cause the application to terminate
	logger := provider.Logger()
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warning message")
	logger.Error("Error message")
	// Don't test Fatal as it would exit the program

	// Test formatted logging
	// Use formatted logging when you need to include variables in your log messages
	logger.Debugf("Debug message with %s", "formatting")
	logger.Infof("Info message with %d", 42)
	logger.Warnf("Warning message with %.2f", 3.14)
	logger.Errorf("Error message with %v", []string{"multiple", "values"})

	// Test structured logging with fields
	// Structured logging is powerful for adding context to your logs
	// that can be used for filtering and analysis
	structuredLogger := logger.With(map[string]interface{}{
		"request_id": "req-123",
		"user_id":    "user-456",
	})
	structuredLogger.Info("Structured log message")

	// Verify log output contains expected content
	logOutput := logBuffer.String()
	t.Logf("Log output: %s", logOutput)
	
	// Check for expected log entries
	if !strings.Contains(logOutput, "Debug message") {
		t.Error("Log output should contain debug message")
	}
	if !strings.Contains(logOutput, "Info message") {
		t.Error("Log output should contain info message")
	}
	if !strings.Contains(logOutput, "Warning message") {
		t.Error("Log output should contain warning message")
	}
	if !strings.Contains(logOutput, "Error message") {
		t.Error("Log output should contain error message")
	}
	if !strings.Contains(logOutput, "request_id") {
		t.Error("Log output should contain structured field 'request_id'")
	}

	// Test tracing
	// Tracing is used to track the flow of requests through your application
	// and measure performance of operations
	tracer := provider.Tracer()
	ctx, span := tracer.Start(ctx, "test-parent-span")
	
	// Add attributes to span
	// Attributes provide additional context to spans and can be used
	// for filtering and analysis in your observability platform
	span.SetAttributes(
		attribute.String("span.attribute", "value"),
		attribute.Int64("span.count", 100),
	)
	
	// Create a child span
	// Child spans represent nested operations and help visualize
	// the hierarchy of operations in your application
	_, childSpan := tracer.Start(ctx, "test-child-span")
	childSpan.SetAttributes(attribute.String("child", "true"))
	childSpan.End()
	
	// End parent span
	// Always remember to end spans when operations complete
	span.End()

	// Test metrics
	// Metrics are used to measure and monitor the performance
	// and behavior of your application
	meter := provider.Meter()
	if meter == nil {
		t.Fatal("Meter should not be nil")
	}

	// Test helper functions
	// Helper functions make it easier to use observability features
	// in your application code
	testCtx := context.Background()
	err = WithSpan(testCtx, provider, "helper-function-span", func(ctx context.Context) error {
		// Record a metric within the span
		// This associates the metric with the current trace
		RecordMetric(ctx, provider, "test.counter", 1.0, 
			attribute.String("operation", "test"),
		)
		
		// Record duration
		// This is useful for measuring the duration of operations
		start := time.Now()
		time.Sleep(10 * time.Millisecond)
		RecordDuration(ctx, provider, "test.duration", start,
			attribute.String("operation", "test"),
		)
		
		return nil
	})
	
	if err != nil {
		t.Fatalf("WithSpan helper function failed: %v", err)
	}

	// Test context propagation
	// Context propagation is essential for distributed tracing
	// across service boundaries
	headers := make(map[string]string)
	InjectContext(ctx, headers)
	
	if len(headers) == 0 {
		t.Error("Context injection should add trace headers")
	}
	
	extractedCtx := ExtractContext(context.Background(), headers)
	if extractedCtx == nil {
		t.Error("Context extraction should not return nil")
	}

	// Shutdown the provider
	// Always shut down the provider when your application exits
	err = provider.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Failed to shutdown provider: %v", err)
	}
	if provider.IsEnabled() {
		t.Fatal("Provider should be disabled after shutdown")
	}
}

// TestDevelopmentDefaults demonstrates using the development defaults option.
//
// The development defaults are optimized for local development:
// - High trace sampling rate (0.5)
// - Debug log level for detailed logging
// - "development" environment setting
//
// Use this configuration during development to get detailed observability
// data without having to specify all options individually.
func TestDevelopmentDefaults(t *testing.T) {
	ctx := context.Background()
	
	// WithDevelopmentDefaults provides a convenient way to set up
	// observability for development environments
	provider, err := New(ctx, WithDevelopmentDefaults())
	if err != nil {
		t.Fatalf("Failed to create provider with development defaults: %v", err)
	}
	defer provider.Shutdown(ctx)
	
	if !provider.IsEnabled() {
		t.Fatal("Provider should be enabled")
	}
	
	// Development defaults should enable debug logging
	// This is useful for seeing detailed information during development
	logger := provider.Logger()
	logger.Debug("This debug message should be logged in development mode")
}

// TestProductionDefaults demonstrates using the production defaults option.
//
// The production defaults are optimized for production environments:
// - Low trace sampling rate (0.1) to reduce overhead
// - Info log level for important but not excessive logging
// - "production" environment setting
//
// Use this configuration in production to get essential observability
// data without excessive overhead.
func TestProductionDefaults(t *testing.T) {
	ctx := context.Background()
	
	// Capture log output to verify log levels
	var logBuffer bytes.Buffer
	
	// WithProductionDefaults provides a convenient way to set up
	// observability for production environments
	provider, err := New(ctx, 
		WithProductionDefaults(),
		WithLogOutput(&logBuffer),
	)
	if err != nil {
		t.Fatalf("Failed to create provider with production defaults: %v", err)
	}
	defer provider.Shutdown(ctx)
	
	if !provider.IsEnabled() {
		t.Fatal("Provider should be enabled")
	}
	
	// Production defaults should set log level to Info
	// This means debug logs won't be output, reducing noise in production
	logger := provider.Logger()
	logger.Debug("This debug message should NOT be logged in production mode")
	logger.Info("This info message should be logged in production mode")
	
	logOutput := logBuffer.String()
	if strings.Contains(logOutput, "This debug message should NOT be logged") {
		t.Error("Debug messages should not be logged with production defaults")
	}
	if !strings.Contains(logOutput, "This info message should be logged") {
		t.Error("Info messages should be logged with production defaults")
	}
}

// TestNoopLogger demonstrates the no-op logger implementation.
//
// The no-op logger is useful when you want to disable logging
// but maintain the same API. All operations are no-ops that do nothing.
//
// This is useful for:
// - Testing where you don't want logs to interfere
// - Environments where logging is not needed or desired
// - Creating mock implementations for testing
func TestNoopLogger(t *testing.T) {
	// Create a no-op logger that doesn't actually log anything
	logger := NewNoopLogger()
	
	// None of these should crash or produce output
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warning message")
	logger.Error("Error message")
	logger.Debugf("Debug message with %s", "formatting")
	logger.Infof("Info message with %d", 42)
	logger.Warnf("Warning message with %.2f", 3.14)
	logger.Errorf("Error message with %v", []string{"multiple", "values"})
	
	// With methods should return a logger
	// This ensures the API remains consistent even with no-op loggers
	structuredLogger := logger.With(map[string]interface{}{
		"request_id": "req-123",
	})
	if structuredLogger == nil {
		t.Fatal("With() should return a non-nil logger")
	}
	
	// WithContext should return a logger
	spanCtx := trace.SpanContext{}
	contextLogger := logger.WithContext(spanCtx)
	if contextLogger == nil {
		t.Fatal("WithContext() should return a non-nil logger")
	}
	
	// WithSpan should return a logger
	spanLogger := logger.WithSpan(nil)
	if spanLogger == nil {
		t.Fatal("WithSpan() should return a non-nil logger")
	}
}

// TestCustomLogOutput demonstrates how to use a custom log output destination.
//
// This shows how to:
// - Direct logs to multiple destinations simultaneously
// - Capture logs for analysis or testing
// - Configure custom log output destinations
//
// In real applications, you might want to send logs to a file,
// a log aggregation service, or multiple destinations at once.
func TestCustomLogOutput(t *testing.T) {
	// Create a multi-writer that writes to both a buffer and os.Stderr
	// This is useful when you want logs to go to multiple destinations
	var buffer bytes.Buffer
	multiWriter := io.MultiWriter(&buffer, os.Stderr)
	
	ctx := context.Background()
	provider, err := New(ctx, 
		WithLogOutput(multiWriter), // Set custom output destination
		WithLogLevel(InfoLevel),    // Only log info and above
	)
	if err != nil {
		t.Fatalf("Failed to create provider with custom log output: %v", err)
	}
	defer provider.Shutdown(ctx)
	
	logger := provider.Logger()
	logger.Info("This message should go to both buffer and stderr")
	
	if !strings.Contains(buffer.String(), "This message should go to both buffer and stderr") {
		t.Error("Log message should be written to the buffer")
	}
}

// TestLoggerWithContext demonstrates how to add trace context to logs.
//
// This shows how to:
// - Correlate logs with traces by adding trace IDs to log entries
// - Create a logger that automatically includes trace context
// - Use the WithContext method to add context to loggers
//
// This is essential for correlating logs with traces in distributed systems.
func TestLoggerWithContext(t *testing.T) {
	var buffer bytes.Buffer
	
	ctx := context.Background()
	provider, err := New(ctx, WithLogOutput(&buffer))
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Shutdown(ctx)
	
	// Create a span to get a valid SpanContext
	// In a real application, this would come from an incoming request
	// or be created at the start of an operation
	tracer := provider.Tracer()
	_, span := tracer.Start(ctx, "test-span")
	defer span.End()
	
	// Get the span context
	// This contains the trace ID and span ID
	spanCtx := span.SpanContext()
	
	// Create a logger with the span context
	// This will automatically add trace_id and span_id fields to all log entries
	logger := provider.Logger().WithContext(spanCtx)
	logger.Info("Log with trace context")
	
	logOutput := buffer.String()
	if !strings.Contains(logOutput, "trace_id") {
		t.Error("Log output should contain trace_id field")
	}
	if !strings.Contains(logOutput, "span_id") {
		t.Error("Log output should contain span_id field")
	}
}

// TestDisabledComponents demonstrates how to selectively enable or disable
// observability components (tracing, metrics, logging).
//
// This shows how to:
// - Create a provider with only specific components enabled
// - Use the WithComponentEnabled option to control which features are active
// - Work with disabled components safely
//
// This is useful when you want to use only certain observability features
// or when you want to minimize overhead in specific environments.
func TestDisabledComponents(t *testing.T) {
	ctx := context.Background()
	
	// Create a provider with only logging enabled
	// This is useful when you only need logging but not tracing or metrics
	provider, err := New(ctx, WithComponentEnabled(false, false, true))
	if err != nil {
		t.Fatalf("Failed to create provider with disabled components: %v", err)
	}
	defer provider.Shutdown(ctx)
	
	// Logger should still work
	logger := provider.Logger()
	logger.Info("This should work even with tracing and metrics disabled")
	
	// Tracer and meter should return no-op implementations
	// This ensures your code doesn't crash even when these components are disabled
	tracer := provider.Tracer()
	_, span := tracer.Start(ctx, "this-should-be-noop")
	span.End()
	
	meter := provider.Meter()
	if meter == nil {
		t.Fatal("Meter should not be nil even when disabled")
	}
}

// TestMetricsCollectorFunctionality demonstrates how to use the MetricsCollector
// to record various types of metrics.
//
// This shows how to:
// - Create and use a MetricsCollector
// - Record API requests with timing and status information
// - Record batch operations with size and latency
// - Record retry attempts
// - Use timers for measuring operation durations
//
// The MetricsCollector provides a high-level API for common metrics
// patterns in service applications.
func TestMetricsCollectorFunctionality(t *testing.T) {
	ctx := context.Background()
	
	// Create a provider with metrics enabled
	// In a real application, you would typically enable all components
	provider, err := New(ctx, 
		WithServiceName("test-metrics-service"),
		WithComponentEnabled(false, true, true), // Only enable metrics and logging
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Shutdown(ctx)
	
	// Create a metrics collector
	// This provides convenience methods for recording common metrics
	collector, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	
	// Test RecordRequest
	// Use this to record API requests with their result and duration
	collector.RecordRequest(
		ctx,
		"test-operation",      // Name of the operation
		"test-resource",       // Type of resource being operated on
		200,                   // HTTP status code
		100*time.Millisecond,  // Duration of the request
		attribute.String("test.attribute", "value"), // Additional attributes
	)
	
	// Test RecordRequest with error status
	// This will increment error counters in addition to request counters
	collector.RecordRequest(
		ctx,
		"test-operation-error",
		"test-resource",
		500,                   // Error status code
		150*time.Millisecond,
		attribute.String("test.attribute", "error"),
	)
	
	// Test RecordBatchRequest
	// Use this to record batch operations with their size and latency
	collector.RecordBatchRequest(
		ctx,
		"test-batch-operation",
		"test-resource",
		10,                    // Batch size
		200*time.Millisecond,  // Duration of the batch operation
		attribute.String("batch.attribute", "value"),
	)
	
	// Test RecordRetry
	// Use this to record retry attempts for operations
	collector.RecordRetry(
		ctx,
		"test-retry-operation",
		"test-resource",
		3,                     // Retry attempt number
		attribute.String("retry.attribute", "value"),
	)
	
	// Test Timer for request
	// Timers provide a convenient way to measure operation duration
	timer := collector.NewTimer(ctx, "test-timer-operation", "test-resource")
	time.Sleep(50 * time.Millisecond) // Simulate work
	timer.Stop(200, attribute.String("timer.attribute", "value"))
	
	// Test Timer for batch
	// Use StopBatch for batch operations to record both duration and batch size
	batchTimer := collector.NewTimer(ctx, "test-batch-timer", "test-resource")
	time.Sleep(50 * time.Millisecond) // Simulate work
	batchTimer.StopBatch(5, attribute.String("batch.timer.attribute", "value"))
}

// TestSpanFunctions demonstrates how to use the span utility functions
// for adding context to traces.
//
// This shows how to:
// - Create spans using the utility functions
// - Add attributes of different types to spans
// - Record errors with additional context
// - Add events to spans
// - Record metrics associated with spans
//
// These utility functions make it easier to work with spans in your code.
func TestSpanFunctions(t *testing.T) {
	ctx := context.Background()
	
	// Start a span using the utility function
	// This uses the default provider created during package initialization
	ctx, span := StartSpan(ctx, "test-utility-span")
	defer span.End()
	
	// Add attributes using utility function
	// This handles type conversion automatically
	AddAttribute(ctx, "string-attribute", "string-value")
	AddAttribute(ctx, "int-attribute", 42)
	AddAttribute(ctx, "float-attribute", 3.14)
	AddAttribute(ctx, "bool-attribute", true)
	AddAttribute(ctx, "complex-attribute", struct{ Name string }{"test"})
	
	// Record an error
	// This adds error information to the span and sets the status to Error
	testErr := fmt.Errorf("test error")
	RecordError(ctx, testErr, "test-error-event", map[string]string{
		"error.type": "test",
		"error.severity": "low",
	})
	
	// Add an event
	// Events represent points in time within a span
	AddEvent(ctx, "test-event", map[string]string{
		"event.type": "test",
		"event.source": "test-function",
	})
	
	// Record a span metric
	// This associates a metric with the current span
	RecordSpanMetric(ctx, "test.span.metric", 42.0)
}

// TestWithSpanHelper demonstrates how to use the WithSpan helper function
// to create spans around code blocks.
//
// This shows how to:
// - Use the WithSpan helper for clean span creation and error handling
// - Record metrics within spans
// - Add events and attributes to spans
// - Handle errors from span operations
//
// The WithSpan helper makes it easier to create properly instrumented code
// with automatic span ending and error handling.
func TestWithSpanHelper(t *testing.T) {
	ctx := context.Background()
	
	provider, err := New(ctx)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Shutdown(ctx)
	
	// Create a metrics collector for use inside the span
	collector, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	
	// Use the WithSpan helper
	// This automatically creates a span, executes the function, and ends the span
	err = WithSpan(ctx, provider, "test-helper-span", func(spanCtx context.Context) error {
		// Add attributes to the span
		span := trace.SpanFromContext(spanCtx)
		span.SetAttributes(attribute.String("helper.attribute", "value"))
		
		// Record metrics within the span
		// This associates the metrics with the current trace
		collector.RecordRequest(
			spanCtx,
			"within-span-operation",
			"test-resource",
			200,
			75*time.Millisecond,
		)
		
		// Add an event
		AddEvent(spanCtx, "within-span-event", map[string]string{
			"event.source": "within-span",
		})
		
		return nil
	})
	
	if err != nil {
		t.Fatalf("WithSpan helper function failed: %v", err)
	}
	
	// Test with an error return
	// WithSpan will record the error on the span and propagate it
	err = WithSpan(ctx, provider, "test-helper-span-error", func(spanCtx context.Context) error {
		return fmt.Errorf("test error from within span")
	})
	
	if err == nil {
		t.Fatal("WithSpan should have propagated the error")
	}
	if err.Error() != "test error from within span" {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestContextPropagation demonstrates how to propagate trace context
// across service boundaries.
//
// This shows how to:
// - Configure propagators for distributed tracing
// - Inject trace context into headers for outgoing requests
// - Extract trace context from headers for incoming requests
// - Create child spans from extracted contexts
//
// Context propagation is essential for distributed tracing across
// service boundaries in microservice architectures.
func TestContextPropagation(t *testing.T) {
	ctx := context.Background()
	
	provider, err := New(ctx, 
		WithPropagators(propagation.TraceContext{}),
		WithPropagationHeaders("X-Request-ID"),
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Shutdown(ctx)
	
	// Create a span to get a valid trace context
	// In a real application, this would be the span for the current operation
	tracer := provider.Tracer()
	ctx, span := tracer.Start(ctx, "test-propagation-span")
	defer span.End()
	
	// Test context injection
	// Use this when making outgoing requests to other services
	headers := make(map[string]string)
	InjectContext(ctx, headers)
	
	// Verify headers were set
	// These headers will contain the trace context information
	if len(headers) == 0 {
		t.Error("Context injection should add trace headers")
	}
	
	// Test context extraction
	// Use this when receiving incoming requests from other services
	extractedCtx := ExtractContext(context.Background(), headers)
	
	// Verify the extracted context has a valid span context
	extractedSpanCtx := trace.SpanContextFromContext(extractedCtx)
	if !extractedSpanCtx.IsValid() {
		t.Error("Extracted context should have a valid span context")
	}
	
	// Create a span with the extracted context
	// This continues the trace from the parent service
	_, childSpan := tracer.Start(extractedCtx, "child-of-extracted-context")
	childSpan.End()
}

// TestObservabilityWithMetricsAndTracing demonstrates how to integrate
// metrics and tracing for comprehensive observability.
//
// This shows how to:
// - Use timers within spans to measure and record operation durations
// - Create hierarchical spans for nested operations
// - Add events and attributes to spans for context
// - Record metrics within the context of a trace
//
// Integrating metrics and tracing provides a complete picture of your
// application's performance and behavior.
func TestObservabilityWithMetricsAndTracing(t *testing.T) {
	ctx := context.Background()
	
	provider, err := New(ctx, 
		WithServiceName("test-integration-service"),
		WithFullTracingSampling(),
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Shutdown(ctx)
	
	// Create a metrics collector
	collector, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	
	// Use WithSpan to create a traced operation
	err = WithSpan(ctx, provider, "integration-test-span", func(spanCtx context.Context) error {
		// Get the current span
		span := trace.SpanFromContext(spanCtx)
		
		// Add attributes to the span
		// These provide context for the operation being performed
		span.SetAttributes(
			attribute.String("operation", "integration-test"),
			attribute.String("component", "observability"),
		)
		
		// Start a timer
		// This will record the duration of the operation
		timer := collector.NewTimer(spanCtx, "integration-operation", "test")
		
		// Simulate work
		time.Sleep(20 * time.Millisecond)
		
		// Add an event
		// Events mark important points within the span
		AddEvent(spanCtx, "work-completed", map[string]string{
			"status": "success",
		})
		
		// Stop the timer
		// This records the duration metric
		timer.Stop(200)
		
		// Create a child operation
		// This demonstrates nested spans for hierarchical operations
		return WithSpan(spanCtx, provider, "child-operation", func(childCtx context.Context) error {
			childTimer := collector.NewTimer(childCtx, "child-operation", "test")
			time.Sleep(10 * time.Millisecond)
			childTimer.Stop(200)
			return nil
		})
	})
	
	if err != nil {
		t.Fatalf("Integration test failed: %v", err)
	}
}

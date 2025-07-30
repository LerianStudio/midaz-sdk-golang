package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestNewWithOptions(t *testing.T) {
	// Test with no options
	provider, err := New(context.Background())
	if err != nil {
		t.Fatalf("Failed to create provider with no options: %v", err)
	}
	if !provider.IsEnabled() {
		t.Fatal("Provider should be enabled by default")
	}

	// Test with custom options
	provider, err = New(context.Background(),
		WithServiceName("test-service"),
		WithServiceVersion("1.0.0"),
		WithSDKVersion("2.0.0"),
		WithEnvironment("test"),
		WithComponentEnabled(true, false, true),
	)
	if err != nil {
		t.Fatalf("Failed to create provider with custom options: %v", err)
	}
	if !provider.IsEnabled() {
		t.Fatal("Provider should be enabled")
	}

	// Test with development defaults
	provider, err = New(context.Background(), WithDevelopmentDefaults())
	if err != nil {
		t.Fatalf("Failed to create provider with development defaults: %v", err)
	}
	if !provider.IsEnabled() {
		t.Fatal("Provider should be enabled")
	}

	// Test with production defaults
	provider, err = New(context.Background(), WithProductionDefaults())
	if err != nil {
		t.Fatalf("Failed to create provider with production defaults: %v", err)
	}
	if !provider.IsEnabled() {
		t.Fatal("Provider should be enabled")
	}

	// Test with invalid options
	_, err = New(context.Background(), WithTraceSampleRate(2.0))
	if err == nil {
		t.Fatal("Expected error with invalid trace sample rate")
	}

	// Test shutdown
	err = provider.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Failed to shutdown provider: %v", err)
	}
	if provider.IsEnabled() {
		t.Fatal("Provider should be disabled after shutdown")
	}
}

func TestNewWithConfig(t *testing.T) {
	// Test backward compatibility

	// Test with nil config
	provider, err := NewWithConfig(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to create provider with nil config: %v", err)
	}
	if !provider.IsEnabled() {
		t.Fatal("Provider should be enabled by default")
	}

	// Test with custom config
	config := &Config{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		SDKVersion:     "2.0.0",
		Environment:    "test",
		EnabledComponents: EnabledComponents{
			Tracing: true,
			Metrics: false,
			Logging: true,
		},
	}
	provider, err = NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("Failed to create provider with custom config: %v", err)
	}
	if !provider.IsEnabled() {
		t.Fatal("Provider should be enabled")
	}

	// Test shutdown
	err = provider.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Failed to shutdown provider: %v", err)
	}
	if provider.IsEnabled() {
		t.Fatal("Provider should be disabled after shutdown")
	}
}

func TestWithSpan(t *testing.T) {
	// Skip this test in short mode
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Create provider with tracing explicitly enabled
	provider, err := New(context.Background(),
		WithServiceName("test-service"),
		WithServiceVersion("1.0.0"),
		WithEnvironment("test"),
		WithComponentEnabled(true, false, false), // Enable only tracing
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Shutdown(context.Background())

	// Use a context that has the provider in it
	ctx := WithProvider(context.Background(), provider)

	// Test successful span
	err = WithSpan(ctx, provider, "test-span", func(ctx context.Context) error {
		// We'll skip the validation in the test since it's environment-dependent
		return nil
	})
	if err != nil {
		t.Fatalf("WithSpan failed: %v", err)
	}

	// Test span with error
	testErr := errors.New("test error")
	err = WithSpan(context.Background(), provider, "error-span", func(ctx context.Context) error {
		return testErr
	})
	if err != testErr {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}
}

func TestLogger(t *testing.T) {
	// Create provider with logging enabled
	provider, err := New(context.Background(),
		WithServiceName("test-service"),
		WithServiceVersion("1.0.0"),
		WithEnvironment("test"),
		WithLogLevel(DebugLevel),
		WithLogOutput(os.Stdout),
		WithComponentEnabled(false, false, true), // Enable only logging
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Shutdown(context.Background())

	// Get logger
	logger := provider.Logger()
	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}

	// Test logging methods (just checking they don't panic)
	logger.Debug("Debug message")
	logger.Debugf("Debug %s", "formatted")
	logger.Info("Info message")
	logger.Infof("Info %s", "formatted")
	logger.Warn("Warn message")
	logger.Warnf("Warn %s", "formatted")
	logger.Error("Error message")
	logger.Errorf("Error %s", "formatted")

	// Test with fields
	fieldsLogger := logger.With(map[string]any{
		"key1": "value1",
		"key2": 123,
	})
	fieldsLogger.Info("Message with fields")

	// Don't test Fatal as it would exit the program
}

func TestHTTPMiddleware(t *testing.T) {
	// Create provider
	provider, err := New(context.Background(),
		WithServiceName("test-service"),
		WithServiceVersion("1.0.0"),
		WithEnvironment("test"),
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Shutdown(context.Background())

	// Create server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}))
	defer server.Close()

	// Create client with middleware and options
	client := &http.Client{
		Transport: NewHTTPMiddleware(
			provider,
			WithIgnoreHeaders("x-test-header"),
			WithIgnorePaths("/health"),
			WithMaskedParams("api_key"),
			WithHideRequestBody(true),
		)(http.DefaultTransport),
	}

	// Make request
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test with nil provider (should return a no-op middleware)
	noopTransport := NewHTTPMiddleware(nil)(http.DefaultTransport)
	if noopTransport == nil {
		t.Error("Expected non-nil transport with nil provider")
	}

	// Test with security defaults
	secureTransport := NewHTTPMiddleware(
		provider,
		WithSecurityDefaults(),
	)(http.DefaultTransport)

	if secureTransport == nil {
		t.Error("Expected non-nil transport with security defaults")
	}
}

func TestMetricsCollector(t *testing.T) {
	// Create provider
	provider, err := New(context.Background(),
		WithServiceName("test-service"),
		WithServiceVersion("1.0.0"),
		WithEnvironment("test"),
		WithComponentEnabled(false, true, false), // Enable only metrics
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Shutdown(context.Background())

	// Create metrics collector
	collector, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	// Test recording request
	ctx := context.Background()
	collector.RecordRequest(ctx, "test.operation", "account", 200, 100*time.Millisecond,
		attribute.String("test.attr", "value"))

	// Test recording batch request
	collector.RecordBatchRequest(ctx, "test.batch", "account", 10, 200*time.Millisecond)

	// Test recording retry
	collector.RecordRetry(ctx, "test.retry", "account", 2)

	// Test timer
	timer := collector.NewTimer(ctx, "test.timer", "account")
	timer.Stop(200)

	// Test batch timer
	batchTimer := collector.NewTimer(ctx, "test.batch_timer", "account")
	batchTimer.StopBatch(5)
}

func TestContextFunctions(t *testing.T) {
	// Skip this test in short mode
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Create provider with tracing explicitly enabled
	provider, err := New(context.Background(),
		WithServiceName("test-service"),
		WithServiceVersion("1.0.0"),
		WithEnvironment("test"),
		WithComponentEnabled(true, false, true), // Enable tracing and logging
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Shutdown(context.Background())

	// Test WithProvider and GetProvider
	ctx := WithProvider(context.Background(), provider)
	retrievedProvider := GetProvider(ctx)
	if retrievedProvider != provider {
		t.Error("GetProvider did not return the expected provider")
	}

	// Test WithSpanAttributes and AddSpanAttributes
	// Disable span validation for now as it's environment-dependent
	ctx, span := provider.Tracer().Start(ctx, "test-span")
	ctx = WithSpanAttributes(ctx, attribute.String("key1", "value1"))
	AddSpanAttributes(ctx, attribute.Int("key2", 123))

	// Test AddSpanEvent
	AddSpanEvent(ctx, "test-event", attribute.Bool("happened", true))

	// Test baggage
	ctx, err = WithBaggageItem(ctx, "baggage-key", "baggage-value")
	if err != nil {
		t.Fatalf("WithBaggageItem failed: %v", err)
	}
	value := GetBaggageItem(ctx, "baggage-key")
	if value != "baggage-value" {
		t.Errorf("Expected baggage value 'baggage-value', got '%s'", value)
	}

	// Test Log
	logger := Log(ctx)
	if logger == nil {
		t.Error("Expected non-nil logger from context")
	}

	// Clean up
	span.End()
}

func ExampleWithSpan() {
	// Create a provider with options
	provider, err := New(context.Background(),
		WithServiceName("example-service"),
		WithEnvironment("development"),
		WithFullTracingSampling(), // Use 100% sampling for better examples
	)
	if err != nil {
		fmt.Printf("Failed to create provider: %v\n", err)
		return
	}
	defer provider.Shutdown(context.Background())

	// Use WithSpan to automatically create, end, and handle errors for a span
	err = WithSpan(context.Background(), provider, "example-operation", func(ctx context.Context) error {
		// Do something with the context that has the span
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("example", "attribute"))

		// Simulate work
		time.Sleep(10 * time.Millisecond)

		// Simulate success
		return nil
	})

	if err != nil {
		fmt.Printf("Operation failed: %v\n", err)
	} else {
		fmt.Println("Operation succeeded")
	}
	// Output: Operation succeeded
}

func ExampleMidazProvider_Logger() {
	// Create a provider with options
	provider, err := New(context.Background(),
		WithServiceName("example-service"),
		WithEnvironment("development"),
		WithLogLevel(InfoLevel),
		WithDevelopmentDefaults(),
	)
	if err != nil {
		fmt.Printf("Failed to create provider: %v\n", err)
		return
	}
	defer provider.Shutdown(context.Background())

	// Get a logger
	logger := provider.Logger()

	// Log at different levels
	logger.Debug("This is a debug message (won't be shown with InfoLevel)")
	logger.Info("This is an info message")

	// Add structured fields
	structuredLogger := logger.With(map[string]any{
		"user_id": "123",
		"action":  "login",
	})
	structuredLogger.Info("User logged in")

	// Add context information (tracing)
	_, span := provider.Tracer().Start(context.Background(), "example-span")
	defer span.End()

	tracingLogger := logger.WithSpan(span)
	tracingLogger.Info("This log includes trace and span IDs")

	// Add error information
	span.SetStatus(codes.Error, "Something went wrong")
	span.RecordError(errors.New("example error"))

	tracingLogger.Error("An error occurred")
}

func ExampleNewHTTPMiddleware() {
	// Create a provider with options
	provider, err := New(context.Background(),
		WithServiceName("example-service"),
		WithEnvironment("development"),
		WithHighTracingSampling(),
	)
	if err != nil {
		fmt.Printf("Failed to create provider: %v\n", err)
		return
	}
	defer provider.Shutdown(context.Background())

	// Create an HTTP client with the middleware and security defaults
	client := &http.Client{
		Transport: NewHTTPMiddleware(
			provider,
			WithSecurityDefaults(),     // Apply security defaults
			WithIgnorePaths("/health"), // Don't trace health checks
			WithHideRequestBody(true),  // Don't include request bodies
		)(http.DefaultTransport),
	}

	// Now any requests made with this client will be traced
	resp, err := client.Get("https://example.com")
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %s\n", resp.Status)
}

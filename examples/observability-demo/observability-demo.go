// This example demonstrates how to use the observability features with the new functional options pattern.
// It shows how to create a provider with options and use it for tracing, metrics, and logging.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
)

func main() {
	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create observability provider with functional options
	provider, err := observability.New(ctx,
		observability.WithServiceName("observability-demo"),
		observability.WithEnvironment("development"),
		observability.WithComponentEnabled(true, true, true), // Enable tracing, metrics, and logging
		observability.WithHighTracingSampling(),              // Use higher sampling rate for development
		observability.WithLogLevel(observability.DebugLevel),
		observability.WithAttributes(
			attribute.String("demo.source", "example"),
			attribute.String("demo.version", "1.0.0"),
		),
	)
	if err != nil {
		fmt.Printf("Error creating observability provider: %v\n", err)
		os.Exit(1)
	}
	defer provider.Shutdown(ctx)

	// Get a logger and log a message
	logger := provider.Logger()
	logger.Info("Starting observability demo")

	// Create a metrics collector
	metrics, _ := observability.NewMetricsCollector(provider)

	// Demonstrate using spans
	if err := observability.WithSpan(ctx, provider, "main_operation", func(ctx context.Context) error {
		// Get a logger with span context
		spanLogger := observability.Log(ctx)
		spanLogger.Info("Starting main operation")

		// Record a metric
		metrics.RecordRequest(ctx, "main_operation", "example", 200, 10*time.Millisecond)

		// Create a child span
		childCtx, childSpan := provider.Tracer().Start(ctx, "child_operation")
		childSpan.SetAttributes(attribute.String("child.attribute", "value"))

		// Do some work in the child span
		time.Sleep(50 * time.Millisecond)
		observability.Log(childCtx).Info("Child operation completed")
		childSpan.End()

		// Add an event and attributes to the main span
		observability.AddSpanEvent(ctx, "important_event",
			attribute.Bool("success", true),
			attribute.Int("count", 42),
		)

		observability.AddSpanAttributes(ctx,
			attribute.String("operation.result", "success"),
			attribute.String("operation.duration", "100ms"),
		)

		return nil
	}); err != nil {
		logger.Errorf("Error in main operation: %v", err)
	}

	// Demonstrate error handling in spans
	if err := observability.WithSpan(ctx, provider, "error_operation", func(ctx context.Context) error {
		// Record a metric
		metrics.RecordRequest(ctx, "error_operation", "example", 500, 5*time.Millisecond)

		// Add an event
		observability.AddSpanEvent(ctx, "processing_error",
			attribute.String("error.type", "validation"),
		)

		// Return an error
		return fmt.Errorf("example error for demonstration")
	}); err != nil {
		logger.Warnf("Expected error occurred: %v", err)
	}

	// Demonstrate custom metrics
	timer := metrics.NewTimer(ctx, "batch_operation", "example")
	time.Sleep(75 * time.Millisecond)
	timer.StopBatch(5) // Record a batch operation with 5 items

	// Demonstrate baggage
	ctx, _ = observability.WithBaggageItem(ctx, "user", "example-user")
	ctx, _ = observability.WithBaggageItem(ctx, "tenant", "example-tenant")

	// Use the baggage in a span
	if err := observability.WithSpan(ctx, provider, "baggage_operation", func(ctx context.Context) error {
		user := observability.GetBaggageItem(ctx, "user")
		tenant := observability.GetBaggageItem(ctx, "tenant")

		observability.Log(ctx).Infof("Operation for user=%s, tenant=%s", user, tenant)
		return nil
	}); err != nil {
		logger.Errorf("Error in baggage operation: %v", err)
	}

	// Using HTTP middleware (demonstrating how it would be used)
	transport := observability.NewHTTPMiddleware(provider,
		observability.WithSecurityDefaults(),
		observability.WithIgnorePaths("/health", "/metrics"),
	)(http.DefaultTransport)

	// Create an HTTP client with the middleware
	client := &http.Client{Transport: transport}

	// This is just to show that the middleware is applied - not actually making a request
	logger.Info("Created HTTP client with middleware (transport wrapped with tracing)")
	_ = client // prevent unused variable warning

	logger.Info("Observability demo completed")
}

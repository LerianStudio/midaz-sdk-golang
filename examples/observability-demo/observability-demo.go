// This example demonstrates how to use the observability features with the new functional options pattern.
// It shows how to create a provider with options and use it for tracing, metrics, and logging.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := createObservabilityProvider(ctx)
	defer shutdownProvider(ctx, provider)

	logger := provider.Logger()
	logger.Info("Starting observability demo")

	metrics := createMetricsCollector(provider, logger)

	demonstrateSpanOperations(ctx, provider, metrics, logger)
	demonstrateErrorHandling(ctx, provider, metrics, logger)
	demonstrateCustomMetrics(ctx, metrics)
	ctx = demonstrateBaggage(ctx, provider, logger)
	demonstrateHTTPMiddleware(provider, logger)

	logger.Info("Observability demo completed")
}

func createObservabilityProvider(ctx context.Context) observability.Provider {
	provider, err := observability.New(ctx,
		observability.WithServiceName("observability-demo"),
		observability.WithEnvironment("development"),
		observability.WithComponentEnabled(true, true, true),
		observability.WithHighTracingSampling(),
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

	return provider
}

func shutdownProvider(ctx context.Context, provider observability.Provider) {
	if err := provider.Shutdown(ctx); err != nil {
		fmt.Printf("Warning: Failed to shutdown observability provider: %v\n", err)
	}
}

func createMetricsCollector(provider observability.Provider, logger observability.Logger) *observability.MetricsCollector {
	metrics, err := observability.NewMetricsCollector(provider)
	if err != nil {
		logger.Errorf("Failed to create metrics collector: %v", err)
		os.Exit(1)
	}

	return metrics
}

func demonstrateSpanOperations(ctx context.Context, provider observability.Provider, metrics *observability.MetricsCollector, logger observability.Logger) {
	if err := observability.WithSpan(ctx, provider, "main_operation", func(ctx context.Context) error {
		spanLogger := observability.Log(ctx)
		spanLogger.Info("Starting main operation")

		metrics.RecordRequest(ctx, "main_operation", "example", 200, 10*time.Millisecond)

		runChildSpan(ctx, provider)

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
}

func runChildSpan(ctx context.Context, provider observability.Provider) {
	childCtx, childSpan := provider.Tracer().Start(ctx, "child_operation")
	childSpan.SetAttributes(attribute.String("child.attribute", "value"))

	time.Sleep(50 * time.Millisecond)
	observability.Log(childCtx).Info("Child operation completed")
	childSpan.End()
}

func demonstrateErrorHandling(ctx context.Context, provider observability.Provider, metrics *observability.MetricsCollector, logger observability.Logger) {
	if err := observability.WithSpan(ctx, provider, "error_operation", func(ctx context.Context) error {
		metrics.RecordRequest(ctx, "error_operation", "example", 500, 5*time.Millisecond)

		observability.AddSpanEvent(ctx, "processing_error",
			attribute.String("error.type", "validation"),
		)

		return fmt.Errorf("example error for demonstration")
	}); err != nil {
		logger.Warnf("Expected error occurred: %v", err)
	}
}

func demonstrateCustomMetrics(ctx context.Context, metrics *observability.MetricsCollector) {
	timer := metrics.NewTimer(ctx, "batch_operation", "example")
	time.Sleep(75 * time.Millisecond)
	timer.StopBatch(5)
}

func demonstrateBaggage(ctx context.Context, provider observability.Provider, logger observability.Logger) context.Context {
	var err error

	ctx, err = observability.WithBaggageItem(ctx, "user", "example-user")
	if err != nil {
		logger.Warnf("Failed to set user baggage: %v", err)
	}

	ctx, err = observability.WithBaggageItem(ctx, "tenant", "example-tenant")
	if err != nil {
		logger.Warnf("Failed to set tenant baggage: %v", err)
	}

	if err := observability.WithSpan(ctx, provider, "baggage_operation", func(ctx context.Context) error {
		user := observability.GetBaggageItem(ctx, "user")
		tenant := observability.GetBaggageItem(ctx, "tenant")

		observability.Log(ctx).Infof("Operation for user=%s, tenant=%s", user, tenant)
		return nil
	}); err != nil {
		logger.Errorf("Error in baggage operation: %v", err)
	}

	return ctx
}

func demonstrateHTTPMiddleware(provider observability.Provider, logger observability.Logger) {
	transport := observability.NewHTTPMiddleware(provider,
		observability.WithSecurityDefaults(),
		observability.WithIgnorePaths("/health", "/metrics"),
	)(http.DefaultTransport)

	client := &http.Client{Transport: transport}
	_ = client.Transport
	_ = client

	logger.Info("Created HTTP client with middleware (transport wrapped with tracing)")
}

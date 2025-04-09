package observability

import (
	"context"
	"testing"
)

// TestSimple is a simple test to ensure the package builds properly
func TestSimple(t *testing.T) {
	// This is just a placeholder test to ensure the package compiles
	// We'll implement actual tests once we've resolved the dependency issues
	t.Log("Simple test passed")
}

// TestObservabilityIntegration is a simple test to ensure the package integrates correctly
// with the SDK. This test just creates a new observability provider with default options
// and checks that basics work.
func TestObservabilityIntegration(t *testing.T) {
	// Create a provider with default options (no options means use defaults)
	provider, err := New(context.Background())
	if err != nil {
		t.Fatalf("Failed to create provider with default options: %v", err)
	}
	if !provider.IsEnabled() {
		t.Fatal("Provider should be enabled by default")
	}

	// Get a logger and make sure it doesn't crash
	logger := provider.Logger()
	logger.Info("Test log message")

	// Get a tracer and make sure it doesn't crash
	tracer := provider.Tracer()
	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	// Make sure we can shut down cleanly
	err = provider.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Failed to shutdown provider: %v", err)
	}
	if provider.IsEnabled() {
		t.Fatal("Provider should be disabled after shutdown")
	}
}

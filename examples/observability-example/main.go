// This example demonstrates how to use the observability features of the Midaz Go SDK.
// It shows how to enable tracing, metrics, and logging, and how to use them to
// monitor and troubleshoot SDK operations.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func main() {
	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create observability provider directly with functional options
	provider, err := observability.New(ctx,
		observability.WithServiceName("go-sdk-example"),
		observability.WithEnvironment("development"),
		observability.WithComponentEnabled(true, true, true), // Enable tracing, metrics, and logging
		observability.WithHighTracingSampling(),              // Use higher sampling rate for development
		observability.WithLogLevel(observability.DebugLevel),
		observability.WithAttributes(
			attribute.String("example.source", "observability-example"),
			attribute.String("example.version", "1.0.0"),
		),
	)
	if err != nil {
		fmt.Printf("Error creating observability provider: %v\n", err)
		os.Exit(1)
	}

	// For demonstration purposes, we're using the provider directly
	// In a real application, you'd pass this to the SDK client
	defer provider.Shutdown(ctx)

	// Create a client that uses the observability provider
	c, err := client.New(
		client.WithAuthToken("test-token"),
		client.WithEnvironment(config.EnvironmentDevelopment),
		client.WithObservabilityProvider(provider), // Attach the provider to the client
		client.UseAllAPIs(),
	)
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}
	defer c.Shutdown(ctx) // Ensure proper shutdown of the client

	// Use the provider directly
	if !provider.IsEnabled() {
		fmt.Println("Observability is not enabled")
		return
	}

	// Get a logger
	logger := provider.Logger()
	logger.Info("Starting observability example")

	// Demonstrate using spans for operations
	err = observability.WithSpan(ctx, provider, "create_organization", func(ctx context.Context) error {
		// Get logger with span context
		spanLogger := observability.Log(ctx)
		spanLogger.Info("Creating organization")

		// Create an organization
		org, err := createOrganization(ctx, provider)
		if err != nil {
			spanLogger.Errorf("Failed to create organization: %v", err)
			return err
		}

		spanLogger.Infof("Created organization with ID: %s", org.ID)

		// Create a nested span for a sub-operation
		ctx, span := observability.Start(ctx, "create_ledger")
		defer span.End()

		// Set attributes on the span
		span.SetAttributes(
			attribute.String("organization.id", org.ID),
			attribute.String("organization.name", org.LegalName),
		)

		// Create a ledger
		ledger, err := createLedger(ctx, provider, org.ID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			spanLogger.Errorf("Failed to create ledger: %v", err)
			return err
		}

		span.SetAttributes(attribute.String("ledger.id", ledger.ID))
		spanLogger.Infof("Created ledger with ID: %s", ledger.ID)

		return nil
	})

	if err != nil {
		logger.Errorf("Error in operation: %v", err)
	}

	// Record a custom metric
	metrics, _ := observability.NewMetricsCollector(provider)
	if metrics != nil {
		metrics.RecordRequest(ctx, "custom_operation", "example", 200, 150*time.Millisecond,
			attribute.String("custom.attribute", "value"))
	}

	logger.Info("Observability example completed successfully")
}

// Helper function to create an organization
// In a real application, you would use c.Entity.Organizations.CreateOrganization instead
func createOrganization(ctx context.Context, provider observability.Provider) (*models.Organization, error) {
	// Add span event
	observability.AddSpanEvent(ctx, "preparing_organization_data")

	// Record the start time for manual timing
	start := time.Now()

	// Simulate work
	time.Sleep(100 * time.Millisecond)

	// Create organization data
	org := &models.Organization{
		LegalName:       "Example Organization",
		LegalDocument:   "EX123456789",
		DoingBusinessAs: "Example DBA",
		Status: models.Status{
			Code: "ACTIVE",
		},
		Address: models.Address{
			Line1:   "123 Example St",
			City:    "Exampleville",
			State:   "EX",
			ZipCode: "12345",
			Country: "US",
		},
		Metadata: map[string]interface{}{
			"createdBy": "observability-example",
		},
	}

	// Add baggage to the context
	var err error
	ctx, err = observability.WithBaggageItem(ctx, "source", "observability-example")
	if err != nil {
		return nil, fmt.Errorf("error adding baggage: %w", err)
	}

	// Record timing
	duration := time.Since(start)
	observability.AddSpanAttributes(ctx,
		attribute.String("operation.duration_ms", fmt.Sprintf("%.2f", float64(duration.Milliseconds()))),
	)

	// Simulating creating and returning the organization
	// In a real application, you would call the API
	org.ID = "org-" + fmt.Sprintf("%d", time.Now().UnixNano())
	org.CreatedAt = time.Now()
	org.UpdatedAt = time.Now()

	return org, nil
}

// Helper function to create a ledger
// In a real application, you would use c.Entity.Ledgers.CreateLedger instead
func createLedger(ctx context.Context, provider observability.Provider, orgID string) (*models.Ledger, error) {
	// Add span event
	observability.AddSpanEvent(ctx, "preparing_ledger_data")

	// Simulate work
	time.Sleep(150 * time.Millisecond)

	// Log within the context of the current span
	observability.Log(ctx).Info("Preparing to create ledger")

	// Simulating creating and returning the ledger
	// In a real application, you would call the API
	ledger := &models.Ledger{
		ID:             "ledger-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		OrganizationID: orgID,
		Name:           "Example Ledger",
		Metadata: map[string]interface{}{
			"description": "Ledger for observability example",
		},
		Status: models.Status{
			Code: "ACTIVE",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return ledger, nil
}

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// This example demonstrates how to use distributed tracing with the Midaz SDK
func main() {
	// Create observability provider with tracing enabled
	provider, err := observability.New(context.Background(),
		observability.WithServiceName("midaz-example-service"),
		observability.WithServiceVersion("1.0.0"),
		observability.WithEnvironment("development"),
		observability.WithComponentEnabled(true, true, true),         // Enable tracing, metrics, and logging
		observability.WithCollectorEndpoint("http://localhost:4317"), // Optional: OTEL collector
		observability.WithHighTracingSampling(),                      // High sampling for development
		observability.WithPropagationHeaders("traceparent", "tracestate", "baggage", "x-request-id"),
	)
	if err != nil {
		log.Fatalf("Failed to create observability provider: %v", err)
	}
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down observability provider: %v", err)
		}
	}()

	// Create Midaz client with observability
	// Set auth token via environment variable or replace "your-api-token" with actual token
	err = os.Setenv("MIDAZ_AUTH_TOKEN", "your-api-token")
	if err != nil {
		log.Fatalf("Failed to set MIDAZ_AUTH_TOKEN environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MIDAZ_AUTH_TOKEN"); err != nil {
			log.Printf("Warning: Failed to unset MIDAZ_AUTH_TOKEN: %v", err)
		}
	}()

	midazClient, err := client.New(
		client.WithBaseURL("https://api.midaz.com"),
		client.WithObservabilityProvider(provider),
	)
	if err != nil {
		log.Fatalf("Failed to create Midaz client: %v", err)
	}

	// Example: Create organization with distributed tracing
	if err := createOrganizationWithTracing(midazClient, provider); err != nil {
		log.Fatalf("Failed to create organization: %v", err)
	}

	// Example: Simulate complex workflow with multiple API calls
	if err := performComplexWorkflowWithTracing(midazClient, provider); err != nil {
		log.Fatalf("Failed to perform complex workflow: %v", err)
	}

	fmt.Println("Examples completed successfully!")
}

// createOrganizationWithTracing demonstrates creating an organization with tracing
func createOrganizationWithTracing(midazClient *client.Client, provider observability.Provider) error {
	// Start a root span for this operation
	tracer := provider.Tracer()
	ctx, span := tracer.Start(context.Background(), "create_organization_workflow")
	defer span.End()

	// Add custom attributes to the span
	observability.AddSpanAttributes(ctx,
		attribute.String("workflow.type", "organization_creation"),
		attribute.String("business.unit", "onboarding"),
	)

	// Add baggage for correlation across services
	ctx, err := observability.WithBaggageItem(ctx, "user-id", "user-123")
	if err != nil {
		return fmt.Errorf("failed to add baggage: %w", err)
	}

	ctx, err = observability.WithBaggageItem(ctx, "request-id", "req-456")
	if err != nil {
		return fmt.Errorf("failed to add baggage: %w", err)
	}

	// Create organization input
	orgInput := models.NewCreateOrganizationInput("Example Corp").
		WithAddress(models.Address{
			Line1:   "123 Main St",
			City:    "San Francisco",
			State:   "CA",
			Country: "US",
			ZipCode: "94105",
		}).
		WithMetadata(map[string]any{
			"industry": "technology",
			"size":     "startup",
		})

	// Create organization - this will automatically include tracing headers
	logger := provider.Logger()
	logger.Info("Creating organization", "name", orgInput.LegalName)

	organization, err := midazClient.Entity.Organizations.CreateOrganization(ctx, orgInput)
	if err != nil {
		// Record error in span
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return fmt.Errorf("failed to create organization: %w", err)
	}

	// Log success and add span attributes
	logger.Info("Organization created successfully",
		"org_id", organization.ID,
		"legal_name", organization.LegalName,
	)

	span.SetAttributes(
		attribute.String("organization.id", organization.ID),
		attribute.String("organization.legal_name", organization.LegalName),
	)

	span.SetStatus(codes.Ok, "Organization created successfully")
	return nil
}

// workflowContext holds shared state for the complex workflow
type workflowContext struct {
	midazClient *client.Client
	provider    observability.Provider
	tracer      trace.Tracer
	logger      observability.Logger
	orgID       string
	ledger      *models.Ledger
}

// listOrganizationsStep lists organizations and returns the first org ID
func (wc *workflowContext) listOrganizationsStep(ctx context.Context) (context.Context, error) {
	ctx, listSpan := wc.tracer.Start(ctx, "list_organizations")
	defer listSpan.End()

	wc.logger.Info("Listing organizations")

	organizations, err := wc.midazClient.Entity.Organizations.ListOrganizations(ctx, nil)
	if err != nil {
		listSpan.SetStatus(codes.Error, err.Error())
		listSpan.RecordError(err)
		return ctx, fmt.Errorf("failed to list organizations: %w", err)
	}

	listSpan.SetAttributes(attribute.Int("organizations.count", len(organizations.Items)))
	listSpan.SetStatus(codes.Ok, "Organizations listed successfully")

	if len(organizations.Items) == 0 {
		return ctx, errors.New("no organizations found")
	}

	wc.orgID = organizations.Items[0].ID
	wc.logger.Info("Using organization", "org_id", wc.orgID)

	return ctx, nil
}

// createLedgerStep creates a ledger in the organization
func (wc *workflowContext) createLedgerStep(ctx context.Context) (context.Context, error) {
	ctx, ledgerSpan := wc.tracer.Start(ctx, "create_ledger")
	defer ledgerSpan.End()

	ledgerSpan.SetAttributes(attribute.String("organization.id", wc.orgID))

	ledgerInput := models.NewCreateLedgerInput("Main Ledger")
	ledger, err := wc.midazClient.Entity.Ledgers.CreateLedger(ctx, wc.orgID, ledgerInput)
	if err != nil {
		ledgerSpan.SetStatus(codes.Error, err.Error())
		ledgerSpan.RecordError(err)
		return ctx, fmt.Errorf("failed to create ledger: %w", err)
	}

	wc.ledger = ledger
	ledgerSpan.SetAttributes(
		attribute.String("ledger.id", ledger.ID),
		attribute.String("ledger.name", ledger.Name),
	)
	ledgerSpan.SetStatus(codes.Ok, "Ledger created successfully")

	return ctx, nil
}

// createAssetsStep creates multiple assets in batch
func (wc *workflowContext) createAssetsStep(ctx context.Context, assetNames []string) context.Context {
	ctx, assetsSpan := wc.tracer.Start(ctx, "create_assets_batch")
	defer assetsSpan.End()

	assetsSpan.SetAttributes(attribute.Int("assets.count", len(assetNames)))

	for _, assetName := range assetNames {
		wc.createSingleAsset(ctx, assetName)
	}

	assetsSpan.SetStatus(codes.Ok, "Assets batch completed")
	return ctx
}

// createSingleAsset creates a single asset with tracing
func (wc *workflowContext) createSingleAsset(ctx context.Context, assetName string) {
	_, assetSpan := wc.tracer.Start(ctx, "create_asset")
	defer assetSpan.End()

	assetSpan.SetAttributes(
		attribute.String("asset.code", assetName),
		attribute.String("ledger.id", wc.ledger.ID),
	)

	assetInput := models.NewCreateAssetInput(assetName, assetName)
	asset, err := wc.midazClient.Entity.Assets.CreateAsset(ctx, wc.orgID, wc.ledger.ID, assetInput)
	if err != nil {
		assetSpan.SetStatus(codes.Error, err.Error())
		assetSpan.RecordError(err)
		wc.logger.Error("Failed to create asset", "asset", assetName, "error", err)
		return
	}

	assetSpan.SetAttributes(attribute.String("asset.id", asset.ID))
	assetSpan.SetStatus(codes.Ok, "Asset created successfully")
	wc.logger.Info("Asset created", "asset_id", asset.ID, "code", asset.Code)
}

// createPortfolioStep creates a portfolio with timing
func (wc *workflowContext) createPortfolioStep(ctx context.Context) (*models.Portfolio, error) {
	_, portfolioSpan := wc.tracer.Start(ctx, "create_portfolio")
	defer portfolioSpan.End()

	startTime := time.Now()
	portfolioInput := models.NewCreatePortfolioInput(wc.orgID, "Main Portfolio")
	portfolio, err := wc.midazClient.Entity.Portfolios.CreatePortfolio(ctx, wc.orgID, wc.ledger.ID, portfolioInput)

	duration := time.Since(startTime)
	portfolioSpan.SetAttributes(attribute.Int64("operation.duration_ms", duration.Milliseconds()))

	if err != nil {
		portfolioSpan.SetStatus(codes.Error, err.Error())
		portfolioSpan.RecordError(err)
		return nil, fmt.Errorf("failed to create portfolio: %w", err)
	}

	portfolioSpan.SetAttributes(
		attribute.String("portfolio.id", portfolio.ID),
		attribute.String("portfolio.name", portfolio.Name),
	)
	portfolioSpan.SetStatus(codes.Ok, "Portfolio created successfully")

	return portfolio, nil
}

// performComplexWorkflowWithTracing demonstrates a complex workflow with multiple API calls and nested spans
func performComplexWorkflowWithTracing(midazClient *client.Client, provider observability.Provider) error {
	tracer := provider.Tracer()
	logger := provider.Logger()

	ctx, rootSpan := tracer.Start(context.Background(), "complex_business_workflow")
	defer rootSpan.End()

	rootSpan.SetAttributes(
		attribute.String("workflow.name", "setup_complete_ledger_system"),
		attribute.String("workflow.version", "1.0"),
	)

	wc := &workflowContext{
		midazClient: midazClient,
		provider:    provider,
		tracer:      tracer,
		logger:      logger,
	}

	// Step 1: List organizations
	ctx, err := wc.listOrganizationsStep(ctx)
	if err != nil {
		return err
	}

	// Step 2: Create ledger
	ctx, err = wc.createLedgerStep(ctx)
	if err != nil {
		return err
	}

	// Step 3: Create assets
	assetNames := []string{"USD", "EUR", "BTC"}
	ctx = wc.createAssetsStep(ctx, assetNames)

	// Step 4: Create portfolio
	portfolio, err := wc.createPortfolioStep(ctx)
	if err != nil {
		return err
	}

	// Set final status on root span
	rootSpan.SetAttributes(
		attribute.String("organization.id", wc.orgID),
		attribute.String("ledger.id", wc.ledger.ID),
		attribute.String("portfolio.id", portfolio.ID),
		attribute.Int("workflow.assets_created", len(assetNames)),
	)
	rootSpan.SetStatus(codes.Ok, "Complex workflow completed successfully")

	logger.Info("Complex workflow completed successfully",
		"org_id", wc.orgID,
		"ledger_id", wc.ledger.ID,
		"portfolio_id", portfolio.ID,
	)

	return nil
}

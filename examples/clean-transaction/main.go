// Package main demonstrates the clean transaction API of the Midaz Go SDK.
// This example shows how to create transactions using the simplified models
// without any dependency on internal implementation details.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/entities"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/observability"
)

func main() {
	// Create an observability provider with our new functional options
	observabilityProvider, _ := observability.New(context.Background(),
		observability.WithServiceName("clean-transaction-example"),
		observability.WithEnvironment("development"),
		observability.WithComponentEnabled(true, true, true), // Enable tracing, metrics, and logging
	)

	// Setup SDK client with the observability provider using the standardized options pattern
	c, err := client.New(
		client.WithAuthToken("test-token"),
		client.WithEnvironment(config.EnvironmentLocal),
		client.WithObservabilityProvider(observabilityProvider),
		client.WithOnboardingURL("http://localhost:3000/v1"),
		client.WithTransactionURL("http://localhost:3001/v1"),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create a simple transaction using the DSL input format
	// Note that we're using only SDK-specific models, with no direct
	// dependency on lib-commons or other internal libraries
	tx, err := createDSLTransaction(context.Background(), c.Entity.Transactions)
	if err != nil {
		log.Fatalf("Failed to create transaction: %v", err)
	}

	fmt.Printf("Created transaction: %s\n", tx.ID)
}

// createDSLTransaction demonstrates creating a transaction using the DSL format
// This function only uses the public SDK API, with no reference to internal
// implementation details.
func createDSLTransaction(ctx context.Context, txService entities.TransactionsService) (*models.Transaction, error) {
	// Create a DSL transaction input
	input := &models.TransactionDSLInput{
		Description: "Test DSL Transaction",
		Metadata: map[string]interface{}{
			"source": "sdk-example",
			"time":   time.Now().Format(time.RFC3339),
		},
		Send: &models.DSLSend{
			Asset: "USD",
			Value: 100_00, // $100.00
			Scale: 2,
			Source: &models.DSLSource{
				From: []models.DSLFromTo{
					{
						Account: "account123",
						Amount: &models.DSLAmount{
							Asset: "USD",
							Value: 100_00, // $100.00
							Scale: 2,
						},
					},
				},
			},
			Distribute: &models.DSLDistribute{
				To: []models.DSLFromTo{
					{
						Account: "account456",
						Amount: &models.DSLAmount{
							Asset: "USD",
							Value: 100_00, // $100.00
							Scale: 2,
						},
					},
				},
			},
		},
	}

	// Create the transaction
	// Note that the SDK handles all internal conversion behind the scenes
	return txService.CreateTransactionWithDSL(ctx, "org123", "ledger456", input)
}

// This example demonstrates that users of the SDK never need to know about
// or interact with internal implementation details like lib-commons. All those
// details are properly abstracted away by the SDK.

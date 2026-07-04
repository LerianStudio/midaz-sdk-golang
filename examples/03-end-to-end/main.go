// Package main demonstrates the clean transaction API of the Midaz Go SDK.
// This example shows how to create transactions using the simplified models
// without any dependency on internal implementation details.
package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v4"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/observability"
)

func main() {
	// Create an observability provider with our new functional options
	observabilityProvider, err := observability.New(context.Background(),
		observability.WithServiceName("end-to-end-example"),
		observability.WithEnvironment("development"),
		observability.WithComponentEnabled(true, true, true), // Enable tracing, metrics, and logging
	)
	if err != nil {
		log.Fatalf("Failed to create observability provider: %v", err)
	}

	// Setup SDK client with the observability provider using the standardized options pattern.
	// WithAnonymous opts out of auth — required for v3's "exactly one auth source" invariant
	// when running against a local stack with auth disabled.
	c, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
		midaz.WithAnonymous(),
		midaz.WithObservabilityProvider(observabilityProvider),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create a simple transaction using the DSL input format
	// Note that we're using only SDK-specific models, with no direct
	// dependency on lib-commons or other internal libraries
	tx, err := createDSLTransaction(context.Background(), c.Transactions)
	if err != nil {
		log.Fatalf("Failed to create transaction: %s", strconv.Quote(err.Error())) // lgtm[go/log-injection]
	}

	fmt.Printf("Created transaction: %q\n", tx.ID)
}

// transactionCreator is the narrow slice of the transactions accessor this
// example needs. Accepting a small consumer-side interface (rather than naming
// the concrete, unexported facade) is the idiomatic v4 pattern and keeps the
// helper trivially mockable.
type transactionCreator interface {
	CreateJSON(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error)
}

// createDSLTransaction demonstrates creating a transaction with the structured
// input posted to /transactions/json. This function only uses the public SDK
// API, with no reference to internal implementation details.
func createDSLTransaction(ctx context.Context, txService transactionCreator) (*models.Transaction, error) {
	input := &models.CreateTransactionInput{
		Description: "Test DSL Transaction",
		Metadata: map[string]any{
			"source": "sdk-example",
			"time":   time.Now().Format(time.RFC3339),
		},
		Send: &models.SendInput{
			Asset: "USD",
			Value: "100",
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{Account: "account123", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{Account: "account456", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
				},
			},
		},
	}

	return txService.CreateJSON(ctx, "org123", "ledger456", input)
}

// This example demonstrates that users of the SDK never need to know about
// or interact with internal implementation details like lib-commons. All those
// details are properly abstracted away by the SDK.

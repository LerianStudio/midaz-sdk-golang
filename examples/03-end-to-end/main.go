// Package main demonstrates the transaction API of the Midaz Go SDK on the
// /v2 surface — the one to build against, since Midaz deprecated all of /v1.
//
// A /v2 transaction is FLAT: an asset, a total, and two leg arrays (debits and
// credits). The action lives in the URL rather than in the body, so the SDK
// spells it as a method — CreateDirect settles immediately, CreateHold reserves
// value for a later Commit. /v1's four creation styles (json, inflow, outflow,
// annotation) with their nested send/source/distribute envelope have no /v2
// twin; they remain reachable as c.V1.Transactions.CreateJSON and friends for
// as long as the server serves /v1.
//
// Each leg names the organization and ledger its account belongs to. The facade
// stamps those from the pair you address the call with, so you write them once
// rather than on every leg — and it refuses a leg that names a DIFFERENT pair
// rather than silently posting into the wrong ledger.
package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
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
	// WithAnonymous opts out of auth — required by the SDK's "exactly one auth source" invariant
	// when running against a local stack with auth disabled.
	c, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
		midaz.WithAnonymous(),
		midaz.WithObservabilityProvider(observabilityProvider),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create a transaction that settles immediately. Note that we're using only
	// SDK-specific models, with no direct dependency on lib-commons or other
	// internal libraries.
	tx, err := createDirectTransaction(context.Background(), c.V2.Transactions)
	if err != nil {
		log.Fatalf("Failed to create transaction: %s", strconv.Quote(err.Error())) // lgtm[go/log-injection]
	}

	fmt.Printf("Created transaction: %q (status %s)\n", tx.ID, tx.Status.Code)
}

// transactionCreator is the narrow slice of the transactions accessor this
// example needs. Accepting a small consumer-side interface (rather than naming
// the concrete, unexported facade) is the idiomatic pattern here and keeps the
// helper trivially mockable.
type transactionCreator interface {
	CreateDirect(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionV2Input) (*models.TransactionV2, error)
}

// createDirectTransaction posts a two-leg transfer to /v2/transactions/direct:
// 100 USD out of one account and into another, settled on acceptance.
//
// Money travels as a decimal STRING on both surfaces, never as a float. Each
// leg carries EXACTLY ONE value expression — an explicit Amount, or a Share of
// the total — and a leg carrying both, or neither, is refused before the
// request leaves the SDK.
//
// The legs leave OrganizationID and LedgerID empty on purpose: the facade fills
// them from the pair passed to CreateDirect, and it fills a COPY, so this input
// can be reused against a second ledger without carrying the first one's scope
// into it.
func createDirectTransaction(ctx context.Context, txService transactionCreator) (*models.TransactionV2, error) {
	input := &models.CreateTransactionV2Input{
		Asset:       "USD",
		Amount:      "100",
		Description: "Test direct transaction",
		Debits:      []models.TransactionV2Leg{{Alias: "account123", Amount: "100"}},
		Credits:     []models.TransactionV2Leg{{Alias: "account456", Amount: "100"}},
		Metadata: map[string]any{
			"source": "sdk-example",
			"time":   time.Now().Format(time.RFC3339),
		},
	}

	return txService.CreateDirect(ctx, "org123", "ledger456", input)
}

// This example demonstrates that users of the SDK never need to know about
// or interact with internal implementation details like lib-commons. All those
// details are properly abstracted away by the SDK.

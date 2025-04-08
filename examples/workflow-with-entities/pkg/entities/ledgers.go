// Package entities contains entity-specific operations for the complete workflow example.
// It provides higher-level functions that wrap the SDK's core functionality to
// simplify common operations and demonstrate best practices.
package entities

import (
	"context"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/entities"
	"github.com/LerianStudio/midaz-sdk-golang/models"
)

// CreateLedger creates a new ledger within an organization.
//
// This function simplifies ledger creation by handling the construction of the
// CreateLedgerInput model and setting up appropriate metadata. It demonstrates
// how to properly structure ledger creation requests using the builder pattern.
//
// A ledger is a financial record-keeping system that contains accounts and transactions.
// In the Midaz system, ledgers are owned by organizations and can contain multiple
// accounts, assets, and transactions.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The organization ID that will own the ledger. Must be a valid UUID.
//   - service: The LedgersService instance to use for the API call.
//
// Returns:
//   - *models.Ledger: The created ledger if successful.
//   - error: Any error encountered during ledger creation.
//
// Example:
//
//	ledger, err := entities.CreateLedger(
//	    ctx,
//	    "org-123",
//	    sdkEntity.Ledgers,
//	)
func CreateLedger(ctx context.Context, orgID string, service entities.LedgersService) (*models.Ledger, error) {
	// Create input using the builder pattern
	input := models.NewCreateLedgerInput("Main Ledger").
		WithMetadata(map[string]any{
			"description": "Main ledger for example workflow",
			"created_by":  "example workflow",
			"version":     "1.0",
		})

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid ledger input: %w", err)
	}

	// Create ledger
	return service.CreateLedger(ctx, orgID, input)
}

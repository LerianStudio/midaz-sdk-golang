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

// CreateAsset creates a new asset within a ledger.
//
// This function simplifies asset creation by handling the construction of the
// CreateAssetInput model and setting up appropriate metadata. It demonstrates
// how to properly structure asset creation requests using the builder pattern.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The organization ID that owns the ledger. Must be a valid UUID.
//   - ledgerID: The ledger ID where the asset will be created. Must be a valid UUID.
//   - name: Human-readable name for the asset (e.g., "US Dollar").
//   - assetType: Type of asset (e.g., "currency", "security", "commodity").
//   - code: The asset code, typically a standard currency code (e.g., "USD", "EUR").
//   - service: The AssetsService instance to use for the API call.
//
// Returns:
//   - *models.Asset: The created asset if successful.
//   - error: Any error encountered during asset creation.
//
// Example:
//
//	asset, err := entities.CreateAsset(
//	    ctx,
//	    "org-123",
//	    "ledger-456",
//	    "US Dollar",
//	    "currency",
//	    "USD",
//	    sdkEntity.Assets,
//	)
func CreateAsset(
	ctx context.Context,
	orgID, ledgerID, name, assetType, code string,
	service entities.AssetsService,
) (*models.Asset, error) {
	// Create input using the builder pattern
	input := models.NewCreateAssetInput(name, code).
		WithType(assetType).
		WithMetadata(map[string]any{
			"description": fmt.Sprintf("%s asset for %s", code, name),
		})

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid asset input: %w", err)
	}

	// Create asset
	return service.CreateAsset(ctx, orgID, ledgerID, input)
}

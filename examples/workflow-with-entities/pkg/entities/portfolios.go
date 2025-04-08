// Package entities contains entity-specific operations for the complete workflow example.
// It provides higher-level functions that wrap the SDK's core functionality to
// simplify common operations and demonstrate best practices.
package entities

import (
	"context"
	"fmt"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/entities"
	"github.com/LerianStudio/midaz-sdk-golang/models"
)

// CreatePortfolio creates a new portfolio within a ledger.
//
// This function simplifies portfolio creation by handling the construction of the
// CreatePortfolioInput model and setting up appropriate metadata. It demonstrates
// how to properly structure portfolio creation requests using the builder pattern.
//
// Portfolios in the Midaz system are collections of accounts that can be managed
// together. They can be used to group accounts by purpose, owner, or any other
// organizational criteria.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The organization ID that owns the ledger. Must be a valid UUID.
//   - ledgerID: The ledger ID where the portfolio will be created. Must be a valid UUID.
//   - name: Human-readable name for the portfolio (e.g., "Investment Portfolio").
//   - description: A description of the portfolio's purpose.
//   - service: The PortfoliosService instance to use for the API call.
//
// Returns:
//   - *models.Portfolio: The created portfolio if successful.
//   - error: Any error encountered during portfolio creation.
//
// Example:
//
//	portfolio, err := entities.CreatePortfolio(
//	    ctx,
//	    "org-123",
//	    "ledger-456",
//	    "Main Portfolio",
//	    "Portfolio for managing customer accounts",
//	    sdkEntity.Portfolios,
//	)
func CreatePortfolio(
	ctx context.Context,
	orgID, ledgerID, name, description string,
	service entities.PortfoliosService,
) (*models.Portfolio, error) {
	// Create a mock entity ID for the portfolio
	// In a real application, this might be a meaningful identifier
	entityID := "entity-" + fmt.Sprintf("%d", time.Now().UnixNano()%100000)

	// Create input using the builder pattern
	input := models.NewCreatePortfolioInput(entityID, name)

	// Add metadata - note that description is stored in metadata
	// since the Portfolio model doesn't have a dedicated description field
	input = input.WithMetadata(map[string]any{
		"purpose":     "Example portfolio",
		"description": description,
		"created_at":  time.Now().Format(time.RFC3339),
	})

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid portfolio input: %w", err)
	}

	// Create portfolio
	return service.CreatePortfolio(ctx, orgID, ledgerID, input)
}

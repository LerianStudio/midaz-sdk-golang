// Package entities contains entity-specific operations for the complete workflow example.
// It provides higher-level functions that wrap the SDK's core functionality to
// simplify common operations and demonstrate best practices.
package entities

import (
	"context"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
)

// CreateSegment creates a new segment within a portfolio.
//
// This function simplifies segment creation by handling the construction of the
// CreateSegmentInput model and setting up appropriate metadata. It demonstrates
// how to properly structure segment creation requests using the builder pattern.
//
// Segments in the Midaz system are subdivisions of portfolios that can be used to
// further categorize accounts. They provide an additional level of organization
// beyond portfolios, allowing for more granular management of accounts.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The organization ID that owns the ledger. Must be a valid UUID.
//   - ledgerID: The ledger ID where the portfolio exists. Must be a valid UUID.
//   - name: Human-readable name for the segment (e.g., "Retail Customers").
//   - service: The SegmentsService instance to use for the API call.
//
// Returns:
//   - *models.Segment: The created segment if successful.
//   - error: Any error encountered during segment creation.
//
// Example:
//
//	segment, err := entities.CreateSegment(
//	    ctx,
//	    "org-123",
//	    "ledger-456",
//	    "Premium Customers",
//	    sdkEntity.Segments,
//	)
func CreateSegment(
	ctx context.Context,
	orgID, ledgerID, name string,
	service entities.SegmentsService,
) (*models.Segment, error) {
	// Create input using the builder pattern
	input := models.NewCreateSegmentInput(name)

	// Add metadata to provide additional information about the segment
	input = input.WithMetadata(map[string]any{
		"purpose":     "Example segment",
		"description": "Segment created for demonstration purposes",
	})

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid segment input: %w", err)
	}

	// Create segment
	return service.CreateSegment(ctx, orgID, ledgerID, input)
}

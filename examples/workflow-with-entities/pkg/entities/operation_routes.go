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

// CreateOperationRoute creates a new operation route within a ledger.
//
// This function simplifies operation route creation by handling the construction of the
// CreateOperationRouteInput model and setting up appropriate metadata. It demonstrates
// how to properly structure operation route creation requests.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The organization ID that owns the ledger. Must be a valid UUID.
//   - ledgerID: The ledger ID where the operation route will be created. Must be a valid UUID.
//   - title: Human-readable title for the operation route (e.g., "Cash-in Route").
//   - description: Description of the operation route's purpose.
//   - routeType: Type of operation route (e.g., "source", "destination").
//   - service: The OperationRoutesService instance to use for the API call.
//
// Returns:
//   - *models.OperationRoute: The created operation route if successful.
//   - error: Any error encountered during operation route creation.
//
// Example:
//
//	operationRoute, err := entities.CreateOperationRoute(
//	    ctx,
//	    "org-123",
//	    "ledger-456",
//	    "Cash-in Route",
//	    "Handles cash-in operations",
//	    "source",
//	    sdkEntity.OperationRoutes,
//	)
func CreateOperationRoute(
	ctx context.Context,
	orgID, ledgerID, title, description, routeType string,
	service entities.OperationRoutesService,
) (*models.OperationRoute, error) {
	// Create input with required fields
	input := models.NewCreateOperationRouteInput(title, description, routeType)

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid operation route input: %w", err)
	}

	// Create operation route
	return service.CreateOperationRoute(ctx, orgID, ledgerID, input)
}

// ListOperationRoutes lists all operation routes for a ledger with consistent pattern.
//
// This function retrieves all operation routes from the specified ledger and converts
// the response to a slice of pointers for easier manipulation in workflow examples.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The organization ID that owns the ledger. Must be a valid UUID.
//   - ledgerID: The ledger ID to query for operation routes. Must be a valid UUID.
//   - service: The OperationRoutesService instance to use for the API call.
//
// Returns:
//   - []*models.OperationRoute: A slice of pointers to operation routes.
//   - error: Any error encountered during the list operation.
//
// Example:
//
//	operationRoutes, err := entities.ListOperationRoutes(
//	    ctx,
//	    "org-123",
//	    "ledger-456",
//	    sdkEntity.OperationRoutes,
//	)
func ListOperationRoutes(
	ctx context.Context,
	orgID, ledgerID string,
	service entities.OperationRoutesService,
) ([]*models.OperationRoute, error) {
	// List operation routes
	response, err := service.ListOperationRoutes(ctx, orgID, ledgerID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list operation routes: %w", err)
	}

	// Extract operation routes from response
	if response == nil {
		return []*models.OperationRoute{}, nil
	}

	// Convert slice of models.OperationRoute to slice of *models.OperationRoute
	operationRoutes := make([]*models.OperationRoute, len(response.Items))
	for i := range response.Items {
		operationRoutes[i] = &response.Items[i]
	}

	return operationRoutes, nil
}

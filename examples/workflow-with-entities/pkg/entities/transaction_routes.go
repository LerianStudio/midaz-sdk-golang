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

// CreateTransactionRoute creates a new transaction route within a ledger.
//
// This function simplifies transaction route creation by handling the construction of the
// CreateTransactionRouteInput model and setting up appropriate metadata. It demonstrates
// how to properly structure transaction route creation requests.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The organization ID that owns the ledger. Must be a valid UUID.
//   - ledgerID: The ledger ID where the transaction route will be created. Must be a valid UUID.
//   - title: Human-readable title for the transaction route (e.g., "Payment Route").
//   - description: Description of the transaction route's purpose.
//   - operationRoutes: List of operation route IDs that belong to this transaction route.
//   - service: The TransactionRoutesService instance to use for the API call.
//
// Returns:
//   - *models.TransactionRoute: The created transaction route if successful.
//   - error: Any error encountered during transaction route creation.
//
// Example:
//
//	transactionRoute, err := entities.CreateTransactionRoute(
//	    ctx,
//	    "org-123",
//	    "ledger-456",
//	    "Payment Route",
//	    "Handles payment transactions",
//	    []string{"route-123", "route-456"},
//	    sdkEntity.TransactionRoutes,
//	)
func CreateTransactionRoute(
	ctx context.Context,
	orgID, ledgerID, title, description string,
	operationRoutes []string,
	service entities.TransactionRoutesService,
) (*models.TransactionRoute, error) {
	// Create input with required fields
	input := models.NewCreateTransactionRouteInput(title, description, operationRoutes)
	input.Metadata = map[string]any{
		"purpose":          "Workflow example transaction route",
		"created_by":       "workflow_example",
		"operation_count":  len(operationRoutes),
		"transaction_type": "standard",
	}

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction route input: %w", err)
	}

	// Create transaction route
	return service.CreateTransactionRoute(ctx, orgID, ledgerID, input)
}

// ListTransactionRoutes lists all transaction routes for a ledger with consistent pattern.
//
// This function retrieves all transaction routes from the specified ledger and converts
// the response to a slice of pointers for easier manipulation in workflow examples.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The organization ID that owns the ledger. Must be a valid UUID.
//   - ledgerID: The ledger ID to query for transaction routes. Must be a valid UUID.
//   - service: The TransactionRoutesService instance to use for the API call.
//
// Returns:
//   - []*models.TransactionRoute: A slice of pointers to transaction routes.
//   - error: Any error encountered during the list operation.
//
// Example:
//
//	transactionRoutes, err := entities.ListTransactionRoutes(
//	    ctx,
//	    "org-123",
//	    "ledger-456",
//	    sdkEntity.TransactionRoutes,
//	)
func ListTransactionRoutes(
	ctx context.Context,
	orgID, ledgerID string,
	service entities.TransactionRoutesService,
) ([]*models.TransactionRoute, error) {
	// List transaction routes
	response, err := service.ListTransactionRoutes(ctx, orgID, ledgerID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list transaction routes: %w", err)
	}

	// Extract transaction routes from response
	if response == nil {
		return []*models.TransactionRoute{}, nil
	}

	// Convert slice of models.TransactionRoute to slice of *models.TransactionRoute
	transactionRoutes := make([]*models.TransactionRoute, len(response.Items))
	for i := range response.Items {
		transactionRoutes[i] = &response.Items[i]
	}

	return transactionRoutes, nil
}

package workflows

import (
	"context"
	"fmt"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
)

// CreateOperationRoutes creates multiple operation routes and returns their models
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - client: The initialized Midaz SDK client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - accountType: The account type to associate with operation routes
//
// Returns:
//   - *models.OperationRoute: The source operation route model
//   - *models.OperationRoute: The destination operation route model
//   - error: Any error encountered during the operation
func CreateOperationRoutes(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, accountType *models.AccountType) (*models.OperationRoute, *models.OperationRoute, error) {
	fmt.Println("\n\n🛤️  STEP 4.6: OPERATION ROUTE CREATION")
	fmt.Println(strings.Repeat("=", 50))

	// Create source operation route following the exact specification provided
	fmt.Println("Creating source operation route (using alias rule)...")
	fmt.Printf("Using external BRL account alias: @external/BRL\n")
	
	// Use the corrected SDK with single string for alias rule (matches API spec)
	sourceInput := models.NewCreateOperationRouteInput(
		"Cashin from service charge",
		"This operation route handles cash-in transactions from service charge collections",
		"source", // source operation type (where money comes FROM)
	).WithAccountAlias("@external/BRL").WithMetadata(map[string]any{
		"customField1": "value1",
	})
	
	sourceOperationRoute, err := midazClient.Entity.OperationRoutes.CreateOperationRoute(
		ctx, orgID, ledgerID, sourceInput,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create source operation route: %w", err)
	}

	if sourceOperationRoute.ID == "" {
		return nil, nil, fmt.Errorf("source operation route created but no ID was returned from the API")
	}

	fmt.Printf("✅ Source operation route created: %s\n", sourceOperationRoute.Title)
	fmt.Printf("   ID: %s\n", sourceOperationRoute.ID)
	fmt.Printf("   OperationType: %s\n", sourceOperationRoute.OperationType)
	fmt.Printf("   Account RuleType: %s\n", sourceOperationRoute.Account.RuleType)
	fmt.Printf("   Account ValidIf: %v\n", sourceOperationRoute.Account.ValidIf)
	fmt.Printf("   Description: %s\n", sourceOperationRoute.Description)
	fmt.Printf("   Created: %s\n", sourceOperationRoute.CreatedAt.Format("2006-01-02 15:04:05"))

	fmt.Println()

	// Create destination operation route using account_type rule with multiple types
	fmt.Println("Creating destination operation route (using account_type rule)...")

	destinationInput := models.NewCreateOperationRouteInput(
		"Revenue Collection Route",
		"Route for revenue and liability operations",
		"destination", // destination operation type (funds entering destination)
	).WithAccountTypes([]string{"liability", "revenue"})

	destinationOperationRoute, err := midazClient.Entity.OperationRoutes.CreateOperationRoute(
		ctx, orgID, ledgerID, destinationInput,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create destination operation route: %w", err)
	}

	if destinationOperationRoute.ID == "" {
		return nil, nil, fmt.Errorf("destination operation route created but no ID was returned from the API")
	}

	fmt.Printf("✅ Destination operation route created: %s\n", destinationOperationRoute.Title)
	fmt.Printf("   ID: %s\n", destinationOperationRoute.ID)
	fmt.Printf("   OperationType: %s\n", destinationOperationRoute.OperationType)
	fmt.Printf("   Account RuleType: %s\n", destinationOperationRoute.Account.RuleType)
	fmt.Printf("   Account ValidIf: %v\n", destinationOperationRoute.Account.ValidIf)
	fmt.Printf("   Description: %s\n", destinationOperationRoute.Description)
	fmt.Printf("   Created: %s\n", destinationOperationRoute.CreatedAt.Format("2006-01-02 15:04:05"))

	return sourceOperationRoute, destinationOperationRoute, nil
}

// GetOperationRoute retrieves a specific operation route by ID
//
// Parameters:
//   - ctx: The context for the operation
//   - client: The initialized Midaz SDK client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - operationRouteID: The operation route ID to retrieve
//
// Returns:
//   - *models.OperationRoute: The retrieved operation route
//   - error: Any error encountered during the operation
func GetOperationRoute(ctx context.Context, midazClient *client.Client, orgID, ledgerID, operationRouteID string) (*models.OperationRoute, error) {
	fmt.Println("\n🔍 Getting Operation Route by ID...")
	fmt.Printf("   Retrieving operation route: %s\n", operationRouteID)

	operationRoute, err := midazClient.Entity.OperationRoutes.GetOperationRoute(ctx, orgID, ledgerID, operationRouteID)
	if err != nil {
		return nil, fmt.Errorf("failed to get operation route: %w", err)
	}

	fmt.Printf("✅ Operation route retrieved: %s\n", operationRoute.Title)
	fmt.Printf("   ID: %s\n", operationRoute.ID)
	fmt.Printf("   OperationType: %s\n", operationRoute.OperationType)
	fmt.Printf("   Description: %s\n", operationRoute.Description)
	fmt.Printf("   Account RuleType: %s\n", operationRoute.Account.RuleType)
	fmt.Printf("   Account ValidIf: %v\n", operationRoute.Account.ValidIf)
	fmt.Printf("   Created: %s\n", operationRoute.CreatedAt.Format("2006-01-02 15:04:05"))

	return operationRoute, nil
}

// ListOperationRoutes lists all operation routes in the specified organization and ledger
//
// Parameters:
//   - ctx: The context for the operation
//   - client: The initialized Midaz SDK client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//
// Returns:
//   - *models.ListResponse[models.OperationRoute]: The list of operation routes
//   - error: Any error encountered during the operation
func ListOperationRoutes(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) (*models.ListResponse[models.OperationRoute], error) {
	fmt.Println("\n📋 Listing Operation Routes...")

	listOpts := &models.ListOptions{
		Limit: 10,
		Page:  1,
	}
	
	routesList, err := midazClient.Entity.OperationRoutes.ListOperationRoutes(ctx, orgID, ledgerID, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list operation routes: %w", err)
	}

	fmt.Printf("✅ Found %d operation routes:\n", len(routesList.Items))
	for i, route := range routesList.Items {
		fmt.Printf("   %d. %s (ID: %s, Type: %s)\n", i+1, route.Title, route.ID, route.OperationType)
		fmt.Printf("      Description: %s\n", route.Description)
		fmt.Printf("      Account: %s - %v\n", route.Account.RuleType, route.Account.ValidIf)
	}

	return routesList, nil
}

// UpdateOperationRoute updates an existing operation route
//
// Parameters:
//   - ctx: The context for the operation
//   - client: The initialized Midaz SDK client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - operationRouteID: The operation route ID to update
//   - title: New title for the operation route
//   - description: New description for the operation route
//   - accountTypes: New account types for validation
//
// Returns:
//   - *models.OperationRoute: The updated operation route
//   - error: Any error encountered during the operation
func UpdateOperationRoute(ctx context.Context, midazClient *client.Client, orgID, ledgerID, operationRouteID, title, description string, accountTypes []string) (*models.OperationRoute, error) {
	fmt.Println("\n✏️  Updating Operation Route...")
	fmt.Printf("   Updating operation route: %s\n", operationRouteID)

	updateInput := models.NewUpdateOperationRouteInput().
		WithTitle(title).
		WithDescription(description).
		WithAccountTypes(accountTypes).
		WithMetadata(map[string]any{
			"updated_at": "workflow_execution",
			"version":    "2.0",
		})

	updatedRoute, err := midazClient.Entity.OperationRoutes.UpdateOperationRoute(ctx, orgID, ledgerID, operationRouteID, updateInput)
	if err != nil {
		return nil, fmt.Errorf("failed to update operation route: %w", err)
	}

	fmt.Printf("✅ Operation route updated: %s\n", updatedRoute.Title)
	fmt.Printf("   ID: %s\n", updatedRoute.ID)
	fmt.Printf("   OperationType: %s (unchanged)\n", updatedRoute.OperationType)
	fmt.Printf("   Account RuleType: %s\n", updatedRoute.Account.RuleType)
	fmt.Printf("   Account ValidIf: %v\n", updatedRoute.Account.ValidIf)
	fmt.Printf("   Updated: %s\n", updatedRoute.UpdatedAt.Format("2006-01-02 15:04:05"))

	return updatedRoute, nil
}

// DeleteOperationRoute deletes an operation route
//
// Parameters:
//   - ctx: The context for the operation
//   - client: The initialized Midaz SDK client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - operationRouteID: The operation route ID to delete
//
// Returns:
//   - error: Any error encountered during the operation
func DeleteOperationRoute(ctx context.Context, midazClient *client.Client, orgID, ledgerID, operationRouteID string) error {
	fmt.Println("\n🗑️  Deleting Operation Route...")
	fmt.Printf("   Deleting operation route: %s\n", operationRouteID)

	err := midazClient.Entity.OperationRoutes.DeleteOperationRoute(ctx, orgID, ledgerID, operationRouteID)
	if err != nil {
		return fmt.Errorf("failed to delete operation route: %w", err)
	}

	fmt.Printf("✅ Operation route deleted: %s\n", operationRouteID)

	// Verify deletion
	fmt.Println("   🔍 Verifying deletion...")
	_, err = midazClient.Entity.OperationRoutes.GetOperationRoute(ctx, orgID, ledgerID, operationRouteID)
	if err != nil {
		fmt.Printf("   ✅ Confirmed deletion - operation route no longer exists\n")
	} else {
		return fmt.Errorf("operation route still exists after deletion")
	}

	return nil
}

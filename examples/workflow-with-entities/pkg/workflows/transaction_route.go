package workflows

import (
	"context"
	"fmt"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/google/uuid"
)

// CreateTransactionRoutes creates multiple transaction routes and returns their models
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - client: The initialized Midaz SDK client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//
// Returns:
//   - *models.TransactionRoute: The payment transaction route model
//   - *models.TransactionRoute: The refund transaction route model
//   - error: Any error encountered during the operation
func CreateTransactionRoutes(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) (*models.TransactionRoute, *models.TransactionRoute, error) {
	var sourceOperationRoute, destinationOperationRoute *models.OperationRoute
	return CreateTransactionRoutesWithOperationRoutes(ctx, midazClient, orgID, ledgerID, sourceOperationRoute, destinationOperationRoute)
}

// CreateTransactionRoutesWithOperationRoutes creates multiple transaction routes linked to operation routes
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - client: The initialized Midaz SDK client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - sourceOperationRoute: The source operation route (can be nil)
//   - destinationOperationRoute: The destination operation route (can be nil)
//
// Returns:
//   - *models.TransactionRoute: The payment transaction route model
//   - *models.TransactionRoute: The refund transaction route model
//   - error: Any error encountered during the operation
func CreateTransactionRoutesWithOperationRoutes(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, sourceOperationRoute, destinationOperationRoute *models.OperationRoute) (*models.TransactionRoute, *models.TransactionRoute, error) {
	fmt.Println("\n\n🗺️  STEP 4.5: TRANSACTION ROUTE CREATION")
	fmt.Println(strings.Repeat("=", 50))

	// Prepare operation route IDs
	var operationRouteIDs []string
	if sourceOperationRoute != nil && destinationOperationRoute != nil {
		operationRouteIDs = []string{sourceOperationRoute.ID.String(), destinationOperationRoute.ID.String()}
		fmt.Printf("🔗 Linking transaction routes to operation routes:\n")
		fmt.Printf("   Source Operation Route: %s (%s)\n", sourceOperationRoute.Title, sourceOperationRoute.ID)
		fmt.Printf("   Destination Operation Route: %s (%s)\n", destinationOperationRoute.Title, destinationOperationRoute.ID)
	} else {
		fmt.Printf("⚠️  No operation routes provided - cannot create transaction routes as they require operation routes\n")
		fmt.Printf("   Note: Transaction routes creation will be skipped due to missing operation routes\n")
		return nil, nil, fmt.Errorf("operation routes are required for transaction routes creation")
	}

	// Create payment transaction route
	fmt.Println("Creating payment transaction route...")

	paymentTransactionRoute, err := midazClient.Entity.TransactionRoutes.CreateTransactionRoute(
		ctx, orgID, ledgerID,
		models.NewCreateTransactionRouteInput(
			"Payment Transaction Route",
			"Handles payment transactions for business operations",
			operationRouteIDs,
		).WithMetadata(map[string]any{"purpose": "payment_processing", "type": "payment"}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create payment transaction route: %w", err)
	}

	if paymentTransactionRoute.ID == uuid.Nil {
		return nil, nil, fmt.Errorf("payment transaction route created but no ID was returned from the API")
	}

	fmt.Printf("✅ Payment transaction route created: %s\n", paymentTransactionRoute.Title)
	fmt.Printf("   ID: %s\n", paymentTransactionRoute.ID)
	fmt.Printf("   Description: %s\n", paymentTransactionRoute.Description)
	fmt.Printf("   Operation Routes: %v\n", paymentTransactionRoute.OperationRoutes)
	fmt.Printf("   Created: %s\n", paymentTransactionRoute.CreatedAt.Format("2006-01-02 15:04:05"))

	fmt.Println()

	// Create refund transaction route
	fmt.Println("Creating refund transaction route...")

	refundTransactionRoute, err := midazClient.Entity.TransactionRoutes.CreateTransactionRoute(
		ctx, orgID, ledgerID,
		models.NewCreateTransactionRouteInput(
			"Refund Transaction Route",
			"Handles refund transactions for business operations",
			operationRouteIDs,
		).WithMetadata(map[string]any{"purpose": "refund_processing", "type": "refund"}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create refund transaction route: %w", err)
	}

	if refundTransactionRoute.ID == uuid.Nil {
		return nil, nil, fmt.Errorf("refund transaction route created but no ID was returned from the API")
	}

	fmt.Printf("✅ Refund transaction route created: %s\n", refundTransactionRoute.Title)
	fmt.Printf("   ID: %s\n", refundTransactionRoute.ID)
	fmt.Printf("   Description: %s\n", refundTransactionRoute.Description)
	fmt.Printf("   Operation Routes: %v\n", refundTransactionRoute.OperationRoutes)
	fmt.Printf("   Created: %s\n", refundTransactionRoute.CreatedAt.Format("2006-01-02 15:04:05"))

	return paymentTransactionRoute, refundTransactionRoute, nil
}

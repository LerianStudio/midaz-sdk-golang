package workflows

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang"
	sdkentities "github.com/LerianStudio/midaz-sdk-golang/entities"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	auth "github.com/LerianStudio/midaz-sdk-golang/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/config"
)

// RunCompleteWorkflow runs a complete workflow demonstrating all the features of the Midaz SDK
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - customerToMerchantTxs: Number of customer to merchant transactions to execute
//   - merchantToCustomerTxs: Number of merchant to customer transactions to execute
//
// Returns:
//   - error: Any error encountered during the operation
func RunCompleteWorkflow(ctx context.Context, entity *sdkentities.Entity, customerToMerchantTxs, merchantToCustomerTxs int) error {
	fmt.Println("\n🚀 STARTING COMPLETE WORKFLOW")
	fmt.Println(strings.Repeat("=", 50))

	// Set the global variables for concurrent transactions
	concurrentCustomerToMerchantTxs = customerToMerchantTxs
	concurrentMerchantToCustomerTxs = merchantToCustomerTxs

	// Get plugin auth configuration from environment variables
	pluginAuthEnabled := os.Getenv("PLUGIN_AUTH_ENABLED") == "true"
	pluginAuthAddress := os.Getenv("PLUGIN_AUTH_ADDRESS")

	// Use MIDAZ_CLIENT_ID and MIDAZ_CLIENT_SECRET as they are defined in the .env file
	clientID := os.Getenv("MIDAZ_CLIENT_ID")
	clientSecret := os.Getenv("MIDAZ_CLIENT_SECRET")

	//Configure plugin auth
	pluginAuth := auth.AccessManager{
		Enabled:      pluginAuthEnabled,
		Address:      pluginAuthAddress,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	cfg, err := config.NewConfig(
		config.WithAccessManager(pluginAuth),
	)
	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
	}

	midazClient, err := client.New(
		client.WithConfig(cfg),
		client.UseEntityAPI(),                      // Enable the Entity API
		client.WithObservability(true, true, true), // Enable observability
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Step 1: Create an organization
	orgID, err := CreateOrganization(ctx, midazClient)
	if err != nil {
		return err
	}

	// Step 2: Update the organization
	if err := UpdateOrganization(ctx, midazClient, orgID); err != nil {
		return err
	}

	// Step 3: Create a ledger
	ledgerID, err := CreateLedger(ctx, midazClient, orgID)
	if err != nil {
		return err
	}

	// Step 4: Create an asset
	if err := CreateAsset(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	// Step 4.1: Create account type
	accountType, err := CreateAccountType(ctx, midazClient, orgID, ledgerID)
	if err != nil {
		return err
	}

	// Step 4.2: Update account type
	if err := UpdateAccountType(ctx, midazClient, orgID, ledgerID, accountType.ID); err != nil {
		return err
	}

	// Step 4.3: Get account type
	if err := GetAccountType(ctx, midazClient, orgID, ledgerID, accountType.ID); err != nil {
		return err
	}

	// Step 4.4: List account types
	if err := ListAccountTypes(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	// Step 4.5: Create operation routes for enhanced transactions
	fmt.Printf("🔍 Testing operation routes API availability...\n")
	sourceOperationRoute, destinationOperationRoute, err := CreateOperationRoutes(ctx, midazClient, orgID, ledgerID, accountType)
	if err != nil {
		fmt.Printf("⚠️  Operation routes API not available on server: %v\n", err)
		fmt.Printf("   Note: SDK has full operation routes implementation, but server endpoint not ready\n")
		fmt.Printf("   Note: Continuing with transaction routes only\n")
		sourceOperationRoute = nil
		destinationOperationRoute = nil
	} else {
		// Step 4.5.1: Demonstrate Operation Route CRUD operations
		fmt.Printf("🧪 Demonstrating Operation Route CRUD operations...\n")
		if err := demonstrateOperationRouteCRUD(ctx, midazClient, orgID, ledgerID, accountType, sourceOperationRoute, destinationOperationRoute); err != nil {
			fmt.Printf("⚠️  Operation Route CRUD demonstration failed: %v\n", err)
		}
	}

	// Step 4.6: Create transaction routes linked to operation routes
	fmt.Printf("🔍 Testing transaction routes API availability...\n")
	paymentTransactionRoute, refundTransactionRoute, err := CreateTransactionRoutesWithOperationRoutes(ctx, midazClient, orgID, ledgerID, sourceOperationRoute, destinationOperationRoute)
	if err != nil {
		fmt.Printf("⚠️  Transaction routes API not available on server: %v\n", err)
		fmt.Printf("   Note: SDK has full transaction routes implementation, but server endpoint not ready\n")
		// Create mock routes for demonstration
		paymentTransactionRoute, refundTransactionRoute = CreateMockTransactionRoutes(orgID, ledgerID)
	}

	// Step 5: Create accounts (pass accountTypeID to link accounts to the account type)
	customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount, err := CreateAccountsWithType(ctx, midazClient, orgID, ledgerID, accountType.ID)
	if err != nil {
		return err
	}

	// Step 6: Execute transactions with enhanced parameters (now with real routes)
	if err := ExecuteTransactionsWithRoutes(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, sourceOperationRoute, destinationOperationRoute, paymentTransactionRoute, refundTransactionRoute); err != nil {
		return err
	}

	// Step 6B: Demonstrate transaction helpers
	if err := DemonstrateTransactionHelpers(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount); err != nil {
		return err
	}

	// Step 7: Create a portfolio
	portfolioID, err := CreatePortfolio(ctx, midazClient, orgID, ledgerID)
	if err != nil {
		return err
	}

	// Step 8: Create segments
	if err := CreateSegments(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	// Step 9: List accounts
	if err := ListAccounts(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	// Step 10: Retrieve organization
	if err := RetrieveOrganization(ctx, midazClient, orgID); err != nil {
		return err
	}

	// Step 11: Test Get methods
	if err := TestGetMethods(ctx, midazClient, orgID, ledgerID, customerAccount.ID, portfolioID); err != nil {
		return err
	}

	// Step 12: Test List methods
	if err := TestListMethods(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	// Step 13: Test Delete methods
	if err := TestDeleteMethods(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	fmt.Println("\n\n✅ COMPLETE WORKFLOW FINISHED SUCCESSFULLY")
	fmt.Println(strings.Repeat("=", 50))
	return nil
}

// CreateMockTransactionRoutes creates mock transaction routes when server API is not available
func CreateMockTransactionRoutes(orgID, ledgerID string) (*models.TransactionRoute, *models.TransactionRoute) {
	fmt.Println("🔧 Creating mock transaction routes (server API not available)...")
	
	paymentRoute := &models.TransactionRoute{
		ID:             "payment-route-" + orgID[:8],
		Title:          "Payment Transaction Route", 
		Description:    "Transaction route for payment operations",
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Metadata: map[string]any{
			"type": "payment",
			"demo": true,
		},
	}
	
	refundRoute := &models.TransactionRoute{
		ID:             "refund-route-" + orgID[:8],
		Title:          "Refund Transaction Route",
		Description:    "Transaction route for refund operations", 
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Metadata: map[string]any{
			"type": "refund",
			"demo": true,
		},
	}
	
	fmt.Printf("✅ Mock transaction routes created:\n")
	fmt.Printf("   Payment: %s (%s)\n", paymentRoute.Title, paymentRoute.ID)
	fmt.Printf("   Refund: %s (%s)\n", refundRoute.Title, refundRoute.ID)
	
	return paymentRoute, refundRoute
}

// demonstrateOperationRouteCRUD demonstrates all CRUD operations for Operation Routes
func demonstrateOperationRouteCRUD(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, accountType *models.AccountType, sourceRoute, destinationRoute *models.OperationRoute) error {
	fmt.Println("\n\n🛤️  OPERATION ROUTE CRUD DEMONSTRATION")
	fmt.Println(strings.Repeat("=", 50))

	// Step 1: List existing operation routes
	fmt.Println("\n📋 Step 1: LIST existing Operation Routes")
	_, err := ListOperationRoutes(ctx, midazClient, orgID, ledgerID)
	if err != nil {
		return fmt.Errorf("failed to list operation routes: %w", err)
	}

	// Step 2: Get operation route by ID (using the source route)
	if sourceRoute != nil {
		fmt.Println("\n🔍 Step 2: GET Operation Route by ID")
		_, err := GetOperationRoute(ctx, midazClient, orgID, ledgerID, sourceRoute.ID)
		if err != nil {
			return fmt.Errorf("failed to get operation route: %w", err)
		}
	}

	// Step 3: Update operation route (using the destination route)
	if destinationRoute != nil {
		fmt.Println("\n✏️  Step 3: UPDATE Operation Route")
		_, err := UpdateOperationRoute(ctx, midazClient, orgID, ledgerID, destinationRoute.ID, 
			"Updated Cash-out Route", 
			"Updated route for cash-out operations with enhanced features",
			[]string{"liability", "revenue", "expense"})
		if err != nil {
			return fmt.Errorf("failed to update operation route: %w", err)
		}

		// Verify the update by getting it again
		fmt.Println("\n🔄 Step 3.1: VERIFY Update by Getting Again")
		_, err = GetOperationRoute(ctx, midazClient, orgID, ledgerID, destinationRoute.ID)
		if err != nil {
			return fmt.Errorf("failed to verify updated operation route: %w", err)
		}
	}

	// Step 4: Create a new operation route for deletion demonstration
	fmt.Println("\n📝 Step 4: CREATE Operation Route for Deletion Demo")
	demoInput := models.NewCreateOperationRouteInput(
		"Demo Route for Deletion",
		"This route will be deleted to demonstrate DELETE operation",
		"source",
	).WithAccountTypes([]string{"demo_type"}).WithMetadata(map[string]any{
		"demo": true,
		"purpose": "deletion_test",
	})

	demoRoute, err := midazClient.Entity.OperationRoutes.CreateOperationRoute(ctx, orgID, ledgerID, demoInput)
	if err != nil {
		return fmt.Errorf("failed to create demo operation route: %w", err)
	}

	fmt.Printf("✅ Demo operation route created for deletion: %s\n", demoRoute.Title)
	fmt.Printf("   ID: %s\n", demoRoute.ID)

	// Step 5: Delete the demo operation route
	fmt.Println("\n🗑️  Step 5: DELETE Operation Route")
	err = DeleteOperationRoute(ctx, midazClient, orgID, ledgerID, demoRoute.ID)
	if err != nil {
		return fmt.Errorf("failed to delete operation route: %w", err)
	}

	fmt.Println("\n🎉 All Operation Route CRUD operations demonstrated successfully!")
	return nil
}

// Function declarations to satisfy the compiler
// These are placeholder functions that will be implemented in separate files
var (
	TestGetMethods    func(ctx context.Context, midazClient *client.Client, orgID, ledgerID, accountID string, portfolioID string) error
	TestListMethods   func(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error
	TestDeleteMethods func(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error
)

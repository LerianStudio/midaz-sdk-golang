package workflows

import (
	"context"
	"fmt"
	"os"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	sdkentities "github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	auth "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
	"github.com/google/uuid"
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

	// Initialize the Midaz client
	midazClient, err := initializeMidazClient()
	if err != nil {
		return err
	}

	// Execute core setup phase
	orgID, ledgerID, accountType, err := executeCoreSetup(ctx, midazClient)
	if err != nil {
		return err
	}

	// Execute routes setup phase
	sourceOperationRoute, destinationOperationRoute, paymentTransactionRoute, refundTransactionRoute := executeRoutesSetup(ctx, midazClient, orgID, ledgerID, accountType)

	// Execute accounts and transactions phase
	accounts, err := executeAccountsAndTransactions(ctx, midazClient, orgID, ledgerID, accountType, sourceOperationRoute, destinationOperationRoute, paymentTransactionRoute, refundTransactionRoute)
	if err != nil {
		return err
	}

	// Execute additional resources phase
	portfolioID, err := executeAdditionalResources(ctx, midazClient, orgID, ledgerID)
	if err != nil {
		return err
	}

	// Execute testing phase
	if err := executeTestingPhase(ctx, midazClient, orgID, ledgerID, accounts.customerAccount.ID, portfolioID); err != nil {
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
		ID:             uuid.New(),
		Title:          "Payment Transaction Route",
		Description:    "Transaction route for payment operations",
		OrganizationID: uuid.MustParse(orgID),
		LedgerID:       uuid.MustParse(ledgerID),
		Metadata: map[string]any{
			"type": "payment",
			"demo": true,
		},
	}

	refundRoute := &models.TransactionRoute{
		ID:             uuid.New(),
		Title:          "Refund Transaction Route",
		Description:    "Transaction route for refund operations",
		OrganizationID: uuid.MustParse(orgID),
		LedgerID:       uuid.MustParse(ledgerID),
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

// workflowAccounts holds the account references for the workflow
type workflowAccounts struct {
	customerAccount *models.Account
	merchantAccount *models.Account
	dummyOneAccount *models.Account
	dummyTwoAccount *models.Account
}

// initializeMidazClient initializes and configures the Midaz client
func initializeMidazClient() (*client.Client, error) {
	pluginAuth := createPluginAuth()

	cfg, err := config.NewConfig(config.WithAccessManager(pluginAuth))
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	midazClient, err := client.New(
		client.WithConfig(cfg),
		client.UseEntityAPI(),                      // Enable the Entity API
		client.WithObservability(true, true, true), // Enable observability
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return midazClient, nil
}

// createPluginAuth creates the plugin authentication configuration
func createPluginAuth() auth.AccessManager {
	pluginAuthEnabled := os.Getenv("PLUGIN_AUTH_ENABLED") == "true"
	pluginAuthAddress := os.Getenv("PLUGIN_AUTH_ADDRESS")
	clientID := os.Getenv("MIDAZ_CLIENT_ID")
	clientSecret := os.Getenv("MIDAZ_CLIENT_SECRET")

	return auth.AccessManager{
		Enabled:      pluginAuthEnabled,
		Address:      pluginAuthAddress,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
}

// executeCoreSetup executes the core setup phase of the workflow
func executeCoreSetup(ctx context.Context, midazClient *client.Client) (string, string, *models.AccountType, error) {
	// Step 1: Create an organization
	orgID, err := CreateOrganization(ctx, midazClient)
	if err != nil {
		return "", "", nil, err
	}

	// Step 2: Update the organization
	if err := UpdateOrganization(ctx, midazClient, orgID); err != nil {
		return "", "", nil, err
	}

	// Step 3: Create a ledger
	ledgerID, err := CreateLedger(ctx, midazClient, orgID)
	if err != nil {
		return "", "", nil, err
	}

	// Step 4: Create an asset
	if err := CreateAsset(ctx, midazClient, orgID, ledgerID); err != nil {
		return "", "", nil, err
	}

	// Step 4.1-4.4: Handle account type operations
	accountType, err := handleAccountTypeOperations(ctx, midazClient, orgID, ledgerID)
	if err != nil {
		return "", "", nil, err
	}

	return orgID, ledgerID, accountType, nil
}

// handleAccountTypeOperations handles all account type related operations
func handleAccountTypeOperations(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) (*models.AccountType, error) {
	// Step 4.1: Create account type
	accountType, err := CreateAccountType(ctx, midazClient, orgID, ledgerID)
	if err != nil {
		return nil, err
	}

	// Step 4.2: Update account type
	if err := UpdateAccountType(ctx, midazClient, orgID, ledgerID, accountType.ID.String()); err != nil {
		return nil, err
	}

	// Step 4.3: Get account type
	if err := GetAccountType(ctx, midazClient, orgID, ledgerID, accountType.ID.String()); err != nil {
		return nil, err
	}

	// Step 4.4: List account types
	if err := ListAccountTypes(ctx, midazClient, orgID, ledgerID); err != nil {
		return nil, err
	}

	return accountType, nil
}

// executeRoutesSetup executes the routes setup phase of the workflow
func executeRoutesSetup(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, accountType *models.AccountType) (*models.OperationRoute, *models.OperationRoute, *models.TransactionRoute, *models.TransactionRoute) {
	// Step 4.5: Create operation routes
	sourceOperationRoute, destinationOperationRoute := handleOperationRoutes(ctx, midazClient, orgID, ledgerID, accountType)

	// Step 4.6: Create transaction routes
	paymentTransactionRoute, refundTransactionRoute := handleTransactionRoutes(ctx, midazClient, orgID, ledgerID, sourceOperationRoute, destinationOperationRoute)

	return sourceOperationRoute, destinationOperationRoute, paymentTransactionRoute, refundTransactionRoute
}

// handleOperationRoutes handles the operation routes creation and CRUD demonstration
func handleOperationRoutes(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, accountType *models.AccountType) (*models.OperationRoute, *models.OperationRoute) {
	fmt.Printf("🔍 Testing operation routes API availability...\n")

	sourceOperationRoute, destinationOperationRoute, err := CreateOperationRoutes(ctx, midazClient, orgID, ledgerID, accountType)
	if err != nil {
		fmt.Printf("⚠️  Operation routes API not available on server: %v\n", err)
		fmt.Printf("   Note: SDK has full operation routes implementation, but server endpoint not ready\n")
		fmt.Printf("   Note: Continuing with transaction routes only\n")

		return nil, nil
	}

	// Demonstrate Operation Route CRUD operations
	fmt.Printf("🧪 Demonstrating Operation Route CRUD operations...\n")

	if err := demonstrateOperationRouteCRUD(ctx, midazClient, orgID, ledgerID, accountType, sourceOperationRoute, destinationOperationRoute); err != nil {
		fmt.Printf("⚠️  Operation Route CRUD demonstration failed: %v\n", err)
	}

	return sourceOperationRoute, destinationOperationRoute
}

// handleTransactionRoutes handles the transaction routes creation
func handleTransactionRoutes(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, sourceOperationRoute, destinationOperationRoute *models.OperationRoute) (*models.TransactionRoute, *models.TransactionRoute) {
	fmt.Printf("🔍 Testing transaction routes API availability...\n")

	paymentTransactionRoute, refundTransactionRoute, err := CreateTransactionRoutesWithOperationRoutes(ctx, midazClient, orgID, ledgerID, sourceOperationRoute, destinationOperationRoute)
	if err != nil {
		fmt.Printf("⚠️  Transaction routes API not available on server: %v\n", err)
		fmt.Printf("   Note: SDK has full transaction routes implementation, but server endpoint not ready\n")
		// Create mock routes for demonstration
		paymentTransactionRoute, refundTransactionRoute = CreateMockTransactionRoutes(orgID, ledgerID)
	}

	return paymentTransactionRoute, refundTransactionRoute
}

// executeAccountsAndTransactions executes the accounts and transactions phase of the workflow
func executeAccountsAndTransactions(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, accountType *models.AccountType, sourceOperationRoute, destinationOperationRoute *models.OperationRoute, paymentTransactionRoute, refundTransactionRoute *models.TransactionRoute) (*workflowAccounts, error) {
	// Step 5: Create accounts
	customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount, err := CreateAccountsWithType(ctx, midazClient, orgID, ledgerID, accountType.ID.String())
	if err != nil {
		return nil, err
	}

	accounts := &workflowAccounts{
		customerAccount: customerAccount,
		merchantAccount: merchantAccount,
		dummyOneAccount: dummyOneAccount,
		dummyTwoAccount: dummyTwoAccount,
	}

	// Step 6: Execute transactions with enhanced parameters
	if err := ExecuteTransactionsWithRoutes(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, sourceOperationRoute, destinationOperationRoute, paymentTransactionRoute, refundTransactionRoute); err != nil {
		return nil, err
	}

	// Step 6B: Demonstrate transaction helpers
	if err := DemonstrateTransactionHelpers(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount); err != nil {
		return nil, err
	}

	return accounts, nil
}

// executeAdditionalResources executes the additional resources phase of the workflow
func executeAdditionalResources(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) (string, error) {
	// Step 7: Create a portfolio
	portfolioID, err := CreatePortfolio(ctx, midazClient, orgID, ledgerID)
	if err != nil {
		return "", err
	}

	// Step 8: Create segments
	if err := CreateSegments(ctx, midazClient, orgID, ledgerID); err != nil {
		return "", err
	}

	// Step 9: List accounts
	if err := ListAccounts(ctx, midazClient, orgID, ledgerID); err != nil {
		return "", err
	}

	// Step 10: Retrieve organization
	if err := RetrieveOrganization(ctx, midazClient, orgID); err != nil {
		return "", err
	}

	return portfolioID, nil
}

// executeTestingPhase executes the testing phase of the workflow
func executeTestingPhase(ctx context.Context, midazClient *client.Client, orgID, ledgerID, customerAccountID, portfolioID string) error {
	// Step 11: Test Get methods
	if err := TestGetMethods(ctx, midazClient, orgID, ledgerID, customerAccountID, portfolioID); err != nil {
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

	return nil
}

// demonstrateOperationRouteCRUD demonstrates all CRUD operations for Operation Routes
func demonstrateOperationRouteCRUD(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, _ /* accountType */ *models.AccountType, sourceRoute, destinationRoute *models.OperationRoute) error {
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

		_, err := GetOperationRoute(ctx, midazClient, orgID, ledgerID, sourceRoute.ID.String())
		if err != nil {
			return fmt.Errorf("failed to get operation route: %w", err)
		}
	}

	// Step 3: Update operation route (using the destination route)
	if destinationRoute != nil {
		fmt.Println("\n✏️  Step 3: UPDATE Operation Route")

		_, err := UpdateOperationRoute(ctx, midazClient, orgID, ledgerID, destinationRoute.ID.String(),
			"Updated Cash-out Route",
			"Updated route for cash-out operations with enhanced features",
			[]string{"liability", "revenue", "expense"})
		if err != nil {
			return fmt.Errorf("failed to update operation route: %w", err)
		}

		// Verify the update by getting it again
		fmt.Println("\n🔄 Step 3.1: VERIFY Update by Getting Again")

		_, err = GetOperationRoute(ctx, midazClient, orgID, ledgerID, destinationRoute.ID.String())
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
		"demo":    true,
		"purpose": "deletion_test",
	})

	demoRoute, err := midazClient.Entity.OperationRoutes.CreateOperationRoute(ctx, orgID, ledgerID, demoInput)
	if err != nil {
		return fmt.Errorf("failed to create demo operation route: %w", err)
	}

	fmt.Printf("✅ Demo operation route created for deletion: %s\n", demoRoute.Title)
	fmt.Printf("   ID: %s\n", demoRoute.ID.String())

	// Step 5: Delete the demo operation route
	fmt.Println("\n🗑️  Step 5: DELETE Operation Route")

	err = DeleteOperationRoute(ctx, midazClient, orgID, ledgerID, demoRoute.ID.String())
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

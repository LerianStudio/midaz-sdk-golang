package workflows

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang"
	sdkentities "github.com/LerianStudio/midaz-sdk-golang/entities"
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
	pluginAuth := auth.PluginAuth{
		Enabled:      pluginAuthEnabled,
		Address:      pluginAuthAddress,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	cfg, err := config.NewConfig(
		config.WithPluginAuth(pluginAuth),
	)
	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
	}

	client, err := client.New(
		client.WithConfig(cfg),
		client.UseEntityAPI(),                      // Enable the Entity API
		client.WithObservability(true, true, true), // Enable observability
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Step 1: Create an organization
	orgID, err := CreateOrganization(ctx, client)
	if err != nil {
		return err
	}

	// Step 2: Update the organization
	if err := UpdateOrganization(ctx, client, orgID); err != nil {
		return err
	}

	// Step 3: Create a ledger
	ledgerID, err := CreateLedger(ctx, client, orgID)
	if err != nil {
		return err
	}

	// Step 4: Create an asset
	if err := CreateAsset(ctx, client, orgID, ledgerID); err != nil {
		return err
	}

	// Step 5: Create accounts
	customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount, err := CreateAccounts(ctx, client, orgID, ledgerID)
	if err != nil {
		return err
	}

	// Step 6: Execute transactions
	if err := ExecuteTransactions(ctx, client, orgID, ledgerID, customerAccount, merchantAccount); err != nil {
		return err
	}

	// Step 6B: Demonstrate transaction helpers
	if err := DemonstrateTransactionHelpers(ctx, client, orgID, ledgerID, customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount); err != nil {
		return err
	}

	// Step 7: Create a portfolio
	portfolioID, err := CreatePortfolio(ctx, client, orgID, ledgerID)
	if err != nil {
		return err
	}

	// Step 8: Create segments
	if err := CreateSegments(ctx, client, orgID, ledgerID); err != nil {
		return err
	}

	// Step 9: List accounts
	if err := ListAccounts(ctx, client, orgID, ledgerID); err != nil {
		return err
	}

	// Step 10: Retrieve organization
	if err := RetrieveOrganization(ctx, client, orgID); err != nil {
		return err
	}

	// Step 11: Test Get methods
	if err := TestGetMethods(ctx, client, orgID, ledgerID, customerAccount.ID, portfolioID); err != nil {
		return err
	}

	// Step 12: Test List methods
	if err := TestListMethods(ctx, client, orgID, ledgerID); err != nil {
		return err
	}

	// Step 13: Test Delete methods
	if err := TestDeleteMethods(ctx, client, orgID, ledgerID); err != nil {
		return err
	}

	fmt.Println("\n\n✅ COMPLETE WORKFLOW FINISHED SUCCESSFULLY")
	fmt.Println(strings.Repeat("=", 50))
	return nil
}

// Function declarations to satisfy the compiler
// These are placeholder functions that will be implemented in separate files
var (
	TestGetMethods    func(ctx context.Context, client *client.Client, orgID, ledgerID, accountID string, portfolioID string) error
	TestListMethods   func(ctx context.Context, client *client.Client, orgID, ledgerID string) error
	TestDeleteMethods func(ctx context.Context, client *client.Client, orgID, ledgerID string) error
)

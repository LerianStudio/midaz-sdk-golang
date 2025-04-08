package workflows

import (
	"context"
	"fmt"
	"strings"

	sdkentities "github.com/LerianStudio/midaz-sdk-golang/entities"
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

	// Step 1: Create an organization
	orgID, err := CreateOrganization(ctx, entity)
	if err != nil {
		return err
	}

	// Step 2: Update the organization
	if err := UpdateOrganization(ctx, entity, orgID); err != nil {
		return err
	}

	// Step 3: Create a ledger
	ledgerID, err := CreateLedger(ctx, entity, orgID)
	if err != nil {
		return err
	}

	// Step 4: Create an asset
	if err := CreateAsset(ctx, entity, orgID, ledgerID); err != nil {
		return err
	}

	// Step 5: Create accounts
	customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount, err := CreateAccounts(ctx, entity, orgID, ledgerID)
	if err != nil {
		return err
	}

	// Step 6: Execute transactions
	if err := ExecuteTransactions(ctx, entity, orgID, ledgerID, customerAccount, merchantAccount); err != nil {
		return err
	}

	// Step 6B: Demonstrate transaction helpers
	if err := DemonstrateTransactionHelpers(ctx, entity, orgID, ledgerID, customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount); err != nil {
		return err
	}

	// Step 7: Create a portfolio
	portfolioID, err := CreatePortfolio(ctx, entity, orgID, ledgerID)
	if err != nil {
		return err
	}

	// Step 8: Create segments
	if err := CreateSegments(ctx, entity, orgID, ledgerID); err != nil {
		return err
	}

	// Step 9: List accounts
	if err := ListAccounts(ctx, entity, orgID, ledgerID); err != nil {
		return err
	}

	// Step 10: Retrieve organization
	if err := RetrieveOrganization(ctx, entity, orgID); err != nil {
		return err
	}

	// Step 11: Test Get methods
	if err := TestGetMethods(ctx, entity, orgID, ledgerID, customerAccount.ID, portfolioID); err != nil {
		return err
	}

	// Step 12: Test List methods
	if err := TestListMethods(ctx, entity, orgID, ledgerID); err != nil {
		return err
	}

	// Step 13: Test Delete methods
	if err := TestDeleteMethods(ctx, entity, orgID, ledgerID); err != nil {
		return err
	}

	fmt.Println("\n\n✅ COMPLETE WORKFLOW FINISHED SUCCESSFULLY")
	fmt.Println(strings.Repeat("=", 50))
	return nil
}

// Function declarations to satisfy the compiler
// These are placeholder functions that will be implemented in separate files
var (
	TestGetMethods    func(ctx context.Context, entity *sdkentities.Entity, orgID, ledgerID, accountID string, portfolioID string) error
	TestListMethods   func(ctx context.Context, entity *sdkentities.Entity, orgID, ledgerID string) error
	TestDeleteMethods func(ctx context.Context, entity *sdkentities.Entity, orgID, ledgerID string) error
)

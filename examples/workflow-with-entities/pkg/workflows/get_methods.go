package workflows

import (
	"context"
	"fmt"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
)

// TestGetMethods tests various Get methods of the Midaz SDK
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - accountID: The ID of the account
//   - portfolioID: The ID of the portfolio
//
// Returns:
//   - error: Any error encountered during the operation
func init() {
	TestGetMethods = testGetMethods
}

func testGetMethods(ctx context.Context, midazClient *client.Client, orgID, ledgerID, accountID, portfolioID string) error {
	fmt.Println("\n\n🔍 STEP 11: TESTING GET METHODS")
	fmt.Println(strings.Repeat("=", 50))

	// Test GetOrganization
	fmt.Println("\nTesting GetOrganization...")
	org, err := midazClient.Entity.Organizations.GetOrganization(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}
	fmt.Printf("✅ Got organization: %s (ID: %s)\n", org.LegalName, org.ID)

	// Test GetLedger
	fmt.Println("\nTesting GetLedger...")
	ledger, err := midazClient.Entity.Ledgers.GetLedger(ctx, orgID, ledgerID)
	if err != nil {
		return fmt.Errorf("failed to get ledger: %w", err)
	}
	fmt.Printf("✅ Got ledger: %s (ID: %s)\n", ledger.Name, ledger.ID)

	// Test GetAccount
	fmt.Println("\nTesting GetAccount...")
	account, err := midazClient.Entity.Accounts.GetAccount(ctx, orgID, ledgerID, accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}
	fmt.Printf("✅ Got account: %s (ID: %s, Type: %s)\n", account.Name, account.ID, account.Type)

	// Test GetPortfolio
	fmt.Println("\nTesting GetPortfolio...")
	portfolio, err := midazClient.Entity.Portfolios.GetPortfolio(ctx, orgID, ledgerID, portfolioID)
	if err != nil {
		return fmt.Errorf("failed to get portfolio: %w", err)
	}
	fmt.Printf("✅ Got portfolio: %s (ID: %s)\n", portfolio.Name, portfolio.ID)

	fmt.Println("\n✅ All Get methods tested successfully")
	return nil
}

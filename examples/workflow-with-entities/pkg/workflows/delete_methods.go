package workflows

import (
	"context"
	"fmt"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang"
)

// TestDeleteMethods tests various Delete methods of the Midaz SDK
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//
// Returns:
//   - error: Any error encountered during the operation
func init() {
	TestDeleteMethods = testDeleteMethods
}

func testDeleteMethods(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("\n\n🗑️ STEP 13: TESTING DELETE METHODS")
	fmt.Println(strings.Repeat("=", 50))

	// Get all segments to delete
	fmt.Println("\nDeleting all segments...")
	segmentsResponse, err := midazClient.Entity.Segments.ListSegments(ctx, orgID, ledgerID, nil)
	if err != nil {
		return fmt.Errorf("failed to list segments: %w", err)
	}

	for _, segment := range segmentsResponse.Items {
		fmt.Printf("   Deleting segment: %s (ID: %s)...\n", segment.Name, segment.ID)
		err := midazClient.Entity.Segments.DeleteSegment(ctx, orgID, ledgerID, segment.ID)
		if err != nil {
			return fmt.Errorf("failed to delete segment %s: %w", segment.ID, err)
		}
		fmt.Printf("   ✅ Segment deleted: %s\n", segment.Name)
	}

	// Get all portfolios to delete
	fmt.Println("\nDeleting all portfolios...")
	portfoliosResponse, err := midazClient.Entity.Portfolios.ListPortfolios(ctx, orgID, ledgerID, nil)
	if err != nil {
		return fmt.Errorf("failed to list portfolios: %w", err)
	}

	for _, portfolio := range portfoliosResponse.Items {
		fmt.Printf("   Deleting portfolio: %s (ID: %s)...\n", portfolio.Name, portfolio.ID)
		err := midazClient.Entity.Portfolios.DeletePortfolio(ctx, orgID, ledgerID, portfolio.ID)
		if err != nil {
			return fmt.Errorf("failed to delete portfolio %s: %w", portfolio.ID, err)
		}
		fmt.Printf("   ✅ Portfolio deleted: %s\n", portfolio.Name)
	}

	// Get all accounts to delete
	fmt.Println("\nDeleting all accounts...")
	accountsResponse, err := midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, nil)
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	for _, account := range accountsResponse.Items {
		// Skip external accounts as they cannot be deleted
		if account.Type == "external" {
			fmt.Printf("   Skipping external account: %s (ID: %s) - External accounts cannot be deleted\n", account.Name, account.ID)
			continue
		}

		fmt.Printf("   Deleting account: %s (ID: %s)...\n", account.Name, account.ID)
		err := midazClient.Entity.Accounts.DeleteAccount(ctx, orgID, ledgerID, account.ID)
		if err != nil {
			return fmt.Errorf("failed to delete account %s: %w", account.ID, err)
		}
		fmt.Printf("   ✅ Account deleted: %s\n", account.Name)
	}

	// Delete the ledger
	fmt.Println("\nDeleting ledger...")
	err = midazClient.Entity.Ledgers.DeleteLedger(ctx, orgID, ledgerID)
	if err != nil {
		return fmt.Errorf("failed to delete ledger %s: %w", ledgerID, err)
	}
	fmt.Printf("   ✅ Ledger deleted (ID: %s)\n", ledgerID)

	// Delete the organization
	fmt.Println("\nDeleting organization...")
	err = midazClient.Entity.Organizations.DeleteOrganization(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete organization %s: %w", orgID, err)
	}
	fmt.Printf("   ✅ Organization deleted (ID: %s)\n", orgID)

	fmt.Println("\n✅ All resources deleted successfully")
	return nil
}

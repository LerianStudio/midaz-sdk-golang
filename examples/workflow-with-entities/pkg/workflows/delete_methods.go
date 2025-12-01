package workflows

import (
	"context"
	"fmt"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
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
	fmt.Println("\n\nSTEP 13: TESTING DELETE METHODS")
	fmt.Println(strings.Repeat("=", 50))

	if err := deleteAllSegments(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	if err := deleteAllPortfolios(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	if err := deleteAllAccounts(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	if err := deleteLedgerAndOrg(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	fmt.Println("\nAll resources deleted successfully")

	return nil
}

func deleteAllSegments(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("\nDeleting all segments...")

	segmentsResponse, err := midazClient.Entity.Segments.ListSegments(ctx, orgID, ledgerID, nil)
	if err != nil {
		return fmt.Errorf("failed to list segments: %w", err)
	}

	for _, segment := range segmentsResponse.Items {
		fmt.Printf("   Deleting segment: %s (ID: %s)...\n", segment.Name, segment.ID)

		if err := midazClient.Entity.Segments.DeleteSegment(ctx, orgID, ledgerID, segment.ID); err != nil {
			return fmt.Errorf("failed to delete segment %s: %w", segment.ID, err)
		}

		fmt.Printf("   Segment deleted: %s\n", segment.Name)
	}

	return nil
}

func deleteAllPortfolios(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("\nDeleting all portfolios...")

	portfoliosResponse, err := midazClient.Entity.Portfolios.ListPortfolios(ctx, orgID, ledgerID, nil)
	if err != nil {
		return fmt.Errorf("failed to list portfolios: %w", err)
	}

	for _, portfolio := range portfoliosResponse.Items {
		fmt.Printf("   Deleting portfolio: %s (ID: %s)...\n", portfolio.Name, portfolio.ID)

		if err := midazClient.Entity.Portfolios.DeletePortfolio(ctx, orgID, ledgerID, portfolio.ID); err != nil {
			return fmt.Errorf("failed to delete portfolio %s: %w", portfolio.ID, err)
		}

		fmt.Printf("   Portfolio deleted: %s\n", portfolio.Name)
	}

	return nil
}

func deleteAllAccounts(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("\nDeleting all accounts...")

	accountsResponse, err := midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, nil)
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	for _, account := range accountsResponse.Items {
		if account.Type == "external" {
			fmt.Printf("   Skipping external account: %s (ID: %s) - External accounts cannot be deleted\n", account.Name, account.ID)
			continue
		}

		fmt.Printf("   Deleting account: %s (ID: %s)...\n", account.Name, account.ID)

		if err := midazClient.Entity.Accounts.DeleteAccount(ctx, orgID, ledgerID, account.ID); err != nil {
			return fmt.Errorf("failed to delete account %s: %w", account.ID, err)
		}

		fmt.Printf("   Account deleted: %s\n", account.Name)
	}

	return nil
}

func deleteLedgerAndOrg(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("\nDeleting ledger...")

	if err := midazClient.Entity.Ledgers.DeleteLedger(ctx, orgID, ledgerID); err != nil {
		fmt.Printf("   ⚠️  Could not delete ledger (ID: %s): %v\n", ledgerID, err)
		fmt.Println("   Note: Ledger deletion may be restricted in staging/production environments")
	} else {
		fmt.Printf("   Ledger deleted (ID: %s)\n", ledgerID)
	}

	fmt.Println("\nDeleting organization...")

	if err := midazClient.Entity.Organizations.DeleteOrganization(ctx, orgID); err != nil {
		fmt.Printf("   ⚠️  Could not delete organization (ID: %s): %v\n", orgID, err)
		fmt.Println("   Note: Organization deletion may be restricted in staging/production environments")
	} else {
		fmt.Printf("   Organization deleted (ID: %s)\n", orgID)
	}

	return nil
}

package workflows

import (
	"context"
	"fmt"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
)

// CreatePortfolio creates a portfolio
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//
// Returns:
//   - string: The ID of the created portfolio
//   - error: Any error encountered during the operation
func CreatePortfolio(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) (string, error) {
	fmt.Println("\n\n📁 STEP 6: PORTFOLIO CREATION")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Println("\nCreating portfolio...")

	portfolio, err := midazClient.Entity.Portfolios.CreatePortfolio(
		ctx, orgID, ledgerID,
		models.NewCreatePortfolioInput(ledgerID, "Main Portfolio"),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create portfolio: %w", err)
	}

	if portfolio.ID == "" {
		return "", fmt.Errorf("portfolio created but no ID was returned from the API")
	}

	fmt.Printf("✅ Portfolio created: %s\n", portfolio.Name)
	fmt.Printf("   ID: %s\n", portfolio.ID)
	fmt.Printf("   Created: %s\n", portfolio.CreatedAt.Format("2006-01-02 15:04:05"))

	return portfolio.ID, nil
}

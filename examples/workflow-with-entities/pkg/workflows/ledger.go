package workflows

import (
	"context"
	"fmt"
	"strings"

	sdkentities "github.com/LerianStudio/midaz-sdk-golang/entities"
	"github.com/LerianStudio/midaz-sdk-golang/models"
)

// CreateLedger creates a ledger in the organization
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//
// Returns:
//   - string: The ID of the created ledger
//   - error: Any error encountered during the operation
func CreateLedger(ctx context.Context, entity *sdkentities.Entity, orgID string) (string, error) {
	fmt.Println("\n\n📒 STEP 2: LEDGER CREATION")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Println("\nCreating ledger...")

	// Create a ledger with the organization ID
	ledger, err := entity.Ledgers.CreateLedger(ctx, orgID, &models.CreateLedgerInput{
		Name: "Main Ledger",
		Metadata: map[string]any{
			"purpose": "example",
			"type":    "main",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create ledger: %w", err)
	}

	if ledger.ID == "" {
		return "", fmt.Errorf("ledger created but no ID was returned from the API")
	}

	fmt.Printf("✅ Ledger created: %s\n", ledger.Name)
	fmt.Printf("   ID: %s\n", ledger.ID)
	fmt.Printf("   Created: %s\n", ledger.CreatedAt.Format("2006-01-02 15:04:05"))

	return ledger.ID, nil
}

package workflows

import (
	"context"
	"fmt"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang/v3"
	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
)

// CreateAsset creates a new USD asset
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//
// Returns:
//   - error: Any error encountered during the operation
func CreateAsset(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("\n\n🏦 STEP 3: ASSET CREATION")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Println("Creating USD asset...")

	reqCtx := sdkctx.WithIdempotencyKey(ctx, fmt.Sprintf("asset-%s-%s-USD", orgID, ledgerID))
	usdAsset, err := midazClient.Assets.CreateAsset(
		reqCtx, orgID, ledgerID,
		models.NewCreateAssetInputWithType("US Dollar", "USD", "currency").
			WithMetadata(map[string]any{"purpose": "main"}),
	)
	if err != nil {
		return fmt.Errorf("failed to create USD asset: %w", err)
	}

	fmt.Printf("✅ USD asset created: %s\n", usdAsset.Name)
	fmt.Printf("   ID: %s\n", usdAsset.ID)
	fmt.Printf("   Code: %s\n", usdAsset.Code)
	fmt.Printf("   Created: %s\n", usdAsset.CreatedAt.Format("2006-01-02 15:04:05"))

	return nil
}

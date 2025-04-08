package workflows

import (
	"context"
	"fmt"
	"strings"

	sdkentities "github.com/LerianStudio/midaz-sdk-golang/entities"
	"github.com/LerianStudio/midaz-sdk-golang/models"
)

// CreateAccounts creates customer and merchant accounts and returns their models
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//
// Returns:
//   - *models.Account: The customer account model
//   - *models.Account: The merchant account model
//   - *models.Account: The dummy 1 account model
//   - *models.Account: The dummy 2 account model
//   - error: Any error encountered during the operation
func CreateAccounts(ctx context.Context, entity *sdkentities.Entity, orgID, ledgerID string) (*models.Account, *models.Account, *models.Account, *models.Account, error) {
	fmt.Println("\n\n📂 STEP 4: ACCOUNT CREATION")
	fmt.Println(strings.Repeat("=", 50))

	// Create customer account
	fmt.Println("Creating customer account...")

	customerAccount, err := entity.Accounts.CreateAccount(
		ctx, orgID, ledgerID, &models.CreateAccountInput{
			Name:      "Customer Account",
			Type:      "deposit",
			AssetCode: "USD",
			Metadata:  map[string]any{"purpose": "main"},
		},
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create customer account: %w", err)
	}

	if customerAccount.ID == "" {
		return nil, nil, nil, nil, fmt.Errorf("customer account created but no ID was returned from the API")
	}

	fmt.Printf("✅ Customer account created: %s\n", customerAccount.Name)
	fmt.Printf("   ID: %s\n", customerAccount.ID)
	fmt.Printf("   Type: %s\n", customerAccount.Type)
	fmt.Printf("   Asset: %s\n", customerAccount.AssetCode)
	fmt.Printf("   Created: %s\n", customerAccount.CreatedAt.Format("2006-01-02 15:04:05"))

	fmt.Println()

	// Create merchant account
	fmt.Println("Creating merchant account...")

	merchantAccount, err := entity.Accounts.CreateAccount(
		ctx, orgID, ledgerID, &models.CreateAccountInput{
			Name:      "Merchant Account",
			Type:      "marketplace",
			AssetCode: "USD",
			Metadata:  map[string]any{"purpose": "main"},
		},
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create merchant account: %w", err)
	}

	if merchantAccount.ID == "" {
		return nil, nil, nil, nil, fmt.Errorf("merchant account created but no ID was returned from the API")
	}

	fmt.Printf("✅ Merchant account created: %s\n", merchantAccount.Name)
	fmt.Printf("   ID: %s\n", merchantAccount.ID)
	fmt.Printf("   Type: %s\n", merchantAccount.Type)
	fmt.Printf("   Asset: %s\n", merchantAccount.AssetCode)
	fmt.Printf("   Created: %s\n", merchantAccount.CreatedAt.Format("2006-01-02 15:04:05"))

	// Create Dummy 1 account
	fmt.Println("Creating dummy 1 account...")

	dummyOneAccount, err := entity.Accounts.CreateAccount(
		ctx, orgID, ledgerID, &models.CreateAccountInput{
			Name:      "Dummy 1 Account",
			Type:      "deposit",
			AssetCode: "USD",
			Metadata:  map[string]any{"purpose": "main"},
		},
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create merchant account: %w", err)
	}

	if dummyOneAccount.ID == "" {
		return nil, nil, nil, nil, fmt.Errorf("dummy account created but no ID was returned from the API")
	}

	fmt.Printf("✅ Dummy account created: %s\n", dummyOneAccount.Name)
	fmt.Printf("   ID: %s\n", dummyOneAccount.ID)
	fmt.Printf("   Type: %s\n", dummyOneAccount.Type)
	fmt.Printf("   Asset: %s\n", dummyOneAccount.AssetCode)
	fmt.Printf("   Created: %s\n", dummyOneAccount.CreatedAt.Format("2006-01-02 15:04:05"))

	// Create Dummy 2 account
	fmt.Println("Creating dummy 2 account...")

	dummyTwoAccount, err := entity.Accounts.CreateAccount(
		ctx, orgID, ledgerID, &models.CreateAccountInput{
			Name:      "Dummy 2 Account",
			Type:      "deposit",
			AssetCode: "USD",
			Metadata:  map[string]any{"purpose": "main"},
		},
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create merchant account: %w", err)
	}

	if dummyTwoAccount.ID == "" {
		return nil, nil, nil, nil, fmt.Errorf("dummy 2 account created but no ID was returned from the API")
	}

	fmt.Printf("✅ Dummy 2 account created: %s\n", dummyTwoAccount.Name)
	fmt.Printf("   ID: %s\n", dummyTwoAccount.ID)
	fmt.Printf("   Type: %s\n", dummyTwoAccount.Type)
	fmt.Printf("   Asset: %s\n", dummyTwoAccount.AssetCode)
	fmt.Printf("   Created: %s\n", dummyTwoAccount.CreatedAt.Format("2006-01-02 15:04:05"))

	return customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount, nil
}

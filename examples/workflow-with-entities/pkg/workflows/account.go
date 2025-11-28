package workflows

import (
	"context"
	"errors"
	"fmt"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
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
//
//nolint:funlen // Demo function - length acceptable for example code showing complete account creation workflow
func CreateAccounts(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) (customerAccount *models.Account, merchantAccount *models.Account, dummyOneAccount *models.Account, dummyTwoAccount *models.Account, err error) {
	fmt.Println("\n\n📂 STEP 4: ACCOUNT CREATION")
	fmt.Println(strings.Repeat("=", 50))

	// Create customer account
	fmt.Println("Creating customer account...")

	customerAccount, err = midazClient.Entity.Accounts.CreateAccount(
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
		return nil, nil, nil, nil, errors.New("customer account created but no ID was returned from the API")
	}

	fmt.Printf("✅ Customer account created: %s\n", customerAccount.Name)
	fmt.Printf("   ID: %s\n", customerAccount.ID)
	fmt.Printf("   Type: %s\n", customerAccount.Type)
	fmt.Printf("   Asset: %s\n", customerAccount.AssetCode)
	fmt.Printf("   Created: %s\n", customerAccount.CreatedAt.Format("2006-01-02 15:04:05"))

	fmt.Println()

	// Create merchant account
	fmt.Println("Creating merchant account...")

	merchantAccount, err = midazClient.Entity.Accounts.CreateAccount(
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
		return nil, nil, nil, nil, errors.New("merchant account created but no ID was returned from the API")
	}

	fmt.Printf("✅ Merchant account created: %s\n", merchantAccount.Name)
	fmt.Printf("   ID: %s\n", merchantAccount.ID)
	fmt.Printf("   Type: %s\n", merchantAccount.Type)
	fmt.Printf("   Asset: %s\n", merchantAccount.AssetCode)
	fmt.Printf("   Created: %s\n", merchantAccount.CreatedAt.Format("2006-01-02 15:04:05"))

	// Create Dummy 1 account
	fmt.Println("Creating dummy 1 account...")

	dummyOneAccount, err = midazClient.Entity.Accounts.CreateAccount(
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
		return nil, nil, nil, nil, errors.New("dummy account created but no ID was returned from the API")
	}

	fmt.Printf("✅ Dummy account created: %s\n", dummyOneAccount.Name)
	fmt.Printf("   ID: %s\n", dummyOneAccount.ID)
	fmt.Printf("   Type: %s\n", dummyOneAccount.Type)
	fmt.Printf("   Asset: %s\n", dummyOneAccount.AssetCode)
	fmt.Printf("   Created: %s\n", dummyOneAccount.CreatedAt.Format("2006-01-02 15:04:05"))

	// Create Dummy 2 account
	fmt.Println("Creating dummy 2 account...")

	dummyTwoAccount, err = midazClient.Entity.Accounts.CreateAccount(
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
		return nil, nil, nil, nil, errors.New("dummy 2 account created but no ID was returned from the API")
	}

	fmt.Printf("✅ Dummy 2 account created: %s\n", dummyTwoAccount.Name)
	fmt.Printf("   ID: %s\n", dummyTwoAccount.ID)
	fmt.Printf("   Type: %s\n", dummyTwoAccount.Type)
	fmt.Printf("   Asset: %s\n", dummyTwoAccount.AssetCode)
	fmt.Printf("   Created: %s\n", dummyTwoAccount.CreatedAt.Format("2006-01-02 15:04:05"))

	return customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount, nil
}

// CreateAccountsWithType creates customer and merchant accounts with specified account type and returns their models
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - client: The initialized Midaz SDK client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - accountTypeID: The ID of the account type to associate with accounts
//
// Returns:
//   - *models.Account: The customer account model
//   - *models.Account: The merchant account model
//   - *models.Account: The dummy 1 account model
//   - *models.Account: The dummy 2 account model
//   - error: Any error encountered during the operation
//
//nolint:funlen // Demo function - length acceptable for example code showing complete account creation with types
func CreateAccountsWithType(ctx context.Context, midazClient *client.Client, orgID, ledgerID, accountTypeID string) (customerAccount *models.Account, merchantAccount *models.Account, dummyOneAccount *models.Account, dummyTwoAccount *models.Account, err error) {
	fmt.Println("\n\n📂 STEP 5: ACCOUNT CREATION WITH ACCOUNT TYPE")
	fmt.Println(strings.Repeat("=", 50))

	// Create customer account with account type
	fmt.Println("Creating customer account with account type...")

	customerAccount, err = midazClient.Entity.Accounts.CreateAccount(
		ctx, orgID, ledgerID, &models.CreateAccountInput{
			Name:      "Customer Account",
			Type:      "liability", // Change to liability to match destination operation route
			AssetCode: "USD",
			Metadata: map[string]any{
				"purpose":         "main",
				"account_type_id": accountTypeID, // Link to account type via metadata
				"category":        "business",
			},
		},
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create customer account: %w", err)
	}

	if customerAccount.ID == "" {
		return nil, nil, nil, nil, errors.New("customer account created but no ID was returned from the API")
	}

	fmt.Printf("✅ Customer account created: %s\n", customerAccount.Name)
	fmt.Printf("   ID: %s\n", customerAccount.ID)
	fmt.Printf("   Type: %s\n", customerAccount.Type)
	fmt.Printf("   Asset: %s\n", customerAccount.AssetCode)

	if accountTypeIDMeta, ok := customerAccount.Metadata["account_type_id"]; ok {
		fmt.Printf("   Account Type ID: %s\n", accountTypeIDMeta)
	}

	fmt.Printf("   Created: %s\n", customerAccount.CreatedAt.Format("2006-01-02 15:04:05"))

	fmt.Println()

	// Create merchant account with account type
	fmt.Println("Creating merchant account with account type...")

	merchantAccount, err = midazClient.Entity.Accounts.CreateAccount(
		ctx, orgID, ledgerID, &models.CreateAccountInput{
			Name:      "Merchant Account",
			Type:      "revenue", // Change to revenue to match destination operation route
			AssetCode: "USD",
			Metadata: map[string]any{
				"purpose":         "main",
				"account_type_id": accountTypeID, // Link to account type via metadata
				"category":        "business",
			},
		},
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create merchant account: %w", err)
	}

	if merchantAccount.ID == "" {
		return nil, nil, nil, nil, errors.New("merchant account created but no ID was returned from the API")
	}

	fmt.Printf("✅ Merchant account created: %s\n", merchantAccount.Name)
	fmt.Printf("   ID: %s\n", merchantAccount.ID)
	fmt.Printf("   Type: %s\n", merchantAccount.Type)
	fmt.Printf("   Asset: %s\n", merchantAccount.AssetCode)

	if accountTypeIDMeta, ok := merchantAccount.Metadata["account_type_id"]; ok {
		fmt.Printf("   Account Type ID: %s\n", accountTypeIDMeta)
	}

	fmt.Printf("   Created: %s\n", merchantAccount.CreatedAt.Format("2006-01-02 15:04:05"))

	// Create Dummy 1 account with account type
	fmt.Println("Creating dummy 1 account with account type...")

	dummyOneAccount, err = midazClient.Entity.Accounts.CreateAccount(
		ctx, orgID, ledgerID, &models.CreateAccountInput{
			Name:      "Dummy 1 Account",
			Type:      "deposit",
			AssetCode: "USD",
			Metadata: map[string]any{
				"purpose":         "main",
				"account_type_id": accountTypeID, // Link to account type via metadata
				"category":        "business",
			},
		},
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create dummy account: %w", err)
	}

	if dummyOneAccount.ID == "" {
		return nil, nil, nil, nil, errors.New("dummy account created but no ID was returned from the API")
	}

	fmt.Printf("✅ Dummy account created: %s\n", dummyOneAccount.Name)
	fmt.Printf("   ID: %s\n", dummyOneAccount.ID)
	fmt.Printf("   Type: %s\n", dummyOneAccount.Type)
	fmt.Printf("   Asset: %s\n", dummyOneAccount.AssetCode)

	if accountTypeIDMeta, ok := dummyOneAccount.Metadata["account_type_id"]; ok {
		fmt.Printf("   Account Type ID: %s\n", accountTypeIDMeta)
	}

	fmt.Printf("   Created: %s\n", dummyOneAccount.CreatedAt.Format("2006-01-02 15:04:05"))

	// Create Dummy 2 account with account type
	fmt.Println("Creating dummy 2 account with account type...")

	dummyTwoAccount, err = midazClient.Entity.Accounts.CreateAccount(
		ctx, orgID, ledgerID, &models.CreateAccountInput{
			Name:      "Dummy 2 Account",
			Type:      "deposit",
			AssetCode: "USD",
			Metadata: map[string]any{
				"purpose":         "main",
				"account_type_id": accountTypeID, // Link to account type via metadata
				"category":        "business",
			},
		},
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create dummy 2 account: %w", err)
	}

	if dummyTwoAccount.ID == "" {
		return nil, nil, nil, nil, errors.New("dummy 2 account created but no ID was returned from the API")
	}

	fmt.Printf("✅ Dummy 2 account created: %s\n", dummyTwoAccount.Name)
	fmt.Printf("   ID: %s\n", dummyTwoAccount.ID)
	fmt.Printf("   Type: %s\n", dummyTwoAccount.Type)
	fmt.Printf("   Asset: %s\n", dummyTwoAccount.AssetCode)

	if accountTypeIDMeta, ok := dummyTwoAccount.Metadata["account_type_id"]; ok {
		fmt.Printf("   Account Type ID: %s\n", accountTypeIDMeta)
	}

	fmt.Printf("   Created: %s\n", dummyTwoAccount.CreatedAt.Format("2006-01-02 15:04:05"))

	return customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount, nil
}

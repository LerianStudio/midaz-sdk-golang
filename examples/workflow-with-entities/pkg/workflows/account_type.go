package workflows

import (
	"context"
	"fmt"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
)

// CreateAccountType creates a new account type in the specified organization and ledger
//
// Parameters:
//   - ctx: The context for the operation
//   - client: The initialized Midaz SDK client
//   - orgID: The organization ID where the account type will be created
//   - ledgerID: The ledger ID where the account type will be created
//
// Returns:
//   - *models.AccountType: The created account type object
//   - error: Any error encountered during the operation
func CreateAccountType(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) (*models.AccountType, error) {
	fmt.Println("\n📋 Creating Account Type...")

	// Create account type input using your exact specification
	input := models.NewCreateAccountTypeInput("Cash Account", "CASH").
		WithDescription("Account type for liquid assets held in cash or cash equivalents.").
		WithMetadata(map[string]any{
			"customField": "customValue",
		})

	// Create the account type
	accountType, err := midazClient.Entity.AccountTypes.CreateAccountType(ctx, orgID, ledgerID, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create account type: %w", err)
	}

	fmt.Printf("   ✅ Account type created successfully: %s\n", accountType.ID)
	fmt.Printf("      - Name: %s\n", accountType.Name)
	if accountType.Description != "" {
		fmt.Printf("      - Description: %s\n", accountType.Description)
	}
	fmt.Printf("      - Organization ID: %s\n", accountType.OrganizationID)
	fmt.Printf("      - Ledger ID: %s\n", accountType.LedgerID)
	fmt.Printf("      - KeyValue: %s\n", accountType.KeyValue)

	return accountType, nil
}

// UpdateAccountType updates an existing account type
//
// Parameters:
//   - ctx: The context for the operation
//   - client: The initialized Midaz SDK client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - accountTypeID: The account type ID to update
//
// Returns:
//   - error: Any error encountered during the operation
func UpdateAccountType(ctx context.Context, midazClient *client.Client, orgID, ledgerID, accountTypeID string) error {
	fmt.Println("\n📝 Updating Account Type...")

	// Create account type update input using the builder pattern
	input := models.NewUpdateAccountTypeInput().
		WithName("Premium Business Account - Updated").
		WithDescription("Updated premium business account type with new enhanced features").
		WithMetadata(map[string]any{
			"category":    "business",
			"tier":        "premium",
			"max_balance": 2000000, // Increased limit
			"updated":     true,
		})

	// Update the account type
	updatedAccountType, err := midazClient.Entity.AccountTypes.UpdateAccountType(ctx, orgID, ledgerID, accountTypeID, input)
	if err != nil {
		return fmt.Errorf("failed to update account type: %w", err)
	}

	fmt.Printf("   ✅ Account type updated successfully: %s\n", updatedAccountType.ID)
	fmt.Printf("      - Name: %s\n", updatedAccountType.Name)
	if updatedAccountType.Description != "" {
		fmt.Printf("      - Description: %s\n", updatedAccountType.Description)
	}
	fmt.Printf("      - Updated At: %s\n", updatedAccountType.UpdatedAt.Format("2006-01-02 15:04:05"))

	return nil
}

// GetAccountType retrieves a specific account type by ID
//
// Parameters:
//   - ctx: The context for the operation
//   - client: The initialized Midaz SDK client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - accountTypeID: The account type ID to retrieve
//
// Returns:
//   - error: Any error encountered during the operation
func GetAccountType(ctx context.Context, midazClient *client.Client, orgID, ledgerID, accountTypeID string) error {
	fmt.Println("\n🔍 Retrieving Account Type...")

	// Get the account type
	accountType, err := midazClient.Entity.AccountTypes.GetAccountType(ctx, orgID, ledgerID, accountTypeID)
	if err != nil {
		return fmt.Errorf("failed to get account type: %w", err)
	}

	fmt.Printf("   ✅ Account type retrieved successfully: %s\n", accountType.ID)
	fmt.Printf("      - Name: %s\n", accountType.Name)
	if accountType.Description != "" {
		fmt.Printf("      - Description: %s\n", accountType.Description)
	}
	fmt.Printf("      - Organization ID: %s\n", accountType.OrganizationID)
	fmt.Printf("      - Ledger ID: %s\n", accountType.LedgerID)
	fmt.Printf("      - Created At: %s\n", accountType.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("      - Updated At: %s\n", accountType.UpdatedAt.Format("2006-01-02 15:04:05"))

	return nil
}

// ListAccountTypes lists all account types in the specified organization and ledger
//
// Parameters:
//   - ctx: The context for the operation
//   - client: The initialized Midaz SDK client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//
// Returns:
//   - error: Any error encountered during the operation
func ListAccountTypes(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("\n📄 Listing Account Types...")

	// List account types with pagination options
	opts := &models.ListOptions{
		Page:  1,
		Limit: 10,
	}
	accountTypes, err := midazClient.Entity.AccountTypes.ListAccountTypes(ctx, orgID, ledgerID, opts)
	if err != nil {
		return fmt.Errorf("failed to list account types: %w", err)
	}

	fmt.Printf("   ✅ Found %d account types:\n", len(accountTypes.Items))
	for i, accountType := range accountTypes.Items {
		fmt.Printf("      %d. %s (ID: %s)\n", i+1, accountType.Name, accountType.ID)
		if accountType.Description != "" {
			fmt.Printf("         Description: %s\n", accountType.Description)
		} else {
			fmt.Printf("         Description: N/A\n")
		}
		fmt.Printf("         Created: %s\n", accountType.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

// DeleteAccountType deletes an account type
//
// Parameters:
//   - ctx: The context for the operation
//   - client: The initialized Midaz SDK client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - accountTypeID: The account type ID to delete
//
// Returns:
//   - error: Any error encountered during the operation
func DeleteAccountType(ctx context.Context, midazClient *client.Client, orgID, ledgerID, accountTypeID string) error {
	fmt.Println("\n🗑️  Deleting Account Type...")

	// Delete the account type
	err := midazClient.Entity.AccountTypes.DeleteAccountType(ctx, orgID, ledgerID, accountTypeID)
	if err != nil {
		return fmt.Errorf("failed to delete account type: %w", err)
	}

	fmt.Printf("   ✅ Account type deleted successfully: %s\n", accountTypeID)
	return nil
}

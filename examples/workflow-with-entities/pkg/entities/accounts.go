// Package entities contains entity-specific operations for the complete workflow example.
// It provides higher-level functions that wrap the SDK's core functionality to
// simplify common operations and demonstrate best practices.
package entities

import (
	"context"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
)

// CreateAccount creates a new account within a ledger.
//
// This function simplifies account creation by handling the construction of the
// CreateAccountInput model and setting up appropriate metadata. It demonstrates
// how to properly structure account creation requests.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The organization ID that owns the ledger. Must be a valid UUID.
//   - ledgerID: The ledger ID where the account will be created. Must be a valid UUID.
//   - name: Human-readable name for the account (e.g., "Customer Account").
//   - accountType: Type of account (e.g., "deposit", "marketplace", "external").
//   - assetCode: The asset code for the account (e.g., "USD").
//   - alias: Optional alias for the account, stored in metadata.
//   - service: The AccountsService instance to use for the API call.
//
// Returns:
//   - *models.Account: The created account if successful.
//   - error: Any error encountered during account creation.
//
// Example:
//
//	account, err := entities.CreateAccount(
//	    ctx,
//	    "org-123",
//	    "ledger-456",
//	    "Customer Account",
//	    "deposit",
//	    "USD",
//	    "customer-123",
//	    sdkEntity.Accounts,
//	)
func CreateAccount(
	ctx context.Context,
	orgID, ledgerID, name, accountType, assetCode, alias string,
	service entities.AccountsService,
) (*models.Account, error) {
	// Create input
	input := models.NewCreateAccountInput(name, assetCode, accountType)
	input.Metadata = map[string]any{
		"alias":       alias,
		"description": fmt.Sprintf("%s account for %s", accountType, alias),
	}

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid account input: %w", err)
	}

	// Create account
	return service.CreateAccount(ctx, orgID, ledgerID, input)
}

// ListAccounts lists all accounts for a ledger.
//
// This function retrieves all accounts in the specified ledger and returns them
// as a slice of account pointers for easier manipulation. It handles pagination
// internally and converts the API response to a more convenient format.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The organization ID that owns the ledger. Must be a valid UUID.
//   - ledgerID: The ledger ID to list accounts from. Must be a valid UUID.
//   - service: The AccountsService instance to use for the API call.
//
// Returns:
//   - []*models.Account: A slice of account pointers if successful.
//   - error: Any error encountered during the listing operation.
//
// Example:
//
//	accounts, err := entities.ListAccounts(
//	    ctx,
//	    "org-123",
//	    "ledger-456",
//	    sdkEntity.Accounts,
//	)
func ListAccounts(
	ctx context.Context,
	orgID, ledgerID string,
	service entities.AccountsService,
) ([]*models.Account, error) {
	// List accounts
	response, err := service.ListAccounts(ctx, orgID, ledgerID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}

	// Extract accounts from response
	if response == nil {
		return []*models.Account{}, nil
	}

	// Convert slice of models.Account to slice of *models.Account
	accounts := make([]*models.Account, len(response.Items))
	for i := range response.Items {
		accounts[i] = &response.Items[i]
	}

	return accounts, nil
}

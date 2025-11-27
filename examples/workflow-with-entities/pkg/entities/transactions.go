// Package entities contains entity-specific operations for the complete workflow example.
// It provides higher-level functions that wrap the SDK's core functionality to
// simplify common operations and demonstrate best practices.
package entities

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/transaction"
	"github.com/google/uuid"
)

// TransferFunds transfers funds between two accounts within a ledger.
//
// This function simplifies the process of creating a transaction that transfers
// funds from one account to another. It handles the complexities of working with
// account aliases and external accounts, and constructs the appropriate transaction
// input structure expected by the Midaz API.
//
// The function supports both internal and external source accounts. For external accounts,
// use the format "@external/ASSET_CODE" as the sourceAccountID. For internal accounts,
// you can use either the account ID or alias.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - entity: The Entity instance containing all service references.
//   - orgID: The organization ID that owns the ledger. Must be a valid UUID.
//   - ledgerID: The ledger ID where the accounts exist. Must be a valid UUID.
//   - sourceAccountID: The source account ID or alias, or "@external/ASSET_CODE" for external accounts.
//   - destAccountID: The destination account ID or alias.
//   - assetCode: The asset code for the transfer (e.g., "USD", "EUR").
//   - amount: The amount to transfer, expressed as an integer.
//   - scale: The scale factor for the amount (e.g., 2 for cents, making amount 100 = $1.00).
//   - description: A human-readable description of the transaction.
//
// Returns:
//   - *models.Transaction: The created transaction if successful.
//   - error: Any error encountered during the transaction creation.
//
// Example:
//
//	transaction, err := entities.TransferFunds(
//	    ctx,
//	    sdkEntity,
//	    "org-123",
//	    "ledger-456",
//	    "account-789",
//	    "account-012",
//	    "USD",
//	    1000,
//	    2,
//	    "Payment for services",
//	)
func TransferFunds(
	ctx context.Context,
	entity *entities.Entity,
	orgID,
	ledgerID,
	sourceAccountID,
	destAccountID,
	assetCode string,
	amount string,
	description string,
) (*models.Transaction, error) {
	// Determine if source is an external account
	isExternalSource := strings.HasPrefix(sourceAccountID, "@external/")

	// Get source account alias - for external accounts, use the ID directly
	sourceAccountAlias := sourceAccountID

	// For internal accounts, get the account details to use the alias
	// Use the account alias if available, otherwise use the ID as alias
	if !isExternalSource {
		sourceAccount, err := entity.Accounts.GetAccount(ctx, orgID, ledgerID, sourceAccountID)
		if err != nil {
			return nil, fmt.Errorf("failed to get source account: %w", err)
		}

		if sourceAccount.Alias != nil {
			sourceAccountAlias = *sourceAccount.Alias
		}
	}

	// Get destination account alias
	destAccountAlias := destAccountID
	destAccount, err := entity.Accounts.GetAccount(ctx, orgID, ledgerID, destAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get destination account: %w", err)
	}

	// Use the account alias if available, otherwise use the ID as alias
	if destAccount.Alias != nil {
		destAccountAlias = *destAccount.Alias
	}

	// Create a transaction using the new format that matches the backend's expectations
	input := &models.CreateTransactionInput{
		ChartOfAccountsGroupName: "default_chart_group", // Required by API specification
		Description:              description,
		Amount:                   amount,
		AssetCode:                assetCode,
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "transfer",
		},
		Send: &models.SendInput{
			Asset: assetCode,
			Value: amount,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: sourceAccountAlias,
						Amount: models.AmountInput{
							Asset: assetCode,
							Value: amount,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account: destAccountAlias,
						Amount: models.AmountInput{
							Asset: assetCode,
							Value: amount,
						},
					},
				},
			},
		},
		IdempotencyKey: uuid.New().String(),
	}

	// Create the transaction
	tx, err := entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
	if err != nil {
		return nil, fmt.Errorf("failed to transfer funds: %w", err)
	}

	return tx, nil
}

// ExecuteTransferWithHelper transfers funds from one account to another using the transaction helpers
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ledger ID
//   - sourceAccountID: The source account ID
//   - destAccountID: The destination account ID
//   - amount: The amount to transfer (as decimal string)
//   - assetCode: The asset code for the transfer
//
// Returns:
//   - *models.Transaction: The created transaction
//   - error: Any error encountered during the operation
func ExecuteTransferWithHelper(
	ctx context.Context,
	midazClient *client.Client,
	orgID, ledgerID string,
	sourceAccountID, destAccountID string,
	amount string,
	assetCode string,
) (*models.Transaction, error) {
	// Use the Transaction helper from the SDK
	transferOptions := &transaction.TransferOptions{
		Description: "Payment using SDK helper",
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "transfer-with-helper",
		},
		IdempotencyKey: uuid.New().String(),
	}

	// Convert amount string to int64 (assuming 2 decimal places)
	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount format: %w", err)
	}

	amountInt := int64(amountFloat * 100) // Convert to cents
	scale := int64(2)

	// Execute the transfer using the helper
	tx, err := transaction.Transfer(
		ctx,
		midazClient.Entity,
		orgID,
		ledgerID,
		sourceAccountID,
		destAccountID,
		amountInt,
		scale,
		assetCode,
		transferOptions,
	)
	if err != nil {
		return nil, fmt.Errorf("transfer with helper failed: %w", err)
	}

	return tx, nil
}

// ExecuteDepositWithHelper deposits funds from an external source to an account
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - accountID: The destination account ID
//   - amount: The amount to deposit (as decimal string)
//   - assetCode: The asset code for the deposit
//
// Returns:
//   - *models.Transaction: The created transaction
//   - error: Any error encountered during the operation
func ExecuteDepositWithHelper(
	ctx context.Context,
	midazClient *client.Client,
	orgID, ledgerID string,
	accountID string,
	amount string,
	assetCode string,
) (*models.Transaction, error) {
	// Use the Deposit helper from the SDK
	depositOptions := &transaction.DepositOptions{
		Description: "Deposit using SDK helper",
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "deposit-with-helper",
		},
		IdempotencyKey: uuid.New().String(),
	}

	// Convert amount string to int64 (assuming 2 decimal places)
	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount format: %w", err)
	}

	amountInt := int64(amountFloat * 100) // Convert to cents
	scale := int64(2)

	// Execute the deposit using the helper
	tx, err := transaction.Deposit(
		ctx,
		midazClient.Entity,
		orgID,
		ledgerID,
		accountID,
		amountInt,
		scale,
		assetCode,
		depositOptions,
	)
	if err != nil {
		return nil, fmt.Errorf("deposit with helper failed: %w", err)
	}

	return tx, nil
}

// ExecuteWithdrawalWithHelper withdraws funds from an account to an external destination
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - accountID: The source account ID
//   - amount: The amount to withdraw (as decimal string)
//   - assetCode: The asset code for the withdrawal
//
// Returns:
//   - *models.Transaction: The created transaction
//   - error: Any error encountered during the operation
func ExecuteWithdrawalWithHelper(
	ctx context.Context,
	midazClient *client.Client,
	orgID, ledgerID string,
	accountID string,
	amount string,
	assetCode string,
) (*models.Transaction, error) {
	// Use the Withdrawal helper from the SDK
	withdrawalOptions := &transaction.WithdrawalOptions{
		Description: "Withdrawal using SDK helper",
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "withdrawal-with-helper",
		},
		IdempotencyKey: uuid.New().String(),
	}

	// Convert amount string to int64 (assuming 2 decimal places)
	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount format: %w", err)
	}

	amountInt := int64(amountFloat * 100) // Convert to cents
	scale := int64(2)

	// Execute the withdrawal using the helper
	tx, err := transaction.Withdrawal(
		ctx,
		midazClient.Entity,
		orgID,
		ledgerID,
		accountID,
		amountInt,
		scale,
		assetCode,
		withdrawalOptions,
	)
	if err != nil {
		return nil, fmt.Errorf("withdrawal with helper failed: %w", err)
	}

	return tx, nil
}

// ExecuteMultiAccountTransferWithHelper performs a transfer with multiple source and/or destination accounts
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - sourceAccounts: Map of source account IDs to their amounts (as decimal strings)
//   - destAccounts: Map of destination account IDs to their amounts (as decimal strings)
//   - totalAmount: The total amount of the transaction (as decimal string)
//   - assetCode: The asset code for the transfer
//
// Returns:
//   - *models.Transaction: The created transaction
//   - error: Any error encountered during the operation
func ExecuteMultiAccountTransferWithHelper(
	ctx context.Context,
	midazClient *client.Client,
	orgID, ledgerID string,
	sourceAccounts map[string]string,
	destAccounts map[string]string,
	totalAmount string,
	assetCode string,
) (*models.Transaction, error) {
	// Use the MultiAccountTransfer helper from the SDK
	multiTransferOptions := &transaction.MultiTransferOptions{
		Description: "Multi-account transfer using SDK helper",
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "multi-transfer-with-helper",
		},
		IdempotencyKey: uuid.New().String(),
	}

	// Convert string amounts to int64 amounts (assuming 2 decimal places)
	sourceAccountsInt := make(map[string]int64)

	for accountID, amountStr := range sourceAccounts {
		amountFloat, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid source amount format for account %s: %w", accountID, err)
		}

		sourceAccountsInt[accountID] = int64(amountFloat * 100)
	}

	destAccountsInt := make(map[string]int64)

	for accountID, amountStr := range destAccounts {
		amountFloat, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid destination amount format for account %s: %w", accountID, err)
		}

		destAccountsInt[accountID] = int64(amountFloat * 100)
	}

	totalAmountFloat, err := strconv.ParseFloat(totalAmount, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid total amount format: %w", err)
	}

	totalAmountInt := int64(totalAmountFloat * 100)
	scale := int64(2)

	// Execute the multi-account transfer using the helper
	tx, err := transaction.MultiAccountTransfer(
		ctx,
		midazClient.Entity,
		orgID,
		ledgerID,
		sourceAccountsInt,
		destAccountsInt,
		totalAmountInt,
		scale,
		assetCode,
		multiTransferOptions,
	)
	if err != nil {
		return nil, fmt.Errorf("multi-account transfer with helper failed: %w", err)
	}

	return tx, nil
}

// ExecuteBatchTransactionsWithHelper processes multiple transactions in a batch
// NOTE: This function is commented out because batch transaction types are not yet implemented
// in the transaction helper package.
/*
func ExecuteBatchTransactionsWithHelper(
	ctx context.Context,
	midazClient *client.Client,
	orgID, ledgerID string,
	inputs []*models.CreateTransactionInput,
) ([]transaction.BatchResult, *transaction.BatchSummary, error) {
	// Batch functionality not yet implemented in transaction helpers
	var results []transaction.BatchResult
	var summary *transaction.BatchSummary
	return results, summary, fmt.Errorf("batch transactions not yet implemented")
}
*/

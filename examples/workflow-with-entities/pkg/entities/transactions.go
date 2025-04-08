// Package entities contains entity-specific operations for the complete workflow example.
// It provides higher-level functions that wrap the SDK's core functionality to
// simplify common operations and demonstrate best practices.
package entities

import (
	"context"
	"fmt"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/entities"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/transaction"
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
	amount,
	scale int64,
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
		Description: description,
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "transfer",
		},
		Send: &models.SendInput{
			Asset: assetCode,
			Value: amount,
			Scale: scale,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: sourceAccountAlias,
						Amount: models.AmountInput{
							Asset: assetCode,
							Value: amount,
							Scale: scale,
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
							Scale: scale,
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
//   - ledgerID: The ID of the ledger
//   - sourceAccountID: The source account ID
//   - destAccountID: The destination account ID
//   - amount: The amount to transfer
//   - scale: The scale/precision of the amount
//   - assetCode: The asset code for the transfer
//
// Returns:
//   - *models.Transaction: The created transaction
//   - error: Any error encountered during the operation
func ExecuteTransferWithHelper(
	ctx context.Context,
	entity *entities.Entity,
	orgID, ledgerID string,
	sourceAccountID, destAccountID string,
	amount, scale int64,
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

	// Execute the transfer using the helper
	tx, err := transaction.Transfer(
		ctx,
		entity,
		orgID,
		ledgerID,
		sourceAccountID,
		destAccountID,
		amount,
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
//   - amount: The amount to deposit
//   - scale: The scale/precision of the amount
//   - assetCode: The asset code for the deposit
//
// Returns:
//   - *models.Transaction: The created transaction
//   - error: Any error encountered during the operation
func ExecuteDepositWithHelper(
	ctx context.Context,
	entity *entities.Entity,
	orgID, ledgerID string,
	accountID string,
	amount, scale int64,
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

	// Execute the deposit using the helper
	tx, err := transaction.Deposit(
		ctx,
		entity,
		orgID,
		ledgerID,
		accountID,
		amount,
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
//   - amount: The amount to withdraw
//   - scale: The scale/precision of the amount
//   - assetCode: The asset code for the withdrawal
//
// Returns:
//   - *models.Transaction: The created transaction
//   - error: Any error encountered during the operation
func ExecuteWithdrawalWithHelper(
	ctx context.Context,
	entity *entities.Entity,
	orgID, ledgerID string,
	accountID string,
	amount, scale int64,
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

	// Execute the withdrawal using the helper
	tx, err := transaction.Withdrawal(
		ctx,
		entity,
		orgID,
		ledgerID,
		accountID,
		amount,
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
//   - sourceAccounts: Map of source account IDs to their amounts
//   - destAccounts: Map of destination account IDs to their amounts
//   - totalAmount: The total amount of the transaction
//   - scale: The scale/precision of the amount
//   - assetCode: The asset code for the transfer
//
// Returns:
//   - *models.Transaction: The created transaction
//   - error: Any error encountered during the operation
func ExecuteMultiAccountTransferWithHelper(
	ctx context.Context,
	entity *entities.Entity,
	orgID, ledgerID string,
	sourceAccounts map[string]int64,
	destAccounts map[string]int64,
	totalAmount, scale int64,
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

	// Execute the multi-account transfer using the helper
	tx, err := transaction.MultiAccountTransfer(
		ctx,
		entity,
		orgID,
		ledgerID,
		sourceAccounts,
		destAccounts,
		totalAmount,
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
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - inputs: The transaction inputs to process
//
// Returns:
//   - []transaction.BatchResult: The results of the batch operation
//   - transaction.BatchSummary: The summary of the batch operation
//   - error: Any error encountered during the operation
func ExecuteBatchTransactionsWithHelper(
	ctx context.Context,
	entity *entities.Entity,
	orgID, ledgerID string,
	inputs []*models.CreateTransactionInput,
) ([]transaction.BatchResult, *transaction.BatchSummary, error) {
	// Use the BatchTransactions helper from the SDK
	batchOptions := &transaction.BatchOptions{
		Concurrency:          5,
		BatchSize:            25,
		RetryCount:           2,
		IdempotencyKeyPrefix: "batch-example",
		OnProgress: func(completed, total int, result transaction.BatchResult) {
			// This callback is called after each transaction is processed
			percent := float64(completed) / float64(total) * 100
			status := "✓"
			if result.Error != nil {
				status = "✗"
			}
			fmt.Printf("\rProcessing: %d/%d (%.1f%%) %s", completed, total, percent, status)
		},
	}

	// Execute the batch operation
	results, err := transaction.BatchTransactions(
		ctx,
		entity,
		orgID,
		ledgerID,
		inputs,
		batchOptions,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("batch transactions failed: %w", err)
	}

	// Get the batch summary
	summary := transaction.GetBatchSummary(results)

	return results, &summary, nil
}

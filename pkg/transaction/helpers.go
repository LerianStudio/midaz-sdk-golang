// Package transaction provides high-level utilities for creating, processing, and managing
// transactions in the Midaz platform. It includes utility functions for common transaction
// patterns, batch processing with error handling, and template-based transaction creation.
package transaction

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/google/uuid"
)

// formatAmount converts an int64 amount with scale to a decimal string
func formatAmount(amount int64, scale int64) string {
	if scale == 0 {
		return strconv.FormatInt(amount, 10)
	}

	divisor := int64(math.Pow10(int(scale)))
	whole := amount / divisor
	fractional := amount % divisor

	if fractional == 0 {
		return strconv.FormatInt(whole, 10)
	}

	// Format with proper decimal places
	formatStr := fmt.Sprintf("%%.%df", scale)

	return fmt.Sprintf(formatStr, float64(amount)/float64(divisor))
}

// TransferOptions provides configuration options for transfer transactions
type TransferOptions struct {
	// Description is a human-readable description of the transaction
	Description string
	// Metadata contains additional custom data for the transaction
	Metadata map[string]any
	// IdempotencyKey is a client-generated key to ensure transaction uniqueness
	IdempotencyKey string
	// Pending indicates whether the transaction should be created in a pending state
	Pending bool
	// ExternalID is an optional identifier for linking to external systems
	ExternalID string
	// ChartOfAccountsGroupName specifies the chart of accounts group to use
	ChartOfAccountsGroupName string
}

// DefaultTransferOptions returns the default options for transfer transactions
func DefaultTransferOptions() *TransferOptions {
	return &TransferOptions{
		Description:    "Transfer between accounts",
		Metadata:       map[string]any{"source": "go-sdk-transaction-helper"},
		Pending:        false,
		IdempotencyKey: uuid.New().String(),
	}
}

// Transfer creates a transaction that transfers funds from one account to another
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout
//   - entity: The Midaz SDK entity client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - fromAccountID: The source account ID
//   - toAccountID: The destination account ID
//   - amount: The amount to transfer (as a fixed-point integer, e.g., 1000 for $10.00 with scale 2)
//   - scale: The scale/precision of the amount (e.g., 2 for cents)
//   - assetCode: The asset code (e.g., "USD")
//   - opts: Options to configure the transfer (optional, pass nil for defaults)
//
// Returns:
//   - The created transaction if successful
//   - An error if the operation fails
func Transfer(
	ctx context.Context,
	entity *entities.Entity,
	orgID, ledgerID string,
	fromAccountID, toAccountID string,
	amount int64,
	scale int64,
	assetCode string,
	opts *TransferOptions,
) (*models.Transaction, error) {
	// Use default options if none provided
	if opts == nil {
		opts = DefaultTransferOptions()
	}

	// Ensure idempotency key is set
	idempotencyKey := opts.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	// Convert amount to string with scale
	amountStr := formatAmount(amount, scale)

	// Create the transaction input
	transferInput := &models.CreateTransactionInput{
		Description:              opts.Description,
		Amount:                   amountStr,
		AssetCode:                assetCode,
		Metadata:                 opts.Metadata,
		Pending:                  opts.Pending,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               opts.ExternalID,
		ChartOfAccountsGroupName: opts.ChartOfAccountsGroupName,
		Send: &models.SendInput{
			Asset: assetCode,
			Value: amountStr,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: fromAccountID,
						Amount: models.AmountInput{
							Asset: assetCode,
							Value: amountStr,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account: toAccountID,
						Amount: models.AmountInput{
							Asset: assetCode,
							Value: amountStr,
						},
					},
				},
			},
		},
	}

	// Create the transaction
	transaction, err := entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, transferInput)
	if err != nil {
		return nil, fmt.Errorf("transfer transaction failed: %w", err)
	}

	return transaction, nil
}

// DepositOptions provides configuration options for deposit transactions
type DepositOptions struct {
	// Description is a human-readable description of the transaction
	Description string
	// Metadata contains additional custom data for the transaction
	Metadata map[string]any
	// IdempotencyKey is a client-generated key to ensure transaction uniqueness
	IdempotencyKey string
	// Pending indicates whether the transaction should be created in a pending state
	Pending bool
	// ExternalID is an optional identifier for linking to external systems
	ExternalID string
	// ChartOfAccountsGroupName specifies the chart of accounts group to use
	ChartOfAccountsGroupName string
	// ExternalAccountID overrides the default external account ID
	ExternalAccountID string
}

// DefaultDepositOptions returns the default options for deposit transactions
func DefaultDepositOptions() *DepositOptions {
	return &DepositOptions{
		Description:       "Deposit from external source",
		Metadata:          map[string]any{"source": "go-sdk-transaction-helper", "type": "deposit"},
		Pending:           false,
		IdempotencyKey:    uuid.New().String(),
		ExternalAccountID: "", // Will be auto-generated based on asset code
	}
}

// Deposit creates a transaction that deposits funds from an external source to an account
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout
//   - entity: The Midaz SDK entity client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - toAccountID: The destination account ID
//   - amount: The amount to deposit (as a fixed-point integer, e.g., 1000 for $10.00 with scale 2)
//   - scale: The scale/precision of the amount (e.g., 2 for cents)
//   - assetCode: The asset code (e.g., "USD")
//   - opts: Options to configure the deposit (optional, pass nil for defaults)
//
// Returns:
//   - The created transaction if successful
//   - An error if the operation fails
func Deposit(
	ctx context.Context,
	entity *entities.Entity,
	orgID, ledgerID string,
	toAccountID string,
	amount int64,
	scale int64,
	assetCode string,
	opts *DepositOptions,
) (*models.Transaction, error) {
	// Use default options if none provided
	if opts == nil {
		opts = DefaultDepositOptions()
	}

	// Ensure idempotency key is set
	idempotencyKey := opts.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	// Generate external account ID if not specified
	externalAccountID := opts.ExternalAccountID
	if externalAccountID == "" {
		externalAccountID = fmt.Sprintf("@external/%s", assetCode)
	}

	// Convert amount to string with scale
	amountStr := formatAmount(amount, scale)

	// Create the transaction input
	depositInput := &models.CreateTransactionInput{
		Description:              opts.Description,
		Amount:                   amountStr,
		AssetCode:                assetCode,
		Metadata:                 opts.Metadata,
		Pending:                  opts.Pending,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               opts.ExternalID,
		ChartOfAccountsGroupName: opts.ChartOfAccountsGroupName,
		Send: &models.SendInput{
			Asset: assetCode,
			Value: amountStr,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: externalAccountID,
						Amount: models.AmountInput{
							Asset: assetCode,
							Value: amountStr,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account: toAccountID,
						Amount: models.AmountInput{
							Asset: assetCode,
							Value: amountStr,
						},
					},
				},
			},
		},
	}

	// Create the transaction
	transaction, err := entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, depositInput)
	if err != nil {
		return nil, fmt.Errorf("deposit transaction failed: %w", err)
	}

	return transaction, nil
}

// WithdrawalOptions provides configuration options for withdrawal transactions
type WithdrawalOptions struct {
	// Description is a human-readable description of the transaction
	Description string
	// Metadata contains additional custom data for the transaction
	Metadata map[string]any
	// IdempotencyKey is a client-generated key to ensure transaction uniqueness
	IdempotencyKey string
	// Pending indicates whether the transaction should be created in a pending state
	Pending bool
	// ExternalID is an optional identifier for linking to external systems
	ExternalID string
	// ChartOfAccountsGroupName specifies the chart of accounts group to use
	ChartOfAccountsGroupName string
	// ExternalAccountID overrides the default external account ID
	ExternalAccountID string
}

// DefaultWithdrawalOptions returns the default options for withdrawal transactions
func DefaultWithdrawalOptions() *WithdrawalOptions {
	return &WithdrawalOptions{
		Description:       "Withdrawal to external destination",
		Metadata:          map[string]any{"source": "go-sdk-transaction-helper", "type": "withdrawal"},
		Pending:           false,
		IdempotencyKey:    uuid.New().String(),
		ExternalAccountID: "", // Will be auto-generated based on asset code
	}
}

// Withdrawal creates a transaction that withdraws funds from an account to an external destination
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout
//   - entity: The Midaz SDK entity client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - fromAccountID: The source account ID
//   - amount: The amount to withdraw (as a fixed-point integer, e.g., 1000 for $10.00 with scale 2)
//   - scale: The scale/precision of the amount (e.g., 2 for cents)
//   - assetCode: The asset code (e.g., "USD")
//   - opts: Options to configure the withdrawal (optional, pass nil for defaults)
//
// Returns:
//   - The created transaction if successful
//   - An error if the operation fails
func Withdrawal(
	ctx context.Context,
	entity *entities.Entity,
	orgID, ledgerID string,
	fromAccountID string,
	amount int64,
	scale int64,
	assetCode string,
	opts *WithdrawalOptions,
) (*models.Transaction, error) {
	// Use default options if none provided
	if opts == nil {
		opts = DefaultWithdrawalOptions()
	}

	// Ensure idempotency key is set
	idempotencyKey := opts.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	// Generate external account ID if not specified
	externalAccountID := opts.ExternalAccountID
	if externalAccountID == "" {
		externalAccountID = fmt.Sprintf("@external/%s", assetCode)
	}

	// Convert amount to string with scale
	amountStr := formatAmount(amount, scale)

	// Create the transaction input
	withdrawalInput := &models.CreateTransactionInput{
		Description:              opts.Description,
		Amount:                   amountStr,
		AssetCode:                assetCode,
		Metadata:                 opts.Metadata,
		Pending:                  opts.Pending,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               opts.ExternalID,
		ChartOfAccountsGroupName: opts.ChartOfAccountsGroupName,
		Send: &models.SendInput{
			Asset: assetCode,
			Value: amountStr,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: fromAccountID,
						Amount: models.AmountInput{
							Asset: assetCode,
							Value: amountStr,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account: externalAccountID,
						Amount: models.AmountInput{
							Asset: assetCode,
							Value: amountStr,
						},
					},
				},
			},
		},
	}

	// Create the transaction
	transaction, err := entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, withdrawalInput)
	if err != nil {
		return nil, fmt.Errorf("withdrawal transaction failed: %w", err)
	}

	return transaction, nil
}

// MultiTransferOptions provides configuration options for multi-leg transfers
type MultiTransferOptions struct {
	// Description is a human-readable description of the transaction
	Description string
	// Metadata contains additional custom data for the transaction
	Metadata map[string]any
	// IdempotencyKey is a client-generated key to ensure transaction uniqueness
	IdempotencyKey string
	// Pending indicates whether the transaction should be created in a pending state
	Pending bool
	// ExternalID is an optional identifier for linking to external systems
	ExternalID string
	// ChartOfAccountsGroupName specifies the chart of accounts group to use
	ChartOfAccountsGroupName string
}

// DefaultMultiTransferOptions returns the default options for multi-leg transfers
func DefaultMultiTransferOptions() *MultiTransferOptions {
	return &MultiTransferOptions{
		Description:    "Multi-account transfer",
		Metadata:       map[string]any{"source": "go-sdk-transaction-helper", "type": "multi-transfer"},
		Pending:        false,
		IdempotencyKey: uuid.New().String(),
	}
}

// MultiAccountTransfer creates a transaction with multiple source and/or destination accounts
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout
//   - entity: The Midaz SDK entity client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - sourceAccounts: Map of source account IDs to their amounts (must sum to totalAmount)
//   - destAccounts: Map of destination account IDs to their amounts (must sum to totalAmount)
//   - totalAmount: The total amount of the transaction
//   - scale: The scale/precision of the amount (e.g., 2 for cents)
//   - assetCode: The asset code (e.g., "USD")
//   - opts: Options to configure the transfer (optional, pass nil for defaults)
//
// Returns:
//   - The created transaction if successful
//   - An error if the operation fails
func MultiAccountTransfer(
	ctx context.Context,
	entity *entities.Entity,
	orgID, ledgerID string,
	sourceAccounts map[string]int64,
	destAccounts map[string]int64,
	totalAmount int64,
	scale int64,
	assetCode string,
	opts *MultiTransferOptions,
) (*models.Transaction, error) {
	// Use default options if none provided
	if opts == nil {
		opts = DefaultMultiTransferOptions()
	}

	// Ensure idempotency key is set
	idempotencyKey := opts.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	// Validate that we have at least one source and one destination account
	if len(sourceAccounts) == 0 {
		return nil, fmt.Errorf("at least one source account is required")
	}

	if len(destAccounts) == 0 {
		return nil, fmt.Errorf("at least one destination account is required")
	}

	// Create FromToInput slices for source and destination accounts
	fromList := make([]models.FromToInput, 0, len(sourceAccounts))
	toList := make([]models.FromToInput, 0, len(destAccounts))

	// Sum source and destination amounts to validate balance
	var sourceSum, destSum int64

	// Build source accounts list
	for accountID, amount := range sourceAccounts {
		if amount <= 0 {
			return nil, fmt.Errorf("amount for source account %s must be positive", accountID)
		}

		amountStr := formatAmount(amount, scale)
		fromList = append(fromList, models.FromToInput{
			Account: accountID,
			Amount: models.AmountInput{
				Asset: assetCode,
				Value: amountStr,
			},
		})

		sourceSum += amount
	}

	// Build destination accounts list
	for accountID, amount := range destAccounts {
		if amount <= 0 {
			return nil, fmt.Errorf("amount for destination account %s must be positive", accountID)
		}

		amountStr := formatAmount(amount, scale)
		toList = append(toList, models.FromToInput{
			Account: accountID,
			Amount: models.AmountInput{
				Asset: assetCode,
				Value: amountStr,
			},
		})

		destSum += amount
	}

	// Verify the transaction is balanced
	if sourceSum != destSum {
		return nil, fmt.Errorf("unbalanced transaction: source amount (%d) does not equal destination amount (%d)", sourceSum, destSum)
	}

	// Verify the total amount is correct
	if sourceSum != totalAmount {
		return nil, fmt.Errorf("total amount mismatch: specified total (%d) does not match sum of accounts (%d)", totalAmount, sourceSum)
	}

	// Convert total amount to string with scale
	totalAmountStr := formatAmount(totalAmount, scale)

	// Create the transaction input
	multiTransferInput := &models.CreateTransactionInput{
		Description:              opts.Description,
		Amount:                   totalAmountStr,
		AssetCode:                assetCode,
		Metadata:                 opts.Metadata,
		Pending:                  opts.Pending,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               opts.ExternalID,
		ChartOfAccountsGroupName: opts.ChartOfAccountsGroupName,
		Send: &models.SendInput{
			Asset: assetCode,
			Value: totalAmountStr,
			Source: &models.SourceInput{
				From: fromList,
			},
			Distribute: &models.DistributeInput{
				To: toList,
			},
		},
	}

	// Create the transaction
	transaction, err := entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, multiTransferInput)
	if err != nil {
		return nil, fmt.Errorf("multi-account transfer failed: %w", err)
	}

	return transaction, nil
}

// TransactionTemplate represents a reusable transaction pattern
type TransactionTemplate struct {
	// Description is a human-readable description of the transaction
	Description string
	// AssetCode is the asset code for the transaction
	AssetCode string
	// Scale is the decimal precision for the amount
	Scale int64
	// Metadata contains additional custom data for the transaction
	Metadata map[string]any
	// Pending indicates whether the transaction should be created in a pending state
	Pending bool
	// ChartOfAccountsGroupName specifies the chart of accounts group to use
	ChartOfAccountsGroupName string
	// BuildSources is a function that constructs the source accounts
	BuildSources func(amount int64) []models.FromToInput
	// BuildDestinations is a function that constructs the destination accounts
	BuildDestinations func(amount int64) []models.FromToInput
}

// CreateFromTemplate creates a transaction from a template with the specified amount
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout
//   - entity: The Midaz SDK entity client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - template: The transaction template to use
//   - amount: The amount for the transaction
//   - metadata: Additional metadata to merge with the template's metadata (optional)
//   - idempotencyKey: A unique key for idempotency (optional, will generate one if empty)
//
// Returns:
//   - The created transaction if successful
//   - An error if the operation fails
func CreateFromTemplate(
	ctx context.Context,
	entity *entities.Entity,
	orgID, ledgerID string,
	template *TransactionTemplate,
	amount int64,
	metadata map[string]any,
	idempotencyKey string,
) (*models.Transaction, error) {
	if template == nil {
		return nil, fmt.Errorf("transaction template cannot be nil")
	}

	// Ensure idempotency key is set
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	// Merge metadata
	mergedMetadata := make(map[string]any)

	if template.Metadata != nil {
		for k, v := range template.Metadata {
			mergedMetadata[k] = v
		}
	}

	for k, v := range metadata {
		mergedMetadata[k] = v
	}
	// Add timestamp to metadata
	mergedMetadata["timestamp"] = time.Now().Unix()

	// Convert amount to string with scale
	amountStr := formatAmount(amount, template.Scale)

	// Create the transaction input
	input := &models.CreateTransactionInput{
		Description:              template.Description,
		Amount:                   amountStr,
		AssetCode:                template.AssetCode,
		Metadata:                 mergedMetadata,
		Pending:                  template.Pending,
		IdempotencyKey:           idempotencyKey,
		ChartOfAccountsGroupName: template.ChartOfAccountsGroupName,
		Send: &models.SendInput{
			Asset: template.AssetCode,
			Value: amountStr,
			Source: &models.SourceInput{
				From: template.BuildSources(amount),
			},
			Distribute: &models.DistributeInput{
				To: template.BuildDestinations(amount),
			},
		},
	}

	// Create the transaction
	transaction, err := entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
	if err != nil {
		return nil, fmt.Errorf("transaction from template failed: %w", err)
	}

	return transaction, nil
}

// IsTransactionSuccessful checks if a transaction was successfully completed
//
// Parameters:
//   - tx: The transaction to check
//
// Returns:
//   - true if the transaction is completed successfully, false otherwise
func IsTransactionSuccessful(tx *models.Transaction) bool {
	if tx == nil {
		return false
	}

	// Check if the status is "COMPLETED"
	return tx.Status.Code == "COMPLETED"
}

// GetTransactionStatus returns a clean status string for a transaction
//
// Parameters:
//   - tx: The transaction to check
//
// Returns:
//   - A clean status string (e.g., "Completed", "Failed", "Pending")
func GetTransactionStatus(tx *models.Transaction) string {
	if tx == nil {
		return "Unknown"
	}

	status := tx.Status.Code
	switch status {
	case "COMPLETED":
		return "Completed"
	case "FAILED":
		return "Failed"
	case "PENDING":
		return "Pending"
	case "CANCELED":
		return "Canceled"
	default:
		return status
	}
}

// CommitPendingTransaction commits a pending transaction
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout
//   - entity: The Midaz SDK entity client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - transactionID: The ID of the pending transaction to commit
//
// Returns:
//   - The committed transaction if successful
//   - An error if the operation fails
func CommitPendingTransaction(
    ctx context.Context,
    entity *entities.Entity,
    orgID, ledgerID, transactionID string,
) (*models.Transaction, error) {
    // Use dedicated commit endpoint
    committed, err := entity.Transactions.CommitTransaction(ctx, orgID, ledgerID, transactionID)
    if err != nil {
        return nil, fmt.Errorf("failed to commit transaction: %w", err)
    }
    return committed, nil
}

// CancelPendingTransaction cancels a pending transaction
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout
//   - entity: The Midaz SDK entity client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - transactionID: The ID of the pending transaction to cancel
//
// Returns:
//   - The canceled transaction if successful
//   - An error if the operation fails
func CancelPendingTransaction(
    ctx context.Context,
    entity *entities.Entity,
    orgID, ledgerID, transactionID string,
) (*models.Transaction, error) {
    // Use dedicated cancel endpoint, which returns no body in our Entities implementation.
    if err := entity.Transactions.CancelTransaction(ctx, orgID, ledgerID, transactionID); err != nil {
        return nil, fmt.Errorf("failed to cancel transaction: %w", err)
    }
    // Fetch the transaction to return its final state (best-effort).
    tx, _ := entity.Transactions.GetTransaction(ctx, orgID, ledgerID, transactionID)
    return tx, nil
}

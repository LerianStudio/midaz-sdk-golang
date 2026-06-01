// Package transaction provides high-level utilities for creating, processing, and managing
// transactions in the Midaz platform. It includes utility functions for common transaction
// patterns, batch processing with error handling, and template-based transaction creation.
package transaction

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/google/uuid"
)

// formatAmount converts an int64 amount with scale to a decimal string using
// integer math only — no float64 conversion. Float64 has only ~15-17 significant
// decimal digits, so round-tripping fixed-point values at scale 8 (BTC sat) or
// scale 18 (wei) silently loses precision past the 2^53 mantissa boundary.
//
// For scale outside [0, 18] the raw integer is returned for backward compatibility.
// Scale 18 is the practical maximum (wei); higher scales overflow int64 divisor math.
func formatAmount(amount int64, scale int64) string {
	if scale <= 0 || scale > 18 {
		return strconv.FormatInt(amount, 10)
	}

	sign := ""
	// Avoid -math.MinInt64 overflow: handle sign via formatting, not negation.
	abs := amount
	if amount < 0 {
		sign = "-"
		// MinInt64 cannot be negated; uint64 conversion handles it.
		if amount == -1<<63 {
			// Format using uint64 to avoid overflow.
			absU := uint64(1 << 63)
			divisor := uint64(1)
			for i := int64(0); i < scale; i++ {
				divisor *= 10
			}
			whole := absU / divisor
			frac := absU % divisor
			return fmt.Sprintf("%s%d.%0*d", sign, whole, int(scale), frac)
		}
		abs = -amount
	}

	divisor := int64(1)
	for i := int64(0); i < scale; i++ {
		divisor *= 10
	}
	whole := abs / divisor
	frac := abs % divisor
	return fmt.Sprintf("%s%d.%0*d", sign, whole, int(scale), frac)
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
		Description: "Transfer between accounts",
		Metadata:    map[string]any{"source": "go-sdk-transaction-helper"},
		Pending:     false,
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
	if entity == nil {
		return nil, errors.New("entity is required")
	}

	if entity.Transactions == nil {
		return nil, errors.New("transactions service is not initialized")
	}

	// Use default options if none provided
	if opts == nil {
		opts = DefaultTransferOptions()
	}

	// Ensure idempotency key is set
	idempotencyKey := opts.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	amountValue := formatAmount(amount, scale)

	// Create the transaction input
	transferInput := &models.CreateTransactionInput{
		Description:              opts.Description,
		Metadata:                 opts.Metadata,
		Pending:                  opts.Pending,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               opts.ExternalID,
		ChartOfAccountsGroupName: opts.ChartOfAccountsGroupName,
		Send: &models.SendInput{
			Asset: assetCode,
			Value: amountValue,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: fromAccountID,
						Amount: models.AmountInput{
							Asset: assetCode,
							Value: amountValue,
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
							Value: amountValue,
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
	if entity == nil {
		return nil, errors.New("entity is required")
	}

	if entity.Transactions == nil {
		return nil, errors.New("transactions service is not initialized")
	}

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
	amountValue := formatAmount(amount, scale)

	// Create the transaction input
	depositInput := &models.CreateTransactionInput{
		Description:              opts.Description,
		Metadata:                 opts.Metadata,
		Pending:                  opts.Pending,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               opts.ExternalID,
		ChartOfAccountsGroupName: opts.ChartOfAccountsGroupName,
		Send: &models.SendInput{
			Asset: assetCode,
			Value: amountValue,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: externalAccountID,
						Amount: models.AmountInput{
							Asset: assetCode,
							Value: amountValue,
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
							Value: amountValue,
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
	if entity == nil {
		return nil, errors.New("entity is required")
	}

	if entity.Transactions == nil {
		return nil, errors.New("transactions service is not initialized")
	}

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
	amountValue := formatAmount(amount, scale)

	// Create the transaction input
	withdrawalInput := &models.CreateTransactionInput{
		Description:              opts.Description,
		Metadata:                 opts.Metadata,
		Pending:                  opts.Pending,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               opts.ExternalID,
		ChartOfAccountsGroupName: opts.ChartOfAccountsGroupName,
		Send: &models.SendInput{
			Asset: assetCode,
			Value: amountValue,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: fromAccountID,
						Amount: models.AmountInput{
							Asset: assetCode,
							Value: amountValue,
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
							Value: amountValue,
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
		Description: "Multi-account transfer",
		Metadata:    map[string]any{"source": "go-sdk-transaction-helper", "type": "multi-transfer"},
		Pending:     false,
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
	if entity == nil {
		return nil, errors.New("entity is required")
	}

	if entity.Transactions == nil {
		return nil, errors.New("transactions service is not initialized")
	}

	opts, idempotencyKey := resolveMultiTransferOptions(opts)

	if err := validateMultiTransferAccounts(sourceAccounts, destAccounts); err != nil {
		return nil, err
	}

	fromList, sourceSum, err := buildAccountInputList(sourceAccounts, scale, assetCode, "source")
	if err != nil {
		return nil, err
	}

	toList, destSum, err := buildAccountInputList(destAccounts, scale, assetCode, "destination")
	if err != nil {
		return nil, err
	}

	if err := validateMultiTransferAmounts(sourceSum, destSum, totalAmount); err != nil {
		return nil, err
	}

	multiTransferInput := buildMultiTransferInput(opts, idempotencyKey, totalAmount, scale, assetCode, fromList, toList)

	transaction, err := entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, multiTransferInput)
	if err != nil {
		return nil, fmt.Errorf("multi-account transfer failed: %w", err)
	}

	return transaction, nil
}

func resolveMultiTransferOptions(opts *MultiTransferOptions) (*MultiTransferOptions, string) {
	if opts == nil {
		opts = DefaultMultiTransferOptions()
	}

	idempotencyKey := opts.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	return opts, idempotencyKey
}

func validateMultiTransferAccounts(sourceAccounts, destAccounts map[string]int64) error {
	if len(sourceAccounts) == 0 {
		return errors.New("at least one source account is required")
	}

	if len(destAccounts) == 0 {
		return errors.New("at least one destination account is required")
	}

	return nil
}

func buildAccountInputList(accounts map[string]int64, scale int64, assetCode, accountType string) ([]models.FromToInput, int64, error) {
	inputList := make([]models.FromToInput, 0, len(accounts))

	var sum int64

	for accountID, amount := range accounts {
		if amount <= 0 {
			return nil, 0, fmt.Errorf("amount for %s account %s must be positive", accountType, accountID)
		}

		amountValue := formatAmount(amount, scale)
		inputList = append(inputList, models.FromToInput{
			Account: accountID,
			Amount: models.AmountInput{
				Asset: assetCode,
				Value: amountValue,
			},
		})

		sum += amount
	}

	return inputList, sum, nil
}

func validateMultiTransferAmounts(sourceSum, destSum, totalAmount int64) error {
	if sourceSum != destSum {
		return fmt.Errorf("unbalanced transaction: source amount (%d) does not equal destination amount (%d)", sourceSum, destSum)
	}

	if sourceSum != totalAmount {
		return fmt.Errorf("total amount mismatch: specified total (%d) does not match sum of accounts (%d)", totalAmount, sourceSum)
	}

	return nil
}

func buildMultiTransferInput(opts *MultiTransferOptions, idempotencyKey string, totalAmount, scale int64, assetCode string, fromList, toList []models.FromToInput) *models.CreateTransactionInput {
	totalAmountValue := formatAmount(totalAmount, scale)

	return &models.CreateTransactionInput{
		Description:              opts.Description,
		Metadata:                 opts.Metadata,
		Pending:                  opts.Pending,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               opts.ExternalID,
		ChartOfAccountsGroupName: opts.ChartOfAccountsGroupName,
		Send: &models.SendInput{
			Asset: assetCode,
			Value: totalAmountValue,
			Source: &models.SourceInput{
				From: fromList,
			},
			Distribute: &models.DistributeInput{
				To: toList,
			},
		},
	}
}

// Template represents a reusable transaction pattern
type Template struct {
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
	template *Template,
	amount int64,
	metadata map[string]any,
	idempotencyKey string,
) (*models.Transaction, error) {
	if entity == nil {
		return nil, errors.New("entity is required")
	}

	if entity.Transactions == nil {
		return nil, errors.New("transactions service is not initialized")
	}

	if template == nil {
		return nil, errors.New("transaction template cannot be nil")
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
	amountValue := formatAmount(amount, template.Scale)

	// Create the transaction input
	input := &models.CreateTransactionInput{
		Description:              template.Description,
		Metadata:                 mergedMetadata,
		Pending:                  template.Pending,
		IdempotencyKey:           idempotencyKey,
		ChartOfAccountsGroupName: template.ChartOfAccountsGroupName,
		Send: &models.SendInput{
			Asset: template.AssetCode,
			Value: amountValue,
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

// IsTransactionSuccessful reports whether a transaction's operations have been
// applied to account balances. Per the Midaz server contract, that terminal
// success state is APPROVED (the result of an immediate create or a commit) —
// there is no COMPLETED status.
//
// Parameters:
//   - tx: The transaction to check
//
// Returns:
//   - true if the transaction status is APPROVED, false otherwise
func IsTransactionSuccessful(tx *models.Transaction) bool {
	if tx == nil {
		return false
	}

	return tx.Status.Code == string(models.TransactionStatusApproved)
}

// GetTransactionStatus returns a clean, human-readable status string for a
// transaction, mapping the server's canonical 5-value vocabulary
// (CREATED/PENDING/APPROVED/CANCELED/NOTED) to title-case display strings.
// Any unrecognized status is returned verbatim.
//
// Parameters:
//   - tx: The transaction to check
//
// Returns:
//   - A clean status string (e.g., "Approved", "Pending", "Canceled")
func GetTransactionStatus(tx *models.Transaction) string {
	if tx == nil {
		return "Unknown"
	}

	switch tx.Status.Code {
	case string(models.TransactionStatusCreated):
		return "Created"
	case string(models.TransactionStatusPending):
		return "Pending"
	case string(models.TransactionStatusApproved):
		return "Approved"
	case string(models.TransactionStatusCanceled):
		return "Canceled"
	case string(models.TransactionStatusNoted):
		return "Noted"
	default:
		return tx.Status.Code
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
	if entity == nil {
		return nil, errors.New("entity is required")
	}

	if entity.Transactions == nil {
		return nil, errors.New("transactions service is not initialized")
	}

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
	if entity == nil {
		return nil, errors.New("entity is required")
	}

	if entity.Transactions == nil {
		return nil, errors.New("transactions service is not initialized")
	}

	tx, err := entity.Transactions.CancelTransactionWithResponse(ctx, orgID, ledgerID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel transaction: %w", err)
	}

	if tx != nil {
		return tx, nil
	}

	fetchedTx, fetchErr := entity.Transactions.GetTransaction(ctx, orgID, ledgerID, transactionID)
	if fetchErr != nil {
		return nil, fmt.Errorf("transaction canceled but final state could not be fetched: %w", fetchErr)
	}

	if fetchedTx == nil {
		return nil, errors.New("transaction canceled but final state was empty")
	}

	return fetchedTx, nil
}

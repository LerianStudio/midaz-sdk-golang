package workflows

import (
	"context"
	"fmt"
	"strings"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/conversion"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/format"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation"
	"github.com/google/uuid"
)

// ExecuteTransactions executes various transactions between accounts
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - merchantAccount: The merchant account model
//
// Returns:
//   - error: Any error encountered during the operation
func ExecuteTransactions(ctx context.Context, client *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account) error {
	// Create a span for observability
	ctx, span := observability.StartSpan(ctx, "ExecuteTransactions")
	defer span.End()

	// Add attributes for tracing
	observability.AddAttribute(ctx, "organization_id", orgID)
	observability.AddAttribute(ctx, "ledger_id", ledgerID)

	fmt.Println("\n\n💸 STEP 5: TRANSACTION EXECUTION")
	fmt.Println(strings.Repeat("=", 50))

	// Validate inputs using the validation package
	if !validation.IsValidUUID(orgID) || !validation.IsValidUUID(ledgerID) {
		err := fmt.Errorf("invalid organization or ledger ID format")
		observability.RecordError(ctx, err, "invalid_input")
		return err
	}

	if customerAccount == nil || merchantAccount == nil {
		err := fmt.Errorf("customer or merchant account is nil")
		observability.RecordError(ctx, err, "nil_account")
		return err
	}

	// Get external account ID
	externalAccountID := fmt.Sprintf("@external/%s", "USD")

	// Apply performance optimizations
	perfOptions := performance.Options{
		EnableHTTPPooling:   true,
		MaxIdleConnsPerHost: 10,
		UseJSONIterator:     true,
	}
	performance.ApplyGlobalPerformanceOptions(perfOptions)

	// Execute initial deposit
	depositCtx, depositSpan := observability.StartSpan(ctx, "InitialDeposit")
	if err := ExecuteInitialDeposit(depositCtx, client, orgID, ledgerID, customerAccount, externalAccountID); err != nil {
		depositSpan.End()
		observability.RecordError(ctx, err, "initial_deposit_failed")
		return err
	}
	depositSpan.End()

	// Execute multiple deposits to both accounts
	multiDepositCtx, multiDepositSpan := observability.StartSpan(ctx, "MultipleDeposits")
	if err := ExecuteMultipleDeposits(multiDepositCtx, client, orgID, ledgerID, customerAccount, merchantAccount, externalAccountID); err != nil {
		multiDepositSpan.End()
		observability.RecordError(ctx, err, "multiple_deposits_failed")
		return err
	}
	multiDepositSpan.End()

	// Execute single transfer from customer to merchant
	singleTransferCtx, singleTransferSpan := observability.StartSpan(ctx, "SingleTransfer")
	if err := ExecuteSingleTransfer(singleTransferCtx, client, orgID, ledgerID, customerAccount, merchantAccount); err != nil {
		singleTransferSpan.End()
		observability.RecordError(ctx, err, "single_transfer_failed")
		return err
	}
	singleTransferSpan.End()

	// Execute multiple transfers between accounts
	multiTransferCtx, multiTransferSpan := observability.StartSpan(ctx, "MultipleTransfers")
	if err := ExecuteMultipleTransfers(multiTransferCtx, client, orgID, ledgerID, customerAccount, merchantAccount); err != nil {
		multiTransferSpan.End()
		observability.RecordError(ctx, err, "multiple_transfers_failed")
		return err
	}
	multiTransferSpan.End()

	// Execute withdrawals to external account
	withdrawalCtx, withdrawalSpan := observability.StartSpan(ctx, "Withdrawals")
	if err := ExecuteWithdrawals(withdrawalCtx, client, orgID, ledgerID, customerAccount, merchantAccount, externalAccountID); err != nil {
		withdrawalSpan.End()
		observability.RecordError(ctx, err, "withdrawals_failed")
		return err
	}
	withdrawalSpan.End()

	// Execute concurrent transactions for TPS testing
	concurrentCtx, concurrentSpan := observability.StartSpan(ctx, "ConcurrentTransactions")
	if err := ExecuteConcurrentTransactions(concurrentCtx, client, orgID, ledgerID, customerAccount, merchantAccount); err != nil {
		concurrentSpan.End()
		observability.RecordError(ctx, err, "concurrent_transactions_failed")
		return err
	}
	concurrentSpan.End()

	// Test transactions with insufficient funds
	insufficientCtx, insufficientSpan := observability.StartSpan(ctx, "InsufficientFundsTest")
	ExecuteInsufficientFundsTransactions(insufficientCtx, client, orgID, ledgerID, customerAccount, merchantAccount, externalAccountID)
	insufficientSpan.End()

	fmt.Println("\n💰 All transactions completed successfully!")
	return nil
}

// ExecuteInitialDeposit performs the initial deposit from external account to customer account
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - externalAccountID: The external account ID
//
// Returns:
//   - error: Any error encountered during the operation
func ExecuteInitialDeposit(ctx context.Context, client *client.Client, orgID, ledgerID string, customerAccount *models.Account, externalAccountID string) error {
	// Create a span for this function
	ctx, span := observability.StartSpan(ctx, "ExecuteInitialDeposit")
	defer span.End()

	fmt.Println("Depositing funds from external account to customer account...")

	// Validate the amount using validation package
	amount := int64(500000) // $5000.00
	scale := int64(2)

	if !validation.IsValidAmount(amount, scale) {
		err := fmt.Errorf("invalid amount for initial deposit")
		observability.RecordError(ctx, err, "invalid_amount")
		return err
	}

	// Create a deposit transaction using the structure expected by the backend
	depositInput := &models.CreateTransactionInput{
		Description: "Initial deposit from external account",
		Amount:      amount,
		Scale:       scale,
		AssetCode:   "USD",
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "deposit",
		},
		Send: &models.SendInput{
			Asset: "USD",
			Value: amount,
			Scale: scale,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: externalAccountID,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
							Scale: scale,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account: *customerAccount.Alias,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
							Scale: scale,
						},
					},
				},
			},
		},
		// Generate a truly unique idempotency key using UUID
		IdempotencyKey: uuid.New().String(),
	}

	// Record the start time for timing metrics
	startTime := time.Now()

	// Configure retry options for this critical transaction using functional options
	retryOptions := retry.DefaultOptions()
	retry.WithMaxRetries(5)(retryOptions)
	retry.WithInitialDelay(100 * time.Millisecond)(retryOptions)
	retry.WithMaxDelay(2 * time.Second)(retryOptions)
	retry.WithBackoffFactor(2.0)(retryOptions)

	// Create a retry context with the options
	retryCtx := retry.WithOptionsContext(ctx, retryOptions)

	// Execute the transaction with retries
	depositTransaction, err := client.Entity.Transactions.CreateTransaction(retryCtx, orgID, ledgerID, depositInput)

	// Record the duration for observability
	duration := time.Since(startTime)
	observability.RecordSpanMetric(ctx, "deposit_duration_ms", float64(duration.Milliseconds()))

	if err != nil {
		// Use standardized error handling to provide better error messages
		errorDetails := sdkerrors.GetErrorDetails(err)
		observability.RecordError(ctx, err, "initial_deposit_failed",
			map[string]string{
				"error_code":  errorDetails.Code,
				"http_status": fmt.Sprintf("%d", errorDetails.HTTPStatus),
			})

		switch {
		case sdkerrors.IsValidationError(err):
			return fmt.Errorf("deposit has invalid parameters: %w", err)
		case sdkerrors.IsInsufficientBalanceError(err):
			return fmt.Errorf("external account has insufficient funds: %w", err)
		case sdkerrors.IsIdempotencyError(err):
			return fmt.Errorf("deposit was already processed (idempotency key conflict): %w", err)
		default:
			// Format the error message for display
			errorMsg := sdkerrors.FormatOperationError(err, "Deposit")
			return fmt.Errorf("deposit failed: %s", errorMsg)
		}
	}

	if depositTransaction.ID == "" {
		err := fmt.Errorf("deposit transaction created but no ID was returned from the API")
		observability.RecordError(ctx, err, "empty_transaction_id")
		return err
	}

	// Add successful transaction data to observability
	observability.AddAttribute(ctx, "transaction_id", depositTransaction.ID)
	observability.AddAttribute(ctx, "transaction_amount", depositTransaction.Amount)
	observability.AddAttribute(ctx, "transaction_asset", depositTransaction.AssetCode)

	// Format the amount using the format package
	formattedAmount := format.FormatCurrency(depositTransaction.Amount, depositTransaction.Scale, depositTransaction.AssetCode)

	fmt.Printf("✅ Deposit executed successfully\n")
	fmt.Printf("   Transaction ID: %s\n", depositTransaction.ID)
	fmt.Printf("   Amount: %s\n", formattedAmount)

	return nil
}

// ExecuteMultipleDeposits performs multiple deposits to customer and merchant accounts
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - merchantAccount: The merchant account model
//   - externalAccountID: The external account ID
//
// Returns:
//   - error: Any error encountered during the operation
func ExecuteMultipleDeposits(ctx context.Context, client *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, externalAccountID string) error {
	// Create span for this operation
	ctx, span := observability.StartSpan(ctx, "ExecuteMultipleDeposits")
	defer span.End()

	fmt.Println("\nExecuting multiple deposits for all accounts...")

	// Define deposit amounts (in cents)
	customerDepositAmounts := []int64{1250, 3500, 7800, 500, 2200, 1800, 4500, 900, 6000, 3300}
	merchantDepositAmounts := []int64{5500, 2800, 1200, 9000, 4200, 7500, 3000, 6500, 1500, 8000}

	// Add operation details to observability
	observability.AddAttribute(ctx, "customer_deposit_count", len(customerDepositAmounts))
	observability.AddAttribute(ctx, "merchant_deposit_count", len(merchantDepositAmounts))

	// Use performance package for batch operations
	performance.ApplyBatchingOptions(performance.Options{
		BatchSize:       5,
		UseJSONIterator: true,
	})

	// Execute 10 deposits to customer account
	fmt.Println("\n📥 Executing 10 deposits to customer account...")
	customerCtx, customerSpan := observability.StartSpan(ctx, "CustomerDeposits")

	for i, amount := range customerDepositAmounts {
		// Validate the amount
		if !validation.IsValidAmount(amount, 2) {
			err := fmt.Errorf("invalid amount for customer deposit #%d: %d", i+1, amount)
			observability.RecordError(customerCtx, err, "invalid_amount")
			customerSpan.End()
			return err
		}

		depositInput := CreateDepositInput(
			fmt.Sprintf("Customer deposit #%d", i+1),
			amount,
			externalAccountID,
			customerAccount.ID,
		)

		// Add additional metadata using the conversion package
		depositInput.Metadata = conversion.EnhanceMetadata(depositInput.Metadata, map[string]interface{}{
			"deposit_index": i + 1,
			"account_type":  "customer",
			"timestamp":     time.Now().Unix(),
		})

		txCtx, txSpan := observability.StartSpan(customerCtx, "CustomerDeposit")
		startTime := time.Now()

		tx, err := client.Entity.Transactions.CreateTransaction(txCtx, orgID, ledgerID, depositInput)

		duration := time.Since(startTime)
		observability.RecordSpanMetric(txCtx, "customer_deposit_duration_ms", float64(duration.Milliseconds()))

		if err != nil {
			observability.RecordError(txCtx, err, "customer_deposit_failed")
			txSpan.End()
			customerSpan.End()
			return fmt.Errorf("failed to deposit funds to customer account: %w", err)
		}

		observability.AddAttribute(txCtx, "transaction_id", tx.ID)
		observability.AddAttribute(txCtx, "amount", amount)
		txSpan.End()

		// Format the amount using the format package
		formattedAmount := format.FormatCurrency(amount, 2, "USD")
		fmt.Printf("✅ Deposit #%d: %s (ID: %s)\n", i+1, formattedAmount, tx.ID)
	}
	customerSpan.End()

	// Execute 10 deposits to merchant account
	fmt.Println("\n📥 Executing 10 deposits to merchant account...")
	merchantCtx, merchantSpan := observability.StartSpan(ctx, "MerchantDeposits")

	for i, amount := range merchantDepositAmounts {
		// Validate the amount
		if !validation.IsValidAmount(amount, 2) {
			err := fmt.Errorf("invalid amount for merchant deposit #%d: %d", i+1, amount)
			observability.RecordError(merchantCtx, err, "invalid_amount")
			merchantSpan.End()
			return err
		}

		depositInput := CreateDepositInput(
			fmt.Sprintf("Merchant deposit #%d", i+1),
			amount,
			externalAccountID,
			merchantAccount.ID,
		)

		// Add additional metadata using the conversion package
		depositInput.Metadata = conversion.EnhanceMetadata(depositInput.Metadata, map[string]interface{}{
			"deposit_index": i + 1,
			"account_type":  "merchant",
			"timestamp":     time.Now().Unix(),
		})

		txCtx, txSpan := observability.StartSpan(merchantCtx, "MerchantDeposit")
		startTime := time.Now()

		tx, err := client.Entity.Transactions.CreateTransaction(txCtx, orgID, ledgerID, depositInput)

		duration := time.Since(startTime)
		observability.RecordSpanMetric(txCtx, "merchant_deposit_duration_ms", float64(duration.Milliseconds()))

		if err != nil {
			observability.RecordError(txCtx, err, "merchant_deposit_failed")
			txSpan.End()
			merchantSpan.End()
			return fmt.Errorf("failed to deposit funds to merchant account: %w", err)
		}

		observability.AddAttribute(txCtx, "transaction_id", tx.ID)
		observability.AddAttribute(txCtx, "amount", amount)
		txSpan.End()

		// Format the amount using the format package
		formattedAmount := format.FormatCurrency(amount, 2, "USD")
		fmt.Printf("✅ Deposit #%d: %s (ID: %s)\n", i+1, formattedAmount, tx.ID)
	}
	merchantSpan.End()

	return nil
}

// CreateDepositInput is a helper function to create a deposit transaction input
//
// Parameters:
//   - description: The description of the transaction
//   - amount: The amount of the transaction (in cents)
//   - sourceAccountID: The ID of the account to deposit from
//   - destinationAccountID: The ID of the account to deposit to
//
// Returns:
//   - *models.CreateTransactionInput: The deposit transaction input
func CreateDepositInput(description string, amount int64, sourceAccountID, destinationAccountID string) *models.CreateTransactionInput {
	// Validate inputs
	if !validation.IsValidExternalAccountID(sourceAccountID) {
		sourceAccountID = fmt.Sprintf("@external/%s", "USD") // Default fallback
	}

	// Create the transaction input with validated data
	return &models.CreateTransactionInput{
		Description: description,
		Amount:      amount,
		Scale:       2, // 2 decimal places
		AssetCode:   "USD",
		Metadata: map[string]any{
			"source":     "go-sdk-example",
			"type":       "deposit",
			"created_at": time.Now().Format(time.RFC3339),
		},
		Send: &models.SendInput{
			Asset: "USD",
			Value: amount,
			Scale: 2,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: sourceAccountID,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
							Scale: 2,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account: destinationAccountID,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
							Scale: 2,
						},
					},
				},
			},
		},
		// Generate a truly unique idempotency key using UUID
		IdempotencyKey: uuid.New().String(),
	}
}

// ExecuteSingleTransfer performs a single transfer from customer to merchant
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - merchantAccount: The merchant account model
//
// Returns:
//   - error: Any error encountered during the operation
func ExecuteSingleTransfer(ctx context.Context, client *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account) error {
	ctx, span := observability.StartSpan(ctx, "ExecuteSingleTransfer")
	defer span.End()

	fmt.Println("\nTransferring funds from customer to merchant...")

	// Validate account IDs
	if !validation.IsValidUUID(customerAccount.ID) || !validation.IsValidUUID(merchantAccount.ID) {
		err := fmt.Errorf("invalid account IDs for transfer")
		observability.RecordError(ctx, err, "invalid_account_ids")
		return err
	}

	// Validate amount
	amount := int64(1000) // $10.00
	scale := int64(2)

	if !validation.IsValidAmount(amount, scale) {
		err := fmt.Errorf("invalid amount for transfer: %d (scale %d)", amount, scale)
		observability.RecordError(ctx, err, "invalid_amount")
		return err
	}

	// Create a transfer transaction using the structure expected by the backend
	transferInput := &models.CreateTransactionInput{
		Description: "Payment for services",
		Amount:      amount,
		Scale:       scale,
		AssetCode:   "USD",
		Metadata: conversion.CreateMetadata(map[string]interface{}{
			"source":    "go-sdk-example",
			"type":      "transfer",
			"timestamp": time.Now().Unix(),
		}),
		Send: &models.SendInput{
			Asset: "USD",
			Value: amount,
			Scale: scale,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: customerAccount.ID,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
							Scale: scale,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account: merchantAccount.ID,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
							Scale: scale,
						},
					},
				},
			},
		},
		IdempotencyKey: uuid.New().String(),
	}

	// Add tracing data
	observability.AddAttribute(ctx, "transfer_amount", amount)
	observability.AddAttribute(ctx, "from_account", customerAccount.ID)
	observability.AddAttribute(ctx, "to_account", merchantAccount.ID)

	// Record the start time for performance measurement
	startTime := time.Now()

	transaction, err := client.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, transferInput)

	// Record transaction duration
	duration := time.Since(startTime)
	observability.RecordSpanMetric(ctx, "transfer_duration_ms", float64(duration.Milliseconds()))

	if err != nil {
		// Get detailed error information
		errorDetails := sdkerrors.GetErrorDetails(err)
		observability.RecordError(ctx, err, "transfer_failed",
			map[string]string{
				"error_code":  errorDetails.Code,
				"http_status": fmt.Sprintf("%d", errorDetails.HTTPStatus),
			})

		// Different approach to standardized error handling
		// First, check for specific Midaz error categories
		category := sdkerrors.GetErrorCategory(err)

		switch category {
		case sdkerrors.CategoryValidation:
			return fmt.Errorf("transfer validation failed: %w", err)
		case sdkerrors.CategoryNotFound:
			return fmt.Errorf("account not found: %w", err)
		case sdkerrors.CategoryUnprocessable:
			// Further analyze unprocessable errors
			if sdkerrors.IsInsufficientBalanceError(err) {
				return fmt.Errorf("customer account has insufficient funds: %w", err)
			}
			return fmt.Errorf("transfer could not be processed: %w", err)
		default:
			// For other errors, use the transaction error formatter
			errorMessage := sdkerrors.FormatOperationError(err, "Transfer")
			return fmt.Errorf("%s", errorMessage)
		}
	}

	if transaction.ID == "" {
		err := fmt.Errorf("transaction created but no ID was returned from the API")
		observability.RecordError(ctx, err, "empty_transaction_id")
		return err
	}

	// Record successful transaction in observability
	observability.AddAttribute(ctx, "transaction_id", transaction.ID)
	observability.AddAttribute(ctx, "transaction_status", transaction.Status)
	observability.AddEvent(ctx, "TransferCompleted", map[string]string{
		"transaction_id": transaction.ID,
		"amount":         format.FormatCurrency(transaction.Amount, transaction.Scale, transaction.AssetCode),
	})

	// Format the output using the format package
	formattedAmount := format.FormatCurrency(transaction.Amount, transaction.Scale, transaction.AssetCode)
	formattedDate := format.FormatDateTime(transaction.CreatedAt)

	fmt.Printf("✅ Transaction executed successfully\n")
	fmt.Printf("   Transaction ID: %s\n", transaction.ID)
	fmt.Printf("   Amount: %s\n", formattedAmount)
	fmt.Printf("   Created: %s\n", formattedDate)

	return nil
}

// CreateTransferInput is a helper function to create a transfer transaction input
//
// Parameters:
//   - description: The description of the transaction
//   - amount: The amount of the transaction (in cents)
//   - fromAccountID: The ID of the account to transfer from
//   - toAccountID: The ID of the account to transfer to
//   - index: The index of the transaction (for logging purposes)
//
// Returns:
//   - *models.CreateTransactionInput: The transfer transaction input
func CreateTransferInput(description string, amount int64, fromAccountID, toAccountID string, index int) *models.CreateTransactionInput {
	// Validate inputs
	if !validation.IsValidAmount(amount, 2) {
		// Default to a minimum amount if invalid
		amount = 1
	}

	// Create properly validated and structured metadata using conversion package
	metadata := conversion.CreateMetadata(map[string]interface{}{
		"source":    "go-sdk-example",
		"type":      "transfer",
		"index":     index,
		"timestamp": time.Now().Unix(),
	})

	return &models.CreateTransactionInput{
		Description: description,
		Amount:      amount,
		Scale:       2,
		AssetCode:   "USD",
		Metadata:    metadata,
		Send: &models.SendInput{
			Asset: "USD",
			Value: amount,
			Scale: 2,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: fromAccountID,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
							Scale: 2,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account: toAccountID,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
							Scale: 2,
						},
					},
				},
			},
		},
		// Generate a truly unique idempotency key using UUID
		IdempotencyKey: uuid.New().String(),
	}
}

// ExecuteMultipleTransfers performs multiple transfers between customer and merchant accounts
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - merchantAccount: The merchant account model
//
// Returns:
//   - error: Any error encountered during the operation
func ExecuteMultipleTransfers(ctx context.Context, client *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account) error {
	ctx, span := observability.StartSpan(ctx, "ExecuteMultipleTransfers")
	defer span.End()

	fmt.Println("\n🔄 Executing multiple transfers between accounts...")

	// Apply performance optimizations for batch operations
	perfOptions := performance.Options{
		BatchSize:       5, // Process in batches of 5
		UseJSONIterator: true,
	}
	performance.ApplyBatchingOptions(perfOptions)

	// Define transfer amounts and descriptions
	transfersData := []struct {
		FromAccount *models.Account
		ToAccount   *models.Account
		Amount      int64
		Description string
	}{
		// Customer to Merchant transfers
		{customerAccount, merchantAccount, 2500, "Payment for premium services"},
		{customerAccount, merchantAccount, 1750, "Subscription fee"},
		{customerAccount, merchantAccount, 3000, "Product purchase"},
		{customerAccount, merchantAccount, 500, "Small tip"},
		{customerAccount, merchantAccount, 4200, "Consulting services"},

		// Merchant to Customer transfers (refunds, rewards, etc.)
		{merchantAccount, customerAccount, 1200, "Refund for returned item"},
		{merchantAccount, customerAccount, 800, "Loyalty reward"},
		{merchantAccount, customerAccount, 350, "Cashback"},
		{merchantAccount, customerAccount, 2000, "Service credit"},
		{merchantAccount, customerAccount, 150, "Promotional bonus"},
	}

	// Record batch size in observability
	observability.AddAttribute(ctx, "transfer_count", len(transfersData))

	// Configure retry options for transfers
	retryOptions := retry.Options{
		MaxRetries:    3,
		InitialDelay:  50 * time.Millisecond,
		MaxDelay:      500 * time.Millisecond,
		BackoffFactor: 1.5,
	}

	// Execute all transfers
	for i, data := range transfersData {
		// Create a descriptive direction string
		direction := "customer → merchant"
		if data.FromAccount.ID == merchantAccount.ID {
			direction = "merchant → customer"
		}

		// Validate amount
		if !validation.IsValidAmount(data.Amount, 2) {
			err := fmt.Errorf("invalid amount for transfer #%d: %d", i+1, data.Amount)
			observability.RecordError(ctx, err, "invalid_amount")
			return err
		}

		fmt.Printf("\nTransfer #%d: %s USD (%s)\n", i+1,
			format.FormatCurrency(data.Amount, 2, "USD"), direction)
		fmt.Printf("Description: %s\n", data.Description)

		// Create context for this individual transfer
		txCtx, txSpan := observability.StartSpan(ctx, "SingleTransfer")
		txCtx = retry.WithOptionsContext(txCtx, &retryOptions)

		observability.AddAttribute(txCtx, "transfer_index", i+1)
		observability.AddAttribute(txCtx, "transfer_amount", data.Amount)
		observability.AddAttribute(txCtx, "transfer_direction", direction)

		transferInput := CreateTransferInput(data.Description, data.Amount, data.FromAccount.ID, data.ToAccount.ID, i+1)

		// Record transaction start time
		startTime := time.Now()

		tx, err := client.Entity.Transactions.CreateTransaction(txCtx, orgID, ledgerID, transferInput)

		// Record transaction duration
		duration := time.Since(startTime)
		observability.RecordSpanMetric(txCtx, "transfer_duration_ms", float64(duration.Milliseconds()))

		if err != nil {
			errorDetails := sdkerrors.GetErrorDetails(err)
			observability.RecordError(txCtx, err, "transfer_failed",
				map[string]string{
					"error_code":  errorDetails.Code,
					"http_status": fmt.Sprintf("%d", errorDetails.HTTPStatus),
				})
			txSpan.End()
			return fmt.Errorf("failed to execute transfer #%d: %w", i+1, err)
		}

		observability.AddAttribute(txCtx, "transaction_id", tx.ID)

		// Record successful event
		observability.AddEvent(txCtx, "TransferCompleted", map[string]string{
			"transaction_id": tx.ID,
			"amount":         format.FormatCurrency(data.Amount, 2, "USD"),
			"direction":      direction,
		})

		txSpan.End()

		fmt.Printf("✅ Transfer completed (ID: %s)\n", tx.ID)
	}

	return nil
}

// ExecuteWithdrawals performs withdrawals from accounts to external account
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - merchantAccount: The merchant account model
//   - externalAccountID: The external account ID
//
// Returns:
//   - error: Any error encountered during the operation
func ExecuteWithdrawals(ctx context.Context, client *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, externalAccountID string) error {
	ctx, span := observability.StartSpan(ctx, "ExecuteWithdrawals")
	defer span.End()

	fmt.Println("\n💱 Executing withdrawals to external account...")

	// Validate external account ID
	if !validation.IsValidExternalAccountID(externalAccountID) {
		err := fmt.Errorf("invalid external account ID: %s", externalAccountID)
		observability.RecordError(ctx, err, "invalid_external_account")
		return err
	}

	// Define withdrawal data
	withdrawalsData := []struct {
		FromAccount *models.Account
		Amount      int64
		Description string
	}{
		// Customer withdrawals
		{customerAccount, 1500, "Customer withdrawal to bank account"},
		{customerAccount, 3000, "Customer withdrawal to credit card"},
		{customerAccount, 750, "Customer withdrawal to payment provider"},
		{customerAccount, 5000, "Customer withdrawal to investment account"},

		// Merchant withdrawals
		{merchantAccount, 2500, "Merchant payout to bank account"},
		{merchantAccount, 10000, "Merchant bulk settlement"},
		{merchantAccount, 4500, "Merchant withdrawal to business account"},
	}

	// Record batch details in observability
	observability.AddAttribute(ctx, "withdrawal_count", len(withdrawalsData))

	// Apply performance optimizations for batch processing
	perfOptions := performance.Options{
		BatchSize:       3, // Process in batches of 3
		UseJSONIterator: true,
	}
	performance.ApplyBatchingOptions(perfOptions)

	// Execute all withdrawals
	for i, data := range withdrawalsData {
		// Create a descriptive source string
		source := "customer"
		if data.FromAccount.ID == merchantAccount.ID {
			source = "merchant"
		}

		// Validate amount
		if !validation.IsValidAmount(data.Amount, 2) {
			err := fmt.Errorf("invalid amount for withdrawal #%d: %d", i+1, data.Amount)
			observability.RecordError(ctx, err, "invalid_amount")
			return err
		}

		fmt.Printf("\nWithdrawal #%d: %s (from %s to external)\n", i+1,
			format.FormatCurrency(data.Amount, 2, "USD"), source)
		fmt.Printf("Description: %s\n", data.Description)

		// Create a span for this individual withdrawal
		txCtx, txSpan := observability.StartSpan(ctx, "SingleWithdrawal")

		observability.AddAttribute(txCtx, "withdrawal_index", i+1)
		observability.AddAttribute(txCtx, "withdrawal_amount", data.Amount)
		observability.AddAttribute(txCtx, "withdrawal_source", source)

		withdrawalInput := CreateTransferInput(
			data.Description,
			data.Amount,
			data.FromAccount.ID,
			externalAccountID,
			i+1,
		)

		// Update metadata to indicate this is a withdrawal using conversion package
		withdrawalInput.Metadata = conversion.EnhanceMetadata(withdrawalInput.Metadata, map[string]interface{}{
			"type":             "withdrawal",
			"withdrawal_index": i + 1,
			"destination":      "external",
		})

		// Record start time for performance measurement
		startTime := time.Now()

		tx, err := client.Entity.Transactions.CreateTransaction(txCtx, orgID, ledgerID, withdrawalInput)

		// Record transaction duration
		duration := time.Since(startTime)
		observability.RecordSpanMetric(txCtx, "withdrawal_duration_ms", float64(duration.Milliseconds()))

		if err != nil {
			errorDetails := sdkerrors.GetErrorDetails(err)
			observability.RecordError(txCtx, err, "withdrawal_failed",
				map[string]string{
					"error_code":  errorDetails.Code,
					"http_status": fmt.Sprintf("%d", errorDetails.HTTPStatus),
				})
			txSpan.End()
			return fmt.Errorf("failed to execute withdrawal #%d: %w", i+1, err)
		}

		observability.AddAttribute(txCtx, "transaction_id", tx.ID)

		// Record successful event
		observability.AddEvent(txCtx, "WithdrawalCompleted", map[string]string{
			"transaction_id": tx.ID,
			"amount":         format.FormatCurrency(data.Amount, 2, "USD"),
			"source":         source,
		})

		txSpan.End()

		fmt.Printf("✅ Withdrawal completed (ID: %s)\n", tx.ID)
	}

	return nil
}

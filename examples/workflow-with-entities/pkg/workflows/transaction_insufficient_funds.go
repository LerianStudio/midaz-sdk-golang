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
	"github.com/LerianStudio/midaz-sdk-golang/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation"
)

// ExecuteInsufficientFundsTransactions attempts transactions that should fail due to insufficient funds
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - merchantAccount: The merchant account model
//   - externalAccountID: The external account ID
func ExecuteInsufficientFundsTransactions(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, externalAccountID string) {
	// Create span for observability
	ctx, span := observability.StartSpan(ctx, "ExecuteInsufficientFundsTransactions")
	defer span.End()

	observability.AddAttribute(ctx, "organization_id", orgID)
	observability.AddAttribute(ctx, "ledger_id", ledgerID)

	fmt.Println("\n⚠️ Testing transactions with insufficient funds...")
	fmt.Println("Note: These transactions are expected to fail")

	// Validate accounts
	if !validation.IsValidUUID(customerAccount.ID) || !validation.IsValidUUID(merchantAccount.ID) {
		err := fmt.Errorf("invalid account IDs")
		observability.RecordError(ctx, err, "invalid_account_ids")
		fmt.Printf("❌ Error: %s\n", err.Error())
		return
	}

	// Define transactions that should fail
	insufficientFundsTests := []struct {
		Description   string
		FromAccount   *models.Account
		ToAccount     string // Can be account ID or external account
		Amount        string
		ExpectedError string
	}{
		{
			Description:   "Customer transfer exceeding balance",
			FromAccount:   customerAccount,
			ToAccount:     merchantAccount.ID,
			Amount:        "100000.00", // $100,000.00 (far exceeds balance)
			ExpectedError: "insufficient funds",
		},
		{
			Description:   "Merchant transfer exceeding balance",
			FromAccount:   merchantAccount,
			ToAccount:     customerAccount.ID,
			Amount:        "500000.00", // $500,000.00 (far exceeds balance)
			ExpectedError: "insufficient funds",
		},
		{
			Description:   "Customer withdrawal exceeding balance",
			FromAccount:   customerAccount,
			ToAccount:     externalAccountID,
			Amount:        "200000.00", // $200,000.00 (far exceeds balance)
			ExpectedError: "insufficient funds",
		},
		{
			Description:   "Merchant withdrawal exceeding balance",
			FromAccount:   merchantAccount,
			ToAccount:     externalAccountID,
			Amount:        "300000.00", // $300,000.00 (far exceeds balance)
			ExpectedError: "insufficient funds",
		},
	}

	// Record test count in observability
	observability.AddAttribute(ctx, "test_count", len(insufficientFundsTests))

	// Attempt each transaction that should fail
	for i, test := range insufficientFundsTests {
		// Create a span for each test
		testCtx, testSpan := observability.StartSpan(ctx, "InsufficientFundsTest")
		observability.AddAttribute(testCtx, "test_index", i+1)
		observability.AddAttribute(testCtx, "test_description", test.Description)
		observability.AddAttribute(testCtx, "expected_error", test.ExpectedError)
		observability.AddAttribute(testCtx, "amount", test.Amount)

		fmt.Printf("\n🔴 Test #%d: %s\n", i+1, test.Description)
		fmt.Printf("   Attempting to transfer %s USD\n", test.Amount)

		// Validate the amount - it's intentionally large so should fail, but we still validate format
		if test.Amount == "" || test.Amount == "0" {
			err := fmt.Errorf("invalid amount format: %s", test.Amount)
			observability.RecordError(testCtx, err, "invalid_amount_format")
			fmt.Printf("⚠️ Note: Amount format is invalid: %s\n", test.Amount)
		}

		// Create the transaction input with enhanced metadata using conversion package
		transferInput := CreateTransferInput(
			test.Description,
			test.Amount,
			test.FromAccount.ID,
			test.ToAccount,
			i+1,
		)

		// Add extra metadata to track these test transactions
		transferInput.Metadata = conversion.EnhanceMetadata(transferInput.Metadata, map[string]any{
			"test_type":        "insufficient_funds",
			"test_index":       i + 1,
			"expected_to_fail": true,
			"timestamp":        time.Now().Unix(),
		})

		// Record the transaction start time
		startTime := time.Now()

		// Attempt the transaction (expecting failure)
		_, err := midazClient.Entity.Transactions.CreateTransaction(testCtx, orgID, ledgerID, transferInput)

		// Record the transaction duration
		duration := time.Since(startTime)
		observability.RecordSpanMetric(testCtx, "test_duration_ms", float64(duration.Milliseconds()))

		// Check if the transaction failed as expected
		if err != nil {
			// Record detailed error information
			errorDetails := sdkerrors.GetErrorDetails(err)

			observability.RecordError(testCtx, err, "expected_transaction_error",
				map[string]string{
					"error_code":  errorDetails.Code,
					"http_status": fmt.Sprintf("%d", errorDetails.HTTPStatus),
				})

			observability.AddAttribute(testCtx, "error_category", string(sdkerrors.GetErrorCategory(err)))
			observability.AddAttribute(testCtx, "is_insufficient_balance", fmt.Sprintf("%t", sdkerrors.IsInsufficientBalanceError(err)))

			// Format the error for display using our standardized error formatter
			formattedError := sdkerrors.FormatErrorDetails(err)
			fmt.Printf("✅ Transaction failed as expected: %s\n", formattedError)

			// Demonstrate error classification using our standardized error system
			errorCategory := sdkerrors.GetErrorCategory(err)
			fmt.Printf("   Error category: %s\n", errorCategory)

			// Check error type using our standardized error functions
			if sdkerrors.IsInsufficientBalanceError(err) {
				fmt.Printf("✅ Error correctly identified as an insufficient balance error\n")

				// Record a successful test
				observability.AddEvent(testCtx, "InsufficientFundsTestPassed", map[string]string{
					"test_index":  fmt.Sprintf("%d", i+1),
					"description": test.Description,
				})
			} else if sdkerrors.IsValidationError(err) {
				fmt.Printf("ℹ️ Error identified as a validation error\n")

				// Record an unexpected error type
				observability.AddEvent(testCtx, "UnexpectedErrorType", map[string]string{
					"test_index": fmt.Sprintf("%d", i+1),
					"expected":   "insufficient_balance",
					"actual":     "validation",
				})
			} else {
				fmt.Printf("⚠️ Error is neither insufficient balance nor validation error\n")

				// Record an unexpected error type
				observability.AddEvent(testCtx, "UnexpectedErrorType", map[string]string{
					"test_index": fmt.Sprintf("%d", i+1),
					"expected":   "insufficient_balance",
					"actual":     string(errorCategory),
				})
			}

			// Get the HTTP status code that would be returned for this error
			statusCode := sdkerrors.GetErrorStatusCode(err)
			fmt.Printf("   HTTP status code: %d\n", statusCode)

			// Format the error for transaction operations
			txError := sdkerrors.FormatOperationError(err, "OverdraftTest")
			fmt.Printf("   Transaction error message: %s\n", txError)

			// Check if the error contains the expected message
			if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.ExpectedError)) {
				fmt.Printf("✅ Error message contains expected text: '%s'\n", test.ExpectedError)

				// Record that the error message matched expectations
				observability.AddAttribute(testCtx, "error_message_matched", true)
			} else {
				fmt.Printf("⚠️ Error message doesn't contain expected text: '%s'\n", test.ExpectedError)
				fmt.Printf("   Actual error: %s\n", err.Error())

				// Record that the error message didn't match expectations
				observability.AddAttribute(testCtx, "error_message_matched", false)
				observability.AddAttribute(testCtx, "actual_error_message", err.Error())
			}
		} else {
			// Record the unexpected success
			observability.RecordError(testCtx, fmt.Errorf("transaction unexpectedly succeeded"), "unexpected_success")

			fmt.Printf("❌ Transaction unexpectedly succeeded! This indicates a potential issue.\n")
		}

		testSpan.End()
	}

	fmt.Println("\n✅ Insufficient funds testing completed")

	// Create summary metrics
	observability.RecordSpanMetric(ctx, "insufficient_funds_tests_completed", float64(len(insufficientFundsTests)))
}

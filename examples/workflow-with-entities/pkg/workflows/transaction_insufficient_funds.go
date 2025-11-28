package workflows

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/conversion"
	pkgerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation"
)

// insufficientFundsTest defines a test case for insufficient funds scenarios
type insufficientFundsTest struct {
	Description   string
	FromAccount   *models.Account
	ToAccount     string
	Amount        string
	ExpectedError string
}

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
	ctx, span := observability.StartSpan(ctx, "ExecuteInsufficientFundsTransactions")
	defer span.End()

	observability.AddAttribute(ctx, "organization_id", orgID)
	observability.AddAttribute(ctx, "ledger_id", ledgerID)

	fmt.Println("\n⚠️ Testing transactions with insufficient funds...")
	fmt.Println("Note: These transactions are expected to fail")

	if !validateInsufficientFundsAccounts(ctx, customerAccount, merchantAccount) {
		return
	}

	tests := buildInsufficientFundsTests(customerAccount, merchantAccount, externalAccountID)
	observability.AddAttribute(ctx, "test_count", len(tests))

	for i, test := range tests {
		runInsufficientFundsTest(ctx, midazClient, orgID, ledgerID, test, i+1)
	}

	fmt.Println("\n✅ Insufficient funds testing completed")
	observability.RecordSpanMetric(ctx, "insufficient_funds_tests_completed", float64(len(tests)))
}

func validateInsufficientFundsAccounts(ctx context.Context, customerAccount, merchantAccount *models.Account) bool {
	if !validation.IsValidUUID(customerAccount.ID) || !validation.IsValidUUID(merchantAccount.ID) {
		err := errors.New("invalid account IDs")
		observability.RecordError(ctx, err, "invalid_account_ids")
		fmt.Printf("❌ Error: %s\n", err.Error())

		return false
	}

	return true
}

func buildInsufficientFundsTests(customerAccount, merchantAccount *models.Account, externalAccountID string) []insufficientFundsTest {
	return []insufficientFundsTest{
		{
			Description:   "Customer transfer exceeding balance",
			FromAccount:   customerAccount,
			ToAccount:     merchantAccount.ID,
			Amount:        "100000.00",
			ExpectedError: "insufficient funds",
		},
		{
			Description:   "Merchant transfer exceeding balance",
			FromAccount:   merchantAccount,
			ToAccount:     customerAccount.ID,
			Amount:        "500000.00",
			ExpectedError: "insufficient funds",
		},
		{
			Description:   "Customer withdrawal exceeding balance",
			FromAccount:   customerAccount,
			ToAccount:     externalAccountID,
			Amount:        "200000.00",
			ExpectedError: "insufficient funds",
		},
		{
			Description:   "Merchant withdrawal exceeding balance",
			FromAccount:   merchantAccount,
			ToAccount:     externalAccountID,
			Amount:        "300000.00",
			ExpectedError: "insufficient funds",
		},
	}
}

func runInsufficientFundsTest(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, test insufficientFundsTest, testIndex int) {
	testCtx, testSpan := observability.StartSpan(ctx, "InsufficientFundsTest")
	defer testSpan.End()

	recordTestAttributes(testCtx, test, testIndex)
	printTestHeader(test, testIndex)
	validateTestAmount(testCtx, test.Amount)

	transferInput := createInsufficientFundsTransferInput(test, testIndex)

	startTime := time.Now()
	_, err := midazClient.Entity.Transactions.CreateTransaction(testCtx, orgID, ledgerID, transferInput)
	duration := time.Since(startTime)

	observability.RecordSpanMetric(testCtx, "test_duration_ms", float64(duration.Milliseconds()))

	if err != nil {
		handleExpectedError(testCtx, err, test, testIndex)
	} else {
		handleUnexpectedSuccess(testCtx)
	}
}

func recordTestAttributes(ctx context.Context, test insufficientFundsTest, testIndex int) {
	observability.AddAttribute(ctx, "test_index", testIndex)
	observability.AddAttribute(ctx, "test_description", test.Description)
	observability.AddAttribute(ctx, "expected_error", test.ExpectedError)
	observability.AddAttribute(ctx, "amount", test.Amount)
}

func printTestHeader(test insufficientFundsTest, testIndex int) {
	fmt.Printf("\n🔴 Test #%d: %s\n", testIndex, test.Description)
	fmt.Printf("   Attempting to transfer %s USD\n", test.Amount)
}

func validateTestAmount(ctx context.Context, amount string) {
	if amount == "" || amount == "0" {
		err := fmt.Errorf("invalid amount format: %s", amount)
		observability.RecordError(ctx, err, "invalid_amount_format")
		fmt.Printf("⚠️ Note: Amount format is invalid: %s\n", amount)
	}
}

func createInsufficientFundsTransferInput(test insufficientFundsTest, testIndex int) *models.CreateTransactionInput {
	transferInput := CreateTransferInput(
		test.Description,
		test.Amount,
		test.FromAccount.ID,
		test.ToAccount,
		testIndex,
	)

	transferInput.Metadata = conversion.EnhanceMetadata(transferInput.Metadata, map[string]any{
		"test_type":        "insufficient_funds",
		"test_index":       testIndex,
		"expected_to_fail": true,
		"timestamp":        time.Now().Unix(),
	})

	return transferInput
}

func handleExpectedError(ctx context.Context, err error, test insufficientFundsTest, testIndex int) {
	recordErrorDetails(ctx, err)
	printErrorClassification(err)
	classifyAndRecordErrorType(ctx, err, test, testIndex)
	printErrorStatusAndMessage(err)
	checkExpectedErrorMessage(ctx, err, test.ExpectedError)
}

func recordErrorDetails(ctx context.Context, err error) {
	errorDetails := pkgerrors.GetErrorDetails(err)

	observability.RecordError(ctx, err, "expected_transaction_error",
		map[string]string{
			"error_code":  errorDetails.Code,
			"http_status": fmt.Sprintf("%d", errorDetails.HTTPStatus),
		})

	observability.AddAttribute(ctx, "error_category", string(pkgerrors.GetErrorCategory(err)))
	observability.AddAttribute(ctx, "is_insufficient_balance", fmt.Sprintf("%t", pkgerrors.IsInsufficientBalanceError(err)))
}

func printErrorClassification(err error) {
	formattedError := pkgerrors.FormatErrorDetails(err)
	fmt.Printf("✅ Transaction failed as expected: %s\n", formattedError)

	errorCategory := pkgerrors.GetErrorCategory(err)
	fmt.Printf("   Error category: %s\n", errorCategory)
}

func classifyAndRecordErrorType(ctx context.Context, err error, test insufficientFundsTest, testIndex int) {
	if pkgerrors.IsInsufficientBalanceError(err) {
		fmt.Printf("✅ Error correctly identified as an insufficient balance error\n")
		observability.AddEvent(ctx, "InsufficientFundsTestPassed", map[string]string{
			"test_index":  fmt.Sprintf("%d", testIndex),
			"description": test.Description,
		})
	} else if pkgerrors.IsValidationError(err) {
		fmt.Printf("ℹ️ Error identified as a validation error\n")
		observability.AddEvent(ctx, "UnexpectedErrorType", map[string]string{
			"test_index": fmt.Sprintf("%d", testIndex),
			"expected":   "insufficient_balance",
			"actual":     "validation",
		})
	} else {
		fmt.Printf("⚠️ Error is neither insufficient balance nor validation error\n")
		observability.AddEvent(ctx, "UnexpectedErrorType", map[string]string{
			"test_index": fmt.Sprintf("%d", testIndex),
			"expected":   "insufficient_balance",
			"actual":     string(pkgerrors.GetErrorCategory(err)),
		})
	}
}

func printErrorStatusAndMessage(err error) {
	statusCode := pkgerrors.GetErrorStatusCode(err)
	fmt.Printf("   HTTP status code: %d\n", statusCode)

	txError := pkgerrors.FormatOperationError(err, "OverdraftTest")
	fmt.Printf("   Transaction error message: %s\n", txError)
}

func checkExpectedErrorMessage(ctx context.Context, err error, expectedError string) {
	if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(expectedError)) {
		fmt.Printf("✅ Error message contains expected text: '%s'\n", expectedError)
		observability.AddAttribute(ctx, "error_message_matched", true)
	} else {
		fmt.Printf("⚠️ Error message doesn't contain expected text: '%s'\n", expectedError)
		fmt.Printf("   Actual error: %s\n", err.Error())
		observability.AddAttribute(ctx, "error_message_matched", false)
		observability.AddAttribute(ctx, "actual_error_message", err.Error())
	}
}

func handleUnexpectedSuccess(ctx context.Context) {
	observability.RecordError(ctx, errors.New("transaction unexpectedly succeeded"), "unexpected_success")
	fmt.Printf("❌ Transaction unexpectedly succeeded! This indicates a potential issue.\n")
}

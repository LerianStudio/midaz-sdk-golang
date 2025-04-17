package workflows

import (
	"context"
	"fmt"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang"
	ourEntities "github.com/LerianStudio/midaz-sdk-golang/examples/workflow-with-entities/pkg/entities"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/format"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/observability"
)

// DemonstrateTransactionHelpers showcases the transaction helpers in the SDK
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - merchantAccount: The merchant account model
//   - dummyOneAccount: The dummy 1 account model
//   - dummyTwoAccount: The dummy 2 account model
//
// Returns:
//   - error: Any error encountered during the operation
func DemonstrateTransactionHelpers(ctx context.Context, client *client.Client, orgID, ledgerID string, customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount *models.Account) error {
	// Create a span for observability
	ctx, span := observability.StartSpan(ctx, "DemonstrateTransactionHelpers")
	defer span.End()

	// Add attributes for tracing
	observability.AddAttribute(ctx, "organization_id", orgID)
	observability.AddAttribute(ctx, "ledger_id", ledgerID)

	fmt.Println("\n\n🚀 STEP 6: TRANSACTION HELPERS DEMONSTRATION")
	fmt.Println(strings.Repeat("=", 50))

	// 1. Demonstrate transfer using helpers
	fmt.Println("\n🔄 Demonstrating transfer using helpers...")
	transferCtx, transferSpan := observability.StartSpan(ctx, "TransferWithHelper")

	tx, err := ourEntities.ExecuteTransferWithHelper(
		transferCtx,
		client,
		orgID,
		ledgerID,
		customerAccount.ID,
		merchantAccount.ID,
		1500, // $15.00
		2,    // 2 decimal places
		"USD",
	)

	transferSpan.End()

	if err != nil {
		observability.RecordError(ctx, err, "transfer_helper_failed")
		return err
	}

	formattedAmount := format.FormatCurrency(tx.Amount, tx.Scale, tx.AssetCode)
	fmt.Printf("✅ Transfer executed successfully with helper\n")
	fmt.Printf("   Transaction ID: %s\n", tx.ID)
	fmt.Printf("   Amount: %s\n", formattedAmount)

	// 2. Demonstrate deposit using helpers
	fmt.Println("\n📥 Demonstrating deposit using helpers...")
	depositCtx, depositSpan := observability.StartSpan(ctx, "DepositWithHelper")

	depositTx, err := ourEntities.ExecuteDepositWithHelper(
		depositCtx,
		client,
		orgID,
		ledgerID,
		customerAccount.ID,
		2000, // $20.00
		2,    // 2 decimal places
		"USD",
	)

	depositSpan.End()

	if err != nil {
		observability.RecordError(ctx, err, "deposit_helper_failed")
		return err
	}

	formattedDepositAmount := format.FormatCurrency(depositTx.Amount, depositTx.Scale, depositTx.AssetCode)
	fmt.Printf("✅ Deposit executed successfully with helper\n")
	fmt.Printf("   Transaction ID: %s\n", depositTx.ID)
	fmt.Printf("   Amount: %s\n", formattedDepositAmount)

	// 3. Demonstrate withdrawal using helpers
	fmt.Println("\n📤 Demonstrating withdrawal using helpers...")
	withdrawalCtx, withdrawalSpan := observability.StartSpan(ctx, "WithdrawalWithHelper")

	withdrawalTx, err := ourEntities.ExecuteWithdrawalWithHelper(
		withdrawalCtx,
		client,
		orgID,
		ledgerID,
		merchantAccount.ID,
		3000, // $30.00
		2,    // 2 decimal places
		"USD",
	)

	withdrawalSpan.End()

	if err != nil {
		observability.RecordError(ctx, err, "withdrawal_helper_failed")
		return err
	}

	formattedWithdrawalAmount := format.FormatCurrency(withdrawalTx.Amount, withdrawalTx.Scale, withdrawalTx.AssetCode)
	fmt.Printf("✅ Withdrawal executed successfully with helper\n")
	fmt.Printf("   Transaction ID: %s\n", withdrawalTx.ID)
	fmt.Printf("   Amount: %s\n", formattedWithdrawalAmount)

	// 4. Demonstrate multi-account transfer
	fmt.Println("\n🔄 Demonstrating multi-account transfer...")
	multiCtx, multiSpan := observability.StartSpan(ctx, "MultiAccountTransferWithHelper")

	// Create source and destination accounts map
	sourceAccounts := map[string]int64{
		customerAccount.ID: 1000, // $10.00
		merchantAccount.ID: 2000, // $20.00
	}

	destAccounts := map[string]int64{
		dummyOneAccount.ID: 750,  // $7.50
		dummyTwoAccount.ID: 2250, // $22.50
	}

	multiTx, err := ourEntities.ExecuteMultiAccountTransferWithHelper(
		multiCtx,
		client,
		orgID,
		ledgerID,
		sourceAccounts,
		destAccounts,
		3000, // $30.00 total
		2,    // 2 decimal places
		"USD",
	)

	multiSpan.End()

	if err != nil {
		observability.RecordError(ctx, err, "multi_account_transfer_helper_failed")
		return err
	}

	formattedMultiAmount := format.FormatCurrency(multiTx.Amount, multiTx.Scale, multiTx.AssetCode)
	fmt.Printf("✅ Multi-account transfer executed successfully with helper\n")
	fmt.Printf("   Transaction ID: %s\n", multiTx.ID)
	fmt.Printf("   Amount: %s\n", formattedMultiAmount)

	// 5. Demonstrate batch transactions
	fmt.Println("\n📦 Demonstrating batch transactions...")
	batchCtx, batchSpan := observability.StartSpan(ctx, "BatchTransactionsWithHelper")

	// Create a set of transaction inputs for the batch
	batchInputs := make([]*models.CreateTransactionInput, 0, 10)

	// Add 5 small transfers from customer to merchant
	for i := 1; i <= 5; i++ {
		amount := int64(i * 100) // $1.00, $2.00, etc.
		input := &models.CreateTransactionInput{
			Description: fmt.Sprintf("Batch transfer #%d", i),
			Amount:      amount,
			Scale:       2,
			AssetCode:   "USD",
			Metadata: map[string]any{
				"source": "go-sdk-example",
				"type":   "batch-transfer",
				"index":  i,
			},
			Send: &models.SendInput{
				Asset: "USD",
				Value: amount,
				Scale: 2,
				Source: &models.SourceInput{
					From: []models.FromToInput{
						{
							Account: customerAccount.ID,
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
							Account: merchantAccount.ID,
							Amount: models.AmountInput{
								Asset: "USD",
								Value: amount,
								Scale: 2,
							},
						},
					},
				},
			},
		}
		batchInputs = append(batchInputs, input)
	}

	// Execute the batch
	_, summary, err := ourEntities.ExecuteBatchTransactionsWithHelper(
		batchCtx,
		client,
		orgID,
		ledgerID,
		batchInputs,
	)

	batchSpan.End()

	if err != nil {
		observability.RecordError(ctx, err, "batch_transactions_helper_failed")
		return err
	}

	// Print batch summary
	fmt.Printf("\n\n✅ Batch processing completed\n")
	fmt.Printf("   Total Transactions: %d\n", summary.TotalTransactions)
	fmt.Printf("   Success Count: %d\n", summary.SuccessCount)
	fmt.Printf("   Error Count: %d\n", summary.ErrorCount)
	fmt.Printf("   Success Rate: %.2f%%\n", summary.SuccessRate)
	fmt.Printf("   Average Duration: %v\n", summary.AverageDuration)
	fmt.Printf("   Transactions Per Second: %.2f\n", summary.TransactionsPerSecond)

	// Record success in observability
	observability.AddEvent(ctx, "TransactionHelpersDemonstrated", nil)

	fmt.Println("\n🎉 All transaction helpers demonstrated successfully!")
	return nil
}

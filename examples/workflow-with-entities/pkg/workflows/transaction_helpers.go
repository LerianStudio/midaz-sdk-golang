package workflows

import (
	"context"
	"fmt"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	ourEntities "github.com/LerianStudio/midaz-sdk-golang/v2/examples/workflow-with-entities/pkg/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
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
func DemonstrateTransactionHelpers(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount *models.Account) error {
	ctx, span := observability.StartSpan(ctx, "DemonstrateTransactionHelpers")
	defer span.End()

	observability.AddAttribute(ctx, "organization_id", orgID)
	observability.AddAttribute(ctx, "ledger_id", ledgerID)

	fmt.Println("\n\n🚀 STEP 6: TRANSACTION HELPERS DEMONSTRATION")
	fmt.Println(strings.Repeat("=", 50))

	if err := demonstrateTransferHelper(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount); err != nil {
		return err
	}

	if err := demonstrateDepositHelper(ctx, midazClient, orgID, ledgerID, customerAccount); err != nil {
		return err
	}

	demonstrateWithdrawalHelper(ctx, midazClient, orgID, ledgerID, customerAccount)

	if err := demonstrateMultiAccountTransfer(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount); err != nil {
		return err
	}

	demonstrateBatchTransactions(ctx, customerAccount, merchantAccount)

	observability.AddEvent(ctx, "TransactionHelpersDemonstrated", nil)
	fmt.Println("\n🎉 All transaction helpers demonstrated successfully!")

	return nil
}

func demonstrateTransferHelper(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account) error {
	fmt.Println("\n🔄 Demonstrating transfer using helpers...")

	transferCtx, transferSpan := observability.StartSpan(ctx, "TransferWithHelper")
	tx, err := ourEntities.ExecuteTransferWithHelper(transferCtx, midazClient, orgID, ledgerID, customerAccount.ID, merchantAccount.ID, "15.00", "USD")
	transferSpan.End()

	if err != nil {
		observability.RecordError(ctx, err, "transfer_helper_failed")
		return err
	}

	printTransactionSuccess("Transfer", tx)

	return nil
}

func demonstrateDepositHelper(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount *models.Account) error {
	fmt.Println("\n📥 Demonstrating deposit using helpers...")

	depositCtx, depositSpan := observability.StartSpan(ctx, "DepositWithHelper")
	depositTx, err := ourEntities.ExecuteDepositWithHelper(depositCtx, midazClient, orgID, ledgerID, customerAccount.ID, "20.00", "USD")
	depositSpan.End()

	if err != nil {
		observability.RecordError(ctx, err, "deposit_helper_failed")
		return err
	}

	printTransactionSuccess("Deposit", depositTx)

	return nil
}

func demonstrateWithdrawalHelper(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount *models.Account) {
	fmt.Println("\n📤 Demonstrating withdrawal using helpers...")

	withdrawalCtx, withdrawalSpan := observability.StartSpan(ctx, "WithdrawalWithHelper")
	withdrawalTx, err := ourEntities.ExecuteWithdrawalWithHelper(withdrawalCtx, midazClient, orgID, ledgerID, customerAccount.ID, "5.00", "USD")
	withdrawalSpan.End()

	if err != nil {
		fmt.Printf("⚠️  Withdrawal helper failed (likely insufficient funds after previous tests)\n")
		fmt.Printf("   Error: %v\n", err)
		fmt.Printf("   Note: This is expected if account balance is low after extensive testing\n")
		fmt.Printf("✅ Transaction helpers demonstration completed with expected limitations\n\n")
		observability.RecordError(ctx, err, "withdrawal_helper_failed_expected")

		return
	}

	printTransactionSuccess("Withdrawal", withdrawalTx)
}

func demonstrateMultiAccountTransfer(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount, dummyOneAccount, dummyTwoAccount *models.Account) error {
	fmt.Println("\n🔄 Demonstrating multi-account transfer...")

	multiCtx, multiSpan := observability.StartSpan(ctx, "MultiAccountTransferWithHelper")

	sourceAccounts := map[string]string{
		customerAccount.ID: "10.00",
		merchantAccount.ID: "20.00",
	}
	destAccounts := map[string]string{
		dummyOneAccount.ID: "7.50",
		dummyTwoAccount.ID: "22.50",
	}

	multiTx, err := ourEntities.ExecuteMultiAccountTransferWithHelper(multiCtx, midazClient, orgID, ledgerID, sourceAccounts, destAccounts, "30.00", "USD")
	multiSpan.End()

	if err != nil {
		observability.RecordError(ctx, err, "multi_account_transfer_helper_failed")
		return err
	}

	printTransactionSuccess("Multi-account transfer", multiTx)

	return nil
}

func demonstrateBatchTransactions(ctx context.Context, customerAccount, merchantAccount *models.Account) {
	fmt.Println("\n📦 Demonstrating batch transactions...")

	_, batchSpan := observability.StartSpan(ctx, "BatchTransactionsWithHelper")
	batchInputs := buildBatchTransactionInputs(customerAccount, merchantAccount)
	batchSpan.End()

	fmt.Printf("\n\n📋 Batch transactions prepared (not executed - batch feature not yet implemented)\n")
	fmt.Printf("   Total Transactions: %d\n", len(batchInputs))
	fmt.Printf("   This feature will be implemented in future versions\n")
}

func buildBatchTransactionInputs(customerAccount, merchantAccount *models.Account) []*models.CreateTransactionInput {
	batchInputs := make([]*models.CreateTransactionInput, 0, 5)

	for i := 1; i <= 5; i++ {
		amount := fmt.Sprintf("%d.00", i)
		input := &models.CreateTransactionInput{
			ChartOfAccountsGroupName: "default_chart_group",
			Description:              fmt.Sprintf("Batch transfer #%d", i),
			Amount:                   amount,
			AssetCode:                "USD",
			Metadata: map[string]any{
				"source": "go-sdk-example",
				"type":   "batch-transfer",
				"index":  i,
			},
			Send: &models.SendInput{
				Asset: "USD",
				Value: amount,
				Source: &models.SourceInput{
					From: []models.FromToInput{
						{Account: customerAccount.ID, Amount: models.AmountInput{Asset: "USD", Value: amount}},
					},
				},
				Distribute: &models.DistributeInput{
					To: []models.FromToInput{
						{Account: merchantAccount.ID, Amount: models.AmountInput{Asset: "USD", Value: amount}},
					},
				},
			},
		}
		batchInputs = append(batchInputs, input)
	}

	return batchInputs
}

func printTransactionSuccess(txType string, tx *models.Transaction) {
	formattedAmount := fmt.Sprintf("%s %s", tx.Amount, tx.AssetCode)
	fmt.Printf("✅ %s executed successfully with helper\n", txType)
	fmt.Printf("   Transaction ID: %s\n", tx.ID)
	fmt.Printf("   Amount: %s\n", formattedAmount)
}

package workflows

import (
	"context"
	"fmt"
	"strconv"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/concurrent"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/format"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/performance"
	"github.com/google/uuid"
)

// ExecuteTransactions executes various transactions between accounts
func ExecuteTransactions(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account) error {
	ctx, span := observability.StartSpan(ctx, "ExecuteTransactions")
	defer span.End()

	fmt.Println("\n\n💸 STEP 5: TRANSACTION EXECUTION")
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")

	// Get external account ID
	externalAccountID := "@external/USD"

	// Execute initial deposit
	fmt.Println("\n📥 Initial deposit...")
	if err := executeInitialDeposit(ctx, midazClient, orgID, ledgerID, customerAccount, externalAccountID); err != nil {
		return fmt.Errorf("initial deposit failed: %w", err)
	}

	// Execute transfer from customer to merchant
	fmt.Println("\n🔄 Transfer from customer to merchant...")
	if err := executeTransfer(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount); err != nil {
		return fmt.Errorf("transfer failed: %w", err)
	}

	fmt.Println("\n💰 All transactions completed successfully!")
	return nil
}

// executeInitialDeposit performs initial deposit from external account
func executeInitialDeposit(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount *models.Account, externalAccountID string) error {
	amount := "5000.00"

	input := &models.CreateTransactionInput{
		ChartOfAccountsGroupName: "external-deposits",
		Description:              "Initial deposit from external account",
		Amount:                   amount,
		AssetCode:                "USD",
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "deposit",
		},
		Send: &models.SendInput{
			Asset: "USD",
			Value: amount,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account:      externalAccountID,
						AccountAlias: externalAccountID,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account:      *customerAccount.Alias,
						AccountAlias: *customerAccount.Alias,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
						},
					},
				},
			},
		},
		IdempotencyKey: uuid.New().String(),
	}

	tx, err := midazClient.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
	if err != nil {
		return fmt.Errorf("failed to create deposit transaction: %w", err)
	}

	// Parse amount for formatting
	amountFloat, _ := strconv.ParseFloat(tx.Amount, 64)
	formattedAmount := format.FormatCurrency(int64(amountFloat*100), 2, tx.AssetCode)

	fmt.Printf("✅ Deposit completed: %s (ID: %s)\n", formattedAmount, tx.ID)
	return nil
}

// executeTransfer performs transfer between two accounts
func executeTransfer(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account) error {
	amount := "10.00"

	input := &models.CreateTransactionInput{
		ChartOfAccountsGroupName: "transfer-transactions",
		Description:              "Payment for services",
		Amount:                   amount,
		AssetCode:                "USD",
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "transfer",
		},
		Send: &models.SendInput{
			Asset: "USD",
			Value: amount,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account:      *customerAccount.Alias,
						AccountAlias: *customerAccount.Alias,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account:      *merchantAccount.Alias,
						AccountAlias: *merchantAccount.Alias,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
						},
					},
				},
			},
		},
		IdempotencyKey: uuid.New().String(),
	}

	tx, err := midazClient.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
	if err != nil {
		return fmt.Errorf("failed to create transfer transaction: %w", err)
	}

	// Parse amount for formatting
	amountFloat, _ := strconv.ParseFloat(tx.Amount, 64)
	formattedAmount := format.FormatCurrency(int64(amountFloat*100), 2, tx.AssetCode)

	fmt.Printf("✅ Transfer completed: %s (ID: %s)\n", formattedAmount, tx.ID)
	return nil
}

// ExecuteMultipleDeposits - simplified placeholder
func ExecuteMultipleDeposits(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, externalAccountID string) error {
	fmt.Println("\n📥 Multiple deposits (simplified)")
	return nil
}

// ExecuteSingleTransfer - simplified placeholder  
func ExecuteSingleTransfer(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account) error {
	fmt.Println("\n🔄 Single transfer (simplified)")
	return executeTransfer(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount)
}

// ExecuteMultipleTransfers - simplified placeholder
func ExecuteMultipleTransfers(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account) error {
	fmt.Println("\n🔄 Multiple transfers (simplified)")
	return nil  
}

// ExecuteWithdrawals - simplified placeholder
func ExecuteWithdrawals(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, externalAccountID string) error {
	fmt.Println("\n💱 Withdrawals (simplified)")
	return nil
}

// ExecuteTransactionsWithRoutes executes transactions using routes
func ExecuteTransactionsWithRoutes(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, sourceOperationRoute, destinationOperationRoute *models.OperationRoute, paymentTransactionRoute, refundTransactionRoute *models.TransactionRoute) error {
	fmt.Println("\n🔀 Executing transactions with routes")
	
	// Get external account ID
	externalAccountID := "@external/USD"
	
	// First do initial deposit using payment transaction route
	fmt.Println("📥 Initial deposit with routes...")
	if err := executeInitialDepositWithRoutes(ctx, midazClient, orgID, ledgerID, customerAccount, externalAccountID, sourceOperationRoute, destinationOperationRoute, paymentTransactionRoute); err != nil {
		return fmt.Errorf("initial deposit failed: %w", err)
	}
	
	// Then do transfer using payment transaction route
	fmt.Println("🔄 Transfer with routes...")
	if err := executeTransferWithRoutes(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, sourceOperationRoute, destinationOperationRoute, paymentTransactionRoute); err != nil {
		return fmt.Errorf("transfer failed: %w", err)
	}

	// Demonstrate parallel transactions with routes
	fmt.Println("🚀 Executing parallel transactions with routes...")
	if err := executeParallelTransactionsWithRoutes(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, sourceOperationRoute, destinationOperationRoute, paymentTransactionRoute); err != nil {
		return fmt.Errorf("parallel transactions failed: %w", err)
	}

	// Demonstrate high-TPS optimized transactions
	fmt.Println("⚡ Executing high-TPS optimized transactions...")
	return executeHighTPSTransactions(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, sourceOperationRoute, destinationOperationRoute, paymentTransactionRoute)
}

// executeInitialDepositWithRoutes performs initial deposit using transaction and operation routes
func executeInitialDepositWithRoutes(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount *models.Account, externalAccountID string, sourceOperationRoute, destinationOperationRoute *models.OperationRoute, transactionRoute *models.TransactionRoute) error {
	amount := "5000.00"

	input := &models.CreateTransactionInput{
		ChartOfAccountsGroupName: "external-deposits",
		Description:              "Initial deposit from external account using routes",
		Amount:                   amount,
		AssetCode:                "USD",
		Route:                    transactionRoute.ID,
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "deposit",
			"useRoutes": true,
		},
		Send: &models.SendInput{
			Asset: "USD",
			Value: amount,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account:      externalAccountID,
						AccountAlias: externalAccountID,
						Route:        sourceOperationRoute.ID,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account:      *customerAccount.Alias,
						AccountAlias: *customerAccount.Alias,
						Route:        destinationOperationRoute.ID,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
						},
					},
				},
			},
		},
		IdempotencyKey: uuid.New().String(),
	}

	// Add transaction route if available
	if transactionRoute != nil {
		input.Metadata["transactionRouteID"] = transactionRoute.ID
		input.Metadata["transactionRouteTitle"] = transactionRoute.Title
	}

	tx, err := midazClient.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
	if err != nil {
		return fmt.Errorf("failed to create deposit transaction with routes: %w", err)
	}

	// Parse amount for formatting
	amountFloat, _ := strconv.ParseFloat(tx.Amount, 64)
	formattedAmount := format.FormatCurrency(int64(amountFloat*100), 2, tx.AssetCode)

	fmt.Printf("✅ Deposit with routes completed: %s (ID: %s)\n", formattedAmount, tx.ID)
	if sourceOperationRoute != nil && destinationOperationRoute != nil {
		fmt.Printf("   📍 Used routes: %s → %s\n", sourceOperationRoute.Title, destinationOperationRoute.Title)
	}
	if transactionRoute != nil {
		fmt.Printf("   🗺️  Transaction Route: %s (%s)\n", transactionRoute.Title, transactionRoute.ID)
	}
	return nil
}

// executeTransferWithRoutes performs transfer using transaction and operation routes
func executeTransferWithRoutes(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, sourceOperationRoute, destinationOperationRoute *models.OperationRoute, transactionRoute *models.TransactionRoute) error {
	amount := "10.00"

	input := &models.CreateTransactionInput{
		ChartOfAccountsGroupName: "transfer-transactions",
		Description:              "Payment for services using routes",
		Amount:                   amount,
		AssetCode:                "USD",
		Route:                    transactionRoute.ID,
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "transfer",
			"useRoutes": true,
		},
		Send: &models.SendInput{
			Asset: "USD",
			Value: amount,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account:      *customerAccount.Alias,
						AccountAlias: *customerAccount.Alias,
						Route:        destinationOperationRoute.ID, // Customer account uses destination route
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account:      *merchantAccount.Alias,
						AccountAlias: *merchantAccount.Alias,
						Route:        destinationOperationRoute.ID, // Merchant account also uses destination route
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
						},
					},
				},
			},
		},
		IdempotencyKey: uuid.New().String(),
	}

	// Add transaction route if available
	if transactionRoute != nil {
		input.Metadata["transactionRouteID"] = transactionRoute.ID
		input.Metadata["transactionRouteTitle"] = transactionRoute.Title
	}

	tx, err := midazClient.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
	if err != nil {
		return fmt.Errorf("failed to create transfer transaction with routes: %w", err)
	}

	// Parse amount for formatting
	amountFloat, _ := strconv.ParseFloat(tx.Amount, 64)
	formattedAmount := format.FormatCurrency(int64(amountFloat*100), 2, tx.AssetCode)

	fmt.Printf("✅ Transfer with routes completed: %s (ID: %s)\n", formattedAmount, tx.ID)
	if sourceOperationRoute != nil && destinationOperationRoute != nil {
		fmt.Printf("   📍 Used operation routes: %s → %s\n", sourceOperationRoute.Title, destinationOperationRoute.Title)
	}
	if transactionRoute != nil {
		fmt.Printf("   🗺️  Transaction Route: %s (%s)\n", transactionRoute.Title, transactionRoute.ID)
	}
	return nil
}

// CreateTransferInput creates a transfer transaction input
func CreateTransferInput(description string, amount string, fromAccountID, toAccountID string, index int) *models.CreateTransactionInput {
	return &models.CreateTransactionInput{
		ChartOfAccountsGroupName: "transfer-transactions",
		Description:              description,
		Amount:                   amount,
		AssetCode:                "USD",
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "transfer",
			"index":  index,
		},
		Send: &models.SendInput{
			Asset: "USD",
			Value: amount,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account:      fromAccountID,
						AccountAlias: fromAccountID,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account:      toAccountID,
						AccountAlias: toAccountID,
						Amount: models.AmountInput{
							Asset: "USD",
							Value: amount,
						},
					},
				},
			},
		},
		IdempotencyKey: uuid.New().String(),
	}
}

// executeParallelTransactionsWithRoutes demonstrates parallel transaction processing with routes
func executeParallelTransactionsWithRoutes(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, sourceOperationRoute, destinationOperationRoute *models.OperationRoute, transactionRoute *models.TransactionRoute) error {
	ctx, span := observability.StartSpan(ctx, "executeParallelTransactionsWithRoutes")
	defer span.End()

	// Create 5 parallel transfer transactions
	transactionCount := 5
	amounts := []string{"1.00", "2.00", "3.00", "4.00", "5.00"}
	
	fmt.Printf("   Creating %d parallel transactions with routes...\n", transactionCount)
	
	// Create transaction indices for parallel processing
	indices := make([]int, transactionCount)
	for i := range indices {
		indices[i] = i
	}
	
	// Define the transaction processing function
	processTransaction := func(ctx context.Context, index int) (*models.Transaction, error) {
		txCtx, txSpan := observability.StartSpan(ctx, "ProcessParallelTransaction")
		defer txSpan.End()
		
		amount := amounts[index]
		
		input := &models.CreateTransactionInput{
			ChartOfAccountsGroupName: "parallel-transfers",
			Description:              fmt.Sprintf("Parallel transfer #%d with routes", index+1),
			Amount:                   amount,
			AssetCode:                "USD",
			Route:                    transactionRoute.ID,
			Metadata: map[string]any{
				"source": "go-sdk-example-parallel",
				"type":   "parallel_transfer",
				"index":  index + 1,
				"useRoutes": true,
			},
			Send: &models.SendInput{
				Asset: "USD",
				Value: amount,
				Source: &models.SourceInput{
					From: []models.FromToInput{
						{
							Account:      *customerAccount.Alias,
							AccountAlias: *customerAccount.Alias,
							Route:        destinationOperationRoute.ID, // Customer uses destination route
							Amount: models.AmountInput{
								Asset: "USD",
								Value: amount,
							},
						},
					},
				},
				Distribute: &models.DistributeInput{
					To: []models.FromToInput{
						{
							Account:      *merchantAccount.Alias,
							AccountAlias: *merchantAccount.Alias,
							Route:        destinationOperationRoute.ID, // Merchant also uses destination route
							Amount: models.AmountInput{
								Asset: "USD",
								Value: amount,
							},
						},
					},
				},
			},
			IdempotencyKey: uuid.New().String(),
		}
		
		// Execute the transaction
		tx, err := midazClient.Entity.Transactions.CreateTransaction(txCtx, orgID, ledgerID, input)
		if err != nil {
			return nil, fmt.Errorf("failed to create parallel transaction #%d: %w", index+1, err)
		}
		
		return tx, nil
	}
	
	// Record start time for performance measurement
	startTime := time.Now()
	
	// Execute transactions in parallel using WorkerPool
	results := concurrent.WorkerPool(
		ctx,
		indices,
		processTransaction,
		concurrent.WithWorkers(3),         // Use 3 concurrent workers
		concurrent.WithBufferSize(transactionCount), // Buffer all transactions
		concurrent.WithUnorderedResults(), // Process in any order for better performance
	)
	
	// Calculate execution time
	duration := time.Since(startTime)
	
	// Process results
	successCount := 0
	var firstError error
	
	for i, result := range results {
		if result.Error != nil {
			if firstError == nil {
				firstError = result.Error
			}
			fmt.Printf("   ❌ Transaction #%d failed: %v\n", i+1, result.Error)
		} else {
			successCount++
			// Parse amount for formatting
			amountFloat, _ := strconv.ParseFloat(result.Value.Amount, 64)
			formattedAmount := format.FormatCurrency(int64(amountFloat*100), 2, result.Value.AssetCode)
			fmt.Printf("   ✅ Transaction #%d completed: %s (ID: %s)\n", i+1, formattedAmount, result.Value.ID)
		}
	}
	
	// Display performance metrics
	fmt.Printf("   📊 Parallel execution completed:\n")
	fmt.Printf("      • Success rate: %d/%d transactions\n", successCount, transactionCount)
	fmt.Printf("      • Total time: %.2f seconds\n", duration.Seconds())
	if duration.Seconds() > 0 {
		fmt.Printf("      • Throughput: %.2f TPS\n", float64(successCount)/duration.Seconds())
	}
	
	// Log transaction route information
	if transactionRoute != nil && sourceOperationRoute != nil && destinationOperationRoute != nil {
		fmt.Printf("   🗺️  Used routes:\n")
		fmt.Printf("      • Transaction Route: %s (%s)\n", transactionRoute.Title, transactionRoute.ID)
		fmt.Printf("      • Operation Routes: %s → %s\n", sourceOperationRoute.Title, destinationOperationRoute.Title)
	}
	
	return firstError
}

// executeHighTPSTransactions demonstrates various TPS optimization techniques
func executeHighTPSTransactions(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, sourceOperationRoute, destinationOperationRoute *models.OperationRoute, transactionRoute *models.TransactionRoute) error {
	ctx, span := observability.StartSpan(ctx, "executeHighTPSTransactions")
	defer span.End()

	fmt.Println("   🔧 TPS Optimization Techniques:")
	
	// Technique 1: Increase Workers and Remove Rate Limiting
	fmt.Println("      1️⃣ High Worker Count (20 workers, no rate limit)")
	if err := demonstrateHighWorkerCount(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, sourceOperationRoute, destinationOperationRoute, transactionRoute); err != nil {
		fmt.Printf("         ❌ Failed: %v\n", err)
	}
	
	// Technique 2: HTTP Connection Pooling Optimization
	fmt.Println("      2️⃣ HTTP Connection Pool Optimization")
	if err := demonstrateConnectionPooling(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, sourceOperationRoute, destinationOperationRoute, transactionRoute); err != nil {
		fmt.Printf("         ❌ Failed: %v\n", err)
	}
	
	// Technique 3: Batch Processing with Optimal Size
	fmt.Println("      3️⃣ Optimal Batch Processing")
	if err := demonstrateBatchProcessing(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, sourceOperationRoute, destinationOperationRoute, transactionRoute); err != nil {
		fmt.Printf("         ❌ Failed: %v\n", err)
	}
	
	// Technique 4: Combined Optimizations
	fmt.Println("      4️⃣ All Optimizations Combined")
	return demonstrateCombinedOptimizations(ctx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, sourceOperationRoute, destinationOperationRoute, transactionRoute)
}

// demonstrateHighWorkerCount shows increased TPS with more workers
func demonstrateHighWorkerCount(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, sourceOperationRoute, destinationOperationRoute *models.OperationRoute, transactionRoute *models.TransactionRoute) error {
	transactionCount := 20
	amounts := make([]string, transactionCount)
	for i := 0; i < transactionCount; i++ {
		amounts[i] = "0.10" // Small amounts for speed
	}
	
	indices := make([]int, transactionCount)
	for i := range indices {
		indices[i] = i
	}
	
	processTransaction := func(ctx context.Context, index int) (*models.Transaction, error) {
		input := &models.CreateTransactionInput{
			ChartOfAccountsGroupName: "high-worker-transfers",
			Description:              fmt.Sprintf("High-worker transfer #%d", index+1),
			Amount:                   amounts[index],
			AssetCode:                "USD",
			Route:                    transactionRoute.ID,
			Send: &models.SendInput{
				Asset: "USD",
				Value: amounts[index],
				Source: &models.SourceInput{
					From: []models.FromToInput{{
						Account: *customerAccount.Alias, AccountAlias: *customerAccount.Alias,
						Route: destinationOperationRoute.ID,
						Amount: models.AmountInput{Asset: "USD", Value: amounts[index]},
					}},
				},
				Distribute: &models.DistributeInput{
					To: []models.FromToInput{{
						Account: *merchantAccount.Alias, AccountAlias: *merchantAccount.Alias,
						Route: destinationOperationRoute.ID,
						Amount: models.AmountInput{Asset: "USD", Value: amounts[index]},
					}},
				},
			},
			IdempotencyKey: uuid.New().String(),
		}
		
		return midazClient.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
	}
	
	startTime := time.Now()
	results := concurrent.WorkerPool(
		ctx, indices, processTransaction,
		concurrent.WithWorkers(20),        // 20 workers instead of 3
		concurrent.WithBufferSize(transactionCount),
		concurrent.WithUnorderedResults(),
		// No rate limiting for maximum speed
	)
	duration := time.Since(startTime)
	
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		}
	}
	
	tps := float64(successCount) / duration.Seconds()
	fmt.Printf("         ✅ %d/%d transactions in %.3fs (%.1f TPS)\n", successCount, transactionCount, duration.Seconds(), tps)
	return nil
}

// demonstrateConnectionPooling shows HTTP connection pool optimization
func demonstrateConnectionPooling(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, sourceOperationRoute, destinationOperationRoute *models.OperationRoute, transactionRoute *models.TransactionRoute) error {
	// Apply performance optimizations
	perfOptions := performance.Options{
		EnableHTTPPooling:   true,
		MaxIdleConnsPerHost: 50,  // Increase from default 10
		UseJSONIterator:     true, // Faster JSON processing
		BatchSize:           100,  // Optimal batch size
	}
	performance.ApplyGlobalPerformanceOptions(perfOptions)
	
	transactionCount := 15
	indices := make([]int, transactionCount)
	for i := range indices {
		indices[i] = i
	}
	
	processTransaction := func(ctx context.Context, index int) (*models.Transaction, error) {
		input := &models.CreateTransactionInput{
			ChartOfAccountsGroupName: "pooled-transfers",
			Description:              fmt.Sprintf("Pooled transfer #%d", index+1),
			Amount:                   "0.15",
			AssetCode:                "USD",
			Route:                    transactionRoute.ID,
			Send: &models.SendInput{
				Asset: "USD", Value: "0.15",
				Source: &models.SourceInput{
					From: []models.FromToInput{{
						Account: *customerAccount.Alias, AccountAlias: *customerAccount.Alias,
						Route: destinationOperationRoute.ID,
						Amount: models.AmountInput{Asset: "USD", Value: "0.15"},
					}},
				},
				Distribute: &models.DistributeInput{
					To: []models.FromToInput{{
						Account: *merchantAccount.Alias, AccountAlias: *merchantAccount.Alias,
						Route: destinationOperationRoute.ID,
						Amount: models.AmountInput{Asset: "USD", Value: "0.15"},
					}},
				},
			},
			IdempotencyKey: uuid.New().String(),
		}
		
		return midazClient.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
	}
	
	startTime := time.Now()
	results := concurrent.WorkerPool(
		ctx, indices, processTransaction,
		concurrent.WithWorkers(15),
		concurrent.WithBufferSize(transactionCount),
		concurrent.WithUnorderedResults(),
	)
	duration := time.Since(startTime)
	
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		}
	}
	
	tps := float64(successCount) / duration.Seconds()
	fmt.Printf("         ✅ %d/%d transactions in %.3fs (%.1f TPS)\n", successCount, transactionCount, duration.Seconds(), tps)
	return nil
}

// demonstrateBatchProcessing shows optimal batch processing
func demonstrateBatchProcessing(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, sourceOperationRoute, destinationOperationRoute *models.OperationRoute, transactionRoute *models.TransactionRoute) error {
	transactionCount := 30
	transactionInputs := make([]*models.CreateTransactionInput, transactionCount)
	
	for i := 0; i < transactionCount; i++ {
		transactionInputs[i] = &models.CreateTransactionInput{
			ChartOfAccountsGroupName: "batch-transfers",
			Description:              fmt.Sprintf("Batch transfer #%d", i+1),
			Amount:                   "0.05",
			AssetCode:                "USD",
			Route:                    transactionRoute.ID,
			Send: &models.SendInput{
				Asset: "USD", Value: "0.05",
				Source: &models.SourceInput{
					From: []models.FromToInput{{
						Account: *customerAccount.Alias, AccountAlias: *customerAccount.Alias,
						Route: destinationOperationRoute.ID,
						Amount: models.AmountInput{Asset: "USD", Value: "0.05"},
					}},
				},
				Distribute: &models.DistributeInput{
					To: []models.FromToInput{{
						Account: *merchantAccount.Alias, AccountAlias: *merchantAccount.Alias,
						Route: destinationOperationRoute.ID,
						Amount: models.AmountInput{Asset: "USD", Value: "0.05"},
					}},
				},
			},
			IdempotencyKey: uuid.New().String(),
		}
	}
	
	batchSize := performance.GetOptimalBatchSize(transactionCount, 10) // Max 10 per batch
	
	processBatch := func(ctx context.Context, batch []*models.CreateTransactionInput) ([]*models.Transaction, error) {
		results := make([]*models.Transaction, 0, len(batch))
		
		// Process batch items in parallel
		indices := make([]int, len(batch))
		for i := range indices {
			indices[i] = i
		}
		
		batchResults := concurrent.WorkerPool(
			ctx, indices,
			func(ctx context.Context, index int) (*models.Transaction, error) {
				return midazClient.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, batch[index])
			},
			concurrent.WithWorkers(5), // 5 workers per batch
			concurrent.WithUnorderedResults(),
		)
		
		for _, result := range batchResults {
			if result.Error == nil {
				results = append(results, result.Value)
			}
		}
		
		return results, nil
	}
	
	startTime := time.Now()
	batchResults := concurrent.Batch(
		ctx, transactionInputs, batchSize, processBatch,
		concurrent.WithWorkers(3), // 3 batches concurrently
	)
	duration := time.Since(startTime)
	
	successCount := 0
	for _, result := range batchResults {
		if result.Error == nil {
			successCount++ // Each result represents one successful transaction
		}
	}
	
	tps := float64(successCount) / duration.Seconds()
	fmt.Printf("         ✅ %d/%d transactions in %.3fs (%.1f TPS)\n", successCount, transactionCount, duration.Seconds(), tps)
	return nil
}

// demonstrateCombinedOptimizations shows all optimizations combined for maximum TPS
func demonstrateCombinedOptimizations(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *models.Account, sourceOperationRoute, destinationOperationRoute *models.OperationRoute, transactionRoute *models.TransactionRoute) error {
	// Apply all performance optimizations
	perfOptions := performance.Options{
		EnableHTTPPooling:   true,
		MaxIdleConnsPerHost: 100, // Maximum connections
		UseJSONIterator:     true,
		BatchSize:           50,  // Large batch size
	}
	performance.ApplyGlobalPerformanceOptions(perfOptions)
	
	transactionCount := 50 // More transactions
	indices := make([]int, transactionCount)
	for i := range indices {
		indices[i] = i
	}
	
	processTransaction := func(ctx context.Context, index int) (*models.Transaction, error) {
		input := &models.CreateTransactionInput{
			ChartOfAccountsGroupName: "optimized-transfers",
			Description:              fmt.Sprintf("Optimized transfer #%d", index+1),
			Amount:                   "0.01", // Minimal amount for speed
			AssetCode:                "USD",
			Route:                    transactionRoute.ID,
			Send: &models.SendInput{
				Asset: "USD", Value: "0.01",
				Source: &models.SourceInput{
					From: []models.FromToInput{{
						Account: *customerAccount.Alias, AccountAlias: *customerAccount.Alias,
						Route: destinationOperationRoute.ID,
						Amount: models.AmountInput{Asset: "USD", Value: "0.01"},
					}},
				},
				Distribute: &models.DistributeInput{
					To: []models.FromToInput{{
						Account: *merchantAccount.Alias, AccountAlias: *merchantAccount.Alias,
						Route: destinationOperationRoute.ID,
						Amount: models.AmountInput{Asset: "USD", Value: "0.01"},
					}},
				},
			},
			IdempotencyKey: uuid.New().String(),
		}
		
		return midazClient.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
	}
	
	startTime := time.Now()
	results := concurrent.WorkerPool(
		ctx, indices, processTransaction,
		concurrent.WithWorkers(30),        // Maximum workers
		concurrent.WithBufferSize(transactionCount),
		concurrent.WithUnorderedResults(),
		// No rate limiting for maximum speed
	)
	duration := time.Since(startTime)
	
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		}
	}
	
	tps := float64(successCount) / duration.Seconds()
	fmt.Printf("         🚀 %d/%d transactions in %.3fs (%.1f TPS) - MAXIMUM OPTIMIZED!\n", successCount, transactionCount, duration.Seconds(), tps)
	
	return nil
}


// Package main demonstrates a real-world example of using concurrency helpers
// to efficiently fetch and process account balances from multiple accounts.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
	"github.com/shopspring/decimal"
)

// AccountBalance holds account ID and its balances.
type AccountBalance struct {
	AccountID string
	Balances  []*models.Balance
}

func main() {
	fmt.Println("Parallel Account Balance Fetching Example")
	fmt.Println("=======================================")

	_, accountIDs, ctx, cancel := setupAndFetchAccounts()
	defer cancel()

	accountBalances, elapsed := fetchBalancesInParallel(ctx, accountIDs)

	displayTotals(accountBalances)
	compareWithSequential(ctx, accountIDs, elapsed)

	fmt.Println("\nUpdating balances in batches...")
	batchUpdateBalances(ctx, accountBalances)
}

func setupAndFetchAccounts() (*client.Client, []string, context.Context, context.CancelFunc) {
	c, err := client.New(
		client.WithEnvironment(config.EnvironmentLocal),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	orgID := "org-123"
	ledgerID := "ledger-456"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	fmt.Println("Fetching accounts...")

	accounts, err := c.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, &models.ListOptions{})
	if err != nil {
		cancel()
		log.Fatalf("Failed to list accounts: %v", err)
	}

	fmt.Printf("Found %d accounts\n", len(accounts.Items))

	accountIDs := make([]string, len(accounts.Items))
	for i, account := range accounts.Items {
		accountIDs[i] = account.ID
	}

	return c, accountIDs, ctx, cancel
}

func fetchBalancesFn(_ context.Context, accountID string) (AccountBalance, error) {
	time.Sleep(100 * time.Millisecond) // Simulate network delay

	balances := []*models.Balance{
		{
			ID:        fmt.Sprintf("bal-%s-usd", accountID),
			AccountID: accountID,
			AssetCode: "USD",
			Available: decimal.NewFromInt(10000),
		},
		{
			ID:        fmt.Sprintf("bal-%s-eur", accountID),
			AccountID: accountID,
			AssetCode: "EUR",
			Available: decimal.NewFromInt(8500),
		},
	}

	return AccountBalance{AccountID: accountID, Balances: balances}, nil
}

func fetchBalancesInParallel(ctx context.Context, accountIDs []string) (map[string][]*models.Balance, time.Duration) {
	fmt.Println("\nFetching balances for all accounts in parallel...")

	startTime := time.Now()

	results := concurrent.WorkerPool(
		ctx,
		accountIDs,
		fetchBalancesFn,
		concurrent.WithWorkers(5),
	)

	elapsed := time.Since(startTime)
	fmt.Printf("Fetched balances for %d accounts in %v\n", len(results), elapsed)

	accountBalances := make(map[string][]*models.Balance)
	var errorCount int

	for _, result := range results {
		if result.Error != nil {
			errorCount++
			fmt.Printf("Error fetching balances for account %s: %v\n", result.Item, result.Error)
		} else {
			accountBalances[result.Value.AccountID] = result.Value.Balances
		}
	}

	fmt.Printf("Successfully fetched balances for %d accounts (with %d errors)\n",
		len(accountBalances), errorCount)

	return accountBalances, elapsed
}

func displayTotals(accountBalances map[string][]*models.Balance) {
	totalsByAsset := make(map[string]decimal.Decimal)

	for _, balances := range accountBalances {
		for _, balance := range balances {
			totalsByAsset[balance.AssetCode] = totalsByAsset[balance.AssetCode].Add(balance.Available)
		}
	}

	fmt.Println("\nTotal balances by asset:")

	for assetCode, total := range totalsByAsset {
		fmt.Printf("%s: %s\n", assetCode, total.String())
	}
}

func compareWithSequential(ctx context.Context, accountIDs []string, parallelElapsed time.Duration) {
	fmt.Println("\nComparing to sequential processing:")

	startTime := time.Now()

	for _, accountID := range accountIDs {
		if _, err := fetchBalancesFn(ctx, accountID); err != nil {
			log.Printf("Error fetching balance for %s: %v", accountID, err)
		}
	}

	sequentialElapsed := time.Since(startTime)

	fmt.Printf("Sequential: %v, Parallel: %v, Speedup: %.2fx\n",
		sequentialElapsed, parallelElapsed, float64(sequentialElapsed)/float64(parallelElapsed))
}

// batchUpdateBalances demonstrates updating balances in batches
func batchUpdateBalances(ctx context.Context, accountBalances map[string][]*models.Balance) {
	// Flatten the balances map into a slice
	var allBalances []*models.Balance
	for _, balances := range accountBalances {
		allBalances = append(allBalances, balances...)
	}

	fmt.Printf("Updating %d balances in batches\n", len(allBalances))

	// Define a batch update function
	updateBalancesBatchFn := func(_ context.Context, balanceBatch []*models.Balance) ([]*models.Balance, error) {
		// In a real app, this would call the Midaz API to update the balances
		// For this example, we'll simulate an API call
		time.Sleep(200 * time.Millisecond) // Simulate network delay

		// Simulate updated balances
		updatedBalances := make([]*models.Balance, len(balanceBatch))

		for i, balance := range balanceBatch {
			// Create a copy of the balance with updated fields
			updatedBalance := *balance
			updatedBalance.UpdatedAt = time.Now()
			updatedBalances[i] = &updatedBalance
		}

		return updatedBalances, nil
	}

	// Use batch processing to update balances
	startTime := time.Now()
	results := concurrent.Batch(
		ctx,
		allBalances,
		10, // Update 10 balances per batch
		updateBalancesBatchFn,
		concurrent.WithWorkers(3), // Process 3 batches concurrently
	)
	elapsed := time.Since(startTime)

	// Count successes and errors
	var successCount, errorCount int

	for _, result := range results {
		if result.Error != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	fmt.Printf("Updated %d balances in %v (errors: %d)\n", successCount, elapsed, errorCount)

	// Compare to sequential processing
	fmt.Println("\nComparing to sequential processing:")

	startTime = time.Now()

	for i := 0; i < len(allBalances); i += 10 {
		end := i + 10
		if end > len(allBalances) {
			end = len(allBalances)
		}

		if _, err := updateBalancesBatchFn(ctx, allBalances[i:end]); err != nil {
			log.Printf("Error updating balances batch: %v", err)
		}
	}

	sequentialElapsed := time.Since(startTime)

	fmt.Printf("Sequential: %v, Parallel: %v, Speedup: %.2fx\n",
		sequentialElapsed, elapsed, float64(sequentialElapsed)/float64(elapsed))
}

// Package concurrent provides utilities for working with concurrent operations in the Midaz SDK.
//
// This file contains high-level helper functions built on top of the core concurrent
// primitives, offering ready-to-use solutions for common financial operations:
// - Parallel account fetching and creation
// - Concurrent transaction processing
// - Generic resource fetching
// - Mixed operation coordination
//
// These helpers are designed to simplify common concurrency patterns in financial
// applications without requiring detailed knowledge of the underlying concurrency mechanisms.
package concurrent

import (
	"context"
	"sync"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
)

// FetchAccountsInParallel fetches multiple accounts concurrently.
// This is useful when you need to retrieve details for many accounts at once.
//
// Use cases:
// - Dashboard loading that needs to display multiple account details
// - Reporting that requires data from many accounts simultaneously
// - Pre-fetching account data for batch processing operations
//
// Example:
//
//	// Fetch 100 customer accounts in parallel with optimized settings
//	accountIDs := fetchRelevantAccountIDs() // e.g., 100 account IDs
//	accounts, err := concurrent.FetchAccountsInParallel(
//		ctx,
//		client.GetAccount, // Function that fetches a single account
//		accountIDs,
//		concurrent.WithWorkers(20),        // Use 20 workers for I/O-bound operation
//		concurrent.WithUnorderedResults(), // Order doesn't matter for map result
//	)
//	if err != nil {
//		handleError(err)
//	}
//	// Now 'accounts' contains all account data, mapped by ID
//
// Parameters:
//   - ctx: The context for the operation, which can be used to cancel all requests.
//   - fetchFn: A function that fetches a single account by ID.
//   - accountIDs: The IDs of the accounts to fetch.
//   - opts: Optional worker pool options.
//
// Returns:
//   - map[string]*models.Account: A map of account ID to account data.
//   - error: The first error encountered, or nil if all operations succeeded.
func FetchAccountsInParallel(
	ctx context.Context,
	fetchFn func(ctx context.Context, accountID string) (*models.Account, error),
	accountIDs []string,
	opts ...PoolOption,
) (map[string]*models.Account, error) {
	// Use the worker pool to fetch accounts in parallel
	results := WorkerPool(ctx, accountIDs, fetchFn, opts...)

	// Collect the results into a map
	accounts := make(map[string]*models.Account)

	for _, result := range results {
		if result.Error != nil {
			return accounts, result.Error
		}

		accounts[result.Item] = result.Value
	}

	return accounts, nil
}

// BatchCreateAccounts creates multiple accounts in batches.
// This is useful when you need to create many accounts at once.
//
// Use cases:
// - Onboarding multiple users from an import process
// - Creating a set of related accounts for a new organization
// - Migrating accounts from another system in bulk
//
// Example:
//
//	// Create 500 customer accounts in batches of 50
//	newAccounts := prepareNewAccounts() // e.g., 500 account objects
//
//	createdAccounts, err := concurrent.BatchCreateAccounts(
//		ctx,
//		client.BatchCreateAccounts, // Function that creates accounts in batches
//		newAccounts,
//		50, // Process in batches of 50 accounts
//		concurrent.WithWorkers(5), // Use 5 concurrent workers
//	)
//	if err != nil {
//		handleError(err)
//	}
//	// Now 'createdAccounts' contains all the created accounts with IDs assigned
//
// Parameters:
//   - ctx: The context for the operation, which can be used to cancel all requests.
//   - createBatchFn: A function that creates a batch of accounts.
//   - accounts: The accounts to create.
//   - batchSize: The maximum number of accounts to create in each batch.
//   - opts: Optional worker pool options.
//
// Returns:
//   - []*models.Account: The created accounts.
//   - error: The first error encountered, or nil if all operations succeeded.
func BatchCreateAccounts(
	ctx context.Context,
	createBatchFn func(ctx context.Context, accounts []*models.Account) ([]*models.Account, error),
	accounts []*models.Account,
	batchSize int,
	opts ...PoolOption,
) ([]*models.Account, error) {
	// Process accounts in batches
	results := Batch(ctx, accounts, batchSize, createBatchFn, opts...)

	// Collect the results
	var createdAccounts []*models.Account

	for _, result := range results {
		if result.Error != nil {
			return createdAccounts, result.Error
		}

		createdAccounts = append(createdAccounts, result.Value)
	}

	return createdAccounts, nil
}

// ProcessTransactionsInParallel processes multiple transactions concurrently.
// This is useful for bulk operations like updating transaction statuses or enriching transaction data.
//
// Use cases:
// - Enriching transaction data with additional metadata
// - Applying categorization or tagging to multiple transactions
// - Validating a batch of pending transactions before submission
// - Updating statuses of multiple transactions in an async workflow
//
// Example:
//
//	// Enrich 1000 transactions with merchant data in parallel
//	transactions := fetchRecentTransactions() // e.g., 1000 transactions
//
//	enrichedTxs, errors := concurrent.ProcessTransactionsInParallel(
//		ctx,
//		enrichTransactionWithMerchantData, // Function that processes a single transaction
//		transactions,
//		concurrent.WithWorkers(25),  // Use 25 concurrent workers
//		concurrent.WithRateLimit(50), // Max 50 operations per second
//	)
//
//	// Handle any errors and use the enriched transactions
//	for i, err := range errors {
//		if err != nil {
//			logTransactionError(transactions[i].ID, err)
//		}
//	}
//
// Parameters:
//   - ctx: The context for the operation, which can be used to cancel all requests.
//   - processFn: A function that processes a single transaction.
//   - transactions: The transactions to process.
//   - opts: Optional worker pool options.
//
// Returns:
//   - []*models.Transaction: The processed transactions.
//   - []error: A slice of errors, one per transaction (nil if no error).
func ProcessTransactionsInParallel(
	ctx context.Context,
	processFn func(ctx context.Context, tx *models.Transaction) (*models.Transaction, error),
	transactions []*models.Transaction,
	opts ...PoolOption,
) ([]*models.Transaction, []error) {
	// Use the worker pool to process transactions in parallel
	results := WorkerPool(ctx, transactions, processFn, opts...)

	// Collect the results and errors
	processedTxs := make([]*models.Transaction, len(results))
	errors := make([]error, len(results))

	for i, result := range results {
		processedTxs[i] = result.Value
		errors[i] = result.Error
	}

	return processedTxs, errors
}

// BulkFetchResourceMap fetches multiple resources concurrently and returns them as a map.
// This is a generic helper for fetching any type of resource.
//
// Use cases:
// - Loading data for multiple merchants in a payment system
// - Fetching configurations for multiple entities in a system
// - Retrieving metadata for a collection of resources
// - Building lookup tables for business operations
//
// Example:
//
//	// Fetch details for multiple merchants in parallel
//	merchantIDs := getMerchantIDsForAnalysis() // e.g., 200 merchant IDs
//
//	// Using the generic function with specific types
//	merchants, err := concurrent.BulkFetchResourceMap(
//		ctx,
//		merchantClient.GetMerchant, // Function that fetches a single merchant
//		merchantIDs,
//		concurrent.WithWorkers(15),
//		concurrent.WithBufferSize(50),
//	)
//	if err != nil {
//		handleError(err)
//	}
//	// Now 'merchants' contains all merchant data mapped by ID
//
// Parameters:
//   - ctx: The context for the operation, which can be used to cancel all requests.
//   - fetchFn: A function that fetches a single resource by ID.
//   - resourceIDs: The IDs of the resources to fetch.
//   - opts: Optional worker pool options.
//
// Returns:
//   - map[K]V: A map of resource ID to resource data.
//   - error: The first error encountered, or nil if all operations succeeded.
func BulkFetchResourceMap[K comparable, V any](
	ctx context.Context,
	fetchFn func(ctx context.Context, id K) (V, error),
	resourceIDs []K,
	opts ...PoolOption,
) (map[K]V, error) {
	// Use the worker pool to fetch resources in parallel
	results := WorkerPool(ctx, resourceIDs, fetchFn, opts...)

	// Collect the results into a map
	resources := make(map[K]V)

	for _, result := range results {
		if result.Error != nil {
			return resources, result.Error
		}

		resources[result.Item] = result.Value
	}

	return resources, nil
}

// RunConcurrentOperations runs multiple operations concurrently and waits for all to complete.
// This is useful when you need to run different types of operations in parallel.
//
// Use cases:
// - Performing system initialization tasks concurrently
// - Running multiple report generation jobs in parallel
// - Executing different kinds of background tasks simultaneously
// - Implementing fan-out patterns for different workloads
//
// Example:
//
//	// Run multiple different operations concurrently as part of a system setup
//	operations := []func(context.Context) error{
//		func(ctx context.Context) error {
//			return initializeDatabase(ctx)
//		},
//		func(ctx context.Context) error {
//			return loadConfigurationData(ctx)
//		},
//		func(ctx context.Context) error {
//			return preWarmCaches(ctx)
//		},
//		func(ctx context.Context) error {
//			return registerSystemWithServiceRegistry(ctx)
//		},
//	}
//
//	errors := concurrent.RunConcurrentOperations(ctx, operations)
//
//	// Check if any operations failed
//	for i, err := range errors {
//		if err != nil {
//			log.Printf("Operation %d failed: %v", i, err)
//		}
//	}
//
// Parameters:
//   - ctx: The context for all operations.
//   - operations: A slice of functions to run concurrently.
//
// Returns:
//   - []error: A slice of errors, one per operation (nil if no error).
func RunConcurrentOperations(
	ctx context.Context,
	operations []func(context.Context) error,
) []error {
	var wg sync.WaitGroup

	errors := make([]error, len(operations))

	// Start all operations
	for i, op := range operations {
		wg.Add(1)

		go func(index int, operation func(context.Context) error) {
			defer wg.Done()

			errors[index] = operation(ctx)
		}(i, op)
	}

	// Wait for all operations to complete
	wg.Wait()

	return errors
}

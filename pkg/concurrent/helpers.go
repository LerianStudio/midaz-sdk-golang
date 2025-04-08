package concurrent

import (
	"context"
	"sync"

	"github.com/LerianStudio/midaz-sdk-golang/models"
)

// FetchAccountsInParallel fetches multiple accounts concurrently.
// This is useful when you need to retrieve details for many accounts at once.
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

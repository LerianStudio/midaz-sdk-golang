package workflows

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sync"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang"
	midazmodels "github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/concurrent"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation"
	"github.com/google/uuid"
)

// Initialize defaults for transaction volume testing
func init() {
	// Set default values if not already set
	if concurrentCustomerToMerchantTxs == 0 {
		concurrentCustomerToMerchantTxs = 20 // Default number of concurrent C2M transactions to run
	}
	if concurrentMerchantToCustomerTxs == 0 {
		concurrentMerchantToCustomerTxs = 20 // Default number of concurrent M2C transactions to run
	}
}

// ExecuteConcurrentTransactions performs concurrent transactions between accounts to test TPS
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
func ExecuteConcurrentTransactions(ctx context.Context, client *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *midazmodels.Account) error {
	// Create a span for observability
	ctx, span := observability.StartSpan(ctx, "ExecuteConcurrentTransactions")
	defer span.End()

	fmt.Println("\n🚀 Executing concurrent transactions for TPS testing...")

	// Record transaction parameters in observability
	observability.AddAttribute(ctx, "organization_id", orgID)
	observability.AddAttribute(ctx, "ledger_id", ledgerID)
	observability.AddAttribute(ctx, "customer_account_id", customerAccount.ID)
	observability.AddAttribute(ctx, "merchant_account_id", merchantAccount.ID)
	observability.AddAttribute(ctx, "c2m_tx_count", concurrentCustomerToMerchantTxs)
	observability.AddAttribute(ctx, "m2c_tx_count", concurrentMerchantToCustomerTxs)

	// Seed the random number generator with a secure random source
	rand.Seed(time.Now().UnixNano())

	// Validate account IDs
	if !validation.IsValidUUID(customerAccount.ID) || !validation.IsValidUUID(merchantAccount.ID) {
		err := fmt.Errorf("invalid account IDs")
		observability.RecordError(ctx, err, "invalid_account_ids")
		return err
	}

	// Execute concurrent transactions from customer to merchant
	fmt.Printf("Running %d concurrent transactions from customer to merchant...\n", concurrentCustomerToMerchantTxs)
	startTimeC2M := time.Now()

	c2mCtx, c2mSpan := observability.StartSpan(ctx, "CustomerToMerchantTransactions")
	if err := ExecuteCustomerToMerchantConcurrent(c2mCtx, client, orgID, ledgerID, customerAccount, merchantAccount, concurrentCustomerToMerchantTxs); err != nil {
		c2mSpan.End()
		observability.RecordError(ctx, err, "c2m_transactions_failed")
		return fmt.Errorf("failed to execute concurrent transactions: %w", err)
	}
	c2mSpan.End()

	customerToMerchantDuration := time.Since(startTimeC2M)
	customerToMerchantTPS := float64(concurrentCustomerToMerchantTxs) / customerToMerchantDuration.Seconds()

	// Record metrics
	observability.RecordSpanMetric(ctx, "c2m_transaction_duration_seconds", customerToMerchantDuration.Seconds())
	observability.RecordSpanMetric(ctx, "c2m_transactions_per_second", customerToMerchantTPS)

	fmt.Printf("✅ Completed %d customer to merchant transactions in %.2f seconds (%.2f TPS)\n\n",
		concurrentCustomerToMerchantTxs, customerToMerchantDuration.Seconds(), customerToMerchantTPS)

	// Execute concurrent transactions from merchant to customer
	fmt.Printf("Running %d concurrent transactions from merchant to customer...\n", concurrentMerchantToCustomerTxs)
	startTimeM2C := time.Now()

	m2cCtx, m2cSpan := observability.StartSpan(ctx, "MerchantToCustomerTransactions")
	if err := ExecuteMerchantToCustomerConcurrent(m2cCtx, client, orgID, ledgerID, customerAccount, merchantAccount, concurrentMerchantToCustomerTxs); err != nil {
		m2cSpan.End()
		observability.RecordError(ctx, err, "m2c_transactions_failed")
		return fmt.Errorf("failed to execute concurrent transactions: %w", err)
	}
	m2cSpan.End()

	merchantToCustomerDuration := time.Since(startTimeM2C)
	merchantToCustomerTPS := float64(concurrentMerchantToCustomerTxs) / merchantToCustomerDuration.Seconds()

	// Record metrics
	observability.RecordSpanMetric(ctx, "m2c_transaction_duration_seconds", merchantToCustomerDuration.Seconds())
	observability.RecordSpanMetric(ctx, "m2c_transactions_per_second", merchantToCustomerTPS)

	fmt.Printf("✅ Completed %d merchant to customer transactions in %.2f seconds (%.2f TPS)\n\n",
		concurrentMerchantToCustomerTxs, merchantToCustomerDuration.Seconds(), merchantToCustomerTPS)

	return nil
}

// GenerateUniqueIdempotencyKey generates a truly unique idempotency key
// by combining multiple sources of uniqueness
//
// Parameters:
//   - prefix: A prefix to identify the source of the key (e.g., "c2m" for customer to merchant)
//   - index: An index to identify the specific transaction
//
// Returns:
//   - string: A unique idempotency key
func GenerateUniqueIdempotencyKey(prefix string, index int) string {
	// Create a hash of multiple unique components
	h := sha256.New()

	// Add a UUID
	uuidStr := uuid.New().String()
	h.Write([]byte(uuidStr))

	// Add the current timestamp with nanosecond precision
	timestamp := time.Now().UnixNano()
	h.Write([]byte(fmt.Sprintf("%d", timestamp)))

	// Add the prefix and index
	h.Write([]byte(fmt.Sprintf("%s-%d", prefix, index)))

	// Add some random bytes
	randomBytes := make([]byte, 16)
	_, err := cryptorand.Read(randomBytes)
	if err != nil {
		// Fallback to non-cryptographic random if crypto/rand fails
		rand.Read(randomBytes)
	}
	h.Write(randomBytes)

	// Get the hash as a hex string
	hash := hex.EncodeToString(h.Sum(nil))

	// Return a combination of components for maximum uniqueness
	return fmt.Sprintf("%s-%s-%d-%d", prefix, hash[:16], index, timestamp)
}

// handleTransactionError processes errors from transaction creation, categorizing them appropriately
func handleTransactionError(ctx context.Context, err error, index int, operation string) error {
	if err == nil {
		return nil
	}

	// Get detailed error information using the errors package
	errDetails := errors.GetErrorDetails(err)

	// Record error in observability system
	observability.RecordError(ctx, err, fmt.Sprintf("%s_transaction_error", operation))
	observability.AddAttribute(ctx, "transaction_index", index)
	observability.AddAttribute(ctx, "error_code", errDetails.Code)
	observability.AddAttribute(ctx, "http_status", errDetails.HTTPStatus)

	// Check if this is a cancellation error (context deadline exceeded)
	if errors.IsCancellationError(err) {
		return fmt.Errorf("%s transaction #%d cancelled: %w", operation, index+1, err)
	}

	// Check if this is a rate limit error that wasn't resolved by retries
	if errors.IsRateLimitError(err) {
		return fmt.Errorf("%s transaction #%d hit rate limit after retries: %w", operation, index+1, err)
	}

	// Check for insufficient balance errors
	if errors.IsInsufficientBalanceError(err) {
		return fmt.Errorf("%s transaction #%d failed due to insufficient balance: %w", operation, index+1, err)
	}

	// Check for validation errors
	if errors.IsValidationError(err) {
		return fmt.Errorf("%s transaction #%d failed validation: %w", operation, index+1, err)
	}

	// Generic error case
	return fmt.Errorf("failed to execute %s transaction #%d: %w", operation, index+1, err)
}

// ExecuteCustomerToMerchantConcurrent executes concurrent transactions from customer to merchant
// using the SDK's concurrency helpers
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - merchantAccount: The merchant account model
//   - count: The number of concurrent transactions to execute
//
// Returns:
//   - error: Any error encountered during the operation
func ExecuteCustomerToMerchantConcurrent(ctx context.Context, client *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *midazmodels.Account, count int) error {
	// Start span for observability
	ctx, span := observability.StartSpan(ctx, "ExecuteCustomerToMerchantConcurrent")
	defer span.End()

	// Create a rate limiter to avoid overwhelming the server
	rateLimiter := concurrent.NewRateLimiter(20000, 20000) // 20000 ops/sec, burst of 20000
	defer rateLimiter.Stop()

	// Create a slice of transaction indices
	indices := make([]int, count)
	for i := range indices {
		indices[i] = i
	}

	// Initialize performance optimizations
	perfOptions := performance.Options{
		BatchSize:       50,
		UseJSONIterator: true,
	}
	performance.ApplyGlobalPerformanceOptions(perfOptions)

	// Create a function to process a single transaction
	processTransaction := func(ctx context.Context, index int) (string, error) {
		// Create span for individual transaction
		txCtx, txSpan := observability.StartSpan(ctx, "ProcessCustomerToMerchantTransaction")
		defer txSpan.End()

		observability.AddAttribute(txCtx, "transaction_index", index)

		// Wait for a rate limiter token
		if err := rateLimiter.Wait(txCtx); err != nil {
			observability.RecordError(txCtx, err, "rate_limiter_wait_error")
			return "", err
		}

		// Generate a truly unique idempotency key
		idempotencyKey := GenerateUniqueIdempotencyKey("c2m", index)

		// Validate the transaction input values using the validation package
		amountValid := validation.IsValidAmount(1, 2)
		if !amountValid {
			err := fmt.Errorf("invalid transaction amount")
			observability.RecordError(txCtx, err, "invalid_amount")
			return "", err
		}

		// Create a transfer transaction with a unique idempotency key
		transferInput := &midazmodels.CreateTransactionInput{
			Description: fmt.Sprintf("Concurrent customer to merchant transfer #%d", index+1),
			Amount:      1, // $0.01
			Scale:       2, // 2 decimal places (cents)
			AssetCode:   "USD",
			Metadata: map[string]any{
				"source": "go-sdk-example",
				"type":   "transfer",
				"index":  index + 1,
			},
			Send: &midazmodels.SendInput{
				Asset: "USD",
				Value: 1, // $0.01
				Scale: 2, // 2 decimal places
				Source: &midazmodels.SourceInput{
					From: []midazmodels.FromToInput{
						{
							Account: customerAccount.ID,
							Amount: midazmodels.AmountInput{
								Asset: "USD",
								Value: 1,
								Scale: 2,
							},
						},
					},
				},
				Distribute: &midazmodels.DistributeInput{
					To: []midazmodels.FromToInput{
						{
							Account: merchantAccount.ID,
							Amount: midazmodels.AmountInput{
								Asset: "USD",
								Value: 1,
								Scale: 2,
							},
						},
					},
				},
			},
			// Use the enhanced idempotency key
			IdempotencyKey: idempotencyKey,
		}

		// Record the start time of the transaction
		startTime := time.Now()

		// Use the entity to create the transaction
		// The underlying HTTP client will automatically handle retries for transient errors
		// such as network timeouts, 5xx server errors, and rate limit errors
		tx, err := client.Entity.Transactions.CreateTransaction(txCtx, orgID, ledgerID, transferInput)

		// Record transaction duration
		duration := time.Since(startTime)
		observability.RecordSpanMetric(txCtx, "transaction_duration_ms", float64(duration.Milliseconds()))

		if err != nil {
			// Use our error handler to properly categorize and format the error
			return "", handleTransactionError(txCtx, err, index, "customer-to-merchant")
		}

		observability.AddAttribute(txCtx, "transaction_id", tx.ID)
		return tx.ID, nil
	}

	// Use concurrent.WorkerPool to process transactions in parallel
	workerOptions := []concurrent.PoolOption{
		concurrent.WithWorkers(10),        // Use 10 workers
		concurrent.WithBufferSize(count),  // Buffer all items
		concurrent.WithUnorderedResults(), // Process in any order
	}

	// Record the start time for metrics
	startTime := time.Now()

	results := concurrent.WorkerPool(
		ctx,
		indices,
		processTransaction,
		workerOptions...,
	)

	// Calculate total duration and throughput
	duration := time.Since(startTime)

	// Count successes and check for errors
	var successCount int
	var firstError error

	for _, result := range results {
		if result.Error != nil {
			if firstError == nil {
				firstError = result.Error
			}
		} else {
			successCount++
		}
	}

	// Record metrics about the batch operation
	observability.RecordSpanMetric(ctx, "c2m_batch_duration_seconds", duration.Seconds())
	observability.RecordSpanMetric(ctx, "c2m_batch_success_count", float64(successCount))
	observability.RecordSpanMetric(ctx, "c2m_batch_error_count", float64(count-successCount))
	if duration.Seconds() > 0 {
		observability.RecordSpanMetric(ctx, "c2m_batch_transactions_per_second", float64(successCount)/duration.Seconds())
	}

	fmt.Printf("Successfully processed %d/%d concurrent customer to merchant transactions\n", successCount, count)

	// Return the first error encountered, if any
	return firstError
}

// ExecuteMerchantToCustomerConcurrent executes concurrent transactions from merchant to customer
// using the SDK's concurrency helpers
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - merchantAccount: The merchant account model
//   - count: The number of concurrent transactions to execute
//
// Returns:
//   - error: Any error encountered during the operation
func ExecuteMerchantToCustomerConcurrent(ctx context.Context, client *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *midazmodels.Account, count int) error {
	// Start span for observability
	ctx, span := observability.StartSpan(ctx, "ExecuteMerchantToCustomerConcurrent")
	defer span.End()

	// Create transaction inputs for each transaction
	transactionInputs := make([]*midazmodels.CreateTransactionInput, count)
	for i := 0; i < count; i++ {
		// Generate a unique idempotency key
		idempotencyKey := GenerateUniqueIdempotencyKey("m2c", i)

		// Validate account IDs using the validation package
		if !validation.IsValidUUID(merchantAccount.ID) || !validation.IsValidUUID(customerAccount.ID) {
			err := fmt.Errorf("invalid account IDs")
			observability.RecordError(ctx, err, "invalid_account_ids")
			return err
		}

		// Create a transfer transaction input
		transactionInputs[i] = &midazmodels.CreateTransactionInput{
			Description: fmt.Sprintf("Concurrent merchant to customer transfer #%d", i+1),
			Amount:      1, // $0.01
			Scale:       2, // 2 decimal places (cents)
			AssetCode:   "USD",
			Metadata: map[string]any{
				"source": "go-sdk-example",
				"type":   "transfer",
				"index":  i + 1,
			},
			Send: &midazmodels.SendInput{
				Asset: "USD",
				Value: 1, // $0.01
				Scale: 2, // 2 decimal places
				Source: &midazmodels.SourceInput{
					From: []midazmodels.FromToInput{
						{
							Account: merchantAccount.ID,
							Amount: midazmodels.AmountInput{
								Asset: "USD",
								Value: 1,
								Scale: 2,
							},
						},
					},
				},
				Distribute: &midazmodels.DistributeInput{
					To: []midazmodels.FromToInput{
						{
							Account: customerAccount.ID,
							Amount: midazmodels.AmountInput{
								Asset: "USD",
								Value: 1,
								Scale: 2,
							},
						},
					},
				},
			},
			IdempotencyKey: idempotencyKey,
		}
	}

	// Apply performance optimizations using the performance package
	perfOptions := performance.Options{
		BatchSize:           100,
		EnableHTTPPooling:   true,
		MaxIdleConnsPerHost: 10,
		UseJSONIterator:     true,
	}
	performance.ApplyBatchingOptions(perfOptions)

	// Define batch processing function with optimized performance
	batchSize := performance.GetOptimalBatchSize(count, 2000) // Max 2000 transactions per batch
	observability.AddAttribute(ctx, "batch_size", batchSize)

	processTransactionBatch := func(ctx context.Context, batch []*midazmodels.CreateTransactionInput) ([]*midazmodels.Transaction, error) {
		// Create span for batch processing
		batchCtx, batchSpan := observability.StartSpan(ctx, "ProcessTransactionBatch")
		defer batchSpan.End()

		observability.AddAttribute(batchCtx, "batch_size", len(batch))

		results := make([]*midazmodels.Transaction, 0, len(batch))
		resultsMutex := &sync.Mutex{} // Mutex to safely append to results

		// Record batch start time
		batchStartTime := time.Now()

		// Use the concurrent.ForEach to process transactions within each batch
		err := concurrent.ForEach(
			batchCtx,
			batch,
			func(ctx context.Context, input *midazmodels.CreateTransactionInput) error {
				// Create span for individual transaction within batch
				txCtx, txSpan := observability.StartSpan(ctx, "ProcessSingleTransaction")
				defer txSpan.End()

				// Extract the index from the metadata to use in the error handler
				var index int
				if idx, ok := input.Metadata["index"]; ok {
					if idxInt, ok := idx.(int); ok {
						index = idxInt - 1 // Convert back to 0-based index
						observability.AddAttribute(txCtx, "transaction_index", index)
					}
				}

				// Record transaction start time
				txStartTime := time.Now()

				// The underlying HTTP client will automatically handle retries for transient errors
				tx, err := client.Entity.Transactions.CreateTransaction(txCtx, orgID, ledgerID, input)

				// Record transaction duration
				txDuration := time.Since(txStartTime)
				observability.RecordSpanMetric(txCtx, "transaction_duration_ms", float64(txDuration.Milliseconds()))

				if err != nil {
					// Use our error handler to properly categorize and format the error
					return handleTransactionError(txCtx, err, index, "merchant-to-customer")
				}

				observability.AddAttribute(txCtx, "transaction_id", tx.ID)

				// Safely append to results with mutex protection
				resultsMutex.Lock()
				results = append(results, tx)
				resultsMutex.Unlock()

				return nil
			},
			concurrent.WithWorkers(3),    // Process 3 transactions at a time within each batch
			concurrent.WithRateLimit(10), // Limit to 10 requests per second
		)

		// Record batch duration and throughput
		batchDuration := time.Since(batchStartTime)
		observability.RecordSpanMetric(batchCtx, "batch_duration_seconds", batchDuration.Seconds())
		if batchDuration.Seconds() > 0 && err == nil {
			observability.RecordSpanMetric(batchCtx, "batch_transactions_per_second", float64(len(results))/batchDuration.Seconds())
		}

		if err != nil {
			observability.RecordError(batchCtx, err, "batch_processing_error")
			return nil, err
		}

		return results, nil
	}

	// Use the concurrent.Batch function to process transactions in batches
	fmt.Println("Processing transactions in batches...")

	// Record batch operation start time
	batchOpStartTime := time.Now()

	batchResults := concurrent.Batch(
		ctx,
		transactionInputs,
		batchSize,
		processTransactionBatch,
		concurrent.WithWorkers(2), // Process 2 batches concurrently
	)

	// Record total batch operation duration
	batchOpDuration := time.Since(batchOpStartTime)
	observability.RecordSpanMetric(ctx, "m2c_batch_operation_duration_seconds", batchOpDuration.Seconds())

	// Count successes and check for errors
	var successCount int
	var firstError error

	for _, result := range batchResults {
		if result.Error != nil {
			if firstError == nil {
				firstError = result.Error
			}
		} else {
			successCount += 1 // Each successful result counts as one
		}
	}

	// Record final metrics
	observability.RecordSpanMetric(ctx, "m2c_batch_success_count", float64(successCount))
	observability.RecordSpanMetric(ctx, "m2c_batch_error_count", float64(count-successCount))
	if batchOpDuration.Seconds() > 0 {
		observability.RecordSpanMetric(ctx, "m2c_batch_transactions_per_second", float64(successCount)/batchOpDuration.Seconds())
	}

	fmt.Printf("Successfully processed %d/%d concurrent merchant to customer transactions\n", successCount, count)

	// Return the first error encountered, if any
	return firstError
}

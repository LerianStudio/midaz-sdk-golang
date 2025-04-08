// Package transaction provides high-level utilities for creating, processing, and managing
// transactions in the Midaz platform. It includes utility functions for common transaction
// patterns, batch processing with error handling, and template-based transaction creation.
package transaction

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/entities"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/errors"
	"github.com/google/uuid"
)

// BatchResult represents the result of a transaction in a batch operation
type BatchResult struct {
	// Index is the position of this transaction in the batch
	Index int
	// TransactionID is the ID of the created transaction if successful
	TransactionID string
	// Error contains any error that occurred during transaction processing
	Error error
	// Duration is how long it took to process this transaction
	Duration time.Duration
}

// BatchOptions configures the behavior of batch operations
type BatchOptions struct {
	// Concurrency is the number of transactions to process in parallel
	// Default is 10 if not specified
	Concurrency int
	// BatchSize is the number of transactions to send in a single batch
	// Default is 100 if not specified
	BatchSize int
	// RetryCount is the number of times to retry failed transactions
	// Default is 3 if not specified
	RetryCount int
	// RetryDelay is the base delay between retries using exponential backoff
	// Default is 100ms if not specified
	RetryDelay time.Duration
	// OnProgress is a callback function that receives progress updates
	// Called after each transaction is processed
	OnProgress func(completed, total int, result BatchResult)
	// IdempotencyKeyPrefix is a prefix to add to generated idempotency keys
	// Default is "batch" if not specified
	IdempotencyKeyPrefix string
	// StopOnError determines if the batch processing should stop on the first error
	// Default is false (continue processing even if some transactions fail)
	StopOnError bool
}

// DefaultBatchOptions returns the default batch processing options
func DefaultBatchOptions() *BatchOptions {
	return &BatchOptions{
		Concurrency:          10,
		BatchSize:            100,
		RetryCount:           3,
		RetryDelay:           100 * time.Millisecond,
		IdempotencyKeyPrefix: "batch",
		StopOnError:          false,
	}
}

// BatchTransactions processes multiple transactions in batches with concurrency and error handling
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout
//   - entity: The Midaz SDK entity client
//   - orgID: The organization ID
//   - ledgerID: The ledger ID
//   - inputs: The transaction inputs to process
//   - options: Options to configure batch processing (optional, pass nil for defaults)
//
// Returns:
//   - A slice of BatchResult containing the result of each transaction
//   - An error if the batch operation couldn't be started
//
// The function ensures idempotency by generating unique keys for each transaction
// if they don't already have one. Results are returned in the same order as inputs,
// regardless of the order in which transactions are processed.
func BatchTransactions(
	ctx context.Context,
	entity *entities.Entity,
	orgID, ledgerID string,
	inputs []*models.CreateTransactionInput,
	options *BatchOptions,
) ([]BatchResult, error) {
	// Use default options if none provided
	if options == nil {
		options = DefaultBatchOptions()
	}

	// Ensure concurrency is at least 1
	if options.Concurrency < 1 {
		options.Concurrency = 1
	}

	// Prepare results slice with the same length as inputs
	results := make([]BatchResult, len(inputs))

	// Define a worker function to process a single transaction
	processTransaction := func(ctx context.Context, index int) error {
		// Start measuring duration
		startTime := time.Now()

		// Get the transaction input
		input := inputs[index]

		// Ensure idempotency key is set (use UUID if not provided)
		if input.IdempotencyKey == "" {
			idempotencyKey := fmt.Sprintf("%s-%s-%d", options.IdempotencyKeyPrefix, uuid.New().String(), index)
			input.IdempotencyKey = idempotencyKey
		}

		// Process transaction with retries
		var tx *models.Transaction
		var err error
		var attempt int

		for attempt = 0; attempt <= options.RetryCount; attempt++ {
			if attempt > 0 {
				// Exponential backoff for retries
				backoffDuration := time.Duration(1<<uint(attempt-1)) * options.RetryDelay
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoffDuration):
					// Continue with retry
				}
			}

			// Create the transaction
			tx, err = entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)

			// If successful or not a retryable error, break
			if err == nil || !isRetryableError(err) {
				break
			}
		}

		// Calculate duration
		duration := time.Since(startTime)

		// Create result
		result := BatchResult{
			Index:         index,
			TransactionID: "",
			Error:         err,
			Duration:      duration,
		}

		// Set transaction ID if successful
		if tx != nil {
			result.TransactionID = tx.ID
		}

		// Store result in the results slice
		results[index] = result

		// Call progress callback if provided
		if options.OnProgress != nil {
			options.OnProgress(index+1, len(inputs), result)
		}

		return err
	}

	// Process transactions with concurrency
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, options.Concurrency)
	errChan := make(chan error, 1)

	// Process inputs in batches
	for i := 0; i < len(inputs); i += options.BatchSize {
		// Calculate end index for this batch
		end := i + options.BatchSize
		if end > len(inputs) {
			end = len(inputs)
		}

		// Process each transaction in the batch
		for j := i; j < end; j++ {
			// Check if we should stop processing due to a previous error
			if options.StopOnError {
				select {
				case err := <-errChan:
					return results, err
				default:
					// Continue processing
				}
			}

			// Wait for a slot in the semaphore
			semaphore <- struct{}{}
			wg.Add(1)

			// Process the transaction
			go func(index int) {
				defer wg.Done()
				defer func() { <-semaphore }()

				err := processTransaction(ctx, index)
				if err != nil && options.StopOnError {
					// If StopOnError is true, send the error to the error channel
					select {
					case errChan <- err:
						// Error sent
					default:
						// Channel already has an error
					}
				}
			}(j)
		}
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Check if we should return an error
	if options.StopOnError {
		select {
		case err := <-errChan:
			return results, err
		default:
			// No error
		}
	}

	return results, nil
}

// BatchSummary provides statistics about a batch operation
type BatchSummary struct {
	// Total number of transactions processed
	TotalTransactions int
	// Number of successful transactions
	SuccessCount int
	// Number of failed transactions
	ErrorCount int
	// Percentage of successful transactions
	SuccessRate float64
	// Total duration of the batch operation
	TotalDuration time.Duration
	// Average duration per transaction
	AverageDuration time.Duration
	// Transactions per second
	TransactionsPerSecond float64
	// Error categories and their counts
	ErrorCategories map[string]int
}

// GetBatchSummary analyzes batch results and returns a summary
func GetBatchSummary(results []BatchResult) BatchSummary {
	total := len(results)
	successCount := 0
	errorCount := 0
	totalDuration := time.Duration(0)
	errorCategories := make(map[string]int)

	for _, result := range results {
		totalDuration += result.Duration

		if result.Error == nil {
			successCount++
		} else {
			errorCount++

			// Categorize errors
			category := errors.GetErrorCategory(result.Error)
			errorCategories[string(category)]++
		}
	}

	var successRate float64
	if total > 0 {
		successRate = float64(successCount) / float64(total) * 100
	}

	var avgDuration time.Duration
	if total > 0 {
		avgDuration = totalDuration / time.Duration(total)
	}

	var tps float64
	if totalDuration > 0 {
		tps = float64(successCount) / totalDuration.Seconds()
	}

	return BatchSummary{
		TotalTransactions:     total,
		SuccessCount:          successCount,
		ErrorCount:            errorCount,
		SuccessRate:           successRate,
		TotalDuration:         totalDuration,
		AverageDuration:       avgDuration,
		TransactionsPerSecond: tps,
		ErrorCategories:       errorCategories,
	}
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types that should be retried
	if errors.IsRateLimitError(err) ||
		errors.IsNetworkError(err) ||
		errors.IsTimeoutError(err) {
		return true
	}

	// Check for transient HTTP errors
	errDetails := errors.GetErrorDetails(err)
	if errDetails.HTTPStatus >= 500 && errDetails.HTTPStatus < 600 {
		return true
	}

	// Other errors should not be retried
	return false
}

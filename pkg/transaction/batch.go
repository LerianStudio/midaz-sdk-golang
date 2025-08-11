// Package transaction provides high-level utilities for creating, processing, and managing
// transactions in the Midaz platform. It includes utility functions for common transaction
// patterns, batch processing with error handling, and template-based transaction creation.
package transaction

import (
	"context"
	"fmt"
	"sync"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
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
	midazClient *client.Client,
	orgID, ledgerID string,
	inputs []*models.CreateTransactionInput,
	options *BatchOptions,
) ([]BatchResult, error) {
	options = normalizeOptions(options)
	results := make([]BatchResult, len(inputs))

	processor := &batchProcessor{
		ctx:      ctx,
		client:   midazClient,
		orgID:    orgID,
		ledgerID: ledgerID,
		inputs:   inputs,
		options:  options,
		results:  results,
	}

	return processor.execute()
}

// normalizeOptions ensures options are valid.
func normalizeOptions(options *BatchOptions) *BatchOptions {
	if options == nil {
		options = DefaultBatchOptions()
	}

	if options.Concurrency < 1 {
		options.Concurrency = 1
	}

	return options
}

// batchProcessor handles the batch transaction processing logic.
type batchProcessor struct {
	ctx      context.Context
	client   *client.Client
	orgID    string
	ledgerID string
	inputs   []*models.CreateTransactionInput
	options  *BatchOptions
	results  []BatchResult
}

// execute runs the batch processing logic.
func (bp *batchProcessor) execute() ([]BatchResult, error) {
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, bp.options.Concurrency)
	errChan := make(chan error, 1)

	for i := 0; i < len(bp.inputs); i += bp.options.BatchSize {
		end := bp.calculateBatchEnd(i)

		if err := bp.processBatch(i, end, &wg, semaphore, errChan); err != nil {
			return bp.results, err
		}
	}

	wg.Wait()

	return bp.checkFinalErrors(errChan)
}

// calculateBatchEnd calculates the end index for a batch.
func (bp *batchProcessor) calculateBatchEnd(start int) int {
	end := start + bp.options.BatchSize
	if end > len(bp.inputs) {
		end = len(bp.inputs)
	}

	return end
}

// processBatch processes a single batch of transactions.
func (bp *batchProcessor) processBatch(start, end int, wg *sync.WaitGroup, semaphore chan struct{}, errChan chan error) error {
	for j := start; j < end; j++ {
		if bp.options.StopOnError {
			if err := bp.checkForEarlyError(errChan); err != nil {
				return err
			}
		}

		bp.startTransactionWorker(j, wg, semaphore, errChan)
	}

	return nil
}

// checkForEarlyError checks if processing should stop due to a previous error.
func (bp *batchProcessor) checkForEarlyError(errChan chan error) error {
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

// startTransactionWorker starts a worker goroutine to process a transaction.
func (bp *batchProcessor) startTransactionWorker(index int, wg *sync.WaitGroup, semaphore chan struct{}, errChan chan error) {
	semaphore <- struct{}{}

	wg.Add(1)

	go func(idx int) {
		defer wg.Done()
		defer func() { <-semaphore }()

		err := bp.processTransaction(idx)
		if err != nil && bp.options.StopOnError {
			select {
			case errChan <- err:
			default:
			}
		}
	}(index)
}

// processTransaction processes a single transaction with retries.
func (bp *batchProcessor) processTransaction(index int) error {
	startTime := time.Now()
	input := bp.inputs[index]

	bp.ensureIdempotencyKey(input, index)
	tx, err := bp.executeWithRetries(input)

	result := bp.createResult(index, tx, err, time.Since(startTime))
	bp.results[index] = result
	bp.callProgressCallback(index, result)

	return err
}

// ensureIdempotencyKey ensures the transaction has an idempotency key.
func (bp *batchProcessor) ensureIdempotencyKey(input *models.CreateTransactionInput, index int) {
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = fmt.Sprintf("%s-%s-%d", bp.options.IdempotencyKeyPrefix, uuid.New().String(), index)
	}
}

// executeWithRetries executes a transaction with retry logic.
func (bp *batchProcessor) executeWithRetries(input *models.CreateTransactionInput) (*models.Transaction, error) {
	var tx *models.Transaction

	var err error

	for attempt := 0; attempt <= bp.options.RetryCount; attempt++ {
		if attempt > 0 {
			if waitErr := bp.waitForRetry(attempt); waitErr != nil {
				return nil, waitErr
			}
		}

		tx, err = bp.client.Entity.Transactions.CreateTransaction(bp.ctx, bp.orgID, bp.ledgerID, input)
		if err == nil || !isRetryableError(err) {
			break
		}
	}

	return tx, err
}

// waitForRetry implements exponential backoff for retries.
func (bp *batchProcessor) waitForRetry(attempt int) error {
	backoffFactor := bp.calculateBackoffFactor(attempt)
	backoffDuration := time.Duration(1<<backoffFactor) * bp.options.RetryDelay

	select {
	case <-bp.ctx.Done():
		return bp.ctx.Err()
	case <-time.After(backoffDuration):
		return nil
	}
}

// calculateBackoffFactor calculates the backoff factor for exponential backoff.
func (bp *batchProcessor) calculateBackoffFactor(attempt int) uint {
	if attempt <= 0 {
		return 0
	}

	// Safely convert attempt to backoff factor with overflow protection
	if attempt > 31 {
		return 30 // Cap to prevent overflow
	}

	// Safe conversion: attempt is guaranteed to be >= 1 and <= 31 here
	// Convert to uint after bounds validation to prevent overflow
	result := attempt - 1
	if result < 0 {
		return 0
	}
	
	return uint(result)
}

// createResult creates a BatchResult for the transaction.
func (bp *batchProcessor) createResult(index int, tx *models.Transaction, err error, duration time.Duration) BatchResult {
	result := BatchResult{
		Index:         index,
		TransactionID: "",
		Error:         err,
		Duration:      duration,
	}

	if tx != nil {
		result.TransactionID = tx.ID
	}

	return result
}

// callProgressCallback calls the progress callback if configured.
func (bp *batchProcessor) callProgressCallback(index int, result BatchResult) {
	if bp.options.OnProgress != nil {
		bp.options.OnProgress(index+1, len(bp.inputs), result)
	}
}

// checkFinalErrors checks for any final errors if StopOnError is enabled.
func (bp *batchProcessor) checkFinalErrors(errChan chan error) ([]BatchResult, error) {
	if bp.options.StopOnError {
		select {
		case err := <-errChan:
			return bp.results, err
		default:
		}
	}

	return bp.results, nil
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

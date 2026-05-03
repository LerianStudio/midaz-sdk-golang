// Package transaction provides high-level utilities for creating, processing, and managing
// transactions in the Midaz platform. It includes utility functions for common transaction
// patterns, batch processing with error handling, and template-based transaction creation.
package transaction

import (
	"context"
	stdErrors "errors"
	"fmt"
	"sync"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
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
	// AllowPartialSuccess controls whether failed individual transactions are returned
	// only in BatchResult.Error without also returning an aggregate error.
	AllowPartialSuccess bool
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
		AllowPartialSuccess:  false,
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
	if ctx == nil {
		ctx = context.Background()
	}

	if midazClient == nil || midazClient.Entity == nil || midazClient.Entity.Transactions == nil {
		return nil, stdErrors.New("transaction service is not initialized")
	}

	if orgID == "" || ledgerID == "" {
		return nil, stdErrors.New("organization and ledger IDs are required")
	}

	options = normalizeOptions(options)
	results := make([]BatchResult, len(inputs))
	recordBatchEvent(ctx, "midaz.transaction.batch.started", orgID, ledgerID, len(inputs), -1, "")

	processor := &batchProcessor{
		ctx:      ctx,
		client:   midazClient,
		orgID:    orgID,
		ledgerID: ledgerID,
		inputs:   inputs,
		options:  options,
		results:  results,
	}

	results, err := processor.execute()

	recordBatchEvent(ctx, "midaz.transaction.batch.completed", orgID, ledgerID, len(inputs), -1, "")

	return results, err
}

// normalizeOptions ensures options are valid.
func normalizeOptions(options *BatchOptions) *BatchOptions {
	if options == nil {
		options = DefaultBatchOptions()
	} else {
		cloned := *options
		options = &cloned
	}

	if options.Concurrency < 1 {
		options.Concurrency = 1
	}

	if options.Concurrency > 100 {
		options.Concurrency = 100
	}

	if options.BatchSize < 1 {
		options.BatchSize = DefaultBatchOptions().BatchSize
	}

	if options.BatchSize > 10_000 {
		options.BatchSize = 10_000
	}

	if options.RetryCount < 0 {
		options.RetryCount = 0
	}

	if options.RetryDelay <= 0 {
		options.RetryDelay = DefaultBatchOptions().RetryDelay
	}

	if options.IdempotencyKeyPrefix == "" {
		options.IdempotencyKeyPrefix = DefaultBatchOptions().IdempotencyKeyPrefix
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

	progressMu sync.Mutex
	completed  int
}

// execute runs the batch processing logic.
func (bp *batchProcessor) execute() ([]BatchResult, error) {
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, bp.options.Concurrency)
	errChan := make(chan error, 1)

	for i := 0; i < len(bp.inputs); i += bp.options.BatchSize {
		if err := bp.ctx.Err(); err != nil {
			bp.markUnscheduledFrom(i, err)
			break
		}

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
		if err := bp.ctx.Err(); err != nil {
			bp.markUnscheduledFrom(j, err)
			return nil
		}

		if bp.options.StopOnError {
			if err := bp.checkForEarlyError(errChan); err != nil {
				return err
			}
		}

		if err := bp.startTransactionWorker(j, wg, semaphore, errChan); err != nil {
			bp.markUnscheduledFrom(j, err)
			return nil
		}
	}

	return nil
}

// checkForEarlyError checks if processing should stop due to a previous error.
func (*batchProcessor) checkForEarlyError(errChan chan error) error {
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

// startTransactionWorker starts a worker goroutine to process a transaction.
func (bp *batchProcessor) startTransactionWorker(index int, wg *sync.WaitGroup, semaphore chan struct{}, errChan chan error) error {
	select {
	case semaphore <- struct{}{}:
	case <-bp.ctx.Done():
		return bp.ctx.Err()
	}

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

	return nil
}

// processTransaction processes a single transaction with retries.
func (bp *batchProcessor) processTransaction(index int) error {
	startTime := time.Now()

	input := bp.inputs[index]
	if input == nil {
		result := bp.createResult(index, nil, fmt.Errorf("transaction input at index %d is nil", index), time.Since(startTime))
		bp.results[index] = result
		bp.callProgressCallback(result)

		return result.Error
	}

	bp.ensureIdempotencyKey(input, index)
	tx, err := bp.executeWithRetries(input)

	result := bp.createResult(index, tx, err, time.Since(startTime))
	bp.results[index] = result
	bp.recordTransactionResultEvent(result)
	bp.callProgressCallback(result)

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

		// Inject idempotency key into context so HTTP layer can add header
		ctx := entities.WithIdempotencyKey(bp.ctx, input.IdempotencyKey)
		tx, err = bp.client.Entity.Transactions.CreateTransaction(ctx, bp.orgID, bp.ledgerID, input)

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
func (*batchProcessor) calculateBackoffFactor(attempt int) uint {
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
func (*batchProcessor) createResult(index int, tx *models.Transaction, err error, duration time.Duration) BatchResult {
	if err == nil && tx == nil {
		err = fmt.Errorf("transaction at index %d returned nil response", index)
	}

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
func (bp *batchProcessor) callProgressCallback(result BatchResult) {
	if bp.options.OnProgress != nil {
		bp.progressMu.Lock()
		bp.completed++
		completed := bp.completed
		bp.options.OnProgress(completed, len(bp.inputs), result)
		bp.progressMu.Unlock()
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

	if !bp.options.AllowPartialSuccess {
		var errs []error

		for i := range bp.results {
			if bp.results[i].Error != nil {
				errs = append(errs, bp.results[i].Error)
			}
		}

		if len(errs) > 0 {
			return bp.results, stdErrors.Join(errs...)
		}
	}

	return bp.results, nil
}

func (bp *batchProcessor) markUnscheduledFrom(start int, err error) {
	if err == nil {
		return
	}

	for i := start; i < len(bp.results); i++ {
		if bp.results[i].Error != nil || bp.results[i].TransactionID != "" {
			continue
		}

		result := bp.createResult(i, nil, err, 0)
		bp.results[i] = result
		bp.recordTransactionResultEvent(result)
		bp.callProgressCallback(result)
	}
}

func (bp *batchProcessor) recordTransactionResultEvent(result BatchResult) {
	event := "midaz.transaction.batch.item.succeeded"
	if result.Error != nil {
		event = "midaz.transaction.batch.item.failed"
	}

	recordBatchEvent(bp.ctx, event, bp.orgID, bp.ledgerID, len(bp.inputs), result.Index, result.TransactionID)
}

func recordBatchEvent(ctx context.Context, event, orgID, ledgerID string, total, index int, transactionID string) {
	attrs := []attribute.KeyValue{
		attribute.String("midaz.business.event", event),
		attribute.String("midaz.business.organizationId", orgID),
		attribute.String("midaz.business.ledgerId", ledgerID),
		attribute.Int("midaz.business.batch.count", total),
	}

	if index >= 0 {
		attrs = append(attrs, attribute.Int("midaz.business.batch.index", index))
	}

	if transactionID != "" {
		attrs = append(attrs, attribute.String("midaz.business.transactionId", transactionID))
	}

	observability.AddSpanEvent(ctx, event, attrs...)
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

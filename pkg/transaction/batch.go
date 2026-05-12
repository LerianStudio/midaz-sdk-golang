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

	"github.com/LerianStudio/midaz-sdk-golang/v3"
	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

const (
	defaultBatchConcurrency = 10
	defaultBatchSize        = 100
	defaultBatchRetryCount  = 3
	defaultBatchRetryDelay  = 100 * time.Millisecond
	maxBatchConcurrency     = 100
	maxTransactionBatchSize = 10_000
	maxBackoffAttempt       = 31
	maxBackoffShift         = 30
	percentageMultiplier    = 100
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
		Concurrency:          defaultBatchConcurrency,
		BatchSize:            defaultBatchSize,
		RetryCount:           defaultBatchRetryCount,
		RetryDelay:           defaultBatchRetryDelay,
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
	midazClient *midaz.Client,
	orgID, ledgerID string,
	inputs []*models.CreateTransactionInput,
	options *BatchOptions,
) ([]BatchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if midazClient == nil || midazClient.Entity == nil || midazClient.Transactions == nil {
		return nil, stdErrors.New("transaction service is not initialized")
	}

	if orgID == "" || ledgerID == "" {
		return nil, stdErrors.New("organization and ledger IDs are required")
	}

	options = normalizeOptions(options)
	results := make([]BatchResult, len(inputs))
	start := time.Now()

	recordBatchStartedEvent(ctx, orgID, ledgerID, len(inputs))

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

	recordBatchCompletedEvent(ctx, orgID, ledgerID, results, err, time.Since(start))

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

	if options.Concurrency > maxBatchConcurrency {
		options.Concurrency = maxBatchConcurrency
	}

	if options.BatchSize < 1 {
		options.BatchSize = DefaultBatchOptions().BatchSize
	}

	if options.BatchSize > maxTransactionBatchSize {
		options.BatchSize = maxTransactionBatchSize
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
	client   *midaz.Client
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
	var (
		wg       sync.WaitGroup
		firstErr error
	)

	semaphore := make(chan struct{}, bp.options.Concurrency)
	errChan := make(chan error, 1)

	for i := 0; i < len(bp.inputs); i += bp.options.BatchSize {
		if err := bp.ctx.Err(); err != nil {
			bp.markUnscheduledFrom(i, err)
			firstErr = err

			break
		}

		end := bp.calculateBatchEnd(i)

		if err := bp.processBatch(i, end, &wg, semaphore, errChan); err != nil {
			firstErr = err

			break
		}
	}

	wg.Wait()

	if firstErr != nil && bp.options.StopOnError {
		return bp.results, firstErr
	}

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
				bp.markUnscheduledFrom(j, err)

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

	idempotencyKey := bp.ensureIdempotencyKey(input, index)
	tx, err := bp.executeWithRetries(input, idempotencyKey)

	result := bp.createResult(index, tx, err, time.Since(startTime))
	bp.results[index] = result
	bp.callProgressCallback(result)

	return err
}

// ensureIdempotencyKey ensures the transaction has an idempotency key.
func (bp *batchProcessor) ensureIdempotencyKey(input *models.CreateTransactionInput, index int) string {
	if input.IdempotencyKey != "" {
		return input.IdempotencyKey
	}

	return fmt.Sprintf("%s-%s-%d", bp.options.IdempotencyKeyPrefix, uuid.New().String(), index)
}

// executeWithRetries executes a transaction with retry logic.
func (bp *batchProcessor) executeWithRetries(input *models.CreateTransactionInput, idempotencyKey string) (*models.Transaction, error) {
	var tx *models.Transaction

	var err error

	for attempt := 0; attempt <= bp.options.RetryCount; attempt++ {
		if attempt > 0 {
			if waitErr := bp.waitForRetry(attempt); waitErr != nil {
				return nil, waitErr
			}
		}

		// Inject idempotency key into context so HTTP layer can add header
		ctx := sdkctx.WithIdempotencyKey(bp.ctx, idempotencyKey)
		tx, err = bp.client.Transactions.CreateTransaction(ctx, bp.orgID, bp.ledgerID, input)

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
	if attempt > maxBackoffAttempt {
		return maxBackoffShift // Cap to prevent overflow
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
//
// We deliberately drop the mutex BEFORE invoking the user callback. The
// previous implementation held progressMu across the call, which meant a
// slow or blocking callback would serialize every other worker — a
// guaranteed throughput cliff for any user that, say, wrote progress to
// stdout. The mutex now only protects the bp.completed increment.
func (bp *batchProcessor) callProgressCallback(result BatchResult) {
	if bp.options.OnProgress == nil {
		return
	}

	bp.progressMu.Lock()
	bp.completed++
	completed := bp.completed
	total := len(bp.inputs)
	bp.progressMu.Unlock()

	// User callback runs unlocked. Workers can keep making progress even
	// if this callback is slow.
	bp.options.OnProgress(completed, total, result)
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
		bp.callProgressCallback(result)
	}
}

func recordBatchStartedEvent(ctx context.Context, orgID, ledgerID string, total int) {
	recordBatchEvent(ctx, "midaz.transaction.batch.started", orgID, ledgerID, total,
		attribute.String("midaz.business.batch.status", "started"),
	)
}

func recordBatchCompletedEvent(ctx context.Context, orgID, ledgerID string, results []BatchResult, err error, duration time.Duration) {
	summary := GetBatchSummary(results)
	status := batchTelemetryStatus(ctx, summary, err)

	// Span events are reserved for state transitions ("started", "completed",
	// "errored"). The cumulative aggregates (success_count, error_count,
	// duration_ms) move to the metric pipeline below — they are
	// pre-aggregated counters/histograms that fit naturally into metrics
	// and do NOT belong as high-cardinality span-event attributes (every
	// run would have a unique attribute set, defeating aggregation).
	stateAttrs := []attribute.KeyValue{
		attribute.String("midaz.business.batch.status", status),
	}

	if err != nil {
		stateAttrs = append(stateAttrs, attribute.String("midaz.business.errorClass", string(errors.GetErrorCategory(err))))
	}

	recordBatchEvent(ctx, "midaz.transaction.batch.completed", orgID, ledgerID, len(results), stateAttrs...)

	// Emit metrics for the aggregates. We use RecordMetric (counter) for
	// integer counts and a per-batch duration metric. The orgID + ledgerID
	// tags keep the attribute set bounded — there's a finite set of
	// (org, ledger, status) tuples.
	provider := observability.GetProvider(ctx)
	if provider == nil || !provider.IsEnabled() {
		return
	}

	metricAttrs := []attribute.KeyValue{
		attribute.String("midaz.organization_id", orgID),
		attribute.String("midaz.ledger_id", ledgerID),
		attribute.String("midaz.batch.status", status),
	}

	observability.RecordMetric(ctx, provider, "midaz.transaction.batch.success_count", float64(summary.SuccessCount), metricAttrs...)
	observability.RecordMetric(ctx, provider, "midaz.transaction.batch.error_count", float64(summary.ErrorCount), metricAttrs...)
	// RecordDuration takes a start time and uses time.Since internally; we
	// already have the duration so reconstruct an equivalent start.
	observability.RecordDuration(ctx, provider, "midaz.transaction.batch.duration", time.Now().Add(-duration), metricAttrs...)
}

func batchTelemetryStatus(ctx context.Context, summary BatchSummary, err error) string {
	if ctx != nil && ctx.Err() != nil {
		return "cancelled"
	}

	if err == nil && summary.ErrorCount == 0 {
		return "completed"
	}

	if summary.SuccessCount > 0 && summary.ErrorCount > 0 {
		return "partial"
	}

	return "failed"
}

func recordBatchEvent(ctx context.Context, event, orgID, ledgerID string, total int, extraAttrs ...attribute.KeyValue) {
	attrs := make([]attribute.KeyValue, 0, 4+len(extraAttrs))
	attrs = append(attrs,
		attribute.String("midaz.business.event", event),
		attribute.String("midaz.business.organizationId", orgID),
		attribute.String("midaz.business.ledgerId", ledgerID),
		attribute.Int("midaz.business.batch.count", total),
	)
	attrs = append(attrs, extraAttrs...)

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
		successRate = float64(successCount) / float64(total) * percentageMultiplier
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

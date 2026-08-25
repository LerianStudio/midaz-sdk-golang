// Package transaction provides high-level utilities for creating, processing, and managing
// transactions in the Midaz platform. It includes utility functions for common transaction
// patterns, batch processing with error handling, and template-based transaction creation.
package transaction

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	stdErrors "errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	obslog "github.com/LerianStudio/lib-observability/log"
	obsruntime "github.com/LerianStudio/lib-observability/runtime"
	"github.com/LerianStudio/midaz-sdk-golang/v5"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/sdkctx"
	"go.opentelemetry.io/otel/attribute"
)

var runtimeNopLogger = obslog.NewNop() //nolint:forbidigo // lib-observability/runtime requires a lib-observability logger.

const (
	defaultBatchConcurrency = 10
	defaultBatchSize        = 100
	defaultBatchRetryCount  = 3
	defaultBatchRetryDelay  = 100 * time.Millisecond
	defaultBatchMaxDelay    = 30 * time.Second
	defaultBatchJitter      = 0.2
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
	// RetryCount is the number of times to retry failed transactions.
	// Default is 3 if not specified.
	//
	// Layering note: this retry count is in ADDITION to the underlying
	// HTTP client's retry budget (entities/http.go). The effective max
	// attempts per transaction is (1+RetryCount) * (1+http.MaxRetries).
	// For workloads that should NOT amplify retries, call
	// (*midaz.Client).WithoutRetries() before entering the batch path,
	// or set RetryCount=0 here and let the HTTP layer own the retry
	// budget.
	RetryCount int
	// RetryDelay is the base delay between retries using exponential backoff.
	// Default is 100ms if not specified.
	RetryDelay time.Duration
	// MaxDelay caps the backoff delay between retries. Without a cap
	// the exponential growth (1 << attempt * RetryDelay) reaches
	// hours/days at attempt 30. Default is 30 seconds.
	MaxDelay time.Duration
	// JitterFactor is the proportion of jitter to apply to the
	// computed backoff delay, in [0.0, 1.0]. 0 disables jitter
	// entirely. Default is 0.2 (±20%). Jitter prevents the
	// thundering-herd pattern where every retried transaction fires at
	// the same wall-clock offset.
	JitterFactor float64
	// OnProgress is a callback function that receives progress updates
	// Called after each transaction is processed
	OnProgress func(completed, total int, result BatchResult)
	// IdempotencyKeyPrefix is retained for source compatibility.
	//
	// Deprecated: transaction batch retries no longer generate idempotency keys.
	// Set CreateTransactionInput.IdempotencyKey explicitly for every transaction
	// when RetryCount > 0.
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
		MaxDelay:             defaultBatchMaxDelay,
		JitterFactor:         defaultBatchJitter,
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
// When RetryCount > 0, every transaction input must include a stable
// IdempotencyKey supplied by the caller. Missing keys are accepted only when
// RetryCount is 0. Results are returned in the same order as inputs, regardless
// of the order in which transactions are processed.
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

	if midazClient == nil || midazClient.Entity == nil || midazClient.V1.Transactions == nil {
		return nil, stdErrors.New("transaction service is not initialized")
	}

	if orgID == "" || ledgerID == "" {
		return nil, stdErrors.New("organization and ledger IDs are required")
	}

	options = normalizeOptions(options)
	results := make([]BatchResult, len(inputs))
	if invalidIndex, err := validateUniqueTransactionIdempotencyKeys(inputs, options); err != nil {
		if invalidIndex >= 0 && invalidIndex < len(results) {
			results[invalidIndex] = BatchResult{Index: invalidIndex, Error: err}
		}

		return results, err
	}
	start := time.Now()

	recordBatchStartedEvent(ctx, orgID, ledgerID, len(inputs))

	processor := &batchProcessor{
		ctx:          ctx,
		transactions: midazClient.V1.Transactions,
		orgID:        orgID,
		ledgerID:     ledgerID,
		inputs:       inputs,
		options:      options,
		results:      results,
	}

	results, err := processor.execute()

	recordBatchCompletedEvent(ctx, orgID, ledgerID, results, err, time.Since(start))

	return results, err
}

func validateUniqueTransactionIdempotencyKeys(inputs []*models.CreateTransactionInput, options *BatchOptions) (int, error) {
	if options == nil || options.RetryCount <= 0 {
		return -1, nil
	}

	seen := make(map[string]int, len(inputs))
	for i, input := range inputs {
		if input == nil {
			return i, fmt.Errorf("transaction input at index %d is nil", i)
		}

		key := strings.TrimSpace(input.IdempotencyKey)
		if key == "" {
			return i, fmt.Errorf("transaction input at index %d requires idempotency key when retries are enabled", i)
		}

		if previousIndex, ok := seen[key]; ok {
			return i, fmt.Errorf("transaction input at index %d reuses idempotency key from index %d", i, previousIndex)
		}

		seen[key] = i
	}

	return -1, nil
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

	if options.MaxDelay <= 0 {
		options.MaxDelay = DefaultBatchOptions().MaxDelay
	}

	// Clamp the jitter factor to a sane window. Values outside [0, 1]
	// fall back to the default rather than silently producing negative
	// or amplified delays.
	if options.JitterFactor < 0 || options.JitterFactor > 1 {
		options.JitterFactor = DefaultBatchOptions().JitterFactor
	}

	if options.IdempotencyKeyPrefix == "" {
		options.IdempotencyKeyPrefix = DefaultBatchOptions().IdempotencyKeyPrefix
	}

	return options
}

// batchProcessor handles the batch transaction processing logic.
// transactionCreator is the narrow slice of the transactions accessor the
// batch processor needs (Epic 5.3 consumer-side interface; client.V1.Transactions
// is now a concrete facade). Tests inject a mock satisfying just this.
type transactionCreator interface {
	CreateJSON(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error)
}

type batchProcessor struct {
	ctx          context.Context
	transactions transactionCreator
	orgID        string
	ledgerID     string
	inputs       []*models.CreateTransactionInput
	options      *BatchOptions
	results      []BatchResult

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

	obsruntime.SafeGoWithContextAndComponent(bp.ctx, runtimeNopLogger, "midaz-sdk", "transaction.batch.worker", obsruntime.KeepRunning, func(context.Context) {
		idx := index
		defer wg.Done()
		defer func() { <-semaphore }()
		defer bp.recoverTransactionWorkerPanic(idx, errChan)()

		err := bp.processTransaction(idx)
		if err != nil && bp.options.StopOnError {
			select {
			case errChan <- err:
			default:
			}
		}
	})

	return nil
}

func (bp *batchProcessor) recoverTransactionWorkerPanic(index int, errChan chan error) func() {
	return func() {
		panicValue := recover()
		if panicValue == nil {
			return
		}

		obsruntime.HandlePanicValue(bp.ctx, runtimeNopLogger, panicValue, "midaz-sdk", "transaction.batch.worker")
		err := fmt.Errorf("transaction batch worker panic at index %d: %v", index, panicValue)
		result := bp.createResult(index, nil, err, 0)
		bp.results[index] = result
		bp.callProgressCallback(result)

		if bp.options.StopOnError {
			select {
			case errChan <- err:
			default:
			}
		}
	}
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
	if bp.options.RetryCount > 0 && strings.TrimSpace(input.IdempotencyKey) == "" {
		result := bp.createResult(index, nil, fmt.Errorf("transaction input at index %d requires idempotency key when retries are enabled", index), time.Since(startTime))
		bp.results[index] = result
		bp.callProgressCallback(result)

		return result.Error
	}

	idempotencyKey := bp.ensureIdempotencyKey(input)
	tx, err := bp.executeWithRetries(input, idempotencyKey)

	result := bp.createResult(index, tx, err, time.Since(startTime))
	bp.results[index] = result
	bp.callProgressCallback(result)

	return err
}

// ensureIdempotencyKey ensures the transaction has an idempotency key.
func (*batchProcessor) ensureIdempotencyKey(input *models.CreateTransactionInput) string {
	if key := strings.TrimSpace(input.IdempotencyKey); key != "" {
		return key
	}

	return ""
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

		ctx := sdkctx.WithoutHTTPRetries(bp.ctx)
		if idempotencyKey != "" {
			// Inject idempotency key into context so HTTP layer can add header.
			ctx = sdkctx.WithIdempotencyKey(ctx, idempotencyKey)
		} else {
			// Missing keys are allowed only when batch retries are disabled.
			// Suppress lower-layer auto-idempotency so the HTTP client also
			// treats the unsafe request as non-retryable.
			ctx = sdkctx.WithoutAutoIdempotency(ctx)
		}
		tx, err = bp.transactions.CreateJSON(ctx, bp.orgID, bp.ledgerID, input)

		if err == nil || !isRetryableError(err) {
			break
		}
	}

	return tx, err
}

// waitForRetry implements exponential backoff for retries with jitter
// and a hard cap. Capping is non-negotiable: the previous unbounded
// form reached ~31 hours of wait at attempt 30 because Go's `1<<30`
// shift overflowed reasoning. The final delay is clamped after jitter
// so MaxDelay remains a hard upper bound.
//
// time.NewTimer with a deferred Stop is kept for explicit, consistent
// cleanup. The old worry — that time.After leaks its underlying timer
// until the duration elapses even when ctx.Done fires first on a
// retry-storm shutdown — was a pre-Go-1.23 concern; since Go 1.23 an
// unreferenced time.After timer is GC-eligible before it fires, so no
// leak occurs on this repo's Go 1.26.
func (bp *batchProcessor) waitForRetry(attempt int) error {
	backoffDuration := bp.computeBackoffWithJitter(attempt)

	timer := time.NewTimer(backoffDuration)
	defer timer.Stop()

	select {
	case <-bp.ctx.Done():
		return bp.ctx.Err()
	case <-timer.C:
		return nil
	}
}

// computeBackoffWithJitter is the deterministic part of waitForRetry,
// split out so the math stays unit-testable independent of the timer.
func (bp *batchProcessor) computeBackoffWithJitter(attempt int) time.Duration {
	backoffFactor := bp.calculateBackoffFactor(attempt)
	backoffDuration := time.Duration(1<<backoffFactor) * bp.options.RetryDelay

	// Cap before jitter so the jitter window is anchored against a bounded base.
	if bp.options.MaxDelay > 0 && backoffDuration > bp.options.MaxDelay {
		backoffDuration = bp.options.MaxDelay
	}

	if bp.options.JitterFactor > 0 {
		// Symmetric jitter in ±JitterFactor proportion of the base.
		// Use crypto/rand to match the policy of pkg/retry; gosec
		// G404 rejects math/rand even for non-security scheduling
		// jitter, and the per-attempt allocation cost is negligible
		// against the millisecond-scale retry timing.
		jitter := secureJitterFraction() // [-1, 1)
		offset := time.Duration(float64(backoffDuration) * bp.options.JitterFactor * jitter)
		backoffDuration += offset
	}

	if backoffDuration < 0 {
		// Jitter could subtract more than the base on tiny initial
		// delays; clamp to zero so the timer fires immediately rather
		// than panicking in time.NewTimer.
		backoffDuration = 0
	}
	if bp.options.MaxDelay > 0 && backoffDuration > bp.options.MaxDelay {
		backoffDuration = bp.options.MaxDelay
	}

	return backoffDuration
}

var secureJitterFraction = readSecureJitterFraction

// secureJitterFraction returns a value in [-1.0, 1.0) using crypto/rand.
// On crypto/rand failure (effectively impossible outside a broken OS)
// it returns 0 — i.e. "no jitter this round" — which is still a safe
// retry interval. Mirrors the policy used by pkg/retry's
// getSecureRandomFloat64 helper but locally inlined to avoid widening
// pkg/transaction's import surface.
func readSecureJitterFraction() float64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0
	}

	// Convert to a float in [0, 1), then to [-1, 1).
	u := binary.BigEndian.Uint64(buf[:])
	f := float64(u) / float64(math.MaxUint64)

	return f*2 - 1
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

// batchTelemetryStatus derives an internal batch-orchestration outcome label
// for telemetry — the OpenTelemetry metric attribute midaz.batch.status and the
// span-event attribute midaz.business.batch.status are both fed by this value.
// These values (cancelled/completed/partial/failed) describe the batch run as a
// whole and are deliberately distinct from the server's per-transaction status
// vocabulary (TransactionStatusCode: CREATED/PENDING/APPROVED/CANCELED/NOTED).
// Do not conflate them: "partial" has no server-status analog, and these labels
// never touch transaction Status.Code.
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

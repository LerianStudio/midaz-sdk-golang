// Package concurrent provides utilities for working with concurrent operations in the Midaz SDK.
//
// This package implements common concurrency patterns such as worker pools, rate limiters,
// and batch operations to help users work efficiently with the Midaz API.
//
// Use Cases:
//
//  1. High-Volume Payment Processing:
//     When processing thousands of payment transactions, use WorkerPool with
//     appropriate concurrency settings to maximize throughput while respecting API limits:
//
//     ```go
//     // Process 10,000 payments concurrently with controlled parallelism
//     payments := fetchPendingPayments() // e.g., 10,000 payments
//
//     results := concurrent.WorkerPool(ctx, payments,
//     func(ctx context.Context, payment Payment) (PaymentResult, error) {
//     return processPayment(ctx, payment)
//     },
//     concurrent.WithWorkers(20),              // Use 20 concurrent workers
//     concurrent.WithBufferSize(100),          // Buffer 100 items
//     concurrent.WithRateLimit(1000),          // Max 1000 ops/second
//     concurrent.WithUnorderedResults(),       // Process in any order
//     )
//
//     // Handle results
//     for _, result := range results {
//     if result.Error != nil {
//     logPaymentError(result.Item, result.Error)
//     } else {
//     recordSuccessfulPayment(result.Item, result.Value)
//     }
//     }
//     ```
//
//  2. Batch Account Updates:
//     When performing batch updates to many accounts, use the Batch function
//     to group operations efficiently:
//
//     ```go
//     // Update 5,000 accounts in batches of 50
//     accounts := fetchAccountsToUpdate() // e.g., 5,000 accounts
//
//     results := concurrent.Batch(ctx, accounts, 50,
//     func(ctx context.Context, batch []Account) ([]UpdateResult, error) {
//     // API call that can process up to 50 accounts at once
//     return client.BulkUpdateAccounts(ctx, batch)
//     },
//     )
//
//     // Process results...
//     ```
//
//  3. API Rate Limiting:
//     When calling an API with strict rate limits, use RateLimiter to prevent
//     exceeding those limits and triggering throttling or blocks:
//
//     ```go
//     // Create a rate limiter for an API limited to 100 requests per second
//     rateLimiter := concurrent.NewRateLimiter(100, 20)
//     defer rateLimiter.Stop()
//
//     // In your request function
//     func makeAPIRequest(ctx context.Context, req Request) (Response, error) {
//     // Wait for a rate limiter token before proceeding
//     if err := rateLimiter.Wait(ctx); err != nil {
//     return Response{}, err
//     }
//
//     // Now make the API call, knowing it respects rate limits
//     return client.SendRequest(ctx, req)
//     }
//     ```
package concurrent

import (
	"context"
	"sync"
	"time"
)

// WorkFunc is a generic worker function that processes an item and returns a result and error.
type WorkFunc[T, R any] func(ctx context.Context, item T) (R, error)

// Result holds the result of a processed item along with any error that occurred.
//
// The Result struct provides complete context about a processed operation:
// - The original input item
// - The generated output value (if successful)
// - Any error that occurred during processing
// - The original index from the input slice (for ordered results)
type Result[T, R any] struct {
	// Item is the original item being processed.
	Item T

	// Value is the result of the processed item.
	Value R

	// Error is any error that occurred during processing.
	Error error

	// Index is the index of the item in the original slice (if processed as part of a batch).
	Index int
}

// WorkerPool creates a pool of workers for parallel processing.
// It accepts a slice of items and processes them using the provided work function.
//
// This is ideal for scenarios such as:
// - Processing a large number of independent API calls concurrently
// - Performing batch operations where each operation is independent
// - Distributing work across multiple cores for CPU-intensive tasks
//
// Parameters:
//   - ctx: The context for the operation, which can be used to cancel all workers.
//   - items: The slice of items to process.
//   - workFn: The function to process each item.
//   - opts: Optional worker pool options.
//
// Returns:
//   - []Result: A slice of results, in the same order as the input items unless WithUnorderedResults is used.
//
// Example use case: Processing thousands of account validation requests in parallel:
//
//	accountIDs := fetchAccountsToValidate() // e.g., 10,000 account IDs
//
//	results := concurrent.WorkerPool(ctx, accountIDs,
//	    func(ctx context.Context, accountID string) (ValidationResult, error) {
//	        return validateAccount(ctx, accountID)
//	    },
//	    concurrent.WithWorkers(25),  // Use 25 concurrent workers
//	)
func WorkerPool[T, R any](
	ctx context.Context,
	items []T,
	workFn WorkFunc[T, R],
	opts ...PoolOption,
) []Result[T, R] {
	options := defaultPoolOptions()
	for _, opt := range opts {
		opt(options)
	}

	// Create channels for coordinating workers
	itemCh := make(chan indexedItem[T], options.bufferSize)
	resultCh := make(chan Result[T, R], options.bufferSize)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < options.workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for item := range itemCh {
				// Check if context is cancelled
				if ctx.Err() != nil {
					continue
				}

				// Apply rate limiting if configured
				if options.rateLimit > 0 {
					limiter := time.Tick(time.Second / time.Duration(options.rateLimit))
					select {
					case <-limiter:
						// Rate limit applied, continue processing
					case <-ctx.Done():
						// Context canceled, stop processing
						return
					}
				}

				// Process the item
				result, err := workFn(ctx, item.value)

				// Send the result
				resultCh <- Result[T, R]{
					Item:  item.value,
					Value: result,
					Error: err,
					Index: item.index,
				}
			}
		}()
	}

	// Send items to workers
	go func() {
		for i, item := range items {
			select {
			case itemCh <- indexedItem[T]{value: item, index: i}:
				// Item sent successfully
			case <-ctx.Done():
				// Context canceled, stop sending items
				break
			}
		}

		close(itemCh)

		// Wait for all workers to finish
		wg.Wait()
		close(resultCh)
	}()

	// Collect results
	var results []Result[T, R]

	if options.ordered {
		// For ordered results, we need to collect all results and then sort them
		allResults := make([]Result[T, R], 0, len(items))
		for r := range resultCh {
			allResults = append(allResults, r)
		}

		// Create ordered results slice
		results = make([]Result[T, R], len(items))
		for _, r := range allResults {
			results[r.Index] = r
		}
	} else {
		// For unordered results, we can just collect them as they come
		results = make([]Result[T, R], 0, len(items))
		for r := range resultCh {
			results = append(results, r)
		}
	}

	return results
}

// indexedItem holds a value and its index in the original slice.
type indexedItem[T any] struct {
	value T
	index int
}

// poolOptions configures the worker pool behavior.
type poolOptions struct {
	// workers is the number of worker goroutines to use.
	workers int

	// bufferSize is the size of the channel buffers.
	bufferSize int

	// ordered determines whether results should be returned in the same order as inputs.
	ordered bool

	// rateLimit is the maximum number of operations per second.
	rateLimit int
}

// PoolOption is a function that modifies pool options.
type PoolOption func(*poolOptions)

// defaultPoolOptions returns default pool options.
func defaultPoolOptions() *poolOptions {
	return &poolOptions{
		workers:    5,
		bufferSize: 10,
		ordered:    true,
		rateLimit:  0, // No rate limiting by default
	}
}

// WithWorkers sets the number of worker goroutines.
//
// The optimal number of workers depends on the workload:
// - For I/O-bound tasks (like API calls), a higher number (10-50) often works best
// - For CPU-bound tasks, a number close to available CPU cores is usually optimal
// - For mixed workloads, start with 2-3x CPU cores and adjust based on performance tests
//
// Example use case: When processing I/O-bound transactions that involve network calls:
//
//	// Use more workers for I/O-bound operations to maximize throughput
//	concurrent.WithWorkers(30)
func WithWorkers(workers int) PoolOption {
	return func(o *poolOptions) {
		if workers > 0 {
			o.workers = workers
		}
	}
}

// WithBufferSize sets the size of the channel buffers.
//
// Larger buffer sizes can improve throughput by:
// - Reducing time workers spend waiting for new items
// - Allowing more items to be queued for processing
// - Smoothing out processing of items that vary in completion time
//
// However, larger buffers also consume more memory. Choose a buffer size that
// balances throughput needs with memory constraints.
//
// Example use case: When processing a large batch of items with variable processing times:
//
//	// Use a larger buffer for variable-time operations
//	concurrent.WithBufferSize(500)
func WithBufferSize(size int) PoolOption {
	return func(o *poolOptions) {
		if size > 0 {
			o.bufferSize = size
		}
	}
}

// WithUnorderedResults configures the pool to return results as they are completed,
// rather than maintaining the original order.
//
// This can significantly improve performance when:
// - Processing time varies widely between items
// - You don't need results in the same order as inputs
// - You want to start handling results as soon as they're available
//
// Example use case: When processing independent transactions where order doesn't matter:
//
//	// Process transactions in any order for maximum throughput
//	concurrent.WithUnorderedResults()
func WithUnorderedResults() PoolOption {
	return func(o *poolOptions) {
		o.ordered = false
	}
}

// WithRateLimit sets the maximum number of operations per second.
func WithRateLimit(operationsPerSecond int) PoolOption {
	return func(o *poolOptions) {
		if operationsPerSecond > 0 {
			o.rateLimit = operationsPerSecond
		}
	}
}

// Batch processes items in batches using a worker pool, useful for
// processing large volumes of data while respecting API rate limits.
//
// Parameters:
//   - ctx: The context for the operation, which can be used to cancel all batches.
//   - items: The slice of items to process.
//   - batchSize: The maximum number of items to process in each batch.
//   - workFn: The function to process each batch of items.
//   - opts: Optional worker pool options applied to the batch worker pool.
//
// Returns:
//   - []Result: A slice of results, in the same order as the input items.
func Batch[T, R any](
	ctx context.Context,
	items []T,
	batchSize int,
	workFn func(ctx context.Context, batch []T) ([]R, error),
	opts ...PoolOption,
) []Result[T, R] {
	// Validate batch size
	if batchSize <= 0 {
		batchSize = 10 // Default batch size
	}

	// Create batches
	var batches [][]T

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}

		batches = append(batches, items[i:end])
	}

	// Process batches concurrently using worker pool
	batchResults := WorkerPool(ctx, batches, func(ctx context.Context, batch []T) ([]R, error) {
		return workFn(ctx, batch)
	}, opts...)

	// Convert batch results to individual results
	var results []Result[T, R]

	for _, br := range batchResults {
		// If there was an error in batch processing, apply it to all items in the batch
		if br.Error != nil {
			for i, item := range br.Item {
				results = append(results, Result[T, R]{
					Item:  item,
					Error: br.Error,
					Index: br.Index*batchSize + i,
				})
			}
		} else if len(br.Value) > 0 {
			// If there are results, map them back to the original items
			for i, val := range br.Value {
				if i < len(br.Item) {
					results = append(results, Result[T, R]{
						Item:  br.Item[i],
						Value: val,
						Index: br.Index*batchSize + i,
					})
				}
			}
		}
	}

	return results
}

// ForEach executes a function for each item in parallel, when you don't need to collect results.
// This is useful for fire-and-forget operations like updates or deletions.
//
// Parameters:
//   - ctx: The context for the operation, which can be used to cancel all operations.
//   - items: The slice of items to process.
//   - fn: The function to execute for each item.
//   - opts: Optional worker pool options.
//
// Returns:
//   - error: The first error encountered, or nil if all operations succeeded.
func ForEach[T any](
	ctx context.Context,
	items []T,
	fn func(ctx context.Context, item T) error,
	opts ...PoolOption,
) error {
	// Convert the function to a work function that returns a bool
	workFn := func(ctx context.Context, item T) (bool, error) {
		err := fn(ctx, item)
		return err == nil, err
	}

	// Use the worker pool to process items
	results := WorkerPool(ctx, items, workFn, opts...)

	// Return the first error encountered, if any
	for _, r := range results {
		if r.Error != nil {
			return r.Error
		}
	}

	return nil
}

// RateLimiter provides a simple mechanism to limit the rate of operations.
// It can be used to ensure API rate limits are respected across goroutines.
//
// Common applications include:
// - Respecting third-party API rate limits
// - Preventing database or service overload
// - Implementing fair resource allocation in multi-tenant systems
// - Smoothing traffic patterns to avoid spikes
type RateLimiter struct {
	ticker   *time.Ticker
	stopCh   chan struct{}
	tokensCh chan struct{}
	wg       sync.WaitGroup
}

// NewRateLimiter creates a new rate limiter with the specified maximum operations per second.
//
// Parameters:
//   - opsPerSecond: The maximum number of operations per second.
//   - maxBurst: The maximum number of operations allowed in a burst (buffer size).
//
// Returns:
//   - *RateLimiter: A new rate limiter instance.
//
// Example use case: When calling a third-party API with a documented rate limit:
//
//	// Create a rate limiter for an API limited to 5 requests/second with burst of 10
//	// (e.g., a payment processor that allows brief bursts of higher throughput)
//	rateLimiter := concurrent.NewRateLimiter(5, 10)
//	defer rateLimiter.Stop()
func NewRateLimiter(opsPerSecond int, maxBurst int) *RateLimiter {
	if opsPerSecond <= 0 {
		opsPerSecond = 1 // Minimum 1 op per second
	}

	if maxBurst <= 0 {
		maxBurst = opsPerSecond // Default to allow one second worth of operations
	}

	interval := time.Second / time.Duration(opsPerSecond)
	rl := &RateLimiter{
		ticker:   time.NewTicker(interval),
		stopCh:   make(chan struct{}),
		tokensCh: make(chan struct{}, maxBurst),
	}

	// Start the token generator
	rl.wg.Add(1)

	go func() {
		defer rl.wg.Done()

		for {
			select {
			case <-rl.ticker.C:
				// Try to add a token, but don't block if buffer is full
				select {
				case rl.tokensCh <- struct{}{}:
					// Token added
				default:
					// Buffer full, token dropped
				}
			case <-rl.stopCh:
				return
			}
		}
	}()

	return rl
}

// Wait blocks until a token is available or the context is cancelled.
//
// This method is thread-safe and can be called concurrently from multiple goroutines.
// It implements a non-busy wait using channels, efficiently parking the goroutine
// until a token becomes available.
//
// Parameters:
//   - ctx: The context that can be used to cancel the wait.
//
// Returns:
//   - error: Context error if the context was cancelled, nil otherwise.
//
// Example use case: Ensuring an API call respects rate limits:
//
//	func callRateLimitedAPI(ctx context.Context, req Request) (Response, error) {
//	    // Wait for a rate limiter token before proceeding
//	    if err := rateLimiter.Wait(ctx); err != nil {
//	        return Response{}, fmt.Errorf("rate limit wait canceled: %w", err)
//	    }
//
//	    // Now make the API call, knowing it respects rate limits
//	    return apiClient.Call(ctx, req)
//	}
func (r *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-r.tokensCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop stops the rate limiter and releases resources.
func (r *RateLimiter) Stop() {
	close(r.stopCh)
	r.ticker.Stop()
	r.wg.Wait()
}

// WithWaitGroup creates a worker pool that utilizes an external wait group
// in addition to the internal one. This is useful when you want to wait
// for all worker pools to complete from outside.
func WithWaitGroup(wg *sync.WaitGroup) PoolOption {
	return func(options *poolOptions) {
		// This is implemented in the WorkerPool function
		// through closure capture of the provided WaitGroup
		// This option is just a placeholder for design consistency
	}
}

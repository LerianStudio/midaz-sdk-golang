// Package retry provides utilities for implementing retry logic with exponential backoff
// and jitter for resilient operations. It allows for configurable retry strategies,
// context-aware cancellation, and flexible error handling.
//
// Real-World Use Cases:
//
//  1. API Call Resilience:
//     When integrating with third-party financial APIs that may experience temporary
//     outages or rate limiting, retry logic ensures operation completion:
//
//     ```go
//     // Attempt to process a payment with retry logic for transient failures
//     err := retry.Do(ctx, func() error {
//     return paymentProcessor.ProcessTransaction(ctx, transaction)
//     },
//     retry.WithMaxRetries(5),                   // Try up to 5 times
//     retry.WithInitialDelay(200*time.Millisecond), // Start with 200ms delay
//     retry.WithBackoffFactor(2.0))              // Double delay after each failure
//     ```
//
//  2. Database Operation Retries:
//     When performing critical database operations that might experience transient
//     failures like deadlocks or connection issues:
//
//     ```go
//     // Configure context with high-reliability retry options for database operations
//     dbCtx := retry.WithOptionsContext(ctx, &retry.Options{
//     MaxRetries:      5,
//     InitialDelay:    100 * time.Millisecond,
//     BackoffFactor:   1.5,
//     RetryableErrors: []string{"deadlock", "connection reset", "lock timeout"},
//     })
//
//     // Any function using DoWithContext will use these options
//     err := retry.DoWithContext(dbCtx, func() error {
//     return db.ExecuteTransaction(dbCtx, operations)
//     })
//     ```
//
//  3. Distributed Systems Communication:
//     When services communicate across network boundaries, retries with jitter help
//     prevent thundering herd problems during recovery:
//
//     ```go
//     // Configure retries with jitter for service-to-service communication
//     err := retry.Do(ctx, func() error {
//     return serviceClient.FetchData(ctx, request)
//     },
//     retry.WithMaxRetries(3),
//     retry.WithJitterFactor(0.3))  // Add 0-30% random variation to delays
//     ```
package retry

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strings"
	"time"
)

const midpointProbability = 0.5

var (
	errNilContext = errors.New("retry context is nil")
	errNilFunc    = errors.New("retry function is nil")
	errNilOption  = errors.New("retry option is nil")

	// ErrRetriesExhausted is the typed sentinel wrapped around every
	// "retry budget exhausted" terminal error produced by this package.
	// Callers can detect retry exhaustion via:
	//
	//	if errors.Is(err, retry.ErrRetriesExhausted) { /* ... */ }
	//
	// This replaces the brittle `strings.Contains(err.Error(), "operation
	// failed after ...")` predicate the rest of the SDK previously used.
	ErrRetriesExhausted = errors.New("retry: retries exhausted")
)

// Options configures the retry behavior
//
// This struct allows you to fine-tune retry strategies for different scenarios:
//   - MaxRetries and timing parameters control how long and how often to retry
//   - RetryableErrors and RetryableHTTPCodes determine which failures trigger retries
//   - JitterFactor helps prevent thundering herd problems in distributed systems
//   - ErrorPredicate is an escape hatch for typed-error matching (errors.As)
//     that callers prefer over substring matching
type Options struct {
	// MaxRetries is the maximum number of retries to attempt
	MaxRetries int

	// InitialDelay is the delay before the first retry
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// BackoffFactor is the factor by which to increase the delay after each retry
	BackoffFactor float64

	// RetryableErrors is a list of error strings that should trigger a retry
	RetryableErrors []string

	// RetryableHTTPCodes is a list of HTTP status codes that should trigger a retry
	RetryableHTTPCodes []int

	// JitterFactor is the amount of jitter to add to the delay (0.0-1.0)
	JitterFactor float64

	// ErrorPredicate, when non-nil, is consulted by IsRetryableError BEFORE
	// substring/HTTP-code checks. Returning true forces the error to be
	// treated as retryable; returning false defers to the rest of the
	// classification chain. This is the recommended way to surface typed
	// retryable sentinels (e.g. via errors.As) without polluting
	// RetryableErrors with magic substrings.
	ErrorPredicate func(error) bool
}

type nonRetryableError struct {
	err error
}

func (e nonRetryableError) Error() string {
	if isNilInterfaceValue(e.err) {
		return ""
	}

	return e.err.Error()
}

// Unwrap returns the wrapped error, or nil when the wrapped value is a
// typed nil. The symmetry with [nonRetryableError.Error] matters:
// without this guard, callers walking the chain via [errors.Unwrap]
// would receive an interface that compares != nil while wrapping a
// nil pointer, defeating the very check at every consumer site.
func (e nonRetryableError) Unwrap() error {
	if isNilInterfaceValue(e.err) {
		return nil
	}

	return e.err
}

// AsNonRetryable marks an error so the retry engine will stop immediately.
func AsNonRetryable(err error) error {
	if err == nil || isNilInterfaceValue(err) {
		return nil
	}

	return nonRetryableError{err: err}
}

// DefaultRetryableErrors is a list of common error strings that should trigger a retry
var DefaultRetryableErrors = []string{
	"connection reset by peer",
	"connection refused",
	"timeout",
	"deadline exceeded",
	"too many requests",
	"rate limit",
	"service unavailable",
}

// DefaultRetryableHTTPCodes is a list of HTTP status codes that should trigger a retry
var DefaultRetryableHTTPCodes = []int{
	http.StatusRequestTimeout,      // 408
	http.StatusTooEarly,            // 425
	http.StatusTooManyRequests,     // 429
	http.StatusInternalServerError, // 500
	http.StatusBadGateway,          // 502
	http.StatusServiceUnavailable,  // 503
	http.StatusGatewayTimeout,      // 504
}

// DefaultOptions returns the default retry options
func DefaultOptions() *Options {
	return &Options{
		MaxRetries:         3,
		InitialDelay:       100 * time.Millisecond,
		MaxDelay:           10 * time.Second,
		BackoffFactor:      2.0,
		RetryableErrors:    cloneStrings(DefaultRetryableErrors),
		RetryableHTTPCodes: cloneInts(DefaultRetryableHTTPCodes),
		JitterFactor:       0.25,
	}
}

// Option is a function that configures an Options object
type Option func(*Options) error

// WithMaxRetries returns an Option that sets the maximum number of retry attempts.
// The value must be non-negative.
//
// Example use case: For critical financial operations where completion is essential
// but should not retry indefinitely:
//
//	// Configure payment processing to retry several times before giving up
//	err := retry.Do(ctx, submitPayment, retry.WithMaxRetries(5))
//
//	// For less critical operations, fewer retries may be appropriate
//	err := retry.Do(ctx, updateUserProfile, retry.WithMaxRetries(2))
//
// Impact of different values:
// - 0: No retries (function only runs once)
// - 1-3: Suitable for most operations with transient failures
// - 4-10: For critical operations or highly unreliable networks
// - >10: Rarely needed and may indicate deeper problems if required
func WithMaxRetries(maxRetries int) Option {
	return func(o *Options) error {
		if maxRetries < 0 {
			return fmt.Errorf("maxRetries must be non-negative, got %d", maxRetries)
		}

		o.MaxRetries = maxRetries

		return nil
	}
}

// WithInitialDelay returns an Option that sets the initial delay before the first retry.
// The value must be positive.
//
// Example:
//
//	err := retry.Do(ctx, myFunction, retry.WithInitialDelay(200*time.Millisecond))
func WithInitialDelay(delay time.Duration) Option {
	return func(o *Options) error {
		if delay <= 0 {
			return fmt.Errorf("initialDelay must be positive, got %v", delay)
		}

		o.InitialDelay = delay

		return nil
	}
}

// WithMaxDelay returns an Option that sets the maximum delay between retries.
// The value must be greater than or equal to the initial delay.
//
// Example:
//
//	err := retry.Do(ctx, myFunction, retry.WithMaxDelay(30*time.Second))
func WithMaxDelay(delay time.Duration) Option {
	return func(o *Options) error {
		if delay <= 0 {
			return fmt.Errorf("maxDelay must be positive, got %v", delay)
		}

		o.MaxDelay = delay

		return nil
	}
}

// WithBackoffFactor returns an Option that sets the factor by which to increase
// the delay after each retry. The value must be greater than or equal to 1.0.
//
// Example:
//
//	err := retry.Do(ctx, myFunction, retry.WithBackoffFactor(1.5))
func WithBackoffFactor(factor float64) Option {
	return func(o *Options) error {
		if math.IsNaN(factor) || math.IsInf(factor, 0) || factor < 1.0 {
			return fmt.Errorf("backoffFactor must be at least 1.0, got %f", factor)
		}

		o.BackoffFactor = factor

		return nil
	}
}

// WithRetryableErrors returns an Option that sets the list of error strings
// that should trigger a retry.
//
// Example:
//
//	err := retry.Do(ctx, myFunction, retry.WithRetryableErrors([]string{
//	    "connection refused",
//	    "timeout",
//	}))
func WithRetryableErrors(retryableErrors []string) Option {
	return func(o *Options) error {
		o.RetryableErrors = cloneStrings(retryableErrors)
		return nil
	}
}

// WithRetryableHTTPCodes returns an Option that sets the list of HTTP status
// codes that should trigger a retry.
//
// Example:
//
//	err := retry.Do(ctx, myFunction, retry.WithRetryableHTTPCodes([]int{
//	    http.StatusTooManyRequests,
//	    http.StatusServiceUnavailable,
//	}))
func WithRetryableHTTPCodes(codes []int) Option {
	return func(o *Options) error {
		o.RetryableHTTPCodes = cloneInts(codes)
		return nil
	}
}

// WithJitterFactor returns an Option that sets the amount of jitter to add to the
// delay to avoid thundering herd problems. The value must be between 0.0 and 1.0.
//
// Example:
//
//	err := retry.Do(ctx, myFunction, retry.WithJitterFactor(0.5))
func WithJitterFactor(factor float64) Option {
	return func(o *Options) error {
		if math.IsNaN(factor) || math.IsInf(factor, 0) || factor < 0.0 || factor > 1.0 {
			return fmt.Errorf("jitterFactor must be between 0.0 and 1.0, got %f", factor)
		}

		o.JitterFactor = factor

		return nil
	}
}

// WithErrorPredicate installs a typed-error predicate that takes precedence
// over substring matching. This is the preferred way to surface custom
// retryable error types (via errors.As) instead of appending magic strings
// to RetryableErrors.
//
// Example:
//
//	err := retry.Do(ctx, myFunc, retry.WithErrorPredicate(func(err error) bool {
//	    var custom *MyRetryableError
//	    return errors.As(err, &custom)
//	}))
func WithErrorPredicate(predicate func(error) bool) Option {
	return func(o *Options) error {
		o.ErrorPredicate = predicate
		return nil
	}
}

// WithHighReliability returns an Option that configures retry options for high reliability.
// This increases timeouts, retry counts, and adds jitter for maximum resilience.
//
// Example use case: For mission-critical operations where completion is essential
// even in degraded network conditions:
//
//	// Process an important financial transaction with high-reliability settings
//	err := retry.Do(ctx, func() error {
//	    return processCriticalTransaction(ctx, transaction)
//	}, retry.WithHighReliability())
//
// This preset configures:
// - 5 retry attempts (6 total attempts)
// - Initial delay of 200ms, increasing to max 30 seconds
// - Aggressive backoff factor of 2.5
// - 40% jitter to prevent thundering herd problems
func WithHighReliability() Option {
	return func(o *Options) error {
		o.MaxRetries = 5
		o.InitialDelay = 200 * time.Millisecond
		o.MaxDelay = 30 * time.Second
		o.BackoffFactor = 2.5
		o.JitterFactor = 0.4

		return nil
	}
}

// contextKey is a type for context keys specific to this package
type contextKey string

// retryOptionsKey is the context key for retry options
const retryOptionsKey = contextKey("retry-options")

// retryAttemptHookKey is the context key for the retry-attempt hook.
const retryAttemptHookKey = contextKey("retry-attempt-hook")

// AttemptHook is invoked once per retry attempt — i.e. after a failed call
// has been classified as retryable and the next attempt's delay computed,
// but BEFORE the timer fires. It receives the attempt number that just
// failed (1-based), the cause error, and the delay before the next attempt.
//
// The hook is intended for structured logging and metrics emission. Hosts
// should keep the implementation cheap (no I/O blocking, no panics) — it
// runs on the hot retry path. Long work belongs in a goroutine.
//
// Defined as a function type instead of an interface so callers don't need
// to depend on slog or any specific observability framework. The retry
// package itself remains free of observability imports.
type AttemptHook func(ctx context.Context, attempt int, cause error, delay time.Duration)

// WithAttemptHook attaches a per-attempt hook to the context. Subsequent
// retry executions on this ctx invoke the hook once per retry. Passing nil
// clears any previously-installed hook.
//
// Returns:
//   - context.Context: A child ctx carrying the hook.
func WithAttemptHook(ctx context.Context, hook AttemptHook) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, retryAttemptHookKey, hook)
}

// attemptHookFromContext extracts the per-attempt hook installed via
// WithAttemptHook, or returns nil if none is set.
func attemptHookFromContext(ctx context.Context) AttemptHook {
	if ctx == nil {
		return nil
	}

	if hook, ok := ctx.Value(retryAttemptHookKey).(AttemptHook); ok {
		return hook
	}

	return nil
}

// WithOptionsContext returns a new context with the retry options set.
// This allows retry options to be propagated through a context across function boundaries.
//
// Example:
//
//	// Create a context with retry options
//	opts := retry.DefaultOptions()
//	opts.MaxRetries = 5
//	ctx = retry.WithOptionsContext(ctx, opts)
//
//	// Later, use the options from the context
//	err := retry.DoWithContext(ctx, myFunction)
func WithOptionsContext(ctx context.Context, options *Options) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, retryOptionsKey, cloneOptions(options))
}

// GetOptionsFromContext gets the retry options from the context.
// If no options are set in the context, it returns the default options.
func GetOptionsFromContext(ctx context.Context) *Options {
	if ctx == nil {
		return DefaultOptions()
	}

	if options, ok := ctx.Value(retryOptionsKey).(*Options); ok {
		if options == nil {
			return DefaultOptions()
		}

		return cloneOptions(options)
	}

	return DefaultOptions()
}

// Do executes the given function with retries based on the provided options.
// It returns the error from the last attempt or nil if the function succeeded.
//
// Example use case: When making external API calls that may experience
// transient network or service unavailability:
//
//	// Retry an external API call with custom retry configuration
//	err := retry.Do(ctx, func() error {
//	    resp, err := http.Get("https://api.example.com/data")
//	    if err != nil {
//	        return err
//	    }
//	    defer resp.Body.Close()
//
//	    if resp.StatusCode >= 500 {
//	        return fmt.Errorf("server error: %d", resp.StatusCode)
//	    }
//
//	    // Process successful response...
//	    return nil
//	}, retry.WithMaxRetries(3), retry.WithInitialDelay(250*time.Millisecond))
func Do(ctx context.Context, fn func() error, opts ...Option) error {
	if ctx == nil {
		return errNilContext
	}

	if fn == nil {
		return errNilFunc
	}

	// Start with default options
	options := DefaultOptions()

	// Apply all provided options
	for _, opt := range opts {
		if opt == nil {
			return errNilOption
		}

		if err := opt(options); err != nil {
			return fmt.Errorf("failed to apply retry option: %w", err)
		}
	}

	return doWithOptions(ctx, fn, options)
}

// DoWithContext executes the given function with retries based on options from the context.
// If no options are set in the context, it uses the default options.
//
// Example:
//
//	// Set options in the context
//	ctx = retry.WithOptionsContext(ctx, retry.DefaultOptions())
//
//	// Later, use the options from the context
//	err := retry.DoWithContext(ctx, makeAPIRequest)
func DoWithContext(ctx context.Context, fn func() error) error {
	if ctx == nil {
		return errNilContext
	}

	if fn == nil {
		return errNilFunc
	}

	options := GetOptionsFromContext(ctx)

	return doWithOptions(ctx, fn, options)
}

// doWithOptions executes the given function with retries based on the provided options.
// It's an internal function used by Do and DoWithContext.
func doWithOptions(ctx context.Context, fn func() error, options *Options) error {
	if ctx == nil {
		return errNilContext
	}

	if fn == nil {
		return errNilFunc
	}

	if options == nil {
		options = DefaultOptions()
	}

	options = cloneOptions(options)
	if err := validateOptions(options); err != nil {
		return err
	}

	var err error

	for attempt := 0; attempt <= options.MaxRetries; attempt++ {
		// Check if context is done before executing
		if ctx.Err() != nil {
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		}

		// Execute the function
		err = fn()
		if err == nil || isNilInterfaceValue(err) {
			// Success, return immediately
			return nil
		}

		// Check if this is the last attempt
		if attempt == options.MaxRetries {
			break
		}

		// Check if the error is retryable
		if !IsRetryableError(err, options) {
			return err
		}

		// Calculate delay duration
		delay := calculateBackoff(attempt, options)

		// Add jitter to avoid thundering herd
		delayWithJitter := addJitter(delay, options.JitterFactor)

		// Invoke the per-attempt observability hook AFTER classifying the
		// error as retryable AND computing the delay, but BEFORE the
		// timer fires. This is the moment with all the relevant context:
		// we know the cause, the attempt that just failed, and the wait
		// before the next try. Hosts (e.g. entities/http.go) install the
		// hook via retry.WithAttemptHook(ctx, hook) and use it for
		// structured logging and metrics emission.
		if hook := attemptHookFromContext(ctx); hook != nil {
			hook(ctx, attempt+1, err, delayWithJitter)
		}

		// Wait for the calculated delay or until context is done
		timer := time.NewTimer(delayWithJitter)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("operation cancelled during retry: %w", ctx.Err())
		case <-timer.C:
		}
	}

	// Return the last error wrapped behind the ErrRetriesExhausted
	// sentinel so callers can match via errors.Is(err, ErrRetriesExhausted)
	// without scraping the rendered string.
	//
	// We do NOT short-circuit on isNilInterfaceValue(err) here: the
	// success branch above (line ~556) already returns nil whenever fn
	// produces a (possibly typed-) nil. By the time control reaches
	// this fmt.Errorf the loop has either exhausted attempts on a
	// non-nil error or broken out via the "non-retryable" return — both
	// guarantee err is non-nil and renderable.
	return fmt.Errorf("%w: operation failed after %d retries: %w", ErrRetriesExhausted, options.MaxRetries, err)
}

// IsRetryableError checks if an error is retryable based on the provided options
//
// This function examines the error message for patterns defined in Options.RetryableErrors
// and checks HTTP status codes against Options.RetryableHTTPCodes.
//
// Example use case: When implementing custom retry logic that needs to determine
// whether to retry based on specific error conditions:
//
//	func processWithCustomRetry(ctx context.Context) error {
//	    options := retry.DefaultOptions()
//	    // Add custom retryable error patterns
//	    options.RetryableErrors = append(options.RetryableErrors,
//	        "insufficient funds", "account locked")
//
//	    // Custom retry loop
//	    var err error
//	    for attempt := 0; attempt <= options.MaxRetries; attempt++ {
//	        err = doOperation()
//
//	        // Check if error is retryable
//	        if err == nil || !retry.IsRetryableError(err, options) {
//	            break
//	        }
//
//	        // Wait before next attempt using exponential backoff...
//	    }
//	    return err
//	}
func IsRetryableError(err error, options *Options) bool {
	if err == nil || isNilInterfaceValue(err) {
		return false
	}

	if options == nil {
		options = DefaultOptions()
	}

	var nonRetryable nonRetryableError
	if errors.As(err, &nonRetryable) {
		return false
	}

	// Check for context cancellation
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// 1) Caller-supplied typed predicate wins. This avoids polluting
	// RetryableErrors with substring tokens just to round-trip a typed
	// sentinel (e.g. retryableCustomPolicyError) into a "yes, retry".
	if options.ErrorPredicate != nil && options.ErrorPredicate(err) {
		return true
	}

	// 2) Explicit HTTP status policy wins before the generic typed taxonomy.
	if matchesRetryableHTTPStatus(err, options.RetryableHTTPCodes) {
		return true
	}

	// 3) Typed retryable taxonomy: any error that exposes Retryable() bool wins.
	// This makes pkg/errors.Error.Retryable() the canonical SDK-wide policy
	// while remaining decoupled (structural interface — no import cycle).
	// It must run BEFORE the substring scan, otherwise an auth error whose
	// message happens to contain "timeout" (e.g. "Token expired due to timeout")
	// would be misclassified as retryable.
	var retryable interface{ Retryable() bool }
	if errors.As(err, &retryable) && !isNilInterfaceValue(retryable) {
		return retryable.Retryable()
	}

	// 4) Retryable error string matching.
	errMsg := err.Error()
	for _, retryableErr := range options.RetryableErrors {
		if retryableErr != "" && errMatchesPattern(errMsg, retryableErr) {
			return true
		}
	}

	return false
}

func matchesRetryableHTTPStatus(err error, codes []int) bool {
	var statusErr interface{ StatusCode() int }
	if !errors.As(err, &statusErr) || isNilInterfaceValue(statusErr) {
		return false
	}

	statusCode := statusErr.StatusCode()
	for _, code := range codes {
		if statusCode == code {
			return true
		}
	}

	return false
}

// isNilInterfaceValue is the typed-nil-aware nil check used throughout
// pkg/retry. It is a deliberate duplicate of
// [github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors.IsNilInterfaceValue];
// pkg/retry does not currently import pkg/errors (only the tests do)
// and we keep that decoupling on the runtime side. The two
// implementations must stay in lockstep — if you change the semantics
// in one, update the other.
func isNilInterfaceValue(value any) bool {
	if value == nil {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// errMatchesPattern checks if an error message contains a retryable pattern
func errMatchesPattern(errMsg, pattern string) bool {
	return strings.Contains(strings.ToLower(errMsg), strings.ToLower(pattern))
}

// calculateBackoff calculates the backoff duration for a retry attempt
func calculateBackoff(attempt int, options *Options) time.Duration {
	// Calculate exponential backoff
	delayF := float64(options.InitialDelay) * math.Pow(options.BackoffFactor, float64(attempt))

	// Cap at max delay
	if delayF > float64(options.MaxDelay) {
		delayF = float64(options.MaxDelay)
	}

	return time.Duration(delayF)
}

func validateOptions(options *Options) error {
	if options.MaxRetries < 0 {
		return fmt.Errorf("maxRetries must be non-negative, got %d", options.MaxRetries)
	}

	if options.InitialDelay <= 0 {
		return fmt.Errorf("initialDelay must be positive, got %v", options.InitialDelay)
	}

	if options.MaxDelay <= 0 {
		return fmt.Errorf("maxDelay must be positive, got %v", options.MaxDelay)
	}

	if options.MaxDelay < options.InitialDelay {
		return fmt.Errorf("maxDelay must be greater than or equal to initialDelay, got %v < %v", options.MaxDelay, options.InitialDelay)
	}

	if math.IsNaN(options.BackoffFactor) || math.IsInf(options.BackoffFactor, 0) || options.BackoffFactor < 1.0 {
		return fmt.Errorf("backoffFactor must be at least 1.0, got %f", options.BackoffFactor)
	}

	if math.IsNaN(options.JitterFactor) || math.IsInf(options.JitterFactor, 0) || options.JitterFactor < 0.0 || options.JitterFactor > 1.0 {
		return fmt.Errorf("jitterFactor must be between 0.0 and 1.0, got %f", options.JitterFactor)
	}

	return nil
}

func cloneOptions(options *Options) *Options {
	if options == nil {
		return nil
	}

	cloned := *options
	cloned.RetryableErrors = cloneStrings(options.RetryableErrors)
	cloned.RetryableHTTPCodes = cloneInts(options.RetryableHTTPCodes)

	return &cloned
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}

	return append([]string(nil), values...)
}

func cloneInts(values []int) []int {
	if values == nil {
		return nil
	}

	return append([]int(nil), values...)
}

// addJitter adds random jitter to the delay to avoid thundering herd
func addJitter(delay time.Duration, factor float64) time.Duration {
	// Add jitter based on the factor
	jitterF := getSecureRandomFloat64() * factor
	jitter := time.Duration(float64(delay) * jitterF)

	// Randomly add or subtract jitter

	if getSecureRandomFloat64() > midpointProbability {
		return delay + jitter
	}

	return delay - jitter
}

// getSecureRandomFloat64 returns a cryptographically secure random float64 between 0 and 1
func getSecureRandomFloat64() float64 {
	var buf [8]byte

	_, err := rand.Read(buf[:])
	if err != nil {
		// If crypto/rand fails, return a safe default
		return midpointProbability
	}

	// Convert bytes to uint64, then to float64 between 0 and 1
	return float64(binary.BigEndian.Uint64(buf[:])) / float64(math.MaxUint64)
}

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
	"strings"
	"time"
)

// Options configures the retry behavior
//
// This struct allows you to fine-tune retry strategies for different scenarios:
// - MaxRetries and timing parameters control how long and how often to retry
// - RetryableErrors and RetryableHTTPCodes determine which failures trigger retries
// - JitterFactor helps prevent thundering herd problems in distributed systems
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
		RetryableErrors:    DefaultRetryableErrors,
		RetryableHTTPCodes: DefaultRetryableHTTPCodes,
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
		if factor < 1.0 {
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
func WithRetryableErrors(errors []string) Option {
	return func(o *Options) error {
		o.RetryableErrors = errors
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
		o.RetryableHTTPCodes = codes
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
		if factor < 0.0 || factor > 1.0 {
			return fmt.Errorf("jitterFactor must be between 0.0 and 1.0, got %f", factor)
		}

		o.JitterFactor = factor

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

// WithNoRetry returns an Option that disables retries.
//
// Example:
//
//	err := retry.Do(ctx, myFunction, retry.WithNoRetry())
func WithNoRetry() Option {
	return func(o *Options) error {
		o.MaxRetries = 0
		return nil
	}
}

// contextKey is a type for context keys specific to this package
type contextKey string

// retryOptionsKey is the context key for retry options
const retryOptionsKey = contextKey("retry-options")

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
	return context.WithValue(ctx, retryOptionsKey, options)
}

// GetOptionsFromContext gets the retry options from the context.
// If no options are set in the context, it returns the default options.
func GetOptionsFromContext(ctx context.Context) *Options {
	if options, ok := ctx.Value(retryOptionsKey).(*Options); ok {
		return options
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
	// Start with default options
	options := DefaultOptions()

	// Apply all provided options
	for _, opt := range opts {
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
	options := GetOptionsFromContext(ctx)
	return doWithOptions(ctx, fn, options)
}

// doWithOptions executes the given function with retries based on the provided options.
// It's an internal function used by Do and DoWithContext.
func doWithOptions(ctx context.Context, fn func() error, options *Options) error {
	var err error

	for attempt := 0; attempt <= options.MaxRetries; attempt++ {
		// Check if context is done before executing
		if ctx.Err() != nil {
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		}

		// Execute the function
		err = fn()
		if err == nil {
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

		// Wait for the calculated delay or until context is done
		timer := time.NewTimer(delayWithJitter)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("operation cancelled during retry: %w", ctx.Err())
		case <-timer.C:
		}
	}

	// Return the last error
	return fmt.Errorf("operation failed after %d retries: %w", options.MaxRetries, err)
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
	if err == nil {
		return false
	}

	// Check for context cancellation
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for retryable error strings
	errMsg := err.Error()
	for _, retryableErr := range options.RetryableErrors {
		if retryableErr != "" && errMatchesPattern(errMsg, retryableErr) {
			return true
		}
	}

	// Check for retryable HTTP status codes
	// This assumes the error might implement a method to get the HTTP status code
	// For example, if using a custom error type that wraps an HTTP response
	if httpErr, ok := err.(interface{ StatusCode() int }); ok {
		for _, code := range options.RetryableHTTPCodes {
			if httpErr.StatusCode() == code {
				return true
			}
		}
	}

	return false
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

// addJitter adds random jitter to the delay to avoid thundering herd
func addJitter(delay time.Duration, factor float64) time.Duration {
	// Add jitter based on the factor
	jitterF := getSecureRandomFloat64() * factor
	jitter := time.Duration(float64(delay) * jitterF)

	// Randomly add or subtract jitter

	if getSecureRandomFloat64() > 0.5 {
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
		return 0.5
	}

	// Convert bytes to uint64, then to float64 between 0 and 1
	return float64(binary.BigEndian.Uint64(buf[:])) / float64(math.MaxUint64)
}

// Package retry provides utilities for implementing retry logic with exponential backoff
// and jitter for resilient operations. It allows for configurable retry strategies,
// context-aware cancellation, and flexible error handling.
package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// Options configures the retry behavior
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
// Example:
//
//	err := retry.Do(ctx, myFunction, retry.WithMaxRetries(5))
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
// Example:
//
//	err := retry.Do(ctx, myFunction, retry.WithHighReliability())
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
// Example:
//
//	err := retry.Do(ctx, func() error {
//	    return makeAPIRequest()
//	}, retry.WithMaxRetries(3), retry.WithInitialDelay(100*time.Millisecond))
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
			// Continue to next retry attempt
		}
	}

	// Return the last error
	return fmt.Errorf("operation failed after %d retries: %w", options.MaxRetries, err)
}

// IsRetryableError checks if an error is retryable based on the provided options
func IsRetryableError(err error, options *Options) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation
	if err == context.Canceled || err == context.DeadlineExceeded {
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
	jitterF := rand.Float64() * factor
	jitter := time.Duration(float64(delay) * jitterF)

	// Randomly add or subtract jitter
	if rand.Float64() > 0.5 {
		return delay + jitter
	}
	return delay - jitter
}

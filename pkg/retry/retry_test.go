package retry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestDo_Success tests successful execution with no retries
func TestDo_Success(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	// Function that succeeds on first try
	fn := func() error {
		callCount++
		return nil
	}

	err := Do(ctx, fn)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("Expected 1 call, got: %d", callCount)
	}
}

// TestDo_EventualSuccess tests successful execution after several retries
func TestDo_EventualSuccess(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	// Function that fails twice then succeeds
	fn := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary error: connection reset by peer")
		}

		return nil
	}

	err := Do(ctx, fn,
		WithMaxRetries(3),
		WithInitialDelay(1*time.Millisecond), // Fast retry for testing
		WithMaxDelay(5*time.Millisecond),
		WithBackoffFactor(2.0),
	)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if callCount != 3 {
		t.Fatalf("Expected 3 calls, got: %d", callCount)
	}
}

// TestDo_MaxRetriesExceeded tests when max retries are exceeded
func TestDo_MaxRetriesExceeded(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	// Function that always fails with a retryable error
	fn := func() error {
		callCount++
		return errors.New("temporary error: connection refused")
	}

	err := Do(ctx, fn,
		WithMaxRetries(2),
		WithInitialDelay(1*time.Millisecond),
		WithMaxDelay(5*time.Millisecond),
		WithBackoffFactor(2.0),
	)
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}

	// Initial attempt + 2 retries = 3 calls
	if callCount != 3 {
		t.Fatalf("Expected 3 calls, got: %d", callCount)
	}

	// Check error message contains info about retry count
	if !strings.Contains(err.Error(), "after 2 retries") {
		t.Fatalf("Expected error to mention retry count, got: %v", err)
	}
}

// TestDo_NonRetryableError tests handling of non-retryable errors
func TestDo_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	// Function that fails with a non-retryable error
	fn := func() error {
		callCount++
		return errors.New("non-retryable error")
	}

	err := Do(ctx, fn,
		WithMaxRetries(3),
		WithInitialDelay(1*time.Millisecond),
		WithMaxDelay(5*time.Millisecond),
		WithBackoffFactor(2.0),
		WithRetryableErrors([]string{"retryable error only"}), // Only retry on this specific error
	)
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}

	// Should only be called once since the error is not retryable
	if callCount != 1 {
		t.Fatalf("Expected 1 call, got: %d", callCount)
	}
}

// TestDo_ContextCancellation tests handling of context cancellation
func TestDo_ContextCancellation(t *testing.T) {
	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	// Function that will keep failing with a retryable error
	fn := func() error {
		callCount++
		return errors.New("temporary error: connection reset by peer")
	}

	// Cancel the context after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := Do(ctx, fn,
		WithMaxRetries(5),
		WithInitialDelay(50*time.Millisecond),
		WithMaxDelay(200*time.Millisecond),
		WithBackoffFactor(2.0),
	)

	// Verify the error is related to context cancellation
	if err == nil {
		t.Fatal("Expected context cancelled error, got nil")
	}

	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("Expected cancellation error, got: %v", err)
	}
}

// TestWithOptionsContext tests setting and getting options from context
func TestWithOptionsContext(t *testing.T) {
	baseCtx := context.Background()
	options := DefaultOptions()
	options.MaxRetries = 10
	options.InitialDelay = 200 * time.Millisecond
	options.MaxDelay = 30 * time.Second
	options.BackoffFactor = 1.5

	// Add options to context
	ctx := WithOptionsContext(baseCtx, options)

	// Get options from context
	retrievedOptions := GetOptionsFromContext(ctx)

	// Check that options match
	if retrievedOptions.MaxRetries != options.MaxRetries {
		t.Errorf("Expected MaxRetries %d, got %d", options.MaxRetries, retrievedOptions.MaxRetries)
	}

	if retrievedOptions.InitialDelay != options.InitialDelay {
		t.Errorf("Expected InitialDelay %v, got %v", options.InitialDelay, retrievedOptions.InitialDelay)
	}

	if retrievedOptions.MaxDelay != options.MaxDelay {
		t.Errorf("Expected MaxDelay %v, got %v", options.MaxDelay, retrievedOptions.MaxDelay)
	}

	if retrievedOptions.BackoffFactor != options.BackoffFactor {
		t.Errorf("Expected BackoffFactor %v, got %v", options.BackoffFactor, retrievedOptions.BackoffFactor)
	}
}

// TestGetOptionsFromContext_Default tests getting default options when none are set
func TestGetOptionsFromContext_Default(t *testing.T) {
	ctx := context.Background()
	options := GetOptionsFromContext(ctx)

	// Check that options match defaults
	defaultOptions := DefaultOptions()
	if options.MaxRetries != defaultOptions.MaxRetries {
		t.Errorf("Expected default MaxRetries %d, got %d", defaultOptions.MaxRetries, options.MaxRetries)
	}
}

// TestCalculateBackoff tests the backoff calculation
func TestCalculateBackoff(t *testing.T) {
	options := &Options{
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
	}

	// Test increasing backoff
	backoff0 := calculateBackoff(0, options)
	backoff1 := calculateBackoff(1, options)
	backoff2 := calculateBackoff(2, options)

	if backoff0 < options.InitialDelay {
		t.Fatalf("Expected backoff >= %v, got: %v", options.InitialDelay, backoff0)
	}

	if backoff1 <= backoff0 {
		t.Fatalf("Expected increasing backoff, got: %v <= %v", backoff1, backoff0)
	}

	if backoff2 <= backoff1 {
		t.Fatalf("Expected increasing backoff, got: %v <= %v", backoff2, backoff1)
	}

	// Test max backoff cap
	backoff10 := calculateBackoff(10, options) // Should hit max
	if backoff10 > options.MaxDelay {
		t.Fatalf("Expected backoff <= %v, got: %v", options.MaxDelay, backoff10)
	}
}

// TestIsRetryableError tests the error matching logic
func TestIsRetryableError(t *testing.T) {
	// Use explicit options rather than defaults to avoid test failures if defaults change
	options := &Options{
		MaxRetries:         3,
		InitialDelay:       100 * time.Millisecond,
		MaxDelay:           10 * time.Second,
		BackoffFactor:      2.0,
		RetryableErrors:    []string{"connection reset", "connection refused", "timeout"},
		RetryableHTTPCodes: []int{http.StatusServiceUnavailable, http.StatusTooManyRequests},
	}

	// Test nil error
	if IsRetryableError(nil, options) {
		t.Error("nil error should not be retryable")
	}

	// Test context errors
	if IsRetryableError(context.Canceled, options) {
		t.Error("context.Canceled should not be retryable")
	}

	if IsRetryableError(context.DeadlineExceeded, options) {
		t.Error("context.DeadlineExceeded should not be retryable")
	}

	// Test retryable error string
	retryableErrors := []string{
		"connection reset by peer",
		"error: connection refused",
		"timeout during operation",
	}
	for _, errMsg := range retryableErrors {
		err := errors.New(errMsg)
		if !IsRetryableError(err, options) {
			t.Errorf("Error containing retryable pattern should be retryable, but wasn't: %v", err)
		}
	}

	// Test non-retryable error
	err := errors.New("some completely different error")
	if IsRetryableError(err, options) {
		t.Errorf("Error '%v' should not be retryable", err)
	}

	// Test HTTP error with retryable status code
	for _, code := range options.RetryableHTTPCodes {
		httpErr := mockHTTPError{statusCode: code}
		if !IsRetryableError(httpErr, options) {
			t.Errorf("HTTP error with status %d should be retryable", code)
		}
	}

	// Test HTTP error with non-retryable status code
	httpErr := mockHTTPError{statusCode: http.StatusBadRequest}
	if IsRetryableError(httpErr, options) {
		t.Errorf("HTTP error with status %d should not be retryable", httpErr.statusCode)
	}
}

// Test the helper functions for options
func TestOptionHelpers(t *testing.T) {
	tests := []struct {
		name    string
		option  Option
		check   func(*Options) bool
		wantErr bool
	}{
		{
			name:   "WithMaxRetries valid",
			option: WithMaxRetries(5),
			check: func(o *Options) bool {
				return o.MaxRetries == 5
			},
			wantErr: false,
		},
		{
			name:    "WithMaxRetries invalid",
			option:  WithMaxRetries(-1),
			check:   func(_ *Options) bool { return true },
			wantErr: true,
		},
		{
			name:   "WithInitialDelay valid",
			option: WithInitialDelay(200 * time.Millisecond),
			check: func(o *Options) bool {
				return o.InitialDelay == 200*time.Millisecond
			},
			wantErr: false,
		},
		{
			name:    "WithInitialDelay invalid",
			option:  WithInitialDelay(0),
			check:   func(_ *Options) bool { return true },
			wantErr: true,
		},
		{
			name:   "WithMaxDelay valid",
			option: WithMaxDelay(5 * time.Second),
			check: func(o *Options) bool {
				return o.MaxDelay == 5*time.Second
			},
			wantErr: false,
		},
		{
			name:    "WithMaxDelay invalid",
			option:  WithMaxDelay(-1 * time.Second),
			check:   func(_ *Options) bool { return true },
			wantErr: true,
		},
		{
			name:   "WithBackoffFactor valid",
			option: WithBackoffFactor(1.5),
			check: func(o *Options) bool {
				return o.BackoffFactor == 1.5
			},
			wantErr: false,
		},
		{
			name:    "WithBackoffFactor invalid",
			option:  WithBackoffFactor(0.5),
			check:   func(_ *Options) bool { return true },
			wantErr: true,
		},
		{
			name:   "WithJitterFactor valid",
			option: WithJitterFactor(0.5),
			check: func(o *Options) bool {
				return o.JitterFactor == 0.5
			},
			wantErr: false,
		},
		{
			name:    "WithJitterFactor invalid high",
			option:  WithJitterFactor(1.5),
			check:   func(_ *Options) bool { return true },
			wantErr: true,
		},
		{
			name:    "WithJitterFactor invalid low",
			option:  WithJitterFactor(-0.5),
			check:   func(_ *Options) bool { return true },
			wantErr: true,
		},
		{
			name:   "WithRetryableErrors",
			option: WithRetryableErrors([]string{"one", "two"}),
			check: func(o *Options) bool {
				return len(o.RetryableErrors) == 2 &&
					o.RetryableErrors[0] == "one" &&
					o.RetryableErrors[1] == "two"
			},
			wantErr: false,
		},
		{
			name:   "WithRetryableHTTPCodes",
			option: WithRetryableHTTPCodes([]int{500, 503}),
			check: func(o *Options) bool {
				return len(o.RetryableHTTPCodes) == 2 &&
					o.RetryableHTTPCodes[0] == 500 &&
					o.RetryableHTTPCodes[1] == 503
			},
			wantErr: false,
		},
		{
			name:   "WithHighReliability",
			option: WithHighReliability(),
			check: func(o *Options) bool {
				return o.MaxRetries == 5 &&
					o.BackoffFactor > 2.0 &&
					o.JitterFactor > 0.3
			},
			wantErr: false,
		},
		{
			name:   "WithNoRetry",
			option: WithNoRetry(),
			check: func(o *Options) bool {
				return o.MaxRetries == 0
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultOptions()
			err := tt.option(opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("Option() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !tt.check(opts) {
				t.Errorf("Option didn't set the expected value")
			}
		})
	}
}

// Test the HTTP options helper functions
func TestHTTPOptionHelpers(t *testing.T) {
	tests := []struct {
		name    string
		option  HTTPOption
		check   func(*HTTPOptions) bool
		wantErr bool
	}{
		{
			name:   "WithHTTPMaxRetries valid",
			option: WithHTTPMaxRetries(5),
			check: func(o *HTTPOptions) bool {
				return o.MaxRetries == 5
			},
			wantErr: false,
		},
		{
			name:    "WithHTTPMaxRetries invalid",
			option:  WithHTTPMaxRetries(-1),
			check:   func(_ *HTTPOptions) bool { return true },
			wantErr: true,
		},
		{
			name:   "WithHTTPInitialDelay valid",
			option: WithHTTPInitialDelay(200 * time.Millisecond),
			check: func(o *HTTPOptions) bool {
				return o.InitialDelay == 200*time.Millisecond
			},
			wantErr: false,
		},
		{
			name:    "WithHTTPInitialDelay invalid",
			option:  WithHTTPInitialDelay(0),
			check:   func(_ *HTTPOptions) bool { return true },
			wantErr: true,
		},
		{
			name:   "WithHTTPMaxDelay valid",
			option: WithHTTPMaxDelay(5 * time.Second),
			check: func(o *HTTPOptions) bool {
				return o.MaxDelay == 5*time.Second
			},
			wantErr: false,
		},
		{
			name:    "WithHTTPMaxDelay invalid",
			option:  WithHTTPMaxDelay(-1 * time.Second),
			check:   func(_ *HTTPOptions) bool { return true },
			wantErr: true,
		},
		{
			name:   "WithHTTPBackoffFactor valid",
			option: WithHTTPBackoffFactor(1.5),
			check: func(o *HTTPOptions) bool {
				return o.BackoffFactor == 1.5
			},
			wantErr: false,
		},
		{
			name:    "WithHTTPBackoffFactor invalid",
			option:  WithHTTPBackoffFactor(0.5),
			check:   func(_ *HTTPOptions) bool { return true },
			wantErr: true,
		},
		{
			name:   "WithHTTPJitterFactor valid",
			option: WithHTTPJitterFactor(0.5),
			check: func(o *HTTPOptions) bool {
				return o.JitterFactor == 0.5
			},
			wantErr: false,
		},
		{
			name:    "WithHTTPJitterFactor invalid high",
			option:  WithHTTPJitterFactor(1.5),
			check:   func(_ *HTTPOptions) bool { return true },
			wantErr: true,
		},
		{
			name:    "WithHTTPJitterFactor invalid low",
			option:  WithHTTPJitterFactor(-0.5),
			check:   func(_ *HTTPOptions) bool { return true },
			wantErr: true,
		},
		{
			name:   "WithHTTPRetryableHTTPCodes",
			option: WithHTTPRetryableHTTPCodes([]int{500, 503}),
			check: func(o *HTTPOptions) bool {
				return len(o.RetryableHTTPCodes) == 2 &&
					o.RetryableHTTPCodes[0] == 500 &&
					o.RetryableHTTPCodes[1] == 503
			},
			wantErr: false,
		},
		{
			name:   "WithHTTPRetryableNetworkErrors",
			option: WithHTTPRetryableNetworkErrors([]string{"one", "two"}),
			check: func(o *HTTPOptions) bool {
				return len(o.RetryableNetworkErrors) == 2 &&
					o.RetryableNetworkErrors[0] == "one" &&
					o.RetryableNetworkErrors[1] == "two"
			},
			wantErr: false,
		},
		{
			name:   "WithHTTPRetryAllServerErrors",
			option: WithHTTPRetryAllServerErrors(true),
			check: func(o *HTTPOptions) bool {
				return o.RetryAllServerErrors
			},
			wantErr: false,
		},
		{
			name:   "WithHTTPRetryOn4xx",
			option: WithHTTPRetryOn4xx([]int{429, 408}),
			check: func(o *HTTPOptions) bool {
				return len(o.RetryOn4xx) == 2 &&
					o.RetryOn4xx[0] == 429 &&
					o.RetryOn4xx[1] == 408
			},
			wantErr: false,
		},
		{
			name: "WithHTTPPreRetryHook",
			option: WithHTTPPreRetryHook(func(_ context.Context, _ *http.Request, _ *HTTPResponse) error {
				return nil
			}),
			check: func(o *HTTPOptions) bool {
				return o.PreRetryHook != nil
			},
			wantErr: false,
		},
		{
			name:   "WithHTTPHighReliability",
			option: WithHTTPHighReliability(),
			check: func(o *HTTPOptions) bool {
				return o.MaxRetries == 5 &&
					o.BackoffFactor > 2.0 &&
					o.JitterFactor > 0.3 &&
					o.RetryAllServerErrors
			},
			wantErr: false,
		},
		{
			name:   "WithHTTPNoRetry",
			option: WithHTTPNoRetry(),
			check: func(o *HTTPOptions) bool {
				return o.MaxRetries == 0
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultHTTPOptions()
			err := tt.option(opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("Option() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !tt.check(opts) {
				t.Errorf("Option didn't set the expected value")
			}
		})
	}
}

// mockHTTPError is a mock error that implements StatusCode() for testing
type mockHTTPError struct {
	statusCode int
}

func (e mockHTTPError) Error() string {
	return fmt.Sprintf("HTTP error: %d", e.statusCode)
}

func (e mockHTTPError) StatusCode() int {
	return e.statusCode
}

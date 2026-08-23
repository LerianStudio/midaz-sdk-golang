package retry

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

func nilContext() context.Context {
	return nil
}

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

func TestDo_TypedNilErrorTreatedAsSuccess(t *testing.T) {
	var typedNil *typedNilRetryClassifierError

	err := Do(context.Background(), func() error {
		return typedNil
	}, WithMaxRetries(0))

	require.NoError(t, err)
}

func TestAsNonRetryable_TypedNilReturnsNil(t *testing.T) {
	var typedNil *typedNilRetryClassifierError

	require.NoError(t, AsNonRetryable(typedNil))
}

// TestNonRetryableError_TypedNilErrReturnsEmptyString pins the Error()
// guard on nonRetryableError: a typed-nil inner must render as the
// empty string instead of panicking on the inner pointer dereference.
func TestNonRetryableError_TypedNilErrReturnsEmptyString(t *testing.T) {
	var typedNil *typedNilRetryClassifierError
	wrapper := nonRetryableError{err: typedNil}

	assert.Empty(t, wrapper.Error())
}

// TestNonRetryableError_UnwrapWithTypedNilReturnsNil pins the symmetric
// guard on Unwrap(): callers walking the chain must see a comparable
// nil, not an interface that carries a typed-nil pointer.
func TestNonRetryableError_UnwrapWithTypedNilReturnsNil(t *testing.T) {
	var typedNil *typedNilRetryClassifierError
	wrapper := nonRetryableError{err: typedNil}

	require.NoError(t, wrapper.Unwrap())
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

func TestRetryExportedAPIs_NilInputsDoNotPanic(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "Do with nil context returns error",
			run: func() error {
				return Do(nilContext(), func() error { return nil })
			},
		},
		{
			name: "Do with nil function returns error",
			run: func() error {
				return Do(context.Background(), nil)
			},
		},
		{
			name: "Do with nil option returns error",
			run: func() error {
				return Do(context.Background(), func() error { return nil }, nil)
			},
		},
		{
			name: "DoWithContext with nil context returns error",
			run: func() error {
				return DoWithContext(nilContext(), func() error { return nil })
			},
		},
		{
			name: "DoWithContext with nil function returns error",
			run: func() error {
				return DoWithContext(context.Background(), nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("expected error instead of panic, got panic: %v", recovered)
				}
			}()

			if err := tt.run(); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestRetryContextOptions_NilInputsReturnDefaults(t *testing.T) {
	t.Run("WithOptionsContext nil parent uses background", func(t *testing.T) {
		ctx := WithOptionsContext(nilContext(), &Options{MaxRetries: 9})
		if ctx == nil {
			t.Fatal("expected context, got nil")
		}

		options := GetOptionsFromContext(ctx)
		if options.MaxRetries != 9 {
			t.Fatalf("expected stored max retries, got %d", options.MaxRetries)
		}
	})

	t.Run("GetOptionsFromContext nil context returns defaults", func(t *testing.T) {
		options := GetOptionsFromContext(nilContext())
		if options == nil {
			t.Fatal("expected default options, got nil")
		}

		if options.MaxRetries != DefaultOptions().MaxRetries {
			t.Fatalf("expected default max retries, got %d", options.MaxRetries)
		}
	})

	t.Run("nil context-stored options return defaults", func(t *testing.T) {
		ctx := WithOptionsContext(context.Background(), nil)

		options := GetOptionsFromContext(ctx)
		if options == nil {
			t.Fatal("expected default options, got nil")
		}

		if options.MaxRetries != DefaultOptions().MaxRetries {
			t.Fatalf("expected default max retries, got %d", options.MaxRetries)
		}
	})
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

func TestProperty_CalculateBackoff_DeterministicBounds(t *testing.T) {
	tests := []struct {
		name    string
		options *Options
	}{
		{
			name: "doubling backoff capped at max",
			options: &Options{
				InitialDelay:  100 * time.Millisecond,
				MaxDelay:      750 * time.Millisecond,
				BackoffFactor: 2,
			},
		},
		{
			name: "linear backoff remains at initial delay",
			options: &Options{
				InitialDelay:  250 * time.Millisecond,
				MaxDelay:      time.Second,
				BackoffFactor: 1,
			},
		},
		{
			name: "fractional backoff is monotonic and capped",
			options: &Options{
				InitialDelay:  80 * time.Millisecond,
				MaxDelay:      500 * time.Millisecond,
				BackoffFactor: 1.5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkBackoffBoundsProperty(t, tt.options)
		})
	}
}

func checkBackoffBoundsProperty(t *testing.T, options *Options) {
	t.Helper()

	property := func(maxAttempt uint8) bool {
		return backoffSequenceWithinBounds(int(maxAttempt%20)+1, options)
	}

	config := &quick.Config{MaxCount: 64, Rand: rand.New(rand.NewSource(1))}
	require.NoError(t, quick.Check(property, config), "backoff bounds property failed")
}

func backoffSequenceWithinBounds(attempts int, options *Options) bool {
	previous := time.Duration(0)

	for attempt := range attempts {
		got := calculateBackoff(attempt, options)
		if got < options.InitialDelay || got > options.MaxDelay || got < previous {
			return false
		}
		previous = got
	}

	return true
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
		RetryableHTTPCodes: []int{http.StatusServiceUnavailable, http.StatusTooManyRequests, http.StatusTooEarly},
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
			// In v3, WithNoRetry was deleted. Use WithMaxRetries(0) directly —
			// the canonical "no retries" expression. This test ensures the
			// equivalent semantic is still reachable through the surviving
			// surface.
			name:   "WithMaxRetries(0)",
			option: WithMaxRetries(0),
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
			// In v3, WithHTTPNoRetry was deleted. Use WithHTTPMaxRetries(0)
			// directly — the canonical "no retries" expression for HTTP.
			name:   "WithHTTPMaxRetries(0)",
			option: WithHTTPMaxRetries(0),
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

type nonRetryableHTTPStatusError struct {
	statusCode int
}

type typedNilRetryClassifierError struct {
	statusCode int
}

type retryWrapperError struct {
	inner error
}

func (e mockHTTPError) Error() string {
	return fmt.Sprintf("HTTP error: %d", e.statusCode)
}

func (e mockHTTPError) StatusCode() int {
	return e.statusCode
}

func (e nonRetryableHTTPStatusError) Error() string {
	return fmt.Sprintf("HTTP error: %d", e.statusCode)
}

func (e nonRetryableHTTPStatusError) StatusCode() int {
	return e.statusCode
}

func (nonRetryableHTTPStatusError) Retryable() bool {
	return false
}

func (e *typedNilRetryClassifierError) Error() string {
	if e == nil {
		return "typed nil retry classifier"
	}

	return fmt.Sprintf("HTTP error: %d", e.statusCode)
}

func (e *typedNilRetryClassifierError) StatusCode() int {
	return e.statusCode
}

func (e *typedNilRetryClassifierError) Retryable() bool {
	return e.statusCode >= http.StatusInternalServerError
}

func (retryWrapperError) Error() string {
	return "wrapped retry classifier"
}

func (e retryWrapperError) Unwrap() error {
	return e.inner
}

// TestIsRetryableError_TypedRetryable_OverridesSubstringMatch verifies that
// the typed Retryable() taxonomy wins over the substring scan. An auth error
// whose Message contains "timeout" must NOT be retryable, even though the
// default RetryableErrors list contains "timeout".
//
// Regression: previously the substring scan ran first and would misclassify
// a 401 with body "Token expired due to timeout, please re-authenticate" as
// retryable, wasting retry budget and (for non-idempotent POSTs) risking
// double-bookkeeping.
func TestIsRetryableError_TypedRetryable_OverridesSubstringMatch(t *testing.T) {
	authErr := &sdkerrors.Error{
		Category: sdkerrors.CategoryAuth,
		Code:     sdkerrors.CodeAuthentication,
		Message:  "Token expired due to timeout, please re-authenticate",
	}

	// Sanity: the substring scan WOULD have matched ("timeout" is a default token).
	require.Contains(t, strings.ToLower(authErr.Error()), "timeout",
		"precondition: error message must contain the substring that previously caused the bug")

	// The typed taxonomy must override the substring match.
	if IsRetryableError(authErr, DefaultOptions()) {
		t.Fatalf("auth error with 'timeout' in message must NOT be retryable; "+
			"typed Retryable() should override DefaultRetryableErrors substring scan; got err=%v", authErr)
	}
}

// TestIsRetryableError_TypedRetryable_NetworkCategory verifies that a typed
// network error is recognised as retryable through the structural Retryable()
// interface even when its message contains no recognised substring tokens.
func TestIsRetryableError_TypedRetryable_NetworkCategory(t *testing.T) {
	netErr := &sdkerrors.Error{
		Category: sdkerrors.CategoryNetwork,
		Code:     sdkerrors.CodeNetwork,
		Message:  "no recognised tokens here",
	}

	if !IsRetryableError(netErr, DefaultOptions()) {
		t.Fatalf("typed network error must be retryable via Error.Retryable(); got err=%v", netErr)
	}
}

// TestIsRetryableError_TypedRetryable_ValidationNotRetryable verifies the
// non-retryable side of the taxonomy: validation errors must not retry, even
// when their message contains "timeout"-like tokens.
func TestIsRetryableError_TypedRetryable_ValidationNotRetryable(t *testing.T) {
	valErr := &sdkerrors.Error{
		Category: sdkerrors.CategoryValidation,
		Code:     sdkerrors.CodeValidation,
		Message:  "field 'deadline' must be a positive timeout duration",
	}

	if IsRetryableError(valErr, DefaultOptions()) {
		t.Fatalf("typed validation error must NOT be retryable, even when message contains 'timeout'; got err=%v", valErr)
	}
}

// TestIsRetryableError_StructuralStatusCode_UnwrapsViaErrorsAs verifies that
// the structural StatusCode() interface assertion now uses errors.As, so a
// retryable HTTP error wrapped via fmt.Errorf("...%w", ...) is still detected.
//
// Regression: a bare type assertion (err.(interface{StatusCode() int})) would
// fail to see through %w wrapping.
func TestIsRetryableError_StructuralStatusCode_UnwrapsViaErrorsAs(t *testing.T) {
	options := &Options{
		MaxRetries:         3,
		InitialDelay:       1 * time.Millisecond,
		MaxDelay:           5 * time.Millisecond,
		BackoffFactor:      2.0,
		RetryableErrors:    []string{}, // disable substring scan for this test
		RetryableHTTPCodes: []int{http.StatusServiceUnavailable},
	}

	httpErr := mockHTTPError{statusCode: http.StatusServiceUnavailable}
	wrapped := fmt.Errorf("transport call failed: %w", httpErr)

	if !IsRetryableError(wrapped, options) {
		t.Fatalf("wrapped HTTP error with retryable status code must be detected via errors.As; got err=%v", wrapped)
	}

	// Negative: non-retryable status code wrapped the same way must not retry.
	nonRetryable := fmt.Errorf("transport call failed: %w", mockHTTPError{statusCode: http.StatusBadRequest})
	if IsRetryableError(nonRetryable, options) {
		t.Fatalf("wrapped HTTP error with 400 must NOT be retryable; got err=%v", nonRetryable)
	}
}

func TestIsRetryableError_ExplicitHTTPStatusOverridesTypedNonRetryable(t *testing.T) {
	options := &Options{
		MaxRetries:         1,
		InitialDelay:       time.Millisecond,
		MaxDelay:           time.Millisecond,
		BackoffFactor:      1,
		RetryableErrors:    []string{},
		RetryableHTTPCodes: []int{http.StatusConflict},
	}

	retryableConflict := nonRetryableHTTPStatusError{statusCode: http.StatusConflict}
	if !IsRetryableError(retryableConflict, options) {
		t.Fatalf("explicit RetryableHTTPCodes must override typed Retryable() false; got err=%v", retryableConflict)
	}

	nonRetryableUnprocessable := nonRetryableHTTPStatusError{statusCode: http.StatusUnprocessableEntity}
	if IsRetryableError(nonRetryableUnprocessable, options) {
		t.Fatalf("non-configured status must still honor typed Retryable() false; got err=%v", nonRetryableUnprocessable)
	}
}

func TestIsRetryableError_TypedNilStructuralInterfacesDoNotPanic(t *testing.T) {
	var typedNil *typedNilRetryClassifierError
	var direct error = typedNil
	wrapped := retryWrapperError{inner: typedNil}

	options := &Options{
		MaxRetries:         1,
		InitialDelay:       time.Millisecond,
		MaxDelay:           time.Millisecond,
		BackoffFactor:      1,
		RetryableErrors:    []string{},
		RetryableHTTPCodes: []int{http.StatusServiceUnavailable},
	}

	require.NotPanics(t, func() {
		if IsRetryableError(direct, options) {
			t.Fatal("typed-nil direct error must not be retryable")
		}
	})

	require.NotPanics(t, func() {
		if IsRetryableError(wrapped, options) {
			t.Fatal("wrapped typed-nil structural error must not be retryable")
		}
	})
}

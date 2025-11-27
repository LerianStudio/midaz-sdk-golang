package transaction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	pkgerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultBatchOptions tests the default batch options
func TestDefaultBatchOptions(t *testing.T) {
	opts := DefaultBatchOptions()

	require.NotNil(t, opts)
	assert.Equal(t, 10, opts.Concurrency)
	assert.Equal(t, 100, opts.BatchSize)
	assert.Equal(t, 3, opts.RetryCount)
	assert.Equal(t, 100*time.Millisecond, opts.RetryDelay)
	assert.Equal(t, "batch", opts.IdempotencyKeyPrefix)
	assert.False(t, opts.StopOnError)
	assert.Nil(t, opts.OnProgress)
}

// TestBatchOptionsFields tests BatchOptions struct fields
func TestBatchOptionsFields(t *testing.T) {
	progressCalled := false
	onProgress := func(completed, total int, result BatchResult) {
		progressCalled = true
	}

	opts := &BatchOptions{
		Concurrency:          5,
		BatchSize:            50,
		RetryCount:           2,
		RetryDelay:           200 * time.Millisecond,
		OnProgress:           onProgress,
		IdempotencyKeyPrefix: "custom-prefix",
		StopOnError:          true,
	}

	assert.Equal(t, 5, opts.Concurrency)
	assert.Equal(t, 50, opts.BatchSize)
	assert.Equal(t, 2, opts.RetryCount)
	assert.Equal(t, 200*time.Millisecond, opts.RetryDelay)
	assert.NotNil(t, opts.OnProgress)
	assert.Equal(t, "custom-prefix", opts.IdempotencyKeyPrefix)
	assert.True(t, opts.StopOnError)

	// Test that onProgress callback works
	opts.OnProgress(1, 10, BatchResult{})
	assert.True(t, progressCalled)
}

// TestBatchResultFields tests BatchResult struct fields
func TestBatchResultStructFields(t *testing.T) {
	err := errors.New("test error")
	result := BatchResult{
		Index:         3,
		TransactionID: "tx-456",
		Error:         err,
		Duration:      500 * time.Millisecond,
	}

	assert.Equal(t, 3, result.Index)
	assert.Equal(t, "tx-456", result.TransactionID)
	assert.Equal(t, err, result.Error)
	assert.Equal(t, 500*time.Millisecond, result.Duration)
}

// TestBatchSummaryFields tests BatchSummary struct fields
func TestBatchSummaryFields(t *testing.T) {
	summary := BatchSummary{
		TotalTransactions:     100,
		SuccessCount:          90,
		ErrorCount:            10,
		SuccessRate:           90.0,
		TotalDuration:         10 * time.Second,
		AverageDuration:       100 * time.Millisecond,
		TransactionsPerSecond: 9.0,
		ErrorCategories: map[string]int{
			"validation": 5,
			"network":    3,
			"internal":   2,
		},
	}

	assert.Equal(t, 100, summary.TotalTransactions)
	assert.Equal(t, 90, summary.SuccessCount)
	assert.Equal(t, 10, summary.ErrorCount)
	assert.Equal(t, 90.0, summary.SuccessRate)
	assert.Equal(t, 10*time.Second, summary.TotalDuration)
	assert.Equal(t, 100*time.Millisecond, summary.AverageDuration)
	assert.Equal(t, 9.0, summary.TransactionsPerSecond)
	assert.Equal(t, 5, summary.ErrorCategories["validation"])
	assert.Equal(t, 3, summary.ErrorCategories["network"])
	assert.Equal(t, 2, summary.ErrorCategories["internal"])
}

// TestGetBatchSummary tests the GetBatchSummary function
func TestGetBatchSummary(t *testing.T) {
	t.Run("empty results", func(t *testing.T) {
		results := []BatchResult{}
		summary := GetBatchSummary(results)

		assert.Equal(t, 0, summary.TotalTransactions)
		assert.Equal(t, 0, summary.SuccessCount)
		assert.Equal(t, 0, summary.ErrorCount)
		assert.Equal(t, float64(0), summary.SuccessRate)
		assert.Empty(t, summary.ErrorCategories)
	})

	t.Run("all success", func(t *testing.T) {
		results := []BatchResult{
			{Index: 0, TransactionID: "tx-1", Duration: 100 * time.Millisecond},
			{Index: 1, TransactionID: "tx-2", Duration: 100 * time.Millisecond},
			{Index: 2, TransactionID: "tx-3", Duration: 100 * time.Millisecond},
		}
		summary := GetBatchSummary(results)

		assert.Equal(t, 3, summary.TotalTransactions)
		assert.Equal(t, 3, summary.SuccessCount)
		assert.Equal(t, 0, summary.ErrorCount)
		assert.Equal(t, float64(100), summary.SuccessRate)
		assert.Equal(t, 300*time.Millisecond, summary.TotalDuration)
		assert.Equal(t, 100*time.Millisecond, summary.AverageDuration)
	})

	t.Run("all failures", func(t *testing.T) {
		results := []BatchResult{
			{Index: 0, Error: errors.New("error 1"), Duration: 50 * time.Millisecond},
			{Index: 1, Error: errors.New("error 2"), Duration: 50 * time.Millisecond},
		}
		summary := GetBatchSummary(results)

		assert.Equal(t, 2, summary.TotalTransactions)
		assert.Equal(t, 0, summary.SuccessCount)
		assert.Equal(t, 2, summary.ErrorCount)
		assert.Equal(t, float64(0), summary.SuccessRate)
		// TPS is based on success count, so 0 successes means 0 TPS
		assert.Equal(t, float64(0), summary.TransactionsPerSecond)
	})

	t.Run("mixed results", func(t *testing.T) {
		results := []BatchResult{
			{Index: 0, TransactionID: "tx-1", Duration: 100 * time.Millisecond},
			{Index: 1, Error: errors.New("error 1"), Duration: 50 * time.Millisecond},
			{Index: 2, TransactionID: "tx-2", Duration: 100 * time.Millisecond},
			{Index: 3, Error: errors.New("error 2"), Duration: 50 * time.Millisecond},
		}
		summary := GetBatchSummary(results)

		assert.Equal(t, 4, summary.TotalTransactions)
		assert.Equal(t, 2, summary.SuccessCount)
		assert.Equal(t, 2, summary.ErrorCount)
		assert.Equal(t, float64(50), summary.SuccessRate)
	})

	t.Run("error categorization with typed errors", func(t *testing.T) {
		validationErr := pkgerrors.NewValidationError("test", "validation failed", nil)
		networkErr := pkgerrors.NewNetworkError("test", nil)
		timeoutErr := pkgerrors.NewTimeoutError("test", "timeout", nil)

		results := []BatchResult{
			{Index: 0, TransactionID: "tx-1", Duration: 100 * time.Millisecond},
			{Index: 1, Error: validationErr, Duration: 50 * time.Millisecond},
			{Index: 2, Error: networkErr, Duration: 50 * time.Millisecond},
			{Index: 3, Error: timeoutErr, Duration: 50 * time.Millisecond},
		}
		summary := GetBatchSummary(results)

		assert.Equal(t, 4, summary.TotalTransactions)
		assert.Equal(t, 1, summary.SuccessCount)
		assert.Equal(t, 3, summary.ErrorCount)
		assert.NotEmpty(t, summary.ErrorCategories)
	})
}

// TestNormalizeOptions tests the normalizeOptions function
func TestNormalizeOptions(t *testing.T) {
	t.Run("nil options returns defaults", func(t *testing.T) {
		opts := normalizeOptions(nil)

		require.NotNil(t, opts)
		assert.Equal(t, 10, opts.Concurrency)
		assert.Equal(t, 100, opts.BatchSize)
	})

	t.Run("zero concurrency becomes 1", func(t *testing.T) {
		opts := normalizeOptions(&BatchOptions{
			Concurrency: 0,
		})

		assert.Equal(t, 1, opts.Concurrency)
	})

	t.Run("negative concurrency becomes 1", func(t *testing.T) {
		opts := normalizeOptions(&BatchOptions{
			Concurrency: -5,
		})

		assert.Equal(t, 1, opts.Concurrency)
	})

	t.Run("valid concurrency unchanged", func(t *testing.T) {
		opts := normalizeOptions(&BatchOptions{
			Concurrency: 5,
		})

		assert.Equal(t, 5, opts.Concurrency)
	})

	t.Run("preserves other options", func(t *testing.T) {
		opts := normalizeOptions(&BatchOptions{
			Concurrency:          5,
			BatchSize:            200,
			RetryCount:           5,
			IdempotencyKeyPrefix: "custom",
			StopOnError:          true,
		})

		assert.Equal(t, 5, opts.Concurrency)
		assert.Equal(t, 200, opts.BatchSize)
		assert.Equal(t, 5, opts.RetryCount)
		assert.Equal(t, "custom", opts.IdempotencyKeyPrefix)
		assert.True(t, opts.StopOnError)
	})
}

// TestIsRetryableError tests the isRetryableError function
func TestIsRetryableError(t *testing.T) {
	t.Run("nil error is not retryable", func(t *testing.T) {
		assert.False(t, isRetryableError(nil))
	})

	t.Run("rate limit error is retryable", func(t *testing.T) {
		err := pkgerrors.NewRateLimitError("test", "rate limited", nil)
		assert.True(t, isRetryableError(err))
	})

	t.Run("network error is retryable", func(t *testing.T) {
		err := pkgerrors.NewNetworkError("test", nil)
		assert.True(t, isRetryableError(err))
	})

	t.Run("timeout error is retryable", func(t *testing.T) {
		err := pkgerrors.NewTimeoutError("test", "timeout", nil)
		assert.True(t, isRetryableError(err))
	})

	t.Run("validation error is not retryable", func(t *testing.T) {
		err := pkgerrors.NewValidationError("test", "invalid", nil)
		assert.False(t, isRetryableError(err))
	})

	t.Run("not found error is not retryable", func(t *testing.T) {
		err := pkgerrors.NewNotFoundError("test", "resource", "123", nil)
		assert.False(t, isRetryableError(err))
	})

	t.Run("authentication error is not retryable", func(t *testing.T) {
		err := pkgerrors.NewAuthenticationError("test", "unauthorized", nil)
		assert.False(t, isRetryableError(err))
	})

	t.Run("server error (500+) is retryable", func(t *testing.T) {
		err := pkgerrors.ErrorFromHTTPResponse(500, "req-123", "internal error", "", "", "")
		assert.True(t, isRetryableError(err))
	})

	t.Run("server error 502 is retryable", func(t *testing.T) {
		err := pkgerrors.ErrorFromHTTPResponse(502, "req-123", "bad gateway", "", "", "")
		assert.True(t, isRetryableError(err))
	})

	t.Run("server error 503 is retryable", func(t *testing.T) {
		err := pkgerrors.ErrorFromHTTPResponse(503, "req-123", "service unavailable", "", "", "")
		assert.True(t, isRetryableError(err))
	})

	t.Run("client error (4xx) is not retryable", func(t *testing.T) {
		err := pkgerrors.ErrorFromHTTPResponse(400, "req-123", "bad request", "", "", "")
		assert.False(t, isRetryableError(err))
	})

	t.Run("generic error is retryable because it maps to 500", func(t *testing.T) {
		// Generic errors that don't match any known pattern are mapped to 500 Internal Server Error
		// by GetErrorDetails.determineHTTPStatusFromError, making them retryable
		err := errors.New("some random error")
		assert.True(t, isRetryableError(err))
	})

	t.Run("conflict error is not retryable", func(t *testing.T) {
		// This specifically tests a non-retryable error that doesn't map to 5xx
		err := pkgerrors.NewConflictError("test", "resource", "123", nil)
		assert.False(t, isRetryableError(err))
	})
}

// TestBatchProcessorCalculateBatchEnd tests the calculateBatchEnd method
func TestBatchProcessorCalculateBatchEnd(t *testing.T) {
	t.Run("batch end within bounds", func(t *testing.T) {
		// Create test inputs
		inputs := make([]*models.CreateTransactionInput, 100)
		for i := range inputs {
			inputs[i] = &models.CreateTransactionInput{}
		}

		bp := &batchProcessor{
			inputs:  inputs,
			options: &BatchOptions{BatchSize: 10},
		}

		// Test start at 0
		end := bp.calculateBatchEnd(0)
		assert.Equal(t, 10, end)

		// Test at boundary
		end = bp.calculateBatchEnd(95)
		assert.Equal(t, 100, end)

		// Test past boundary
		end = bp.calculateBatchEnd(98)
		assert.Equal(t, 100, end)
	})
}

// TestBatchProcessorCalculateBackoffFactor tests the calculateBackoffFactor method
func TestBatchProcessorCalculateBackoffFactor(t *testing.T) {
	bp := &batchProcessor{
		options: DefaultBatchOptions(),
	}

	tests := []struct {
		name     string
		attempt  int
		expected uint
	}{
		{"attempt 0 returns 0", 0, 0},
		{"attempt 1 returns 0", 1, 0},
		{"attempt 2 returns 1", 2, 1},
		{"attempt 3 returns 2", 3, 2},
		{"attempt 10 returns 9", 10, 9},
		{"attempt 31 returns 30", 31, 30},
		{"attempt 32 capped at 30", 32, 30},
		{"attempt 100 capped at 30", 100, 30},
		{"negative attempt returns 0", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bp.calculateBackoffFactor(tt.attempt)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestBatchProcessorCreateResult tests the createResult method
func TestBatchProcessorCreateResult(t *testing.T) {
	bp := &batchProcessor{
		options: DefaultBatchOptions(),
	}

	t.Run("successful result with transaction", func(t *testing.T) {
		tx := &models.Transaction{ID: "tx-123"}
		result := bp.createResult(0, tx, nil, 100*time.Millisecond)

		assert.Equal(t, 0, result.Index)
		assert.Equal(t, "tx-123", result.TransactionID)
		assert.Nil(t, result.Error)
		assert.Equal(t, 100*time.Millisecond, result.Duration)
	})

	t.Run("failed result with error", func(t *testing.T) {
		err := errors.New("transaction failed")
		result := bp.createResult(5, nil, err, 50*time.Millisecond)

		assert.Equal(t, 5, result.Index)
		assert.Empty(t, result.TransactionID)
		assert.Equal(t, err, result.Error)
		assert.Equal(t, 50*time.Millisecond, result.Duration)
	})
}

// TestBatchProcessorCallProgressCallback tests the progress callback
func TestBatchProcessorCallProgressCallback(t *testing.T) {
	t.Run("callback is called when set", func(t *testing.T) {
		var calledWith struct {
			completed int
			total     int
			result    BatchResult
		}

		inputs := make([]*models.CreateTransactionInput, 10)
		for i := range inputs {
			inputs[i] = &models.CreateTransactionInput{}
		}

		bp := &batchProcessor{
			inputs: inputs,
			options: &BatchOptions{
				OnProgress: func(completed, total int, result BatchResult) {
					calledWith.completed = completed
					calledWith.total = total
					calledWith.result = result
				},
			},
		}

		// Simulate input count
		result := BatchResult{Index: 3, TransactionID: "tx-123"}
		bp.callProgressCallback(3, result)

		assert.Equal(t, 4, calledWith.completed) // index + 1
		assert.Equal(t, 10, calledWith.total)
		assert.Equal(t, "tx-123", calledWith.result.TransactionID)
	})

	t.Run("no panic when callback is nil", func(t *testing.T) {
		inputs := make([]*models.CreateTransactionInput, 10)
		for i := range inputs {
			inputs[i] = &models.CreateTransactionInput{}
		}

		bp := &batchProcessor{
			inputs:  inputs,
			options: &BatchOptions{OnProgress: nil},
		}

		// Should not panic
		assert.NotPanics(t, func() {
			bp.callProgressCallback(0, BatchResult{})
		})
	})
}

// TestBatchProcessorWaitForRetry tests the waitForRetry method with context cancellation
func TestBatchProcessorWaitForRetry(t *testing.T) {
	t.Run("returns nil on successful wait", func(t *testing.T) {
		ctx := context.Background()
		bp := &batchProcessor{
			ctx: ctx,
			options: &BatchOptions{
				RetryDelay: 1 * time.Millisecond,
			},
		}

		err := bp.waitForRetry(1)
		assert.NoError(t, err)
	})

	t.Run("returns context error on cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		bp := &batchProcessor{
			ctx: ctx,
			options: &BatchOptions{
				RetryDelay: 1 * time.Hour, // Long delay to ensure cancellation wins
			},
		}

		err := bp.waitForRetry(1)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("returns context error on timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Give it a moment for the timeout to trigger
		time.Sleep(1 * time.Millisecond)

		bp := &batchProcessor{
			ctx: ctx,
			options: &BatchOptions{
				RetryDelay: 1 * time.Hour,
			},
		}

		err := bp.waitForRetry(1)
		assert.Error(t, err)
	})
}

// TestBatchProcessorCheckForEarlyError tests early error checking
func TestBatchProcessorCheckForEarlyError(t *testing.T) {
	t.Run("returns nil when no error in channel", func(t *testing.T) {
		bp := &batchProcessor{}
		errChan := make(chan error, 1)

		err := bp.checkForEarlyError(errChan)
		assert.NoError(t, err)
	})

	t.Run("returns error when error in channel", func(t *testing.T) {
		bp := &batchProcessor{}
		errChan := make(chan error, 1)
		expectedErr := errors.New("early error")
		errChan <- expectedErr

		err := bp.checkForEarlyError(errChan)
		assert.Equal(t, expectedErr, err)
	})
}

// TestBatchSummaryCalculations tests the math in GetBatchSummary
func TestBatchSummaryCalculations(t *testing.T) {
	t.Run("success rate calculation", func(t *testing.T) {
		// 7 successes out of 10
		results := make([]BatchResult, 10)
		for i := 0; i < 7; i++ {
			results[i] = BatchResult{Index: i, TransactionID: "tx"}
		}
		for i := 7; i < 10; i++ {
			results[i] = BatchResult{Index: i, Error: errors.New("error")}
		}

		summary := GetBatchSummary(results)
		assert.Equal(t, float64(70), summary.SuccessRate)
	})

	t.Run("average duration calculation", func(t *testing.T) {
		results := []BatchResult{
			{Duration: 100 * time.Millisecond},
			{Duration: 200 * time.Millisecond},
			{Duration: 300 * time.Millisecond},
		}

		summary := GetBatchSummary(results)
		assert.Equal(t, 200*time.Millisecond, summary.AverageDuration)
	})

	t.Run("TPS calculation", func(t *testing.T) {
		results := []BatchResult{
			{TransactionID: "tx-1", Duration: 500 * time.Millisecond},
			{TransactionID: "tx-2", Duration: 500 * time.Millisecond},
		}

		summary := GetBatchSummary(results)
		// 2 successes in 1 second total = 2 TPS
		assert.Equal(t, 2.0, summary.TransactionsPerSecond)
	})

	t.Run("zero duration doesn't cause division by zero", func(t *testing.T) {
		results := []BatchResult{
			{TransactionID: "tx-1", Duration: 0},
		}

		// Should not panic
		assert.NotPanics(t, func() {
			summary := GetBatchSummary(results)
			// With 0 duration, TPS would be infinity or undefined
			// The implementation should handle this gracefully
			_ = summary.TransactionsPerSecond
		})
	})
}

// TestBatchOptionsDefaults tests that default values are properly set
func TestBatchOptionsDefaults(t *testing.T) {
	opts := DefaultBatchOptions()

	// Verify all defaults are reasonable
	assert.Greater(t, opts.Concurrency, 0, "Concurrency should be positive")
	assert.Greater(t, opts.BatchSize, 0, "BatchSize should be positive")
	assert.GreaterOrEqual(t, opts.RetryCount, 0, "RetryCount should be non-negative")
	assert.Greater(t, opts.RetryDelay, time.Duration(0), "RetryDelay should be positive")
	assert.NotEmpty(t, opts.IdempotencyKeyPrefix, "IdempotencyKeyPrefix should not be empty")
}

// TestBatchResultWithNilValues tests BatchResult with nil/zero values
func TestBatchResultWithNilValues(t *testing.T) {
	result := BatchResult{}

	assert.Equal(t, 0, result.Index)
	assert.Empty(t, result.TransactionID)
	assert.Nil(t, result.Error)
	assert.Equal(t, time.Duration(0), result.Duration)
}

// TestBatchProcessorCheckFinalErrors tests the checkFinalErrors method
func TestBatchProcessorCheckFinalErrors(t *testing.T) {
	t.Run("returns results when StopOnError is false", func(t *testing.T) {
		bp := &batchProcessor{
			options: &BatchOptions{StopOnError: false},
			results: []BatchResult{
				{Index: 0, TransactionID: "tx-1"},
				{Index: 1, TransactionID: "tx-2"},
			},
		}

		errChan := make(chan error, 1)
		errChan <- errors.New("some error")

		results, err := bp.checkFinalErrors(errChan)
		assert.NoError(t, err) // StopOnError is false, so error is ignored
		assert.Len(t, results, 2)
	})

	t.Run("returns error when StopOnError is true and error in channel", func(t *testing.T) {
		bp := &batchProcessor{
			options: &BatchOptions{StopOnError: true},
			results: []BatchResult{
				{Index: 0, TransactionID: "tx-1"},
			},
		}

		errChan := make(chan error, 1)
		expectedErr := errors.New("stop error")
		errChan <- expectedErr

		results, err := bp.checkFinalErrors(errChan)
		assert.Equal(t, expectedErr, err)
		assert.Len(t, results, 1)
	})

	t.Run("returns nil error when StopOnError is true and no error", func(t *testing.T) {
		bp := &batchProcessor{
			options: &BatchOptions{StopOnError: true},
			results: []BatchResult{
				{Index: 0, TransactionID: "tx-1"},
			},
		}

		errChan := make(chan error, 1)
		// No error in channel

		results, err := bp.checkFinalErrors(errChan)
		assert.NoError(t, err)
		assert.Len(t, results, 1)
	})
}

// TestBatchProcessorEnsureIdempotencyKey tests the ensureIdempotencyKey method
func TestBatchProcessorEnsureIdempotencyKey(t *testing.T) {
	t.Run("sets idempotency key when empty", func(t *testing.T) {
		bp := &batchProcessor{
			options: &BatchOptions{IdempotencyKeyPrefix: "test-prefix"},
		}

		input := &models.CreateTransactionInput{
			IdempotencyKey: "",
		}

		bp.ensureIdempotencyKey(input, 5)

		assert.NotEmpty(t, input.IdempotencyKey)
		assert.Contains(t, input.IdempotencyKey, "test-prefix")
		assert.Contains(t, input.IdempotencyKey, "-5")
	})

	t.Run("preserves existing idempotency key", func(t *testing.T) {
		bp := &batchProcessor{
			options: &BatchOptions{IdempotencyKeyPrefix: "test-prefix"},
		}

		input := &models.CreateTransactionInput{
			IdempotencyKey: "existing-key",
		}

		bp.ensureIdempotencyKey(input, 5)

		assert.Equal(t, "existing-key", input.IdempotencyKey)
	})
}

// TestBatchProcessorEmptyInputs tests batch processing with empty inputs
func TestBatchProcessorEmptyInputs(t *testing.T) {
	bp := &batchProcessor{
		inputs:  []*models.CreateTransactionInput{},
		options: DefaultBatchOptions(),
	}

	// Test calculateBatchEnd with empty inputs
	end := bp.calculateBatchEnd(0)
	assert.Equal(t, 0, end)
}

// TestBatchSummaryWithZeroTotalDuration tests TPS calculation with zero duration
func TestBatchSummaryWithZeroTotalDuration(t *testing.T) {
	results := []BatchResult{
		{TransactionID: "tx-1", Duration: 0},
		{TransactionID: "tx-2", Duration: 0},
	}

	summary := GetBatchSummary(results)

	assert.Equal(t, 2, summary.TotalTransactions)
	assert.Equal(t, 2, summary.SuccessCount)
	// TPS should be 0 when total duration is 0
	assert.Equal(t, float64(0), summary.TransactionsPerSecond)
}

// TestErrorCategorizationInSummary tests that different error types are properly categorized
func TestErrorCategorizationInSummary(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		expectEmpty   bool
		expectedCount int
	}{
		{
			name:          "validation error categorized",
			err:           pkgerrors.NewValidationError("test", "invalid", nil),
			expectedCount: 1,
		},
		{
			name:          "not found error categorized",
			err:           pkgerrors.NewNotFoundError("test", "resource", "123", nil),
			expectedCount: 1,
		},
		{
			name:          "authentication error categorized",
			err:           pkgerrors.NewAuthenticationError("test", "unauthorized", nil),
			expectedCount: 1,
		},
		{
			name:          "rate limit error categorized",
			err:           pkgerrors.NewRateLimitError("test", "rate limited", nil),
			expectedCount: 1,
		},
		{
			name:          "timeout error categorized",
			err:           pkgerrors.NewTimeoutError("test", "timeout", nil),
			expectedCount: 1,
		},
		{
			name:          "network error categorized",
			err:           pkgerrors.NewNetworkError("test", nil),
			expectedCount: 1,
		},
		{
			name:          "internal error categorized",
			err:           pkgerrors.NewInternalError("test", nil),
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := []BatchResult{
				{Index: 0, Error: tt.err, Duration: 50 * time.Millisecond},
			}

			summary := GetBatchSummary(results)

			assert.Equal(t, 1, summary.ErrorCount)
			totalCategorized := 0
			for _, count := range summary.ErrorCategories {
				totalCategorized += count
			}
			assert.Equal(t, tt.expectedCount, totalCategorized)
		})
	}
}

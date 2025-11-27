package concurrent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogger is a simple CBLogger implementation for testing.
type testLogger struct {
	buf *bytes.Buffer
}

func newTestLogger() *testLogger {
	return &testLogger{buf: &bytes.Buffer{}}
}

func (l *testLogger) Printf(format string, v ...any) {
	fmt.Fprintf(l.buf, format, v...)
}

func (l *testLogger) String() string {
	return l.buf.String()
}

func TestCircuitBreakerTransitions(t *testing.T) {
	cb := NewCircuitBreaker(2, 1, 100*time.Millisecond)

	// Two consecutive failures should open the circuit
	var executed int

	failFn := func() error { executed++; return errors.New("boom") }
	_ = cb.Execute(context.Background(), failFn)
	_ = cb.Execute(context.Background(), failFn)

	if executed != 2 {
		t.Fatalf("expected 2 executions before open, got %d", executed)
	}

	// Third attempt should be blocked
	err := cb.Execute(context.Background(), failFn)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	// Wait for half-open transition
	time.Sleep(120 * time.Millisecond)

	// Successful probe should close the circuit (successThreshold=1)
	successFn := func() error { executed++; return nil }
	if err := cb.Execute(context.Background(), successFn); err != nil {
		t.Fatalf("expected success in half-open, got %v", err)
	}

	// Verify subsequent calls are allowed (closed state)
	if err := cb.Execute(context.Background(), successFn); err != nil {
		t.Fatalf("expected success in closed state, got %v", err)
	}
}

func TestCircuitBreakerDefaults(t *testing.T) {
	// Test default values when invalid parameters are passed
	t.Run("ZeroFailureThreshold", func(t *testing.T) {
		cb := NewCircuitBreaker(0, 2, 5*time.Second)
		assert.NotNil(t, cb)
		assert.Equal(t, 5, cb.failureThreshold) // Default value
	})

	t.Run("NegativeFailureThreshold", func(t *testing.T) {
		cb := NewCircuitBreaker(-10, 2, 5*time.Second)
		assert.NotNil(t, cb)
		assert.Equal(t, 5, cb.failureThreshold) // Default value
	})

	t.Run("ZeroSuccessThreshold", func(t *testing.T) {
		cb := NewCircuitBreaker(5, 0, 5*time.Second)
		assert.NotNil(t, cb)
		assert.Equal(t, 2, cb.successThreshold) // Default value
	})

	t.Run("NegativeSuccessThreshold", func(t *testing.T) {
		cb := NewCircuitBreaker(5, -5, 5*time.Second)
		assert.NotNil(t, cb)
		assert.Equal(t, 2, cb.successThreshold) // Default value
	})

	t.Run("ZeroOpenTimeout", func(t *testing.T) {
		cb := NewCircuitBreaker(5, 2, 0)
		assert.NotNil(t, cb)
		assert.Equal(t, 5*time.Second, cb.openTimeout) // Default value
	})

	t.Run("NegativeOpenTimeout", func(t *testing.T) {
		cb := NewCircuitBreaker(5, 2, -1*time.Second)
		assert.NotNil(t, cb)
		assert.Equal(t, 5*time.Second, cb.openTimeout) // Default value
	})

	t.Run("AllDefaultsApplied", func(t *testing.T) {
		cb := NewCircuitBreaker(0, 0, 0)
		assert.NotNil(t, cb)
		assert.Equal(t, 5, cb.failureThreshold)
		assert.Equal(t, 2, cb.successThreshold)
		assert.Equal(t, 5*time.Second, cb.openTimeout)
	})
}

func TestCircuitBreakerNamed(t *testing.T) {
	t.Run("CreatesNamedCircuitBreaker", func(t *testing.T) {
		cb := NewCircuitBreakerNamed("test-service", 3, 2, 100*time.Millisecond)
		assert.NotNil(t, cb)
		assert.Equal(t, "test-service", cb.name)
		assert.Equal(t, 3, cb.failureThreshold)
		assert.Equal(t, 2, cb.successThreshold)
		assert.Equal(t, 100*time.Millisecond, cb.openTimeout)
	})

	t.Run("NamedWithDefaultValues", func(t *testing.T) {
		cb := NewCircuitBreakerNamed("my-service", 0, 0, 0)
		assert.NotNil(t, cb)
		assert.Equal(t, "my-service", cb.name)
		assert.Equal(t, 5, cb.failureThreshold)
		assert.Equal(t, 2, cb.successThreshold)
		assert.Equal(t, 5*time.Second, cb.openTimeout)
	})
}

func TestCircuitBreakerWithLogger(t *testing.T) {
	t.Run("LogsStateTransitions", func(t *testing.T) {
		logger := newTestLogger()

		cb := NewCircuitBreakerNamed("logged-service", 2, 1, 100*time.Millisecond)
		cb.WithLogger(logger)

		// Trigger failures to open circuit
		failFn := func() error { return errors.New("fail") }
		_ = cb.Execute(context.Background(), failFn)
		_ = cb.Execute(context.Background(), failFn)

		// Check that circuit opened log was written
		assert.Contains(t, logger.String(), "circuit 'logged-service' opened")

		// Wait for half-open
		time.Sleep(120 * time.Millisecond)

		// Succeed to close circuit
		successFn := func() error { return nil }
		_ = cb.Execute(context.Background(), successFn)

		// Check that circuit closed log was written
		assert.Contains(t, logger.String(), "circuit 'logged-service' closed")
	})

	t.Run("NoLogsWithoutName", func(t *testing.T) {
		logger := newTestLogger()

		cb := NewCircuitBreaker(2, 1, 100*time.Millisecond)
		cb.WithLogger(logger)

		// Trigger failures to open circuit
		failFn := func() error { return errors.New("fail") }
		_ = cb.Execute(context.Background(), failFn)
		_ = cb.Execute(context.Background(), failFn)

		// No logs should be written without a name
		assert.Empty(t, logger.String())
	})

	t.Run("ChainingWithLogger", func(t *testing.T) {
		logger := newTestLogger()

		cb := NewCircuitBreakerNamed("chained", 2, 1, 100*time.Millisecond).WithLogger(logger)
		assert.NotNil(t, cb)
		assert.Equal(t, logger, cb.logger)
	})
}

func TestCircuitBreakerClosedState(t *testing.T) {
	t.Run("SuccessResetsFailureCount", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 1, 100*time.Millisecond)

		// Add some failures
		failFn := func() error { return errors.New("fail") }
		_ = cb.Execute(context.Background(), failFn)
		_ = cb.Execute(context.Background(), failFn)

		// Success should reset failure count
		successFn := func() error { return nil }
		err := cb.Execute(context.Background(), successFn)
		require.NoError(t, err)

		// Now we should be able to have 2 more failures before opening
		_ = cb.Execute(context.Background(), failFn)
		_ = cb.Execute(context.Background(), failFn)

		// Circuit should still be closed
		err = cb.Execute(context.Background(), successFn)
		require.NoError(t, err)
	})

	t.Run("ExactThresholdOpensCircuit", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 1, 100*time.Millisecond)

		failFn := func() error { return errors.New("fail") }

		// First two failures
		_ = cb.Execute(context.Background(), failFn)
		_ = cb.Execute(context.Background(), failFn)

		// Third failure should open the circuit
		_ = cb.Execute(context.Background(), failFn)

		// Circuit should now be open
		err := cb.Execute(context.Background(), failFn)
		require.ErrorIs(t, err, ErrCircuitOpen)
	})
}

func TestCircuitBreakerHalfOpenState(t *testing.T) {
	t.Run("FailureInHalfOpenReopensCircuit", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 2, 50*time.Millisecond)

		// Open the circuit
		failFn := func() error { return errors.New("fail") }
		_ = cb.Execute(context.Background(), failFn)
		_ = cb.Execute(context.Background(), failFn)

		// Wait for half-open
		time.Sleep(60 * time.Millisecond)

		// Fail in half-open should reopen circuit
		_ = cb.Execute(context.Background(), failFn)

		// Circuit should be open again
		err := cb.Execute(context.Background(), failFn)
		require.ErrorIs(t, err, ErrCircuitOpen)
	})

	t.Run("MultipleSuccessesNeeded", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 3, 50*time.Millisecond)

		// Open the circuit
		failFn := func() error { return errors.New("fail") }
		_ = cb.Execute(context.Background(), failFn)
		_ = cb.Execute(context.Background(), failFn)

		// Wait for half-open
		time.Sleep(60 * time.Millisecond)

		successFn := func() error { return nil }

		// First success - still half-open
		err := cb.Execute(context.Background(), successFn)
		require.NoError(t, err)

		// Second success - still half-open
		err = cb.Execute(context.Background(), successFn)
		require.NoError(t, err)

		// Third success - should close circuit
		err = cb.Execute(context.Background(), successFn)
		require.NoError(t, err)

		// Circuit should be fully closed now
		err = cb.Execute(context.Background(), successFn)
		require.NoError(t, err)
	})
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	t.Run("ConcurrentExecutions", func(t *testing.T) {
		cb := NewCircuitBreaker(100, 10, 100*time.Millisecond)

		var wg sync.WaitGroup

		const (
			numGoroutines   = 50
			opsPerGoroutine = 10
		)

		var (
			successCount int32
			errorCount   int32
		)

		successFn := func() error { return nil }

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				for j := 0; j < opsPerGoroutine; j++ {
					err := cb.Execute(context.Background(), successFn)
					if err == nil {
						atomic.AddInt32(&successCount, 1)
					} else {
						atomic.AddInt32(&errorCount, 1)
					}
				}
			}()
		}

		wg.Wait()

		// All operations should succeed
		assert.Equal(t, int32(numGoroutines*opsPerGoroutine), successCount)
		assert.Equal(t, int32(0), errorCount)
	})

	t.Run("ConcurrentFailuresOpenCircuit", func(t *testing.T) {
		cb := NewCircuitBreaker(10, 5, 200*time.Millisecond)

		var wg sync.WaitGroup

		const numGoroutines = 20

		var openErrors int32

		failFn := func() error { return errors.New("fail") }

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				err := cb.Execute(context.Background(), failFn)
				if errors.Is(err, ErrCircuitOpen) {
					atomic.AddInt32(&openErrors, 1)
				}
			}()
		}

		wg.Wait()

		// Some executions should have gotten ErrCircuitOpen
		assert.Positive(t, atomic.LoadInt32(&openErrors))
	})
}

func TestCircuitBreakerOpenTimeout(t *testing.T) {
	t.Run("CircuitStaysOpenBeforeTimeout", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 1, 200*time.Millisecond)

		// Open the circuit
		failFn := func() error { return errors.New("fail") }
		_ = cb.Execute(context.Background(), failFn)
		_ = cb.Execute(context.Background(), failFn)

		// Try immediately - should be open
		err := cb.Execute(context.Background(), failFn)
		require.ErrorIs(t, err, ErrCircuitOpen)

		// Try after 50ms - should still be open
		time.Sleep(50 * time.Millisecond)

		err = cb.Execute(context.Background(), failFn)
		require.ErrorIs(t, err, ErrCircuitOpen)

		// Try after total 150ms - should still be open
		time.Sleep(100 * time.Millisecond)

		err = cb.Execute(context.Background(), failFn)
		require.ErrorIs(t, err, ErrCircuitOpen)
	})

	t.Run("CircuitTransitionsToHalfOpenAfterTimeout", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 1, 100*time.Millisecond)

		// Open the circuit
		failFn := func() error { return errors.New("fail") }
		_ = cb.Execute(context.Background(), failFn)
		_ = cb.Execute(context.Background(), failFn)

		// Wait for timeout
		time.Sleep(120 * time.Millisecond)

		// Should be half-open now, allowing one request
		successFn := func() error { return nil }
		err := cb.Execute(context.Background(), successFn)
		require.NoError(t, err)
	})
}

func TestCircuitBreakerFunctionErrors(t *testing.T) {
	t.Run("ExecuteFnReturnsError", func(t *testing.T) {
		cb := NewCircuitBreaker(5, 2, 100*time.Millisecond)

		expectedErr := errors.New("specific error")
		fn := func() error { return expectedErr }

		err := cb.Execute(context.Background(), fn)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("ExecuteFnReturnsNil", func(t *testing.T) {
		cb := NewCircuitBreaker(5, 2, 100*time.Millisecond)

		fn := func() error { return nil }

		err := cb.Execute(context.Background(), fn)
		require.NoError(t, err)
	})

	t.Run("ExecuteFnPanics", func(t *testing.T) {
		cb := NewCircuitBreaker(5, 2, 100*time.Millisecond)

		fn := func() error {
			panic("test panic")
		}

		assert.Panics(t, func() {
			_ = cb.Execute(context.Background(), fn)
		})
	})
}

func TestCircuitBreakerStateConsistency(t *testing.T) {
	t.Run("FullLifecycle", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 2, 100*time.Millisecond)

		failFn := func() error { return errors.New("fail") }
		successFn := func() error { return nil }

		// Phase 1: Closed state - success
		err := cb.Execute(context.Background(), successFn)
		require.NoError(t, err)

		// Phase 2: Closed state - failures
		_ = cb.Execute(context.Background(), failFn)
		_ = cb.Execute(context.Background(), failFn)

		// Phase 3: Open state
		err = cb.Execute(context.Background(), failFn)
		require.ErrorIs(t, err, ErrCircuitOpen)

		// Phase 4: Wait for half-open
		time.Sleep(120 * time.Millisecond)

		// Phase 5: Half-open - first success
		err = cb.Execute(context.Background(), successFn)
		require.NoError(t, err)

		// Phase 6: Half-open - second success, closes circuit
		err = cb.Execute(context.Background(), successFn)
		require.NoError(t, err)

		// Phase 7: Closed state again
		err = cb.Execute(context.Background(), successFn)
		require.NoError(t, err)
	})

	t.Run("RepeatedOpenClose", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 1, 50*time.Millisecond)

		failFn := func() error { return errors.New("fail") }
		successFn := func() error { return nil }

		for cycle := 0; cycle < 3; cycle++ {
			// Open the circuit
			_ = cb.Execute(context.Background(), failFn)

			// Verify it's open
			err := cb.Execute(context.Background(), failFn)
			require.ErrorIs(t, err, ErrCircuitOpen, "Cycle %d: expected open", cycle)

			// Wait for half-open
			time.Sleep(60 * time.Millisecond)

			// Close the circuit
			err = cb.Execute(context.Background(), successFn)
			require.NoError(t, err, "Cycle %d: expected success in half-open", cycle)

			// Verify it's closed
			err = cb.Execute(context.Background(), successFn)
			require.NoError(t, err, "Cycle %d: expected success in closed", cycle)
		}
	})
}

func TestErrCircuitOpen(t *testing.T) {
	t.Run("ErrorString", func(t *testing.T) {
		assert.Equal(t, "circuit breaker open", ErrCircuitOpen.Error())
	})

	t.Run("ErrorComparison", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 1, 100*time.Millisecond)

		failFn := func() error { return errors.New("fail") }
		_ = cb.Execute(context.Background(), failFn)

		err := cb.Execute(context.Background(), failFn)
		require.ErrorIs(t, err, ErrCircuitOpen)
	})
}

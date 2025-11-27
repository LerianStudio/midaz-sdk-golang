package generator

import (
	"context"
	"errors"
	"testing"

	conc "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
	"github.com/stretchr/testify/assert"
)

func TestExecuteWithCircuitBreaker_NoBreaker(t *testing.T) {
	ctx := context.Background()
	executed := false

	err := executeWithCircuitBreaker(ctx, func() error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestExecuteWithCircuitBreaker_NoBreaker_WithError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("function error")

	err := executeWithCircuitBreaker(ctx, func() error {
		return expectedErr
	})

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestExecuteWithCircuitBreaker_WithBreaker_Success(t *testing.T) {
	cb := conc.NewCircuitBreaker(5, 2, 1000)
	ctx := WithCircuitBreaker(context.Background(), cb)
	executed := false

	err := executeWithCircuitBreaker(ctx, func() error {
		executed = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestExecuteWithCircuitBreaker_WithBreaker_Error(t *testing.T) {
	cb := conc.NewCircuitBreaker(5, 2, 1000)
	ctx := WithCircuitBreaker(context.Background(), cb)
	expectedErr := errors.New("cb function error")

	err := executeWithCircuitBreaker(ctx, func() error {
		return expectedErr
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cb function error")
}

func TestExecuteWithCircuitBreaker_NilFunction(t *testing.T) {
	ctx := context.Background()

	// The function panics on nil input due to dereferencing
	assert.Panics(t, func() {
		_ = executeWithCircuitBreaker(ctx, nil)
	})
}

func TestExecuteWithCircuitBreaker_MultipleExecutions(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	for i := 0; i < 5; i++ {
		err := executeWithCircuitBreaker(ctx, func() error {
			callCount++
			return nil
		})
		assert.NoError(t, err)
	}

	assert.Equal(t, 5, callCount)
}

func TestExecuteWithCircuitBreaker_WithBreaker_MultipleExecutions(t *testing.T) {
	cb := conc.NewCircuitBreaker(5, 2, 1000)
	ctx := WithCircuitBreaker(context.Background(), cb)
	callCount := 0

	for i := 0; i < 5; i++ {
		err := executeWithCircuitBreaker(ctx, func() error {
			callCount++
			return nil
		})
		assert.NoError(t, err)
	}

	assert.Equal(t, 5, callCount)
}

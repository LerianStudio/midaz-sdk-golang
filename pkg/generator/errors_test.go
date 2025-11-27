package generator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorsJoin(t *testing.T) {
	tests := []struct {
		name     string
		errs     []error
		expected func(t *testing.T, result error)
	}{
		{
			name: "No errors returns nil",
			errs: []error{},
			expected: func(t *testing.T, result error) {
				t.Helper()
				require.NoError(t, result)
			},
		},
		{
			name: "Single error returns that error",
			errs: []error{errors.New("single error")},
			expected: func(t *testing.T, result error) {
				t.Helper()
				require.Error(t, result)
				assert.Equal(t, "single error", result.Error())
			},
		},
		{
			name: "Multiple errors are joined",
			errs: []error{
				errors.New("error 1"),
				errors.New("error 2"),
				errors.New("error 3"),
			},
			expected: func(t *testing.T, result error) {
				t.Helper()
				require.Error(t, result)
				assert.Contains(t, result.Error(), "error 1")
				assert.Contains(t, result.Error(), "error 2")
				assert.Contains(t, result.Error(), "error 3")
			},
		},
		{
			name: "Two errors are joined",
			errs: []error{
				errors.New("first"),
				errors.New("second"),
			},
			expected: func(t *testing.T, result error) {
				t.Helper()
				require.Error(t, result)
				assert.Contains(t, result.Error(), "first")
				assert.Contains(t, result.Error(), "second")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errorsJoin(tt.errs...)
			tt.expected(t, result)
		})
	}
}

func TestErrorsJoin_ErrorsIs(t *testing.T) {
	t.Run("Single error preserves identity", func(t *testing.T) {
		originalErr := errors.New("original")
		result := errorsJoin(originalErr)
		assert.ErrorIs(t, result, originalErr)
	})

	t.Run("Joined errors preserve identity", func(t *testing.T) {
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")
		result := errorsJoin(err1, err2)

		assert.True(t, errors.Is(result, err1) || containsError(result, err1))
		assert.True(t, errors.Is(result, err2) || containsError(result, err2))
	})
}

func containsError(err, target error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, target)
}

func TestErrorsJoin_WithNilInSlice(t *testing.T) {
	err1 := errors.New("error 1")

	var nilErr error = nil

	result := errorsJoin(err1, nilErr)
	require.Error(t, result)
}

func TestErrorsJoin_AllNils(t *testing.T) {
	var (
		err1 error = nil
		err2 error = nil
	)

	// errors.Join returns nil if all errors are nil
	result := errorsJoin(err1, err2)
	require.NoError(t, result)
}

package errors_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// --------------------------------
// Legacy MidazError Tests
// --------------------------------

func TestMidazError(t *testing.T) {
	t.Run("Error method with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("underlying error")
		midazErr := &sdkerrors.MidazError{
			Code:    sdkerrors.CodeValidation,
			Message: "validation failed",
			Err:     underlyingErr,
		}

		expected := "validation_error: validation failed: underlying error"
		assert.Equal(t, expected, midazErr.Error())
	})

	t.Run("Error method without underlying error", func(t *testing.T) {
		midazErr := &sdkerrors.MidazError{
			Code:    sdkerrors.CodeNotFound,
			Message: "resource not found",
		}

		expected := "not_found: resource not found"
		assert.Equal(t, expected, midazErr.Error())
	})

	t.Run("Unwrap method", func(t *testing.T) {
		underlyingErr := errors.New("underlying error")
		midazErr := &sdkerrors.MidazError{
			Code:    sdkerrors.CodeValidation,
			Message: "validation failed",
			Err:     underlyingErr,
		}

		assert.Equal(t, underlyingErr, midazErr.Unwrap())
	})
}

func TestNewMidazError(t *testing.T) {
	t.Run("Creates error with correct properties", func(t *testing.T) {
		underlyingErr := errors.New("test error")
		midazErr := sdkerrors.NewMidazError(sdkerrors.CodeValidation, underlyingErr)

		assert.Equal(t, sdkerrors.CodeValidation, midazErr.Code)
		assert.Equal(t, "test error", midazErr.Message)
		assert.Equal(t, underlyingErr, midazErr.Err)
	})
}

// --------------------------------
// Legacy Error Checking Functions
// --------------------------------

func TestErrorCheckingFunctions(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		checkFunc      func(error) bool
		expectedResult bool
	}{
		{
			name:           "IsValidationError with ErrValidation",
			err:            sdkerrors.ErrValidation,
			checkFunc:      sdkerrors.IsValidationError,
			expectedResult: true,
		},
		{
			name:           "IsValidationError with string containing 'validation'",
			err:            errors.New("this is a validation error"),
			checkFunc:      sdkerrors.IsValidationError,
			expectedResult: true,
		},
		{
			name:           "IsValidationError with unrelated error",
			err:            errors.New("unrelated error"),
			checkFunc:      sdkerrors.IsValidationError,
			expectedResult: false,
		},
		{
			name:           "IsInsufficientBalanceError with ErrInsufficientBalance",
			err:            sdkerrors.ErrInsufficientBalance,
			checkFunc:      sdkerrors.IsInsufficientBalanceError,
			expectedResult: true,
		},
		{
			name:           "IsAccountEligibilityError with ErrAccountEligibility",
			err:            sdkerrors.ErrAccountEligibility,
			checkFunc:      sdkerrors.IsAccountEligibilityError,
			expectedResult: true,
		},
		{
			name:           "IsAssetMismatchError with ErrAssetMismatch",
			err:            sdkerrors.ErrAssetMismatch,
			checkFunc:      sdkerrors.IsAssetMismatchError,
			expectedResult: true,
		},
		{
			name:           "IsAuthenticationError with ErrAuthentication",
			err:            sdkerrors.ErrAuthentication,
			checkFunc:      sdkerrors.IsAuthenticationError,
			expectedResult: true,
		},
		{
			name:           "IsPermissionError with ErrPermission",
			err:            sdkerrors.ErrPermission,
			checkFunc:      sdkerrors.IsPermissionError,
			expectedResult: true,
		},
		{
			name:           "IsNotFoundError with ErrNotFound",
			err:            sdkerrors.ErrNotFound,
			checkFunc:      sdkerrors.IsNotFoundError,
			expectedResult: true,
		},
		{
			name:           "IsIdempotencyError with ErrIdempotency",
			err:            sdkerrors.ErrIdempotency,
			checkFunc:      sdkerrors.IsIdempotencyError,
			expectedResult: true,
		},
		{
			name:           "IsRateLimitError with ErrRateLimit",
			err:            sdkerrors.ErrRateLimit,
			checkFunc:      sdkerrors.IsRateLimitError,
			expectedResult: true,
		},
		{
			name:           "IsTimeoutError with ErrTimeout",
			err:            sdkerrors.ErrTimeout,
			checkFunc:      sdkerrors.IsTimeoutError,
			expectedResult: true,
		},
		{
			name:           "IsInternalError with ErrInternal",
			err:            sdkerrors.ErrInternal,
			checkFunc:      sdkerrors.IsInternalError,
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.checkFunc(tc.err)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

// --------------------------------
// Standardized Error Tests
// --------------------------------

func TestNewValidationError(t *testing.T) {
	err := sdkerrors.NewValidationError("CreateTransaction", "invalid input", fmt.Errorf("field is required"))

	assert.Equal(t, sdkerrors.CategoryValidation, err.Category)
	assert.Equal(t, sdkerrors.CodeValidation, err.Code)
	assert.Equal(t, "CreateTransaction", err.Operation)
	assert.Equal(t, "invalid input: field is required", err.Message)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
}

func TestNewNotFoundError(t *testing.T) {
	err := sdkerrors.NewNotFoundError("GetAccount", "account", "acc123", nil)

	assert.Equal(t, sdkerrors.CategoryNotFound, err.Category)
	assert.Equal(t, sdkerrors.CodeNotFound, err.Code)
	assert.Equal(t, "GetAccount", err.Operation)
	assert.Equal(t, "account", err.Resource)
	assert.Equal(t, "acc123", err.ResourceID)
	assert.Equal(t, "account not found: acc123", err.Message)
	assert.Equal(t, http.StatusNotFound, err.StatusCode)
}

func TestIsValidationErrorWithNewErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "validation error",
			err:      sdkerrors.NewValidationError("Test", "invalid input", nil),
			expected: true,
		},
		{
			name:     "not found error",
			err:      sdkerrors.NewNotFoundError("Test", "account", "acc123", nil),
			expected: false,
		},
		{
			name:     "wrapped validation error",
			err:      fmt.Errorf("wrapper: %w", sdkerrors.NewValidationError("Test", "invalid input", nil)),
			expected: true,
		},
		{
			name:     "legacy error",
			err:      sdkerrors.NewMidazError(sdkerrors.CodeValidation, fmt.Errorf("invalid input")),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sdkerrors.IsValidationError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetErrorCategory(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected sdkerrors.ErrorCategory
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "validation error",
			err:      sdkerrors.NewValidationError("Test", "invalid input", nil),
			expected: sdkerrors.CategoryValidation,
		},
		{
			name:     "not found error",
			err:      sdkerrors.NewNotFoundError("Test", "account", "acc123", nil),
			expected: sdkerrors.CategoryNotFound,
		},
		{
			name:     "authentication error",
			err:      sdkerrors.NewAuthenticationError("Test", "invalid credentials", nil),
			expected: sdkerrors.CategoryAuthentication,
		},
		{
			name:     "wrapped error",
			err:      fmt.Errorf("wrapper: %w", sdkerrors.NewNotFoundError("Test", "account", "acc123", nil)),
			expected: sdkerrors.CategoryNotFound,
		},
		{
			name:     "legacy error",
			err:      sdkerrors.NewMidazError(sdkerrors.CodeValidation, fmt.Errorf("invalid input")),
			expected: sdkerrors.CategoryValidation,
		},
		{
			name:     "generic error",
			err:      fmt.Errorf("something went wrong"),
			expected: sdkerrors.CategoryInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sdkerrors.GetErrorCategory(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestErrorFromHTTPResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		category   sdkerrors.ErrorCategory
	}{
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			message:    "Invalid input",
			category:   sdkerrors.CategoryValidation,
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			message:    "Invalid credentials",
			category:   sdkerrors.CategoryAuthentication,
		},
		{
			name:       "forbidden",
			statusCode: http.StatusForbidden,
			message:    "Insufficient permissions",
			category:   sdkerrors.CategoryAuthorization,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			message:    "Resource not found",
			category:   sdkerrors.CategoryNotFound,
		},
		{
			name:       "conflict",
			statusCode: http.StatusConflict,
			message:    "Resource already exists",
			category:   sdkerrors.CategoryConflict,
		},
		{
			name:       "too many requests",
			statusCode: http.StatusTooManyRequests,
			message:    "Rate limit exceeded",
			category:   sdkerrors.CategoryLimitExceeded,
		},
		{
			name:       "gateway timeout",
			statusCode: http.StatusGatewayTimeout,
			message:    "Operation timed out",
			category:   sdkerrors.CategoryTimeout,
		},
		{
			name:       "internal server error",
			statusCode: http.StatusInternalServerError,
			message:    "Internal server error",
			category:   sdkerrors.CategoryInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sdkerrors.ErrorFromHTTPResponse(tt.statusCode, "req-123", tt.message, "", "", "")

			var mdzErr *sdkerrors.Error

			assert.True(t, errors.As(err, &mdzErr))
			assert.Equal(t, tt.category, mdzErr.Category)
			assert.Equal(t, tt.statusCode, mdzErr.StatusCode)
			assert.Equal(t, "req-123", mdzErr.RequestID)
			assert.Equal(t, tt.message, mdzErr.Message)
		})
	}
}

func TestFormatErrorForDisplay(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "validation error",
			err:      sdkerrors.NewValidationError("Test", "invalid input", nil),
			expected: "Invalid request: invalid input",
		},
		{
			name:     "not found error",
			err:      sdkerrors.NewNotFoundError("Test", "account", "acc123", nil),
			expected: "Resource not found: account not found: acc123",
		},
		{
			name:     "authentication error",
			err:      sdkerrors.NewAuthenticationError("Test", "invalid credentials", nil),
			expected: "Authentication failed. Please check your credentials.",
		},
		{
			name:     "generic error",
			err:      fmt.Errorf("something went wrong"),
			expected: "something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sdkerrors.FormatErrorForDisplay(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *sdkerrors.Error
		expected string
	}{
		{
			name: "basic error",
			err: &sdkerrors.Error{
				Category: sdkerrors.CategoryValidation,
				Message:  "invalid input",
			},
			expected: "validation error: invalid input",
		},
		{
			name: "with resource",
			err: &sdkerrors.Error{
				Category: sdkerrors.CategoryNotFound,
				Message:  "not found",
				Resource: "account",
			},
			expected: "not_found error for account: not found",
		},
		{
			name: "with resource and ID",
			err: &sdkerrors.Error{
				Category:   sdkerrors.CategoryNotFound,
				Message:    "not found",
				Resource:   "account",
				ResourceID: "acc123",
			},
			expected: "not_found error for account acc123: not found",
		},
		{
			name: "with operation",
			err: &sdkerrors.Error{
				Category:  sdkerrors.CategoryValidation,
				Message:   "invalid input",
				Operation: "CreateAccount",
			},
			expected: "validation error during CreateAccount: invalid input",
		},
		{
			name: "complete error",
			err: &sdkerrors.Error{
				Category:   sdkerrors.CategoryNotFound,
				Message:    "not found",
				Resource:   "account",
				ResourceID: "acc123",
				Operation:  "GetAccount",
			},
			expected: "not_found error for account acc123 during GetAccount: not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	underlyingErr := fmt.Errorf("underlying error")
	err := sdkerrors.NewValidationError("Test", "invalid input", underlyingErr)

	unwrapped := err.Unwrap()
	assert.Equal(t, underlyingErr, unwrapped)
}

func TestGetStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: http.StatusOK,
		},
		{
			name:     "validation error",
			err:      sdkerrors.NewValidationError("Test", "invalid input", nil),
			expected: http.StatusBadRequest,
		},
		{
			name:     "not found error",
			err:      sdkerrors.NewNotFoundError("Test", "account", "acc123", nil),
			expected: http.StatusNotFound,
		},
		{
			name:     "legacy error",
			err:      sdkerrors.NewMidazError(sdkerrors.CodeValidation, fmt.Errorf("invalid input")),
			expected: http.StatusBadRequest,
		},
		{
			name:     "generic error",
			err:      fmt.Errorf("something went wrong"),
			expected: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sdkerrors.GetStatusCode(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --------------------------------
// Error Formatting Tests
// --------------------------------

func TestFormatTransactionError(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		operationType  string
		expectedResult string
	}{
		{
			name:           "nil error",
			err:            nil,
			operationType:  "Transfer",
			expectedResult: "",
		},
		{
			name:           "validation error",
			err:            sdkerrors.ErrValidation,
			operationType:  "Transfer",
			expectedResult: "Transfer failed: Invalid parameters - validation error",
		},
		{
			name:           "insufficient balance error",
			err:            sdkerrors.ErrInsufficientBalance,
			operationType:  "Withdrawal",
			expectedResult: "Withdrawal failed: Insufficient account balance - insufficient balance",
		},
		{
			name:           "account eligibility error",
			err:            sdkerrors.ErrAccountEligibility,
			operationType:  "Deposit",
			expectedResult: "Deposit failed: Account not eligible - account eligibility error",
		},
		{
			name:           "asset mismatch error",
			err:            sdkerrors.ErrAssetMismatch,
			operationType:  "Exchange",
			expectedResult: "Exchange failed: Asset type mismatch - asset mismatch",
		},
		{
			name:           "authentication error",
			err:            sdkerrors.ErrAuthentication,
			operationType:  "Transfer",
			expectedResult: "Transfer failed: Authentication error - authentication error",
		},
		{
			name:           "permission error",
			err:            sdkerrors.ErrPermission,
			operationType:  "Transfer",
			expectedResult: "Transfer failed: Permission denied - permission error",
		},
		{
			name:           "not found error",
			err:            sdkerrors.ErrNotFound,
			operationType:  "Transfer",
			expectedResult: "Transfer failed: Resource not found - not found",
		},
		{
			name:           "idempotency error",
			err:            sdkerrors.ErrIdempotency,
			operationType:  "Transfer",
			expectedResult: "Transfer failed: Idempotency issue - idempotency error",
		},
		{
			name:           "rate limit error",
			err:            sdkerrors.ErrRateLimit,
			operationType:  "Transfer",
			expectedResult: "Transfer failed: Rate limit exceeded - rate limit exceeded",
		},
		{
			name:           "timeout error",
			err:            sdkerrors.ErrTimeout,
			operationType:  "Transfer",
			expectedResult: "Transfer failed: Operation timed out - timeout",
		},
		{
			name:           "internal error",
			err:            sdkerrors.ErrInternal,
			operationType:  "Transfer",
			expectedResult: "Transfer failed: internal error",
		},
		{
			name:           "unknown error",
			err:            errors.New("unknown error"),
			operationType:  "Transfer",
			expectedResult: "Transfer failed: unknown error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sdkerrors.FormatTransactionError(tc.err, tc.operationType)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestCategorizeTransactionError(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		expectedResult string
	}{
		{
			name:           "nil error",
			err:            nil,
			expectedResult: "none",
		},
		{
			name:           "validation error",
			err:            sdkerrors.ErrValidation,
			expectedResult: "validation",
		},
		{
			name:           "insufficient balance error",
			err:            sdkerrors.ErrInsufficientBalance,
			expectedResult: "insufficient_balance",
		},
		{
			name:           "account eligibility error",
			err:            sdkerrors.ErrAccountEligibility,
			expectedResult: "account_eligibility",
		},
		{
			name:           "asset mismatch error",
			err:            sdkerrors.ErrAssetMismatch,
			expectedResult: "asset_mismatch",
		},
		{
			name:           "authentication error",
			err:            sdkerrors.ErrAuthentication,
			expectedResult: "authentication",
		},
		{
			name:           "permission error",
			err:            sdkerrors.ErrPermission,
			expectedResult: "permission",
		},
		{
			name:           "not found error",
			err:            sdkerrors.ErrNotFound,
			expectedResult: "not_found",
		},
		{
			name:           "idempotency error",
			err:            sdkerrors.ErrIdempotency,
			expectedResult: "idempotency",
		},
		{
			name:           "rate limit error",
			err:            sdkerrors.ErrRateLimit,
			expectedResult: "rate_limit",
		},
		{
			name:           "timeout error",
			err:            sdkerrors.ErrTimeout,
			expectedResult: "timeout",
		},
		{
			name:           "internal error",
			err:            sdkerrors.ErrInternal,
			expectedResult: "internal",
		},
		{
			name:           "unknown error",
			err:            errors.New("unknown error"),
			expectedResult: "unknown",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sdkerrors.CategorizeTransactionError(tc.err)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

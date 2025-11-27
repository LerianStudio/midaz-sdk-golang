package errors_test

import (
	"context"
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

// --------------------------------
// Additional Error Constructor Tests
// --------------------------------

func TestNewInvalidInputError(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("field is required")
		err := sdkerrors.NewInvalidInputError("CreateAccount", underlyingErr)

		assert.Equal(t, sdkerrors.CategoryValidation, err.Category)
		assert.Equal(t, sdkerrors.CodeValidation, err.Code)
		assert.Equal(t, "CreateAccount", err.Operation)
		assert.Equal(t, "invalid input: field is required", err.Message)
		assert.Equal(t, http.StatusBadRequest, err.StatusCode)
		assert.Equal(t, underlyingErr, err.Err)
	})

	t.Run("without underlying error", func(t *testing.T) {
		err := sdkerrors.NewInvalidInputError("CreateAccount", nil)

		assert.Equal(t, sdkerrors.CategoryValidation, err.Category)
		assert.Equal(t, "invalid input", err.Message)
		assert.Nil(t, err.Err)
	})
}

func TestNewMissingParameterError(t *testing.T) {
	err := sdkerrors.NewMissingParameterError("CreateTransaction", "amount")

	assert.Equal(t, sdkerrors.CategoryValidation, err.Category)
	assert.Equal(t, sdkerrors.CodeValidation, err.Code)
	assert.Equal(t, "CreateTransaction", err.Operation)
	assert.Equal(t, "missing required parameter: amount", err.Message)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	assert.NotNil(t, err.Err)
	assert.Equal(t, "missing required parameter: amount", err.Err.Error())
}

func TestNewAuthenticationError(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("token expired")
		err := sdkerrors.NewAuthenticationError("GetAccount", "authentication failed", underlyingErr)

		assert.Equal(t, sdkerrors.CategoryAuthentication, err.Category)
		assert.Equal(t, sdkerrors.CodeAuthentication, err.Code)
		assert.Equal(t, "GetAccount", err.Operation)
		assert.Equal(t, "authentication failed: token expired", err.Message)
		assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
		assert.Equal(t, underlyingErr, err.Err)
	})

	t.Run("without underlying error", func(t *testing.T) {
		err := sdkerrors.NewAuthenticationError("GetAccount", "authentication failed", nil)

		assert.Equal(t, "authentication failed", err.Message)
		assert.Nil(t, err.Err)
	})
}

func TestNewAuthorizationError(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("insufficient permissions")
		err := sdkerrors.NewAuthorizationError("DeleteAccount", "access denied", underlyingErr)

		assert.Equal(t, sdkerrors.CategoryAuthorization, err.Category)
		assert.Equal(t, sdkerrors.CodePermission, err.Code)
		assert.Equal(t, "DeleteAccount", err.Operation)
		assert.Equal(t, "access denied: insufficient permissions", err.Message)
		assert.Equal(t, http.StatusForbidden, err.StatusCode)
		assert.Equal(t, underlyingErr, err.Err)
	})

	t.Run("without underlying error", func(t *testing.T) {
		err := sdkerrors.NewAuthorizationError("DeleteAccount", "access denied", nil)

		assert.Equal(t, "access denied", err.Message)
		assert.Nil(t, err.Err)
	})
}

func TestNewConflictError(t *testing.T) {
	t.Run("with resource ID", func(t *testing.T) {
		underlyingErr := errors.New("duplicate key")
		err := sdkerrors.NewConflictError("CreateAccount", "account", "acc123", underlyingErr)

		assert.Equal(t, sdkerrors.CategoryConflict, err.Category)
		assert.Equal(t, sdkerrors.CodeAlreadyExists, err.Code)
		assert.Equal(t, "CreateAccount", err.Operation)
		assert.Equal(t, "account", err.Resource)
		assert.Equal(t, "acc123", err.ResourceID)
		assert.Equal(t, "account already exists: acc123", err.Message)
		assert.Equal(t, http.StatusConflict, err.StatusCode)
		assert.Equal(t, underlyingErr, err.Err)
	})

	t.Run("without resource ID", func(t *testing.T) {
		err := sdkerrors.NewConflictError("CreateAccount", "account", "", nil)

		assert.Equal(t, "account already exists", err.Message)
		assert.Equal(t, "", err.ResourceID)
	})
}

func TestNewRateLimitError(t *testing.T) {
	t.Run("with custom message", func(t *testing.T) {
		underlyingErr := errors.New("too many requests")
		err := sdkerrors.NewRateLimitError("CreateTransaction", "API rate limit exceeded", underlyingErr)

		assert.Equal(t, sdkerrors.CategoryLimitExceeded, err.Category)
		assert.Equal(t, sdkerrors.CodeRateLimit, err.Code)
		assert.Equal(t, "CreateTransaction", err.Operation)
		assert.Equal(t, "API rate limit exceeded", err.Message)
		assert.Equal(t, http.StatusTooManyRequests, err.StatusCode)
		assert.Equal(t, underlyingErr, err.Err)
	})

	t.Run("with empty message", func(t *testing.T) {
		err := sdkerrors.NewRateLimitError("CreateTransaction", "", nil)

		assert.Equal(t, "rate limit exceeded", err.Message)
	})
}

func TestNewTimeoutError(t *testing.T) {
	t.Run("with custom message", func(t *testing.T) {
		underlyingErr := errors.New("context deadline exceeded")
		err := sdkerrors.NewTimeoutError("GetAccount", "request timed out", underlyingErr)

		assert.Equal(t, sdkerrors.CategoryTimeout, err.Category)
		assert.Equal(t, sdkerrors.CodeTimeout, err.Code)
		assert.Equal(t, "GetAccount", err.Operation)
		assert.Equal(t, "request timed out", err.Message)
		assert.Equal(t, http.StatusGatewayTimeout, err.StatusCode)
		assert.Equal(t, underlyingErr, err.Err)
	})

	t.Run("with empty message", func(t *testing.T) {
		err := sdkerrors.NewTimeoutError("GetAccount", "", nil)

		assert.Equal(t, "operation timed out", err.Message)
	})
}

func TestNewCancellationError(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("context canceled")
		err := sdkerrors.NewCancellationError("CreateTransaction", underlyingErr)

		assert.Equal(t, sdkerrors.CategoryCancellation, err.Category)
		assert.Equal(t, sdkerrors.CodeCancellation, err.Code)
		assert.Equal(t, "CreateTransaction", err.Operation)
		assert.Equal(t, "operation cancelled: context canceled", err.Message)
		assert.Equal(t, 499, err.StatusCode)
		assert.Equal(t, underlyingErr, err.Err)
	})

	t.Run("without underlying error", func(t *testing.T) {
		err := sdkerrors.NewCancellationError("CreateTransaction", nil)

		assert.Equal(t, "operation cancelled", err.Message)
		assert.Nil(t, err.Err)
	})
}

func TestNewNetworkError(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("connection refused")
		err := sdkerrors.NewNetworkError("GetAccount", underlyingErr)

		assert.Equal(t, sdkerrors.CategoryNetwork, err.Category)
		assert.Equal(t, sdkerrors.CodeNetwork, err.Code)
		assert.Equal(t, "GetAccount", err.Operation)
		assert.Equal(t, "network error: connection refused", err.Message)
		assert.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
		assert.Equal(t, underlyingErr, err.Err)
	})

	t.Run("without underlying error", func(t *testing.T) {
		err := sdkerrors.NewNetworkError("GetAccount", nil)

		assert.Equal(t, "network error", err.Message)
		assert.Nil(t, err.Err)
	})
}

func TestNewInternalError(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("unexpected panic")
		err := sdkerrors.NewInternalError("ProcessTransaction", underlyingErr)

		assert.Equal(t, sdkerrors.CategoryInternal, err.Category)
		assert.Equal(t, sdkerrors.CodeInternal, err.Code)
		assert.Equal(t, "ProcessTransaction", err.Operation)
		assert.Equal(t, "internal error: unexpected panic", err.Message)
		assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
		assert.Equal(t, underlyingErr, err.Err)
	})

	t.Run("without underlying error", func(t *testing.T) {
		err := sdkerrors.NewInternalError("ProcessTransaction", nil)

		assert.Equal(t, "internal error", err.Message)
		assert.Nil(t, err.Err)
	})
}

func TestNewUnprocessableError(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("invalid state")
		err := sdkerrors.NewUnprocessableError("ProcessTransaction", "transaction", underlyingErr)

		assert.Equal(t, sdkerrors.CategoryUnprocessable, err.Category)
		assert.Equal(t, sdkerrors.CodeInternal, err.Code)
		assert.Equal(t, "ProcessTransaction", err.Operation)
		assert.Equal(t, "transaction", err.Resource)
		assert.Equal(t, "unprocessable transaction: invalid state", err.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, err.StatusCode)
		assert.Equal(t, underlyingErr, err.Err)
	})

	t.Run("without underlying error", func(t *testing.T) {
		err := sdkerrors.NewUnprocessableError("ProcessTransaction", "transaction", nil)

		assert.Equal(t, "unprocessable transaction", err.Message)
		assert.Nil(t, err.Err)
	})
}

func TestNewInsufficientBalanceError(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("balance is 0")
		err := sdkerrors.NewInsufficientBalanceError("Transfer", "acc123", underlyingErr)

		assert.Equal(t, sdkerrors.CategoryUnprocessable, err.Category)
		assert.Equal(t, sdkerrors.CodeInsufficientBalance, err.Code)
		assert.Equal(t, "Transfer", err.Operation)
		assert.Equal(t, "account", err.Resource)
		assert.Equal(t, "acc123", err.ResourceID)
		assert.Equal(t, "insufficient balance: balance is 0", err.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, err.StatusCode)
		assert.Equal(t, underlyingErr, err.Err)
	})

	t.Run("without underlying error", func(t *testing.T) {
		err := sdkerrors.NewInsufficientBalanceError("Transfer", "acc123", nil)

		assert.Equal(t, "insufficient balance", err.Message)
		assert.Nil(t, err.Err)
	})
}

func TestNewAssetMismatchError(t *testing.T) {
	underlyingErr := errors.New("asset type mismatch")
	err := sdkerrors.NewAssetMismatchError("Transfer", "USD", "EUR", underlyingErr)

	assert.Equal(t, sdkerrors.CategoryValidation, err.Category)
	assert.Equal(t, sdkerrors.CodeAssetMismatch, err.Code)
	assert.Equal(t, "Transfer", err.Operation)
	assert.Equal(t, "asset mismatch: expected USD, got EUR", err.Message)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	assert.Equal(t, underlyingErr, err.Err)
}

func TestNewAccountEligibilityError(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("account is frozen")
		err := sdkerrors.NewAccountEligibilityError("Transfer", "acc123", underlyingErr)

		assert.Equal(t, sdkerrors.CategoryValidation, err.Category)
		assert.Equal(t, sdkerrors.CodeAccountEligibility, err.Code)
		assert.Equal(t, "Transfer", err.Operation)
		assert.Equal(t, "account", err.Resource)
		assert.Equal(t, "acc123", err.ResourceID)
		assert.Equal(t, "account eligibility error: account is frozen", err.Message)
		assert.Equal(t, http.StatusBadRequest, err.StatusCode)
		assert.Equal(t, underlyingErr, err.Err)
	})

	t.Run("without underlying error", func(t *testing.T) {
		err := sdkerrors.NewAccountEligibilityError("Transfer", "acc123", nil)

		assert.Equal(t, "account not eligible for this operation", err.Message)
		assert.Nil(t, err.Err)
	})
}

func TestNewNotFoundError_EdgeCases(t *testing.T) {
	t.Run("without resource ID", func(t *testing.T) {
		err := sdkerrors.NewNotFoundError("GetAccount", "account", "", nil)

		assert.Equal(t, "account not found", err.Message)
		assert.Equal(t, "", err.ResourceID)
	})

	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("database error")
		err := sdkerrors.NewNotFoundError("GetAccount", "account", "acc123", underlyingErr)

		assert.Equal(t, underlyingErr, err.Err)
	})
}

func TestNewValidationError_NilError(t *testing.T) {
	err := sdkerrors.NewValidationError("CreateTransaction", "invalid input", nil)

	assert.Equal(t, "invalid input", err.Message)
	assert.Nil(t, err.Err)
}

// --------------------------------
// Error.Is Method Tests
// --------------------------------

func TestError_Is(t *testing.T) {
	t.Run("same category and code", func(t *testing.T) {
		err1 := &sdkerrors.Error{
			Category: sdkerrors.CategoryValidation,
			Code:     sdkerrors.CodeValidation,
			Message:  "error 1",
		}
		err2 := &sdkerrors.Error{
			Category: sdkerrors.CategoryValidation,
			Code:     sdkerrors.CodeValidation,
			Message:  "error 2",
		}

		assert.True(t, err1.Is(err2))
	})

	t.Run("different category", func(t *testing.T) {
		err1 := &sdkerrors.Error{
			Category: sdkerrors.CategoryValidation,
			Code:     sdkerrors.CodeValidation,
		}
		err2 := &sdkerrors.Error{
			Category: sdkerrors.CategoryNotFound,
			Code:     sdkerrors.CodeValidation,
		}

		assert.False(t, err1.Is(err2))
	})

	t.Run("different code", func(t *testing.T) {
		err1 := &sdkerrors.Error{
			Category: sdkerrors.CategoryValidation,
			Code:     sdkerrors.CodeValidation,
		}
		err2 := &sdkerrors.Error{
			Category: sdkerrors.CategoryValidation,
			Code:     sdkerrors.CodeAssetMismatch,
		}

		assert.False(t, err1.Is(err2))
	})

	t.Run("target with empty category matches any", func(t *testing.T) {
		err1 := &sdkerrors.Error{
			Category: sdkerrors.CategoryValidation,
			Code:     sdkerrors.CodeValidation,
		}
		err2 := &sdkerrors.Error{
			Category: "",
			Code:     sdkerrors.CodeValidation,
		}

		assert.True(t, err1.Is(err2))
	})

	t.Run("target with empty code matches any", func(t *testing.T) {
		err1 := &sdkerrors.Error{
			Category: sdkerrors.CategoryValidation,
			Code:     sdkerrors.CodeValidation,
		}
		err2 := &sdkerrors.Error{
			Category: sdkerrors.CategoryValidation,
			Code:     "",
		}

		assert.True(t, err1.Is(err2))
	})

	t.Run("non-Error type returns false", func(t *testing.T) {
		err1 := &sdkerrors.Error{
			Category: sdkerrors.CategoryValidation,
			Code:     sdkerrors.CodeValidation,
		}
		err2 := errors.New("standard error")

		assert.False(t, err1.Is(err2))
	})

	t.Run("using errors.Is with wrapped error", func(t *testing.T) {
		innerErr := sdkerrors.NewValidationError("Test", "validation failed", nil)
		wrappedErr := fmt.Errorf("wrapped: %w", innerErr)

		assert.True(t, errors.Is(wrappedErr, sdkerrors.ErrValidation))
	})

	t.Run("errors.Is with sentinel errors", func(t *testing.T) {
		err := sdkerrors.NewNotFoundError("GetAccount", "account", "acc123", nil)

		assert.True(t, errors.Is(err, sdkerrors.ErrNotFound))
		assert.False(t, errors.Is(err, sdkerrors.ErrValidation))
	})
}

// --------------------------------
// MidazError.Is Method Tests
// --------------------------------

func TestMidazError_Is(t *testing.T) {
	t.Run("same code matches", func(t *testing.T) {
		err1 := &sdkerrors.MidazError{
			Code:    sdkerrors.CodeValidation,
			Message: "error 1",
		}
		err2 := &sdkerrors.MidazError{
			Code:    sdkerrors.CodeValidation,
			Message: "error 2",
		}

		assert.True(t, err1.Is(err2))
	})

	t.Run("different code does not match", func(t *testing.T) {
		err1 := &sdkerrors.MidazError{
			Code:    sdkerrors.CodeValidation,
			Message: "error 1",
		}
		err2 := &sdkerrors.MidazError{
			Code:    sdkerrors.CodeNotFound,
			Message: "error 2",
		}

		assert.False(t, err1.Is(err2))
	})

	t.Run("non-MidazError type returns false", func(t *testing.T) {
		err1 := &sdkerrors.MidazError{
			Code: sdkerrors.CodeValidation,
		}
		err2 := errors.New("standard error")

		assert.False(t, err1.Is(err2))
	})

	t.Run("using errors.Is with MidazError", func(t *testing.T) {
		err := sdkerrors.NewMidazError(sdkerrors.CodeValidation, errors.New("test"))

		target := &sdkerrors.MidazError{Code: sdkerrors.CodeValidation}
		assert.True(t, errors.Is(err, target))
	})
}

// --------------------------------
// Error Getter Methods Tests
// --------------------------------

func TestError_GetterMethods(t *testing.T) {
	err := &sdkerrors.Error{
		Category:   sdkerrors.CategoryNotFound,
		Code:       sdkerrors.CodeNotFound,
		Message:    "resource not found",
		Operation:  "GetAccount",
		Resource:   "account",
		ResourceID: "acc123",
		StatusCode: http.StatusNotFound,
		RequestID:  "req-456",
	}

	t.Run("GetCategory", func(t *testing.T) {
		assert.Equal(t, sdkerrors.CategoryNotFound, err.GetCategory())
	})

	t.Run("GetStatusCode", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, err.GetStatusCode())
	})

	t.Run("GetRequestID", func(t *testing.T) {
		assert.Equal(t, "req-456", err.GetRequestID())
	})

	t.Run("GetResource", func(t *testing.T) {
		assert.Equal(t, "account", err.GetResource())
	})

	t.Run("GetResourceID", func(t *testing.T) {
		assert.Equal(t, "acc123", err.GetResourceID())
	})

	t.Run("GetOperation", func(t *testing.T) {
		assert.Equal(t, "GetAccount", err.GetOperation())
	})
}

// --------------------------------
// Additional Check Functions Tests
// --------------------------------

func TestCheckFunctions_NilErrors(t *testing.T) {
	assert.False(t, sdkerrors.CheckValidationError(nil))
	assert.False(t, sdkerrors.CheckNotFoundError(nil))
	assert.False(t, sdkerrors.CheckAuthenticationError(nil))
	assert.False(t, sdkerrors.CheckAuthorizationError(nil))
	assert.False(t, sdkerrors.CheckConflictError(nil))
	assert.False(t, sdkerrors.CheckRateLimitError(nil))
	assert.False(t, sdkerrors.CheckTimeoutError(nil))
	assert.False(t, sdkerrors.CheckCancellationError(nil))
	assert.False(t, sdkerrors.CheckNetworkError(nil))
	assert.False(t, sdkerrors.CheckInternalError(nil))
	assert.False(t, sdkerrors.CheckInsufficientBalanceError(nil))
	assert.False(t, sdkerrors.CheckIdempotencyError(nil))
	assert.False(t, sdkerrors.CheckAccountEligibilityError(nil))
	assert.False(t, sdkerrors.CheckAssetMismatchError(nil))
}

func TestCheckCancellationError(t *testing.T) {
	t.Run("with context.Canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		assert.True(t, sdkerrors.CheckCancellationError(ctx.Err()))
	})

	t.Run("with context.DeadlineExceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		<-ctx.Done()
		assert.True(t, sdkerrors.CheckCancellationError(ctx.Err()))
	})

	t.Run("with NewCancellationError", func(t *testing.T) {
		err := sdkerrors.NewCancellationError("Test", nil)
		assert.True(t, sdkerrors.CheckCancellationError(err))
	})
}

func TestCheckNetworkError(t *testing.T) {
	t.Run("with NewNetworkError", func(t *testing.T) {
		err := sdkerrors.NewNetworkError("Test", errors.New("connection refused"))
		assert.True(t, sdkerrors.CheckNetworkError(err))
	})

	t.Run("with non-network error", func(t *testing.T) {
		err := errors.New("some error")
		assert.False(t, sdkerrors.CheckNetworkError(err))
	})
}

func TestCheckIdempotencyError(t *testing.T) {
	t.Run("with ErrIdempotency", func(t *testing.T) {
		assert.True(t, sdkerrors.CheckIdempotencyError(sdkerrors.ErrIdempotency))
	})

	t.Run("with Error type with idempotency code", func(t *testing.T) {
		err := &sdkerrors.Error{
			Category: sdkerrors.CategoryConflict,
			Code:     sdkerrors.CodeIdempotency,
		}
		assert.True(t, sdkerrors.CheckIdempotencyError(err))
	})
}

func TestCheckAccountEligibilityError(t *testing.T) {
	t.Run("with ErrAccountEligibility", func(t *testing.T) {
		assert.True(t, sdkerrors.CheckAccountEligibilityError(sdkerrors.ErrAccountEligibility))
	})

	t.Run("with NewAccountEligibilityError", func(t *testing.T) {
		err := sdkerrors.NewAccountEligibilityError("Test", "acc123", nil)
		assert.True(t, sdkerrors.CheckAccountEligibilityError(err))
	})
}

func TestCheckAssetMismatchError(t *testing.T) {
	t.Run("with ErrAssetMismatch", func(t *testing.T) {
		assert.True(t, sdkerrors.CheckAssetMismatchError(sdkerrors.ErrAssetMismatch))
	})

	t.Run("with NewAssetMismatchError", func(t *testing.T) {
		err := sdkerrors.NewAssetMismatchError("Test", "USD", "EUR", nil)
		assert.True(t, sdkerrors.CheckAssetMismatchError(err))
	})
}

func TestCheckInsufficientBalanceError(t *testing.T) {
	t.Run("with unknown error message returns false", func(t *testing.T) {
		err := errors.New("unknown error")
		assert.False(t, sdkerrors.CheckInsufficientBalanceError(err))
	})

	t.Run("with NewInsufficientBalanceError", func(t *testing.T) {
		err := sdkerrors.NewInsufficientBalanceError("Test", "acc123", nil)
		assert.True(t, sdkerrors.CheckInsufficientBalanceError(err))
	})
}

// --------------------------------
// Alias Functions Tests
// --------------------------------

func TestAliasFunctions(t *testing.T) {
	validationErr := sdkerrors.NewValidationError("Test", "invalid", nil)
	notFoundErr := sdkerrors.NewNotFoundError("Test", "account", "acc123", nil)
	authErr := sdkerrors.NewAuthenticationError("Test", "invalid credentials", nil)
	authzErr := sdkerrors.NewAuthorizationError("Test", "access denied", nil)
	conflictErr := sdkerrors.NewConflictError("Test", "account", "acc123", nil)
	rateLimitErr := sdkerrors.NewRateLimitError("Test", "", nil)
	timeoutErr := sdkerrors.NewTimeoutError("Test", "", nil)
	networkErr := sdkerrors.NewNetworkError("Test", nil)
	cancellationErr := sdkerrors.NewCancellationError("Test", nil)
	internalErr := sdkerrors.NewInternalError("Test", nil)
	insufficientBalanceErr := sdkerrors.NewInsufficientBalanceError("Test", "acc123", nil)
	accountEligibilityErr := sdkerrors.NewAccountEligibilityError("Test", "acc123", nil)
	assetMismatchErr := sdkerrors.NewAssetMismatchError("Test", "USD", "EUR", nil)

	assert.True(t, sdkerrors.IsValidationError(validationErr))
	assert.True(t, sdkerrors.IsNotFoundError(notFoundErr))
	assert.True(t, sdkerrors.IsAuthenticationError(authErr))
	assert.True(t, sdkerrors.IsAuthorizationError(authzErr))
	assert.True(t, sdkerrors.IsConflictError(conflictErr))
	assert.True(t, sdkerrors.IsPermissionError(authzErr))
	assert.True(t, sdkerrors.IsAlreadyExistsError(conflictErr))
	assert.True(t, sdkerrors.IsIdempotencyError(sdkerrors.ErrIdempotency))
	assert.True(t, sdkerrors.IsRateLimitError(rateLimitErr))
	assert.True(t, sdkerrors.IsTimeoutError(timeoutErr))
	assert.True(t, sdkerrors.IsNetworkError(networkErr))
	assert.True(t, sdkerrors.IsCancellationError(cancellationErr))
	assert.True(t, sdkerrors.IsInternalError(internalErr))
	assert.True(t, sdkerrors.IsInsufficientBalanceError(insufficientBalanceErr))
	assert.True(t, sdkerrors.IsAccountEligibilityError(accountEligibilityErr))
	assert.True(t, sdkerrors.IsAssetMismatchError(assetMismatchErr))
}

// --------------------------------
// ValueOfOriginalType Tests
// --------------------------------

func TestValueOfOriginalType(t *testing.T) {
	t.Run("with MidazError", func(t *testing.T) {
		original := &sdkerrors.MidazError{Code: sdkerrors.CodeValidation}
		result := sdkerrors.ValueOfOriginalType(original, sdkerrors.CodeNotFound)

		var midazErr *sdkerrors.MidazError
		ok := errors.As(result, &midazErr)
		assert.True(t, ok)
		assert.Equal(t, sdkerrors.CodeNotFound, midazErr.Code)
	})

	t.Run("with non-MidazError returns original", func(t *testing.T) {
		original := errors.New("standard error")
		result := sdkerrors.ValueOfOriginalType(original, sdkerrors.CodeNotFound)

		assert.Equal(t, original, result)
	})

	t.Run("with MidazError and non-ErrorCode value returns original", func(t *testing.T) {
		original := &sdkerrors.MidazError{Code: sdkerrors.CodeValidation}
		result := sdkerrors.ValueOfOriginalType(original, "not an error code")

		assert.Equal(t, original, result)
	})
}

// --------------------------------
// MidazError Edge Cases
// --------------------------------

func TestMidazError_EdgeCases(t *testing.T) {
	t.Run("Error method with empty code", func(t *testing.T) {
		err := &sdkerrors.MidazError{
			Code:    "",
			Message: "some message",
		}
		assert.Equal(t, ": some message", err.Error())
	})

	t.Run("Error method with only code", func(t *testing.T) {
		err := &sdkerrors.MidazError{
			Code: sdkerrors.CodeValidation,
		}
		assert.Equal(t, "validation_error", err.Error())
	})

	t.Run("Unwrap returns nil when no underlying error", func(t *testing.T) {
		err := &sdkerrors.MidazError{Code: sdkerrors.CodeValidation}
		assert.Nil(t, err.Unwrap())
	})
}

func TestNewMidazError_NilError(t *testing.T) {
	err := sdkerrors.NewMidazError(sdkerrors.CodeValidation, nil)

	assert.Equal(t, sdkerrors.CodeValidation, err.Code)
	assert.Equal(t, "", err.Message)
	assert.Nil(t, err.Err)
}

// --------------------------------
// ErrorFromHTTPResponse Additional Tests
// --------------------------------

func TestErrorFromHTTPResponse_AllCodes(t *testing.T) {
	tests := []struct {
		statusCode int
		category   sdkerrors.ErrorCategory
		code       sdkerrors.ErrorCode
	}{
		{http.StatusBadRequest, sdkerrors.CategoryValidation, sdkerrors.CodeValidation},
		{http.StatusUnauthorized, sdkerrors.CategoryAuthentication, sdkerrors.CodeAuthentication},
		{http.StatusForbidden, sdkerrors.CategoryAuthorization, sdkerrors.CodePermission},
		{http.StatusNotFound, sdkerrors.CategoryNotFound, sdkerrors.CodeNotFound},
		{http.StatusConflict, sdkerrors.CategoryConflict, sdkerrors.CodeAlreadyExists},
		{http.StatusTooManyRequests, sdkerrors.CategoryLimitExceeded, sdkerrors.CodeRateLimit},
		{http.StatusGatewayTimeout, sdkerrors.CategoryTimeout, sdkerrors.CodeTimeout},
		{http.StatusUnprocessableEntity, sdkerrors.CategoryUnprocessable, sdkerrors.CodeInternal},
		{http.StatusServiceUnavailable, sdkerrors.CategoryNetwork, sdkerrors.CodeInternal},
		{http.StatusInternalServerError, sdkerrors.CategoryInternal, sdkerrors.CodeInternal},
		{999, sdkerrors.CategoryInternal, sdkerrors.CodeInternal}, // Unknown status code
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			err := sdkerrors.ErrorFromHTTPResponse(tt.statusCode, "req-123", "test message", "test-code", "entity", "resource-id")

			var mdzErr *sdkerrors.Error
			assert.True(t, errors.As(err, &mdzErr))
			assert.Equal(t, tt.category, mdzErr.Category)
			assert.Equal(t, tt.code, mdzErr.Code)
			assert.Equal(t, tt.statusCode, mdzErr.StatusCode)
			assert.Equal(t, "req-123", mdzErr.RequestID)
		})
	}
}

// --------------------------------
// FormatErrorForDisplay Additional Tests
// --------------------------------

func TestFormatErrorForDisplay_AllCategories(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{"authorization", sdkerrors.NewAuthorizationError("Test", "access denied", nil), "You don't have permission"},
		{"conflict", sdkerrors.NewConflictError("Test", "account", "acc123", nil), "Resource conflict"},
		{"rate limit", sdkerrors.NewRateLimitError("Test", "", nil), "Rate limit exceeded"},
		{"timeout", sdkerrors.NewTimeoutError("Test", "", nil), "operation timed out"},
		{"network", sdkerrors.NewNetworkError("Test", nil), "Network error"},
		{"unprocessable", sdkerrors.NewUnprocessableError("Test", "transaction", nil), "Operation could not be processed"},
		{"internal", sdkerrors.NewInternalError("Test", nil), "An unexpected error occurred"},
		// Cancellation falls through to default case which returns "unexpected error"
		{"cancellation", sdkerrors.NewCancellationError("Test", nil), "unexpected error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sdkerrors.FormatErrorForDisplay(tt.err)
			assert.Contains(t, result, tt.contains)
		})
	}
}

// --------------------------------
// FormatTransactionError Additional Tests
// --------------------------------

func TestFormatTransactionError_WithNewErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		opType   string
		contains string
	}{
		{
			name:     "new validation error",
			err:      sdkerrors.NewValidationError("Test", "invalid input", nil),
			opType:   "Transfer",
			contains: "Invalid parameters",
		},
		{
			name:     "new insufficient balance error",
			err:      sdkerrors.NewInsufficientBalanceError("Test", "acc123", nil),
			opType:   "Withdrawal",
			contains: "Insufficient account balance",
		},
		{
			name:     "new asset mismatch error",
			err:      sdkerrors.NewAssetMismatchError("Test", "USD", "EUR", nil),
			opType:   "Exchange",
			contains: "Asset type mismatch",
		},
		{
			name:     "new account eligibility error",
			err:      sdkerrors.NewAccountEligibilityError("Test", "acc123", nil),
			opType:   "Deposit",
			contains: "Account not eligible",
		},
		{
			name:     "new conflict error",
			err:      sdkerrors.NewConflictError("Test", "account", "acc123", nil),
			opType:   "Create",
			contains: "Resource already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sdkerrors.FormatTransactionError(tt.err, tt.opType)
			assert.Contains(t, result, tt.contains)
			assert.Contains(t, result, tt.opType+" failed")
		})
	}
}

// --------------------------------
// errors.As Tests
// --------------------------------

func TestErrorsAs(t *testing.T) {
	t.Run("Error type", func(t *testing.T) {
		err := sdkerrors.NewValidationError("Test", "invalid input", nil)
		wrapped := fmt.Errorf("wrapped: %w", err)

		var mdzErr *sdkerrors.Error
		assert.True(t, errors.As(wrapped, &mdzErr))
		assert.Equal(t, sdkerrors.CategoryValidation, mdzErr.Category)
	})

	t.Run("MidazError type", func(t *testing.T) {
		err := sdkerrors.NewMidazError(sdkerrors.CodeValidation, errors.New("test"))
		wrapped := fmt.Errorf("wrapped: %w", err)

		var mdzErr *sdkerrors.MidazError
		assert.True(t, errors.As(wrapped, &mdzErr))
		assert.Equal(t, sdkerrors.CodeValidation, mdzErr.Code)
	})
}

// --------------------------------
// Error Chain Tests
// --------------------------------

func TestErrorChain(t *testing.T) {
	t.Run("nested Error wrapping", func(t *testing.T) {
		innerErr := sdkerrors.NewValidationError("Inner", "inner error", nil)
		outerErr := sdkerrors.NewInternalError("Outer", innerErr)

		// Outer error should be internal
		assert.Equal(t, sdkerrors.CategoryInternal, outerErr.Category)

		// Inner error should be accessible via Unwrap
		unwrapped := outerErr.Unwrap()
		assert.Equal(t, innerErr, unwrapped)

		// errors.Is should work
		assert.True(t, errors.Is(outerErr, sdkerrors.ErrInternal))

		// errors.As should extract the outer error
		var extractedErr *sdkerrors.Error
		assert.True(t, errors.As(outerErr, &extractedErr))
		assert.Equal(t, sdkerrors.CategoryInternal, extractedErr.Category)
	})

	t.Run("triple wrapping", func(t *testing.T) {
		baseErr := errors.New("base error")
		validationErr := sdkerrors.NewValidationError("Validate", "validation failed", baseErr)
		wrappedErr := fmt.Errorf("wrapped: %w", validationErr)

		assert.True(t, errors.Is(wrappedErr, sdkerrors.ErrValidation))
		assert.True(t, sdkerrors.IsValidationError(wrappedErr))

		// Can unwrap to base error
		var mdzErr *sdkerrors.Error
		assert.True(t, errors.As(wrappedErr, &mdzErr))
		assert.Equal(t, baseErr, mdzErr.Err)
	})
}

// --------------------------------
// ErrorDetails Tests (details.go)
// --------------------------------

func TestGetErrorDetails(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		details := sdkerrors.GetErrorDetails(nil)

		assert.Equal(t, "", details.Message)
		assert.Equal(t, "", details.Code)
		assert.Equal(t, 0, details.HTTPStatus)
		assert.Nil(t, details.OriginalError)
	})

	t.Run("standard error", func(t *testing.T) {
		err := errors.New("standard error")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, "standard error", details.Message)
		assert.Equal(t, err, details.OriginalError)
		assert.Equal(t, http.StatusInternalServerError, details.HTTPStatus)
	})

	t.Run("error with not found message", func(t *testing.T) {
		err := errors.New("resource not found")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusNotFound, details.HTTPStatus)
	})

	t.Run("error with unauthorized message", func(t *testing.T) {
		err := errors.New("unauthorized access")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusUnauthorized, details.HTTPStatus)
	})

	t.Run("error with permission message", func(t *testing.T) {
		err := errors.New("permission denied")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusUnauthorized, details.HTTPStatus)
	})

	t.Run("error with forbidden message", func(t *testing.T) {
		err := errors.New("forbidden action")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusForbidden, details.HTTPStatus)
	})

	t.Run("error with invalid message", func(t *testing.T) {
		err := errors.New("invalid input")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusBadRequest, details.HTTPStatus)
	})

	t.Run("error with bad request message", func(t *testing.T) {
		err := errors.New("bad request")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusBadRequest, details.HTTPStatus)
	})

	t.Run("error with conflict message", func(t *testing.T) {
		err := errors.New("resource conflict")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusConflict, details.HTTPStatus)
	})

	t.Run("error with already exists message", func(t *testing.T) {
		err := errors.New("resource already exists")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusConflict, details.HTTPStatus)
	})

	t.Run("error with timeout message", func(t *testing.T) {
		err := errors.New("operation timeout")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusGatewayTimeout, details.HTTPStatus)
	})

	t.Run("error with deadline exceeded message", func(t *testing.T) {
		err := errors.New("deadline exceeded")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusGatewayTimeout, details.HTTPStatus)
	})

	t.Run("error with rate limit message", func(t *testing.T) {
		err := errors.New("rate limit exceeded")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusTooManyRequests, details.HTTPStatus)
	})

	t.Run("error with too many requests message", func(t *testing.T) {
		err := errors.New("too many requests")
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusTooManyRequests, details.HTTPStatus)
	})
}

// errorWithCode is a test error type that implements Code() interface
type errorWithCode struct {
	code string
	msg  string
}

func (e *errorWithCode) Error() string {
	return e.msg
}

func (e *errorWithCode) Code() string {
	return e.code
}

// errorWithErrorCode is a test error type that implements ErrorCode() interface
type errorWithErrorCode struct {
	code string
	msg  string
}

func (e *errorWithErrorCode) Error() string {
	return e.msg
}

func (e *errorWithErrorCode) ErrorCode() string {
	return e.code
}

// errorWithStatusCode is a test error type that implements StatusCode() interface
type errorWithStatusCode struct {
	statusCode int
	msg        string
}

func (e *errorWithStatusCode) Error() string {
	return e.msg
}

func (e *errorWithStatusCode) StatusCode() int {
	return e.statusCode
}

// errorWithHTTPStatusCode is a test error type that implements HTTPStatusCode() interface
type errorWithHTTPStatusCode struct {
	statusCode int
	msg        string
}

func (e *errorWithHTTPStatusCode) Error() string {
	return e.msg
}

func (e *errorWithHTTPStatusCode) HTTPStatusCode() int {
	return e.statusCode
}

func TestGetErrorDetails_WithInterfaces(t *testing.T) {
	t.Run("error with Code() method", func(t *testing.T) {
		err := &errorWithCode{code: "CUSTOM_CODE", msg: "custom error"}
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, "CUSTOM_CODE", details.Code)
		assert.Equal(t, "custom error", details.Message)
	})

	t.Run("error with ErrorCode() method", func(t *testing.T) {
		err := &errorWithErrorCode{code: "ERROR_CODE", msg: "error code error"}
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, "ERROR_CODE", details.Code)
		assert.Equal(t, "error code error", details.Message)
	})

	t.Run("error with StatusCode() method", func(t *testing.T) {
		err := &errorWithStatusCode{statusCode: http.StatusTeapot, msg: "teapot error"}
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusTeapot, details.HTTPStatus)
		assert.Equal(t, "teapot error", details.Message)
	})

	t.Run("error with HTTPStatusCode() method", func(t *testing.T) {
		err := &errorWithHTTPStatusCode{statusCode: http.StatusPaymentRequired, msg: "payment required"}
		details := sdkerrors.GetErrorDetails(err)

		assert.Equal(t, http.StatusPaymentRequired, details.HTTPStatus)
		assert.Equal(t, "payment required", details.Message)
	})
}

func TestGetErrorStatusCode(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		status := sdkerrors.GetErrorStatusCode(nil)
		assert.Equal(t, 0, status)
	})

	t.Run("not found error", func(t *testing.T) {
		err := errors.New("resource not found")
		status := sdkerrors.GetErrorStatusCode(err)
		assert.Equal(t, http.StatusNotFound, status)
	})

	t.Run("error with StatusCode interface", func(t *testing.T) {
		err := &errorWithStatusCode{statusCode: http.StatusCreated, msg: "created"}
		status := sdkerrors.GetErrorStatusCode(err)
		assert.Equal(t, http.StatusCreated, status)
	})
}

func TestFormatErrorDetails(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		result := sdkerrors.FormatErrorDetails(nil)
		assert.Equal(t, "", result)
	})

	t.Run("standard error", func(t *testing.T) {
		err := errors.New("standard error message")
		result := sdkerrors.FormatErrorDetails(err)
		assert.Equal(t, "standard error message", result)
	})

	t.Run("error with code", func(t *testing.T) {
		err := &errorWithCode{code: "ERR_001", msg: "error with code"}
		result := sdkerrors.FormatErrorDetails(err)
		assert.Equal(t, "[ERR_001] error with code", result)
	})
}

func TestFormatOperationError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		result := sdkerrors.FormatOperationError(nil, "CreateAccount")
		assert.Equal(t, "", result)
	})

	t.Run("standard error", func(t *testing.T) {
		err := errors.New("operation failed")
		result := sdkerrors.FormatOperationError(err, "CreateAccount")
		assert.Equal(t, "CreateAccount failed: operation failed", result)
	})

	t.Run("error with code", func(t *testing.T) {
		err := &errorWithCode{code: "ERR_002", msg: "operation error"}
		result := sdkerrors.FormatOperationError(err, "DeleteAccount")
		assert.Equal(t, "DeleteAccount failed: [ERR_002] operation error", result)
	})
}

// --------------------------------
// GetStatusCode Comprehensive Tests
// --------------------------------

func TestGetStatusCode_CategoryMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		expectedSC int
	}{
		{
			name:       "cancellation category",
			err:        sdkerrors.NewCancellationError("Test", nil),
			expectedSC: 499,
		},
		{
			name:       "unprocessable category",
			err:        sdkerrors.NewUnprocessableError("Test", "resource", nil),
			expectedSC: http.StatusUnprocessableEntity,
		},
		{
			name:       "network category",
			err:        sdkerrors.NewNetworkError("Test", nil),
			expectedSC: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sdkerrors.GetStatusCode(tt.err)
			assert.Equal(t, tt.expectedSC, result)
		})
	}
}

// --------------------------------
// FormatErrorForDisplay Cancellation Test
// --------------------------------

func TestFormatErrorForDisplay_Cancellation(t *testing.T) {
	err := sdkerrors.NewCancellationError("Test", nil)
	result := sdkerrors.FormatErrorForDisplay(err)
	// Cancellation falls through to default in FormatErrorForDisplay
	assert.Contains(t, result, "unexpected error")
}

// --------------------------------
// FormatUnifiedTransactionError Tests
// --------------------------------

func TestFormatUnifiedTransactionError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		result := sdkerrors.FormatUnifiedTransactionError(nil, "Transfer")
		assert.Equal(t, "", result)
	})

	t.Run("unknown error", func(t *testing.T) {
		result := sdkerrors.FormatUnifiedTransactionError(errors.New("unknown error"), "Transfer")
		assert.Equal(t, "Transfer failed: unknown error", result)
	})

	t.Run("with idempotency error code", func(t *testing.T) {
		err := &sdkerrors.Error{
			Category: sdkerrors.CategoryConflict,
			Code:     sdkerrors.CodeIdempotency,
			Message:  "duplicate request",
		}
		result := sdkerrors.FormatUnifiedTransactionError(err, "Payment")
		assert.Contains(t, result, "Idempotency issue")
	})

	t.Run("with error code not in mapping", func(t *testing.T) {
		err := &sdkerrors.Error{
			Category: sdkerrors.CategoryNetwork,
			Code:     sdkerrors.CodeNetwork,
			Message:  "connection refused",
		}
		result := sdkerrors.FormatUnifiedTransactionError(err, "Sync")
		assert.Contains(t, result, "Sync failed")
		assert.Contains(t, result, "connection refused")
	})
}

// --------------------------------
// Error Code Constants Test
// --------------------------------

func TestErrorCodeConstants(t *testing.T) {
	// Verify error codes are defined correctly
	assert.Equal(t, sdkerrors.ErrorCode("validation_error"), sdkerrors.CodeValidation)
	assert.Equal(t, sdkerrors.ErrorCode("not_found"), sdkerrors.CodeNotFound)
	assert.Equal(t, sdkerrors.ErrorCode("already_exists"), sdkerrors.CodeAlreadyExists)
	assert.Equal(t, sdkerrors.ErrorCode("authentication_error"), sdkerrors.CodeAuthentication)
	assert.Equal(t, sdkerrors.ErrorCode("permission_error"), sdkerrors.CodePermission)
	assert.Equal(t, sdkerrors.ErrorCode("insufficient_balance"), sdkerrors.CodeInsufficientBalance)
	assert.Equal(t, sdkerrors.ErrorCode("account_eligibility_error"), sdkerrors.CodeAccountEligibility)
	assert.Equal(t, sdkerrors.ErrorCode("asset_mismatch"), sdkerrors.CodeAssetMismatch)
	assert.Equal(t, sdkerrors.ErrorCode("idempotency_error"), sdkerrors.CodeIdempotency)
	assert.Equal(t, sdkerrors.ErrorCode("rate_limit_exceeded"), sdkerrors.CodeRateLimit)
	assert.Equal(t, sdkerrors.ErrorCode("timeout"), sdkerrors.CodeTimeout)
	assert.Equal(t, sdkerrors.ErrorCode("cancelled"), sdkerrors.CodeCancellation)
	assert.Equal(t, sdkerrors.ErrorCode("internal_error"), sdkerrors.CodeInternal)
	assert.Equal(t, sdkerrors.ErrorCode("network_error"), sdkerrors.CodeNetwork)
}

// --------------------------------
// Error Category Constants Test
// --------------------------------

func TestErrorCategoryConstants(t *testing.T) {
	// Verify error categories are defined correctly
	assert.Equal(t, sdkerrors.ErrorCategory("validation"), sdkerrors.CategoryValidation)
	assert.Equal(t, sdkerrors.ErrorCategory("authentication"), sdkerrors.CategoryAuthentication)
	assert.Equal(t, sdkerrors.ErrorCategory("authorization"), sdkerrors.CategoryAuthorization)
	assert.Equal(t, sdkerrors.ErrorCategory("not_found"), sdkerrors.CategoryNotFound)
	assert.Equal(t, sdkerrors.ErrorCategory("conflict"), sdkerrors.CategoryConflict)
	assert.Equal(t, sdkerrors.ErrorCategory("limit_exceeded"), sdkerrors.CategoryLimitExceeded)
	assert.Equal(t, sdkerrors.ErrorCategory("timeout"), sdkerrors.CategoryTimeout)
	assert.Equal(t, sdkerrors.ErrorCategory("cancellation"), sdkerrors.CategoryCancellation)
	assert.Equal(t, sdkerrors.ErrorCategory("network"), sdkerrors.CategoryNetwork)
	assert.Equal(t, sdkerrors.ErrorCategory("internal"), sdkerrors.CategoryInternal)
	assert.Equal(t, sdkerrors.ErrorCategory("unprocessable"), sdkerrors.CategoryUnprocessable)
}

// --------------------------------
// Sentinel Errors Test
// --------------------------------

func TestSentinelErrors(t *testing.T) {
	// Verify sentinel errors have correct categories and codes
	assert.Equal(t, sdkerrors.CategoryValidation, sdkerrors.ErrValidation.Category)
	assert.Equal(t, sdkerrors.CategoryUnprocessable, sdkerrors.ErrInsufficientBalance.Category)
	assert.Equal(t, sdkerrors.CategoryValidation, sdkerrors.ErrAccountEligibility.Category)
	assert.Equal(t, sdkerrors.CategoryValidation, sdkerrors.ErrAssetMismatch.Category)
	assert.Equal(t, sdkerrors.CategoryAuthentication, sdkerrors.ErrAuthentication.Category)
	assert.Equal(t, sdkerrors.CategoryAuthorization, sdkerrors.ErrPermission.Category)
	assert.Equal(t, sdkerrors.CategoryNotFound, sdkerrors.ErrNotFound.Category)
	assert.Equal(t, sdkerrors.CategoryConflict, sdkerrors.ErrAlreadyExists.Category)
	assert.Equal(t, sdkerrors.CategoryConflict, sdkerrors.ErrIdempotency.Category)
	assert.Equal(t, sdkerrors.CategoryLimitExceeded, sdkerrors.ErrRateLimit.Category)
	assert.Equal(t, sdkerrors.CategoryTimeout, sdkerrors.ErrTimeout.Category)
	assert.Equal(t, sdkerrors.CategoryCancellation, sdkerrors.ErrCancellation.Category)
	assert.Equal(t, sdkerrors.CategoryInternal, sdkerrors.ErrInternal.Category)
}

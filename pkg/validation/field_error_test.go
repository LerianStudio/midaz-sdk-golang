package validation

import (
	"errors"
	"strings"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldError_Error(t *testing.T) {
	tests := []struct {
		name       string
		fieldError *FieldError
		wantParts  []string
	}{
		{
			name: "Basic field error with all fields",
			fieldError: &FieldError{
				Field:       "email",
				Value:       "invalid",
				Message:     "invalid email format",
				Code:        "INVALID_EMAIL",
				Constraint:  "format",
				Suggestions: []string{"Use format user@domain.com", "Check for typos"},
			},
			wantParts: []string{
				"Invalid field 'email'",
				"[REDACTED]",
				"invalid email format",
				"constraint: format",
				"Suggestions:",
				"Use format user@domain.com",
				"Check for typos",
			},
		},
		{
			name: "Field error without value",
			fieldError: &FieldError{
				Field:   "name",
				Message: "name is required",
			},
			wantParts: []string{
				"Invalid field 'name'",
				"name is required",
			},
		},
		{
			name: "Field error without message",
			fieldError: &FieldError{
				Field: "amount",
				Value: -100,
			},
			wantParts: []string{
				"Invalid field 'amount'",
				"'-100'",
			},
		},
		{
			name: "Field error with nil value",
			fieldError: &FieldError{
				Field:   "data",
				Value:   nil,
				Message: "data cannot be nil",
			},
			wantParts: []string{
				"Invalid field 'data'",
				"data cannot be nil",
			},
		},
		{
			name: "Field error with constraint but no suggestions",
			fieldError: &FieldError{
				Field:      "age",
				Value:      -5,
				Message:    "age must be positive",
				Constraint: "min",
			},
			wantParts: []string{
				"Invalid field 'age'",
				"'-5'",
				"age must be positive",
				"constraint: min",
			},
		},
		{
			name: "Nested field path",
			fieldError: &FieldError{
				Field:   "address.zipCode",
				Value:   "",
				Message: "zip code is required",
			},
			wantParts: []string{
				"Invalid field 'address.zipCode'",
				"zip code is required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.fieldError.Error()
			for _, part := range tt.wantParts {
				assert.Contains(t, errStr, part, "Error string should contain '%s'", part)
			}
		})
	}
}

func TestBuildFieldError(t *testing.T) {
	t.Run("Creates field error with basic fields", func(t *testing.T) {
		err := BuildFieldError("field_name", "field_value", "error message")
		assert.NotNil(t, err)
		assert.Equal(t, "field_name", err.Field)
		assert.Equal(t, "field_value", err.Value)
		assert.Equal(t, "error message", err.Message)
		assert.Empty(t, err.Code)
		assert.Empty(t, err.Constraint)
		assert.Nil(t, err.Suggestions)
	})

	t.Run("Creates field error with nil value", func(t *testing.T) {
		err := BuildFieldError("field", nil, "message")
		assert.NotNil(t, err)
		assert.Nil(t, err.Value)
	})

	t.Run("Creates field error with complex value", func(t *testing.T) {
		value := map[string]any{"key": "value"}
		err := BuildFieldError("data", value, "invalid data")
		assert.NotNil(t, err)
		assert.Equal(t, value, err.Value)
	})
}

func TestFieldError_WithCode(t *testing.T) {
	t.Run("Adds code to field error", func(t *testing.T) {
		err := BuildFieldError("field", "value", "message")
		result := err.WithCode("ERROR_CODE")
		assert.Equal(t, "ERROR_CODE", err.Code)
		assert.Same(t, err, result, "Should return the same pointer for chaining")
	})

	t.Run("Overwrites existing code", func(t *testing.T) {
		err := &FieldError{
			Field: "field",
			Code:  "OLD_CODE",
		}
		err.WithCode("NEW_CODE")
		assert.Equal(t, "NEW_CODE", err.Code)
	})
}

func TestFieldError_WithConstraint(t *testing.T) {
	t.Run("Adds constraint to field error", func(t *testing.T) {
		err := BuildFieldError("field", "value", "message")
		result := err.WithConstraint("required")
		assert.Equal(t, "required", err.Constraint)
		assert.Same(t, err, result, "Should return the same pointer for chaining")
	})

	t.Run("Overwrites existing constraint", func(t *testing.T) {
		err := &FieldError{
			Field:      "field",
			Constraint: "min",
		}
		err.WithConstraint("max")
		assert.Equal(t, "max", err.Constraint)
	})
}

func TestFieldError_WithSuggestions(t *testing.T) {
	t.Run("Adds suggestions to field error", func(t *testing.T) {
		err := BuildFieldError("field", "value", "message")
		result := err.WithSuggestions("suggestion1", "suggestion2")
		assert.Len(t, err.Suggestions, 2)
		assert.Contains(t, err.Suggestions, "suggestion1")
		assert.Contains(t, err.Suggestions, "suggestion2")
		assert.Same(t, err, result, "Should return the same pointer for chaining")
	})

	t.Run("Overwrites existing suggestions", func(t *testing.T) {
		err := &FieldError{
			Field:       "field",
			Suggestions: []string{"old1", "old2"},
		}
		err.WithSuggestions("new1")
		assert.Len(t, err.Suggestions, 1)
		assert.Equal(t, "new1", err.Suggestions[0])
	})

	t.Run("Handles empty suggestions", func(t *testing.T) {
		err := BuildFieldError("field", "value", "message")
		err.WithSuggestions()
		assert.Empty(t, err.Suggestions)
	})
}

func TestFieldError_Chaining(t *testing.T) {
	t.Run("Allows method chaining", func(t *testing.T) {
		err := BuildFieldError("email", "invalid", "invalid format").
			WithCode("INVALID_EMAIL").
			WithConstraint("format").
			WithSuggestions("Check format", "Try again")

		assert.Equal(t, "email", err.Field)
		assert.Equal(t, "invalid", err.Value)
		assert.Equal(t, "invalid format", err.Message)
		assert.Equal(t, "INVALID_EMAIL", err.Code)
		assert.Equal(t, "format", err.Constraint)
		assert.Len(t, err.Suggestions, 2)
	})
}

func TestFieldErrors_Add(t *testing.T) {
	t.Run("Adds new error to collection", func(t *testing.T) {
		fe := NewFieldErrors()
		result := fe.Add("field1", "value1", "message1")

		assert.NotNil(t, result)
		assert.Len(t, fe.Errors, 1)
		assert.Equal(t, "field1", fe.Errors[0].Field)
		assert.Equal(t, "value1", fe.Errors[0].Value)
		assert.Equal(t, "message1", fe.Errors[0].Message)
	})

	t.Run("Adds multiple errors", func(t *testing.T) {
		fe := NewFieldErrors()
		fe.Add("field1", "value1", "message1")
		fe.Add("field2", "value2", "message2")
		fe.Add("field3", "value3", "message3")

		assert.Len(t, fe.Errors, 3)
	})

	t.Run("Returns field error for further modification", func(t *testing.T) {
		fe := NewFieldErrors()
		err := fe.Add("field", "value", "message")
		err.WithCode("CODE")

		assert.Equal(t, "CODE", fe.Errors[0].Code)
	})
}

func TestFieldErrors_AddError(t *testing.T) {
	t.Run("Adds existing field error", func(t *testing.T) {
		fe := NewFieldErrors()
		existingErr := &FieldError{
			Field:   "field",
			Value:   "value",
			Message: "message",
			Code:    "CODE",
		}
		fe.AddError(existingErr)

		assert.Len(t, fe.Errors, 1)
		assert.Same(t, existingErr, fe.Errors[0])
	})

	t.Run("Adds multiple existing errors", func(t *testing.T) {
		fe := NewFieldErrors()
		fe.AddError(&FieldError{Field: "field1"})
		fe.AddError(&FieldError{Field: "field2"})

		assert.Len(t, fe.Errors, 2)
	})
}

func TestFieldErrors_HasErrors(t *testing.T) {
	t.Run("Returns false for empty collection", func(t *testing.T) {
		fe := NewFieldErrors()
		assert.False(t, fe.HasErrors())
	})

	t.Run("Returns true when errors exist", func(t *testing.T) {
		fe := NewFieldErrors()
		fe.Add("field", "value", "message")
		assert.True(t, fe.HasErrors())
	})
}

func TestFieldErrors_Error(t *testing.T) {
	t.Run("Returns empty string for no errors", func(t *testing.T) {
		fe := NewFieldErrors()
		assert.Empty(t, fe.Error())
	})

	t.Run("Returns flat string for single error with rich context", func(t *testing.T) {
		// Add() populates Value, so the per-field rich-renderer is used.
		fe := NewFieldErrors()
		fe.Add("email", "bad@", "invalid email")

		errStr := fe.Error()
		assert.True(t, strings.HasPrefix(errStr, "validation failed: "),
			"expected flat prefix, got: %q", errStr)
		assert.Contains(t, errStr, "email")
		assert.Contains(t, errStr, "invalid email")
	})

	t.Run("Returns flat semicolon-joined string for multiple errors", func(t *testing.T) {
		fe := NewFieldErrors()
		fe.Add("email", "bad@", "invalid email")
		fe.Add("name", "", "name required")

		errStr := fe.Error()
		assert.True(t, strings.HasPrefix(errStr, "validation failed: "),
			"expected flat prefix, got: %q", errStr)
		// Both field errors appear, separated by '; '.
		assert.Contains(t, errStr, "email")
		assert.Contains(t, errStr, "name")
		assert.Contains(t, errStr, "; ")
	})

	t.Run("Append-only entries render as '<field> <message>'", func(t *testing.T) {
		// Append() does NOT populate Value/Code/Constraint/Suggestions,
		// so the flat renderer fires. This is the path the model
		// Validate() methods use, and it's what preserves the
		// '<field> <message>' substring contract for tests like
		// strings.Contains(err.Error(), "name is required").
		var fe FieldErrors
		fe.Append("name", "is required")
		fe.Append("code", "is required")

		errStr := fe.Error()
		assert.Equal(t, "validation failed: name is required; code is required", errStr)
	})
}

func TestFieldErrors_GetFieldErrors(t *testing.T) {
	t.Run("Returns empty slice for no errors", func(t *testing.T) {
		fe := NewFieldErrors()
		errs := fe.GetFieldErrors()
		assert.Empty(t, errs)
	})

	t.Run("Returns all errors", func(t *testing.T) {
		fe := NewFieldErrors()
		fe.Add("field1", "value1", "message1")
		fe.Add("field2", "value2", "message2")

		errs := fe.GetFieldErrors()
		assert.Len(t, errs, 2)
	})
}

func TestFieldErrors_GetErrorsForField(t *testing.T) {
	t.Run("Returns empty for non-existent field", func(t *testing.T) {
		fe := NewFieldErrors()
		fe.Add("email", "bad@", "invalid")

		errs := fe.GetErrorsForField("name")
		assert.Empty(t, errs)
	})

	t.Run("Returns errors for exact field match", func(t *testing.T) {
		fe := NewFieldErrors()
		fe.Add("email", "bad@", "invalid email")
		fe.Add("name", "", "name required")

		errs := fe.GetErrorsForField("email")
		assert.Len(t, errs, 1)
		assert.Equal(t, "email", errs[0].Field)
	})

	t.Run("Returns errors for nested field paths", func(t *testing.T) {
		fe := NewFieldErrors()
		fe.Add("address.line1", "", "line1 required")
		fe.Add("address.zipCode", "", "zipCode required")
		fe.Add("name", "", "name required")

		errs := fe.GetErrorsForField("address")
		assert.Len(t, errs, 2)
	})

	t.Run("Returns specific nested field", func(t *testing.T) {
		fe := NewFieldErrors()
		fe.Add("address.line1", "", "line1 required")
		fe.Add("address.zipCode", "", "zipCode required")

		errs := fe.GetErrorsForField("address.line1")
		assert.Len(t, errs, 1)
		assert.Equal(t, "address.line1", errs[0].Field)
	})

	t.Run("Does not return partial matches", func(t *testing.T) {
		fe := NewFieldErrors()
		fe.Add("addressLine1", "", "required")
		fe.Add("address.line1", "", "required")

		errs := fe.GetErrorsForField("address")
		assert.Len(t, errs, 1)
		assert.Equal(t, "address.line1", errs[0].Field)
	})
}

func TestNewFieldErrors(t *testing.T) {
	t.Run("Creates empty FieldErrors", func(t *testing.T) {
		fe := NewFieldErrors()
		assert.NotNil(t, fe)
		assert.NotNil(t, fe.Errors)
		assert.Empty(t, fe.Errors)
	})
}

func TestWrapError(t *testing.T) {
	t.Run("Wraps standard error", func(t *testing.T) {
		stdErr := errors.New("standard error message")
		fieldErr := WrapError("field", "value", stdErr)

		assert.NotNil(t, fieldErr)
		assert.Equal(t, "field", fieldErr.Field)
		assert.Equal(t, "value", fieldErr.Value)
		assert.Equal(t, "standard error message", fieldErr.Message)
	})

	t.Run("Wraps error with nil value", func(t *testing.T) {
		stdErr := errors.New("error")
		fieldErr := WrapError("field", nil, stdErr)

		assert.Nil(t, fieldErr.Value)
	})
}

// 8B — accumulator ergonomics tests.

func TestFieldErrors_Append(t *testing.T) {
	t.Run("appends a single field error with field and message", func(t *testing.T) {
		var fe FieldErrors
		fe.Append("name", "is required")

		assert.Equal(t, 1, fe.Len())
		assert.Equal(t, "name", fe.Errs()[0].Field)
		assert.Equal(t, "is required", fe.Errs()[0].Message)
		assert.Nil(t, fe.Errs()[0].Value)
	})

	t.Run("preserves order of multiple appends", func(t *testing.T) {
		var fe FieldErrors
		fe.Append("a", "first")
		fe.Append("b", "second")
		fe.Append("c", "third")

		assert.Equal(t, 3, fe.Len())
		assert.Equal(t, "a", fe.Errs()[0].Field)
		assert.Equal(t, "b", fe.Errs()[1].Field)
		assert.Equal(t, "c", fe.Errs()[2].Field)
	})

	t.Run("nil receiver is a no-op", func(t *testing.T) {
		var fe *FieldErrors
		assert.NotPanics(t, func() { fe.Append("name", "msg") })
	})
}

func TestFieldErrors_AppendWith(t *testing.T) {
	t.Run("applies all options", func(t *testing.T) {
		var fe FieldErrors
		fe.AppendWith("assetCode", "must be uppercase",
			Value("usd"),
			Code("INVALID_ASSET_CODE"),
			Constraint("format"),
			Suggest("Use ISO 4217 codes like USD, EUR, BTC"),
		)

		require := assert.New(t)
		require.Equal(1, fe.Len())

		item := fe.Errs()[0]
		assert.Equal(t, "assetCode", item.Field)
		assert.Equal(t, "must be uppercase", item.Message)
		assert.Equal(t, "usd", item.Value)
		assert.Equal(t, "INVALID_ASSET_CODE", item.Code)
		assert.Equal(t, "format", item.Constraint)
		assert.Equal(t, []string{"Use ISO 4217 codes like USD, EUR, BTC"}, item.Suggestions)
	})

	t.Run("no options is equivalent to Append", func(t *testing.T) {
		var fe FieldErrors
		fe.AppendWith("x", "msg")

		assert.Equal(t, 1, fe.Len())
		assert.Equal(t, "x", fe.Errs()[0].Field)
		assert.Equal(t, "msg", fe.Errs()[0].Message)
	})

	t.Run("nil option is skipped", func(t *testing.T) {
		var fe FieldErrors
		fe.AppendWith("x", "msg", nil, Code("C"))

		assert.Equal(t, "C", fe.Errs()[0].Code)
	})

	t.Run("nil receiver is a no-op", func(t *testing.T) {
		var fe *FieldErrors
		assert.NotPanics(t, func() { fe.AppendWith("name", "msg", Code("X")) })
	})
}

// TestFieldErrors_OrNil_NilSafety verifies the Go interface-nil pitfall is
// handled. A naïve `return &fe` returns a non-nil error interface even when
// the slice is empty, which silently breaks `if err != nil` checks.
func TestFieldErrors_OrNil(t *testing.T) {
	t.Run("empty accumulator returns untyped nil", func(t *testing.T) {
		var fe FieldErrors
		err := fe.OrNil()

		// Critical: must be untyped-nil, not a typed-nil-pointer wrapped
		// in a non-nil error interface. The Go interface-nil pitfall is
		// the entire reason OrNil exists.
		require.NoError(t, err)
		//nolint:testifylint // explicit untyped-nil check is the contract under test
		assert.True(t, err == nil, "must compare equal to untyped nil")
	})

	t.Run("non-empty accumulator returns the accumulator", func(t *testing.T) {
		var fe FieldErrors
		fe.Append("name", "is required")
		err := fe.OrNil()

		require.Error(t, err)
		var extracted *FieldErrors
		require.ErrorAs(t, err, &extracted)
		assert.Equal(t, 1, extracted.Len())
	})

	t.Run("nil receiver returns nil", func(t *testing.T) {
		var fe *FieldErrors
		require.NoError(t, fe.OrNil())
	})
}

// TestFieldErrors_Is_Bridge verifies the bridge to sdkerrors.ErrValidation.
// errors.Is(err, sdkerrors.ErrValidation) must succeed for any non-empty
// FieldErrors so callers can write unified validation predicates.
func TestFieldErrors_Is(t *testing.T) {
	t.Run("matches ErrValidation when non-empty", func(t *testing.T) {
		var fe FieldErrors
		fe.Append("name", "is required")
		err := fe.OrNil()

		require.ErrorIs(t, err, sdkerrors.ErrValidation,
			"errors.Is(err, sdkerrors.ErrValidation) must match a non-empty FieldErrors")
	})

	t.Run("does not match unrelated sentinel", func(t *testing.T) {
		var fe FieldErrors
		fe.Append("name", "is required")
		err := fe.OrNil()

		require.NotErrorIs(t, err, sdkerrors.ErrNotFound)
		require.NotErrorIs(t, err, sdkerrors.ErrAuth)
	})

	t.Run("errors.As extracts FieldErrors for field walk", func(t *testing.T) {
		var fe FieldErrors
		fe.Append("name", "is required")
		fe.Append("email", "invalid format")
		err := fe.OrNil()

		var extracted *FieldErrors
		require.ErrorAs(t, err, &extracted)
		require.Equal(t, 2, extracted.Len())
		require.Equal(t, "name", extracted.Errs()[0].Field)
		require.Equal(t, "email", extracted.Errs()[1].Field)
	})
}

// TestFieldErrors_Errs_NilSafety covers the accessor on a nil receiver.
func TestFieldErrors_Errs_NilSafety(t *testing.T) {
	var fe *FieldErrors
	assert.Nil(t, fe.Errs())
	assert.Equal(t, 0, fe.Len())
}

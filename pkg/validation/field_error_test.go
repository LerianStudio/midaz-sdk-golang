package validation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
				"'invalid'",
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
		assert.Equal(t, "", fe.Error())
	})

	t.Run("Returns formatted string for single error", func(t *testing.T) {
		fe := NewFieldErrors()
		fe.Add("email", "bad@", "invalid email")

		errStr := fe.Error()
		assert.Contains(t, errStr, "Validation failed with 1 field errors")
		assert.Contains(t, errStr, "email")
	})

	t.Run("Returns formatted string for multiple errors", func(t *testing.T) {
		fe := NewFieldErrors()
		fe.Add("email", "bad@", "invalid email")
		fe.Add("name", "", "name required")

		errStr := fe.Error()
		assert.Contains(t, errStr, "Validation failed with 2 field errors")
		assert.Contains(t, errStr, "1.")
		assert.Contains(t, errStr, "2.")
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

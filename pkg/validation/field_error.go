package validation

import (
	"fmt"
	"strings"
)

// FieldError represents a validation error for a specific field
// with rich context and suggestions for fixing the problem.
type FieldError struct {
	// Field is the path to the field that has a validation error
	// For nested fields, use dot notation (e.g., "metadata.user.address")
	Field string

	// Value is the invalid value that caused the error
	Value any

	// Message is a human-readable description of the error
	Message string

	// Code is an error code for programmatic error handling
	Code string

	// Constraint is the specific constraint that was violated (e.g., "required", "min", "max")
	Constraint string

	// Suggestions are potential ways to fix the error
	Suggestions []string
}

// Error implements the error interface for FieldError
func (fe *FieldError) Error() string {
	var builder strings.Builder

	// Start with the field name
	builder.WriteString(fmt.Sprintf("Invalid field '%s'", fe.Field))

	// Add the value if available
	if fe.Value != nil {
		builder.WriteString(fmt.Sprintf(": '%v'", fe.Value))
	}

	// Add the message
	if fe.Message != "" {
		builder.WriteString(fmt.Sprintf(" - %s", fe.Message))
	}

	// Add constraint information if provided
	if fe.Constraint != "" {
		builder.WriteString(fmt.Sprintf(" (constraint: %s)", fe.Constraint))
	}

	// Add suggestions if available
	if len(fe.Suggestions) > 0 {
		builder.WriteString("\nSuggestions:")
		for _, suggestion := range fe.Suggestions {
			builder.WriteString(fmt.Sprintf("\n- %s", suggestion))
		}
	}

	return builder.String()
}

// BuildFieldError creates a field error with common fields
func BuildFieldError(field string, value any, message string) *FieldError {
	return &FieldError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// WithCode adds an error code to a field error
func (fe *FieldError) WithCode(code string) *FieldError {
	fe.Code = code
	return fe
}

// WithConstraint adds a constraint to a field error
func (fe *FieldError) WithConstraint(constraint string) *FieldError {
	fe.Constraint = constraint
	return fe
}

// WithSuggestions adds suggestions to a field error
func (fe *FieldError) WithSuggestions(suggestions ...string) *FieldError {
	fe.Suggestions = suggestions
	return fe
}

// FieldErrors represents a collection of field errors
type FieldErrors struct {
	Errors []*FieldError
}

// Add adds a new field error to the collection
func (fe *FieldErrors) Add(field string, value any, message string) *FieldError {
	fieldError := BuildFieldError(field, value, message)
	fe.Errors = append(fe.Errors, fieldError)
	return fieldError
}

// AddError adds an existing field error to the collection
func (fe *FieldErrors) AddError(err *FieldError) {
	fe.Errors = append(fe.Errors, err)
}

// HasErrors returns true if there are any errors in the collection
func (fe *FieldErrors) HasErrors() bool {
	return len(fe.Errors) > 0
}

// Error implements the error interface for FieldErrors
func (fe *FieldErrors) Error() string {
	if !fe.HasErrors() {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Validation failed with %d field errors:\n", len(fe.Errors)))

	for i, err := range fe.Errors {
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, err.Error()))
	}

	return builder.String()
}

// GetFieldErrors returns all field errors in the collection
func (fe *FieldErrors) GetFieldErrors() []*FieldError {
	return fe.Errors
}

// GetErrorsForField returns all errors for a specific field
func (fe *FieldErrors) GetErrorsForField(field string) []*FieldError {
	var errors []*FieldError
	for _, err := range fe.Errors {
		// Match exact field or field with dot notation path
		if err.Field == field || strings.HasPrefix(err.Field, field+".") {
			errors = append(errors, err)
		}
	}
	return errors
}

// NewFieldErrors creates a new empty FieldErrors collection
func NewFieldErrors() *FieldErrors {
	return &FieldErrors{
		Errors: []*FieldError{},
	}
}

// WrapError wraps a regular error as a field error
func WrapError(field string, value any, err error) *FieldError {
	return BuildFieldError(field, value, err.Error())
}

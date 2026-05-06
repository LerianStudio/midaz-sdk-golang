package validation

import (
	"fmt"
	"reflect"
	"strings"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
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
	if fe == nil {
		return "<nil field error>"
	}

	var builder strings.Builder

	// Start with the field name
	_, _ = fmt.Fprintf(&builder, "Invalid field '%s'", fe.Field)

	// Add the value if available
	if fe.Value != nil {
		_, _ = fmt.Fprintf(&builder, ": '%s'", fe.safeValue())
	}

	// Add the message
	if fe.Message != "" {
		_, _ = fmt.Fprintf(&builder, " - %s", fe.Message)
	}

	// Add constraint information if provided
	if fe.Constraint != "" {
		_, _ = fmt.Fprintf(&builder, " (constraint: %s)", fe.Constraint)
	}

	// Add suggestions if available
	if len(fe.Suggestions) > 0 {
		_, _ = builder.WriteString("\nSuggestions:")

		for _, suggestion := range fe.Suggestions {
			_, _ = fmt.Fprintf(&builder, "\n- %s", suggestion)
		}
	}

	return builder.String()
}

func (fe *FieldError) safeValue() string {
	if strings.Contains(strings.ToLower(fe.Field), "metadata") {
		return "<redacted>"
	}

	rv := reflect.ValueOf(fe.Value)
	if rv.IsValid() {
		switch rv.Kind() {
		case reflect.Map, reflect.Slice, reflect.Array:
			return fmt.Sprintf("<%T redacted>", fe.Value)
		}
	}

	value := fmt.Sprint(fe.Value)
	if len(value) > 128 {
		return value[:128] + "..."
	}

	return value
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

// FieldErrors represents a collection of field errors. The canonical
// multi-field validation accumulator used by every Validate() method
// in [github.com/LerianStudio/midaz-sdk-golang/v3/models].
//
// See also:
//   - [FieldErrors.Append] — record a field problem.
//   - [FieldErrors.AppendWith] — record with structured Value/Code/Constraint/Suggest.
//   - [FieldErrors.OrNil] — return nil when no errors accumulated (Go nil-interface trap fix).
//   - [FieldErrors.Errs] — programmatic walk over accumulated errors.
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors.ErrValidation] — sentinel for errors.Is.
type FieldErrors struct {
	Errors []*FieldError
}

// Add adds a new field error to the collection
func (fe *FieldErrors) Add(field string, value any, message string) *FieldError {
	if fe == nil {
		return BuildFieldError(field, value, message)
	}

	fieldError := BuildFieldError(field, value, message)
	fe.Errors = append(fe.Errors, fieldError)

	return fieldError
}

// AddError adds an existing field error to the collection
func (fe *FieldErrors) AddError(err *FieldError) {
	if fe == nil || err == nil {
		return
	}

	fe.Errors = append(fe.Errors, err)
}

// HasErrors returns true if there are any errors in the collection
func (fe *FieldErrors) HasErrors() bool {
	if fe == nil {
		return false
	}

	return len(fe.Errors) > 0
}

// Error implements the error interface for FieldErrors.
//
// The render format is a single-line, semicolon-joined sequence:
//
//	validation failed: <field> <message>; <field> <message>; ...
//
// This shape preserves the "<field> <message>" substring contract that
// callers and tests rely on (e.g., strings.Contains(err.Error(),
// "name is required")) while keeping the output compact and log-friendly.
//
// Field errors that include richer context (Value, Code, Constraint,
// Suggestions) defer to the per-field [FieldError.Error] renderer
// rather than the flat shape, since the structured form is more useful
// when those fields are populated.
func (fe *FieldErrors) Error() string {
	if !fe.HasErrors() {
		return ""
	}

	var builder strings.Builder

	_, _ = builder.WriteString("validation failed: ")

	first := true

	for _, err := range fe.Errors {
		if err == nil {
			continue
		}

		if !first {
			_, _ = builder.WriteString("; ")
		}

		first = false

		// If the field error carries only the basic Field + Message,
		// render the flat form. Anything richer falls through to the
		// per-field renderer, which surfaces value/constraint/suggestions.
		if err.Value == nil && err.Code == "" && err.Constraint == "" && len(err.Suggestions) == 0 {
			if err.Field != "" {
				_, _ = fmt.Fprintf(&builder, "%s %s", err.Field, err.Message)
			} else {
				_, _ = builder.WriteString(err.Message)
			}

			continue
		}

		_, _ = builder.WriteString(err.Error())
	}

	return builder.String()
}

// GetFieldErrors returns all field errors in the collection
func (fe *FieldErrors) GetFieldErrors() []*FieldError {
	if fe == nil {
		return nil
	}

	return fe.Errors
}

// GetErrorsForField returns all errors for a specific field
func (fe *FieldErrors) GetErrorsForField(field string) []*FieldError {
	if fe == nil {
		return nil
	}

	var errors []*FieldError

	for _, err := range fe.Errors {
		if err == nil {
			continue
		}

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
	if err == nil {
		return nil
	}

	return BuildFieldError(field, value, err.Error())
}

// FieldOption configures a FieldError when added via [FieldErrors.AppendWith].
// Options are composable and applied left-to-right; later options overwrite
// earlier ones for the same field.
type FieldOption func(*FieldError)

// Value attaches the offending value to a FieldError. Sensitive fields
// (any path containing the substring "metadata") have their value
// redacted automatically when rendered.
func Value(value any) FieldOption {
	return func(fe *FieldError) { fe.Value = value }
}

// Code attaches a machine-readable code to a FieldError.
func Code(code string) FieldOption {
	return func(fe *FieldError) { fe.Code = code }
}

// Constraint attaches a constraint name (e.g., "required", "min", "max",
// "format", "enum") to a FieldError.
func Constraint(constraint string) FieldOption {
	return func(fe *FieldError) { fe.Constraint = constraint }
}

// Suggest attaches one or more remediation suggestions to a FieldError.
// Each call replaces any prior suggestions; pass all suggestions in a
// single call.
func Suggest(suggestions ...string) FieldOption {
	return func(fe *FieldError) { fe.Suggestions = suggestions }
}

// Append records a single field-level validation problem with the given
// message. It is the ergonomic shortcut for the common case where the
// caller does not need to attach a value, code, constraint, or
// suggestions. Append is nil-safe; calling it on a nil receiver is a
// no-op (matching the pattern other accumulator methods use).
//
// Example:
//
//	var errs validation.FieldErrors
//	if input.Name == "" {
//	    errs.Append("name", "is required")
//	}
//	return errs.OrNil()
func (fe *FieldErrors) Append(field, message string) {
	if fe == nil {
		return
	}

	fe.Errors = append(fe.Errors, &FieldError{Field: field, Message: message})
}

// AppendWith records a field-level validation problem and applies one or
// more [FieldOption]s for additional context. Use this when a plain
// message is not enough — e.g., to attach a constraint name, the
// offending value, or remediation suggestions.
//
// Example:
//
//	var errs validation.FieldErrors
//	errs.AppendWith("assetCode", "must be 3-4 uppercase letters",
//	    validation.Constraint("format"),
//	    validation.Suggest("Use codes like USD, EUR, BTC"),
//	)
func (fe *FieldErrors) AppendWith(field, message string, opts ...FieldOption) {
	if fe == nil {
		return
	}

	item := &FieldError{Field: field, Message: message}
	for _, opt := range opts {
		if opt != nil {
			opt(item)
		}
	}

	fe.Errors = append(fe.Errors, item)
}

// OrNil returns nil when the accumulator is empty, and the accumulator
// itself otherwise. Use this as the final statement of a Validate
// method to return a typed-nil-safe error.
//
// Without OrNil, the naïve pattern
//
//	func (i *Input) Validate() error { var fe validation.FieldErrors; ...; return &fe }
//
// returns a non-nil *FieldErrors wrapped in a non-nil error interface
// even when no problems were collected — the classic Go interface-nil
// pitfall. OrNil sidesteps it by returning an untyped nil when
// appropriate.
//
// Example:
//
//	func (i *CreateInput) Validate() error {
//	    var errs validation.FieldErrors
//	    if i.Name == "" { errs.Append("name", "is required") }
//	    return errs.OrNil()
//	}
func (fe *FieldErrors) OrNil() error {
	if fe == nil || len(fe.Errors) == 0 {
		return nil
	}

	return fe
}

// Errs returns the accumulated field errors. Returns nil for a nil
// receiver and an empty slice for an empty accumulator — never nil
// when the accumulator has been initialized.
func (fe *FieldErrors) Errs() []*FieldError {
	if fe == nil {
		return nil
	}

	return fe.Errors
}

// Len returns the number of accumulated field errors.
func (fe *FieldErrors) Len() int {
	if fe == nil {
		return 0
	}

	return len(fe.Errors)
}

// Is reports whether this FieldErrors collection should be treated as
// the same kind of error as target. It returns true when target is
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors.ErrValidation],
// enabling the canonical pattern:
//
//	if errors.Is(err, sdkerrors.ErrValidation) { ... }
//
// to match both server-side validation errors (returned as *Error with
// CategoryValidation) and client-side accumulators alike.
//
// Callers who want to walk individual field problems should use
// [errors.As] to extract the *FieldErrors:
//
//	var fe *validation.FieldErrors
//	if errors.As(err, &fe) {
//	    for _, item := range fe.Errs() {
//	        log.Printf("field=%s message=%s", item.Field, item.Message)
//	    }
//	}
func (fe *FieldErrors) Is(target error) bool {
	if fe == nil || target == nil {
		return false
	}

	t, ok := target.(*sdkerrors.Error)
	if !ok || t == nil {
		return false
	}

	return t.Category == sdkerrors.CategoryValidation
}

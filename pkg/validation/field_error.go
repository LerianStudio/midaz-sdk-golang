package validation

import (
	"fmt"
	"reflect"
	"strings"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// FieldError represents a validation error for a specific field
// with rich context and suggestions for fixing the problem.
//
// Audit C4 (CRITICAL): every caller-derived field is annotated
// `json:"-"` so a naive json.Marshal cannot leak credentials or PII.
// The renderer in [FieldError.Error] always passes the composed string
// through [sdkerrors.RedactSensitiveString] as defense in depth, and
// [FieldError.safeValue] consults [sdkerrors.IsSensitiveFieldName]
// before exposing a value.
type FieldError struct {
	// Field is the path to the field that has a validation error
	// For nested fields, use dot notation (e.g., "metadata.user.address")
	Field string `json:"field,omitempty"`

	// Value is the invalid value that caused the error.
	// Marked json:"-" because the value is the most common leak source —
	// passwords, tokens, document numbers, and IBANs all show up here
	// when a field-level validation fails.
	Value any `json:"-"`

	// Message is a human-readable description of the error.
	// Marked json:"-" because dynamic messages can echo the offending
	// value; consumers should rely on Error() (always redacted) for
	// rendered output.
	Message string `json:"-"`

	// Code is an error code for programmatic error handling
	Code string `json:"code,omitempty"`

	// Constraint is the specific constraint that was violated (e.g., "required", "min", "max")
	Constraint string `json:"constraint,omitempty"`

	// Suggestions are potential ways to fix the error.
	// Marked json:"-" because suggestion strings often quote the
	// offending value back to the caller.
	Suggestions []string `json:"-"`
}

// Error implements the error interface for FieldError.
//
// Audit C7 (CRITICAL): the composed string is always passed through
// [sdkerrors.RedactSensitiveString] before return, so the rendered
// error cannot leak credentials even if the caller stuffs them into
// the Message or Suggestions slice. The per-field [safeValue] redactor
// strips the Value first; the package-level redactor here is defense
// in depth on the rendered string.
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

	return sdkerrors.RedactSensitiveString(builder.String())
}

// safeValueMaxBytes caps the rendered value before it lands in the
// composed error string. The cap is 128 bytes; anything longer is
// truncated with an ellipsis. The redactor runs BEFORE truncation so
// that a token sitting in the truncated suffix cannot bleed through.
const safeValueMaxBytes = 128

// safeValue renders fe.Value for inclusion in the error string with
// three layers of protection:
//
//  1. If the field name matches the SDK-wide sensitive allowlist
//     ([sdkerrors.IsSensitiveFieldName]) — password, apiKey,
//     authorization, document, cpf, X-API-Key, etc. — return a fixed
//     "<redacted>" placeholder without ever touching the value.
//  2. For maps/slices/arrays, return only the type name; the contents
//     could be anything.
//  3. Otherwise, run the value through [sdkerrors.RedactSensitiveString]
//     before truncating to [safeValueMaxBytes]. Redaction-then-truncate
//     order matters: truncating first could amputate a sensitive
//     substring mid-redaction and leak the prefix.
//
// Audit C4: the v2 implementation only redacted when the field name
// contained the substring "metadata". That left password, apiKey,
// document, cpf, creditCard, and Authorization fields rendering their
// raw values. The shared predicate now covers all of them.
func (fe *FieldError) safeValue() string {
	if sdkerrors.IsSensitiveFieldName(fe.Field) {
		return "<redacted>"
	}

	rv := reflect.ValueOf(fe.Value)
	if rv.IsValid() {
		switch rv.Kind() {
		case reflect.Map, reflect.Slice, reflect.Array:
			return fmt.Sprintf("<%T redacted>", fe.Value)
		}
	}

	value := sdkerrors.RedactSensitiveString(fmt.Sprint(fe.Value))
	if len(value) > safeValueMaxBytes {
		return value[:safeValueMaxBytes] + "..."
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
//
// Audit C7: the composed string is always passed through
// [sdkerrors.RedactSensitiveString] before return so a credential in a
// field's Message cannot leak even if the per-field renderer's
// safeguards are bypassed (e.g., callers who construct *FieldError
// directly without going through [BuildFieldError]).
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

	return sdkerrors.RedactSensitiveString(builder.String())
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

// Value attaches the offending value to a FieldError. Values for
// sensitive field names — passwords, API keys, authorization headers,
// document numbers, credit cards, idempotency keys, and any other
// field matching [sdkerrors.IsSensitiveFieldName] — are redacted
// automatically when the error is rendered. Map/slice/array values
// are also collapsed to a "<type redacted>" placeholder. The rendered
// string then passes through [sdkerrors.RedactSensitiveString] as
// defense in depth.
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
// the same kind of error as target. It returns true when target is the
// broad [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors.ErrValidation]
// sentinel, enabling the canonical pattern:
//
//	if errors.Is(err, sdkerrors.ErrValidation) { ... }
//
// to match both server-side validation errors (returned as *Error with
// CategoryValidation) and client-side accumulators alike.
//
// Audit M12: narrow Code-bearing sentinels (ErrAssetMismatch,
// ErrAccountEligibility, etc.) no longer over-match. The previous
// implementation matched any target with CategoryValidation, so
// `errors.Is(fe, ErrAssetMismatch)` returned true for any
// *FieldErrors. The check now requires either:
//   - target with no Code (broad sentinel like ErrValidation), or
//   - target whose Code matches some field error's Code.
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

	if t.Category != sdkerrors.CategoryValidation {
		return false
	}

	// Broad sentinel (no Code or matches the generic validation
	// sentinel pointer): any non-empty FieldErrors counts.
	if t.Code == "" || t.Code == sdkerrors.CodeValidation {
		return true
	}

	// Narrow sentinel (e.g. ErrAssetMismatch, ErrAccountEligibility):
	// only match when at least one field error carries the same Code.
	for _, item := range fe.Errors {
		if item != nil && item.Code == string(t.Code) {
			return true
		}
	}

	return false
}

package validation_test

import (
	"errors"
	"fmt"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/validation"
)

// ExampleFieldErrors_Append demonstrates the canonical accumulator
// pattern for multi-field validation. Append every problem you find,
// then return errs.OrNil() — when no errors were appended, OrNil
// returns a typed nil that errors.Is treats as 'no error.'
func ExampleFieldErrors_Append() {
	var errs validation.FieldErrors
	errs.Append("name", "is required")
	errs.Append("email", "is not a valid email address")

	fmt.Println(errs.Len())
	fmt.Println(errs.Error())
	// Output:
	// 2
	// validation failed: name is required; email is not a valid email address
}

// ExampleFieldErrors_OrNil shows the Go nil-interface trap fix.
// Returning *FieldErrors(nil) directly would produce a non-nil error
// interface (the typed nil trap). OrNil returns a real nil error when
// the accumulator is empty, which errors.Is and == nil checks handle
// correctly.
func ExampleFieldErrors_OrNil() {
	noProblems := func() error {
		var errs validation.FieldErrors
		// (no Append calls)
		return errs.OrNil()
	}

	fmt.Println(noProblems() == nil)
	// Output: true
}

// ExampleFieldErrors_AppendWith demonstrates the rich-context variant.
// FieldOption composables (Value, Code, Constraint, Suggest) decorate
// each FieldError with structured information for programmatic
// downstream consumption — for instance, a UI that highlights the
// offending field and shows the suggested values.
func ExampleFieldErrors_AppendWith() {
	var errs validation.FieldErrors
	errs.AppendWith("status", "must be one of allowed values",
		validation.Value("UNKNOWN"),
		validation.Code("invalid_enum"),
		validation.Constraint("oneof: ACTIVE, PENDING, CLOSED"),
		validation.Suggest("ACTIVE", "PENDING", "CLOSED"),
	)

	for _, fe := range errs.Errs() {
		fmt.Printf("field=%s value=%v code=%s\n", fe.Field, fe.Value, fe.Code)
	}
	// Output: field=status value=UNKNOWN code=invalid_enum
}

// ExampleFieldErrors_Is shows the bridge to the SDK-wide
// sdkerrors.ErrValidation sentinel. errors.Is(err, ErrValidation)
// returns true for both *FieldErrors and *Error values with
// CategoryValidation, so consumer code can use one predicate
// regardless of which layer produced the error.
func ExampleFieldErrors_Is() {
	var errs validation.FieldErrors
	errs.Append("amount", "must be positive")

	if errors.Is(errs.OrNil(), sdkerrors.ErrValidation) {
		fmt.Println("input failed validation")
	}
	// Output: input failed validation
}

// ExampleFieldErrors_Errs demonstrates programmatic walking of every
// accumulated error — the right shape for surfacing structured details
// to a UI, or batch-emitting metrics per failed field.
func ExampleFieldErrors_Errs() {
	var errs validation.FieldErrors
	errs.Append("legalName", "is required")
	errs.Append("legalDocument", "must be a valid CPF or CNPJ")

	for _, fe := range errs.Errs() {
		fmt.Printf("- %s: %s\n", fe.Field, fe.Message)
	}
	// Output:
	// - legalName: is required
	// - legalDocument: must be a valid CPF or CNPJ
}

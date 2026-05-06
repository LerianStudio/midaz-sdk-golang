package errors_test

import (
	"errors"
	"fmt"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// ExampleIsNotFoundError demonstrates the canonical not-found check.
// Prefer the typed predicate over substring matching on err.Error() —
// the predicate works for both *Error values (Code == CodeNotFound)
// and the [ErrNotFound] sentinel.
func ExampleIsNotFoundError() {
	err := &sdkerrors.Error{
		Category: sdkerrors.CategoryNotFound,
		Code:     sdkerrors.CodeNotFound,
		Message:  "account not found",
	}

	if sdkerrors.IsNotFoundError(err) {
		fmt.Println("the resource does not exist")
	}
	// Output: the resource does not exist
}

// ExampleIsValidationError shows the validation-error predicate. It
// matches any *Error whose Category is CategoryValidation, and is the
// recommended way to discriminate caller-fixable errors (4xx) from
// server-side problems (5xx) at the entity layer.
func ExampleIsValidationError() {
	err := &sdkerrors.Error{
		Category: sdkerrors.CategoryValidation,
		Code:     sdkerrors.CodeValidation,
		Message:  "name is required",
	}

	if sdkerrors.IsValidationError(err) {
		fmt.Println("caller must fix the input before retrying")
	}
	// Output: caller must fix the input before retrying
}

// ExampleIsNetworkError shows that the v3 transport layer classifies
// real stdlib transport failures into typed *Error values with
// CategoryNetwork — DNS lookup failures, connection-refused, TLS handshake
// errors, etc. The predicate is the intended consumer-side surface.
func ExampleIsNetworkError() {
	err := &sdkerrors.Error{
		Category: sdkerrors.CategoryNetwork,
		Message:  "dial tcp: lookup midaz.example: no such host",
	}

	if sdkerrors.IsNetworkError(err) {
		fmt.Println("transient transport failure — retry safe")
	}
	// Output: transient transport failure — retry safe
}

// ExampleIsAuthError demonstrates the unified auth predicate. It
// returns true for both 401 (CategoryAuthentication) and 403
// (CategoryAuthorization) errors, plus any *Error with the unified
// CategoryAuth. Use this when a single 're-prompt for credentials'
// branch covers both flavors.
func ExampleIsAuthError() {
	err := &sdkerrors.Error{
		Category: sdkerrors.CategoryAuthentication,
		Message:  "token expired",
	}

	if sdkerrors.IsAuthError(err) {
		fmt.Println("re-authenticate the user")
	}
	// Output: re-authenticate the user
}

// ExampleError_Retryable shows the canonical retry-policy source. Any
// SDK error can answer 'should the caller retry?' via Retryable —
// derived from Category. The retry layer uses this internally; consumer
// code should as well rather than re-implementing a category switch.
func ExampleError_Retryable() {
	transient := &sdkerrors.Error{Category: sdkerrors.CategoryTimeout}
	permanent := &sdkerrors.Error{Category: sdkerrors.CategoryValidation}

	fmt.Println(transient.Retryable())
	fmt.Println(permanent.Retryable())
	// Output:
	// true
	// false
}

// ExampleAs demonstrates extracting a typed *Error via [errors.As] for
// programmatic field access (Category, Code, Operation, Resource, etc.).
// This is the structured-error path; predicates like IsNotFoundError
// are a thin wrapper over the same machinery.
func ExampleAs() {
	err := fmt.Errorf("wrap: %w", &sdkerrors.Error{
		Category:  sdkerrors.CategoryNotFound,
		Code:      sdkerrors.CodeNotFound,
		Operation: "GetOrganization",
		Resource:  "organization",
		Message:   "organization not found",
	})

	var sdkErr *sdkerrors.Error
	if errors.As(err, &sdkErr) {
		fmt.Printf("op=%s resource=%s\n", sdkErr.Operation, sdkErr.Resource)
	}
	// Output: op=GetOrganization resource=organization
}

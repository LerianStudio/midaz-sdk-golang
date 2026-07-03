package errors

import (
	"errors"
	"strconv"
)

// Onboarding / CRM API error codes (the *Error.APICode field), mirroring
// github.com/LerianStudio/midaz/v3/pkg/constant (server source of truth, pinned
// in contract/drift_test.go). Prefer the predicates below over hardcoding these
// strings at call sites.
//
// Retryability note: 0490/0491 are server-side 422 (UnprocessableOperationError)
// and CRM-0006 is 404 (EntityNotFoundError) — all non-retryable, already
// classified correctly by the SDK's HTTP-status→category mapping. These codes
// therefore need NO retryability override; the predicates below are ergonomic
// (let callers branch on the specific business condition), not money-path.
const (
	// APICodeSkipNotPermitted (0490): a per-call skip was requested without the
	// enabling ledger override.
	APICodeSkipNotPermitted = "0490"

	// APICodeHolderRequired (0491): account creation requires a holder (KYC).
	APICodeHolderRequired = "0491"

	// APICodeHolderNotFound (CRM-0006): the referenced CRM holder does not exist.
	APICodeHolderNotFound = "CRM-0006"
)

// feeCodeRange bounds the fee/billing engine's contiguous error-code block on
// the server (0179 ErrFeeCalculationFieldType … 0233 ErrDeductibleFeeExceedsAmount).
//
// ponytail: a numeric-range predicate over the fee block, not 55 discrete
// const+predicate pairs — callers branch on "is this a fee/billing error", not
// each internal code. Retryability across the block is mixed (mostly 4xx; a few
// compute-only 500s like ErrCalculateFee) but all correct via status: the 500s
// are side-effect-free reads (verified), so the default 5xx→retryable holds and
// no override is needed. Add discrete predicates only if a caller must branch on
// a specific fee code.
const (
	feeCodeRangeStart = 179
	feeCodeRangeEnd   = 233
)

// ErrFeatureNotAvailable is the sentinel marking a deployment feature that is
// unavailable (envelope encryption in legacy mode — the routes are not
// registered because no KMS vendor is configured, surfaced as a 404). The
// encryption facade joins it onto that 404 via MarkFeatureNotAvailable, so
// IsFeatureNotAvailable keys on the marker rather than the 404 status — a
// generic not-found does NOT satisfy it.
var ErrFeatureNotAvailable = errors.New("feature not available")

// IsSkipNotPermitted reports whether err carries the server's 0490 code (a
// per-call skip requested without the enabling ledger override).
func IsSkipNotPermitted(err error) bool { return apiCodeOf(err) == APICodeSkipNotPermitted }

// IsHolderRequired reports whether err carries the server's 0491 code (account
// creation requires a holder).
func IsHolderRequired(err error) bool { return apiCodeOf(err) == APICodeHolderRequired }

// IsHolderNotFound reports whether err carries the server's CRM-0006 code (the
// referenced CRM holder was not found). Distinct from the generic
// IsNotFoundError: this pins the CRM holder resource specifically.
func IsHolderNotFound(err error) bool { return apiCodeOf(err) == APICodeHolderNotFound }

// IsFeeError reports whether err is a fee/billing-engine error — its server code
// suffix falls in the 0179–0233 block. Family predicate: callers branch on
// "fee/billing problem", not each of the ~55 internal codes.
func IsFeeError(err error) bool {
	suffix := apiCodeSuffix(apiCodeOf(err))
	if suffix == "" {
		return false
	}

	n, convErr := strconv.Atoi(suffix)
	if convErr != nil {
		return false
	}

	return n >= feeCodeRangeStart && n <= feeCodeRangeEnd
}

// IsFeatureNotAvailable reports whether err signals a deployment feature is
// unavailable (envelope encryption in legacy mode). It keys on the
// ErrFeatureNotAvailable marker, NOT the 404 status, so a generic NotFound does
// not match — the underlying *Error{StatusCode:404} is still reachable via
// errors.As / IsNotFoundError.
func IsFeatureNotAvailable(err error) bool { return errors.Is(err, ErrFeatureNotAvailable) }

// MarkFeatureNotAvailable joins the ErrFeatureNotAvailable sentinel onto a
// not-found error, preserving the underlying *Error (404) for errors.As field
// access and IsNotFoundError. A nil or non-not-found err is returned unchanged.
// The encryption facade uses it to tag its legacy-mode 404s.
func MarkFeatureNotAvailable(err error) error {
	if !IsNotFoundError(err) {
		return err
	}

	return errors.Join(err, ErrFeatureNotAvailable)
}

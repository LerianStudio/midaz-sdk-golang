package errors

import "errors"

// Transaction lifecycle / revert business-error API codes.
//
// These are the raw Midaz API error codes (the *Error.APICode field) emitted by
// the server for transaction commit / cancel / revert preconditions and revert
// idempotency. Values mirror github.com/LerianStudio/midaz/v3/pkg/constant
// (server source of truth). Prefer the predicates below over hardcoding these
// strings at call sites.
const (
	// APICodeParentTransactionIDNotFound (0021): the revert target's parent
	// transaction could not be found.
	APICodeParentTransactionIDNotFound = "0021"

	// APICodeRevertAlreadyExists (0087): the target transaction already has a
	// child reversal transaction. This is the canonical "already reverted"
	// idempotency signal — re-attempting a revert returns this rather than
	// mutating the original (which stays APPROVED).
	APICodeRevertAlreadyExists = "0087"

	// APICodeAlreadyARevert (0088): the target transaction is itself a reversal
	// transaction and therefore cannot be reverted again.
	APICodeAlreadyARevert = "0088"

	// APICodeCannotRevert (0089): the reversal would be empty (nothing to
	// reverse), so the server refuses to create it.
	APICodeCannotRevert = "0089"

	// APICodeAmbiguousRevert (0090): the revert target is ambiguous.
	APICodeAmbiguousRevert = "0090"

	// APICodeParentIDSameID (0091): the supplied parent transaction ID equals
	// the transaction's own ID.
	APICodeParentIDSameID = "0091"

	// APICodeStatusPreconditionFailed (0099): a commit / cancel / revert was
	// attempted against a transaction whose status does not satisfy the
	// precondition (commit/cancel require PENDING; revert requires APPROVED).
	APICodeStatusPreconditionFailed = "0099"

	// APICodeRevertOnlyBidirectional (0165): revert is restricted to
	// bidirectional routes for this transaction.
	APICodeRevertOnlyBidirectional = "0165"
)

// apiCodeOf returns the raw API error code carried by err, or "" if err is nil
// or carries no *Error. It mirrors the extraction pattern used by the Is*Error
// predicate family.
func apiCodeOf(err error) string {
	if isNilError(err) {
		return ""
	}

	var sdkErr *Error
	if errors.As(err, &sdkErr) && sdkErr != nil {
		return sdkErr.APICode
	}

	return ""
}

// IsRevertAlreadyExistsError reports whether err signals that the target
// transaction has already been reverted (API code 0087) or is itself a reversal
// transaction that cannot be reverted again (0088). This is the idempotency
// signal a revert caller needs: a re-attempt on an already-reverted transaction
// returns one of these codes, and it is the canonical "already done" proof —
// the original transaction's status never changes.
func IsRevertAlreadyExistsError(err error) bool {
	switch apiCodeOf(err) {
	case APICodeRevertAlreadyExists, APICodeAlreadyARevert:
		return true
	default:
		return false
	}
}

// IsStatusPreconditionError reports whether err signals a transaction-status
// precondition failure (API code 0099): a commit or cancel attempted on a
// non-PENDING transaction, or a revert attempted on a non-APPROVED transaction.
func IsStatusPreconditionError(err error) bool {
	return apiCodeOf(err) == APICodeStatusPreconditionFailed
}

// IsCannotRevertError reports whether err signals that a transaction cannot be
// reverted because the reversal would be empty (API code 0089).
func IsCannotRevertError(err error) bool {
	return apiCodeOf(err) == APICodeCannotRevert
}

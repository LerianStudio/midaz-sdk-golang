package errors

import (
	"errors"
	"strings"
)

// Partner (actor) refusal API codes (the *Error.APICode field).
//
// A plugin that calls the Ledger on behalf of a partner relays the partner's
// bearer token, and the Ledger's authorization layer evaluates that relayed
// partner. When it refuses the relayed partner, it answers 403 with one of the
// codes below. The values mirror lib-auth `auth/middleware/actor.go` (checked
// at lib-auth v5.22.0). They are unexported literals of an internal module, so
// the contract module cannot pin them; they are documented literals here.
// lib-auth answers APICodePartnerNotAllowed for any actor refusal reason it does
// not recognize, which keeps this set stable.
//
// These codes are distinct from AUT-1020, which lib-auth emits when it refuses
// the caller's own credential (not the relayed partner), and from AUT-1009 /
// AUT-1010, which refuse a partner credential presented directly (401). Neither
// is a partner refusal.
//
// Classification note: these codes are NOT in the API-code mapping tables, so
// the error's Code (permission_error), Category (authorization) and retry
// classification stay exactly as the HTTP status yields. Prefer
// IsPartnerRefusal / GetPartnerRefusalReason over hardcoding these strings at
// call sites.
const (
	// APICodePartnerOtherTenant (AUT-1016): the relayed partner belongs to
	// another tenant.
	APICodePartnerOtherTenant = "AUT-1016"

	// APICodePartnerSuspended (AUT-1017): the relayed partner is suspended.
	APICodePartnerSuspended = "AUT-1017"

	// APICodePartnerNoLongerValid (AUT-1018): the relayed partner is outside its
	// validity period or its credential is no longer valid.
	APICodePartnerNoLongerValid = "AUT-1018"

	// APICodePartnerNotAllowed (AUT-1019): the relayed partner is not allowed to
	// perform the request. lib-auth also uses it for unrecognized refusal
	// reasons.
	APICodePartnerNotAllowed = "AUT-1019"
)

// PartnerRefusalReason is the reason the server refused the partner a request
// was made on behalf of. See GetPartnerRefusalReason.
type PartnerRefusalReason string

// Partner refusal reasons, one per partner refusal API code.
const (
	// PartnerRefusalOtherTenant maps APICodePartnerOtherTenant (AUT-1016).
	PartnerRefusalOtherTenant PartnerRefusalReason = "other_tenant"

	// PartnerRefusalSuspended maps APICodePartnerSuspended (AUT-1017).
	PartnerRefusalSuspended PartnerRefusalReason = "suspended"

	// PartnerRefusalNoLongerValid maps APICodePartnerNoLongerValid (AUT-1018).
	PartnerRefusalNoLongerValid PartnerRefusalReason = "no_longer_valid"

	// PartnerRefusalNotAllowed maps APICodePartnerNotAllowed (AUT-1019).
	PartnerRefusalNotAllowed PartnerRefusalReason = "not_allowed"
)

// partnerRefusalReasons maps each partner refusal API code to its reason.
var partnerRefusalReasons = map[string]PartnerRefusalReason{
	APICodePartnerOtherTenant:   PartnerRefusalOtherTenant,
	APICodePartnerSuspended:     PartnerRefusalSuspended,
	APICodePartnerNoLongerValid: PartnerRefusalNoLongerValid,
	APICodePartnerNotAllowed:    PartnerRefusalNotAllowed,
}

// partnerRefusalMaxDepth bounds how many SDK errors partnerRefusalReasonOf
// visits, so a self-referencing or cyclic chain of *Error values cannot loop
// forever. Real chains hold one or two SDK errors.
const partnerRefusalMaxDepth = 32

// GetPartnerRefusalReason returns the reason the server refused the partner the
// request was made on behalf of, and true, when an SDK error in err's chain
// carries one of the partner refusal API codes (AUT-1016..AUT-1019, from
// lib-auth `auth/middleware/actor.go`). Otherwise it returns "" and false.
//
// Unlike the other code predicates of this package, which read only the
// outermost SDK error, it visits every SDK error in the chain, outermost first,
// and returns the reason of the first partner refusal code found. So an SDK
// error with no code, or with another code, that wraps an SDK error carrying
// AUT-1019 is still a partner refusal. The walk moves from each SDK error to the
// error it wraps; in an errors.Join tree it follows only the branch holding the
// first SDK error found, so a refusal in a later branch is not seen.
//
// The API code MUST match a whole code after trimming surrounding spaces; a
// matching numeric suffix alone (for example CRM-1016 or 1016) MUST NOT match.
// The comparison is case-sensitive. The HTTP status does not take part in the
// decision. AUT-1020, the refusal of the caller's own credential, is not a
// partner refusal.
func GetPartnerRefusalReason(err error) (PartnerRefusalReason, bool) {
	return partnerRefusalReasonOf(err)
}

// partnerRefusalReasonOf walks the SDK errors in err's chain, outermost first,
// and returns the reason of the first partner refusal code found. Each step
// extracts the first *Error reachable from the current error (errors.As, which
// also sees through the redacting shell of (*Error).Unwrap) and continues from
// that *Error's own Err, so every step moves strictly inward; the depth bound
// covers a chain that points back at itself.
func partnerRefusalReasonOf(err error) (PartnerRefusalReason, bool) {
	for range partnerRefusalMaxDepth {
		if isNilError(err) {
			return "", false
		}

		var sdkErr *Error
		if !errors.As(err, &sdkErr) || sdkErr == nil {
			return "", false
		}

		if reason, ok := partnerRefusalReasons[strings.TrimSpace(sdkErr.APICode)]; ok {
			return reason, true
		}

		err = sdkErr.Err
	}

	return "", false
}

// IsPartnerRefusal reports whether err (or any error it wraps) signals that the
// server refused the partner the request was made on behalf of: API code
// AUT-1016, AUT-1017, AUT-1018 or AUT-1019 (from lib-auth
// `auth/middleware/actor.go`) on any SDK error in the chain. It follows the
// same chain walk and matching rules as GetPartnerRefusalReason. AUT-1020, the
// refusal of the caller's own credential, MUST NOT satisfy it.
func IsPartnerRefusal(err error) bool {
	_, ok := GetPartnerRefusalReason(err)

	return ok
}

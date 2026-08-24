// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"fmt"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// requirePathIDs rejects a request whose path ids cannot safely become one URL
// path segment, before anything reaches the wire.
//
// An empty id is not a harmless 404. The generated client formats every id
// straight into the URL, so an empty TRAILING id builds ".../balances/" — and
// Midaz runs Fiber with StrictRouting unset, which trims that slash and routes
// the request to the COLLECTION endpoint. The collection answers 200 with a
// paginated envelope, the single-object decoder unmarshals it into a
// zero-valued models.Balance, and the caller reads Available: 0 with a nil
// error. On a reconciliation path that books zero and says nothing.
//
// An empty MIDDLE id ("/organizations//ledgers/x") does 404 loudly today, but
// it is guarded all the same: a local, typed error naming the parameter beats a
// server 404 that names nothing, and it costs one comparison.
//
// A dot segment is the same defect wearing a different hat, and a worse one.
// The generated client formats an id with the "simple" style, which does not
// escape ".", ".." or "/", and only then resolves the operation path against
// the base URL — where RFC 3986 removes dot segments. So "." rebuilds exactly
// the ".../balances/" the empty-id guard exists to prevent, and ".." pops a
// segment: deleting a ledger whose id is ".." issues DELETE against the
// ORGANIZATION, and one level up it reaches "DELETE /v1/". Rejecting the id
// locally is the only place the caller's intent still exists; by the time the
// URL is resolved, a scope-escalated delete is indistinguishable from the
// delete the caller asked for.
//
// The separator rejection is safe on every parameter this helper guards,
// including the alias and external-code lookups. Midaz constrains a
// client-supplied alias to ^[a-zA-Z0-9@:_-]+$ and explicitly prohibits the
// "@external/" prefix; the external account is reached through its own route,
// which takes the bare asset code. No id Midaz accepts contains a separator, so
// no per-parameter exemption is needed.
//
// Arguments alternate name and value, so the error can name the parameter the
// caller got wrong:
//
//	requirePathIDs(operation, "organizationID", orgID, "balanceID", balanceID)
func requirePathIDs(operation string, pairs ...string) error {
	if len(pairs)%2 != 0 {
		return errors.NewInternalError(operation,
			fmt.Errorf("requirePathIDs got %d arguments; name/value pairs must be even", len(pairs)))
	}

	for i := 0; i+1 < len(pairs); i += 2 {
		value := strings.TrimSpace(pairs[i+1])
		if value == "" {
			return errors.NewMissingParameterError(operation, pairs[i])
		}

		if value == "." || value == ".." || strings.ContainsAny(value, `/\`) {
			return errors.NewValidationError(operation,
				"path id must not be a dot segment or contain a path separator",
				fmt.Errorf("%s = %q", pairs[i], pairs[i+1]))
		}
	}

	return nil
}

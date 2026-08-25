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
// Surrounding whitespace is refused for the same reason, and to keep the guard
// honest: the caller forwards the ORIGINAL string to the generated client, so a
// guard reading only the trimmed form checks a value that is not the one sent.
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
// "%" is rejected for the same reason one step earlier. A percent-encoded dot
// segment ("%2e%2e", "..%2f") is not a dot segment on our side: the generated
// client escapes the id once on its way into the path, so "%2e" leaves as
// "%252e" and RFC 3986 removal finds nothing. That safety is REAL but it is
// borrowed — it holds only while the client escapes exactly once and nothing
// between here and the ledger decodes twice, and neither of those is this SDK's
// to promise. A gateway that normalizes before forwarding turns a 404 into the
// scope-escalated DELETE the dot-segment guard exists to prevent, silently.
//
// The reason it costs nothing to refuse is that no value Midaz can serve here
// contains "%" — not because a lookup would reject one, but because none can
// ever have been CREATED:
//
//   - Every UUID parameter is uuid.Parse'd, which accepts hex and hyphens only.
//   - entity_name is matched against a closed map of entity names.
//   - An alias is constrained to ^[a-zA-Z0-9@:_-]+$ when the account is created
//     (pkg/constant/account.go:10), so no stored alias carries one.
//   - A metadata index key is constrained to ^[a-zA-Z][a-zA-Z0-9_]*$ when the
//     index is created (pkg/net/http/withBody.go, validateMetadataKeyFormat),
//     so no index that exists can be named with one.
//   - Asset and external codes are ISO-style tickers.
//
// Some of those lookups do not re-validate the path segment, so the server would
// accept a "%" and simply match nothing. That is an absence of validation, not
// an allowance: a "%" reaching this guard is a mistake or an attack, never a
// record someone is trying to read.
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
		value := pairs[i+1]
		if strings.TrimSpace(value) == "" {
			return errors.NewMissingParameterError(operation, pairs[i])
		}

		// Padding is refused rather than tolerated, so the value CHECKED here and
		// the value SENT are the same value. The caller forwards the original
		// string to the generated client, which percent-encodes it into the path,
		// so an id like "acc-1 " used to pass every guard below on its trimmed
		// form and then leave as "acc-1%20" — a server 404 naming nothing, in
		// place of the local typed error this helper exists to give.
		if value != strings.TrimSpace(value) {
			return errors.NewValidationError(operation,
				"path id must not have leading or trailing whitespace",
				fmt.Errorf("%s = %q", pairs[i], value))
		}

		if value == "." || value == ".." || strings.ContainsAny(value, `/\%`) {
			return errors.NewValidationError(operation,
				"path id must not be a dot segment or contain a path separator or percent sign",
				fmt.Errorf("%s = %q", pairs[i], value))
		}
	}

	return nil
}

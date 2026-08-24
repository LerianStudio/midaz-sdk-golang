// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"fmt"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// requirePathIDs rejects a request whose path ids are empty, before anything
// reaches the wire.
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
// Arguments alternate name and value, so the error can name the parameter the
// caller left empty:
//
//	requirePathIDs(operation, "organizationID", orgID, "balanceID", balanceID)
func requirePathIDs(operation string, pairs ...string) error {
	if len(pairs)%2 != 0 {
		return errors.NewInternalError(operation,
			fmt.Errorf("requirePathIDs got %d arguments; name/value pairs must be even", len(pairs)))
	}

	for i := 0; i < len(pairs); i += 2 {
		if strings.TrimSpace(pairs[i+1]) == "" {
			return errors.NewMissingParameterError(operation, pairs[i])
		}
	}

	return nil
}

// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation"
)

// AuditEvent is a single protection audit event as surfaced by the ledger
// plane's GET .../protection/audit listing.
//
// Every field is a string on the wire (server auditEventResponse,
// audit.go:38): FromStatus/ToStatus are lifted out of the event Details,
// Timestamp is RFC3339 UTC, and RequestID is the correlation id. The JSON
// tags mirror the server DTO byte-for-byte.
type AuditEvent struct {
	ID         string `json:"id"`
	Action     string `json:"action"`
	Actor      string `json:"actor"`
	Outcome    string `json:"outcome"`
	Reason     string `json:"reason"`
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
	Timestamp  string `json:"timestamp"`
	RequestID  string `json:"request_id"`
}

// AuditOutcome* are the restricted outcome-filter enum accepted by the audit
// endpoint (server allowedAuditOutcomes, audit.go:65). conflict and not_found
// are deferred server-side and rejected as 400, so the SDK rejects them
// client-side in Validate.
const (
	AuditOutcomeSuccess       = "success"
	AuditOutcomeFailure       = "failure"
	AuditOutcomeAlreadyExists = "already_exists"
)

// AuditEventsListOpts is the typed options struct for ListAuditEvents and the
// ListAll/ListPages iterators.
//
// AuditEventsListOpts is a value type. Concurrent-safe by construction — the
// facade layer never mutates a caller's opts.
//
// The audit endpoint is cursor-paginated: callers iterate by passing back the
// NextCursor surfaced in the previous response's Pagination shape. Page-based
// fields are intentionally absent — the endpoint does not honor them.
//
// Embeds CursorListOpts for the shared cursor/sort/date-range fields. The
// audit-specific filters (Action/Actor/Outcome) are flat fields; StartDate and
// EndDate live in the embedded CursorListOpts.
type AuditEventsListOpts struct {
	CursorListOpts

	// Action narrows results to a single audit action. Empty means no filter.
	Action string

	// Actor narrows results to a single actor identity. Empty means no filter.
	Actor string

	// Outcome narrows results to one of the restricted outcomes
	// (success/failure/already_exists). Empty means no filter. Validate
	// rejects any other non-empty value.
	Outcome string
}

// Validate enforces SDK-side preconditions on AuditEventsListOpts.
//
// Returns a typed validation error when the shared cursor/sort/date-range
// preconditions fail (see ValidateCursorListOpts) or when Outcome is non-empty
// and not one of the restricted values (success/failure/already_exists). An
// empty Outcome is allowed and means "no outcome filter".
//
// Validate is safe to call on a zero-value AuditEventsListOpts; the facade
// method calls it automatically before issuing the HTTP request.
func (o AuditEventsListOpts) Validate() error {
	const operation = "AuditEventsListOpts.Validate"

	if err := ValidateCursorListOpts(operation, o.CursorListOpts); err != nil {
		return err
	}

	switch o.Outcome {
	case "", AuditOutcomeSuccess, AuditOutcomeFailure, AuditOutcomeAlreadyExists:
		return nil
	default:
		var errs validation.FieldErrors
		errs.AppendWith("outcome", "must be one of success, failure, already_exists",
			validation.Constraint("enum"), validation.Value(o.Outcome))

		return errs.OrNil()
	}
}

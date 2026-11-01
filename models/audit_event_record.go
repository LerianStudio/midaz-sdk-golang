// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import "time"

// Audit result enum for the tracer audit trail. The server is the sole authority
// on these values; the SDK exposes them for callers to compare against and to
// pass as a list filter. Distinct from the ledger protection-audit AuditOutcome*
// (3 values, models/audit_event.go) — the tracer result also carries the
// validation verdicts (ALLOW/DENY/REVIEW).
const (
	AuditResultSuccess = "SUCCESS"
	AuditResultFailed  = "FAILED"
	AuditResultAllow   = "ALLOW"
	AuditResultDeny    = "DENY"
	AuditResultReview  = "REVIEW"
)

// Actor is who (or what) performed an audited action on the tracer plane.
// Mirrors the generated gentracer.Actor.
type Actor struct {
	// ActorType is the actor kind (user, api_key, system).
	ActorType string `json:"actorType"`

	// ID is the actor identity.
	ID string `json:"id"`

	// IPAddress is the origin address, when captured.
	IPAddress string `json:"ipAddress"`

	// Name is the actor display name.
	Name string `json:"name"`

	// Role is the actor role, when applicable.
	Role *string `json:"role,omitempty"`
}

// AuditEventRecord is one entry in the tracer's tamper-evident audit trail. It is
// the TRACER-plane audit event — DISTINCT from the ledger-plane protection audit
// (models.AuditEvent), a different feature with a flat status-transition shape.
// Events written before the server's batch-link cutover carry Hash and
// PreviousHash chaining each record to the previous one (SHA-256). Newer events
// carry Hash as the digest of their own row and no PreviousHash; their chain
// evidence lives in batch links the API does not expose. The server verifies
// both (see HashChainVerificationResult). Mirrors the generated
// gentracer.AuditEvent with the UUID as a plain string.
type AuditEventRecord struct {
	// EventID is the record identity (UUID).
	EventID string `json:"eventId"`

	// EventType is the audited event kind (e.g. TRANSACTION_VALIDATED, RULE_CREATED).
	EventType string `json:"eventType"`

	// Action is the audited action (VALIDATE, CREATE, UPDATE, ...).
	Action string `json:"action"`

	// Result is the outcome (SUCCESS, FAILED, ALLOW, DENY, REVIEW).
	Result string `json:"result"`

	// ResourceType is the audited resource kind (transaction, rule, limit, reservation).
	ResourceType string `json:"resourceType"`

	// ResourceID is the audited resource identity.
	ResourceID string `json:"resourceId"`

	// Actor is who performed the action.
	Actor Actor `json:"actor"`

	// Hash is this record's SHA-256: chained to PreviousHash for older events, the
	// digest of the row alone for newer ones.
	Hash *string `json:"hash,omitempty"`

	// PreviousHash chains this record to the prior one; absent on events written
	// after the batch-link cutover.
	PreviousHash *string `json:"previousHash,omitempty"`

	// Context is optional structured context captured with the event.
	Context map[string]any `json:"context,omitempty"`

	// Metadata is optional free-form context stored with the record.
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is when the record was written.
	CreatedAt time.Time `json:"createdAt"`
}

// HashChainVerificationResult is the tracer server's verdict on the audit
// integrity (GET /audit-events/{id}/verify). The server computes it and the SDK
// performs NO crypto — it only decodes the verdict. For events written before
// the batch-link cutover it walks the hash chain up to the event. For newer
// events it proves only the event's own batch and the hash pointers of its
// neighbouring links, not every earlier record. When IsValid is false,
// FirstInvalidID locates the first tampered record. An event not yet linked
// (up to ~2 s after it was written) is answered with HTTP 409, code 0566.
type HashChainVerificationResult struct {
	// IsValid reports whether the verification described above passed.
	IsValid bool `json:"isValid"`

	// FirstInvalidID locates the first tampered record when IsValid is false.
	FirstInvalidID *int64 `json:"firstInvalidId,omitempty"`

	// TotalChecked counts records walked for older events, and chain links (at
	// most 3) for newer ones.
	TotalChecked int64 `json:"totalChecked"`

	// Message is the human-readable verdict explanation.
	Message string `json:"message"`
}

// AuditEventRecordsListOpts is the typed options struct for the tracer
// audit-events cursor list. It embeds CursorListOpts for the shared
// cursor/sort-order/date fields, adds SortBy, and attaches a typed Filters
// sub-struct.
//
// Like the validations list (and unlike rules/limits), this endpoint HAS native
// start_date/end_date slots AND the tracer server strict-parses them as RFC3339 —
// diverging from the ledger plane's YYYY-MM-DD — so Validate uses
// ValidateCursorListOptsRFC3339Dates. Callers must pass RFC3339 (e.g.
// 2026-01-01T00:00:00Z), never a bare date.
type AuditEventRecordsListOpts struct {
	CursorListOpts

	// SortBy names the sort field (created_at, event_type). Empty falls back to
	// the server default. Passed through verbatim.
	SortBy string

	// Filters narrows the result set. Zero value means no narrowing.
	Filters AuditEventRecordFilters
}

// AuditEventRecordFilters is the typed filter set for the tracer audit-events
// endpoint. Each field maps to a native query-param slot on the generated
// ListAuditEventsParams.
type AuditEventRecordFilters struct {
	EventType       string
	Action          string
	Result          string
	ResourceType    string
	ResourceID      string
	ActorType       string
	ActorID         string
	AccountID       string
	SegmentID       string
	PortfolioID     string
	TransactionType string
	MatchedRuleID   string
}

// Validate enforces the shared cursor-list preconditions (limit bounds, sort
// direction, date range). Date filtering IS supported here — the generated
// ListAuditEventsParams has native start_date/end_date slots — but the tracer
// server strict-parses those as RFC3339, so this defers to
// ValidateCursorListOptsRFC3339Dates (diverging from the ledger plane's
// YYYY-MM-DD). Filter values are passed through; the server validates them.
func (o AuditEventRecordsListOpts) Validate() error {
	return ValidateCursorListOptsRFC3339Dates("AuditEventRecordsListOpts.Validate", o.CursorListOpts)
}

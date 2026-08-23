// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"
	"strconv"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/gentracer"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// auditEventsFacade is the tracer-plane audit-events facade (a twin of
// rulesFacade/limitsFacade/validationsFacade). It exposes only models.* +
// *errors.Error; the generated types never leak. All three ops are read-only
// GETs — no writes, no money, no idempotency. Verify decodes the server's
// hash-chain verdict; the SDK performs NO crypto.
//
// This is the TRACER audit trail, DISTINCT from the ledger-plane protection audit
// (entities.auditFacade over genledger / models.AuditEvent), which is a different
// feature with a flat status-transition shape.
type auditEventsFacade struct {
	tracer *gentracer.ClientWithResponses
}

// newAuditEventsFacade wires the facade over a tracer plane client.
func newAuditEventsFacade(tracer *gentracer.ClientWithResponses) *auditEventsFacade {
	return &auditEventsFacade{tracer: tracer}
}

// List retrieves one cursor page of audit-event records.
//
// The tracer serializes the list as the FLAT domain-keyed envelope
// {auditEvents:[...],hasMore,nextCursor} — NOT the {items,pagination} shape
// models.ListResponse[T].UnmarshalJSON expects (it reads Items only from the
// "items" key). Decoding the raw body straight into ListResponse would yield
// EMPTY Items — total data loss. So the body decodes into a local envelope struct
// keyed on "auditEvents" and maps into the public ListResponse. (The ledger
// protection-audit facade unmarshals straight into ListResponse — that works only
// because the LEDGER endpoint emits the {items,pagination} shape; do NOT copy it
// here, the tracer emits the flat envelope.)
func (f *auditEventsFacade) List(ctx context.Context, opts models.AuditEventRecordsListOpts) (*models.ListResponse[models.AuditEventRecord], error) {
	const operation = "AuditEvents.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.tracer.ListAuditEventsWithResponse(ctx, listAuditEventRecordsParams(opts))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var env struct {
		AuditEvents []models.AuditEventRecord `json:"auditEvents"`
		NextCursor  string                    `json:"nextCursor"`
	}
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &models.ListResponse[models.AuditEventRecord]{
		Items:      env.AuditEvents,
		Pagination: models.Pagination{NextCursor: env.NextCursor},
	}, nil
}

// ListPages yields one cursor page per iteration, advancing by the response
// nextCursor until it is empty.
func (f *auditEventsFacade) ListPages(ctx context.Context, opts models.AuditEventRecordsListOpts) iter.Seq2[*models.ListResponse[models.AuditEventRecord], error] {
	return func(yield func(*models.ListResponse[models.AuditEventRecord], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := f.List(ctx, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			// Cursor-pure stop: the tracer paginates by nextCursor, so the only
			// sound terminal signal is an empty cursor. HasMore()'s page-based
			// heuristic does not apply to this flat cursor envelope.
			if page.Pagination.NextCursor == "" {
				return
			}

			current.Cursor = page.Pagination.NextCursor
		}
	}
}

// ListAll yields every audit-event record across cursor pages, transparently
// advancing pagination.
func (f *auditEventsFacade) ListAll(ctx context.Context, opts models.AuditEventRecordsListOpts) iter.Seq2[models.AuditEventRecord, error] {
	return flattenPages(f.ListPages(ctx, opts))
}

// Get retrieves one audit-event record by ID.
func (f *auditEventsFacade) Get(ctx context.Context, id string) (*models.AuditEventRecord, error) {
	const operation = "AuditEvents.Get"

	resp, err := f.tracer.GetAuditEventWithResponse(ctx, id)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.AuditEventRecord](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Verify returns the tracer server's hash-chain integrity verdict for the audit
// trail up to the given event ID. The server (a Postgres function,
// verify_audit_hash_chain) walks the SHA-256 chain; the SDK only decodes the
// verdict — it performs NO crypto and never re-computes the chain.
func (f *auditEventsFacade) Verify(ctx context.Context, id string) (*models.HashChainVerificationResult, error) {
	const operation = "AuditEvents.Verify"

	resp, err := f.tracer.VerifyAuditEventWithResponse(ctx, id)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.HashChainVerificationResult](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// listAuditEventRecordsParams renders the typed opts into the generated
// ListAuditEventsParams. Every filter has a native *string slot; there is no
// request editor.
//
//nolint:gocyclo,cyclop // A flat sequence of independent, optional filter guards (one per query param); complexity is the field count (18 slots), not branching logic. Any "simplification" (loop/table) would obscure the exact query-param wiring this audit-trail list depends on.
func listAuditEventRecordsParams(opts models.AuditEventRecordsListOpts) *gentracer.ListAuditEventsParams {
	params := &gentracer.ListAuditEventsParams{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.Cursor != "" {
		params.Cursor = strPtr(opts.Cursor)
	}

	if opts.SortDirection != "" {
		params.SortOrder = strPtr(string(opts.SortDirection))
	}

	if opts.SortBy != "" {
		params.SortBy = strPtr(opts.SortBy)
	}

	if opts.StartDate != "" {
		params.StartDate = strPtr(opts.StartDate)
	}

	if opts.EndDate != "" {
		params.EndDate = strPtr(opts.EndDate)
	}

	if opts.Filters.EventType != "" {
		params.EventType = strPtr(opts.Filters.EventType)
	}

	if opts.Filters.Action != "" {
		params.Action = strPtr(opts.Filters.Action)
	}

	if opts.Filters.Result != "" {
		params.Result = strPtr(opts.Filters.Result)
	}

	if opts.Filters.ResourceType != "" {
		params.ResourceType = strPtr(opts.Filters.ResourceType)
	}

	if opts.Filters.ResourceID != "" {
		params.ResourceId = strPtr(opts.Filters.ResourceID)
	}

	if opts.Filters.ActorType != "" {
		params.ActorType = strPtr(opts.Filters.ActorType)
	}

	if opts.Filters.ActorID != "" {
		params.ActorId = strPtr(opts.Filters.ActorID)
	}

	if opts.Filters.AccountID != "" {
		params.AccountId = strPtr(opts.Filters.AccountID)
	}

	if opts.Filters.SegmentID != "" {
		params.SegmentId = strPtr(opts.Filters.SegmentID)
	}

	if opts.Filters.PortfolioID != "" {
		params.PortfolioId = strPtr(opts.Filters.PortfolioID)
	}

	if opts.Filters.TransactionType != "" {
		params.TransactionType = strPtr(opts.Filters.TransactionType)
	}

	if opts.Filters.MatchedRuleID != "" {
		params.MatchedRuleId = strPtr(opts.Filters.MatchedRuleID)
	}

	return params
}

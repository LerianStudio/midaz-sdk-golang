// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"
	"strconv"

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

// auditFacade is the Epic 3.3 (Task 3.3.2) hand-written facade over the
// generated genledger.ClientWithResponses for the protection audit listing
// (GET .../protection/audit). The public surface is exactly models.* +
// *errors.Error; the generated types never leak.
//
// The listing is CURSOR-paginated. Pages advances by echoing the response
// next_cursor back into the request and stops on an empty cursor — never
// HasMore(), whose page-based heuristic can loop forever on a full terminal
// page that carries no cursor (the server can return a full page with no
// next_cursor as the last page).
//
// 404 semantics: a 404 means envelope encryption — and thus the protection
// audit trail — is disabled for this deployment (legacy mode; the route is not
// registered because the KMS vendor is not configured). It is mapped cleanly to
// *errors.Error{StatusCode:404} via the standard non-2xx path
// (DecodeProblemJSON, which degrades gracefully on a bare non-RFC-9457 404
// body). This says the feature is unavailable at the deployment level, distinct
// from an empty result set (a 200 with an empty items array).
type auditFacade struct {
	ledger *genledger.ClientWithResponses
}

// newAuditFacade wires the facade over a ledger plane client.
func newAuditFacade(ledger *genledger.ClientWithResponses) *auditFacade {
	return &auditFacade{ledger: ledger}
}

// ListAuditEvents retrieves one cursor page of protection audit events for an
// organization. A 404 means the feature is disabled (legacy mode) and returns
// *errors.Error{StatusCode:404}.
func (f *auditFacade) ListAuditEvents(ctx context.Context, orgID string, opts models.AuditEventsListOpts) (*models.ListResponse[models.AuditEvent], error) {
	const operation = "Audit.ListAuditEvents"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.ledger.GetAuditEventsWithResponse(ctx, orgID, listAuditEventsParams(opts))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var page models.ListResponse[models.AuditEvent]
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &page, nil
}

// ListAuditEventsPages yields one cursor page per iteration, advancing by the
// response next_cursor until it is empty.
func (f *auditFacade) ListAuditEventsPages(ctx context.Context, orgID string, opts models.AuditEventsListOpts) iter.Seq2[*models.ListResponse[models.AuditEvent], error] {
	return func(yield func(*models.ListResponse[models.AuditEvent], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := f.ListAuditEvents(ctx, orgID, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			// Cursor-pure stop: this endpoint paginates by next_cursor, so the
			// only sound terminal signal is an empty cursor. HasMore()'s
			// page-based heuristic would loop forever on a full terminal page
			// that carries no next_cursor.
			if page.Pagination.NextCursor == "" {
				return
			}

			current.Cursor = page.Pagination.NextCursor
		}
	}
}

// ListAuditEventsAll yields every audit event across cursor pages, transparently
// advancing pagination.
func (f *auditFacade) ListAuditEventsAll(ctx context.Context, orgID string, opts models.AuditEventsListOpts) iter.Seq2[models.AuditEvent, error] {
	return flattenPages(f.ListAuditEventsPages(ctx, orgID, opts))
}

// listAuditEventsParams renders the cursor/sort/date fields plus the
// action/actor/outcome filters into the generated GetAuditEventsParams. Every
// field has a native *string slot, so no request editor is needed.
func listAuditEventsParams(opts models.AuditEventsListOpts) *genledger.GetAuditEventsParams {
	params := &genledger.GetAuditEventsParams{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.Cursor != "" {
		params.Cursor = strPtr(opts.Cursor)
	}

	if opts.SortDirection != "" {
		params.SortOrder = strPtr(string(opts.SortDirection))
	}

	if opts.Action != "" {
		params.Action = strPtr(opts.Action)
	}

	if opts.Actor != "" {
		params.Actor = strPtr(opts.Actor)
	}

	if opts.Outcome != "" {
		params.Outcome = strPtr(opts.Outcome)
	}

	if opts.StartDate != "" {
		params.StartDate = strPtr(opts.StartDate)
	}

	if opts.EndDate != "" {
		params.EndDate = strPtr(opts.EndDate)
	}

	return params
}

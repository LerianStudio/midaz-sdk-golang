// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"io"
	"iter"
	"net/http"
	"strconv"

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
)

// holdersFacade is the Epic 3.1 (Task 3.1.2) hand-written facade over the
// generated genledger.ClientWithResponses for CRM holders.
//
// RE-HOMING: this targets the GENERATED ledger-plane surface —
// /organizations/{org}/holders (org-in-path). It deliberately does NOT match
// the superseded legacy entities/holders.go wire ({crm}/holders, org-in-header);
// that legacy file is a model/method-set reference only.
//
// Holders are treated as CURSOR-paginated: Pages advances by echoing the
// response next_cursor back into the request as a query param and stops on an
// empty cursor — never HasMore(), whose page-based heuristic can loop forever on
// a full terminal page that carries no cursor. The generated ListHoldersParams
// has no cursor slot, so the cursor is injected via a request editor (like the
// name/status filters below); the caller's opts are never mutated.
//
// The generated ListHoldersParams exposes slots for limit/sort_order/
// include_deleted/external_id/document. The name and status filters have no
// slot, so the facade injects each as a query param via a request editor.
//
// Auto-idempotency IS wired: each write threads idempotencyEditors(ctx,
// f.enableIdempotency), which stamps X-Idempotency (and X-TTL when set) on the
// outbound request, gated on enableIdempotency. An explicit or context-supplied
// key (sdkctx.WithIdempotencyKey) stamps regardless of the gate. Writes stay
// replay-safe via the rewindable *bytes.Reader body in writeJSON. The public
// surface stays models.* + *errors.Error; the generated types never leak.
type holdersFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newHoldersFacade wires the facade over a ledger plane client.
func newHoldersFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *holdersFacade {
	return &holdersFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one cursor page of holders under an organization. The cursor is
// seeded from opts.Cursor (empty means the first page); Pages advances it by
// echoing the response next_cursor. The generated ListHoldersParams has no cursor
// slot, so the cursor is injected as a query param via listHoldersReqEditors; the
// caller's opts are never mutated.
func (f *holdersFacade) List(ctx context.Context, orgID string, opts models.HoldersListOpts) (*models.ListResponse[models.Holder], error) {
	const operation = "Holders.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.ledger.ListHoldersWithResponse(ctx, orgID, listHoldersParams(opts), listHoldersReqEditors(opts)...)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var page models.ListResponse[models.Holder]
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &page, nil
}

// Pages yields one cursor page per iteration, advancing by the response
// next_cursor until it is empty. current is a value copy seeded from opts, so the
// caller's opts is never mutated.
func (f *holdersFacade) Pages(ctx context.Context, orgID string, opts models.HoldersListOpts) iter.Seq2[*models.ListResponse[models.Holder], error] {
	return func(yield func(*models.ListResponse[models.Holder], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := f.List(ctx, orgID, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			// Cursor-pure stop: paginate by next_cursor, so the only sound
			// terminal signal is an empty cursor. HasMore()'s page-based
			// heuristic would loop forever on a full terminal page that carries
			// no next_cursor.
			if page.Pagination.NextCursor == "" {
				return
			}

			current.Cursor = page.Pagination.NextCursor
		}
	}
}

// All yields every holder across cursor pages, transparently advancing
// pagination.
func (f *holdersFacade) All(ctx context.Context, orgID string, opts models.HoldersListOpts) iter.Seq2[models.Holder, error] {
	return flattenPages(f.Pages(ctx, orgID, opts))
}

// Create registers a new holder under an organization via the write-facade
// pattern (marshal input -> rewindable *bytes.Reader -> WithBody variant).
// Auto-idempotency is wired via idempotencyEditors (gated on enableIdempotency).
// The server returns 201 on success.
func (f *holdersFacade) Create(ctx context.Context, orgID string, input *models.CreateHolderInput) (*models.Holder, error) {
	const operation = "Holders.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Holder](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateHolderWithBody(ctx, orgID, &genledger.CreateHolderParams{}, jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one holder by ID under an organization. When the context is
// tagged with sdkctx.WithIncludeDeleted, soft-deleted holders are included
// (mirrors legacy holders.go).
func (f *holdersFacade) Get(ctx context.Context, orgID, id string) (*models.Holder, error) {
	const operation = "Holders.Get"

	params := &genledger.GetHolderByIDParams{}
	if sdkctx.IncludeDeletedFromContext(ctx) {
		params.IncludeDeleted = strPtr("true")
	}

	resp, err := f.ledger.GetHolderByIDWithResponse(ctx, orgID, id, params)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Holder](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Update patches a holder by ID under an organization. Same write-facade pattern
// as Create; the server returns 200 on success.
func (f *holdersFacade) Update(ctx context.Context, orgID, id string, input *models.UpdateHolderInput) (*models.Holder, error) {
	const operation = "Holders.Update"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Holder](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateHolderWithBody(ctx, orgID, id, jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes a holder by ID under an organization. Soft delete by default;
// when the context is tagged with sdkctx.WithHardDelete the deletion is
// permanent. The server returns 204 with no body on success.
func (f *holdersFacade) Delete(ctx context.Context, orgID, id string) error {
	const operation = "Holders.Delete"

	params := &genledger.DeleteHolderParams{}
	if sdkctx.HardDeleteFromContext(ctx) {
		params.HardDelete = strPtr("true")
	}

	resp, err := f.ledger.DeleteHolderWithResponse(ctx, orgID, id, params, idempotencyEditors(ctx, f.enableIdempotency)...)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	return nil
}

// listHoldersParams renders the fields that have a slot in the generated
// ListHoldersParams. The name/status filters have no slot and are carried by
// listHoldersReqEditors instead.
func listHoldersParams(opts models.HoldersListOpts) *genledger.ListHoldersParams {
	params := &genledger.ListHoldersParams{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.SortDirection != "" {
		params.SortOrder = strPtr(string(opts.SortDirection))
	}

	if opts.Filters.Document != "" {
		params.Document = strPtr(opts.Filters.Document)
	}

	if opts.Filters.ExternalID != "" {
		params.ExternalId = strPtr(opts.Filters.ExternalID)
	}

	if opts.Filters.IncludeDeleted {
		params.IncludeDeleted = strPtr("true")
	}

	return params
}

// listHoldersReqEditors carries the cursor pagination token and the name/status
// filters — none of which the generated ListHoldersParams has a slot for. The
// ledger OAS omits cursor, name, and status from the holders list endpoint, so
// the SDK injects each as a query param rather than dropping it silently. Dates
// are intentionally NOT emitted: the endpoint declares no start_date/end_date
// and there is no evidence the server honors them, so HoldersListOpts.Validate
// (ValidateCursorListOptsNoDates) rejects a set date rather than sending a
// silently-ignored filter. Returns nil when none is set so the common path adds
// zero overhead.
func listHoldersReqEditors(opts models.HoldersListOpts) []genledger.RequestEditorFn {
	var editors []genledger.RequestEditorFn

	if opts.Cursor != "" {
		editors = append(editors, setQueryParam("cursor", opts.Cursor))
	}

	if opts.Filters.Name != "" {
		editors = append(editors, setQueryParam("name", opts.Filters.Name))
	}

	if opts.Filters.Status != "" {
		editors = append(editors, setQueryParam("status", opts.Filters.Status))
	}

	return editors
}

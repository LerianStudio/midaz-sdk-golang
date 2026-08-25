// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"iter"
	"net/http"
	"strconv"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// segmentsFacade is the Phase 2 (Task 2.1.a) hand-written facade over the
// generated genledger.ClientWithResponses, following the Organizations exemplar.
// The public surface is exactly models.Segment + *errors.Error + the
// List/Pages/All trinaldo + full CRUD.
//
// Segments are organization+ledger scoped, so every method threads orgID and
// ledgerID through to the generated client.
//
// The ledger OAS omits every segment filter (name, status) from
// ListSegmentsParams, so the facade injects them as query params via request
// editors (see listSegmentsReqEditors) rather than dropping them silently.
type segmentsFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newSegmentsFacade wires the facade over a ledger plane client.
func newSegmentsFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *segmentsFacade {
	return &segmentsFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of segments under an org+ledger, normalized into the
// public model.
func (f *segmentsFacade) List(ctx context.Context, orgID, ledgerID string, opts models.SegmentsListOpts) (*models.ListResponse[models.Segment], error) {
	const operation = "Segments.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListSegments(ctx, orgID, ledgerID, listSegmentsParams(opts), listSegmentsReqEditors(opts)...)

	return readList[models.Segment](operation, resp, err)
}

// Pages yields one full page per iteration, advancing while the response reports
// more results.
func (f *segmentsFacade) Pages(ctx context.Context, orgID, ledgerID string, opts models.SegmentsListOpts) iter.Seq2[*models.ListResponse[models.Segment], error] {
	return func(yield func(*models.ListResponse[models.Segment], error) bool) {
		current := opts
		if current.Page == 0 {
			current.Page = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := f.List(ctx, orgID, ledgerID, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			if !page.Pagination.HasMore() {
				return
			}

			current.Page++
		}
	}
}

// All yields every segment across pages, transparently advancing pagination.
func (f *segmentsFacade) All(ctx context.Context, orgID, ledgerID string, opts models.SegmentsListOpts) iter.Seq2[models.Segment, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new segment under an org+ledger via the write-facade
// pattern.
func (f *segmentsFacade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateSegmentInput) (*models.Segment, error) {
	const operation = "Segments.Create"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Segment](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateSegmentWithBody(ctx, orgID, ledgerID, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one segment by ID under an org+ledger.
func (f *segmentsFacade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.Segment, error) {
	const operation = "Segments.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetSegmentByID(ctx, orgID, ledgerID, id)

	return readOne[models.Segment](operation, resp, err)
}

// Update patches a segment by ID under an org+ledger. Same write-facade pattern
// as Create.
func (f *segmentsFacade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateSegmentInput) (*models.Segment, error) {
	const operation = "Segments.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Segment](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateSegmentWithBody(ctx, orgID, ledgerID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes a segment by ID under an org+ledger. The server returns 204
// with no body on success.
func (f *segmentsFacade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "Segments.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteSegment(ctx, orgID, ledgerID, id, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// Count returns the total number of segments under an org+ledger via
// HEAD .../segments/metrics/count, reading the X-Total-Count header. It routes
// through the raw CountSegments + readCount so a headers-only error reply (empty
// body) maps to the real status rather than an internal error.
func (f *segmentsFacade) Count(ctx context.Context, orgID, ledgerID string) (int, error) {
	if err := requirePathIDs("Segments.Count", "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return 0, err
	}

	//nolint:bodyclose // readCount (transactions_facade.go) closes resp.Body via defer.
	return readCount(f.ledger.CountSegments(ctx, orgID, ledgerID))
}

// listSegmentsParams renders only the pagination/sort/date fields the generated
// ListSegmentsParams exposes. The segment filters (name/status) have no slot and
// are carried by listSegmentsReqEditors instead.
func listSegmentsParams(opts models.SegmentsListOpts) *genledger.ListSegmentsParams {
	params := &genledger.ListSegmentsParams{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.Page > 0 {
		params.Page = strPtr(strconv.Itoa(opts.Page))
	}

	if opts.SortDirection != "" {
		params.SortOrder = strPtr(string(opts.SortDirection))
	}

	if opts.StartDate != "" {
		params.StartDate = strPtr(opts.StartDate)
	}

	if opts.EndDate != "" {
		params.EndDate = strPtr(opts.EndDate)
	}

	return params
}

// listSegmentsReqEditors carries the segment filters the generated
// ListSegmentsParams cannot express. The OAS omits name/status/include_deleted,
// so the SDK injects each set filter as a query param (matching the wire names in
// models.SegmentsListOpts.ToQueryParams) rather than dropping it silently.
// Returns nil when no filter is set.
func listSegmentsReqEditors(opts models.SegmentsListOpts) []genledger.RequestEditorFn {
	var editors []genledger.RequestEditorFn

	if opts.Filters.Name != "" {
		editors = append(editors, setQueryParam("name", opts.Filters.Name))
	}

	if opts.Filters.Status != "" {
		editors = append(editors, setQueryParam("status", opts.Filters.Status))
	}

	if opts.Filters.IncludeDeleted {
		editors = append(editors, setQueryParam("include_deleted", "true"))
	}

	return editors
}

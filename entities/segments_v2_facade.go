// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"iter"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// segmentsV2Facade serves the /v2 segment surface — the secondary
// classification accounts are reported by.
type segmentsV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newSegmentsV2Facade wires the facade over a ledger plane client.
func newSegmentsV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *segmentsV2Facade {
	return &segmentsV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of segments under an org+ledger.
func (f *segmentsV2Facade) List(ctx context.Context, orgID, ledgerID string, opts models.SegmentsListOpts) (*models.ListResponse[models.Segment], error) {
	const operation = "V2.Segments.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListSegmentsV2(ctx, orgID, ledgerID, listSegmentsV2Params(opts), listSegmentsReqEditors(opts)...)

	return readList[models.Segment](operation, resp, err)
}

// Pages yields one full page per iteration, advancing by page number.
func (f *segmentsV2Facade) Pages(ctx context.Context, orgID, ledgerID string, opts models.SegmentsListOpts) iter.Seq2[*models.ListResponse[models.Segment], error] {
	return pageSeq(ctx, opts,
		func(o *models.SegmentsListOpts) *int { return &o.Page },
		func(current models.SegmentsListOpts) (*models.ListResponse[models.Segment], error) {
			return f.List(ctx, orgID, ledgerID, current)
		})
}

// All yields every segment across pages.
func (f *segmentsV2Facade) All(ctx context.Context, orgID, ledgerID string, opts models.SegmentsListOpts) iter.Seq2[models.Segment, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new segment under an org+ledger.
func (f *segmentsV2Facade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateSegmentInput) (*models.Segment, error) {
	const operation = "V2.Segments.Create"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Segment](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateSegmentV2WithBody(ctx, orgID, ledgerID, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one segment by ID.
func (f *segmentsV2Facade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.Segment, error) {
	const operation = "V2.Segments.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetSegmentByIDV2(ctx, orgID, ledgerID, id)

	return readOne[models.Segment](operation, resp, err)
}

// Update patches a segment by ID.
func (f *segmentsV2Facade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateSegmentInput) (*models.Segment, error) {
	const operation = "V2.Segments.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Segment](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateSegmentV2WithBody(ctx, orgID, ledgerID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes a segment by ID.
func (f *segmentsV2Facade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "V2.Segments.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteSegmentV2(ctx, orgID, ledgerID, id, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// Count returns the total number of segments under an org+ledger, read from
// the X-Total-Count header of a HEAD request.
func (f *segmentsV2Facade) Count(ctx context.Context, orgID, ledgerID string) (int, error) {
	if err := requirePathIDs("V2.Segments.Count", "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return 0, err
	}

	//nolint:bodyclose // readCount closes resp.Body via defer.
	return readCount(f.ledger.CountSegmentsV2(ctx, orgID, ledgerID))
}

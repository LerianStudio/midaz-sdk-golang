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
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// assetRatesFacade is the Phase 2 (Task 2.3.3) hand-written facade over the
// generated genledger.ClientWithResponses. It preserves the prior wire format
// byte-for-byte: the upsert PUT
// to .../asset-rates, the read GET .../asset-rates/{externalId}, and the
// cursor-paginated list GET .../asset-rates/from/{assetCode}.
//
// AssetRates is upsert-only: there is no separate Update endpoint. The server
// keys on the from/to asset-code tuple, so CreateOrUpdateAssetRate is a single
// PUT — the SDK surfaces exactly that, never a fictitious Create+Update pair.
//
// Money-path wire shape: the rate/scale ride the wire as JSON integers in the
// int+scale fixed-point shape ({rate:525,scale:2} == 5.25), which is
// json.Marshal(*models.CreateAssetRateInput) byte-for-byte — the input carries
// no custom marshaler, and a JSON integer is a valid OAS number. Sending a float
// would lose fixed-point precision, so the facade never divides rate by scale.
// The 200 body decodes into models.AssetRate, whose Rate is a *decimal.Decimal
// that preserves the full fixed-point integer with no truncation.
//
// The list is CURSOR-paginated. Pages advances by echoing the response
// next_cursor back into the request and stops on an empty cursor — never
// HasMore(), whose page-based heuristic can loop forever on a full terminal
// page that carries no cursor. The to[] filter has a native slot in
// GetAllAssetRatesByAssetCodeParams that serializes explode=false as a single
// comma-joined param (to=BRL,EUR), byte-identical to the legacy ToQueryParams
// strings.Join, so no request editor is needed.
//
// Auto-idempotency IS wired: each write threads idempotencyEditors(ctx,
// f.enableIdempotency), which stamps X-Idempotency (and X-TTL when set) on the
// outbound request, gated on enableIdempotency. An explicit or context-supplied
// key (sdkctx.WithIdempotencyKey) stamps regardless of the gate. Writes stay
// replay-safe via the rewindable *bytes.Reader body in writeJSON. The public
// surface stays models.* + *errors.Error; the generated types never leak.
type assetRatesFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newAssetRatesFacade wires the facade over a ledger plane client.
func newAssetRatesFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *assetRatesFacade {
	return &assetRatesFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// CreateOrUpdateAssetRate upserts an asset rate under an org+ledger via the
// write-facade pattern (marshal input -> rewindable *bytes.Reader -> WithBody
// variant). It is a PUT keyed on the from/to tuple; the server returns 200 on
// success. The rate/scale serialize as fixed-point integers (see the facade
// doc). There is no separate Update endpoint.
func (f *assetRatesFacade) CreateOrUpdateAssetRate(ctx context.Context, orgID, ledgerID string, input *models.CreateAssetRateInput) (*models.AssetRate, error) {
	const operation = "AssetRates.CreateOrUpdateAssetRate"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.AssetRate](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateOrUpdateAssetRateWithBody(ctx, orgID, ledgerID, jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// GetAssetRate retrieves one asset rate by its external ID under an org+ledger.
func (f *assetRatesFacade) GetAssetRate(ctx context.Context, orgID, ledgerID, externalID string) (*models.AssetRate, error) {
	const operation = "AssetRates.GetAssetRate"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "externalID", externalID); err != nil {
		return nil, err
	}

	resp, err := f.ledger.GetAssetRateByExternalIDWithResponse(ctx, orgID, ledgerID, externalID)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.AssetRate](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// ListAssetRatesByAssetCode retrieves one cursor page of asset rates for a
// source asset code under an org+ledger.
func (f *assetRatesFacade) ListAssetRatesByAssetCode(ctx context.Context, orgID, ledgerID, assetCode string, opts models.AssetRatesListOpts) (*models.ListResponse[models.AssetRate], error) {
	const operation = "AssetRates.ListAssetRatesByAssetCode"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "assetCode", assetCode); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAllAssetRatesByAssetCode(ctx, orgID, ledgerID, assetCode, listAssetRatesParams(opts))

	return readList[models.AssetRate](operation, resp, err)
}

// ListAssetRatesByAssetCodePages yields one cursor page per iteration, advancing
// by the response next_cursor until it is empty.
func (f *assetRatesFacade) ListAssetRatesByAssetCodePages(ctx context.Context, orgID, ledgerID, assetCode string, opts models.AssetRatesListOpts) iter.Seq2[*models.ListResponse[models.AssetRate], error] {
	return func(yield func(*models.ListResponse[models.AssetRate], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := f.ListAssetRatesByAssetCode(ctx, orgID, ledgerID, assetCode, current)
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

// ListAssetRatesByAssetCodeAll yields every asset rate across cursor pages,
// transparently advancing pagination.
func (f *assetRatesFacade) ListAssetRatesByAssetCodeAll(ctx context.Context, orgID, ledgerID, assetCode string, opts models.AssetRatesListOpts) iter.Seq2[models.AssetRate, error] {
	return flattenPages(f.ListAssetRatesByAssetCodePages(ctx, orgID, ledgerID, assetCode, opts))
}

// listAssetRatesParams renders the cursor/sort/date fields plus the to[] filter
// into the generated GetAllAssetRatesByAssetCodeParams. Every field has a native
// slot; to[] serializes explode=false as a single comma-joined param
// (to=BRL,EUR), byte-identical to the legacy ToQueryParams strings.Join, so no
// request editor is needed.
func listAssetRatesParams(opts models.AssetRatesListOpts) *genledger.GetAllAssetRatesByAssetCodeParams {
	params := &genledger.GetAllAssetRatesByAssetCodeParams{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.Cursor != "" {
		params.Cursor = strPtr(opts.Cursor)
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

	if len(opts.Filters.To) > 0 {
		to := opts.Filters.To
		params.To = &to
	}

	return params
}

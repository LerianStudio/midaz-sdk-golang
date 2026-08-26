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

// assetsFacade is the Phase 2 (Task 2.1.a) hand-written facade over the
// generated genledger.ClientWithResponses, following the Organizations exemplar.
// The public surface is exactly models.Asset + *errors.Error + the List/Pages/All
// trinaldo + full CRUD.
//
// Assets are organization+ledger scoped, so every method threads orgID and
// ledgerID through to the generated client.
//
// Unlike the other Phase 2 resources, the ledger OAS omits every asset filter
// (code, type, status) from ListAssetsParams. Rather than drop those filters
// silently, the facade injects all three as query params via request editors
// (see listAssetsReqEditors).
type assetsFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newAssetsFacade wires the facade over a ledger plane client.
func newAssetsFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *assetsFacade {
	return &assetsFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of assets under an org+ledger, normalized into the
// public model.
func (f *assetsFacade) List(ctx context.Context, orgID, ledgerID string, opts models.AssetsListOpts) (*models.ListResponse[models.Asset], error) {
	const operation = "Assets.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListAssets(ctx, orgID, ledgerID, listAssetsParams(opts), listAssetsReqEditors(opts)...)

	return readList[models.Asset](operation, resp, err)
}

// Pages yields one full page per iteration, advancing while the response reports
// more results.
func (f *assetsFacade) Pages(ctx context.Context, orgID, ledgerID string, opts models.AssetsListOpts) iter.Seq2[*models.ListResponse[models.Asset], error] {
	return func(yield func(*models.ListResponse[models.Asset], error) bool) {
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

// All yields every asset across pages, transparently advancing pagination.
func (f *assetsFacade) All(ctx context.Context, orgID, ledgerID string, opts models.AssetsListOpts) iter.Seq2[models.Asset, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new asset under an org+ledger via the write-facade pattern.
// CreateAsset carries a generated params struct (an optional Authorization
// override); the facade leaves it empty and lets the auth round tripper stamp
// the bearer token.
func (f *assetsFacade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateAssetInput) (*models.Asset, error) {
	const operation = "Assets.Create"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Asset](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateAssetWithBody(ctx, orgID, ledgerID, &genledger.CreateAssetParams{},
			jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one asset by ID under an org+ledger.
func (f *assetsFacade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.Asset, error) {
	const operation = "Assets.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAssetByID(ctx, orgID, ledgerID, id)

	return readOne[models.Asset](operation, resp, err)
}

// Update patches an asset by ID under an org+ledger. Same write-facade pattern
// as Create.
func (f *assetsFacade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateAssetInput) (*models.Asset, error) {
	const operation = "Assets.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Asset](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateAssetWithBody(ctx, orgID, ledgerID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes an asset by ID under an org+ledger. The server returns 204 with
// no body on success.
func (f *assetsFacade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "Assets.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteAsset(ctx, orgID, ledgerID, id, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// Count returns the total number of assets under an org+ledger via
// HEAD .../assets/metrics/count, reading the X-Total-Count header. It routes
// through the raw CountAssets + readCount so a headers-only error reply (empty
// body) maps to the real status rather than an internal error.
func (f *assetsFacade) Count(ctx context.Context, orgID, ledgerID string) (int, error) {
	if err := requirePathIDs("Assets.Count", "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return 0, err
	}

	//nolint:bodyclose // readCount (transactions_facade.go) closes resp.Body via defer.
	return readCount(f.ledger.CountAssets(ctx, orgID, ledgerID))
}

// listAssetsParams renders only the pagination/sort/date fields the generated
// ListAssetsParams exposes. The asset filters (code/type/status) have no slot
// and are carried by listAssetsReqEditors instead.
func listAssetsParams(opts models.AssetsListOpts) *genledger.ListAssetsParams {
	params := &genledger.ListAssetsParams{}

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

// listAssetsReqEditors carries the asset filters the generated ListAssetsParams
// cannot express. The ledger OAS omits code/type/status from the assets list
// endpoint, so the SDK injects each set filter as a query param (matching the
// wire names in models.AssetsListOpts.ToQueryParams) rather than dropping it
// silently. Returns nil when no filter is set.
func listAssetsReqEditors(opts models.AssetsListOpts) []genledger.RequestEditorFn {
	var editors []genledger.RequestEditorFn

	if opts.Filters.Code != "" {
		editors = append(editors, setQueryParam("code", opts.Filters.Code))
	}

	if opts.Filters.Type != "" {
		editors = append(editors, setQueryParam("type", opts.Filters.Type))
	}

	if opts.Filters.Status != "" {
		editors = append(editors, setQueryParam("status", opts.Filters.Status))
	}

	return editors
}

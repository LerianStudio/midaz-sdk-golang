// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"iter"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v6/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
)

// assetsV2Facade serves the /v2 asset surface — the currencies and
// instruments a ledger's accounts are denominated in.
type assetsV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newAssetsV2Facade wires the facade over a ledger plane client.
func newAssetsV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *assetsV2Facade {
	return &assetsV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of assets under an org+ledger.
func (f *assetsV2Facade) List(ctx context.Context, orgID, ledgerID string, opts models.AssetsListOpts) (*models.ListResponse[models.Asset], error) {
	const operation = "V2.Assets.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListAssetsV2(ctx, orgID, ledgerID, listAssetsV2Params(opts), listAssetsReqEditors(opts)...)

	return readList[models.Asset](operation, resp, err)
}

// Pages yields one full page per iteration, advancing by page number.
func (f *assetsV2Facade) Pages(ctx context.Context, orgID, ledgerID string, opts models.AssetsListOpts) iter.Seq2[*models.ListResponse[models.Asset], error] {
	return pageSeq(ctx, opts,
		func(o *models.AssetsListOpts) *int { return &o.Page },
		func(current models.AssetsListOpts) (*models.ListResponse[models.Asset], error) {
			return f.List(ctx, orgID, ledgerID, current)
		})
}

// All yields every asset across pages.
func (f *assetsV2Facade) All(ctx context.Context, orgID, ledgerID string, opts models.AssetsListOpts) iter.Seq2[models.Asset, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new asset under an org+ledger.
func (f *assetsV2Facade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateAssetInput) (*models.Asset, error) {
	const operation = "V2.Assets.Create"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Asset](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateAssetV2WithBody(ctx, orgID, ledgerID, &genledger.CreateAssetV2Params{}, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one asset by ID.
func (f *assetsV2Facade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.Asset, error) {
	const operation = "V2.Assets.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAssetByIDV2(ctx, orgID, ledgerID, id)

	return readOne[models.Asset](operation, resp, err)
}

// Update patches an asset by ID.
func (f *assetsV2Facade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateAssetInput) (*models.Asset, error) {
	const operation = "V2.Assets.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Asset](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateAssetV2WithBody(ctx, orgID, ledgerID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes an asset by ID.
func (f *assetsV2Facade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "V2.Assets.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteAssetV2(ctx, orgID, ledgerID, id, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// Count returns the total number of assets under an org+ledger, read from
// the X-Total-Count header of a HEAD request.
func (f *assetsV2Facade) Count(ctx context.Context, orgID, ledgerID string) (int, error) {
	if err := requirePathIDs("V2.Assets.Count", "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return 0, err
	}

	//nolint:bodyclose // readCount closes resp.Body via defer.
	return readCount(f.ledger.CountAssetsV2(ctx, orgID, ledgerID))
}

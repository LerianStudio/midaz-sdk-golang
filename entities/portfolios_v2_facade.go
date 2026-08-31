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

// portfoliosV2Facade serves the /v2 portfolio surface — the grouping that
// ties a set of accounts to one entity.
type portfoliosV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newPortfoliosV2Facade wires the facade over a ledger plane client.
func newPortfoliosV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *portfoliosV2Facade {
	return &portfoliosV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of portfolios under an org+ledger.
func (f *portfoliosV2Facade) List(ctx context.Context, orgID, ledgerID string, opts models.PortfoliosListOpts) (*models.ListResponse[models.Portfolio], error) {
	const operation = "V2.Portfolios.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListPortfoliosV2(ctx, orgID, ledgerID, listPortfoliosV2Params(opts), listPortfoliosReqEditors(opts)...)

	return readList[models.Portfolio](operation, resp, err)
}

// Pages yields one full page per iteration, advancing by page number.
func (f *portfoliosV2Facade) Pages(ctx context.Context, orgID, ledgerID string, opts models.PortfoliosListOpts) iter.Seq2[*models.ListResponse[models.Portfolio], error] {
	return pageSeq(ctx, opts,
		func(o *models.PortfoliosListOpts) *int { return &o.Page },
		func(current models.PortfoliosListOpts) (*models.ListResponse[models.Portfolio], error) {
			return f.List(ctx, orgID, ledgerID, current)
		})
}

// All yields every portfolio across pages.
func (f *portfoliosV2Facade) All(ctx context.Context, orgID, ledgerID string, opts models.PortfoliosListOpts) iter.Seq2[models.Portfolio, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new portfolio under an org+ledger.
func (f *portfoliosV2Facade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreatePortfolioInput) (*models.Portfolio, error) {
	const operation = "V2.Portfolios.Create"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Portfolio](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreatePortfolioV2WithBody(ctx, orgID, ledgerID, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one portfolio by ID.
func (f *portfoliosV2Facade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.Portfolio, error) {
	const operation = "V2.Portfolios.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetPortfolioByIDV2(ctx, orgID, ledgerID, id)

	return readOne[models.Portfolio](operation, resp, err)
}

// Update patches a portfolio by ID.
func (f *portfoliosV2Facade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdatePortfolioInput) (*models.Portfolio, error) {
	const operation = "V2.Portfolios.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Portfolio](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdatePortfolioV2WithBody(ctx, orgID, ledgerID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes a portfolio by ID.
func (f *portfoliosV2Facade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "V2.Portfolios.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeletePortfolioV2(ctx, orgID, ledgerID, id, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// Count returns the total number of portfolios under an org+ledger, read from
// the X-Total-Count header of a HEAD request.
func (f *portfoliosV2Facade) Count(ctx context.Context, orgID, ledgerID string) (int, error) {
	if err := requirePathIDs("V2.Portfolios.Count", "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return 0, err
	}

	//nolint:bodyclose // readCount closes resp.Body via defer.
	return readCount(f.ledger.CountPortfoliosV2(ctx, orgID, ledgerID))
}

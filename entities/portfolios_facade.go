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
)

// portfoliosFacade is the Phase 2 (Task 2.1.a) hand-written facade over the
// generated genledger.ClientWithResponses, following the Organizations exemplar.
// The public surface is exactly models.Portfolio + *errors.Error + the
// List/Pages/All trinaldo + full CRUD.
//
// Portfolios are organization+ledger scoped, so every method threads orgID and
// ledgerID through to the generated client.
type portfoliosFacade struct {
	ledger *genledger.ClientWithResponses
}

// newPortfoliosFacade wires the facade over a ledger plane client.
func newPortfoliosFacade(ledger *genledger.ClientWithResponses) *portfoliosFacade {
	return &portfoliosFacade{ledger: ledger}
}

// List retrieves one page of portfolios under an org+ledger, normalized into the
// public model.
func (f *portfoliosFacade) List(ctx context.Context, orgID, ledgerID string, opts models.PortfoliosListOpts) (*models.ListResponse[models.Portfolio], error) {
	const operation = "Portfolios.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.ledger.ListPortfoliosWithResponse(ctx, orgID, ledgerID, listPortfoliosParams(opts), listPortfoliosReqEditors(opts)...)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var page models.ListResponse[models.Portfolio]
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &page, nil
}

// Pages yields one full page per iteration, advancing while the response reports
// more results.
func (f *portfoliosFacade) Pages(ctx context.Context, orgID, ledgerID string, opts models.PortfoliosListOpts) iter.Seq2[*models.ListResponse[models.Portfolio], error] {
	return func(yield func(*models.ListResponse[models.Portfolio], error) bool) {
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

// All yields every portfolio across pages, transparently advancing pagination.
func (f *portfoliosFacade) All(ctx context.Context, orgID, ledgerID string, opts models.PortfoliosListOpts) iter.Seq2[models.Portfolio, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new portfolio under an org+ledger via the write-facade
// pattern.
func (f *portfoliosFacade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreatePortfolioInput) (*models.Portfolio, error) {
	const operation = "Portfolios.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Portfolio](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.CreatePortfolioWithBodyWithResponse(ctx, orgID, ledgerID, "application/json", body)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// Get retrieves one portfolio by ID under an org+ledger.
func (f *portfoliosFacade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.Portfolio, error) {
	const operation = "Portfolios.Get"

	resp, err := f.ledger.GetPortfolioByIDWithResponse(ctx, orgID, ledgerID, id)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Portfolio](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Update patches a portfolio by ID under an org+ledger. Same write-facade
// pattern as Create.
func (f *portfoliosFacade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdatePortfolioInput) (*models.Portfolio, error) {
	const operation = "Portfolios.Update"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Portfolio](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.UpdatePortfolioWithBodyWithResponse(ctx, orgID, ledgerID, id, "application/json", body)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// Delete removes a portfolio by ID under an org+ledger. The server returns 204
// with no body on success.
func (f *portfoliosFacade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "Portfolios.Delete"

	resp, err := f.ledger.DeletePortfolioWithResponse(ctx, orgID, ledgerID, id)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	return nil
}

// Count returns the total number of portfolios under an org+ledger via
// HEAD .../portfolios/metrics/count, reading the X-Total-Count header. It routes
// through the raw CountPortfolios + readCount so a headers-only error reply
// (empty body) maps to the real status rather than an internal error.
func (f *portfoliosFacade) Count(ctx context.Context, orgID, ledgerID string) (int, error) {
	//nolint:bodyclose // readCount (transactions_facade.go) closes resp.Body via defer.
	return readCount(f.ledger.CountPortfolios(ctx, orgID, ledgerID))
}

// listPortfoliosParams renders the typed opts into the generated params.
// EntityID and Status map to generated slots; Name and IncludeDeleted have no
// slot and are injected via request editors (see listPortfoliosReqEditors).
func listPortfoliosParams(opts models.PortfoliosListOpts) *genledger.ListPortfoliosParams {
	params := &genledger.ListPortfoliosParams{}

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

	if opts.Filters.EntityID != "" {
		params.EntityId = strPtr(opts.Filters.EntityID)
	}

	if opts.Filters.Status != "" {
		params.Status = strPtr(opts.Filters.Status)
	}

	return params
}

// listPortfoliosReqEditors carries filters the generated ListPortfoliosParams
// cannot express. The OAS omits name (a filter the endpoint honors) and
// include_deleted, so the SDK injects each set filter as a query param (matching
// the wire names in models.PortfoliosListOpts.ToQueryParams) rather than dropping
// it silently. Returns nil when no filter is set.
func listPortfoliosReqEditors(opts models.PortfoliosListOpts) []genledger.RequestEditorFn {
	var editors []genledger.RequestEditorFn

	if opts.Filters.Name != "" {
		editors = append(editors, setQueryParam("name", opts.Filters.Name))
	}

	if opts.Filters.IncludeDeleted {
		editors = append(editors, setQueryParam("include_deleted", "true"))
	}

	return editors
}

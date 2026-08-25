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

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/gentracer"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// validationsFacade is the tracer-plane validations facade (a twin of
// rulesFacade/limitsFacade). It exposes only models.* + *errors.Error; the
// generated types never leak. Money (Amount, the LimitUsageDetail triple) is
// shopspring/decimal end to end — never float.
//
// It creates the shared context models (models.AccountContext et al.) that
// Epic 4.2's reservation facade reuses.
type validationsFacade struct {
	tracer *gentracer.ClientWithResponses
}

// newValidationsFacade wires the facade over a tracer plane client.
func newValidationsFacade(tracer *gentracer.ClientWithResponses) *validationsFacade {
	return &validationsFacade{tracer: tracer}
}

// Evaluate submits a transaction for validation against the active rules/limits.
//
// The server returns 200 on an idempotent replay (dedup on the requestId BODY
// field, never the X-Idempotency header) and 201 on a new verdict, but the
// generated ValidateTransactionResp parser fills JSON200 on an exact 200 only —
// a status-exact parse would DROP the 201 body. So the write routes through the
// raw ...WithBody call + readRawResponse + the 2xx success gate in writeJSON,
// which decodes ANY 2xx (200 and 201 alike) into models.ValidationResponse
// WITHOUT surfacing the replay/new distinction. The opaque openapi_types.File
// body forces the WithBody variant regardless.
func (f *validationsFacade) Evaluate(ctx context.Context, input *models.ValidateTransactionInput) (*models.ValidationResponse, error) {
	const operation = "Validations.Evaluate"

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.ValidationResponse](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.tracer.ValidateTransactionWithBody(ctx, jsonContentType, body))
	})
}

// Get retrieves one stored validation record by ID.
func (f *validationsFacade) Get(ctx context.Context, id string) (*models.TransactionValidation, error) {
	const operation = "Validations.Get"

	if err := requirePathIDs(operation, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.tracer.GetValidation(ctx, id)

	return readOne[models.TransactionValidation](operation, resp, err)
}

// List retrieves one cursor page of stored validations.
//
// The tracer serializes the list as the FLAT domain-keyed envelope
// {transactionValidations:[...],hasMore,nextCursor} — NOT the {items,pagination}
// shape models.ListResponse[T].UnmarshalJSON expects (it reads Items only from
// the "items" key). Decoding the raw body straight into ListResponse would yield
// EMPTY Items — total data loss. So the body decodes into a local envelope struct
// keyed on "transactionValidations" and maps into the public ListResponse.
func (f *validationsFacade) List(ctx context.Context, opts models.ValidationsListOpts) (*models.ListResponse[models.ValidationSummary], error) {
	const operation = "Validations.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	httpResp, body, err := readRawResponse(f.tracer.ListValidations(ctx, listValidationsParams(opts)))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if err := guardListBody(operation, httpResp.StatusCode, body, httpResp); err != nil {
		return nil, err
	}

	var env struct {
		TransactionValidations []models.ValidationSummary `json:"transactionValidations"`
		NextCursor             string                     `json:"nextCursor"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, errors.NewResponseDecodeError(operation, httpResp.StatusCode, err)
	}

	return &models.ListResponse[models.ValidationSummary]{
		Items:      env.TransactionValidations,
		Pagination: models.Pagination{NextCursor: env.NextCursor},
	}, nil
}

// ListPages yields one cursor page per iteration, advancing by the response
// nextCursor until it is empty.
func (f *validationsFacade) ListPages(ctx context.Context, opts models.ValidationsListOpts) iter.Seq2[*models.ListResponse[models.ValidationSummary], error] {
	return func(yield func(*models.ListResponse[models.ValidationSummary], error) bool) {
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

			if page.Pagination.NextCursor == "" {
				return
			}

			current.Cursor = page.Pagination.NextCursor
		}
	}
}

// ListAll yields every validation across cursor pages, transparently advancing
// pagination.
func (f *validationsFacade) ListAll(ctx context.Context, opts models.ValidationsListOpts) iter.Seq2[models.ValidationSummary, error] {
	return flattenPages(f.ListPages(ctx, opts))
}

// listValidationsParams renders the typed opts into the generated
// ListValidationsParams. Every filter has a native *string slot; there is no
// request editor.
func listValidationsParams(opts models.ValidationsListOpts) *gentracer.ListValidationsParams {
	params := &gentracer.ListValidationsParams{}

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

	if opts.Filters.Decision != "" {
		params.Decision = strPtr(opts.Filters.Decision)
	}

	if opts.Filters.AccountID != "" {
		params.AccountId = strPtr(opts.Filters.AccountID)
	}

	if opts.Filters.MatchedRuleID != "" {
		params.MatchedRuleId = strPtr(opts.Filters.MatchedRuleID)
	}

	if opts.Filters.ExceededLimitID != "" {
		params.ExceededLimitId = strPtr(opts.Filters.ExceededLimitID)
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

	return params
}

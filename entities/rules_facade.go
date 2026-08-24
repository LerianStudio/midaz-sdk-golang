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

// rulesFacade is the FIRST hand-written facade over the TRACER plane
// (gentracer.ClientWithResponses), the tracer twin of the ledger-plane facades.
// It exposes only models.* + *errors.Error; the generated types never leak.
type rulesFacade struct {
	tracer *gentracer.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless. Per-instance so
	// clients with different WithIdempotency settings stay isolated.
	enableIdempotency bool
}

// newRulesFacade wires the facade over a tracer plane client.
func newRulesFacade(tracer *gentracer.ClientWithResponses, enableIdempotency bool) *rulesFacade {
	return &rulesFacade{tracer: tracer, enableIdempotency: enableIdempotency}
}

// Create registers a new rule. The server returns 201, and the generated
// CreateRuleResp parser is status-EXACT (it fills JSON201 on exactly 201 and
// nothing else), so the write routes through the raw ...WithBody call +
// readRawResponse + the 2xx success gate in writeJSON instead: the facade decodes
// any 2xx into models.Rule and therefore does not break if the server ever
// answers a different success status. The opaque openapi_types.File body forces
// the WithBody variant regardless.
func (f *rulesFacade) Create(ctx context.Context, input *models.CreateRuleInput) (*models.Rule, error) {
	const operation = "Rules.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Rule](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.tracer.CreateRuleWithBody(ctx, jsonContentType, body, idempotencyEditorsTracer(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one rule by ID.
func (f *rulesFacade) Get(ctx context.Context, id string) (*models.Rule, error) {
	const operation = "Rules.Get"

	resp, err := f.tracer.GetRuleWithResponse(ctx, id)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Rule](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Update patches a rule by ID (PATCH, 200).
func (f *rulesFacade) Update(ctx context.Context, id string, input *models.UpdateRuleInput) (*models.Rule, error) {
	const operation = "Rules.Update"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Rule](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.tracer.UpdateRuleWithBody(ctx, id, jsonContentType, body, idempotencyEditorsTracer(ctx, f.enableIdempotency)...))
	})
}

// Delete removes a rule by ID. The server returns 204 with no body, so there is
// nothing to decode — only a non-2xx maps into the unified error.
func (f *rulesFacade) Delete(ctx context.Context, id string) error {
	const operation = "Rules.Delete"

	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	resp, body, err := readRawResponse(f.tracer.DeleteRule(ctx, id, idempotencyEditorsTracer(ctx, f.enableIdempotency)...))
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode) {
		return errors.DecodeProblemJSON(resp.StatusCode, body, requestIDOf(resp))
	}

	return nil
}

// Activate transitions a rule to ACTIVE (body-less POST, 200 + rule).
func (f *rulesFacade) Activate(ctx context.Context, id string) (*models.Rule, error) {
	return ruleTransition(ctx, "Rules.Activate", f.tracer.ActivateRule, id)
}

// Deactivate transitions a rule to INACTIVE (body-less POST, 200 + rule).
func (f *rulesFacade) Deactivate(ctx context.Context, id string) (*models.Rule, error) {
	return ruleTransition(ctx, "Rules.Deactivate", f.tracer.DeactivateRule, id)
}

// Draft transitions a rule back to DRAFT (body-less POST, 200 + rule).
func (f *rulesFacade) Draft(ctx context.Context, id string) (*models.Rule, error) {
	return ruleTransition(ctx, "Rules.Draft", f.tracer.DraftRule, id)
}

// ruleTransition runs a body-less lifecycle POST through the raw call so success
// is decided on 2xx (never the status-exact generated parser) and the 200 body
// decodes into models.Rule. Lifecycle transitions are actions (autoGen=false):
// no auto-gen key, but a caller-supplied ctx/explicit key still rides.
func ruleTransition(ctx context.Context, operation string, call func(context.Context, string, ...gentracer.RequestEditorFn) (*http.Response, error), id string) (*models.Rule, error) {
	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	resp, body, err := readRawResponse(call(ctx, id, idempotencyEditorsTracer(ctx, false)...))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Rule](operation, resp.StatusCode, body, resp)
}

// List retrieves one cursor page of rules.
//
// The tracer serializes lists as the FLAT domain-keyed envelope
// {rules:[...],hasMore,nextCursor} — NOT the {items,pagination} shape
// models.ListResponse[T].UnmarshalJSON expects (it reads Items only from the
// "items" key). Decoding the raw body straight into ListResponse would yield
// EMPTY Items — total data loss. So the body decodes into a local envelope struct
// (models.Rule tags match the wire) and maps into the public ListResponse.
func (f *rulesFacade) List(ctx context.Context, opts models.RulesListOpts) (*models.ListResponse[models.Rule], error) {
	const operation = "Rules.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.tracer.ListRulesWithResponse(ctx, listRulesParams(opts))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var env struct {
		Rules      []models.Rule `json:"rules"`
		NextCursor string        `json:"nextCursor"`
	}
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &models.ListResponse[models.Rule]{
		Items:      env.Rules,
		Pagination: models.Pagination{NextCursor: env.NextCursor},
	}, nil
}

// ListPages yields one cursor page per iteration, advancing by the response
// nextCursor until it is empty.
func (f *rulesFacade) ListPages(ctx context.Context, opts models.RulesListOpts) iter.Seq2[*models.ListResponse[models.Rule], error] {
	return func(yield func(*models.ListResponse[models.Rule], error) bool) {
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

			// Cursor-pure stop: the tracer paginates by nextCursor, so the only
			// sound terminal signal is an empty cursor. HasMore()'s page-based
			// heuristic does not apply to this flat cursor envelope.
			if page.Pagination.NextCursor == "" {
				return
			}

			current.Cursor = page.Pagination.NextCursor
		}
	}
}

// ListAll yields every rule across cursor pages, transparently advancing
// pagination.
func (f *rulesFacade) ListAll(ctx context.Context, opts models.RulesListOpts) iter.Seq2[models.Rule, error] {
	return flattenPages(f.ListPages(ctx, opts))
}

// listRulesParams renders the typed opts into the generated ListRulesParams.
// Every filter has a native *string slot; there is no request editor.
func listRulesParams(opts models.RulesListOpts) *gentracer.ListRulesParams {
	params := &gentracer.ListRulesParams{}

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

	if opts.Filters.Name != "" {
		params.Name = strPtr(opts.Filters.Name)
	}

	if opts.Filters.Status != "" {
		params.Status = strPtr(opts.Filters.Status)
	}

	if opts.Filters.Action != "" {
		params.Action = strPtr(opts.Filters.Action)
	}

	if opts.Filters.AccountID != "" {
		params.AccountId = strPtr(opts.Filters.AccountID)
	}

	if opts.Filters.SegmentID != "" {
		params.SegmentId = strPtr(opts.Filters.SegmentID)
	}

	if opts.Filters.PortfolioID != "" {
		params.PortfolioId = strPtr(opts.Filters.PortfolioID)
	}

	if opts.Filters.MerchantID != "" {
		params.MerchantId = strPtr(opts.Filters.MerchantID)
	}

	if opts.Filters.TransactionType != "" {
		params.TransactionType = strPtr(opts.Filters.TransactionType)
	}

	if opts.Filters.SubType != "" {
		params.SubType = strPtr(opts.Filters.SubType)
	}

	return params
}

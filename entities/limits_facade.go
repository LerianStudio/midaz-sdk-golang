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

// limitsFacade is the tracer-plane limits facade (the twin of rulesFacade). It
// exposes only models.* + *errors.Error; the generated types never leak. Money
// (models.Limit.MaxAmount, UsageSnapshot.CurrentUsage/LimitAmount) is
// shopspring/decimal end to end — never float.
type limitsFacade struct {
	tracer *gentracer.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newLimitsFacade wires the facade over a tracer plane client.
func newLimitsFacade(tracer *gentracer.ClientWithResponses, enableIdempotency bool) *limitsFacade {
	return &limitsFacade{tracer: tracer, enableIdempotency: enableIdempotency}
}

// Create registers a new limit. The server returns 201, and the generated
// CreateLimitResp parser is status-EXACT (it fills JSON201 on exactly 201 and
// nothing else), so the write routes through the raw ...WithBody call +
// readRawResponse + the 2xx success gate in writeJSON instead: the facade decodes
// any 2xx into models.Limit and therefore does not break if the server ever
// answers a different success status. The opaque openapi_types.File body forces
// the WithBody variant regardless.
func (f *limitsFacade) Create(ctx context.Context, input *models.CreateLimitInput) (*models.Limit, error) {
	const operation = "Limits.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Limit](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.tracer.CreateLimitWithBody(ctx, jsonContentType, body, idempotencyEditorsTracer(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one limit by ID.
func (f *limitsFacade) Get(ctx context.Context, id string) (*models.Limit, error) {
	const operation = "Limits.Get"

	if err := requirePathIDs(operation, "id", id); err != nil {
		return nil, err
	}

	resp, err := f.tracer.GetLimitWithResponse(ctx, id)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Limit](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Update patches a limit by ID (PATCH, 200). LimitType and Asset are immutable
// and structurally absent from UpdateLimitInput, so the body never carries them.
func (f *limitsFacade) Update(ctx context.Context, id string, input *models.UpdateLimitInput) (*models.Limit, error) {
	const operation = "Limits.Update"

	if err := requirePathIDs(operation, "id", id); err != nil {
		return nil, err
	}

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Limit](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.tracer.UpdateLimitWithBody(ctx, id, jsonContentType, body, idempotencyEditorsTracer(ctx, f.enableIdempotency)...))
	})
}

// Delete removes a limit by ID. The server returns 204 with no body, so there is
// nothing to decode — only a non-2xx maps into the unified error.
func (f *limitsFacade) Delete(ctx context.Context, id string) error {
	const operation = "Limits.Delete"

	if err := requirePathIDs(operation, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	resp, body, err := readRawResponse(f.tracer.DeleteLimit(ctx, id, idempotencyEditorsTracer(ctx, f.enableIdempotency)...))
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode) {
		return errors.DecodeProblemJSON(resp.StatusCode, body, requestIDOf(resp))
	}

	return nil
}

// Activate transitions a limit to ACTIVE (body-less POST, 200 + limit).
func (f *limitsFacade) Activate(ctx context.Context, id string) (*models.Limit, error) {
	return limitTransition(ctx, "Limits.Activate", f.tracer.ActivateLimit, id)
}

// Deactivate transitions a limit to INACTIVE (body-less POST, 200 + limit).
func (f *limitsFacade) Deactivate(ctx context.Context, id string) (*models.Limit, error) {
	return limitTransition(ctx, "Limits.Deactivate", f.tracer.DeactivateLimit, id)
}

// Draft transitions a limit back to DRAFT (body-less POST, 200 + limit).
func (f *limitsFacade) Draft(ctx context.Context, id string) (*models.Limit, error) {
	return limitTransition(ctx, "Limits.Draft", f.tracer.DraftLimit, id)
}

// GetUsage retrieves the point-in-time usage snapshot for a limit (200). Money
// fields decode into decimal.Decimal; UtilizationPercent is a display float.
func (f *limitsFacade) GetUsage(ctx context.Context, id string) (*models.UsageSnapshot, error) {
	const operation = "Limits.GetUsage"

	if err := requirePathIDs(operation, "id", id); err != nil {
		return nil, err
	}

	resp, err := f.tracer.GetLimitUsageWithResponse(ctx, id)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.UsageSnapshot](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// limitTransition runs a body-less lifecycle POST through the raw call so success
// is decided on 2xx (never the status-exact generated parser) and the 200 body
// decodes into models.Limit. Lifecycle transitions are actions (autoGen=false):
// no auto-gen key, but a caller-supplied ctx/explicit key still rides.
func limitTransition(ctx context.Context, operation string, call func(context.Context, string, ...gentracer.RequestEditorFn) (*http.Response, error), id string) (*models.Limit, error) {
	if err := requirePathIDs(operation, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	resp, body, err := readRawResponse(call(ctx, id, idempotencyEditorsTracer(ctx, false)...))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Limit](operation, resp.StatusCode, body, resp)
}

// List retrieves one cursor page of limits.
//
// The tracer serializes lists as the FLAT domain-keyed envelope
// {limits:[...],hasMore,nextCursor} — NOT the {items,pagination} shape
// models.ListResponse[T].UnmarshalJSON expects (it reads Items only from the
// "items" key). Decoding the raw body straight into ListResponse would yield
// EMPTY Items — total data loss. So the body decodes into a local envelope struct
// (models.Limit tags match the wire) and maps into the public ListResponse.
func (f *limitsFacade) List(ctx context.Context, opts models.LimitsListOpts) (*models.ListResponse[models.Limit], error) {
	const operation = "Limits.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.tracer.ListLimitsWithResponse(ctx, listLimitsParams(opts))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var env struct {
		Limits     []models.Limit `json:"limits"`
		NextCursor string         `json:"nextCursor"`
	}
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &models.ListResponse[models.Limit]{
		Items:      env.Limits,
		Pagination: models.Pagination{NextCursor: env.NextCursor},
	}, nil
}

// ListPages yields one cursor page per iteration, advancing by the response
// nextCursor until it is empty.
func (f *limitsFacade) ListPages(ctx context.Context, opts models.LimitsListOpts) iter.Seq2[*models.ListResponse[models.Limit], error] {
	return func(yield func(*models.ListResponse[models.Limit], error) bool) {
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

// ListAll yields every limit across cursor pages, transparently advancing
// pagination.
func (f *limitsFacade) ListAll(ctx context.Context, opts models.LimitsListOpts) iter.Seq2[models.Limit, error] {
	return flattenPages(f.ListPages(ctx, opts))
}

// listLimitsParams renders the typed opts into the generated ListLimitsParams.
// Every filter has a native *string slot; there is no request editor.
func listLimitsParams(opts models.LimitsListOpts) *gentracer.ListLimitsParams {
	params := &gentracer.ListLimitsParams{}

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

	if opts.Filters.LimitType != "" {
		params.LimitType = strPtr(opts.Filters.LimitType)
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

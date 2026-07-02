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

// operationRoutesFacade is the Phase 2 (Task 2.3.1) hand-written facade over the
// generated genledger.ClientWithResponses. It mirrors the legacy
// operationRoutesEntity (entities/operation_routes.go) wire byte-for-byte: same
// org+ledger-scoped /operation-routes path, same verbs (List/Get GET, Create
// POST, Update PATCH, Delete DELETE), and the same query params.
//
// Operation routes are a CURSOR-paginated list. Unlike the page-based exemplar,
// Pages advances by echoing the response next_cursor back into the request and
// stops on an empty cursor — never HasMore(), whose page-based heuristic can
// loop forever on a full terminal page that carries no cursor.
//
// ListOperationRoutesParams exposes slots for limit/start_date/end_date/
// sort_order/cursor. The name/status/operation_type filters have no slot in the
// ledger OAS, so the facade injects each as a query param via a request editor
// (matching the legacy ToQueryParams wire).
//
// No idempotency is wired here: routes are deferred to the Epic 5.1 retrofit.
// Writes stay replay-safe regardless via the rewindable *bytes.Reader body in
// writeJSON. The public surface stays models.* + *errors.Error; the generated
// types never leak.
type operationRoutesFacade struct {
	ledger *genledger.ClientWithResponses
}

// newOperationRoutesFacade wires the facade over a ledger plane client.
func newOperationRoutesFacade(ledger *genledger.ClientWithResponses) *operationRoutesFacade {
	return &operationRoutesFacade{ledger: ledger}
}

// List retrieves one cursor page of operation routes under an org+ledger.
func (f *operationRoutesFacade) List(ctx context.Context, orgID, ledgerID string, opts models.OperationRoutesListOpts) (*models.ListResponse[models.OperationRoute], error) {
	const operation = "OperationRoutes.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.ledger.ListOperationRoutesWithResponse(ctx, orgID, ledgerID, listOperationRoutesParams(opts), listOperationRoutesReqEditors(opts)...)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var page models.ListResponse[models.OperationRoute]
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &page, nil
}

// Pages yields one cursor page per iteration, advancing by the response
// next_cursor until it is empty.
func (f *operationRoutesFacade) Pages(ctx context.Context, orgID, ledgerID string, opts models.OperationRoutesListOpts) iter.Seq2[*models.ListResponse[models.OperationRoute], error] {
	return func(yield func(*models.ListResponse[models.OperationRoute], error) bool) {
		current := opts

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

// All yields every operation route across cursor pages, transparently advancing
// pagination.
func (f *operationRoutesFacade) All(ctx context.Context, orgID, ledgerID string, opts models.OperationRoutesListOpts) iter.Seq2[models.OperationRoute, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new operation route under an org+ledger via the
// write-facade pattern (marshal input -> rewindable *bytes.Reader -> WithBody
// variant). The server returns 200 on success.
func (f *operationRoutesFacade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
	const operation = "OperationRoutes.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.OperationRoute](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateOperationRouteWithBody(ctx, orgID, ledgerID, jsonContentType, body))
	})
}

// Get retrieves one operation route by ID under an org+ledger.
func (f *operationRoutesFacade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.OperationRoute, error) {
	const operation = "OperationRoutes.Get"

	resp, err := f.ledger.GetOperationRouteByIDWithResponse(ctx, orgID, ledgerID, id)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.OperationRoute](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Update patches an operation route by ID under an org+ledger. Same write-facade
// pattern as Create; the server returns 200 on success.
func (f *operationRoutesFacade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateOperationRouteInput) (*models.OperationRoute, error) {
	const operation = "OperationRoutes.Update"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.OperationRoute](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateOperationRouteWithBody(ctx, orgID, ledgerID, id, jsonContentType, body))
	})
}

// Delete removes an operation route by ID under an org+ledger. The server
// returns 204 with no body on success.
func (f *operationRoutesFacade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "OperationRoutes.Delete"

	resp, err := f.ledger.DeleteOperationRouteWithResponse(ctx, orgID, ledgerID, id)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	return nil
}

// listOperationRoutesParams renders the cursor/sort/date fields that have a slot
// in the generated ListOperationRoutesParams. The name/status/operation_type
// filters have no slot and are carried by listOperationRoutesReqEditors instead.
func listOperationRoutesParams(opts models.OperationRoutesListOpts) *genledger.ListOperationRoutesParams {
	params := &genledger.ListOperationRoutesParams{}

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

	return params
}

// listOperationRoutesReqEditors carries the filters the generated
// ListOperationRoutesParams cannot express. The ledger OAS omits name, status,
// and operation_type from the operation-routes list endpoint, so the SDK injects
// each as a query param rather than dropping it silently. Returns nil when none
// is set so the common path adds zero overhead.
func listOperationRoutesReqEditors(opts models.OperationRoutesListOpts) []genledger.RequestEditorFn {
	var editors []genledger.RequestEditorFn

	if opts.Filters.Name != "" {
		editors = append(editors, setQueryParam("name", opts.Filters.Name))
	}

	if opts.Filters.Status != "" {
		editors = append(editors, setQueryParam("status", opts.Filters.Status))
	}

	if opts.Filters.OperationType != "" {
		editors = append(editors, setQueryParam("operation_type", opts.Filters.OperationType))
	}

	return editors
}

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

// transactionRoutesFacade is the Phase 2 (Task 2.3.2) hand-written facade over
// the generated genledger.ClientWithResponses. It mirrors the legacy
// transactionRoutesEntity (entities/transaction_routes.go) wire byte-for-byte:
// same org+ledger-scoped /transaction-routes path, same verbs (List/Get GET,
// Create POST, Update PATCH, Delete DELETE), and the same query params.
//
// Transaction routes compose operation-routes by ID. The create body carries
// operationRoutes as a FLAT array of UUID strings (models.CreateTransactionRouteInput
// embeds []uuid.UUID, which json.Marshal renders as ["<uuid>", ...]) — never an
// array of objects. This matches the legacy json.Marshal(input) wire.
//
// Like operation-routes, this is a CURSOR-paginated list. Pages advances by
// echoing the response next_cursor back into the request and stops on an empty
// cursor — never HasMore(), whose page-based heuristic can loop forever on a
// full terminal page that carries no cursor.
//
// ListTransactionRoutesParams exposes slots for limit/start_date/end_date/
// sort_order/cursor. The name/status/operation_route_id filters have no slot in
// the ledger OAS, so the facade injects each as a query param via a request
// editor (matching the legacy ToQueryParams wire keys).
//
// No idempotency is wired here: routes are deferred to the Epic 5.1 retrofit.
// Writes stay replay-safe regardless via the rewindable *bytes.Reader body in
// writeJSON. The public surface stays models.* + *errors.Error; the generated
// types never leak.
type transactionRoutesFacade struct {
	ledger *genledger.ClientWithResponses
}

// newTransactionRoutesFacade wires the facade over a ledger plane client.
func newTransactionRoutesFacade(ledger *genledger.ClientWithResponses) *transactionRoutesFacade {
	return &transactionRoutesFacade{ledger: ledger}
}

// List retrieves one cursor page of transaction routes under an org+ledger.
func (f *transactionRoutesFacade) List(ctx context.Context, orgID, ledgerID string, opts models.TransactionRoutesListOpts) (*models.ListResponse[models.TransactionRoute], error) {
	const operation = "TransactionRoutes.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.ledger.ListTransactionRoutesWithResponse(ctx, orgID, ledgerID, listTransactionRoutesParams(opts), listTransactionRoutesReqEditors(opts)...)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var page models.ListResponse[models.TransactionRoute]
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &page, nil
}

// Pages yields one cursor page per iteration, advancing by the response
// next_cursor until it is empty.
func (f *transactionRoutesFacade) Pages(ctx context.Context, orgID, ledgerID string, opts models.TransactionRoutesListOpts) iter.Seq2[*models.ListResponse[models.TransactionRoute], error] {
	return func(yield func(*models.ListResponse[models.TransactionRoute], error) bool) {
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

// All yields every transaction route across cursor pages, transparently
// advancing pagination.
func (f *transactionRoutesFacade) All(ctx context.Context, orgID, ledgerID string, opts models.TransactionRoutesListOpts) iter.Seq2[models.TransactionRoute, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new transaction route under an org+ledger via the
// write-facade pattern (marshal input -> rewindable *bytes.Reader -> WithBody
// variant). The input's operationRoutes serialize as a flat UUID array. The
// server returns 200 on success.
func (f *transactionRoutesFacade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
	const operation = "TransactionRoutes.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.TransactionRoute](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateTransactionRouteWithBody(ctx, orgID, ledgerID, jsonContentType, body))
	})
}

// Get retrieves one transaction route by ID under an org+ledger.
func (f *transactionRoutesFacade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.TransactionRoute, error) {
	const operation = "TransactionRoutes.Get"

	resp, err := f.ledger.GetTransactionRouteByIDWithResponse(ctx, orgID, ledgerID, id)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.TransactionRoute](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Update patches a transaction route by ID under an org+ledger. Same
// write-facade pattern as Create; the server returns 200 on success.
func (f *transactionRoutesFacade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateTransactionRouteInput) (*models.TransactionRoute, error) {
	const operation = "TransactionRoutes.Update"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.TransactionRoute](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateTransactionRouteWithBody(ctx, orgID, ledgerID, id, jsonContentType, body))
	})
}

// Delete removes a transaction route by ID under an org+ledger. The server
// returns 204 with no body on success.
func (f *transactionRoutesFacade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "TransactionRoutes.Delete"

	resp, err := f.ledger.DeleteTransactionRouteWithResponse(ctx, orgID, ledgerID, id)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	return nil
}

// listTransactionRoutesParams renders the cursor/sort/date fields that have a
// slot in the generated ListTransactionRoutesParams. The name/status/
// operation_route_id filters have no slot and are carried by
// listTransactionRoutesReqEditors instead.
func listTransactionRoutesParams(opts models.TransactionRoutesListOpts) *genledger.ListTransactionRoutesParams {
	params := &genledger.ListTransactionRoutesParams{}

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

// listTransactionRoutesReqEditors carries the filters the generated
// ListTransactionRoutesParams cannot express. The ledger OAS omits name,
// status, and operation_route_id from the transaction-routes list endpoint, so
// the SDK injects each as a query param (matching the legacy ToQueryParams wire
// keys) rather than dropping it silently. Returns nil when none is set so the
// common path adds zero overhead.
func listTransactionRoutesReqEditors(opts models.TransactionRoutesListOpts) []genledger.RequestEditorFn {
	var editors []genledger.RequestEditorFn

	if opts.Filters.Name != "" {
		editors = append(editors, setQueryParam("name", opts.Filters.Name))
	}

	if opts.Filters.Status != "" {
		editors = append(editors, setQueryParam("status", opts.Filters.Status))
	}

	if opts.Filters.OperationRouteID != "" {
		editors = append(editors, setQueryParam("operation_route_id", opts.Filters.OperationRouteID))
	}

	return editors
}

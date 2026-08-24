// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"iter"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// operationRoutesV2Facade serves the /v2 operation-route surface — the rule
// that decides which account and balance one leg of a transaction posts against.
//
// The list is CURSOR-paginated, not page-numbered.
type operationRoutesV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newOperationRoutesV2Facade wires the facade over a ledger plane client.
func newOperationRoutesV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *operationRoutesV2Facade {
	return &operationRoutesV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one cursor page of operation routes under an org+ledger.
func (f *operationRoutesV2Facade) List(ctx context.Context, orgID, ledgerID string, opts models.OperationRoutesListOpts) (*models.ListResponse[models.OperationRoute], error) {
	const operation = "V2.OperationRoutes.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListOperationRoutesV2(ctx, orgID, ledgerID, listOperationRoutesV2Params(opts), listOperationRoutesReqEditors(opts)...)

	return readList[models.OperationRoute](operation, resp, err)
}

// Pages yields one cursor page per iteration, advancing by the response
// next_cursor until it is empty.
func (f *operationRoutesV2Facade) Pages(ctx context.Context, orgID, ledgerID string, opts models.OperationRoutesListOpts) iter.Seq2[*models.ListResponse[models.OperationRoute], error] {
	return cursorSeq(ctx, opts,
		func(o *models.OperationRoutesListOpts) *string { return &o.Cursor },
		func(current models.OperationRoutesListOpts) (*models.ListResponse[models.OperationRoute], error) {
			return f.List(ctx, orgID, ledgerID, current)
		})
}

// All yields every operation route across pages.
func (f *operationRoutesV2Facade) All(ctx context.Context, orgID, ledgerID string, opts models.OperationRoutesListOpts) iter.Seq2[models.OperationRoute, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new operation route under an org+ledger.
func (f *operationRoutesV2Facade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
	const operation = "V2.OperationRoutes.Create"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.OperationRoute](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateOperationRouteV2WithBody(ctx, orgID, ledgerID, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one operation route by ID.
func (f *operationRoutesV2Facade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.OperationRoute, error) {
	const operation = "V2.OperationRoutes.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetOperationRouteByIDV2(ctx, orgID, ledgerID, id)

	return readOne[models.OperationRoute](operation, resp, err)
}

// Update patches an operation route by ID.
func (f *operationRoutesV2Facade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateOperationRouteInput) (*models.OperationRoute, error) {
	const operation = "V2.OperationRoutes.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.OperationRoute](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateOperationRouteV2WithBody(ctx, orgID, ledgerID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes an operation route by ID.
func (f *operationRoutesV2Facade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "V2.OperationRoutes.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteOperationRouteV2(ctx, orgID, ledgerID, id, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

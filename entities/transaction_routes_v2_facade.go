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

// transactionRoutesV2Facade serves the /v2 transaction-route surface — the
// named shape a transaction follows, binding the operation routes each leg uses.
//
// The list is CURSOR-paginated, not page-numbered.
type transactionRoutesV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newTransactionRoutesV2Facade wires the facade over a ledger plane client.
func newTransactionRoutesV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *transactionRoutesV2Facade {
	return &transactionRoutesV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one cursor page of transaction routes under an org+ledger.
func (f *transactionRoutesV2Facade) List(ctx context.Context, orgID, ledgerID string, opts models.TransactionRoutesListOpts) (*models.ListResponse[models.TransactionRoute], error) {
	const operation = "V2.TransactionRoutes.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListTransactionRoutesV2(ctx, orgID, ledgerID, listTransactionRoutesV2Params(opts), listTransactionRoutesReqEditors(opts)...)

	return readList[models.TransactionRoute](operation, resp, err)
}

// Pages yields one cursor page per iteration, advancing by the response
// next_cursor until it is empty.
func (f *transactionRoutesV2Facade) Pages(ctx context.Context, orgID, ledgerID string, opts models.TransactionRoutesListOpts) iter.Seq2[*models.ListResponse[models.TransactionRoute], error] {
	return cursorSeq(ctx, opts,
		func(o *models.TransactionRoutesListOpts) *string { return &o.Cursor },
		func(current models.TransactionRoutesListOpts) (*models.ListResponse[models.TransactionRoute], error) {
			return f.List(ctx, orgID, ledgerID, current)
		})
}

// All yields every transaction route across pages.
func (f *transactionRoutesV2Facade) All(ctx context.Context, orgID, ledgerID string, opts models.TransactionRoutesListOpts) iter.Seq2[models.TransactionRoute, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new transaction route under an org+ledger.
func (f *transactionRoutesV2Facade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
	const operation = "V2.TransactionRoutes.Create"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.TransactionRoute](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateTransactionRouteV2WithBody(ctx, orgID, ledgerID, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one transaction route by ID.
func (f *transactionRoutesV2Facade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.TransactionRoute, error) {
	const operation = "V2.TransactionRoutes.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetTransactionRouteByIDV2(ctx, orgID, ledgerID, id)

	return readOne[models.TransactionRoute](operation, resp, err)
}

// Update patches a transaction route by ID.
func (f *transactionRoutesV2Facade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateTransactionRouteInput) (*models.TransactionRoute, error) {
	const operation = "V2.TransactionRoutes.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.TransactionRoute](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateTransactionRouteV2WithBody(ctx, orgID, ledgerID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes a transaction route by ID.
func (f *transactionRoutesV2Facade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "V2.TransactionRoutes.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteTransactionRouteV2(ctx, orgID, ledgerID, id, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

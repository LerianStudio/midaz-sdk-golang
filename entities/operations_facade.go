// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"io"
	"iter"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// operationsFacade serves the operation surface over the generated
// genledger.ClientWithResponses, following the Accounts exemplar.
//
// Operations are the individual debits and credits a transaction is made of, so
// they are read where they are observed and written where they are owned, and
// the two scopes differ:
//
//   - Reads are ACCOUNT-scoped (.../accounts/{id}/operations[/{opId}]). That
//     surface is deprecated server-side but still served, so it is still what
//     the SDK calls.
//   - The update is TRANSACTION-scoped
//     (.../transactions/{txId}/operations/{opId}), because a transaction owns
//     its operations.
//
// The list advances by opaque cursor. Amounts stay decimal end to end: the
// response decodes straight into models.Operation.
type operationsFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newOperationsFacade wires the facade over a ledger plane client.
func newOperationsFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *operationsFacade {
	return &operationsFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// ListOperations retrieves one cursor page of an account's operations.
//
// Advance by reading page.Pagination.NextCursor into opts.Cursor, or let
// ListOperationsPages / ListOperationsAll do it. The filters this endpoint
// honors are operation type, accounting direction, and the operation route by id
// or code.
func (f *operationsFacade) ListOperations(ctx context.Context, organizationID, ledgerID, accountID string, opts models.OperationsListOpts) (*models.ListResponse[models.Operation], error) {
	const operation = "Operations.ListOperations"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.ledger.GetAllOperationsByAccountWithResponse(ctx, organizationID, ledgerID, accountID, operationsByAccountParams(opts))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var page models.ListResponse[models.Operation]
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return nil, errors.NewResponseDecodeError(operation, resp.StatusCode(), err)
	}

	if page.Items == nil {
		page.Items = []models.Operation{}
	}

	return &page, nil
}

// ListOperationsPages yields one cursor page per iteration, advancing by the
// response next_cursor until it is empty.
//
// The stop condition is an EMPTY next_cursor, deliberately not
// Pagination.HasMore(): HasMore()'s page-based branch can report true on a full
// terminal page that carries a page field but no cursor, which would reset the
// cursor to "" and re-request the first page forever.
func (f *operationsFacade) ListOperationsPages(ctx context.Context, organizationID, ledgerID, accountID string, opts models.OperationsListOpts) iter.Seq2[*models.ListResponse[models.Operation], error] {
	return func(yield func(*models.ListResponse[models.Operation], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := f.ListOperations(ctx, organizationID, ledgerID, accountID, current)
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

// ListOperationsAll yields every operation on the account across cursor pages.
func (f *operationsFacade) ListOperationsAll(ctx context.Context, organizationID, ledgerID, accountID string, opts models.OperationsListOpts) iter.Seq2[models.Operation, error] {
	return flattenPages(f.ListOperationsPages(ctx, organizationID, ledgerID, accountID, opts))
}

// GetOperation retrieves one operation as observed from an account.
func (f *operationsFacade) GetOperation(ctx context.Context, organizationID, ledgerID, accountID, operationID string) (*models.Operation, error) {
	const operation = "Operations.GetOperation"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID, "accountID", accountID, "operationID", operationID); err != nil {
		return nil, err
	}

	resp, err := f.ledger.GetOperationByAccountWithResponse(ctx, organizationID, ledgerID, accountID, operationID)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Operation](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// UpdateTransactionOperation patches an operation through the transaction that
// owns it. Only the description and metadata are mutable; the amounts and
// accounts an operation records are immutable by design.
func (f *operationsFacade) UpdateTransactionOperation(ctx context.Context, organizationID, ledgerID, transactionID, operationID string, input *models.UpdateOperationInput) (*models.Operation, error) {
	const operation = "Operations.UpdateTransactionOperation"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID, "transactionID", transactionID, "operationID", operationID); err != nil {
		return nil, err
	}

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Operation](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.UpdateOperationWithBodyWithResponse(ctx, organizationID, ledgerID, transactionID, operationID, "application/json", body, idempotencyEditors(ctx, f.enableIdempotency)...)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

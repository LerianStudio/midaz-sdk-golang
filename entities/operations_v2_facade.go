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

// operationsV2Facade serves the /v2 operation surface — the individual debits
// and credits a transaction is made of.
//
// The two scopes /v1 uses survive unchanged on /v2, and the split is not
// arbitrary: operations are READ where they are observed and WRITTEN where they
// are owned.
//
//   - Reads are ACCOUNT-scoped (.../accounts/{id}/operations[/{opId}]).
//   - The update is TRANSACTION-scoped
//     (.../transactions/{txId}/operations/{opId}), because a transaction owns
//     its operations.
//
// The transaction-scoped update lives HERE and not on [transactionsV2Facade].
// /v1 spells it on both, which means two entry points to keep in sync; /v2 has
// one, on the facade named after the resource it returns.
//
// CONTRACT SURPRISE worth knowing: getOperationByAccountV2 and updateOperationV2
// answer with the SAME schema /v1 does, not with the trimmed OperationV2. That
// type appears only nested inside a v2 transaction. So these three methods
// return models.Operation — chartOfAccounts and route included — while
// TransactionV2.Operations carries models.OperationV2 without them. It reads
// like an inconsistency because it IS one, on the server's side; the SDK
// reports what each endpoint sends rather than papering over it.
type operationsV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newOperationsV2Facade wires the facade over a ledger plane client.
func newOperationsV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *operationsV2Facade {
	return &operationsV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// ListOperations retrieves one cursor page of an account's operations.
//
// The filters this endpoint honors are the operation type, the accounting
// direction, and the operation route by id or code — nothing else. A filter with
// no wire slot is worse than an absent one: the caller reads a full unfiltered
// result set as if it had been narrowed.
func (f *operationsV2Facade) ListOperations(ctx context.Context, orgID, ledgerID, accountID string, opts models.OperationsListOpts) (*models.ListResponse[models.Operation], error) {
	const operation = "V2.Operations.ListOperations"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAllOperationsByAccountV2(ctx, orgID, ledgerID, accountID, operationsByAccountV2Params(opts))

	return readList[models.Operation](operation, resp, err)
}

// ListOperationsPages yields one cursor page per iteration, advancing by the
// response next_cursor until it is empty.
func (f *operationsV2Facade) ListOperationsPages(ctx context.Context, orgID, ledgerID, accountID string, opts models.OperationsListOpts) iter.Seq2[*models.ListResponse[models.Operation], error] {
	return cursorSeq(ctx, opts,
		func(o *models.OperationsListOpts) *string { return &o.Cursor },
		func(current models.OperationsListOpts) (*models.ListResponse[models.Operation], error) {
			return f.ListOperations(ctx, orgID, ledgerID, accountID, current)
		})
}

// ListOperationsAll yields every operation on the account across cursor pages.
func (f *operationsV2Facade) ListOperationsAll(ctx context.Context, orgID, ledgerID, accountID string, opts models.OperationsListOpts) iter.Seq2[models.Operation, error] {
	return flattenPages(f.ListOperationsPages(ctx, orgID, ledgerID, accountID, opts))
}

// GetOperation retrieves one operation as observed from an account.
func (f *operationsV2Facade) GetOperation(ctx context.Context, orgID, ledgerID, accountID, operationID string) (*models.Operation, error) {
	const operation = "V2.Operations.GetOperation"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID,
		"accountID", accountID, "operationID", operationID); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetOperationByAccountV2(ctx, orgID, ledgerID, accountID, operationID)

	return readOne[models.Operation](operation, resp, err)
}

// UpdateTransactionOperation patches an operation through the transaction that
// owns it. Only the description and metadata are mutable; the amount an
// operation recorded and the account it posted against are immutable by design.
func (f *operationsV2Facade) UpdateTransactionOperation(ctx context.Context, orgID, ledgerID, transactionID, operationID string, input *models.UpdateOperationInput) (*models.Operation, error) {
	const operation = "V2.Operations.UpdateTransactionOperation"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID,
		"transactionID", transactionID, "operationID", operationID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Operation](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateOperationV2WithBody(ctx, orgID, ledgerID, transactionID, operationID,
			jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

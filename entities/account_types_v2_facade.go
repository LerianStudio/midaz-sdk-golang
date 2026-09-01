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

// accountTypesV2Facade serves the /v2 account-type surface — the
// classification an account carries and the balance keys it may hold.
//
// There is no count endpoint on either version, so this facade has no Count.
type accountTypesV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newAccountTypesV2Facade wires the facade over a ledger plane client.
func newAccountTypesV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *accountTypesV2Facade {
	return &accountTypesV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of account types under an org+ledger.
func (f *accountTypesV2Facade) List(ctx context.Context, orgID, ledgerID string, opts models.AccountTypesListOpts) (*models.ListResponse[models.AccountType], error) {
	const operation = "V2.AccountTypes.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListAccountTypesV2(ctx, orgID, ledgerID, listAccountTypesV2Params(opts), listAccountTypesReqEditors(opts)...)

	return readList[models.AccountType](operation, resp, err)
}

// Pages yields one full page per iteration, advancing by page number.
func (f *accountTypesV2Facade) Pages(ctx context.Context, orgID, ledgerID string, opts models.AccountTypesListOpts) iter.Seq2[*models.ListResponse[models.AccountType], error] {
	return pageSeq(ctx, opts,
		func(o *models.AccountTypesListOpts) *int { return &o.Page },
		func(current models.AccountTypesListOpts) (*models.ListResponse[models.AccountType], error) {
			return f.List(ctx, orgID, ledgerID, current)
		})
}

// All yields every account type across pages.
func (f *accountTypesV2Facade) All(ctx context.Context, orgID, ledgerID string, opts models.AccountTypesListOpts) iter.Seq2[models.AccountType, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new account type under an org+ledger.
func (f *accountTypesV2Facade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
	const operation = "V2.AccountTypes.Create"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.AccountType](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateAccountTypeV2WithBody(ctx, orgID, ledgerID, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one account type by ID.
func (f *accountTypesV2Facade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.AccountType, error) {
	const operation = "V2.AccountTypes.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountTypeByIDV2(ctx, orgID, ledgerID, id)

	return readOne[models.AccountType](operation, resp, err)
}

// Update patches an account type by ID.
func (f *accountTypesV2Facade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateAccountTypeInput) (*models.AccountType, error) {
	const operation = "V2.AccountTypes.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.AccountType](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateAccountTypeV2WithBody(ctx, orgID, ledgerID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes an account type by ID.
func (f *accountTypesV2Facade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "V2.AccountTypes.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteAccountTypeV2(ctx, orgID, ledgerID, id, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

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

// accountsV2Facade serves the /v2 account surface.
//
// It is NARROWER than [accountsFacade] by design, and the difference is a
// deliberate de-duplication rather than a gap. On /v1 the same three
// account-scoped endpoints — a account's balances, its operations, its balances
// at an instant — are reachable through both V1.Accounts and
// V1.Balances/V1.Operations. Both spellings work, both are tested, and both
// exist because they were written at different times.
//
// V2 has ONE spelling per endpoint: those three live on [balancesV2Facade] and
// [operationsV2Facade], where the resource they return is named. A caller
// looking for balances reaches for Balances, and there is no second answer to
// keep in sync.
//
// What V2 adds over the V1 sibling is the external-account lookup, which /v1
// generated but never exposed until this epic.
type accountsV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newAccountsV2Facade wires the facade over a ledger plane client.
func newAccountsV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *accountsV2Facade {
	return &accountsV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of accounts under an org+ledger.
func (f *accountsV2Facade) List(ctx context.Context, orgID, ledgerID string, opts models.AccountsListOpts) (*models.ListResponse[models.Account], error) {
	const operation = "V2.Accounts.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListAccountsV2(ctx, orgID, ledgerID, listAccountsV2Params(opts), listAccountsReqEditors(opts)...)

	return readList[models.Account](operation, resp, err)
}

// Pages yields one full page per iteration, advancing by page number.
func (f *accountsV2Facade) Pages(ctx context.Context, orgID, ledgerID string, opts models.AccountsListOpts) iter.Seq2[*models.ListResponse[models.Account], error] {
	return pageSeq(ctx, opts,
		func(o *models.AccountsListOpts) *int { return &o.Page },
		func(current models.AccountsListOpts) (*models.ListResponse[models.Account], error) {
			return f.List(ctx, orgID, ledgerID, current)
		})
}

// All yields every account across pages.
func (f *accountsV2Facade) All(ctx context.Context, orgID, ledgerID string, opts models.AccountsListOpts) iter.Seq2[models.Account, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new account under an org+ledger.
func (f *accountsV2Facade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountInput) (*models.Account, error) {
	const operation = "V2.Accounts.Create"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Account](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateAccountV2WithBody(ctx, orgID, ledgerID, &genledger.CreateAccountV2Params{},
			jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one account by ID.
func (f *accountsV2Facade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
	const operation = "V2.Accounts.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountByIDV2(ctx, orgID, ledgerID, id)

	return readOne[models.Account](operation, resp, err)
}

// GetByAlias retrieves one account by its alias, a path segment on
// .../accounts/alias/{alias}.
func (f *accountsV2Facade) GetByAlias(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error) {
	const operation = "V2.Accounts.GetByAlias"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "alias", alias); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountByAliasV2(ctx, orgID, ledgerID, alias)

	return readOne[models.Account](operation, resp, err)
}

// GetByExternalCode retrieves the ledger's EXTERNAL account for an asset code —
// the counterparty every deposit is drawn from and every withdrawal is paid
// into. The code is the bare asset code ("USD"); the "@external/USD" alias
// spelling is prohibited on the alias route, so this is the only way in.
func (f *accountsV2Facade) GetByExternalCode(ctx context.Context, orgID, ledgerID, code string) (*models.Account, error) {
	const operation = "V2.Accounts.GetByExternalCode"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "code", code); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountExternalByCodeV2(ctx, orgID, ledgerID, code)

	return readOne[models.Account](operation, resp, err)
}

// Update patches an account by ID.
func (f *accountsV2Facade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateAccountInput) (*models.Account, error) {
	const operation = "V2.Accounts.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Account](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateAccountV2WithBody(ctx, orgID, ledgerID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes an account by ID.
func (f *accountsV2Facade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "V2.Accounts.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteAccountV2(ctx, orgID, ledgerID, id, &genledger.DeleteAccountV2Params{},
		idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// Count returns the total number of accounts under an org+ledger.
func (f *accountsV2Facade) Count(ctx context.Context, orgID, ledgerID string) (int, error) {
	if err := requirePathIDs("V2.Accounts.Count", "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return 0, err
	}

	//nolint:bodyclose // readCount closes resp.Body via defer.
	return readCount(f.ledger.CountAccountsV2(ctx, orgID, ledgerID))
}

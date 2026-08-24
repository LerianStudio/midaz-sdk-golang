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

// balancesV2Facade serves the /v2 balance surface — the complete one, including
// the account-scoped reads that V1 also spells on V1.Accounts.
//
// Balances carry money, so the two properties the V1 sibling documents hold here
// unchanged and are the reason this facade is not a thin passthrough:
//
//   - Amounts never pass through a float. The body decodes straight into
//     models.Balance / models.BalanceHistory, whose Available and OnHold are
//     shopspring decimals, so "1500.00000001" arrives intact.
//   - Every list here advances by opaque CURSOR, never by page number. The
//     server drops "page" on the floor, so an iterator that incremented one
//     would re-request the first page forever.
//
// The alias and external-code reads take no options and have no Pages/All
// sibling: the server accepts no query parameters on them and answers with the
// account's whole balance set in one response. An iterator over a single fixed
// page can only mislead.
type balancesV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newBalancesV2Facade wires the facade over a ledger plane client.
func newBalancesV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *balancesV2Facade {
	return &balancesV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// ListBalances retrieves one cursor page of every balance on a ledger.
func (f *balancesV2Facade) ListBalances(ctx context.Context, orgID, ledgerID string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
	const operation = "V2.Balances.ListBalances"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAllBalancesV2(ctx, orgID, ledgerID, balancesListV2Params(opts))

	return readList[models.Balance](operation, resp, err)
}

// ListBalancesPages yields one cursor page per iteration.
func (f *balancesV2Facade) ListBalancesPages(ctx context.Context, orgID, ledgerID string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	return cursorSeq(ctx, opts,
		func(o *models.BalancesListOpts) *string { return &o.Cursor },
		func(current models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
			return f.ListBalances(ctx, orgID, ledgerID, current)
		})
}

// ListBalancesAll yields every balance on the ledger across cursor pages.
func (f *balancesV2Facade) ListBalancesAll(ctx context.Context, orgID, ledgerID string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error] {
	return flattenPages(f.ListBalancesPages(ctx, orgID, ledgerID, opts))
}

// ListAccountBalances retrieves one cursor page of an account's balances — one
// entry per asset and balance key the account holds.
func (f *balancesV2Facade) ListAccountBalances(ctx context.Context, orgID, ledgerID, accountID string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
	const operation = "V2.Balances.ListAccountBalances"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAllBalancesByAccountIDV2(ctx, orgID, ledgerID, accountID, accountBalancesListV2Params(opts))

	return readList[models.Balance](operation, resp, err)
}

// ListAccountBalancesPages yields one cursor page of the account's balances per
// iteration.
func (f *balancesV2Facade) ListAccountBalancesPages(ctx context.Context, orgID, ledgerID, accountID string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	return cursorSeq(ctx, opts,
		func(o *models.BalancesListOpts) *string { return &o.Cursor },
		func(current models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
			return f.ListAccountBalances(ctx, orgID, ledgerID, accountID, current)
		})
}

// ListAccountBalancesAll yields every balance the account holds across cursor
// pages.
func (f *balancesV2Facade) ListAccountBalancesAll(ctx context.Context, orgID, ledgerID, accountID string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error] {
	return flattenPages(f.ListAccountBalancesPages(ctx, orgID, ledgerID, accountID, opts))
}

// GetBalance retrieves one balance by ID.
func (f *balancesV2Facade) GetBalance(ctx context.Context, orgID, ledgerID, balanceID string) (*models.Balance, error) {
	const operation = "V2.Balances.GetBalance"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "balanceID", balanceID); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetBalanceByIDV2(ctx, orgID, ledgerID, balanceID)

	return readOne[models.Balance](operation, resp, err)
}

// GetBalanceHistory returns a balance as it stood at a point in time.
//
// The date must name an INSTANT, not a day: "2026-01-02 03:04:05",
// "2026-01-02T03:04:05" or RFC3339. It is required, and a date with no time
// component is refused here because the server refuses it too. An omitted date
// used to be dropped from the query, which asked the server for "now" while the
// caller believed they had asked about the past.
func (f *balancesV2Facade) GetBalanceHistory(ctx context.Context, orgID, ledgerID, balanceID, date string) (*models.BalanceHistory, error) {
	const operation = "V2.Balances.GetBalanceHistory"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "balanceID", balanceID); err != nil {
		return nil, err
	}

	if err := validateBalanceHistoryDate(operation, date); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetBalanceAtTimestampV2(ctx, orgID, ledgerID, balanceID,
		&genledger.GetBalanceAtTimestampV2Params{Date: strPtr(date)})

	return readOne[models.BalanceHistory](operation, resp, err)
}

// GetAccountBalancesHistory returns every balance of an account as it stood at a
// point in time. The response is a bare array, not a paginated envelope. Same
// instant contract as GetBalanceHistory.
func (f *balancesV2Facade) GetAccountBalancesHistory(ctx context.Context, orgID, ledgerID, accountID, date string) ([]models.BalanceHistory, error) {
	const operation = "V2.Balances.GetAccountBalancesHistory"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := validateBalanceHistoryDate(operation, date); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readSlice drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountBalancesAtTimestampV2(ctx, orgID, ledgerID, accountID,
		&genledger.GetAccountBalancesAtTimestampV2Params{Date: strPtr(date)})

	return readSlice[models.BalanceHistory](operation, resp, err)
}

// CreateBalance adds a balance to an account, letting one account hold several
// balances under different keys.
func (f *balancesV2Facade) CreateBalance(ctx context.Context, orgID, ledgerID, accountID string, input *models.CreateBalanceInput) (*models.Balance, error) {
	const operation = "V2.Balances.CreateBalance"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Balance](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateAdditionalBalanceV2WithBody(ctx, orgID, ledgerID, accountID,
			jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// UpdateBalance patches a balance's send/receive permissions and settings.
// Metadata is not part of the server's update contract and the input's own
// validation refuses it.
func (f *balancesV2Facade) UpdateBalance(ctx context.Context, orgID, ledgerID, balanceID string, input *models.UpdateBalanceInput) (*models.Balance, error) {
	const operation = "V2.Balances.UpdateBalance"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "balanceID", balanceID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Balance](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateBalanceV2WithBody(ctx, orgID, ledgerID, balanceID, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// DeleteBalance removes a balance.
func (f *balancesV2Facade) DeleteBalance(ctx context.Context, orgID, ledgerID, balanceID string) error {
	const operation = "V2.Balances.DeleteBalance"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "balanceID", balanceID); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteBalanceV2(ctx, orgID, ledgerID, balanceID, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// ListBalancesByAccountAlias returns every balance of the account carrying an
// alias, resolved by the alias as a path segment. One shot, no options — see the
// facade doc.
func (f *balancesV2Facade) ListBalancesByAccountAlias(ctx context.Context, orgID, ledgerID, alias string) (*models.ListResponse[models.Balance], error) {
	const operation = "V2.Balances.ListBalancesByAccountAlias"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "alias", alias); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetBalancesByAliasV2(ctx, orgID, ledgerID, alias)

	return readList[models.Balance](operation, resp, err)
}

// ListBalancesByExternalCode returns the balances of the ledger's external
// account for an asset code. Same one-shot, no-query contract as
// ListBalancesByAccountAlias.
func (f *balancesV2Facade) ListBalancesByExternalCode(ctx context.Context, orgID, ledgerID, code string) (*models.ListResponse[models.Balance], error) {
	const operation = "V2.Balances.ListBalancesByExternalCode"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "code", code); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetBalancesExternalByCodeV2(ctx, orgID, ledgerID, code)

	return readList[models.Balance](operation, resp, err)
}

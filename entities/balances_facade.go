// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	stderrors "errors"
	"io"
	"iter"
	"net/http"
	"strconv"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// balancesFacade serves the balance surface over the generated
// genledger.ClientWithResponses, following the Accounts exemplar.
//
// Balances carry money, so two properties of this facade are load-bearing:
//
//   - Amounts never pass through a float. The response body is decoded straight
//     into models.Balance / models.BalanceHistory, whose Available and OnHold are
//     shopspring decimals, so a value the server sent as "1500.00000001" arrives
//     intact.
//   - Every list here advances by opaque cursor, never by page number. The three
//     lists that paginate echo the response next_cursor into the next request;
//     the two account-lookup lists (by alias, by external code) are not
//     paginated at all and answer in one shot.
//
// The public surface stays models.* + *errors.Error; the generated types never
// leak.
type balancesFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newBalancesFacade wires the facade over a ledger plane client.
func newBalancesFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *balancesFacade {
	return &balancesFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// ListBalances retrieves one cursor page of every balance on a ledger.
//
// Advance by reading page.Pagination.NextCursor into opts.Cursor, or let
// ListBalancesPages / ListBalancesAll do it.
func (f *balancesFacade) ListBalances(ctx context.Context, organizationID, ledgerID string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
	const operation = "Balances.ListBalances"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAllBalances(ctx, organizationID, ledgerID, balancesListParams(opts))

	return readList[models.Balance](operation, resp, err)
}

// ListBalancesPages yields one cursor page per iteration, advancing by the
// response next_cursor until it is empty.
func (f *balancesFacade) ListBalancesPages(ctx context.Context, organizationID, ledgerID string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	return cursorSeq(ctx, opts,
		func(o *models.BalancesListOpts) *string { return &o.Cursor },
		func(current models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
			return f.ListBalances(ctx, organizationID, ledgerID, current)
		})
}

// ListBalancesAll yields every balance on the ledger across cursor pages.
func (f *balancesFacade) ListBalancesAll(ctx context.Context, organizationID, ledgerID string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error] {
	return flattenPages(f.ListBalancesPages(ctx, organizationID, ledgerID, opts))
}

// ListAccountBalances retrieves one cursor page of an account's balances — one
// entry per asset and balance key the account holds.
func (f *balancesFacade) ListAccountBalances(ctx context.Context, organizationID, ledgerID, accountID string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
	const operation = "Balances.ListAccountBalances"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAllBalancesByAccountID(ctx, organizationID, ledgerID, accountID, accountBalancesListParams(opts))

	return readList[models.Balance](operation, resp, err)
}

// ListAccountBalancesPages yields one cursor page of the account's balances per
// iteration.
func (f *balancesFacade) ListAccountBalancesPages(ctx context.Context, organizationID, ledgerID, accountID string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	return cursorSeq(ctx, opts,
		func(o *models.BalancesListOpts) *string { return &o.Cursor },
		func(current models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
			return f.ListAccountBalances(ctx, organizationID, ledgerID, accountID, current)
		})
}

// ListAccountBalancesAll yields every balance the account holds across cursor
// pages.
func (f *balancesFacade) ListAccountBalancesAll(ctx context.Context, organizationID, ledgerID, accountID string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error] {
	return flattenPages(f.ListAccountBalancesPages(ctx, organizationID, ledgerID, accountID, opts))
}

// GetBalance retrieves one balance by ID.
func (f *balancesFacade) GetBalance(ctx context.Context, organizationID, ledgerID, balanceID string) (*models.Balance, error) {
	const operation = "Balances.GetBalance"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID, "balanceID", balanceID); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetBalanceByID(ctx, organizationID, ledgerID, balanceID)

	return readOne[models.Balance](operation, resp, err)
}

// GetBalanceHistory returns a balance as it stood at a point in time.
//
// The date must name an instant, not a day: "2026-01-02 03:04:05",
// "2026-01-02T03:04:05" or RFC3339. A date with no time component is rejected
// here rather than on the wire, because the server rejects it too.
func (f *balancesFacade) GetBalanceHistory(ctx context.Context, organizationID, ledgerID, balanceID, date string) (*models.BalanceHistory, error) {
	const operation = "Balances.GetBalanceHistory"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID, "balanceID", balanceID); err != nil {
		return nil, err
	}

	if err := validateBalanceHistoryDate(operation, date); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetBalanceAtTimestamp(ctx, organizationID, ledgerID, balanceID,
		&genledger.GetBalanceAtTimestampParams{Date: strPtr(date)})

	return readOne[models.BalanceHistory](operation, resp, err)
}

// GetAccountBalancesHistory returns every balance of an account as it stood at a
// point in time. The response is a bare array, not a paginated envelope. Same
// date contract as GetBalanceHistory.
func (f *balancesFacade) GetAccountBalancesHistory(ctx context.Context, organizationID, ledgerID, accountID, date string) ([]models.BalanceHistory, error) {
	const operation = "Balances.GetAccountBalancesHistory"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := validateBalanceHistoryDate(operation, date); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readSlice drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountBalancesAtTimestamp(ctx, organizationID, ledgerID, accountID,
		&genledger.GetAccountBalancesAtTimestampParams{Date: strPtr(date)})

	return readSlice[models.BalanceHistory](operation, resp, err)
}

// UpdateBalance patches a balance's send/receive permissions and settings.
//
// Metadata is not part of the server's update contract and the input's own
// validation rejects it.
func (f *balancesFacade) UpdateBalance(ctx context.Context, organizationID, ledgerID, balanceID string, input *models.UpdateBalanceInput) (*models.Balance, error) {
	const operation = "Balances.UpdateBalance"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID, "balanceID", balanceID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Balance](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateBalanceWithBody(ctx, organizationID, ledgerID, balanceID,
			jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// DeleteBalance removes a balance. The server answers 204 with no body.
func (f *balancesFacade) DeleteBalance(ctx context.Context, organizationID, ledgerID, balanceID string) error {
	const operation = "Balances.DeleteBalance"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID, "balanceID", balanceID); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteBalance(ctx, organizationID, ledgerID, balanceID, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// CreateBalance adds a balance to an account, letting one account hold several
// balances under different keys.
func (f *balancesFacade) CreateBalance(ctx context.Context, organizationID, ledgerID, accountID string, input *models.CreateBalanceInput) (*models.Balance, error) {
	const operation = "Balances.CreateBalance"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Balance](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateAdditionalBalanceWithBody(ctx, organizationID, ledgerID, accountID,
			jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// ListBalancesByAccountAlias returns every balance of the account carrying an
// alias, resolved by the alias as a path segment.
//
// There is no opts parameter and no Pages / All sibling, and that is the
// endpoint's contract rather than an omission: the server accepts no query
// parameters here and answers with the account's full balance set in one
// response. A limit or cursor would be dropped on the way out, and an iterator
// over a single fixed page is a loop that can only mislead.
func (f *balancesFacade) ListBalancesByAccountAlias(ctx context.Context, organizationID, ledgerID, alias string) (*models.ListResponse[models.Balance], error) {
	const operation = "Balances.ListBalancesByAccountAlias"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID, "alias", alias); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetBalancesByAlias(ctx, organizationID, ledgerID, alias)

	return readList[models.Balance](operation, resp, err)
}

// ListBalancesByExternalCode returns the balances of the external account for an
// asset code. Same one-shot, no-query contract as ListBalancesByAccountAlias.
func (f *balancesFacade) ListBalancesByExternalCode(ctx context.Context, organizationID, ledgerID, code string) (*models.ListResponse[models.Balance], error) {
	const operation = "Balances.ListBalancesByExternalCode"

	if err := requirePathIDs(operation, "organizationID", organizationID, "ledgerID", ledgerID, "code", code); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetBalancesExternalByCode(ctx, organizationID, ledgerID, code)

	return readList[models.Balance](operation, resp, err)
}

// balancesListParams renders the cursor/sort/date fields the ledger-wide
// balances list honors. The endpoint carries no other filters.
func balancesListParams(opts models.BalancesListOpts) *genledger.GetAllBalancesParams {
	params := &genledger.GetAllBalancesParams{}

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

// accountBalancesListParams is the account-scoped sibling of balancesListParams.
// The account is pinned by the path, so the honored query set is identical.
func accountBalancesListParams(opts models.BalancesListOpts) *genledger.GetAllBalancesByAccountIDParams {
	params := balancesListParams(opts)

	return &genledger.GetAllBalancesByAccountIDParams{
		Limit:     params.Limit,
		Cursor:    params.Cursor,
		SortOrder: params.SortOrder,
		StartDate: params.StartDate,
		EndDate:   params.EndDate,
	}
}

// balanceHistoryDateLayouts are the instant formats the server's date parser
// accepts. A date-only value ("2026-01-02") parses there too but is then
// rejected for lacking a time component, so it is absent here on purpose.
var balanceHistoryDateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
}

// validateBalanceHistoryDate enforces the point-in-time date contract before the
// request leaves the SDK.
func validateBalanceHistoryDate(operation, date string) error {
	if date == "" {
		return errors.NewMissingParameterError(operation, "date")
	}

	for _, layout := range balanceHistoryDateLayouts {
		if _, err := time.Parse(layout, date); err == nil {
			return nil
		}
	}

	return errors.NewValidationError(operation,
		`date must name an instant, for example "2026-01-02 03:04:05" or an RFC3339 timestamp`,
		stderrors.New("got "+date))
}

// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"iter"
	"net/http"
	"strconv"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// accountsFacade is the Phase 2 (Task 2.1.c) hand-written facade over the
// generated genledger.ClientWithResponses. Accounts are money-path-adjacent, so
// this facade follows the Organizations exemplar exactly on CRUD + the
// page-based List/Pages/All trinaldo, and adds three account-specific shapes:
//
//   - GetByAlias: an alias lookup where the alias is a path segment
//     (/accounts/alias/{alias}), decoded like Get.
//   - ListBalances / ListOperations: CURSOR-paginated sub-lists. Unlike the
//     page-based trinaldo, their Pages advance by echoing the response
//     next_cursor into the next request's cursor param — never Page++.
//   - BalancesAtTimestamp: a non-paginated point-in-time snapshot decoded
//     straight into a []models.BalanceHistory slice.
//
// The public surface stays models.* + *errors.Error; the generated types never
// leak.
type accountsFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newAccountsFacade wires the facade over a ledger plane client.
func newAccountsFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *accountsFacade {
	return &accountsFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of accounts under an org+ledger.
func (f *accountsFacade) List(ctx context.Context, orgID, ledgerID string, opts models.AccountsListOpts) (*models.ListResponse[models.Account], error) {
	const operation = "Accounts.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListAccounts(ctx, orgID, ledgerID, listAccountsParams(opts), listAccountsReqEditors(opts)...)

	return readList[models.Account](operation, resp, err)
}

// Pages yields one full page per iteration, advancing page-by-page while the
// response reports more results.
func (f *accountsFacade) Pages(ctx context.Context, orgID, ledgerID string, opts models.AccountsListOpts) iter.Seq2[*models.ListResponse[models.Account], error] {
	return func(yield func(*models.ListResponse[models.Account], error) bool) {
		current := opts
		if current.Page == 0 {
			current.Page = 1
		}

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

			if !page.Pagination.HasMore() {
				return
			}

			current.Page++
		}
	}
}

// All yields every account across pages, transparently advancing pagination.
func (f *accountsFacade) All(ctx context.Context, orgID, ledgerID string, opts models.AccountsListOpts) iter.Seq2[models.Account, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new account under an org+ledger via the write-facade
// pattern. CreateAccount carries a generated params struct (an optional
// Authorization override); the facade leaves it empty and lets the auth round
// tripper stamp the bearer token.
func (f *accountsFacade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountInput) (*models.Account, error) {
	const operation = "Accounts.Create"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Account](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateAccountWithBody(ctx, orgID, ledgerID, &genledger.CreateAccountParams{},
			jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one account by ID under an org+ledger.
func (f *accountsFacade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
	const operation = "Accounts.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountByID(ctx, orgID, ledgerID, id)

	return readOne[models.Account](operation, resp, err)
}

// GetByAlias retrieves one account by its alias. The alias is a path segment
// (/accounts/alias/{alias}), not a query filter, so the generated client takes
// it directly.
func (f *accountsFacade) GetByAlias(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error) {
	const operation = "Accounts.GetByAlias"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "alias", alias); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountByAlias(ctx, orgID, ledgerID, alias)

	return readOne[models.Account](operation, resp, err)
}

// GetByExternalCode retrieves the ledger's EXTERNAL account for an asset code —
// the counterparty every deposit is drawn from and every withdrawal is paid
// into, which is what makes an inflow or an outflow balance.
//
// The code is the bare asset code ("USD"), a path segment on
// .../accounts/external/{code}. The alias spelling of the same account
// ("@external/USD") is deliberately NOT accepted anywhere: Midaz prohibits the
// "@external/" prefix on a client-supplied alias, so this route is the only way
// to reach it.
func (f *accountsFacade) GetByExternalCode(ctx context.Context, orgID, ledgerID, code string) (*models.Account, error) {
	const operation = "Accounts.GetByExternalCode"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "code", code); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountExternalByCode(ctx, orgID, ledgerID, code)

	return readOne[models.Account](operation, resp, err)
}

// Update patches an account by ID under an org+ledger. Same write-facade
// pattern as Create.
func (f *accountsFacade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateAccountInput) (*models.Account, error) {
	const operation = "Accounts.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Account](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateAccountWithBody(ctx, orgID, ledgerID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes an account by ID under an org+ledger. The server returns 204
// with no body on success. DeleteAccount carries an optional Authorization
// override param; the facade leaves it empty.
func (f *accountsFacade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "Accounts.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteAccount(ctx, orgID, ledgerID, id, &genledger.DeleteAccountParams{}, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// ListBalances retrieves one cursor page of an account's balances. This
// sub-list is cursor-paginated: the opts carry a Cursor (never a Page), and
// Pages advances by echoing the response next_cursor back into the request.
func (f *accountsFacade) ListBalances(ctx context.Context, orgID, ledgerID, accountID string, opts models.CursorListOpts) (*models.ListResponse[models.Balance], error) {
	const operation = "Accounts.ListBalances"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := models.ValidateCursorListOpts(operation, opts); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAllBalancesByAccountID(ctx, orgID, ledgerID, accountID, balancesByAccountParams(opts))

	return readList[models.Balance](operation, resp, err)
}

// ListBalancesPages yields one cursor page per iteration, advancing by the
// response next_cursor until it is empty.
func (f *accountsFacade) ListBalancesPages(ctx context.Context, orgID, ledgerID, accountID string, opts models.CursorListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	return func(yield func(*models.ListResponse[models.Balance], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := f.ListBalances(ctx, orgID, ledgerID, accountID, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			// Cursor-pure stop: this endpoint paginates by next_cursor, so the
			// only sound terminal signal is an empty cursor. HasMore()'s
			// page-based heuristic (branch 4) can return true on a full
			// terminal page that carries a page field but no cursor, which
			// would set current.Cursor = "" and refetch page 1 forever.
			if page.Pagination.NextCursor == "" {
				return
			}

			current.Cursor = page.Pagination.NextCursor
		}
	}
}

// ListBalancesAll yields every balance across cursor pages.
func (f *accountsFacade) ListBalancesAll(ctx context.Context, orgID, ledgerID, accountID string, opts models.CursorListOpts) iter.Seq2[models.Balance, error] {
	return flattenPages(f.ListBalancesPages(ctx, orgID, ledgerID, accountID, opts))
}

// ListOperations retrieves one cursor page of an account's operations. Like
// ListBalances it is cursor-paginated; it additionally carries the
// account-operations filter set (type/direction/route).
func (f *accountsFacade) ListOperations(ctx context.Context, orgID, ledgerID, accountID string, opts models.AccountOperationsListOpts) (*models.ListResponse[models.Operation], error) {
	const operation = "Accounts.ListOperations"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAllOperationsByAccount(ctx, orgID, ledgerID, accountID, operationsByAccountParams(opts))

	return readList[models.Operation](operation, resp, err)
}

// ListOperationsPages yields one cursor page per iteration, advancing by the
// response next_cursor until it is empty.
func (f *accountsFacade) ListOperationsPages(ctx context.Context, orgID, ledgerID, accountID string, opts models.AccountOperationsListOpts) iter.Seq2[*models.ListResponse[models.Operation], error] {
	return func(yield func(*models.ListResponse[models.Operation], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := f.ListOperations(ctx, orgID, ledgerID, accountID, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			// Cursor-pure stop: see ListBalancesPages. HasMore()'s page-based
			// heuristic would loop forever on a full terminal page with no
			// next_cursor.
			if page.Pagination.NextCursor == "" {
				return
			}

			current.Cursor = page.Pagination.NextCursor
		}
	}
}

// ListOperationsAll yields every operation across cursor pages.
func (f *accountsFacade) ListOperationsAll(ctx context.Context, orgID, ledgerID, accountID string, opts models.AccountOperationsListOpts) iter.Seq2[models.Operation, error] {
	return flattenPages(f.ListOperationsPages(ctx, orgID, ledgerID, accountID, opts))
}

// BalancesAtTimestamp returns an account's balances as of a point in time. The
// endpoint is non-paginated: the response is a bare array, decoded straight into
// a []models.BalanceHistory slice.
//
// The timestamp must name an instant ("2026-01-02 03:04:05", "2026-01-02T03:04:05"
// or RFC3339), and it is required. This is the SAME wire call as
// [balancesFacade.GetAccountBalancesHistory], so it enforces the SAME date
// contract through [validateBalanceHistoryDate] — one endpoint cannot have two
// contracts depending on which spelling the caller reached for. An empty
// timestamp used to be omitted from the query silently, which asked the server
// for "now" while the caller believed they had asked for a moment in the past.
func (f *accountsFacade) BalancesAtTimestamp(ctx context.Context, orgID, ledgerID, accountID, timestamp string) ([]models.BalanceHistory, error) {
	const operation = "Accounts.BalancesAtTimestamp"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := validateBalanceHistoryDate(operation, timestamp); err != nil {
		return nil, err
	}

	params := &genledger.GetAccountBalancesAtTimestampParams{Date: strPtr(timestamp)}

	//nolint:bodyclose // readSlice drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountBalancesAtTimestamp(ctx, orgID, ledgerID, accountID, params)

	return readSlice[models.BalanceHistory](operation, resp, err)
}

// Count returns the total number of accounts under an org+ledger via
// HEAD .../accounts/metrics/count, reading the X-Total-Count header. It routes
// through the raw CountAccounts + readCount so a headers-only error reply (empty
// body) maps to the real status rather than an internal error.
func (f *accountsFacade) Count(ctx context.Context, orgID, ledgerID string) (int, error) {
	if err := requirePathIDs("Accounts.Count", "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return 0, err
	}

	//nolint:bodyclose // readCount (transactions_facade.go) closes resp.Body via defer.
	return readCount(f.ledger.CountAccounts(ctx, orgID, ledgerID))
}

// listAccountsParams renders the pagination/sort/date fields plus every filter
// that has a slot in the generated ListAccountsParams. holder_id and
// include_deleted have no slot and are carried by listAccountsReqEditors.
//
//nolint:gocyclo,cyclop // A flat sequence of independent, optional filter guards (one per query param); complexity is the field count, not branching logic. Any "simplification" (loop/table) would obscure the exact query-param wiring this money-path list depends on.
func listAccountsParams(opts models.AccountsListOpts) *genledger.ListAccountsParams {
	params := &genledger.ListAccountsParams{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.Page > 0 {
		params.Page = strPtr(strconv.Itoa(opts.Page))
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

	if opts.Filters.Type != "" {
		params.Type = strPtr(opts.Filters.Type)
	}

	if opts.Filters.Status != "" {
		params.Status = strPtr(opts.Filters.Status)
	}

	if opts.Filters.AssetCode != "" {
		params.AssetCode = strPtr(opts.Filters.AssetCode)
	}

	if opts.Filters.PortfolioID != "" {
		params.PortfolioId = strPtr(opts.Filters.PortfolioID)
	}

	if opts.Filters.SegmentID != "" {
		params.SegmentId = strPtr(opts.Filters.SegmentID)
	}

	if opts.Filters.Alias != "" {
		params.Alias = strPtr(opts.Filters.Alias)
	}

	if opts.Filters.ParentAccountID != "" {
		params.ParentAccountId = strPtr(opts.Filters.ParentAccountID)
	}

	if opts.Filters.Name != "" {
		params.Name = strPtr(opts.Filters.Name)
	}

	if opts.Filters.EntityID != "" {
		params.EntityId = strPtr(opts.Filters.EntityID)
	}

	if opts.Filters.Blocked {
		params.Blocked = strPtr("true")
	}

	return params
}

// listAccountsReqEditors carries the account filters the generated
// ListAccountsParams cannot express: the ledger OAS omits holder_id and
// include_deleted from the accounts list endpoint, so the SDK injects each as a
// query param rather than dropping it silently. Returns nil when neither is set
// so the common path adds zero overhead.
func listAccountsReqEditors(opts models.AccountsListOpts) []genledger.RequestEditorFn {
	var editors []genledger.RequestEditorFn

	if opts.Filters.HolderID != "" {
		editors = append(editors, setQueryParam("holder_id", opts.Filters.HolderID))
	}

	if opts.Filters.IncludeDeleted {
		editors = append(editors, setQueryParam("include_deleted", "true"))
	}

	return editors
}

// balancesByAccountParams renders the cursor/sort/date fields the balances
// sub-list honors. The account is pinned by the path, so this endpoint carries
// no additional filters.
func balancesByAccountParams(opts models.CursorListOpts) *genledger.GetAllBalancesByAccountIDParams {
	params := &genledger.GetAllBalancesByAccountIDParams{}

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

// operationsByAccountParams renders the cursor/sort/date fields plus the
// endpoint-specific filters (type/direction/route) the operations sub-list
// honors. Every filter has a native slot here, so no request editor is needed.
func operationsByAccountParams(opts models.AccountOperationsListOpts) *genledger.GetAllOperationsByAccountParams {
	params := &genledger.GetAllOperationsByAccountParams{}

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

	if opts.Filters.Type != "" {
		params.Type = strPtr(opts.Filters.Type)
	}

	if opts.Filters.Direction != "" {
		params.Direction = strPtr(opts.Filters.Direction)
	}

	if opts.Filters.RouteID != "" {
		params.RouteId = strPtr(opts.Filters.RouteID)
	}

	if opts.Filters.RouteCode != "" {
		params.RouteCode = strPtr(opts.Filters.RouteCode)
	}

	return params
}

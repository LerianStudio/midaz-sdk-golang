// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"iter"
	"net/http"
	"strconv"

	"github.com/LerianStudio/midaz-sdk-golang/v6/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
)

// accountPlansFacade is the hand-written facade over the generated
// genledger.ClientWithResponses for account plans: per-account fee plans,
// their versions, their assignment to accounts and the free-quota usage of an
// account. The public surface is models.* plus *errors.Error; the generated
// types never leak.
//
// # Server versions
//
// Midaz serves these routes on /v2 only, from 4.4.0. Against an older server
// the SDK passes the 404 on. Rounding and the received fee direction require
// 4.6.0; see models.AccountPlanTriggerDirectionReceived for the rollout note.
// The facade does not check the server version before calling.
//
// # No X-Idempotency, no automatic retry
//
// The server ignores X-Idempotency on these routes: the same key with the same
// body answers 409/0543 on a create rather than replaying the first answer. The
// writes (Create, Update, Delete, Assign, Unassign) therefore never send the
// header — not by default, not with WithIdempotency(true) and not with a key in
// the context — and, being keyless, the SDK sends each of them exactly once:
// a timeout or a 5xx is not retried. Retrying would be unsafe here, because an
// update that sets fees or rounding creates a new plan version each time it
// lands. Assign is idempotent on the server (assigning the active plan again
// changes nothing), so a caller may safely repeat it after an unknown outcome.
//
// # Pagination
//
// The three listings are PAGE-paginated. Their total is the number of items on
// the page, not across pages, so Pagination.HasMore() is not reliable here; the
// iterators start at page 1 and stop on the first page shorter than the limit
// the server echoes, like feePackagesFacade.Pages.
type accountPlansFacade struct {
	ledger *genledger.ClientWithResponses
}

// newAccountPlansFacade wires the facade over a ledger plane client. It takes
// no idempotency gate: see the type doc.
func newAccountPlansFacade(ledger *genledger.ClientWithResponses) *accountPlansFacade {
	return &accountPlansFacade{ledger: ledger}
}

// Create creates an account plan in a ledger and returns it with the version
// in effect. The body carries only the fields the input sets, always with
// enable. It is sent once, without X-Idempotency: see the type doc.
func (f *accountPlansFacade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountPlanInput) (*models.AccountPlanView, error) {
	const operation = "AccountPlans.Create"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.AccountPlanView](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateAccountPlanV2WithBody(ctx, orgID, ledgerID, jsonContentType, body))
	})
}

// List retrieves one page of the account plans of a ledger.
//
// The response total is the number of items on this page, not across pages,
// so Pagination.HasMore() is not reliable on this listing: iterate with Pages
// or All, which stop on the first short page.
func (f *accountPlansFacade) List(ctx context.Context, orgID, ledgerID string, opts models.AccountPlansListOpts) (*models.ListResponse[models.AccountPlan], error) {
	const operation = "AccountPlans.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAllAccountPlansV2(ctx, orgID, ledgerID, accountPlansListParams(opts))

	return readList[models.AccountPlan](operation, resp, err)
}

// Pages yields one page of account plans per iteration, starting at page 1
// (or opts.Page) and stopping on the first page shorter than the limit.
func (f *accountPlansFacade) Pages(ctx context.Context, orgID, ledgerID string, opts models.AccountPlansListOpts) iter.Seq2[*models.ListResponse[models.AccountPlan], error] {
	return shortPageSeq(ctx, opts,
		func(o *models.AccountPlansListOpts) *int { return &o.Page },
		func(o models.AccountPlansListOpts) (*models.ListResponse[models.AccountPlan], error) {
			return f.List(ctx, orgID, ledgerID, o)
		})
}

// All yields every account plan of a ledger across pages.
func (f *accountPlansFacade) All(ctx context.Context, orgID, ledgerID string, opts models.AccountPlansListOpts) iter.Seq2[models.AccountPlan, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Get reads an account plan with the version in effect at the time of the
// read. A version without rounding comes back with a nil Rounding.
func (f *accountPlansFacade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.AccountPlanView, error) {
	const operation = "AccountPlans.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountPlanByIDV2(ctx, orgID, ledgerID, id)

	return readOne[models.AccountPlanView](operation, resp, err)
}

// Update patches an account plan and returns it with the version in effect.
// Only the fields the input sets are sent; setting fees or rounding creates a
// new version. An empty input is refused locally. It is sent once, without
// X-Idempotency: see the type doc.
func (f *accountPlansFacade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateAccountPlanInput) (*models.AccountPlanView, error) {
	const operation = "AccountPlans.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.AccountPlanView](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateAccountPlanV2WithBody(ctx, orgID, ledgerID, id, jsonContentType, body))
	})
}

// Delete deletes an account plan. The server refuses a plan still assigned to
// an account (409/0546). It is sent once, without X-Idempotency.
func (f *accountPlansFacade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "AccountPlans.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteAccountPlanV2(ctx, orgID, ledgerID, id)

	return deleteResource(operation, resp, err)
}

// ListVersions retrieves one page of the versions of an account plan. As on
// List, the response total is the number of items on the page.
func (f *accountPlansFacade) ListVersions(ctx context.Context, orgID, ledgerID, id string, opts models.AccountPlanVersionsListOpts) (*models.ListResponse[models.AccountPlanVersion], error) {
	const operation = "AccountPlans.ListVersions"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountPlanVersionsV2(ctx, orgID, ledgerID, id, accountPlanVersionsParams(opts))

	return readList[models.AccountPlanVersion](operation, resp, err)
}

// VersionPages yields one page of plan versions per iteration, stopping on the
// first page shorter than the limit.
func (f *accountPlansFacade) VersionPages(ctx context.Context, orgID, ledgerID, id string, opts models.AccountPlanVersionsListOpts) iter.Seq2[*models.ListResponse[models.AccountPlanVersion], error] {
	return shortPageSeq(ctx, opts,
		func(o *models.AccountPlanVersionsListOpts) *int { return &o.Page },
		func(o models.AccountPlanVersionsListOpts) (*models.ListResponse[models.AccountPlanVersion], error) {
			return f.ListVersions(ctx, orgID, ledgerID, id, o)
		})
}

// AllVersions yields every version of an account plan across pages.
func (f *accountPlansFacade) AllVersions(ctx context.Context, orgID, ledgerID, id string, opts models.AccountPlanVersionsListOpts) iter.Seq2[models.AccountPlanVersion, error] {
	return flattenPages(f.VersionPages(ctx, orgID, ledgerID, id, opts))
}

// ListAccounts retrieves one page of the account assignments of an account
// plan. The response carries no total.
func (f *accountPlansFacade) ListAccounts(ctx context.Context, orgID, ledgerID, id string, opts models.AccountPlanAccountsListOpts) (*models.ListResponse[models.AccountPlanAssignment], error) {
	return f.listPlanAccounts(ctx, orgID, ledgerID, id, opts)
}

// listPlanAccounts is the body of ListAccounts. It exists because the path-id
// and seam scans match generated operations by name, and genledger has a /v1
// ListAccounts operation: AccountPages calling f.ListAccounts would read to
// them as an unguarded call to that operation.
func (f *accountPlansFacade) listPlanAccounts(ctx context.Context, orgID, ledgerID, id string, opts models.AccountPlanAccountsListOpts) (*models.ListResponse[models.AccountPlanAssignment], error) {
	const operation = "AccountPlans.ListAccounts"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "id", id); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListAccountPlanAccountsV2(ctx, orgID, ledgerID, id, accountPlanAccountsParams(opts))

	return readList[models.AccountPlanAssignment](operation, resp, err)
}

// AccountPages yields one page of plan assignments per iteration, stopping on
// the first page shorter than the limit.
func (f *accountPlansFacade) AccountPages(ctx context.Context, orgID, ledgerID, id string, opts models.AccountPlanAccountsListOpts) iter.Seq2[*models.ListResponse[models.AccountPlanAssignment], error] {
	return shortPageSeq(ctx, opts,
		func(o *models.AccountPlanAccountsListOpts) *int { return &o.Page },
		func(o models.AccountPlanAccountsListOpts) (*models.ListResponse[models.AccountPlanAssignment], error) {
			return f.listPlanAccounts(ctx, orgID, ledgerID, id, o)
		})
}

// AllAccounts yields every assignment of an account plan across pages.
func (f *accountPlansFacade) AllAccounts(ctx context.Context, orgID, ledgerID, id string, opts models.AccountPlanAccountsListOpts) iter.Seq2[models.AccountPlanAssignment, error] {
	return flattenPages(f.AccountPages(ctx, orgID, ledgerID, id, opts))
}

// Assign assigns an account plan to an account, replacing its active plan, and
// returns the resulting assignment. It is sent once, without X-Idempotency;
// assigning the active plan again changes nothing on the server, so a caller
// may repeat it after an unknown outcome.
func (f *accountPlansFacade) Assign(ctx context.Context, orgID, ledgerID, accountID string, input *models.AssignAccountPlanInput) (*models.AccountPlanAssignment, error) {
	const operation = "AccountPlans.Assign"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.AccountPlanAssignment](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.AssignAccountPlanV2WithBody(ctx, orgID, ledgerID, accountID, jsonContentType, body))
	})
}

// GetAssignment reads the account plan assignment state of an account: the
// active assignment, nil when the account has none, and its history.
func (f *accountPlansFacade) GetAssignment(ctx context.Context, orgID, ledgerID, accountID string) (*models.AccountPlanAssignments, error) {
	const operation = "AccountPlans.GetAssignment"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountPlanAssignmentV2(ctx, orgID, ledgerID, accountID)

	return readOne[models.AccountPlanAssignments](operation, resp, err)
}

// Unassign ends the active account plan assignment of an account. The server
// answers 404/0554 when the account has no active plan. It is sent once,
// without X-Idempotency.
func (f *accountPlansFacade) Unassign(ctx context.Context, orgID, ledgerID, accountID string) error {
	const operation = "AccountPlans.Unassign"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.UnassignAccountPlanV2(ctx, orgID, ledgerID, accountID)

	return deleteResource(operation, resp, err)
}

// GetUsage reads the free-quota usage of an account under its active plan for
// one period: the current one when opts.Period is empty. The server answers
// 404/0554 when the account has no active plan.
func (f *accountPlansFacade) GetUsage(ctx context.Context, orgID, ledgerID, accountID string, opts models.AccountPlanUsageOpts) (*models.AccountPlanUsage, error) {
	const operation = "AccountPlans.GetUsage"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "accountID", accountID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, opts.Validate()); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAccountPlanUsageV2(ctx, orgID, ledgerID, accountID, accountPlanUsageParams(opts))

	return readOne[models.AccountPlanUsage](operation, resp, err)
}

// shortPageSeq drives a PAGE-numbered list whose total cannot end the
// iteration: it starts at page 1 (or the page the caller set), advances the
// page number, and stops on the first page shorter than the limit the server
// echoes, or when the server echoes no limit. An error is yielded once and
// ends the iteration, and a consumer that stops early causes no further
// request.
func shortPageSeq[O, T any](
	ctx context.Context,
	opts O,
	page func(*O) *int,
	fetch func(O) (*models.ListResponse[T], error),
) iter.Seq2[*models.ListResponse[T], error] {
	return func(yield func(*models.ListResponse[T], error) bool) {
		current := opts
		if *page(&current) == 0 {
			*page(&current) = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			result, err := fetch(current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(result, nil) {
				return
			}

			if result.Pagination.Limit == 0 || len(result.Items) < result.Pagination.Limit {
				return
			}

			*page(&current)++
		}
	}
}

// accountPlansListParams renders the plan listing options into the generated
// params; a field is set only when the option is.
func accountPlansListParams(opts models.AccountPlansListOpts) *genledger.GetAllAccountPlansV2Params {
	params := &genledger.GetAllAccountPlansV2Params{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.Page > 0 {
		params.Page = strPtr(strconv.Itoa(opts.Page))
	}

	if opts.Filters.Enable != nil {
		params.Enable = strPtr(strconv.FormatBool(*opts.Filters.Enable))
	}

	return params
}

// accountPlanVersionsParams renders the version listing options into the
// generated params.
func accountPlanVersionsParams(opts models.AccountPlanVersionsListOpts) *genledger.GetAccountPlanVersionsV2Params {
	params := &genledger.GetAccountPlanVersionsV2Params{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.Page > 0 {
		params.Page = strPtr(strconv.Itoa(opts.Page))
	}

	return params
}

// accountPlanAccountsParams renders the plan-accounts listing options into the
// generated params; SortDirection is sent as sort_order.
func accountPlanAccountsParams(opts models.AccountPlanAccountsListOpts) *genledger.ListAccountPlanAccountsV2Params {
	params := &genledger.ListAccountPlanAccountsV2Params{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.Page > 0 {
		params.Page = strPtr(strconv.Itoa(opts.Page))
	}

	if opts.SortDirection != "" {
		params.SortOrder = strPtr(string(opts.SortDirection))
	}

	return params
}

// accountPlanUsageParams renders the usage options into the generated params.
func accountPlanUsageParams(opts models.AccountPlanUsageOpts) *genledger.GetAccountPlanUsageV2Params {
	params := &genledger.GetAccountPlanUsageV2Params{}

	if opts.Period != "" {
		params.Period = strPtr(opts.Period)
	}

	return params
}

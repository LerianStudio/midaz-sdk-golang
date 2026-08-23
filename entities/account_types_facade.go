// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"io"
	"iter"
	"net/http"
	"strconv"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// accountTypesFacade is the Phase 2 (Task 2.1.b) hand-written facade over the
// generated genledger.ClientWithResponses, following the Organizations exemplar.
// The public surface is exactly models.AccountType + *errors.Error + the
// List/Pages/All trinaldo + full CRUD.
//
// Account types are organization+ledger scoped, so every method threads orgID
// and ledgerID through to the generated client.
//
// Two wire notes specific to this resource:
//   - The response carries uuid.UUID IDs (models.AccountType.ID et al.); they
//     decode straight from the JSON body via json.Unmarshal, so nothing special
//     is needed on the read path.
//   - ListAccountTypesParams exposes both Page and Cursor. This facade is
//     page-based (like the exemplar): it sets Page and never touches Cursor.
type accountTypesFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newAccountTypesFacade wires the facade over a ledger plane client.
func newAccountTypesFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *accountTypesFacade {
	return &accountTypesFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of account types under an org+ledger, normalized into
// the public model.
func (f *accountTypesFacade) List(ctx context.Context, orgID, ledgerID string, opts models.AccountTypesListOpts) (*models.ListResponse[models.AccountType], error) {
	const operation = "AccountTypes.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.ledger.ListAccountTypesWithResponse(ctx, orgID, ledgerID, listAccountTypesParams(opts), listAccountTypesReqEditors(opts)...)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var page models.ListResponse[models.AccountType]
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &page, nil
}

// Pages yields one full page per iteration, advancing while the response reports
// more results.
func (f *accountTypesFacade) Pages(ctx context.Context, orgID, ledgerID string, opts models.AccountTypesListOpts) iter.Seq2[*models.ListResponse[models.AccountType], error] {
	return func(yield func(*models.ListResponse[models.AccountType], error) bool) {
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

// All yields every account type across pages, transparently advancing
// pagination.
func (f *accountTypesFacade) All(ctx context.Context, orgID, ledgerID string, opts models.AccountTypesListOpts) iter.Seq2[models.AccountType, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Create registers a new account type under an org+ledger via the write-facade
// pattern (marshal input -> rewindable *bytes.Reader -> WithBody variant).
func (f *accountTypesFacade) Create(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
	const operation = "AccountTypes.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.AccountType](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.CreateAccountTypeWithBodyWithResponse(ctx, orgID, ledgerID, "application/json", body, idempotencyEditors(ctx, f.enableIdempotency)...)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// Get retrieves one account type by ID under an org+ledger.
func (f *accountTypesFacade) Get(ctx context.Context, orgID, ledgerID, id string) (*models.AccountType, error) {
	const operation = "AccountTypes.Get"

	resp, err := f.ledger.GetAccountTypeByIDWithResponse(ctx, orgID, ledgerID, id)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.AccountType](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Update patches an account type by ID under an org+ledger. Same write-facade
// pattern as Create.
func (f *accountTypesFacade) Update(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateAccountTypeInput) (*models.AccountType, error) {
	const operation = "AccountTypes.Update"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.AccountType](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.UpdateAccountTypeWithBodyWithResponse(ctx, orgID, ledgerID, id, "application/json", body, idempotencyEditors(ctx, f.enableIdempotency)...)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// Delete removes an account type by ID under an org+ledger. The server returns
// 204 with no body on success.
func (f *accountTypesFacade) Delete(ctx context.Context, orgID, ledgerID, id string) error {
	const operation = "AccountTypes.Delete"

	resp, err := f.ledger.DeleteAccountTypeWithResponse(ctx, orgID, ledgerID, id, idempotencyEditors(ctx, f.enableIdempotency)...)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	return nil
}

// listAccountTypesParams renders the pagination/sort/date fields plus the one
// filter (key_value) that has a slot in the generated ListAccountTypesParams.
// Page-based: Page is set, Cursor is left nil. The name and include_deleted
// filters have no slot and are carried by listAccountTypesReqEditors instead.
func listAccountTypesParams(opts models.AccountTypesListOpts) *genledger.ListAccountTypesParams {
	params := &genledger.ListAccountTypesParams{}

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

	if opts.Filters.KeyValue != "" {
		params.KeyValue = strPtr(opts.Filters.KeyValue)
	}

	return params
}

// listAccountTypesReqEditors carries the filters the generated
// ListAccountTypesParams cannot express. The ledger OAS omits name and
// include_deleted from the account-types list endpoint, so the SDK injects each
// as a query param rather than dropping it silently. Returns nil when neither
// is set so the common path adds zero overhead.
func listAccountTypesReqEditors(opts models.AccountTypesListOpts) []genledger.RequestEditorFn {
	var editors []genledger.RequestEditorFn

	if opts.Filters.Name != "" {
		editors = append(editors, setQueryParam("name", opts.Filters.Name))
	}

	if opts.Filters.IncludeDeleted {
		editors = append(editors, setQueryParam("include_deleted", "true"))
	}

	return editors
}

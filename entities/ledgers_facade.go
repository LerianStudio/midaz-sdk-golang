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

// ledgersFacade is the Phase 2 (Task 2.1.a) hand-written facade over the
// generated genledger.ClientWithResponses. It follows the Organizations
// exemplar (organizations_facade.go): the public surface is exactly
// models.Ledger + *errors.Error + the List/Pages/All trinaldo + full CRUD, and
// the generated types never enter the public path.
//
// Ledgers are organization-scoped, so every method threads orgID through to
// the generated client.
type ledgersFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newLedgersFacade wires the facade over a ledger plane client.
func newLedgersFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *ledgersFacade {
	return &ledgersFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of ledgers under an organization, normalized into the
// public model. Decodes the raw body into models.ListResponse[models.Ledger] so
// the generated Pagination (untyped Items) never leaks.
func (f *ledgersFacade) List(ctx context.Context, orgID string, opts models.LedgersListOpts) (*models.ListResponse[models.Ledger], error) {
	const operation = "Ledgers.List"

	if err := requirePathIDs(operation, "orgID", orgID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListLedgers(ctx, orgID, listLedgersParams(opts), listLedgersReqEditors(opts)...)

	return readList[models.Ledger](operation, resp, err)
}

// Pages yields one full page per iteration, advancing while the response reports
// more results.
func (f *ledgersFacade) Pages(ctx context.Context, orgID string, opts models.LedgersListOpts) iter.Seq2[*models.ListResponse[models.Ledger], error] {
	return func(yield func(*models.ListResponse[models.Ledger], error) bool) {
		current := opts
		if current.Page == 0 {
			current.Page = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := f.List(ctx, orgID, current)
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

// All yields every ledger across pages, transparently advancing pagination.
func (f *ledgersFacade) All(ctx context.Context, orgID string, opts models.LedgersListOpts) iter.Seq2[models.Ledger, error] {
	return flattenPages(f.Pages(ctx, orgID, opts))
}

// Create registers a new ledger under an organization via the write-facade
// pattern (see writeJSON / organizations_facade.go).
func (f *ledgersFacade) Create(ctx context.Context, orgID string, input *models.CreateLedgerInput) (*models.Ledger, error) {
	const operation = "Ledgers.Create"

	if err := requirePathIDs(operation, "orgID", orgID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Ledger](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateLedgerWithBody(ctx, orgID, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one ledger by ID under an organization, normalized into the
// public model.
func (f *ledgersFacade) Get(ctx context.Context, orgID, id string) (*models.Ledger, error) {
	const operation = "Ledgers.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetLedgerByID(ctx, orgID, id)

	return readOne[models.Ledger](operation, resp, err)
}

// Update patches a ledger by ID under an organization. Same write-facade
// pattern as Create.
func (f *ledgersFacade) Update(ctx context.Context, orgID, id string, input *models.UpdateLedgerInput) (*models.Ledger, error) {
	const operation = "Ledgers.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Ledger](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateLedgerWithBody(ctx, orgID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes a ledger by ID under an organization. The server returns 204
// with no body on success.
func (f *ledgersFacade) Delete(ctx context.Context, orgID, id string) error {
	const operation = "Ledgers.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteLedger(ctx, orgID, id, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// GetSettings retrieves the tri-block settings for a ledger (accounting,
// overrides, tracer), normalized into the public model.
func (f *ledgersFacade) GetSettings(ctx context.Context, orgID, id string) (*models.LedgerSettings, error) {
	const operation = "Ledgers.GetSettings"

	if err := requirePathIDs(operation, "orgID", orgID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetLedgerSettings(ctx, orgID, id)

	return readOne[models.LedgerSettings](operation, resp, err)
}

// UpdateSettings patches the tri-block settings for a ledger. Same write-facade
// pattern as Update: the partial patch marshals only the blocks a setter
// touched, sent via a rewindable body so the auth round tripper can replay
// after a 401.
func (f *ledgersFacade) UpdateSettings(ctx context.Context, orgID, id string, input *models.UpdateLedgerSettingsInput) (*models.LedgerSettings, error) {
	const operation = "Ledgers.UpdateSettings"

	if err := requirePathIDs(operation, "orgID", orgID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.LedgerSettings](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateLedgerSettingsWithBody(ctx, orgID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Count returns the total number of ledgers under an org via
// HEAD .../ledgers/metrics/count, reading the X-Total-Count header. It routes
// through the raw CountLedgers + readCount so a headers-only error reply (empty
// body) maps to the real status rather than an internal error.
func (f *ledgersFacade) Count(ctx context.Context, orgID string) (int, error) {
	if err := requirePathIDs("Ledgers.Count", "orgID", orgID); err != nil {
		return 0, err
	}

	//nolint:bodyclose // readCount (transactions_facade.go) closes resp.Body via defer.
	return readCount(f.ledger.CountLedgers(ctx, orgID))
}

// listLedgersParams renders the typed opts into the generated params. Name and
// Status map to generated slots; IncludeDeleted has no slot and is injected via
// a request editor (see listLedgersReqEditors).
func listLedgersParams(opts models.LedgersListOpts) *genledger.ListLedgersParams {
	params := &genledger.ListLedgersParams{}

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

	if opts.Filters.Name != "" {
		params.Name = strPtr(opts.Filters.Name)
	}

	if opts.Filters.Status != "" {
		params.Status = strPtr(opts.Filters.Status)
	}

	return params
}

// listLedgersReqEditors carries query params the generated ListLedgersParams
// cannot express. The OAS spec omits include_deleted, so the SDK injects it via
// an editor rather than dropping the filter silently. Returns nil when no editor
// is needed.
func listLedgersReqEditors(opts models.LedgersListOpts) []genledger.RequestEditorFn {
	if !opts.Filters.IncludeDeleted {
		return nil
	}

	return []genledger.RequestEditorFn{setQueryParam("include_deleted", "true")}
}

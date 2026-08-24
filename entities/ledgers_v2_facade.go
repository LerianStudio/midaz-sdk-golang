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

// ledgersV2Facade serves the /v2 ledger surface, including the tri-block ledger
// settings (accounting, overrides, tracer). Ledgers are organization-scoped.
type ledgersV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newLedgersV2Facade wires the facade over a ledger plane client.
func newLedgersV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *ledgersV2Facade {
	return &ledgersV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of ledgers under an organization.
func (f *ledgersV2Facade) List(ctx context.Context, orgID string, opts models.LedgersListOpts) (*models.ListResponse[models.Ledger], error) {
	const operation = "V2.Ledgers.List"

	if err := requirePathIDs(operation, "orgID", orgID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListLedgersV2(ctx, orgID, listLedgersV2Params(opts), listLedgersReqEditors(opts)...)

	return readList[models.Ledger](operation, resp, err)
}

// Pages yields one full page per iteration, advancing by page number.
func (f *ledgersV2Facade) Pages(ctx context.Context, orgID string, opts models.LedgersListOpts) iter.Seq2[*models.ListResponse[models.Ledger], error] {
	return pageSeq(ctx, opts,
		func(o *models.LedgersListOpts) *int { return &o.Page },
		func(current models.LedgersListOpts) (*models.ListResponse[models.Ledger], error) {
			return f.List(ctx, orgID, current)
		})
}

// All yields every ledger across pages.
func (f *ledgersV2Facade) All(ctx context.Context, orgID string, opts models.LedgersListOpts) iter.Seq2[models.Ledger, error] {
	return flattenPages(f.Pages(ctx, orgID, opts))
}

// Create registers a new ledger under an organization.
func (f *ledgersV2Facade) Create(ctx context.Context, orgID string, input *models.CreateLedgerInput) (*models.Ledger, error) {
	const operation = "V2.Ledgers.Create"

	if err := requirePathIDs(operation, "orgID", orgID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Ledger](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateLedgerV2WithBody(ctx, orgID, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one ledger by ID.
func (f *ledgersV2Facade) Get(ctx context.Context, orgID, id string) (*models.Ledger, error) {
	const operation = "V2.Ledgers.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetLedgerByIDV2(ctx, orgID, id)

	return readOne[models.Ledger](operation, resp, err)
}

// Update patches a ledger by ID.
func (f *ledgersV2Facade) Update(ctx context.Context, orgID, id string, input *models.UpdateLedgerInput) (*models.Ledger, error) {
	const operation = "V2.Ledgers.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Ledger](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateLedgerV2WithBody(ctx, orgID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes a ledger by ID.
func (f *ledgersV2Facade) Delete(ctx context.Context, orgID, id string) error {
	const operation = "V2.Ledgers.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteLedgerV2(ctx, orgID, id, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// GetSettings retrieves the tri-block settings for a ledger.
func (f *ledgersV2Facade) GetSettings(ctx context.Context, orgID, id string) (*models.LedgerSettings, error) {
	const operation = "V2.Ledgers.GetSettings"

	if err := requirePathIDs(operation, "orgID", orgID, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetLedgerSettingsV2(ctx, orgID, id)

	return readOne[models.LedgerSettings](operation, resp, err)
}

// UpdateSettings patches the tri-block settings for a ledger. The partial patch
// marshals only the blocks a setter touched.
func (f *ledgersV2Facade) UpdateSettings(ctx context.Context, orgID, id string, input *models.UpdateLedgerSettingsInput) (*models.LedgerSettings, error) {
	const operation = "V2.Ledgers.UpdateSettings"

	if err := requirePathIDs(operation, "orgID", orgID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.LedgerSettings](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateLedgerSettingsV2WithBody(ctx, orgID, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Count returns the total number of ledgers under an organization.
func (f *ledgersV2Facade) Count(ctx context.Context, orgID string) (int, error) {
	if err := requirePathIDs("V2.Ledgers.Count", "orgID", orgID); err != nil {
		return 0, err
	}

	//nolint:bodyclose // readCount closes resp.Body via defer.
	return readCount(f.ledger.CountLedgersV2(ctx, orgID))
}

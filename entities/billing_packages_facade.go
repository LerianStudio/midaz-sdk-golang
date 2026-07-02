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

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

// billingPackagesFacade is the Epic 3.2 (Task 3.2.3) hand-written facade over the
// generated genledger.ClientWithResponses for billing-package definitions,
// following the FeePackages exemplar. The public surface is exactly
// models.BillingPackage + *errors.Error + the List/ListPages/ListAll trinaldo +
// full CRUD.
//
// Billing packages are organization-scoped (no ledger path segment) and PAGE-mode
// paginated: unlike the cursor facades (holders/transactions/routes) that stop on
// an empty NextCursor, ListPages advances Page++ and stops on !HasMore(). The
// page-mode envelope emits no cursor, so an empty-cursor stop would drop page 2.
//
// Money is money-adjacent here: FeeAmount, every pricing-tier UnitPrice, and every
// discount-tier DiscountPercentage ride the wire as JSON strings and are modeled
// as string end to end — no float hop.
//
// The two list filters (type/ledgerId) have native slots on the generated
// GetAllBillingPackagesParams, so no request-editor injection is needed.
//
// SUCCESS-GATE: the server returns 201 Created on create while the OAS spec says
// 200 (CreateBillingPackageResp gates JSON200 on 200). Create/Update route through
// the RAW ...WithBody + readRawResponse + isSuccess(2xx), never the JSON200
// wrapper, so a 201 is a success.
type billingPackagesFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newBillingPackagesFacade wires the facade over a ledger plane client.
func newBillingPackagesFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *billingPackagesFacade {
	return &billingPackagesFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of billing packages under an organization, normalized
// into the public model. The wire envelope is {items, limit, page, total};
// decoding the raw body into models.ListResponse[models.BillingPackage] reads it
// directly, so the generated FeePagination (whose Items is interface{}) never
// enters the public path.
func (f *billingPackagesFacade) List(ctx context.Context, orgID string, opts models.BillingPackagesListOpts) (*models.ListResponse[models.BillingPackage], error) {
	const operation = "BillingPackages.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.ledger.GetAllBillingPackagesWithResponse(ctx, orgID, listBillingPackagesParams(opts))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var page models.ListResponse[models.BillingPackage]
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &page, nil
}

// ListPages yields one full page per iteration. PAGE mode: it initializes Page=1,
// advances Page++, and stops on !HasMore() (which reads the response total). It
// does NOT stop on an empty cursor — the page-mode envelope carries none, so that
// would truncate the result set after page 1.
func (f *billingPackagesFacade) ListPages(ctx context.Context, orgID string, opts models.BillingPackagesListOpts) iter.Seq2[*models.ListResponse[models.BillingPackage], error] {
	return func(yield func(*models.ListResponse[models.BillingPackage], error) bool) {
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

// ListAll yields every billing package across pages, transparently advancing
// pagination.
func (f *billingPackagesFacade) ListAll(ctx context.Context, orgID string, opts models.BillingPackagesListOpts) iter.Seq2[models.BillingPackage, error] {
	return flattenPages(f.ListPages(ctx, orgID, opts))
}

// Create registers a new billing package under an organization via the
// write-facade pattern (rewindable body so the auth round tripper can replay
// after a 401). It routes through the RAW WithBody + isSuccess(2xx), so the
// server's 201 Created is a success even though the OAS spec says 200.
func (f *billingPackagesFacade) Create(ctx context.Context, orgID string, input *models.CreateBillingPackageInput) (*models.BillingPackage, error) {
	const operation = "BillingPackages.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.BillingPackage](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateBillingPackageWithBody(ctx, orgID, jsonContentType, body))
	})
}

// Get retrieves one billing package by ID under an organization.
func (f *billingPackagesFacade) Get(ctx context.Context, orgID, id string) (*models.BillingPackage, error) {
	const operation = "BillingPackages.Get"

	resp, err := f.ledger.GetBillingPackageByIDWithResponse(ctx, orgID, id)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.BillingPackage](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Update patches a billing package by ID under an organization. Same write-facade
// pattern as Create; UpdateBillingPackageInput.MarshalJSON omits unset fields.
func (f *billingPackagesFacade) Update(ctx context.Context, orgID, id string, input *models.UpdateBillingPackageInput) (*models.BillingPackage, error) {
	const operation = "BillingPackages.Update"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.BillingPackage](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateBillingPackageWithBody(ctx, orgID, id, jsonContentType, body))
	})
}

// Delete removes a billing package by ID under an organization. The server returns
// a no-body success, so there is nothing to decode — only a non-2xx maps to the
// unified error.
func (f *billingPackagesFacade) Delete(ctx context.Context, orgID, id string) error {
	const operation = "BillingPackages.Delete"

	resp, err := f.ledger.DeleteBillingPackageWithResponse(ctx, orgID, id)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	return nil
}

// listBillingPackagesParams renders the typed opts into the generated params.
// Both filters have a native slot, and the pagination fields serialize to the
// *string form the generated query layer expects.
func listBillingPackagesParams(opts models.BillingPackagesListOpts) *genledger.GetAllBillingPackagesParams {
	params := &genledger.GetAllBillingPackagesParams{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.Page > 0 {
		params.Page = strPtr(strconv.Itoa(opts.Page))
	}

	if opts.Filters.Type != "" {
		params.Type = strPtr(opts.Filters.Type)
	}

	if opts.Filters.LedgerID != "" {
		params.LedgerId = strPtr(opts.Filters.LedgerID)
	}

	return params
}

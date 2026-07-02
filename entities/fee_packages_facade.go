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

// feePackagesFacade is the Epic 3.2 (Task 3.2.1) hand-written facade over the
// generated genledger.ClientWithResponses for fee-package definitions, following
// the Organizations/Segments exemplar. The public surface is exactly
// models.FeePackage + *errors.Error + the List/Pages/All trinaldo + full CRUD.
//
// Fee packages are organization-scoped (no ledger path segment) and PAGE-mode
// paginated: unlike the cursor facades (holders/transactions/routes) that stop on
// an empty NextCursor, Pages advances Page++ and stops on !HasMore(). The
// page-mode envelope emits no cursor, so an empty-cursor stop would drop page 2.
//
// Money is money-adjacent here: minimumAmount/maximumAmount and every fee
// calculation value ride the wire as JSON strings and are modeled as string end
// to end — no float hop.
//
// All list filters (segmentId/ledgerId/transactionRoute/enable) have native
// slots on the generated GetAllPackagesParams, so no request-editor injection is
// needed.
type feePackagesFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newFeePackagesFacade wires the facade over a ledger plane client.
func newFeePackagesFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *feePackagesFacade {
	return &feePackagesFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of fee packages under an organization, normalized into
// the public model. The wire envelope is {items, limit, page, total}; decoding
// the raw body into models.ListResponse[models.FeePackage] reads it directly, so
// the generated FeePagination (whose Items is interface{}) never enters the
// public path.
func (f *feePackagesFacade) List(ctx context.Context, orgID string, opts models.PackagesListOpts) (*models.ListResponse[models.FeePackage], error) {
	const operation = "FeePackages.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.ledger.GetAllPackagesWithResponse(ctx, orgID, listPackagesParams(opts))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var page models.ListResponse[models.FeePackage]
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &page, nil
}

// Pages yields one full page per iteration. PAGE mode: it initializes Page=1,
// advances Page++, and stops on !HasMore() (which reads the response total).
// It does NOT stop on an empty cursor — the page-mode envelope carries none, so
// that would truncate the result set after page 1.
func (f *feePackagesFacade) Pages(ctx context.Context, orgID string, opts models.PackagesListOpts) iter.Seq2[*models.ListResponse[models.FeePackage], error] {
	return func(yield func(*models.ListResponse[models.FeePackage], error) bool) {
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

// All yields every fee package across pages, transparently advancing pagination.
func (f *feePackagesFacade) All(ctx context.Context, orgID string, opts models.PackagesListOpts) iter.Seq2[models.FeePackage, error] {
	return flattenPages(f.Pages(ctx, orgID, opts))
}

// Create registers a new fee package under an organization via the write-facade
// pattern (rewindable body so the auth round tripper can replay after a 401).
func (f *feePackagesFacade) Create(ctx context.Context, orgID string, input *models.CreatePackageInput) (*models.FeePackage, error) {
	const operation = "FeePackages.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.FeePackage](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreatePackageWithBody(ctx, orgID, jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one fee package by ID under an organization.
func (f *feePackagesFacade) Get(ctx context.Context, orgID, id string) (*models.FeePackage, error) {
	const operation = "FeePackages.Get"

	resp, err := f.ledger.GetPackageByIDWithResponse(ctx, orgID, id)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.FeePackage](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Update patches a fee package by ID under an organization. Same write-facade
// pattern as Create; UpdatePackageInput.MarshalJSON omits unset fields.
func (f *feePackagesFacade) Update(ctx context.Context, orgID, id string, input *models.UpdatePackageInput) (*models.FeePackage, error) {
	const operation = "FeePackages.Update"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.FeePackage](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdatePackageWithBody(ctx, orgID, id, jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes a fee package by ID under an organization. The server returns a
// no-body success, so there is nothing to decode — only a non-2xx maps to the
// unified error.
func (f *feePackagesFacade) Delete(ctx context.Context, orgID, id string) error {
	const operation = "FeePackages.Delete"

	resp, err := f.ledger.DeletePackageWithResponse(ctx, orgID, id, idempotencyEditors(ctx, f.enableIdempotency)...)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	return nil
}

// listPackagesParams renders the typed opts into the generated params. Every
// filter has a native slot, and the pagination fields serialize to the *string
// form the generated query layer expects.
func listPackagesParams(opts models.PackagesListOpts) *genledger.GetAllPackagesParams {
	params := &genledger.GetAllPackagesParams{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.Page > 0 {
		params.Page = strPtr(strconv.Itoa(opts.Page))
	}

	if opts.Filters.SegmentID != "" {
		params.SegmentId = strPtr(opts.Filters.SegmentID)
	}

	if opts.Filters.LedgerID != "" {
		params.LedgerId = strPtr(opts.Filters.LedgerID)
	}

	if opts.Filters.TransactionRoute != "" {
		params.TransactionRoute = strPtr(opts.Filters.TransactionRoute)
	}

	if opts.Filters.Enable != nil {
		params.Enable = strPtr(strconv.FormatBool(*opts.Filters.Enable))
	}

	return params
}

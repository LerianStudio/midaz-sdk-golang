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

// The V2 facades in this package serve the SAME resources as their V1 siblings
// over the /v2 paths Midaz added while deprecating /v1. They exist as separate
// types rather than as a version flag on one facade because the two surfaces are
// not interchangeable: /v2 dropped asset rates and the four V1 transaction
// creation styles, /v1 answers 404 for holders and the whole billing family, and
// the transaction request and response shapes diverge outright. A flag would
// turn each of those into a runtime 404; separate accessors make them a build
// error. See version_groups.go.
//
// Three properties are shared by every V2 facade and stated once here:
//
//   - Every response is read from the RAW generated call, never through the
//     generated Parse*Resp wrapper. See facade_responses.go for the three
//     failure modes that wrapper introduces.
//   - The public surface is models.* plus *errors.Error; a generated type never
//     reaches a caller.
//   - Operation names are prefixed "V2." so an error tells the reader which
//     surface refused, without them having to recognise the resource.
//
// Where /v2 serves a resource identically to /v1 the facade reuses the V1 input
// models, the V1 list options and the V1 query mapper (see v2_params.go). That
// is a statement about the contract, verified field by field against the
// generated types, not a convenience: the two versions really do declare the
// same request for those families.

// organizationsV2Facade serves the /v2 organization surface — the tenant root.
type organizationsV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newOrganizationsV2Facade wires the facade over a ledger plane client.
func newOrganizationsV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *organizationsV2Facade {
	return &organizationsV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one page of organizations.
func (f *organizationsV2Facade) List(ctx context.Context, opts models.OrganizationsListOpts) (*models.ListResponse[models.Organization], error) {
	const operation = "V2.Organizations.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListOrganizationsV2(ctx, listOrganizationsV2Params(opts), listOrganizationsReqEditors(opts)...)

	return readList[models.Organization](operation, resp, err)
}

// Pages yields one full page per iteration, advancing by page number.
func (f *organizationsV2Facade) Pages(ctx context.Context, opts models.OrganizationsListOpts) iter.Seq2[*models.ListResponse[models.Organization], error] {
	return pageSeq(ctx, opts,
		func(o *models.OrganizationsListOpts) *int { return &o.Page },
		func(current models.OrganizationsListOpts) (*models.ListResponse[models.Organization], error) {
			return f.List(ctx, current)
		})
}

// All yields every organization across pages.
func (f *organizationsV2Facade) All(ctx context.Context, opts models.OrganizationsListOpts) iter.Seq2[models.Organization, error] {
	return flattenPages(f.Pages(ctx, opts))
}

// Create registers a new organization.
func (f *organizationsV2Facade) Create(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error) {
	const operation = "V2.Organizations.Create"

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Organization](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateOrganizationV2WithBody(ctx, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one organization by ID.
func (f *organizationsV2Facade) Get(ctx context.Context, id string) (*models.Organization, error) {
	const operation = "V2.Organizations.Get"

	if err := requirePathIDs(operation, "id", id); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetOrganizationByIDV2(ctx, id)

	return readOne[models.Organization](operation, resp, err)
}

// Update patches an organization by ID.
func (f *organizationsV2Facade) Update(ctx context.Context, id string, input *models.UpdateOrganizationInput) (*models.Organization, error) {
	const operation = "V2.Organizations.Update"

	if err := requirePathIDs(operation, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Organization](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateOrganizationV2WithBody(ctx, id, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes an organization by ID.
func (f *organizationsV2Facade) Delete(ctx context.Context, id string) error {
	const operation = "V2.Organizations.Delete"

	if err := requirePathIDs(operation, "id", id); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteOrganizationV2(ctx, id, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// Count returns the total number of organizations, read from the X-Total-Count
// header of a HEAD request.
func (f *organizationsV2Facade) Count(ctx context.Context) (int, error) {
	//nolint:bodyclose // readCount closes resp.Body via defer.
	return readCount(f.ledger.CountOrganizationsV2(ctx))
}

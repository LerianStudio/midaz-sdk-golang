// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"iter"
	"strconv"

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

// organizationsFacade is the Phase 1 exemplar (Task 1.P1): a hand-written
// facade over the generated genledger.ClientWithResponses that lists
// organizations end-to-end while keeping the generated types out of the
// public SDK surface.
//
// The public surface is exactly models.Organization + *errors.Error + the
// List/Pages/All trinaldo. Phase 2 replicates this shape onto the remaining
// resources; this file is the reference the rest of the facade layer follows.
type organizationsFacade struct {
	ledger *genledger.ClientWithResponses
}

// newOrganizationsFacade wires the facade over a ledger plane client.
func newOrganizationsFacade(ledger *genledger.ClientWithResponses) *organizationsFacade {
	return &organizationsFacade{ledger: ledger}
}

// List retrieves one page of organizations, normalized into the public model.
//
// The request is page-based (Page + Limit); the response carries a cursor
// (next_cursor). We decode the raw response body straight into
// models.ListResponse[models.Organization] — its UnmarshalJSON already reads
// the top-level items + next_cursor wire shape — so the generated Pagination
// (whose Items is an untyped interface{}) never enters the public path.
func (f *organizationsFacade) List(ctx context.Context, opts models.OrganizationsListOpts) (*models.ListResponse[models.Organization], error) {
	const operation = "Organizations.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.ledger.ListOrganizationsWithResponse(ctx, listOrganizationsParams(opts))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != 200 {
		// DecodeProblemJSON maps the unified RFC 9457 envelope both planes emit
		// into *errors.Error with retryability keyed on status + code suffix.
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, "")
	}

	var page models.ListResponse[models.Organization]
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &page, nil
}

// Pages yields one full page per iteration, advancing page-by-page while the
// response reports more results (HasMore prioritizes the response next_cursor).
func (f *organizationsFacade) Pages(ctx context.Context, opts models.OrganizationsListOpts) iter.Seq2[*models.ListResponse[models.Organization], error] {
	return func(yield func(*models.ListResponse[models.Organization], error) bool) {
		current := opts
		if current.Page == 0 {
			current.Page = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := f.List(ctx, current)
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

// All yields every organization across pages, transparently advancing
// pagination.
func (f *organizationsFacade) All(ctx context.Context, opts models.OrganizationsListOpts) iter.Seq2[models.Organization, error] {
	return flattenPages(f.Pages(ctx, opts))
}

// listOrganizationsParams renders the typed opts into the generated params,
// serializing the int pagination fields into the *string form the generated
// query layer expects.
func listOrganizationsParams(opts models.OrganizationsListOpts) *genledger.ListOrganizationsParams {
	params := &genledger.ListOrganizationsParams{}

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

	if opts.Filters.LegalName != "" {
		params.LegalName = strPtr(opts.Filters.LegalName)
	}

	if opts.Filters.Status != "" {
		params.Status = strPtr(opts.Filters.Status)
	}

	return params
}

func strPtr(s string) *string { return &s }

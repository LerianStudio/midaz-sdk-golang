// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"
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

	reqEditors := listOrganizationsReqEditors(opts)

	resp, err := f.ledger.ListOrganizationsWithResponse(ctx, listOrganizationsParams(opts), reqEditors...)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != 200 {
		// DecodeProblemJSON maps the unified RFC 9457 envelope both planes emit
		// into *errors.Error with retryability keyed on status + code suffix.
		// The server's X-Request-ID is threaded through so a client-side
		// failure correlates with the server-side log/trace.
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
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

// listOrganizationsReqEditors builds the request editors that carry query
// params the generated ListOrganizationsParams cannot express. Today that is
// only include_deleted: the ledger OAS spec omits it from ListOrganizations
// (a server-side gap), so the SDK injects the legacy include_deleted=true
// query param through an editor rather than dropping the filter silently.
// Returns nil when no editor is needed so the common path adds zero overhead.
func listOrganizationsReqEditors(opts models.OrganizationsListOpts) []genledger.RequestEditorFn {
	if !opts.Filters.IncludeDeleted {
		return nil
	}

	return []genledger.RequestEditorFn{setQueryParam("include_deleted", "true")}
}

// setQueryParam returns a RequestEditorFn that sets one query parameter on the
// outbound request without disturbing the params the generated client already
// encoded (it re-reads, sets, and re-encodes the existing query).
func setQueryParam(key, value string) genledger.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		q := req.URL.Query()
		q.Set(key, value)
		req.URL.RawQuery = q.Encode()

		return nil
	}
}

// requestIDOf extracts the server's X-Request-ID from the response for
// server↔client failure correlation. Nil-safe: returns "" when the response is
// absent (it never is on this path — we return early on transport error before
// reaching here — but the guard is free on a money-path helper).
func requestIDOf(resp *http.Response) string {
	if resp == nil {
		return ""
	}

	return resp.Header.Get("X-Request-ID")
}

func strPtr(s string) *string { return &s }

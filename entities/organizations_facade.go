// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"bytes"
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

	if resp.StatusCode() != http.StatusOK {
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

// Create registers a new organization. This is the WRITE exemplar the rest of
// the facade layer copies: the generated write op takes an opaque body
// (openapi_types.File), so we marshal the SDK-native input and hand it to the
// ...WithBody variant as a *bytes.Reader. A concrete reader is mandatory — it
// is what lets net/http populate GetBody, the hook the auth round tripper uses
// to rewind and replay the request after a 401 refresh. A non-rewindable body
// would replay empty and silently drop the write.
func (f *organizationsFacade) Create(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error) {
	const operation = "Organizations.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Organization](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.CreateOrganizationWithBodyWithResponse(ctx, &genledger.CreateOrganizationParams{}, "application/json", body)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// Get retrieves one organization by ID, normalized into the public model. Like
// List, it decodes the raw response body into models.Organization so the
// generated type (resp.JSON200) never enters the public path.
func (f *organizationsFacade) Get(ctx context.Context, id string) (*models.Organization, error) {
	const operation = "Organizations.Get"

	resp, err := f.ledger.GetOrganizationByIDWithResponse(ctx, id)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Organization](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Update patches an organization by ID. Same write-facade pattern as Create.
func (f *organizationsFacade) Update(ctx context.Context, id string, input *models.UpdateOrganizationInput) (*models.Organization, error) {
	const operation = "Organizations.Update"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Organization](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.UpdateOrganizationWithBodyWithResponse(ctx, id, "application/json", body)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// Delete removes an organization by ID. The server returns 204 with no body on
// success, so there is nothing to decode — we only map a non-2xx into the
// unified error.
func (f *organizationsFacade) Delete(ctx context.Context, id string) error {
	const operation = "Organizations.Delete"

	resp, err := f.ledger.DeleteOrganizationWithResponse(ctx, id)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	return nil
}

// Count returns the total number of organizations via
// HEAD /organizations/metrics/count, reading the X-Total-Count header. It routes
// through the raw CountOrganizations + readCount so a headers-only error reply
// (empty body) maps to the real status rather than an internal error.
func (f *organizationsFacade) Count(ctx context.Context) (int, error) {
	//nolint:bodyclose // readCount (transactions_facade.go) closes resp.Body via defer.
	return readCount(f.ledger.CountOrganizations(ctx))
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

// writeJSON is the shared write path for the facade layer (the money-path
// exemplar). It marshals a SDK-native input, hands it to send as a *bytes.Reader
// (rewindable so the auth round tripper can replay after a 401), and decodes the
// success body into T. send returns the raw *http.Response + body so error
// mapping and request-ID correlation stay identical to the read path.
func writeJSON[T any](_ context.Context, operation string, input any, send func(body io.Reader) (*http.Response, []byte, error)) (*T, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	//nolint:bodyclose // send is always a readRawResponse closure (transactions_facade.go:58), which closes resp.Body via defer before returning.
	httpResp, body, err := send(bytes.NewReader(payload))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[T](operation, statusOf(httpResp), body, httpResp)
}

// decodeOne maps a single-object response: a non-2xx status decodes the unified
// RFC 9457 error (threading X-Request-ID), otherwise the body unmarshals into T.
// Shared by Get and the write path so the public surface is always models.T or
// *errors.Error — the generated types never leak.
func decodeOne[T any](operation string, status int, body []byte, httpResp *http.Response) (*T, error) {
	if !isSuccess(status) {
		return nil, errors.DecodeProblemJSON(status, body, requestIDOf(httpResp))
	}

	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &out, nil
}

// isSuccess reports whether status is a 2xx.
func isSuccess(status int) bool { return status >= 200 && status < 300 }

// statusOf is a nil-safe status extractor for a response.
func statusOf(resp *http.Response) int {
	if resp == nil {
		return 0
	}

	return resp.StatusCode
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

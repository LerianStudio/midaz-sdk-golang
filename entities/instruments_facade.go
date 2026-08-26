// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strconv"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/sdkctx"
)

// instrumentsFacade is the Epic 3.1 (Task 3.1.3) hand-written facade over the
// generated genledger.ClientWithResponses for CRM instruments.
//
// RE-HOMING: this targets the GENERATED ledger-plane surface. The
// Create/Get/Update/Delete verbs are holder-in-path
// (/organizations/{org}/holders/{holderId}/instruments); the List verb is
// org-scoped (/organizations/{org}/instruments) and narrows to a holder via the
// holder_id query param — NOT a path segment. It deliberately does NOT match the
// superseded legacy entities/instruments wire; that legacy file is a
// model/method-set reference only.
//
// Instruments are treated as CURSOR-paginated: ListPages advances by echoing the
// response next_cursor back into the request as a query param and stops on an
// empty cursor — never HasMore(), whose page-based heuristic can loop forever on
// a full terminal page that carries no cursor. The generated ListInstrumentsV2Params
// has no cursor slot, so the cursor is injected via a request editor (like the
// type filter below); the caller's opts are never mutated.
//
// The generated ListInstrumentsV2Params exposes slots for holder_id/limit/
// sort_order/include_deleted/document. The type filter has no slot, so the
// facade injects it as a query param via a request editor.
//
// Auto-idempotency IS wired: each write threads idempotencyEditors(ctx,
// f.enableIdempotency), which stamps X-Idempotency (and X-TTL when set) on the
// outbound request, gated on enableIdempotency. An explicit or context-supplied
// key (sdkctx.WithIdempotencyKey) stamps regardless of the gate. Writes stay
// replay-safe via the rewindable *bytes.Reader body in writeJSON. The public
// surface stays models.* + *errors.Error; the generated types never leak.
type instrumentsFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newInstrumentsFacade wires the facade over a ledger plane client.
func newInstrumentsFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *instrumentsFacade {
	return &instrumentsFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves one cursor page of instruments for a holder. The generated list
// endpoint is org-scoped, so the holder is carried as the holder_id query param.
// The cursor is seeded from opts.Cursor (empty means the first page); ListPages
// advances it by echoing the response next_cursor. The generated
// ListInstrumentsV2Params has no cursor slot, so the cursor is injected as a query
// param via listInstrumentsReqEditors; the caller's opts are never mutated.
func (f *instrumentsFacade) List(ctx context.Context, orgID, holderID string, opts models.InstrumentsListOpts) (*models.ListResponse[models.Instrument], error) {
	const operation = "Instruments.List"

	// holderID is the one QUERY parameter this helper guards, and that is
	// deliberate rather than an oversight. It is what scopes the list to a
	// single holder, so an empty one does not 400 — it drops out of the query
	// string and widens the request to every instrument in the organization.
	// The caller asked for one holder's instruments and would page through the
	// whole tenant instead, so it is refused here alongside the path ids.
	if err := requirePathIDs(operation, "orgID", orgID, "holderID", holderID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListInstrumentsV2(ctx, orgID, listInstrumentsParams(holderID, opts), listInstrumentsReqEditors(opts)...)

	return readList[models.Instrument](operation, resp, err)
}

// ListPages yields one cursor page per iteration, advancing by the response
// next_cursor until it is empty. current is a value copy seeded from opts, so the
// caller's opts is never mutated.
func (f *instrumentsFacade) ListPages(ctx context.Context, orgID, holderID string, opts models.InstrumentsListOpts) iter.Seq2[*models.ListResponse[models.Instrument], error] {
	return func(yield func(*models.ListResponse[models.Instrument], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := f.List(ctx, orgID, holderID, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			// Cursor-pure stop: paginate by next_cursor, so the only sound
			// terminal signal is an empty cursor. HasMore()'s page-based
			// heuristic would loop forever on a full terminal page that carries
			// no next_cursor.
			if page.Pagination.NextCursor == "" {
				return
			}

			current.Cursor = page.Pagination.NextCursor
		}
	}
}

// ListAll yields every instrument for a holder across cursor pages, transparently
// advancing pagination.
func (f *instrumentsFacade) ListAll(ctx context.Context, orgID, holderID string, opts models.InstrumentsListOpts) iter.Seq2[models.Instrument, error] {
	return flattenPages(f.ListPages(ctx, orgID, holderID, opts))
}

// Create registers a new instrument under a holder via the write-facade pattern
// (marshal input -> rewindable *bytes.Reader -> WithBody variant). Auto-
// idempotency is wired via idempotencyEditors (gated on enableIdempotency). The
// server returns 201 on success.
func (f *instrumentsFacade) Create(ctx context.Context, orgID, holderID string, input *models.CreateInstrumentInput) (*models.Instrument, error) {
	const operation = "Instruments.Create"

	if err := requirePathIDs(operation, "orgID", orgID, "holderID", holderID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Instrument](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateInstrumentV2WithBody(ctx, orgID, holderID, &genledger.CreateInstrumentV2Params{}, jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one instrument by ID under a holder. When the context is tagged
// with sdkctx.WithIncludeDeleted, soft-deleted instruments are included.
func (f *instrumentsFacade) Get(ctx context.Context, orgID, holderID, id string) (*models.Instrument, error) {
	const operation = "Instruments.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "holderID", holderID, "id", id); err != nil {
		return nil, err
	}

	params := &genledger.GetInstrumentByIDV2Params{}
	if sdkctx.IncludeDeletedFromContext(ctx) {
		params.IncludeDeleted = strPtr("true")
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetInstrumentByIDV2(ctx, orgID, holderID, id, params)

	return readOne[models.Instrument](operation, resp, err)
}

// Update patches an instrument by ID under a holder. Same write-facade pattern as
// Create; the server returns 200 on success.
func (f *instrumentsFacade) Update(ctx context.Context, orgID, holderID, id string, input *models.UpdateInstrumentInput) (*models.Instrument, error) {
	const operation = "Instruments.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "holderID", holderID, "id", id); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Instrument](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateInstrumentV2WithBody(ctx, orgID, holderID, id, jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes an instrument by ID under a holder. Soft delete by default; when
// the context is tagged with sdkctx.WithHardDelete the deletion is permanent. The
// server returns 204 with no body on success.
func (f *instrumentsFacade) Delete(ctx context.Context, orgID, holderID, id string) error {
	const operation = "Instruments.Delete"

	if err := requirePathIDs(operation, "orgID", orgID, "holderID", holderID, "id", id); err != nil {
		return err
	}

	params := &genledger.DeleteInstrumentV2Params{}
	if sdkctx.HardDeleteFromContext(ctx) {
		params.HardDelete = strPtr("true")
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteInstrumentV2(ctx, orgID, holderID, id, params, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// DeleteRelatedParty removes one related party from an instrument via
// DELETE .../instruments/{instrumentId}/related-parties/{relatedPartyId}. The
// server returns 204 with no body on success.
func (f *instrumentsFacade) DeleteRelatedParty(ctx context.Context, orgID, holderID, instrumentID, relatedPartyID string) error {
	const operation = "Instruments.DeleteRelatedParty"

	if err := requirePathIDs(operation, "orgID", orgID, "holderID", holderID, "instrumentID", instrumentID, "relatedPartyID", relatedPartyID); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteRelatedPartyV2(ctx, orgID, holderID, instrumentID, relatedPartyID, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

// ListAccountsByHolder retrieves one cursor page of accounts owned by a holder
// via GET .../holders/{holderId}/accounts. Cursor injected via editor (the
// generated ListAccountsByHolderV2Params has no cursor slot); stops on an empty
// next_cursor.
//
// The ledger is a parameter even though it is not a path segment: the endpoint
// requires it as a query parameter. See listAccountsCursor for why it is
// injected by hand.
func (f *instrumentsFacade) ListAccountsByHolder(ctx context.Context, orgID, ledgerID, holderID string, opts models.AccountsListOpts) (*models.ListResponse[models.Account], error) {
	return f.listAccountsCursor(ctx, orgID, ledgerID, holderID, opts, "")
}

// listAccountsCursor fetches a single accounts page, optionally seeded with a
// cursor injected as a query param. The caller keeps the cursor as loop state so
// opts is never mutated.
func (f *instrumentsFacade) listAccountsCursor(ctx context.Context, orgID, ledgerID, holderID string, opts models.AccountsListOpts, cursor string) (*models.ListResponse[models.Account], error) {
	const operation = "Instruments.ListAccountsByHolder"

	// ledgerID is guarded alongside the two path ids although it travels as a
	// query parameter, for the same reason List guards holderID: without it the
	// request is refused by the server, so an empty one is a 400 the caller can
	// be told about locally.
	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "holderID", holderID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	if err := refuseUndeclaredHolderAccountFilters(operation, opts); err != nil {
		return nil, err
	}

	// ledger_id is injected by hand rather than filled into a generated params
	// slot, and the reason is SERVER-SIDE CONTRACT DRIFT rather than a codegen
	// bug: the holder-accounts route REQUIRES ledger_id as a query parameter at
	// runtime (midaz, components/ledger/internal/bootstrap/holder_wiring.go —
	// deliberate there), while the published OpenAPI document does not declare
	// it. oapi-codegen can only generate slots the contract names, so
	// ListAccountsByHolderV2Params has none, and every call without the param
	// 400s with a missing-parameter error. Reported upstream; when the contract
	// catches up this moves into listAccountsByHolderParams and the editor goes
	// away. The cursor below is injected the same way, for the same structural
	// reason.
	editors := []genledger.RequestEditorFn{setQueryParam("ledger_id", ledgerID)}
	if cursor != "" {
		editors = append(editors, setQueryParam("cursor", cursor))
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.ListAccountsByHolderV2(ctx, orgID, holderID, listAccountsByHolderParams(opts), editors...)

	return readList[models.Account](operation, resp, err)
}

// ListAccountsByHolderPages yields one cursor page of holder accounts per
// iteration, advancing by the response next_cursor until it is empty.
func (f *instrumentsFacade) ListAccountsByHolderPages(ctx context.Context, orgID, ledgerID, holderID string, opts models.AccountsListOpts) iter.Seq2[*models.ListResponse[models.Account], error] {
	return func(yield func(*models.ListResponse[models.Account], error) bool) {
		cursor := ""

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := f.listAccountsCursor(ctx, orgID, ledgerID, holderID, opts, cursor)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			if page.Pagination.NextCursor == "" {
				return
			}

			cursor = page.Pagination.NextCursor
		}
	}
}

// ListAccountsByHolderAll yields every account owned by a holder across cursor
// pages, transparently advancing pagination.
func (f *instrumentsFacade) ListAccountsByHolderAll(ctx context.Context, orgID, ledgerID, holderID string, opts models.AccountsListOpts) iter.Seq2[models.Account, error] {
	return flattenPages(f.ListAccountsByHolderPages(ctx, orgID, ledgerID, holderID, opts))
}

// listInstrumentsParams renders the fields that have a slot in the generated
// ListInstrumentsV2Params. holder_id is always set (the list is org-scoped and
// scopes to a holder via this query param). The type filter has no slot and is
// carried by listInstrumentsReqEditors instead.
func listInstrumentsParams(holderID string, opts models.InstrumentsListOpts) *genledger.ListInstrumentsV2Params {
	params := &genledger.ListInstrumentsV2Params{HolderId: strPtr(holderID)}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.SortDirection != "" {
		params.SortOrder = strPtr(string(opts.SortDirection))
	}

	if opts.Filters.Document != "" {
		params.Document = strPtr(opts.Filters.Document)
	}

	if opts.Filters.IncludeDeleted {
		params.IncludeDeleted = strPtr("true")
	}

	return params
}

// listInstrumentsReqEditors carries the cursor pagination token and the type
// filter — neither of which the generated ListInstrumentsV2Params has a slot for.
// The ledger OAS omits cursor and type from the instruments list endpoint, so
// the SDK injects each as a query param rather than dropping it silently. Dates
// are intentionally NOT emitted: the endpoint declares no start_date/end_date
// and there is no evidence the server honors them, so InstrumentsListOpts.Validate
// (ValidateCursorListOptsNoDates) rejects a set date rather than sending a
// silently-ignored filter. Returns nil when none is set so the common path adds
// zero overhead.
func listInstrumentsReqEditors(opts models.InstrumentsListOpts) []genledger.RequestEditorFn {
	var editors []genledger.RequestEditorFn

	if opts.Cursor != "" {
		editors = append(editors, setQueryParam("cursor", opts.Cursor))
	}

	if opts.Filters.Type != "" {
		editors = append(editors, setQueryParam("type", opts.Filters.Type))
	}

	return editors
}

// refuseUndeclaredHolderAccountFilters rejects the AccountsListOpts fields the
// holder-accounts endpoint cannot express, rather than accepting and dropping
// them.
//
// This endpoint reuses AccountsListOpts because it answers with
// models.Account, but it honours almost none of it: only limit and sort_order
// reach the wire, and the cursor is injected by hand. Page cannot work at all —
// the route is driven by next_cursor, so a caller setting Page=3 got page one
// with a nil error. The date range has no declared parameter, and none of the
// twelve AccountsFilters fields does either.
//
// Refusing is the position the instruments list already takes for its own dates
// (ValidateCursorListOptsNoDates): a filter with no wire slot is worse than an
// absent one, because the caller reads an unnarrowed result set as a narrowed
// one. The refusal is FACADE-LOCAL rather than in AccountsListOpts.Validate,
// because the regular accounts list does honour every one of these fields.
func refuseUndeclaredHolderAccountFilters(operation string, opts models.AccountsListOpts) error {
	undeclared := []struct {
		field string
		set   bool
	}{
		{"Page", opts.Page > 0},
		{"StartDate", opts.StartDate != ""},
		{"EndDate", opts.EndDate != ""},
		{"Filters.Type", opts.Filters.Type != ""},
		{"Filters.Status", opts.Filters.Status != ""},
		{"Filters.AssetCode", opts.Filters.AssetCode != ""},
		{"Filters.HolderID", opts.Filters.HolderID != ""},
		{"Filters.PortfolioID", opts.Filters.PortfolioID != ""},
		{"Filters.SegmentID", opts.Filters.SegmentID != ""},
		{"Filters.Alias", opts.Filters.Alias != ""},
		{"Filters.ParentAccountID", opts.Filters.ParentAccountID != ""},
		{"Filters.Name", opts.Filters.Name != ""},
		{"Filters.EntityID", opts.Filters.EntityID != ""},
		{"Filters.IncludeDeleted", opts.Filters.IncludeDeleted},
		{"Filters.Blocked", opts.Filters.Blocked},
	}

	var named []string

	for _, f := range undeclared {
		if f.set {
			named = append(named, f.field)
		}
	}

	if len(named) == 0 {
		return nil
	}

	return errors.NewValidationError(operation, fmt.Sprintf(
		"the holder-accounts listing does not narrow by %s; it honours only Limit and "+
			"SortDirection, and advances by cursor rather than by Page, so sending these would "+
			"return every account the holder owns as if it had been narrowed. Narrow client-side, "+
			"or use V2.Accounts.List with Filters.HolderID",
		strings.Join(named, ", ")), nil)
}

// listAccountsByHolderParams renders the fields that have a slot in the generated
// ListAccountsByHolderV2Params (limit/sort_order). Cursor pagination is injected
// separately via a request editor.
func listAccountsByHolderParams(opts models.AccountsListOpts) *genledger.ListAccountsByHolderV2Params {
	params := &genledger.ListAccountsByHolderV2Params{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.SortDirection != "" {
		params.SortOrder = strPtr(string(opts.SortDirection))
	}

	return params
}

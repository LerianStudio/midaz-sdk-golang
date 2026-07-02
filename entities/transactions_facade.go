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

// transactionsFacade is the money-write crown jewel (Epic 2.2): a hand-written
// facade over the generated genledger.ClientWithResponses covering the
// transaction lifecycle. This file (Task 2.2.1) implements the four create
// paths: /json, /inflow, /outflow, /annotation.
//
// It follows the accounts/organizations write exemplar exactly — a rewindable
// *bytes.Reader body (via writeJSON, so the auth round tripper can replay after
// a 401), raw-bytes decode into models.Transaction (never the generated type,
// whose openapi_types.UUID would eager-validate on 200), and unified RFC 9457
// error mapping — with one money-path addition: every create wires an
// idempotency key + TTL through resolveIdempotency (Task 2.2.0). Without it a
// network retry would post a second balance mutation (double-entry violation).
//
// Two subtleties distinguish transactions from the onboarding resources:
//
//   - Wire shape is the endpoint-specific mapper output, NOT json.Marshal(input).
//     /json and /annotation serialize via ToLibTransaction(); /inflow and
//     /outflow via ToMap(). The generated request body is opaque
//     (openapi_types.File), so the facade owns the shape; it must match the
//     legacy transactions service byte-for-byte (entities/transactions.go).
//   - Success is HTTP 200 (not the 201 onboarding creates return). isSuccess
//     (2xx) covers both, so nothing hardcodes the code.
type transactionsFacade struct {
	ledger *genledger.ClientWithResponses
}

// newTransactionsFacade wires the facade over a ledger plane client.
func newTransactionsFacade(ledger *genledger.ClientWithResponses) *transactionsFacade {
	return &transactionsFacade{ledger: ledger}
}

// jsonContentType is the content type every create sends. The generated request
// body is opaque, so the facade names the media type explicitly.
const jsonContentType = "application/json"

// CreateJSON creates a standard transaction (source + distribute) via
// POST .../transactions/json. The wire body is input.ToLibTransaction().
func (f *transactionsFacade) CreateJSON(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error) {
	const operation = "Transactions.CreateJSON"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	params := &genledger.CreateTransactionJSONParams{}
	key, ttl := resolveIdempotency(ctx, input.IdempotencyKey, true)
	applyIdempotency(&params.XIdempotency, &params.XTTL, key, ttl)

	return writeJSON[models.Transaction](ctx, operation, input.ToLibTransaction(), func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.CreateTransactionJSONWithBodyWithResponse(ctx, orgID, ledgerID, params, jsonContentType, body)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// CreateInflow creates an inflow transaction (no source; funds flow into
// destination accounts) via POST .../transactions/inflow. The wire body is
// input.ToMap().
func (f *transactionsFacade) CreateInflow(ctx context.Context, orgID, ledgerID string, input *models.CreateInflowInput) (*models.Transaction, error) {
	const operation = "Transactions.CreateInflow"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	params := &genledger.CreateTransactionInflowParams{}
	key, ttl := resolveIdempotency(ctx, "", true)
	applyIdempotency(&params.XIdempotency, &params.XTTL, key, ttl)

	return writeJSON[models.Transaction](ctx, operation, input.ToMap(), func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.CreateTransactionInflowWithBodyWithResponse(ctx, orgID, ledgerID, params, jsonContentType, body)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// CreateOutflow creates an outflow transaction (no destination; funds flow out
// of source accounts) via POST .../transactions/outflow. The wire body is
// input.ToMap().
func (f *transactionsFacade) CreateOutflow(ctx context.Context, orgID, ledgerID string, input *models.CreateOutflowInput) (*models.Transaction, error) {
	const operation = "Transactions.CreateOutflow"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	params := &genledger.CreateTransactionOutflowParams{}
	key, ttl := resolveIdempotency(ctx, "", true)
	applyIdempotency(&params.XIdempotency, &params.XTTL, key, ttl)

	return writeJSON[models.Transaction](ctx, operation, input.ToMap(), func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.CreateTransactionOutflowWithBodyWithResponse(ctx, orgID, ledgerID, params, jsonContentType, body)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// CreateAnnotation creates an annotation transaction (metadata-only, no balance
// impact) via POST .../transactions/annotation. The wire body is
// input.ToLibTransaction().
func (f *transactionsFacade) CreateAnnotation(ctx context.Context, orgID, ledgerID string, input *models.CreateAnnotationInput) (*models.Transaction, error) {
	const operation = "Transactions.CreateAnnotation"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	params := &genledger.CreateTransactionAnnotationParams{}
	key, ttl := resolveIdempotency(ctx, "", true)
	applyIdempotency(&params.XIdempotency, &params.XTTL, key, ttl)

	return writeJSON[models.Transaction](ctx, operation, input.ToLibTransaction(), func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.CreateTransactionAnnotationWithBodyWithResponse(ctx, orgID, ledgerID, params, jsonContentType, body)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// Commit finalizes a PENDING transaction (PENDING → APPROVED) via
// POST .../transactions/{id}/commit. Success is HTTP 201. The action carries no
// body and is not auto-idempotent: it stamps X-Idempotency only when the caller
// supplied a key (input struct has none, so via sdkctx.WithIdempotencyKey) —
// parity with the legacy transactionActionContext.
func (f *transactionsFacade) Commit(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "Transactions.Commit"

	resp, err := f.ledger.CommitTransactionWithResponse(ctx, orgID, ledgerID, transactionID, actionIdempotencyEditors(ctx)...)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Transaction](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Revert reverses a committed transaction via POST .../transactions/{id}/revert.
// It returns the CHILD (reversal) transaction — a new record whose
// ParentTransactionID points at the original — and never mutates the original.
// Success is HTTP 201; same non-auto-idempotent action semantics as Commit.
func (f *transactionsFacade) Revert(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "Transactions.Revert"

	resp, err := f.ledger.RevertTransactionWithResponse(ctx, orgID, ledgerID, transactionID, actionIdempotencyEditors(ctx)...)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Transaction](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// Cancel aborts a PENDING transaction (PENDING → CANCELED) via
// POST .../transactions/{id}/cancel. Success is HTTP 201. The cancel endpoint
// sometimes returns an empty (or "null") body; in that case we synthesize a
// CANCELED transaction rather than failing the decode, so callers always get a
// status-bearing value — parity with the legacy CancelTransactionWithResponse.
func (f *transactionsFacade) Cancel(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "Transactions.Cancel"

	resp, err := f.ledger.CancelTransactionWithResponse(ctx, orgID, ledgerID, transactionID, actionIdempotencyEditors(ctx)...)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	if isEmptyBody(resp.Body) {
		return &models.Transaction{ID: transactionID, Status: models.Status{Code: string(models.TransactionStatusCanceled)}}, nil
	}

	return decodeOne[models.Transaction](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// isEmptyBody reports whether a success body carries no transaction — an empty
// body or the JSON literal "null" (the cancel endpoint emits either).
func isEmptyBody(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

// actionIdempotencyEditors returns the request editors for a lifecycle action.
// Actions are not auto-idempotent (autoGen=false): a key rides only when the
// caller supplied one via sdkctx.WithIdempotencyKey. Returns nil (no editor,
// no header) otherwise, so the common path stays header-free.
func actionIdempotencyEditors(ctx context.Context) []genledger.RequestEditorFn {
	key, _ := resolveIdempotency(ctx, "", false)
	if key == "" {
		return nil
	}

	return []genledger.RequestEditorFn{setHeader(idempotencyHeader, key)}
}

// UpdateTransaction patches a transaction's mutable fields (metadata +
// description) via PATCH .../transactions/{id}. Same write-facade pattern as the
// creates — a rewindable *bytes.Reader body so the auth round tripper can replay
// after a 401 — but the wire body is the whole input object (json.Marshal),
// sent as plain application/json (parity with the legacy PATCH; NOT merge-patch).
// input.Validate rejects an empty payload before any request leaves the process.
// Success is HTTP 200. No idempotency: a patch is not a balance mutation, and
// the legacy path carried none.
func (f *transactionsFacade) UpdateTransaction(ctx context.Context, orgID, ledgerID, transactionID string, input *models.UpdateTransactionInput) (*models.Transaction, error) {
	const operation = "Transactions.UpdateTransaction"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Transaction](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.UpdateTransactionWithBodyWithResponse(ctx, orgID, ledgerID, transactionID, jsonContentType, body)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// UpdateOperation patches an operation's mutable fields (metadata + description)
// via PATCH .../transactions/{txID}/operations/{opID}. Same write-facade pattern
// as UpdateTransaction, but the 200 body decodes into models.Operation (this
// endpoint returns the operation, not the parent transaction). input.Validate
// rejects an empty payload before any request leaves the process.
func (f *transactionsFacade) UpdateOperation(ctx context.Context, orgID, ledgerID, transactionID, operationID string, input *models.UpdateOperationInput) (*models.Operation, error) {
	const operation = "Transactions.UpdateOperation"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.Operation](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.UpdateOperationWithBodyWithResponse(ctx, orgID, ledgerID, transactionID, operationID, jsonContentType, body)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// Get retrieves one transaction by ID under an org+ledger. Like the onboarding
// reads, it decodes the raw response body into models.Transaction (never the
// generated genledger.Transaction, whose openapi_types.UUID would eager-validate
// on 200), so the generated type never enters the public path.
func (f *transactionsFacade) Get(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "Transactions.Get"

	resp, err := f.ledger.GetTransactionWithResponse(ctx, orgID, ledgerID, transactionID)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Transaction](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)
}

// List retrieves one cursor page of transactions under an org+ledger. The
// endpoint is CURSOR-paginated: opts carry a Cursor (never a Page). The
// generated GetAllTransactionsParams only exposes the cursor/limit/sort/date
// slots — the six transaction filters (asset_code/status/reference/
// source_account/destination_account/route) and the metadata predicate have no
// slot, so they ride as query-param request editors rather than being dropped.
func (f *transactionsFacade) List(ctx context.Context, orgID, ledgerID string, opts models.TransactionsListOpts) (*models.ListResponse[models.Transaction], error) {
	const operation = "Transactions.List"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	resp, err := f.ledger.GetAllTransactionsWithResponse(ctx, orgID, ledgerID, listTransactionsParams(opts), listTransactionsReqEditors(opts)...)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if resp.StatusCode() != 200 {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var page models.ListResponse[models.Transaction]
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return &page, nil
}

// Pages yields one cursor page per iteration, advancing by the response
// next_cursor until it is empty.
func (f *transactionsFacade) Pages(ctx context.Context, orgID, ledgerID string, opts models.TransactionsListOpts) iter.Seq2[*models.ListResponse[models.Transaction], error] {
	return func(yield func(*models.ListResponse[models.Transaction], error) bool) {
		current := opts

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

			// Cursor-pure stop: this endpoint paginates by next_cursor, so the
			// only sound terminal signal is an empty cursor. HasMore()'s
			// page-based heuristic (branch 3) returns true on a full terminal
			// page that carries a page field but no cursor, which would set
			// current.Cursor = "" and refetch the first page forever.
			if page.Pagination.NextCursor == "" {
				return
			}

			current.Cursor = page.Pagination.NextCursor
		}
	}
}

// All yields every transaction across cursor pages, transparently advancing
// pagination via the server-issued next_cursor.
func (f *transactionsFacade) All(ctx context.Context, orgID, ledgerID string, opts models.TransactionsListOpts) iter.Seq2[models.Transaction, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Count returns the number of transactions matching the count-endpoint filters
// (status/route/date-range). The endpoint is HEAD .../transactions/metrics/count
// and reports the total in the X-Total-Count header; the count params struct
// mirrors exactly the four filters the endpoint honors, so no editor injection
// is needed. A missing/blank/non-integer/negative header is an error, never a
// silent zero.
func (f *transactionsFacade) Count(ctx context.Context, orgID, ledgerID string, opts models.TransactionsListOpts) (int, error) {
	const operation = "Transactions.Count"

	if err := opts.Validate(); err != nil {
		return 0, err
	}

	resp, err := f.ledger.CountTransactionsByFiltersWithResponse(ctx, orgID, ledgerID, countTransactionsParams(opts))
	if err != nil {
		return 0, errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return 0, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	count, err := parseTotalCountHeader(resp.HTTPResponse.Header)
	if err != nil {
		return 0, errors.NewInternalError(operation, err)
	}

	return count, nil
}

// listTransactionsParams renders the cursor/limit/sort/date fields into the
// generated GetAllTransactionsParams. The filters have no slot here and are
// carried by listTransactionsReqEditors.
func listTransactionsParams(opts models.TransactionsListOpts) *genledger.GetAllTransactionsParams {
	params := &genledger.GetAllTransactionsParams{}

	if opts.Limit > 0 {
		params.Limit = strPtr(strconv.Itoa(opts.Limit))
	}

	if opts.Cursor != "" {
		params.Cursor = strPtr(opts.Cursor)
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

	return params
}

// listTransactionsReqEditors carries the transaction filters the generated
// GetAllTransactionsParams cannot express. The ledger OAS omits every
// TransactionsFilters field from the list params (it exposes only a single
// opaque metadata JSON slot), so the SDK injects each filter as a query param
// under its legacy wire name rather than dropping it silently — including the
// single metadata predicate rendered as metadata.<key>=<value>. Returns nil
// when no filter is set so the common path adds zero overhead.
func listTransactionsReqEditors(opts models.TransactionsListOpts) []genledger.RequestEditorFn {
	var editors []genledger.RequestEditorFn

	if opts.Filters.AssetCode != "" {
		editors = append(editors, setQueryParam("asset_code", opts.Filters.AssetCode))
	}

	if opts.Filters.Status != "" {
		editors = append(editors, setQueryParam("status", opts.Filters.Status))
	}

	if opts.Filters.Reference != "" {
		editors = append(editors, setQueryParam("reference", opts.Filters.Reference))
	}

	if opts.Filters.SourceAccount != "" {
		editors = append(editors, setQueryParam("source_account", opts.Filters.SourceAccount))
	}

	if opts.Filters.DestinationAccount != "" {
		editors = append(editors, setQueryParam("destination_account", opts.Filters.DestinationAccount))
	}

	if opts.Filters.Route != "" {
		editors = append(editors, setQueryParam("route", opts.Filters.Route))
	}

	if opts.Filters.MetadataKey != "" && opts.Filters.MetadataValue != "" {
		editors = append(editors, setQueryParam("metadata."+opts.Filters.MetadataKey, opts.Filters.MetadataValue))
	}

	return editors
}

// countTransactionsParams renders ONLY the four filters the HEAD count endpoint
// honors (status/route/start_date/end_date) — parity with the legacy
// transactionMetricsCountQueryParams. Cursor/limit/sort do not apply to a count.
func countTransactionsParams(opts models.TransactionsListOpts) *genledger.CountTransactionsByFiltersParams {
	params := &genledger.CountTransactionsByFiltersParams{}

	if opts.Filters.Status != "" {
		params.Status = strPtr(opts.Filters.Status)
	}

	if opts.Filters.Route != "" {
		params.Route = strPtr(opts.Filters.Route)
	}

	if opts.StartDate != "" {
		params.StartDate = strPtr(opts.StartDate)
	}

	if opts.EndDate != "" {
		params.EndDate = strPtr(opts.EndDate)
	}

	return params
}

// applyIdempotency stamps the resolved key/TTL onto a generated create's params
// pointers, setting each only when non-empty so an unset value omits the header
// and lets the server apply its default (X-TTL default 300s). The generated
// request builder emits X-Idempotency / X-TTL from these params — never
// X-Idempotency-Key.
func applyIdempotency(idempotency, ttl **string, key, ttlValue string) {
	if key != "" {
		*idempotency = &key
	}

	if ttlValue != "" {
		*ttl = &ttlValue
	}
}

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

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
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
//   - Wire shape is the endpoint-specific mapper output, and json.Marshal(input)
//     now IS that output: every create input implements MarshalJSON delegating to
//     its mapper (ToLibTransaction for /json and /annotation, ToMap for /inflow
//     and /outflow), so the facade hands the input straight to writeJSON. One
//     path, no second serialization to drift from — the struct tags describe
//     nothing and cannot lie about what moved money. The generated request body
//     is opaque (openapi_types.File), so the shape stays the facade's contract
//     and must match the legacy transactions service byte-for-byte
//     (entities/transactions.go); entities/transactions_facade_wire_test.go pins
//     the equivalence per endpoint.
//   - Success is any HTTP 2xx, read from the RAW response. The writes bypass the
//     generated typed parser (Parse{Op}Resp) on purpose: that parser gates on the
//     one status code the OAS declares (creates 200, actions 201, updates 200)
//     and routes every other status — including a 2xx the server really returned
//     (async 202, or an OAS-vs-server drift) — into a json.Unmarshal against the
//     Error type, whose status field is an int64. A real transaction body carries
//     status as an object, so that unmarshal fails and a CONFIRMED write surfaces
//     as a spurious internal error. Reading the raw bytes + isSuccess(2xx)
//     accepts any 2xx and decodes into models.Transaction, matching the legacy
//     transactions service (entities/http.go, StatusCode < 400).
type transactionsFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on creates; an
	// explicit input/context key stamps regardless, and lifecycle actions
	// (commit/cancel/revert, autoGen=false) are unaffected.
	enableIdempotency bool
}

// readRawResponse drains a generated lower-level call's raw response into bytes,
// closing the body, so the write path can decide success on isSuccess(2xx) and
// decode into models.* — never through the status-exact generated parser (see
// the transactionsFacade doc). Threads any transport error straight through.
func readRawResponse(resp *http.Response, err error) (*http.Response, []byte, error) {
	if err != nil {
		return nil, nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	// Bound the read so a hostile/broken server cannot pin arbitrary memory on
	// the plane write path. The plane retry round tripper returns 2xx untouched
	// (retry_roundtripper.go), so this shared drain is the only cap point for
	// the money-path writes — mirroring the legacy client's overflow rejection
	// (http_retry_response.go:186). Read one byte past the cap to detect overflow.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBodyBytes+1))
	if err != nil {
		return nil, nil, err
	}

	if int64(len(body)) > maxHTTPResponseBodyBytes {
		return nil, nil, fmt.Errorf("response body exceeds %d bytes", maxHTTPResponseBodyBytes)
	}

	return resp, body, nil
}

// newTransactionsFacade wires the facade over a ledger plane client.
func newTransactionsFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *transactionsFacade {
	return &transactionsFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// jsonContentType is the content type every create sends. The generated request
// body is opaque, so the facade names the media type explicitly.
const jsonContentType = "application/json"

// CreateJSON creates a standard transaction (source + distribute) via
// POST .../transactions/json. The wire body is json.Marshal(input), which the
// input's MarshalJSON routes through ToLibTransaction().
func (f *transactionsFacade) CreateJSON(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error) {
	const operation = "Transactions.CreateJSON"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	params := &genledger.CreateTransactionJSONParams{}
	key, ttl := resolveIdempotency(ctx, input.IdempotencyKey, f.enableIdempotency)
	applyIdempotency(&params.XIdempotency, &params.XTTL, key, ttl)

	return writeJSON[models.Transaction](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateTransactionJSONWithBody(ctx, orgID, ledgerID, params, jsonContentType, body))
	})
}

// CreateInflow creates an inflow transaction (no source; funds flow into
// destination accounts) via POST .../transactions/inflow. The wire body is
// json.Marshal(input), which the input's MarshalJSON routes through ToMap().
func (f *transactionsFacade) CreateInflow(ctx context.Context, orgID, ledgerID string, input *models.CreateInflowInput) (*models.Transaction, error) {
	const operation = "Transactions.CreateInflow"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	params := &genledger.CreateTransactionInflowParams{}
	key, ttl := resolveIdempotency(ctx, "", f.enableIdempotency)
	applyIdempotency(&params.XIdempotency, &params.XTTL, key, ttl)

	return writeJSON[models.Transaction](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateTransactionInflowWithBody(ctx, orgID, ledgerID, params, jsonContentType, body))
	})
}

// CreateOutflow creates an outflow transaction (no destination; funds flow out
// of source accounts) via POST .../transactions/outflow. The wire body is
// json.Marshal(input), which the input's MarshalJSON routes through ToMap().
func (f *transactionsFacade) CreateOutflow(ctx context.Context, orgID, ledgerID string, input *models.CreateOutflowInput) (*models.Transaction, error) {
	const operation = "Transactions.CreateOutflow"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	params := &genledger.CreateTransactionOutflowParams{}
	key, ttl := resolveIdempotency(ctx, "", f.enableIdempotency)
	applyIdempotency(&params.XIdempotency, &params.XTTL, key, ttl)

	return writeJSON[models.Transaction](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateTransactionOutflowWithBody(ctx, orgID, ledgerID, params, jsonContentType, body))
	})
}

// CreateAnnotation creates an annotation transaction (metadata-only, no balance
// impact) via POST .../transactions/annotation. The wire body is
// json.Marshal(input), which the input's MarshalJSON routes through
// ToLibTransaction().
func (f *transactionsFacade) CreateAnnotation(ctx context.Context, orgID, ledgerID string, input *models.CreateAnnotationInput) (*models.Transaction, error) {
	const operation = "Transactions.CreateAnnotation"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	params := &genledger.CreateTransactionAnnotationParams{}
	key, ttl := resolveIdempotency(ctx, "", f.enableIdempotency)
	applyIdempotency(&params.XIdempotency, &params.XTTL, key, ttl)

	return writeJSON[models.Transaction](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateTransactionAnnotationWithBody(ctx, orgID, ledgerID, params, jsonContentType, body))
	})
}

// CreateBlock creates a transaction that BLOCKS value on the accounts it names,
// via POST .../transactions/block.
//
// The body is the same shape as CreateJSON — the endpoint takes
// CreateTransactionInput and the wire body is json.Marshal(input), routed
// through ToLibTransaction(). What differs is entirely server-side: the ledger
// stamps every resulting operation with the BLOCK type and forces the
// transaction non-pending, so it settles immediately instead of waiting for a
// commit. Reverse it with CreateUnblock, not with Cancel.
func (f *transactionsFacade) CreateBlock(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error) {
	const operation = "Transactions.CreateBlock"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	params := &genledger.CreateTransactionBlockParams{}
	key, ttl := resolveIdempotency(ctx, input.IdempotencyKey, f.enableIdempotency)
	applyIdempotency(&params.XIdempotency, &params.XTTL, key, ttl)

	return writeJSON[models.Transaction](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateTransactionBlockWithBody(ctx, orgID, ledgerID, params, jsonContentType, body))
	})
}

// CreateUnblock releases value a block transaction held, via
// POST .../transactions/unblock. Same body and same immediate-settlement
// semantics as CreateBlock; the ledger stamps UNBLOCK instead of BLOCK.
func (f *transactionsFacade) CreateUnblock(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error) {
	const operation = "Transactions.CreateUnblock"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	params := &genledger.CreateTransactionUnblockParams{}
	key, ttl := resolveIdempotency(ctx, input.IdempotencyKey, f.enableIdempotency)
	applyIdempotency(&params.XIdempotency, &params.XTTL, key, ttl)

	return writeJSON[models.Transaction](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateTransactionUnblockWithBody(ctx, orgID, ledgerID, params, jsonContentType, body))
	})
}

// Commit finalizes a PENDING transaction (PENDING → APPROVED) via
// POST .../transactions/{id}/commit. Success is HTTP 201. The action carries no
// body and is not auto-idempotent: it stamps X-Idempotency only when the caller
// supplied a key (input struct has none, so via sdkctx.WithIdempotencyKey) —
// parity with the prior action-context idempotency handling.
func (f *transactionsFacade) Commit(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "Transactions.Commit"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "transactionID", transactionID); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readRawResponse (transactions_facade.go:58) closes resp.Body via defer before returning.
	resp, body, err := readRawResponse(f.ledger.CommitTransaction(ctx, orgID, ledgerID, transactionID, actionIdempotencyEditors(ctx)...))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Transaction](operation, resp.StatusCode, body, resp)
}

// Revert reverses a committed transaction via POST .../transactions/{id}/revert.
// It returns the CHILD (reversal) transaction — a new record whose
// ParentTransactionID points at the original — and never mutates the original.
// Success is HTTP 201; same non-auto-idempotent action semantics as Commit.
func (f *transactionsFacade) Revert(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "Transactions.Revert"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "transactionID", transactionID); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readRawResponse (transactions_facade.go:58) closes resp.Body via defer before returning.
	resp, body, err := readRawResponse(f.ledger.RevertTransaction(ctx, orgID, ledgerID, transactionID, actionIdempotencyEditors(ctx)...))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[models.Transaction](operation, resp.StatusCode, body, resp)
}

// Cancel aborts a PENDING transaction (PENDING → CANCELED) via
// POST .../transactions/{id}/cancel. Success is HTTP 201.
//
// The cancel endpoint has been observed returning an empty (or "null") body, and
// this is the one single-object call on either surface that tolerates it: rather
// than failing the decode, the facade synthesizes a CANCELED transaction —
// parity with the legacy CancelTransactionWithResponse. Every other 2xx that
// carries no resource is refused in decodeOne, Commit and Revert included. See
// transactionsV2Facade.Cancel for why cancel alone can synthesize and they
// cannot.
func (f *transactionsFacade) Cancel(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "Transactions.Cancel"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "transactionID", transactionID); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readRawResponse (transactions_facade.go:58) closes resp.Body via defer before returning.
	resp, body, err := readRawResponse(f.ledger.CancelTransaction(ctx, orgID, ledgerID, transactionID, actionIdempotencyEditors(ctx)...))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode) {
		return nil, errors.DecodeProblemJSON(resp.StatusCode, body, requestIDOf(resp))
	}

	if isEmptyBody(body) {
		return &models.Transaction{ID: transactionID, Status: models.Status{Code: string(models.TransactionStatusCanceled)}}, nil
	}

	return decodeOne[models.Transaction](operation, resp.StatusCode, body, resp)
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
// Success is HTTP 200. Idempotency IS stamped (gated on enableIdempotency): the
// legacy PATCH auto-generated a key and honored a caller's ctx key, so the
// facade matches.
func (f *transactionsFacade) UpdateTransaction(ctx context.Context, orgID, ledgerID, transactionID string, input *models.UpdateTransactionInput) (*models.Transaction, error) {
	const operation = "Transactions.UpdateTransaction"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "transactionID", transactionID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Transaction](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateTransactionWithBody(ctx, orgID, ledgerID, transactionID, jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// UpdateOperation patches an operation's mutable fields (metadata + description)
// via PATCH .../transactions/{txID}/operations/{opID}. Same write-facade pattern
// as UpdateTransaction, but the 200 body decodes into models.Operation (this
// endpoint returns the operation, not the parent transaction). input.Validate
// rejects an empty payload before any request leaves the process.
func (f *transactionsFacade) UpdateOperation(ctx context.Context, orgID, ledgerID, transactionID, operationID string, input *models.UpdateOperationInput) (*models.Operation, error) {
	const operation = "Transactions.UpdateOperation"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "transactionID", transactionID, "operationID", operationID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.Operation](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateOperationWithBody(ctx, orgID, ledgerID, transactionID, operationID, jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Get retrieves one transaction by ID under an org+ledger. It decodes the RAW
// response body into models.Transaction (never the generated
// genledger.Transaction, whose openapi_types.UUID would eager-validate on 200),
// so the generated type never enters the public path.
//
// Raw rather than through GetTransactionWithResponse, matching every other
// method in this money-path facade: the generated parser unmarshals before the
// facade sees anything, so a 2xx whose body a gateway dropped fails INSIDE it
// and surfaces as an internal error — "the SDK is broken" — instead of as the
// response-decode error that tells a caller the server answered and the read
// could not be trusted. Reading raw puts every 2xx shape in front of the shared
// decodeOne guard.
func (f *transactionsFacade) Get(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "Transactions.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "transactionID", transactionID); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetTransaction(ctx, orgID, ledgerID, transactionID)

	return readOne[models.Transaction](operation, resp, err)
}

// List retrieves one cursor page of transactions under an org+ledger. The
// endpoint is CURSOR-paginated: opts carry a Cursor (never a Page). The
// generated GetAllTransactionsParams only exposes the cursor/limit/sort/date
// slots — the six transaction filters (asset_code/status/reference/
// source_account/destination_account/route) and the metadata predicate have no
// slot, so they ride as query-param request editors rather than being dropped.
func (f *transactionsFacade) List(ctx context.Context, orgID, ledgerID string, opts models.TransactionsListOpts) (*models.ListResponse[models.Transaction], error) {
	const operation = "Transactions.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAllTransactions(ctx, orgID, ledgerID, listTransactionsParams(opts), listTransactionsReqEditors(opts)...)

	return readList[models.Transaction](operation, resp, err)
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
//
// It calls the LOWER-LEVEL raw CountTransactionsByFilters (returning the raw
// *http.Response) rather than the generated WithResponse variant on purpose: a
// HEAD count is a headers-only reply, so an error status arrives with a JSON
// content-type header and an EMPTY body. The generated parser gates on "json"
// in the content type and json.Unmarshals that empty body, which errors —
// misclassifying a real 403 as an internal error. readCount decodes the status
// directly (DecodeProblemJSON handles the empty body), so the true status
// surfaces.
func (f *transactionsFacade) Count(ctx context.Context, orgID, ledgerID string, opts models.TransactionsListOpts) (int, error) {
	if err := requirePathIDs("Transactions.Count", "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return 0, err
	}

	if err := opts.Validate(); err != nil {
		return 0, err
	}

	//nolint:bodyclose // readCount (transactions_facade.go) closes resp.Body via defer.
	return readCount(f.ledger.CountTransactionsByFilters(ctx, orgID, ledgerID, countTransactionsParams(opts)))
}

// readCount maps a HEAD count response into the total. A transport error becomes
// an internal error; a non-2xx decodes the unified RFC 9457 envelope via
// DecodeProblemJSON (which handles the empty body a HEAD error carries, unlike
// the generated status-exact parser); a 2xx reads the X-Total-Count header,
// where a missing/blank/non-integer/negative value is an error, never a silent
// zero. Shared by the transactions count and the six onboarding counts.
func readCount(resp *http.Response, err error) (int, error) {
	const operation = "Count"

	if err != nil {
		return 0, errors.NewInternalError(operation, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if !isSuccess(resp.StatusCode) {
		// A read error here just yields a nil body; DecodeProblemJSON degrades a
		// nil/empty body to a status-only error, so the true status still surfaces.
		body, _ := io.ReadAll(resp.Body) //nolint:errcheck // see comment: nil body degrades cleanly.
		return 0, errors.DecodeProblemJSON(resp.StatusCode, body, requestIDOf(resp))
	}

	count, err := parseTotalCountHeader(resp.Header)
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
// honors (status/route/start_date/end_date). Cursor/limit/sort do not apply to
// a count.
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

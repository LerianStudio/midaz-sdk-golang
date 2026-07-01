// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"bytes"
	"context"
	"io"
	"net/http"

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

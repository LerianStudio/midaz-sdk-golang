// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
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

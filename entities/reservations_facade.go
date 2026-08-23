// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/gentracer"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// reservationsFacade is the tracer-plane two-phase reservation facade (a twin of
// rulesFacade/limitsFacade/validationsFacade). It exposes only models.* +
// *errors.Error; the generated types never leak. Money (ReserveInput.Amount) is
// shopspring/decimal end to end — never float. It reuses the shared party context
// models (models.AccountContext et al.) verbatim.
type reservationsFacade struct {
	tracer *gentracer.ClientWithResponses
}

// newReservationsFacade wires the facade over a tracer plane client.
func newReservationsFacade(tracer *gentracer.ClientWithResponses) *reservationsFacade {
	return &reservationsFacade{tracer: tracer}
}

// Reserve holds capacity against the active limits for a transaction
// (POST /v1/reservations).
//
// The server returns 201 on a new reservation, but the generated
// CreateReservationResp parser only fills JSON200 on an exact 200 — a status-exact
// parse would DROP the 201 body. So the write routes through the raw ...WithBody
// call + readRawResponse + the 2xx success gate in writeJSON, which decodes any
// 2xx (including 201) into models.ReserveResponse. The tracer dedups on the
// TransactionID BODY field, so no X-Idempotency header is wired. The opaque
// openapi_types.File body forces the WithBody variant regardless.
func (f *reservationsFacade) Reserve(ctx context.Context, input *models.ReserveInput) (*models.ReserveResponse, error) {
	const operation = "Reservations.Reserve"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.ReserveResponse](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.tracer.CreateReservationWithBody(ctx, jsonContentType, body))
	})
}

// Confirm commits a single reservation by id (body-less POST, 200). Idempotent: a
// retry against an already-terminal reservation returns the same status.
func (f *reservationsFacade) Confirm(ctx context.Context, id string) (*models.ReservationActionResponse, error) {
	return reservationTransition[models.ReservationActionResponse](ctx, "Reservations.Confirm", f.tracer.ConfirmReservation, id)
}

// Release cancels a single reservation by id (body-less POST, 200). Idempotent.
func (f *reservationsFacade) Release(ctx context.Context, id string) (*models.ReservationActionResponse, error) {
	return reservationTransition[models.ReservationActionResponse](ctx, "Reservations.Release", f.tracer.ReleaseReservation, id)
}

// ConfirmByTransaction commits every RESERVED reservation a transaction holds
// (body-less POST, 200). Flipped==0 is a valid idempotent no-op success — it
// returns (resp, nil), never an error.
func (f *reservationsFacade) ConfirmByTransaction(ctx context.Context, transactionID string) (*models.TransactionActionResponse, error) {
	return reservationTransition[models.TransactionActionResponse](ctx, "Reservations.ConfirmByTransaction", f.tracer.ConfirmReservationByTransaction, transactionID)
}

// ReleaseByTransaction cancels every RESERVED reservation a transaction holds
// (body-less POST, 200). Flipped==0 is a valid idempotent no-op success.
func (f *reservationsFacade) ReleaseByTransaction(ctx context.Context, transactionID string) (*models.TransactionActionResponse, error) {
	return reservationTransition[models.TransactionActionResponse](ctx, "Reservations.ReleaseByTransaction", f.tracer.ReleaseReservationByTransaction, transactionID)
}

// reservationTransition runs a body-less lifecycle POST (by id or by transaction)
// through the raw call so success is decided on 2xx (never the status-exact
// generated parser) and the 200 body decodes into T. Threads transport errors and
// non-2xx problem+json through the shared decodeOne path.
func reservationTransition[T any](ctx context.Context, operation string, call func(context.Context, string, ...gentracer.RequestEditorFn) (*http.Response, error), id string) (*T, error) {
	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	resp, body, err := readRawResponse(call(ctx, id))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[T](operation, resp.StatusCode, body, resp)
}

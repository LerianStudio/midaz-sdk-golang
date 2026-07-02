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

// compositionFacade is the Epic 3.1 (Task 3.1.4) hand-written facade over the
// generated genledger.ClientWithResponses for the account+instrument
// composition endpoint. It opens a holder-owned account and, when the input
// carries any instrument field, its instrument in a single call, on the
// org+ledger+holder-in-path ledger-plane route
// POST /organizations/{org}/ledgers/{ledgerId}/holders/{id}/accounts.
//
// SUCCESS-GATE (the reason this bypasses the generated ...WithResponse parser):
// the OAS declares only a JSON200 body but the server returns HTTP 201 on
// success. The typed CreateHolderAccountWithResponse parser gates on the one
// declared status, so a real 201 falls to the default branch and is
// misclassified as an error (the active Phase 2 defect). Reading the RAW
// response via writeJSON + isSuccess(2xx) accepts BOTH 200 and 201.
//
// PARTIAL-FAILURE (the correctness crown jewel): the server may persist the
// account, fail the instrument, and STILL return HTTP 201 with
// {account, instrument:null, instrumentError:{status:FAILED,reason:...}}. There
// is no compensating delete — the account is real. That response is a SUCCESS:
// the facade returns (resp, nil) with InstrumentError populated; it MUST NEVER
// become a Go error. ONLY a transport error or a non-2xx status becomes
// *errors.Error.
//
// No idempotency is wired: the endpoint has no idempotency slot and the retrofit
// is deferred to Epic 5.1. The write stays 401-replay safe regardless via the
// rewindable *bytes.Reader body in writeJSON. Authorization is passed as nil so
// the client-level auth round tripper injects the Bearer token. The public
// surface stays models.* + *errors.Error; the generated types never leak.
type compositionFacade struct {
	ledger *genledger.ClientWithResponses
}

// newCompositionFacade wires the facade over a ledger plane client.
func newCompositionFacade(ledger *genledger.ClientWithResponses) *compositionFacade {
	return &compositionFacade{ledger: ledger}
}

// CreateHolderAccount opens a holder-owned account (and, when instrument fields
// are present, its instrument) in one call. On a 2xx it returns the composite
// response: a populated InstrumentError means the account committed but the
// instrument write failed (no rollback) — this is a success, not an error.
func (f *compositionFacade) CreateHolderAccount(ctx context.Context, orgID, ledgerID, holderID string, input *models.CreateHolderAccountInput) (*models.HolderAccountResponse, error) {
	const operation = "Composition.CreateHolderAccount"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	// Authorization is nil: the client-level auth editor injects the Bearer
	// token. No idempotency slot exists on this endpoint (Epic 5.1 retrofit).
	params := &genledger.CreateHolderAccountParams{}

	return writeJSON[models.HolderAccountResponse](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateHolderAccountWithBody(ctx, orgID, ledgerID, holderID, params, jsonContentType, body))
	})
}

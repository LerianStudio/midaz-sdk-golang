// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// billingCalculateFacade is the Epic 3.2 (Task 3.2.4) hand-written facade over the
// generated genledger.ClientWithResponses for period billing calculation. The
// public surface is exactly models.BillingCalculateInput ->
// models.BillingCalculateResponse + *errors.Error; the generated compound type
// (whose response body is opaque) never leaks.
//
// Money is money-adjacent here: every result's TotalNetAmount and the summary
// TotalNetAmount ride the wire as JSON strings and are modeled as string end to
// end (models.BillingCalculationResult / BillingCalculateSummary) — no float hop.
// TransactionPayload stays raw JSON so its nested decimal metadata never round-
// trips through a float.
//
// SUCCESS-GATE: the write routes through the RAW ...WithBody + readRawResponse +
// isSuccess(2xx), never the generated JSON200 wrapper (CalculateBillingResp gates
// JSON200 on the single status the OAS declares), so any 2xx — including a 201 —
// is a success and a null results array is a success, not an error.
type billingCalculateFacade struct {
	ledger *genledger.ClientWithResponses
}

// newBillingCalculateFacade wires the facade over a ledger plane client.
func newBillingCalculateFacade(ledger *genledger.ClientWithResponses) *billingCalculateFacade {
	return &billingCalculateFacade{ledger: ledger}
}

// CalculateBilling runs a billing calculation for a period under a LEDGER.
// Same write-facade pattern as the creates: a rewindable body so the auth round
// tripper can replay after a 401. A 2xx with null results (no packages matched)
// returns (resp, nil) with empty Results — never an error.
//
// SCOPE: billing calculation is ledger-scoped on the server
// (POST /v2/organizations/{org}/ledgers/{ledger}/billing/calculate). The
// removed /v1 route was organization-scoped; ledgerID is not optional.
//
// The ledger travels in the path AND in the body (the server schema requires
// ledgerId). An empty input.LedgerID inherits the path ledger; a different one is
// rejected — see [reconcileBodyLedgerID]. The caller's input is never mutated.
func (f *billingCalculateFacade) CalculateBilling(ctx context.Context, orgID, ledgerID string, input *models.BillingCalculateInput) (*models.BillingCalculateResponse, error) {
	const operation = "BillingCalculate.CalculateBilling"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	payload := input

	if input != nil {
		reconciled := *input
		if err := reconcileBodyLedgerID(operation, ledgerID, &reconciled.LedgerID); err != nil {
			return nil, err
		}

		payload = &reconciled
	}

	if err := payload.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.BillingCalculateResponse](ctx, operation, payload, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CalculateBillingV2WithBody(ctx, orgID, ledgerID, jsonContentType, body))
	})
}

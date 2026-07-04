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

// feeEstimateFacade is the Epic 3.2 (Task 3.2.2) hand-written facade over the
// generated genledger.ClientWithResponses for dry-run fee estimation. The public
// surface is exactly models.FeeEstimateInput -> models.FeeEstimateResponse +
// *errors.Error; the generated types (whose response body is opaque *[]byte)
// never leak.
//
// Money is money-adjacent here: the fee-adjusted send/amount values ride the wire
// as JSON strings and are modeled as string end to end (models.FeeAdjustedSend) —
// no float hop.
//
// SUCCESS shape: the estimate endpoint returns 2xx with either a populated
// feesApplied (rules matched) OR feesApplied:null (no fee/gratuity rules found).
// BOTH are successes — the null branch is a message-only result, NOT a Go error.
// Only a transport failure or a non-2xx status maps to *errors.Error. The write
// routes through the RAW ...WithBody + readRawResponse + isSuccess(2xx), never
// the generated JSON200 wrapper (which would reject the message-only response).
type feeEstimateFacade struct {
	ledger *genledger.ClientWithResponses
}

// newFeeEstimateFacade wires the facade over a ledger plane client.
func newFeeEstimateFacade(ledger *genledger.ClientWithResponses) *feeEstimateFacade {
	return &feeEstimateFacade{ledger: ledger}
}

// EstimateFee runs a dry-run fee estimation for a transaction under an
// organization. A 2xx with feesApplied:null (no rules matched) returns
// (resp, nil) with FeesApplied == nil — never an error. Same write-facade
// pattern as the creates: a rewindable body so the auth round tripper can replay
// after a 401.
func (f *feeEstimateFacade) EstimateFee(ctx context.Context, orgID string, input *models.FeeEstimateInput) (*models.FeeEstimateResponse, error) {
	const operation = "FeeEstimate.EstimateFee"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.FeeEstimateResponse](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.EstimateFeeCalculationWithBody(ctx, orgID, jsonContentType, body))
	})
}

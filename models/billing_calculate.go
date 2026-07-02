// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation"
)

// BillingCalculateInput is the billing-calculation request. It mirrors the server
// DTO (feeshared/model/billing_calculation.go BillingCalculateRequest):
// OrganizationID travels as a path segment (not in the body), LedgerID and Period
// are required, and Type is optional. When Type is empty both "volume" and
// "maintenance" packages are calculated; otherwise it restricts the calculation
// to the named type.
//
// Period must be "YYYY-MM" (monthly), "YYYY-Www" (weekly), or "YYYY-MM-DD"
// (daily), e.g. "2026-01", "2026-W13", or "2026-01-15".
type BillingCalculateInput struct {
	LedgerID string `json:"ledgerId"`
	Period   string `json:"period"`
	Type     string `json:"type,omitempty"`
}

// Validate enforces the SDK-side preconditions the server requires: LedgerID and
// Period are required. Period format is validated server-side.
func (input *BillingCalculateInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	if strings.TrimSpace(input.LedgerID) == "" {
		errs.Append("ledgerId", "is required")
	}

	if strings.TrimSpace(input.Period) == "" {
		errs.Append("period", "is required")
	}

	return errs.OrNil()
}

// BillingCalculateResponse is the compound billing-calculation result. It mirrors
// the generated genledger.FeeBillingCalculateResponse: a per-package Results slice
// and a Summary. Results is empty when no packages matched the period.
type BillingCalculateResponse struct {
	Results []BillingCalculationResult `json:"results"`
	Summary BillingCalculateSummary    `json:"summary"`
}

// BillingCalculationResult is the billing outcome for a single billing package
// within the requested period. It mirrors the server DTO
// (billing_calculation.go BillingCalculationResult).
//
// Money is string, never float: TotalNetAmount rides the wire as a JSON string
// (the server marshals a decimal.Decimal with swaggertype:"string") and is
// modeled as string so a value like 0.333333333333333333 survives with no
// precision loss. TransactionPayload is the fee-engine transaction payload kept
// as raw JSON so its nested decimal metadata (unitPrice, discountAmount, ...) is
// never round-tripped through a float.
type BillingCalculationResult struct {
	BillingPackageID    string          `json:"billingPackageId"`
	BillingPackageLabel string          `json:"billingPackageLabel"`
	BillingType         string          `json:"billingType"`
	Period              string          `json:"period"`
	TotalAccounts       int64           `json:"totalAccounts"`
	TotalCharged        int64           `json:"totalCharged"`
	TotalSkipped        int64           `json:"totalSkipped"`
	TotalNetAmount      string          `json:"totalNetAmount"`
	TransactionPayload  json.RawMessage `json:"transactionPayload,omitempty"`
}

// BillingCalculateSummary aggregates the calculation across every result. It
// mirrors the server DTO (billing_calculation.go). TotalNetAmount is string
// (never float) — the money third rail.
type BillingCalculateSummary struct {
	TotalResults     int64  `json:"totalResults"`
	TotalVolume      int64  `json:"totalVolume"`
	TotalMaintenance int64  `json:"totalMaintenance"`
	TotalNetAmount   string `json:"totalNetAmount"`
}

// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"errors"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/validation"
)

// FeeEstimateInput is the dry-run fee-estimate request. It mirrors the server
// DTO (components/ledger/pkg/feeshared/model/fees.go FeeEstimate): PackageID and
// LedgerID are required UUIDs and Transaction is the full transaction the fee
// engine estimates against, rendered on the wire as the transaction-input shape.
type FeeEstimateInput struct {
	PackageID   string                      `json:"packageId"`
	LedgerID    string                      `json:"ledgerId"`
	Transaction FeeEstimateTransactionInput `json:"transaction"`
}

// FeeEstimateTransactionInput is the transaction body the fee engine estimates
// against. It reuses the Phase 2 transaction request leg types (SendInput /
// SourceInput / DistributeInput / FromToInput) so the wire shape matches a real
// transaction create byte-for-byte.
type FeeEstimateTransactionInput struct {
	ChartOfAccountsGroupName string         `json:"chartOfAccountsGroupName,omitempty"`
	Description              string         `json:"description,omitempty"`
	Code                     string         `json:"code,omitempty"`
	Pending                  bool           `json:"pending,omitempty"`
	Metadata                 map[string]any `json:"metadata,omitempty"`
	RouteID                  *string        `json:"routeId,omitempty"`
	TransactionDate          string         `json:"transactionDate,omitempty"`
	Send                     *SendInput     `json:"send"`
}

// Validate enforces SDK-side preconditions on the estimate request: the required
// package and ledger IDs, and a valid send leg.
func (input *FeeEstimateInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	if strings.TrimSpace(input.PackageID) == "" {
		errs.Append("packageId", "is required")
	}

	if strings.TrimSpace(input.LedgerID) == "" {
		errs.Append("ledgerId", "is required")
	}

	if input.Transaction.Send == nil {
		errs.Append("transaction.send", "is required")
	} else if err := input.Transaction.Send.Validate(); err != nil {
		errs.Append("transaction.send", "invalid: "+err.Error())
	}

	return errs.OrNil()
}

// NewFeeEstimateInput builds a FeeEstimateInput with the required package ID,
// ledger ID, and the send leg the fee engine estimates against. Optional
// transaction fields are set with the With* methods.
func NewFeeEstimateInput(packageID, ledgerID string, send *SendInput) *FeeEstimateInput {
	return &FeeEstimateInput{
		PackageID:   packageID,
		LedgerID:    ledgerID,
		Transaction: FeeEstimateTransactionInput{Send: send},
	}
}

// WithChartOfAccountsGroupName sets the estimate transaction's chart-of-accounts group name.
func (input *FeeEstimateInput) WithChartOfAccountsGroupName(name string) *FeeEstimateInput {
	if input == nil {
		return nil
	}

	input.Transaction.ChartOfAccountsGroupName = name

	return input
}

// WithDescription sets the estimate transaction's description.
func (input *FeeEstimateInput) WithDescription(description string) *FeeEstimateInput {
	if input == nil {
		return nil
	}

	input.Transaction.Description = description

	return input
}

// WithCode sets the estimate transaction's code.
func (input *FeeEstimateInput) WithCode(code string) *FeeEstimateInput {
	if input == nil {
		return nil
	}

	input.Transaction.Code = code

	return input
}

// WithPending sets the estimate transaction's pending flag.
func (input *FeeEstimateInput) WithPending(pending bool) *FeeEstimateInput {
	if input == nil {
		return nil
	}

	input.Transaction.Pending = pending

	return input
}

// WithMetadata sets the estimate transaction's metadata.
func (input *FeeEstimateInput) WithMetadata(metadata map[string]any) *FeeEstimateInput {
	if input == nil {
		return nil
	}

	input.Transaction.Metadata = metadata

	return input
}

// WithRouteID sets the estimate transaction's route ID.
func (input *FeeEstimateInput) WithRouteID(routeID string) *FeeEstimateInput {
	if input == nil {
		return nil
	}

	input.Transaction.RouteID = &routeID

	return input
}

// WithTransactionDate sets the estimate transaction's date (RFC3339).
func (input *FeeEstimateInput) WithTransactionDate(date string) *FeeEstimateInput {
	if input == nil {
		return nil
	}

	input.Transaction.TransactionDate = date

	return input
}

// FeeEstimateResponse is the dry-run fee-estimate response. It mirrors the server
// DTO (fees.go FeeEstimateResponse): Message always carries a human-readable
// outcome, and FeesApplied is nil when no fee or gratuity rules matched (a valid
// success — not an error).
type FeeEstimateResponse struct {
	Message     string             `json:"message"`
	FeesApplied *FeeEstimateResult `json:"feesApplied"`
}

// FeeEstimateResult is the fee-adjusted projection returned when rules matched.
// It mirrors the server DTO (fee_estimate_result.go FeeEstimateResult).
type FeeEstimateResult struct {
	LedgerID    string                 `json:"ledgerId"`
	SegmentID   *string                `json:"segmentId,omitempty"`
	Transaction FeeAdjustedTransaction `json:"transaction"`
}

// FeeAdjustedTransaction is the fee-adjusted transaction returned by the estimate
// endpoint. It mirrors the server DTO (fee_estimate_result.go). Money is string,
// never float: the fee-adjusted send/amount values ride the wire as JSON strings
// and are modeled as string on read via FeeAdjustedSend so a value like
// 0.333333333333333333 survives with no precision loss.
type FeeAdjustedTransaction struct {
	ChartOfAccountsGroupName string           `json:"chartOfAccountsGroupName,omitempty"`
	Description              string           `json:"description,omitempty"`
	Code                     string           `json:"code,omitempty"`
	Pending                  bool             `json:"pending,omitempty"`
	Metadata                 map[string]any   `json:"metadata,omitempty"`
	RouteID                  *string          `json:"routeId,omitempty"`
	TransactionDate          *string          `json:"transactionDate,omitempty"`
	Send                     *FeeAdjustedSend `json:"send"`
}

// FeeAdjustedSend is the read-side send leg of a fee-adjusted transaction. Its
// Value fields are string (never any/float) to lock the money third rail: the
// server marshals decimal amounts as JSON strings and any float hop would
// silently drift on values like 1/3.
type FeeAdjustedSend struct {
	Asset      string                `json:"asset"`
	Value      string                `json:"value"`
	Source     FeeAdjustedSource     `json:"source"`
	Distribute FeeAdjustedDistribute `json:"distribute"`
}

// FeeAdjustedSource is the read-side source leg of a fee-adjusted transaction.
type FeeAdjustedSource struct {
	Remaining string              `json:"remaining,omitempty"`
	From      []FeeAdjustedFromTo `json:"from"`
}

// FeeAdjustedDistribute is the read-side distribute leg of a fee-adjusted transaction.
type FeeAdjustedDistribute struct {
	Remaining string              `json:"remaining,omitempty"`
	To        []FeeAdjustedFromTo `json:"to"`
}

// FeeAdjustedFromTo is one read-side source/destination entry of a fee-adjusted
// transaction. Amount.Value is string (never float) — the money third rail.
//
// AccountAlias mirrors the leg identity the fee engine returns: its response
// projects the same DTO the request carries (feeshared/model.FeeEstimateResult
// over pkg/mtransaction.Transaction, whose FromTo exposes accountAlias only), so
// there is no account key to read on either side.
type FeeAdjustedFromTo struct {
	AccountAlias string            `json:"accountAlias,omitempty"`
	Amount       FeeAdjustedAmount `json:"amount"`
	Share        *Share            `json:"share,omitempty"`
	Remaining    string            `json:"remaining,omitempty"`
	Rate         *Rate             `json:"rate,omitempty"`
	Description  string            `json:"description,omitempty"`
	Metadata     map[string]any    `json:"metadata,omitempty"`
	RouteID      *string           `json:"routeId,omitempty"`
}

// FeeAdjustedAmount is a read-side amount. Value is string (never float) so a
// fee-adjusted amount survives the wire with no precision loss.
type FeeAdjustedAmount struct {
	Asset string `json:"asset"`
	Value string `json:"value"`
}

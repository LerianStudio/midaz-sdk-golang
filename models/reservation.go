// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/validation"
	"github.com/shopspring/decimal"
)

// ReserveInput is the SDK-native payload for the two-phase reserve
// (POST /v1/reservations). It carries the same transaction-shape fields as
// models.ValidateTransactionInput — reusing the shared party contexts verbatim —
// plus the ledger TransactionID (the correlation handle the confirm/release
// lifecycle is keyed on) and the optional LongLived TTL selector.
//
// The tracer dedups on the TransactionID BODY field (never the X-Idempotency
// header), so a retried reserve with the same TransactionID replays the prior
// handle. Amount is exact decimal money, NEVER float — it encodes as a quoted
// string on the wire.
//
// Validate is RELAXED (mirrors the server's ValidateForReserve): TransactionID,
// RequestID, Amount, Asset and TransactionTimestamp are required, but
// TransactionType and Account are optional — the ledger cannot always supply a
// rail type or an internal account UUID at the reserve anchor.
type ReserveInput struct {
	// TransactionID is the ledger transaction correlation id (UUID). Required.
	TransactionID string `json:"transactionId"`

	// RequestID is the caller-supplied idempotency key (UUID). Required.
	RequestID string `json:"requestId"`

	// Amount is the transaction amount as exact decimal money. Required, > 0.
	Amount decimal.Decimal `json:"amount"`

	// Asset is the ISO-4217 asset code. Required, exactly 3 chars.
	Asset string `json:"asset"`

	// TransactionTimestamp is the transaction time (RFC3339). Required; the
	// server enforces a window (≤ now+skew, ≥ now-24h).
	TransactionTimestamp string `json:"transactionTimestamp"`

	// LongLived selects the reservation lifetime: false (default, a direct
	// transaction) gets the short reaper-swept TTL; true (a PENDING transaction)
	// gets the long-lived TTL.
	LongLived bool `json:"longLived,omitempty"`

	// Account is the optional account party (relaxed — not required for reserve).
	Account *AccountContext `json:"account,omitempty"`

	// Segment is the optional segment party.
	Segment *SegmentContext `json:"segment,omitempty"`

	// Portfolio is the optional portfolio party.
	Portfolio *PortfolioContext `json:"portfolio,omitempty"`

	// Merchant is the optional merchant party.
	Merchant *MerchantContext `json:"merchant,omitempty"`

	// TransactionType is the optional transaction type (e.g. CARD, WIRE, PIX).
	TransactionType *string `json:"transactionType,omitempty"`

	// SubType is the optional transaction sub-type.
	SubType *string `json:"subType,omitempty"`

	// Metadata is optional free-form context attached to the request.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewReserveInput builds a reserve payload with the required fields. Account and
// TransactionType are optional (relaxed reserve rules) and set via the builders.
func NewReserveInput(transactionID, requestID string, amount decimal.Decimal, asset, transactionTimestamp string) *ReserveInput {
	return &ReserveInput{
		TransactionID:        transactionID,
		RequestID:            requestID,
		Amount:               amount,
		Asset:                asset,
		TransactionTimestamp: transactionTimestamp,
	}
}

// WithLongLived sets the long-lived TTL selector (true for a PENDING transaction).
func (input *ReserveInput) WithLongLived(longLived bool) *ReserveInput {
	if input == nil {
		return nil
	}

	input.LongLived = longLived

	return input
}

// WithAccount sets the optional account context.
func (input *ReserveInput) WithAccount(account AccountContext) *ReserveInput {
	if input == nil {
		return nil
	}

	input.Account = &account

	return input
}

// WithSegment sets the optional segment context.
func (input *ReserveInput) WithSegment(segment SegmentContext) *ReserveInput {
	if input == nil {
		return nil
	}

	input.Segment = &segment

	return input
}

// WithPortfolio sets the optional portfolio context.
func (input *ReserveInput) WithPortfolio(portfolio PortfolioContext) *ReserveInput {
	if input == nil {
		return nil
	}

	input.Portfolio = &portfolio

	return input
}

// WithMerchant sets the optional merchant context.
func (input *ReserveInput) WithMerchant(merchant MerchantContext) *ReserveInput {
	if input == nil {
		return nil
	}

	input.Merchant = &merchant

	return input
}

// WithTransactionType sets the optional transaction type.
func (input *ReserveInput) WithTransactionType(transactionType string) *ReserveInput {
	if input == nil {
		return nil
	}

	input.TransactionType = &transactionType

	return input
}

// WithSubType sets the optional transaction sub-type.
func (input *ReserveInput) WithSubType(subType string) *ReserveInput {
	if input == nil {
		return nil
	}

	input.SubType = &subType

	return input
}

// WithMetadata sets the optional request metadata.
func (input *ReserveInput) WithMetadata(metadata map[string]any) *ReserveInput {
	if input == nil {
		return nil
	}

	input.Metadata = metadata

	return input
}

// Validate enforces the RELAXED reserve preconditions (mirrors the server's
// ValidateForReserve): TransactionID non-empty, RequestID non-empty, Amount
// strictly positive, Asset exactly 3 chars, TransactionTimestamp non-empty.
// TransactionType and Account are intentionally NOT required.
func (input *ReserveInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	if strings.TrimSpace(input.TransactionID) == "" {
		errs.Append("transactionId", "is required")
	}

	if strings.TrimSpace(input.RequestID) == "" {
		errs.Append("requestId", "is required")
	}

	if !input.Amount.GreaterThan(decimal.Zero) {
		errs.Append("amount", "must be greater than zero")
	}

	if len(input.Asset) != assetCodeLength {
		errs.Append("asset", fmt.Sprintf("must be exactly %d characters", assetCodeLength))
	}

	if strings.TrimSpace(input.TransactionTimestamp) == "" {
		errs.Append("transactionTimestamp", "is required")
	}

	return errs.OrNil()
}

// ReserveResponse is the handle returned on a successful reserve. When Denied is
// true the limit was exceeded and no capacity was held (ReservationIDs empty);
// otherwise ReservationIDs holds one id per counter-backed limit the ledger must
// later confirm or release. Mirrors the generated gentracer.ReserveResponse with
// UUIDs as plain strings.
type ReserveResponse struct {
	// TransactionID echoes the ledger transaction correlation id (UUID).
	TransactionID string `json:"transactionId"`

	// Denied reports the limit-exceeded decision (no capacity held).
	Denied bool `json:"denied"`

	// ReservationIDs are the per-limit reservation handles (UUID strings).
	ReservationIDs []string `json:"reservationIds"`
}

// ReservationActionResponse is the body returned by confirm and release BY ID.
// Status is the terminal state (CONFIRMED or RELEASED). Both are idempotent: a
// retry against an already-terminal reservation returns the same status with 200.
type ReservationActionResponse struct {
	// ReservationID is the reservation the action resolved (UUID).
	ReservationID string `json:"reservationId"`

	// Status is the terminal state (CONFIRMED, RELEASED).
	Status string `json:"status"`
}

// TransactionActionResponse is the body returned by confirm and release BY
// TRANSACTION. The tracer flips every RESERVED reservation the transaction holds
// and reports how many were transitioned. Flipped==0 is a VALID idempotent no-op
// success (the transaction never reserved, or every reservation was already
// terminal) — never an error.
type TransactionActionResponse struct {
	// TransactionID echoes the ledger transaction correlation id (UUID).
	TransactionID string `json:"transactionId"`

	// Status is the terminal state (CONFIRMED, RELEASED).
	Status string `json:"status"`

	// Flipped is the count of reservations transitioned. Zero is a valid no-op.
	Flipped int `json:"flipped"`
}

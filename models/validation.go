// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/validation"
	"github.com/shopspring/decimal"
)

// Validation decision enum. The tracer's evaluation verdict for a transaction.
// The server is the sole authority on these values; the SDK exposes them for
// callers to compare against and to pass as a list filter.
const (
	DecisionAllow  = "ALLOW"
	DecisionDeny   = "DENY"
	DecisionReview = "REVIEW"
)

// ValidateTransactionInput is the SDK-native payload for evaluating a
// transaction against the tracer's active rules and limits (POST /v1/validations).
//
// RequestID is the caller-supplied idempotency key: the tracer dedups on this
// BODY field (not the X-Idempotency header), so a repeated RequestID replays the
// prior verdict (server returns 200) instead of re-evaluating (201). Amount is
// exact decimal money, NEVER float — it encodes as a quoted string on the wire.
type ValidateTransactionInput struct {
	// RequestID is the caller-supplied idempotency key (UUID). Required.
	RequestID string `json:"requestId"`

	// Amount is the transaction amount as exact decimal money. Required, > 0.
	Amount decimal.Decimal `json:"amount"`

	// Currency is the ISO-4217 currency code. Required, exactly 3 chars.
	Currency string `json:"currency"`

	// TransactionTimestamp is the transaction time (RFC3339). Required; the
	// server enforces a window (≤ now+skew, ≥ now-24h).
	TransactionTimestamp string `json:"transactionTimestamp"`

	// Account is the account party. Required.
	Account AccountContext `json:"account"`

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

// NewValidateTransactionInput builds a validation payload with the required
// fields. Amount must be strictly positive and Currency exactly 3 characters —
// Validate enforces both before the round trip.
func NewValidateTransactionInput(requestID string, amount decimal.Decimal, currency, transactionTimestamp string, account AccountContext) *ValidateTransactionInput {
	return &ValidateTransactionInput{
		RequestID:            requestID,
		Amount:               amount,
		Currency:             currency,
		TransactionTimestamp: transactionTimestamp,
		Account:              account,
	}
}

// WithSegment sets the optional segment context.
func (input *ValidateTransactionInput) WithSegment(segment SegmentContext) *ValidateTransactionInput {
	if input == nil {
		return nil
	}

	input.Segment = &segment

	return input
}

// WithPortfolio sets the optional portfolio context.
func (input *ValidateTransactionInput) WithPortfolio(portfolio PortfolioContext) *ValidateTransactionInput {
	if input == nil {
		return nil
	}

	input.Portfolio = &portfolio

	return input
}

// WithMerchant sets the optional merchant context.
func (input *ValidateTransactionInput) WithMerchant(merchant MerchantContext) *ValidateTransactionInput {
	if input == nil {
		return nil
	}

	input.Merchant = &merchant

	return input
}

// WithTransactionType sets the optional transaction type.
func (input *ValidateTransactionInput) WithTransactionType(transactionType string) *ValidateTransactionInput {
	if input == nil {
		return nil
	}

	input.TransactionType = &transactionType

	return input
}

// WithSubType sets the optional transaction sub-type.
func (input *ValidateTransactionInput) WithSubType(subType string) *ValidateTransactionInput {
	if input == nil {
		return nil
	}

	input.SubType = &subType

	return input
}

// WithMetadata sets the optional request metadata.
func (input *ValidateTransactionInput) WithMetadata(metadata map[string]any) *ValidateTransactionInput {
	if input == nil {
		return nil
	}

	input.Metadata = metadata

	return input
}

// Validate enforces SDK-side preconditions before the round trip: RequestID
// non-empty, Amount strictly positive, Currency exactly 3 chars,
// TransactionTimestamp non-empty, and an Account present (non-empty AccountID).
func (input *ValidateTransactionInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	if strings.TrimSpace(input.RequestID) == "" {
		errs.Append("requestId", "is required")
	}

	if !input.Amount.GreaterThan(decimal.Zero) {
		errs.Append("amount", "must be greater than zero")
	}

	if len(input.Currency) != currencyCodeLength {
		errs.Append("currency", fmt.Sprintf("must be exactly %d characters", currencyCodeLength))
	}

	if strings.TrimSpace(input.TransactionTimestamp) == "" {
		errs.Append("transactionTimestamp", "is required")
	}

	if strings.TrimSpace(input.Account.AccountID) == "" {
		errs.Append("account", "is required")
	}

	return errs.OrNil()
}

// ValidationResponse is the tracer's verdict for a just-evaluated transaction
// (the POST /v1/validations body). Decision is one of DecisionAllow/Deny/Review.
// ProcessingTimeMs is a display float (never money); the money lives in the
// per-limit LimitUsageDetail triple.
type ValidationResponse struct {
	// Decision is the verdict (ALLOW, DENY, REVIEW).
	Decision string `json:"decision"`

	// Reason is the human-readable explanation for the decision.
	Reason string `json:"reason"`

	// MatchedRuleIDs are the rules that matched (UUID strings).
	MatchedRuleIDs []string `json:"matchedRuleIds"`

	// EvaluatedRuleIDs are all rules evaluated (UUID strings).
	EvaluatedRuleIDs []string `json:"evaluatedRuleIds"`

	// LimitUsageDetails is the per-limit usage breakdown (money is decimal).
	LimitUsageDetails []LimitUsageDetail `json:"limitUsageDetails"`

	// ProcessingTimeMs is the evaluation duration in milliseconds (display float).
	ProcessingTimeMs float64 `json:"processingTimeMs"`

	// TotalRulesLoaded is the number of rules considered.
	TotalRulesLoaded int `json:"totalRulesLoaded"`

	// Truncated reports whether the rule/limit set was truncated for cost.
	Truncated bool `json:"truncated"`

	// ValidationID is the server-issued validation record identity (UUID).
	ValidationID string `json:"validationId"`

	// RequestID echoes the caller-supplied idempotency key (UUID).
	RequestID string `json:"requestId"`

	// EvaluatedAt is when the evaluation ran.
	EvaluatedAt time.Time `json:"evaluatedAt"`
}

// TransactionValidation is the full stored validation record returned by Get. It
// carries the party contexts and the decimal Amount the verdict was computed
// against. Mirrors the generated gentracer.TransactionValidation with money as
// decimal.Decimal.
type TransactionValidation struct {
	// ValidationID is the record identity (UUID).
	ValidationID string `json:"validationId"`

	// RequestID is the caller-supplied idempotency key (UUID).
	RequestID string `json:"requestId"`

	// Decision is the verdict (ALLOW, DENY, REVIEW).
	Decision string `json:"decision"`

	// Reason is the human-readable explanation for the decision.
	Reason string `json:"reason"`

	// Amount is the evaluated transaction amount as exact decimal money.
	Amount decimal.Decimal `json:"amount"`

	// Currency is the ISO-4217 currency code.
	Currency string `json:"currency"`

	// TransactionType is the transaction type (e.g. CARD, WIRE, PIX).
	TransactionType string `json:"transactionType"`

	// SubType is the optional transaction sub-type.
	SubType *string `json:"subType,omitempty"`

	// Account is the account party.
	Account AccountContext `json:"account"`

	// Segment is the optional segment party.
	Segment *SegmentContext `json:"segment,omitempty"`

	// Portfolio is the optional portfolio party.
	Portfolio *PortfolioContext `json:"portfolio,omitempty"`

	// Merchant is the optional merchant party.
	Merchant *MerchantContext `json:"merchant,omitempty"`

	// MatchedRuleIDs are the rules that matched (UUID strings).
	MatchedRuleIDs []string `json:"matchedRuleIds"`

	// EvaluatedRuleIDs are all rules evaluated (UUID strings).
	EvaluatedRuleIDs []string `json:"evaluatedRuleIds"`

	// LimitUsageDetails is the per-limit usage breakdown (money is decimal).
	LimitUsageDetails []LimitUsageDetail `json:"limitUsageDetails"`

	// ProcessingTimeMs is the evaluation duration in milliseconds (display float).
	ProcessingTimeMs float64 `json:"processingTimeMs"`

	// TotalRulesLoaded is the number of rules considered.
	TotalRulesLoaded int `json:"totalRulesLoaded"`

	// Truncated reports whether the rule/limit set was truncated for cost.
	Truncated bool `json:"truncated"`

	// Metadata is optional free-form context stored with the record.
	Metadata map[string]any `json:"metadata,omitempty"`

	// TransactionTimestamp is the transaction time the verdict applied to.
	TransactionTimestamp time.Time `json:"transactionTimestamp"`

	// CreatedAt is when the record was stored.
	CreatedAt time.Time `json:"createdAt"`
}

// ValidationSummary is one item in the validations cursor list: a compact stored
// verdict with the account/segment/portfolio identity as bare UUID strings and
// the decimal Amount. Mirrors the generated gentracer.ValidationSummary.
type ValidationSummary struct {
	// ValidationID is the record identity (UUID).
	ValidationID string `json:"validationId"`

	// AccountID is the account the verdict applied to (UUID).
	AccountID string `json:"accountId"`

	// SegmentID is the optional segment the verdict applied to (UUID).
	SegmentID *string `json:"segmentId,omitempty"`

	// PortfolioID is the optional portfolio the verdict applied to (UUID).
	PortfolioID *string `json:"portfolioId,omitempty"`

	// Amount is the evaluated transaction amount as exact decimal money.
	Amount decimal.Decimal `json:"amount"`

	// Currency is the ISO-4217 currency code.
	Currency string `json:"currency"`

	// Decision is the verdict (ALLOW, DENY, REVIEW).
	Decision string `json:"decision"`

	// Reason is the human-readable explanation for the decision.
	Reason string `json:"reason"`

	// TransactionType is the transaction type (e.g. CARD, WIRE, PIX).
	TransactionType string `json:"transactionType"`

	// MatchedRuleIDs are the rules that matched (UUID strings).
	MatchedRuleIDs []string `json:"matchedRuleIds"`

	// ExceededLimitIDs are the limits the transaction exceeded (UUID strings).
	ExceededLimitIDs []string `json:"exceededLimitIds"`

	// ProcessingTimeMs is the evaluation duration in milliseconds (display float).
	ProcessingTimeMs float64 `json:"processingTimeMs"`

	// CreatedAt is when the record was stored.
	CreatedAt time.Time `json:"createdAt"`
}

// LimitUsageDetail is one limit's usage against a transaction. LimitAmount,
// CurrentUsage, and AttemptedAmount are exact decimal money (quoted strings on
// the wire), NEVER float. Exceeded/Skipped are the per-limit outcome flags.
type LimitUsageDetail struct {
	// LimitID is the limit this detail describes (UUID).
	LimitID string `json:"limitId"`

	// LimitAmount is the limit ceiling as exact decimal money.
	LimitAmount decimal.Decimal `json:"limitAmount"`

	// CurrentUsage is the usage already accrued in the window (decimal money).
	CurrentUsage decimal.Decimal `json:"currentUsage"`

	// AttemptedAmount is the amount this transaction would add (decimal money).
	AttemptedAmount decimal.Decimal `json:"attemptedAmount"`

	// Exceeded reports whether this limit would be breached.
	Exceeded bool `json:"exceeded"`

	// Period is the accrual window (server-defined).
	Period string `json:"period"`

	// Scope is the matched scope descriptor (server-defined).
	Scope string `json:"scope"`

	// SkipReason explains why the limit was skipped, when applicable.
	SkipReason *string `json:"skipReason,omitempty"`

	// Skipped reports whether the limit was skipped during evaluation.
	Skipped *bool `json:"skipped,omitempty"`
}

// ValidationsListOpts is the typed options struct for the validations cursor
// list. It embeds CursorListOpts for the shared cursor/sort-order/date fields,
// adds SortBy, and attaches a typed Filters sub-struct.
//
// Unlike the rules and limits lists, this endpoint HAS native start_date/end_date
// query slots, so date filtering IS supported. But the tracer server strict-parses
// those dates as RFC3339 — diverging from the ledger plane's YYYY-MM-DD — so
// Validate uses ValidateCursorListOptsRFC3339Dates (not the plain
// ValidateCursorListOpts, and not the NoDates variant). Callers must pass RFC3339
// (e.g. 2026-01-01T00:00:00Z), never a bare date.
type ValidationsListOpts struct {
	CursorListOpts

	// SortBy names the sort field (created_at, processing_time_ms). Empty falls
	// back to the server default. Passed through verbatim.
	SortBy string

	// Filters narrows the result set. Zero value means no narrowing.
	Filters ValidationsFilters
}

// ValidationsFilters is the typed filter set for the validations endpoint. Each
// field maps to a native query-param slot on the generated ListValidationsParams.
type ValidationsFilters struct {
	Decision        string
	AccountID       string
	MatchedRuleID   string
	ExceededLimitID string
	SegmentID       string
	PortfolioID     string
	TransactionType string
}

// Validate enforces the shared cursor-list preconditions (limit bounds, sort
// direction, date range). Date filtering IS supported here — the generated
// ListValidationsParams has native start_date/end_date slots — but the tracer
// server strict-parses those as RFC3339, so this defers to
// ValidateCursorListOptsRFC3339Dates (diverging from the ledger plane's
// YYYY-MM-DD). Filter values are passed through; the server validates them.
func (o ValidationsListOpts) Validate() error {
	return ValidateCursorListOptsRFC3339Dates("ValidationsListOpts.Validate", o.CursorListOpts)
}

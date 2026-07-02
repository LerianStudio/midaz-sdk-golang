// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation"
	"github.com/shopspring/decimal"
)

// Limit type enum. A limit's LimitType selects the accrual window the server
// resets usage on. The server is the sole authority; the SDK validates against
// this closed set to fail fast before the round trip.
const (
	LimitTypeDaily          = "DAILY"
	LimitTypeWeekly         = "WEEKLY"
	LimitTypeMonthly        = "MONTHLY"
	LimitTypePerTransaction = "PER_TRANSACTION"
	LimitTypeCustom         = "CUSTOM"
)

const (
	maxLimitNameLength        = 255
	maxLimitDescriptionLength = 1000
	maxLimitScopes            = 100
	currencyCodeLength        = 3
)

// Limit is the SDK-native tracer limit: a MaxAmount ceiling in a Currency,
// accrued over a LimitType window and scoped by one or more Scopes, with a
// DRAFT → ACTIVE → INACTIVE lifecycle. The wire shape mirrors the generated
// gentracer.Limit (camelCase tags, limitId as the identity field).
//
// MaxAmount is a shopspring/decimal.Decimal, NEVER a float: money must never
// round-trip through binary floating point. The wire encodes it as a quoted
// string and shopspring's default marshal is string-quoted, so the value
// survives create → decode at full precision.
type Limit struct {
	// ID is the server-issued limit identity (UUID).
	ID string `json:"limitId"`

	// Name is the human-readable limit name.
	Name string `json:"name"`

	// Description is an optional free-form description.
	Description *string `json:"description,omitempty"`

	// LimitType is the accrual window (DAILY, WEEKLY, MONTHLY, PER_TRANSACTION,
	// CUSTOM). Immutable after create — see UpdateLimitInput.
	LimitType string `json:"limitType"`

	// MaxAmount is the ceiling, as exact decimal money (never float).
	MaxAmount decimal.Decimal `json:"maxAmount"`

	// Currency is the ISO-4217 currency code. Immutable after create.
	Currency string `json:"currency"`

	// Scopes narrows the transactions this limit applies to (1-100 entries).
	Scopes []Scope `json:"scopes,omitempty"`

	// Status is the lifecycle state (DRAFT, ACTIVE, INACTIVE).
	Status string `json:"status"`

	// ActiveTimeStart bounds the daily active window start (HH:MM, optional).
	ActiveTimeStart *string `json:"activeTimeStart,omitempty"`

	// ActiveTimeEnd bounds the daily active window end (HH:MM, optional).
	ActiveTimeEnd *string `json:"activeTimeEnd,omitempty"`

	// CustomStartDate is the CUSTOM-window lower bound (optional).
	CustomStartDate *time.Time `json:"customStartDate,omitempty"`

	// CustomEndDate is the CUSTOM-window upper bound (optional).
	CustomEndDate *time.Time `json:"customEndDate,omitempty"`

	// ResetAt is when the current accrual window next resets.
	ResetAt *time.Time `json:"resetAt,omitempty"`

	// CreatedAt is the creation timestamp.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is the last-update timestamp.
	UpdatedAt time.Time `json:"updatedAt"`
}

// UsageSnapshot is the point-in-time usage of a limit: how much of MaxAmount is
// consumed in the current window. CurrentUsage and LimitAmount are exact decimal
// money; UtilizationPercent is a display ratio (CurrentUsage/LimitAmount) and is
// a float64 by design — it is never used to move money.
type UsageSnapshot struct {
	// LimitID is the limit this snapshot describes (UUID).
	LimitID string `json:"limitId"`

	// CurrentUsage is the amount consumed in the current window (decimal money).
	CurrentUsage decimal.Decimal `json:"currentUsage"`

	// LimitAmount is the ceiling for the current window (decimal money).
	LimitAmount decimal.Decimal `json:"limitAmount"`

	// UtilizationPercent is the display ratio CurrentUsage/LimitAmount. Float64
	// is fine: it is a presentation value, never a money path.
	UtilizationPercent float64 `json:"utilizationPercent"`

	// NearLimit flags that usage is close to the ceiling.
	NearLimit bool `json:"nearLimit"`

	// ResetAt is when the current window resets.
	ResetAt *time.Time `json:"resetAt,omitempty"`
}

// CreateLimitInput is the SDK-native limit creation payload. Name, LimitType,
// MaxAmount, Currency, and at least one Scope are required. The date fields are
// wire strings on input (the server parses them).
type CreateLimitInput struct {
	Name            string          `json:"name"`
	Description     *string         `json:"description,omitempty"`
	LimitType       string          `json:"limitType"`
	MaxAmount       decimal.Decimal `json:"maxAmount"`
	Currency        string          `json:"currency"`
	Scopes          []Scope         `json:"scopes,omitempty"`
	ActiveTimeStart *string         `json:"activeTimeStart,omitempty"`
	ActiveTimeEnd   *string         `json:"activeTimeEnd,omitempty"`
	CustomStartDate *string         `json:"customStartDate,omitempty"`
	CustomEndDate   *string         `json:"customEndDate,omitempty"`
}

// NewCreateLimitInput builds a limit creation payload with the required fields.
func NewCreateLimitInput(name, limitType string, maxAmount decimal.Decimal, currency string) *CreateLimitInput {
	return &CreateLimitInput{
		Name:      name,
		LimitType: limitType,
		MaxAmount: maxAmount,
		Currency:  currency,
	}
}

// WithDescription sets the optional description.
func (input *CreateLimitInput) WithDescription(description string) *CreateLimitInput {
	if input == nil {
		return nil
	}

	input.Description = &description

	return input
}

// WithScopes replaces the scope set.
func (input *CreateLimitInput) WithScopes(scopes []Scope) *CreateLimitInput {
	if input == nil {
		return nil
	}

	input.Scopes = scopes

	return input
}

// WithScope appends a single scope.
func (input *CreateLimitInput) WithScope(scope Scope) *CreateLimitInput {
	if input == nil {
		return nil
	}

	input.Scopes = append(input.Scopes, scope)

	return input
}

// WithActiveTimeWindow sets the daily active window (HH:MM strings).
func (input *CreateLimitInput) WithActiveTimeWindow(start, end string) *CreateLimitInput {
	if input == nil {
		return nil
	}

	input.ActiveTimeStart = &start
	input.ActiveTimeEnd = &end

	return input
}

// WithCustomDateRange sets the CUSTOM-window date bounds (wire date strings).
func (input *CreateLimitInput) WithCustomDateRange(start, end string) *CreateLimitInput {
	if input == nil {
		return nil
	}

	input.CustomStartDate = &start
	input.CustomEndDate = &end

	return input
}

// Validate enforces SDK-side preconditions before the round trip: Name 1-255,
// LimitType in the closed enum, MaxAmount strictly positive, Currency exactly 3
// chars, 1-100 scopes.
func (input *CreateLimitInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	validateLimitName(&errs, input.Name)
	validateLimitType(&errs, input.LimitType)
	validateLimitMaxAmount(&errs, input.MaxAmount)
	validateLimitCurrency(&errs, input.Currency)
	validateLimitScopes(&errs, input.Scopes)

	if input.Description != nil && len(*input.Description) > maxLimitDescriptionLength {
		errs.Append("description", fmt.Sprintf("must be at most %d characters", maxLimitDescriptionLength))
	}

	return errs.OrNil()
}

// UpdateLimitInput is the PATCH payload for a limit. It deliberately OMITS
// LimitType and Currency: those are immutable after create and the server
// rejects any update body containing either with a 400 (ErrLimitImmutableField).
// Because the fields do not exist here, the marshaled body can never carry them.
//
// Every field is a pointer: nil means "leave unchanged". MarshalJSON emits only
// the set fields (omit-unset) and Validate rejects a no-op PATCH.
type UpdateLimitInput struct {
	Name            *string          `json:"-"`
	Description     *string          `json:"-"`
	MaxAmount       *decimal.Decimal `json:"-"`
	Scopes          *[]Scope         `json:"-"`
	ActiveTimeStart *string          `json:"-"`
	ActiveTimeEnd   *string          `json:"-"`
	CustomStartDate *string          `json:"-"`
	CustomEndDate   *string          `json:"-"`
}

// NewUpdateLimitInput builds an empty limit update payload.
func NewUpdateLimitInput() *UpdateLimitInput {
	return &UpdateLimitInput{}
}

// WithName sets the name for update.
func (input *UpdateLimitInput) WithName(name string) *UpdateLimitInput {
	if input == nil {
		return nil
	}

	input.Name = &name

	return input
}

// WithDescription sets the description for update.
func (input *UpdateLimitInput) WithDescription(description string) *UpdateLimitInput {
	if input == nil {
		return nil
	}

	input.Description = &description

	return input
}

// WithMaxAmount sets the ceiling for update (exact decimal money).
func (input *UpdateLimitInput) WithMaxAmount(maxAmount decimal.Decimal) *UpdateLimitInput {
	if input == nil {
		return nil
	}

	input.MaxAmount = &maxAmount

	return input
}

// WithScopes sets the scope set for update.
func (input *UpdateLimitInput) WithScopes(scopes []Scope) *UpdateLimitInput {
	if input == nil {
		return nil
	}

	input.Scopes = &scopes

	return input
}

// WithActiveTimeWindow sets the daily active window for update (HH:MM strings).
func (input *UpdateLimitInput) WithActiveTimeWindow(start, end string) *UpdateLimitInput {
	if input == nil {
		return nil
	}

	input.ActiveTimeStart = &start
	input.ActiveTimeEnd = &end

	return input
}

// WithCustomDateRange sets the CUSTOM-window date bounds for update (wire date strings).
func (input *UpdateLimitInput) WithCustomDateRange(start, end string) *UpdateLimitInput {
	if input == nil {
		return nil
	}

	input.CustomStartDate = &start
	input.CustomEndDate = &end

	return input
}

// IsEmpty reports whether the PATCH carries no changes (a no-op update).
func (input *UpdateLimitInput) IsEmpty() bool {
	if input == nil {
		return true
	}

	return input.Name == nil &&
		input.Description == nil &&
		input.MaxAmount == nil &&
		input.Scopes == nil &&
		input.ActiveTimeStart == nil &&
		input.ActiveTimeEnd == nil &&
		input.CustomStartDate == nil &&
		input.CustomEndDate == nil
}

// Validate rejects an empty PATCH and checks each set field against the same
// bounds as create (LimitType and Currency are absent — immutable).
func (input *UpdateLimitInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.IsEmpty() {
		return errors.New("empty update payload not allowed")
	}

	var errs validation.FieldErrors

	if input.Name != nil {
		validateLimitName(&errs, *input.Name)
	}

	if input.MaxAmount != nil {
		validateLimitMaxAmount(&errs, *input.MaxAmount)
	}

	if input.Scopes != nil {
		validateLimitScopes(&errs, *input.Scopes)
	}

	if input.Description != nil && len(*input.Description) > maxLimitDescriptionLength {
		errs.Append("description", fmt.Sprintf("must be at most %d characters", maxLimitDescriptionLength))
	}

	return errs.OrNil()
}

// MarshalJSON emits only the fields explicitly set on the PATCH input, under
// their server wire names. LimitType and Currency are structurally absent, so
// the body can never carry an immutable field.
func (input *UpdateLimitInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	payload := map[string]any{}

	if input.Name != nil {
		payload["name"] = input.Name
	}

	if input.Description != nil {
		payload["description"] = input.Description
	}

	if input.MaxAmount != nil {
		payload["maxAmount"] = input.MaxAmount
	}

	if input.Scopes != nil {
		payload["scopes"] = input.Scopes
	}

	if input.ActiveTimeStart != nil {
		payload["activeTimeStart"] = input.ActiveTimeStart
	}

	if input.ActiveTimeEnd != nil {
		payload["activeTimeEnd"] = input.ActiveTimeEnd
	}

	if input.CustomStartDate != nil {
		payload["customStartDate"] = input.CustomStartDate
	}

	if input.CustomEndDate != nil {
		payload["customEndDate"] = input.CustomEndDate
	}

	return json.Marshal(payload)
}

// LimitsListOpts is the typed options struct for the limits cursor list. It
// embeds CursorListOpts for the shared cursor/sort-order/date fields, adds SortBy
// (the tracer accepts a sort_by field), and attaches a typed Filters sub-struct
// carrying only the filters the limits endpoint honors.
//
// This is a CURSOR-PAGINATED endpoint: no Page or Offset. Iterate by passing
// back the previous response's NextCursor.
type LimitsListOpts struct {
	CursorListOpts

	// SortBy names the sort field (created_at, updated_at, name, max_amount).
	// Empty falls back to the server default. Passed through verbatim.
	SortBy string

	// Filters narrows the result set. Zero value means no narrowing.
	Filters LimitsFilters
}

// LimitsFilters is the typed filter set for the limits endpoint. Each field maps
// to a native query-param slot on the generated ListLimitsParams.
type LimitsFilters struct {
	Name            string
	Status          string
	LimitType       string
	AccountID       string
	SegmentID       string
	PortfolioID     string
	MerchantID      string
	TransactionType string
	SubType         string
}

// Validate enforces the shared cursor-list preconditions (limit bounds, sort
// direction, date range). Filter values are passed through — the server
// validates them.
func (o LimitsListOpts) Validate() error {
	return ValidateCursorListOpts("LimitsListOpts.Validate", o.CursorListOpts)
}

func validateLimitName(errs *validation.FieldErrors, name string) {
	trimmed := strings.TrimSpace(name)

	switch {
	case trimmed == "":
		errs.Append("name", "is required")
	case len(name) > maxLimitNameLength:
		errs.Append("name", fmt.Sprintf("must be at most %d characters", maxLimitNameLength))
	}
}

func validateLimitType(errs *validation.FieldErrors, limitType string) {
	switch limitType {
	case LimitTypeDaily, LimitTypeWeekly, LimitTypeMonthly, LimitTypePerTransaction, LimitTypeCustom:
	default:
		errs.Append("limitType", fmt.Sprintf("must be one of %s, %s, %s, %s, %s",
			LimitTypeDaily, LimitTypeWeekly, LimitTypeMonthly, LimitTypePerTransaction, LimitTypeCustom))
	}
}

func validateLimitMaxAmount(errs *validation.FieldErrors, maxAmount decimal.Decimal) {
	if !maxAmount.GreaterThan(decimal.Zero) {
		errs.Append("maxAmount", "must be greater than zero")
	}
}

func validateLimitCurrency(errs *validation.FieldErrors, currency string) {
	if len(currency) != currencyCodeLength {
		errs.Append("currency", fmt.Sprintf("must be exactly %d characters", currencyCodeLength))
	}
}

func validateLimitScopes(errs *validation.FieldErrors, scopes []Scope) {
	switch {
	case len(scopes) == 0:
		errs.Append("scopes", "at least one scope is required")
	case len(scopes) > maxLimitScopes:
		errs.Append("scopes", fmt.Sprintf("must be at most %d entries", maxLimitScopes))
	}
}

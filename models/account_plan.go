// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation"
)

// maxFeeRoundingScale is the largest rounding scale the contract accepts.
const maxFeeRoundingScale = 18

// accountPlanFeeKeyPattern is the fee key format the contract declares.
var accountPlanFeeKeyPattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)

// FeeRoundingMode is how a rounded fee total is rounded.
type FeeRoundingMode string

const (
	// FeeRoundingModeFloor rounds each fee total down.
	FeeRoundingModeFloor FeeRoundingMode = "floor"

	// FeeRoundingModeHalfUp rounds each fee total half away from zero.
	FeeRoundingModeHalfUp FeeRoundingMode = "halfUp"
)

// FeeRounding rounds every fee an account plan charges to one scale, whatever
// the asset. Each payer's share rounds toward zero and the largest payer takes
// the remainder, so the shares sum to the total.
//
// Requires Midaz 4.6.0 or later. A plan whose version carries no rounding
// charges unrounded amounts; that is modeled as a nil *FeeRounding, which is
// distinct from a rounding of scale zero.
type FeeRounding struct {
	// Scale is the number of decimal places, from 0 to 18.
	Scale int `json:"scale"`

	// Mode is FeeRoundingModeFloor or FeeRoundingModeHalfUp.
	Mode FeeRoundingMode `json:"mode"`
}

// AccountPlanTriggerType is what makes an account plan fee apply.
type AccountPlanTriggerType string

const (
	// AccountPlanTriggerTypeTransaction charges the fee on a matching transaction.
	AccountPlanTriggerTypeTransaction AccountPlanTriggerType = "transaction"

	// AccountPlanTriggerTypeService charges the fee through a service charge
	// identified by the trigger's ServiceCode.
	AccountPlanTriggerTypeService AccountPlanTriggerType = "service"
)

// AccountPlanTriggerDirection is the side of a transaction a transaction fee
// incurs on. It is required for a transaction fee and refused on a service fee.
type AccountPlanTriggerDirection string

const (
	// AccountPlanTriggerDirectionSent charges the account that sends, on top of
	// what it sends.
	AccountPlanTriggerDirectionSent AccountPlanTriggerDirection = "sent"

	// AccountPlanTriggerDirectionReceived charges the account that receives,
	// deducted from what it receives. A received fee cannot be deferrable.
	//
	// Rollout note: received requires Midaz 4.6.0 or later. Create a received fee
	// only when every server pod runs 4.6.0; otherwise an older pod charges the
	// sender instead.
	AccountPlanTriggerDirectionReceived AccountPlanTriggerDirection = "received"
)

// AccountPlan is an account plan as listed by the ledger: the plan registry
// without its fees. Midaz serves account plans from 4.4.0 on the /v2 surface.
type AccountPlan struct {
	// ID is the plan identifier.
	ID string `json:"id"`

	// OrganizationID is the organization that owns the plan.
	OrganizationID string `json:"organizationId"`

	// LedgerID is the ledger the plan belongs to.
	LedgerID string `json:"ledgerId"`

	// Label is the plan name, unique among the ledger's plans that are not deleted.
	Label string `json:"label"`

	// Description is the optional plan description; nil when the plan has none.
	Description *string `json:"description,omitempty"`

	// Enable reports whether new accounts can be assigned to the plan.
	Enable bool `json:"enable"`

	// Timezone is the IANA timezone the monthly free quota is counted in.
	Timezone string `json:"timezone"`

	// CurrentVersion is the number of the plan's latest version.
	CurrentVersion int `json:"currentVersion"`

	// CreatedAt is when the plan was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the plan was last updated.
	UpdatedAt time.Time `json:"updatedAt"`

	// DeletedAt is when the plan was deleted; nil while it is not.
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// AccountPlanView is an account plan together with the version in effect at
// the time of the read: the shape returned by create, read and update.
type AccountPlanView struct {
	AccountPlan

	// Version is the number of the version in effect.
	Version int `json:"version"`

	// EffectiveFrom is when the version in effect started to apply.
	EffectiveFrom time.Time `json:"effectiveFrom"`

	// Fees are the fees of the version in effect, keyed by fee key.
	Fees map[string]AccountPlanFee `json:"fees"`

	// Rounding is the rounding of the version in effect; nil when the version
	// charges unrounded amounts.
	Rounding *FeeRounding `json:"rounding,omitempty"`
}

// AccountPlanVersion is one version of an account plan. A version is created
// whenever an update changes the fees or the rounding.
type AccountPlanVersion struct {
	// AccountPlanID is the plan the version belongs to.
	AccountPlanID string `json:"accountPlanId"`

	// Version is the version number, starting at 1.
	Version int `json:"version"`

	// EffectiveFrom is when the version starts to apply.
	EffectiveFrom time.Time `json:"effectiveFrom"`

	// Fees are the version's fees, keyed by fee key.
	Fees map[string]AccountPlanFee `json:"fees"`

	// Rounding is the version's rounding; nil when it charges unrounded amounts.
	Rounding *FeeRounding `json:"rounding,omitempty"`

	// CreatedAt is when the version was created.
	CreatedAt time.Time `json:"createdAt"`
}

// AccountPlanFee is one fee of an account plan version, as stored by the
// server with every default applied.
type AccountPlanFee struct {
	// FeeLabel is the fee name.
	FeeLabel string `json:"feeLabel"`

	// Trigger is what makes the fee apply.
	Trigger AccountPlanTrigger `json:"trigger"`

	// CalculationModel is how the fee is computed.
	CalculationModel FeeCalculationModel `json:"calculationModel"`

	// ReferenceAmount is the amount the fee is calculated on (originalAmount).
	ReferenceAmount string `json:"referenceAmount"`

	// CreditAccount is the alias of the ledger account that receives the fee.
	CreditAccount string `json:"creditAccount"`

	// FreeQuota is the number of units per period charged nothing.
	FreeQuota int `json:"freeQuota"`

	// Period is the window the free quota resets on (monthly).
	Period string `json:"period"`

	// Deferrable reports whether the share a payer cannot fund becomes a fee
	// debt instead of refusing the transaction.
	Deferrable bool `json:"deferrable"`

	// Accounting holds the fee routes; nil when the fee has none.
	Accounting *AccountPlanAccounting `json:"accounting,omitempty"`
}

// AccountPlanTrigger is what makes an account plan fee apply.
type AccountPlanTrigger struct {
	// Type is AccountPlanTriggerTypeTransaction or AccountPlanTriggerTypeService.
	Type AccountPlanTriggerType `json:"type"`

	// Direction is the side of the transaction a transaction fee incurs on;
	// empty on a service fee.
	Direction AccountPlanTriggerDirection `json:"direction,omitempty"`

	// Schemes are the transaction schemes a transaction fee matches; empty
	// matches every scheme.
	Schemes []string `json:"schemes,omitempty"`

	// Metadata are the metadata pairs a transaction must all carry for a
	// transaction fee to match.
	Metadata map[string]string `json:"metadata,omitempty"`

	// ServiceCode is the service code a service fee is charged through.
	ServiceCode string `json:"serviceCode,omitempty"`
}

// AccountPlanAccounting holds the routes an account plan fee posts through.
type AccountPlanAccounting struct {
	// TransactionRoute is the transaction route a transaction must carry for
	// the fee to match.
	TransactionRoute string `json:"transactionRoute"`

	// OperationRouteFrom is the operation route of the fee debit leg.
	OperationRouteFrom string `json:"operationRouteFrom"`

	// OperationRouteTo is the operation route of the fee credit leg.
	OperationRouteTo string `json:"operationRouteTo"`
}

// AccountPlanAssignment is the assignment of an account plan to an account.
type AccountPlanAssignment struct {
	// ID is the assignment identifier.
	ID string `json:"id"`

	// OrganizationID is the organization that owns the account.
	OrganizationID string `json:"organizationId"`

	// LedgerID is the ledger of the account and the plan.
	LedgerID string `json:"ledgerId"`

	// AccountID is the assigned account.
	AccountID string `json:"accountId"`

	// AccountAlias is the alias of the assigned account.
	AccountAlias string `json:"accountAlias"`

	// AccountPlanID is the assigned plan.
	AccountPlanID string `json:"accountPlanId"`

	// StartedAt is when the assignment started.
	StartedAt time.Time `json:"startedAt"`

	// EndedAt is when the assignment ended; nil while it is active.
	EndedAt *time.Time `json:"endedAt"`
}

// AccountPlanAssignments is the account plan assignment state of an account:
// the active assignment, if any, and every assignment it has had.
type AccountPlanAssignments struct {
	// Active is the active assignment; nil when the account has no plan.
	Active *AccountPlanAssignment `json:"active"`

	// History lists every assignment of the account, the active one included.
	History []AccountPlanAssignment `json:"history"`
}

// AccountPlanUsage is the free-quota usage of an account under its active
// account plan for one period.
type AccountPlanUsage struct {
	// AccountID is the account the usage belongs to.
	AccountID string `json:"accountId"`

	// AccountPlanID is the account's active plan.
	AccountPlanID string `json:"accountPlanId"`

	// AccountPlanVersion is the plan version the usage was read against.
	AccountPlanVersion int `json:"accountPlanVersion"`

	// Period is the period read.
	Period QuotaUsagePeriod `json:"period"`

	// Items holds the usage of each fee of the plan.
	Items []AccountPlanFeeUsage `json:"items"`
}

// AccountPlanFeeUsage is the free-quota usage of one account plan fee.
type AccountPlanFeeUsage struct {
	// FeeKey is the key of the fee in the plan.
	FeeKey string `json:"feeKey"`

	// Trigger is the server's identifier of the fee's usage counter.
	Trigger string `json:"trigger"`

	// Period is the period the counters refer to.
	Period QuotaUsagePeriod `json:"period"`

	// FreeQuota is the number of units per period charged nothing.
	FreeQuota int `json:"freeQuota"`

	// Used is the number of units used in the period.
	Used int `json:"used"`

	// Remaining is the number of free units left in the period.
	Remaining int `json:"remaining"`

	// Charged is the number of units charged in the period.
	Charged int `json:"charged"`
}

// QuotaUsagePeriod is a free-quota period in the plan timezone.
type QuotaUsagePeriod struct {
	// Label is the period in the YYYY-MM format.
	Label string `json:"label"`

	// Start is the instant the period starts.
	Start time.Time `json:"start"`

	// End is the instant the period ends.
	End time.Time `json:"end"`
}

// CreateAccountPlanInput is the payload for creating an account plan.
//
// The ledger is not part of this payload: it travels only as a path segment.
// The server schema is closed, so every optional field is omitted when unset.
// Enable is required by the server; NewCreateAccountPlanInput always sets it,
// so enable false is sent rather than omitted.
type CreateAccountPlanInput struct {
	// Label is the plan name, unique among the ledger's plans that are not
	// deleted. Required.
	Label string `json:"label"`

	// Description is the optional plan description.
	Description *string `json:"description,omitempty"`

	// Enable reports whether new accounts can be assigned to the plan. Required.
	Enable *bool `json:"enable"`

	// Timezone is the optional IANA timezone the monthly free quota is counted
	// in. The server defaults it to America/Sao_Paulo.
	Timezone *string `json:"timezone,omitempty"`

	// Rounding is the optional rounding of every fee the plan charges. Omit it
	// to charge unrounded amounts. Requires Midaz 4.6.0 or later.
	Rounding *FeeRounding `json:"rounding,omitempty"`

	// Fees are the plan fees keyed by fee key, at least one. A key must match
	// ^[a-z][a-zA-Z0-9]*$.
	Fees map[string]AccountPlanFeeInput `json:"fees"`
}

// AccountPlanFeeInput is one fee of an account plan being created, or a fee
// added by an update. Optional fields are omitted when zero; their zero value
// is the server default.
type AccountPlanFeeInput struct {
	// FeeLabel is the fee name. Required.
	FeeLabel string `json:"feeLabel"`

	// Trigger is what makes the fee apply. Required.
	Trigger AccountPlanTriggerInput `json:"trigger"`

	// CalculationModel is how the fee is computed. Required.
	CalculationModel FeeCalculationModel `json:"calculationModel"`

	// ReferenceAmount is the amount the fee is calculated on. Only
	// originalAmount is accepted, and it is the server default.
	ReferenceAmount string `json:"referenceAmount,omitempty"`

	// CreditAccount is the alias of the ledger account that receives the fee.
	// It must exist in the ledger. Required.
	CreditAccount string `json:"creditAccount"`

	// FreeQuota is the number of units per period charged nothing. Defaults to 0.
	FreeQuota int `json:"freeQuota,omitempty"`

	// Period is the window the free quota resets on. Only monthly is accepted,
	// and it is the server default.
	Period string `json:"period,omitempty"`

	// Deferrable makes the share a payer cannot fund a fee debt instead of
	// refusing the transaction. Defaults to false; refused on a received fee.
	Deferrable bool `json:"deferrable,omitempty"`

	// Accounting holds the fee routes. Required by the server when the ledger
	// validates accounting routes; only stored otherwise.
	Accounting *AccountPlanAccountingInput `json:"accounting,omitempty"`
}

// AccountPlanTriggerInput is the trigger of an account plan fee being written.
//
// Which fields a trigger needs depends on its type and on the ledger settings
// (a transaction fee needs a direction; a service fee needs a service code and
// refuses direction, schemes and metadata). Those rules are enforced by the
// server; Validate checks only the accepted values of Type and Direction.
type AccountPlanTriggerInput struct {
	// Type is AccountPlanTriggerTypeTransaction or AccountPlanTriggerTypeService.
	// Required.
	Type AccountPlanTriggerType `json:"type"`

	// Direction is the side of the transaction a transaction fee incurs on.
	// See AccountPlanTriggerDirectionReceived for its rollout note.
	Direction AccountPlanTriggerDirection `json:"direction,omitempty"`

	// Schemes are the transaction schemes a transaction fee matches. Omit them
	// to match every scheme.
	Schemes []string `json:"schemes,omitempty"`

	// Metadata are the metadata pairs a transaction must all carry for a
	// transaction fee to match.
	Metadata map[string]string `json:"metadata,omitempty"`

	// ServiceCode is the service code a service fee is charged through.
	ServiceCode string `json:"serviceCode,omitempty"`
}

// AccountPlanAccountingInput holds the routes of an account plan fee being
// written. Each field is omitted when empty.
type AccountPlanAccountingInput struct {
	// TransactionRoute is the transaction route a transaction must carry for
	// the fee to match.
	TransactionRoute string `json:"transactionRoute,omitempty"`

	// OperationRouteFrom is the operation route of the fee debit leg.
	OperationRouteFrom string `json:"operationRouteFrom,omitempty"`

	// OperationRouteTo is the operation route of the fee credit leg.
	OperationRouteTo string `json:"operationRouteTo,omitempty"`
}

// NewCreateAccountPlanInput builds an account plan create payload with the
// required fields set, enable included. The ledger comes from the facade
// call's path argument, not from this payload.
func NewCreateAccountPlanInput(label string, enable bool, fees map[string]AccountPlanFeeInput) *CreateAccountPlanInput {
	return &CreateAccountPlanInput{
		Label:  label,
		Enable: &enable,
		Fees:   fees,
	}
}

// WithDescription sets the optional plan description.
func (input *CreateAccountPlanInput) WithDescription(description string) *CreateAccountPlanInput {
	if input == nil {
		return nil
	}

	input.Description = &description

	return input
}

// WithTimezone sets the optional IANA timezone of the free quota.
func (input *CreateAccountPlanInput) WithTimezone(timezone string) *CreateAccountPlanInput {
	if input == nil {
		return nil
	}

	input.Timezone = &timezone

	return input
}

// WithRounding sets the rounding of every fee the plan charges. Requires Midaz
// 4.6.0 or later.
func (input *CreateAccountPlanInput) WithRounding(rounding FeeRounding) *CreateAccountPlanInput {
	if input == nil {
		return nil
	}

	input.Rounding = &rounding

	return input
}

// Validate enforces what the contract declares for a create payload: a label,
// enable, at least one fee, each fee key format, each fee's required fields,
// the accepted trigger type and direction, a non-negative free quota, and the
// rounding scale and mode. Rules that depend on server state (accounting routes,
// required schemes, existing accounts) are left to the server.
func (input *CreateAccountPlanInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	if strings.TrimSpace(input.Label) == "" {
		errs.Append("label", "is required")
	}

	if input.Enable == nil {
		errs.Append("enable", "is required")
	}

	if len(input.Fees) == 0 {
		errs.Append("fees", "at least one fee is required")
	}

	if input.Rounding != nil {
		validateFeeRounding(&errs, *input.Rounding)
	}

	for _, key := range slices.Sorted(maps.Keys(input.Fees)) {
		prefix := accountPlanFeePrefix(key)
		validateAccountPlanFeeKey(&errs, prefix, key)
		validateAccountPlanFeeInput(&errs, prefix, input.Fees[key])
	}

	return errs.OrNil()
}

// accountPlanFeePrefix is the field key of one fee, as fees[<key>].
func accountPlanFeePrefix(key string) string {
	return "fees[" + key + "]"
}

func validateAccountPlanFeeKey(errs *validation.FieldErrors, prefix, key string) {
	if !accountPlanFeeKeyPattern.MatchString(key) {
		errs.Append(prefix, "fee key must match "+accountPlanFeeKeyPattern.String())
	}
}

func validateAccountPlanFeeInput(errs *validation.FieldErrors, prefix string, fee AccountPlanFeeInput) {
	if strings.TrimSpace(fee.FeeLabel) == "" {
		errs.Append(prefix+".feeLabel", "is required")
	}

	if strings.TrimSpace(fee.CreditAccount) == "" {
		errs.Append(prefix+".creditAccount", "is required")
	}

	if fee.CalculationModel.ApplicationRule == "" && len(fee.CalculationModel.Calculations) == 0 {
		errs.Append(prefix+".calculationModel", "is required")
	}

	if fee.FreeQuota < 0 {
		errs.Append(prefix+".freeQuota", "must be greater than or equal to 0")
	}

	validateAccountPlanTrigger(errs, prefix+".trigger", fee.Trigger)
}

func validateAccountPlanTrigger(errs *validation.FieldErrors, prefix string, trigger AccountPlanTriggerInput) {
	switch trigger.Type {
	case AccountPlanTriggerTypeTransaction, AccountPlanTriggerTypeService:
	case "":
		errs.Append(prefix+".type", "is required")
	default:
		errs.Append(prefix+".type", "must be one of: transaction, service")
	}

	switch trigger.Direction {
	case "", AccountPlanTriggerDirectionSent, AccountPlanTriggerDirectionReceived:
	default:
		errs.Append(prefix+".direction", "must be one of: sent, received")
	}
}

func validateFeeRounding(errs *validation.FieldErrors, rounding FeeRounding) {
	if rounding.Scale < 0 || rounding.Scale > maxFeeRoundingScale {
		errs.Append("rounding.scale", "must be between 0 and 18")
	}

	switch rounding.Mode {
	case FeeRoundingModeFloor, FeeRoundingModeHalfUp:
	default:
		errs.Append("rounding.mode", "must be one of: floor, halfUp")
	}
}

// UpdateAccountPlanInput is the PATCH payload for an account plan. MarshalJSON
// emits only the fields that were set, so an unchanged field is never sent.
//
// Setting fees or rounding creates a new plan version with the full resulting
// fee map; the other fields change the plan registry without a new version.
//
// The server accepts an empty PATCH as a no-op; Validate refuses it locally
// instead, to avoid the round trip.
type UpdateAccountPlanInput struct {
	// Label replaces the plan name.
	Label *string `json:"-"`

	// Description replaces the plan description.
	Description *string `json:"-"`

	// Enable replaces whether new accounts can be assigned to the plan.
	Enable *bool `json:"-"`

	// Timezone replaces the IANA timezone of the free quota.
	Timezone *string `json:"-"`

	// Rounding replaces the stored rounding whole and creates a new version.
	// An update cannot remove a stored rounding.
	Rounding *FeeRounding `json:"-"`

	// EffectiveFrom is when the new version applies from. The server accepts it
	// only together with fees or rounding, defaults it to the request instant,
	// and refuses one that precedes the latest version.
	EffectiveFrom *time.Time `json:"-"`

	// Fees holds the fee changes keyed by fee key. A non-nil patch on a stored
	// key replaces the fields it sets; a non-nil patch on any other key adds a
	// fee and must carry every required field; a nil entry removes the fee.
	// Use WithFeePatch and WithoutFee to fill it.
	Fees map[string]*AccountPlanFeePatch `json:"-"`
}

// AccountPlanFeePatch is a partial change to one account plan fee. Every field
// is optional and sent only when non-nil; pkg/conversion.ToPtr builds the
// pointers inline. A patch that sets no field would remove the fee, so Validate
// refuses it: use WithoutFee to remove a fee.
type AccountPlanFeePatch struct {
	// FeeLabel replaces the fee name.
	FeeLabel *string `json:"feeLabel,omitempty"`

	// Trigger replaces the fee trigger.
	Trigger *AccountPlanTriggerInput `json:"trigger,omitempty"`

	// CalculationModel replaces how the fee is computed.
	CalculationModel *FeeCalculationModel `json:"calculationModel,omitempty"`

	// ReferenceAmount replaces the amount the fee is calculated on.
	ReferenceAmount *string `json:"referenceAmount,omitempty"`

	// CreditAccount replaces the alias of the account that receives the fee.
	CreditAccount *string `json:"creditAccount,omitempty"`

	// FreeQuota replaces the number of units per period charged nothing.
	FreeQuota *int `json:"freeQuota,omitempty"`

	// Period replaces the window the free quota resets on.
	Period *string `json:"period,omitempty"`

	// Deferrable replaces whether an unfunded share becomes a fee debt.
	Deferrable *bool `json:"deferrable,omitempty"`

	// Accounting replaces the fee routes.
	Accounting *AccountPlanAccountingInput `json:"accounting,omitempty"`
}

// NewUpdateAccountPlanInput builds an empty account plan update payload.
func NewUpdateAccountPlanInput() *UpdateAccountPlanInput {
	return &UpdateAccountPlanInput{}
}

// WithLabel sets the plan name for update.
func (input *UpdateAccountPlanInput) WithLabel(label string) *UpdateAccountPlanInput {
	if input == nil {
		return nil
	}

	input.Label = &label

	return input
}

// WithDescription sets the plan description for update.
func (input *UpdateAccountPlanInput) WithDescription(description string) *UpdateAccountPlanInput {
	if input == nil {
		return nil
	}

	input.Description = &description

	return input
}

// WithEnable sets whether new accounts can be assigned to the plan.
func (input *UpdateAccountPlanInput) WithEnable(enable bool) *UpdateAccountPlanInput {
	if input == nil {
		return nil
	}

	input.Enable = &enable

	return input
}

// WithTimezone sets the IANA timezone of the free quota for update.
func (input *UpdateAccountPlanInput) WithTimezone(timezone string) *UpdateAccountPlanInput {
	if input == nil {
		return nil
	}

	input.Timezone = &timezone

	return input
}

// WithRounding replaces the stored rounding whole, creating a new version.
// Requires Midaz 4.6.0 or later.
func (input *UpdateAccountPlanInput) WithRounding(rounding FeeRounding) *UpdateAccountPlanInput {
	if input == nil {
		return nil
	}

	input.Rounding = &rounding

	return input
}

// WithEffectiveFrom sets when the new version applies from. Use it only with
// fee changes or rounding.
func (input *UpdateAccountPlanInput) WithEffectiveFrom(effectiveFrom time.Time) *UpdateAccountPlanInput {
	if input == nil {
		return nil
	}

	input.EffectiveFrom = &effectiveFrom

	return input
}

// WithFeePatch sets the change to the fee under key. A later WithFeePatch or
// WithoutFee on the same key replaces it.
func (input *UpdateAccountPlanInput) WithFeePatch(key string, patch AccountPlanFeePatch) *UpdateAccountPlanInput {
	if input == nil {
		return nil
	}

	if input.Fees == nil {
		input.Fees = make(map[string]*AccountPlanFeePatch)
	}

	input.Fees[key] = &patch

	return input
}

// WithoutFee removes the fee under key; it is sent as null. A later
// WithFeePatch or WithoutFee on the same key replaces it.
func (input *UpdateAccountPlanInput) WithoutFee(key string) *UpdateAccountPlanInput {
	if input == nil {
		return nil
	}

	if input.Fees == nil {
		input.Fees = make(map[string]*AccountPlanFeePatch)
	}

	input.Fees[key] = nil

	return input
}

func (input *UpdateAccountPlanInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.Label != nil ||
		input.Description != nil ||
		input.Enable != nil ||
		input.Timezone != nil ||
		input.Rounding != nil ||
		input.EffectiveFrom != nil ||
		len(input.Fees) > 0
}

// Validate refuses a nil or empty update, and checks what the contract
// declares for the fields that were set: the rounding scale and mode, each fee
// key format, the accepted trigger type and direction, and a non-negative free
// quota. It also refuses a fee patch that sets no field, which the server would
// treat as a removal.
func (input *UpdateAccountPlanInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	var errs validation.FieldErrors

	if input.Rounding != nil {
		validateFeeRounding(&errs, *input.Rounding)
	}

	for _, key := range slices.Sorted(maps.Keys(input.Fees)) {
		prefix := accountPlanFeePrefix(key)
		validateAccountPlanFeeKey(&errs, prefix, key)

		if patch := input.Fees[key]; patch != nil {
			validateAccountPlanFeePatch(&errs, prefix, *patch)
		}
	}

	return errs.OrNil()
}

func validateAccountPlanFeePatch(errs *validation.FieldErrors, prefix string, patch AccountPlanFeePatch) {
	if patch == (AccountPlanFeePatch{}) {
		errs.Append(prefix, "patch sets no field; use WithoutFee to remove the fee")

		return
	}

	if patch.FreeQuota != nil && *patch.FreeQuota < 0 {
		errs.Append(prefix+".freeQuota", "must be greater than or equal to 0")
	}

	if patch.Trigger != nil {
		validateAccountPlanTrigger(errs, prefix+".trigger", *patch.Trigger)
	}
}

// MarshalJSON emits only the fields that were set, under their server wire
// names, with keys in a stable order. A removed fee is emitted as null.
//
// It has a value receiver so a pointer and a value marshal to the same body;
// encoding/json emits null for a nil *UpdateAccountPlanInput.
func (input UpdateAccountPlanInput) MarshalJSON() ([]byte, error) {
	payload := map[string]any{}

	if input.Label != nil {
		payload["label"] = input.Label
	}

	if input.Description != nil {
		payload["description"] = input.Description
	}

	if input.Enable != nil {
		payload["enable"] = input.Enable
	}

	if input.Timezone != nil {
		payload["timezone"] = input.Timezone
	}

	if input.Rounding != nil {
		payload["rounding"] = input.Rounding
	}

	if input.EffectiveFrom != nil {
		payload["effectiveFrom"] = input.EffectiveFrom
	}

	if len(input.Fees) > 0 {
		payload["fees"] = input.Fees
	}

	return json.Marshal(payload)
}

// AssignAccountPlanInput is the payload for assigning an account plan to an
// account. Assigning replaces the account's active plan, if any, and assigning
// the active plan again is accepted by the server without change.
type AssignAccountPlanInput struct {
	// AccountPlanID is the plan to assign, as a UUID. The plan must belong to
	// the ledger of the call, not be deleted and be enabled. Required.
	AccountPlanID string `json:"accountPlanId"`
}

// NewAssignAccountPlanInput builds an assignment payload for the given plan.
func NewAssignAccountPlanInput(accountPlanID string) *AssignAccountPlanInput {
	return &AssignAccountPlanInput{AccountPlanID: accountPlanID}
}

// Validate refuses a nil input and an accountPlanId that is not a UUID, the
// format the contract declares.
func (input *AssignAccountPlanInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	switch {
	case strings.TrimSpace(input.AccountPlanID) == "":
		errs.Append("accountPlanId", "is required")
	case !validation.IsValidUUID(input.AccountPlanID):
		errs.Append("accountPlanId", "must be a UUID")
	}

	return errs.OrNil()
}

// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
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

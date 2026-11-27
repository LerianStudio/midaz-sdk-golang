// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"time"
)

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

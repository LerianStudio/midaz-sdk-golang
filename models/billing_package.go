// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation"
)

// Billing-package type discriminants. A package is either a volume package
// (event-filtered, tiered pricing) or a maintenance package (flat fee against an
// account target). These mirror the server constants
// (feeshared/model/billing_package.go).
const (
	BillingPackageTypeVolume      = "volume"
	BillingPackageTypeMaintenance = "maintenance"
)

// BillingPackage is a billing-package definition returned by the ledger fee
// endpoints. It mirrors the generated genledger.FeeBillingPackage read shape.
//
// Money is string, never float: FeeAmount, every pricing-tier UnitPrice, and
// every discount-tier DiscountPercentage ride the wire as JSON strings and are
// modeled as string so a value like 0.333333333333333333 survives with no
// precision loss. A float hop would silently drift such a value.
type BillingPackage struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	LedgerID       string  `json:"ledgerId"`
	Label          string  `json:"label"`
	Description    *string `json:"description,omitempty"`
	Type           string  `json:"type"`
	Enable         *bool   `json:"enable"`

	// Volume-specific fields.
	EventFilter        *BillingEventFilter    `json:"eventFilter,omitempty"`
	PricingModel       *string                `json:"pricingModel,omitempty"`
	Tiers              *[]BillingPricingTier  `json:"tiers"`
	FreeQuota          *int64                 `json:"freeQuota,omitempty"`
	DiscountTiers      *[]BillingDiscountTier `json:"discountTiers"`
	CountMode          *string                `json:"countMode,omitempty"`
	AssetCode          *string                `json:"assetCode,omitempty"`
	DebitAccountAlias  *string                `json:"debitAccountAlias,omitempty"`
	CreditAccountAlias *string                `json:"creditAccountAlias,omitempty"`

	// Maintenance-specific fields.
	FeeAmount                *string               `json:"feeAmount,omitempty"`
	MaintenanceCreditAccount *string               `json:"maintenanceCreditAccount,omitempty"`
	AccountTarget            *BillingAccountTarget `json:"accountTarget,omitempty"`

	// Timestamps (RFC 3339 strings, mirroring the generated read shape).
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
	DeletedAt *string `json:"deletedAt,omitempty"`
}

// BillingAccountTarget identifies which accounts a maintenance package targets.
// Mirrors genledger.FeeAccountTarget. Exactly one of SegmentID, PortfolioID, or
// Aliases is set (server-enforced).
type BillingAccountTarget struct {
	SegmentID   *string   `json:"segmentId,omitempty"`
	PortfolioID *string   `json:"portfolioId,omitempty"`
	Aliases     *[]string `json:"aliases"`
}

// BillingEventFilter matches the transaction route and status of a billing event.
// Mirrors genledger.FeeEventFilter.
type BillingEventFilter struct {
	TransactionRoute string `json:"transactionRoute"`
	Status           string `json:"status"`
}

// BillingPricingTier is one quantity range and the unit price within it. Mirrors
// genledger.FeePricingTier. UnitPrice is money-adjacent and rides the wire as a
// string, never a float.
type BillingPricingTier struct {
	MinQuantity int64  `json:"minQuantity"`
	MaxQuantity *int64 `json:"maxQuantity,omitempty"`
	UnitPrice   string `json:"unitPrice"`
}

// BillingDiscountTier is a quantity threshold above which a discount applies.
// Mirrors genledger.FeeDiscountTier. DiscountPercentage rides the wire as a
// string, never a float.
type BillingDiscountTier struct {
	MinQuantity        int64  `json:"minQuantity"`
	DiscountPercentage string `json:"discountPercentage"`
}

// CreateBillingPackageInput is the payload for creating a billing package. It is
// byte-for-byte with the server DTO — the server binds the request body into a
// full model.BillingPackage (feeshared/model/billing_package.go), then stamps
// OrganizationID from the path. Server-owned fields (id/organizationId/timestamps)
// are omitted here; the SDK never sends them on create.
//
// Money is string throughout: FeeAmount and every tier price ride the wire as
// strings, never floats.
//
// The ledger is NOT part of this payload — it travels only as a path segment and
// is the sole ledger authority. The server schema is closed
// (additionalProperties: false), so a body carrying ledgerId is rejected.
type CreateBillingPackageInput struct {
	Label       string  `json:"label"`
	Type        string  `json:"type"`
	Description *string `json:"description,omitempty"`
	Enable      *bool   `json:"enable"`

	// Volume-specific fields.
	EventFilter        *BillingEventFilter   `json:"eventFilter,omitempty"`
	PricingModel       *string               `json:"pricingModel,omitempty"`
	Tiers              []BillingPricingTier  `json:"tiers,omitempty"`
	FreeQuota          *int64                `json:"freeQuota,omitempty"`
	DiscountTiers      []BillingDiscountTier `json:"discountTiers,omitempty"`
	CountMode          *string               `json:"countMode,omitempty"`
	AssetCode          *string               `json:"assetCode,omitempty"`
	DebitAccountAlias  *string               `json:"debitAccountAlias,omitempty"`
	CreditAccountAlias *string               `json:"creditAccountAlias,omitempty"`

	// Maintenance-specific fields.
	FeeAmount                *string               `json:"feeAmount,omitempty"`
	MaintenanceCreditAccount *string               `json:"maintenanceCreditAccount,omitempty"`
	AccountTarget            *BillingAccountTarget `json:"accountTarget,omitempty"`
}

// NewCreateVolumeBillingPackageInput builds a volume billing-package create
// payload with the required common + volume fields set. The ledger comes from the
// facade call's path argument, not from this payload. Chain WithEventFilter,
// WithPricingModel, WithPricingTiers, and WithEnable to complete it.
func NewCreateVolumeBillingPackageInput(label, assetCode, debitAlias, creditAlias string) *CreateBillingPackageInput {
	return &CreateBillingPackageInput{
		Label:              label,
		Type:               BillingPackageTypeVolume,
		AssetCode:          &assetCode,
		DebitAccountAlias:  &debitAlias,
		CreditAccountAlias: &creditAlias,
	}
}

// NewCreateMaintenanceBillingPackageInput builds a maintenance billing-package
// create payload with the required common + maintenance fields set. FeeAmount is
// a money string. The ledger comes from the facade call's path argument, not from
// this payload. Chain WithAccountTarget and WithEnable to complete it.
func NewCreateMaintenanceBillingPackageInput(label, assetCode, feeAmount, maintenanceCreditAccount string) *CreateBillingPackageInput {
	return &CreateBillingPackageInput{
		Label:                    label,
		Type:                     BillingPackageTypeMaintenance,
		AssetCode:                &assetCode,
		FeeAmount:                &feeAmount,
		MaintenanceCreditAccount: &maintenanceCreditAccount,
	}
}

// WithDescription sets the optional package description.
func (input *CreateBillingPackageInput) WithDescription(description string) *CreateBillingPackageInput {
	if input == nil {
		return nil
	}

	input.Description = &description

	return input
}

// WithEnable sets the required enable flag.
func (input *CreateBillingPackageInput) WithEnable(enable bool) *CreateBillingPackageInput {
	if input == nil {
		return nil
	}

	input.Enable = &enable

	return input
}

// WithEventFilter sets the volume event filter (transaction route + status).
func (input *CreateBillingPackageInput) WithEventFilter(transactionRoute, status string) *CreateBillingPackageInput {
	if input == nil {
		return nil
	}

	input.EventFilter = &BillingEventFilter{TransactionRoute: transactionRoute, Status: status}

	return input
}

// WithPricingModel sets the volume pricing model (tiered or fixed).
func (input *CreateBillingPackageInput) WithPricingModel(model string) *CreateBillingPackageInput {
	if input == nil {
		return nil
	}

	input.PricingModel = &model

	return input
}

// WithPricingTiers sets the volume pricing tiers.
func (input *CreateBillingPackageInput) WithPricingTiers(tiers ...BillingPricingTier) *CreateBillingPackageInput {
	if input == nil {
		return nil
	}

	input.Tiers = append([]BillingPricingTier(nil), tiers...)

	return input
}

// WithFreeQuota sets the volume free quota.
func (input *CreateBillingPackageInput) WithFreeQuota(quota int64) *CreateBillingPackageInput {
	if input == nil {
		return nil
	}

	input.FreeQuota = &quota

	return input
}

// WithDiscountTiers sets the volume discount tiers.
func (input *CreateBillingPackageInput) WithDiscountTiers(tiers ...BillingDiscountTier) *CreateBillingPackageInput {
	if input == nil {
		return nil
	}

	input.DiscountTiers = append([]BillingDiscountTier(nil), tiers...)

	return input
}

// WithCountMode sets the volume count mode (perRoute or perAccount).
func (input *CreateBillingPackageInput) WithCountMode(mode string) *CreateBillingPackageInput {
	if input == nil {
		return nil
	}

	input.CountMode = &mode

	return input
}

// WithAccountTarget sets the maintenance account target.
func (input *CreateBillingPackageInput) WithAccountTarget(target BillingAccountTarget) *CreateBillingPackageInput {
	if input == nil {
		return nil
	}

	input.AccountTarget = &target

	return input
}

// Validate enforces the SDK trust-boundary preconditions: the required common
// fields, a valid type, and the type-discriminated required fields. It mirrors
// the server's required-field gate (validateVolumeFields / validateMaintenanceFields)
// but not the deeper business invariants (tier overlap/gap, discount range) —
// those are server-authoritative and rejected there.
//
// ponytail: presence + type discrimination only; the tier-overlap/gap math is
// server-owned business logic. Re-implement here only if a caller needs it
// before the round trip.
func (input *CreateBillingPackageInput) Validate() error {
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

	switch input.Type {
	case BillingPackageTypeVolume:
		input.validateVolume(&errs)
	case BillingPackageTypeMaintenance:
		input.validateMaintenance(&errs)
	default:
		errs.Append("type", "must be one of: volume, maintenance")
	}

	return errs.OrNil()
}

func (input *CreateBillingPackageInput) validateVolume(errs *validation.FieldErrors) {
	if input.EventFilter == nil {
		errs.Append("eventFilter", "is required for volume packages")
	}

	if input.PricingModel == nil {
		errs.Append("pricingModel", "is required for volume packages")
	}

	if len(input.Tiers) == 0 {
		errs.Append("tiers", "at least one tier is required for volume packages")
	}

	if !isNonBlankStrPtr(input.AssetCode) {
		errs.Append("assetCode", "is required for volume packages")
	}

	if !isNonBlankStrPtr(input.DebitAccountAlias) {
		errs.Append("debitAccountAlias", "is required for volume packages")
	}

	if !isNonBlankStrPtr(input.CreditAccountAlias) {
		errs.Append("creditAccountAlias", "is required for volume packages")
	}
}

func (input *CreateBillingPackageInput) validateMaintenance(errs *validation.FieldErrors) {
	if !isNonBlankStrPtr(input.FeeAmount) {
		errs.Append("feeAmount", "is required for maintenance packages")
	}

	if !isNonBlankStrPtr(input.AssetCode) {
		errs.Append("assetCode", "is required for maintenance packages")
	}

	if !isNonBlankStrPtr(input.MaintenanceCreditAccount) {
		errs.Append("maintenanceCreditAccount", "is required for maintenance packages")
	}

	if input.AccountTarget == nil {
		errs.Append("accountTarget", "is required for maintenance packages")
	}
}

// isNonBlankStrPtr reports whether a *string is non-nil and non-whitespace.
func isNonBlankStrPtr(s *string) bool {
	return s != nil && strings.TrimSpace(*s) != ""
}

// UpdateBillingPackageInput is the PATCH payload for a billing package. It
// matches the server BillingPackageUpdate DTO (label/description/enable only) and
// MarshalJSON emits only the fields that were set (omit-unset), so an unchanged
// field is never clobbered server-side.
type UpdateBillingPackageInput struct {
	Label       *string `json:"-"`
	Description *string `json:"-"`
	Enable      *bool   `json:"-"`
}

// NewUpdateBillingPackageInput builds an empty billing-package update payload.
func NewUpdateBillingPackageInput() *UpdateBillingPackageInput {
	return &UpdateBillingPackageInput{}
}

// WithLabel sets the label for update.
func (input *UpdateBillingPackageInput) WithLabel(label string) *UpdateBillingPackageInput {
	if input == nil {
		return nil
	}

	input.Label = &label

	return input
}

// WithDescription sets the description for update.
func (input *UpdateBillingPackageInput) WithDescription(description string) *UpdateBillingPackageInput {
	if input == nil {
		return nil
	}

	input.Description = &description

	return input
}

// WithEnable sets the enable flag for update.
func (input *UpdateBillingPackageInput) WithEnable(enable bool) *UpdateBillingPackageInput {
	if input == nil {
		return nil
	}

	input.Enable = &enable

	return input
}

func (input *UpdateBillingPackageInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.Label != nil || input.Description != nil || input.Enable != nil
}

// Validate rejects an empty PATCH (a no-op round trip) and a blank label, matching
// the server BillingPackageUpdate.Validate.
func (input *UpdateBillingPackageInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	if input.Label != nil && strings.TrimSpace(*input.Label) == "" {
		return errors.New("label must not be empty")
	}

	return nil
}

// MarshalJSON emits only the fields that were set, under their server wire names.
func (input *UpdateBillingPackageInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

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

	return json.Marshal(payload)
}

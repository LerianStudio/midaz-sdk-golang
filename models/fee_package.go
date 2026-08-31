// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation"
)

// FeePackage is a fee-package definition returned by the ledger fee endpoints.
// It mirrors the generated genledger.FeePackage read shape.
//
// Money is string, never float: MinimumAmount, MaximumAmount, and every fee
// calculation Value ride the wire as JSON strings and are modeled as string so
// a value like 0.333333333333333333 survives with no precision loss.
type FeePackage struct {
	ID               string         `json:"id"`
	FeeGroupLabel    string         `json:"feeGroupLabel"`
	Description      *string        `json:"description"`
	SegmentID        string         `json:"segmentId"`
	LedgerID         string         `json:"ledgerId"`
	TransactionRoute *string        `json:"transactionRoute"`
	MinimumAmount    string         `json:"minimumAmount"`
	MaximumAmount    string         `json:"maximumAmount"`
	WaivedAccounts   *[]string      `json:"waivedAccounts"`
	Fees             map[string]Fee `json:"fees"`
	Enable           *bool          `json:"enable"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        *time.Time     `json:"deletedAt"`
}

// Fee is a single fee rule within a fee package. It mirrors the generated
// genledger.Fee read shape.
type Fee struct {
	CalculationModel FeeCalculationModel `json:"calculationModel"`
	CreditAccount    string              `json:"creditAccount"`
	FeeLabel         string              `json:"feeLabel"`
	IsDeductibleFrom *bool               `json:"isDeductibleFrom"`
	Priority         *int64              `json:"priority,omitempty"`
	ReferenceAmount  string              `json:"referenceAmount"`
	RouteFrom        *string             `json:"routeFrom,omitempty"`
	RouteTo          *string             `json:"routeTo,omitempty"`
}

// FeeCalculationModel describes how a fee is computed. It mirrors the generated
// genledger.FeeCalculationModel read shape.
type FeeCalculationModel struct {
	ApplicationRule string        `json:"applicationRule"`
	Calculations    []Calculation `json:"calculations"`
}

// Calculation is one calculation entry. Value is a money-adjacent amount and
// rides the wire as a string, never a float.
type Calculation struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// CreatePackageInput is the payload for creating a fee package. It is FLAT and
// byte-for-byte with the server DTO
// (components/ledger/pkg/feeshared/model/create_package_input.go): MinAmount and
// MaxAmount serialize as "minimumAmount"/"maximumAmount", Fee as "fees", and
// Enable is a required pointer.
type CreatePackageInput struct {
	FeeGroupLabel    string         `json:"feeGroupLabel"`
	Description      *string        `json:"description,omitempty"`
	SegmentID        *string        `json:"segmentId"`
	LedgerID         string         `json:"ledgerId"`
	TransactionRoute *string        `json:"transactionRoute,omitempty"`
	MinAmount        string         `json:"minimumAmount"`
	MaxAmount        string         `json:"maximumAmount"`
	WaivedAccounts   *[]string      `json:"waivedAccounts,omitempty"`
	Fee              map[string]Fee `json:"fees"`
	Enable           *bool          `json:"enable"`
}

// NewCreatePackageInput builds a fee-package create payload with the required
// fields set. Enable is required by the server and defaults to false; set it
// explicitly with WithEnable.
func NewCreatePackageInput(feeGroupLabel, ledgerID, minAmount, maxAmount string, fees map[string]Fee) *CreatePackageInput {
	return &CreatePackageInput{
		FeeGroupLabel: feeGroupLabel,
		LedgerID:      ledgerID,
		MinAmount:     minAmount,
		MaxAmount:     maxAmount,
		Fee:           fees,
	}
}

// WithDescription sets the optional package description.
func (input *CreatePackageInput) WithDescription(description string) *CreatePackageInput {
	if input == nil {
		return nil
	}

	input.Description = &description

	return input
}

// WithSegmentID sets the optional segment filter.
func (input *CreatePackageInput) WithSegmentID(segmentID string) *CreatePackageInput {
	if input == nil {
		return nil
	}

	input.SegmentID = &segmentID

	return input
}

// WithTransactionRoute sets the optional transaction route.
func (input *CreatePackageInput) WithTransactionRoute(route string) *CreatePackageInput {
	if input == nil {
		return nil
	}

	input.TransactionRoute = &route

	return input
}

// WithWaivedAccounts sets the accounts exempt from the package fees.
func (input *CreatePackageInput) WithWaivedAccounts(accounts ...string) *CreatePackageInput {
	if input == nil {
		return nil
	}

	clone := append([]string(nil), accounts...)
	input.WaivedAccounts = &clone

	return input
}

// WithEnable sets the required enable flag.
func (input *CreatePackageInput) WithEnable(enable bool) *CreatePackageInput {
	if input == nil {
		return nil
	}

	input.Enable = &enable

	return input
}

// Validate enforces SDK-side preconditions on the create payload: the required
// scalar fields and at least one fee.
func (input *CreatePackageInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	if strings.TrimSpace(input.FeeGroupLabel) == "" {
		errs.Append("feeGroupLabel", "is required")
	}

	if strings.TrimSpace(input.LedgerID) == "" {
		errs.Append("ledgerId", "is required")
	}

	if strings.TrimSpace(input.MinAmount) == "" {
		errs.Append("minimumAmount", "is required")
	}

	if strings.TrimSpace(input.MaxAmount) == "" {
		errs.Append("maximumAmount", "is required")
	}

	if len(input.Fee) == 0 {
		errs.Append("fees", "at least one fee is required")
	}

	if input.Enable == nil {
		errs.Append("enable", "is required")
	}

	// The server create-package handler runs go-playground ValidateStruct on the
	// bound body, and Fee carries validate:"...,dive" (create_package_input.go +
	// feeshared/model/package.go): each inner Fee's struct tags are enforced at the
	// wire, so an incomplete inner fee is an HTTP 400. Mirror that gate here so a
	// caller gets client-side signal instead of a round-trip rejection. This is a
	// PATCH-partial exemption: UpdatePackageInput deliberately does not dive.
	for key, fee := range input.Fee {
		validateFee(&errs, key, fee)
	}

	return errs.OrNil()
}

// validateFee mirrors the server-side struct-tag constraints on a single inner
// Fee (feeshared/model/package.go: Fee + CalculationModel + Calculation) that the
// create-package handler enforces via dive. Keys are namespaced as
// fees[<key>].<field> so the caller can locate the offending fee.
func validateFee(errs *validation.FieldErrors, key string, fee Fee) {
	prefix := "fees[" + key + "]"

	if strings.TrimSpace(fee.FeeLabel) == "" {
		errs.Append(prefix+".feeLabel", "is required")
	}

	// SDK models CalculationModel as a value (not a *pointer like the server), so
	// the server's "required non-nil pointer" maps to a non-zero model here: an
	// empty applicationRule with no calculations is the SDK equivalent of a missing
	// model and is what a nil server pointer would reject.
	if fee.CalculationModel.ApplicationRule == "" && len(fee.CalculationModel.Calculations) == 0 {
		errs.Append(prefix+".calculationModel", "is required")
	} else {
		validateCalculationModel(errs, prefix, fee.CalculationModel)
	}

	switch fee.ReferenceAmount {
	case "originalAmount", "afterFeesAmount":
	default:
		errs.Append(prefix+".referenceAmount", "must be one of: originalAmount, afterFeesAmount")
	}

	if fee.IsDeductibleFrom == nil {
		errs.Append(prefix+".isDeductibleFrom", "is required")
	}

	if strings.TrimSpace(fee.CreditAccount) == "" {
		errs.Append(prefix+".creditAccount", "is required")
	}

	if fee.Priority != nil && *fee.Priority < 0 {
		errs.Append(prefix+".priority", "must be greater than or equal to 0")
	}
}

// validateCalculationModel mirrors the server CalculationModel + Calculation tags:
// applicationRule is a oneof, each calculation type is a oneof, and each value is
// required.
func validateCalculationModel(errs *validation.FieldErrors, prefix string, model FeeCalculationModel) {
	switch model.ApplicationRule {
	case "maxBetweenTypes", "flatFee", "percentual":
	default:
		errs.Append(prefix+".calculationModel.applicationRule", "must be one of: maxBetweenTypes, flatFee, percentual")
	}

	for i, calc := range model.Calculations {
		calcPrefix := prefix + ".calculationModel.calculations[" + strconv.Itoa(i) + "]"

		switch calc.Type {
		case "percentage", "flat":
		default:
			errs.Append(calcPrefix+".type", "must be one of: percentage, flat")
		}

		if strings.TrimSpace(calc.Value) == "" {
			errs.Append(calcPrefix+".value", "is required")
		}
	}
}

// UpdatePackageInput is the PATCH payload for a fee package. MarshalJSON emits
// only the fields that were set (omit-unset), so an unchanged field is never
// clobbered server-side.
type UpdatePackageInput struct {
	FeeGroupLabel    *string        `json:"-"`
	Description      *string        `json:"-"`
	SegmentID        *string        `json:"-"`
	TransactionRoute *string        `json:"-"`
	MinAmount        *string        `json:"-"`
	MaxAmount        *string        `json:"-"`
	WaivedAccounts   *[]string      `json:"-"`
	Fee              map[string]Fee `json:"-"`
	Enable           *bool          `json:"-"`
}

// NewUpdatePackageInput builds an empty fee-package update payload.
func NewUpdatePackageInput() *UpdatePackageInput {
	return &UpdatePackageInput{}
}

// WithFeeGroupLabel sets the fee-group label for update.
func (input *UpdatePackageInput) WithFeeGroupLabel(label string) *UpdatePackageInput {
	if input == nil {
		return nil
	}

	input.FeeGroupLabel = &label

	return input
}

// WithDescription sets the description for update.
func (input *UpdatePackageInput) WithDescription(description string) *UpdatePackageInput {
	if input == nil {
		return nil
	}

	input.Description = &description

	return input
}

// WithSegmentID sets the segment filter for update.
func (input *UpdatePackageInput) WithSegmentID(segmentID string) *UpdatePackageInput {
	if input == nil {
		return nil
	}

	input.SegmentID = &segmentID

	return input
}

// WithTransactionRoute sets the transaction route for update.
func (input *UpdatePackageInput) WithTransactionRoute(route string) *UpdatePackageInput {
	if input == nil {
		return nil
	}

	input.TransactionRoute = &route

	return input
}

// WithMinAmount sets the minimum amount for update.
func (input *UpdatePackageInput) WithMinAmount(amount string) *UpdatePackageInput {
	if input == nil {
		return nil
	}

	input.MinAmount = &amount

	return input
}

// WithMaxAmount sets the maximum amount for update.
func (input *UpdatePackageInput) WithMaxAmount(amount string) *UpdatePackageInput {
	if input == nil {
		return nil
	}

	input.MaxAmount = &amount

	return input
}

// WithWaivedAccounts sets the waived accounts for update.
func (input *UpdatePackageInput) WithWaivedAccounts(accounts ...string) *UpdatePackageInput {
	if input == nil {
		return nil
	}

	clone := append([]string(nil), accounts...)
	input.WaivedAccounts = &clone

	return input
}

// WithFees sets the fee map for update.
func (input *UpdatePackageInput) WithFees(fees map[string]Fee) *UpdatePackageInput {
	if input == nil {
		return nil
	}

	input.Fee = fees

	return input
}

// WithEnable sets the enable flag for update.
func (input *UpdatePackageInput) WithEnable(enable bool) *UpdatePackageInput {
	if input == nil {
		return nil
	}

	input.Enable = &enable

	return input
}

func (input *UpdatePackageInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.FeeGroupLabel != nil ||
		input.Description != nil ||
		input.SegmentID != nil ||
		input.TransactionRoute != nil ||
		input.MinAmount != nil ||
		input.MaxAmount != nil ||
		input.WaivedAccounts != nil ||
		input.Fee != nil ||
		input.Enable != nil
}

// Validate rejects an empty PATCH; an empty update payload is a no-op round trip.
func (input *UpdatePackageInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	return nil
}

// MarshalJSON emits only the fields that were set, under their server wire names.
func (input *UpdatePackageInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	payload := map[string]any{}

	if input.FeeGroupLabel != nil {
		payload["feeGroupLabel"] = input.FeeGroupLabel
	}

	if input.Description != nil {
		payload["description"] = input.Description
	}

	if input.SegmentID != nil {
		payload["segmentId"] = input.SegmentID
	}

	if input.TransactionRoute != nil {
		payload["transactionRoute"] = input.TransactionRoute
	}

	if input.MinAmount != nil {
		payload["minimumAmount"] = input.MinAmount
	}

	if input.MaxAmount != nil {
		payload["maximumAmount"] = input.MaxAmount
	}

	if input.WaivedAccounts != nil {
		payload["waivedAccounts"] = input.WaivedAccounts
	}

	if input.Fee != nil {
		payload["fees"] = input.Fee
	}

	if input.Enable != nil {
		payload["enable"] = input.Enable
	}

	return json.Marshal(payload)
}

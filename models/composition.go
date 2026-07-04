// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"errors"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation/core"
)

// CreateHolderAccountInput is the composite request body for opening a
// holder-owned account and, optionally, its instrument in a single call.
//
// The field names and json tags MIRROR the server pkg/mmodel.CreateHolderAccountInput
// byte-for-byte. holderId is intentionally NOT on the body: the holder is
// sourced from the path parameter, never a body value.
//
// The server writes an instrument if and only if ANY of BankingDetails,
// RegulatoryFields, or RelatedParties is present. There is no nested
// "instrument" object: the presence of any of those top-level fields is the
// switch. When none are present the composition writes no instrument.
type CreateHolderAccountInput struct {
	Name             string            `json:"name"`
	ParentAccountID  *string           `json:"parentAccountId"`
	EntityID         *string           `json:"entityId"`
	AssetCode        string            `json:"assetCode"`
	PortfolioID      *string           `json:"portfolioId"`
	SegmentID        *string           `json:"segmentId"`
	Status           Status            `json:"status"`
	Alias            *string           `json:"alias"`
	Type             string            `json:"type"`
	Blocked          *bool             `json:"blocked"`
	Metadata         map[string]any    `json:"metadata"`
	Skip             *AccountSkip      `json:"skip,omitempty"`
	BankingDetails   *BankingDetails   `json:"bankingDetails,omitempty"`
	RegulatoryFields *RegulatoryFields `json:"regulatoryFields,omitempty"`
	RelatedParties   []*RelatedParty   `json:"relatedParties,omitempty"`
}

// AccountSkip carries the per-call control skips honored on the account create
// path. A skip is honored only when the request sets it AND the ledger opts in
// via its matching override; a skip requested without the override is rejected
// with HTTP 422.
type AccountSkip struct {
	// Holder skips the holder existence check on account creation.
	Holder bool `json:"holder,omitempty"`
}

// Validate validates the CreateHolderAccountInput fields. AssetCode and Type
// are required; the instrument fields are optional.
func (input *CreateHolderAccountInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	if strings.TrimSpace(input.AssetCode) == "" {
		errs.Append("assetCode", "is required")
	}

	if strings.TrimSpace(input.Type) == "" {
		errs.Append("type", "is required")
	}

	if len(input.Metadata) > 0 {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	if err := validateRelatedParties(input.RelatedParties); err != nil {
		errs.Append("relatedParties", err.Error())
	}

	return errs.OrNil()
}

// NewCreateHolderAccountInput builds a CreateHolderAccountInput with the required
// asset code and account type. Optional account and instrument fields are set
// with the With* methods; the composition writes an instrument if and only if
// any of WithBankingDetails/WithRegulatoryFields/WithRelatedParties is set.
func NewCreateHolderAccountInput(assetCode, accountType string) *CreateHolderAccountInput {
	return &CreateHolderAccountInput{AssetCode: assetCode, Type: accountType}
}

// WithName sets the account name.
func (input *CreateHolderAccountInput) WithName(name string) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.Name = name

	return input
}

// WithParentAccountID sets the parent account ID.
func (input *CreateHolderAccountInput) WithParentAccountID(id string) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.ParentAccountID = &id

	return input
}

// WithEntityID sets the entity ID.
func (input *CreateHolderAccountInput) WithEntityID(id string) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.EntityID = &id

	return input
}

// WithPortfolioID sets the portfolio ID.
func (input *CreateHolderAccountInput) WithPortfolioID(id string) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.PortfolioID = &id

	return input
}

// WithSegmentID sets the segment ID.
func (input *CreateHolderAccountInput) WithSegmentID(id string) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.SegmentID = &id

	return input
}

// WithStatus sets the account status.
func (input *CreateHolderAccountInput) WithStatus(status Status) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

// WithAlias sets the account alias.
func (input *CreateHolderAccountInput) WithAlias(alias string) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.Alias = &alias

	return input
}

// WithBlocked sets the account blocked flag.
func (input *CreateHolderAccountInput) WithBlocked(blocked bool) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.Blocked = &blocked

	return input
}

// WithMetadata sets the account metadata.
func (input *CreateHolderAccountInput) WithMetadata(metadata map[string]any) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithSkip sets the per-call control skips (honored only with the matching ledger override).
func (input *CreateHolderAccountInput) WithSkip(skip *AccountSkip) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.Skip = skip

	return input
}

// WithBankingDetails sets the instrument banking details (triggers an instrument write).
func (input *CreateHolderAccountInput) WithBankingDetails(bankingDetails *BankingDetails) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.BankingDetails = bankingDetails

	return input
}

// WithRegulatoryFields sets the instrument regulatory fields (triggers an instrument write).
func (input *CreateHolderAccountInput) WithRegulatoryFields(regulatoryFields *RegulatoryFields) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.RegulatoryFields = regulatoryFields

	return input
}

// WithRelatedParties sets the instrument related parties (triggers an instrument write).
func (input *CreateHolderAccountInput) WithRelatedParties(relatedParties []*RelatedParty) *CreateHolderAccountInput {
	if input == nil {
		return nil
	}

	input.RelatedParties = cloneRelatedParties(relatedParties)

	return input
}

// HolderAccountResponse is the composite response for opening a holder-owned
// account. Account is always present on success. Instrument is null when none
// was requested (account-only path) or when the instrument write failed.
// InstrumentError is set ONLY when the account committed but the instrument
// write failed: the account remains persisted and usable (no compensating
// delete), and the failure is surfaced for client-driven retry.
//
// All three fields are pointers to preserve the null/absent distinction on the
// wire.
type HolderAccountResponse struct {
	// Account is the account that was created (always present on success).
	Account *Account `json:"account"`

	// Instrument is the instrument that was created, or null when none was
	// requested or the instrument write failed.
	Instrument *Instrument `json:"instrument"`

	// InstrumentError is the typed failure block, set only when the account
	// committed but the instrument write failed. Omitted on full success and on
	// the account-only path.
	InstrumentError *InstrumentFailure `json:"instrumentError,omitempty"`
}

// InstrumentFailure is the typed partial-failure block surfaced when the
// account committed but the instrument write failed. Reason is a stable,
// client-actionable code (never raw internal error text).
type InstrumentFailure struct {
	// Status is the outcome status of the instrument write (e.g. FAILED).
	Status string `json:"status"`

	// Reason is a stable, client-actionable reason code for the failure.
	Reason string `json:"reason"`
}

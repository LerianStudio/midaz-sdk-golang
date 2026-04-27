// Package models defines the data models used by the Midaz SDK.
package models

import (
	"errors"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

// Ledger is an alias for mmodel.Ledger to maintain compatibility while using midaz entities.
type Ledger = mmodel.Ledger

// LedgerSettings is an alias for the Midaz ledger settings payload.
type LedgerSettings = mmodel.LedgerSettings

// UpdateLedgerSettingsAccountingInput is the partial accounting settings payload.
type UpdateLedgerSettingsAccountingInput struct {
	ValidateAccountType *bool `json:"validateAccountType,omitempty"`
	ValidateRoutes      *bool `json:"validateRoutes,omitempty"`
}

// UpdateLedgerSettingsInput is the partial ledger settings patch payload.
type UpdateLedgerSettingsInput struct {
	Accounting *UpdateLedgerSettingsAccountingInput `json:"accounting,omitempty"`
}

// NewUpdateLedgerSettingsInput creates a new empty ledger settings patch.
func NewUpdateLedgerSettingsInput() *UpdateLedgerSettingsInput {
	return &UpdateLedgerSettingsInput{}
}

// WithValidateAccountType sets the validateAccountType accounting flag.
func (input *UpdateLedgerSettingsInput) WithValidateAccountType(enabled bool) *UpdateLedgerSettingsInput {
	if input.Accounting == nil {
		input.Accounting = &UpdateLedgerSettingsAccountingInput{}
	}

	input.Accounting.ValidateAccountType = &enabled

	return input
}

// WithValidateRoutes sets the validateRoutes accounting flag.
func (input *UpdateLedgerSettingsInput) WithValidateRoutes(enabled bool) *UpdateLedgerSettingsInput {
	if input.Accounting == nil {
		input.Accounting = &UpdateLedgerSettingsAccountingInput{}
	}

	input.Accounting.ValidateRoutes = &enabled

	return input
}

// CreateLedgerInput wraps mmodel.CreateLedgerInput to maintain compatibility while using midaz entities.
type CreateLedgerInput struct {
	mmodel.CreateLedgerInput
}

// UpdateLedgerInput wraps mmodel.UpdateLedgerInput to maintain compatibility while using midaz entities.
type UpdateLedgerInput struct {
	mmodel.UpdateLedgerInput
}

// NewCreateLedgerInput creates a new CreateLedgerInput with required fields.
func NewCreateLedgerInput(name string) *CreateLedgerInput {
	return &CreateLedgerInput{
		CreateLedgerInput: mmodel.CreateLedgerInput{
			Name: name,
		},
	}
}

// WithStatus sets the status.
func (input *CreateLedgerInput) WithStatus(status Status) *CreateLedgerInput {
	input.Status = status
	return input
}

// WithMetadata sets the metadata.
func (input *CreateLedgerInput) WithMetadata(metadata map[string]any) *CreateLedgerInput {
	input.Metadata = metadata
	return input
}

// Validate validates the CreateLedgerInput fields.
func (input *CreateLedgerInput) Validate() error {
	if input.Name == "" {
		return errors.New("name is required")
	}

	return nil
}

// NewUpdateLedgerInput creates a new UpdateLedgerInput.
func NewUpdateLedgerInput() *UpdateLedgerInput {
	return &UpdateLedgerInput{
		UpdateLedgerInput: mmodel.UpdateLedgerInput{},
	}
}

// WithName sets the name for UpdateLedgerInput.
func (input *UpdateLedgerInput) WithName(name string) *UpdateLedgerInput {
	input.Name = name
	return input
}

// WithStatus sets the status for UpdateLedgerInput.
func (input *UpdateLedgerInput) WithStatus(status Status) *UpdateLedgerInput {
	input.Status = status
	return input
}

// WithMetadata sets the metadata for UpdateLedgerInput.
func (input *UpdateLedgerInput) WithMetadata(metadata map[string]any) *UpdateLedgerInput {
	input.Metadata = metadata
	return input
}

// Validate validates the UpdateLedgerInput fields.
func (*UpdateLedgerInput) Validate() error {
	// For update operations, most fields are optional
	return nil
}

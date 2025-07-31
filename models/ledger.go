// Package models defines the data models used by the Midaz SDK.
package models

import (
	"fmt"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

// Ledger is an alias for mmodel.Ledger to maintain compatibility while using midaz entities.
type Ledger = mmodel.Ledger

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
		return fmt.Errorf("name is required")
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
func (input *UpdateLedgerInput) Validate() error {
	// For update operations, most fields are optional
	return nil
}
// Package models defines the data models used by the Midaz SDK.
package models

import (
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

// Balance is an alias for mmodel.Balance to maintain compatibility while using midaz entities.
type Balance = mmodel.Balance

// UpdateBalanceInput is the input for updating a balance.
// This structure contains the fields that can be modified when updating an existing balance.
type UpdateBalanceInput struct {
	// Metadata contains additional custom data associated with the balance.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Validate validates the UpdateBalanceInput fields.
func (input *UpdateBalanceInput) Validate() error {
	// For balance updates, validation is minimal since most fields are controlled by the system
	return nil
}

// NewUpdateBalanceInput creates a new UpdateBalanceInput.
func NewUpdateBalanceInput() *UpdateBalanceInput {
	return &UpdateBalanceInput{}
}

// WithMetadata sets the metadata for UpdateBalanceInput.
func (input *UpdateBalanceInput) WithMetadata(metadata map[string]any) *UpdateBalanceInput {
	input.Metadata = metadata
	return input
}

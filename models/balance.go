// Package models defines the data models used by the Midaz SDK.
package models

import (
	"errors"

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
func (*UpdateBalanceInput) Validate() error {
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

// CreateBalanceInput is the input for creating an additional balance for an account.
// This allows an account to have multiple balance entries (e.g., for different purposes like frozen funds).
type CreateBalanceInput struct {
	// Key is the unique identifier for this balance within the account
	Key string `json:"key"`

	// AllowSending indicates whether this balance can be used for outgoing transactions
	AllowSending *bool `json:"allowSending,omitempty"`

	// AllowReceiving indicates whether this balance can receive incoming transactions
	AllowReceiving *bool `json:"allowReceiving,omitempty"`
}

// NewCreateBalanceInput creates a new CreateBalanceInput with the required key.
func NewCreateBalanceInput(key string) *CreateBalanceInput {
	return &CreateBalanceInput{
		Key: key,
	}
}

// WithAllowSending sets whether this balance can be used for outgoing transactions.
func (input *CreateBalanceInput) WithAllowSending(allow bool) *CreateBalanceInput {
	input.AllowSending = &allow
	return input
}

// WithAllowReceiving sets whether this balance can receive incoming transactions.
func (input *CreateBalanceInput) WithAllowReceiving(allow bool) *CreateBalanceInput {
	input.AllowReceiving = &allow
	return input
}

// Validate validates the CreateBalanceInput fields.
func (input *CreateBalanceInput) Validate() error {
	if input.Key == "" {
		return errors.New("key is required")
	}

	return nil
}

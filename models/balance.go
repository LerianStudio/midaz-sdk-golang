// Package models defines the data models used by the Midaz SDK.
package models

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
	"github.com/shopspring/decimal"
)

// Balance is an alias for mmodel.Balance to maintain compatibility while using midaz entities.
type Balance = mmodel.Balance

// BalanceHistory is an alias for mmodel.BalanceHistory.
type BalanceHistory = mmodel.BalanceHistory

const (
	// BalanceScopeTransactional identifies a balance that participates in transactions.
	BalanceScopeTransactional = "transactional"
	// BalanceScopeInternal identifies a system-managed internal balance.
	BalanceScopeInternal = "internal"
)

// BalanceSettings captures optional per-balance overdraft and scope settings.
type BalanceSettings struct {
	BalanceScope          string  `json:"balanceScope,omitempty"`
	AllowOverdraft        bool    `json:"allowOverdraft"`
	OverdraftLimitEnabled bool    `json:"overdraftLimitEnabled"`
	OverdraftLimit        *string `json:"overdraftLimit,omitempty"`
}

// NewDefaultBalanceSettings returns balance settings initialized with safe defaults.
func NewDefaultBalanceSettings() *BalanceSettings {
	return &BalanceSettings{BalanceScope: BalanceScopeTransactional}
}

// Validate enforces the balance settings contract.
func (s *BalanceSettings) Validate() error {
	if s == nil {
		return nil
	}

	switch s.BalanceScope {
	case "", BalanceScopeTransactional, BalanceScopeInternal:
	default:
		return fmt.Errorf("balanceScope must be %q or %q", BalanceScopeTransactional, BalanceScopeInternal)
	}

	if s.OverdraftLimitEnabled && !s.AllowOverdraft {
		return errors.New("allowOverdraft must be true when overdraftLimitEnabled is true")
	}

	if !s.OverdraftLimitEnabled {
		if s.OverdraftLimit != nil {
			return errors.New("overdraftLimit must be omitted when overdraftLimitEnabled is false")
		}

		return nil
	}

	if s.OverdraftLimit == nil || *s.OverdraftLimit == "" {
		return errors.New("overdraftLimit is required when overdraftLimitEnabled is true")
	}

	limit, err := decimal.NewFromString(*s.OverdraftLimit)
	if err != nil || !limit.IsPositive() {
		return errors.New("overdraftLimit must be a positive decimal")
	}

	return nil
}

// UpdateBalanceInput is the input for updating a balance.
// This structure contains the fields that can be modified when updating an existing balance.
type UpdateBalanceInput struct {
	// AllowSending controls whether this balance can be used for outgoing transactions.
	AllowSending *bool `json:"allowSending,omitempty"`

	// AllowReceiving controls whether this balance can receive incoming transactions.
	AllowReceiving *bool `json:"allowReceiving,omitempty"`

	// Metadata is retained for backward compatibility, but is not part of the
	// current Midaz UpdateBalance contract.
	Metadata map[string]any `json:"-"`

	// Settings controls overdraft and balance scope behavior.
	Settings *BalanceSettings `json:"settings,omitempty"`
}

// Validate validates the UpdateBalanceInput fields.
func (input *UpdateBalanceInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.Metadata != nil {
		return errors.New("metadata updates are no longer supported in UpdateBalanceInput")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	if input.Settings != nil {
		return input.Settings.Validate()
	}

	return nil
}

func (input *UpdateBalanceInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.AllowSending != nil || input.AllowReceiving != nil || input.Settings != nil
}

// NewUpdateBalanceInput creates a new UpdateBalanceInput.
func NewUpdateBalanceInput() *UpdateBalanceInput {
	return &UpdateBalanceInput{}
}

// WithAllowSending sets whether this balance can be used for outgoing transactions.
func (input *UpdateBalanceInput) WithAllowSending(allow bool) *UpdateBalanceInput {
	if input == nil {
		return nil
	}

	input.AllowSending = &allow

	return input
}

// WithAllowReceiving sets whether this balance can receive incoming transactions.
func (input *UpdateBalanceInput) WithAllowReceiving(allow bool) *UpdateBalanceInput {
	if input == nil {
		return nil
	}

	input.AllowReceiving = &allow

	return input
}

// WithMetadata sets legacy metadata data on UpdateBalanceInput.
// Deprecated: metadata is not sent to the Midaz UpdateBalance endpoint.
func (input *UpdateBalanceInput) WithMetadata(metadata map[string]any) *UpdateBalanceInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithSettings sets per-balance overdraft and scope settings.
func (input *UpdateBalanceInput) WithSettings(settings *BalanceSettings) *UpdateBalanceInput {
	if input == nil {
		return nil
	}

	input.Settings = settings

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

	// Direction is the accounting direction of the balance, such as "credit" or "debit".
	Direction *string `json:"direction,omitempty"`

	// Settings controls overdraft and balance scope behavior.
	Settings *BalanceSettings `json:"settings,omitempty"`
}

// NewCreateBalanceInput creates a new CreateBalanceInput with the required key.
func NewCreateBalanceInput(key string) *CreateBalanceInput {
	return &CreateBalanceInput{
		Key: key,
	}
}

// WithAllowSending sets whether this balance can be used for outgoing transactions.
func (input *CreateBalanceInput) WithAllowSending(allow bool) *CreateBalanceInput {
	if input == nil {
		return nil
	}

	input.AllowSending = &allow

	return input
}

// WithAllowReceiving sets whether this balance can receive incoming transactions.
func (input *CreateBalanceInput) WithAllowReceiving(allow bool) *CreateBalanceInput {
	if input == nil {
		return nil
	}

	input.AllowReceiving = &allow

	return input
}

// WithDirection sets the accounting direction for the balance.
func (input *CreateBalanceInput) WithDirection(direction string) *CreateBalanceInput {
	if input == nil {
		return nil
	}

	input.Direction = &direction

	return input
}

// WithSettings sets per-balance overdraft and scope settings.
func (input *CreateBalanceInput) WithSettings(settings *BalanceSettings) *CreateBalanceInput {
	if input == nil {
		return nil
	}

	input.Settings = settings

	return input
}

// Validate validates the CreateBalanceInput fields.
func (input *CreateBalanceInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.Key == "" {
		return errors.New("key is required")
	}

	if input.Settings != nil {
		return input.Settings.Validate()
	}

	return nil
}

// MarshalJSON ensures zero-value account collections encode items as an empty array.
func (a Accounts) MarshalJSON() ([]byte, error) {
	type alias Accounts

	out := alias(a)
	if out.Items == nil {
		out.Items = []Account{}
	}

	return json.Marshal(out)
}

// MarshalJSON ensures zero-value account collections encode items as an empty array.
func (r ListAccountResponse) MarshalJSON() ([]byte, error) {
	type alias ListAccountResponse

	out := alias(r)
	if out.Items == nil {
		out.Items = []Account{}
	}

	return json.Marshal(out)
}

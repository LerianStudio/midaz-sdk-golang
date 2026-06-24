package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation"
	"github.com/shopspring/decimal"
)

// Balance is the SDK-native balance response type (Track 7E — audit 7.1).
type Balance struct {
	ID             string          `json:"id" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	OrganizationID string          `json:"organizationId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	LedgerID       string          `json:"ledgerId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	AccountID      string          `json:"accountId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	Alias          string          `json:"alias" example:"@person1" maxLength:"256"`
	Key            string          `json:"key" example:"asset-freeze" maxLength:"100"`
	AssetCode      string          `json:"assetCode" example:"USD" minLength:"2" maxLength:"10"`
	Available      decimal.Decimal `json:"available" example:"1500" minimum:"0"`
	OnHold         decimal.Decimal `json:"onHold" example:"500" minimum:"0"`
	Version        int64           `json:"version" example:"1" minimum:"1"`
	AccountType    string          `json:"accountType" example:"creditCard" maxLength:"50"`
	Direction      string          `json:"direction,omitempty" example:"credit"`
	AllowSending   bool            `json:"allowSending" example:"true"`
	AllowReceiving bool            `json:"allowReceiving" example:"true"`
	CreatedAt      time.Time       `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	UpdatedAt      time.Time       `json:"updatedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	DeletedAt      *time.Time      `json:"deletedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	Metadata       map[string]any  `json:"metadata,omitempty"`
}

// BalanceHistory is the SDK-native balance history response type.
type BalanceHistory struct {
	ID             string          `json:"id" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	OrganizationID string          `json:"organizationId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	LedgerID       string          `json:"ledgerId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	AccountID      string          `json:"accountId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	Alias          string          `json:"alias" example:"@person1" maxLength:"256"`
	Key            string          `json:"key" example:"asset-freeze" maxLength:"100"`
	AssetCode      string          `json:"assetCode" example:"USD" minLength:"2" maxLength:"10"`
	Available      decimal.Decimal `json:"available" example:"1500" minimum:"0"`
	OnHold         decimal.Decimal `json:"onHold" example:"500" minimum:"0"`
	Version        int64           `json:"version" example:"1" minimum:"1"`
	AccountType    string          `json:"accountType" example:"creditCard" maxLength:"50"`
	Direction      string          `json:"direction,omitempty" example:"credit"`
	AllowSending   bool            `json:"allowSending" example:"true"`
	AllowReceiving bool            `json:"allowReceiving" example:"true"`
	CreatedAt      time.Time       `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	UpdatedAt      time.Time       `json:"updatedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	DeletedAt      *time.Time      `json:"deletedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	Metadata       map[string]any  `json:"metadata,omitempty"`
}

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

	var errs validation.FieldErrors

	switch s.BalanceScope {
	case "", BalanceScopeTransactional, BalanceScopeInternal:
	default:
		errs.Append("balanceScope", fmt.Sprintf("must be %q or %q", BalanceScopeTransactional, BalanceScopeInternal))
	}

	if s.OverdraftLimitEnabled && !s.AllowOverdraft {
		errs.Append("allowOverdraft", "must be true when overdraftLimitEnabled is true")
	}

	switch {
	case !s.OverdraftLimitEnabled:
		if s.OverdraftLimit != nil {
			errs.Append("overdraftLimit", "must be omitted when overdraftLimitEnabled is false")
		}
	case s.OverdraftLimit == nil || *s.OverdraftLimit == "":
		errs.Append("overdraftLimit", "is required when overdraftLimitEnabled is true")
	default:
		limit, err := decimal.NewFromString(*s.OverdraftLimit)
		if err != nil || !limit.IsPositive() {
			errs.Append("overdraftLimit", "must be a positive decimal")
		}
	}

	return errs.OrNil()
}

// UpdateBalanceInput is the input for updating a balance.
// This structure contains the fields that can be modified when updating an existing balance.
type UpdateBalanceInput struct {
	// AllowSending controls whether this balance can be used for outgoing transactions.
	AllowSending *bool `json:"allowSending,omitempty"`

	// AllowReceiving controls whether this balance can receive incoming transactions.
	AllowReceiving *bool `json:"allowReceiving,omitempty"`

	// Metadata is retained for backward compatibility only. It is not part of the
	// current Midaz UpdateBalance contract, is never serialized, and validation
	// rejects any attempt to set it.
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
// Deprecated: metadata is unsupported by the current Midaz UpdateBalance
// endpoint. The value is not sent and Validate rejects inputs that set it.
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

	var errs validation.FieldErrors

	if input.Key == "" {
		errs.Append("key", "is required")
	}

	if input.Settings != nil {
		if err := input.Settings.Validate(); err != nil {
			// Settings.Validate already returns a FieldErrors with
			// the per-field messages. Surface its rendered form
			// under the "settings" namespace so callers see the
			// hierarchical context.
			errs.Append("settings", strings.TrimPrefix(err.Error(), "validation failed: "))
		}
	}

	return errs.OrNil()
}

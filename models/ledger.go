// Package models defines the data models used by the Midaz SDK.
package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
)

const maxLedgerNameLength = 256

// Ledger is the SDK-native ledger response type (Track 7E — audit 7.1).
type Ledger struct {
	ID             string          `json:"id" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	Name           string          `json:"name" example:"Treasury Operations" maxLength:"256"`
	OrganizationID string          `json:"organizationId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	Status         Status          `json:"status"`
	CreatedAt      time.Time       `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	UpdatedAt      time.Time       `json:"updatedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	DeletedAt      *time.Time      `json:"deletedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	Metadata       map[string]any  `json:"metadata,omitempty"`
	Settings       *LedgerSettings `json:"settings,omitempty"`
}

// LedgerSettings is the SDK-native ledger settings response type.
type LedgerSettings struct {
	Accounting AccountingValidation `json:"accounting"`
}

// AccountingValidation is the accounting-related validation settings struct.
type AccountingValidation struct {
	ValidateAccountType bool `json:"validateAccountType"`
	ValidateRoutes      bool `json:"validateRoutes"`
}

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
	if input == nil {
		return nil
	}

	if input.Accounting == nil {
		input.Accounting = &UpdateLedgerSettingsAccountingInput{}
	}

	input.Accounting.ValidateAccountType = &enabled

	return input
}

// WithValidateRoutes sets the validateRoutes accounting flag.
func (input *UpdateLedgerSettingsInput) WithValidateRoutes(enabled bool) *UpdateLedgerSettingsInput {
	if input == nil {
		return nil
	}

	if input.Accounting == nil {
		input.Accounting = &UpdateLedgerSettingsAccountingInput{}
	}

	input.Accounting.ValidateRoutes = &enabled

	return input
}

// hasChanges reports whether any field on the patch has been set.
func (input *UpdateLedgerSettingsInput) hasChanges() bool {
	if input == nil || input.Accounting == nil {
		return false
	}

	return input.Accounting.ValidateAccountType != nil ||
		input.Accounting.ValidateRoutes != nil
}

// Validate validates the UpdateLedgerSettingsInput fields.
//
// An empty patch (no setters invoked) is rejected. The server treats an
// empty PATCH as a no-op which would silently mask a programming error,
// so the SDK refuses to send one.
func (input *UpdateLedgerSettingsInput) Validate() error {
	if input == nil {
		return errors.New("input is required")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	return nil
}

// CreateLedgerInput is the SDK-native ledger creation payload.
type CreateLedgerInput struct {
	Name     string          `json:"name" maxLength:"256"`
	Status   Status          `json:"status"`
	Metadata map[string]any  `json:"metadata"`
	Settings *LedgerSettings `json:"settings,omitempty"`
}

// UpdateLedgerInput is the SDK-native ledger patch payload.
type UpdateLedgerInput struct {
	Name     string         `json:"name" example:"Treasury Operations Global" maxLength:"256"`
	Status   Status         `json:"status"`
	Metadata map[string]any `json:"metadata"`
}

// NewCreateLedgerInput creates a new CreateLedgerInput with required fields.
func NewCreateLedgerInput(name string) *CreateLedgerInput {
	return &CreateLedgerInput{
		Name: name,
	}
}

// WithStatus sets the status.
func (input *CreateLedgerInput) WithStatus(status Status) *CreateLedgerInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

// WithMetadata sets the metadata.
func (input *CreateLedgerInput) WithMetadata(metadata map[string]any) *CreateLedgerInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// Validate validates the CreateLedgerInput fields.
func (input *CreateLedgerInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return errors.New("name is required")
	}

	if len(name) > maxLedgerNameLength {
		return fmt.Errorf("name must be at most %d characters", maxLedgerNameLength)
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

// MarshalJSON omits optional create fields when callers leave them unset.
func (input *CreateLedgerInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "name", input.Name)
	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	if input.Settings != nil {
		fields["settings"] = input.Settings
	}

	return json.Marshal(fields)
}

// NewUpdateLedgerInput creates a new UpdateLedgerInput.
func NewUpdateLedgerInput() *UpdateLedgerInput {
	return &UpdateLedgerInput{}
}

// WithName sets the name for UpdateLedgerInput.
func (input *UpdateLedgerInput) WithName(name string) *UpdateLedgerInput {
	if input == nil {
		return nil
	}

	input.Name = name

	return input
}

// WithStatus sets the status for UpdateLedgerInput.
func (input *UpdateLedgerInput) WithStatus(status Status) *UpdateLedgerInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

// WithMetadata sets the metadata for UpdateLedgerInput.
func (input *UpdateLedgerInput) WithMetadata(metadata map[string]any) *UpdateLedgerInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// Validate validates the UpdateLedgerInput fields.
func (input *UpdateLedgerInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.Name != "" && strings.TrimSpace(input.Name) == "" {
		return errors.New("name must not be blank")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	if len(strings.TrimSpace(input.Name)) > maxLedgerNameLength {
		return fmt.Errorf("name must be at most %d characters", maxLedgerNameLength)
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

func (input *UpdateLedgerInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return strings.TrimSpace(input.Name) != "" || !IsStatusEmpty(input.Status) || input.Metadata != nil
}

// MarshalJSON emits only fields explicitly set on the SDK PATCH input.
func (input *UpdateLedgerInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "name", input.Name)
	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

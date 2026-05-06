// Package models defines the data models used by the Midaz SDK.
package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

const maxLedgerNameLength = 256

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
	return &UpdateLedgerInput{
		UpdateLedgerInput: mmodel.UpdateLedgerInput{},
	}
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

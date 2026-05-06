// Package models defines the data models used by the Midaz SDK.
package models

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

const (
	maxAccountTypeNameLength        = 100
	maxAccountTypeDescriptionLength = 500
	maxAccountTypeKeyValueLength    = 50
)

// AccountType represents an account type in the Midaz Ledger.
// Account types define templates or categories for accounts, specifying
// their behavior and characteristics within the ledger system.
//
// AccountTypes provide a way to standardize and categorize accounts by defining:
//   - Name: Human-readable name for the account type
//   - Description: Detailed description of the account type's purpose
//   - KeyValue: Unique identifier within the organization/ledger
//   - Metadata: Custom attributes for account type configuration
//
// Example Usage:
//
//	// Create a cash account type
//	cashType := &models.AccountType{
//	    ID:          "at-123",
//	    Name:        "Cash Account",
//	    Description: "Account type for liquid assets held in cash or cash equivalents.",
//	    KeyValue:    "CASH",
//	    Metadata: map[string]any{
//	        "category":    "liquid_assets",
//	        "risk_level":  "low",
//	        "currency":    "USD",
//	    },
//	}
//
//	// Create a receivables account type
//	receivablesType := &models.AccountType{
//	    ID:          "at-456",
//	    Name:        "Accounts Receivable",
//	    Description: "Account type for amounts owed by customers.",
//	    KeyValue:    "AR",
//	    Metadata: map[string]any{
//	        "category":        "receivables",
//	        "aging_required":  true,
//	        "credit_terms":    "net30",
//	    },
//	}
//
// AccountType is an alias for mmodel.AccountType to maintain compatibility while using midaz entities.
type AccountType = mmodel.AccountType

// CreateAccountTypeInput wraps mmodel.CreateAccountTypeInput to maintain compatibility while using midaz entities.
type CreateAccountTypeInput struct {
	mmodel.CreateAccountTypeInput
}

// UpdateAccountTypeInput wraps mmodel.UpdateAccountTypeInput to maintain
// compatibility while using midaz entities.
//
// An empty update payload — no setters and no null-fields — returns a
// marshal error from MarshalJSON. This is intentional: an empty PATCH
// would be a no-op round trip. Use the dedicated builder helpers to
// either set a value or explicitly null out a field.
type UpdateAccountTypeInput struct {
	mmodel.UpdateAccountTypeInput
}

// Validate validates the CreateAccountTypeInput fields.
func (input *CreateAccountTypeInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.Name == "" {
		return errors.New("name is required")
	}

	if input.KeyValue == "" {
		return errors.New("keyValue is required")
	}

	if err := validateAccountTypeLengths(input.Name, input.Description, input.KeyValue, true); err != nil {
		return err
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

func (input *UpdateAccountTypeInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.Name != "" || input.Description != "" || input.Metadata != nil
}

func validateAccountTypeLengths(name, description, keyValue string, validateKey bool) error {
	if len(name) > maxAccountTypeNameLength {
		return fmt.Errorf("name must be at most %d characters", maxAccountTypeNameLength)
	}

	if len(description) > maxAccountTypeDescriptionLength {
		return fmt.Errorf("description must be at most %d characters", maxAccountTypeDescriptionLength)
	}

	if validateKey && len(keyValue) > maxAccountTypeKeyValueLength {
		return fmt.Errorf("keyValue must be at most %d characters", maxAccountTypeKeyValueLength)
	}

	return nil
}

// MarshalJSON omits optional create fields when callers leave them unset.
func (input *CreateAccountTypeInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "name", input.Name)
	addStringField(fields, "description", input.Description)
	addStringField(fields, "keyValue", input.KeyValue)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

// Validate validates the UpdateAccountTypeInput fields.
func (input *UpdateAccountTypeInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	if err := validateAccountTypeLengths(input.Name, input.Description, "", false); err != nil {
		return err
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

// NewCreateAccountTypeInput creates a new CreateAccountTypeInput with required fields.
// This constructor ensures that all mandatory fields are provided when creating an account type input.
//
// Parameters:
//   - name: Human-readable name for the account type
//   - keyValue: Unique identifier within the organization/ledger
//
// Returns:
//   - A pointer to the newly created CreateAccountTypeInput
func NewCreateAccountTypeInput(name, keyValue string) *CreateAccountTypeInput {
	return &CreateAccountTypeInput{
		CreateAccountTypeInput: mmodel.CreateAccountTypeInput{
			Name:     name,
			KeyValue: keyValue,
		},
	}
}

// WithDescription sets the description for CreateAccountTypeInput.
func (input *CreateAccountTypeInput) WithDescription(description string) *CreateAccountTypeInput {
	if input == nil {
		return nil
	}

	input.Description = description

	return input
}

// WithMetadata sets the metadata for CreateAccountTypeInput.
func (input *CreateAccountTypeInput) WithMetadata(metadata map[string]any) *CreateAccountTypeInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// NewUpdateAccountTypeInput creates a new UpdateAccountTypeInput.
// This constructor initializes an empty update input that can be customized
// using the With* helper functions.
//
// Returns:
//   - A pointer to the newly created UpdateAccountTypeInput
func NewUpdateAccountTypeInput() *UpdateAccountTypeInput {
	return &UpdateAccountTypeInput{
		UpdateAccountTypeInput: mmodel.UpdateAccountTypeInput{},
	}
}

// WithName sets the name for UpdateAccountTypeInput.
func (input *UpdateAccountTypeInput) WithName(name string) *UpdateAccountTypeInput {
	if input == nil {
		return nil
	}

	input.Name = name

	return input
}

// WithDescription sets the description for UpdateAccountTypeInput.
func (input *UpdateAccountTypeInput) WithDescription(description string) *UpdateAccountTypeInput {
	if input == nil {
		return nil
	}

	input.Description = description

	return input
}

// WithMetadata sets the metadata for UpdateAccountTypeInput.
func (input *UpdateAccountTypeInput) WithMetadata(metadata map[string]any) *UpdateAccountTypeInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// MarshalJSON emits only fields explicitly set on the SDK PATCH input.
func (input *UpdateAccountTypeInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "name", input.Name)
	addStringField(fields, "description", input.Description)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

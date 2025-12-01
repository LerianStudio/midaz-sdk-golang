// Package models defines the data models used by the Midaz SDK.
package models

import (
	"errors"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
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

// UpdateAccountTypeInput wraps mmodel.UpdateAccountTypeInput to maintain compatibility while using midaz entities.
type UpdateAccountTypeInput struct {
	mmodel.UpdateAccountTypeInput
}

// Validate validates the CreateAccountTypeInput fields.
func (input *CreateAccountTypeInput) Validate() error {
	if input.Name == "" {
		return errors.New("name is required")
	}

	if input.KeyValue == "" {
		return errors.New("keyValue is required")
	}

	return nil
}

// Validate validates the UpdateAccountTypeInput fields.
func (*UpdateAccountTypeInput) Validate() error {
	// For update operations, most fields are optional
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

// WithCreateAccountTypeDescription sets the description for CreateAccountTypeInput.
// This adds a detailed description to the account type.
//
// Parameters:
//   - input: The CreateAccountTypeInput to modify
//   - description: The description for the account type
//
// Returns:
//   - A pointer to the modified CreateAccountTypeInput for method chaining
func WithCreateAccountTypeDescription(input *CreateAccountTypeInput, description string) *CreateAccountTypeInput {
	input.Description = description
	return input
}

// WithDescription sets the description for CreateAccountTypeInput (method on struct).
func (input *CreateAccountTypeInput) WithDescription(description string) *CreateAccountTypeInput {
	input.Description = description
	return input
}

// WithMetadata sets the metadata for CreateAccountTypeInput (method on struct).
func (input *CreateAccountTypeInput) WithMetadata(metadata map[string]any) *CreateAccountTypeInput {
	input.Metadata = metadata
	return input
}

// WithCreateAccountTypeMetadata sets the metadata for CreateAccountTypeInput.
// Metadata can store additional custom information about the account type.
//
// Parameters:
//   - input: The CreateAccountTypeInput to modify
//   - metadata: A map of key-value pairs to store as metadata
//
// Returns:
//   - A pointer to the modified CreateAccountTypeInput for method chaining
func WithCreateAccountTypeMetadata(input *CreateAccountTypeInput, metadata map[string]any) *CreateAccountTypeInput {
	input.Metadata = metadata
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

// WithUpdateAccountTypeName sets the name for UpdateAccountTypeInput.
// This updates the human-readable name of the account type.
//
// Parameters:
//   - input: The UpdateAccountTypeInput to modify
//   - name: The new name for the account type
//
// Returns:
//   - A pointer to the modified UpdateAccountTypeInput for method chaining
func WithUpdateAccountTypeName(input *UpdateAccountTypeInput, name string) *UpdateAccountTypeInput {
	input.Name = name
	return input
}

// WithName sets the name for UpdateAccountTypeInput (method on struct).
func (input *UpdateAccountTypeInput) WithName(name string) *UpdateAccountTypeInput {
	input.Name = name
	return input
}

// WithDescription sets the description for UpdateAccountTypeInput (method on struct).
func (input *UpdateAccountTypeInput) WithDescription(description string) *UpdateAccountTypeInput {
	input.Description = description
	return input
}

// WithMetadata sets the metadata for UpdateAccountTypeInput (method on struct).
func (input *UpdateAccountTypeInput) WithMetadata(metadata map[string]any) *UpdateAccountTypeInput {
	input.Metadata = metadata
	return input
}

// WithUpdateAccountTypeDescription sets the description for UpdateAccountTypeInput.
// This updates the detailed description of the account type.
//
// Parameters:
//   - input: The UpdateAccountTypeInput to modify
//   - description: The new description for the account type
//
// Returns:
//   - A pointer to the modified UpdateAccountTypeInput for method chaining
func WithUpdateAccountTypeDescription(input *UpdateAccountTypeInput, description string) *UpdateAccountTypeInput {
	input.Description = description
	return input
}

// WithUpdateAccountTypeMetadata sets the metadata for UpdateAccountTypeInput.
// This updates the custom metadata associated with the account type.
//
// Parameters:
//   - input: The UpdateAccountTypeInput to modify
//   - metadata: The new metadata map
//
// Returns:
//   - A pointer to the modified UpdateAccountTypeInput for method chaining
func WithUpdateAccountTypeMetadata(input *UpdateAccountTypeInput, metadata map[string]any) *UpdateAccountTypeInput {
	input.Metadata = metadata
	return input
}

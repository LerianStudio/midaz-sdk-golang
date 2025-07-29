// Package models defines the data models used by the Midaz SDK.
package models

import (
	"fmt"
	"time"
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
type AccountType struct {
	// ID is the unique identifier for the account type
	// This is a system-generated UUID that uniquely identifies the account type
	// across the entire Midaz platform.
	ID string `json:"id"`

	// Name is the human-readable name of the account type
	// This should be descriptive and meaningful to users, with a maximum
	// length of 256 characters.
	Name string `json:"name"`

	// Description is an optional detailed description of the account type
	// This provides additional context about the purpose and usage of the account type.
	Description *string `json:"description,omitempty"`

	// KeyValue is a unique identifier within the organization and ledger
	// This is used to identify the account type programmatically and must be
	// unique within the scope of the organization and ledger.
	KeyValue string `json:"keyValue"`

	// OrganizationID is the ID of the organization that owns this account type
	// All account types must belong to an organization, which provides the
	// top-level ownership and access control.
	OrganizationID string `json:"organizationId"`

	// LedgerID is the ID of the ledger that contains this account type
	// Account types are always created within a specific ledger, which defines
	// the accounting boundaries and rules.
	LedgerID string `json:"ledgerId"`

	// Metadata stores additional custom information about the account type
	// This can include any arbitrary key-value pairs for application-specific
	// data that doesn't fit into the standard account type fields.
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is the timestamp when the account type was created
	// This is automatically set by the system and cannot be modified.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is the timestamp when the account type was last updated
	// This is automatically updated by the system whenever the account type is modified.
	UpdatedAt time.Time `json:"updatedAt"`

	// DeletedAt is the timestamp when the account type was deleted, if applicable
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// CreateAccountTypeInput is the input for creating an account type.
// This structure contains all the fields that can be specified when creating a new account type.
type CreateAccountTypeInput struct {
	// Name is the human-readable name of the account type.
	// Required. Max length: 256 characters.
	Name string `json:"name"`

	// Description is an optional detailed description of the account type.
	// Max length: 1000 characters.
	Description *string `json:"description,omitempty"`

	// KeyValue is a unique identifier within the organization and ledger.
	// Required. Must be unique within the organization/ledger scope.
	// Max length: 100 characters.
	KeyValue string `json:"keyValue"`

	// Metadata contains additional custom data associated with the account type.
	// Keys max length: 100 characters, Values max length: 2000 characters.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Validate checks if the CreateAccountTypeInput meets the validation requirements.
// It returns an error if any of the validation checks fail.
func (input *CreateAccountTypeInput) Validate() error {
	if input.Name == "" {
		return fmt.Errorf("name is required")
	}

	if len(input.Name) > 256 {
		return fmt.Errorf("name must be at most 256 characters")
	}

	if input.KeyValue == "" {
		return fmt.Errorf("keyValue is required")
	}

	if len(input.KeyValue) > 100 {
		return fmt.Errorf("keyValue must be at most 100 characters")
	}

	if input.Description != nil && len(*input.Description) > 1000 {
		return fmt.Errorf("description must be at most 1000 characters")
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
		Name:     name,
		KeyValue: keyValue,
	}
}

// WithDescription sets the description.
// This adds a detailed description to the account type.
//
// Parameters:
//   - description: The description for the account type
//
// Returns:
//   - A pointer to the modified CreateAccountTypeInput for method chaining
func (input *CreateAccountTypeInput) WithDescription(description string) *CreateAccountTypeInput {
	input.Description = &description
	return input
}

// WithMetadata sets the metadata.
// Metadata can store additional custom information about the account type.
//
// Parameters:
//   - metadata: A map of key-value pairs to store as metadata
//
// Returns:
//   - A pointer to the modified CreateAccountTypeInput for method chaining
func (input *CreateAccountTypeInput) WithMetadata(metadata map[string]any) *CreateAccountTypeInput {
	input.Metadata = metadata
	return input
}

// UpdateAccountTypeInput is the input for updating an account type.
// This structure contains the fields that can be modified when updating an existing account type.
type UpdateAccountTypeInput struct {
	// Name is the human-readable name of the account type.
	// Max length: 256 characters.
	Name *string `json:"name,omitempty"`

	// Description is an optional detailed description of the account type.
	// Max length: 1000 characters.
	Description *string `json:"description,omitempty"`

	// Metadata contains additional custom data associated with the account type.
	// Keys max length: 100 characters, Values max length: 2000 characters.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Validate checks if the UpdateAccountTypeInput meets the validation requirements.
// It returns an error if any of the validation checks fail.
func (input *UpdateAccountTypeInput) Validate() error {
	if input.Name != nil && len(*input.Name) > 256 {
		return fmt.Errorf("name must be at most 256 characters")
	}

	if input.Description != nil && len(*input.Description) > 1000 {
		return fmt.Errorf("description must be at most 1000 characters")
	}

	return nil
}

// NewUpdateAccountTypeInput creates a new UpdateAccountTypeInput.
// This constructor initializes an empty update input that can be customized
// using the With* methods.
//
// Returns:
//   - A pointer to the newly created UpdateAccountTypeInput
func NewUpdateAccountTypeInput() *UpdateAccountTypeInput {
	return &UpdateAccountTypeInput{}
}

// WithName sets the name.
// This updates the human-readable name of the account type.
//
// Parameters:
//   - name: The new name for the account type
//
// Returns:
//   - A pointer to the modified UpdateAccountTypeInput for method chaining
func (input *UpdateAccountTypeInput) WithName(name string) *UpdateAccountTypeInput {
	input.Name = &name
	return input
}

// WithDescription sets the description.
// This updates the detailed description of the account type.
//
// Parameters:
//   - description: The new description for the account type
//
// Returns:
//   - A pointer to the modified UpdateAccountTypeInput for method chaining
func (input *UpdateAccountTypeInput) WithDescription(description string) *UpdateAccountTypeInput {
	input.Description = &description
	return input
}

// WithMetadata sets the metadata.
// This updates the custom metadata associated with the account type.
//
// Parameters:
//   - metadata: The new metadata map
//
// Returns:
//   - A pointer to the modified UpdateAccountTypeInput for method chaining
func (input *UpdateAccountTypeInput) WithMetadata(metadata map[string]any) *UpdateAccountTypeInput {
	input.Metadata = metadata
	return input
}

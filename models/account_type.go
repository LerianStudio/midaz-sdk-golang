package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
	"github.com/google/uuid"
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
// AccountType is the SDK-native account type response (Track 7E — audit 7.1).
type AccountType struct {
	ID             uuid.UUID      `json:"id,omitempty" example:"01965ed9-7fa4-75b2-8872-fc9e8509ab0a"`
	OrganizationID uuid.UUID      `json:"organizationId,omitempty" example:"01965ed9-7fa4-75b2-8872-fc9e8509ab0a"`
	LedgerID       uuid.UUID      `json:"ledgerId,omitempty" example:"01965ed9-7fa4-75b2-8872-fc9e8509ab0a"`
	Name           string         `json:"name,omitempty" example:"Current Assets"`
	Description    string         `json:"description,omitempty" example:"Assets that are expected to be converted to cash within one year"`
	KeyValue       string         `json:"keyValue,omitempty" example:"current_assets"`
	CreatedAt      time.Time      `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	UpdatedAt      time.Time      `json:"updatedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	DeletedAt      *time.Time     `json:"deletedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// CreateAccountTypeInput is the SDK-native account-type creation payload.
type CreateAccountTypeInput struct {
	Name        string         `json:"name" example:"Current Assets"`
	Description string         `json:"description,omitempty" example:"Assets that are expected to be converted to cash within one year"`
	KeyValue    string         `json:"keyValue" example:"current_assets"`
	Metadata    map[string]any `json:"metadata"`
}

// UpdateAccountTypeInput is the SDK-native account-type patch payload.
//
// An empty update payload (no fields set) is rejected by Validate() with
// an "empty update payload not allowed" error. This is intentional: an
// empty PATCH would be a no-op round trip. Use the builder helpers to
// set at least one field before validating.
//
// MarshalJSON itself does not enforce the empty-payload rule — it trusts
// that the entity layer called Validate() first. Reaching MarshalJSON
// with no changes results in an empty `{}` rather than a duplicate error
// to keep the source of truth in Validate().
type UpdateAccountTypeInput struct {
	Name        string         `json:"name,omitempty" example:"Current Assets"`
	Description string         `json:"description,omitempty" example:"Assets that are expected to be converted to cash within one year"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Validate validates the CreateAccountTypeInput fields.
func (input *CreateAccountTypeInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	if input.Name == "" {
		errs.Append("name", "is required")
	}

	if input.KeyValue == "" {
		errs.Append("keyValue", "is required")
	}

	appendAccountTypeLengths(&errs, input.Name, input.Description, input.KeyValue, true)

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

func (input *UpdateAccountTypeInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.Name != "" || input.Description != "" || input.Metadata != nil
}

// appendAccountTypeLengths records all length-bound violations onto errs.
// validateKey gates the keyValue check (false on Update where keyValue
// is immutable and not part of the patch payload).
func appendAccountTypeLengths(errs *validation.FieldErrors, name, description, keyValue string, validateKey bool) {
	if len(name) > maxAccountTypeNameLength {
		errs.Append("name", fmt.Sprintf("must be at most %d characters", maxAccountTypeNameLength))
	}

	if len(description) > maxAccountTypeDescriptionLength {
		errs.Append("description", fmt.Sprintf("must be at most %d characters", maxAccountTypeDescriptionLength))
	}

	if validateKey && len(keyValue) > maxAccountTypeKeyValueLength {
		errs.Append("keyValue", fmt.Sprintf("must be at most %d characters", maxAccountTypeKeyValueLength))
	}
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

	var errs validation.FieldErrors

	appendAccountTypeLengths(&errs, input.Name, input.Description, "", false)

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
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
	return &UpdateAccountTypeInput{}
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

package models

import (
	"errors"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation/core"
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

const (
	maxRouteTitleLength       = 255
	maxRouteDescriptionLength = 250
	maxRouteCodeLength        = 100
)

// OperationRoute is an alias for mmodel.OperationRoute to maintain compatibility while using midaz entities.
type OperationRoute = mmodel.OperationRoute

// CreateOperationRouteInput wraps mmodel.CreateOperationRouteInput to maintain compatibility while using midaz entities.
type CreateOperationRouteInput struct {
	mmodel.CreateOperationRouteInput
}

// Validate validates the CreateOperationRouteInput fields.
func (input *CreateOperationRouteInput) Validate() error {
	if input == nil {
		return errors.New("input is required")
	}

	if input.Title == "" {
		return errors.New("title is required")
	}

	if err := validateRouteText(input.Title, maxRouteTitleLength, "title"); err != nil {
		return err
	}

	if err := validateRouteText(input.Description, maxRouteDescriptionLength, "description"); err != nil {
		return err
	}

	if err := validateRouteText(input.Code, maxRouteCodeLength, "code"); err != nil { //nolint:staticcheck // Code is deprecated but still accepted on compatibility DTOs and must be bounded.
		return err
	}

	if input.OperationType == "" {
		return errors.New("operationType is required")
	}
	// Validate operation type
	if input.OperationType != "source" && input.OperationType != "destination" && input.OperationType != "bidirectional" {
		return errors.New("operationType must be 'source', 'destination', or 'bidirectional'")
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

// UpdateOperationRouteInput wraps mmodel.UpdateOperationRouteInput to maintain compatibility while using midaz entities.
type UpdateOperationRouteInput struct {
	mmodel.UpdateOperationRouteInput
}

// Validate validates the UpdateOperationRouteInput fields.
func (input *UpdateOperationRouteInput) Validate() error {
	if input == nil {
		return errors.New("input is required")
	}

	if err := validateRouteText(input.Title, maxRouteTitleLength, "title"); err != nil {
		return err
	}

	if err := validateRouteText(input.Description, maxRouteDescriptionLength, "description"); err != nil {
		return err
	}

	if err := validateRouteText(input.Code, maxRouteCodeLength, "code"); err != nil { //nolint:staticcheck // Code is deprecated but still accepted on compatibility DTOs and must be bounded.
		return err
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

func validateRouteText(value string, maxLength int, field string) error {
	if len(value) > maxLength {
		return fmt.Errorf("%s must be at most %d characters", field, maxLength)
	}

	return nil
}

// WithAccountAlias sets the account rule to use alias-based selection (method on struct).
func (input *CreateOperationRouteInput) WithAccountAlias(alias string) *CreateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Account = &AccountRule{
		RuleType: "alias",
		ValidIf:  alias,
	}

	return input
}

// WithAccountTypes sets the account rule to use account type-based selection (method on struct).
func (input *CreateOperationRouteInput) WithAccountTypes(accountTypes []string) *CreateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Account = &AccountRule{
		RuleType: "account_type",
		ValidIf:  accountTypes,
	}

	return input
}

// WithTitle sets the title for UpdateOperationRouteInput (method on struct).
func (input *UpdateOperationRouteInput) WithTitle(title string) *UpdateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Title = title

	return input
}

// WithMetadata sets the metadata for CreateOperationRouteInput (method on struct).
func (input *CreateOperationRouteInput) WithMetadata(metadata map[string]any) *CreateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Metadata = metadata

	return input
}

// WithDescription sets the description for UpdateOperationRouteInput (method on struct).
func (input *UpdateOperationRouteInput) WithDescription(description string) *UpdateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Description = description

	return input
}

// WithAccountTypes sets the account rule to use account type-based selection for UpdateOperationRouteInput (method on struct).
func (input *UpdateOperationRouteInput) WithAccountTypes(accountTypes []string) *UpdateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Account = &AccountRule{
		RuleType: "account_type",
		ValidIf:  accountTypes,
	}

	return input
}

// WithMetadata sets the metadata for UpdateOperationRouteInput (method on struct).
func (input *UpdateOperationRouteInput) WithMetadata(metadata map[string]any) *UpdateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Metadata = metadata

	return input
}

// AccountRule is an alias for mmodel.AccountRule to maintain compatibility while using midaz entities.
type AccountRule = mmodel.AccountRule

// OperationRouteType represents the type of operation route for backward compatibility
type OperationRouteType string

// OperationRouteType constants define operation route types.
const (
	// OperationRouteTypeDebit represents a legacy response value.
	// Deprecated: debit is not accepted when creating or updating operation routes.
	OperationRouteTypeDebit OperationRouteType = "debit"
	// OperationRouteTypeCredit represents a legacy response value.
	// Deprecated: credit is not accepted when creating or updating operation routes.
	OperationRouteTypeCredit OperationRouteType = "credit"
	// OperationRouteTypeSource represents source operation type
	OperationRouteTypeSource OperationRouteType = "source"
	// OperationRouteTypeDestination represents destination operation type
	OperationRouteTypeDestination OperationRouteType = "destination"
	// OperationRouteTypeBidirectional represents bidirectional operation type
	OperationRouteTypeBidirectional OperationRouteType = "bidirectional"
)

// OperationRouteInputType represents the type for operation route input (different from response)
type OperationRouteInputType string

// OperationRouteInputType constants define valid input types.
const (
	// OperationRouteInputTypeSource represents source input type
	OperationRouteInputTypeSource OperationRouteInputType = "source"
	// OperationRouteInputTypeDestination represents destination input type
	OperationRouteInputTypeDestination OperationRouteInputType = "destination"
	// OperationRouteInputTypeBidirectional represents bidirectional input type
	OperationRouteInputTypeBidirectional OperationRouteInputType = "bidirectional"
)

// NewCreateOperationRouteInput creates a new CreateOperationRouteInput with required fields.
//
// Parameters:
//   - title: Short text summarizing the purpose of the operation
//   - description: Detailed description of the operation route purpose and usage
//   - operationType: The type of the operation route ("source", "destination", or "bidirectional")
//
// Returns:
//   - A pointer to the newly created CreateOperationRouteInput
func NewCreateOperationRouteInput(title, description, operationType string) *CreateOperationRouteInput {
	return &CreateOperationRouteInput{
		CreateOperationRouteInput: mmodel.CreateOperationRouteInput{
			Title:         title,
			Description:   description,
			OperationType: operationType,
		},
	}
}

// WithCreateOperationRouteAccountAlias sets the account rule to use alias-based selection.
//
// Parameters:
//   - input: The CreateOperationRouteInput to modify
//   - alias: The account alias to use for selection
//
// Returns:
//   - A pointer to the modified CreateOperationRouteInput for method chaining
func WithCreateOperationRouteAccountAlias(input *CreateOperationRouteInput, alias string) *CreateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Account = &AccountRule{
		RuleType: "alias",
		ValidIf:  alias,
	}

	return input
}

// WithCreateOperationRouteAccountType sets the account rule to use account type-based selection.
//
// Parameters:
//   - input: The CreateOperationRouteInput to modify
//   - accountTypes: The account types to use for selection
//
// Returns:
//   - A pointer to the modified CreateOperationRouteInput for method chaining
func WithCreateOperationRouteAccountType(input *CreateOperationRouteInput, accountTypes []string) *CreateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Account = &AccountRule{
		RuleType: "account_type",
		ValidIf:  accountTypes,
	}

	return input
}

// WithCreateOperationRouteMetadata sets the metadata for CreateOperationRouteInput.
//
// Parameters:
//   - input: The CreateOperationRouteInput to modify
//   - metadata: A map of key-value pairs to store as metadata
//
// Returns:
//   - A pointer to the modified CreateOperationRouteInput for method chaining
func WithCreateOperationRouteMetadata(input *CreateOperationRouteInput, metadata map[string]any) *CreateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Metadata = metadata

	return input
}

// NewUpdateOperationRouteInput creates a new UpdateOperationRouteInput.
//
// Returns:
//   - A pointer to the newly created UpdateOperationRouteInput
func NewUpdateOperationRouteInput() *UpdateOperationRouteInput {
	return &UpdateOperationRouteInput{}
}

// WithUpdateOperationRouteTitle sets the title for UpdateOperationRouteInput.
//
// Parameters:
//   - input: The UpdateOperationRouteInput to modify
//   - title: The new title for the operation route
//
// Returns:
//   - A pointer to the modified UpdateOperationRouteInput for method chaining
func WithUpdateOperationRouteTitle(input *UpdateOperationRouteInput, title string) *UpdateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Title = title

	return input
}

// WithUpdateOperationRouteDescription sets the description for UpdateOperationRouteInput.
//
// Parameters:
//   - input: The UpdateOperationRouteInput to modify
//   - description: The new description for the operation route
//
// Returns:
//   - A pointer to the modified UpdateOperationRouteInput for method chaining
func WithUpdateOperationRouteDescription(input *UpdateOperationRouteInput, description string) *UpdateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Description = description

	return input
}

// WithUpdateOperationRouteAccountAlias sets the account rule to use alias-based selection.
//
// Parameters:
//   - input: The UpdateOperationRouteInput to modify
//   - alias: The account alias to use for selection
//
// Returns:
//   - A pointer to the modified UpdateOperationRouteInput for method chaining
func WithUpdateOperationRouteAccountAlias(input *UpdateOperationRouteInput, alias string) *UpdateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Account = &AccountRule{
		RuleType: "alias",
		ValidIf:  alias,
	}

	return input
}

// WithUpdateOperationRouteAccountType sets the account rule to use account type-based selection.
//
// Parameters:
//   - input: The UpdateOperationRouteInput to modify
//   - accountTypes: The account types to use for selection
//
// Returns:
//   - A pointer to the modified UpdateOperationRouteInput for method chaining
func WithUpdateOperationRouteAccountType(input *UpdateOperationRouteInput, accountTypes []string) *UpdateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Account = &AccountRule{
		RuleType: "account_type",
		ValidIf:  accountTypes,
	}

	return input
}

// WithUpdateOperationRouteMetadata sets the metadata for UpdateOperationRouteInput.
//
// Parameters:
//   - input: The UpdateOperationRouteInput to modify
//   - metadata: A map of key-value pairs to store as metadata
//
// Returns:
//   - A pointer to the modified UpdateOperationRouteInput for method chaining
func WithUpdateOperationRouteMetadata(input *UpdateOperationRouteInput, metadata map[string]any) *UpdateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Metadata = metadata

	return input
}

// Note: For backward compatibility, you can use the helper functions:
// - WithCreateOperationRouteAccountAlias(input, alias) instead of input.WithAccountAlias(alias)
// - WithCreateOperationRouteMetadata(input, metadata) instead of input.WithMetadata(metadata)

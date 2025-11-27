package models

import (
	"errors"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

// OperationRoute is an alias for mmodel.OperationRoute to maintain compatibility while using midaz entities.
type OperationRoute = mmodel.OperationRoute

// CreateOperationRouteInput wraps mmodel.CreateOperationRouteInput to maintain compatibility while using midaz entities.
type CreateOperationRouteInput struct {
	mmodel.CreateOperationRouteInput
}

// Validate validates the CreateOperationRouteInput fields.
func (input *CreateOperationRouteInput) Validate() error {
	if input.Title == "" {
		return errors.New("title is required")
	}

	if input.Description == "" {
		return errors.New("description is required")
	}

	if input.OperationType == "" {
		return errors.New("operationType is required")
	}
	// Validate operation type
	if input.OperationType != "source" && input.OperationType != "destination" {
		return errors.New("operationType must be 'source' or 'destination'")
	}

	return nil
}

// UpdateOperationRouteInput wraps mmodel.UpdateOperationRouteInput to maintain compatibility while using midaz entities.
type UpdateOperationRouteInput struct {
	mmodel.UpdateOperationRouteInput
}

// Validate validates the UpdateOperationRouteInput fields.
func (*UpdateOperationRouteInput) Validate() error {
	// For updates, fields are optional so validation is minimal
	return nil
}

// WithAccountAlias sets the account rule to use alias-based selection (method on struct).
func (input *CreateOperationRouteInput) WithAccountAlias(alias string) *CreateOperationRouteInput {
	input.Account = &AccountRule{
		RuleType: "alias",
		ValidIf:  alias,
	}

	return input
}

// WithAccountTypes sets the account rule to use account type-based selection (method on struct).
func (input *CreateOperationRouteInput) WithAccountTypes(accountTypes []string) *CreateOperationRouteInput {
	input.Account = &AccountRule{
		RuleType: "account_type",
		ValidIf:  accountTypes,
	}

	return input
}

// WithTitle sets the title for UpdateOperationRouteInput (method on struct).
func (input *UpdateOperationRouteInput) WithTitle(title string) *UpdateOperationRouteInput {
	input.Title = title
	return input
}

// WithMetadata sets the metadata for CreateOperationRouteInput (method on struct).
func (input *CreateOperationRouteInput) WithMetadata(metadata map[string]any) *CreateOperationRouteInput {
	input.Metadata = metadata
	return input
}

// WithDescription sets the description for UpdateOperationRouteInput (method on struct).
func (input *UpdateOperationRouteInput) WithDescription(description string) *UpdateOperationRouteInput {
	input.Description = description
	return input
}

// WithAccountTypes sets the account rule to use account type-based selection for UpdateOperationRouteInput (method on struct).
func (input *UpdateOperationRouteInput) WithAccountTypes(accountTypes []string) *UpdateOperationRouteInput {
	input.Account = &AccountRule{
		RuleType: "account_type",
		ValidIf:  accountTypes,
	}

	return input
}

// WithMetadata sets the metadata for UpdateOperationRouteInput (method on struct).
func (input *UpdateOperationRouteInput) WithMetadata(metadata map[string]any) *UpdateOperationRouteInput {
	input.Metadata = metadata
	return input
}

// AccountRule is an alias for mmodel.AccountRule to maintain compatibility while using midaz entities.
type AccountRule = mmodel.AccountRule

// OperationRouteType represents the type of operation route for backward compatibility
type OperationRouteType string

const (
	OperationRouteTypeDebit  OperationRouteType = "debit"
	OperationRouteTypeCredit OperationRouteType = "credit"
	// Response values that correspond to input values
	OperationRouteTypeSource      OperationRouteType = "source"      // Alternative name for source operations
	OperationRouteTypeDestination OperationRouteType = "destination" // Alternative name for destination operations
)

// OperationRouteInputType represents the type for operation route input (different from response)
type OperationRouteInputType string

const (
	OperationRouteInputTypeSource      OperationRouteInputType = "source"
	OperationRouteInputTypeDestination OperationRouteInputType = "destination"
)

// NewCreateOperationRouteInput creates a new CreateOperationRouteInput with required fields.
//
// Parameters:
//   - title: Short text summarizing the purpose of the operation
//   - description: Detailed description of the operation route purpose and usage
//   - operationType: The type of the operation route ("source" or "destination")
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

// WithAccountAlias sets the account rule to use alias-based selection.
//
// Parameters:
//   - input: The CreateOperationRouteInput to modify
//   - alias: The account alias to use for selection
//
// Returns:
//   - A pointer to the modified CreateOperationRouteInput for method chaining
func WithCreateOperationRouteAccountAlias(input *CreateOperationRouteInput, alias string) *CreateOperationRouteInput {
	input.Account = &AccountRule{
		RuleType: "alias",
		ValidIf:  alias,
	}

	return input
}

// WithAccountType sets the account rule to use account type-based selection.
//
// Parameters:
//   - input: The CreateOperationRouteInput to modify
//   - accountTypes: The account types to use for selection
//
// Returns:
//   - A pointer to the modified CreateOperationRouteInput for method chaining
func WithCreateOperationRouteAccountType(input *CreateOperationRouteInput, accountTypes []string) *CreateOperationRouteInput {
	input.Account = &AccountRule{
		RuleType: "account_type",
		ValidIf:  accountTypes,
	}

	return input
}

// WithMetadata sets the metadata for CreateOperationRouteInput.
//
// Parameters:
//   - input: The CreateOperationRouteInput to modify
//   - metadata: A map of key-value pairs to store as metadata
//
// Returns:
//   - A pointer to the modified CreateOperationRouteInput for method chaining
func WithCreateOperationRouteMetadata(input *CreateOperationRouteInput, metadata map[string]any) *CreateOperationRouteInput {
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

// WithTitle sets the title for UpdateOperationRouteInput.
//
// Parameters:
//   - input: The UpdateOperationRouteInput to modify
//   - title: The new title for the operation route
//
// Returns:
//   - A pointer to the modified UpdateOperationRouteInput for method chaining
func WithUpdateOperationRouteTitle(input *UpdateOperationRouteInput, title string) *UpdateOperationRouteInput {
	input.Title = title
	return input
}

// WithDescription sets the description for UpdateOperationRouteInput.
//
// Parameters:
//   - input: The UpdateOperationRouteInput to modify
//   - description: The new description for the operation route
//
// Returns:
//   - A pointer to the modified UpdateOperationRouteInput for method chaining
func WithUpdateOperationRouteDescription(input *UpdateOperationRouteInput, description string) *UpdateOperationRouteInput {
	input.Description = description
	return input
}

// WithAccountAlias sets the account rule to use alias-based selection for UpdateOperationRouteInput.
//
// Parameters:
//   - input: The UpdateOperationRouteInput to modify
//   - alias: The account alias to use for selection
//
// Returns:
//   - A pointer to the modified UpdateOperationRouteInput for method chaining
func WithUpdateOperationRouteAccountAlias(input *UpdateOperationRouteInput, alias string) *UpdateOperationRouteInput {
	input.Account = &AccountRule{
		RuleType: "alias",
		ValidIf:  alias,
	}

	return input
}

// WithAccountType sets the account rule to use account type-based selection for UpdateOperationRouteInput.
//
// Parameters:
//   - input: The UpdateOperationRouteInput to modify
//   - accountTypes: The account types to use for selection
//
// Returns:
//   - A pointer to the modified UpdateOperationRouteInput for method chaining
func WithUpdateOperationRouteAccountType(input *UpdateOperationRouteInput, accountTypes []string) *UpdateOperationRouteInput {
	input.Account = &AccountRule{
		RuleType: "account_type",
		ValidIf:  accountTypes,
	}

	return input
}

// WithMetadata sets the metadata for UpdateOperationRouteInput.
//
// Parameters:
//   - input: The UpdateOperationRouteInput to modify
//   - metadata: A map of key-value pairs to store as metadata
//
// Returns:
//   - A pointer to the modified UpdateOperationRouteInput for method chaining
func WithUpdateOperationRouteMetadata(input *UpdateOperationRouteInput, metadata map[string]any) *UpdateOperationRouteInput {
	input.Metadata = metadata
	return input
}

// Note: For backward compatibility, you can use the helper functions:
// - WithCreateOperationRouteAccountAlias(input, alias) instead of input.WithAccountAlias(alias)
// - WithCreateOperationRouteMetadata(input, metadata) instead of input.WithMetadata(metadata)

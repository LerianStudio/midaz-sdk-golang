package models

import (
	"fmt"
	"time"
)

// OperationRouteType represents the type of operation route
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

// OperationRouteAccount represents the account rules for an operation route.
// The ValidIf field accepts different types based on RuleType:
//   - For "alias": string (single alias) or []string (multiple aliases)
//   - For "account_type": string (single type) or []string (multiple types)
// This flexibility matches the API specification requirements.
type OperationRouteAccount struct {
	RuleType string      `json:"ruleType"`
	ValidIf  interface{} `json:"validIf"`
}

// OperationRoute represents an operation route entity
type OperationRoute struct {
	ID             string                 `json:"id"`
	OrganizationID string                 `json:"organizationId"`
	LedgerID       string                 `json:"ledgerId"`
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	OperationType  string                 `json:"operationType"`           // source or destination
	Account        OperationRouteAccount  `json:"account"`                 // account rules
	CreatedAt      time.Time              `json:"createdAt"`
	UpdatedAt      time.Time              `json:"updatedAt"`
	DeletedAt      *time.Time             `json:"deletedAt,omitempty"`
	Metadata       map[string]any         `json:"metadata,omitempty"`
}

// CreateOperationRouteInput represents the input for creating an operation route
type CreateOperationRouteInput struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	OperationType string               `json:"operationType"`           // source or destination
	Account     OperationRouteAccount  `json:"account"`                 // account rules
	Metadata    map[string]any         `json:"metadata,omitempty"`
}

// NewCreateOperationRouteInput creates a new CreateOperationRouteInput with required fields
func NewCreateOperationRouteInput(title, description, operationType string) *CreateOperationRouteInput {
	return &CreateOperationRouteInput{
		Title:       title,
		Description: description,
		OperationType: operationType,
	}
}

// WithAccountTypes sets the account types validation for the operation route (ruleType: account_type)
func (input *CreateOperationRouteInput) WithAccountTypes(accountTypes []string) *CreateOperationRouteInput {
	input.Account = OperationRouteAccount{
		RuleType: "account_type",
		ValidIf:  accountTypes,
	}
	return input
}

// WithAccountType sets a single account type validation for the operation route (ruleType: account_type)
func (input *CreateOperationRouteInput) WithAccountType(accountType string) *CreateOperationRouteInput {
	input.Account = OperationRouteAccount{
		RuleType: "account_type",
		ValidIf:  accountType,
	}
	return input
}

// WithAccountAliases sets multiple account aliases validation for the operation route (ruleType: alias)
func (input *CreateOperationRouteInput) WithAccountAliases(accountAliases []string) *CreateOperationRouteInput {
	input.Account = OperationRouteAccount{
		RuleType: "alias",
		ValidIf:  accountAliases,
	}
	return input
}

// WithAccountAlias sets a single account alias validation for the operation route (ruleType: alias)
func (input *CreateOperationRouteInput) WithAccountAlias(accountAlias string) *CreateOperationRouteInput {
	input.Account = OperationRouteAccount{
		RuleType: "alias",
		ValidIf:  accountAlias,
	}
	return input
}

// WithMetadata sets the metadata for the operation route
func (input *CreateOperationRouteInput) WithMetadata(metadata map[string]any) *CreateOperationRouteInput {
	input.Metadata = metadata
	return input
}

// Validate validates the CreateOperationRouteInput
func (input *CreateOperationRouteInput) Validate() error {
	if input.Title == "" {
		return fmt.Errorf("title is required")
	}

	if input.OperationType == "" {
		return fmt.Errorf("operationType is required")
	}

	// Validate operationType is one of the allowed values
	validTypes := []string{"source", "destination"}
	isValid := false
	for _, validType := range validTypes {
		if input.OperationType == validType {
			isValid = true
			break
		}
	}
	if !isValid {
		return fmt.Errorf("operationType must be one of: source, destination")
	}

	if input.Account.RuleType == "" {
		return fmt.Errorf("account.ruleType is required")
	}

	// Validate ruleType is one of the allowed values
	if input.Account.RuleType != "alias" && input.Account.RuleType != "account_type" {
		return fmt.Errorf("account.ruleType must be 'alias' or 'account_type'")
	}

	if input.Account.ValidIf == nil {
		return fmt.Errorf("account.validIf is required")
	}

	// Validate validIf based on ruleType (accepts both string and []string)
	if input.Account.RuleType == "alias" {
		switch v := input.Account.ValidIf.(type) {
		case string:
			if v == "" {
				return fmt.Errorf("account.validIf cannot be empty when ruleType is 'alias'")
			}
		case []string:
			if len(v) == 0 {
				return fmt.Errorf("account.validIf cannot be empty when ruleType is 'alias'")
			}
			for _, alias := range v {
				if alias == "" {
					return fmt.Errorf("account.validIf cannot contain empty strings when ruleType is 'alias'")
				}
			}
		default:
			return fmt.Errorf("account.validIf must be a string or string array when ruleType is 'alias'")
		}
	} else if input.Account.RuleType == "account_type" {
		switch v := input.Account.ValidIf.(type) {
		case string:
			if v == "" {
				return fmt.Errorf("account.validIf cannot be empty when ruleType is 'account_type'")
			}
		case []string:
			if len(v) == 0 {
				return fmt.Errorf("account.validIf cannot be empty when ruleType is 'account_type'")
			}
			for _, accountType := range v {
				if accountType == "" {
					return fmt.Errorf("account.validIf cannot contain empty strings when ruleType is 'account_type'")
				}
			}
		default:
			return fmt.Errorf("account.validIf must be a string or string array when ruleType is 'account_type'")
		}
	}

	return nil
}

// UpdateOperationRouteInput represents the input for updating an operation route
type UpdateOperationRouteInput struct {
	Title       *string                `json:"title,omitempty"`
	Description *string                `json:"description,omitempty"`
	Account     *OperationRouteAccount `json:"account,omitempty"`
	Metadata    *map[string]any        `json:"metadata,omitempty"`
}

// NewUpdateOperationRouteInput creates a new UpdateOperationRouteInput
func NewUpdateOperationRouteInput() *UpdateOperationRouteInput {
	return &UpdateOperationRouteInput{}
}

// WithTitle sets the title for the update
func (input *UpdateOperationRouteInput) WithTitle(title string) *UpdateOperationRouteInput {
	input.Title = &title
	return input
}

// WithDescription sets the description for the update
func (input *UpdateOperationRouteInput) WithDescription(description string) *UpdateOperationRouteInput {
	input.Description = &description
	return input
}


// WithAccount sets the account rules for the update
func (input *UpdateOperationRouteInput) WithAccount(account *OperationRouteAccount) *UpdateOperationRouteInput {
	input.Account = account
	return input
}

// WithAccountTypes sets the account types validation for the operation route update (ruleType: account_type)
func (input *UpdateOperationRouteInput) WithAccountTypes(accountTypes []string) *UpdateOperationRouteInput {
	input.Account = &OperationRouteAccount{
		RuleType: "account_type",
		ValidIf:  accountTypes,
	}
	return input
}

// WithAccountType sets a single account type validation for the operation route update (ruleType: account_type)
func (input *UpdateOperationRouteInput) WithAccountType(accountType string) *UpdateOperationRouteInput {
	input.Account = &OperationRouteAccount{
		RuleType: "account_type",
		ValidIf:  accountType,
	}
	return input
}

// WithAccountAliases sets multiple account aliases validation for the operation route update (ruleType: alias)
func (input *UpdateOperationRouteInput) WithAccountAliases(accountAliases []string) *UpdateOperationRouteInput {
	input.Account = &OperationRouteAccount{
		RuleType: "alias",
		ValidIf:  accountAliases,
	}
	return input
}

// WithAccountAlias sets a single account alias validation for the operation route update (ruleType: alias)
func (input *UpdateOperationRouteInput) WithAccountAlias(accountAlias string) *UpdateOperationRouteInput {
	input.Account = &OperationRouteAccount{
		RuleType: "alias",
		ValidIf:  accountAlias,
	}
	return input
}

// WithMetadata sets the metadata for the update
func (input *UpdateOperationRouteInput) WithMetadata(metadata map[string]any) *UpdateOperationRouteInput {
	input.Metadata = &metadata
	return input
}

// Validate validates the UpdateOperationRouteInput
func (input *UpdateOperationRouteInput) Validate() error {
	// Validate account rules if provided
	if input.Account != nil {
		if input.Account.RuleType == "" {
			return fmt.Errorf("validation failed: account.ruleType is required when account is provided")
		}

		if input.Account.RuleType != "alias" && input.Account.RuleType != "account_type" {
			return fmt.Errorf("validation failed: account.ruleType must be 'alias' or 'account_type'")
		}

		if input.Account.ValidIf == nil {
			return fmt.Errorf("validation failed: account.validIf is required when account is provided")
		}

		// Validate validIf based on ruleType (accepts both string and []string)
		if input.Account.RuleType == "alias" {
			switch v := input.Account.ValidIf.(type) {
			case string:
				if v == "" {
					return fmt.Errorf("validation failed: account.validIf cannot be empty when ruleType is 'alias'")
				}
			case []string:
				if len(v) == 0 {
					return fmt.Errorf("validation failed: account.validIf cannot be empty when ruleType is 'alias'")
				}
				for _, alias := range v {
					if alias == "" {
						return fmt.Errorf("validation failed: account.validIf cannot contain empty strings when ruleType is 'alias'")
					}
				}
			default:
				return fmt.Errorf("validation failed: account.validIf must be a string or string array when ruleType is 'alias'")
			}
		} else if input.Account.RuleType == "account_type" {
			switch v := input.Account.ValidIf.(type) {
			case string:
				if v == "" {
					return fmt.Errorf("validation failed: account.validIf cannot be empty when ruleType is 'account_type'")
				}
			case []string:
				if len(v) == 0 {
					return fmt.Errorf("validation failed: account.validIf cannot be empty when ruleType is 'account_type'")
				}
				for _, accountType := range v {
					if accountType == "" {
						return fmt.Errorf("validation failed: account.validIf cannot contain empty strings when ruleType is 'account_type'")
					}
				}
			default:
				return fmt.Errorf("validation failed: account.validIf must be a string or string array when ruleType is 'account_type'")
			}
		}
	}

	return nil
}

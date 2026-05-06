package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
	"github.com/google/uuid"
)

const (
	maxRouteTitleLength       = 255
	maxRouteDescriptionLength = 250
	maxRouteCodeLength        = 100
)

// AccountingEntries is the structured accounting payload alias.
//
// NOTE: We keep AccountingEntries as an alias to mmodel.AccountingEntries
// because the deeper accounting tree (AccountingEntries → AccountingEntry →
// AccountingRubric) is a wire-format concern with strict server-side
// scenario validation. Hand-mirroring it adds ~150 lines without changing
// the public API shape callers see. The Track 7E decoupling specifically
// targets the 8 entity families flagged in audit 7.1; the accounting tree
// is a transport detail referenced from inside two of those families
// (OperationRoute/UpdateOperationRouteInput) but is not itself flagged.
type AccountingEntries = mmodel.AccountingEntries

// OperationRoute is the SDK-native operation route response type (Track 7E — audit 7.1).
type OperationRoute struct {
	ID                   uuid.UUID          `json:"id,omitempty" example:"01965ed9-7fa4-75b2-8872-fc9e8509ab0a"`
	OrganizationID       uuid.UUID          `json:"organizationId,omitempty" example:"01965ed9-7fa4-75b2-8872-fc9e8509ab0a"`
	LedgerID             uuid.UUID          `json:"ledgerId,omitempty" example:"01965ed9-7fa4-75b2-8872-fc9e8509ab0a"`
	Title                string             `json:"title,omitempty" example:"Cashin from service charge"`
	Description          string             `json:"description,omitempty" example:"This operation route handles cash-in transactions from service charge collections"`
	Code                 string             `json:"code,omitempty" example:"EXT-001"`
	OperationType        string             `json:"operationType,omitempty" example:"source" enums:"source,destination,bidirectional"`
	AccountingEntries    *AccountingEntries `json:"accountingEntries,omitempty"`
	AccountingEntriesRaw json.RawMessage    `json:"-"`
	Metadata             map[string]any     `json:"metadata,omitempty"`
	Account              *AccountRule       `json:"account,omitempty"`
	CreatedAt            time.Time          `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	UpdatedAt            time.Time          `json:"updatedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	DeletedAt            *time.Time         `json:"deletedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
}

// CreateOperationRouteInput is the SDK-native operation route creation payload.
type CreateOperationRouteInput struct {
	Title             string             `json:"title,omitempty" example:"Cashin from service charge"`
	Description       string             `json:"description,omitempty" example:"This operation route handles cash-in transactions from service charge collections"`
	Code              string             `json:"code,omitempty" example:"EXT-001"`
	OperationType     string             `json:"operationType,omitempty" example:"source" enum:"source,destination,bidirectional"`
	AccountingEntries *AccountingEntries `json:"accountingEntries,omitempty"`
	Metadata          map[string]any     `json:"metadata"`
	Account           *AccountRule       `json:"account,omitempty"`
}

// Validate validates the CreateOperationRouteInput fields.
func (input *CreateOperationRouteInput) Validate() error {
	if input == nil {
		return errors.New("input is required")
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return errors.New("title is required")
	}

	if err := validateRouteText(title, maxRouteTitleLength, "title"); err != nil {
		return err
	}

	if err := validateRouteText(input.Description, maxRouteDescriptionLength, "description"); err != nil {
		return err
	}

	if err := validateRouteText(input.Code, maxRouteCodeLength, "code"); err != nil {
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

// UpdateOperationRouteInput is the SDK-native operation route patch payload.
//
// An empty update payload returns a validation error because it would be a
// no-op PATCH. AccountingEntriesRaw preserves explicit null JSON for callers
// that need RFC 7396 merge-patch removal semantics.
type UpdateOperationRouteInput struct {
	Title                string             `json:"title,omitempty" example:"Cashin from service charge"`
	Description          string             `json:"description,omitempty" example:"This operation route handles cash-in transactions from service charge collections"`
	Code                 string             `json:"code,omitempty" example:"EXT-001"`
	AccountingEntries    *AccountingEntries `json:"accountingEntries,omitempty"`
	AccountingEntriesRaw json.RawMessage    `json:"-"`
	Metadata             map[string]any     `json:"metadata"`
	Account              *AccountRule       `json:"account,omitempty"`
}

// Validate validates the UpdateOperationRouteInput fields.
func (input *UpdateOperationRouteInput) Validate() error {
	if input == nil {
		return errors.New("input is required")
	}

	if input.Title != "" && strings.TrimSpace(input.Title) == "" {
		return errors.New("title is required")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	if err := validateRouteText(strings.TrimSpace(input.Title), maxRouteTitleLength, "title"); err != nil {
		return err
	}

	if err := validateRouteText(input.Description, maxRouteDescriptionLength, "description"); err != nil {
		return err
	}

	if err := validateRouteText(input.Code, maxRouteCodeLength, "code"); err != nil {
		return err
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

func (input *UpdateOperationRouteInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return strings.TrimSpace(input.Title) != "" ||
		input.Description != "" ||
		input.Code != "" ||
		input.Metadata != nil ||
		input.Account != nil ||
		input.AccountingEntries != nil ||
		len(input.AccountingEntriesRaw) > 0
}

// MarshalJSON emits only fields explicitly set on the SDK PATCH input.
func (input UpdateOperationRouteInput) MarshalJSON() ([]byte, error) {
	fields := map[string]any{}
	addStringField(fields, "title", input.Title)
	addStringField(fields, "description", input.Description)
	addStringField(fields, "code", input.Code)

	if len(input.AccountingEntriesRaw) > 0 {
		fields["accountingEntries"] = input.AccountingEntriesRaw
	} else if input.AccountingEntries != nil {
		fields["accountingEntries"] = input.AccountingEntries
	}

	addMetadataField(fields, input.Metadata)

	if input.Account != nil {
		fields["account"] = input.Account
	}

	return json.Marshal(fields)
}

func validateRouteText(value string, maxLength int, field string) error {
	if utf8.RuneCountInString(value) > maxLength {
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

	input.Metadata = cloneAnyMap(metadata)

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

// WithAccountAlias sets the account rule to use alias-based selection for UpdateOperationRouteInput.
//
// Parameters:
//   - alias: The account alias to use for selection
//
// Returns:
//   - A pointer to the modified UpdateOperationRouteInput for method chaining
func (input *UpdateOperationRouteInput) WithAccountAlias(alias string) *UpdateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Account = &AccountRule{
		RuleType: "alias",
		ValidIf:  alias,
	}

	return input
}

// WithAccountTypes sets the account rule to use account type-based selection for UpdateOperationRouteInput.
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

// WithMetadata sets the metadata for UpdateOperationRouteInput.
func (input *UpdateOperationRouteInput) WithMetadata(metadata map[string]any) *UpdateOperationRouteInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// AccountRule is the SDK-native account-rule type for operation routes.
type AccountRule struct {
	RuleType string `json:"ruleType,omitempty" example:"alias" enum:"alias,account_type"`
	ValidIf  any    `json:"validIf,omitempty"`
}

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
		Title:         title,
		Description:   description,
		OperationType: operationType,
	}
}

// NewUpdateOperationRouteInput creates a new UpdateOperationRouteInput.
//
// Returns:
//   - A pointer to the newly created UpdateOperationRouteInput
func NewUpdateOperationRouteInput() *UpdateOperationRouteInput {
	return &UpdateOperationRouteInput{}
}

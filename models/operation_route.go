package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
	"github.com/google/uuid"
)

const (
	maxRouteTitleLength       = 255
	maxRouteDescriptionLength = 250
	maxRouteCodeLength        = 100
)

// AccountingRubric represents an accounting rubric with a code and description.
type AccountingRubric struct {
	Code        string `json:"code" validate:"required,max=50" example:"1001"`
	Description string `json:"description" validate:"required,max=250" example:"Cash"`
}

// AccountingEntry represents a single accounting entry with debit and credit rubrics.
type AccountingEntry struct {
	Debit  *AccountingRubric `json:"debit,omitempty" validate:"omitempty"`
	Credit *AccountingRubric `json:"credit,omitempty" validate:"omitempty"`
}

// AccountingEntries groups accounting entries by transaction action type.
type AccountingEntries struct {
	Direct *AccountingEntry `json:"direct,omitempty"`
	Hold   *AccountingEntry `json:"hold,omitempty"`
	Commit *AccountingEntry `json:"commit,omitempty"`
	Cancel *AccountingEntry `json:"cancel,omitempty"`
	Revert *AccountingEntry `json:"revert,omitempty"`
}

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
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	title := strings.TrimSpace(input.Title)
	if title == "" {
		errs.Append("title", "is required")
	} else {
		appendRouteTextLength(&errs, title, maxRouteTitleLength, "title")
	}

	appendRouteTextLength(&errs, input.Description, maxRouteDescriptionLength, "description")
	appendRouteTextLength(&errs, input.Code, maxRouteCodeLength, "code")

	switch input.OperationType {
	case "":
		errs.Append("operationType", "is required")
	case "source", "destination", "bidirectional":
	default:
		errs.Append("operationType", "must be 'source', 'destination', or 'bidirectional'")
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
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
	Metadata             map[string]any     `json:"metadata,omitempty"`
	Account              *AccountRule       `json:"account,omitempty"`
}

// Validate validates the UpdateOperationRouteInput fields.
func (input *UpdateOperationRouteInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	var errs validation.FieldErrors

	if input.Title != "" && strings.TrimSpace(input.Title) == "" {
		errs.Append("title", "is required")
	} else {
		appendRouteTextLength(&errs, strings.TrimSpace(input.Title), maxRouteTitleLength, "title")
	}

	appendRouteTextLength(&errs, input.Description, maxRouteDescriptionLength, "description")
	appendRouteTextLength(&errs, input.Code, maxRouteCodeLength, "code")

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
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
func (input *UpdateOperationRouteInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

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

// appendRouteTextLength records a length-bound violation onto errs when
// value's rune count exceeds maxLength. No-op for empty values.
func appendRouteTextLength(errs *validation.FieldErrors, value string, maxLength int, field string) {
	if utf8.RuneCountInString(value) > maxLength {
		errs.Append(field, fmt.Sprintf("must be at most %d characters", maxLength))
	}
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

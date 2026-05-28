package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
)

const maxAccountFieldLength = 256

// Account is the SDK-native account response type (Track 7E — audit 7.1).
//
// Account types are deployment-defined Midaz account categories such as
// deposit, savings, loans, marketplace, and creditCard.
//
// Account Statuses:
//   - ACTIVE: The account is in use and can participate in transactions
//   - INACTIVE: The account is temporarily not in use but can be reactivated
//   - CLOSED: The account is permanently closed and cannot be used in new transactions
//   - PENDING: The account is awaiting approval or activation
type Account struct {
	ID              string         `json:"id" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	Name            string         `json:"name" example:"Corporate Checking Account" maxLength:"256"`
	ParentAccountID *string        `json:"parentAccountId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	EntityID        *string        `json:"entityId" example:"EXT-ACC-12345" maxLength:"256"`
	AssetCode       string         `json:"assetCode" example:"USD" maxLength:"100"`
	OrganizationID  string         `json:"organizationId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	LedgerID        string         `json:"ledgerId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	PortfolioID     *string        `json:"portfolioId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	SegmentID       *string        `json:"segmentId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	Status          Status         `json:"status"`
	Alias           *string        `json:"alias" example:"@treasury_checking" maxLength:"100"`
	Type            string         `json:"type" example:"deposit"`
	Blocked         *bool          `json:"blocked"`
	CreatedAt       time.Time      `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	UpdatedAt       time.Time      `json:"updatedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	DeletedAt       *time.Time     `json:"deletedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	NullFields      []string       `json:"-"`
}

// AccountHelpers provides utility functions for working with Account entities.
// These helper functions provide SDK-specific conveniences for working with
// the SDK-native Account struct.

// GetAccountAlias safely returns the account alias or empty string if nil.
// This function prevents nil pointer exceptions when accessing the alias.
func GetAccountAlias(account Account) string {
	if account.Alias == nil {
		return ""
	}

	return *account.Alias
}

// GetAccountIdentifier returns the best identifier for an account:
// - Returns the alias if available
// - Falls back to ID if alias is not set
//
// This helps prevent nil pointer exceptions and provides a consistent
// way to reference accounts across the application.
func GetAccountIdentifier(account Account) string {
	if account.Alias != nil {
		return *account.Alias
	}

	return account.ID
}

// CreateAccountInput is the input for creating an account.
// This structure contains all the fields that can be specified when creating a new account.
//
// See also:
//   - [CreateAccountInput.Validate] — multi-field validation accumulator.
//   - [UpdateAccountInput] — partial-update shape.
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/entities.AccountsService.CreateAccount]
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithIdempotencyKey] — make creation safe under retries.
type CreateAccountInput struct {
	// Name is the human-readable name of the account.
	// Max length: 256 characters. Optional in the Midaz API.
	Name string `json:"name,omitempty"`

	// ParentAccountID is the ID of the parent account, if this is a sub-account.
	// Must be a valid UUID if provided.
	ParentAccountID *string `json:"parentAccountId,omitempty"`

	// EntityID is an optional external identifier for the account owner.
	// Max length: 256 characters.
	EntityID *string `json:"entityId,omitempty"`

	// Blocked indicates whether the account should start blocked.
	Blocked *bool `json:"blocked,omitempty"`

	// AssetCode identifies the type of asset held in this account.
	// Required. Max length: 100 characters.
	AssetCode string `json:"assetCode"`

	// PortfolioID is the optional ID of the portfolio this account belongs to.
	// Must be a valid UUID if provided.
	PortfolioID *string `json:"portfolioId,omitempty"`

	// SegmentID is the optional ID of the segment this account belongs to.
	// Must be a valid UUID if provided.
	SegmentID *string `json:"segmentId,omitempty"`

	// Status represents the current status of the account (e.g., "ACTIVE", "CLOSED").
	Status Status `json:"status"`

	// Alias is an optional human-friendly identifier for the account.
	// Max length: 100 characters.
	Alias *string `json:"alias,omitempty"`

	// Type defines the account type (e.g., "deposit", "savings").
	// Required.
	Type string `json:"type"`

	// Metadata contains additional custom data associated with the account.
	// Keys max length: 100 characters, Values max length: 2000 characters.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Validate checks if the CreateAccountInput meets the validation requirements.
// It returns an error if any of the validation checks fail.
//
// Client-side preconditions worth knowing:
//
//   - ParentAccountID, PortfolioID, and SegmentID, when set, MUST be valid
//     UUID strings. The SDK validates these locally so a typo is surfaced
//     at the call site rather than as a generic 400 from the backend.
//   - AssetCode is required and is loosely validated as a currency code;
//     custom (non-ISO-4217) codes are accepted because the backend owns
//     the canonical asset registry.
//   - Type is required.
func (input *CreateAccountInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	appendStringLength(&errs, "name", input.Name)

	if input.EntityID != nil {
		appendStringLength(&errs, "entityId", *input.EntityID)
	}

	appendOptionalUUID(&errs, "parentAccountId", input.ParentAccountID)
	appendOptionalUUID(&errs, "portfolioId", input.PortfolioID)
	appendOptionalUUID(&errs, "segmentId", input.SegmentID)

	if input.AssetCode == "" {
		errs.Append("assetCode", "asset code is required")
	}
	// Note: ISO-4217 currency-code validation is intentionally not
	// enforced client-side. Callers may use custom asset codes which
	// the server validates against the configured asset universe.

	appendAccountTypeContract(&errs, input.Type)

	if input.Alias != nil && *input.Alias != "" {
		if err := core.ValidateAccountAlias(*input.Alias); err != nil {
			errs.Append("alias", err.Error())
		}
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// NewCreateAccountInput creates a new CreateAccountInput with required fields.
// This constructor ensures that all mandatory fields are provided when creating an account input.
//
// Parameters:
//   - name: Optional human-readable name for the account
//   - assetCode: Code identifying the type of asset for this account
//   - accountType: Type of the account (e.g., "deposit", "savings")
//
// Returns:
//   - A pointer to the newly created CreateAccountInput. Status is left
//     zero so the server applies its canonical default (ACTIVE today, but
//     the SDK does not encode that policy locally — see audit 7.11). Set
//     it explicitly with WithStatus when you need a non-default value.
func NewCreateAccountInput(name, assetCode, accountType string) *CreateAccountInput {
	return &CreateAccountInput{
		Name:      name,
		AssetCode: assetCode,
		Type:      accountType,
	}
}

// WithParentAccountID sets the parent account ID.
// This is used when creating a sub-account that belongs to a parent account.
//
// Parameters:
//   - parentAccountID: The ID of the parent account
//
// Returns:
//   - A pointer to the modified CreateAccountInput for method chaining
func (input *CreateAccountInput) WithParentAccountID(parentAccountID string) *CreateAccountInput {
	if input == nil {
		return nil
	}

	input.ParentAccountID = &parentAccountID

	return input
}

// WithEntityID sets the entity ID.
// The entity ID can be used to associate the account with an external entity.
//
// Parameters:
//   - entityID: The external entity identifier
//
// Returns:
//   - A pointer to the modified CreateAccountInput for method chaining
func (input *CreateAccountInput) WithEntityID(entityID string) *CreateAccountInput {
	if input == nil {
		return nil
	}

	input.EntityID = &entityID

	return input
}

// WithPortfolioID sets the portfolio ID.
// This associates the account with a specific portfolio.
//
// Parameters:
//   - portfolioID: The ID of the portfolio
//
// Returns:
//   - A pointer to the modified CreateAccountInput for method chaining
func (input *CreateAccountInput) WithPortfolioID(portfolioID string) *CreateAccountInput {
	if input == nil {
		return nil
	}

	input.PortfolioID = &portfolioID

	return input
}

// WithSegmentID sets the segment ID.
// This associates the account with a specific segment within a portfolio.
//
// Parameters:
//   - segmentID: The ID of the segment
//
// Returns:
//   - A pointer to the modified CreateAccountInput for method chaining
func (input *CreateAccountInput) WithSegmentID(segmentID string) *CreateAccountInput {
	if input == nil {
		return nil
	}

	input.SegmentID = &segmentID

	return input
}

// WithStatus sets a custom status.
// This overrides the default "ACTIVE" status set by the constructor.
//
// Parameters:
//   - status: The status to set for the account
//
// Returns:
//   - A pointer to the modified CreateAccountInput for method chaining
func (input *CreateAccountInput) WithStatus(status Status) *CreateAccountInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

// WithAlias sets the account alias.
// An alias provides a human-friendly identifier for the account.
//
// Parameters:
//   - alias: The alias to set for the account
//
// Returns:
//   - A pointer to the modified CreateAccountInput for method chaining
func (input *CreateAccountInput) WithAlias(alias string) *CreateAccountInput {
	if input == nil {
		return nil
	}

	input.Alias = &alias

	return input
}

// WithMetadata sets the metadata.
// Metadata can store additional custom information about the account.
//
// Parameters:
//   - metadata: A map of key-value pairs to store as metadata
//
// Returns:
//   - A pointer to the modified CreateAccountInput for method chaining
func (input *CreateAccountInput) WithMetadata(metadata map[string]any) *CreateAccountInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithBlocked sets whether the account should start blocked.
func (input *CreateAccountInput) WithBlocked(blocked bool) *CreateAccountInput {
	if input == nil {
		return nil
	}

	input.Blocked = &blocked

	return input
}

// NOTE: ToMmodel was retired in Track 7E. CreateAccountInput is now SDK-owned
// with identical JSON tags to the wire format, so the bridge function is
// unnecessary. Internal callers should pass the SDK type directly.

// UpdateAccountInput is the input for updating an account.
// This structure contains the fields that can be modified when updating an existing account.
type UpdateAccountInput struct {
	// Name is the human-readable name of the account.
	// Max length: 256 characters.
	Name string `json:"name,omitempty"`

	// SegmentID is the optional ID of the segment this account belongs to.
	// Must be a valid UUID if provided.
	SegmentID *string `json:"segmentId,omitempty"`

	// PortfolioID is the optional ID of the portfolio this account belongs to.
	// Must be a valid UUID if provided.
	PortfolioID *string `json:"portfolioId,omitempty"`

	// Status represents the current status of the account (e.g., "ACTIVE", "CLOSED").
	Status Status `json:"status"`

	// Metadata contains additional custom data associated with the account.
	// Keys max length: 100 characters, Values max length: 2000 characters.
	Metadata map[string]any `json:"metadata,omitempty"`

	// EntityID is an optional external identifier for the account owner.
	EntityID *string `json:"entityId,omitempty"`

	// Blocked indicates whether the account should be blocked.
	Blocked *bool `json:"blocked,omitempty"`
}

// Validate checks if the UpdateAccountInput meets the validation requirements.
// All field-level violations are accumulated and surfaced together.
// The empty-payload check (no changes set) is the only gate that
// short-circuits — when nothing is being updated the request is
// rejected before per-field analysis.
func (input *UpdateAccountInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	var errs validation.FieldErrors

	appendStringLength(&errs, "name", input.Name)

	if input.EntityID != nil {
		appendStringLength(&errs, "entityId", *input.EntityID)
	}

	appendOptionalUUID(&errs, "portfolioId", input.PortfolioID)
	appendOptionalUUID(&errs, "segmentId", input.SegmentID)

	// Status is an enum type validated by the server.

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// MarshalJSON emits only fields explicitly set on the SDK PATCH input.
func (input *UpdateAccountInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "name", input.Name)

	if input.SegmentID != nil {
		fields["segmentId"] = input.SegmentID
	}

	if input.PortfolioID != nil {
		fields["portfolioId"] = input.PortfolioID
	}

	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	if input.EntityID != nil {
		fields["entityId"] = input.EntityID
	}

	if input.Blocked != nil {
		fields["blocked"] = input.Blocked
	}

	return json.Marshal(fields)
}

// appendStringLength records a length-bound violation onto errs when
// value exceeds maxAccountFieldLength. No-op for empty values (those are
// handled by required-field checks at the call site).
func appendStringLength(errs *validation.FieldErrors, field, value string) {
	if value != "" && len(value) > maxAccountFieldLength {
		errs.Append(field, fmt.Sprintf("must be at most %d characters", maxAccountFieldLength))
	}
}

// appendOptionalUUID records a UUID-format violation onto errs when a
// pointer is non-nil but holds an invalid value. No-op for nil pointers.
func appendOptionalUUID(errs *validation.FieldErrors, field string, value *string) {
	if value == nil {
		return
	}

	if *value == "" || !validation.IsValidUUID(*value) {
		errs.Append(field, "must be a valid UUID")
	}
}

// appendAccountTypeContract records the account-type-specific contract
// violations onto errs. Required, length-bounded, and the "external"
// type is forbidden client-side because that name is reserved for
// system-managed accounts (see audit 7E).
func appendAccountTypeContract(errs *validation.FieldErrors, accountType string) {
	if accountType == "" {
		errs.Append("type", "account type is required")
		return
	}

	if len(accountType) > maxAccountFieldLength {
		errs.Append("type", fmt.Sprintf("must be at most %d characters", maxAccountFieldLength))
	}

	if accountType == "external" {
		errs.Append("type", "cannot be external")
	}
}

func (input *UpdateAccountInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.Name != "" ||
		input.SegmentID != nil ||
		input.PortfolioID != nil ||
		!IsStatusEmpty(input.Status) ||
		input.Metadata != nil ||
		input.EntityID != nil ||
		input.Blocked != nil
}

// NewUpdateAccountInput creates a new UpdateAccountInput.
// This constructor initializes an empty update input that can be customized
// using the With* methods.
//
// Returns:
//   - A pointer to the newly created UpdateAccountInput
func NewUpdateAccountInput() *UpdateAccountInput {
	return &UpdateAccountInput{}
}

// WithName sets the name.
// This updates the human-readable name of the account.
//
// Parameters:
//   - name: The new name for the account
//
// Returns:
//   - A pointer to the modified UpdateAccountInput for method chaining
func (input *UpdateAccountInput) WithName(name string) *UpdateAccountInput {
	if input == nil {
		return nil
	}

	input.Name = name

	return input
}

// WithEntityID sets the external entity identifier.
func (input *UpdateAccountInput) WithEntityID(entityID string) *UpdateAccountInput {
	if input == nil {
		return nil
	}

	input.EntityID = &entityID

	return input
}

// WithSegmentID sets the segment ID.
// This updates the segment association of the account.
//
// Parameters:
//   - segmentID: The new segment ID
//
// Returns:
//   - A pointer to the modified UpdateAccountInput for method chaining
func (input *UpdateAccountInput) WithSegmentID(segmentID string) *UpdateAccountInput {
	if input == nil {
		return nil
	}

	input.SegmentID = &segmentID

	return input
}

// WithPortfolioID sets the portfolio ID.
// This updates the portfolio association of the account.
//
// Parameters:
//   - portfolioID: The new portfolio ID
//
// Returns:
//   - A pointer to the modified UpdateAccountInput for method chaining
func (input *UpdateAccountInput) WithPortfolioID(portfolioID string) *UpdateAccountInput {
	if input == nil {
		return nil
	}

	input.PortfolioID = &portfolioID

	return input
}

// WithStatus sets the status.
// This updates the status of the account.
//
// Parameters:
//   - status: The new status for the account
//
// Returns:
//   - A pointer to the modified UpdateAccountInput for method chaining
func (input *UpdateAccountInput) WithStatus(status Status) *UpdateAccountInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

// WithMetadata sets the metadata.
// This updates the custom metadata associated with the account.
//
// Parameters:
//   - metadata: The new metadata map
//
// Returns:
//   - A pointer to the modified UpdateAccountInput for method chaining
func (input *UpdateAccountInput) WithMetadata(metadata map[string]any) *UpdateAccountInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithBlocked sets whether the account should be blocked.
func (input *UpdateAccountInput) WithBlocked(blocked bool) *UpdateAccountInput {
	if input == nil {
		return nil
	}

	input.Blocked = &blocked

	return input
}

// NOTE: ToMmodel was retired in Track 7E. See the CreateAccountInput note above.

package models

import (
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation/core"
	"github.com/shopspring/decimal"
)

// maxOperationDescriptionLength bounds the operation Description field on
// updates. The constant is operation-scoped to avoid the previous reuse of
// maxAccountFieldLength, which made "why is the limit on operation
// descriptions named like an account constant" a routine confusion.
const maxOperationDescriptionLength = 256

// Status is defined in common.go.

// Amount structure for marshaling/unmarshalling JSON.
//
// swagger:model Amount
// @Description Amount represents the exact decimal value of an operation amount.
type Amount struct {
	// The exact decimal amount value.
	// example: 1500
	// minimum: 0
	Value *decimal.Decimal `json:"value" example:"1500" minimum:"0"`
} // @name Amount

// IsEmpty method that set empty or nil in fields
func (a Amount) IsEmpty() bool {
	return a.Value == nil
}

// OperationBalance structure for marshaling/unmarshalling JSON.
// Named OperationBalance to avoid conflict with existing Balance model
//
// swagger:model OperationBalance
// @Description OperationBalance represents the account balance snapshot before or after an operation.
type OperationBalance struct {
	// Amount available for transactions.
	// example: 1500
	// minimum: 0
	Available *decimal.Decimal `json:"available" example:"1500" minimum:"0"`

	// Amount on hold and unavailable for transactions.
	// example: 500
	// minimum: 0
	OnHold *decimal.Decimal `json:"onHold" example:"500" minimum:"0"`

	// Version is the optimistic concurrency version of the balance.
	Version int64 `json:"version,omitempty" example:"1" minimum:"1"`

	// OverdraftUsed is the amount of overdraft consumed by this balance snapshot.
	OverdraftUsed *decimal.Decimal `json:"overdraftUsed,omitempty" example:"0" minimum:"0"`
} // @name OperationBalance

// IsEmpty method that set empty or nil in fields
func (b OperationBalance) IsEmpty() bool {
	return b.Available == nil && b.OnHold == nil && b.Version == 0 && b.OverdraftUsed == nil
}

// Operation is a struct designed to encapsulate response payload data.
//
// swagger:model Operation
// @Description Operation is a struct designed to store operation data. Represents a financial operation that affects account balances, including details such as amount, balance before and after, transaction association, and metadata.
type Operation struct {
	// Unique identifier for the operation
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	ID string `json:"id" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// Parent transaction identifier
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	TransactionID string `json:"transactionId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// Human-readable description of the operation
	// example: Credit card operation
	// maxLength: 256
	Description string `json:"description" example:"Credit card operation" maxLength:"256"`

	// Type of operation (e.g., DEBIT, CREDIT)
	// example: DEBIT
	// maxLength: 50
	Type string `json:"type" example:"DEBIT" maxLength:"50"`

	// Asset code for the operation
	// example: BRL
	// minLength: 2
	// maxLength: 10
	AssetCode string `json:"assetCode" example:"BRL" minLength:"2" maxLength:"10"`

	// Chart of accounts code for accounting purposes
	// example: 1000
	// maxLength: 20
	ChartOfAccounts string `json:"chartOfAccounts" example:"1000" maxLength:"20"`

	// Operation amount information
	Amount Amount `json:"amount"`

	// Balance before the operation
	Balance OperationBalance `json:"balance"`

	// Balance after the operation
	BalanceAfter OperationBalance `json:"balanceAfter"`

	// Operation status information
	Status Status `json:"status"`

	// Account identifier associated with this operation
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	AccountID string `json:"accountId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// Human-readable alias for the account
	// example: @person1
	// maxLength: 256
	AccountAlias string `json:"accountAlias" example:"@person1" maxLength:"256"`

	// Balance identifier affected by this operation
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	BalanceID string `json:"balanceId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// Organization identifier
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	OrganizationID string `json:"organizationId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// Ledger identifier
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	LedgerID string `json:"ledgerId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// Route
	// example: 00000000-0000-0000-0000-000000000000
	// format: string
	Route string `json:"route" example:"00000000-0000-0000-0000-000000000000" format:"string"`

	// RouteID is the UUID of the operation route.
	RouteID string `json:"routeId,omitempty"`

	// RouteCode is the human-readable code of the operation route.
	RouteCode string `json:"routeCode,omitempty"`

	// RouteDescription is the human-readable description of the operation route.
	RouteDescription string `json:"routeDescription,omitempty"`

	// Direction indicates whether the operation is a debit or credit.
	Direction string `json:"direction,omitempty"`

	// BalanceAffected indicates whether the operation changes balances.
	BalanceAffected *bool `json:"balanceAffected,omitempty"`

	// BalanceKey is the key of the affected balance.
	BalanceKey string `json:"balanceKey,omitempty"`

	// Timestamp when the operation was created
	// example: 2021-01-01T00:00:00Z
	// format: date-time
	CreatedAt time.Time `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`

	// Timestamp when the operation was last updated
	// example: 2021-01-01T00:00:00Z
	// format: date-time
	UpdatedAt time.Time `json:"updatedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`

	// Timestamp when the operation was deleted (if soft-deleted)
	// example: 2021-01-01T00:00:00Z
	// format: date-time
	DeletedAt *time.Time `json:"deletedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`

	// Additional custom attributes
	// example: {"reason": "Purchase refund", "reference": "INV-12345"}
	Metadata map[string]any `json:"metadata"`
} // @name Operation

// MetadataOrEmpty returns o.Metadata when non-nil, or a freshly-allocated
// empty map when the server omitted metadata. The SDK no longer mutates
// the wire shape (nil stays nil), so consumers that want a guaranteed-
// non-nil view for downstream code should reach for this accessor
// instead of writing the same nil-check at every call site.
func (o *Operation) MetadataOrEmpty() map[string]any {
	if o == nil || o.Metadata == nil {
		return map[string]any{}
	}

	return o.Metadata
}

// UpdateOperationInput is a struct design to encapsulate payload data.
//
// swagger:model UpdateOperationInput
// @Description UpdateOperationInput is the input payload to update an operation. Contains fields that can be modified after an operation is created.
type UpdateOperationInput struct {
	// Human-readable description of the operation
	// example: Credit card operation
	// maxLength: 256
	Description string `json:"description,omitempty" validate:"max=256" example:"Credit card operation" maxLength:"256"`

	// Additional custom attributes
	// example: {"reason": "Purchase refund", "reference": "INV-12345"}
	Metadata map[string]any `json:"metadata,omitempty"`
} // @name UpdateOperationInput

// Validate validates the UpdateOperationInput fields.
func (input *UpdateOperationInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.Description == "" && len(input.Metadata) == 0 {
		return errors.New("empty update payload not allowed")
	}

	var errs validation.FieldErrors

	if utf8.RuneCountInString(input.Description) > maxOperationDescriptionLength {
		errs.Append("description", fmt.Sprintf("must be at most %d characters", maxOperationDescriptionLength))
	}

	if len(input.Metadata) > 0 {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// OperationAmount represents the amount structure in operation responses
// This is SDK-specific and used for backward compatibility
type OperationAmount struct {
	// Value is the string representation of the amount
	Value string `json:"value"`
}

// OperationType represents the type of an operation.
// This is typically either a debit or credit in double-entry accounting.
type OperationType string

const (
	// OperationTypeDebit represents a debit operation.
	// In accounting, a debit typically increases asset and expense accounts,
	// and decreases liability, equity, and revenue accounts.
	OperationTypeDebit OperationType = "DEBIT"

	// OperationTypeCredit represents a credit operation.
	// In accounting, a credit typically increases liability, equity, and revenue accounts,
	// and decreases asset and expense accounts.
	OperationTypeCredit OperationType = "CREDIT"
)

// Source represents the source of an operation.
// This identifies where funds or assets are coming from in a transaction.
type Source struct {
	// ID is the unique identifier for the source account
	ID string `json:"id"`

	// Alias is an optional human-readable name for the source account
	Alias *string `json:"alias,omitempty"`

	// Destination indicates if this source is also a destination
	Destination bool `json:"destination"`
}

// Destination represents the destination of an operation.
// This identifies where funds or assets are going to in a transaction.
type Destination struct {
	// ID is the unique identifier for the destination account
	ID string `json:"id"`

	// Alias is an optional human-readable name for the destination account
	Alias *string `json:"alias,omitempty"`

	// Source indicates if this destination is also a source
	Source bool `json:"source"`
}

// CreateOperationInput is the input for creating an operation.
// This structure contains all the fields needed to create a new operation
// as part of a transaction.
type CreateOperationInput struct {
	// Type indicates whether this is a debit or credit operation.
	// Must be either "DEBIT" or "CREDIT" (canonical uppercase per the
	// Midaz API contract; see OperationTypeDebit / OperationTypeCredit).
	Type string `json:"type"`

	// AccountID is the identifier of the account to be affected
	// This must be a valid account ID in the ledger
	AccountID string `json:"accountId"`

	// Amount is the exact decimal value of the operation. Use Amount or *Amount
	// when decimal scaling is required, or pass an already-formatted decimal
	// string. Raw numeric types such as int and float are treated as literal
	// decimal strings by normalizedOperationAmount through decimalStringFromAny.
	Amount any `json:"amount"`

	// AssetCode identifies the currency or asset type for this operation
	// Common examples include "USD", "EUR", "BTC", etc.
	AssetCode string `json:"assetCode,omitempty"`

	// AccountAlias is an optional human-readable name for the account
	// This can be used to reference accounts by their alias instead of ID
	// Format is typically "<type>:<identifier>[:subtype]", e.g., "customer:john.doe"
	AccountAlias *string `json:"accountAlias,omitempty"`

	// Route is the operation route identifier to use for this operation
	// This links the operation to a specific routing rule that determines
	// how the operation should be processed and what account rules to apply
	Route string `json:"route,omitempty"`
}

// Validate checks that the CreateOperationInput meets all validation requirements.
// It ensures that required fields are present and that all fields meet their
// validation constraints as defined in the API specification.
//
// Returns:
//   - error: An error if validation fails, nil otherwise
func (input *CreateOperationInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	switch {
	case input.Type == "":
		errs.Append("type", "is required")
	case input.Type != string(OperationTypeDebit) && input.Type != string(OperationTypeCredit):
		errs.Append("type", fmt.Sprintf("must be either %s or %s, got %s", OperationTypeDebit, OperationTypeCredit, input.Type))
	}

	if input.AccountID == "" {
		errs.Append("accountId", "is required")
	}

	if input.Amount == nil {
		errs.Append("amount", "is required")
	} else if err := validatePositiveDecimalString(input.Amount, "amount"); err != nil {
		errs.Append("amount", "invalid: "+err.Error())
	}

	if input.AssetCode == "" {
		errs.Append("assetCode", "is required")
	}

	return errs.OrNil()
}

// NewCreateOperationInput creates a new CreateOperationInput with the required fields.
// Use the With* methods to set optional fields fluently.
//
// Parameters:
//   - operationType: One of OperationTypeDebit ("DEBIT") or OperationTypeCredit ("CREDIT").
//   - accountID:     The unique identifier of the account to be affected.
//   - amount:        The exact decimal value of the operation. Use *decimal.Decimal,
//     models.Amount, or a pre-formatted decimal string.
//   - assetCode:     The currency or asset code (e.g. "USD", "EUR", "BTC").
//
// Returns:
//   - A pointer to the newly created CreateOperationInput.
//
// Example:
//
//	input := models.NewCreateOperationInput(
//	    string(models.OperationTypeDebit),
//	    "00000000-0000-0000-0000-000000000000",
//	    "150.00",
//	    "USD",
//	).WithRoute("payment-route").WithAccountAlias("@customer.john")
func NewCreateOperationInput(operationType, accountID string, amount any, assetCode string) *CreateOperationInput {
	return &CreateOperationInput{
		Type:      operationType,
		AccountID: accountID,
		Amount:    amount,
		AssetCode: assetCode,
	}
}

// WithAccountAlias sets the optional human-readable alias for the account.
// Returns the modified input for chaining.
func (input *CreateOperationInput) WithAccountAlias(alias string) *CreateOperationInput {
	if input == nil {
		return nil
	}

	input.AccountAlias = &alias

	return input
}

// WithRoute sets the optional operation-route identifier.
// Returns the modified input for chaining.
func (input *CreateOperationInput) WithRoute(route string) *CreateOperationInput {
	if input == nil {
		return nil
	}

	input.Route = route

	return input
}

// NewUpdateOperationInput creates a new empty UpdateOperationInput.
// Use the With* methods to set optional fields fluently.
//
// Returns:
//   - A pointer to the newly created UpdateOperationInput.
//
// Example:
//
//	input := models.NewUpdateOperationInput().
//	    WithDescription("refund for invoice 12345").
//	    WithMetadata(map[string]any{"reason": "customer requested"})
func NewUpdateOperationInput() *UpdateOperationInput {
	return &UpdateOperationInput{}
}

// WithDescription sets the human-readable description on the update payload.
// Returns the modified input for chaining.
func (input *UpdateOperationInput) WithDescription(description string) *UpdateOperationInput {
	if input == nil {
		return nil
	}

	input.Description = description

	return input
}

// WithMetadata sets the operation metadata on the update payload. The map is
// deep-copied so subsequent caller mutations do not leak into the input.
// Returns the modified input for chaining.
func (input *UpdateOperationInput) WithMetadata(metadata map[string]any) *UpdateOperationInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

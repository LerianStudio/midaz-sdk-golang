package models

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Note: Status type is defined in common.go as Status = mmodel.Status

// Amount structure for marshaling/unmarshalling JSON.
//
// swagger:model Amount
// @Description Amount is the struct designed to represent the amount of an operation. Contains the value and scale (decimal places) of an operation amount.
type Amount struct {
	// The amount value in the smallest unit of the asset (e.g., cents)
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
// @Description OperationBalance is the struct designed to represent the account balance. Contains available and on-hold amounts along with the scale (decimal places).
type OperationBalance struct {
	// Amount available for transactions (in the smallest unit of asset)
	// example: 1500
	// minimum: 0
	Available *decimal.Decimal `json:"available" example:"1500" minimum:"0"`

	// Amount on hold and unavailable for transactions (in the smallest unit of asset)
	// example: 500
	// minimum: 0
	OnHold *decimal.Decimal `json:"onHold" example:"500" minimum:"0"`
} // @name OperationBalance

// IsEmpty method that set empty or nil in fields
func (b OperationBalance) IsEmpty() bool {
	return b.Available == nil && b.OnHold == nil
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

// UpdateOperationInput is a struct design to encapsulate payload data.
//
// swagger:model UpdateOperationInput
// @Description UpdateOperationInput is the input payload to update an operation. Contains fields that can be modified after an operation is created.
type UpdateOperationInput struct {
	// Human-readable description of the operation
	// example: Credit card operation
	// maxLength: 256
	Description string `json:"description" validate:"max=256" example:"Credit card operation" maxLength:"256"`

	// Additional custom attributes
	// example: {"reason": "Purchase refund", "reference": "INV-12345"}
	Metadata map[string]any `json:"metadata" validate:"dive,keys,keymax=100,endkeys,omitempty,nonested,valuemax=2000"`
} // @name UpdateOperationInput

// Operations represents a paginated list of operations.
//
// swagger:model Operations
// @Description Operations represents a paginated response containing a list of operations with pagination metadata.
type Operations struct {
	// Array of operation records returned in this page
	Items []Operation `json:"items"`

	// Pagination information
	Pagination struct {
		Limit      int     `json:"limit"`
		NextCursor *string `json:"next_cursor,omitempty"`
		PrevCursor *string `json:"prev_cursor,omitempty"`
	} `json:"pagination"`
} // @name Operations

// OperationResponse represents a success response containing a single operation.
//
// swagger:response OperationResponse
// @Description Successful response containing a single operation entity.
type OperationResponse struct {
	// in: body
	Body Operation
}

// OperationsResponse represents a success response containing a paginated list of operations.
//
// swagger:response OperationsResponse
// @Description Successful response containing a paginated list of operations.
type OperationsResponse struct {
	// in: body
	Body Operations
}

// OperationLog is a struct designed to represent the operation data that should be stored in the audit log
//
// @Description Immutable log entry for audit purposes representing a snapshot of operation state at a specific point in time.
type OperationLog struct {
	// Unique identifier for the operation
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	ID string `json:"id" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// Parent transaction identifier
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	TransactionID string `json:"transactionId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// Type of operation (e.g., creditCard, transfer, payment)
	// example: creditCard
	// maxLength: 50
	Type string `json:"type" example:"creditCard" maxLength:"50"`

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

	// Timestamp when the operation log was created
	// example: 2021-01-01T00:00:00Z
	// format: date-time
	CreatedAt time.Time `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`

	// Additional custom attributes for audit tracking
	// example: {"audit_user": "system", "source": "api"}
	Metadata map[string]any `json:"metadata"`
} // @name OperationLog

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
	// Type indicates whether this is a debit or credit operation
	// Must be either "debit" or "credit"
	Type string `json:"type"`

	// AccountID is the identifier of the account to be affected
	// This must be a valid account ID in the ledger
	AccountID string `json:"accountId"`

	// Amount is the numeric value of the operation as a decimal string
	// Examples: "100.50", "1000.00", "0.25"
	Amount string `json:"amount"`

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
	// Validate required fields
	if input.Type == "" {
		return fmt.Errorf("type is required")
	}

	// Validate type is a valid operation type
	if input.Type != string(OperationTypeDebit) && input.Type != string(OperationTypeCredit) {
		return fmt.Errorf("type must be either %s or %s, got %s", OperationTypeDebit, OperationTypeCredit, input.Type)
	}

	if input.AccountID == "" {
		return fmt.Errorf("accountId is required")
	}

	// Validate amount
	if input.Amount == "" {
		return fmt.Errorf("amount is required")
	}

	// Validate asset code if provided
	if input.AssetCode == "" {
		return fmt.Errorf("assetCode is required")
	}

	return nil
}

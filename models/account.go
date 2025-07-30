// Package models defines the data models used by the Midaz SDK.
package models

import (
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation/core"
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

// Account represents an account in the Midaz Ledger.
// This is now an alias to mmodel.Account to avoid duplication while maintaining
// SDK-specific documentation and examples.
//
// Account Types:
//   - ASSET: Represents resources owned by the entity (e.g., cash, inventory, receivables)
//   - LIABILITY: Represents obligations owed by the entity (e.g., loans, payables)
//   - EQUITY: Represents the residual interest in the assets after deducting liabilities
//   - REVENUE: Represents increases in economic benefits during the accounting period
//   - EXPENSE: Represents decreases in economic benefits during the accounting period
//
// Account Statuses:
//   - ACTIVE: The account is in use and can participate in transactions
//   - INACTIVE: The account is temporarily not in use but can be reactivated
//   - CLOSED: The account is permanently closed and cannot be used in new transactions
//   - PENDING: The account is awaiting approval or activation
//
// Example Usage:
//
//	// Create a new customer asset account using the builder pattern
//	customerAccount := models.NewCreateAccountInput(
//	    "John Doe",
//	    "USD", 
//	    "ASSET",
//	).WithAlias("customer:john.doe").
//	  WithMetadata(map[string]any{
//	    "customer_id": "cust-123",
//	    "email": "john.doe@example.com",
//	    "account_manager": "manager-456",
//	  })
//
// Portfolio and Segment Organization:
// Accounts can be organized into portfolios and segments for better categorization
// and reporting. Portfolios represent high-level groupings (e.g., "Investments"),
// while segments provide finer-grained classification within portfolios
// (e.g., "US Equities", "International Bonds").
type Account = mmodel.Account



// AccountHelpers provides utility functions for working with Account entities.
// These helper functions provide SDK-specific conveniences while using mmodel.Account directly.

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
type CreateAccountInput struct {
	// Name is the human-readable name of the account.
	// Max length: 256 characters.
	Name string `json:"name"`

	// ParentAccountID is the ID of the parent account, if this is a sub-account.
	// Must be a valid UUID if provided.
	ParentAccountID *string `json:"parentAccountId,omitempty"`

	// EntityID is an optional external identifier for the account owner.
	// Max length: 256 characters.
	EntityID *string `json:"entityId,omitempty"`

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

	// Type defines the account type (e.g., "ASSET", "LIABILITY", "EQUITY").
	// Required.
	Type string `json:"type"`

	// Metadata contains additional custom data associated with the account.
	// Keys max length: 100 characters, Values max length: 2000 characters.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Validate checks if the CreateAccountInput meets the validation requirements.
// It returns an error if any of the validation checks fail.
func (input *CreateAccountInput) Validate() error {
	if input.Name == "" {
		return fmt.Errorf("name is required")
	}

	if len(input.Name) > 256 {
		return fmt.Errorf("name must be at most 256 characters")
	}

	if input.AssetCode == "" {
		return fmt.Errorf("asset code is required")
	}

	// Validate asset code using the core validation package
	if err := core.ValidateCurrencyCode(input.AssetCode); err != nil { //nolint:revive,staticcheck // Intentionally empty to allow custom asset codes
		// If not a valid currency, it might be a custom asset code
		// which should be validated by the backend
	}

	if input.Type == "" {
		return fmt.Errorf("account type is required")
	}

	// Validate account type using the core validation package
	if err := core.ValidateAccountType(input.Type); err != nil {
		return fmt.Errorf("invalid account type: %w", err)
	}

	// Validate alias if provided using the core validation package
	if input.Alias != nil && *input.Alias != "" {
		if err := core.ValidateAccountAlias(*input.Alias); err != nil {
			return err
		}
	}

	return nil
}

// NewCreateAccountInput creates a new CreateAccountInput with required fields.
// This constructor ensures that all mandatory fields are provided when creating an account input.
//
// Parameters:
//   - name: Human-readable name for the account
//   - assetCode: Code identifying the type of asset for this account
//   - accountType: Type of the account (e.g., "ASSET", "LIABILITY", "EQUITY")
//
// Returns:
//   - A pointer to the newly created CreateAccountInput with default active status
func NewCreateAccountInput(name, assetCode, accountType string) *CreateAccountInput {
	return &CreateAccountInput{
		Name:      name,
		AssetCode: assetCode,
		Type:      accountType,
		Status:    NewStatus("ACTIVE"), // Default status
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
	input.Metadata = metadata
	return input
}

// ToMmodel converts the SDK CreateAccountInput to mmodel.CreateAccountInput.
// This method is used internally to convert between SDK and backend models.
func (input CreateAccountInput) ToMmodel() mmodel.CreateAccountInput {
	return mmodel.CreateAccountInput{
		Name:            input.Name,
		ParentAccountID: input.ParentAccountID,
		EntityID:        input.EntityID,
		AssetCode:       input.AssetCode,
		PortfolioID:     input.PortfolioID,
		SegmentID:       input.SegmentID,
		Status:          input.Status,
		Alias:           input.Alias,
		Type:            input.Type,
		Metadata:        input.Metadata,
	}
}

// UpdateAccountInput is the input for updating an account.
// This structure contains the fields that can be modified when updating an existing account.
type UpdateAccountInput struct {
	// Name is the human-readable name of the account.
	// Max length: 256 characters.
	Name string `json:"name"`

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
}

// Validate checks if the UpdateAccountInput meets the validation requirements.
// It returns an error if any of the validation checks fail.
func (input *UpdateAccountInput) Validate() error {
	if input.Name != "" && len(input.Name) > 256 {
		return fmt.Errorf("name must be at most 256 characters")
	}

	// Validate status if provided
	// Status is an enum type, so we don't need additional validation here
	// The API will validate if the status is valid

	// Validate metadata if provided
	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
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
	input.Name = name
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
	input.Metadata = metadata
	return input
}

// ToMmodel converts the SDK UpdateAccountInput to mmodel.UpdateAccountInput.
// This method is used internally to convert between SDK and backend models.
func (input UpdateAccountInput) ToMmodel() mmodel.UpdateAccountInput {
	return mmodel.UpdateAccountInput{
		Name:        input.Name,
		SegmentID:   input.SegmentID,
		PortfolioID: input.PortfolioID,
		Status:      input.Status,
		Metadata:    input.Metadata,
	}
}

// Accounts represents a list of accounts.
// This structure is used for paginated responses when listing accounts.
type Accounts struct {
	// Items is the collection of accounts in the current page
	Items []Account `json:"items"`

	// Page is the current page number
	Page int `json:"page"`

	// Limit is the maximum number of items per page
	Limit int `json:"limit"`
}

// FromMmodel converts mmodel.Accounts to SDK Accounts.
// Since Account is now an alias to mmodel.Account, no conversion is needed for items.
func FromMmodel(accounts mmodel.Accounts) Accounts {
	return Accounts{
		Items: accounts.Items, // Direct assignment since Account = mmodel.Account
		Page:  accounts.Page,
		Limit: accounts.Limit,
	}
}

// AccountFilter for filtering accounts in listings.
// This structure defines the criteria for filtering accounts when listing them.
type AccountFilter struct {
	// Status is a list of status codes to filter by
	Status []string `json:"status,omitempty"`
}

// ListAccountInput for configuring account listing requests.
// This structure defines the parameters for listing accounts.
type ListAccountInput struct {
	// Page is the page number to retrieve
	Page int `json:"page,omitempty"`

	// PerPage is the number of items per page
	PerPage int `json:"perPage,omitempty"`

	// Filter contains the filtering criteria
	Filter AccountFilter `json:"filter,omitempty"`
}

// Validate checks if the ListAccountInput meets the validation requirements.
// It returns an error if any of the validation checks fail.
//
// Returns:
//   - error: An error if the input is invalid, nil otherwise
func (input *ListAccountInput) Validate() error {
	// Validate page number if provided
	if input.Page < 0 {
		return fmt.Errorf("page number cannot be negative")
	}

	// Validate per page count if provided
	if input.PerPage < 0 {
		return fmt.Errorf("perPage cannot be negative")
	}

	// Validate maximum per page to prevent excessive resource usage
	if input.PerPage > 100 {
		return fmt.Errorf("perPage cannot exceed 100")
	}

	return nil
}

// ListAccountResponse for account listing responses.
// This structure represents the response from a list accounts request.
type ListAccountResponse struct {
	// Items is the collection of accounts in the current page
	Items []Account `json:"items"`

	// Total is the total number of accounts matching the criteria
	Total int `json:"total"`

	// CurrentPage is the current page number
	CurrentPage int `json:"currentPage"`

	// PageSize is the number of items per page
	PageSize int `json:"pageSize"`

	// TotalPages is the total number of pages
	TotalPages int `json:"totalPages"`
}

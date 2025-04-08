// Package models defines the data models used by the Midaz SDK.
package models

import (
	"fmt"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation/core"
	"github.com/LerianStudio/midaz/pkg/mmodel"
)

// Account represents an account in the Midaz Ledger.
// Accounts are the fundamental entities for tracking assets and their movements
// within the ledger system. Each account belongs to a specific organization and ledger,
// and is associated with a particular asset type.
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
//	// Create a new customer asset account
//	customerAccount := models.NewAccount(
//	    "acc-123",
//	    "John Doe",
//	    "USD",
//	    "org-456",
//	    "ledger-789",
//	    "ASSET",
//	    models.StatusActive,
//	).WithAlias("customer:john.doe").
//	  WithMetadata(map[string]any{
//	    "customer_id": "cust-123",
//	    "email": "john.doe@example.com",
//	    "account_manager": "manager-456",
//	  })
//
//	// Create a revenue account
//	revenueAccount := models.NewAccount(
//	    "acc-456",
//	    "Subscription Revenue",
//	    "USD",
//	    "org-456",
//	    "ledger-789",
//	    "REVENUE",
//	    models.StatusActive,
//	).WithAlias("revenue:subscriptions")
//
// Portfolio and Segment Organization:
// Accounts can be organized into portfolios and segments for better categorization
// and reporting. Portfolios represent high-level groupings (e.g., "Investments"),
// while segments provide finer-grained classification within portfolios
// (e.g., "US Equities", "International Bonds").
type Account struct {
	// ID is the unique identifier for the account
	// This is a system-generated UUID that uniquely identifies the account
	// across the entire Midaz platform.
	ID string `json:"id"`

	// Name is the human-readable name of the account
	// This should be descriptive and meaningful to users, with a maximum
	// length of 256 characters.
	Name string `json:"name"`

	// ParentAccountID is the ID of the parent account, if this is a sub-account
	// This enables hierarchical account structures, where accounts can be
	// nested under parent accounts for better organization.
	ParentAccountID *string `json:"parentAccountId,omitempty"`

	// EntityID is an optional external identifier for the account owner
	// This can be used to link the account to an external system or entity,
	// such as a customer ID in a CRM system.
	EntityID *string `json:"entityId,omitempty"`

	// AssetCode identifies the type of asset held in this account
	// Examples include currency codes like "USD", "EUR", or custom asset
	// codes for commodities, cryptocurrencies, or other assets.
	AssetCode string `json:"assetCode"`

	// OrganizationID is the ID of the organization that owns this account
	// All accounts must belong to an organization, which provides the
	// top-level ownership and access control.
	OrganizationID string `json:"organizationId"`

	// LedgerID is the ID of the ledger that contains this account
	// Accounts are always created within a specific ledger, which defines
	// the accounting boundaries and rules.
	LedgerID string `json:"ledgerId"`

	// PortfolioID is the optional ID of the portfolio this account belongs to
	// Portfolios allow for grouping related accounts together for reporting
	// and management purposes.
	PortfolioID *string `json:"portfolioId,omitempty"`

	// SegmentID is the optional ID of the segment this account belongs to
	// Segments provide finer-grained classification within portfolios,
	// enabling more detailed reporting and analysis.
	SegmentID *string `json:"segmentId,omitempty"`

	// Status represents the current status of the account (e.g., "ACTIVE", "CLOSED")
	// The status determines whether the account can participate in transactions.
	Status Status `json:"status"`

	// Alias is an optional human-friendly identifier for the account
	// Aliases can be used in place of IDs in many API calls, making it easier
	// to reference accounts with meaningful names like "customer:john.doe".
	Alias *string `json:"alias,omitempty"`

	// Type defines the account type (e.g., "ASSET", "LIABILITY", "EQUITY")
	// The type determines the account's behavior in accounting operations
	// and its position in financial statements.
	Type string `json:"type"`

	// Metadata stores additional custom information about the account
	// This can include any arbitrary key-value pairs for application-specific
	// data that doesn't fit into the standard account fields.
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is the timestamp when the account was created
	// This is automatically set by the system and cannot be modified.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is the timestamp when the account was last updated
	// This is automatically updated by the system whenever the account is modified.
	UpdatedAt time.Time `json:"updatedAt"`

	// DeletedAt is the timestamp when the account was deleted, if applicable
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// NewAccount creates a new Account with required fields.
// This constructor ensures that all mandatory fields are provided when creating an account.
//
// Parameters:
//   - id: Unique identifier for the account
//   - name: Human-readable name for the account
//   - assetCode: Code identifying the type of asset for this account
//   - organizationID: ID of the organization that owns this account
//   - ledgerID: ID of the ledger that contains this account
//   - accountType: Type of the account (e.g., "ASSET", "LIABILITY", "EQUITY")
//   - status: Current status of the account
//
// Returns:
//   - A pointer to the newly created Account
func NewAccount(id, name, assetCode, organizationID, ledgerID, accountType string, status Status) *Account {
	return &Account{
		ID:             id,
		Name:           name,
		AssetCode:      assetCode,
		OrganizationID: organizationID,
		LedgerID:       ledgerID,
		Status:         status,
		Type:           accountType,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// WithParentAccountID sets the parent account ID.
// This is used when creating a sub-account that belongs to a parent account.
//
// Parameters:
//   - parentAccountID: The ID of the parent account
//
// Returns:
//   - A pointer to the modified Account for method chaining
func (a *Account) WithParentAccountID(parentAccountID string) *Account {
	a.ParentAccountID = &parentAccountID
	return a
}

// WithEntityID sets the entity ID.
// The entity ID can be used to associate the account with an external entity.
//
// Parameters:
//   - entityID: The external entity identifier
//
// Returns:
//   - A pointer to the modified Account for method chaining
func (a *Account) WithEntityID(entityID string) *Account {
	a.EntityID = &entityID
	return a
}

// WithPortfolioID sets the portfolio ID.
// This associates the account with a specific portfolio.
//
// Parameters:
//   - portfolioID: The ID of the portfolio
//
// Returns:
//   - A pointer to the modified Account for method chaining
func (a *Account) WithPortfolioID(portfolioID string) *Account {
	a.PortfolioID = &portfolioID
	return a
}

// WithSegmentID sets the segment ID.
// This associates the account with a specific segment within a portfolio.
//
// Parameters:
//   - segmentID: The ID of the segment
//
// Returns:
//   - A pointer to the modified Account for method chaining
func (a *Account) WithSegmentID(segmentID string) *Account {
	a.SegmentID = &segmentID
	return a
}

// WithAlias sets the account alias.
// An alias provides a human-friendly identifier for the account.
//
// Parameters:
//   - alias: The alias to set for the account
//
// Returns:
//   - A pointer to the modified Account for method chaining
func (a *Account) WithAlias(alias string) *Account {
	a.Alias = &alias
	return a
}

// WithMetadata adds metadata to the account.
// Metadata can store additional custom information about the account.
//
// Parameters:
//   - metadata: A map of key-value pairs to store as metadata
//
// Returns:
//   - A pointer to the modified Account for method chaining
func (a *Account) WithMetadata(metadata map[string]any) *Account {
	a.Metadata = metadata
	return a
}

// GetAlias safely returns the account alias or empty string if nil.
// This method prevents nil pointer exceptions when accessing the alias.
//
// Returns:
//   - The account alias if set, or an empty string if not set
func (a *Account) GetAlias() string {
	// Alias must be set
	if a.Alias == nil {
		return ""
	}

	return *a.Alias
}

// GetIdentifier returns the best identifier for an account:
// - Returns the alias if available
// - Falls back to ID if alias is not set
//
// This helps prevent nil pointer exceptions and provides a consistent
// way to reference accounts across the application.
//
// Returns:
//   - The account alias if set, or the account ID if alias is not set
func (a *Account) GetIdentifier() string {
	if a.Alias != nil {
		return *a.Alias
	}

	return a.ID
}

// FromMmodelAccount converts an mmodel Account to an SDK Account.
// This function is used internally to convert between backend and SDK models.
//
// Parameters:
//   - account: The mmodel.Account to convert
//
// Returns:
//   - A models.Account instance with the same values
func FromMmodelAccount(account mmodel.Account) Account {
	return Account{
		ID:              account.ID,
		Name:            account.Name,
		ParentAccountID: account.ParentAccountID,
		EntityID:        account.EntityID,
		AssetCode:       account.AssetCode,
		OrganizationID:  account.OrganizationID,
		LedgerID:        account.LedgerID,
		PortfolioID:     account.PortfolioID,
		SegmentID:       account.SegmentID,
		Status:          FromMmodelStatus(account.Status),
		Alias:           account.Alias,
		Type:            account.Type,
		Metadata:        account.Metadata,
		CreatedAt:       account.CreatedAt,
		UpdatedAt:       account.UpdatedAt,
		DeletedAt:       account.DeletedAt,
	}
}

// ToMmodelAccount converts an SDK Account to an mmodel Account.
// This method is used internally to convert between SDK and backend models.
//
// Returns:
//   - An mmodel.Account instance with the same values
func (a Account) ToMmodelAccount() mmodel.Account {
	return mmodel.Account{
		ID:              a.ID,
		Name:            a.Name,
		ParentAccountID: a.ParentAccountID,
		EntityID:        a.EntityID,
		AssetCode:       a.AssetCode,
		OrganizationID:  a.OrganizationID,
		LedgerID:        a.LedgerID,
		PortfolioID:     a.PortfolioID,
		SegmentID:       a.SegmentID,
		Status:          a.Status.ToMmodelStatus(),
		Alias:           a.Alias,
		Type:            a.Type,
		Metadata:        a.Metadata,
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
		DeletedAt:       a.DeletedAt,
	}
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

// ToMmodelCreateAccountInput converts an SDK CreateAccountInput to an mmodel CreateAccountInput.
// This method is used internally to convert between SDK and backend models.
//
// Returns:
//   - An mmodel.CreateAccountInput instance with the same values
func (input CreateAccountInput) ToMmodelCreateAccountInput() mmodel.CreateAccountInput {
	return mmodel.CreateAccountInput{
		Name:            input.Name,
		ParentAccountID: input.ParentAccountID,
		EntityID:        input.EntityID,
		AssetCode:       input.AssetCode,
		PortfolioID:     input.PortfolioID,
		SegmentID:       input.SegmentID,
		Status:          input.Status.ToMmodelStatus(),
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

// ToMmodelUpdateAccountInput converts an SDK UpdateAccountInput to an mmodel UpdateAccountInput.
// This method is used internally to convert between SDK and backend models.
//
// Returns:
//   - An mmodel.UpdateAccountInput instance with the same values
func (input UpdateAccountInput) ToMmodelUpdateAccountInput() mmodel.UpdateAccountInput {
	return mmodel.UpdateAccountInput{
		Name:        input.Name,
		SegmentID:   input.SegmentID,
		PortfolioID: input.PortfolioID,
		Status:      input.Status.ToMmodelStatus(),
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

// FromMmodelAccounts converts an mmodel Accounts to an SDK Accounts.
// This function is used internally to convert between backend and SDK models.
//
// Parameters:
//   - accounts: The mmodel.Accounts to convert
//
// Returns:
//   - A models.Accounts instance with the same values
func FromMmodelAccounts(accounts mmodel.Accounts) Accounts {
	items := make([]Account, len(accounts.Items))
	for i, account := range accounts.Items {
		items[i] = FromMmodelAccount(account)
	}

	return Accounts{
		Items: items,
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

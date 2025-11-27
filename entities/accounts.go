package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

// AccountsService defines the interface for account-related operations.
// It provides methods to create, read, update, and delete accounts,
// as well as manage account balances.
type AccountsService interface {
	// ListAccounts retrieves a paginated list of accounts for a ledger with optional filters.
	// The organizationID and ledgerID parameters specify which organization and ledger to query.
	// The opts parameter can be used to specify pagination, sorting, and filtering options.
	// Returns a ListResponse containing the accounts and pagination information, or an error if the operation fails.
	ListAccounts(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Account], error)

	// GetAccount retrieves a specific account by its ID.
	// The organizationID and ledgerID parameters specify which organization and ledger the account belongs to.
	// The id parameter is the unique identifier of the account to retrieve.
	// Returns the account if found, or an error if the operation fails or the account doesn't exist.
	GetAccount(ctx context.Context, organizationID, ledgerID, id string) (*models.Account, error)

	// GetAccountByAlias retrieves a specific account by its alias.
	// The organizationID and ledgerID parameters specify which organization and ledger the account belongs to.
	// The alias parameter is the unique alias of the account to retrieve.
	// Returns the account if found, or an error if the operation fails or the account doesn't exist.
	GetAccountByAlias(ctx context.Context, organizationID, ledgerID, alias string) (*models.Account, error)

	// CreateAccount creates a new account in the specified ledger.
	//
	// This method creates a new account in the specified organization and ledger.
	// Accounts are used to track assets and balances within the Midaz system.
	// Each account has a type, name, and can be associated with a specific asset code.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization that owns the ledger. Must be a valid organization ID.
	//   - ledgerID: The ID of the ledger where the account will be created. Must be a valid ledger ID.
	//   - input: The account details, including name, type, asset code, and optional fields.
	//     Required fields in the input are:
	//     - Name: The human-readable name of the account (max 256 characters)
	//     - Type: The account type (e.g., "customer", "revenue", "liability")
	//     - AssetCode: The currency or asset code (e.g., "USD", "EUR") if applicable
	//
	// Returns:
	//   - *models.Account: The created account if successful, containing the account ID,
	//     status, and other properties.
	//   - error: An error if the operation fails. Possible errors include:
	//     - Invalid input (missing required fields)
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Resource not found (invalid organization or ledger ID)
	//     - Network or server errors
	//
	// Example - Creating a basic customer account:
	//
	//	// Create a customer account
	//	account, err := accountsService.CreateAccount(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    &models.CreateAccountInput{
	//	        Name: "John Doe",
	//	        Type: "customer",
	//	        AssetCode: "USD",
	//	        Metadata: map[string]any{
	//	            "customer_id": "cust-789",
	//	            "email": "john.doe@example.com",
	//	        },
	//	    },
	//	)
	//
	//	if err != nil {
	//	    // Handle error
	//	    return err
	//	}
	//
	//	// Use the account
	//	fmt.Printf("Account created: %s (alias: %s)\n", account.ID, account.Alias)
	//
	// Example - Creating an account with portfolio and segment:
	//
	//	// Create an account within a portfolio and segment
	//	account, err := accountsService.CreateAccount(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    &models.CreateAccountInput{
	//	        Name: "Investment Account",
	//	        Type: "investment",
	//	        AssetCode: "USD",
	//	        PortfolioID: "portfolio-789",
	//	        SegmentID: "segment-012",
	//	        Status: models.StatusActive,
	//	    },
	//	)
	//
	//	if err != nil {
	//	    // Handle error
	//	    return err
	//	}
	//
	//	// Use the account
	//	fmt.Printf("Account created: %s (status: %s)\n", account.ID, account.Status)
	CreateAccount(ctx context.Context, organizationID, ledgerID string, input *models.CreateAccountInput) (*models.Account, error)

	// UpdateAccount updates an existing account.
	// The organizationID and ledgerID parameters specify which organization and ledger the account belongs to.
	// The id parameter is the unique identifier of the account to update.
	// The input parameter contains the account details to update, such as name or status.
	// Returns the updated account, or an error if the operation fails.
	UpdateAccount(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateAccountInput) (*models.Account, error)

	// DeleteAccount deletes an account.
	// The organizationID and ledgerID parameters specify which organization and ledger the account belongs to.
	// The id parameter is the unique identifier of the account to delete.
	// Returns an error if the operation fails.
	DeleteAccount(ctx context.Context, organizationID, ledgerID, id string) error

	// GetBalance retrieves the balance for a specific account.
	// The organizationID and ledgerID parameters specify which organization and ledger the account belongs to.
	// The accountID parameter is the unique identifier of the account to get the balance for.
	// Returns the balance information, or an error if the operation fails.
	GetBalance(ctx context.Context, organizationID, ledgerID, accountID string) (*models.Balance, error)

	// GetAccountsMetricsCount retrieves the count metrics for accounts in a ledger.
	// The organizationID and ledgerID parameters specify which organization and ledger to get metrics for.
	// Returns the metrics count if successful, or an error if the operation fails.
	GetAccountsMetricsCount(ctx context.Context, organizationID, ledgerID string) (*models.MetricsCount, error)

	// GetExternalAccount retrieves an external account by asset code.
	// External accounts are special accounts that represent external systems or parties.
	// The organizationID and ledgerID parameters specify which organization and ledger to query.
	// The assetCode parameter is the asset code that identifies the external account (e.g., "USD", "BRL").
	// Returns the external account if found, or an error if the operation fails.
	GetExternalAccount(ctx context.Context, organizationID, ledgerID, assetCode string) (*models.Account, error)

	// GetExternalAccountBalance retrieves the balance for an external account by asset code.
	// The organizationID and ledgerID parameters specify which organization and ledger to query.
	// The assetCode parameter is the asset code that identifies the external account (e.g., "USD", "BRL").
	// Returns the balance information for the external account, or an error if the operation fails.
	GetExternalAccountBalance(ctx context.Context, organizationID, ledgerID, assetCode string) (*models.Balance, error)

	// GetAccountByAliasPath retrieves a specific account by its alias using the dedicated path endpoint.
	// This uses the path-based endpoint /accounts/alias/{alias} instead of query parameters.
	// The organizationID and ledgerID parameters specify which organization and ledger the account belongs to.
	// The alias parameter is the unique alias of the account to retrieve.
	// Returns the account if found, or an error if the operation fails.
	// Deprecated: Consider using GetAccountByAlias which provides the same functionality.
	GetAccountByAliasPath(ctx context.Context, organizationID, ledgerID, alias string) (*models.Account, error)
}

// accountsEntity implements the AccountsService interface.
// It handles the communication with the Midaz API for account-related operations.
type accountsEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

// NewAccountsEntity creates a new accounts entity.
//
// Parameters:
//   - httpClient: The HTTP client used for API requests. Can be configured with custom timeouts
//     and transport options. If nil, a default client will be used.
//   - authToken: The authentication token for API authorization. Must be a valid JWT token
//     issued by the Midaz authentication service.
//   - baseURLs: Map of service names to base URLs. Must include an "onboarding" key with
//     the URL of the onboarding service (e.g., "https://api.midaz.io/v1").
//
// Returns:
//   - AccountsService: An implementation of the AccountsService interface that provides
//     methods for creating, retrieving, updating, and managing accounts.
//
// Example:
//
//	// Create an accounts entity with default HTTP client
//	accountsEntity := entities.NewAccountsEntity(
//	    &http.Client{Timeout: 30 * time.Second},
//	    "your-auth-token",
//	    map[string]string{"onboarding": "https://api.midaz.io/v1"},
//	)
//
//	// Use the entity to create an account
//	account, err := accountsEntity.CreateAccount(
//	    context.Background(),
//	    "org-123",
//	    "ledger-456",
//	    &models.CreateAccountInput{
//	        Name: "Customer Account",
//	        Type: "customer",
//	        AssetCode: "USD",
//	    },
//	)
//
//	if err != nil {
//	    log.Fatalf("Failed to create account: %v", err)
//	}
//
//	fmt.Printf("Account created: %s\n", account.ID)
func NewAccountsEntity(client *http.Client, authToken string, baseURLs map[string]string) AccountsService {
	// Create a new HTTP client with the shared implementation
	httpClient := NewHTTPClient(client, authToken, nil)

	// Check if we're using the debug flag from the environment
	if debugEnv := os.Getenv("MIDAZ_DEBUG"); debugEnv == "true" {
		httpClient.debug = true
	}

	return &accountsEntity{
		httpClient: httpClient,
		baseURLs:   baseURLs,
	}
}

// ListAccounts lists accounts for a ledger with optional filters.
func (e *accountsEntity) ListAccounts(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Account], error) {
	const operation = "ListAccounts"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	url := e.buildURL(organizationID, ledgerID, "")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	// Add query parameters if provided
	if opts != nil {
		q := req.URL.Query()

		for key, value := range opts.ToQueryParams() {
			q.Add(key, value)
		}

		req.URL.RawQuery = q.Encode()
	}

	var response models.ListResponse[models.Account]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetAccount gets an account by ID.
func (e *accountsEntity) GetAccount(ctx context.Context, organizationID, ledgerID, id string) (*models.Account, error) {
	const operation = "GetAccount"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if id == "" {
		return nil, errors.NewMissingParameterError(operation, "id")
	}

	url := e.buildURL(organizationID, ledgerID, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var account models.Account
	if err := e.httpClient.sendRequest(req, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// GetAccountByAlias gets an account by alias.
func (e *accountsEntity) GetAccountByAlias(ctx context.Context, organizationID, ledgerID, alias string) (*models.Account, error) {
	const operation = "GetAccountByAlias"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if alias == "" {
		return nil, errors.NewMissingParameterError(operation, "alias")
	}

	url := fmt.Sprintf("%s?alias=%s", e.buildURL(organizationID, ledgerID, ""), alias)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var accounts models.ListResponse[models.Account]
	if err := e.httpClient.sendRequest(req, &accounts); err != nil {
		return nil, err
	}

	if len(accounts.Items) == 0 {
		return nil, errors.NewNotFoundError(operation, "account", alias, nil)
	}

	return &accounts.Items[0], nil
}

// CreateAccount creates a new account in the specified ledger.
func (e *accountsEntity) CreateAccount(ctx context.Context, organizationID, ledgerID string, input *models.CreateAccountInput) (*models.Account, error) {
	const operation = "CreateAccount"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	url := e.buildURL(organizationID, ledgerID, "")

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var account models.Account
	if err := e.httpClient.sendRequest(req, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// UpdateAccount updates an existing account.
func (e *accountsEntity) UpdateAccount(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateAccountInput) (*models.Account, error) {
	const operation = "UpdateAccount"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if id == "" {
		return nil, errors.NewMissingParameterError(operation, "id")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	url := e.buildURL(organizationID, ledgerID, id)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var account models.Account
	if err := e.httpClient.sendRequest(req, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// DeleteAccount deletes an account.
func (e *accountsEntity) DeleteAccount(ctx context.Context, organizationID, ledgerID, id string) error {
	const operation = "DeleteAccount"

	if organizationID == "" {
		return errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return errors.NewMissingParameterError(operation, "ledgerID")
	}

	if id == "" {
		return errors.NewMissingParameterError(operation, "id")
	}

	url := e.buildURL(organizationID, ledgerID, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if err := e.httpClient.sendRequest(req, nil); err != nil {
		return err
	}

	return nil
}

// GetBalance gets an account's balance.
func (e *accountsEntity) GetBalance(ctx context.Context, organizationID, ledgerID, accountID string) (*models.Balance, error) {
	const operation = "GetBalance"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if accountID == "" {
		return nil, errors.NewMissingParameterError(operation, "accountID")
	}

	// First get the account details to get the alias
	account, err := e.GetAccount(ctx, organizationID, ledgerID, accountID)
	if err != nil {
		return nil, err
	}

	if account.Alias == nil || *account.Alias == "" {
		return nil, errors.NewValidationError(operation, "account has no alias", nil)
	}

	// Build URL with balance endpoint using alias instead of ID
	base := e.baseURLs["transaction"]
	urlPath := path.Join("v1", "organizations", organizationID, "ledgers", ledgerID, "balances")

	url := fmt.Sprintf("%s/%s?account=%s", base, urlPath, *account.Alias)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var balance models.Balance
	if err := e.httpClient.sendRequest(req, &balance); err != nil {
		return nil, err
	}

	return &balance, nil
}

// GetAccountsMetricsCount gets the count metrics for accounts in a ledger.
func (e *accountsEntity) GetAccountsMetricsCount(ctx context.Context, organizationID, ledgerID string) (*models.MetricsCount, error) {
	const operation = "GetAccountsMetricsCount"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	url := e.buildMetricsURL(organizationID, ledgerID)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var metrics models.MetricsCount
	if err := e.httpClient.sendRequest(req, &metrics); err != nil {
		return nil, err
	}

	return &metrics, nil
}

// buildURL builds the URL for accounts API calls.
func (e *accountsEntity) buildURL(organizationID, ledgerID, accountID string) string {
	baseURL := e.baseURLs["onboarding"]

	if accountID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts", baseURL, organizationID, ledgerID)
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/%s", baseURL, organizationID, ledgerID, accountID)
}

// buildMetricsURL builds the URL for accounts metrics API calls.
func (e *accountsEntity) buildMetricsURL(organizationID, ledgerID string) string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/metrics/count", baseURL, organizationID, ledgerID)
}

// GetExternalAccount gets an external account by asset code.
func (e *accountsEntity) GetExternalAccount(ctx context.Context, organizationID, ledgerID, assetCode string) (*models.Account, error) {
	const operation = "GetExternalAccount"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if assetCode == "" {
		return nil, errors.NewMissingParameterError(operation, "assetCode")
	}

	url := e.buildExternalAccountURL(organizationID, ledgerID, assetCode)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var account models.Account
	if err := e.httpClient.sendRequest(req, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// GetExternalAccountBalance gets the balance for an external account by asset code.
func (e *accountsEntity) GetExternalAccountBalance(ctx context.Context, organizationID, ledgerID, assetCode string) (*models.Balance, error) {
	const operation = "GetExternalAccountBalance"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if assetCode == "" {
		return nil, errors.NewMissingParameterError(operation, "assetCode")
	}

	url := e.buildExternalAccountBalanceURL(organizationID, ledgerID, assetCode)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var balance models.Balance
	if err := e.httpClient.sendRequest(req, &balance); err != nil {
		return nil, err
	}

	return &balance, nil
}

// buildExternalAccountURL builds the URL for external account API calls.
func (e *accountsEntity) buildExternalAccountURL(organizationID, ledgerID, assetCode string) string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/external/%s", baseURL, organizationID, ledgerID, assetCode)
}

// buildExternalAccountBalanceURL builds the URL for external account balance API calls.
func (e *accountsEntity) buildExternalAccountBalanceURL(organizationID, ledgerID, assetCode string) string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/external/%s/balances", baseURL, organizationID, ledgerID, assetCode)
}

// GetAccountByAliasPath retrieves a specific account by its alias using the dedicated path endpoint.
func (e *accountsEntity) GetAccountByAliasPath(ctx context.Context, organizationID, ledgerID, alias string) (*models.Account, error) {
	const operation = "GetAccountByAliasPath"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if alias == "" {
		return nil, errors.NewMissingParameterError(operation, "alias")
	}

	url := e.buildAliasURL(organizationID, ledgerID, alias)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var account models.Account
	if err := e.httpClient.sendRequest(req, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// buildAliasURL builds the URL for account alias path endpoint.
func (e *accountsEntity) buildAliasURL(organizationID, ledgerID, alias string) string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/alias/%s", baseURL, organizationID, ledgerID, alias)
}

package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

// BalancesService defines the interface for balance-related operations.
// It provides methods to list, retrieve, update, and delete balances
// for both ledgers and specific accounts.
type BalancesService interface {
	// ListBalances retrieves a paginated list of all balances for a specified ledger.
	//
	// This method returns all balances within a ledger, with optional filtering and
	// pagination controls. Balances represent the current state of funds for each
	// account-asset combination in the ledger.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - orgID: The ID of the organization that owns the ledger. Must be a valid organization ID.
	//   - ledgerID: The ID of the ledger to retrieve balances from. Must be a valid ledger ID.
	//   - opts: Optional pagination and filtering options:
	//     - Page: The page number to retrieve (1-based indexing)
	//     - Limit: The maximum number of items per page
	//     - Filter: Criteria to filter balances by (e.g., by account ID or asset code)
	//     - Sort: Sorting options for the results
	//     If nil, default pagination settings will be used.
	//
	// Returns:
	//   - *models.ListResponse[models.Balance]: A paginated list of balances, including:
	//     - Items: The array of balance objects for the current page
	//     - Page: The current page number
	//     - Limit: The maximum number of items per page
	//     - Total: The total number of balances matching the filter criteria
	//   - error: An error if the operation fails. Possible errors include:
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Resource not found (invalid organization or ledger ID)
	//     - Network or server errors
	//
	// Example - Basic usage:
	//
	//	// List balances with default pagination
	//	balances, err := balancesService.ListBalances(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    nil, // Use default pagination
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to list balances: %v", err)
	//	}

	//
	//	// Process the balances
	//	fmt.Printf("Retrieved %d balances (page %d of %d)\n",
	//	    len(balances.Items), balances.Page, balances.TotalPages)
	//
	//	for _, balance := range balances.Items {
	//	    fmt.Printf("Balance: %s, Asset: %s, Available: %d/%d\n",
	//	        balance.ID, balance.AssetCode, balance.Available, balance.Scale)
	//	}

	//
	// Example - With pagination and filtering:
	//
	//	// Create pagination options with filtering
	//	opts := &models.ListOptions{
	//	    Page: 1,
	//	    Limit: 10,
	//	    Filter: map[string]any{
	//	        "assetCode": "USD", // Only show USD balances
	//	    },
	//	    Sort: []string{"available:desc"}, // Sort by available amount (descending)
	//	}

	//
	//	// List balances with pagination and filtering
	//	balances, err := balancesService.ListBalances(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    opts,
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to list balances: %v", err)
	//	}

	//
	//	// Process the balances
	//	fmt.Printf("Retrieved %d USD balances\n", len(balances.Items))
	ListBalances(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error)

	// ListAccountBalances retrieves a paginated list of all balances for a specific account.
	//
	// This method returns all balances for a single account within a ledger, with optional
	// filtering and pagination controls. Each balance represents a different asset held
	// by the account.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - orgID: The ID of the organization that owns the ledger. Must be a valid organization ID.
	//   - ledgerID: The ID of the ledger containing the account. Must be a valid ledger ID.
	//   - accountID: The ID of the account to retrieve balances for. Must be a valid account ID.
	//   - opts: Optional pagination and filtering options:
	//     - Page: The page number to retrieve (1-based indexing)
	//     - Limit: The maximum number of items per page
	//     - Filter: Criteria to filter balances by (e.g., by asset code)
	//     - Sort: Sorting options for the results
	//     If nil, default pagination settings will be used.
	//
	// Returns:
	//   - *models.ListResponse[models.Balance]: A paginated list of balances for the account, including:
	//     - Items: The array of balance objects for the current page
	//     - Page: The current page number
	//     - Limit: The maximum number of items per page
	//     - Total: The total number of balances matching the filter criteria
	//   - error: An error if the operation fails. Possible errors include:
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Resource not found (invalid organization, ledger, or account ID)
	//     - Network or server errors
	//
	// Example - Basic usage:
	//
	//	// List all balances for an account with default pagination
	//	balances, err := balancesService.ListAccountBalances(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    "account-789",
	//	    nil, // Use default pagination
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to list account balances: %v", err)
	//	}

	//
	//	// Process the balances
	//	fmt.Printf("Account has %d different asset balances\n", len(balances.Items))
	//
	//	for _, balance := range balances.Items {
	//	    // Calculate the decimal value of the balance
	//	    decimalValue := float64(balance.Available) / math.Pow10(int(balance.Scale))
	//	    fmt.Printf("Asset: %s, Available: %.2f\n", balance.AssetCode, decimalValue)
	//	}

	//
	// Example - With filtering by asset code:
	//
	//	// Create pagination options with filtering for specific assets
	//	opts := &models.ListOptions{
	//	    Filter: map[string]any{
	//	        "assetCode": []string{"USD", "EUR", "GBP"}, // Only show these currencies
	//	    },
	//	    Sort: []string{"assetCode:asc"}, // Sort alphabetically by asset code
	//	}

	//
	//	// List filtered balances for an account
	//	balances, err := balancesService.ListAccountBalances(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    "account-789",
	//	    opts,
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to list account balances: %v", err)
	//	}

	//
	//	// Process the balances
	//	fmt.Println("Currency balances for account:")
	//	for _, balance := range balances.Items {
	//	    decimalValue := float64(balance.Available) / math.Pow10(int(balance.Scale))
	//	    fmt.Printf("%s: %.2f\n", balance.AssetCode, decimalValue)
	//	}

	ListAccountBalances(ctx context.Context, orgID, ledgerID, accountID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error)

	// GetBalance retrieves a specific balance by its ID.
	// The orgID, ledgerID, and balanceID parameters specify which organization, ledger, and balance to retrieve.
	// Returns the balance if found, or an error if the operation fails or the balance doesn't exist.
	GetBalance(ctx context.Context, orgID, ledgerID, balanceID string) (*models.Balance, error)

	// UpdateBalance updates an existing balance.
	// The orgID, ledgerID, and balanceID parameters specify which organization, ledger, and balance to update.
	// The input parameter contains the balance details to update, such as amount or metadata.
	// Returns the updated balance, or an error if the operation fails.
	UpdateBalance(ctx context.Context, orgID, ledgerID, balanceID string, input *models.UpdateBalanceInput) (*models.Balance, error)

	// DeleteBalance deletes a balance.
	// The orgID, ledgerID, and balanceID parameters specify which organization, ledger, and balance to delete.
	// Returns an error if the operation fails.
	DeleteBalance(ctx context.Context, orgID, ledgerID, balanceID string) error

	// CreateBalance creates an additional balance for an account.
	// This allows an account to have multiple balance entries (e.g., for different purposes).
	// The orgID, ledgerID, and accountID parameters specify which account to add the balance to.
	// Returns the created balance, or an error if the operation fails.
	CreateBalance(ctx context.Context, orgID, ledgerID, accountID string, input *models.CreateBalanceInput) (*models.Balance, error)

	// ListBalancesByAccountAlias retrieves balances for an account identified by its alias.
	// The alias is a human-readable identifier for the account.
	// Returns a paginated list of balances, or an error if the operation fails.
	ListBalancesByAccountAlias(ctx context.Context, orgID, ledgerID, alias string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error)

	// ListBalancesByExternalCode retrieves balances for an account identified by its external code.
	// The external code links the account to external systems.
	// Returns a paginated list of balances, or an error if the operation fails.
	ListBalancesByExternalCode(ctx context.Context, orgID, ledgerID, code string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error)
}

// balancesEntity implements the BalancesService interface.
// It handles the communication with the Midaz API for balance-related operations.
type balancesEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

// NewBalancesEntity creates a new balances entity.
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
//   - BalancesService: An implementation of the BalancesService interface that provides
//     methods for retrieving and managing balances.
//
// Example:
//
//	// Create a balances entity with default HTTP client
//	balancesEntity := entities.NewBalancesEntity(
//	    &http.Client{Timeout: 30 * time.Second},
//	    "your-auth-token",
//	    map[string]string{"onboarding": "https://api.midaz.io/v1"},
//	)
//
//	// Use the entity to get a balance
//	balance, err := balancesEntity.GetBalance(
//	    context.Background(),
//	    "org-123",
//	    "ledger-456",
//	    "account-789",
//	)
//
//	if err != nil {
//	    log.Fatalf("Failed to get balance: %v", err)
//	}
//
//	fmt.Printf("Balance: %f %s\n", balance.Amount, balance.AssetCode)
func NewBalancesEntity(client *http.Client, authToken string, baseURLs map[string]string) BalancesService {
	// Create a new HTTP client with the shared implementation
	httpClient := NewHTTPClient(client, authToken, nil)

	// Check if we're using the debug flag from the environment
	if debugEnv := os.Getenv(EnvMidazDebug); debugEnv == BoolTrue {
		httpClient.debug = true
	}

	return &balancesEntity{
		httpClient: httpClient,
		baseURLs:   baseURLs,
	}
}

// ListBalances lists all balances for a ledger.
// The orgID and ledgerID parameters specify which organization and ledger to query.
// The opts parameter can be used to specify pagination, sorting, and filtering options.
// Returns a ListResponse containing the balances and pagination information, or an error if the operation fails.
func (e *balancesEntity) ListBalances(
	ctx context.Context,
	orgID,
	ledgerID string,
	opts *models.ListOptions,
) (*models.ListResponse[models.Balance], error) {
	const operation = "ListBalances"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	endpoint := e.buildURL(orgID, ledgerID, "")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

	var response models.ListResponse[models.Balance]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &response, nil
}

// ListAccountBalances lists all balances for a specific account.
// The orgID, ledgerID, and accountID parameters specify which organization, ledger, and account to query.
// The opts parameter can be used to specify pagination, sorting, and filtering options.
// Returns a ListResponse containing the account balances and pagination information, or an error if the operation fails.
func (e *balancesEntity) ListAccountBalances(
	ctx context.Context,
	orgID,
	ledgerID,
	accountID string,
	opts *models.ListOptions,
) (*models.ListResponse[models.Balance], error) {
	const operation = "ListAccountBalances"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if accountID == "" {
		return nil, errors.NewMissingParameterError(operation, "accountID")
	}

	endpoint := e.buildAccountURL(orgID, ledgerID, accountID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

	var response models.ListResponse[models.Balance]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &response, nil
}

// GetBalance retrieves a balance by its ID.
// The orgID, ledgerID, and balanceID parameters specify which organization, ledger, and balance to retrieve.
// Returns the balance if found, or an error if the operation fails or the balance doesn't exist.
func (e *balancesEntity) GetBalance(
	ctx context.Context,
	orgID,
	ledgerID,
	balanceID string,
) (*models.Balance, error) {
	const operation = "GetBalance"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if balanceID == "" {
		return nil, errors.NewMissingParameterError(operation, "balanceID")
	}

	endpoint := e.buildURL(orgID, ledgerID, balanceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var balance models.Balance
	if err := e.httpClient.sendRequest(req, &balance); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &balance, nil
}

// UpdateBalance updates an existing balance.
// The orgID, ledgerID, and balanceID parameters specify which organization, ledger, and balance to update.
// The input parameter contains the balance details to update, such as amount or metadata.
// Returns the updated balance, or an error if the operation fails.
func (e *balancesEntity) UpdateBalance(
	ctx context.Context,
	orgID,
	ledgerID,
	balanceID string,
	input *models.UpdateBalanceInput,
) (*models.Balance, error) {
	const operation = "UpdateBalance"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if balanceID == "" {
		return nil, errors.NewMissingParameterError(operation, "balanceID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	// Validate the input using the model's validation method
	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "invalid balance update input", err)
	}

	endpoint := e.buildURL(orgID, ledgerID, balanceID)

	payload, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var balance models.Balance
	if err := e.httpClient.sendRequest(req, &balance); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &balance, nil
}

// DeleteBalance deletes a balance.
// The orgID, ledgerID, and balanceID parameters specify which organization, ledger, and balance to delete.
// Returns an error if the operation fails.
func (e *balancesEntity) DeleteBalance(
	ctx context.Context,
	orgID,
	ledgerID,
	balanceID string,
) error {
	const operation = "DeleteBalance"

	if orgID == "" {
		return errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return errors.NewMissingParameterError(operation, "ledgerID")
	}

	if balanceID == "" {
		return errors.NewMissingParameterError(operation, "balanceID")
	}

	endpoint := e.buildURL(orgID, ledgerID, balanceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	// HTTPClient.DoRequest already returns proper error types
	return e.httpClient.sendRequest(req, nil)
}

// buildURL builds the URL for balances API calls.
// The orgID and ledgerID parameters specify which organization and ledger to query.
// The balanceID parameter is the unique identifier of the balance to retrieve, or an empty string for a list of balances.
// Returns the built URL.
// Note: Assumes base URL already includes version path (e.g., "https://api.example.com/v1").
func (e *balancesEntity) buildURL(organizationID, ledgerID, balanceID string) string {
	baseURL := e.baseURLs["transaction"]

	if balanceID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/balances", baseURL, organizationID, ledgerID)
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/balances/%s", baseURL, organizationID, ledgerID, balanceID)
}

// buildAccountURL builds the URL for account balances API calls.
// The orgID, ledgerID, and accountID parameters specify which organization, ledger, and account to query.
// Returns the built URL for retrieving balances for a specific account.
// Note: Assumes base URL already includes version path (e.g., "https://api.example.com/v1").
func (e *balancesEntity) buildAccountURL(orgID, ledgerID, accountID string) string {
	baseURL := e.baseURLs["transaction"]

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/%s/balances", baseURL, orgID, ledgerID, accountID)
}

// CreateBalance creates an additional balance for an account.
func (e *balancesEntity) CreateBalance(ctx context.Context, orgID, ledgerID, accountID string, input *models.CreateBalanceInput) (*models.Balance, error) {
	const operation = "CreateBalance"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if accountID == "" {
		return nil, errors.NewMissingParameterError(operation, "accountID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "invalid input", err)
	}

	endpoint := e.buildAccountURL(orgID, ledgerID, accountID)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req.Header.Set("Content-Type", "application/json")

	var balance models.Balance
	if err := e.httpClient.sendRequest(req, &balance); err != nil {
		return nil, err
	}

	return &balance, nil
}

// ListBalancesByAccountAlias retrieves balances for an account identified by its alias.
func (e *balancesEntity) ListBalancesByAccountAlias(ctx context.Context, orgID, ledgerID, alias string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
	const operation = "ListBalancesByAccountAlias"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if alias == "" {
		return nil, errors.NewMissingParameterError(operation, "alias")
	}

	endpoint := e.buildAccountAliasURL(orgID, ledgerID, alias)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

	var response models.ListResponse[models.Balance]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ListBalancesByExternalCode retrieves balances for an account identified by its external code.
func (e *balancesEntity) ListBalancesByExternalCode(ctx context.Context, orgID, ledgerID, code string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
	const operation = "ListBalancesByExternalCode"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if code == "" {
		return nil, errors.NewMissingParameterError(operation, "code")
	}

	endpoint := e.buildExternalCodeURL(orgID, ledgerID, code)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

	var response models.ListResponse[models.Balance]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// buildAccountAliasURL builds the URL for balance lookups by account alias.
func (e *balancesEntity) buildAccountAliasURL(orgID, ledgerID, alias string) string {
	baseURL := e.baseURLs["transaction"]

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/alias/%s/balances", baseURL, orgID, ledgerID, url.PathEscape(alias))
}

// buildExternalCodeURL builds the URL for balance lookups by external code.
func (e *balancesEntity) buildExternalCodeURL(orgID, ledgerID, code string) string {
	baseURL := e.baseURLs["transaction"]

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/external/%s/balances", baseURL, orgID, ledgerID, url.PathEscape(code))
}

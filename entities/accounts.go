package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
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
	//     - Type: The account type (e.g., "deposit", "savings", "loans")
	//     - AssetCode: The currency or asset code (e.g., "USD", "EUR")
	//     Optional fields include:
	//     - Name: The human-readable name of the account (max 256 characters)
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
	//	        Type: "deposit",
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
	//	        Status: models.NewStatus(models.StatusActive),
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

	// GetBalance retrieves a single balance for a specific account.
	// If the account has multiple balances, callers should use BalancesService.ListAccountBalances
	// to retrieve the full set explicitly.
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

	// GetExternalAccountBalance retrieves a single balance for an external account by asset code.
	// If the external account has multiple balances, callers should use
	// BalancesService.ListBalancesByExternalCode instead.
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

func (e *accountsEntity) setDefaultTenantID(tenantID string) {
	e.httpClient.setTenantIDLocked(tenantID)
}

// newAccountsEntity wires the AccountsService backed by the shared HTTP transport.
// Internal: invoked by Entity.initServices; callers should reach the service via Client.Accounts.
func newAccountsEntity(client *http.Client, authToken string, baseURLs map[string]string) AccountsService {
	// Create a new HTTP client with the shared implementation
	httpClient := NewHTTPClient(client, authToken, nil)

	return &accountsEntity{
		httpClient: httpClient,
		baseURLs:   prepareServiceBaseURLs(baseURLs),
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

	endpoint := e.buildURL(organizationID, ledgerID, "")

	req, err := newRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

	endpoint := e.buildURL(organizationID, ledgerID, id)

	req, err := newRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

	endpoint := e.buildAliasURL(organizationID, ledgerID, alias)

	req, err := newRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var account models.Account
	if err := e.httpClient.sendRequest(req, &account); err != nil {
		return nil, err
	}

	return &account, nil
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

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "account validation failed", err)
	}

	endpoint := e.buildURL(organizationID, ledgerID, "")

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var account models.Account
	if err := e.httpClient.sendRequest(req, &account); err != nil {
		e.httpClient.emitBusinessError(ctx, businessEventAccountCreated, map[string]any{"operation": operation, "organizationId": organizationID, "ledgerId": ledgerID}, err)

		return nil, err
	}

	e.httpClient.emitBusinessEvent(ctx, businessEventAccountCreated, map[string]any{"operation": operation, "organizationId": organizationID, "ledgerId": ledgerID, "accountId": account.ID, "status": account.Status.Code})

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

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "account validation failed", err)
	}

	endpoint := e.buildURL(organizationID, ledgerID, id)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var account models.Account
	if err := e.httpClient.sendRequest(req, &account); err != nil {
		e.httpClient.emitBusinessError(ctx, businessEventAccountUpdated, map[string]any{"operation": operation, "organizationId": organizationID, "ledgerId": ledgerID, "accountId": id}, err)

		return nil, err
	}

	e.httpClient.emitBusinessEvent(ctx, businessEventAccountUpdated, map[string]any{"operation": operation, "organizationId": organizationID, "ledgerId": ledgerID, "accountId": account.ID, "status": account.Status.Code})

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

	endpoint := e.buildURL(organizationID, ledgerID, id)

	req, err := newRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if err := e.httpClient.sendRequest(req, nil); err != nil {
		e.httpClient.emitBusinessError(ctx, businessEventAccountDeleted, map[string]any{"operation": operation, "organizationId": organizationID, "ledgerId": ledgerID, "accountId": id}, err)

		return err
	}

	e.httpClient.emitBusinessEvent(ctx, businessEventAccountDeleted, map[string]any{"operation": operation, "organizationId": organizationID, "ledgerId": ledgerID, "accountId": id})

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

	accountsURL := e.buildAccountBalanceURL(organizationID, ledgerID, accountID)

	req, err := newRequestWithContext(ctx, http.MethodGet, accountsURL, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var response models.ListResponse[models.Balance]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return selectSingleBalance(operation, accountID, response.Items)
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

	endpoint := e.buildMetricsURL(organizationID, ledgerID)

	count, err := e.httpClient.doCountRequest(ctx, countRequestMethod(), endpoint, countRequestHeaders())
	if err != nil {
		return nil, err
	}

	return &models.MetricsCount{AccountsCount: count}, nil
}

// buildURL builds the URL for accounts API calls.
func (e *accountsEntity) buildURL(organizationID, ledgerID, accountID string) string {
	baseURL := e.baseURLs["onboarding"]

	if accountID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts", baseURL, pathSegment(organizationID), pathSegment(ledgerID))
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/%s", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(accountID))
}

// buildMetricsURL builds the URL for accounts metrics API calls.
func (e *accountsEntity) buildMetricsURL(organizationID, ledgerID string) string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/metrics/count", baseURL, pathSegment(organizationID), pathSegment(ledgerID))
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

	endpoint := e.buildExternalAccountURL(organizationID, ledgerID, assetCode)

	req, err := newRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

	endpoint := e.buildExternalAccountBalanceURL(organizationID, ledgerID, assetCode)

	req, err := newRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var response models.ListResponse[models.Balance]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return selectSingleBalance(operation, assetCode, response.Items)
}

// buildExternalAccountURL builds the URL for external account API calls.
func (e *accountsEntity) buildExternalAccountURL(organizationID, ledgerID, assetCode string) string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/external/%s", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(assetCode))
}

// balanceQueryLimit is the page size used for "fetch the balance for this
// single (account, asset)" lookups. Setting limit=2 (NOT 1) is intentional:
// the upstream balances endpoint can return more than one record when an
// account has been split across balance scopes, and we need to detect that
// multi-balance shape so we can surface a clear error instead of silently
// returning the first one. Reducing to 1 would mask the multi-balance case.
const balanceQueryLimit = 2

// buildExternalAccountBalanceURL builds the URL for external account balance API calls.
func (e *accountsEntity) buildExternalAccountBalanceURL(organizationID, ledgerID, assetCode string) string {
	baseURL := e.baseURLs["transaction"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/external/%s/balances?limit=%d", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(assetCode), balanceQueryLimit)
}

func (e *accountsEntity) buildAccountBalanceURL(organizationID, ledgerID, accountID string) string {
	baseURL := e.baseURLs["transaction"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/%s/balances?limit=%d", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(accountID), balanceQueryLimit)
}

// GetAccountByAliasPath retrieves a specific account by its alias using the dedicated path endpoint.
func (e *accountsEntity) GetAccountByAliasPath(ctx context.Context, organizationID, ledgerID, alias string) (*models.Account, error) {
	return e.GetAccountByAlias(ctx, organizationID, ledgerID, alias)
}

// buildAliasURL builds the URL for account alias path endpoint.
func (e *accountsEntity) buildAliasURL(organizationID, ledgerID, alias string) string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/alias/%s", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(alias))
}

func selectSingleBalance(operation, identifier string, balances []models.Balance) (*models.Balance, error) {
	if len(balances) == 0 {
		return nil, errors.NewNotFoundError(operation, "balance", identifier, nil)
	}

	if len(balances) > 1 {
		return nil, errors.NewValidationError(operation, "multiple balances returned; use the balances service list methods for full results", nil)
	}

	return &balances[0], nil
}

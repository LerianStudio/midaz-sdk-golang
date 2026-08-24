package entities

//go:generate mockgen -source=balances.go -destination=mocks/mock_balances.go -package=mocks BalancesService

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// BalancesService defines the interface for balance-related operations.
// It provides methods to list, retrieve, update, and delete balances
// for both ledgers and specific accounts.
//
// See also:
//   - [github.com/LerianStudio/midaz-sdk-golang/v5.Client.Balances] — the production wiring.
//   - [github.com/LerianStudio/midaz-sdk-golang/v5/entities/mocks.NewMockBalancesService] — generated mock for unit tests.
//   - examples/concurrency/balance-fetch — concurrent balance fetching pattern.
type BalancesService interface {
	// ListBalances retrieves one page of balances for a specified ledger.
	// Balances represent the current state of funds for each account-asset combination.
	// This endpoint uses page-based pagination; advance pages by incrementing opts.Page.
	//
	// Parameters:
	//   - ctx: Context for the request.
	//   - organizationID: The organization ID.
	//   - ledgerID: The ledger ID.
	//   - opts: Typed page list options. Limit caps page size; Filters narrow results
	//     (AccountID, AssetCode, Status).
	//
	// Returns:
	//   - *models.ListResponse[models.Balance]: One page of balances.
	//   - error: A typed *errors.Error. Validation errors return category validation
	//     before any HTTP request is sent.
	//
	// Example - One page with filtering:
	//
	//	opts := models.BalancesListOpts{
	//	    PageListOpts: models.PageListOpts{Limit: 10, Page: 1},
	//	    Filters:      models.BalancesFilters{AssetCode: "USD"},
	//	}
	//	page, err := c.Balances.ListBalances(ctx, "org-123", "ledger-456", opts)
	//	if err != nil { return err }
	//	for _, b := range page.Items {
	//	    fmt.Printf("balance %s: %s available=%s\n", b.ID, b.AssetCode, b.Available.String())
	//	}
	//
	// For multi-page traversal, prefer ListBalancesAll (auto page advance, range-loop friendly).
	ListBalances(ctx context.Context, organizationID, ledgerID string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error)

	// ListBalancesAll returns an iter.Seq2 that yields each Balance across every page until
	// the ledger is exhausted or the context is cancelled.
	ListBalancesAll(ctx context.Context, organizationID, ledgerID string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error]

	// ListBalancesPages returns an iter.Seq2 that yields each *ListResponse page. Use this
	// when you need page-level metadata (Pagination, ItemCount) rather than flattened items.
	ListBalancesPages(ctx context.Context, organizationID, ledgerID string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error]

	// ListAccountBalances retrieves one page of balances for a specific account.
	// Each balance represents a different asset held by the account.
	// This endpoint uses page-based pagination; advance pages by incrementing opts.Page.
	//
	// Parameters:
	//   - ctx: Context for the request.
	//   - organizationID: The organization ID.
	//   - ledgerID: The ledger ID.
	//   - accountID: The account ID.
	//   - opts: Typed page list options. Limit caps page size; Filters narrow results.
	//
	// Returns:
	//   - *models.ListResponse[models.Balance]: One page of balances for the account.
	//   - error: A typed *errors.Error.
	//
	// Example:
	//
	//	opts := models.BalancesListOpts{
	//	    PageListOpts: models.PageListOpts{Limit: 10},
	//	    Filters:      models.BalancesFilters{AssetCode: "USD"},
	//	}
	//	page, err := c.Balances.ListAccountBalances(ctx, "org", "ledger", "account", opts)
	//
	// For multi-page traversal, prefer ListAccountBalancesAll.
	ListAccountBalances(ctx context.Context, organizationID, ledgerID, accountID string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error)

	// ListAccountBalancesAll returns an iter.Seq2 that yields each Balance for the account
	// across every page until exhausted or the context is cancelled.
	ListAccountBalancesAll(ctx context.Context, organizationID, ledgerID, accountID string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error]

	// ListAccountBalancesPages returns an iter.Seq2 that yields each *ListResponse page.
	ListAccountBalancesPages(ctx context.Context, organizationID, ledgerID, accountID string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error]

	// GetBalance retrieves a specific balance by its ID.
	// The organizationID, ledgerID, and balanceID parameters specify which organization, ledger, and balance to retrieve.
	// Returns the balance if found, or an error if the operation fails or the balance doesn't exist.
	GetBalance(ctx context.Context, organizationID, ledgerID, balanceID string) (*models.Balance, error)

	// GetBalanceHistory retrieves the historical state of a balance at a specific point in time.
	// The date parameter must be RFC3339/RFC3339Nano with an explicit timezone
	// offset, for example "2026-01-02T03:04:05Z" or "2026-01-02T03:04:05-03:00".
	GetBalanceHistory(ctx context.Context, organizationID, ledgerID, balanceID, date string) (*models.BalanceHistory, error)

	// UpdateBalance updates an existing balance.
	// The organizationID, ledgerID, and balanceID parameters specify which organization, ledger, and balance to update.
	// The input parameter contains the balance details to update. Metadata updates are deprecated
	// and rejected by validation because the current Midaz UpdateBalance contract does not accept metadata.
	// Returns the updated balance, or an error if the operation fails.
	UpdateBalance(ctx context.Context, organizationID, ledgerID, balanceID string, input *models.UpdateBalanceInput) (*models.Balance, error)

	// DeleteBalance deletes a balance.
	// The organizationID, ledgerID, and balanceID parameters specify which organization, ledger, and balance to delete.
	// Returns an error if the operation fails.
	DeleteBalance(ctx context.Context, organizationID, ledgerID, balanceID string) error

	// CreateBalance creates an additional balance for an account.
	// This allows an account to have multiple balance entries (e.g., for different purposes).
	// The organizationID, ledgerID, and accountID parameters specify which account to add the balance to.
	// Returns the created balance, or an error if the operation fails.
	CreateBalance(ctx context.Context, organizationID, ledgerID, accountID string, input *models.CreateBalanceInput) (*models.Balance, error)

	// ListBalancesByAccountAlias retrieves balances for an account identified by its alias.
	// The alias is a human-readable identifier for the account.
	// Returns a paginated list of balances, or an error if the operation fails.
	ListBalancesByAccountAlias(ctx context.Context, organizationID, ledgerID, alias string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error)
	// ListBalancesByAccountAliasAll yields every balance for an account alias across all pages, transparently advancing pagination.
	ListBalancesByAccountAliasAll(ctx context.Context, organizationID, ledgerID, alias string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error]
	// ListBalancesByAccountAliasPages yields one *ListResponse[Balance] per page for an account alias.
	ListBalancesByAccountAliasPages(ctx context.Context, organizationID, ledgerID, alias string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error]

	// ListBalancesByExternalCode retrieves balances for an account identified by its external code.
	// The external code links the account to external systems.
	// Returns a paginated list of balances, or an error if the operation fails.
	ListBalancesByExternalCode(ctx context.Context, organizationID, ledgerID, code string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error)
	// ListBalancesByExternalCodeAll yields every balance for an external code across all pages, transparently advancing pagination.
	ListBalancesByExternalCodeAll(ctx context.Context, organizationID, ledgerID, code string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error]
	// ListBalancesByExternalCodePages yields one *ListResponse[Balance] per page for an external code.
	ListBalancesByExternalCodePages(ctx context.Context, organizationID, ledgerID, code string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error]

	// GetAccountBalancesHistory retrieves the historical state of all balances for an account at a specific point in time.
	// The date parameter must be RFC3339/RFC3339Nano with an explicit timezone offset.
	GetAccountBalancesHistory(ctx context.Context, organizationID, ledgerID, accountID, date string) ([]models.BalanceHistory, error)
}

// balancesEntity implements the BalancesService interface.
// It handles the communication with the Midaz API for balance-related operations.
type balancesEntity struct {
	serviceEntity
}

// newBalancesEntity wires the BalancesService backed by the shared HTTP transport.
// Internal: invoked by Entity.initServices; callers should reach the service via Client.Balances.
func newBalancesEntity(client *http.Client, authToken string, baseURLs map[string]string) BalancesService {
	return &balancesEntity{serviceEntity: newServiceEntity(client, authToken, baseURLs)}
}

// ListBalances lists all balances for a ledger.
// The organizationID and ledgerID parameters specify which organization and ledger to query.
// The opts parameter can be used to specify pagination, sorting, and filtering options.
// Returns a ListResponse containing the balances and pagination information, or an error if the operation fails.
func (e *balancesEntity) ListBalances(
	ctx context.Context,
	organizationID, ledgerID string,
	opts models.BalancesListOpts,
) (*models.ListResponse[models.Balance], error) {
	const operation = "ListBalances"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	endpoint := e.buildURL(organizationID, ledgerID, "")

	req, err := newRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if params := opts.ToQueryParams(); len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
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
// The organizationID, ledgerID, and accountID parameters specify which organization, ledger, and account to query.
// The opts parameter can be used to specify pagination, sorting, and filtering options.
// Returns a ListResponse containing the account balances and pagination information, or an error if the operation fails.
func (e *balancesEntity) ListAccountBalances(
	ctx context.Context,
	organizationID, ledgerID, accountID string,
	opts models.BalancesListOpts,
) (*models.ListResponse[models.Balance], error) {
	const operation = "ListAccountBalances"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if accountID == "" {
		return nil, errors.NewMissingParameterError(operation, "accountID")
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	endpoint := e.buildAccountURL(organizationID, ledgerID, accountID)

	req, err := newRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if params := opts.ToQueryParams(); len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
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
// The organizationID, ledgerID, and balanceID parameters specify which organization, ledger, and balance to retrieve.
// Returns the balance if found, or an error if the operation fails or the balance doesn't exist.
func (e *balancesEntity) GetBalance(
	ctx context.Context,
	organizationID,
	ledgerID,
	balanceID string,
) (*models.Balance, error) {
	const operation = "GetBalance"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if balanceID == "" {
		return nil, errors.NewMissingParameterError(operation, "balanceID")
	}

	endpoint := e.buildURL(organizationID, ledgerID, balanceID)

	req, err := newRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

// GetBalanceHistory retrieves the historical state of a balance at a specific point in time.
func (e *balancesEntity) GetBalanceHistory(ctx context.Context, organizationID, ledgerID, balanceID, date string) (*models.BalanceHistory, error) {
	const operation = "GetBalanceHistory"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if balanceID == "" {
		return nil, errors.NewMissingParameterError(operation, "balanceID")
	}

	if date == "" {
		return nil, errors.NewMissingParameterError(operation, "date")
	}

	if err := validateBalanceHistoryDate(date); err != nil {
		return nil, errors.NewValidationError(operation, "invalid date", err)
	}

	endpoint := e.buildBalanceHistoryURL(organizationID, ledgerID, balanceID, date)

	req, err := newRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var response models.BalanceHistory
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// UpdateBalance updates an existing balance.
// The organizationID, ledgerID, and balanceID parameters specify which organization, ledger, and balance to update.
// The input parameter contains the balance details to update, such as amount or metadata.
// Returns the updated balance, or an error if the operation fails.
func (e *balancesEntity) UpdateBalance(
	ctx context.Context,
	organizationID,
	ledgerID,
	balanceID string,
	input *models.UpdateBalanceInput,
) (*models.Balance, error) {
	const operation = "UpdateBalance"

	if organizationID == "" {
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

	endpoint := e.buildURL(organizationID, ledgerID, balanceID)

	payload, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req.Header.Set("Content-Type", "application/json")

	var balance models.Balance
	if err := e.httpClient.sendRequest(req, &balance); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &balance, nil
}

// DeleteBalance deletes a balance.
// The organizationID, ledgerID, and balanceID parameters specify which organization, ledger, and balance to delete.
// Returns an error if the operation fails.
func (e *balancesEntity) DeleteBalance(
	ctx context.Context,
	organizationID,
	ledgerID,
	balanceID string,
) error {
	const operation = "DeleteBalance"

	if organizationID == "" {
		return errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return errors.NewMissingParameterError(operation, "ledgerID")
	}

	if balanceID == "" {
		return errors.NewMissingParameterError(operation, "balanceID")
	}

	endpoint := e.buildURL(organizationID, ledgerID, balanceID)

	req, err := newRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	// HTTPClient.DoRequest already returns proper error types
	return e.httpClient.sendRequest(req, nil)
}

// buildURL builds the URL for balances API calls.
// The organizationID and ledgerID parameters specify which organization and ledger to query.
// The balanceID parameter is the unique identifier of the balance to retrieve, or an empty string for a list of balances.
// Returns the built URL.
// Note: the configured base URL is bare; the "/v1" this service needs is stamped
// on here by [serviceEntity.legacyV1BaseURL].
func (e *balancesEntity) buildURL(organizationID, ledgerID, balanceID string) string {
	baseURL := e.legacyV1BaseURL("transaction")

	if balanceID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/balances", baseURL, pathSegment(organizationID), pathSegment(ledgerID))
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/balances/%s", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(balanceID))
}

// buildAccountURL builds the URL for account balances API calls.
// The organizationID, ledgerID, and accountID parameters specify which organization, ledger, and account to query.
// Returns the built URL for retrieving balances for a specific account.
// Note: the configured base URL is bare; the "/v1" this service needs is stamped
// on here by [serviceEntity.legacyV1BaseURL].
func (e *balancesEntity) buildAccountURL(organizationID, ledgerID, accountID string) string {
	baseURL := e.legacyV1BaseURL("transaction")

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/%s/balances", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(accountID))
}

// CreateBalance creates an additional balance for an account.
func (e *balancesEntity) CreateBalance(ctx context.Context, organizationID, ledgerID, accountID string, input *models.CreateBalanceInput) (*models.Balance, error) {
	const operation = "CreateBalance"

	if organizationID == "" {
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

	endpoint := e.buildAccountURL(organizationID, ledgerID, accountID)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
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
func (e *balancesEntity) ListBalancesByAccountAlias(ctx context.Context, organizationID, ledgerID, alias string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
	const operation = "ListBalancesByAccountAlias"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if alias == "" {
		return nil, errors.NewMissingParameterError(operation, "alias")
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	endpoint := e.buildAccountAliasURL(organizationID, ledgerID, alias)

	req, err := newRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if params := opts.ToQueryParams(); len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
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
func (e *balancesEntity) ListBalancesByExternalCode(ctx context.Context, organizationID, ledgerID, code string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
	const operation = "ListBalancesByExternalCode"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if code == "" {
		return nil, errors.NewMissingParameterError(operation, "code")
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	endpoint := e.buildExternalCodeURL(organizationID, ledgerID, code)

	req, err := newRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if params := opts.ToQueryParams(); len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
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

// GetAccountBalancesHistory retrieves the historical state of all balances for an account at a specific point in time.
func (e *balancesEntity) GetAccountBalancesHistory(ctx context.Context, organizationID, ledgerID, accountID, date string) ([]models.BalanceHistory, error) {
	const operation = "GetAccountBalancesHistory"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if accountID == "" {
		return nil, errors.NewMissingParameterError(operation, "accountID")
	}

	if date == "" {
		return nil, errors.NewMissingParameterError(operation, "date")
	}

	if err := validateBalanceHistoryDate(date); err != nil {
		return nil, errors.NewValidationError(operation, "invalid date", err)
	}

	endpoint := e.buildAccountHistoryURL(organizationID, ledgerID, accountID, date)

	req, err := newRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var response []models.BalanceHistory
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return response, nil
}

// buildAccountAliasURL builds the URL for balance lookups by account alias.
func (e *balancesEntity) buildAccountAliasURL(organizationID, ledgerID, alias string) string {
	baseURL := e.legacyV1BaseURL("transaction")

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/alias/%s/balances", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(alias))
}

// buildExternalCodeURL builds the URL for balance lookups by external code.
func (e *balancesEntity) buildExternalCodeURL(organizationID, ledgerID, code string) string {
	baseURL := e.legacyV1BaseURL("transaction")

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/external/%s/balances", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(code))
}

func (e *balancesEntity) buildBalanceHistoryURL(organizationID, ledgerID, balanceID, date string) string {
	baseURL := e.legacyV1BaseURL("transaction")
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/balances/%s/history?date=%s", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(balanceID), url.QueryEscape(date))
}

func (e *balancesEntity) buildAccountHistoryURL(organizationID, ledgerID, accountID, date string) string {
	baseURL := e.legacyV1BaseURL("transaction")
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/%s/balances/history?date=%s", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(accountID), url.QueryEscape(date))
}

func validateBalanceHistoryDate(date string) error {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
	}

	for _, layout := range layouts {
		if _, err := time.Parse(layout, date); err == nil {
			return nil
		}
	}

	return stderrors.New("date must use RFC3339 or RFC3339Nano with an explicit timezone")
}

// ─────────────────────────────────────────────────────────────────────
// iter.Seq2 iterators (Track 5 Batch 5D)
// ─────────────────────────────────────────────────────────────────────

// ListBalancesAll yields every balance on the ledger, transparently advancing pagination.
func (e *balancesEntity) ListBalancesAll(ctx context.Context, organizationID, ledgerID string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error] {
	return flattenPages(e.ListBalancesPages(ctx, organizationID, ledgerID, opts))
}

// ListBalancesPages yields one full *ListResponse[Balance] per page.
func (e *balancesEntity) ListBalancesPages(ctx context.Context, organizationID, ledgerID string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	ctx = requestContext(ctx)

	return func(yield func(*models.ListResponse[models.Balance], error) bool) {
		current := opts
		if current.Page == 0 {
			current.Page = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListBalances(ctx, organizationID, ledgerID, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			if !page.Pagination.HasMore() {
				return
			}

			current.Page++
		}
	}
}

// ListAccountBalancesAll yields every balance for an account, transparently advancing pagination.
func (e *balancesEntity) ListAccountBalancesAll(ctx context.Context, organizationID, ledgerID, accountID string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error] {
	return flattenPages(e.ListAccountBalancesPages(ctx, organizationID, ledgerID, accountID, opts))
}

// ListAccountBalancesPages yields one full *ListResponse[Balance] per page for an account.
func (e *balancesEntity) ListAccountBalancesPages(ctx context.Context, organizationID, ledgerID, accountID string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	ctx = requestContext(ctx)

	return func(yield func(*models.ListResponse[models.Balance], error) bool) {
		current := opts
		if current.Page == 0 {
			current.Page = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListAccountBalances(ctx, organizationID, ledgerID, accountID, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			if !page.Pagination.HasMore() {
				return
			}

			current.Page++
		}
	}
}

// ListBalancesByAccountAliasAll yields every balance for an account alias, transparently advancing pagination.
func (e *balancesEntity) ListBalancesByAccountAliasAll(ctx context.Context, organizationID, ledgerID, alias string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error] {
	return flattenPages(e.ListBalancesByAccountAliasPages(ctx, organizationID, ledgerID, alias, opts))
}

// ListBalancesByAccountAliasPages yields one full *ListResponse[Balance] per page for an alias.
func (e *balancesEntity) ListBalancesByAccountAliasPages(ctx context.Context, organizationID, ledgerID, alias string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	ctx = requestContext(ctx)

	return func(yield func(*models.ListResponse[models.Balance], error) bool) {
		current := opts
		if current.Page == 0 {
			current.Page = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListBalancesByAccountAlias(ctx, organizationID, ledgerID, alias, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			if !page.Pagination.HasMore() {
				return
			}

			current.Page++
		}
	}
}

// ListBalancesByExternalCodeAll yields every balance for an external code, transparently advancing pagination.
func (e *balancesEntity) ListBalancesByExternalCodeAll(ctx context.Context, organizationID, ledgerID, code string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error] {
	return flattenPages(e.ListBalancesByExternalCodePages(ctx, organizationID, ledgerID, code, opts))
}

// ListBalancesByExternalCodePages yields one full *ListResponse[Balance] per page for an external code.
func (e *balancesEntity) ListBalancesByExternalCodePages(ctx context.Context, organizationID, ledgerID, code string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	ctx = requestContext(ctx)

	return func(yield func(*models.ListResponse[models.Balance], error) bool) {
		current := opts
		if current.Page == 0 {
			current.Page = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListBalancesByExternalCode(ctx, organizationID, ledgerID, code, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			if !page.Pagination.HasMore() {
				return
			}

			current.Page++
		}
	}
}

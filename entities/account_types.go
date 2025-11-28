package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

// AccountTypesService defines the interface for account type-related operations.
// It provides methods to create, read, update, and delete account types within a ledger.
type AccountTypesService interface {
	// ListAccountTypes retrieves a paginated list of account types for a ledger with optional filters.
	// The organizationID and ledgerID parameters specify which organization and ledger to query.
	// The opts parameter can be used to specify pagination, sorting, and filtering options.
	// Returns a ListResponse containing the account types and pagination information, or an error if the operation fails.
	ListAccountTypes(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.AccountType], error)

	// GetAccountType retrieves a specific account type by its ID.
	// The organizationID and ledgerID parameters specify which organization and ledger the account type belongs to.
	// The id parameter is the unique identifier of the account type to retrieve.
	// Returns the account type if found, or an error if the operation fails or the account type doesn't exist.
	GetAccountType(ctx context.Context, organizationID, ledgerID, id string) (*models.AccountType, error)

	// CreateAccountType creates a new account type in the specified ledger.
	//
	// This method creates a new account type that can be used as a template for creating accounts.
	// Account types define the characteristics and behavior of accounts within the ledger.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization that owns the ledger. Must be a valid organization ID.
	//   - ledgerID: The ID of the ledger where the account type will be created. Must be a valid ledger ID.
	//   - input: The account type details, including name, keyValue, and optional fields.
	//     Required fields in the input are:
	//     - Name: The human-readable name of the account type (max 256 characters)
	//     - KeyValue: Unique identifier within the organization/ledger (max 100 characters)
	//
	// Returns:
	//   - *models.AccountType: The created account type if successful, containing the account type ID,
	//     timestamps, and other properties.
	//   - error: An error if the operation fails. Possible errors include:
	//     - Invalid input (missing required fields)
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Resource not found (invalid organization or ledger ID)
	//     - Conflict (keyValue already exists)
	//     - Network or server errors
	//
	// Example - Creating a basic cash account type:
	//
	//	// Create a cash account type
	//	accountType, err := accountTypesService.CreateAccountType(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    &models.CreateAccountTypeInput{
	//	        Name: "Cash Account",
	//	        KeyValue: "CASH",
	//	        Description: &description,
	//	        Metadata: map[string]any{
	//	            "category": "liquid_assets",
	//	            "risk_level": "low",
	//	        },
	//	    },
	//	)
	//
	//	if err != nil {
	//	    // Handle error
	//	    return err
	//	}
	//
	//	// Use the account type
	//	fmt.Printf("Account type created: %s (keyValue: %s)\n", accountType.ID, accountType.KeyValue)
	CreateAccountType(ctx context.Context, organizationID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error)

	// UpdateAccountType updates an existing account type.
	// The organizationID and ledgerID parameters specify which organization and ledger the account type belongs to.
	// The id parameter is the unique identifier of the account type to update.
	// The input parameter contains the account type details to update, such as name or description.
	// Note that the keyValue field cannot be updated after creation.
	// Returns the updated account type, or an error if the operation fails.
	UpdateAccountType(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateAccountTypeInput) (*models.AccountType, error)

	// DeleteAccountType deletes an account type.
	// The organizationID and ledgerID parameters specify which organization and ledger the account type belongs to.
	// The id parameter is the unique identifier of the account type to delete.
	// Note that account types that are in use by existing accounts cannot be deleted.
	// Returns an error if the operation fails.
	DeleteAccountType(ctx context.Context, organizationID, ledgerID, id string) error

	// GetAccountTypesMetricsCount retrieves the count metrics for account types in a ledger.
	// The organizationID and ledgerID parameters specify which organization and ledger to get metrics for.
	// Returns the metrics count if successful, or an error if the operation fails.
	GetAccountTypesMetricsCount(ctx context.Context, organizationID, ledgerID string) (*models.MetricsCount, error)
}

// accountTypesEntity implements the AccountTypesService interface.
// It handles the communication with the Midaz API for account type-related operations.
type accountTypesEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

// NewAccountTypesEntity creates a new account types entity.
//
// Parameters:
//   - client: The HTTP client used for API requests. Can be configured with custom timeouts
//     and transport options. If nil, a default client will be used.
//   - authToken: The authentication token for API authorization. Must be a valid JWT token
//     issued by the Midaz authentication service.
//   - baseURLs: Map of service names to base URLs. Must include an "onboarding" key with
//     the URL of the onboarding service (e.g., "https://api.midaz.io/v1").
//
// Returns:
//   - AccountTypesService: An implementation of the AccountTypesService interface that provides
//     methods for creating, retrieving, updating, and managing account types.
//
// Example:
//
//	// Create an account types entity with default HTTP client
//	accountTypesEntity := entities.NewAccountTypesEntity(
//	    &http.Client{Timeout: 30 * time.Second},
//	    "your-auth-token",
//	    map[string]string{"onboarding": "https://api.midaz.io/v1"},
//	)
//
//	// Use the entity to create an account type
//	accountType, err := accountTypesEntity.CreateAccountType(
//	    context.Background(),
//	    "org-123",
//	    "ledger-456",
//	    &models.CreateAccountTypeInput{
//	        Name: "Cash Account",
//	        KeyValue: "CASH",
//	    },
//	)
//
//	if err != nil {
//	    log.Fatalf("Failed to create account type: %v", err)
//	}
//
//	fmt.Printf("Account type created: %s\n", accountType.ID)
func NewAccountTypesEntity(client *http.Client, authToken string, baseURLs map[string]string) AccountTypesService {
	// Create a new HTTP client with the shared implementation
	httpClient := NewHTTPClient(client, authToken, nil)

	// Check if we're using the debug flag from the environment
	if debugEnv := os.Getenv(EnvMidazDebug); debugEnv == BoolTrue {
		httpClient.debug = true
	}

	return &accountTypesEntity{
		httpClient: httpClient,
		baseURLs:   baseURLs,
	}
}

// buildURL constructs the URL for account type operations.
func (e *accountTypesEntity) buildURL(organizationID, ledgerID, accountTypeID string) string {
	baseURL := e.baseURLs["onboarding"]

	if accountTypeID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/account-types", baseURL, organizationID, ledgerID)
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/account-types/%s", baseURL, organizationID, ledgerID, accountTypeID)
}

// ListAccountTypes lists account types for a ledger with optional filters.
func (e *accountTypesEntity) ListAccountTypes(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.AccountType], error) {
	const operation = "ListAccountTypes"

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

	var response models.ListResponse[models.AccountType]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetAccountType gets an account type by ID.
func (e *accountTypesEntity) GetAccountType(ctx context.Context, organizationID, ledgerID, id string) (*models.AccountType, error) {
	const operation = "GetAccountType"

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

	var accountType models.AccountType
	if err := e.httpClient.sendRequest(req, &accountType); err != nil {
		return nil, err
	}

	return &accountType, nil
}

// CreateAccountType creates a new account type.
func (e *accountTypesEntity) CreateAccountType(ctx context.Context, organizationID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
	const operation = "CreateAccountType"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "account type validation failed", err)
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

	var accountType models.AccountType
	if err := e.httpClient.sendRequest(req, &accountType); err != nil {
		return nil, err
	}

	return &accountType, nil
}

// UpdateAccountType updates an existing account type.
func (e *accountTypesEntity) UpdateAccountType(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateAccountTypeInput) (*models.AccountType, error) {
	const operation = "UpdateAccountType"

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

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "account type validation failed", err)
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

	var accountType models.AccountType
	if err := e.httpClient.sendRequest(req, &accountType); err != nil {
		return nil, err
	}

	return &accountType, nil
}

// DeleteAccountType deletes an account type.
func (e *accountTypesEntity) DeleteAccountType(ctx context.Context, organizationID, ledgerID, id string) error {
	const operation = "DeleteAccountType"

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

	return e.httpClient.sendRequest(req, nil)
}

// GetAccountTypesMetricsCount retrieves the count metrics for account types in a ledger.
func (e *accountTypesEntity) GetAccountTypesMetricsCount(ctx context.Context, organizationID, ledgerID string) (*models.MetricsCount, error) {
	const operation = "GetAccountTypesMetricsCount"

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

// buildMetricsURL builds the URL for account types metrics API calls.
func (e *accountTypesEntity) buildMetricsURL(organizationID, ledgerID string) string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/account-types/metrics/count", baseURL, organizationID, ledgerID)
}

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

// LedgersService defines the interface for ledger-related operations.
// It provides methods to create, read, update, and delete ledgers
// within an organization.
type LedgersService interface {
	// ListLedgers retrieves a paginated list of ledgers for an organization with optional filters.
	// The organizationID parameter specifies which organization to query.
	// The opts parameter can be used to specify pagination, sorting, and filtering options.
	// Returns a ListResponse containing the ledgers and pagination information, or an error if the operation fails.
	ListLedgers(ctx context.Context, organizationID string, opts *models.ListOptions) (*models.ListResponse[models.Ledger], error)

	// GetLedger retrieves a specific ledger by its ID.
	// The organizationID parameter specifies which organization the ledger belongs to.
	// The id parameter is the unique identifier of the ledger to retrieve.
	// Returns the ledger if found, or an error if the operation fails or the ledger doesn't exist.
	GetLedger(ctx context.Context, organizationID, id string) (*models.Ledger, error)

	// CreateLedger creates a new ledger in the specified organization.
	//
	// Ledgers are the top-level financial record-keeping systems that contain accounts
	// and track all transactions between those accounts. Each ledger belongs to a specific
	// organization and can have multiple accounts.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization where the ledger will be created.
	//     Must be a valid organization ID.
	//   - input: The ledger details, including required fields:
	//     - Name: The human-readable name of the ledger (max length: 256 characters)
	//     Optional fields include:
	//     - Status: The initial status (defaults to ACTIVE if not specified)
	//     - Metadata: Additional custom information about the ledger
	//
	// Returns:
	//   - *models.Ledger: The created ledger if successful, containing the ledger ID,
	//     name, status, and other properties.
	//   - error: An error if the operation fails. Possible errors include:
	//     - Invalid input (missing required fields or invalid values)
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Resource not found (invalid organization ID)
	//     - Network or server errors
	//
	// Example - Creating a basic ledger:
	//
	//	// Create a simple ledger with just a name
	//	ledger, err := ledgersService.CreateLedger(
	//	    context.Background(),
	//	    "org-123",
	//	    models.NewCreateLedgerInput("Main Ledger"),
	//	)
	//
	//	if err != nil {
	//	    // Handle error
	//	    return err
	//	}
	//
	//	// Use the ledger
	//	fmt.Printf("Ledger created: %s (status: %s)\n", ledger.ID, ledger.Status.Code)
	//
	// Example - Creating a ledger with metadata:
	//
	//	// Create a ledger with custom status and metadata
	//	ledger, err := ledgersService.CreateLedger(
	//	    context.Background(),
	//	    "org-123",
	//	    models.NewCreateLedgerInput("Finance Ledger").
	//	        WithStatus(models.StatusActive).
	//	        WithMetadata(map[string]any{
	//	            "department": "Finance",
	//	            "fiscalYear": 2025,
	//	            "currency": "USD",
	//	            "description": "Primary ledger for financial operations",
	//	        }),
	//	)
	//
	//	if err != nil {
	//	    // Handle error
	//	    return err
	//	}
	//
	//	// Use the ledger
	//	fmt.Printf("Finance ledger created: %s\n", ledger.ID)
	CreateLedger(ctx context.Context, organizationID string, input *models.CreateLedgerInput) (*models.Ledger, error)

	// UpdateLedger updates an existing ledger.
	// The organizationID parameter specifies which organization the ledger belongs to.
	// The id parameter is the unique identifier of the ledger to update.
	// The input parameter contains the ledger details to update, such as name, description, or status.
	// Returns the updated ledger, or an error if the operation fails.
	UpdateLedger(ctx context.Context, organizationID, id string, input *models.UpdateLedgerInput) (*models.Ledger, error)

	// DeleteLedger deletes a ledger.
	// The organizationID parameter specifies which organization the ledger belongs to.
	// The id parameter is the unique identifier of the ledger to delete.
	// Returns an error if the operation fails.
	DeleteLedger(ctx context.Context, organizationID, id string) error

	// GetLedgersMetricsCount retrieves the count metrics for ledgers in an organization.
	// The organizationID parameter specifies which organization to get metrics for.
	// Returns the metrics count if successful, or an error if the operation fails.
	GetLedgersMetricsCount(ctx context.Context, organizationID string) (*models.MetricsCount, error)
}

// ledgersEntity implements the LedgersService interface.
// It handles the communication with the Midaz API for ledger-related operations.
type ledgersEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

// NewLedgersEntity creates a new ledgers entity.
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
//   - LedgersService: An implementation of the LedgersService interface that provides
//     methods for creating, retrieving, updating, and managing ledgers.
//
// Example:
//
//	// Create a ledgers entity with default HTTP client
//	ledgersEntity := entities.NewLedgersEntity(
//	    &http.Client{Timeout: 30 * time.Second},
//	    "your-auth-token",
//	    map[string]string{"onboarding": "https://api.midaz.io/v1"},
//	)
//
//	// Use the entity to create a new ledger
//	ledger, err := ledgersEntity.CreateLedger(
//	    context.Background(),
//	    "org-123",
//	    models.NewCreateLedgerInput("Main Ledger").
//	        WithMetadata(map[string]any{
//	            "department": "Finance",
//	            "fiscalYear": 2025,
//	        }),
//	)
//
//	if err != nil {
//	    log.Fatalf("Failed to create ledger: %v", err)
//	}
//
//	fmt.Printf("Ledger created: %s\n", ledger.ID)
func NewLedgersEntity(client *http.Client, authToken string, baseURLs map[string]string) LedgersService {
	// Create a new HTTP client with the shared implementation
	httpClient := NewHTTPClient(client, authToken, nil)

	// Check if we're using the debug flag from the environment
	if debugEnv := os.Getenv("MIDAZ_DEBUG"); debugEnv == "true" {
		httpClient.debug = true
	}

	return &ledgersEntity{
		httpClient: httpClient,
		baseURLs:   baseURLs,
	}
}

// ListLedgers lists all ledgers for an organization with optional filters.
// The organizationID parameter specifies which organization to query.
// The opts parameter can be used to specify pagination, sorting, and filtering options.
// Returns a ListResponse containing the ledgers and pagination information, or an error if the operation fails.
func (e *ledgersEntity) ListLedgers(
	ctx context.Context,
	organizationID string,
	opts *models.ListOptions,
) (*models.ListResponse[models.Ledger], error) {
	const operation = "ListLedgers"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	url := e.buildURL(organizationID, "")

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

	var response models.ListResponse[models.Ledger]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetLedger gets a ledger by ID.
// The organizationID parameter specifies which organization the ledger belongs to.
// The id parameter is the unique identifier of the ledger to retrieve.
// Returns the ledger if found, or an error if the operation fails or the ledger doesn't exist.
func (e *ledgersEntity) GetLedger(
	ctx context.Context,
	organizationID, id string,
) (*models.Ledger, error) {
	const operation = "GetLedger"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if id == "" {
		return nil, errors.NewMissingParameterError(operation, "id")
	}

	url := e.buildURL(organizationID, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var ledger models.Ledger
	if err := e.httpClient.sendRequest(req, &ledger); err != nil {
		return nil, err
	}

	return &ledger, nil
}

// CreateLedger creates a new ledger in the specified organization.
func (e *ledgersEntity) CreateLedger(
	ctx context.Context,
	organizationID string,
	input *models.CreateLedgerInput,
) (*models.Ledger, error) {
	const operation = "CreateLedger"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	url := e.buildURL(organizationID, "")

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var ledger models.Ledger
	if err := e.httpClient.sendRequest(req, &ledger); err != nil {
		return nil, err
	}

	return &ledger, nil
}

// UpdateLedger updates an existing ledger.
// The organizationID parameter specifies which organization the ledger belongs to.
// The id parameter is the unique identifier of the ledger to update.
// The input parameter contains the ledger details to update, such as name, description, or status.
// Returns the updated ledger, or an error if the operation fails.
func (e *ledgersEntity) UpdateLedger(
	ctx context.Context,
	organizationID, id string,
	input *models.UpdateLedgerInput,
) (*models.Ledger, error) {
	const operation = "UpdateLedger"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if id == "" {
		return nil, errors.NewMissingParameterError(operation, "id")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	url := e.buildURL(organizationID, id)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var ledger models.Ledger
	if err := e.httpClient.sendRequest(req, &ledger); err != nil {
		return nil, err
	}

	return &ledger, nil
}

// DeleteLedger deletes a ledger.
// The organizationID parameter specifies which organization the ledger belongs to.
// The id parameter is the unique identifier of the ledger to delete.
// Returns an error if the operation fails.
func (e *ledgersEntity) DeleteLedger(
	ctx context.Context,
	organizationID, id string,
) error {
	const operation = "DeleteLedger"

	if organizationID == "" {
		return errors.NewMissingParameterError(operation, "organizationID")
	}

	if id == "" {
		return errors.NewMissingParameterError(operation, "id")
	}

	url := e.buildURL(organizationID, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if err := e.httpClient.sendRequest(req, nil); err != nil {
		return err
	}

	return nil
}

// GetLedgersMetricsCount gets the count metrics for ledgers in an organization.
func (e *ledgersEntity) GetLedgersMetricsCount(ctx context.Context, organizationID string) (*models.MetricsCount, error) {
	const operation = "GetLedgersMetricsCount"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	url := e.buildMetricsURL(organizationID)

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

// buildURL builds the URL for ledgers API calls.
func (e *ledgersEntity) buildURL(organizationID, ledgerID string) string {
	baseURL := e.baseURLs["onboarding"]

	if ledgerID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers", baseURL, organizationID)
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s", baseURL, organizationID, ledgerID)
}

// buildMetricsURL builds the URL for ledgers metrics API calls.
func (e *ledgersEntity) buildMetricsURL(organizationID string) string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/metrics/count", baseURL, organizationID)
}

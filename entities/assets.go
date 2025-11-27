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

// AssetsService defines the interface for asset-related operations.
// It provides methods to create, read, update, and delete assets.
type AssetsService interface {
	// ListAssets retrieves a paginated list of assets for a ledger with optional filters.
	// The organizationID and ledgerID parameters specify which organization and ledger to query.
	// The opts parameter can be used to specify pagination, sorting, and filtering options.
	// Returns a ListResponse containing the assets and pagination information, or an error if the operation fails.
	ListAssets(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Asset], error)

	// GetAsset retrieves a specific asset by its ID.
	// The organizationID and ledgerID parameters specify which organization and ledger the asset belongs to.
	// The id parameter is the unique identifier of the asset to retrieve.
	// Returns the asset if found, or an error if the operation fails or the asset doesn't exist.
	GetAsset(ctx context.Context, organizationID, ledgerID, id string) (*models.Asset, error)

	// CreateAsset creates a new asset in the specified ledger.
	//
	// Assets represent units of value that can be tracked and transferred within the Midaz
	// ledger system. Each asset has a unique code and can be used in transactions.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization that owns the ledger. Must be a valid organization ID.
	//   - ledgerID: The ID of the ledger where the asset will be created. Must be a valid ledger ID.
	//   - input: The asset details, including required fields:
	//     - Name: The human-readable name of the asset (e.g., "US Dollar")
	//     - Code: The unique asset code (e.g., "USD")
	//     Optional fields include:
	//     - Type: The asset type (e.g., "CURRENCY", "SECURITY", "COMMODITY")
	//     - Status: The initial status (defaults to ACTIVE if not specified)
	//     - Metadata: Additional custom information about the asset
	//
	// Returns:
	//   - *models.Asset: The created asset if successful, containing the asset ID,
	//     status, and other properties.
	//   - error: An error if the operation fails. Possible errors include:
	//     - Invalid input (missing required fields)
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Resource not found (invalid organization or ledger ID)
	//     - Conflict (asset code already exists)
	//     - Network or server errors
	//
	// Example - Creating a basic currency asset:
	//
	//	// Create a currency asset
	//	asset, err := assetsService.CreateAsset(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    &models.CreateAssetInput{
	//	        Name: "US Dollar",
	//	        Code: "USD",
	//	        Type: "CURRENCY",
	//	    },
	//	)
	//
	//	if err != nil {
	//	    // Handle error
	//	    return err
	//	}
	//
	//	// Use the asset
	//	fmt.Printf("Asset created: %s (code: %s)\n", asset.ID, asset.Code)
	//
	// Example - Creating an asset with metadata:
	//
	//	// Create a security asset with metadata
	//	asset, err := assetsService.CreateAsset(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    models.NewCreateAssetInput("Apple Inc. Stock", "AAPL").
	//	        WithType("SECURITY").
	//	        WithStatus(models.StatusActive).
	//	        WithMetadata(map[string]any{
	//	            "exchange": "NASDAQ",
	//	            "sector": "Technology",
	//	            "currency": "USD",
	//	            "isin": "US0378331005",
	//	        }),
	//	)
	//
	//	if err != nil {
	//	    // Handle error
	//	    return err
	//	}
	//
	//	// Use the asset
	//	fmt.Printf("Security asset created: %s\n", asset.ID)
	CreateAsset(ctx context.Context, organizationID, ledgerID string, input *models.CreateAssetInput) (*models.Asset, error)

	// UpdateAsset updates an existing asset.
	// The organizationID and ledgerID parameters specify which organization and ledger the asset belongs to.
	// The id parameter is the unique identifier of the asset to update.
	// The input parameter contains the asset details to update, such as name or status.
	// Returns the updated asset, or an error if the operation fails.
	UpdateAsset(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateAssetInput) (*models.Asset, error)

	// DeleteAsset deletes an asset.
	// The organizationID and ledgerID parameters specify which organization and ledger the asset belongs to.
	// The id parameter is the unique identifier of the asset to delete.
	// Returns an error if the operation fails.
	DeleteAsset(ctx context.Context, organizationID, ledgerID, id string) error

	// GetAssetsMetricsCount retrieves the count metrics for assets in a ledger.
	// The organizationID and ledgerID parameters specify which organization and ledger to get metrics for.
	// Returns the metrics count if successful, or an error if the operation fails.
	GetAssetsMetricsCount(ctx context.Context, organizationID, ledgerID string) (*models.MetricsCount, error)
}

// assetsEntity implements the AssetsService interface.
// It handles the communication with the Midaz API for asset-related operations.
type assetsEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

// NewAssetsEntity creates a new assets entity.
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
//   - AssetsService: An implementation of the AssetsService interface that provides
//     methods for creating, retrieving, updating, and managing assets.
//
// Example:
//
//	// Create an assets entity with default HTTP client
//	assetsEntity := entities.NewAssetsEntity(
//	    &http.Client{Timeout: 30 * time.Second},
//	    "your-auth-token",
//	    map[string]string{"onboarding": "https://api.midaz.io/v1"},
//	)
//
//	// Use the entity to create an asset
//	asset, err := assetsEntity.CreateAsset(
//	    context.Background(),
//	    "org-123",
//	    "ledger-456",
//	    &models.CreateAssetInput{
//	        Name: "US Dollar",
//	        Code: "USD",
//	        Type: "CURRENCY",
//	    },
//	)
//
//	if err != nil {
//	    log.Fatalf("Failed to create asset: %v", err)
//	}
//
//	fmt.Printf("Asset created: %s\n", asset.ID)
func NewAssetsEntity(client *http.Client, authToken string, baseURLs map[string]string) AssetsService {
	// Create a new HTTP client with the shared implementation
	httpClient := NewHTTPClient(client, authToken, nil)

	// Check if we're using the debug flag from the environment
	if debugEnv := os.Getenv(EnvMidazDebug); debugEnv == BoolTrue {
		httpClient.debug = true
	}

	return &assetsEntity{
		httpClient: httpClient,
		baseURLs:   baseURLs,
	}
}

// ListAssets lists assets for a ledger with optional filters.
// The organizationID and ledgerID parameters specify which organization and ledger to query.
// The opts parameter can be used to specify pagination, sorting, and filtering options.
// Returns a ListResponse containing the assets and pagination information, or an error if the operation fails.
func (e *assetsEntity) ListAssets(
	ctx context.Context,
	organizationID, ledgerID string,
	opts *models.ListOptions,
) (*models.ListResponse[models.Asset], error) {
	const operation = "ListAssets"

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

	var response models.ListResponse[models.Asset]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &response, nil
}

// GetAsset gets an asset by ID.
// The organizationID and ledgerID parameters specify which organization and ledger the asset belongs to.
// The id parameter is the unique identifier of the asset to retrieve.
// Returns the asset if found, or an error if the operation fails or the asset doesn't exist.
func (e *assetsEntity) GetAsset(
	ctx context.Context,
	organizationID, ledgerID, id string,
) (*models.Asset, error) {
	const operation = "GetAsset"

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

	var asset models.Asset
	if err := e.httpClient.sendRequest(req, &asset); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &asset, nil
}

// CreateAsset creates a new asset in the specified ledger.
func (e *assetsEntity) CreateAsset(
	ctx context.Context,
	organizationID, ledgerID string,
	input *models.CreateAssetInput,
) (*models.Asset, error) {
	const operation = "CreateAsset"

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

	var asset models.Asset
	if err := e.httpClient.sendRequest(req, &asset); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &asset, nil
}

// UpdateAsset updates an existing asset.
// The organizationID and ledgerID parameters specify which organization and ledger the asset belongs to.
// The id parameter is the unique identifier of the asset to update.
// The input parameter contains the asset details to update, such as name or status.
// Returns the updated asset, or an error if the operation fails.
func (e *assetsEntity) UpdateAsset(
	ctx context.Context,
	organizationID, ledgerID, id string,
	input *models.UpdateAssetInput,
) (*models.Asset, error) {
	const operation = "UpdateAsset"

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

	var asset models.Asset
	if err := e.httpClient.sendRequest(req, &asset); err != nil {
		return nil, err
	}

	return &asset, nil
}

// DeleteAsset deletes an asset.
// The organizationID and ledgerID parameters specify which organization and ledger the asset belongs to.
// The id parameter is the unique identifier of the asset to delete.
// Returns an error if the operation fails.
func (e *assetsEntity) DeleteAsset(
	ctx context.Context,
	organizationID, ledgerID, id string,
) error {
	const operation = "DeleteAsset"

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
		// HTTPClient.DoRequest already returns proper error types
		return err
	}

	return nil
}

// GetAssetsMetricsCount gets the count metrics for assets in a ledger.
func (e *assetsEntity) GetAssetsMetricsCount(ctx context.Context, organizationID, ledgerID string) (*models.MetricsCount, error) {
	const operation = "GetAssetsMetricsCount"

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

// buildURL builds the URL for assets API calls.
// The organizationID and ledgerID parameters specify which organization and ledger to query.
// The assetID parameter is the unique identifier of the asset to retrieve, or an empty string for a list of assets.
// Returns the built URL.
func (e *assetsEntity) buildURL(organizationID, ledgerID, assetID string) string {
	baseURL := e.baseURLs["onboarding"]

	if assetID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/assets", baseURL, organizationID, ledgerID)
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/assets/%s", baseURL, organizationID, ledgerID, assetID)
}

// buildMetricsURL builds the URL for assets metrics API calls.
func (e *assetsEntity) buildMetricsURL(organizationID, ledgerID string) string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/assets/metrics/count", baseURL, organizationID, ledgerID)
}

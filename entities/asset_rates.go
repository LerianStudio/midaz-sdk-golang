package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

// AssetRatesService defines the interface for asset rate operations.
// It provides methods to create, update, and retrieve asset conversion rates.
type AssetRatesService interface {
	// CreateOrUpdateAssetRate creates a new asset rate or updates an existing one.
	//
	// This method uses PUT semantics - if an asset rate with the same from/to asset
	// codes exists, it will be updated; otherwise, a new one will be created.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization that owns the ledger.
	//   - ledgerID: The ID of the ledger where the asset rate will be stored.
	//   - input: The asset rate details including source/target assets and conversion rate.
	//
	// Returns:
	//   - *models.AssetRate: The created or updated asset rate if successful.
	//   - error: An error if the operation fails.
	//
	// Example:
	//
	//	rate, err := assetRatesService.CreateOrUpdateAssetRate(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    models.NewCreateAssetRateInput("USD", "BRL", 500).
	//	        WithScale(2).
	//	        WithSource("Central Bank"),
	//	)
	CreateOrUpdateAssetRate(ctx context.Context, organizationID, ledgerID string, input *models.CreateAssetRateInput) (*models.AssetRate, error)

	// GetAssetRate retrieves an asset rate by its external ID.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization that owns the ledger.
	//   - ledgerID: The ID of the ledger containing the asset rate.
	//   - externalID: The external identifier of the asset rate to retrieve.
	//
	// Returns:
	//   - *models.AssetRate: The asset rate if found.
	//   - error: An error if the operation fails or the asset rate doesn't exist.
	//
	// Example:
	//
	//	rate, err := assetRatesService.GetAssetRate(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    "external-id-789",
	//	)
	GetAssetRate(ctx context.Context, organizationID, ledgerID, externalID string) (*models.AssetRate, error)

	// ListAssetRatesByAssetCode retrieves all asset rates for a specific source asset code.
	//
	// This method returns a paginated list of asset rates where the source asset
	// matches the specified asset code.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization that owns the ledger.
	//   - ledgerID: The ID of the ledger containing the asset rates.
	//   - assetCode: The source asset code to filter by (e.g., "USD").
	//   - opts: Optional parameters for filtering and pagination.
	//
	// Returns:
	//   - *models.AssetRatesResponse: A paginated list of asset rates.
	//   - error: An error if the operation fails.
	//
	// Example:
	//
	//	rates, err := assetRatesService.ListAssetRatesByAssetCode(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    "USD",
	//	    models.NewAssetRateListOptions().WithTo("BRL", "EUR").WithLimit(10),
	//	)
	ListAssetRatesByAssetCode(ctx context.Context, organizationID, ledgerID, assetCode string, opts *models.AssetRateListOptions) (*models.AssetRatesResponse, error)
}

// assetRatesEntity implements the AssetRatesService interface.
type assetRatesEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

// NewAssetRatesEntity creates a new asset rates entity.
//
// Parameters:
//   - client: The HTTP client used for API requests.
//   - authToken: The authentication token for API authorization.
//   - baseURLs: Map of service names to base URLs.
//
// Returns:
//   - AssetRatesService: An implementation of the AssetRatesService interface.
func NewAssetRatesEntity(client *http.Client, authToken string, baseURLs map[string]string) AssetRatesService {
	httpClient := NewHTTPClient(client, authToken, nil)

	if debugEnv := os.Getenv(EnvMidazDebug); debugEnv == BoolTrue {
		httpClient.debug = true
	}

	return &assetRatesEntity{
		httpClient: httpClient,
		baseURLs:   baseURLs,
	}
}

// CreateOrUpdateAssetRate creates a new asset rate or updates an existing one.
func (e *assetRatesEntity) CreateOrUpdateAssetRate(
	ctx context.Context,
	organizationID, ledgerID string,
	input *models.CreateAssetRateInput,
) (*models.AssetRate, error) {
	const operation = "CreateOrUpdateAssetRate"

	if strings.TrimSpace(organizationID) == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if strings.TrimSpace(ledgerID) == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "invalid asset rate input", err)
	}

	url := e.buildURL(organizationID, ledgerID, "")

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var assetRate models.AssetRate
	if err := e.httpClient.sendRequest(req, &assetRate); err != nil {
		return nil, err
	}

	return &assetRate, nil
}

// GetAssetRate retrieves an asset rate by its external ID.
func (e *assetRatesEntity) GetAssetRate(
	ctx context.Context,
	organizationID, ledgerID, externalID string,
) (*models.AssetRate, error) {
	const operation = "GetAssetRate"

	if strings.TrimSpace(organizationID) == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if strings.TrimSpace(ledgerID) == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if strings.TrimSpace(externalID) == "" {
		return nil, errors.NewMissingParameterError(operation, "externalID")
	}

	url := e.buildURL(organizationID, ledgerID, externalID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var assetRate models.AssetRate
	if err := e.httpClient.sendRequest(req, &assetRate); err != nil {
		return nil, err
	}

	return &assetRate, nil
}

// ListAssetRatesByAssetCode retrieves all asset rates for a specific source asset code.
func (e *assetRatesEntity) ListAssetRatesByAssetCode(
	ctx context.Context,
	organizationID, ledgerID, assetCode string,
	opts *models.AssetRateListOptions,
) (*models.AssetRatesResponse, error) {
	const operation = "ListAssetRatesByAssetCode"

	if strings.TrimSpace(organizationID) == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if strings.TrimSpace(ledgerID) == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if strings.TrimSpace(assetCode) == "" {
		return nil, errors.NewMissingParameterError(operation, "assetCode")
	}

	url := e.buildFromAssetURL(organizationID, ledgerID, assetCode)

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

	var response models.AssetRatesResponse
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// buildURL builds the URL for asset rates API calls.
func (e *assetRatesEntity) buildURL(organizationID, ledgerID, externalID string) string {
	baseURL := e.baseURLs["transaction"]

	if externalID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/asset-rates", baseURL, organizationID, ledgerID)
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/asset-rates/%s", baseURL, organizationID, ledgerID, externalID)
}

// buildFromAssetURL builds the URL for listing asset rates by source asset code.
func (e *assetRatesEntity) buildFromAssetURL(organizationID, ledgerID, assetCode string) string {
	baseURL := e.baseURLs["transaction"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/asset-rates/from/%s", baseURL, organizationID, ledgerID, assetCode)
}

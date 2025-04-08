package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/errors"
)

// AssetRatesService defines the interface for asset rate-related operations.
// It provides methods to manage exchange rates between different assets.
type AssetRatesService interface {
	// GetAssetRate retrieves the exchange rate between two assets.
	// The organizationID and ledgerID parameters specify which organization and ledger the assets belong to.
	// The sourceAssetCode and destinationAssetCode parameters specify the assets for which to get the exchange rate.
	// Returns the asset rate if found, or an error if the operation fails or the rate doesn't exist.
	GetAssetRate(ctx context.Context, organizationID, ledgerID, sourceAssetCode, destinationAssetCode string) (*models.AssetRate, error)

	// CreateOrUpdateAssetRate creates a new asset rate or updates an existing one.
	// The organizationID and ledgerID parameters specify which organization and ledger to create/update the asset rate in.
	// The input parameter contains the asset rate details such as source asset, destination asset, and rate.
	// Returns the created or updated asset rate, or an error if the operation fails.
	CreateOrUpdateAssetRate(ctx context.Context, organizationID, ledgerID string, input *models.UpdateAssetRateInput) (*models.AssetRate, error)
}

// assetRatesEntity implements the AssetRatesService interface.
// It handles the communication with the Midaz API for asset rate-related operations.
type assetRatesEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

// NewAssetRatesEntity creates a new asset rates entity.
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
//   - AssetRatesService: An implementation of the AssetRatesService interface that provides
//     methods for creating, retrieving, and managing asset exchange rates.
func NewAssetRatesEntity(client *http.Client, authToken string, baseURLs map[string]string) AssetRatesService {
	// Create a new HTTP client with the shared implementation
	httpClient := NewHTTPClient(client, authToken, nil)

	// Check if we're using the debug flag from the environment
	if debugEnv := os.Getenv("MIDAZ_DEBUG"); debugEnv == "true" {
		httpClient.debug = true
	}

	return &assetRatesEntity{
		httpClient: httpClient,
		baseURLs:   baseURLs,
	}
}

// GetAssetRate gets an asset rate by source and destination asset codes.
func (e *assetRatesEntity) GetAssetRate(ctx context.Context, organizationID, ledgerID, sourceAssetCode, destinationAssetCode string) (*models.AssetRate, error) {
	const operation = "GetAssetRate"
	const resource = "asset rate"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if sourceAssetCode == "" {
		return nil, errors.NewMissingParameterError(operation, "sourceAssetCode")
	}

	if destinationAssetCode == "" {
		return nil, errors.NewMissingParameterError(operation, "destinationAssetCode")
	}

	// Special case: same asset conversion is always 1.0
	if sourceAssetCode == destinationAssetCode {
		now := time.Now()
		return &models.AssetRate{
			ID:           fmt.Sprintf("generated-%s-%s", sourceAssetCode, destinationAssetCode),
			FromAsset:    sourceAssetCode,
			ToAsset:      destinationAssetCode,
			Rate:         1.0,
			CreatedAt:    now,
			UpdatedAt:    now,
			EffectiveAt:  now,
			ExpirationAt: now.AddDate(10, 0, 0), // 10 years in the future
		}, nil
	}

	// Use the from/{asset_code} endpoint and filter by destination
	url := e.buildFromAssetURL(organizationID, ledgerID, sourceAssetCode)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewNetworkError(operation, err)
	}

	// Get all rates for the source asset
	var response struct {
		Items []models.AssetRate `json:"items"`
	}
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	// Find the rate for the specific destination asset
	for _, rate := range response.Items {
		if rate.ToAsset == destinationAssetCode {
			return &rate, nil
		}
	}

	return nil, errors.NewNotFoundError(
		operation,
		resource,
		fmt.Sprintf("%s to %s", sourceAssetCode, destinationAssetCode),
		nil,
	)
}

// CreateOrUpdateAssetRate creates or updates an asset rate.
//
// Asset rates define the exchange ratio between two assets within the same ledger.
// If a rate already exists for the specified asset pair, it will be updated; otherwise,
// a new rate will be created.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - organizationID: The ID of the organization that owns the ledger. Must be a valid organization ID.
//   - ledgerID: The ID of the ledger where the asset rate will be created/updated. Must be a valid ledger ID.
//   - input: The asset rate details, including:
//   - FromAsset: The source asset code (e.g., "USD")
//   - ToAsset: The target asset code (e.g., "EUR")
//   - Rate: The exchange rate value (e.g., 0.92 means 1 unit of FromAsset = 0.92 units of ToAsset)
//   - EffectiveAt: The timestamp when the rate becomes effective
//   - ExpirationAt: The timestamp when the rate expires
//
// Returns:
//   - *models.AssetRate: The created or updated asset rate if successful, containing the rate ID,
//     source and target assets, rate value, and effective/expiration dates.
//   - error: An error if the operation fails. Possible errors include:
//   - Invalid input (missing required fields or invalid values)
//   - Authentication failure (invalid auth token)
//   - Authorization failure (insufficient permissions)
//   - Resource not found (invalid organization, ledger, or asset codes)
//   - Network or server errors
func (e *assetRatesEntity) CreateOrUpdateAssetRate(ctx context.Context, organizationID, ledgerID string, input *models.UpdateAssetRateInput) (*models.AssetRate, error) {
	const operation = "CreateOrUpdateAssetRate"
	const resource = "asset rate"

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
		return nil, errors.NewValidationError(operation, "asset rate validation failed", err)
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

	var assetRate models.AssetRate
	if err := e.httpClient.sendRequest(req, &assetRate); err != nil {
		return nil, err
	}

	return &assetRate, nil
}

// buildURL builds the URL for asset rates API calls.
func (e *assetRatesEntity) buildURL(organizationID, ledgerID, query string) string {
	base := e.baseURLs["transaction"]

	// Ensure the base URL doesn't end with a trailing slash
	base = strings.TrimSuffix(base, "/")

	url := fmt.Sprintf("%s/organizations/%s/ledgers/%s/asset-rates", base, organizationID, ledgerID)

	if query != "" {
		// Check if the query already starts with a question mark
		if !strings.HasPrefix(query, "?") {
			url += "?"
		}
		url += query
	}

	return url
}

// buildFromAssetURL builds the URL for retrieving asset rates by source asset code.
func (e *assetRatesEntity) buildFromAssetURL(organizationID, ledgerID, sourceAssetCode string) string {
	base := e.baseURLs["transaction"]

	// Ensure the base URL doesn't end with a trailing slash
	base = strings.TrimSuffix(base, "/")

	url := fmt.Sprintf("%s/organizations/%s/ledgers/%s/asset-rates/from/%s", base, organizationID, ledgerID, sourceAssetCode)

	return url
}

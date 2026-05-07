package entities

//go:generate mockgen -source=assets.go -destination=mocks/mock_assets.go -package=mocks AssetsService

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// AssetsService defines the interface for asset-related operations.
// It provides methods to create, read, update, and delete assets.
type AssetsService interface {
	// ListAssets retrieves a paginated list of assets for a ledger with optional filters.
	// The organizationID and ledgerID parameters specify which organization and ledger to query.
	// The opts parameter can be used to specify pagination, sorting, and filtering options.
	// Returns a ListResponse containing the assets and pagination information, or an error if the operation fails.
	ListAssets(ctx context.Context, organizationID, ledgerID string, opts models.AssetsListOpts) (*models.ListResponse[models.Asset], error)

	ListAssetsAll(ctx context.Context, organizationID, ledgerID string, opts models.AssetsListOpts) iter.Seq2[models.Asset, error]

	ListAssetsPages(ctx context.Context, organizationID, ledgerID string, opts models.AssetsListOpts) iter.Seq2[*models.ListResponse[models.Asset], error]

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
	//     - Type: The asset type (e.g., "currency", "security", "commodity")
	//     Optional fields include:
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
	//	        Type: "currency",
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
	//	    models.NewCreateAssetInputWithType("Apple Inc. Stock", "AAPL", "security").
	//	        WithStatus(models.NewStatus(models.StatusActive)).
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
	serviceEntity
}

// newAssetsEntity wires the AssetsService backed by the shared HTTP transport.
// Internal: invoked by Entity.initServices; callers should reach the service via Client.Assets.
func newAssetsEntity(client *http.Client, authToken string, baseURLs map[string]string) AssetsService {
	return &assetsEntity{serviceEntity: newServiceEntity(client, authToken, baseURLs)}
}

// ListAssets lists assets for a ledger with optional filters.
// The organizationID and ledgerID parameters specify which organization and ledger to query.
// The opts parameter can be used to specify pagination, sorting, and filtering options.
// Returns a ListResponse containing the assets and pagination information, or an error if the operation fails.
func (e *assetsEntity) ListAssets(ctx context.Context, organizationID, ledgerID string, opts models.AssetsListOpts) (*models.ListResponse[models.Asset], error) {
	const operation = "ListAssets"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	url := e.buildURL(organizationID, ledgerID, "")

	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
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

	var response models.ListResponse[models.Asset]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &response, nil
}

// ListAssetsAll yields every asset matching the request, transparently advancing pagination.
func (e *assetsEntity) ListAssetsAll(ctx context.Context, organizationID, ledgerID string, opts models.AssetsListOpts) iter.Seq2[models.Asset, error] {
	return flattenPages(e.ListAssetsPages(ctx, organizationID, ledgerID, opts))
}

// ListAssetsPages yields one full *ListResponse[Asset] per page.
func (e *assetsEntity) ListAssetsPages(ctx context.Context, organizationID, ledgerID string, opts models.AssetsListOpts) iter.Seq2[*models.ListResponse[models.Asset], error] {
	ctx = requestContext(ctx)

	return func(yield func(*models.ListResponse[models.Asset], error) bool) {
		current := opts
		if current.Page == 0 {
			current.Page = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListAssets(ctx, organizationID, ledgerID, current)
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

	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
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

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "asset validation failed", err)
	}

	url := e.buildURL(organizationID, ledgerID, "")

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var asset models.Asset
	if err := e.httpClient.sendRequest(req, &asset); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		e.httpClient.emitBusinessError(ctx, businessEventAssetCreated, map[string]any{businessFieldOperation: operation, businessFieldOrganizationID: organizationID, businessFieldLedgerID: ledgerID}, err)

		return nil, err
	}

	e.httpClient.emitBusinessEvent(ctx, businessEventAssetCreated, map[string]any{businessFieldOperation: operation, businessFieldOrganizationID: organizationID, businessFieldLedgerID: ledgerID, businessFieldAssetID: asset.ID, businessFieldStatus: asset.Status.Code})

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

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "asset validation failed", err)
	}

	url := e.buildURL(organizationID, ledgerID, id)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var asset models.Asset
	if err := e.httpClient.sendRequest(req, &asset); err != nil {
		e.httpClient.emitBusinessError(ctx, businessEventAssetUpdated, map[string]any{businessFieldOperation: operation, businessFieldOrganizationID: organizationID, businessFieldLedgerID: ledgerID, businessFieldAssetID: id}, err)

		return nil, err
	}

	e.httpClient.emitBusinessEvent(ctx, businessEventAssetUpdated, map[string]any{businessFieldOperation: operation, businessFieldOrganizationID: organizationID, businessFieldLedgerID: ledgerID, businessFieldAssetID: asset.ID, businessFieldStatus: asset.Status.Code})

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

	req, err := newRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if err := e.httpClient.sendRequest(req, nil); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		e.httpClient.emitBusinessError(ctx, businessEventAssetDeleted, map[string]any{businessFieldOperation: operation, businessFieldOrganizationID: organizationID, businessFieldLedgerID: ledgerID, businessFieldAssetID: id}, err)

		return err
	}

	e.httpClient.emitBusinessEvent(ctx, businessEventAssetDeleted, map[string]any{businessFieldOperation: operation, businessFieldOrganizationID: organizationID, businessFieldLedgerID: ledgerID, businessFieldAssetID: id})

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

	count, err := e.httpClient.doCountRequest(ctx, countRequestMethod(), url, countRequestHeaders())
	if err != nil {
		return nil, err
	}

	return &models.MetricsCount{AssetsCount: count}, nil
}

// buildURL builds the URL for assets API calls.
// The organizationID and ledgerID parameters specify which organization and ledger to query.
// The assetID parameter is the unique identifier of the asset to retrieve, or an empty string for a list of assets.
// Returns the built URL.
func (e *assetsEntity) buildURL(organizationID, ledgerID, assetID string) string {
	baseURL := e.baseURLs["onboarding"]

	// Ensure the base URL doesn't end with a trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	if assetID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/assets", baseURL, pathSegment(organizationID), pathSegment(ledgerID))
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/assets/%s", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(assetID))
}

// buildMetricsURL builds the URL for assets metrics API calls.
func (e *assetsEntity) buildMetricsURL(organizationID, ledgerID string) string {
	baseURL := e.baseURLs["onboarding"]
	baseURL = strings.TrimSuffix(baseURL, "/")

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/assets/metrics/count", baseURL, pathSegment(organizationID), pathSegment(ledgerID))
}

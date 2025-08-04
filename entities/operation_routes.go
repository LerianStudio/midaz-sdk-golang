package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/errors"
)

// OperationRoutesService defines the interface for operation route operations
type OperationRoutesService interface {
	// ListOperationRoutes retrieves a paginated list of operation routes for a specific ledger
	//
	// Parameters:
	//   - ctx: Context for the request
	//   - organizationID: The unique identifier of the organization
	//   - ledgerID: The unique identifier of the ledger
	//   - opts: Optional parameters for pagination and filtering
	//
	// Returns:
	//   - *models.ListResponse[models.OperationRoute]: A paginated list of operation routes
	//   - error: An error if the request fails
	//
	// Example:
	//   opts := &models.ListOptions{
	//       Limit: 10,
	//       SortOrder: "asc",
	//   }
	//   routes, err := client.OperationRoutes.ListOperationRoutes(ctx, "org-123", "ledger-456", opts)
	ListOperationRoutes(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.OperationRoute], error)

	// GetOperationRoute retrieves a specific operation route by ID
	//
	// Parameters:
	//   - ctx: Context for the request
	//   - organizationID: The unique identifier of the organization
	//   - ledgerID: The unique identifier of the ledger
	//   - operationRouteID: The unique identifier of the operation route
	//
	// Returns:
	//   - *models.OperationRoute: The operation route details
	//   - error: An error if the request fails
	//
	// Example:
	//   route, err := client.OperationRoutes.GetOperationRoute(ctx, "org-123", "ledger-456", "route-789")
	GetOperationRoute(ctx context.Context, organizationID, ledgerID, operationRouteID string) (*models.OperationRoute, error)

	// CreateOperationRoute creates a new operation route
	//
	// Parameters:
	//   - ctx: Context for the request
	//   - organizationID: The unique identifier of the organization
	//   - ledgerID: The unique identifier of the ledger
	//   - input: The operation route creation data
	//
	// Returns:
	//   - *models.OperationRoute: The created operation route
	//   - error: An error if the request fails
	//
	// Example:
	//   input := models.NewCreateOperationRouteInput("Cash-in Route", "Handles cash-in operations", "source").
	//       WithMetadata(map[string]any{"department": "finance"})
	//   route, err := client.OperationRoutes.CreateOperationRoute(ctx, "org-123", "ledger-456", input)
	CreateOperationRoute(ctx context.Context, organizationID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error)

	// UpdateOperationRoute updates an existing operation route
	//
	// Parameters:
	//   - ctx: Context for the request
	//   - organizationID: The unique identifier of the organization
	//   - ledgerID: The unique identifier of the ledger
	//   - operationRouteID: The unique identifier of the operation route
	//   - input: The operation route update data
	//
	// Returns:
	//   - *models.OperationRoute: The updated operation route
	//   - error: An error if the request fails
	//
	// Example:
	//   input := models.NewUpdateOperationRouteInput().
	//       WithTitle("Updated Cash-in Route").
	//       WithDescription("Updated description")
	//   route, err := client.OperationRoutes.UpdateOperationRoute(ctx, "org-123", "ledger-456", "route-789", input)
	UpdateOperationRoute(ctx context.Context, organizationID, ledgerID, operationRouteID string, input *models.UpdateOperationRouteInput) (*models.OperationRoute, error)

	// DeleteOperationRoute deletes an operation route
	//
	// Parameters:
	//   - ctx: Context for the request
	//   - organizationID: The unique identifier of the organization
	//   - ledgerID: The unique identifier of the ledger
	//   - operationRouteID: The unique identifier of the operation route
	//
	// Returns:
	//   - error: An error if the request fails
	//
	// Example:
	//   err := client.OperationRoutes.DeleteOperationRoute(ctx, "org-123", "ledger-456", "route-789")
	DeleteOperationRoute(ctx context.Context, organizationID, ledgerID, operationRouteID string) error
}

// operationRoutesEntity implements the OperationRoutesService interface
type operationRoutesEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

// NewOperationRoutesEntity creates a new OperationRoutesService instance
func NewOperationRoutesEntity(client *http.Client, authToken string, baseURLs map[string]string) OperationRoutesService {
	httpClient := NewHTTPClient(client, authToken, nil)

	if debugEnv := os.Getenv("MIDAZ_DEBUG"); debugEnv == "true" {
		httpClient.debug = true
	}

	return &operationRoutesEntity{
		httpClient: httpClient,
		baseURLs:   baseURLs,
	}
}

// buildURL constructs the URL for operation route endpoints
func (e *operationRoutesEntity) buildURL(organizationID, ledgerID, operationRouteID string) string {
	baseURL := e.baseURLs["transaction"]

	if operationRouteID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/operation-routes", baseURL, organizationID, ledgerID)
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/operation-routes/%s", baseURL, organizationID, ledgerID, operationRouteID)
}

// ListOperationRoutes retrieves a paginated list of operation routes
func (e *operationRoutesEntity) ListOperationRoutes(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.OperationRoute], error) {
	operation := "ListOperationRoutes"

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

	if opts != nil {
		q := req.URL.Query()
		for key, value := range opts.ToQueryParams() {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	var result models.ListResponse[models.OperationRoute]
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetOperationRoute retrieves a specific operation route by ID
func (e *operationRoutesEntity) GetOperationRoute(ctx context.Context, organizationID, ledgerID, operationRouteID string) (*models.OperationRoute, error) {
	operation := "GetOperationRoute"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}
	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}
	if operationRouteID == "" {
		return nil, errors.NewMissingParameterError(operation, "operationRouteID")
	}

	url := e.buildURL(organizationID, ledgerID, operationRouteID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var result models.OperationRoute
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CreateOperationRoute creates a new operation route
func (e *operationRoutesEntity) CreateOperationRoute(ctx context.Context, organizationID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
	operation := "CreateOperationRoute"

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
		return nil, errors.NewValidationError(operation, "operation route validation failed", err)
	}

	url := e.buildURL(organizationID, ledgerID, "")

	if e.httpClient.debug {
		fmt.Printf("DEBUG [%s]: Creating operation route with input: %+v\n", operation, input)
	}

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var result models.OperationRoute
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// UpdateOperationRoute updates an existing operation route
func (e *operationRoutesEntity) UpdateOperationRoute(ctx context.Context, organizationID, ledgerID, operationRouteID string, input *models.UpdateOperationRouteInput) (*models.OperationRoute, error) {
	operation := "UpdateOperationRoute"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}
	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}
	if operationRouteID == "" {
		return nil, errors.NewMissingParameterError(operation, "operationRouteID")
	}
	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "operation route validation failed", err)
	}

	url := e.buildURL(organizationID, ledgerID, operationRouteID)

	if e.httpClient.debug {
		fmt.Printf("DEBUG [%s]: Updating operation route %s with input: %+v\n", operation, operationRouteID, input)
	}

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var result models.OperationRoute
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteOperationRoute deletes an operation route
func (e *operationRoutesEntity) DeleteOperationRoute(ctx context.Context, organizationID, ledgerID, operationRouteID string) error {
	operation := "DeleteOperationRoute"

	if organizationID == "" {
		return errors.NewMissingParameterError(operation, "organizationID")
	}
	if ledgerID == "" {
		return errors.NewMissingParameterError(operation, "ledgerID")
	}
	if operationRouteID == "" {
		return errors.NewMissingParameterError(operation, "operationRouteID")
	}

	url := e.buildURL(organizationID, ledgerID, operationRouteID)

	if e.httpClient.debug {
		fmt.Printf("DEBUG [%s]: Deleting operation route %s\n", operation, operationRouteID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if err := e.httpClient.sendRequest(req, nil); err != nil {
		return err
	}

	return nil
}

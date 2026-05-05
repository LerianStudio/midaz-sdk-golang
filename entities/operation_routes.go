package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
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
	//   routes, err := c.Entity.OperationRoutes.ListOperationRoutes(ctx, "org-123", "ledger-456", opts)
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
	//   route, err := c.Entity.OperationRoutes.GetOperationRoute(ctx, "org-123", "ledger-456", "route-789")
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
	//   route, err := c.Entity.OperationRoutes.CreateOperationRoute(ctx, "org-123", "ledger-456", input)
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
	//   route, err := c.Entity.OperationRoutes.UpdateOperationRoute(ctx, "org-123", "ledger-456", "route-789", input)
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
	//   err := c.Entity.OperationRoutes.DeleteOperationRoute(ctx, "org-123", "ledger-456", "route-789")
	DeleteOperationRoute(ctx context.Context, organizationID, ledgerID, operationRouteID string) error
}

// operationRoutesEntity implements the OperationRoutesService interface
type operationRoutesEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

func (e *operationRoutesEntity) setDefaultTenantID(tenantID string) {
	e.httpClient.SetTenantID(tenantID)
}

// newOperationRoutesEntity creates a new OperationRoutesService instance
func newOperationRoutesEntity(client *http.Client, authToken string, baseURLs map[string]string) OperationRoutesService {
	httpClient := NewHTTPClient(client, authToken, nil)

	return &operationRoutesEntity{
		httpClient: httpClient,
		baseURLs:   prepareServiceBaseURLs(baseURLs),
	}
}

// buildURL constructs the URL for operation route endpoints
func (e *operationRoutesEntity) buildURL(organizationID, ledgerID, operationRouteID string) string {
	if operationRouteID == "" {
		return buildLedgerScopedURL(e.baseURLs["transaction"], organizationID, ledgerID, "operation-routes")
	}

	return buildLedgerScopedURL(e.baseURLs["transaction"], organizationID, ledgerID, "operation-routes", operationRouteID)
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

	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if opts != nil {
		q := req.URL.Query()
		for key, value := range cursorListQueryParams(opts) {
			q.Add(key, value)
		}

		req.URL.RawQuery = q.Encode()
	}

	var result models.ListResponse[models.OperationRoute]
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	if result.Items == nil {
		result.Items = []models.OperationRoute{}
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

	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
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

	e.httpClient.debugLog("[%s]: Creating operation route", operation)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
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

	e.httpClient.debugLog("[%s]: Updating operation route %s", operation, operationRouteID)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
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

	e.httpClient.debugLog("[%s]: Deleting operation route %s", operation, operationRouteID)

	req, err := newRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	return e.httpClient.sendRequest(req, nil)
}

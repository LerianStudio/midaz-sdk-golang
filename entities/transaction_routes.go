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

// TransactionRoutesService defines the interface for transaction route operations
type TransactionRoutesService interface {
	// ListTransactionRoutes retrieves a paginated list of transaction routes for a specific ledger
	//
	// Parameters:
	//   - ctx: Context for the request
	//   - organizationID: The unique identifier of the organization
	//   - ledgerID: The unique identifier of the ledger
	//   - opts: Optional parameters for pagination and filtering
	//
	// Returns:
	//   - *models.ListResponse[models.TransactionRoute]: A paginated list of transaction routes
	//   - error: An error if the request fails
	//
	// Example:
	//   opts := &models.ListOptions{
	//       Limit: 10,
	//       SortOrder: "asc",
	//   }
	//   routes, err := client.TransactionRoutes.ListTransactionRoutes(ctx, "org-123", "ledger-456", opts)
	ListTransactionRoutes(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.TransactionRoute], error)

	// GetTransactionRoute retrieves a specific transaction route by ID
	//
	// Parameters:
	//   - ctx: Context for the request
	//   - organizationID: The unique identifier of the organization
	//   - ledgerID: The unique identifier of the ledger
	//   - transactionRouteID: The unique identifier of the transaction route
	//
	// Returns:
	//   - *models.TransactionRoute: The transaction route details
	//   - error: An error if the request fails
	//
	// Example:
	//   route, err := client.TransactionRoutes.GetTransactionRoute(ctx, "org-123", "ledger-456", "route-789")
	GetTransactionRoute(ctx context.Context, organizationID, ledgerID, transactionRouteID string) (*models.TransactionRoute, error)

	// CreateTransactionRoute creates a new transaction route
	//
	// Parameters:
	//   - ctx: Context for the request
	//   - organizationID: The unique identifier of the organization
	//   - ledgerID: The unique identifier of the ledger
	//   - input: The transaction route creation data
	//
	// Returns:
	//   - *models.TransactionRoute: The created transaction route
	//   - error: An error if the request fails
	//
	// Example:
	//   operationRoutes := []string{"route1-id", "route2-id"}
	//   input := models.NewCreateTransactionRouteInput("Settlement Route", "Handles settlements", operationRoutes).
	//       WithMetadata(map[string]any{"department": "finance"})
	//   route, err := client.TransactionRoutes.CreateTransactionRoute(ctx, "org-123", "ledger-456", input)
	CreateTransactionRoute(ctx context.Context, organizationID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error)

	// UpdateTransactionRoute updates an existing transaction route
	//
	// Parameters:
	//   - ctx: Context for the request
	//   - organizationID: The unique identifier of the organization
	//   - ledgerID: The unique identifier of the ledger
	//   - transactionRouteID: The unique identifier of the transaction route
	//   - input: The transaction route update data
	//
	// Returns:
	//   - *models.TransactionRoute: The updated transaction route
	//   - error: An error if the request fails
	//
	// Example:
	//   input := models.NewUpdateTransactionRouteInput().
	//       WithTitle("Updated Settlement Route").
	//       WithDescription("Updated description")
	//   route, err := client.TransactionRoutes.UpdateTransactionRoute(ctx, "org-123", "ledger-456", "route-789", input)
	UpdateTransactionRoute(ctx context.Context, organizationID, ledgerID, transactionRouteID string, input *models.UpdateTransactionRouteInput) (*models.TransactionRoute, error)

	// DeleteTransactionRoute deletes a transaction route
	//
	// Parameters:
	//   - ctx: Context for the request
	//   - organizationID: The unique identifier of the organization
	//   - ledgerID: The unique identifier of the ledger
	//   - transactionRouteID: The unique identifier of the transaction route
	//
	// Returns:
	//   - error: An error if the request fails
	//
	// Example:
	//   err := client.TransactionRoutes.DeleteTransactionRoute(ctx, "org-123", "ledger-456", "route-789")
	DeleteTransactionRoute(ctx context.Context, organizationID, ledgerID, transactionRouteID string) error
}

// transactionRoutesEntity implements the TransactionRoutesService interface
type transactionRoutesEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

// NewTransactionRoutesEntity creates a new TransactionRoutesService instance
func NewTransactionRoutesEntity(client *http.Client, authToken string, baseURLs map[string]string) TransactionRoutesService {
	httpClient := NewHTTPClient(client, authToken, nil)

	if debugEnv := os.Getenv(EnvMidazDebug); debugEnv == BoolTrue {
		httpClient.debug = true
	}

	return &transactionRoutesEntity{
		httpClient: httpClient,
		baseURLs:   baseURLs,
	}
}

// buildURL constructs the URL for transaction route endpoints
func (e *transactionRoutesEntity) buildURL(organizationID, ledgerID, transactionRouteID string) string {
	baseURL := e.baseURLs["transaction"]

	if transactionRouteID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/transaction-routes", baseURL, organizationID, ledgerID)
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/transaction-routes/%s", baseURL, organizationID, ledgerID, transactionRouteID)
}

// ListTransactionRoutes retrieves a paginated list of transaction routes
func (e *transactionRoutesEntity) ListTransactionRoutes(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.TransactionRoute], error) {
	operation := "ListTransactionRoutes"

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

	var result models.ListResponse[models.TransactionRoute]
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetTransactionRoute retrieves a specific transaction route by ID
func (e *transactionRoutesEntity) GetTransactionRoute(ctx context.Context, organizationID, ledgerID, transactionRouteID string) (*models.TransactionRoute, error) {
	operation := "GetTransactionRoute"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if transactionRouteID == "" {
		return nil, errors.NewMissingParameterError(operation, "transactionRouteID")
	}

	url := e.buildURL(organizationID, ledgerID, transactionRouteID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var result models.TransactionRoute
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CreateTransactionRoute creates a new transaction route
func (e *transactionRoutesEntity) CreateTransactionRoute(ctx context.Context, organizationID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
	operation := "CreateTransactionRoute"

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
		return nil, errors.NewValidationError(operation, "transaction route validation failed", err)
	}

	url := e.buildURL(organizationID, ledgerID, "")

	e.httpClient.debugLog("[%s]: Creating transaction route with input: %+v", operation, input)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var result models.TransactionRoute
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// UpdateTransactionRoute updates an existing transaction route
func (e *transactionRoutesEntity) UpdateTransactionRoute(ctx context.Context, organizationID, ledgerID, transactionRouteID string, input *models.UpdateTransactionRouteInput) (*models.TransactionRoute, error) {
	operation := "UpdateTransactionRoute"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if transactionRouteID == "" {
		return nil, errors.NewMissingParameterError(operation, "transactionRouteID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "transaction route validation failed", err)
	}

	url := e.buildURL(organizationID, ledgerID, transactionRouteID)

	e.httpClient.debugLog("[%s]: Updating transaction route %s with input: %+v", operation, transactionRouteID, input)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var result models.TransactionRoute
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteTransactionRoute deletes a transaction route
func (e *transactionRoutesEntity) DeleteTransactionRoute(ctx context.Context, organizationID, ledgerID, transactionRouteID string) error {
	operation := "DeleteTransactionRoute"

	if organizationID == "" {
		return errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return errors.NewMissingParameterError(operation, "ledgerID")
	}

	if transactionRouteID == "" {
		return errors.NewMissingParameterError(operation, "transactionRouteID")
	}

	url := e.buildURL(organizationID, ledgerID, transactionRouteID)

	e.httpClient.debugLog("[%s]: Deleting transaction route %s", operation, transactionRouteID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if err := e.httpClient.sendRequest(req, nil); err != nil {
		return err
	}

	return nil
}

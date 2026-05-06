package entities

//go:generate mockgen -source=transaction_routes.go -destination=mocks/mock_transaction_routes.go -package=mocks TransactionRoutesService

import (
	"bytes"
	"context"
	"encoding/json"
	"iter"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// TransactionRoutesService defines the interface for transaction route operations
type TransactionRoutesService interface {
	// ListTransactionRoutes retrieves a paginated page of transaction routes for a specific ledger.
	// This endpoint uses cursor-based pagination; advance pages by reading
	// page.Pagination.NextCursor and assigning it to opts.Cursor.
	//
	// Parameters:
	//   - ctx: Context for the request
	//   - organizationID: The unique identifier of the organization
	//   - ledgerID: The unique identifier of the ledger
	//   - opts: Typed cursor list options. Limit caps the page size; Filters narrow results.
	//
	// Returns:
	//   - *models.ListResponse[models.TransactionRoute]: A page of transaction routes
	//   - error: An error if the request fails. Validation errors return *errors.Error
	//     (category validation) before any HTTP request is sent.
	//
	// Example:
	//
	//	opts := models.TransactionRoutesListOpts{
	//	    CursorListOpts: models.CursorListOpts{Limit: 10, SortDirection: models.SortAsc},
	//	    Filters: models.TransactionRoutesFilters{Status: "ACTIVE"},
	//	}
	//	routes, err := c.Entity.TransactionRoutes.ListTransactionRoutes(ctx, "org-123", "ledger-456", opts)
	ListTransactionRoutes(ctx context.Context, organizationID, ledgerID string, opts models.TransactionRoutesListOpts) (*models.ListResponse[models.TransactionRoute], error)

	// ListTransactionRoutesAll returns an iter.Seq2 that yields each TransactionRoute
	// across every page until the cursor is exhausted or the context is cancelled.
	// Idiomatic v3 iteration:
	//
	//	for route, err := range c.Entity.TransactionRoutes.ListTransactionRoutesAll(ctx, orgID, ledgerID, opts) {
	//	    if err != nil { return err }
	//	    process(route)
	//	}
	ListTransactionRoutesAll(ctx context.Context, organizationID, ledgerID string, opts models.TransactionRoutesListOpts) iter.Seq2[models.TransactionRoute, error]

	// ListTransactionRoutesPages returns an iter.Seq2 that yields each *ListResponse
	// page. Use this when you need page-level metadata (Pagination, ItemCount) rather
	// than flattened items.
	ListTransactionRoutesPages(ctx context.Context, organizationID, ledgerID string, opts models.TransactionRoutesListOpts) iter.Seq2[*models.ListResponse[models.TransactionRoute], error]

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
	//   route, err := c.Entity.TransactionRoutes.GetTransactionRoute(ctx, "org-123", "ledger-456", "route-789")
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
	//   route, err := c.Entity.TransactionRoutes.CreateTransactionRoute(ctx, "org-123", "ledger-456", input)
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
	//   route, err := c.Entity.TransactionRoutes.UpdateTransactionRoute(ctx, "org-123", "ledger-456", "route-789", input)
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
	//   err := c.Entity.TransactionRoutes.DeleteTransactionRoute(ctx, "org-123", "ledger-456", "route-789")
	DeleteTransactionRoute(ctx context.Context, organizationID, ledgerID, transactionRouteID string) error
}

// transactionRoutesEntity implements the TransactionRoutesService interface
type transactionRoutesEntity struct {
	serviceEntity
}

// newTransactionRoutesEntity creates a new TransactionRoutesService instance
func newTransactionRoutesEntity(client *http.Client, authToken string, baseURLs map[string]string) TransactionRoutesService {
	return &transactionRoutesEntity{serviceEntity: newServiceEntity(client, authToken, baseURLs)}
}

// buildURL constructs the URL for transaction route endpoints
func (e *transactionRoutesEntity) buildURL(organizationID, ledgerID, transactionRouteID string) string {
	if transactionRouteID == "" {
		return buildLedgerScopedURL(e.baseURLs["transaction"], organizationID, ledgerID, "transaction-routes")
	}

	return buildLedgerScopedURL(e.baseURLs["transaction"], organizationID, ledgerID, "transaction-routes", transactionRouteID)
}

// ListTransactionRoutes retrieves a paginated list of transaction routes
func (e *transactionRoutesEntity) ListTransactionRoutes(ctx context.Context, organizationID, ledgerID string, opts models.TransactionRoutesListOpts) (*models.ListResponse[models.TransactionRoute], error) {
	operation := "ListTransactionRoutes"

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

	if params := opts.ToQueryParams(); len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
			q.Add(key, value)
		}

		req.URL.RawQuery = q.Encode()
	}

	var result models.ListResponse[models.TransactionRoute]
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	if result.Items == nil {
		result.Items = []models.TransactionRoute{}
	}

	return &result, nil
}

// ListTransactionRoutesAll yields every transactionroute matching the request,
// transparently advancing pagination via the server-issued NextCursor.
func (e *transactionRoutesEntity) ListTransactionRoutesAll(ctx context.Context, organizationID, ledgerID string, opts models.TransactionRoutesListOpts) iter.Seq2[models.TransactionRoute, error] {
	return flattenPages(e.ListTransactionRoutesPages(ctx, organizationID, ledgerID, opts))
}

// ListTransactionRoutesPages yields one full *ListResponse[TransactionRoute] per page,
// transparently advancing pagination via the server-issued NextCursor.
func (e *transactionRoutesEntity) ListTransactionRoutesPages(ctx context.Context, organizationID, ledgerID string, opts models.TransactionRoutesListOpts) iter.Seq2[*models.ListResponse[models.TransactionRoute], error] {
	return func(yield func(*models.ListResponse[models.TransactionRoute], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListTransactionRoutes(ctx, organizationID, ledgerID, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			next := page.Pagination.NextCursor
			if next == "" {
				return
			}

			current.Cursor = next
		}
	}
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

	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
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

	e.httpClient.debugLog("[%s]: Creating transaction route", operation)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
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

	e.httpClient.debugLog("[%s]: Updating transaction route %s", operation, transactionRouteID)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
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

	req, err := newRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	return e.httpClient.sendRequest(req, nil)
}

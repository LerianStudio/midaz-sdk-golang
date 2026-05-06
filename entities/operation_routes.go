package entities

//go:generate mockgen -source=operation_routes.go -destination=mocks/mock_operation_routes.go -package=mocks OperationRoutesService

import (
	"bytes"
	"context"
	"encoding/json"
	"iter"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// OperationRoutesService defines the interface for operation route operations
type OperationRoutesService interface {
	// ListOperationRoutes retrieves a paginated page of operation routes for a specific ledger.
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
	//   - *models.ListResponse[models.OperationRoute]: A page of operation routes
	//   - error: An error if the request fails. Validation errors return *errors.Error
	//     (category validation) before any HTTP request is sent.
	//
	// Example:
	//
	//	opts := models.OperationRoutesListOpts{
	//	    CursorListOpts: models.CursorListOpts{Limit: 10, SortDirection: models.SortAsc},
	//	    Filters: models.OperationRoutesFilters{OperationType: "credit"},
	//	}
	//	routes, err := c.Entity.OperationRoutes.ListOperationRoutes(ctx, "org-123", "ledger-456", opts)
	ListOperationRoutes(ctx context.Context, organizationID, ledgerID string, opts models.OperationRoutesListOpts) (*models.ListResponse[models.OperationRoute], error)

	// ListOperationRoutesAll returns an iter.Seq2 that yields each OperationRoute
	// across every page until the cursor is exhausted or the context is cancelled.
	// Idiomatic v3 iteration:
	//
	//	for route, err := range c.Entity.OperationRoutes.ListOperationRoutesAll(ctx, orgID, ledgerID, opts) {
	//	    if err != nil { return err }
	//	    process(route)
	//	}
	ListOperationRoutesAll(ctx context.Context, organizationID, ledgerID string, opts models.OperationRoutesListOpts) iter.Seq2[models.OperationRoute, error]

	// ListOperationRoutesPages returns an iter.Seq2 that yields each *ListResponse
	// page. Use this when you need page-level metadata (Pagination, ItemCount) rather
	// than flattened items.
	ListOperationRoutesPages(ctx context.Context, organizationID, ledgerID string, opts models.OperationRoutesListOpts) iter.Seq2[*models.ListResponse[models.OperationRoute], error]

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
	serviceEntity
}

// newOperationRoutesEntity creates a new OperationRoutesService instance
func newOperationRoutesEntity(client *http.Client, authToken string, baseURLs map[string]string) OperationRoutesService {
	return &operationRoutesEntity{serviceEntity: newServiceEntity(client, authToken, baseURLs)}
}

// buildURL constructs the URL for operation route endpoints
func (e *operationRoutesEntity) buildURL(organizationID, ledgerID, operationRouteID string) string {
	if operationRouteID == "" {
		return buildLedgerScopedURL(e.baseURLs["transaction"], organizationID, ledgerID, "operation-routes")
	}

	return buildLedgerScopedURL(e.baseURLs["transaction"], organizationID, ledgerID, "operation-routes", operationRouteID)
}

// ListOperationRoutes retrieves a paginated list of operation routes
func (e *operationRoutesEntity) ListOperationRoutes(ctx context.Context, organizationID, ledgerID string, opts models.OperationRoutesListOpts) (*models.ListResponse[models.OperationRoute], error) {
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

	if params := opts.ToQueryParams(); len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
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

// ListOperationRoutesAll yields every operationroute matching the request,
// transparently advancing pagination via the server-issued NextCursor.
func (e *operationRoutesEntity) ListOperationRoutesAll(ctx context.Context, organizationID, ledgerID string, opts models.OperationRoutesListOpts) iter.Seq2[models.OperationRoute, error] {
	return flattenPages(e.ListOperationRoutesPages(ctx, organizationID, ledgerID, opts))
}

// ListOperationRoutesPages yields one full *ListResponse[OperationRoute] per page,
// transparently advancing pagination via the server-issued NextCursor.
func (e *operationRoutesEntity) ListOperationRoutesPages(ctx context.Context, organizationID, ledgerID string, opts models.OperationRoutesListOpts) iter.Seq2[*models.ListResponse[models.OperationRoute], error] {
	ctx = requestContext(ctx)

	return func(yield func(*models.ListResponse[models.OperationRoute], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListOperationRoutes(ctx, organizationID, ledgerID, current)
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

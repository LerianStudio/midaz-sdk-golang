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

// PortfoliosService defines the interface for portfolio-related operations.
// It provides methods to create, read, update, and delete portfolios
// within a ledger and organization.
type PortfoliosService interface {
	// ListPortfolios retrieves a paginated list of portfolios for a ledger with optional filters.
	// The organizationID and ledgerID parameters specify which organization and ledger to query.
	// The opts parameter can be used to specify pagination, sorting, and filtering options.
	// Returns a ListResponse containing the portfolios and pagination information, or an error if the operation fails.
	ListPortfolios(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Portfolio], error)

	// GetPortfolio retrieves a specific portfolio by its ID.
	// The organizationID and ledgerID parameters specify which organization and ledger the portfolio belongs to.
	// The id parameter is the unique identifier of the portfolio to retrieve.
	// Returns the portfolio if found, or an error if the operation fails or the portfolio doesn't exist.
	GetPortfolio(ctx context.Context, organizationID, ledgerID, id string) (*models.Portfolio, error)

	// CreatePortfolio creates a new portfolio in the specified ledger.
	//
	// Portfolios are collections of accounts that belong to a specific entity within
	// an organization and ledger. They help organize accounts for better management
	// and reporting.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization where the portfolio will be created.
	//     Must be a valid organization ID.
	//   - ledgerID: The ID of the ledger where the portfolio will be created.
	//     Must be a valid ledger ID within the specified organization.
	//   - input: The portfolio details, including required fields:
	//     - EntityID: The ID of the entity that will own this portfolio
	//     - Name: The human-readable name of the portfolio
	//     Optional fields include:
	//     - Status: The initial status (defaults to ACTIVE if not specified)
	//     - Metadata: Additional custom information about the portfolio
	//
	// Returns:
	//   - *models.Portfolio: The created portfolio if successful, containing the portfolio ID,
	//     name, entity ID, and other properties.
	//   - error: An error if the operation fails. Possible errors include:
	//     - Invalid input (missing required fields or invalid values)
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Resource not found (invalid organization or ledger ID)
	//     - Network or server errors
	//
	// Example - Creating a basic portfolio:
	//
	//	// Create a simple portfolio with just required fields
	//	portfolio, err := portfoliosService.CreatePortfolio(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    models.NewCreatePortfolioInput(
	//	        "entity-789",
	//	        "Investment Portfolio",
	//	    ),
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to create portfolio: %v", err)
	//	}

	//
	//	// Use the portfolio
	//	fmt.Printf("Portfolio created: %s (status: %s)\n",
	//	    portfolio.ID, portfolio.Status.Code)
	//
	// Example - Creating a portfolio with metadata:
	//
	//	// Create a portfolio with custom metadata
	//	input := models.NewCreatePortfolioInput(
	//	    "entity-789",
	//	    "Retirement Portfolio",
	//	).WithStatus(
	//	    models.StatusActive,
	//	).WithMetadata(
	//	    map[string]any{
	//	        "portfolioType": "retirement",
	//	        "riskProfile": "moderate",
	//	        "targetYear": 2045,
	//	        "manager": "Jane Smith",
	//	    },
	//	)
	//
	//	portfolio, err := portfoliosService.CreatePortfolio(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    input,
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to create portfolio: %v", err)
	//	}
	//
	//	// Use the portfolio
	//	fmt.Printf("Portfolio created: %s\n", portfolio.ID)
	//	fmt.Printf("Name: %s\n", portfolio.Name)
	//	fmt.Printf("Entity: %s\n", portfolio.EntityID)
	CreatePortfolio(ctx context.Context, organizationID, ledgerID string, input *models.CreatePortfolioInput) (*models.Portfolio, error)

	// UpdatePortfolio updates an existing portfolio.
	// The organizationID and ledgerID parameters specify which organization and ledger the portfolio belongs to.
	// The id parameter is the unique identifier of the portfolio to update.
	// The input parameter contains the portfolio details to update, such as name, description, or status.
	// Returns the updated portfolio, or an error if the operation fails.
	UpdatePortfolio(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdatePortfolioInput) (*models.Portfolio, error)

	// DeletePortfolio deletes a portfolio.
	// The organizationID and ledgerID parameters specify which organization and ledger the portfolio belongs to.
	// The id parameter is the unique identifier of the portfolio to delete.
	// Returns an error if the operation fails.
	DeletePortfolio(ctx context.Context, organizationID, ledgerID, id string) error

	// GetPortfoliosMetricsCount retrieves the count metrics for portfolios in a ledger.
	// The organizationID and ledgerID parameters specify which organization and ledger to get metrics for.
	// Returns the metrics count if successful, or an error if the operation fails.
	GetPortfoliosMetricsCount(ctx context.Context, organizationID, ledgerID string) (*models.MetricsCount, error)
}

// portfoliosEntity implements the PortfoliosService interface.
// It provides methods for creating, retrieving, updating, and deleting portfolios.
type portfoliosEntity struct {
	HTTPClient *HTTPClient
	baseURLs   map[string]string
}

// NewPortfoliosEntity creates a new portfolios entity.
// It initializes the HTTP client and base URLs for API requests.
func NewPortfoliosEntity(client *http.Client, authToken string, baseURLs map[string]string) PortfoliosService {
	// Create a new HTTP client with the shared implementation
	httpClient := NewHTTPClient(client, authToken, nil)

	// Check if we're using the debug flag from the environment
	if debugEnv := os.Getenv("MIDAZ_DEBUG"); debugEnv == "true" {
		httpClient.debug = true
	}

	return &portfoliosEntity{
		HTTPClient: httpClient,
		baseURLs:   baseURLs,
	}
}

// ListPortfolios lists portfolios for a ledger with optional filters.
func (e *portfoliosEntity) ListPortfolios(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Portfolio], error) {
	const operation = "ListPortfolios"

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

	var response models.ListResponse[models.Portfolio]
	if err := e.HTTPClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetPortfolio gets a portfolio by ID.
func (e *portfoliosEntity) GetPortfolio(ctx context.Context, organizationID, ledgerID, id string) (*models.Portfolio, error) {
	const operation = "GetPortfolio"

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

	var portfolio models.Portfolio
	if err := e.HTTPClient.sendRequest(req, &portfolio); err != nil {
		return nil, err
	}

	return &portfolio, nil
}

// CreatePortfolio creates a new portfolio in the specified ledger.
func (e *portfoliosEntity) CreatePortfolio(ctx context.Context, organizationID, ledgerID string, input *models.CreatePortfolioInput) (*models.Portfolio, error) {
	const operation = "CreatePortfolio"

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

	var portfolio models.Portfolio
	if err := e.HTTPClient.sendRequest(req, &portfolio); err != nil {
		return nil, err
	}

	return &portfolio, nil
}

// UpdatePortfolio updates an existing portfolio.
func (e *portfoliosEntity) UpdatePortfolio(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdatePortfolioInput) (*models.Portfolio, error) {
	const operation = "UpdatePortfolio"

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

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var portfolio models.Portfolio
	if err := e.HTTPClient.sendRequest(req, &portfolio); err != nil {
		return nil, err
	}

	return &portfolio, nil
}

// DeletePortfolio deletes a portfolio.
func (e *portfoliosEntity) DeletePortfolio(ctx context.Context, organizationID, ledgerID, id string) error {
	const operation = "DeletePortfolio"

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

	if err := e.HTTPClient.sendRequest(req, nil); err != nil {
		return err
	}

	return nil
}

// GetPortfoliosMetricsCount gets the count metrics for portfolios in a ledger.
func (e *portfoliosEntity) GetPortfoliosMetricsCount(ctx context.Context, organizationID, ledgerID string) (*models.MetricsCount, error) {
	const operation = "GetPortfoliosMetricsCount"

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
	if err := e.HTTPClient.sendRequest(req, &metrics); err != nil {
		return nil, err
	}

	return &metrics, nil
}

// buildURL builds the URL for portfolios API calls.
func (e *portfoliosEntity) buildURL(organizationID, ledgerID, portfolioID string) string {
	baseURL := e.baseURLs["onboarding"]

	if portfolioID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/portfolios", baseURL, organizationID, ledgerID)
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/portfolios/%s", baseURL, organizationID, ledgerID, portfolioID)
}

// buildMetricsURL builds the URL for portfolios metrics API calls.
func (e *portfoliosEntity) buildMetricsURL(organizationID, ledgerID string) string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/portfolios/metrics/count", baseURL, organizationID, ledgerID)
}

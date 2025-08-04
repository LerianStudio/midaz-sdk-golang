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

// OrganizationsService defines the interface for organization-related operations.
// It provides methods to create, read, update, and delete organizations
// in the Midaz platform.
type OrganizationsService interface {
	// ListOrganizations retrieves a paginated list of organizations with optional filters.
	// The opts parameter can be used to specify pagination, sorting, and filtering options.
	// Returns a ListResponse containing the organizations and pagination information, or an error if the operation fails.
	ListOrganizations(ctx context.Context, opts *models.ListOptions) (*models.ListResponse[models.Organization], error)

	// GetOrganization retrieves a specific organization by its ID.
	// The id parameter is the unique identifier of the organization to retrieve.
	// Returns the organization if found, or an error if the operation fails or the organization doesn't exist.
	GetOrganization(ctx context.Context, id string) (*models.Organization, error)

	// CreateOrganization creates a new organization.
	//
	// Organizations are the top-level entities in the Midaz system that own ledgers,
	// accounts, and other resources. Each organization has a legal identity and
	// can manage multiple ledgers.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - input: The organization details, including required fields:
	//     - LegalName: The official registered name of the organization
	//     - LegalDocument: The official identification document (e.g., tax ID)
	//     Optional fields include:
	//     - Status: The initial status (defaults to ACTIVE if not specified)
	//     - Address: The physical address of the organization
	//     - Metadata: Additional custom information about the organization
	//     - ParentOrganizationID: Reference to a parent organization, if applicable
	//     - DoingBusinessAs: Trading or brand name, if different from legal name
	//
	// Returns:
	//   - *models.Organization: The created organization if successful, containing the organization ID,
	//     legal name, status, and other properties.
	//   - error: An error if the operation fails. Possible errors include:
	//     - Invalid input (missing required fields or invalid values)
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Network or server errors
	//
	// Example - Creating a basic organization:
	//
	//	// Create a simple organization with just required fields
	//	organization, err := organizationsService.CreateOrganization(
	//	    context.Background(),
	//	    models.NewCreateOrganizationInput(
	//	        "Acme Corporation",
	//	        "123456789",
	//	    ),
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to create organization: %v", err)
	//	}
	//
	//	// Use the organization
	//	fmt.Printf("Organization created: %s (status: %s)\n",
	//	    organization.ID, organization.Status.Code)
	//
	// Example - Creating an organization with all options:
	//
	//	// Create an organization with all available options
	//	input := models.NewCreateOrganizationInput(
	//	    "Acme Corporation",
	//	    "123456789",
	//	).WithStatus(
	//	    models.StatusActive,
	//	).WithAddress(
	//	    models.Address{
	//	        Line1:      "123 Main Street",
	//	        City:       "San Francisco",
	//	        State:      "CA",
	//	        PostalCode: "94105",
	//	        Country:    "US",
	//	    },
	//	).WithMetadata(
	//	    map[string]any{
	//	        "industry": "Technology",
	//	        "founded": 2023,
	//	        "website": "https://acme.example.com",
	//	    },
	//	).WithDoingBusinessAs(
	//	    "Acme Tech",
	//	)
	//
	//	organization, err := organizationsService.CreateOrganization(
	//	    context.Background(),
	//	    input,
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to create organization: %v", err)
	//	}
	//
	//	// Use the organization
	//	fmt.Printf("Organization created: %s\n", organization.ID)
	//	fmt.Printf("Legal name: %s\n", organization.LegalName)
	//	if organization.DoingBusinessAs != nil {
	//	    fmt.Printf("DBA: %s\n", *organization.DoingBusinessAs)
	//	}
	CreateOrganization(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error)

	// UpdateOrganization updates an existing organization.
	// The id parameter is the unique identifier of the organization to update.
	// The input parameter contains the organization details to update, such as name, description, or status.
	// Returns the updated organization, or an error if the operation fails.
	UpdateOrganization(ctx context.Context, id string, input *models.UpdateOrganizationInput) (*models.Organization, error)

	// DeleteOrganization deletes an organization.
	// The id parameter is the unique identifier of the organization to delete.
	// Returns an error if the operation fails.
	DeleteOrganization(ctx context.Context, id string) error

	// GetOrganizationsMetricsCount retrieves the count metrics for organizations.
	// This method returns aggregate statistics about the number of organizations in the system.
	// Returns the metrics count if successful, or an error if the operation fails.
	GetOrganizationsMetricsCount(ctx context.Context) (*models.MetricsCount, error)
}

// organizationsEntity implements the OrganizationsService interface.
// It handles the communication with the Midaz API for organization-related operations.
type organizationsEntity struct {
	HTTPClient *HTTPClient
	baseURLs   map[string]string
}

// NewOrganizationsEntity creates a new organizations entity.
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
//   - OrganizationsService: An implementation of the OrganizationsService interface that provides
//     methods for creating, retrieving, updating, and managing organizations.
//
// Example:
//
//	// Create an organizations entity with default HTTP client
//	organizationsEntity := entities.NewOrganizationsEntity(
//	    &http.Client{Timeout: 30 * time.Second},
//	    "your-auth-token",
//	    map[string]string{"onboarding": "https://api.midaz.io/v1"},
//	)
//
//	// Use the entity to retrieve organizations
//	organizations, err := organizationsEntity.ListOrganizations(
//	    context.Background(),
//	    nil, // No pagination options
//	)
//
//	if err != nil {
//	    log.Fatalf("Failed to retrieve organizations: %v", err)
//	}
//
//	fmt.Printf("Retrieved %d organizations\n", len(organizations.Items))
func NewOrganizationsEntity(client *http.Client, authToken string, baseURLs map[string]string) OrganizationsService {
	// Create a new HTTP client with the shared implementation
	httpClient := NewHTTPClient(client, authToken, nil)

	// Check if we're using the debug flag from the environment
	if debugEnv := os.Getenv("MIDAZ_DEBUG"); debugEnv == "true" {
		httpClient.debug = true
	}

	return &organizationsEntity{
		HTTPClient: httpClient,
		baseURLs:   baseURLs,
	}
}

// ListOrganizations lists organizations with optional filters.
func (e *organizationsEntity) ListOrganizations(ctx context.Context, opts *models.ListOptions) (*models.ListResponse[models.Organization], error) {
	const operation = "ListOrganizations"
	url := e.buildURL("")

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

	var response models.ListResponse[models.Organization]
	if err := e.HTTPClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetOrganization gets an organization by ID.
func (e *organizationsEntity) GetOrganization(ctx context.Context, id string) (*models.Organization, error) {
	const operation = "GetOrganization"

	if id == "" {
		return nil, errors.NewMissingParameterError(operation, "id")
	}

	url := e.buildURL(id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var organization models.Organization
	if err := e.HTTPClient.sendRequest(req, &organization); err != nil {
		return nil, err
	}

	return &organization, nil
}

// CreateOrganization creates a new organization.
//
// Organizations are the top-level entities in the Midaz system that own ledgers,
// accounts, and other resources. Each organization has a legal identity and
// can manage multiple ledgers.
func (e *organizationsEntity) CreateOrganization(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error) {
	const operation = "CreateOrganization"

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if input.LegalName == "" {
		return nil, errors.NewValidationError(operation, "legal name is required", nil)
	}

	if input.LegalDocument == "" {
		return nil, errors.NewValidationError(operation, "legal document is required", nil)
	}

	url := e.buildURL("")

	// Convert the input to the mmodel format using our utility
	mmodelInput := input.ToMmodelCreateOrganizationInput()

	// Using structured logging would be beneficial here to debug conversions
	if e.HTTPClient.debug {
		fmt.Printf("DEBUG [%s]: Converting SDK input to backend format\n", operation)
		fmt.Printf("DEBUG [%s]: Original: %+v\n", operation, input)
		fmt.Printf("DEBUG [%s]: Converted: %+v\n", operation, mmodelInput)
	}

	// Marshal the input to JSON
	body, err := json.Marshal(mmodelInput)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var organization models.Organization
	if err := e.HTTPClient.sendRequest(req, &organization); err != nil {
		return nil, err
	}

	return &organization, nil
}

// UpdateOrganization updates an existing organization.
func (e *organizationsEntity) UpdateOrganization(ctx context.Context, id string, input *models.UpdateOrganizationInput) (*models.Organization, error) {
	const operation = "UpdateOrganization"

	if id == "" {
		return nil, errors.NewMissingParameterError(operation, "id")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	url := e.buildURL(id)

	// Convert the input to the mmodel format
	mmodelInput := input.ToMmodelUpdateOrganizationInput()

	// Marshal the input to JSON
	body, err := json.Marshal(mmodelInput)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var organization models.Organization
	if err := e.HTTPClient.sendRequest(req, &organization); err != nil {
		return nil, err
	}

	return &organization, nil
}

// DeleteOrganization deletes an organization.
func (e *organizationsEntity) DeleteOrganization(ctx context.Context, id string) error {
	const operation = "DeleteOrganization"

	if id == "" {
		return errors.NewMissingParameterError(operation, "id")
	}

	url := e.buildURL(id)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if err := e.HTTPClient.sendRequest(req, nil); err != nil {
		return err
	}

	return nil
}

// GetOrganizationsMetricsCount gets the count metrics for organizations.
func (e *organizationsEntity) GetOrganizationsMetricsCount(ctx context.Context) (*models.MetricsCount, error) {
	const operation = "GetOrganizationsMetricsCount"

	url := e.buildMetricsURL()

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

// buildURL builds the URL for organizations API calls.
func (e *organizationsEntity) buildURL(id string) string {
	baseURL := e.baseURLs["onboarding"]

	if id == "" {
		return fmt.Sprintf("%s/organizations", baseURL)
	}

	return fmt.Sprintf("%s/organizations/%s", baseURL, id)
}

// buildMetricsURL builds the URL for organizations metrics API calls.
func (e *organizationsEntity) buildMetricsURL() string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/metrics/count", baseURL)
}

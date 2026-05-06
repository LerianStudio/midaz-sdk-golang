package entities

//go:generate mockgen -source=organizations.go -destination=mocks/mock_organizations.go -package=mocks OrganizationsService

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// OrganizationsService defines the interface for organization-related operations.
// It provides methods to create, read, update, and delete organizations
// in the Midaz platform.
type OrganizationsService interface {
	// ListOrganizations retrieves one page of organizations.
	ListOrganizations(ctx context.Context, opts models.OrganizationsListOpts) (*models.ListResponse[models.Organization], error)

	// ListOrganizationsAll yields every organization, transparently advancing pagination.
	ListOrganizationsAll(ctx context.Context, opts models.OrganizationsListOpts) iter.Seq2[models.Organization, error]

	// ListOrganizationsPages yields one full *ListResponse[Organization] per page.
	ListOrganizationsPages(ctx context.Context, opts models.OrganizationsListOpts) iter.Seq2[*models.ListResponse[models.Organization], error]

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
	//	        ZipCode: "94105",
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
	serviceEntity
}

// newOrganizationsEntity wires the OrganizationsService backed by the shared HTTP transport.
// Internal: invoked by Entity.initServices; callers should reach the service via Client.Organizations.
func newOrganizationsEntity(client *http.Client, authToken string, baseURLs map[string]string) OrganizationsService {
	return &organizationsEntity{serviceEntity: newServiceEntity(client, authToken, baseURLs)}
}

// ListOrganizations lists one page of organizations.
func (e *organizationsEntity) ListOrganizations(ctx context.Context, opts models.OrganizationsListOpts) (*models.ListResponse[models.Organization], error) {
	const operation = "ListOrganizations"

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	url := e.buildURL("")

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

	var response models.ListResponse[models.Organization]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ListOrganizationsAll yields every organization, transparently advancing pagination.
func (e *organizationsEntity) ListOrganizationsAll(ctx context.Context, opts models.OrganizationsListOpts) iter.Seq2[models.Organization, error] {
	return flattenPages(e.ListOrganizationsPages(ctx, opts))
}

// ListOrganizationsPages yields one full *ListResponse[Organization] per page.
func (e *organizationsEntity) ListOrganizationsPages(ctx context.Context, opts models.OrganizationsListOpts) iter.Seq2[*models.ListResponse[models.Organization], error] {
	return func(yield func(*models.ListResponse[models.Organization], error) bool) {
		current := opts
		if current.Page <= 0 {
			current.Page = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListOrganizations(ctx, current)
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

// GetOrganization gets an organization by ID.
func (e *organizationsEntity) GetOrganization(ctx context.Context, id string) (*models.Organization, error) {
	const operation = "GetOrganization"

	if id == "" {
		return nil, errors.NewMissingParameterError(operation, "id")
	}

	url := e.buildURL(id)

	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var organization models.Organization
	if err := e.httpClient.sendRequest(req, &organization); err != nil {
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

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "organization validation failed", err)
	}

	url := e.buildURL("")

	e.httpClient.debugLog("[%s]: Converting SDK input to backend format", operation)
	e.httpClient.debugLog("[%s]: Payload: [REDACTED]", operation)

	// Marshal the input to JSON
	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var organization models.Organization
	if err := e.httpClient.sendRequest(req, &organization); err != nil {
		e.httpClient.emitBusinessError(ctx, businessEventOrganizationCreated, map[string]any{"operation": operation}, err)

		return nil, err
	}

	e.httpClient.emitBusinessEvent(ctx, businessEventOrganizationCreated, map[string]any{"operation": operation, "organizationId": organization.ID, "status": organization.Status.Code})

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

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "organization validation failed", err)
	}

	// Marshal the input to JSON
	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var organization models.Organization
	if err := e.httpClient.sendRequest(req, &organization); err != nil {
		e.httpClient.emitBusinessError(ctx, businessEventOrganizationUpdated, map[string]any{"operation": operation, "organizationId": id}, err)

		return nil, err
	}

	e.httpClient.emitBusinessEvent(ctx, businessEventOrganizationUpdated, map[string]any{"operation": operation, "organizationId": organization.ID, "status": organization.Status.Code})

	return &organization, nil
}

// DeleteOrganization deletes an organization.
func (e *organizationsEntity) DeleteOrganization(ctx context.Context, id string) error {
	const operation = "DeleteOrganization"

	if id == "" {
		return errors.NewMissingParameterError(operation, "id")
	}

	url := e.buildURL(id)

	req, err := newRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if err := e.httpClient.sendRequest(req, nil); err != nil {
		e.httpClient.emitBusinessError(ctx, businessEventOrganizationDeleted, map[string]any{"operation": operation, "organizationId": id}, err)

		return err
	}

	e.httpClient.emitBusinessEvent(ctx, businessEventOrganizationDeleted, map[string]any{"operation": operation, "organizationId": id})

	return nil
}

// GetOrganizationsMetricsCount gets the count metrics for organizations.
func (e *organizationsEntity) GetOrganizationsMetricsCount(ctx context.Context) (*models.MetricsCount, error) {
	url := e.buildMetricsURL()

	count, err := e.httpClient.doCountRequest(ctx, countRequestMethod(), url, countRequestHeaders())
	if err != nil {
		return nil, err
	}

	return &models.MetricsCount{OrganizationsCount: count}, nil
}

// buildURL builds the URL for organizations API calls.
func (e *organizationsEntity) buildURL(id string) string {
	baseURL := e.baseURLs["onboarding"]

	if id == "" {
		return fmt.Sprintf("%s/organizations", baseURL)
	}

	return fmt.Sprintf("%s/organizations/%s", baseURL, pathSegment(id))
}

// buildMetricsURL builds the URL for organizations metrics API calls.
func (e *organizationsEntity) buildMetricsURL() string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/metrics/count", baseURL)
}

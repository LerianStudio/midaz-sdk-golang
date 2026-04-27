package entities

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

// HoldersService defines CRM holder operations.
type HoldersService interface {
	// ListHolders retrieves holders for an organization.
	ListHolders(ctx context.Context, organizationID string, opts *models.ListOptions) (*models.ListResponse[models.Holder], error)
	// CreateHolder creates a holder.
	CreateHolder(ctx context.Context, organizationID string, input *models.CreateHolderInput) (*models.Holder, error)
	// GetHolder retrieves a holder by ID.
	GetHolder(ctx context.Context, organizationID, holderID string, includeDeleted bool) (*models.Holder, error)
	// UpdateHolder updates a holder by ID.
	UpdateHolder(ctx context.Context, organizationID, holderID string, input *models.UpdateHolderInput) (*models.Holder, error)
	// DeleteHolder deletes a holder by ID.
	DeleteHolder(ctx context.Context, organizationID, holderID string, hardDelete bool) error
}

type holdersEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

func (e *holdersEntity) setDefaultTenantID(tenantID string) {
	e.httpClient.SetTenantID(tenantID)
}

// NewHoldersEntity creates a new HoldersService instance.
func NewHoldersEntity(client *http.Client, authToken string, baseURLs map[string]string) HoldersService {
	httpClient := NewHTTPClient(client, authToken, nil)
	if debugEnv := os.Getenv(EnvMidazDebug); debugEnv == BoolTrue {
		httpClient.debug = true
	}

	return &holdersEntity{httpClient: httpClient, baseURLs: baseURLs}
}

// ListHolders retrieves holders for an organization.
func (e *holdersEntity) ListHolders(ctx context.Context, organizationID string, opts *models.ListOptions) (*models.ListResponse[models.Holder], error) {
	const operation = "ListHolders"
	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.buildURL(""), nil)
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

	var response models.ListResponse[models.Holder]
	if err := e.httpClient.doRequest(ctx, http.MethodGet, req.URL.String(), crmHeaders(organizationID), nil, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateHolder creates a holder.
func (e *holdersEntity) CreateHolder(ctx context.Context, organizationID string, input *models.CreateHolderInput) (*models.Holder, error) {
	const operation = "CreateHolder"
	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "holder validation failed", err)
	}

	var holder models.Holder
	if err := e.httpClient.doRequest(ctx, http.MethodPost, e.buildURL(""), crmHeaders(organizationID), input, &holder); err != nil {
		return nil, err
	}

	return &holder, nil
}

// GetHolder retrieves a holder by ID.
func (e *holdersEntity) GetHolder(ctx context.Context, organizationID, holderID string, includeDeleted bool) (*models.Holder, error) {
	const operation = "GetHolder"
	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if holderID == "" {
		return nil, errors.NewMissingParameterError(operation, "holderID")
	}

	endpoint := e.buildURL(holderID)
	if includeDeleted {
		endpoint += "?include_deleted=true"
	}

	var holder models.Holder
	if err := e.httpClient.doRequest(ctx, http.MethodGet, endpoint, crmHeaders(organizationID), nil, &holder); err != nil {
		return nil, err
	}

	return &holder, nil
}

// UpdateHolder updates a holder by ID.
func (e *holdersEntity) UpdateHolder(ctx context.Context, organizationID, holderID string, input *models.UpdateHolderInput) (*models.Holder, error) {
	const operation = "UpdateHolder"
	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if holderID == "" {
		return nil, errors.NewMissingParameterError(operation, "holderID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "holder validation failed", err)
	}

	endpoint := e.buildURL(holderID)

	var holder models.Holder
	if err := e.httpClient.doRequest(ctx, http.MethodPatch, endpoint, crmHeaders(organizationID), input, &holder); err != nil {
		return nil, err
	}

	return &holder, nil
}

// DeleteHolder deletes a holder by ID.
func (e *holdersEntity) DeleteHolder(ctx context.Context, organizationID, holderID string, hardDelete bool) error {
	const operation = "DeleteHolder"
	if organizationID == "" {
		return errors.NewMissingParameterError(operation, "organizationID")
	}

	if holderID == "" {
		return errors.NewMissingParameterError(operation, "holderID")
	}

	endpoint := e.buildURL(holderID)
	if hardDelete {
		endpoint += "?hard_delete=true"
	}

	return e.httpClient.doRequest(ctx, http.MethodDelete, endpoint, crmHeaders(organizationID), nil, nil)
}

func (e *holdersEntity) buildURL(holderID string) string {
	baseURL := e.baseURLs["crm"]
	if holderID == "" {
		return fmt.Sprintf("%s/holders", baseURL)
	}

	return fmt.Sprintf("%s/holders/%s", baseURL, pathSegment(holderID))
}

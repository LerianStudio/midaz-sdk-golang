package entities

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

// AliasesService defines CRM alias operations.
type AliasesService interface {
	// ListAliases retrieves aliases for an organization.
	ListAliases(ctx context.Context, organizationID string, opts *models.ListOptions) (*models.ListResponse[models.Alias], error)
	// CreateAlias creates an alias for a holder.
	CreateAlias(ctx context.Context, organizationID, holderID string, input *models.CreateAliasInput) (*models.Alias, error)
	// GetAlias retrieves an alias by ID.
	GetAlias(ctx context.Context, organizationID, holderID, aliasID string, includeDeleted bool) (*models.Alias, error)
	// UpdateAlias updates an alias by ID.
	UpdateAlias(ctx context.Context, organizationID, holderID, aliasID string, input *models.UpdateAliasInput) (*models.Alias, error)
	// DeleteAlias deletes an alias by ID.
	DeleteAlias(ctx context.Context, organizationID, holderID, aliasID string, hardDelete bool) error
	// DeleteRelatedParty deletes a related party from an alias.
	DeleteRelatedParty(ctx context.Context, organizationID, holderID, aliasID, relatedPartyID string) error
}

type aliasesEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

func (e *aliasesEntity) setDefaultTenantID(tenantID string) {
	e.httpClient.SetTenantID(tenantID)
}

// NewAliasesEntity creates a new AliasesService instance.
func NewAliasesEntity(client *http.Client, authToken string, baseURLs map[string]string) AliasesService {
	httpClient := NewHTTPClient(client, authToken, nil)
	if debugEnv := os.Getenv(EnvMidazDebug); debugEnv == BoolTrue {
		httpClient.debug = true
	}

	return &aliasesEntity{httpClient: httpClient, baseURLs: baseURLs}
}

// ListAliases retrieves aliases for an organization.
func (e *aliasesEntity) ListAliases(ctx context.Context, organizationID string, opts *models.ListOptions) (*models.ListResponse[models.Alias], error) {
	const operation = "ListAliases"
	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.listURL(), nil)
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

	var response models.ListResponse[models.Alias]
	if err := e.httpClient.doRequest(ctx, http.MethodGet, req.URL.String(), crmHeaders(organizationID), nil, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateAlias creates an alias for a holder.
func (e *aliasesEntity) CreateAlias(ctx context.Context, organizationID, holderID string, input *models.CreateAliasInput) (*models.Alias, error) {
	const operation = "CreateAlias"
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
		return nil, errors.NewValidationError(operation, "alias validation failed", err)
	}

	var alias models.Alias
	if err := e.httpClient.doRequest(ctx, http.MethodPost, e.aliasURL(holderID, ""), crmHeaders(organizationID), input, &alias); err != nil {
		return nil, err
	}

	return &alias, nil
}

// GetAlias retrieves an alias by ID.
func (e *aliasesEntity) GetAlias(ctx context.Context, organizationID, holderID, aliasID string, includeDeleted bool) (*models.Alias, error) {
	const operation = "GetAlias"
	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if holderID == "" {
		return nil, errors.NewMissingParameterError(operation, "holderID")
	}

	if aliasID == "" {
		return nil, errors.NewMissingParameterError(operation, "aliasID")
	}

	endpoint := e.aliasURL(holderID, aliasID)
	if includeDeleted {
		endpoint += "?include_deleted=true"
	}

	var alias models.Alias
	if err := e.httpClient.doRequest(ctx, http.MethodGet, endpoint, crmHeaders(organizationID), nil, &alias); err != nil {
		return nil, err
	}

	return &alias, nil
}

// UpdateAlias updates an alias by ID.
func (e *aliasesEntity) UpdateAlias(ctx context.Context, organizationID, holderID, aliasID string, input *models.UpdateAliasInput) (*models.Alias, error) {
	const operation = "UpdateAlias"
	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if holderID == "" {
		return nil, errors.NewMissingParameterError(operation, "holderID")
	}

	if aliasID == "" {
		return nil, errors.NewMissingParameterError(operation, "aliasID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "alias validation failed", err)
	}

	var alias models.Alias
	if err := e.httpClient.doRequest(ctx, http.MethodPatch, e.aliasURL(holderID, aliasID), crmHeaders(organizationID), input, &alias); err != nil {
		return nil, err
	}

	return &alias, nil
}

// DeleteAlias deletes an alias by ID.
func (e *aliasesEntity) DeleteAlias(ctx context.Context, organizationID, holderID, aliasID string, hardDelete bool) error {
	const operation = "DeleteAlias"
	if organizationID == "" {
		return errors.NewMissingParameterError(operation, "organizationID")
	}

	if holderID == "" {
		return errors.NewMissingParameterError(operation, "holderID")
	}

	if aliasID == "" {
		return errors.NewMissingParameterError(operation, "aliasID")
	}

	endpoint := e.aliasURL(holderID, aliasID)
	if hardDelete {
		endpoint += "?hard_delete=true"
	}

	return e.httpClient.doRequest(ctx, http.MethodDelete, endpoint, crmHeaders(organizationID), nil, nil)
}

// DeleteRelatedParty deletes a related party from an alias.
func (e *aliasesEntity) DeleteRelatedParty(ctx context.Context, organizationID, holderID, aliasID, relatedPartyID string) error {
	const operation = "DeleteRelatedParty"
	if organizationID == "" {
		return errors.NewMissingParameterError(operation, "organizationID")
	}

	if holderID == "" {
		return errors.NewMissingParameterError(operation, "holderID")
	}

	if aliasID == "" {
		return errors.NewMissingParameterError(operation, "aliasID")
	}

	if relatedPartyID == "" {
		return errors.NewMissingParameterError(operation, "relatedPartyID")
	}

	endpoint := fmt.Sprintf("%s/holders/%s/aliases/%s/related-parties/%s", e.baseURLs["crm"], pathSegment(holderID), pathSegment(aliasID), pathSegment(relatedPartyID))

	return e.httpClient.doRequest(ctx, http.MethodDelete, endpoint, crmHeaders(organizationID), nil, nil)
}

func (e *aliasesEntity) listURL() string {
	return fmt.Sprintf("%s/aliases", e.baseURLs["crm"])
}

func (e *aliasesEntity) aliasURL(holderID, aliasID string) string {
	baseURL := e.baseURLs["crm"]
	if aliasID == "" {
		return fmt.Sprintf("%s/holders/%s/aliases", baseURL, pathSegment(holderID))
	}

	return fmt.Sprintf("%s/holders/%s/aliases/%s", baseURL, pathSegment(holderID), pathSegment(aliasID))
}

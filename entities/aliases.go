package entities

import (
	"context"
	"fmt"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
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
	e.httpClient.setTenantIDLocked(tenantID)
}

// newAliasesEntity creates a new AliasesService instance.
func newAliasesEntity(client *http.Client, authToken string, baseURLs map[string]string) AliasesService {
	httpClient := NewHTTPClient(client, authToken, nil)
	return &aliasesEntity{httpClient: httpClient, baseURLs: prepareServiceBaseURLs(baseURLs)}
}

// ListAliases retrieves aliases for an organization.
func (e *aliasesEntity) ListAliases(ctx context.Context, organizationID string, opts *models.ListOptions) (*models.ListResponse[models.Alias], error) {
	const operation = "ListAliases"

	organizationID, err := validateCRMOrganizationID(operation, organizationID)
	if err != nil {
		return nil, err
	}

	req, err := newRequestWithContext(ctx, http.MethodGet, e.listURL(), nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	params, err := validatedAliasListQueryParams(operation, opts)
	if err != nil {
		return nil, err
	}

	if len(params) > 0 {
		q := req.URL.Query()

		for key, value := range params {
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

	organizationID, err := validateCRMOrganizationID(operation, organizationID)
	if err != nil {
		return nil, err
	}

	holderID, err = validateCRMUUIDParam(operation, "holderID", holderID)
	if err != nil {
		return nil, err
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

	organizationID, err := validateCRMOrganizationID(operation, organizationID)
	if err != nil {
		return nil, err
	}

	holderID, err = validateCRMUUIDParam(operation, "holderID", holderID)
	if err != nil {
		return nil, err
	}

	aliasID, err = validateCRMUUIDParam(operation, "aliasID", aliasID)
	if err != nil {
		return nil, err
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

	organizationID, err := validateCRMOrganizationID(operation, organizationID)
	if err != nil {
		return nil, err
	}

	holderID, err = validateCRMUUIDParam(operation, "holderID", holderID)
	if err != nil {
		return nil, err
	}

	aliasID, err = validateCRMUUIDParam(operation, "aliasID", aliasID)
	if err != nil {
		return nil, err
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

	organizationID, err := validateCRMOrganizationID(operation, organizationID)
	if err != nil {
		return err
	}

	holderID, err = validateCRMUUIDParam(operation, "holderID", holderID)
	if err != nil {
		return err
	}

	aliasID, err = validateCRMUUIDParam(operation, "aliasID", aliasID)
	if err != nil {
		return err
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

	organizationID, err := validateCRMOrganizationID(operation, organizationID)
	if err != nil {
		return err
	}

	holderID, err = validateCRMUUIDParam(operation, "holderID", holderID)
	if err != nil {
		return err
	}

	aliasID, err = validateCRMUUIDParam(operation, "aliasID", aliasID)
	if err != nil {
		return err
	}

	relatedPartyID, err = validateCRMUUIDParam(operation, "relatedPartyID", relatedPartyID)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/related-parties/%s", e.aliasURL(holderID, aliasID), pathSegment(relatedPartyID))

	return e.httpClient.doRequest(ctx, http.MethodDelete, endpoint, crmHeaders(organizationID), nil, nil)
}

func (e *aliasesEntity) listURL() string {
	return fmt.Sprintf("%s/aliases", e.baseURLs["crm"])
}

func validatedAliasListQueryParams(operation string, opts *models.ListOptions) (map[string]string, error) {
	if opts == nil {
		return map[string]string{}, nil
	}

	params := opts.ToQueryParams()

	holderID, ok := params["holder_id"]
	if !ok {
		return params, nil
	}

	validatedHolderID, err := validateOptionalCRMUUIDParam(operation, "holder_id", holderID)
	if err != nil {
		return nil, err
	}

	if validatedHolderID == "" {
		delete(params, "holder_id")
	} else {
		params["holder_id"] = validatedHolderID
	}

	return params, nil
}

func (e *aliasesEntity) aliasURL(holderID, aliasID string) string {
	baseURL := e.baseURLs["crm"]
	if aliasID == "" {
		return fmt.Sprintf("%s/holders/%s/aliases", baseURL, pathSegment(holderID))
	}

	return fmt.Sprintf("%s/holders/%s/aliases/%s", baseURL, pathSegment(holderID), pathSegment(aliasID))
}

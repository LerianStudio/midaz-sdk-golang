package entities

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"net/url"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
)

// AliasesService defines CRM alias operations.
type AliasesService interface {
	// ListAliases retrieves aliases for an organization.
	ListAliases(ctx context.Context, organizationID string, opts models.AliasesListOpts) (*models.ListResponse[models.Alias], error)

	ListAliasesAll(ctx context.Context, organizationID string, opts models.AliasesListOpts) iter.Seq2[models.Alias, error]

	ListAliasesPages(ctx context.Context, organizationID string, opts models.AliasesListOpts) iter.Seq2[*models.ListResponse[models.Alias], error]
	// CreateAlias creates an alias for a holder.
	CreateAlias(ctx context.Context, organizationID, holderID string, input *models.CreateAliasInput) (*models.Alias, error)
	// GetAlias retrieves an alias by ID.
	//
	// To include soft-deleted aliases in the response, tag the context with
	// [sdkctx.WithIncludeDeleted](ctx, true) before calling.
	GetAlias(ctx context.Context, organizationID, holderID, aliasID string) (*models.Alias, error)
	// UpdateAlias updates an alias by ID.
	UpdateAlias(ctx context.Context, organizationID, holderID, aliasID string, input *models.UpdateAliasInput) (*models.Alias, error)
	// DeleteAlias deletes an alias by ID.
	//
	// By default the operation performs a soft delete (the record is marked deleted
	// but preserved). To perform a hard delete (permanent removal), tag the context
	// with [sdkctx.WithHardDelete](ctx, true) before calling.
	DeleteAlias(ctx context.Context, organizationID, holderID, aliasID string) error
	// DeleteRelatedParty deletes a related party from an alias.
	DeleteRelatedParty(ctx context.Context, organizationID, holderID, aliasID, relatedPartyID string) error
}

type aliasesEntity struct {
	serviceEntity
}

// newAliasesEntity creates a new AliasesService instance.
func newAliasesEntity(client *http.Client, authToken string, baseURLs map[string]string) AliasesService {
	return &aliasesEntity{serviceEntity: newServiceEntity(client, authToken, baseURLs)}
}

// ListAliases retrieves aliases for an organization.
func (e *aliasesEntity) ListAliases(ctx context.Context, organizationID string, opts models.AliasesListOpts) (*models.ListResponse[models.Alias], error) {
	const operation = "ListAliases"

	organizationID, err := validateCRMOrganizationID(operation, organizationID)
	if err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
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

// ListAliasesAll yields every alias matching the request, transparently advancing pagination.
func (e *aliasesEntity) ListAliasesAll(ctx context.Context, organizationID string, opts models.AliasesListOpts) iter.Seq2[models.Alias, error] {
	return flattenPages(e.ListAliasesPages(ctx, organizationID, opts))
}

// ListAliasesPages yields one full *ListResponse[Alias] per page.
func (e *aliasesEntity) ListAliasesPages(ctx context.Context, organizationID string, opts models.AliasesListOpts) iter.Seq2[*models.ListResponse[models.Alias], error] {
	ctx = requestContext(ctx)

	return func(yield func(*models.ListResponse[models.Alias], error) bool) {
		current := opts
		if current.Page <= 0 {
			current.Page = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListAliases(ctx, organizationID, current)
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
//
// Use [sdkctx.WithIncludeDeleted](ctx, true) to include soft-deleted aliases.
func (e *aliasesEntity) GetAlias(ctx context.Context, organizationID, holderID, aliasID string) (*models.Alias, error) {
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
	if sdkctx.IncludeDeletedFromContext(ctx) {
		q := url.Values{}
		q.Set("include_deleted", "true")
		endpoint = endpoint + "?" + q.Encode()
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
//
// The default is a soft delete (record preserved, marked deleted). Use
// [sdkctx.WithHardDelete](ctx, true) to perform a hard delete (permanent).
func (e *aliasesEntity) DeleteAlias(ctx context.Context, organizationID, holderID, aliasID string) error {
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
	if sdkctx.HardDeleteFromContext(ctx) {
		q := url.Values{}
		q.Set("hard_delete", "true")
		endpoint = endpoint + "?" + q.Encode()
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

func validatedAliasListQueryParams(operation string, opts models.AliasesListOpts) (map[string]string, error) {
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

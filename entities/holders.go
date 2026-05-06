package entities

import (
	"context"
	"fmt"
	"iter"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
)

// HoldersService defines CRM holder operations.
type HoldersService interface {
	// ListHolders retrieves holders for an organization.
	ListHolders(ctx context.Context, organizationID string, opts models.HoldersListOpts) (*models.ListResponse[models.Holder], error)

	ListHoldersAll(ctx context.Context, organizationID string, opts models.HoldersListOpts) iter.Seq2[models.Holder, error]

	ListHoldersPages(ctx context.Context, organizationID string, opts models.HoldersListOpts) iter.Seq2[*models.ListResponse[models.Holder], error]
	// CreateHolder creates a holder.
	CreateHolder(ctx context.Context, organizationID string, input *models.CreateHolderInput) (*models.Holder, error)
	// GetHolder retrieves a holder by ID.
	//
	// To include soft-deleted holders in the response, tag the context with
	// [sdkctx.WithIncludeDeleted](ctx, true) before calling.
	GetHolder(ctx context.Context, organizationID, holderID string) (*models.Holder, error)
	// UpdateHolder updates a holder by ID.
	UpdateHolder(ctx context.Context, organizationID, holderID string, input *models.UpdateHolderInput) (*models.Holder, error)
	// DeleteHolder deletes a holder by ID.
	//
	// By default the operation performs a soft delete (the record is marked deleted
	// but preserved). To perform a hard delete (permanent removal), tag the context
	// with [sdkctx.WithHardDelete](ctx, true) before calling.
	DeleteHolder(ctx context.Context, organizationID, holderID string) error
}

type holdersEntity struct {
	serviceEntity
}

// newHoldersEntity creates a new HoldersService instance.
func newHoldersEntity(client *http.Client, authToken string, baseURLs map[string]string) HoldersService {
	return &holdersEntity{serviceEntity: newServiceEntity(client, authToken, baseURLs)}
}

// ListHolders retrieves holders for an organization.
func (e *holdersEntity) ListHolders(ctx context.Context, organizationID string, opts models.HoldersListOpts) (*models.ListResponse[models.Holder], error) {
	const operation = "ListHolders"

	organizationID, err := validateCRMOrganizationID(operation, organizationID)
	if err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	req, err := newRequestWithContext(ctx, http.MethodGet, e.buildURL(""), nil)
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

	var response models.ListResponse[models.Holder]
	if err := e.httpClient.doRequest(ctx, http.MethodGet, req.URL.String(), crmHeaders(organizationID), nil, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ListHoldersAll yields every holder matching the request, transparently advancing pagination.
func (e *holdersEntity) ListHoldersAll(ctx context.Context, organizationID string, opts models.HoldersListOpts) iter.Seq2[models.Holder, error] {
	return flattenPages(e.ListHoldersPages(ctx, organizationID, opts))
}

// ListHoldersPages yields one full *ListResponse[Holder] per page.
func (e *holdersEntity) ListHoldersPages(ctx context.Context, organizationID string, opts models.HoldersListOpts) iter.Seq2[*models.ListResponse[models.Holder], error] {
	return func(yield func(*models.ListResponse[models.Holder], error) bool) {
		current := opts
		if current.Page <= 0 {
			current.Page = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListHolders(ctx, organizationID, current)
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

// CreateHolder creates a holder.
func (e *holdersEntity) CreateHolder(ctx context.Context, organizationID string, input *models.CreateHolderInput) (*models.Holder, error) {
	const operation = "CreateHolder"

	organizationID, err := validateCRMOrganizationID(operation, organizationID)
	if err != nil {
		return nil, err
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
//
// Use [sdkctx.WithIncludeDeleted](ctx, true) to include soft-deleted holders.
func (e *holdersEntity) GetHolder(ctx context.Context, organizationID, holderID string) (*models.Holder, error) {
	const operation = "GetHolder"

	organizationID, err := validateCRMOrganizationID(operation, organizationID)
	if err != nil {
		return nil, err
	}

	holderID, err = validateCRMUUIDParam(operation, "holderID", holderID)
	if err != nil {
		return nil, err
	}

	endpoint := e.buildURL(holderID)
	if sdkctx.IncludeDeletedFromContext(ctx) {
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
//
// The default is a soft delete (record preserved, marked deleted). Use
// [sdkctx.WithHardDelete](ctx, true) to perform a hard delete (permanent).
func (e *holdersEntity) DeleteHolder(ctx context.Context, organizationID, holderID string) error {
	const operation = "DeleteHolder"

	organizationID, err := validateCRMOrganizationID(operation, organizationID)
	if err != nil {
		return err
	}

	holderID, err = validateCRMUUIDParam(operation, "holderID", holderID)
	if err != nil {
		return err
	}

	endpoint := e.buildURL(holderID)
	if sdkctx.HardDeleteFromContext(ctx) {
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

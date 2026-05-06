package entities

import (
	"context"
	"fmt"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// MetadataIndexesService defines the service for Ledger metadata-index endpoints.
type MetadataIndexesService interface {
	// ListMetadataIndexes lists metadata indexes, optionally filtered by entity name.
	ListMetadataIndexes(ctx context.Context, entityName string) ([]models.MetadataIndex, error)
	// CreateMetadataIndex creates a metadata index for an entity type.
	CreateMetadataIndex(ctx context.Context, entityName string, input *models.CreateMetadataIndexInput) (*models.MetadataIndex, error)
	// DeleteMetadataIndex deletes a metadata index by entity name and metadata key.
	DeleteMetadataIndex(ctx context.Context, entityName, metadataKey string) error
}

type metadataIndexesEntity struct {
	serviceEntity
}

// newMetadataIndexesEntity creates a new MetadataIndexesService instance.
func newMetadataIndexesEntity(client *http.Client, authToken string, baseURLs map[string]string) MetadataIndexesService {
	return &metadataIndexesEntity{serviceEntity: newServiceEntity(client, authToken, baseURLs)}
}

func (e *metadataIndexesEntity) buildBaseURL() string {
	return fmt.Sprintf("%s/settings/metadata-indexes", e.baseURLs["transaction"])
}

// ListMetadataIndexes lists metadata indexes, optionally filtered by entity name.
func (e *metadataIndexesEntity) ListMetadataIndexes(ctx context.Context, entityName string) ([]models.MetadataIndex, error) {
	const operation = "ListMetadataIndexes"

	req, err := newRequestWithContext(ctx, http.MethodGet, e.buildBaseURL(), nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if entityName != "" {
		if !models.IsValidMetadataIndexEntity(entityName) {
			return nil, errors.NewValidationError(operation, "invalid entityName", fmt.Errorf("unsupported metadata index entity %q", entityName))
		}

		q := req.URL.Query()
		q.Set("entity_name", entityName)
		req.URL.RawQuery = q.Encode()
	}

	var result []models.MetadataIndex
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	if result == nil {
		result = []models.MetadataIndex{}
	}

	return result, nil
}

// CreateMetadataIndex creates a metadata index for an entity type.
func (e *metadataIndexesEntity) CreateMetadataIndex(ctx context.Context, entityName string, input *models.CreateMetadataIndexInput) (*models.MetadataIndex, error) {
	const operation = "CreateMetadataIndex"

	if entityName == "" {
		return nil, errors.NewMissingParameterError(operation, "entityName")
	}

	if !models.IsValidMetadataIndexEntity(entityName) {
		return nil, errors.NewValidationError(operation, "invalid entityName", fmt.Errorf("unsupported metadata index entity %q", entityName))
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "metadata index validation failed", err)
	}

	endpoint := fmt.Sprintf("%s/entities/%s", e.buildBaseURL(), pathSegment(entityName))

	var result models.MetadataIndex
	if err := e.httpClient.doRequest(ctx, http.MethodPost, endpoint, nil, input, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteMetadataIndex deletes a metadata index by entity name and metadata key.
func (e *metadataIndexesEntity) DeleteMetadataIndex(ctx context.Context, entityName, metadataKey string) error {
	const operation = "DeleteMetadataIndex"

	if entityName == "" {
		return errors.NewMissingParameterError(operation, "entityName")
	}

	if !models.IsValidMetadataIndexEntity(entityName) {
		return errors.NewValidationError(operation, "invalid entityName", fmt.Errorf("unsupported metadata index entity %q", entityName))
	}

	if metadataKey == "" {
		return errors.NewMissingParameterError(operation, "metadataKey")
	}

	endpoint := fmt.Sprintf("%s/entities/%s/key/%s", e.buildBaseURL(), pathSegment(entityName), pathSegment(metadataKey))

	req, err := newRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	return e.httpClient.sendRequest(req, nil)
}

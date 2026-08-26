// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// metadataIndexesV2Facade serves the /v2 metadata-index surface — the indexes an
// operator creates so a metadata filter does not become a collection scan.
//
// The resource is UNSCOPED (/v2/settings/metadata-indexes, no organization or
// ledger in the path) and NOT paginated: the list answers with a bare JSON
// array, so there is no Pages/All trinity. There is no update either — an index
// is created or dropped.
//
// The entity name sits in a different place per operation, which the generated
// layer routes: a query parameter on the list, a path segment on create and
// delete. The facade takes it uniformly.
type metadataIndexesV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newMetadataIndexesV2Facade wires the facade over a ledger plane client.
func newMetadataIndexesV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *metadataIndexesV2Facade {
	return &metadataIndexesV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves the metadata indexes, optionally narrowed to one entity. An
// empty entityName lists every index rather than none, matching the server: the
// filter is omitted from the query, not sent empty.
func (f *metadataIndexesV2Facade) List(ctx context.Context, entityName string) ([]models.MetadataIndex, error) {
	const operation = "V2.MetadataIndexes.List"

	params := &genledger.GetAllMetadataIndexesV2Params{}
	if entityName != "" {
		params.EntityName = strPtr(entityName)
	}

	//nolint:bodyclose // readSlice drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAllMetadataIndexesV2(ctx, params)

	return readSlice[models.MetadataIndex](operation, resp, err)
}

// Create registers a new metadata index for an entity.
func (f *metadataIndexesV2Facade) Create(ctx context.Context, entityName string, input *models.CreateMetadataIndexInput) (*models.MetadataIndex, error) {
	const operation = "V2.MetadataIndexes.Create"

	if err := requirePathIDs(operation, "entityName", entityName); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.MetadataIndex](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateMetadataIndexV2WithBody(ctx, entityName, jsonContentType, body,
			idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Delete removes a metadata index by its key for an entity.
func (f *metadataIndexesV2Facade) Delete(ctx context.Context, entityName, indexKey string) error {
	const operation = "V2.MetadataIndexes.Delete"

	if err := requirePathIDs(operation, "entityName", entityName, "indexKey", indexKey); err != nil {
		return err
	}

	//nolint:bodyclose // deleteResource drains and closes the body via readRawResponse.
	resp, err := f.ledger.DeleteMetadataIndexV2(ctx, entityName, indexKey, idempotencyEditors(ctx, f.enableIdempotency)...)

	return deleteResource(operation, resp, err)
}

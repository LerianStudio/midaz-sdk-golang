// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

// metadataIndexesFacade is the Phase 2 (Task 2.1.d) hand-written facade over the
// generated genledger.ClientWithResponses for the GLOBAL metadata-indexes
// resource. The public surface is exactly models.MetadataIndex + *errors.Error.
//
// This resource is unscoped (no org/ledger in the path) and NOT paginated: the
// list endpoint returns a bare JSON array, so there is no List/Pages/All
// trinaldo. List decodes the slice directly; Create/Delete follow the
// Organizations write-exemplar. There is no Update.
//
// Wire note: the generated client puts entityName in different positions per
// operation — a query param (entity_name) on the global list, a path segment on
// create/delete. The facade signatures take entityName uniformly and let the
// generated layer route it correctly.
type metadataIndexesFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newMetadataIndexesFacade wires the facade over a ledger plane client.
func newMetadataIndexesFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *metadataIndexesFacade {
	return &metadataIndexesFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// List retrieves every metadata index for an entity, normalized into the public
// model. The endpoint returns a bare JSON array (no pagination envelope), so the
// body unmarshals straight into a []models.MetadataIndex.
func (f *metadataIndexesFacade) List(ctx context.Context, entityName string) ([]models.MetadataIndex, error) {
	const operation = "MetadataIndexes.List"

	params := &genledger.GetAllMetadataIndexesParams{}
	if entityName != "" {
		params.EntityName = strPtr(entityName)
	}

	resp, err := f.ledger.GetAllMetadataIndexesWithResponse(ctx, params)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return nil, errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	var indexes []models.MetadataIndex
	if err := json.Unmarshal(resp.Body, &indexes); err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return indexes, nil
}

// Create registers a new metadata index for an entity via the write-facade
// pattern (marshal input -> rewindable *bytes.Reader -> WithBody variant so the
// auth round tripper can replay after a 401).
func (f *metadataIndexesFacade) Create(ctx context.Context, entityName string, input *models.CreateMetadataIndexInput) (*models.MetadataIndex, error) {
	const operation = "MetadataIndexes.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	return writeJSON[models.MetadataIndex](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		resp, err := f.ledger.CreateMetadataIndexWithBodyWithResponse(ctx, entityName, "application/json", body, idempotencyEditors(ctx, f.enableIdempotency)...)
		if err != nil {
			return nil, nil, err
		}

		return resp.HTTPResponse, resp.Body, nil
	})
}

// Delete removes a metadata index by its key for an entity. The server returns
// 204 with no body on success, so there is nothing to decode.
func (f *metadataIndexesFacade) Delete(ctx context.Context, entityName, indexKey string) error {
	const operation = "MetadataIndexes.Delete"

	resp, err := f.ledger.DeleteMetadataIndexWithResponse(ctx, entityName, indexKey, idempotencyEditors(ctx, f.enableIdempotency)...)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode()) {
		return errors.DecodeProblemJSON(resp.StatusCode(), resp.Body, requestIDOf(resp.HTTPResponse))
	}

	return nil
}

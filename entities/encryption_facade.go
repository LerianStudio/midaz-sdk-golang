// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// encryptionFacade is the Epic 3.3 (Task 3.3.1) hand-written facade over the
// generated genledger.ClientWithResponses for envelope-encryption provisioning.
// The public surface is exactly models.* + *errors.Error; the generated types
// never leak.
//
// 404 semantics (both methods): a 404 means envelope encryption is disabled for
// this deployment (legacy mode — the routes are not registered because the KMS
// vendor is not configured). It is mapped cleanly to *errors.Error{StatusCode:404}
// via the standard non-2xx path. This is DIFFERENT from a provisioned:false 200:
// a 404 says the feature is unavailable (not fixable by the caller), whereas a
// provisioned:false 200 says the org simply has not been provisioned yet
// (fixable via Provision). The facade never collapses a 404 into a fabricated
// provisioned:false — that would destroy the distinction.
type encryptionFacade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on writes; an
	// explicit or context-supplied key stamps regardless.
	enableIdempotency bool
}

// newEncryptionFacade wires the facade over a ledger plane client.
func newEncryptionFacade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *encryptionFacade {
	return &encryptionFacade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// Provision provisions envelope encryption for an organization.
//
// This routes through the RAW ProvisionEncryptionV2WithBody + readRawResponse and
// gates success on isSuccess(2xx) via writeJSON — NEVER through the generated
// ...WithResponse parser, whose ParseProvisionEncryptionResp gates JSON200 on
// StatusCode==200 exactly. The server returns 201 Created on success, so the
// parser would drop the decoded body and fall through to the error branch. The
// raw 2xx path accepts the 201.
//
// A 404 means envelope encryption is disabled for this deployment (legacy mode);
// it returns *errors.Error{StatusCode:404}, which is not the same as a
// provisioned:false 200 (see the facade doc).
func (f *encryptionFacade) Provision(ctx context.Context, orgID string, input *models.ProvisionEncryptionInput) (*models.ProvisionEncryptionResponse, error) {
	const operation = "Encryption.Provision"

	if err := requirePathIDs(operation, "orgID", orgID); err != nil {
		return nil, err
	}

	if err := input.Validate(); err != nil {
		return nil, err
	}

	resp, err := writeJSON[models.ProvisionEncryptionResponse](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.ProvisionEncryptionV2WithBody(ctx, orgID, &genledger.ProvisionEncryptionV2Params{}, jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})

	// A 404 here means envelope encryption is disabled (legacy mode); tag it so
	// callers can IsFeatureNotAvailable it, distinct from a generic NotFound.
	return resp, errors.MarkFeatureNotAvailable(err)
}

// GetProvisioningStatus reports whether an organization has provisioned envelope
// encryption. A non-provisioned but feature-available org returns 200 with
// provisioned:false.
//
// A 404 means envelope encryption is disabled for this deployment (legacy mode);
// it returns *errors.Error{StatusCode:404}, which is not the same as a
// provisioned:false 200 (see the facade doc).
func (f *encryptionFacade) GetProvisioningStatus(ctx context.Context, orgID string) (*models.ProvisioningStatusResponse, error) {
	const operation = "Encryption.GetProvisioningStatus"

	if err := requirePathIDs(operation, "orgID", orgID); err != nil {
		return nil, err
	}

	resp, err := f.ledger.GetProvisioningStatusV2WithResponse(ctx, orgID, &genledger.GetProvisioningStatusV2Params{})
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	res, err := decodeOne[models.ProvisioningStatusResponse](operation, resp.StatusCode(), resp.Body, resp.HTTPResponse)

	// A 404 here means envelope encryption is disabled (legacy mode); tag it so
	// callers can IsFeatureNotAvailable it, distinct from a generic NotFound.
	return res, errors.MarkFeatureNotAvailable(err)
}

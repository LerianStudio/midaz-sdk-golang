// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"errors"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation"
)

// ProvisionEncryptionInput is the request payload to provision envelope
// encryption for an organization. Both fields are required by the server
// (mmodel.ProvisionEncryptionInput, validate:"required").
type ProvisionEncryptionInput struct {
	// Actor is the identity requesting provisioning (required).
	Actor string `json:"actor"`

	// Reason is the audit reason for provisioning (required).
	Reason string `json:"reason"`
}

// NewProvisionEncryptionInput creates a ProvisionEncryptionInput with the
// required fields.
func NewProvisionEncryptionInput(actor, reason string) *ProvisionEncryptionInput {
	return &ProvisionEncryptionInput{Actor: actor, Reason: reason}
}

// Validate reports every field-level violation together. Actor and Reason are
// both required.
func (input *ProvisionEncryptionInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	if input.Actor == "" {
		errs.Append("actor", "is required")
	}

	if input.Reason == "" {
		errs.Append("reason", "is required")
	}

	return errs.OrNil()
}

// ProvisionEncryptionResponse is the server's reply after provisioning envelope
// encryption. Key IDs mirror the SERVER DTO (mmodel.ProvisionEncryptionResponse,
// uint32) rather than the generated int32 — cosmetic for real values, fidelity
// to the source DTO. These are key identifiers, not monetary values.
type ProvisionEncryptionResponse struct {
	OrganizationID   string `json:"organization_id"`
	KEKPath          string `json:"kek_path"`
	AEADPrimaryKeyID uint32 `json:"aead_primary_key_id"`
	PRFPrimaryKeyID  uint32 `json:"prf_primary_key_id"`
	Status           string `json:"status"`
}

// ProvisioningStatusResponse reports whether an organization has provisioned
// envelope encryption.
//
// Provisioned is a real server-computed bool: a non-provisioned but
// feature-available org returns HTTP 200 with provisioned:false. This is NOT
// the same as a 404 (which means the whole envelope-encryption feature is
// disabled at the deployment level — see the facade docstrings).
type ProvisioningStatusResponse struct {
	OrganizationID string `json:"organization_id"`
	Provisioned    bool   `json:"provisioned"`
	Status         string `json:"status,omitempty"`
}

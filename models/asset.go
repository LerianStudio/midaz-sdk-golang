// Package models defines the data models used by the Midaz SDK.
package models

import (
	"errors"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

// Asset is an alias for mmodel.Asset to maintain compatibility while using midaz entities.
type Asset = mmodel.Asset

// CreateAssetInput wraps mmodel.CreateAssetInput to maintain compatibility while using midaz entities.
type CreateAssetInput struct {
	mmodel.CreateAssetInput
}

// UpdateAssetInput wraps mmodel.UpdateAssetInput to maintain compatibility while using midaz entities.
type UpdateAssetInput struct {
	mmodel.UpdateAssetInput
}

// NewCreateAssetInput creates a new CreateAssetInput with required fields.
func NewCreateAssetInput(name, code string) *CreateAssetInput {
	return &CreateAssetInput{
		CreateAssetInput: mmodel.CreateAssetInput{
			Name: name,
			Code: code,
		},
	}
}

// WithType sets the asset type.
func (input *CreateAssetInput) WithType(assetType string) *CreateAssetInput {
	input.Type = assetType
	return input
}

// WithStatus sets the status.
func (input *CreateAssetInput) WithStatus(status Status) *CreateAssetInput {
	input.Status = status
	return input
}

// WithMetadata sets the metadata.
func (input *CreateAssetInput) WithMetadata(metadata map[string]any) *CreateAssetInput {
	input.Metadata = metadata
	return input
}

// Validate validates the CreateAssetInput fields.
func (input *CreateAssetInput) Validate() error {
	if input.Name == "" {
		return errors.New("name is required")
	}

	if input.Code == "" {
		return errors.New("code is required")
	}

	return nil
}

// NewUpdateAssetInput creates a new UpdateAssetInput.
func NewUpdateAssetInput() *UpdateAssetInput {
	return &UpdateAssetInput{
		UpdateAssetInput: mmodel.UpdateAssetInput{},
	}
}

// WithName sets the name for UpdateAssetInput.
func (input *UpdateAssetInput) WithName(name string) *UpdateAssetInput {
	input.Name = name
	return input
}

// WithStatus sets the status for UpdateAssetInput.
func (input *UpdateAssetInput) WithStatus(status Status) *UpdateAssetInput {
	input.Status = status
	return input
}

// WithMetadata sets the metadata for UpdateAssetInput.
func (input *UpdateAssetInput) WithMetadata(metadata map[string]any) *UpdateAssetInput {
	input.Metadata = metadata
	return input
}

// Validate validates the UpdateAssetInput fields.
func (*UpdateAssetInput) Validate() error {
	// For update operations, most fields are optional
	return nil
}

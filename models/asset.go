// Package models defines the data models used by the Midaz SDK.
package models

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation/core"
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

const (
	maxAssetNameLength = 256
	maxAssetCodeLength = 100
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

// NewCreateAssetInput creates a new CreateAssetInput.
// Deprecated: use NewCreateAssetInputWithType so the required Midaz asset type
// is provided at construction time.
func NewCreateAssetInput(name, code string) *CreateAssetInput {
	return &CreateAssetInput{
		CreateAssetInput: mmodel.CreateAssetInput{
			Name: name,
			Code: code,
		},
	}
}

// NewCreateAssetInputWithType creates a new CreateAssetInput with all required fields.
func NewCreateAssetInputWithType(name, code, assetType string) *CreateAssetInput {
	return NewCreateAssetInput(name, code).WithType(assetType)
}

// WithType sets the asset type.
func (input *CreateAssetInput) WithType(assetType string) *CreateAssetInput {
	if input == nil {
		return nil
	}

	input.Type = assetType

	return input
}

// WithStatus sets the status.
func (input *CreateAssetInput) WithStatus(status Status) *CreateAssetInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

// WithMetadata sets the metadata.
func (input *CreateAssetInput) WithMetadata(metadata map[string]any) *CreateAssetInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// Validate validates the CreateAssetInput fields.
func (input *CreateAssetInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.Name == "" {
		return errors.New("name is required")
	}

	if len(input.Name) > maxAssetNameLength {
		return fmt.Errorf("name must be at most %d characters", maxAssetNameLength)
	}

	if input.Code == "" {
		return errors.New("code is required")
	}

	if len(input.Code) > maxAssetCodeLength {
		return fmt.Errorf("code must be at most %d characters", maxAssetCodeLength)
	}

	if input.Type == "" {
		return errors.New("type is required")
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

// MarshalJSON omits optional create fields when callers leave them unset.
func (input *CreateAssetInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "name", input.Name)
	addStringField(fields, "type", input.Type)
	addStringField(fields, "code", input.Code)
	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

// NewUpdateAssetInput creates a new UpdateAssetInput.
func NewUpdateAssetInput() *UpdateAssetInput {
	return &UpdateAssetInput{
		UpdateAssetInput: mmodel.UpdateAssetInput{},
	}
}

// WithName sets the name for UpdateAssetInput.
func (input *UpdateAssetInput) WithName(name string) *UpdateAssetInput {
	if input == nil {
		return nil
	}

	input.Name = name

	return input
}

// WithStatus sets the status for UpdateAssetInput.
func (input *UpdateAssetInput) WithStatus(status Status) *UpdateAssetInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

// WithMetadata sets the metadata for UpdateAssetInput.
func (input *UpdateAssetInput) WithMetadata(metadata map[string]any) *UpdateAssetInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// Validate validates the UpdateAssetInput fields.
func (input *UpdateAssetInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	if len(input.Name) > maxAssetNameLength {
		return fmt.Errorf("name must be at most %d characters", maxAssetNameLength)
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

func (input *UpdateAssetInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.Name != "" || !IsStatusEmpty(input.Status) || input.Metadata != nil
}

// MarshalJSON emits only fields explicitly set on the SDK PATCH input.
func (input *UpdateAssetInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "name", input.Name)
	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

// Package models defines the data models used by the Midaz SDK.
package models

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation/core"
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

const maxSegmentNameLength = 256

// Segment is an alias for mmodel.Segment to maintain compatibility while using midaz entities.
type Segment = mmodel.Segment

// CreateSegmentInput wraps mmodel.CreateSegmentInput to maintain compatibility while using midaz entities.
type CreateSegmentInput struct {
	mmodel.CreateSegmentInput
}

// UpdateSegmentInput wraps mmodel.UpdateSegmentInput to maintain compatibility while using midaz entities.
type UpdateSegmentInput struct {
	mmodel.UpdateSegmentInput
}

// NewCreateSegmentInput creates a new CreateSegmentInput with required fields.
func NewCreateSegmentInput(name string) *CreateSegmentInput {
	return &CreateSegmentInput{
		CreateSegmentInput: mmodel.CreateSegmentInput{
			Name: name,
		},
	}
}

// WithStatus sets the status.
func (input *CreateSegmentInput) WithStatus(status Status) *CreateSegmentInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

// WithMetadata sets the metadata.
func (input *CreateSegmentInput) WithMetadata(metadata map[string]any) *CreateSegmentInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// Validate validates the CreateSegmentInput fields.
func (input *CreateSegmentInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.Name == "" {
		return errors.New("name is required")
	}

	if len(input.Name) > maxSegmentNameLength {
		return fmt.Errorf("name must be at most %d characters", maxSegmentNameLength)
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

// MarshalJSON omits optional create fields when callers leave them unset.
func (input *CreateSegmentInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "name", input.Name)
	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

// NewUpdateSegmentInput creates a new UpdateSegmentInput.
func NewUpdateSegmentInput() *UpdateSegmentInput {
	return &UpdateSegmentInput{
		UpdateSegmentInput: mmodel.UpdateSegmentInput{},
	}
}

// WithName sets the name for UpdateSegmentInput.
func (input *UpdateSegmentInput) WithName(name string) *UpdateSegmentInput {
	if input == nil {
		return nil
	}

	input.Name = name

	return input
}

// WithStatus sets the status for UpdateSegmentInput.
func (input *UpdateSegmentInput) WithStatus(status Status) *UpdateSegmentInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

// WithMetadata sets the metadata for UpdateSegmentInput.
func (input *UpdateSegmentInput) WithMetadata(metadata map[string]any) *UpdateSegmentInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// Validate validates the UpdateSegmentInput fields.
func (input *UpdateSegmentInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	if len(input.Name) > maxSegmentNameLength {
		return fmt.Errorf("name must be at most %d characters", maxSegmentNameLength)
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

func (input *UpdateSegmentInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.Name != "" || !IsStatusEmpty(input.Status) || input.Metadata != nil
}

// MarshalJSON emits only fields explicitly set on the SDK PATCH input.
func (input *UpdateSegmentInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "name", input.Name)
	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

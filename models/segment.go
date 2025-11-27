// Package models defines the data models used by the Midaz SDK.
package models

import (
	"errors"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

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
	input.Status = status
	return input
}

// WithMetadata sets the metadata.
func (input *CreateSegmentInput) WithMetadata(metadata map[string]any) *CreateSegmentInput {
	input.Metadata = metadata
	return input
}

// Validate validates the CreateSegmentInput fields.
func (input *CreateSegmentInput) Validate() error {
	if input.Name == "" {
		return errors.New("name is required")
	}

	return nil
}

// NewUpdateSegmentInput creates a new UpdateSegmentInput.
func NewUpdateSegmentInput() *UpdateSegmentInput {
	return &UpdateSegmentInput{
		UpdateSegmentInput: mmodel.UpdateSegmentInput{},
	}
}

// WithName sets the name for UpdateSegmentInput.
func (input *UpdateSegmentInput) WithName(name string) *UpdateSegmentInput {
	input.Name = name
	return input
}

// WithStatus sets the status for UpdateSegmentInput.
func (input *UpdateSegmentInput) WithStatus(status Status) *UpdateSegmentInput {
	input.Status = status
	return input
}

// WithMetadata sets the metadata for UpdateSegmentInput.
func (input *UpdateSegmentInput) WithMetadata(metadata map[string]any) *UpdateSegmentInput {
	input.Metadata = metadata
	return input
}

// Validate validates the UpdateSegmentInput fields.
func (*UpdateSegmentInput) Validate() error {
	// For update operations, most fields are optional
	return nil
}

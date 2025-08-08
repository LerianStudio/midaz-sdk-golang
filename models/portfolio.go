// Package models defines the data models used by the Midaz SDK.
package models

import (
	"fmt"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

// Portfolio is an alias for mmodel.Portfolio to maintain compatibility while using midaz entities.
type Portfolio = mmodel.Portfolio

// CreatePortfolioInput wraps mmodel.CreatePortfolioInput to maintain compatibility while using midaz entities.
type CreatePortfolioInput struct {
	mmodel.CreatePortfolioInput
}

// UpdatePortfolioInput wraps mmodel.UpdatePortfolioInput to maintain compatibility while using midaz entities.
type UpdatePortfolioInput struct {
	mmodel.UpdatePortfolioInput
}

// NewCreatePortfolioInput creates a new CreatePortfolioInput with required fields.
func NewCreatePortfolioInput(entityID, name string) *CreatePortfolioInput {
	return &CreatePortfolioInput{
		CreatePortfolioInput: mmodel.CreatePortfolioInput{
			EntityID: entityID,
			Name:     name,
		},
	}
}

// WithStatus sets the status.
func (input *CreatePortfolioInput) WithStatus(status Status) *CreatePortfolioInput {
	input.Status = status
	return input
}

// WithMetadata sets the metadata.
func (input *CreatePortfolioInput) WithMetadata(metadata map[string]any) *CreatePortfolioInput {
	input.Metadata = metadata
	return input
}

// Validate validates the CreatePortfolioInput fields.
func (input *CreatePortfolioInput) Validate() error {
	if input.Name == "" {
		return fmt.Errorf("name is required")
	}

	if input.EntityID == "" {
		return fmt.Errorf("entityID is required")
	}

	return nil
}

// NewUpdatePortfolioInput creates a new UpdatePortfolioInput.
func NewUpdatePortfolioInput() *UpdatePortfolioInput {
	return &UpdatePortfolioInput{
		UpdatePortfolioInput: mmodel.UpdatePortfolioInput{},
	}
}

// WithName sets the name for UpdatePortfolioInput.
func (input *UpdatePortfolioInput) WithName(name string) *UpdatePortfolioInput {
	input.Name = name
	return input
}

// WithStatus sets the status for UpdatePortfolioInput.
func (input *UpdatePortfolioInput) WithStatus(status Status) *UpdatePortfolioInput {
	input.Status = status
	return input
}

// WithMetadata sets the metadata for UpdatePortfolioInput.
func (input *UpdatePortfolioInput) WithMetadata(metadata map[string]any) *UpdatePortfolioInput {
	input.Metadata = metadata
	return input
}

// Validate validates the UpdatePortfolioInput fields.
func (input *UpdatePortfolioInput) Validate() error {
	// For update operations, most fields are optional
	return nil
}

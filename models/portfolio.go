// Package models defines the data models used by the Midaz SDK.
package models

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation/core"
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

const maxPortfolioFieldLength = 256

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
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

// WithEntityID sets the optional external entity identifier.
func (input *CreatePortfolioInput) WithEntityID(entityID string) *CreatePortfolioInput {
	if input == nil {
		return nil
	}

	input.EntityID = entityID

	return input
}

// WithMetadata sets the metadata.
func (input *CreatePortfolioInput) WithMetadata(metadata map[string]any) *CreatePortfolioInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// Validate validates the CreatePortfolioInput fields.
func (input *CreatePortfolioInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.Name == "" {
		return errors.New("name is required")
	}

	if len(input.Name) > maxPortfolioFieldLength {
		return fmt.Errorf("name must be at most %d characters", maxPortfolioFieldLength)
	}

	if len(input.EntityID) > maxPortfolioFieldLength {
		return fmt.Errorf("entityId must be at most %d characters", maxPortfolioFieldLength)
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

// MarshalJSON omits optional create fields when callers leave them unset.
func (input *CreatePortfolioInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "entityId", input.EntityID)
	addStringField(fields, "name", input.Name)
	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

// NewUpdatePortfolioInput creates a new UpdatePortfolioInput.
func NewUpdatePortfolioInput() *UpdatePortfolioInput {
	return &UpdatePortfolioInput{
		UpdatePortfolioInput: mmodel.UpdatePortfolioInput{},
	}
}

// WithName sets the name for UpdatePortfolioInput.
func (input *UpdatePortfolioInput) WithName(name string) *UpdatePortfolioInput {
	if input == nil {
		return nil
	}

	input.Name = name

	return input
}

// WithEntityID sets the optional external entity identifier for update.
func (input *UpdatePortfolioInput) WithEntityID(entityID string) *UpdatePortfolioInput {
	if input == nil {
		return nil
	}

	input.EntityID = entityID

	return input
}

// WithStatus sets the status for UpdatePortfolioInput.
func (input *UpdatePortfolioInput) WithStatus(status Status) *UpdatePortfolioInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

// WithMetadata sets the metadata for UpdatePortfolioInput.
func (input *UpdatePortfolioInput) WithMetadata(metadata map[string]any) *UpdatePortfolioInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// Validate validates the UpdatePortfolioInput fields.
func (input *UpdatePortfolioInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	if len(input.Name) > maxPortfolioFieldLength {
		return fmt.Errorf("name must be at most %d characters", maxPortfolioFieldLength)
	}

	if len(input.EntityID) > maxPortfolioFieldLength {
		return fmt.Errorf("entityId must be at most %d characters", maxPortfolioFieldLength)
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

func (input *UpdatePortfolioInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.EntityID != "" || input.Name != "" || !IsStatusEmpty(input.Status) || input.Metadata != nil
}

// MarshalJSON emits only fields explicitly set on the SDK PATCH input.
func (input *UpdatePortfolioInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "entityId", input.EntityID)
	addStringField(fields, "name", input.Name)
	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation/core"
)

const maxPortfolioFieldLength = 256

// Portfolio is the SDK-native portfolio response type (Track 7E — audit 7.1).
type Portfolio struct {
	ID             string         `json:"id" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	Name           string         `json:"name" example:"My Portfolio" maxLength:"256"`
	EntityID       string         `json:"entityId,omitempty" example:"00000000-0000-0000-0000-000000000000" maxLength:"256"`
	LedgerID       string         `json:"ledgerId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	OrganizationID string         `json:"organizationId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	Status         Status         `json:"status"`
	CreatedAt      time.Time      `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	UpdatedAt      time.Time      `json:"updatedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	DeletedAt      *time.Time     `json:"deletedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// CreatePortfolioInput is the SDK-native portfolio creation payload.
type CreatePortfolioInput struct {
	EntityID string         `json:"entityId" example:"00000000-0000-0000-0000-000000000000"`
	Name     string         `json:"name" example:"My Portfolio"`
	Status   Status         `json:"status"`
	Metadata map[string]any `json:"metadata"`
}

// UpdatePortfolioInput is the SDK-native portfolio patch payload.
type UpdatePortfolioInput struct {
	EntityID string         `json:"entityId" example:"00000000-0000-0000-0000-000000000000"`
	Name     string         `json:"name" example:"My Portfolio Updated"`
	Status   Status         `json:"status"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewCreatePortfolioInput creates a new CreatePortfolioInput with required fields.
func NewCreatePortfolioInput(entityID, name string) *CreatePortfolioInput {
	return &CreatePortfolioInput{
		EntityID: entityID,
		Name:     name,
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

	var errs validation.FieldErrors

	if input.Name == "" {
		errs.Append("name", "is required")
	} else if len(input.Name) > maxPortfolioFieldLength {
		errs.Append("name", fmt.Sprintf("must be at most %d characters", maxPortfolioFieldLength))
	}

	if len(input.EntityID) > maxPortfolioFieldLength {
		errs.Append("entityId", fmt.Sprintf("must be at most %d characters", maxPortfolioFieldLength))
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
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
	return &UpdatePortfolioInput{}
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

	var errs validation.FieldErrors

	if len(input.Name) > maxPortfolioFieldLength {
		errs.Append("name", fmt.Sprintf("must be at most %d characters", maxPortfolioFieldLength))
	}

	if len(input.EntityID) > maxPortfolioFieldLength {
		errs.Append("entityId", fmt.Sprintf("must be at most %d characters", maxPortfolioFieldLength))
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
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

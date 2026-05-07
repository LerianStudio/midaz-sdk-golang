// Package models defines the data models used by the Midaz SDK.
package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
)

const maxSegmentNameLength = 256

// Segment is the SDK-native segment response type (Track 7E — audit 7.1).
type Segment struct {
	ID             string         `json:"id" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	Name           string         `json:"name" example:"My Segment" maxLength:"256"`
	LedgerID       string         `json:"ledgerId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	OrganizationID string         `json:"organizationId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	Status         Status         `json:"status"`
	CreatedAt      time.Time      `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	UpdatedAt      time.Time      `json:"updatedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	DeletedAt      *time.Time     `json:"deletedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// CreateSegmentInput is the SDK-native segment creation payload.
type CreateSegmentInput struct {
	Name     string         `json:"name" example:"My Segment"`
	Status   Status         `json:"status"`
	Metadata map[string]any `json:"metadata"`
}

// UpdateSegmentInput is the SDK-native segment patch payload.
type UpdateSegmentInput struct {
	Name     string         `json:"name" example:"My Segment Updated"`
	Status   Status         `json:"status"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewCreateSegmentInput creates a new CreateSegmentInput with required fields.
func NewCreateSegmentInput(name string) *CreateSegmentInput {
	return &CreateSegmentInput{
		Name: name,
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

	var errs validation.FieldErrors

	name := strings.TrimSpace(input.Name)
	if name == "" {
		errs.Append("name", "is required")
	} else if len(name) > maxSegmentNameLength {
		errs.Append("name", fmt.Sprintf("must be at most %d characters", maxSegmentNameLength))
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
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
	return &UpdateSegmentInput{}
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

	var errs validation.FieldErrors

	if input.Name != "" {
		trimmed := strings.TrimSpace(input.Name)
		switch {
		case trimmed == "":
			errs.Append("name", "must not be blank")
		case len(trimmed) > maxSegmentNameLength:
			errs.Append("name", fmt.Sprintf("must be at most %d characters", maxSegmentNameLength))
		}
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

func (input *UpdateSegmentInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return strings.TrimSpace(input.Name) != "" || !IsStatusEmpty(input.Status) || input.Metadata != nil
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

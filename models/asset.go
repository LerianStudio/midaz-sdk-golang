package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation/core"
)

const (
	maxAssetNameLength = 256
	maxAssetCodeLength = 100
)

// Asset is the SDK-native asset response type (Track 7E — audit 7.1).
//
// Wire format alignment with mmodel.Asset is preserved via JSON tags;
// the SDK no longer aliases into the server-internal mmodel package.
type Asset struct {
	ID             string         `json:"id" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	Name           string         `json:"name" example:"US Dollar" maxLength:"256"`
	Type           string         `json:"type" example:"currency"`
	Code           string         `json:"code" example:"USD" maxLength:"100"`
	Status         Status         `json:"status"`
	LedgerID       string         `json:"ledgerId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	OrganizationID string         `json:"organizationId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	CreatedAt      time.Time      `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	UpdatedAt      time.Time      `json:"updatedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	DeletedAt      *time.Time     `json:"deletedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// CreateAssetInput is the SDK-native asset creation payload.
type CreateAssetInput struct {
	Name     string         `json:"name" example:"US Dollar"`
	Type     string         `json:"type" example:"currency"`
	Code     string         `json:"code" example:"USD"`
	Status   Status         `json:"status"`
	Metadata map[string]any `json:"metadata"`
}

// UpdateAssetInput is the SDK-native asset patch payload.
type UpdateAssetInput struct {
	Name     string         `json:"name" example:"Bitcoin"`
	Status   Status         `json:"status"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewCreateAssetInput creates a new CreateAssetInput.
// Deprecated: use NewCreateAssetInputWithType so the required Midaz asset type
// is provided at construction time.
func NewCreateAssetInput(name, code string) *CreateAssetInput {
	return &CreateAssetInput{
		Name: name,
		Code: code,
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

	var errs validation.FieldErrors

	name := strings.TrimSpace(input.Name)
	if name == "" {
		errs.Append("name", "is required")
	} else if len(name) > maxAssetNameLength {
		errs.Append("name", fmt.Sprintf("must be at most %d characters", maxAssetNameLength))
	}

	code := strings.TrimSpace(input.Code)
	if code == "" {
		errs.Append("code", "is required")
	} else if len(code) > maxAssetCodeLength {
		errs.Append("code", fmt.Sprintf("must be at most %d characters", maxAssetCodeLength))
	}

	if strings.TrimSpace(input.Type) == "" {
		errs.Append("type", "is required")
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
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
	return &UpdateAssetInput{}
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

	var errs validation.FieldErrors

	if input.Name != "" {
		trimmed := strings.TrimSpace(input.Name)
		switch {
		case trimmed == "":
			errs.Append("name", "must not be blank")
		case len(trimmed) > maxAssetNameLength:
			errs.Append("name", fmt.Sprintf("must be at most %d characters", maxAssetNameLength))
		}
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

func (input *UpdateAssetInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return strings.TrimSpace(input.Name) != "" || !IsStatusEmpty(input.Status) || input.Metadata != nil
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

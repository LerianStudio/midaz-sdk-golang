// Package models defines the data models used by the Midaz SDK.
package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
)

const maxOrganizationFieldLength = 256

// Organization is the SDK-native organization response (Track 7E — audit 7.1).
type Organization struct {
	ID                   string         `json:"id" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`
	ParentOrganizationID *string        `json:"parentOrganizationId" format:"uuid"`
	LegalName            string         `json:"legalName" example:"Lerian Financial Services Ltd." maxLength:"256"`
	DoingBusinessAs      *string        `json:"doingBusinessAs" example:"Lerian FS" maxLength:"256"`
	LegalDocument        string         `json:"legalDocument" example:"123456789012345" maxLength:"256"`
	Address              Address        `json:"address"`
	Status               Status         `json:"status"`
	CreatedAt            time.Time      `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	UpdatedAt            time.Time      `json:"updatedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	DeletedAt            *time.Time     `json:"deletedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

// CreateOrganizationInput is the SDK-native organization creation payload.
type CreateOrganizationInput struct {
	LegalName            string         `json:"legalName" example:"Lerian Financial Services Ltd." maxLength:"256"`
	ParentOrganizationID *string        `json:"parentOrganizationId" format:"uuid"`
	DoingBusinessAs      *string        `json:"doingBusinessAs" example:"Lerian FS" maxLength:"256"`
	LegalDocument        string         `json:"legalDocument" example:"123456789012345" maxLength:"256"`
	Address              Address        `json:"address"`
	Status               Status         `json:"status"`
	Metadata             map[string]any `json:"metadata"`
}

// Validate validates the CreateOrganizationInput fields.
func (input *CreateOrganizationInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.LegalName == "" {
		return errors.New("legalName is required")
	}

	if input.LegalDocument == "" {
		return errors.New("legalDocument is required")
	}

	if err := validateOrganizationStringLength("legalName", input.LegalName); err != nil {
		return err
	}

	if err := validateOrganizationStringLength("legalDocument", input.LegalDocument); err != nil {
		return err
	}

	if input.DoingBusinessAs != nil {
		if err := validateOrganizationStringLength("doingBusinessAs", *input.DoingBusinessAs); err != nil {
			return err
		}
	}

	if err := validateOptionalOrganizationUUID("parentOrganizationId", input.ParentOrganizationID); err != nil {
		return err
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

// MarshalJSON omits optional create fields when callers leave them unset.
func (input *CreateOrganizationInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "legalName", input.LegalName)
	addStringField(fields, "legalDocument", input.LegalDocument)
	addStringPtrField(fields, "parentOrganizationId", input.ParentOrganizationID)
	addStringPtrField(fields, "doingBusinessAs", input.DoingBusinessAs)

	if !input.Address.IsEmpty() {
		fields["address"] = input.Address
	}

	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

// UpdateOrganizationInput is the SDK-native organization patch payload.
type UpdateOrganizationInput struct {
	LegalName            string         `json:"legalName" example:"Lerian Financial Group Ltd." maxLength:"256"`
	ParentOrganizationID *string        `json:"parentOrganizationId" format:"uuid"`
	DoingBusinessAs      *string        `json:"doingBusinessAs" example:"Lerian Group" maxLength:"256"`
	Address              Address        `json:"address"`
	Status               Status         `json:"status"`
	Metadata             map[string]any `json:"metadata"`
}

// Validate validates the UpdateOrganizationInput fields.
func (input *UpdateOrganizationInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	if err := validateOrganizationStringLength("legalName", input.LegalName); err != nil {
		return err
	}

	if input.DoingBusinessAs != nil {
		if err := validateOrganizationStringLength("doingBusinessAs", *input.DoingBusinessAs); err != nil {
			return err
		}
	}

	if err := validateOptionalOrganizationUUID("parentOrganizationId", input.ParentOrganizationID); err != nil {
		return err
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	return nil
}

func validateOrganizationStringLength(field, value string) error {
	if value != "" && utf8.RuneCountInString(value) > maxOrganizationFieldLength {
		return fmt.Errorf("%s must be at most %d characters", field, maxOrganizationFieldLength)
	}

	return nil
}

func validateOptionalOrganizationUUID(field string, value *string) error {
	if value == nil || *value == "" {
		return nil
	}

	if !validation.IsValidUUID(*value) {
		return fmt.Errorf("%s must be a valid UUID", field)
	}

	return nil
}

func (input *UpdateOrganizationInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.LegalName != "" ||
		input.ParentOrganizationID != nil ||
		input.DoingBusinessAs != nil ||
		!input.Address.IsEmpty() ||
		!IsStatusEmpty(input.Status) ||
		input.Metadata != nil
}

// MarshalJSON emits only fields explicitly set on the SDK PATCH input.
func (input *UpdateOrganizationInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	fields := map[string]any{}
	addStringField(fields, "legalName", input.LegalName)
	addStringPtrField(fields, "parentOrganizationId", input.ParentOrganizationID)
	addStringPtrField(fields, "doingBusinessAs", input.DoingBusinessAs)

	if !input.Address.IsEmpty() {
		fields["address"] = input.Address
	}

	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

// NewCreateOrganizationInput creates a new CreateOrganizationInput with required fields.
func NewCreateOrganizationInput(legalName, legalDocument string) *CreateOrganizationInput {
	return &CreateOrganizationInput{
		LegalName:     legalName,
		LegalDocument: legalDocument,
	}
}

// WithDoingBusinessAs sets the doing business as name.
func (input *CreateOrganizationInput) WithDoingBusinessAs(dba string) *CreateOrganizationInput {
	if input == nil {
		return nil
	}

	input.DoingBusinessAs = &dba

	return input
}

// WithLegalDocument sets the legal document number.
func (input *CreateOrganizationInput) WithLegalDocument(doc string) *CreateOrganizationInput {
	if input == nil {
		return nil
	}

	input.LegalDocument = doc

	return input
}

// WithStatus sets the organization status.
func (input *CreateOrganizationInput) WithStatus(status Status) *CreateOrganizationInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

// WithAddress sets the organization address.
func (input *CreateOrganizationInput) WithAddress(address Address) *CreateOrganizationInput {
	if input == nil {
		return nil
	}

	input.Address = address

	return input
}

// WithMetadata sets the organization metadata.
func (input *CreateOrganizationInput) WithMetadata(metadata map[string]any) *CreateOrganizationInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// NewUpdateOrganizationInput creates a new UpdateOrganizationInput.
func NewUpdateOrganizationInput() *UpdateOrganizationInput {
	return &UpdateOrganizationInput{}
}

// WithLegalName sets the legal name for update.
func (input *UpdateOrganizationInput) WithLegalName(legalName string) *UpdateOrganizationInput {
	if input == nil {
		return nil
	}

	input.LegalName = legalName

	return input
}

// WithMetadata sets the metadata for update.
//
// Track 7F (audit 7.14) — the v2 *Update-suffixed siblings
// (WithUpdateMetadata, WithDoingBusinessAsUpdate, WithAddressUpdate,
// WithStatusUpdate) have been retired. They duplicated the canonical
// non-suffixed setters with no semantic difference. Callers using the
// suffixed names should switch to the canonical setters below.
func (input *UpdateOrganizationInput) WithMetadata(metadata map[string]any) *UpdateOrganizationInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithDoingBusinessAs sets the doing business as name for update.
func (input *UpdateOrganizationInput) WithDoingBusinessAs(dba string) *UpdateOrganizationInput {
	if input == nil {
		return nil
	}

	input.DoingBusinessAs = &dba

	return input
}

// WithAddress sets the organization address for update.
func (input *UpdateOrganizationInput) WithAddress(address Address) *UpdateOrganizationInput {
	if input == nil {
		return nil
	}

	input.Address = address

	return input
}

// WithStatus sets the organization status for update.
func (input *UpdateOrganizationInput) WithStatus(status Status) *UpdateOrganizationInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

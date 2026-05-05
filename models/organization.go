// Package models defines the data models used by the Midaz SDK.
package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

const maxOrganizationFieldLength = 256

// Organization is an alias for mmodel.Organization to maintain compatibility while using midaz entities.
type Organization = mmodel.Organization

// CreateOrganizationInput wraps mmodel.CreateOrganizationInput to maintain compatibility while using midaz entities.
type CreateOrganizationInput struct {
	mmodel.CreateOrganizationInput
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

// ToMmodelCreateOrganizationInput converts the SDK CreateOrganizationInput to mmodel CreateOrganizationInput.
func (input *CreateOrganizationInput) ToMmodelCreateOrganizationInput() *mmodel.CreateOrganizationInput {
	if input == nil {
		return nil
	}

	return &input.CreateOrganizationInput
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
		fields["address"] = Address(input.Address)
	}

	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

// UpdateOrganizationInput wraps mmodel.UpdateOrganizationInput to maintain compatibility while using midaz entities.
type UpdateOrganizationInput struct {
	mmodel.UpdateOrganizationInput
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

// ToMmodelUpdateOrganizationInput converts the SDK UpdateOrganizationInput to mmodel UpdateOrganizationInput.
func (input *UpdateOrganizationInput) ToMmodelUpdateOrganizationInput() *mmodel.UpdateOrganizationInput {
	if input == nil {
		return nil
	}

	return &input.UpdateOrganizationInput
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
		fields["address"] = Address(input.Address)
	}

	addStatusField(fields, input.Status)
	addMetadataField(fields, input.Metadata)

	return json.Marshal(fields)
}

// NewCreateOrganizationInput creates a new CreateOrganizationInput with required fields.
func NewCreateOrganizationInput(legalName, legalDocument string) *CreateOrganizationInput {
	return &CreateOrganizationInput{
		CreateOrganizationInput: mmodel.CreateOrganizationInput{
			LegalName:     legalName,
			LegalDocument: legalDocument,
		},
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

	input.Address = mmodel.Address(address)

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
	return &UpdateOrganizationInput{
		UpdateOrganizationInput: mmodel.UpdateOrganizationInput{},
	}
}

// WithLegalName sets the legal name for update.
func (input *UpdateOrganizationInput) WithLegalName(legalName string) *UpdateOrganizationInput {
	if input == nil {
		return nil
	}

	input.LegalName = legalName

	return input
}

// WithUpdateMetadata sets the metadata for update.
func (input *UpdateOrganizationInput) WithUpdateMetadata(metadata map[string]any) *UpdateOrganizationInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithMetadata sets the metadata for update.
func (input *UpdateOrganizationInput) WithMetadata(metadata map[string]any) *UpdateOrganizationInput {
	return input.WithUpdateMetadata(metadata)
}

// WithDoingBusinessAs sets the doing business as name for update.
func (input *UpdateOrganizationInput) WithDoingBusinessAs(dba string) *UpdateOrganizationInput {
	return input.WithDoingBusinessAsUpdate(dba)
}

// WithAddress sets the organization address for update.
func (input *UpdateOrganizationInput) WithAddress(address Address) *UpdateOrganizationInput {
	return input.WithAddressUpdate(address)
}

// WithStatus sets the organization status for update.
func (input *UpdateOrganizationInput) WithStatus(status Status) *UpdateOrganizationInput {
	return input.WithStatusUpdate(status)
}

// WithDoingBusinessAsUpdate sets the doing business as name for update.
func (input *UpdateOrganizationInput) WithDoingBusinessAsUpdate(dba string) *UpdateOrganizationInput {
	if input == nil {
		return nil
	}

	input.DoingBusinessAs = &dba

	return input
}

// WithAddressUpdate sets the organization address for update.
func (input *UpdateOrganizationInput) WithAddressUpdate(address Address) *UpdateOrganizationInput {
	if input == nil {
		return nil
	}

	input.Address = mmodel.Address(address)

	return input
}

// WithStatusUpdate sets the organization status for update.
func (input *UpdateOrganizationInput) WithStatusUpdate(status Status) *UpdateOrganizationInput {
	if input == nil {
		return nil
	}

	input.Status = status

	return input
}

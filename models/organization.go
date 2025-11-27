// Package models defines the data models used by the Midaz SDK.
package models

import (
	"errors"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
)

// Organization is an alias for mmodel.Organization to maintain compatibility while using midaz entities.
type Organization = mmodel.Organization

// CreateOrganizationInput wraps mmodel.CreateOrganizationInput to maintain compatibility while using midaz entities.
type CreateOrganizationInput struct {
	mmodel.CreateOrganizationInput
}

// Validate validates the CreateOrganizationInput fields.
func (input *CreateOrganizationInput) Validate() error {
	if input.LegalName == "" {
		return errors.New("legalName is required")
	}

	return nil
}

// ToMmodelCreateOrganizationInput converts the SDK CreateOrganizationInput to mmodel CreateOrganizationInput.
func (input *CreateOrganizationInput) ToMmodelCreateOrganizationInput() *mmodel.CreateOrganizationInput {
	return &input.CreateOrganizationInput
}

// UpdateOrganizationInput wraps mmodel.UpdateOrganizationInput to maintain compatibility while using midaz entities.
type UpdateOrganizationInput struct {
	mmodel.UpdateOrganizationInput
}

// Validate validates the UpdateOrganizationInput fields.
func (*UpdateOrganizationInput) Validate() error {
	// For updates, fields are optional so validation is minimal
	return nil
}

// ToMmodelUpdateOrganizationInput converts the SDK UpdateOrganizationInput to mmodel UpdateOrganizationInput.
func (input *UpdateOrganizationInput) ToMmodelUpdateOrganizationInput() *mmodel.UpdateOrganizationInput {
	return &input.UpdateOrganizationInput
}

// NewCreateOrganizationInput creates a new CreateOrganizationInput with required fields.
func NewCreateOrganizationInput(legalName string) *CreateOrganizationInput {
	return &CreateOrganizationInput{
		CreateOrganizationInput: mmodel.CreateOrganizationInput{
			LegalName: legalName,
		},
	}
}

// WithDoingBusinessAs sets the doing business as name.
func (input *CreateOrganizationInput) WithDoingBusinessAs(dba string) *CreateOrganizationInput {
	input.DoingBusinessAs = &dba
	return input
}

// WithLegalDocument sets the legal document number.
func (input *CreateOrganizationInput) WithLegalDocument(doc string) *CreateOrganizationInput {
	input.LegalDocument = doc
	return input
}

// WithStatus sets the organization status.
func (input *CreateOrganizationInput) WithStatus(status Status) *CreateOrganizationInput {
	input.Status = status
	return input
}

// WithAddress sets the organization address.
func (input *CreateOrganizationInput) WithAddress(address Address) *CreateOrganizationInput {
	input.Address = mmodel.Address(address)
	return input
}

// WithMetadata sets the organization metadata.
func (input *CreateOrganizationInput) WithMetadata(metadata map[string]any) *CreateOrganizationInput {
	input.Metadata = metadata
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
	input.LegalName = legalName
	return input
}

// WithUpdateMetadata sets the metadata for update.
func (input *UpdateOrganizationInput) WithUpdateMetadata(metadata map[string]any) *UpdateOrganizationInput {
	input.Metadata = metadata
	return input
}

// WithDoingBusinessAsUpdate sets the doing business as name for update.
func (input *UpdateOrganizationInput) WithDoingBusinessAsUpdate(dba string) *UpdateOrganizationInput {
	input.DoingBusinessAs = dba
	return input
}

// WithAddressUpdate sets the organization address for update.
func (input *UpdateOrganizationInput) WithAddressUpdate(address Address) *UpdateOrganizationInput {
	input.Address = mmodel.Address(address)
	return input
}

// WithStatusUpdate sets the organization status for update.
func (input *UpdateOrganizationInput) WithStatusUpdate(status Status) *UpdateOrganizationInput {
	input.Status = status
	return input
}

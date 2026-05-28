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
//
// See also:
//   - [CreateOrganizationInput.Validate] — multi-field validation accumulator.
//   - [UpdateOrganizationInput] — partial-update shape.
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/entities.OrganizationsService.CreateOrganization]
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithIdempotencyKey] — make creation safe under retries.
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

	var errs validation.FieldErrors

	if input.LegalName == "" {
		errs.Append("legalName", "is required")
	} else {
		appendOrganizationStringLength(&errs, "legalName", input.LegalName)
	}

	if input.LegalDocument == "" {
		errs.Append("legalDocument", "is required")
	} else {
		appendOrganizationStringLength(&errs, "legalDocument", input.LegalDocument)
	}

	if input.DoingBusinessAs != nil {
		appendOrganizationStringLength(&errs, "doingBusinessAs", *input.DoingBusinessAs)
	}

	appendOrganizationOptionalUUID(&errs, "parentOrganizationId", input.ParentOrganizationID)

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
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
	Metadata             map[string]any `json:"metadata,omitempty"`
}

// Validate validates the UpdateOrganizationInput fields.
//
// Empty-payload check short-circuits — when nothing is being updated
// the request is rejected before per-field analysis. Otherwise all
// field-level violations are accumulated and surfaced together.
func (input *UpdateOrganizationInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	var errs validation.FieldErrors

	appendOrganizationStringLength(&errs, "legalName", input.LegalName)

	if input.DoingBusinessAs != nil {
		appendOrganizationStringLength(&errs, "doingBusinessAs", *input.DoingBusinessAs)
	}

	appendOrganizationOptionalUUID(&errs, "parentOrganizationId", input.ParentOrganizationID)

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// appendOrganizationStringLength records a length-bound violation onto
// errs when value exceeds maxOrganizationFieldLength. No-op for empty
// values (handled by required-field checks at the call site).
func appendOrganizationStringLength(errs *validation.FieldErrors, field, value string) {
	if value != "" && utf8.RuneCountInString(value) > maxOrganizationFieldLength {
		errs.Append(field, fmt.Sprintf("must be at most %d characters", maxOrganizationFieldLength))
	}
}

// appendOrganizationOptionalUUID records a UUID-format violation onto
// errs when a pointer is non-nil and non-empty but holds an invalid
// value. Both nil pointers and empty strings are no-ops here.
func appendOrganizationOptionalUUID(errs *validation.FieldErrors, field string, value *string) {
	if value == nil || *value == "" {
		return
	}

	if !validation.IsValidUUID(*value) {
		errs.Append(field, "must be a valid UUID")
	}
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

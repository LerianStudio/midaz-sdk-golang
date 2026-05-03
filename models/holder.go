package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation/core"
)

const (
	// HolderTypeNaturalPerson identifies an individual CRM holder.
	HolderTypeNaturalPerson = "NATURAL_PERSON"
	// HolderTypeLegalPerson identifies a company CRM holder.
	HolderTypeLegalPerson = "LEGAL_PERSON"
)

// CreateHolderInput is the payload for creating a CRM holder.
type CreateHolderInput struct {
	ExternalID    *string        `json:"externalId,omitempty"`
	Type          *string        `json:"type"`
	Name          string         `json:"name"`
	Document      string         `json:"document"`
	Addresses     *Addresses     `json:"addresses,omitempty"`
	Contact       *Contact       `json:"contact,omitempty"`
	NaturalPerson *NaturalPerson `json:"naturalPerson,omitempty"`
	LegalPerson   *LegalPerson   `json:"legalPerson,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// UpdateHolderInput is the payload for updating a CRM holder.
type UpdateHolderInput struct {
	ExternalID    *string        `json:"externalId,omitempty"`
	Name          *string        `json:"name,omitempty"`
	Addresses     *Addresses     `json:"addresses,omitempty"`
	Contact       *Contact       `json:"contact,omitempty"`
	NaturalPerson *NaturalPerson `json:"naturalPerson,omitempty"`
	LegalPerson   *LegalPerson   `json:"legalPerson,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	NullFields    []string       `json:"-"`
}

// NewCreateHolderInput creates a holder create payload with required fields set.
func NewCreateHolderInput(holderType, name, document string) *CreateHolderInput {
	return &CreateHolderInput{
		Type:     holderStringPtr(holderType),
		Name:     name,
		Document: document,
	}
}

// WithExternalID sets the holder external identifier.
func (input *CreateHolderInput) WithExternalID(externalID string) *CreateHolderInput {
	if input == nil {
		return nil
	}

	input.ExternalID = holderStringPtr(externalID)

	return input
}

// WithAddresses sets holder addresses.
func (input *CreateHolderInput) WithAddresses(addresses *Addresses) *CreateHolderInput {
	if input == nil {
		return nil
	}

	input.Addresses = addresses

	return input
}

// WithContact sets holder contact data.
func (input *CreateHolderInput) WithContact(contact *Contact) *CreateHolderInput {
	if input == nil {
		return nil
	}

	input.Contact = contact

	return input
}

// WithNaturalPerson sets natural-person holder details.
func (input *CreateHolderInput) WithNaturalPerson(naturalPerson *NaturalPerson) *CreateHolderInput {
	if input == nil {
		return nil
	}

	input.NaturalPerson = naturalPerson

	return input
}

// WithLegalPerson sets legal-person holder details.
func (input *CreateHolderInput) WithLegalPerson(legalPerson *LegalPerson) *CreateHolderInput {
	if input == nil {
		return nil
	}

	input.LegalPerson = legalPerson

	return input
}

// WithMetadata sets holder metadata.
func (input *CreateHolderInput) WithMetadata(metadata map[string]any) *CreateHolderInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// NewUpdateHolderInput creates an empty holder update payload.
func NewUpdateHolderInput() *UpdateHolderInput {
	return &UpdateHolderInput{}
}

// WithExternalID sets the holder external identifier for update.
func (input *UpdateHolderInput) WithExternalID(externalID string) *UpdateHolderInput {
	if input == nil {
		return nil
	}

	input.ExternalID = holderStringPtr(externalID)

	return input
}

// WithName sets the holder name for update.
func (input *UpdateHolderInput) WithName(name string) *UpdateHolderInput {
	if input == nil {
		return nil
	}

	input.Name = holderStringPtr(name)

	return input
}

// WithAddresses sets holder addresses for update.
func (input *UpdateHolderInput) WithAddresses(addresses *Addresses) *UpdateHolderInput {
	if input == nil {
		return nil
	}

	input.Addresses = addresses

	return input
}

// WithContact sets holder contact data for update.
func (input *UpdateHolderInput) WithContact(contact *Contact) *UpdateHolderInput {
	if input == nil {
		return nil
	}

	input.Contact = contact

	return input
}

// WithNaturalPerson sets natural-person holder details for update.
func (input *UpdateHolderInput) WithNaturalPerson(naturalPerson *NaturalPerson) *UpdateHolderInput {
	if input == nil {
		return nil
	}

	input.NaturalPerson = naturalPerson

	return input
}

// WithLegalPerson sets legal-person holder details for update.
func (input *UpdateHolderInput) WithLegalPerson(legalPerson *LegalPerson) *UpdateHolderInput {
	if input == nil {
		return nil
	}

	input.LegalPerson = legalPerson

	return input
}

// WithMetadata sets holder metadata for update.
func (input *UpdateHolderInput) WithMetadata(metadata map[string]any) *UpdateHolderInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithNullField marks one field for explicit null removal.
func (input *UpdateHolderInput) WithNullField(field string) *UpdateHolderInput {
	return input.WithNullFields(field)
}

func holderStringPtr(value string) *string {
	return &value
}

// Validate validates the CreateHolderInput fields.
func (input *CreateHolderInput) Validate() error {
	if input == nil {
		return errors.New("input is required")
	}

	if input.Type == nil || *input.Type == "" {
		return errors.New("type is required")
	}

	if !isValidHolderType(*input.Type) {
		return errors.New("type must be NATURAL_PERSON or LEGAL_PERSON")
	}

	if strings.TrimSpace(input.Name) == "" {
		return errors.New("name is required")
	}

	if strings.TrimSpace(input.Document) == "" {
		return errors.New("document is required")
	}

	if len(input.Metadata) > 0 {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

// WithNullFields marks fields that should be sent as explicit JSON null in PATCH requests.
func (input *UpdateHolderInput) WithNullFields(fields ...string) *UpdateHolderInput {
	if input == nil {
		return nil
	}

	input.NullFields = append(input.NullFields, fields...)

	return input
}

// MarshalJSON emits only set fields plus fields explicitly marked for null removal.
func (input UpdateHolderInput) MarshalJSON() ([]byte, error) {
	if err := input.validateNullFieldConflicts(); err != nil {
		return nil, err
	}

	payload := map[string]any{}
	if input.ExternalID != nil {
		payload["externalId"] = input.ExternalID
	}

	if input.Name != nil {
		payload["name"] = input.Name
	}

	if input.Addresses != nil {
		payload["addresses"] = input.Addresses
	}

	if input.Contact != nil {
		payload["contact"] = input.Contact
	}

	if input.NaturalPerson != nil {
		payload["naturalPerson"] = input.NaturalPerson
	}

	if input.LegalPerson != nil {
		payload["legalPerson"] = input.LegalPerson
	}

	if input.Metadata != nil {
		payload["metadata"] = input.Metadata
	}

	for _, field := range input.NullFields {
		field = strings.TrimSpace(field)
		if !validHolderNullFields[field] {
			return nil, fmt.Errorf("unsupported null field %q", field)
		}

		payload[field] = nil
	}

	if len(payload) == 0 {
		return nil, errors.New("empty update payload not allowed")
	}

	return json.Marshal(payload)
}

func (input *UpdateHolderInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.ExternalID != nil ||
		input.Name != nil ||
		input.Addresses != nil ||
		input.Contact != nil ||
		input.NaturalPerson != nil ||
		input.LegalPerson != nil ||
		input.Metadata != nil ||
		len(input.NullFields) > 0
}

func (input *UpdateHolderInput) validateNullFieldConflicts() error {
	if input == nil {
		return nil
	}

	setFields := map[string]bool{
		"externalId":    input.ExternalID != nil,
		"name":          input.Name != nil,
		"addresses":     input.Addresses != nil,
		"contact":       input.Contact != nil,
		"naturalPerson": input.NaturalPerson != nil,
		"legalPerson":   input.LegalPerson != nil,
		"metadata":      input.Metadata != nil,
	}

	for _, field := range input.NullFields {
		field = strings.TrimSpace(field)
		if setFields[field] {
			return fmt.Errorf("field %q cannot be set and cleared in the same request", field)
		}
	}

	return nil
}

var validHolderNullFields = map[string]bool{
	"externalId":    true,
	"name":          true,
	"addresses":     true,
	"contact":       true,
	"naturalPerson": true,
	"legalPerson":   true,
	"metadata":      true,
}

func isValidHolderType(holderType string) bool {
	switch holderType {
	case HolderTypeNaturalPerson, HolderTypeLegalPerson:
		return true
	default:
		return false
	}
}

func validateCRMNullFields(fields []string, allowed map[string]bool) error {
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			return errors.New("null field cannot be empty")
		}

		if !allowed[field] {
			return fmt.Errorf("unsupported null field %q", field)
		}
	}

	return nil
}

// Validate validates the UpdateHolderInput fields.
func (input *UpdateHolderInput) Validate() error {
	if input == nil {
		return errors.New("input is required")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	if len(input.Metadata) > 0 {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	if err := validateCRMNullFields(input.NullFields, validHolderNullFields); err != nil {
		return err
	}

	return input.validateNullFieldConflicts()
}

// Holder represents a CRM holder.
type Holder struct {
	ID            *uuid.UUID     `json:"id,omitempty"`
	ExternalID    *string        `json:"externalId,omitempty"`
	Type          *string        `json:"type,omitempty"`
	Name          *string        `json:"name,omitempty"`
	Document      *string        `json:"document,omitempty"`
	Addresses     *Addresses     `json:"addresses,omitempty"`
	Contact       *Contact       `json:"contact,omitempty"`
	NaturalPerson *NaturalPerson `json:"naturalPerson,omitempty"`
	LegalPerson   *LegalPerson   `json:"legalPerson,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     *time.Time     `json:"deletedAt"`
}

// Addresses stores primary and additional holder addresses.
type Addresses struct {
	Primary     *Address `json:"primary,omitempty"`
	Additional1 *Address `json:"additional1,omitempty"`
	Additional2 *Address `json:"additional2,omitempty"`
}

// Contact stores holder contact information.
type Contact struct {
	PrimaryEmail   *string `json:"primaryEmail,omitempty"`
	SecondaryEmail *string `json:"secondaryEmail,omitempty"`
	MobilePhone    *string `json:"mobilePhone,omitempty"`
	OtherPhone     *string `json:"otherPhone,omitempty"`
}

// NaturalPerson stores natural-person holder details.
type NaturalPerson struct {
	FavoriteName *string `json:"favoriteName,omitempty"`
	SocialName   *string `json:"socialName,omitempty"`
	Gender       *string `json:"gender,omitempty"`
	BirthDate    *string `json:"birthDate,omitempty"`
	CivilStatus  *string `json:"civilStatus,omitempty"`
	Nationality  *string `json:"nationality,omitempty"`
	MotherName   *string `json:"motherName,omitempty"`
	FatherName   *string `json:"fatherName,omitempty"`
	Status       *string `json:"status,omitempty"`
}

// LegalPerson stores legal-person holder details.
type LegalPerson struct {
	TradeName      *string         `json:"tradeName,omitempty"`
	Activity       *string         `json:"activity,omitempty"`
	Type           *string         `json:"type,omitempty"`
	FoundingDate   *string         `json:"foundingDate,omitempty"`
	Size           *string         `json:"size,omitempty"`
	Status         *string         `json:"status,omitempty"`
	Representative *Representative `json:"representative,omitempty"`
}

// Representative stores legal-person representative data.
type Representative struct {
	Name     *string `json:"name,omitempty"`
	Document *string `json:"document,omitempty"`
	Email    *string `json:"email,omitempty"`
	Role     *string `json:"role,omitempty"`
}

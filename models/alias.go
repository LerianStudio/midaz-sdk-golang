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

// RelatedPartyRolePrimaryHolder identifies the primary holder related-party role.
const RelatedPartyRolePrimaryHolder = "PRIMARY_HOLDER"

// RelatedPartyRoleLegalRepresentative identifies the legal representative related-party role.
const RelatedPartyRoleLegalRepresentative = "LEGAL_REPRESENTATIVE"

// RelatedPartyRoleResponsibleParty identifies the responsible party related-party role.
const RelatedPartyRoleResponsibleParty = "RESPONSIBLE_PARTY"

// RegulatoryFields contains regulatory-specific fields for an alias.
type RegulatoryFields struct {
	ParticipantDocument *string `json:"participantDocument,omitempty"`
}

// RelatedParty represents a party related to an alias.
type RelatedParty struct {
	ID        *uuid.UUID `json:"id,omitempty"`
	Document  string     `json:"document"`
	Name      string     `json:"name"`
	Role      string     `json:"role"`
	StartDate string     `json:"startDate"`
	EndDate   *string    `json:"endDate,omitempty"`
}

// CreateAliasInput is the payload for creating an account alias.
type CreateAliasInput struct {
	LedgerID         string            `json:"ledgerId"`
	AccountID        string            `json:"accountId"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
	BankingDetails   *BankingDetails   `json:"bankingDetails,omitempty"`
	RegulatoryFields *RegulatoryFields `json:"regulatoryFields,omitempty"`
	RelatedParties   []*RelatedParty   `json:"relatedParties,omitempty"`
}

// UpdateAliasInput is the payload for updating an account alias.
type UpdateAliasInput struct {
	Metadata         map[string]any    `json:"metadata,omitempty"`
	BankingDetails   *BankingDetails   `json:"bankingDetails,omitempty"`
	RegulatoryFields *RegulatoryFields `json:"regulatoryFields,omitempty"`
	RelatedParties   []*RelatedParty   `json:"relatedParties,omitempty"`
	NullFields       []string          `json:"-"`
}

// NewCreateAliasInput creates an alias create payload with required fields set.
func NewCreateAliasInput(ledgerID, accountID string) *CreateAliasInput {
	return &CreateAliasInput{
		LedgerID:  ledgerID,
		AccountID: accountID,
	}
}

// WithMetadata sets alias metadata.
func (input *CreateAliasInput) WithMetadata(metadata map[string]any) *CreateAliasInput {
	if input == nil {
		return nil
	}

	input.Metadata = metadata

	return input
}

// WithBankingDetails sets alias banking details.
func (input *CreateAliasInput) WithBankingDetails(bankingDetails *BankingDetails) *CreateAliasInput {
	if input == nil {
		return nil
	}

	input.BankingDetails = bankingDetails

	return input
}

// WithRegulatoryFields sets alias regulatory fields.
func (input *CreateAliasInput) WithRegulatoryFields(regulatoryFields *RegulatoryFields) *CreateAliasInput {
	if input == nil {
		return nil
	}

	input.RegulatoryFields = regulatoryFields

	return input
}

// WithRelatedParties sets related parties for alias creation.
func (input *CreateAliasInput) WithRelatedParties(relatedParties []*RelatedParty) *CreateAliasInput {
	if input == nil {
		return nil
	}

	input.RelatedParties = relatedParties

	return input
}

// NewUpdateAliasInput creates an empty alias update payload.
func NewUpdateAliasInput() *UpdateAliasInput {
	return &UpdateAliasInput{}
}

// WithMetadata sets alias metadata for update.
func (input *UpdateAliasInput) WithMetadata(metadata map[string]any) *UpdateAliasInput {
	if input == nil {
		return nil
	}

	input.Metadata = metadata

	return input
}

// WithBankingDetails sets alias banking details for update.
func (input *UpdateAliasInput) WithBankingDetails(bankingDetails *BankingDetails) *UpdateAliasInput {
	if input == nil {
		return nil
	}

	input.BankingDetails = bankingDetails

	return input
}

// WithRegulatoryFields sets alias regulatory fields for update.
func (input *UpdateAliasInput) WithRegulatoryFields(regulatoryFields *RegulatoryFields) *UpdateAliasInput {
	if input == nil {
		return nil
	}

	input.RegulatoryFields = regulatoryFields

	return input
}

// WithRelatedParties appends related parties to the existing alias on update.
func (input *UpdateAliasInput) WithRelatedParties(relatedParties []*RelatedParty) *UpdateAliasInput {
	if input == nil {
		return nil
	}

	input.RelatedParties = relatedParties

	return input
}

// Validate validates the CreateAliasInput fields.
func (input *CreateAliasInput) Validate() error {
	if input == nil {
		return errors.New("input is required")
	}

	if strings.TrimSpace(input.LedgerID) == "" {
		return errors.New("ledgerId is required")
	}

	if strings.TrimSpace(input.AccountID) == "" {
		return errors.New("accountId is required")
	}

	if err := validateAliasMetadata(input.Metadata); err != nil {
		return err
	}

	return validateRelatedParties(input.RelatedParties)
}

// WithNullFields marks fields that should be sent as explicit JSON null in PATCH requests.
func (input *UpdateAliasInput) WithNullFields(fields ...string) *UpdateAliasInput {
	if input == nil {
		return nil
	}

	input.NullFields = append(input.NullFields, fields...)

	return input
}

// MarshalJSON emits only set fields plus fields explicitly marked for null removal.
func (input UpdateAliasInput) MarshalJSON() ([]byte, error) {
	if err := input.validateNullFieldConflicts(); err != nil {
		return nil, err
	}

	payload := map[string]any{}
	if input.Metadata != nil {
		payload["metadata"] = input.Metadata
	}

	if input.BankingDetails != nil {
		payload["bankingDetails"] = input.BankingDetails
	}

	if input.RegulatoryFields != nil {
		payload["regulatoryFields"] = input.RegulatoryFields
	}

	if input.RelatedParties != nil {
		payload["relatedParties"] = input.RelatedParties
	}

	for _, field := range input.NullFields {
		field = strings.TrimSpace(field)
		if !validAliasNullFields[field] {
			return nil, fmt.Errorf("unsupported null field %q", field)
		}

		payload[field] = nil
	}

	if len(payload) == 0 {
		return nil, errors.New("empty update payload not allowed")
	}

	return json.Marshal(payload)
}

func (input *UpdateAliasInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.Metadata != nil ||
		input.BankingDetails != nil ||
		input.RegulatoryFields != nil ||
		input.RelatedParties != nil ||
		len(input.NullFields) > 0
}

func (input *UpdateAliasInput) validateNullFieldConflicts() error {
	if input == nil {
		return nil
	}

	setFields := map[string]bool{
		"metadata":         input.Metadata != nil,
		"bankingDetails":   input.BankingDetails != nil,
		"regulatoryFields": input.RegulatoryFields != nil,
		"relatedParties":   input.RelatedParties != nil,
	}

	for _, field := range input.NullFields {
		field = strings.TrimSpace(field)
		if setFields[field] {
			return fmt.Errorf("field %q cannot be set and cleared in the same request", field)
		}
	}

	return nil
}

var validAliasNullFields = map[string]bool{
	"metadata":         true,
	"bankingDetails":   true,
	"regulatoryFields": true,
	"relatedParties":   true,
}

// Validate validates the UpdateAliasInput fields.
func (input *UpdateAliasInput) Validate() error {
	if input == nil {
		return errors.New("input is required")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	if err := validateAliasMetadata(input.Metadata); err != nil {
		return err
	}

	if err := validateRelatedParties(input.RelatedParties); err != nil {
		return err
	}

	if err := validateCRMNullFields(input.NullFields, validAliasNullFields); err != nil {
		return err
	}

	return input.validateNullFieldConflicts()
}

func validateAliasMetadata(metadata map[string]any) error {
	if len(metadata) == 0 {
		return nil
	}

	if err := core.ValidateMetadata(metadata); err != nil {
		return fmt.Errorf("invalid metadata: %w", err)
	}

	return nil
}

func validateRelatedParties(parties []*RelatedParty) error {
	for i, party := range parties {
		if party == nil {
			return fmt.Errorf("relatedParties[%d] is required", i)
		}

		if err := validateRelatedPartyRequiredFields(i, party); err != nil {
			return err
		}

		if err := validateRelatedPartyRole(i, party.Role); err != nil {
			return err
		}

		if err := validateRelatedPartyDates(i, party.StartDate, party.EndDate); err != nil {
			return err
		}
	}

	return nil
}

func validateRelatedPartyRequiredFields(index int, party *RelatedParty) error {
	requiredFields := []struct {
		name  string
		value string
	}{
		{name: "document", value: party.Document},
		{name: "name", value: party.Name},
		{name: "role", value: party.Role},
	}

	for _, field := range requiredFields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("relatedParties[%d].%s is required", index, field.name)
		}
	}

	return nil
}

func validateRelatedPartyRole(index int, role string) error {
	switch role {
	case RelatedPartyRolePrimaryHolder, RelatedPartyRoleLegalRepresentative, RelatedPartyRoleResponsibleParty:
		return nil
	default:
		return fmt.Errorf("relatedParties[%d].role must be PRIMARY_HOLDER, LEGAL_REPRESENTATIVE, or RESPONSIBLE_PARTY", index)
	}
}

func validateRelatedPartyDates(index int, startDateValue string, endDateValue *string) error {
	trimmedStartDate := strings.TrimSpace(startDateValue)
	if trimmedStartDate == "" {
		return fmt.Errorf("relatedParties[%d].startDate is required", index)
	}

	if startDateValue != trimmedStartDate {
		return fmt.Errorf("relatedParties[%d].startDate must not contain leading/trailing whitespace", index)
	}

	startDate, err := parseCRMDate(startDateValue)
	if err != nil {
		return fmt.Errorf("relatedParties[%d].startDate must be YYYY-MM-DD or RFC3339", index)
	}

	return validateRelatedPartyEndDate(index, startDate, endDateValue)
}

func validateRelatedPartyEndDate(index int, startDate time.Time, endDateValue *string) error {
	if endDateValue == nil {
		return nil
	}

	if *endDateValue != strings.TrimSpace(*endDateValue) {
		return fmt.Errorf("relatedParties[%d].endDate must not contain leading/trailing whitespace", index)
	}

	endDate, err := parseCRMDate(*endDateValue)
	if err != nil {
		return fmt.Errorf("relatedParties[%d].endDate must be YYYY-MM-DD or RFC3339", index)
	}

	if endDate.Before(startDate) {
		return fmt.Errorf("relatedParties[%d].endDate must not be before startDate", index)
	}

	return nil
}

func parseCRMDate(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		return parsed, nil
	}

	return time.Parse(time.RFC3339, trimmed)
}

// Alias represents a CRM account alias.
type Alias struct {
	ID               *uuid.UUID        `json:"id,omitempty"`
	Document         *string           `json:"document,omitempty"`
	Type             *string           `json:"type,omitempty"`
	LedgerID         *string           `json:"ledgerId"`
	AccountID        *string           `json:"accountId"`
	HolderID         *uuid.UUID        `json:"holderId"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
	BankingDetails   *BankingDetails   `json:"bankingDetails,omitempty"`
	RegulatoryFields *RegulatoryFields `json:"regulatoryFields,omitempty"`
	RelatedParties   []*RelatedParty   `json:"relatedParties,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
	DeletedAt        *time.Time        `json:"deletedAt"`
}

// BankingDetails stores account banking details associated with an alias.
type BankingDetails struct {
	Branch      *string `json:"branch,omitempty"`
	Account     *string `json:"account,omitempty"`
	Type        *string `json:"type,omitempty"`
	OpeningDate *string `json:"openingDate,omitempty"`
	ClosingDate *string `json:"closingDate,omitempty"`
	IBAN        *string `json:"iban,omitempty"`
	CountryCode *string `json:"countryCode,omitempty"`
	BankID      *string `json:"bankId,omitempty"`
}

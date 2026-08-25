package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// This file holds the CRM value types that more than one CRM resource shares:
// banking details, regulatory fields, and related parties. They arrived with the
// alias resource, which Midaz has since renamed to "instruments" and serves only
// on /v2; the alias service is gone, but holders, instruments, and the
// holder-account composition endpoint all still carry these three shapes.

// RelatedPartyRolePrimaryHolder identifies the primary holder related-party role.
const RelatedPartyRolePrimaryHolder = "PRIMARY_HOLDER"

// RelatedPartyRoleLegalRepresentative identifies the legal representative related-party role.
const RelatedPartyRoleLegalRepresentative = "LEGAL_REPRESENTATIVE"

// RelatedPartyRoleResponsibleParty identifies the responsible party related-party role.
const RelatedPartyRoleResponsibleParty = "RESPONSIBLE_PARTY"

// RegulatoryFields contains regulatory-specific fields for a CRM instrument.
type RegulatoryFields struct {
	ParticipantDocument *string `json:"participantDocument,omitempty"`
}

// RelatedParty represents a party related to a CRM instrument.
type RelatedParty struct {
	ID        *uuid.UUID `json:"id,omitempty"`
	Document  string     `json:"document"`
	Name      string     `json:"name"`
	Role      string     `json:"role"`
	StartDate string     `json:"startDate"`
	EndDate   *string    `json:"endDate,omitempty"`
}

// BankingDetails stores the account banking details a CRM instrument carries.
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

// cloneRelatedParties returns an independent copy of parties. Each non-nil
// element is deep-copied including its pointer fields (ID *uuid.UUID and
// EndDate *string), so subsequent mutations to the source slice — or to
// the pointed-at values — cannot leak into the clone. The previous shallow
// copy aliased these pointers, which made "I changed party.EndDate on my
// input and the saved entity changed too" a real bug.
func cloneRelatedParties(parties []*RelatedParty) []*RelatedParty {
	if parties == nil {
		return nil
	}

	clone := make([]*RelatedParty, len(parties))
	for i, party := range parties {
		if party == nil {
			continue
		}

		partyCopy := *party

		if party.ID != nil {
			idCopy := *party.ID
			partyCopy.ID = &idCopy
		}

		if party.EndDate != nil {
			endDateCopy := *party.EndDate
			partyCopy.EndDate = &endDateCopy
		}

		clone[i] = &partyCopy
	}

	return clone
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

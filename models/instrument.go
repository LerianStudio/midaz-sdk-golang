// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/validation/core"
)

// Instrument is the SDK-native ledger-plane instrument response.
//
// It mirrors the generated internal/genledger Instrument wire shape but is
// expressed in public models.* types: uuid.UUID instead of the generated
// openapi UUID alias, and the shared BankingDetails / RegulatoryFields /
// RelatedParty types instead of their generated twins. HolderId is required
// (non-pointer) because an instrument always belongs to a holder.
type Instrument struct {
	AccountID        *uuid.UUID        `json:"accountId"`
	BankingDetails   *BankingDetails   `json:"bankingDetails,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	DeletedAt        *time.Time        `json:"deletedAt"`
	Document         *string           `json:"document,omitempty"`
	HolderID         uuid.UUID         `json:"holderId"`
	ID               *uuid.UUID        `json:"id,omitempty"`
	LedgerID         *uuid.UUID        `json:"ledgerId"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
	RegulatoryFields *RegulatoryFields `json:"regulatoryFields,omitempty"`
	RelatedParties   *[]RelatedParty   `json:"relatedParties"`
	Type             *string           `json:"type,omitempty"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}

// CreateInstrumentInput is the payload for creating a ledger-plane instrument.
//
// It mirrors the server contract exactly, because the facade marshals this
// struct straight to the wire: the endpoint declares
// additionalProperties: false, so an extra field is not ignored — it makes the
// whole request fail. The contract names six properties and requires four of
// them (ledgerId, accountId, bankingDetails, metadata).
//
// The holder is path-sourced, so it does NOT appear here. The ledger and the
// account do: an instrument belongs to an account, and the server makes the
// caller name which one rather than inferring it.
//
// An earlier version of this struct carried `type` and `document` — neither of
// which the endpoint accepts — and carried neither identifier, both of which it
// requires. No body it produced could be created by any server, which is why
// removing those two fields breaks no working caller.
type CreateInstrumentInput struct {
	LedgerID         string            `json:"ledgerId"`
	AccountID        string            `json:"accountId"`
	BankingDetails   *BankingDetails   `json:"bankingDetails"`
	Metadata         map[string]any    `json:"metadata"`
	RegulatoryFields *RegulatoryFields `json:"regulatoryFields,omitempty"`
	RelatedParties   []*RelatedParty   `json:"relatedParties,omitempty"`
}

// UpdateInstrumentInput is the PATCH payload for updating an instrument.
//
// It mirrors the server contract, which declares additionalProperties: false
// over exactly {bankingDetails, metadata, regulatoryFields, relatedParties} and
// marks metadata and bankingDetails REQUIRED. Required-on-PATCH is the server's
// choice, not this SDK's reading of it: the endpoint refuses a partial update
// that omits either, so Validate refuses one too rather than letting the caller
// discover it as a 422.
//
// It also used to carry a `document` field the endpoint has no slot for. No
// request setting it could succeed anywhere, which is why removing it breaks no
// working caller — the same reason the create payload lost its two phantoms.
//
// MarshalJSON emits only set fields plus fields explicitly marked for null
// removal (UpdateHolderInput/UpdateAliasInput pattern). Empty-payload
// rejection lives in Validate(), not MarshalJSON.
type UpdateInstrumentInput struct {
	BankingDetails   *BankingDetails   `json:"bankingDetails"`
	Metadata         map[string]any    `json:"metadata"`
	RegulatoryFields *RegulatoryFields `json:"regulatoryFields,omitempty"`
	RelatedParties   []*RelatedParty   `json:"relatedParties,omitempty"`
	NullFields       []string          `json:"-"`
}

// NewCreateInstrumentInput creates an instrument create payload with the two
// required identifiers set: the ledger the instrument belongs to and the
// account it is an instrument of.
//
// Banking details and metadata are required too — the contract lists all four —
// so a payload straight out of this constructor does not yet Validate. Add them
// with WithBankingDetails and WithMetadata.
func NewCreateInstrumentInput(ledgerID, accountID string) *CreateInstrumentInput {
	return &CreateInstrumentInput{LedgerID: ledgerID, AccountID: accountID}
}

// WithBankingDetails sets instrument banking details.
func (input *CreateInstrumentInput) WithBankingDetails(bankingDetails *BankingDetails) *CreateInstrumentInput {
	if input == nil {
		return nil
	}

	input.BankingDetails = bankingDetails

	return input
}

// WithRegulatoryFields sets instrument regulatory fields.
func (input *CreateInstrumentInput) WithRegulatoryFields(regulatoryFields *RegulatoryFields) *CreateInstrumentInput {
	if input == nil {
		return nil
	}

	input.RegulatoryFields = regulatoryFields

	return input
}

// WithRelatedParties replaces the related-parties slice on the create payload.
func (input *CreateInstrumentInput) WithRelatedParties(relatedParties []*RelatedParty) *CreateInstrumentInput {
	if input == nil {
		return nil
	}

	input.RelatedParties = cloneRelatedParties(relatedParties)

	return input
}

// WithMetadata sets instrument metadata.
func (input *CreateInstrumentInput) WithMetadata(metadata map[string]any) *CreateInstrumentInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// Validate validates the CreateInstrumentInput fields against the server
// contract: ledgerId, accountId, bankingDetails and metadata are all required,
// and the two identifiers are declared format: uuid.
func (input *CreateInstrumentInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	validateInstrumentUUID(&errs, "ledgerId", input.LedgerID)
	validateInstrumentUUID(&errs, "accountId", input.AccountID)

	if input.BankingDetails == nil {
		errs.Append("bankingDetails", "is required")
	}

	if input.Metadata == nil {
		errs.Append("metadata", "is required")
	} else if err := core.ValidateMetadata(input.Metadata); err != nil {
		errs.Append("metadata", "invalid: "+err.Error())
	}

	if err := validateRelatedParties(input.RelatedParties); err != nil {
		errs.Append("relatedParties", err.Error())
	}

	return errs.OrNil()
}

// validateInstrumentUUID refuses an identifier the endpoint would refuse: the
// contract declares ledgerId and accountId as format: uuid, so anything else is
// a round trip that comes back 422 with a field name this can attach locally.
func validateInstrumentUUID(errs *validation.FieldErrors, field, value string) {
	if strings.TrimSpace(value) == "" {
		errs.Append(field, "is required")

		return
	}

	if _, err := uuid.Parse(value); err != nil {
		errs.Append(field, "must be a UUID")
	}
}

// NewUpdateInstrumentInput creates an empty instrument update payload.
func NewUpdateInstrumentInput() *UpdateInstrumentInput {
	return &UpdateInstrumentInput{}
}

// WithBankingDetails sets instrument banking details for update.
func (input *UpdateInstrumentInput) WithBankingDetails(bankingDetails *BankingDetails) *UpdateInstrumentInput {
	if input == nil {
		return nil
	}

	input.BankingDetails = bankingDetails

	return input
}

// WithRegulatoryFields sets instrument regulatory fields for update.
func (input *UpdateInstrumentInput) WithRegulatoryFields(regulatoryFields *RegulatoryFields) *UpdateInstrumentInput {
	if input == nil {
		return nil
	}

	input.RegulatoryFields = regulatoryFields

	return input
}

// WithRelatedParties replaces the related-parties slice on the update payload.
func (input *UpdateInstrumentInput) WithRelatedParties(relatedParties []*RelatedParty) *UpdateInstrumentInput {
	if input == nil {
		return nil
	}

	input.RelatedParties = cloneRelatedParties(relatedParties)

	return input
}

// WithMetadata sets instrument metadata for update.
func (input *UpdateInstrumentInput) WithMetadata(metadata map[string]any) *UpdateInstrumentInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithNullField marks one field for explicit null removal.
func (input *UpdateInstrumentInput) WithNullField(field string) *UpdateInstrumentInput {
	return input.WithNullFields(field)
}

// WithNullFields marks fields that should be sent as explicit JSON null in PATCH requests.
func (input *UpdateInstrumentInput) WithNullFields(fields ...string) *UpdateInstrumentInput {
	if input == nil {
		return nil
	}

	input.NullFields = append(input.NullFields, fields...)

	return input
}

// validInstrumentNullFields are the update properties that can be cleared with
// an explicit JSON null. It is the contract's property set MINUS the ones it
// marks required: nulling a required property produces a body the endpoint
// refuses, so it is not a supported clear at all rather than a clear that
// happens to fail.
var validInstrumentNullFields = map[string]bool{
	"regulatoryFields": true,
	"relatedParties":   true,
}

// requiredInstrumentUpdateFields are the properties the update contract marks
// required. They exist as their own set so a caller who tries to clear one gets
// told WHY rather than the generic "unsupported null field".
var requiredInstrumentUpdateFields = map[string]bool{
	"bankingDetails": true,
	"metadata":       true,
}

// MarshalJSON emits only set fields plus fields explicitly marked for null removal.
func (input *UpdateInstrumentInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	if err := input.validateNullFieldConflicts(); err != nil {
		return nil, err
	}

	payload := map[string]any{}
	if input.BankingDetails != nil {
		payload["bankingDetails"] = input.BankingDetails
	}

	if input.RegulatoryFields != nil {
		payload["regulatoryFields"] = input.RegulatoryFields
	}

	if input.RelatedParties != nil {
		payload["relatedParties"] = input.RelatedParties
	}

	if input.Metadata != nil {
		payload["metadata"] = input.Metadata
	}

	for _, field := range input.NullFields {
		field = strings.TrimSpace(field)
		if !validInstrumentNullFields[field] {
			return nil, fmt.Errorf("unsupported null field %q", field)
		}

		payload[field] = nil
	}

	// NOTE: empty-payload rejection lives in Validate(), not here. See
	// UpdateHolderInput.MarshalJSON for the architectural rationale.
	return json.Marshal(payload)
}

func (input *UpdateInstrumentInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.BankingDetails != nil ||
		input.RegulatoryFields != nil ||
		input.RelatedParties != nil ||
		input.Metadata != nil ||
		len(input.NullFields) > 0
}

func (input *UpdateInstrumentInput) validateNullFieldConflicts() error {
	if input == nil {
		return nil
	}

	setFields := map[string]bool{
		"bankingDetails":   input.BankingDetails != nil,
		"regulatoryFields": input.RegulatoryFields != nil,
		"relatedParties":   input.RelatedParties != nil,
		"metadata":         input.Metadata != nil,
	}

	for _, field := range input.NullFields {
		field = strings.TrimSpace(field)
		if setFields[field] {
			return fmt.Errorf("field %q cannot be set and cleared in the same request", field)
		}
	}

	return nil
}

// Validate validates the UpdateInstrumentInput fields against the server
// contract: bankingDetails and metadata are required on the PATCH body, the two
// remaining properties are optional, and only those two can be cleared with an
// explicit null.
func (input *UpdateInstrumentInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	var errs validation.FieldErrors

	if input.BankingDetails == nil {
		errs.Append("bankingDetails", "is required")
	}

	if input.Metadata == nil {
		errs.Append("metadata", "is required")
	} else if err := core.ValidateMetadata(input.Metadata); err != nil {
		errs.Append("metadata", "invalid: "+err.Error())
	}

	if err := validateRelatedParties(input.RelatedParties); err != nil {
		errs.Append("relatedParties", err.Error())
	}

	input.validateNullFields(&errs)

	if err := input.validateNullFieldConflicts(); err != nil {
		errs.Append("nullFields", err.Error())
	}

	return errs.OrNil()
}

// validateNullFields refuses a clear the contract cannot express, and says which
// of the two reasons applies: the property is REQUIRED (so nulling it makes the
// whole body invalid) or the endpoint has no such property at all.
//
// This is the one CRM update surface that does not route through
// validateCRMNullFields: that helper answers "is this field clearable" with one
// message, and here the answer has two distinct causes a caller acts on
// differently — send a value instead, versus stop sending the field.
func (input *UpdateInstrumentInput) validateNullFields(errs *validation.FieldErrors) {
	for _, field := range input.NullFields {
		field = strings.TrimSpace(field)

		switch {
		case field == "":
			errs.Append("nullFields", "null field cannot be empty")
		case requiredInstrumentUpdateFields[field]:
			errs.Append("nullFields", fmt.Sprintf("%q is required by the update contract and cannot be cleared", field))
		case !validInstrumentNullFields[field]:
			errs.Append("nullFields", fmt.Sprintf("unsupported null field %q", field))
		}
	}
}

// InstrumentsListOpts is the typed options struct for listing instruments and
// its All/Pages iterators. Mirrors HoldersListOpts: embeds CursorListOpts for
// the shared cursor/sort fields and attaches an InstrumentFilters sub-struct
// with only the filters the endpoint honors.
//
// InstrumentsListOpts is a value type. Concurrent-safe by construction — the
// entity layer never mutates a caller's opts.
//
// IMPORTANT: This is a CURSOR-PAGINATED endpoint. The struct does NOT expose
// Page or Offset fields. Seed Cursor to resume a mid-stream page, or read the
// NextCursor from the previous response's Pagination to fetch the next page.
// v3 compile-time prevents the v2 footgun where setting WithPage on a cursor
// endpoint silently dropped the value (audit finding 5.5). The endpoint has no
// start_date/end_date slot, so Validate REJECTS any date filter rather than
// shipping a silently-ignored one.
type InstrumentsListOpts struct {
	CursorListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters InstrumentFilters
}

// InstrumentFilters is the typed filter set for the instruments endpoint.
type InstrumentFilters struct {
	// Type narrows by instrument type.
	Type string

	// Document narrows by instrument document.
	Document string

	// IncludeDeleted, when true, includes soft-deleted instruments.
	IncludeDeleted bool
}

// Validate enforces the shared cursor-list preconditions (limit bounds, sort
// direction) and REJECTS any date filter: the generated ListInstrumentsParams has
// no start_date/end_date slot, so a date would validate then silently drop,
// returning the full unfiltered set.
func (o InstrumentsListOpts) Validate() error {
	return ValidateCursorListOptsNoDates("InstrumentsListOpts.Validate", o.CursorListOpts)
}

// ToQueryParams renders an InstrumentsListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it. Treat as an
// internal contract; not part of the user-facing API.
func (o InstrumentsListOpts) ToQueryParams() map[string]string {
	params := CursorQueryParams(o.CursorListOpts)

	if o.Filters.Type != "" {
		params["type"] = o.Filters.Type
	}

	if o.Filters.Document != "" {
		params["document"] = o.Filters.Document
	}

	if o.Filters.IncludeDeleted {
		params["include_deleted"] = "true"
	}

	return params
}

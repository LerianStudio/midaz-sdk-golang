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

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation/core"
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
// It carries only the writable subset; the holder is path-sourced and the
// ledger/account IDs are threaded by the server, so none appear on the body.
// Optional fields use json:"...,omitempty"; absent ≡ null per RFC 7396.
type CreateInstrumentInput struct {
	Type             *string           `json:"type"`
	Document         *string           `json:"document,omitempty"`
	BankingDetails   *BankingDetails   `json:"bankingDetails,omitempty"`
	RegulatoryFields *RegulatoryFields `json:"regulatoryFields,omitempty"`
	RelatedParties   []*RelatedParty   `json:"relatedParties,omitempty"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
}

// UpdateInstrumentInput is the PATCH payload for updating an instrument.
//
// MarshalJSON emits only set fields plus fields explicitly marked for null
// removal (UpdateHolderInput/UpdateAliasInput pattern). Empty-payload
// rejection lives in Validate(), not MarshalJSON.
type UpdateInstrumentInput struct {
	Document         *string           `json:"document,omitempty"`
	BankingDetails   *BankingDetails   `json:"bankingDetails,omitempty"`
	RegulatoryFields *RegulatoryFields `json:"regulatoryFields,omitempty"`
	RelatedParties   []*RelatedParty   `json:"relatedParties,omitempty"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
	NullFields       []string          `json:"-"`
}

// NewCreateInstrumentInput creates an instrument create payload with the
// required type set.
func NewCreateInstrumentInput(instrumentType string) *CreateInstrumentInput {
	return &CreateInstrumentInput{Type: instrumentStringPtr(instrumentType)}
}

// WithDocument sets the instrument document.
func (input *CreateInstrumentInput) WithDocument(document string) *CreateInstrumentInput {
	if input == nil {
		return nil
	}

	input.Document = instrumentStringPtr(document)

	return input
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

// Validate validates the CreateInstrumentInput fields.
func (input *CreateInstrumentInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	if input.Type == nil || strings.TrimSpace(*input.Type) == "" {
		errs.Append("type", "is required")
	}

	if len(input.Metadata) > 0 {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	if err := validateRelatedParties(input.RelatedParties); err != nil {
		errs.Append("relatedParties", err.Error())
	}

	return errs.OrNil()
}

// NewUpdateInstrumentInput creates an empty instrument update payload.
func NewUpdateInstrumentInput() *UpdateInstrumentInput {
	return &UpdateInstrumentInput{}
}

// WithDocument sets the instrument document for update.
func (input *UpdateInstrumentInput) WithDocument(document string) *UpdateInstrumentInput {
	if input == nil {
		return nil
	}

	input.Document = instrumentStringPtr(document)

	return input
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

var validInstrumentNullFields = map[string]bool{
	"document":         true,
	"bankingDetails":   true,
	"regulatoryFields": true,
	"relatedParties":   true,
	"metadata":         true,
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
	if input.Document != nil {
		payload["document"] = input.Document
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

	return input.Document != nil ||
		input.BankingDetails != nil ||
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
		"document":         input.Document != nil,
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

// Validate validates the UpdateInstrumentInput fields.
func (input *UpdateInstrumentInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	var errs validation.FieldErrors

	if len(input.Metadata) > 0 {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	if err := validateRelatedParties(input.RelatedParties); err != nil {
		errs.Append("relatedParties", err.Error())
	}

	if err := validateCRMNullFields(input.NullFields, validInstrumentNullFields); err != nil {
		errs.Append("nullFields", err.Error())
	}

	if err := input.validateNullFieldConflicts(); err != nil {
		errs.Append("nullFields", err.Error())
	}

	return errs.OrNil()
}

func instrumentStringPtr(value string) *string {
	return &value
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

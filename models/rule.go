// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation"
)

// Rule action enum. A rule's Action decides the outcome when its CEL Expression
// matches a transaction. The server is the sole authority on these values; the
// SDK validates against this closed set to fail fast before the round trip.
const (
	RuleActionAllow  = "ALLOW"
	RuleActionDeny   = "DENY"
	RuleActionReview = "REVIEW"
)

const (
	maxRuleNameLength        = 255
	maxRuleDescriptionLength = 1000
	maxRuleExpressionLength  = 5000
	maxRuleScopes            = 100
)

// Rule is the SDK-native tracer rule: a CEL Expression plus an Action, scoped by
// zero or more Scopes, with a DRAFT → ACTIVE → INACTIVE lifecycle. The wire shape
// mirrors the generated gentracer.Rule (camelCase tags, ruleId as the identity
// field). Expression is an opaque CEL string — the server validates syntax, type,
// and cost; the SDK never parses it.
type Rule struct {
	// ID is the server-issued rule identity (UUID).
	ID string `json:"ruleId"`

	// Name is the human-readable rule name.
	Name string `json:"name"`

	// Description is an optional free-form description.
	Description *string `json:"description,omitempty"`

	// Expression is the CEL predicate evaluated against a transaction.
	Expression string `json:"expression"`

	// Action is the outcome when Expression matches (ALLOW, DENY, REVIEW).
	Action string `json:"action"`

	// Scopes narrows the transactions this rule applies to. Empty means all.
	Scopes []Scope `json:"scopes,omitempty"`

	// Status is the lifecycle state (DRAFT, ACTIVE, INACTIVE).
	Status string `json:"status"`

	// ActivatedAt is set when the rule last transitioned to ACTIVE.
	ActivatedAt *time.Time `json:"activatedAt,omitempty"`

	// DeactivatedAt is set when the rule last transitioned to INACTIVE.
	DeactivatedAt *time.Time `json:"deactivatedAt,omitempty"`

	// DeletedAt is set when the rule was soft-deleted.
	DeletedAt *time.Time `json:"deletedAt,omitempty"`

	// CreatedAt is the creation timestamp.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is the last-update timestamp.
	UpdatedAt time.Time `json:"updatedAt"`
}

// CreateRuleInput is the SDK-native rule creation payload. Name, Expression, and
// Action are required; Description and Scopes are optional.
type CreateRuleInput struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Expression  string  `json:"expression"`
	Action      string  `json:"action"`
	Scopes      []Scope `json:"scopes,omitempty"`
}

// NewCreateRuleInput builds a rule creation payload with the required fields.
func NewCreateRuleInput(name, expression, action string) *CreateRuleInput {
	return &CreateRuleInput{
		Name:       name,
		Expression: expression,
		Action:     action,
	}
}

// WithDescription sets the optional description.
func (input *CreateRuleInput) WithDescription(description string) *CreateRuleInput {
	if input == nil {
		return nil
	}

	input.Description = &description

	return input
}

// WithScopes replaces the scope set.
func (input *CreateRuleInput) WithScopes(scopes []Scope) *CreateRuleInput {
	if input == nil {
		return nil
	}

	input.Scopes = scopes

	return input
}

// WithScope appends a single scope.
func (input *CreateRuleInput) WithScope(scope Scope) *CreateRuleInput {
	if input == nil {
		return nil
	}

	input.Scopes = append(input.Scopes, scope)

	return input
}

// Validate enforces SDK-side preconditions before the round trip: Name 1-255,
// Expression 1-5000, Action in the closed enum, at most 100 scopes. CEL syntax is
// NOT checked here — the server is the sole CEL validator.
func (input *CreateRuleInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	validateRuleName(&errs, input.Name)
	validateRuleExpression(&errs, input.Expression)
	validateRuleAction(&errs, input.Action)
	validateRuleScopes(&errs, input.Scopes)

	if input.Description != nil && len(*input.Description) > maxRuleDescriptionLength {
		errs.Append("description", fmt.Sprintf("must be at most %d characters", maxRuleDescriptionLength))
	}

	return errs.OrNil()
}

// MarshalJSON is the default create marshal (all set fields), kept explicit so the
// wire body stays camelCase and Description/Scopes omit when unset.
func (input *CreateRuleInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	type alias CreateRuleInput

	return json.Marshal((*alias)(input))
}

// UpdateRuleInput is the PATCH payload for a rule. Every field is a pointer:
// nil means "leave unchanged". MarshalJSON emits only the set fields (omit-unset)
// so an unchanged field is never clobbered server-side, and Validate rejects a
// no-op update (empty PATCH), mirroring the server IsEmpty → ErrNothingToUpdate
// probe.
type UpdateRuleInput struct {
	Name        *string  `json:"-"`
	Description *string  `json:"-"`
	Expression  *string  `json:"-"`
	Action      *string  `json:"-"`
	Scopes      *[]Scope `json:"-"`
}

// NewUpdateRuleInput builds an empty rule update payload.
func NewUpdateRuleInput() *UpdateRuleInput {
	return &UpdateRuleInput{}
}

// WithName sets the name for update.
func (input *UpdateRuleInput) WithName(name string) *UpdateRuleInput {
	if input == nil {
		return nil
	}

	input.Name = &name

	return input
}

// WithDescription sets the description for update.
func (input *UpdateRuleInput) WithDescription(description string) *UpdateRuleInput {
	if input == nil {
		return nil
	}

	input.Description = &description

	return input
}

// WithExpression sets the CEL expression for update.
func (input *UpdateRuleInput) WithExpression(expression string) *UpdateRuleInput {
	if input == nil {
		return nil
	}

	input.Expression = &expression

	return input
}

// WithAction sets the action for update.
func (input *UpdateRuleInput) WithAction(action string) *UpdateRuleInput {
	if input == nil {
		return nil
	}

	input.Action = &action

	return input
}

// WithScopes sets the scope set for update.
func (input *UpdateRuleInput) WithScopes(scopes []Scope) *UpdateRuleInput {
	if input == nil {
		return nil
	}

	input.Scopes = &scopes

	return input
}

// IsEmpty reports whether the PATCH carries no changes (a no-op update).
func (input *UpdateRuleInput) IsEmpty() bool {
	if input == nil {
		return true
	}

	return input.Name == nil &&
		input.Description == nil &&
		input.Expression == nil &&
		input.Action == nil &&
		input.Scopes == nil
}

// Validate rejects an empty PATCH and checks each set field against the same
// bounds as create.
func (input *UpdateRuleInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.IsEmpty() {
		return errors.New("empty update payload not allowed")
	}

	var errs validation.FieldErrors

	if input.Name != nil {
		validateRuleName(&errs, *input.Name)
	}

	if input.Expression != nil {
		validateRuleExpression(&errs, *input.Expression)
	}

	if input.Action != nil {
		validateRuleAction(&errs, *input.Action)
	}

	if input.Scopes != nil {
		validateRuleScopes(&errs, *input.Scopes)
	}

	if input.Description != nil && len(*input.Description) > maxRuleDescriptionLength {
		errs.Append("description", fmt.Sprintf("must be at most %d characters", maxRuleDescriptionLength))
	}

	return errs.OrNil()
}

// MarshalJSON emits only the fields explicitly set on the PATCH input, under their
// server wire names.
func (input *UpdateRuleInput) MarshalJSON() ([]byte, error) {
	if input == nil {
		return []byte("null"), nil
	}

	payload := map[string]any{}

	if input.Name != nil {
		payload["name"] = input.Name
	}

	if input.Description != nil {
		payload["description"] = input.Description
	}

	if input.Expression != nil {
		payload["expression"] = input.Expression
	}

	if input.Action != nil {
		payload["action"] = input.Action
	}

	if input.Scopes != nil {
		payload["scopes"] = input.Scopes
	}

	return json.Marshal(payload)
}

// RulesListOpts is the typed options struct for the rules cursor list. It embeds
// CursorListOpts for the shared cursor/sort-order/date fields, adds SortBy (the
// tracer accepts a sort_by field in addition to sort_order), and attaches a typed
// Filters sub-struct carrying only the filters the rules endpoint honors.
//
// This is a CURSOR-PAGINATED endpoint: no Page or Offset. Iterate by passing back
// the previous response's NextCursor.
type RulesListOpts struct {
	CursorListOpts

	// SortBy names the sort field (created_at, updated_at, name, status). Empty
	// falls back to the server default. Passed through verbatim — the server
	// validates the field name.
	SortBy string

	// Filters narrows the result set. Zero value means no narrowing.
	Filters RulesFilters
}

// RulesFilters is the typed filter set for the rules endpoint. Each field maps to
// a native query-param slot on the generated ListRulesParams.
type RulesFilters struct {
	Name            string
	Status          string
	Action          string
	AccountID       string
	SegmentID       string
	PortfolioID     string
	MerchantID      string
	TransactionType string
	SubType         string
}

// Validate enforces the shared cursor-list preconditions (limit bounds, sort
// direction, date range). Filter values are passed through — the server validates
// them.
func (o RulesListOpts) Validate() error {
	return ValidateCursorListOpts("RulesListOpts.Validate", o.CursorListOpts)
}

func validateRuleName(errs *validation.FieldErrors, name string) {
	trimmed := strings.TrimSpace(name)

	switch {
	case trimmed == "":
		errs.Append("name", "is required")
	case len(name) > maxRuleNameLength:
		errs.Append("name", fmt.Sprintf("must be at most %d characters", maxRuleNameLength))
	}
}

func validateRuleExpression(errs *validation.FieldErrors, expression string) {
	switch {
	case strings.TrimSpace(expression) == "":
		errs.Append("expression", "is required")
	case len(expression) > maxRuleExpressionLength:
		errs.Append("expression", fmt.Sprintf("must be at most %d characters", maxRuleExpressionLength))
	}
}

func validateRuleAction(errs *validation.FieldErrors, action string) {
	switch action {
	case RuleActionAllow, RuleActionDeny, RuleActionReview:
	default:
		errs.Append("action", fmt.Sprintf("must be one of %s, %s, %s", RuleActionAllow, RuleActionDeny, RuleActionReview))
	}
}

func validateRuleScopes(errs *validation.FieldErrors, scopes []Scope) {
	if len(scopes) > maxRuleScopes {
		errs.Append("scopes", fmt.Sprintf("must be at most %d entries", maxRuleScopes))
	}
}

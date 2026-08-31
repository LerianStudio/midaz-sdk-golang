// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation/core"
)

// The /v2 transaction surface is a different contract from /v1, not a renamed
// one, so it gets its own types instead of shared structs with
// version-conditional fields.
//
// What actually changed, read off the server contract rather than inferred:
//
//   - The REQUEST is flat. /v1 nests a send envelope (send.source.from,
//     send.distribute.to) with four creation styles behind four endpoints; /v2
//     has two leg arrays, debits and credits, and the action lives in the URL
//     (direct, hold, block, unblock).
//   - Each leg names the organization and ledger its account belongs to. All of
//     them must name the SAME pair, and that pair is what the transaction is
//     created in — the URL carries no scope at all.
//   - A leg carries EXACTLY ONE value expression: an explicit amount, or a share
//     of the transaction total. Both, or neither, is refused.
//   - The RESPONSE dropped four /v1 fields (chartOfAccountsGroupName, route,
//     source, destination) and kept two /v1 dropped (feesSkipped, tracerSkipped),
//     renaming the alias lists to debit/credit.
//
// Money values travel as decimal STRINGS on both surfaces, never as floats.

// Leg-count and share bounds the server publishes and enforces. Checking them
// here turns a round trip into a local refusal, and keeps the SDK's message
// specific about which side and which entry is wrong.
const (
	// maxTransactionV2Legs bounds one side of a v2 transaction.
	maxTransactionV2Legs = 500

	// maxSharePercentage bounds both share factors. They share an upper bound, so
	// whether a body is accepted does not depend on which factor carries the
	// larger number.
	maxSharePercentage = 100
)

// TransactionV2 is a transaction as the /v2 surface returns it.
//
// It is NOT models.Transaction with fields added. Four /v1 fields are absent
// because /v2 does not serve them — chartOfAccountsGroupName, route, source and
// destination — and reading them off a v2 response was never possible. In
// exchange it carries FeesSkipped and TracerSkipped, which /v1 dropped, and
// names the participating account aliases Debit and Credit rather than Source
// and Destination.
type TransactionV2 struct {
	// ID is the server-generated identifier of the transaction.
	ID string `json:"id"`

	// Amount is the transaction total as an exact decimal string. It is the
	// value the legs' share expressions divide.
	Amount string `json:"amount"`

	// AssetCode identifies the asset every leg moves.
	AssetCode string `json:"assetCode"`

	// Description is the human-readable description of the transaction.
	Description string `json:"description,omitempty"`

	// Status is the processing status. The canonical set is the same as /v1:
	// CREATED, PENDING, APPROVED, CANCELED, NOTED.
	Status Status `json:"status"`

	// Debit lists the aliases of the accounts value moved out of.
	Debit []string `json:"debit,omitempty"`

	// Credit lists the aliases of the accounts value moved into.
	Credit []string `json:"credit,omitempty"`

	// LedgerID is the ledger the transaction was created in — the ledger every
	// leg of the request named.
	LedgerID string `json:"ledgerId"`

	// OrganizationID is the organization the transaction was created in.
	OrganizationID string `json:"organizationId"`

	// ParentTransactionID points at the transaction this one reverses, when it
	// is a reversal child.
	ParentTransactionID string `json:"parentTransactionId,omitempty"`

	// RouteID is the UUID of the transaction route that shaped it.
	RouteID string `json:"routeId,omitempty"`

	// FeesSkipped reports that the fee engine did not run for this transaction.
	// /v1 dropped this field; /v2 keeps it.
	FeesSkipped bool `json:"feesSkipped"`

	// TracerSkipped reports that Tracer validation did not run for this
	// transaction. /v1 dropped this field; /v2 keeps it.
	TracerSkipped bool `json:"tracerSkipped"`

	// Operations are the individual debits and credits the transaction produced.
	Operations []OperationV2 `json:"operations,omitempty"`

	// Metadata holds the flat custom attributes carried on the transaction.
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is when the transaction was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the transaction was last updated.
	UpdatedAt time.Time `json:"updatedAt"`

	// DeletedAt is when the transaction was soft-deleted, if it was.
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// OperationV2 is one accounting entry of a /v2 transaction.
//
// It differs from models.Operation by exactly the two fields /v2 dropped:
// chartOfAccounts and route. The operation route survives as the RouteID /
// RouteCode / RouteDescription trio; only the legacy free-text route label is
// gone.
type OperationV2 struct {
	// ID is the server-generated identifier of the operation.
	ID string `json:"id"`

	// TransactionID is the transaction that produced this operation.
	TransactionID string `json:"transactionId"`

	// Description is the human-readable description of the operation.
	Description string `json:"description,omitempty"`

	// Type is the operation type (DEBIT, CREDIT, and the BLOCK / UNBLOCK labels
	// the block actions stamp).
	Type string `json:"type"`

	// AssetCode identifies the asset moved.
	AssetCode string `json:"assetCode"`

	// Amount is the value moved, as an exact decimal.
	Amount Amount `json:"amount"`

	// Balance is the affected balance as it stood BEFORE the operation.
	Balance OperationBalance `json:"balance"`

	// BalanceAfter is the affected balance as it stood AFTER the operation.
	BalanceAfter OperationBalance `json:"balanceAfter"`

	// Status is the operation status.
	Status Status `json:"status"`

	// AccountID is the account the operation posted against.
	AccountID string `json:"accountId"`

	// AccountAlias is the human-readable alias of that account.
	AccountAlias string `json:"accountAlias,omitempty"`

	// BalanceID is the specific balance the operation moved.
	BalanceID string `json:"balanceId"`

	// BalanceKey names which of the account's balances was moved.
	BalanceKey string `json:"balanceKey,omitempty"`

	// BalanceAffected reports whether the operation changed a balance.
	BalanceAffected *bool `json:"balanceAffected,omitempty"`

	// Direction is the accounting direction, debit or credit.
	Direction string `json:"direction,omitempty"`

	// OrganizationID is the organization the operation belongs to.
	OrganizationID string `json:"organizationId"`

	// LedgerID is the ledger the operation belongs to.
	LedgerID string `json:"ledgerId"`

	// RouteID is the UUID of the operation route applied.
	RouteID string `json:"routeId,omitempty"`

	// RouteCode is the human-readable code of that operation route.
	RouteCode string `json:"routeCode,omitempty"`

	// RouteDescription is the human-readable description of that route.
	RouteDescription string `json:"routeDescription,omitempty"`

	// Metadata holds the flat custom attributes carried on the operation.
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is when the operation was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the operation was last updated.
	UpdatedAt time.Time `json:"updatedAt"`

	// DeletedAt is when the operation was soft-deleted, if it was.
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// CreateTransactionV2Input is the request body every /v2 create action takes —
// direct, hold, block and unblock share it exactly. Which action runs is decided
// by the endpoint, never by this payload.
//
// Debits and Credits are both required and non-empty; one debit against many
// credits, or the reverse, is a valid request. Amount is the transaction total
// that the legs' share expressions divide.
//
// The organization and ledger on each leg may be left empty when the input is
// handed to a facade: the facade stamps the pair the caller addressed onto every
// leg and refuses a leg that names a different one. Filling them by hand is
// supported and verified rather than ignored.
type CreateTransactionV2Input struct {
	// Asset is the asset code every leg moves. Required.
	Asset string `json:"asset"`

	// Amount is the transaction total, an exact decimal string. Required and
	// strictly positive.
	Amount string `json:"amount"`

	// Debits are the legs value moves OUT of. Required, 1..500 entries.
	Debits []TransactionV2Leg `json:"debits"`

	// Credits are the legs value moves INTO. Required, 1..500 entries.
	Credits []TransactionV2Leg `json:"credits"`

	// Description is an optional human-readable description.
	Description string `json:"description,omitempty"`

	// Code is an optional reference code.
	Code string `json:"code,omitempty"`

	// RouteID is the optional TRANSACTION route UUID.
	RouteID string `json:"routeId,omitempty"`

	// OperationRouteID is the optional default OPERATION route UUID. A leg's own
	// OperationRouteID overrides it for that leg.
	OperationRouteID string `json:"operationRouteId,omitempty"`

	// Metadata holds flat custom attributes. Values must be flat — no nesting.
	Metadata map[string]any `json:"metadata,omitempty"`

	// IdempotencyKey is the caller-supplied key for this create. It travels as
	// the X-Idempotency HEADER, never in the body, which is why it is excluded
	// from the wire shape. Leave it empty to let the SDK generate one.
	IdempotencyKey string `json:"-"`
}

// TransactionV2Leg is one leg of a transaction side.
//
// Fill EXACTLY ONE value expression: Amount for an explicit value, or Share for
// a percentage of the transaction total. A leg carrying both, or neither, is
// refused before the request leaves.
type TransactionV2Leg struct {
	// Alias is the account this leg posts against. Required.
	Alias string `json:"alias"`

	// OrganizationID is the organization that account belongs to. Required on
	// the wire; a facade fills it from the organization the caller addressed
	// when it is left empty here.
	OrganizationID string `json:"organizationId"`

	// LedgerID is the ledger that account belongs to. Same filling rule as
	// OrganizationID.
	LedgerID string `json:"ledgerId"`

	// Amount is this leg's explicit value, an exact decimal string. Mutually
	// exclusive with Share.
	Amount string `json:"amount,omitempty"`

	// Share expresses this leg's value as a percentage of the transaction total
	// instead of an absolute amount. Mutually exclusive with Amount.
	Share *TransactionV2Share `json:"share,omitempty"`

	// OperationRouteID overrides the request-level operation route for this leg.
	OperationRouteID string `json:"operationRouteId,omitempty"`
}

// TransactionV2Share expresses a leg's value as a percentage of the transaction
// total. The resolved value is total x (Percentage/100) x
// (PercentageOfPercentage/100).
type TransactionV2Share struct {
	// Percentage is this leg's share of the total, in percent. Required, 1..100.
	// Zero moves nothing while the transaction still commits, so it is refused
	// rather than accepted as "no share".
	Percentage int64 `json:"percentage"`

	// PercentageOfPercentage narrows Percentage, in percent: 25 against a
	// Percentage of 60 yields 15% of the total.
	//
	// ZERO MEANS NO NARROWING, not a zero share — on an int64 it is
	// indistinguishable from the field being absent, and absent has to mean
	// "take the whole Percentage". That is why its lower bound is 0 where
	// Percentage's is 1.
	PercentageOfPercentage int64 `json:"percentageOfPercentage,omitempty"`
}

// Validate enforces the /v2 create contract locally, so a malformed transaction
// is refused before it can reach a ledger.
//
// It checks the same obligations the server does, in the server's own terms: the
// asset and a strictly positive total, both sides present and within the 500-leg
// bound, one value expression per leg, a complete scope on every leg, and flat
// metadata. What it deliberately does NOT check is whether each side's legs sum
// to the total — that needs the resolved per-leg values a share expression only
// yields server-side, and a client-side half-answer would refuse valid bodies.
func (input *CreateTransactionV2Input) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	if input.Asset == "" {
		errs.Append("asset", "is required")
	}

	if err := validatePositiveDecimalString(input.Amount); err != nil {
		errs.Append("amount", err.Error())
	}

	appendV2SideErrors(&errs, "debits", input.Debits)
	appendV2SideErrors(&errs, "credits", input.Credits)

	if len(input.Metadata) > 0 {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// appendV2SideErrors validates one side's leg array, naming each failure by the
// side and the index a caller can find it at.
func appendV2SideErrors(errs *validation.FieldErrors, side string, legs []TransactionV2Leg) {
	switch {
	case len(legs) == 0:
		errs.Append(side, "must carry at least one leg")
		return
	case len(legs) > maxTransactionV2Legs:
		errs.Append(side, fmt.Sprintf("carries %d legs, at most %d are allowed", len(legs), maxTransactionV2Legs))

		return
	}

	for i, leg := range legs {
		if err := leg.validate(); err != nil {
			errs.Append(fmt.Sprintf("%s[%d]", side, i), err.Error())
		}
	}
}

// aliasSeparator is the character Midaz builds and parses its COMPOSITE aliases
// with, and therefore the one character a client-supplied alias may not contain.
// See TransactionV2Leg.validate.
const aliasSeparator = "#"

// validate enforces the per-leg obligations: an alias free of the composite
// separator, a complete scope, and exactly one value expression.
func (leg TransactionV2Leg) validate() error {
	if leg.Alias == "" {
		return errors.New("alias is required")
	}

	// The ledger refuses an alias containing '#' on every v2 leg, and the reason
	// is a lost entry rather than a formatting preference. Downstream it rewrites
	// each accepted alias into a composite "index#alias#balanceKey" form and keys
	// its per-entry maps on that string; an alias that ALREADY looks composite is
	// left spelled exactly as the client sent it, so a client-forged one reaches
	// those maps unmutated. There it either collides with another entry's key or
	// matches none of them, and a transaction that loses one side's entry moves
	// value in one direction only.
	//
	// The rule is narrow on purpose — only '#', not the full registered-alias
	// charset — because '/' must stay legal for "@external/<ASSET>", the alias
	// every ledger's external account carries and the only way to spell funding
	// or withdrawal on a surface with no inflow/outflow action.
	// (pkg/mtransaction/v2_input.go validateV2Alias; AliasSeparator in
	// pkg/mtransaction/transaction.go.)
	if strings.Contains(leg.Alias, aliasSeparator) {
		return fmt.Errorf("alias %q must not contain %q: the ledger builds its internal composite "+
			"aliases with that character and refuses a client-supplied one", leg.Alias, aliasSeparator)
	}

	if leg.OrganizationID == "" || leg.LedgerID == "" {
		return errors.New("organizationId and ledgerId are required on every leg")
	}

	hasAmount := leg.Amount != ""
	hasShare := leg.Share != nil

	switch {
	case hasAmount && hasShare:
		return errors.New("a leg carries either amount or share, never both")
	case !hasAmount && !hasShare:
		return errors.New("a leg must carry either amount or share")
	case hasAmount:
		return validatePositiveDecimalString(leg.Amount)
	default:
		return leg.Share.validate()
	}
}

// validate enforces the two share bounds.
func (share *TransactionV2Share) validate() error {
	if share.Percentage < 1 || share.Percentage > maxSharePercentage {
		return fmt.Errorf("share percentage must be between 1 and %d, got %d", maxSharePercentage, share.Percentage)
	}

	if share.PercentageOfPercentage < 0 || share.PercentageOfPercentage > maxSharePercentage {
		return fmt.Errorf("share percentageOfPercentage must be between 0 and %d, got %d",
			maxSharePercentage, share.PercentageOfPercentage)
	}

	return nil
}

// UpdateTransactionV2Input patches the mutable fields of a /v2 transaction:
// description and metadata, which are the whole of what a posted transaction can
// change. The value it moved and the accounts it touched are immutable by
// design.
//
// It is a distinct type from UpdateTransactionInput only because /v1's carries a
// vestigial ExternalID field. Both surfaces send the SAME request schema —
// updateTransaction and updateTransactionV2 both reference
// TransactionUpdateTransactionInput — and /v1's ExternalID is json:"-", so it
// never reaches the wire either. The two types are wire-identical.
//
// The ledger spec marks BOTH description and metadata required on that schema,
// and this input omits each when empty anyway. That is safe, not a gamble: the
// server never applies that schema at request time. Both update routes register
// with SkipValidateBody and read a raw body
// (components/ledger/internal/adapters/http/in/transaction_routes.go:142 and
// transaction_v2_mirror_register.go:60), then decode imperatively into a struct
// whose only validators are "max=256" on the description and per-entry bounds on
// the metadata — no required tag on either
// (components/ledger/internal/adapters/postgres/transaction/transaction.go:90).
// A description-only patch is accepted.
type UpdateTransactionV2Input struct {
	// Description replaces the transaction description.
	Description string `json:"description,omitempty"`

	// Metadata replaces the transaction metadata.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Validate refuses an empty patch: a PATCH that changes nothing is a write that
// costs a round trip and an audit entry for no effect, and it is far more often
// a caller bug than an intention.
//
// The description bound matches UpdateTransactionInput.Validate on /v1, because
// the two surfaces share one server-side struct and one "max=256" validator.
func (input *UpdateTransactionV2Input) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.Description == "" && len(input.Metadata) == 0 {
		return errors.New("update requires at least one of description or metadata")
	}

	if len(input.Description) > maxTransactionDescriptionLength {
		return fmt.Errorf("description must not exceed %d characters", maxTransactionDescriptionLength)
	}

	if len(input.Metadata) > 0 {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return err
		}
	}

	return nil
}

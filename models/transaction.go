package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation/core"
	"github.com/shopspring/decimal"
)

const maxDecimalInputLength = 128

// Transaction represents a transaction in the Midaz Ledger.
// A transaction is a financial event that affects one or more accounts
// through a series of operations (debits and credits).
//
// Transactions are the core financial records in the Midaz system, representing
// the movement of assets between accounts. Each transaction consists of one or more
// operations (debits and credits) that must balance (sum to zero) for each asset type.
//
// Transactions can be in different states as indicated by their Status field.
// The canonical status set (see TransactionStatusCode) is:
//   - CREATED: Initial status of a child reversal transaction before approval
//   - PENDING: Created with pending:true; awaits commit or cancel, balances untouched
//   - APPROVED: Operations applied to balances (immediate create or commit) — terminal success
//   - CANCELED: A PENDING transaction cancelled before commit
//   - NOTED: An annotation transaction (metadata-only, no balance impact)
//
// A revert does not mutate the original (which stays APPROVED); it creates a new
// child reversal transaction. There is no COMPLETED, FAILED, or REJECTED status.
//
// Example usage:
//
//	// Accessing transaction details
//	fmt.Printf("Transaction ID: %s\n", transaction.ID)
//	fmt.Printf("Amount: %s\n", transaction.Amount)
//	fmt.Printf("Asset: %s\n", transaction.AssetCode)
//	fmt.Printf("Status: %s\n", transaction.Status)
//	fmt.Printf("Created: %s\n", transaction.CreatedAt.Format(time.RFC3339))
//
//	// Iterating through operations
//	for i, op := range transaction.Operations {
//	    fmt.Printf("Operation %d: %s %s %s on account %s\n",
//	        i+1, op.Type, op.AssetCode, op.Amount, op.AccountID)
//	}
//
//	// Accessing metadata
//	if reference, ok := transaction.Metadata["reference"].(string); ok {
//	    fmt.Printf("Reference: %s\n", reference)
//	}
type Transaction struct {
	// ID is the unique identifier for the transaction
	// This is a system-generated UUID that uniquely identifies the transaction
	ID string `json:"id"`

	// Template is an optional identifier for the transaction template used
	// Templates can be used to create standardized transactions with predefined
	// structures and validation rules
	Template string `json:"template,omitempty"`

	// Amount is the exact decimal value of the transaction.
	Amount string `json:"amount"`

	// AssetCode identifies the currency or asset type for this transaction
	// Common examples include "USD", "EUR", "BTC", etc.
	AssetCode string `json:"assetCode"`

	// Route is the transaction route identifier that defines the overall flow
	// of the transaction, including the structure of operations to be executed
	Route string `json:"route,omitempty"`

	// RouteID is the UUID of the transaction route.
	RouteID string `json:"routeId,omitempty"`

	// Status indicates the current processing status of the transaction.
	// See TransactionStatusCode for the canonical values
	// (CREATED, PENDING, APPROVED, CANCELED, NOTED).
	Status Status `json:"status"`

	// ChartOfAccountsGroupName specifies the chart of accounts group to use
	// This categorizes the transaction under a specific group for accounting purposes
	ChartOfAccountsGroupName string `json:"chartOfAccountsGroupName,omitempty"`

	// Source contains the list of source account aliases used in this transaction
	// These are the accounts from which funds are debited
	Source []string `json:"source,omitempty"`

	// Destination contains the list of destination account aliases used in this transaction
	// These are the accounts to which funds are credited
	Destination []string `json:"destination,omitempty"`

	// Pending indicates whether the transaction is in a pending state
	// Pending transactions require explicit commitment before affecting account balances
	Pending bool `json:"pending,omitempty"`

	// LedgerID identifies the ledger this transaction belongs to
	// A ledger is a collection of accounts and transactions within an organization
	LedgerID string `json:"ledgerId"`

	// OrganizationID identifies the organization this transaction belongs to
	// An organization is the top-level entity that owns ledgers and accounts
	OrganizationID string `json:"organizationId"`

	// ParentTransactionID identifies the parent transaction for linked/reversal flows.
	ParentTransactionID string `json:"parentTransactionId,omitempty"`

	// Operations contains the individual debit and credit operations
	// Each operation represents a single accounting entry (debit or credit)
	// The sum of all operations for each asset must balance to zero
	Operations []Operation `json:"operations,omitempty"`

	// Metadata contains additional custom data for the transaction
	// This can be used to store application-specific information
	// such as references to external systems, tags, or other contextual data
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is the timestamp when the transaction was created
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is the timestamp when the transaction was last updated
	UpdatedAt time.Time `json:"updatedAt"`

	// DeletedAt is the timestamp when the transaction was deleted, if applicable
	// This field is only set if the transaction has been soft-deleted
	DeletedAt *time.Time `json:"deletedAt,omitempty"`

	// ExternalID is an optional identifier for linking to external systems
	// This can be used to correlate transactions with records in other systems
	// and to prevent duplicate transactions
	ExternalID string `json:"externalId,omitempty"`

	// Description is a human-readable description of the transaction
	// This should provide context about the purpose or nature of the transaction
	Description string `json:"description,omitempty"`

	// FeesSkipped reports whether fee calculation was skipped for this
	// transaction (the server applies it based on ledger settings). The server
	// always emits this flag; the SDK surfaces it so callers can tell whether a
	// fee package was bypassed.
	FeesSkipped bool `json:"feesSkipped,omitempty"`

	// TracerSkipped reports whether tracer-plane validation (limits, rules,
	// reservations) was skipped for this transaction. Emitted by the server and
	// surfaced here so callers can tell an unvalidated transaction from a
	// validated one.
	TracerSkipped bool `json:"tracerSkipped,omitempty"`
}

func validatePositiveDecimalString(value any, field string) error {
	if err := validateDecimalInputBound(value, field); err != nil {
		return err
	}

	parsed, err := decimal.NewFromString(strings.TrimSpace(decimalStringFromAny(value)))
	if err != nil {
		return fmt.Errorf("%s must be a valid decimal", field)
	}

	if !parsed.IsPositive() {
		return fmt.Errorf("%s must be greater than zero", field)
	}

	return nil
}

func validateDecimalInputBound(value any, field string) error {
	if floatValue, ok := value.(float64); ok && (math.IsInf(floatValue, 0) || math.IsNaN(floatValue)) {
		return fmt.Errorf("%s must be a finite decimal", field)
	}

	if decimalText := strings.TrimSpace(decimalStringFromAny(value)); len(decimalText) > maxDecimalInputLength {
		return fmt.Errorf("%s exceeds maximum length of %d characters", field, maxDecimalInputLength)
	}

	return nil
}

func decimalStringFromAny(value any) string {
	if decimalValue, ok := amountDecimalString(value); ok {
		return decimalValue
	}

	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case json.Number:
		return v.String()
	case decimal.Decimal:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func amountDecimalString(value any) (string, bool) {
	switch v := value.(type) {
	case *decimal.Decimal:
		if v == nil {
			return "", true
		}

		return v.String(), true
	case Amount:
		if v.Value == nil {
			return "", true
		}

		return v.Value.String(), true
	case *Amount:
		if v == nil || v.Value == nil {
			return "", true
		}

		return v.Value.String(), true
	default:
		return "", false
	}
}

// DecimalStringFromAny converts common numeric response values to exact decimal strings.
func DecimalStringFromAny(value any) string {
	return decimalStringFromAny(value)
}

// CreateTransactionInput is the input for creating a transaction.
// This structure contains all the fields needed to create a new transaction.
//
// CreateTransactionInput is used with the Transactions accessor's CreateJSON method
// to create new transactions in the standard format. The money movement lives in
// Send; the rest of the fields carry presentation, routing, and metadata.
//
// When creating a transaction, the send payload must include a source and a
// distribution whose values balance for each asset. Set IdempotencyKey or use
// sdkctx.WithIdempotencyKey for retry-safe unsafe requests.
//
// Example - Creating a simple payment transaction:
//
//	input := models.NewCreateTransactionInput("USD", "100.00").WithSend(&models.SendInput{
//	    Asset: "USD",
//	    Value: "100.00",
//	    Source: &models.SourceInput{From: []models.FromToInput{
//	        {AccountAlias: "customer_john_doe", Amount: models.AmountInput{Asset: "USD", Value: "100.00"}},
//	    }},
//	    Distribute: &models.DistributeInput{To: []models.FromToInput{
//	        {AccountAlias: "merchant_primary", Amount: models.AmountInput{Asset: "USD", Value: "100.00"}},
//	    }},
//	}).WithMetadata(map[string]any{"invoice_id": "inv-123"})
//	input.IdempotencyKey = "payment-inv123-20230401"
//
// Example - Creating a pending transaction:
//
//	input := models.NewCreateTransactionInput("USD", "1000.00").WithPending(true)
//	input = input.WithSend(&models.SendInput{/* source and distribute omitted for brevity */})
//
//	// Later, after approval:
//	// c.Transactions.Commit(ctx, orgID, ledgerID, tx.ID)
type CreateTransactionInput struct {
	// Template is retained for backwards compatibility with the pre-send API.
	Template string `json:"template,omitempty"`

	// ChartOfAccountsGroupName optionally categorizes the transaction under a chart of accounts group.
	ChartOfAccountsGroupName string `json:"chartOfAccountsGroupName,omitempty"`

	// Description is an optional human-readable description of the transaction.
	Description string `json:"description,omitempty"`

	// Pending indicates whether the transaction should be created in a pending state
	// Pending transactions require explicit commitment before affecting account balances
	Pending bool `json:"pending,omitempty"`

	// Code is a transaction reference code.
	Code string `json:"code,omitempty"`

	// Route is the transaction route identifier (optional)
	// This defines the overall flow of the transaction structure.
	// Prefer RouteID (UUID); Route is retained for server-side alias
	// compatibility. Setting both is rejected by Validate.
	Route string `json:"route,omitempty"`

	// RouteID is the UUID transaction route identifier and the preferred way to
	// address a transaction route. Mutually exclusive with Route.
	RouteID string `json:"routeId,omitempty"`

	// TransactionDate is the effective date/time for the transaction.
	TransactionDate string `json:"transactionDate,omitempty"`

	// Metadata contains additional custom data for the transaction
	// This can be used to store application-specific information
	// such as references to external systems, tags, or other contextual data
	Metadata map[string]any `json:"metadata,omitempty"`

	// IdempotencyKey is a client-generated key to ensure transaction uniqueness
	// If a transaction with the same idempotency key already exists, that transaction
	// will be returned instead of creating a new one
	// Note: This is sent as a header (X-Idempotency), not in the request body
	IdempotencyKey string `json:"-"`

	// Send contains the source and distribution information for the transaction.
	// It is the only way to describe the money movement of a create; the legacy
	// Operations list was removed in v4.2.
	Send *SendInput `json:"send"`
}

// SendInput represents the send information for a transaction.
// This structure contains the source and distribution details for a transaction.
type SendInput struct {
	// Asset identifies the currency or asset type for this transaction
	Asset string `json:"asset"`

	// Value is the exact decimal value of the transaction.
	Value any `json:"value"`

	// Source contains the source accounts for the transaction
	Source *SourceInput `json:"source"`

	// Distribute contains the destination accounts for the transaction
	Distribute *DistributeInput `json:"distribute"`
}

// SourceInput represents the source information for a transaction.
// This structure contains the source accounts for a transaction.
type SourceInput struct {
	// From contains the list of source accounts and amounts
	From []FromToInput `json:"from"`
}

// DistributeInput represents the distribution information for a transaction.
// This structure contains the destination accounts for a transaction.
type DistributeInput struct {
	// To contains the list of destination accounts and amounts
	To []FromToInput `json:"to"`
}

// FromToInput represents a single source or destination account in a transaction.
// This structure contains the account and amount details.
type FromToInput struct {
	// AccountAlias identifies the account affected by this operation. It is the
	// leg's only account identity and is sent as accountAlias; the Midaz
	// transaction endpoints resolve an alias or an account ID from it.
	AccountAlias string `json:"accountAlias"`

	// Amount specifies the amount details for this operation
	Amount AmountInput `json:"amount"`

	// Route is the operation route identifier for this operation (optional).
	// Prefer RouteID (UUID); Route is retained for server-side alias
	// compatibility. Setting both is rejected by Validate.
	Route string `json:"route,omitempty"`

	// RouteID is the operation route UUID used by canonical Midaz route
	// validation and the preferred way to address an operation route.
	// Mutually exclusive with Route.
	RouteID *string `json:"routeId,omitempty"`

	// BalanceKey targets a non-default balance for this entry.
	BalanceKey string `json:"balanceKey,omitempty"`

	// Share specifies proportional distribution semantics.
	Share *Share `json:"share,omitempty"`

	// Remaining specifies the remaining-account token for split transactions.
	Remaining string `json:"remaining,omitempty"`

	// Rate specifies exchange-rate information for multi-asset flows.
	Rate *Rate `json:"rate,omitempty"`

	// Description provides additional context for this operation (optional)
	Description string `json:"description,omitempty"`

	// ChartOfAccounts specifies the chart of accounts for this operation (optional)
	ChartOfAccounts string `json:"chartOfAccounts,omitempty"`

	// Metadata contains additional custom data for this operation
	Metadata map[string]any `json:"metadata,omitempty"`
}

// AmountInput represents the amount details for an operation.
// This structure contains the value and asset code for an amount.
type AmountInput struct {
	// Asset identifies the currency or asset type for this amount
	Asset string `json:"asset"`

	// Value is the exact decimal value of the amount.
	Value any `json:"value"`
}

// Share specifies proportional (percentage) distribution on a transaction leg.
// It is a live money-path type used by FromToInput.Share (share-based distribute);
// relocated here from the removed transaction_dsl.go in Task 5.5.3.
type Share struct {
	Percentage             int64 `json:"percentage"`
	PercentageOfPercentage int64 `json:"percentageOfPercentage,omitempty"`
}

// Rate specifies exchange-rate information on a transaction leg. It is a live
// money-path type used by FromToInput.Rate; relocated here from the removed
// transaction_dsl.go in Task 5.5.3.
type Rate struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Value      any    `json:"value"`
	ExternalID string `json:"externalId"`
}

const (
	maxTransactionDescriptionLength = 256
	maxTransactionCodeLength        = 100
)

// transactionDateFormats require explicit timezone information to avoid local-date ambiguity.
var transactionDateFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000Z",
	"2006-01-02T15:04:05Z",
}

// Validate checks that the CreateTransactionInput meets all validation requirements.
// It returns an error if any of the validation checks fail.
func (input *CreateTransactionInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	appendTransactionCreateCommon(&errs, input.Description, input.Code, input.Metadata,
		input.Route, input.RouteID, input.TransactionDate, input.Pending)

	if input.Send == nil {
		errs.Append("send", "is required; legacy Operations input was removed in v4.2")
	} else if err := input.Send.Validate(); err != nil {
		errs.Append("send", "invalid: "+err.Error())
	}

	return errs.OrNil()
}

// appendTransactionCreateCommon collects the field-level validation
// problems shared by every transaction-create flow (Create, Inflow,
// Outflow, Annotation). Each helper-discovered violation lands on errs
// under its proper field name; nothing short-circuits.
func appendTransactionCreateCommon(errs *validation.FieldErrors, description, code string, metadata map[string]any, route, routeID, transactionDate string, pending bool) {
	if len(description) > maxTransactionDescriptionLength {
		errs.Append("description", "must be at most 256 characters")
	}

	if len(code) > maxTransactionCodeLength {
		errs.Append("code", "must be at most 100 characters")
	}

	if route != "" && len(route) > 250 {
		errs.Append("route", "must be at most 250 characters")
	}

	if routeID != "" && !validation.IsValidUUID(routeID) {
		errs.Append("routeId", "must be a valid UUID")
	}

	// The serializers emit whichever of route/routeId is non-empty, so a payload
	// carrying both hands the routing decision to the ledger. Reject the pair.
	if route != "" && routeID != "" {
		errs.Append("route", "transaction-level route and routeId are mutually exclusive; keep routeId")
	}

	if transactionDate != "" {
		parsedDate, err := parseTransactionDate(transactionDate)
		switch {
		case err != nil:
			errs.Append("transactionDate", err.Error())
		case parsedDate.After(time.Now()):
			errs.Append("transactionDate", "cannot be in the future")
		case pending:
			errs.Append("transactionDate", "pending transactions cannot have a custom transactionDate")
		}
	}

	if len(metadata) > 0 {
		if err := core.ValidateMetadata(metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}
}

func parseTransactionDate(value string) (time.Time, error) {
	for _, format := range transactionDateFormats {
		if parsed, err := time.Parse(format, value); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, errors.New("transactionDate must be RFC3339 date-time with explicit timezone")
}

func transactionCommonMap(chartOfAccountsGroupName, description, code string, metadata map[string]any, route, routeID, transactionDate string, pending bool) map[string]any {
	tx := map[string]any{}
	if chartOfAccountsGroupName != "" {
		tx["chartOfAccountsGroupName"] = chartOfAccountsGroupName
	}

	if description != "" {
		tx["description"] = description
	}

	if code != "" {
		tx["code"] = code
	}

	if pending {
		tx["pending"] = pending
	}

	if len(metadata) > 0 {
		tx["metadata"] = metadata
	}

	if route != "" {
		tx["route"] = route
	}

	if routeID != "" {
		tx["routeId"] = routeID
	}

	if transactionDate != "" {
		tx["transactionDate"] = transactionDate
	}

	return tx
}

// NewCreateTransactionInput creates a new CreateTransactionInput with the send asset/value initialized.
func NewCreateTransactionInput(assetCode string, amount any) *CreateTransactionInput {
	amountValue := decimalStringFromAny(amount)

	return &CreateTransactionInput{
		Send: &SendInput{
			Asset: assetCode,
			Value: amountValue,
		},
	}
}

// WithDescription sets the description.
// This adds a human-readable description to the transaction.
func (input *CreateTransactionInput) WithDescription(description string) *CreateTransactionInput {
	if input == nil {
		return nil
	}

	input.Description = description

	return input
}

// WithMetadata sets the metadata.
// This adds custom key-value data to the transaction.
func (input *CreateTransactionInput) WithMetadata(metadata map[string]any) *CreateTransactionInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithRouteID sets the route UUID.
func (input *CreateTransactionInput) WithRouteID(routeID string) *CreateTransactionInput {
	if input == nil {
		return nil
	}

	input.RouteID = routeID

	return input
}

// WithTransactionDate sets the transaction effective date/time.
func (input *CreateTransactionInput) WithTransactionDate(transactionDate string) *CreateTransactionInput {
	if input == nil {
		return nil
	}

	input.TransactionDate = transactionDate

	return input
}

// WithPending sets whether the transaction should be created in pending state.
func (input *CreateTransactionInput) WithPending(pending bool) *CreateTransactionInput {
	if input == nil {
		return nil
	}

	input.Pending = pending

	return input
}

// WithSend sets the send structure.
// This provides an alternative way to define transaction flow.
func (input *CreateTransactionInput) WithSend(send *SendInput) *CreateTransactionInput {
	if input == nil {
		return nil
	}

	input.Send = send

	return input
}

// Validate checks that the SendInput meets all validation requirements.
// It returns an error if any of the validation checks fail. Field-level
// violations are accumulated.
func (input *SendInput) Validate() error {
	if input == nil {
		return errors.New("send is required")
	}

	var errs validation.FieldErrors

	if input.Asset == "" {
		errs.Append("asset", "is required")
	}

	if err := validatePositiveDecimalString(input.Value, "value"); err != nil {
		errs.Append("value", err.Error())
	}

	if input.Source == nil {
		errs.Append("source", "is required")
	} else if err := input.Source.Validate(); err != nil {
		errs.Append("source", "invalid: "+err.Error())
	}

	if input.Distribute == nil {
		errs.Append("distribute", "is required")
	} else if err := input.Distribute.Validate(); err != nil {
		errs.Append("distribute", "invalid: "+err.Error())
	}

	appendSendBalanceErrors(&errs, input)

	return errs.OrNil()
}

func appendSendBalanceErrors(errs *validation.FieldErrors, input *SendInput) {
	balances, ok := fixedSendBalance(input)
	if errs == nil || !ok {
		return
	}

	if balances.sourceAsset != "" && balances.sourceAsset != balances.asset {
		errs.Append("source", "amount assets must match send asset")
	}

	if balances.distributeAsset != "" && balances.distributeAsset != balances.asset {
		errs.Append("distribute", "amount assets must match send asset")
	}

	if !balances.sourceTotal.Equal(balances.distributeTotal) {
		errs.Append("send", "source and distribute totals must match")
	}

	if !balances.sourceTotal.Equal(balances.sendValue) || !balances.distributeTotal.Equal(balances.sendValue) {
		errs.Append("send", "value must equal source and distribute totals")
	}
}

type sendBalanceTotals struct {
	asset           string
	sendValue       decimal.Decimal
	sourceTotal     decimal.Decimal
	distributeTotal decimal.Decimal
	sourceAsset     string
	distributeAsset string
}

func fixedSendBalance(input *SendInput) (sendBalanceTotals, bool) {
	if input == nil || input.Source == nil || input.Distribute == nil {
		return sendBalanceTotals{}, false
	}

	asset := strings.TrimSpace(input.Asset)
	if asset == "" {
		return sendBalanceTotals{}, false
	}

	sendValue, err := decimal.NewFromString(strings.TrimSpace(decimalStringFromAny(input.Value)))
	if err != nil {
		return sendBalanceTotals{}, false
	}

	sourceTotal, sourceOK, sourceAsset := sumFixedAmountEntries(input.Source.From, asset)
	distributeTotal, distributeOK, distributeAsset := sumFixedAmountEntries(input.Distribute.To, asset)
	if !sourceOK || !distributeOK {
		return sendBalanceTotals{}, false
	}

	return sendBalanceTotals{
		asset:           asset,
		sendValue:       sendValue,
		sourceTotal:     sourceTotal,
		distributeTotal: distributeTotal,
		sourceAsset:     sourceAsset,
		distributeAsset: distributeAsset,
	}, true
}

func sumFixedAmountEntries(entries []FromToInput, expectedAsset string) (decimal.Decimal, bool, string) {
	total := decimal.Zero

	for _, entry := range entries {
		if entry.Share != nil || strings.TrimSpace(entry.Remaining) != "" || entry.Rate != nil {
			return decimal.Zero, false, ""
		}

		asset := strings.TrimSpace(entry.Amount.Asset)
		if asset != "" && asset != expectedAsset {
			return decimal.Zero, true, asset
		}

		amount, err := decimal.NewFromString(strings.TrimSpace(decimalStringFromAny(entry.Amount.Value)))
		if err != nil {
			return decimal.Zero, false, ""
		}

		total = total.Add(amount)
	}

	return total, true, ""
}

// Validate checks that the SourceInput meets all validation requirements.
// All per-entry violations are accumulated.
func (input *SourceInput) Validate() error {
	if input == nil {
		return errors.New("source is required")
	}

	if len(input.From) == 0 {
		return errors.New("from is required")
	}

	var errs validation.FieldErrors

	for i, from := range input.From {
		if err := from.Validate(); err != nil {
			errs.Append(fmt.Sprintf("from[%d]", i), "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// Validate checks that the DistributeInput meets all validation requirements.
// All per-entry violations are accumulated.
func (input *DistributeInput) Validate() error {
	if input == nil {
		return errors.New("distribute is required")
	}

	if len(input.To) == 0 {
		return errors.New("to is required")
	}

	var errs validation.FieldErrors

	for i, to := range input.To {
		if err := to.Validate(); err != nil {
			errs.Append(fmt.Sprintf("to[%d]", i), "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// Validate checks that the FromToInput meets all validation requirements.
// Field-level violations are accumulated.
func (input *FromToInput) Validate() error {
	if input == nil {
		return errors.New("from/to entry is required")
	}

	var errs validation.FieldErrors

	if strings.TrimSpace(input.AccountAlias) == "" {
		errs.Append("accountAlias", "is required")
	}

	input.appendValueErrors(&errs)

	if input.RouteID != nil && strings.TrimSpace(*input.RouteID) != "" {
		if !validation.IsValidUUID(*input.RouteID) {
			errs.Append("routeId", "must be a valid UUID")
		}
	}

	if input.Route != "" && input.RouteID != nil && *input.RouteID != "" {
		errs.Append("route", fmt.Sprintf(
			"leg accountAlias=%s: route and routeId are mutually exclusive; keep routeId", input.AccountAlias))
	}

	if len(input.Metadata) > 0 {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// appendValueErrors validates the leg's value mechanism: exactly one of a fixed
// amount, a percentage share, a remaining-balance token, or an exchange rate.
// Share/remaining/rate legs are resolved server-side (see sumFixedAmountEntries,
// which skips the client balance check when any is present), so only a
// fixed-amount leg is amount-validated here. Validating amount unconditionally
// rejected every share leg and made GenerateWithDSL/GenerateBatch ship zero
// transactions.
func (input *FromToInput) appendValueErrors(errs *validation.FieldErrors) {
	amountValue := strings.TrimSpace(decimalStringFromAny(input.Amount.Value))
	hasAmount := input.Amount.Asset != "" || amountValue != ""

	// A leg specifies exactly ONE value mechanism. The /transactions/json server
	// rejects a leg carrying more than one of amount/share/remaining/rate with a
	// 422; enforce it client-side so the two agree (there is no valid amount+share
	// combination).
	if hasAmount && (input.Share != nil || strings.TrimSpace(input.Remaining) != "" || input.Rate != nil) {
		errs.Append("amount", "cannot be combined with share, remaining, or rate")

		return
	}

	switch {
	case hasAmount:
		if err := input.Amount.Validate(); err != nil {
			errs.Append("amount", "invalid: "+err.Error())
		}
	case input.Share != nil:
		if input.Share.Percentage < 1 || input.Share.Percentage > 100 {
			errs.Append("share.percentage", "must be between 1 and 100")
		}

		if input.Share.PercentageOfPercentage < 0 || input.Share.PercentageOfPercentage > 100 {
			errs.Append("share.percentageOfPercentage", "must be between 0 and 100")
		}
	case strings.TrimSpace(input.Remaining) != "", input.Rate != nil:
		// remaining/rate legs carry no client-checkable amount; the server resolves them.
	default:
		errs.Append("amount", "one of amount, share, remaining, or rate is required")
	}
}

// Validate checks that the AmountInput meets all validation requirements.
func (input *AmountInput) Validate() error {
	if input == nil {
		return errors.New("amount is required")
	}

	var errs validation.FieldErrors

	if input.Asset == "" {
		errs.Append("asset", "is required")
	}

	if err := validatePositiveDecimalString(input.Value, "value"); err != nil {
		errs.Append("value", err.Error())
	}

	return errs.OrNil()
}

// ToLibTransaction converts a CreateTransactionInput into the loosely-typed
// map[string]any payload expected by the Midaz transactions endpoint. It is
// the documented adapter between the SDK's typed input shape and the
// wire-level JSON object — consumers may call it directly when they need
// to inspect or mutate the payload before it leaves the process (for
// example, to attach extension fields the typed model doesn't yet
// expose).
//
// This is stable internal-use API: stable enough that the SDK's own
// transaction service depends on it on every Create call.
func (input *CreateTransactionInput) ToLibTransaction() map[string]any {
	if input == nil {
		return nil
	}

	// Create a map to hold the transaction data
	tx := map[string]any{}

	// Add chart of accounts group name if provided.
	if input.ChartOfAccountsGroupName != "" {
		tx["chartOfAccountsGroupName"] = input.ChartOfAccountsGroupName
	}

	// Only add description if provided.
	if input.Description != "" {
		tx["description"] = input.Description
	}

	// Add pending field if set
	if input.Pending {
		tx["pending"] = input.Pending
	}

	if input.Code != "" {
		tx["code"] = input.Code
	}

	// Add route if provided
	if input.Route != "" {
		tx["route"] = input.Route
	}

	if input.RouteID != "" {
		tx["routeId"] = input.RouteID
	}

	if input.TransactionDate != "" {
		tx["transactionDate"] = input.TransactionDate
	}

	// Add send information if present (required by API)
	if input.Send != nil {
		tx["send"] = input.Send.ToMap()
	}

	// Only add metadata if provided
	if len(input.Metadata) > 0 {
		tx["metadata"] = input.Metadata
	}

	return tx
}

// ToMap converts a SendInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *SendInput) ToMap() map[string]any {
	if input == nil {
		return nil
	}

	send := map[string]any{
		"asset": input.Asset,
		"value": decimalStringFromAny(input.Value),
	}

	// Add source information if present
	if input.Source != nil {
		send["source"] = input.Source.ToMap()
	}

	// Add distribute information if present
	if input.Distribute != nil {
		send["distribute"] = input.Distribute.ToMap()
	}

	return send
}

// ToMap converts a SourceInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *SourceInput) ToMap() map[string]any {
	if input == nil {
		return nil
	}

	source := map[string]any{}

	// Add from information if present
	if len(input.From) > 0 {
		fromList := make([]map[string]any, 0, len(input.From))
		for _, from := range input.From {
			fromList = append(fromList, from.ToMap())
		}

		source["from"] = fromList
	}

	return source
}

// ToMap converts a DistributeInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *DistributeInput) ToMap() map[string]any {
	if input == nil {
		return nil
	}

	distribute := map[string]any{}

	// Add to information if present
	if len(input.To) > 0 {
		toList := make([]map[string]any, 0, len(input.To))
		for _, to := range input.To {
			toList = append(toList, to.ToMap())
		}

		distribute["to"] = toList
	}

	return distribute
}

// ToMap converts a FromToInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input FromToInput) ToMap() map[string]any {
	fromTo := map[string]any{
		"accountAlias": input.AccountAlias,
	}

	if input.BalanceKey != "" {
		fromTo["balanceKey"] = input.BalanceKey
	}

	if amount := input.Amount.ToMap(); amount != nil {
		fromTo["amount"] = amount
	}

	if input.Share != nil {
		fromTo["share"] = input.Share
	}

	if input.Remaining != "" {
		fromTo["remaining"] = input.Remaining
	}

	if input.Rate != nil {
		fromTo["rate"] = input.Rate
	}

	if input.Description != "" {
		fromTo["description"] = input.Description
	}

	if input.ChartOfAccounts != "" {
		fromTo["chartOfAccounts"] = input.ChartOfAccounts
	}

	if len(input.Metadata) > 0 {
		fromTo["metadata"] = input.Metadata
	}

	if input.Route != "" {
		fromTo["route"] = input.Route
	}

	if input.RouteID != nil && *input.RouteID != "" {
		fromTo["routeId"] = *input.RouteID
	}

	return fromTo
}

// ToMap converts an AmountInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *AmountInput) ToMap() map[string]any {
	if input == nil {
		return nil
	}

	// Value is `any`, so the old `input.Value == ""` guard never fired for a
	// nil Value (nil != ""). Render through decimalStringFromAny, which returns
	// "" for both nil and "": an empty amount then omits the map entirely so a
	// share/remaining/rate leg does not ship amount:{"asset":"","value":""}
	// beside its share (which the /transactions/json contract rejects).
	value := decimalStringFromAny(input.Value)
	if input.Asset == "" && value == "" {
		return nil
	}

	return map[string]any{
		"asset": input.Asset,
		"value": value,
	}
}

// The transaction create inputs are wire-authoritative: MarshalJSON routes each
// of them through its endpoint mapper (ToLibTransaction for /json, ToMap for the
// nested nodes), so json.Marshal(input) IS the body the SDK sends. Struct tags
// cannot drift from the wire because nothing serializes through them any more —
// anyone logging, diffing, or persisting a marshaled input sees exactly what the
// ledger received. Receivers are values so a leg copied out of a slice marshals
// identically to a pointer to it.

// MarshalJSON emits the /transactions/json request body (ToLibTransaction).
func (input CreateTransactionInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(input.ToLibTransaction())
}

// MarshalJSON emits the send envelope as the transaction endpoints accept it.
func (input SendInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(input.ToMap())
}

// MarshalJSON emits the source envelope as the transaction endpoints accept it.
func (input SourceInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(input.ToMap())
}

// MarshalJSON emits the distribute envelope as the transaction endpoints accept it.
func (input DistributeInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(input.ToMap())
}

// MarshalJSON emits a transaction leg as the transaction endpoints accept it:
// one account identity (accountAlias), never the Account/AccountAlias pair.
func (input FromToInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(input.ToMap())
}

// MarshalJSON emits a leg amount, or null when the leg carries no amount (a
// share, remaining, or rate leg — the endpoints reject an empty amount object).
func (input AmountInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(input.ToMap())
}

// ToTransactionMap converts an SDK Transaction to a map for API requests.
// This method is used internally to prepare data for the backend API.
func (t *Transaction) ToTransactionMap() map[string]any {
	if t == nil {
		return nil
	}

	transaction := map[string]any{
		"description": t.Description,
		"metadata":    t.Metadata,
	}

	// Build send structure
	send := map[string]any{
		"asset": t.AssetCode,
		"value": t.Amount,
	}

	// Source (debits)
	source := map[string]any{}
	fromEntries := []map[string]any{}

	// Distribute (credits)
	distribute := map[string]any{}
	toEntries := []map[string]any{}

	// Convert Operations
	for _, op := range t.Operations {
		accountAlias := op.AccountAlias
		if accountAlias == "" {
			accountAlias = op.AccountID
		}

		entry := map[string]any{
			"accountAlias": accountAlias,
			"amount": map[string]any{
				"value": op.Amount.Value,
				"asset": op.AssetCode,
			},
		}

		// Add to appropriate list based on operation type.
		// The Midaz API returns operation types in uppercase ("DEBIT"/"CREDIT");
		// EqualFold defends against any casing drift. A direction inversion here
		// would silently mis-route every debit, so we compare case-insensitively.
		if strings.EqualFold(op.Type, string(OperationTypeDebit)) {
			fromEntries = append(fromEntries, entry)
		} else {
			toEntries = append(toEntries, entry)
		}
	}

	// Add from entries if any exist
	if len(fromEntries) > 0 {
		source["from"] = fromEntries
		send["source"] = source
	}

	// Add to entries if any exist
	if len(toEntries) > 0 {
		distribute["to"] = toEntries
		send["distribute"] = distribute
	}

	// Add send to transaction
	transaction["send"] = send

	return transaction
}

// UpdateTransactionInput represents the input for updating a transaction.
// This structure contains the fields that can be updated on an existing transaction.
//
// UpdateTransactionInput is used with the Transactions accessor's UpdateTransaction method
// to update existing transactions. It allows for updating metadata and other mutable
// properties of a transaction.
//
// Note that not all fields of a transaction can be updated after creation, especially
// for transactions that have already been committed. Typically, only metadata and
// certain status-related fields can be modified.
//
// Example - Updating transaction metadata:
//
//	// Update a transaction's metadata
//	input := &models.UpdateTransactionInput{
//	    Metadata: map[string]any{
//	        "updated_by": "admin",
//	        "approval_status": "approved",
//	        "notes": "Verified and approved by finance team",
//	    },
//	}
//
//	updatedTx, err := c.Transactions.UpdateTransaction(
//	    ctx, orgID, ledgerID, transactionID, input,
//	)
type UpdateTransactionInput struct {
	// Metadata contains additional custom data for the transaction
	// This can be used to store application-specific information
	// such as references to external systems, tags, or other contextual data
	Metadata map[string]any `json:"metadata,omitempty"`

	// Description is a human-readable description of the transaction
	// This should provide context about the purpose or nature of the transaction
	Description string `json:"description,omitempty"`

	// ExternalID is retained for backward compatibility but is not part of the
	// current Midaz UpdateTransaction contract.
	ExternalID string `json:"-"`
}

// Validate checks if the UpdateTransactionInput meets the validation requirements.
// It returns an error if any of the validation checks fail.
//
// Returns:
//   - error: An error if the input is invalid, nil otherwise
func (input *UpdateTransactionInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	var errs validation.FieldErrors

	if input.Description != "" && len(input.Description) > 256 {
		errs.Append("description", "must not exceed 256 characters")
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// hasChanges reports whether the update payload contains any actionable
// mutation. ExternalID is intentionally excluded — it is documented as
// deprecated and the backend silently ignores it on update; including it
// here would make "set ExternalID, no other change" look like a real
// update and fire a marshal/round-trip for an effectively no-op call.
func (input *UpdateTransactionInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.Metadata != nil || input.Description != ""
}

// NewUpdateTransactionInput creates a new UpdateTransactionInput.
// This constructor initializes an empty update input that can be customized
// using the With* methods for a fluent API experience.
//
// Returns:
//   - A pointer to the newly created UpdateTransactionInput
func NewUpdateTransactionInput() *UpdateTransactionInput {
	return &UpdateTransactionInput{}
}

// WithMetadata sets the metadata.
// This method allows updating the custom metadata associated with the transaction.
//
// Parameters:
//   - metadata: A map of key-value pairs to store as transaction metadata
//
// Returns:
//   - A pointer to the modified UpdateTransactionInput for method chaining
func (input *UpdateTransactionInput) WithMetadata(metadata map[string]any) *UpdateTransactionInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithDescription sets the description.
// This method allows updating the human-readable description of the transaction.
//
// Parameters:
//   - description: The new description for the transaction
//
// Returns:
//   - A pointer to the modified UpdateTransactionInput for method chaining
func (input *UpdateTransactionInput) WithDescription(description string) *UpdateTransactionInput {
	if input == nil {
		return nil
	}

	input.Description = description

	return input
}

// WithExternalID sets the external ID.
// This method allows setting or updating the external system identifier for the transaction.
// Deprecated: externalId is not sent in the current Midaz UpdateTransaction contract.
//
// Parameters:
//   - externalID: The external identifier for linking to other systems
//
// Returns:
//   - A pointer to the modified UpdateTransactionInput for method chaining
func (input *UpdateTransactionInput) WithExternalID(externalID string) *UpdateTransactionInput {
	if input == nil {
		return nil
	}

	input.ExternalID = externalID

	return input
}

// CreateInflowInput represents input for creating an inflow transaction.
// Inflow transactions have no source - funds flow into the system (e.g., deposits, funding).

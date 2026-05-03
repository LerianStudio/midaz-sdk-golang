// Package validation provides validation utilities for the Midaz SDK.
//
// This package contains functions for validating various aspects of Midaz data:
// - Transaction validation (DSL, standard inputs)
// - Asset code and type validation
// - Account alias and type validation
// - Metadata validation
// - Address validation
// - Date range validation
//
// These utilities help ensure that data is valid before sending it to the API,
// providing early feedback and preventing unnecessary API calls with invalid data.
//
// The package implements the functional options pattern for configuring validation behavior.
// Example:
//
//	validator, err := NewValidator(
//	    WithMaxMetadataSize(8192),
//	    WithStrictMode(true),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use the validator
//	if err := validator.ValidateMetadata(metadata); err != nil {
//	    log.Fatal(err)
//	}
package validation

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation/core"
	midazutils "github.com/LerianStudio/midaz/v3/pkg/utils"
)

// Validator is a configurable validation instance that can be used to perform validations
// with specific configuration options.
type Validator struct {
	config *core.ValidationConfig
}

// DefaultValidator returns a new validator with default configuration
func DefaultValidator() *Validator {
	config := core.DefaultValidationConfig()

	return &Validator{
		config: config,
	}
}

// NewValidator creates a new Validator with the provided options
func NewValidator(options ...core.ValidationOption) (*Validator, error) {
	config, err := core.NewValidationConfig(options...)
	if err != nil {
		return nil, err
	}

	return &Validator{
		config: config,
	}, nil
}

// The package also provides standalone functions for backward compatibility.
// All standalone functions use the default validator configuration.
var defaultValidator = DefaultValidator()

// Operation type constants for transaction operations.
const (
	// OpTypeDebit represents a debit operation type.
	OpTypeDebit = "DEBIT"
	// OpTypeCredit represents a credit operation type.
	OpTypeCredit = "CREDIT"
)

// externalAccountPattern mirrors core.ExternalAccountPattern. Both bind the
// captured asset code to 3-4 uppercase letters; we re-declare here to keep
// the call sites in this file self-contained.
var externalAccountPattern = core.ExternalAccountPattern

// assetCodePattern mirrors core.AssetCodePattern (3-4 uppercase letters).
var assetCodePattern = core.AssetCodePattern

// chartOfAccountsGroupNamePattern is the regex pattern for chart of accounts group names.
// Allows alphanumeric characters, spaces, underscores, and hyphens.
var chartOfAccountsGroupNamePattern = regexp.MustCompile(`^[a-zA-Z0-9 _-]+$`)

// TransactionDSLValidator defines an interface for transaction DSL validation
type TransactionDSLValidator interface {
	GetAsset() string
	GetValue() float64
	GetSourceAccounts() []AccountReference
	GetDestinationAccounts() []AccountReference
	GetMetadata() map[string]any
}

// AccountReference defines an interface for account references in transactions
type AccountReference interface {
	GetAccount() string
}

// ValidateTransactionDSL performs pre-validation of transaction DSL input
// before sending to the API to catch common errors early
func ValidateTransactionDSL(input TransactionDSLValidator) error {
	if input == nil {
		return errors.New("transaction input cannot be nil")
	}

	// Validate asset code
	asset := input.GetAsset()
	if asset == "" {
		return errors.New("asset code is required")
	}

	if !assetCodePattern.MatchString(asset) {
		return fmt.Errorf("invalid asset code format: %s (must be 3-4 uppercase letters)", asset)
	}

	// Validate amount
	if !isFinitePositive(input.GetValue()) {
		return errors.New("transaction amount must be greater than zero")
	}

	// Validate source accounts
	sourceAccounts := input.GetSourceAccounts()
	if len(sourceAccounts) == 0 {
		return errors.New("at least one source account is required")
	}

	if err := validateAccountReferences(sourceAccounts, asset, "source"); err != nil {
		return err
	}

	// Validate destination accounts
	destAccounts := input.GetDestinationAccounts()
	if len(destAccounts) == 0 {
		return errors.New("at least one destination account is required")
	}

	if err := validateAccountReferences(destAccounts, asset, "destination"); err != nil {
		return err
	}

	// Validate asset consistency across external accounts
	if err := validateAssetConsistency(input); err != nil {
		return err
	}

	// Validate metadata if present
	metadata := input.GetMetadata()
	if metadata != nil {
		if err := ValidateMetadata(metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

func validateAccountReferences(accounts []AccountReference, asset, label string) error {
	for i, account := range accounts {
		if account == nil {
			return fmt.Errorf("invalid %s account at index %d: account reference cannot be nil", label, i)
		}

		if err := validateAccountReference(account.GetAccount(), asset); err != nil {
			return fmt.Errorf("invalid %s account at index %d: %w", label, i, err)
		}
	}

	return nil
}

// validateAssetConsistency checks that all accounts in the transaction
// are using the same asset code
func validateAssetConsistency(input TransactionDSLValidator) error {
	for _, account := range input.GetSourceAccounts() {
		if account == nil {
			continue
		}

		matches := externalAccountPattern.FindStringSubmatch(account.GetAccount())
		if len(matches) > 1 {
			externalAsset := matches[1]
			if externalAsset != input.GetAsset() {
				return fmt.Errorf("asset code mismatch: transaction uses %s but external account uses %s",
					input.GetAsset(), externalAsset)
			}
		}
	}

	for _, account := range input.GetDestinationAccounts() {
		if account == nil {
			continue
		}

		matches := externalAccountPattern.FindStringSubmatch(account.GetAccount())
		if len(matches) > 1 {
			externalAsset := matches[1]
			if externalAsset != input.GetAsset() {
				return fmt.Errorf("asset code mismatch: transaction uses %s but external account uses %s",
					input.GetAsset(), externalAsset)
			}
		}
	}

	return nil
}

// validateAccountReference checks if an account reference is valid
// for both regular accounts and external accounts
func validateAccountReference(account string, transactionAsset string) error {
	if account == "" {
		return errors.New("account reference cannot be empty")
	}

	// Check if it's an external account reference
	if strings.HasPrefix(account, "@external/") {
		// First check if it matches our expected pattern
		matches := externalAccountPattern.FindStringSubmatch(account)
		if len(matches) == 0 {
			return fmt.Errorf("invalid external account format: %s (must be @external/XXX where XXX is a valid asset code)", account)
		}

		externalAsset := matches[1]
		// Validate the external asset code format
		if !assetCodePattern.MatchString(externalAsset) {
			return fmt.Errorf("invalid asset code in external account: %s (must be 3-4 uppercase letters)", externalAsset)
		}

		// Validate that the external asset matches the transaction asset
		if externalAsset != transactionAsset {
			return fmt.Errorf("external account asset (%s) must match transaction asset (%s)",
				externalAsset, transactionAsset)
		}
	}

	return nil
}

func isFinitePositive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}

// GetExternalAccountReference creates a properly formatted external account reference
// for the given asset code
func GetExternalAccountReference(assetCode string) string {
	return fmt.Sprintf("@external/%s", assetCode)
}

// ValidateAssetCode checks if an asset code is valid.
// Asset codes should be 3-4 uppercase letters (e.g., USD, EUR, BTC).
//
// Example:
//
//	if err := validation.ValidateAssetCode("USD"); err != nil {
//	    log.Fatal(err)
//	}
func ValidateAssetCode(assetCode string) error {
	return core.ValidateAssetCode(assetCode)
}

// ValidateAccountAlias checks if an account alias is valid.
// Account aliases should be alphanumeric with optional underscores and hyphens.
//
// Example:
//
//	if err := validation.ValidateAccountAlias("savings_account"); err != nil {
//	    log.Fatal(err)
//	}
func ValidateAccountAlias(alias string) error {
	return core.ValidateAccountAlias(alias)
}

// ValidateTransactionCode checks if a transaction code is valid.
// Transaction codes should be alphanumeric with optional underscores and hyphens.
//
// Example:
//
//	if err := validation.ValidateTransactionCode("TX_123456"); err != nil {
//	    log.Fatal(err)
//	}
func ValidateTransactionCode(code string) error {
	return core.ValidateTransactionCode(code)
}

// ValidateMetadata checks if transaction metadata is valid with the default validator.
// This function verifies that metadata values are of supported types.
//
// Example:
//
//	metadata := map[string]any{
//	    "reference": "inv123",
//	    "amount": 100.50,
//	    "customer_id": 12345,
//	}
//	if err := validation.ValidateMetadata(metadata); err != nil {
//	    log.Fatal(err)
//	}
func ValidateMetadata(metadata map[string]any) error {
	return defaultValidator.ValidateMetadata(metadata)
}

// ValidateMetadata checks if transaction metadata is valid using this validator's configuration.
// This method verifies that metadata values are of supported types.
func (v *Validator) ValidateMetadata(metadata map[string]any) error {
	if v == nil || v.config == nil {
		return errors.New("validator cannot be nil")
	}

	if metadata == nil {
		return nil
	}

	// Validate metadata keys and values
	for key, value := range metadata {
		if err := v.validateMetadataKey(key); err != nil {
			return err
		}

		if err := v.validateMetadataValue(key, value); err != nil {
			return err
		}
	}

	// Check total metadata size
	return v.validateMetadataSize(metadata)
}

// validateMetadataKey validates a single metadata key.
//
// In addition to the empty-string and length checks, this rejects keys that
// MongoDB treats specially:
//   - keys containing '.' would be interpreted as dotted-path lookups, and
//   - keys with a leading '$' are reserved for operators ($set, $inc, ...).
//
// Catching these at the SDK boundary surfaces the failure where the user
// can fix it instead of after the backend persists a corrupt document.
func (*Validator) validateMetadataKey(key string) error {
	if key == "" {
		return errors.New("metadata key cannot be empty")
	}

	if len(key) > 100 {
		return fmt.Errorf("metadata key '%s' exceeds maximum length of 100 characters", key)
	}

	if strings.Contains(key, ".") || strings.HasPrefix(key, "$") {
		return fmt.Errorf("metadata key '%s' must not contain '.' or start with '$' (reserved by storage layer)", key)
	}

	return nil
}

// validateMetadataValue validates a single metadata value
func (v *Validator) validateMetadataValue(key string, value any) error {
	// Validate value type
	if !v.isValidMetadataValueType(value) {
		return fmt.Errorf("metadata value for key '%s' has unsupported type: %T (supported types: string, bool, number, array, nil)", key, value)
	}

	// Check string value length
	if strValue, ok := value.(string); ok {
		if len(strValue) > v.config.MaxStringLength {
			return fmt.Errorf("metadata string value for key '%s' exceeds maximum length of %d characters",
				key, v.config.MaxStringLength)
		}

		return nil
	}

	return core.ValidateMetadata(map[string]any{key: value})
}

// validateMetadataSize validates the total size of metadata
func (v *Validator) validateMetadataSize(metadata map[string]any) error {
	totalSize := 0
	for key, value := range metadata {
		totalSize += len(key)

		switch val := value.(type) {
		case string:
			totalSize += len(val)
		case bool, int, int32, int64, float32, float64:
			totalSize += 8 // Approximate size for these types
		case []any:
			totalSize += len(fmt.Sprint(val))
		}
	}

	if totalSize > v.config.MaxMetadataSize {
		return fmt.Errorf("total metadata size exceeds maximum allowed size of %d bytes",
			v.config.MaxMetadataSize)
	}

	return nil
}

// isValidMetadataValueType checks if a value is of a type supported in metadata
func (*Validator) isValidMetadataValueType(value any) bool {
	switch value.(type) {
	case string, bool, int, int32, int64, float32, float64, []any, nil:
		return true
	default:
		return false
	}
}

// ValidateDateRange checks if a date range is valid.
// The start date must not be after the end date.
//
// Example:
//
//	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
//	end := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
//	if err := validation.ValidateDateRange(start, end); err != nil {
//	    log.Fatal(err)
//	}
func ValidateDateRange(start, end time.Time) error {
	// Check if either date is zero
	if start.IsZero() {
		return errors.New("start date cannot be empty")
	}

	if end.IsZero() {
		return errors.New("end date cannot be empty")
	}

	// Check if start date is after end date
	if start.After(end) {
		return fmt.Errorf("start date (%s) cannot be after end date (%s)",
			start.Format("2006-01-02"), end.Format("2006-01-02"))
	}

	return nil
}

// Summary holds the results of a validation operation
// with multiple potential errors
type Summary struct {
	Valid  bool
	Errors []error
}

// AddError adds an error to the validation summary and marks it as invalid
func (vs *Summary) AddError(err error) {
	if err == nil {
		return
	}

	vs.Valid = false
	vs.Errors = append(vs.Errors, err)
}

// GetErrorMessages returns all error messages as a slice of strings
func (vs *Summary) GetErrorMessages() []string {
	if vs.Valid {
		return nil
	}

	messages := make([]string, len(vs.Errors))
	for i, err := range vs.Errors {
		if err == nil {
			messages[i] = "<nil validation error>"
			continue
		}

		messages[i] = err.Error()
	}

	return messages
}

// GetErrorSummary returns a single string with all error messages
func (vs *Summary) GetErrorSummary() string {
	if vs.Valid {
		return ""
	}

	var builder strings.Builder

	_, _ = fmt.Fprintf(&builder, "Validation failed with %d errors:\n", len(vs.Errors))

	for i, err := range vs.Errors {
		if err == nil {
			_, _ = fmt.Fprintf(&builder, "%d. <nil validation error>\n", i+1)
			continue
		}

		_, _ = fmt.Fprintf(&builder, "%d. %s\n", i+1, err.Error())
	}

	return builder.String()
}

// validateOperationType validates the operation type field
func validateOperationType(op map[string]any, index int) error {
	if op["type"] == nil {
		return fmt.Errorf("operation %d: type is required", index)
	}

	opType, ok := op["type"].(string)
	if !ok {
		return fmt.Errorf("operation %d: type must be a string", index)
	}

	if opType != OpTypeDebit && opType != OpTypeCredit {
		return fmt.Errorf("operation %d: invalid type '%s' (must be %s or %s)", index, opType, OpTypeDebit, OpTypeCredit)
	}

	return nil
}

// validateOperationAccountAlias validates the account alias field if provided
func validateOperationAccountAlias(op map[string]any, index int) error {
	if op["account_alias"] == nil {
		return nil
	}

	accountAlias, ok := op["account_alias"].(string)
	if !ok {
		return fmt.Errorf("operation %d: account_alias must be a string", index)
	}

	if accountAlias != "" {
		if err := ValidateAccountAlias(accountAlias); err != nil {
			return fmt.Errorf("operation %d: %w", index, err)
		}
	}

	return nil
}

// validateOperationAssetCode validates the asset code field if provided
func validateOperationAssetCode(op map[string]any, index int, transactionAssetCode string) error {
	if op["asset_code"] == nil {
		return nil
	}

	assetCode, ok := op["asset_code"].(string)
	if !ok {
		return fmt.Errorf("operation %d: asset_code must be a string", index)
	}

	if assetCode != "" && assetCode != transactionAssetCode {
		return fmt.Errorf("operation %d: asset code '%s' must match transaction asset code '%s'",
			index, assetCode, transactionAssetCode)
	}

	return nil
}

// validateOperationAmount validates the amount field
func validateOperationAmount(op map[string]any, index int) error {
	if op["amount"] == nil {
		return fmt.Errorf("operation %d: amount is required", index)
	}

	amount, ok := op["amount"].(float64)
	if !ok {
		// Try int conversion as JSON may unmarshal as int
		if intAmount, intOk := op["amount"].(int); intOk {
			amount = float64(intAmount)
			ok = true
		}
	}

	if !ok {
		return fmt.Errorf("operation %d: amount must be a number", index)
	}

	if !isFinitePositive(amount) {
		return fmt.Errorf("operation %d: amount must be greater than zero", index)
	}

	if math.Trunc(amount) != amount {
		return fmt.Errorf("operation %d: amount must be an integer minor unit", index)
	}

	return nil
}

// validateOperation validates a single operation in a transaction
func validateOperation(op map[string]any, index int, transactionAssetCode string) ([]error, bool) {
	var opErrors []error

	valid := true

	// Validate operation type
	if err := validateOperationType(op, index); err != nil {
		opErrors = append(opErrors, err)
		valid = false
	}

	// Validate account ID
	if op["account_id"] == nil {
		opErrors = append(opErrors, fmt.Errorf("operation %d: account ID is required", index))
		valid = false
	}

	// Validate account alias if provided
	if err := validateOperationAccountAlias(op, index); err != nil {
		opErrors = append(opErrors, err)
		valid = false
	}

	// Validate asset code if provided
	if err := validateOperationAssetCode(op, index, transactionAssetCode); err != nil {
		opErrors = append(opErrors, err)
		valid = false
	}

	// Validate amount
	if err := validateOperationAmount(op, index); err != nil {
		opErrors = append(opErrors, err)
		valid = false
	}

	return opErrors, valid
}

// validateChartOfAccountsGroupName validates the chart of accounts group name
func validateChartOfAccountsGroupName(name string) error {
	if name == "" {
		return errors.New("chart of accounts group name cannot be empty")
	}

	if len(name) > 100 {
		return fmt.Errorf("chart of accounts group name '%s' exceeds maximum length of 100 characters", name)
	}

	// Allow alphanumeric characters, spaces, underscores, and hyphens
	if !chartOfAccountsGroupNamePattern.MatchString(name) {
		return fmt.Errorf("chart of accounts group name '%s' contains invalid characters (allowed: alphanumeric, space, underscore, hyphen)", name)
	}

	return nil
}

// ValidateCreateTransactionInput performs comprehensive validation on a transaction input
// Returns a validation summary with multiple errors if found
//
// Example:
//
//	// Create a transaction
//	input := map[string]any{
//		"amount": 10000,
//		"scale":  2,
//		"asset_code": "USD",
//		"operations": []map[string]any{
//			{
//				"type":         "DEBIT",
//				"account_id":   "acc_123",
//				"account_alias": "savings",
//				"amount":       10000,
//			},
//			{
//				"type":         "CREDIT",
//				"account_id":   "acc_456",
//				"account_alias": "checking",
//				"amount":       10000,
//			},
//		},
//		"metadata": map[string]any{
//			"reference": "TX-123456",
//			"purpose": "Monthly transfer",
//		},
//	}
//
//	// Validate the input
//	summary := validation.ValidateCreateTransactionInput(input)
//	if !summary.Valid {
//		// Handle validation errors
//		fmt.Println(summary.GetErrorSummary())
//		return fmt.Errorf("transaction validation failed: %d errors found", len(summary.Errors))
//	}
//
//	// Proceed with creating the transaction
func ValidateCreateTransactionInput(input map[string]any) Summary {
	summary := Summary{
		Valid:  true,
		Errors: []error{},
	}

	if input == nil {
		summary.AddError(errors.New("transaction input cannot be nil"))
		return summary
	}

	if _, ok := input["send"]; ok {
		validateSendTransactionFields(&summary, input)
		validateAdditionalTransactionFields(&summary, input)

		return summary
	}

	validateBasicTransactionFields(&summary, input)
	validateTransactionOperations(&summary, input)
	validateAdditionalTransactionFields(&summary, input)

	return summary
}

func validateSendTransactionFields(summary *Summary, input map[string]any) {
	send, ok := input["send"].(map[string]any)
	if !ok {
		summary.AddError(errors.New("send must be an object"))
		return
	}

	asset, ok := send["asset"].(string)
	if !ok {
		summary.AddError(errors.New("send.asset must be a string"))
	}

	if err := ValidateAssetCode(asset); err != nil {
		summary.AddError(fmt.Errorf("send.asset: %w", err))
	}

	value, ok := extractNumericAmount(send["value"])
	if !ok || !isFinitePositive(value) {
		summary.AddError(errors.New("send.value must be a finite number greater than zero"))
	}

	validateSendEndpointList(summary, send["source"], "send.source.from", "from")
	validateSendEndpointList(summary, send["distribute"], "send.distribute.to", "to")
}

func validateSendEndpointList(summary *Summary, container any, field, child string) {
	containerMap, ok := container.(map[string]any)
	if !ok {
		summary.AddError(fmt.Errorf("%s parent must be an object", field))
		return
	}

	items, ok := extractObjectList(containerMap[child])
	if !ok || len(items) == 0 {
		summary.AddError(fmt.Errorf("%s must contain at least one object", field))
		return
	}

	for i, item := range items {
		validateSendEndpointItem(summary, field, i, item)
	}
}

func validateSendEndpointItem(summary *Summary, field string, index int, item map[string]any) {
	if alias, ok := item["accountAlias"].(string); ok && alias != "" {
		if err := ValidateAccountAlias(alias); err != nil {
			summary.AddError(fmt.Errorf("%s[%d].accountAlias: %w", field, index, err))
		}
	}

	amount, ok := item["amount"].(map[string]any)
	if !ok {
		return
	}

	validateSendAmount(summary, field, index, amount)
}

func validateSendAmount(summary *Summary, field string, index int, amount map[string]any) {
	if asset, ok := amount["asset"].(string); ok && asset != "" {
		if err := ValidateAssetCode(asset); err != nil {
			summary.AddError(fmt.Errorf("%s[%d].amount.asset: %w", field, index, err))
		}
	}

	if value, ok := extractNumericAmount(amount["value"]); !ok || !isFinitePositive(value) {
		summary.AddError(fmt.Errorf("%s[%d].amount.value must be a finite number greater than zero", field, index))
	}
}

func extractObjectList(value any) ([]map[string]any, bool) {
	switch items := value.(type) {
	case []map[string]any:
		return items, true
	case []any:
		result := make([]map[string]any, 0, len(items))
		for _, item := range items {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}

			result = append(result, m)
		}

		return result, true
	default:
		return nil, false
	}
}

// validateBasicTransactionFields validates the basic required fields of a transaction
func validateBasicTransactionFields(summary *Summary, input map[string]any) {
	validateAssetCodeField(summary, input)
	validateAmountField(summary, input)
	validateScaleField(summary, input)
}

// validateAssetCodeField validates the asset_code field of a transaction.
func validateAssetCodeField(summary *Summary, input map[string]any) {
	if input["asset_code"] == nil {
		summary.AddError(errors.New("asset code is required"))
		return
	}

	assetCode, ok := input["asset_code"].(string)
	if !ok {
		summary.AddError(errors.New("asset_code must be a string"))
		return
	}

	if err := ValidateAssetCode(assetCode); err != nil {
		summary.AddError(err)
	}
}

// validateAmountField validates the amount field of a transaction.
func validateAmountField(summary *Summary, input map[string]any) {
	if input["amount"] == nil {
		summary.AddError(errors.New("amount is required"))
		return
	}

	amount, ok := extractNumericAmount(input["amount"])
	if !ok {
		summary.AddError(errors.New("amount must be a finite number"))
		return
	}

	if !isFinitePositive(amount) {
		summary.AddError(fmt.Errorf("amount must be greater than zero (got %.2f)", amount))
	}
}

// extractNumericAmount extracts a float64 amount from various numeric types.
func extractNumericAmount(value any) (float64, bool) {
	if amount, ok := value.(float64); ok {
		if math.IsNaN(amount) || math.IsInf(amount, 0) {
			return 0, false
		}

		return amount, true
	}

	if intAmount, ok := value.(int); ok {
		return float64(intAmount), true
	}

	if intAmount, ok := value.(int64); ok {
		return float64(intAmount), true
	}

	if stringAmount, ok := value.(string); ok {
		amount, err := strconv.ParseFloat(strings.TrimSpace(stringAmount), 64)
		if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) {
			return 0, false
		}

		return amount, true
	}

	return 0, false
}

// validateScaleField validates the scale field of a transaction.
func validateScaleField(summary *Summary, input map[string]any) {
	if input["scale"] == nil {
		summary.AddError(errors.New("scale is required"))
		return
	}

	scale, ok := extractIntegerScale(input["scale"])
	if !ok {
		summary.AddError(errors.New("scale must be an integer"))
		return
	}

	if scale < 0 || scale > 18 {
		summary.AddError(errors.New("scale must be between 0 and 18"))
	}
}

// extractIntegerScale extracts an int scale from various numeric types.
func extractIntegerScale(value any) (int, bool) {
	if scale, ok := value.(int); ok {
		return scale, true
	}

	if floatScale, ok := value.(float64); ok {
		if math.IsNaN(floatScale) || math.IsInf(floatScale, 0) || math.Trunc(floatScale) != floatScale {
			return 0, false
		}

		return int(floatScale), true
	}

	return 0, false
}

// extractOperationAmount safely extracts an amount value from an operation.
// Returns the amount and a boolean indicating success.
func extractOperationAmount(op map[string]any) (float64, bool) {
	if op["amount"] == nil {
		return 0, false
	}

	amount, ok := op["amount"].(float64)
	if ok {
		if math.IsNaN(amount) || math.IsInf(amount, 0) {
			return 0, false
		}

		return amount, true
	}

	if intAmount, intOk := op["amount"].(int); intOk {
		return float64(intAmount), true
	}

	return 0, false
}

// errNonIntegerOperationAmount is a sentinel returned by
// accumulateOperationTotals when an operation amount cannot be coerced to a
// whole int64. Without this sentinel, a non-integer amount silently dropped
// out of the totals accumulator, which then surfaced downstream as a
// misleading "operations total (0) != transaction amount (X.00)" error —
// hiding the real problem (the non-integer amount itself).
var errNonIntegerOperationAmount = errors.New("operation amount must be an integer (got a non-integer value)")

// accumulateOperationTotals accumulates debit and credit totals from an
// operation. Returns errNonIntegerOperationAmount when the amount has a
// non-zero fractional part so the caller can emit a focused error instead
// of a generic totals-mismatch.
func accumulateOperationTotals(op map[string]any, totalDebits, totalCredits *int64) error {
	if op["type"] == nil {
		return nil
	}

	opType, typeOk := op["type"].(string)
	amount, amountOk := extractOperationAmount(op)

	if !typeOk || !amountOk {
		return nil
	}

	if math.Trunc(amount) != amount {
		return errNonIntegerOperationAmount
	}

	switch opType {
	case OpTypeDebit:
		*totalDebits += int64(amount)
	case OpTypeCredit:
		*totalCredits += int64(amount)
	}

	return nil
}

// validateTransactionOperations validates the operations in a transaction
func validateTransactionOperations(summary *Summary, input map[string]any) {
	if input["operations"] == nil {
		summary.AddError(errors.New("at least one operation is required"))
		return
	}

	operations, ok := input["operations"].([]map[string]any)
	if !ok {
		summary.AddError(errors.New("operations must be an array of objects"))
		return
	}

	assetCode := ""
	if ac, ok := input["asset_code"].(string); ok {
		assetCode = ac
	}

	var (
		totalDebits, totalCredits int64
		sawNonIntegerAmount       bool
	)

	for i, op := range operations {
		validationErrs, valid := validateOperation(op, i, assetCode)
		if !valid {
			for _, err := range validationErrs {
				summary.AddError(err)
			}
		}

		if err := accumulateOperationTotals(op, &totalDebits, &totalCredits); err != nil {
			// Surface the precise reason for the running totals being off
			// instead of letting it bubble up as a "totals mismatch" later.
			if errors.Is(err, errNonIntegerOperationAmount) {
				sawNonIntegerAmount = true
				summary.AddError(fmt.Errorf("operation %d: %w", i, err))
			}
		}
	}

	// If any operation amount wasn't an integer, the totals are not
	// authoritative and the downstream "operations total != transaction
	// amount" message is more confusing than helpful. Skip the totals
	// reconciliation in that case — the per-operation error already
	// pinpoints the problem.
	if !sawNonIntegerAmount {
		validateTransactionBalance(summary, input, totalDebits, totalCredits)
	}
}

// validateTransactionBalance validates that the transaction is balanced
func validateTransactionBalance(summary *Summary, input map[string]any, totalDebits, totalCredits int64) {
	validateDebitsCreditsBalance(summary, totalDebits, totalCredits)
	validateOperationTotalsMatchAmount(summary, input, totalDebits)
}

// validateDebitsCreditsBalance checks if debits and credits are equal.
func validateDebitsCreditsBalance(summary *Summary, totalDebits, totalCredits int64) {
	if totalDebits != totalCredits {
		summary.AddError(fmt.Errorf("transaction is unbalanced: total debits (%d) do not equal total credits (%d)",
			totalDebits, totalCredits))
	}
}

// validateOperationTotalsMatchAmount checks if operation totals match the transaction amount.
func validateOperationTotalsMatchAmount(summary *Summary, input map[string]any, totalDebits int64) {
	if input["amount"] == nil {
		return
	}

	amount, ok := extractNumericAmount(input["amount"])
	if !ok {
		return
	}

	if totalDebits != int64(amount) {
		summary.AddError(fmt.Errorf("operation amounts do not match transaction amount: operations total (%d) != transaction amount (%.2f)",
			totalDebits, amount))
	}
}

// validateAdditionalTransactionFields validates optional fields in the transaction
func validateAdditionalTransactionFields(summary *Summary, input map[string]any) {
	validateChartOfAccountsGroupNameField(summary, input)
	validateMetadataField(summary, input)
}

// validateChartOfAccountsGroupNameField validates the chart_of_accounts_group_name field if present.
func validateChartOfAccountsGroupNameField(summary *Summary, input map[string]any) {
	if input["chart_of_accounts_group_name"] == nil {
		return
	}

	groupName, ok := input["chart_of_accounts_group_name"].(string)
	if !ok {
		summary.AddError(errors.New("chart_of_accounts_group_name must be a string"))
		return
	}

	if groupName == "" {
		return
	}

	if err := validateChartOfAccountsGroupName(groupName); err != nil {
		summary.AddError(err)
	}
}

// validateMetadataField validates the metadata field if present.
func validateMetadataField(summary *Summary, input map[string]any) {
	if input["metadata"] == nil {
		return
	}

	metadata, ok := input["metadata"].(map[string]any)
	if !ok {
		summary.AddError(errors.New("metadata must be an object"))
		return
	}

	if err := ValidateMetadata(metadata); err != nil {
		summary.AddError(fmt.Errorf("invalid metadata: %w", err))
	}
}

// ValidateAssetType validates if the asset type is one of the supported types
// in the Midaz system.
func ValidateAssetType(assetType string) error {
	if assetType == "" {
		return errors.New("asset type is required")
	}

	// Use commons.ValidateType to ensure consistency with backend APIs
	// Note: commons.ValidateType expects lowercase types, so we convert to lowercase
	if err := midazutils.ValidateType(strings.ToLower(assetType)); err != nil {
		// Create a list of valid types for the error message
		validTypes := []string{"crypto", "currency", "commodity", "others"}

		return fmt.Errorf("invalid asset type: %s. Valid types are: %s",
			assetType, strings.Join(validTypes, ", "))
	}

	return nil
}

// ValidateAccountType validates if the account type is one of the supported types
// in the Midaz system.
func ValidateAccountType(accountType string) error {
	return core.ValidateAccountType(accountType)
}

// ValidateCurrencyCode checks if the currency code is valid according to ISO 4217.
func ValidateCurrencyCode(code string) error {
	if code == "" {
		return errors.New("currency code cannot be empty")
	}

	// Use commons.ValidateCurrency to ensure consistency with backend APIs
	if err := midazutils.ValidateCurrency(code); err != nil {
		return fmt.Errorf("invalid currency code: %s", code)
	}

	return nil
}

// ValidateCountryCode checks if the country code is valid according to ISO 3166-1 alpha-2.
func ValidateCountryCode(code string) error {
	if code == "" {
		return errors.New("country code cannot be empty")
	}

	// Use commons.ValidateCountryAddress to ensure consistency with backend APIs
	if err := midazutils.ValidateCountryAddress(code); err != nil {
		return fmt.Errorf("invalid country code: %s (must be a valid ISO 3166-1 alpha-2 code)", code)
	}

	return nil
}

// Address is a simplified address structure for validation purposes.
type Address struct {
	Line1   string
	Line2   *string
	ZipCode string
	City    string
	State   string
	Country string
}

// ValidateAddress validates an address structure using the default validator.
// This function validates for completeness and correctness.
func ValidateAddress(address *Address) error {
	return defaultValidator.ValidateAddress(address)
}

// ValidateAddress validates an address structure using this validator's configuration.
// This method validates for completeness and correctness.
func (v *Validator) ValidateAddress(address *Address) error {
	if v == nil || v.config == nil {
		return errors.New("validator cannot be nil")
	}

	if address == nil {
		return errors.New("address cannot be nil")
	}

	// Validate required fields
	if address.Line1 == "" {
		return errors.New("address line 1 is required")
	}

	if len(address.Line1) > v.config.MaxAddressLineLength {
		return fmt.Errorf("address line 1 exceeds maximum length of %d characters",
			v.config.MaxAddressLineLength)
	}

	// Validate optional line 2
	if address.Line2 != nil && len(*address.Line2) > v.config.MaxAddressLineLength {
		return fmt.Errorf("address line 2 exceeds maximum length of %d characters",
			v.config.MaxAddressLineLength)
	}

	// Validate zip code
	if address.ZipCode == "" {
		return errors.New("zip code is required")
	}

	if len(address.ZipCode) > v.config.MaxZipCodeLength {
		return fmt.Errorf("zip code exceeds maximum length of %d characters",
			v.config.MaxZipCodeLength)
	}

	// Validate city
	if address.City == "" {
		return errors.New("city is required")
	}

	if len(address.City) > v.config.MaxCityLength {
		return fmt.Errorf("city exceeds maximum length of %d characters",
			v.config.MaxCityLength)
	}

	// Validate state
	if address.State == "" {
		return errors.New("state is required")
	}

	if len(address.State) > v.config.MaxStateLength {
		return fmt.Errorf("state exceeds maximum length of %d characters",
			v.config.MaxStateLength)
	}

	// Validate country
	if address.Country == "" {
		return errors.New("country is required")
	}

	return ValidateCountryCode(address.Country)
}

// where the final return value is intentionally discarded. The error is already
// added to the FieldErrors collection via Add(), and the chained methods modify
// the same pointer in-place. Discarding the return value is safe and intentional.
//
//nolint:errcheck // This file uses fluent API pattern (Add().WithConstraint().WithSuggestions())
package validation

import (
	"fmt"
	"strings"
	"time"
)

// EnhancedValidateAssetCode checks if an asset code is valid and returns field-level errors
// with suggestions when invalid.
func EnhancedValidateAssetCode(assetCode string) *FieldError {
	if assetCode == "" {
		return BuildFieldError("assetCode", assetCode, "Asset code is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("assetCode", assetCode, Required)...)
	}

	if !assetCodePattern.MatchString(assetCode) {
		return BuildFieldError("assetCode", assetCode, "Invalid asset code format").
			WithConstraint("format").
			WithSuggestions(GetCommonSuggestions("assetCode", assetCode, Format)...)
	}

	return nil
}

// EnhancedValidateAccountAlias checks if an account alias is valid and returns field-level errors
// with suggestions when invalid.
func EnhancedValidateAccountAlias(alias string) *FieldError {
	if alias == "" {
		return BuildFieldError("alias", alias, "Account alias cannot be empty").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("alias", alias, Required)...)
	}

	if !accountAliasPattern.MatchString(alias) {
		return BuildFieldError("alias", alias, "Invalid account alias format").
			WithConstraint("format").
			WithSuggestions(GetCommonSuggestions("alias", alias, Format)...)
	}

	return nil
}

// EnhancedValidateAssetType checks if an asset type is valid and returns field-level errors
// with suggestions when invalid.
func EnhancedValidateAssetType(assetType string) *FieldError {
	if assetType == "" {
		return BuildFieldError("assetType", assetType, "Asset type is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("assetType", assetType, Required)...)
	}

	// Use the existing implementation to check validity
	if err := ValidateAssetType(assetType); err != nil {
		return BuildFieldError("assetType", assetType, err.Error()).
			WithConstraint("enumeration").
			WithSuggestions(GetCommonSuggestions("assetType", assetType, Enumeration)...)
	}

	return nil
}

// EnhancedValidateAccountType checks if an account type is valid and returns field-level errors
// with suggestions when invalid.
func EnhancedValidateAccountType(accountType string) *FieldError {
	if accountType == "" {
		return BuildFieldError("accountType", accountType, "Account type is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("accountType", accountType, Required)...)
	}

	// Use the existing implementation to check validity
	if err := ValidateAccountType(accountType); err != nil {
		return BuildFieldError("accountType", accountType, err.Error()).
			WithConstraint("enumeration").
			WithSuggestions(GetCommonSuggestions("accountType", accountType, Enumeration)...)
	}

	return nil
}

// EnhancedValidateAmount validates a transaction amount and returns field-level errors
// with suggestions when invalid.
func EnhancedValidateAmount(amount, scale int64) *FieldError {
	if amount <= 0 {
		return BuildFieldError("amount", amount, "Amount must be greater than zero").
			WithConstraint("min").
			WithSuggestions(GetCommonSuggestions("amount", amount, Range)...)
	}

	if scale < 0 || scale > 18 {
		return BuildFieldError("scale", scale, "Scale must be between 0 and 18").
			WithConstraint("range").
			WithSuggestions(GetCommonSuggestions("scale", scale, Range)...)
	}

	return nil
}

// EnhancedValidateDateRange checks if a date range is valid and returns field-level errors
// with suggestions when invalid.
func EnhancedValidateDateRange(start, end time.Time, startField, endField string) *FieldErrors {
	errors := NewFieldErrors()

	// Check if either date is zero
	if start.IsZero() {
		errors.Add(startField, start, "Start date cannot be empty").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions(startField, start, Required)...)
	}

	if end.IsZero() {
		errors.Add(endField, end, "End date cannot be empty").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions(endField, end, Required)...)
	}

	// Don't check range if either date is invalid
	if start.IsZero() || end.IsZero() {
		return errors
	}

	// Check if start date is after end date
	if start.After(end) {
		errors.Add("dateRange", nil, fmt.Sprintf("Start date (%s) cannot be after end date (%s)",
			start.Format("2006-01-02"), end.Format("2006-01-02"))).
			WithConstraint("consistency").
			WithSuggestions(GetCommonSuggestions("dateRange", nil, Consistency)...)
	}

	return errors
}

// EnhancedValidateMetadata checks if metadata is valid and returns field-level errors
// with suggestions when invalid.
func EnhancedValidateMetadata(metadata map[string]any) *FieldErrors {
	errors := NewFieldErrors()

	if metadata == nil {
		return errors
	}

	for key, value := range metadata {
		validateMetadataEntry(errors, key, value)
	}

	validateTotalMetadataSize(errors, metadata)

	return errors
}

func validateMetadataEntry(errors *FieldErrors, key string, value any) {
	if !validateMetadataKey(errors, key) {
		return
	}

	if !validateMetadataValueType(errors, key, value) {
		return
	}

	validateMetadataStringLength(errors, key, value)
	validateMetadataNumericRange(errors, key, value)
}

func validateMetadataKey(errors *FieldErrors, key string) bool {
	if key == "" {
		errors.Add("metadata.key", key, "Metadata key cannot be empty").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("metadata.key", key, Required)...)

		return false
	}

	if len(key) > 64 {
		errors.Add(fmt.Sprintf("metadata.%s", key), key,
			fmt.Sprintf("Metadata key exceeds maximum length of 64 characters (length: %d)", len(key))).
			WithConstraint("maxLength").
			WithSuggestions(GetCommonSuggestions("metadata.key", key, Range)...)
	}

	return true
}

func validateMetadataValueType(errors *FieldErrors, key string, value any) bool {
	if !isValidMetadataValueType(value) {
		errors.Add(fmt.Sprintf("metadata.%s", key), value,
			fmt.Sprintf("Metadata value has unsupported type: %T", value)).
			WithConstraint("type").
			WithSuggestions(GetCommonSuggestions("metadata.value", value, Format)...)

		return false
	}

	return true
}

func validateMetadataStringLength(errors *FieldErrors, key string, value any) {
	strValue, ok := value.(string)
	if !ok || len(strValue) <= 256 {
		return
	}

	errors.Add(fmt.Sprintf("metadata.%s", key), strValue,
		fmt.Sprintf("Metadata string value exceeds maximum length of 256 characters (length: %d)", len(strValue))).
		WithConstraint("maxLength").
		WithSuggestions(GetCommonSuggestions("metadata.value", strValue, Range)...)
}

func validateMetadataNumericRange(errors *FieldErrors, key string, value any) {
	const (
		maxIntValue   = 9999999999
		maxFloatValue = 9999999999.0
	)

	switch v := value.(type) {
	case int:
		if v < -maxIntValue || v > maxIntValue {
			errors.Add(fmt.Sprintf("metadata.%s", key), v,
				"Integer value is outside allowed range (-9999999999 to 9999999999)").
				WithConstraint("range").
				WithSuggestions(GetCommonSuggestions("metadata.value", v, Range)...)
		}
	case float64:
		if v < -maxFloatValue || v > maxFloatValue {
			errors.Add(fmt.Sprintf("metadata.%s", key), v,
				"Float value is outside allowed range (-9999999999.0 to 9999999999.0)").
				WithConstraint("range").
				WithSuggestions(GetCommonSuggestions("metadata.value", v, Range)...)
		}
	}
}

func validateTotalMetadataSize(errors *FieldErrors, metadata map[string]any) {
	if err := validateMetadataSize(metadata); err != nil {
		errors.Add("metadata", metadata, "Total metadata size exceeds maximum allowed size of 4KB").
			WithConstraint("maxSize").
			WithSuggestions(GetCommonSuggestions("metadata", metadata, Range)...)
	}
}

// EnhancedValidateAddress validates an address structure and returns field-level errors
// with suggestions when invalid.
func EnhancedValidateAddress(address *Address, fieldPrefix string) *FieldErrors {
	errors := NewFieldErrors()

	if address == nil {
		errors.Add(fieldPrefix, nil, "Address cannot be nil").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions(fieldPrefix, nil, Required)...)

		return errors
	}

	// Validate required fields
	if address.Line1 == "" {
		errors.Add(fmt.Sprintf("%s.line1", fieldPrefix), address.Line1, "Address line 1 is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("address.line1", address.Line1, Required)...)
	} else if len(address.Line1) > 256 {
		errors.Add(fmt.Sprintf("%s.line1", fieldPrefix), address.Line1,
			fmt.Sprintf("Address line 1 exceeds maximum length of 256 characters (length: %d)", len(address.Line1))).
			WithConstraint("maxLength").
			WithSuggestions(GetCommonSuggestions("address.line1", address.Line1, Range)...)
	}

	// Validate optional line 2
	if address.Line2 != nil && len(*address.Line2) > 256 {
		errors.Add(fmt.Sprintf("%s.line2", fieldPrefix), *address.Line2,
			fmt.Sprintf("Address line 2 exceeds maximum length of 256 characters (length: %d)", len(*address.Line2))).
			WithConstraint("maxLength").
			WithSuggestions(GetCommonSuggestions("address.line2", *address.Line2, Range)...)
	}

	// Validate zip code
	if address.ZipCode == "" {
		errors.Add(fmt.Sprintf("%s.zipCode", fieldPrefix), address.ZipCode, "Zip code is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("address.zipCode", address.ZipCode, Required)...)
	} else if len(address.ZipCode) > 20 {
		errors.Add(fmt.Sprintf("%s.zipCode", fieldPrefix), address.ZipCode,
			fmt.Sprintf("Zip code exceeds maximum length of 20 characters (length: %d)", len(address.ZipCode))).
			WithConstraint("maxLength").
			WithSuggestions(GetCommonSuggestions("address.zipCode", address.ZipCode, Range)...)
	}

	// Validate city
	if address.City == "" {
		errors.Add(fmt.Sprintf("%s.city", fieldPrefix), address.City, "City is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("address.city", address.City, Required)...)
	} else if len(address.City) > 100 {
		errors.Add(fmt.Sprintf("%s.city", fieldPrefix), address.City,
			fmt.Sprintf("City exceeds maximum length of 100 characters (length: %d)", len(address.City))).
			WithConstraint("maxLength").
			WithSuggestions(GetCommonSuggestions("address.city", address.City, Range)...)
	}

	// Validate state
	if address.State == "" {
		errors.Add(fmt.Sprintf("%s.state", fieldPrefix), address.State, "State is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("address.state", address.State, Required)...)
	} else if len(address.State) > 100 {
		errors.Add(fmt.Sprintf("%s.state", fieldPrefix), address.State,
			fmt.Sprintf("State exceeds maximum length of 100 characters (length: %d)", len(address.State))).
			WithConstraint("maxLength").
			WithSuggestions(GetCommonSuggestions("address.state", address.State, Range)...)
	}

	// Validate country
	if address.Country == "" {
		errors.Add(fmt.Sprintf("%s.country", fieldPrefix), address.Country, "Country is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("address.country", address.Country, Required)...)
	} else if err := ValidateCountryCode(address.Country); err != nil {
		errors.Add(fmt.Sprintf("%s.country", fieldPrefix), address.Country, err.Error()).
			WithConstraint("format").
			WithSuggestions(GetCommonSuggestions("address.country", address.Country, Format)...)
	}

	return errors
}

// EnhancedValidateExternalAccount checks if an external account reference is valid
// with enhanced error information
func EnhancedValidateExternalAccount(account string) *FieldError {
	if !strings.HasPrefix(account, "@external/") {
		return BuildFieldError("externalAccount", account, "Invalid external account format").
			WithConstraint("format").
			WithSuggestions(
				"Use format '@external/XXX' where XXX is a valid asset code",
				"Asset code must be 3-4 uppercase letters",
				"Example: '@external/USD'",
			)
	}

	matches := externalAccountPattern.FindStringSubmatch(account)
	if len(matches) == 0 {
		return BuildFieldError("externalAccount", account, "Invalid external account format").
			WithConstraint("format").
			WithSuggestions(
				"Use format '@external/XXX' where XXX is a valid asset code",
				"Asset code must be 3-4 uppercase letters",
				"Example: '@external/USD'",
			)
	}

	externalAsset := matches[1]
	// Validate the external asset code format
	if !assetCodePattern.MatchString(externalAsset) {
		return BuildFieldError("externalAccount.assetCode", externalAsset, "Invalid asset code in external account").
			WithConstraint("format").
			WithSuggestions(GetCommonSuggestions("assetCode", externalAsset, Format)...)
	}

	return nil
}

// EnhancedValidateExternalAccountWithTransactionAsset validates an external account with transaction asset
func EnhancedValidateExternalAccountWithTransactionAsset(account string, transactionAsset string) *FieldError {
	// First do basic validation
	if err := EnhancedValidateExternalAccount(account); err != nil {
		return err
	}

	// Extract the asset code and check consistency
	matches := externalAccountPattern.FindStringSubmatch(account)
	externalAsset := matches[1]

	if externalAsset != transactionAsset {
		return BuildFieldError("externalAccount.consistency", []string{externalAsset, transactionAsset},
			"External account asset must match transaction asset").
			WithConstraint("consistency").
			WithSuggestions(
				"Use the same asset code for external accounts as the transaction",
				"Current transaction asset: "+transactionAsset,
				"External account asset: "+externalAsset,
			)
	}

	return nil
}

// EnhancedValidateAccountReference checks if an account reference is valid
// with enhanced error information, for both internal and external accounts
func EnhancedValidateAccountReference(account string, transactionAsset string) *FieldError {
	if account == "" {
		return BuildFieldError("account", account, "Account reference cannot be empty").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("account", account, Required)...)
	}

	// Check if it's an external account reference
	if strings.HasPrefix(account, "@external/") {
		return EnhancedValidateExternalAccountWithTransactionAsset(account, transactionAsset)
	}

	// For internal accounts, we could add additional validation here
	// For now, we just ensure it's not empty
	return nil
}

// EnhancedValidateTransactionCode checks if a transaction code is valid and returns field-level errors
// with suggestions when invalid.
func EnhancedValidateTransactionCode(code string) *FieldError {
	if code == "" {
		return BuildFieldError("transactionCode", code, "Transaction code is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("transactionCode", code, Required)...)
	}

	if !accountAliasPattern.MatchString(code) {
		return BuildFieldError("transactionCode", code, "Invalid transaction code format").
			WithConstraint("format").
			WithSuggestions(GetCommonSuggestions("transactionCode", code, Format)...)
	}

	return nil
}

// EnhancedValidateCurrencyCode validates a currency code with enhanced error information
func EnhancedValidateCurrencyCode(code string) *FieldError {
	if code == "" {
		return BuildFieldError("currencyCode", code, "Currency code is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("currencyCode", code, Required)...)
	}

	if err := ValidateCurrencyCode(code); err != nil {
		return BuildFieldError("currencyCode", code, "Invalid currency code").
			WithConstraint("format").
			WithSuggestions(
				"Use a valid ISO 4217 currency code (e.g., 'USD', 'EUR', 'JPY')",
				"Currency codes must be uppercase three-letter codes",
				"Check for typos in the currency code",
			)
	}

	return nil
}

// EnhancedValidateCountryCode validates a country code with enhanced error information
func EnhancedValidateCountryCode(code string) *FieldError {
	if code == "" {
		return BuildFieldError("countryCode", code, "Country code is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("countryCode", code, Required)...)
	}

	if err := ValidateCountryCode(code); err != nil {
		return BuildFieldError("countryCode", code, "Invalid country code").
			WithConstraint("format").
			WithSuggestions(
				"Use a valid ISO 3166-1 alpha-2 country code (e.g., 'US', 'GB', 'JP')",
				"Country codes must be uppercase two-letter codes",
				"Check for typos in the country code",
			)
	}

	return nil
}

// EnhancedValidateTransactionInput validates a transaction input and returns field-level errors
// with suggestions when invalid.
func EnhancedValidateTransactionInput(input map[string]any) *FieldErrors {
	validator := &transactionInputValidator{
		input:  input,
		errors: NewFieldErrors(),
	}

	return validator.validate()
}

// transactionInputValidator handles validation of transaction input.
type transactionInputValidator struct {
	input  map[string]any
	errors *FieldErrors
}

// validate performs comprehensive validation of transaction input.
func (v *transactionInputValidator) validate() *FieldErrors {
	if v.input == nil {
		v.addNilInputError()
		return v.errors
	}

	v.validateAssetCode()
	v.validateAmount()
	v.validateScale()
	v.validateOperations()
	v.validateMetadata()
	v.validateTransactionCode()
	v.validateChartOfAccountsGroupName()

	return v.errors
}

// addNilInputError adds an error for nil transaction input.
func (v *transactionInputValidator) addNilInputError() {
	v.errors.Add("transaction", nil, "Transaction input cannot be nil").
		WithConstraint("required").
		WithSuggestions(GetCommonSuggestions("transaction", nil, Required)...)
}

// validateAssetCode validates the asset code field.
func (v *transactionInputValidator) validateAssetCode() {
	if v.input["asset_code"] == nil {
		v.errors.Add("assetCode", nil, "Asset code is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("assetCode", nil, Required)...)

		return
	}

	assetCode, ok := v.input["asset_code"].(string)
	if !ok {
		v.errors.Add("assetCode", v.input["asset_code"], "Asset code must be a string").
			WithConstraint("type").
			WithSuggestions(GetCommonSuggestions("assetCode", v.input["asset_code"], Format)...)

		return
	}

	if err := EnhancedValidateAssetCode(assetCode); err != nil {
		v.errors.AddError(err)
	}
}

// validateAmount validates the amount field.
func (v *transactionInputValidator) validateAmount() {
	if v.input["amount"] == nil {
		v.errors.Add("amount", nil, "Amount is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("amount", nil, Required)...)

		return
	}

	amount, ok := v.input["amount"].(float64)
	if !ok {
		v.errors.Add("amount", v.input["amount"], "Amount must be a number").
			WithConstraint("type").
			WithSuggestions(GetCommonSuggestions("amount", v.input["amount"], Format)...)

		return
	}

	if amount <= 0 {
		v.errors.Add("amount", amount, "Amount must be greater than zero").
			WithConstraint("min").
			WithSuggestions(GetCommonSuggestions("amount", amount, Range)...)
	}
}

// validateScale validates the scale field.
func (v *transactionInputValidator) validateScale() {
	if v.input["scale"] == nil {
		v.errors.Add("scale", nil, "Scale is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("scale", nil, Required)...)

		return
	}

	scale, ok := v.input["scale"].(int)
	if !ok {
		v.errors.Add("scale", v.input["scale"], "Scale must be an integer").
			WithConstraint("type").
			WithSuggestions(GetCommonSuggestions("scale", v.input["scale"], Format)...)

		return
	}

	if scale < 0 || scale > 18 {
		v.errors.Add("scale", scale, "Scale must be between 0 and 18").
			WithConstraint("range").
			WithSuggestions(GetCommonSuggestions("scale", scale, Range)...)
	}
}

// validateOperations validates the operations field.
func (v *transactionInputValidator) validateOperations() {
	if v.input["operations"] == nil {
		v.addMissingOperationsError()
		return
	}

	operations, ok := v.input["operations"].([]map[string]any)
	if !ok || len(operations) == 0 {
		v.addMissingOperationsError()
		return
	}

	validateTransactionOperationsEnhanced(v.errors, v.input)
}

// addMissingOperationsError adds an error for missing or invalid operations.
func (v *transactionInputValidator) addMissingOperationsError() {
	v.errors.Add("operations", nil, "At least one operation is required").
		WithConstraint("required").
		WithSuggestions(GetCommonSuggestions("operations", nil, Required)...)
}

// validateMetadata validates the metadata field if present.
func (v *transactionInputValidator) validateMetadata() {
	if v.input["metadata"] == nil {
		return
	}

	metadata, ok := v.input["metadata"].(map[string]any)
	if !ok {
		v.errors.Add("metadata", v.input["metadata"], "Metadata must be an object").
			WithConstraint("type").
			WithSuggestions(GetCommonSuggestions("metadata", v.input["metadata"], Format)...)

		return
	}

	metadataErrors := EnhancedValidateMetadata(metadata)
	for _, err := range metadataErrors.Errors {
		v.errors.AddError(err)
	}
}

// validateTransactionCode validates the transaction code field if present.
func (v *transactionInputValidator) validateTransactionCode() {
	if v.input["transaction_code"] == nil {
		return
	}

	txCode, ok := v.input["transaction_code"].(string)
	if !ok {
		v.errors.Add("transactionCode", v.input["transaction_code"], "Transaction code must be a string").
			WithConstraint("type").
			WithSuggestions(GetCommonSuggestions("transactionCode", v.input["transaction_code"], Format)...)

		return
	}

	if err := EnhancedValidateTransactionCode(txCode); err != nil {
		v.errors.AddError(err)
	}
}

// validateChartOfAccountsGroupName validates the chart of accounts group name if present.
func (v *transactionInputValidator) validateChartOfAccountsGroupName() {
	if v.input["chart_of_accounts_group_name"] == nil {
		return
	}

	groupName, ok := v.input["chart_of_accounts_group_name"].(string)
	if !ok || groupName == "" {
		return
	}

	if err := validateChartOfAccountsGroupName(groupName); err != nil {
		v.errors.Add("chartOfAccountsGroupName", groupName, err.Error()).
			WithConstraint("format").
			WithSuggestions(
				"Use alphanumeric characters, spaces, underscores, and hyphens",
				"Keep the name under 100 characters",
				"Example: 'Standard Chart' or 'GAAP_2023'",
			)
	}
}

// validateTransactionOperationsEnhanced validates the operations in a transaction
// and adds field-level errors with suggestions.
func validateTransactionOperationsEnhanced(errors *FieldErrors, input map[string]any) {
	operations := input["operations"].([]map[string]any)

	validator := &operationValidator{
		errors:    errors,
		input:     input,
		assetCode: getAssetCodeFromInput(input),
	}

	for i, op := range operations {
		validator.validateSingleOperation(op, i)
	}

	// Validate transaction structure
	validateTransactionStructureEnhanced(errors, validator.debitCount, validator.creditCount, validator.totalDebits, validator.totalCredits, input)
}

// operationValidator holds state for validating transaction operations.
type operationValidator struct {
	errors                    *FieldErrors
	input                     map[string]any
	assetCode                 string
	totalDebits, totalCredits int64
	debitCount, creditCount   int
}

// getAssetCodeFromInput safely extracts asset code from input.
func getAssetCodeFromInput(input map[string]any) string {
	if assetCode, ok := input["asset_code"].(string); ok {
		return assetCode
	}

	return ""
}

// validateSingleOperation validates a single transaction operation.
func (v *operationValidator) validateSingleOperation(op map[string]any, index int) {
	field := fmt.Sprintf("operations[%d]", index)

	v.validateOperationType(op, field)
	v.validateAccountID(op, field)
	v.validateAccountAlias(op, field)
	v.validateAmount(op, field)
	v.validateAssetCode(op, field)
	v.validateMetadata(op, field)
}

// validateOperationType validates the operation type field.
func (v *operationValidator) validateOperationType(op map[string]any, field string) {
	if op["type"] == nil {
		v.errors.Add(fmt.Sprintf("%s.type", field), nil, "Operation type is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("operation.type", nil, Required)...)

		return
	}

	opType, ok := op["type"].(string)
	if !ok {
		v.errors.Add(fmt.Sprintf("%s.type", field), op["type"], "Operation type must be a string").
			WithConstraint("type").
			WithSuggestions(GetCommonSuggestions("operation.type", op["type"], Format)...)

		return
	}

	if opType != OpTypeDebit && opType != OpTypeCredit {
		v.errors.Add(fmt.Sprintf("%s.type", field), opType, "Invalid operation type").
			WithConstraint("enumeration").
			WithCode("invalid_operation_type").
			WithSuggestions(GetCommonSuggestions("operation.type", opType, Enumeration)...)

		return
	}

	v.trackOperationTotals(op, opType)
}

// trackOperationTotals updates the debit/credit totals for balance validation.
func (v *operationValidator) trackOperationTotals(op map[string]any, opType string) {
	amount, ok := op["amount"].(float64)
	if !ok {
		return
	}

	switch opType {
	case OpTypeDebit:
		v.debitCount++
		v.totalDebits += int64(amount)
	case OpTypeCredit:
		v.creditCount++
		v.totalCredits += int64(amount)
	}
}

// validateAccountID validates the account ID field.
func (v *operationValidator) validateAccountID(op map[string]any, field string) {
	if op["account_id"] == nil {
		v.errors.Add(fmt.Sprintf("%s.accountId", field), nil, "Account ID is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("account.id", nil, Required)...)

		return
	}

	accountID, ok := op["account_id"].(string)
	if !ok {
		return
	}

	if strings.HasPrefix(accountID, "@external/") && v.assetCode != "" {
		if err := EnhancedValidateExternalAccountWithTransactionAsset(accountID, v.assetCode); err != nil {
			err.Field = fmt.Sprintf("%s.accountId", field)
			v.errors.AddError(err)
		}
	}
}

// validateAccountAlias validates the account alias field if provided.
func (v *operationValidator) validateAccountAlias(op map[string]any, field string) {
	if op["account_alias"] == nil {
		return
	}

	alias, ok := op["account_alias"].(string)
	if !ok || alias == "" {
		return
	}

	if err := EnhancedValidateAccountAlias(alias); err != nil {
		err.Field = fmt.Sprintf("%s.accountAlias", field)
		v.errors.AddError(err)
	}
}

// validateAmount validates the operation amount field.
func (v *operationValidator) validateAmount(op map[string]any, field string) {
	if op["amount"] == nil {
		v.errors.Add(fmt.Sprintf("%s.amount", field), nil, "Operation amount is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("amount", nil, Required)...)

		return
	}

	amount, ok := op["amount"].(float64)
	if !ok {
		v.errors.Add(fmt.Sprintf("%s.amount", field), op["amount"], "Operation amount must be a number").
			WithConstraint("type").
			WithSuggestions(GetCommonSuggestions("amount", op["amount"], Format)...)

		return
	}

	if amount <= 0 {
		v.errors.Add(fmt.Sprintf("%s.amount", field), amount, "Operation amount must be greater than zero").
			WithConstraint("min").
			WithSuggestions(GetCommonSuggestions("amount", amount, Range)...)
	}
}

// validateAssetCode validates that operation asset code matches transaction asset code.
func (v *operationValidator) validateAssetCode(op map[string]any, field string) {
	if op["asset_code"] == nil {
		return
	}

	opAssetCode, ok := op["asset_code"].(string)
	if !ok || opAssetCode == "" || v.assetCode == "" {
		return
	}

	if opAssetCode != v.assetCode {
		v.errors.Add(fmt.Sprintf("%s.assetCode", field), opAssetCode,
			fmt.Sprintf("Operation asset code must match transaction asset code (expected: %s)", v.assetCode)).
			WithConstraint("consistency").
			WithSuggestions(GetCommonSuggestions("asset.code", opAssetCode, Consistency)...)
	}
}

// validateMetadata validates the operation metadata field if present.
func (v *operationValidator) validateMetadata(op map[string]any, field string) {
	if op["metadata"] == nil {
		return
	}

	metadata, ok := op["metadata"].(map[string]any)
	if !ok {
		v.errors.Add(fmt.Sprintf("%s.metadata", field), op["metadata"], "Operation metadata must be an object").
			WithConstraint("type").
			WithSuggestions(GetCommonSuggestions("metadata", op["metadata"], Format)...)

		return
	}

	metadataErrors := EnhancedValidateMetadata(metadata)
	for _, err := range metadataErrors.Errors {
		err.Field = fmt.Sprintf("%s.metadata.%s", field, err.Field)
		v.errors.AddError(err)
	}
}

// validateTransactionStructureEnhanced validates the overall transaction structure
// and adds field-level errors with suggestions.
func validateTransactionStructureEnhanced(errors *FieldErrors, debitCount, creditCount int, totalDebits, totalCredits int64, input map[string]any) {
	// Check if there are both debits and credits
	if debitCount == 0 {
		errors.Add("transaction.operations", nil, "Transaction must have at least one DEBIT operation").
			WithConstraint("required").
			WithSuggestions(
				"Add at least one DEBIT operation",
				"A balanced transaction requires both DEBIT and CREDIT operations",
			)
	}

	if creditCount == 0 {
		errors.Add("transaction.operations", nil, "Transaction must have at least one CREDIT operation").
			WithConstraint("required").
			WithSuggestions(
				"Add at least one CREDIT operation",
				"A balanced transaction requires both DEBIT and CREDIT operations",
			)
	}

	// Check if debits and credits balance
	if totalDebits != totalCredits {
		errors.Add("transaction.balance", nil,
			fmt.Sprintf("Transaction is unbalanced: total debits (%d) do not equal total credits (%d)",
				totalDebits, totalCredits)).
			WithConstraint("balance").
			WithSuggestions(
				"Adjust operation amounts to make debits equal credits",
				"Check for calculation errors in your amounts",
				"Verify all operations have the correct type (DEBIT/CREDIT)",
			)
	}

	// Check if total matches transaction amount
	if amount, ok := input["amount"].(float64); ok && totalDebits != int64(amount) {
		errors.Add("transaction.amount", nil,
			fmt.Sprintf("Operation amounts do not match transaction amount: operations total (%d) != transaction amount (%.2f)",
				totalDebits, amount)).
			WithConstraint("consistency").
			WithSuggestions(
				"Set the transaction amount equal to the total of all operations",
				fmt.Sprintf("Expected transaction amount: %d", totalDebits),
				"Or adjust operation amounts to match the transaction amount",
			)
	}
}

// EnhancedValidateTransactionDSL performs validation of transaction DSL input
// with enhanced error information.
func EnhancedValidateTransactionDSL(input TransactionDSLValidator) *FieldErrors {
	errors := NewFieldErrors()

	if input == nil {
		errors.Add("transaction", nil, "Transaction input cannot be nil").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("transaction", nil, Required)...)

		return errors
	}

	// Validate asset code
	asset := input.GetAsset()
	if asset == "" {
		errors.Add("asset", asset, "Asset code is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("asset", asset, Required)...)
	} else if !assetCodePattern.MatchString(asset) {
		errors.Add("asset", asset, "Invalid asset code format").
			WithConstraint("format").
			WithSuggestions(GetCommonSuggestions("asset", asset, Format)...)
	}

	// Validate amount
	value := input.GetValue()
	if value <= 0 {
		errors.Add("value", value, "Transaction amount must be greater than zero").
			WithConstraint("min").
			WithSuggestions(GetCommonSuggestions("amount", value, Range)...)
	}

	// Validate source accounts
	validateTransactionDSLSourceAccounts(errors, input)

	// Validate destination accounts
	validateTransactionDSLDestinationAccounts(errors, input)

	// Validate metadata if present
	metadata := input.GetMetadata()
	if metadata != nil {
		metadataErrors := EnhancedValidateMetadata(metadata)
		for _, err := range metadataErrors.Errors {
			errors.AddError(err)
		}
	}

	if !errors.HasErrors() {
		return nil
	}

	return errors
}

// validateTransactionDSLSourceAccounts validates source accounts in transaction DSL.
func validateTransactionDSLSourceAccounts(errors *FieldErrors, input TransactionDSLValidator) {
	sourceAccounts := input.GetSourceAccounts()
	if len(sourceAccounts) == 0 {
		errors.Add("sourceAccounts", nil, "At least one source account is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("sourceAccounts", nil, Required)...)

		return
	}

	asset := input.GetAsset()

	for i, account := range sourceAccounts {
		if account.GetAccount() == "" {
			errors.Add(fmt.Sprintf("sourceAccounts[%d]", i), account.GetAccount(), "Source account cannot be empty").
				WithConstraint("required").
				WithSuggestions(GetCommonSuggestions("account", account.GetAccount(), Required)...)

			continue
		}

		if err := EnhancedValidateAccountReference(account.GetAccount(), asset); err != nil {
			err.Field = fmt.Sprintf("sourceAccounts[%d]", i)
			errors.AddError(err)
		}
	}
}

// validateTransactionDSLDestinationAccounts validates destination accounts in transaction DSL.
func validateTransactionDSLDestinationAccounts(errors *FieldErrors, input TransactionDSLValidator) {
	destAccounts := input.GetDestinationAccounts()
	if len(destAccounts) == 0 {
		errors.Add("destinationAccounts", nil, "At least one destination account is required").
			WithConstraint("required").
			WithSuggestions(GetCommonSuggestions("destinationAccounts", nil, Required)...)

		return
	}

	asset := input.GetAsset()

	for i, account := range destAccounts {
		if account.GetAccount() == "" {
			errors.Add(fmt.Sprintf("destinationAccounts[%d]", i), account.GetAccount(), "Destination account cannot be empty").
				WithConstraint("required").
				WithSuggestions(GetCommonSuggestions("account", account.GetAccount(), Required)...)

			continue
		}

		if err := EnhancedValidateAccountReference(account.GetAccount(), asset); err != nil {
			err.Field = fmt.Sprintf("destinationAccounts[%d]", i)
			errors.AddError(err)
		}
	}
}

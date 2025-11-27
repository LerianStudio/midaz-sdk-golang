package validation

import (
	"fmt"
	"regexp"
	"strings"
)

// SuggestionType represents the type of suggestion to provide
type SuggestionType string

const (
	// Format issues
	Format SuggestionType = "format"

	// Required field issues
	Required SuggestionType = "required"

	// Range issues (min/max)
	Range SuggestionType = "range"

	// Enumeration issues (invalid value from a set)
	Enumeration SuggestionType = "enumeration"

	// Consistency issues (fields that must match)
	Consistency SuggestionType = "consistency"

	// Structural issues (missing or extra elements)
	Structure SuggestionType = "structure"
)

// GetCommonSuggestions returns suggestions for common validation issues
func GetCommonSuggestions(field string, value any, sugType SuggestionType) []string {
	switch sugType {
	case Format:
		return getFormatSuggestions(field, value)
	case Required:
		return getRequiredSuggestions(field)
	case Range:
		return getRangeSuggestions(field, value)
	case Enumeration:
		return getEnumerationSuggestions(field, value)
	case Consistency:
		return getConsistencySuggestions(field)
	case Structure:
		return getStructureSuggestions(field)
	default:
		return []string{fmt.Sprintf("Check the documentation for field '%s' requirements", field)}
	}
}

// formatSuggestionMatcher defines a pattern matcher and its corresponding suggestions
type formatSuggestionMatcher struct {
	pattern     string
	suggestions func(value any) []string
}

// getFormatSuggestions provides suggestions for format issues
func getFormatSuggestions(field string, value any) []string {
	fieldLower := strings.ToLower(field)

	matchers := buildFormatSuggestionMatchers(field, value)

	for _, matcher := range matchers {
		if match(fieldLower, matcher.pattern) {
			return matcher.suggestions(value)
		}
	}

	return getGenericFormatSuggestions(field)
}

func buildFormatSuggestionMatchers(_ string, _ any) []formatSuggestionMatcher {
	return []formatSuggestionMatcher{
		{pattern: "asset.*code|asset_code|currency|code", suggestions: getAssetCodeSuggestions},
		{pattern: "alias|account.*alias", suggestions: getAliasSuggestions},
		{pattern: "transaction.*code|tx.*code", suggestions: getTransactionCodeSuggestions},
		{pattern: "date|timestamp|time", suggestions: getDateFormatSuggestions},
		{pattern: "email", suggestions: getEmailSuggestions},
		{pattern: "id|uuid", suggestions: getUUIDSuggestions},
		{pattern: "metadata", suggestions: getMetadataSuggestions},
		{pattern: "chart.*accounts|coa", suggestions: getChartOfAccountsSuggestions},
		{pattern: "external.*account", suggestions: getExternalAccountSuggestions},
		{pattern: "country|country.*code", suggestions: getCountryCodeSuggestions},
	}
}

func getAssetCodeSuggestions(_ any) []string {
	return []string{
		"Use 3-4 uppercase letters for asset codes (e.g., 'USD', 'EUR', 'BTC')",
		"Check for incorrect letter case, asset codes must be all uppercase",
	}
}

func getAliasSuggestions(value any) []string {
	return []string{
		"Use alphanumeric characters with optional underscores or hyphens",
		"Ensure length is between 1-50 characters",
		fmt.Sprintf("Current value '%v' may contain invalid characters", value),
	}
}

func getTransactionCodeSuggestions(_ any) []string {
	return []string{
		"Use alphanumeric characters with optional underscores or hyphens",
		"Ensure length is between 1-100 characters",
		"Example: 'TX_123456' or 'payment-2023-01'",
	}
}

func getDateFormatSuggestions(_ any) []string {
	return []string{
		"Use ISO 8601 date format (YYYY-MM-DDTHH:MM:SSZ)",
		"Example: '2023-01-15T14:30:00Z'",
		"Ensure the date is not in the wrong format or timezone",
	}
}

func getEmailSuggestions(_ any) []string {
	return []string{
		"Use a valid email format (e.g., 'user@example.com')",
		"Check for missing @ symbol or domain part",
	}
}

func getUUIDSuggestions(_ any) []string {
	return []string{
		"Use a valid UUID v4 format",
		"Example: '123e4567-e89b-12d3-a456-426614174000'",
		"Ensure all hexadecimal characters and hyphens are present",
	}
}

func getMetadataSuggestions(_ any) []string {
	return []string{
		"Metadata keys must be 1-64 characters",
		"Metadata values must be strings, numbers, booleans, or nil",
		"String values must be 1-256 characters",
		"Total metadata size must be under 4KB",
	}
}

func getChartOfAccountsSuggestions(_ any) []string {
	return []string{
		"Use alphanumeric characters with optional spaces, underscores or hyphens",
		"Ensure length is between 1-100 characters",
		"Example: 'Standard Chart' or 'GAAP_2023'",
	}
}

func getExternalAccountSuggestions(_ any) []string {
	return []string{
		"Use format '@external/XXX' where XXX is the asset code",
		"Example: '@external/USD'",
		"Asset code must be 3-4 uppercase letters",
	}
}

func getCountryCodeSuggestions(_ any) []string {
	return []string{
		"Use ISO 3166-1 alpha-2 country codes (2 letters)",
		"Examples: 'US', 'GB', 'CA', 'JP'",
		"Country codes must be uppercase",
	}
}

func getGenericFormatSuggestions(field string) []string {
	return []string{
		fmt.Sprintf("Check the documentation for the correct format of '%s'", field),
		"Ensure there are no leading or trailing spaces",
		"Check for incorrect character case (uppercase/lowercase)",
	}
}

// getRequiredSuggestions provides suggestions for required field issues
func getRequiredSuggestions(field string) []string {
	return []string{
		fmt.Sprintf("Field '%s' is required and cannot be empty", field),
		"Check if the field is misspelled in your request",
		"Ensure the field is included in your request body",
	}
}

// getRangeSuggestions provides suggestions for range issues (min/max values)
func getRangeSuggestions(field string, value any) []string {
	// Convert field to lowercase for matching
	fieldLower := strings.ToLower(field)

	// Amount suggestions
	if match(fieldLower, "amount|value|sum|total") {
		return []string{
			"Amount must be greater than zero",
			"Check if the amount is using the correct scale",
			"Example: to represent $10.00, use 1000 with scale 2",
		}
	}

	// Scale suggestions
	if match(fieldLower, "scale|precision|decimals") {
		return []string{
			"Scale must be between 0 and 18",
			"Most currencies use scale 2 (e.g., cents)",
			"Some cryptocurrencies like Bitcoin use scale 8",
		}
	}

	// String length suggestions
	if match(fieldLower, "name|description|title") {
		return []string{
			fmt.Sprintf("Check the length of the '%s' field", field),
			"Most string fields have a maximum length between 50-256 characters",
			"Consider shortening the text",
		}
	}

	// Generic range suggestions
	return []string{
		fmt.Sprintf("The value '%v' for field '%s' is outside the acceptable range", value, field),
		"Check the documentation for the minimum and maximum values",
		"Ensure the value uses the correct units or scale",
	}
}

// getEnumerationSuggestions provides suggestions for enumeration issues (invalid value from a set)
func getEnumerationSuggestions(field string, value any) []string {
	// Convert field to lowercase for matching
	fieldLower := strings.ToLower(field)

	// Asset type suggestions
	if match(fieldLower, "asset.*type") {
		return []string{
			"Valid asset types are: 'crypto', 'currency', 'commodity', 'others'",
			fmt.Sprintf("The value '%v' is not a valid asset type", value),
			"Asset types are case-sensitive",
		}
	}

	// Account type suggestions
	if match(fieldLower, "account.*type") {
		return []string{
			"Valid account types are: 'deposit', 'savings', 'loans', 'marketplace', 'creditCard'",
			fmt.Sprintf("The value '%v' is not a valid account type", value),
			"Account types are case-sensitive",
		}
	}

	// Operation type suggestions
	if match(fieldLower, "operation.*type|op.*type") {
		return []string{
			"Valid operation types are: 'DEBIT', 'CREDIT'",
			fmt.Sprintf("The value '%v' is not a valid operation type", value),
			"Operation types must be uppercase",
		}
	}

	// Transaction status suggestions
	if match(fieldLower, "transaction.*status|tx.*status|status") {
		return []string{
			"Valid transaction statuses are: 'PENDING', 'COMPLETED', 'FAILED', 'CANCELED'",
			fmt.Sprintf("The value '%v' is not a valid transaction status", value),
			"Transaction statuses must be uppercase",
		}
	}

	// Generic enumeration suggestions
	return []string{
		fmt.Sprintf("The value '%v' for field '%s' is not one of the allowed values", value, field),
		"Check the documentation for the list of valid values",
		"Values may be case-sensitive",
	}
}

// getConsistencySuggestions provides suggestions for consistency issues
func getConsistencySuggestions(field string) []string {
	// Convert field to lowercase for matching
	fieldLower := strings.ToLower(field)

	// Asset code consistency
	if match(fieldLower, "asset.*code") {
		return []string{
			"Asset codes must be consistent throughout the transaction",
			"Check that external accounts use the same asset code as the transaction",
			"All source and destination accounts must use the same asset code",
		}
	}

	// Transaction balance consistency
	if match(fieldLower, "balance|total") {
		return []string{
			"Transaction debits and credits must balance",
			"The sum of all debit operations must equal the sum of all credit operations",
			"Check for rounding issues or missing operations",
		}
	}

	// Date range consistency
	if match(fieldLower, "date|time") {
		return []string{
			"Start date must be before or equal to end date",
			"Check the date format and timezone",
			"Ensure dates are in ISO 8601 format (YYYY-MM-DDTHH:MM:SSZ)",
		}
	}

	// Generic consistency suggestions
	return []string{
		fmt.Sprintf("Check for consistency issues with field '%s'", field),
		"Ensure related fields have consistent values",
		"Double-check calculations and totals",
	}
}

// getStructureSuggestions provides suggestions for structural issues
func getStructureSuggestions(field string) []string {
	// Convert field to lowercase for matching
	fieldLower := strings.ToLower(field)

	// Operations structure
	if match(fieldLower, "operations|send|distribute") {
		return []string{
			"At least one source and one destination account is required",
			"Each operation must have an account ID and amount",
			"Check the structure of the transaction operations",
		}
	}

	// Metadata structure
	if match(fieldLower, "metadata") {
		return []string{
			"Metadata must be a valid JSON object",
			"Metadata keys must be strings",
			"Metadata values must be strings, numbers, booleans, or null",
			"Nested metadata objects must follow the same rules",
		}
	}

	// Address structure
	if match(fieldLower, "address") {
		return []string{
			"Address must include line1, zipCode, city, state, and country",
			"Country must be a valid ISO 3166-1 alpha-2 code (e.g., 'US')",
			"Check for missing or invalid address fields",
		}
	}

	// Generic structure suggestions
	return []string{
		fmt.Sprintf("Check the structure of the '%s' field", field),
		"Ensure all required subfields are present",
		"Verify the format of complex objects",
	}
}

// match checks if the field name matches a regular expression pattern.
func match(field, pattern string) bool {
	matched, err := regexp.MatchString(pattern, field)
	if err != nil {
		// Invalid pattern - return false rather than panic
		return false
	}

	return matched
}

// GetExampleValue provides example values for common fields
func GetExampleValue(field string) string {
	// Convert field to lowercase for matching
	fieldLower := strings.ToLower(field)

	switch {
	// Asset codes
	case match(fieldLower, "asset.*code|currency|code"):
		return "USD"

	// Account aliases
	case match(fieldLower, "alias|account.*alias"):
		return "customer_savings"

	// Transaction codes
	case match(fieldLower, "transaction.*code|tx.*code"):
		return "TX_12345"

	// Amounts
	case match(fieldLower, "amount|value"):
		return "10000 (for $100.00 with scale 2)"

	// Asset types
	case match(fieldLower, "asset.*type"):
		return "currency"

	// Account types
	case match(fieldLower, "account.*type"):
		return "savings"

	// External accounts
	case match(fieldLower, "external.*account"):
		return "@external/USD"

	// Country codes
	case match(fieldLower, "country|country.*code"):
		return "US"

	// Default example
	default:
		return "See documentation for examples"
	}
}

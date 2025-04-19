// Package core provides fundamental validation utilities for the Midaz SDK.
package core

// TypeValidator defines the interface for validating asset types
type TypeValidator interface {
	// ValidateType validates if an asset type is valid
	ValidateType(assetType string) error
}

// AccountTypeValidator defines the interface for validating account types
type AccountTypeValidator interface {
	// ValidateAccountType validates if an account type is valid
	ValidateAccountType(accountType string) error
}

// CurrencyValidator defines the interface for validating currency codes
type CurrencyValidator interface {
	// ValidateCurrency validates if a currency code is valid
	ValidateCurrency(code string) error
}

// CountryValidator defines the interface for validating country codes
type CountryValidator interface {
	// ValidateCountryAddress validates if a country code is valid
	ValidateCountryAddress(code string) error
}

// ValidatorProvider defines a provider for all validators
type ValidatorProvider interface {
	TypeValidator
	AccountTypeValidator
	CurrencyValidator
	CountryValidator
}

// DefaultValidatorProvider returns a default implementation of ValidatorProvider.
// This function is used when no custom validator is specified.
func DefaultValidatorProvider() ValidatorProvider {
	return defaultValidator
}

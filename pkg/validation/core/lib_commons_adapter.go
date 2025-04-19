package core

import (
	"strings"

	"github.com/LerianStudio/lib-commons/commons"
)

// LibCommonsValidator is an implementation of ValidatorProvider that uses lib-commons
type LibCommonsValidator struct{}

// NewLibCommonsValidator creates a new validator that uses lib-commons
func NewLibCommonsValidator() *LibCommonsValidator {
	return &LibCommonsValidator{}
}

// ValidateType implements TypeValidator
func (v *LibCommonsValidator) ValidateType(assetType string) error {
	// lib-commons expects lowercase types
	return commons.ValidateType(strings.ToLower(assetType))
}

// ValidateAccountType implements AccountTypeValidator
func (v *LibCommonsValidator) ValidateAccountType(accountType string) error {
	return commons.ValidateAccountType(accountType)
}

// ValidateCurrency implements CurrencyValidator
func (v *LibCommonsValidator) ValidateCurrency(code string) error {
	return commons.ValidateCurrency(code)
}

// ValidateCountryAddress implements CountryValidator
func (v *LibCommonsValidator) ValidateCountryAddress(code string) error {
	return commons.ValidateCountryAddress(code)
}

// Default validator instance for use in the core package
var defaultValidator = NewLibCommonsValidator()

// Package testing provides testing utilities for the validation package.
package testing

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation/core"
)

// MockValidatorProvider is a mock implementation of ValidatorProvider for testing
type MockValidatorProvider struct {
	// Control for validation responses
	ShouldReturnError bool
	ErrorMessage      string

	// Valid values
	ValidAssetTypes    []string
	ValidAccountTypes  []string
	ValidCurrencyCodes []string
	ValidCountryCodes  []string
}

// NewMockValidator creates a new mock validator with default valid values
func NewMockValidator() *MockValidatorProvider {
	return &MockValidatorProvider{
		ShouldReturnError: false,
		ErrorMessage:      "validation error",
		// Include both asset types and currency codes in ValidAssetTypes for backward compatibility
		ValidAssetTypes:    []string{"crypto", "currency", "commodity", "others", "USD", "EUR", "GBP", "JPY"},
		ValidAccountTypes:  []string{"deposit", "savings", "loans", "marketplace", "creditCard"},
		ValidCurrencyCodes: []string{"USD", "EUR", "GBP", "JPY"},
		ValidCountryCodes:  []string{"US", "GB", "FR", "DE", "JP"},
	}
}

// ValidateType implements TypeValidator
func (m *MockValidatorProvider) ValidateType(assetType string) error {
	if m.ShouldReturnError {
		return fmt.Errorf("%s", m.ErrorMessage)
	}

	// Check against valid types (case-insensitive)
	lowerType := strings.ToLower(assetType)
	for _, valid := range m.ValidAssetTypes {
		if strings.ToLower(valid) == lowerType {
			return nil
		}
	}

	return fmt.Errorf("invalid asset type: %s", assetType)
}

// ValidateAccountType implements AccountTypeValidator
func (m *MockValidatorProvider) ValidateAccountType(accountType string) error {
	if m.ShouldReturnError {
		return fmt.Errorf("%s", m.ErrorMessage)
	}

	// Check against valid account types
	for _, valid := range m.ValidAccountTypes {
		if valid == accountType {
			return nil
		}
	}

	return fmt.Errorf("invalid account type: %s", accountType)
}

// ValidateCurrency implements CurrencyValidator
func (m *MockValidatorProvider) ValidateCurrency(code string) error {
	if m.ShouldReturnError {
		return fmt.Errorf("%s", m.ErrorMessage)
	}

	// Check against valid currency codes
	codePattern := regexp.MustCompile(`^[A-Z]{3}$`)
	if !codePattern.MatchString(code) {
		return fmt.Errorf("invalid currency code format: %s", code)
	}

	for _, valid := range m.ValidCurrencyCodes {
		if valid == code {
			return nil
		}
	}

	return fmt.Errorf("invalid currency code: %s", code)
}

// ValidateCountryAddress implements CountryValidator
func (m *MockValidatorProvider) ValidateCountryAddress(code string) error {
	if m.ShouldReturnError {
		return fmt.Errorf("%s", m.ErrorMessage)
	}

	// Check against valid country codes
	countryPattern := regexp.MustCompile(`^[A-Z]{2}$`)
	if !countryPattern.MatchString(code) {
		return fmt.Errorf("invalid country code format: %s", code)
	}

	for _, valid := range m.ValidCountryCodes {
		if valid == code {
			return nil
		}
	}

	return fmt.Errorf("invalid country code: %s", code)
}

// Ensure MockValidatorProvider implements ValidatorProvider
var _ core.ValidatorProvider = (*MockValidatorProvider)(nil)

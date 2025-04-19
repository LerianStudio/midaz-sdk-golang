package validation

import (
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation/adapters"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation/core"
	validationTesting "github.com/LerianStudio/midaz-sdk-golang/pkg/validation/testing"
)

func TestValidatorProvider(t *testing.T) {
	// Test default validator
	defaultValidator := core.DefaultValidatorProvider()
	if defaultValidator == nil {
		t.Errorf("Expected non-nil default validator")
	}

	// Test lib-commons adapter
	libCommonsValidator := adapters.NewLibCommonsValidator()
	if libCommonsValidator == nil {
		t.Errorf("Expected non-nil lib-commons validator")
	}

	// Test creating validator with provider
	validator, err := NewValidator(
		[]core.ValidationOption{
			core.WithMaxMetadataSize(1024),
			core.WithStrictMode(true),
		},
		WithValidatorProviderOption(libCommonsValidator),
	)
	if err != nil {
		t.Errorf("Failed to create validator: %v", err)
	}

	if validator == nil {
		t.Errorf("Expected non-nil validator")
	}

	// Test with mock validator
	mockValidator := validationTesting.NewMockValidator()
	if mockValidator == nil {
		t.Errorf("Expected non-nil mock validator")
	}

	// Test creating validator with mock provider
	validator, err = NewValidator(
		[]core.ValidationOption{
			core.WithMaxMetadataSize(1024),
		},
		WithValidatorProviderOption(mockValidator),
	)
	if err != nil {
		t.Errorf("Failed to create validator with mock provider: %v", err)
	}

	if validator == nil {
		t.Errorf("Expected non-nil validator")
	}
}

func TestMockValidator(t *testing.T) {
	// Create a mock validator that always succeeds
	mockValidator := validationTesting.NewMockValidator()

	// Test with success case
	err := mockValidator.ValidateType("USD")
	if err != nil {
		t.Errorf("Expected success with valid asset type, got error: %v", err)
	}

	// Test with failure case
	mockValidator.ShouldReturnError = true
	mockValidator.ErrorMessage = "mock validation error"

	err = mockValidator.ValidateType("USD")
	if err == nil {
		t.Errorf("Expected error when ShouldReturnError is true")
	} else if err.Error() != "mock validation error" {
		t.Errorf("Expected error message '%s', got '%s'", "mock validation error", err.Error())
	}

	// Test with specific validation failure
	mockValidator.ShouldReturnError = false
	mockValidator.ValidAssetTypes = []string{"USD", "EUR"}

	err = mockValidator.ValidateType("USD")
	if err != nil {
		t.Errorf("Expected success with allowed asset type, got error: %v", err)
	}

	err = mockValidator.ValidateType("BTC")
	if err == nil {
		t.Errorf("Expected error with disallowed asset type")
	}
}

func TestValidatorWithProvider(t *testing.T) {
	// Create a test validator that only allows specific values
	mockValidator := validationTesting.NewMockValidator()
	mockValidator.ValidAssetTypes = []string{"USD", "EUR"}
	mockValidator.ValidAccountTypes = []string{"deposit"}
	mockValidator.ValidCurrencyCodes = []string{"USD", "EUR"}
	mockValidator.ValidCountryCodes = []string{"US", "DE"}

	// Create a validator with our mock provider
	validator, err := NewValidator(
		[]core.ValidationOption{
			core.WithMaxMetadataSize(1024),
		},
		WithValidatorProviderOption(mockValidator),
	)
	if err != nil {
		t.Errorf("Failed to create validator: %v", err)
	}

	// Test asset type validation
	t.Run("AssetType", func(t *testing.T) {
		// Valid asset type
		if err := validator.ValidateAssetType("USD"); err != nil {
			t.Errorf("Expected success for valid asset type, got: %v", err)
		}

		// Invalid asset type
		if err := validator.ValidateAssetType("BTC"); err == nil {
			t.Errorf("Expected error for invalid asset type")
		}
	})

	// Test account type validation
	t.Run("AccountType", func(t *testing.T) {
		// Valid account type
		if err := validator.ValidateAccountType("deposit"); err != nil {
			t.Errorf("Expected success for valid account type, got: %v", err)
		}

		// Invalid account type
		if err := validator.ValidateAccountType("savings"); err == nil {
			t.Errorf("Expected error for invalid account type")
		}
	})

	// Test currency code validation
	t.Run("CurrencyCode", func(t *testing.T) {
		// Valid currency code
		if err := validator.ValidateCurrencyCode("USD"); err != nil {
			t.Errorf("Expected success for valid currency code, got: %v", err)
		}

		// Invalid currency code
		if err := validator.ValidateCurrencyCode("JPY"); err == nil {
			t.Errorf("Expected error for invalid currency code")
		}
	})

	// Test country code validation
	t.Run("CountryCode", func(t *testing.T) {
		// Valid country code
		if err := validator.ValidateCountryCode("US"); err != nil {
			t.Errorf("Expected success for valid country code, got: %v", err)
		}

		// Invalid country code
		if err := validator.ValidateCountryCode("JP"); err == nil {
			t.Errorf("Expected error for invalid country code")
		}
	})
}

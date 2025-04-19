package standalone

import (
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation"
	validationCore "github.com/LerianStudio/midaz-sdk-golang/pkg/validation/core"
)

// CustomValidator is an example of a custom validator implementation
type CustomValidator struct {
	// Add custom fields here if needed
}

// ValidateType implements TypeValidator
func (v *CustomValidator) ValidateType(assetType string) error {
	// Example: Only allow certain asset types
	validTypes := map[string]bool{
		"crypto":    true,
		"currency":  true,
		"commodity": true,
		// Notice "others" is not in this list - this is our custom restriction
	}

	if !validTypes[assetType] {
		return fmt.Errorf("custom validator: invalid asset type: %s", assetType)
	}
	return nil
}

// ValidateAccountType implements AccountTypeValidator
func (v *CustomValidator) ValidateAccountType(accountType string) error {
	// Example: Only allow certain account types
	validTypes := map[string]bool{
		"checking": true,
		"savings":  true,
		// Notice "investment" is not in this list - this is our custom restriction
	}

	if !validTypes[accountType] {
		return fmt.Errorf("custom validator: invalid account type: %s", accountType)
	}
	return nil
}

// ValidateCurrency implements CurrencyValidator
func (v *CustomValidator) ValidateCurrency(code string) error {
	// Example: Only allow certain currencies
	validCurrencies := map[string]bool{
		"USD": true,
		"EUR": true,
		// Notice "JPY" is not in this list - this is our custom restriction
	}

	if !validCurrencies[code] {
		return fmt.Errorf("custom validator: invalid currency code: %s", code)
	}
	return nil
}

// ValidateCountryAddress implements CountryAddressValidator
func (v *CustomValidator) ValidateCountryAddress(code string) error {
	// Example: Only allow certain country codes
	validCountries := map[string]bool{
		"US": true,
		"CA": true,
		// Notice "UK" is not in this list - this is our custom restriction
	}

	if !validCountries[code] {
		return fmt.Errorf("custom validator: invalid country code: %s", code)
	}
	return nil
}

// RunExample demonstrates using a custom validator without a client
func RunExample() {
	// Create a new instance of our custom validator
	customValidator := &CustomValidator{}

	// Wrap it as a ValidatorProvider
	customProvider := validationCore.ValidatorProvider(customValidator)

	// Create a validator with our custom provider
	validator, err := validation.NewValidator(
		[]validationCore.ValidationOption{},
		validation.WithValidatorProviderOption(customProvider),
	)
	if err != nil {
		fmt.Printf("Error creating validator: %v\n", err)
		return
	}

	fmt.Println("Testing standalone custom validator:")

	// Test asset type validation
	fmt.Println("\nAsset Type Validation:")
	assetTypes := []string{"crypto", "currency", "commodity", "others"}
	for _, assetType := range assetTypes {
		err := validator.ValidateAssetType(assetType)
		if err != nil {
			fmt.Printf("❌ Asset type '%s' is invalid: %v\n", assetType, err)
		} else {
			fmt.Printf("✅ Asset type '%s' is valid\n", assetType)
		}
	}

	// Test account type validation
	fmt.Println("\nAccount Type Validation:")
	accountTypes := []string{"checking", "savings", "investment"}
	for _, accountType := range accountTypes {
		err := validator.ValidateAccountType(accountType)
		if err != nil {
			fmt.Printf("❌ Account type '%s' is invalid: %v\n", accountType, err)
		} else {
			fmt.Printf("✅ Account type '%s' is valid\n", accountType)
		}
	}

	// Test currency validation
	fmt.Println("\nCurrency Code Validation:")
	currencies := []string{"USD", "EUR", "JPY"}
	for _, currency := range currencies {
		// Direct validation using the provider
		err := customValidator.ValidateCurrency(currency)
		if err != nil {
			fmt.Printf("❌ Currency '%s' is invalid: %v\n", currency, err)
		} else {
			fmt.Printf("✅ Currency '%s' is valid\n", currency)
		}
	}

	// Test country code validation
	fmt.Println("\nCountry Code Validation:")
	countries := []string{"US", "CA", "UK"}
	for _, country := range countries {
		// Direct validation using the provider
		err := customValidator.ValidateCountryAddress(country)
		if err != nil {
			fmt.Printf("❌ Country code '%s' is invalid: %v\n", country, err)
		} else {
			fmt.Printf("✅ Country code '%s' is valid\n", country)
		}
	}

	fmt.Println("\nStandalone validator tests completed!")
}

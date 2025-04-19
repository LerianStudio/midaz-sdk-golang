package client

import (
	"fmt"
	"net/http"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation/adapters"
)

// CustomValidator is a simple example of a custom validator for use with the client
type CustomValidator struct{}

func (v *CustomValidator) ValidateType(assetType string) error {
	// Only allow specific assets in our custom validator
	validAssets := map[string]bool{
		"USD": true,
		"EUR": true,
		"BTC": true,
	}

	if !validAssets[assetType] {
		return fmt.Errorf("custom validator: unsupported asset type: %s (only USD, EUR, BTC allowed)", assetType)
	}

	return nil
}

func (v *CustomValidator) ValidateAccountType(accountType string) error {
	// Only allow specific account types in our custom validator
	validAccountTypes := map[string]bool{
		"deposit":     true,
		"savings":     true,
		"marketplace": true,
	}

	if !validAccountTypes[accountType] {
		return fmt.Errorf("custom validator: unsupported account type: %s", accountType)
	}

	return nil
}

func (v *CustomValidator) ValidateCurrency(code string) error {
	// Only allow specific currencies in our custom validator
	validCurrencies := map[string]bool{
		"USD": true,
		"EUR": true,
	}

	if !validCurrencies[code] {
		return fmt.Errorf("custom validator: unsupported currency code: %s", code)
	}

	return nil
}

func (v *CustomValidator) ValidateCountryAddress(code string) error {
	// Only allow specific countries in our custom validator
	validCountries := map[string]bool{
		"US": true,
		"GB": true,
		"FR": true,
	}

	if !validCountries[code] {
		return fmt.Errorf("custom validator: unsupported country code: %s", code)
	}

	return nil
}

// RunExample demonstrates using a custom validator with the client
func RunExample() {
	// Create an HTTP client with a timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Example 1: Using the default validator
	fmt.Println("Example 1: Creating a client with default validator")
	defaultClient, err := client.New(
		client.WithHTTPClient(httpClient),
		client.WithEnvironment("development"),
	)
	if err != nil {
		fmt.Printf("Error creating default client: %v\n", err)
		return
	}

	// Example 2: Using a custom validator
	fmt.Println("\nExample 2: Creating a client with a custom validator")
	customClient, err := client.New(
		client.WithHTTPClient(httpClient),
		client.WithEnvironment("development"),
		client.WithValidatorProvider(&CustomValidator{}),
	)
	if err != nil {
		fmt.Printf("Error creating custom client: %v\n", err)
		return
	}

	// Example 3: Using the lib-commons adapter explicitly
	fmt.Println("\nExample 3: Creating a client with the lib-commons adapter explicitly")
	adapterClient, err := client.New(
		client.WithHTTPClient(httpClient),
		client.WithEnvironment("development"),
		client.WithValidatorProvider(adapters.NewLibCommonsValidator()),
	)
	if err != nil {
		fmt.Printf("Error creating adapter client: %v\n", err)
		return
	}

	// Test validation with the custom validator
	fmt.Println("\nTesting validation with custom validator:")

	// Create a transaction with USD (allowed)
	txUSD := &models.TransactionDSLInput{
		Description: "Test transaction with USD",
		Send: &models.DSLSend{
			Asset: "USD",
			Value: 100,
			Scale: 2,
			Source: &models.DSLSource{
				From: []models.DSLFromTo{
					{Account: "acc_123"},
				},
			},
			Distribute: &models.DSLDistribute{
				To: []models.DSLFromTo{
					{Account: "acc_456"},
				},
			},
		},
	}

	// Validate with custom client
	err = txUSD.ValidateWithProvider(customClient.GetValidatorProvider())
	if err != nil {
		fmt.Printf("❌ USD validation failed: %v\n", err)
	} else {
		fmt.Println("✅ USD validation passed with custom validator")
	}

	// Create a transaction with JPY (not allowed)
	txJPY := &models.TransactionDSLInput{
		Description: "Test transaction with JPY",
		Send: &models.DSLSend{
			Asset: "JPY",
			Value: 10000,
			Scale: 0,
			Source: &models.DSLSource{
				From: []models.DSLFromTo{
					{Account: "acc_123"},
				},
			},
			Distribute: &models.DSLDistribute{
				To: []models.DSLFromTo{
					{Account: "acc_456"},
				},
			},
		},
	}

	// Validate with custom client (should fail)
	err = txJPY.ValidateWithProvider(customClient.GetValidatorProvider())
	if err != nil {
		fmt.Printf("✅ JPY validation correctly failed: %v\n", err)
	} else {
		fmt.Println("❌ JPY validation unexpectedly passed")
	}

	// Validate with default client (should pass)
	err = txJPY.ValidateWithProvider(defaultClient.GetValidatorProvider())
	if err != nil {
		fmt.Printf("❌ JPY validation with default validator failed: %v\n", err)
	} else {
		fmt.Println("✅ JPY validation passed with default validator")
	}

	// Validate with adapter client (should pass)
	err = txJPY.ValidateWithProvider(adapterClient.GetValidatorProvider())
	if err != nil {
		fmt.Printf("❌ JPY validation with adapter validator failed: %v\n", err)
	} else {
		fmt.Println("✅ JPY validation passed with adapter validator")
	}
}

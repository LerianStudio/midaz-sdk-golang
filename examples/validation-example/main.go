package main

import (
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/examples/validation-example/client"
	"github.com/LerianStudio/midaz-sdk-golang/examples/validation-example/standalone"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation"
)

// This file serves as the entry point for the validation examples.
// It demonstrates different validation approaches in the Midaz SDK.

func main() {
	fmt.Println("Midaz SDK - Enhanced Validation Examples")
	fmt.Println("========================================")

	// Run the standalone validator example
	fmt.Println("\n1. Standalone Custom Validator Example")
	fmt.Println("---------------------------------------")
	standalone.RunExample()

	// Run the client validator example
	fmt.Println("\n2. Client with Custom Validator Example")
	fmt.Println("----------------------------------------")
	client.RunExample()

	// Run the original validation examples
	fmt.Println("\n3. Basic Validation Examples")
	fmt.Println("----------------------------")
	runBasicValidationExamples()
}

// runBasicValidationExamples contains the original validation examples
func runBasicValidationExamples() {
	// Example 1: Asset Code Validation
	fmt.Println("\nAsset Code Validation:")
	validateAssetCode("USD")  // Valid
	validateAssetCode("usd")  // Invalid - lowercase
	validateAssetCode("US12") // Invalid - contains numbers

	// Example 2: Metadata Validation
	fmt.Println("\nMetadata Validation:")
	validateMetadata(map[string]interface{}{
		"reference": "INV-123",
		"amount":    100.50,
		"approved":  true,
	})

	validateMetadata(map[string]interface{}{
		"reference": "INV-123",
		"items":     []string{"item1", "item2"}, // Invalid - array not supported
	})

	// Example 3: Account Type Validation
	fmt.Println("\nAccount Type Validation:")
	validateAccountType("checking") // Valid
	validateAccountType("SAVINGS")  // Invalid - uppercase
	validateAccountType("loan")     // Invalid - not supported

	// Example 4: Country Code Validation
	fmt.Println("\nCountry Code Validation:")
	validateCountryCode("US")  // Valid
	validateCountryCode("us")  // Invalid - lowercase
	validateCountryCode("USA") // Invalid - wrong format

	// Example 5: Transaction Validation
	fmt.Println("\nTransaction Validation:")
	validateTransaction(createValidTransaction())
	validateTransaction(createInvalidTransaction())

	// Example 6: Address Validation
	fmt.Println("\nAddress Validation:")
	validateAddress()
}

func validateAssetCode(code string) {
	fmt.Printf("Validating asset code '%s': ", code)
	if err := validation.ValidateAssetCode(code); err != nil {
		fmt.Printf("❌ Invalid: %v\n", err)
	} else {
		fmt.Println("✅ Valid")
	}
}

func validateAccountType(accountType string) {
	fmt.Printf("Validating account type '%s': ", accountType)
	if err := validation.ValidateAccountType(accountType); err != nil {
		fmt.Printf("❌ Invalid: %v\n", err)
	} else {
		fmt.Println("✅ Valid")
	}
}

func validateCountryCode(code string) {
	fmt.Printf("Validating country code '%s': ", code)
	if err := validation.ValidateCountryCode(code); err != nil {
		fmt.Printf("❌ Invalid: %v\n", err)
	} else {
		fmt.Println("✅ Valid")
	}
}

func validateMetadata(metadata map[string]interface{}) {
	fmt.Println("Validating metadata:")
	errors := validation.EnhancedValidateMetadata(metadata)
	if errors.HasErrors() {
		fmt.Printf("❌ Invalid metadata: %v\n", errors)
	} else {
		fmt.Println("✅ Valid metadata")
	}
}

func validateTransaction(tx map[string]interface{}) {
	fmt.Println("Validating transaction:")
	errors := validation.EnhancedValidateTransactionInput(tx)
	if errors.HasErrors() {
		fmt.Printf("❌ Invalid transaction: %v\n", errors)
	} else {
		fmt.Println("✅ Valid transaction")
	}
}

func validateAddress() {
	// Valid address
	fmt.Println("Validating valid address:")
	validAddress := &validation.Address{
		Line1:   "123 Main St",
		City:    "New York",
		State:   "NY",
		ZipCode: "10001",
		Country: "US",
	}
	if err := validation.ValidateAddress(validAddress); err != nil {
		fmt.Printf("❌ Invalid address: %v\n", err)
	} else {
		fmt.Println("✅ Valid address")
	}

	// Invalid address
	fmt.Println("Validating invalid address:")
	invalidAddress := &validation.Address{
		Line1:   "123 Main St",
		City:    "New York",
		State:   "NY",
		ZipCode: "10001",
		Country: "USA", // Invalid country code - should be 2 letters
	}
	if err := validation.ValidateAddress(invalidAddress); err != nil {
		fmt.Printf("❌ Invalid address: %v\n", err)
	} else {
		fmt.Println("✅ Valid address")
	}
}

func createValidTransaction() map[string]interface{} {
	return map[string]interface{}{
		"asset_code": "USD",
		"amount":     float64(1000),
		"scale":      2,
		"operations": []map[string]interface{}{
			{
				"type":       "DEBIT",
				"account_id": "account1",
				"amount":     float64(1000),
			},
			{
				"type":       "CREDIT",
				"account_id": "account2",
				"amount":     float64(1000),
			},
		},
		"metadata": map[string]interface{}{
			"reference": "INV-12345",
			"customer":  "John Doe",
		},
	}
}

func createInvalidTransaction() map[string]interface{} {
	return map[string]interface{}{
		"asset_code": "USD",
		"amount":     float64(1000),
		"scale":      2,
		"operations": []map[string]interface{}{
			{
				"type":       "DEBIT",
				"account_id": "account1",
				"amount":     float64(900), // Doesn't match transaction amount
			},
			{
				"type":       "CREDIT",
				"account_id": "account2",
				"amount":     float64(1000),
			},
		},
	}
}

package main

import (
	"fmt"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation"
)

func main() {
	fmt.Println("Midaz SDK - Enhanced Validation Example")
	fmt.Println("----------------------------------------")

	// Example 1: Asset Code Validation
	fmt.Println("\n1. Asset Code Validation:")
	validateAssetCode("USD")  // Valid
	validateAssetCode("usd")  // Invalid - lowercase
	validateAssetCode("US12") // Invalid - contains numbers

	// Example 2: Metadata Validation
	fmt.Println("\n2. Metadata Validation:")
	validateMetadata(map[string]any{
		"reference": "INV-123",
		"amount":    100.50,
		"approved":  true,
	}) // Valid

	validateMetadata(map[string]any{
		"reference": "INV-123",
		"items":     []string{"item1", "item2"}, // Invalid - array not supported
	})

	// Example 3: Transaction Validation
	fmt.Println("\n3. Transaction Validation:")
	validateTransaction(createValidTransaction())
	validateTransaction(createInvalidTransaction())

	// Example 4: Date Range Validation
	fmt.Println("\n4. Date Range Validation:")
	now := time.Now()
	past := now.AddDate(0, -1, 0)
	future := now.AddDate(0, 1, 0)

	validateDateRange(past, now)   // Valid
	validateDateRange(future, now) // Invalid - start after end

	// Example 5: Address Validation
	fmt.Println("\n5. Address Validation:")
	validateAddress(createValidAddress())
	validateAddress(createInvalidAddress())
}

func validateAssetCode(code string) {
	fmt.Printf("Validating asset code: %s\n", code)
	if err := validation.EnhancedValidateAssetCode(code); err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
	} else {
		fmt.Printf("✓ Valid asset code\n")
	}
}

func validateMetadata(metadata map[string]any) {
	fmt.Println("Validating metadata:")
	errors := validation.EnhancedValidateMetadata(metadata)
	if errors.HasErrors() {
		fmt.Printf("ERROR: %s\n", errors.Error())
	} else {
		fmt.Printf("✓ Valid metadata\n")
	}
}

func validateTransaction(tx map[string]any) {
	fmt.Println("Validating transaction:")
	errors := validation.EnhancedValidateTransactionInput(tx)
	if errors.HasErrors() {
		fmt.Printf("ERROR: %s\n", errors.Error())
	} else {
		fmt.Printf("✓ Valid transaction\n")
	}
}

func validateDateRange(start, end time.Time) {
	fmt.Printf("Validating date range: %s to %s\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
	errors := validation.EnhancedValidateDateRange(start, end, "startDate", "endDate")
	if errors.HasErrors() {
		fmt.Printf("ERROR: %s\n", errors.Error())
	} else {
		fmt.Printf("✓ Valid date range\n")
	}
}

func validateAddress(address *validation.Address) {
	fmt.Println("Validating address:")
	errors := validation.EnhancedValidateAddress(address, "address")
	if errors.HasErrors() {
		fmt.Printf("ERROR: %s\n", errors.Error())
	} else {
		fmt.Printf("✓ Valid address\n")
	}
}

func createValidTransaction() map[string]any {
	return map[string]any{
		"asset_code": "USD",
		"amount":     float64(1000),
		"scale":      2,
		"operations": []map[string]any{
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
		"metadata": map[string]any{
			"reference": "INV-12345",
			"customer":  "John Doe",
		},
	}
}

func createInvalidTransaction() map[string]any {
	return map[string]any{
		"asset_code": "USD",
		"amount":     float64(1000),
		"scale":      2,
		"operations": []map[string]any{
			{
				"type":       "DEBIT",
				"account_id": "account1",
				"amount":     float64(1000),
			},
			{
				"type":       "CREDIT",
				"account_id": "account2",
				"amount":     float64(500), // Unbalanced - only half the debit amount
			},
		},
	}
}

func createValidAddress() *validation.Address {
	return &validation.Address{
		Line1:   "123 Main St",
		ZipCode: "12345",
		City:    "New York",
		State:   "NY",
		Country: "US",
	}
}

func createInvalidAddress() *validation.Address {
	return &validation.Address{
		Line1:   "123 Main St",
		ZipCode: "12345",
		City:    "New York",
		State:   "NY",
		Country: "USA", // Invalid country code - should be 2 letters
	}
}

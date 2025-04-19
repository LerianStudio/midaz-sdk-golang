package main

import (
	"fmt"
	"os"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	validationTesting "github.com/LerianStudio/midaz-sdk-golang/pkg/validation/testing"
)

// Simple integration test to verify that the validator provider is correctly
// passed to the entity and used for validation.
func main() {
	// Create a mock validator that only allows specific values
	mockValidator := validationTesting.NewMockValidator()
	mockValidator.ValidAssetTypes = []string{"USD", "EUR"} // Only allow USD and EUR

	// Create a client with the mock validator
	c, err := client.New(
		client.WithBaseURL("http://localhost:3000"),
		client.UseEntity(),                          // Enable entity API
		client.WithValidatorProvider(mockValidator), // Use client package's WithValidatorProvider
	)
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	// Create a test transaction with USD (allowed)
	fmt.Println("Testing valid asset code (USD)...")
	txInput := &models.TransactionDSLInput{
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

	// Use the client's validator provider for validation
	err = txInput.ValidateWithProvider(c.GetValidatorProvider())
	if err != nil {
		fmt.Printf("Error validating USD transaction: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("USD transaction validated successfully!")

	// Create a test transaction with BTC (not allowed)
	fmt.Println("\nTesting invalid asset code (BTC)...")
	txInput.Send.Asset = "BTC"

	// This should fail since BTC is not in the allowed list
	err = txInput.ValidateWithProvider(c.GetValidatorProvider())
	if err == nil {
		fmt.Println("Error: BTC transaction should have failed validation")
		os.Exit(1)
	}
	fmt.Printf("BTC transaction correctly failed validation: %v\n", err)

	fmt.Println("\nAll tests passed!")
}

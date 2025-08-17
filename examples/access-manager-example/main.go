package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	auth "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Get plugin auth configuration from environment variables
	pluginAuthEnabled := os.Getenv("PLUGIN_AUTH_ENABLED") == "true"
	pluginAuthAddress := os.Getenv("PLUGIN_AUTH_ADDRESS")

	// Use MIDAZ_CLIENT_ID and MIDAZ_CLIENT_SECRET as they are defined in the .env file
	clientID := os.Getenv("MIDAZ_CLIENT_ID")
	clientSecret := os.Getenv("MIDAZ_CLIENT_SECRET")

	//Configure plugin auth
	pluginAuth := auth.AccessManager{
		Enabled:      pluginAuthEnabled,
		Address:      pluginAuthAddress,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	// Create a configuration with plugin auth
	cfg, err := config.NewConfig(
		config.WithAccessManager(pluginAuth),
	)
	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
	}

	// Create a client with the configuration
	c, err := client.New(
		client.WithConfig(cfg),
		client.UseEntityAPI(),                      // Enable the Entity API
		client.WithObservability(true, true, true), // Enable observability
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Debug: Print configuration information
	log.Printf("Debug: SDK Version: %s", client.Version)
	log.Printf("Debug: Environment: %s", cfg.Environment)

	// Create a context with timeout for the API call
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a trace span for the organization creation
	ctx, span := c.GetObservabilityProvider().Tracer().Start(ctx, "create_organization")
	defer span.End()

	// Prepare pointer fields
	description := "Ledger Test"
	line2 := "CJ 203"

	// Create a simplified organization input for testing
	// Create organization input using builder pattern
	input := models.NewCreateOrganizationInput("Acme Corporation").
		WithLegalDocument("78425230000190").
		WithDoingBusinessAs("The ledger.io").
		WithStatus(models.Status{
			Code:        "ACTIVE",
			Description: &description,
		}).
		WithAddress(models.Address{
			Line1:   "Avenida Paulista, 1234",
			Line2:   &line2,
			ZipCode: "01310916",
			City:    "São Paulo",
			State:   "SP",
			Country: "BR",
		}).
		WithMetadata(map[string]any{
			"source": "plugin-auth-example",
		})

	// Validate the input
	if err := input.Validate(); err != nil {
		log.Fatalf("Organization input validation failed: %v", err)
	}

	// Log the input for debugging
	log.Printf("Creating organization with legal name: %s", input.LegalName)

	// Execute the request
	organization, err := c.Entity.Organizations.CreateOrganization(ctx, input)

	if err != nil {
		log.Printf("Failed to create organization: %v", err)

		// Try to get more details about the error
		if strings.Contains(err.Error(), "Internal Server Error") {
			log.Printf("This is a server-side error. Check the following:")
			log.Printf("1. Is the plugin auth service running and accessible at %s?", pluginAuth.Address)
			log.Printf("2. Are the client ID and secret correct?")
			log.Printf("3. Does the token have the necessary permissions?")
			log.Printf("4. Is the Midaz API server running and properly configured?")
		}

		// Check for authentication issues
		if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "authentication") {
			log.Printf("This appears to be an authentication error. Check your plugin auth configuration.")
		}
	} else {
		fmt.Println("Organization created successfully!")
		fmt.Println("Plugin Auth:")
		fmt.Printf("- Enabled: %t\n", pluginAuth.Enabled)
		fmt.Printf("- ID: %s\n", organization.ID)
		fmt.Printf("- Legal Name: %s\n", organization.LegalName)

		if organization.DoingBusinessAs != nil {
			fmt.Printf("- Doing Business As: %s\n", *organization.DoingBusinessAs)
		} else {
			fmt.Printf("- Doing Business As: <not set>\n")
		}

		fmt.Printf("- Status: %s\n", organization.Status.Code)
		fmt.Printf("- Created At: %s\n", organization.CreatedAt.Format(time.RFC3339))
	}

	fmt.Println("\nTest completed.")
}

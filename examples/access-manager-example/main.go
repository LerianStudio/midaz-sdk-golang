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
	pluginAuth, cfg := setupConfiguration()

	c, err := createClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx, span := c.GetObservabilityProvider().Tracer().Start(ctx, "create_organization")
	defer span.End()

	input := buildOrganizationInput()

	if err := input.Validate(); err != nil {
		log.Fatalf("Organization input validation failed: %v", err)
	}

	log.Printf("Creating organization with legal name: %s", input.LegalName)

	organization, err := c.Entity.Organizations.CreateOrganization(ctx, input)
	if err != nil {
		handleCreationError(err, pluginAuth)
	} else {
		printOrganizationDetails(organization, pluginAuth)
	}

	fmt.Println("\nTest completed.")
}

func setupConfiguration() (auth.AccessManager, *config.Config) {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	pluginAuth := auth.AccessManager{
		Enabled:      os.Getenv("PLUGIN_AUTH_ENABLED") == "true",
		Address:      os.Getenv("PLUGIN_AUTH_ADDRESS"),
		ClientID:     os.Getenv("MIDAZ_CLIENT_ID"),
		ClientSecret: os.Getenv("MIDAZ_CLIENT_SECRET"),
	}

	cfg, err := config.NewConfig(config.WithAccessManager(pluginAuth))
	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
	}

	log.Printf("Debug: SDK Version: %s", client.Version)
	log.Printf("Debug: Environment: %s", cfg.Environment)

	return pluginAuth, cfg
}

func createClient(cfg *config.Config) (*client.Client, error) {
	return client.New(
		client.WithConfig(cfg),
		client.UseEntityAPI(),
		client.WithObservability(true, true, true),
	)
}

func buildOrganizationInput() *models.CreateOrganizationInput {
	description := "Ledger Test"
	line2 := "CJ 203"

	return models.NewCreateOrganizationInput("Acme Corporation").
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
}

func handleCreationError(err error, pluginAuth auth.AccessManager) {
	log.Printf("Failed to create organization: %v", err)

	if strings.Contains(err.Error(), "Internal Server Error") {
		log.Printf("This is a server-side error. Check the following:")
		log.Printf("1. Is the plugin auth service running and accessible at %s?", pluginAuth.Address)
		log.Printf("2. Are the client ID and secret correct?")
		log.Printf("3. Does the token have the necessary permissions?")
		log.Printf("4. Is the Midaz API server running and properly configured?")
	}

	if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "authentication") {
		log.Printf("This appears to be an authentication error. Check your plugin auth configuration.")
	}
}

func printOrganizationDetails(organization *models.Organization, pluginAuth auth.AccessManager) {
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

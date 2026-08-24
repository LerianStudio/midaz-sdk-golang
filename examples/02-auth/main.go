// Package main demonstrates v3 Access Manager authentication.
//
// Run with credentials in your .env or shell:
//
//	PLUGIN_AUTH_ENABLED=true
//	PLUGIN_AUTH_ADDRESS=https://your-auth-service.example.com
//	MIDAZ_CLIENT_ID=...
//	MIDAZ_CLIENT_SECRET=...
//
// The SDK reads these via config.FromEnvironment() — explicit opt-in,
// no implicit shell-set behavior. See docs/auth.md for the full setup.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/joho/godotenv"
)

func main() {
	c, err := buildClient()
	if err != nil {
		log.Fatalf("midaz.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx, span := c.GetObservabilityProvider().Tracer().Start(ctx, "create_organization")
	defer span.End()

	input := buildOrganizationInput()

	if err := input.Validate(); err != nil {
		log.Fatalf("Organization input validation failed: %v", err)
	}

	log.Printf("Creating organization with legal name: %q", input.LegalName)

	organization, err := c.V1.Organizations.Create(ctx, input)
	if err != nil {
		handleCreationError(err, c.GetConfig().AccessManager.Address)
	} else {
		printOrganizationDetails(organization, c.GetConfig().AccessManager.Enabled)
	}

	fmt.Println("\nTest completed.")
}

// buildClient assembles a *midaz.Client from environment variables. The
// pattern is: load .env (best-effort), build a *config.Config that reads
// PLUGIN_AUTH_* via FromEnvironment, then pass it to midaz.New through
// WithConfig. The auth-required gate accepts FromEnvironment-driven
// AccessManager (when PLUGIN_AUTH_ENABLED=true) the same way it accepts a
// programmatic midaz.WithAccessManager call.
func buildClient() (*midaz.Client, error) {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: error loading .env file: %v", err)
	}

	cfg, err := config.NewConfig(config.FromEnvironment())
	if err != nil {
		return nil, fmt.Errorf("config.NewConfig: %w", err)
	}

	log.Printf("Debug: SDK Version: %q", midaz.Version)
	log.Printf("Debug: Environment: %q", cfg.Environment)

	return midaz.New(
		midaz.WithConfig(cfg),
		// v3: midaz.WithObservability(t,m,l bool) was deleted. The canonical
		// expression chains observability.WithComponentEnabled through
		// midaz.WithObservabilityOptions, which composes uniformly with
		// every other observability.Option (service name, collector endpoint,
		// sampling, attributes, etc.).
		midaz.WithObservabilityOptions(
			observability.WithComponentEnabled(true, true, true),
		),
	)
}

func buildOrganizationInput() *models.CreateOrganizationInput {
	description := "Ledger Test"
	line2 := "CJ 203"

	return models.NewCreateOrganizationInput("Acme Corporation", "78425230000190").
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

func handleCreationError(err error, accessManagerAddress string) {
	log.Printf("Failed to create organization: %v", err)

	if strings.Contains(err.Error(), "Internal Server Error") {
		log.Printf("This is a server-side error. Check the following:")
		log.Printf("1. Is the plugin auth service running and accessible at %q?", accessManagerAddress)
		log.Printf("2. Are the client ID and secret correct?")
		log.Printf("3. Does the token have the necessary permissions?")
		log.Printf("4. Is the Midaz API server running and properly configured?")
	}

	if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "authentication") {
		log.Printf("This appears to be an authentication error. Check your plugin auth configuration.")
	}
}

func printOrganizationDetails(organization *models.Organization, accessManagerEnabled bool) {
	fmt.Println("Organization created successfully!")
	fmt.Println("Plugin Auth:")
	fmt.Printf("- Enabled: %t\n", accessManagerEnabled)
	fmt.Printf("- ID: %q\n", organization.ID)
	fmt.Printf("- Legal Name: %q\n", organization.LegalName)

	if organization.DoingBusinessAs != nil {
		fmt.Printf("- Doing Business As: %q\n", *organization.DoingBusinessAs)
	} else {
		fmt.Printf("- Doing Business As: <not set>\n")
	}

	fmt.Printf("- Status: %q\n", organization.Status.Code)
	fmt.Printf("- Created At: %q\n", organization.CreatedAt.Format(time.RFC3339))
}

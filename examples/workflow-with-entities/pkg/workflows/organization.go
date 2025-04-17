package workflows

import (
	"context"
	"fmt"
	"strings"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
)

// CreateOrganization creates a new organization and returns its ID
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - client: The initialized Midaz SDK client
//
// Returns:
//   - string: The ID of the created organization
//   - error: Any error encountered during the operation
func CreateOrganization(ctx context.Context, client *client.Client) (string, error) {
	fmt.Println("\n\n🏢 STEP 1: ORGANIZATION CREATION")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Println("\nCreating organization...")

	// Get plugin auth configuration from environment variables

	organization, err := client.Entity.Organizations.CreateOrganization(ctx, &models.CreateOrganizationInput{
		LegalName:     "Example Corp",
		LegalDocument: "123456789",
		Address: models.Address{
			Country: "US",
		},
		Status: models.Status{
			Code: "ACTIVE",
		},
		Metadata: map[string]any{
			"industry": "Technology",
			"size":     "Small",
		},
		DoingBusinessAs: "Example Corp DBA",
	})
	if err != nil {
		return "", fmt.Errorf("failed to create organization: %w", err)
	}

	if organization.ID == "" {
		return "", fmt.Errorf("organization created but no ID was returned from the API")
	}

	fmt.Printf("✅ Organization created: %s\n", organization.LegalName)
	fmt.Printf("   ID: %s\n", organization.ID)
	fmt.Printf("   Created: %s\n", organization.CreatedAt.Format("2006-01-02 15:04:05"))

	return organization.ID, nil
}

// UpdateOrganization updates the organization metadata
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//
// Returns:
//   - error: Any error encountered during the operation
func UpdateOrganization(ctx context.Context, client *client.Client, orgID string) error {
	fmt.Println("\n\n🔄 STEP 9: ORGANIZATION UPDATE")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Println("\nUpdating organization...")

	// Get the organization first
	org, err := client.Entity.Organizations.GetOrganization(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	// Update the organization metadata
	updatedOrg, err := client.Entity.Organizations.UpdateOrganization(ctx, orgID, &models.UpdateOrganizationInput{
		LegalName:       org.LegalName,
		DoingBusinessAs: org.DoingBusinessAs,
		Address:         org.Address,
		Status:          org.Status,
		Metadata: map[string]any{
			"industry":      "Technology",
			"size":          "Medium", // Changed from "Small" to "Medium"
			"lastUpdatedAt": time.Now().Format(time.RFC3339),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}

	fmt.Printf("✅ Organization updated: %s\n", updatedOrg.LegalName)
	fmt.Printf("   ID: %s\n", updatedOrg.ID)
	fmt.Printf("   Updated: %s\n", updatedOrg.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("   Metadata: %v\n", updatedOrg.Metadata)

	return nil
}

// RetrieveOrganization retrieves the organization by ID
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//
// Returns:
//   - error: Any error encountered during the operation
func RetrieveOrganization(ctx context.Context, client *client.Client, orgID string) error {
	fmt.Println("\n\n🔍 STEP 10: ORGANIZATION RETRIEVAL")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Println("\nRetrieving organization...")

	org, err := client.Entity.Organizations.GetOrganization(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	fmt.Printf("✅ Organization retrieved: %s\n", org.LegalName)
	fmt.Printf("   ID: %s\n", org.ID)
	fmt.Printf("   Created: %s\n", org.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("   Updated: %s\n", org.UpdatedAt.Format("2006-01-02 15:04:05"))

	// Add nil check for Metadata
	metadataValue := "nil"
	if org.Metadata != nil {
		metadataValue = fmt.Sprintf("%v", org.Metadata)
	}
	fmt.Printf("   Metadata: %s\n", metadataValue)

	return nil
}

package workflows

import (
	"context"
	"fmt"
	"strings"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
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
func CreateOrganization(ctx context.Context, midazClient *client.Client) (string, error) {
	fmt.Println("\n\n🏢 STEP 1: ORGANIZATION CREATION")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Println("\nCreating organization...")

	// Get plugin auth configuration from environment variables

	organization, err := midazClient.Entity.Organizations.CreateOrganization(ctx,
		models.NewCreateOrganizationInput("Example Corp").
			WithDoingBusinessAs("Example Corp DBA").
			WithLegalDocument("123456789").
			WithAddress(models.Address{
				Country: "US",
			}).
			WithStatus(models.Status{
				Code: "ACTIVE",
			}).
			WithMetadata(map[string]any{
				"industry": "Technology",
				"size":     "Small",
			}),
	)
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
func UpdateOrganization(ctx context.Context, midazClient *client.Client, orgID string) error {
	fmt.Println("\n\n🔄 STEP 9: ORGANIZATION UPDATE")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Println("\nUpdating organization...")

	// Get the organization first
	org, err := midazClient.Entity.Organizations.GetOrganization(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	// Update the organization metadata
	var dbaValue string
	if org.DoingBusinessAs != nil {
		dbaValue = *org.DoingBusinessAs
	}

	updatedOrg, err := midazClient.Entity.Organizations.UpdateOrganization(ctx, orgID,
		models.NewUpdateOrganizationInput().
			WithLegalName(org.LegalName).
			WithDoingBusinessAsUpdate(dbaValue).
			WithAddressUpdate(models.Address(org.Address)).
			WithStatusUpdate(org.Status).
			WithUpdateMetadata(map[string]any{
				"industry":      "Technology",
				"size":          "Medium", // Changed from "Small" to "Medium"
				"lastUpdatedAt": time.Now().Format(time.RFC3339),
			}),
	)
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
func RetrieveOrganization(ctx context.Context, midazClient *client.Client, orgID string) error {
	fmt.Println("\n\n🔍 STEP 10: ORGANIZATION RETRIEVAL")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Println("\nRetrieving organization...")

	org, err := midazClient.Entity.Organizations.GetOrganization(ctx, orgID)
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

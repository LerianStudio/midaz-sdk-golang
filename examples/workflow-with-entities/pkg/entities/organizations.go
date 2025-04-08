// Package entities contains entity-specific operations for the complete workflow example.
// It provides higher-level functions that wrap the SDK's core functionality to
// simplify common operations and demonstrate best practices.
package entities

import (
	"context"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/entities"
	"github.com/LerianStudio/midaz-sdk-golang/models"
)

// CreateOrganization creates a new organization in the Midaz system.
//
// This function simplifies organization creation by handling the construction of the
// CreateOrganizationInput model and setting up appropriate fields and metadata.
// It demonstrates how to properly structure organization creation requests,
// including handling of optional pointer fields.
//
// Organizations are the top-level entities in the Midaz system and contain ledgers,
// which in turn contain accounts, assets, and transactions.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - service: The OrganizationsService instance to use for the API call.
//
// Returns:
//   - *models.Organization: The created organization if successful.
//   - error: Any error encountered during organization creation.
//
// Example:
//
//	organization, err := entities.CreateOrganization(
//	    ctx,
//	    sdkEntity.Organizations,
//	)
func CreateOrganization(ctx context.Context, service entities.OrganizationsService) (*models.Organization, error) {
	// Optional fields need to be pointers
	line2 := "Suite 100"
	dba := "Acme Corp"
	description := "Organization created"

	// Create input
	input := &models.CreateOrganizationInput{
		LegalName: "Acme Corporation",
		// Note: This is an API design choice - legalName is required and doingBusinessAs is optional
		DoingBusinessAs: dba,
		LegalDocument:   "123456789",
		Status: models.Status{
			Code:        "ACTIVE",
			Description: &description,
		},
		Address: models.Address{
			Line1:   "123 Main Street",
			Line2:   &line2,
			City:    "San Francisco",
			State:   "CA",
			ZipCode: "94105",
			Country: "US",
		},
		Metadata: map[string]any{
			"industry": "Technology",
			"size":     "Small",
		},
	}

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid organization input: %w", err)
	}

	// Create organization using the SDK
	return service.CreateOrganization(ctx, input)
}

// UpdateOrganization updates an existing organization.
//
// This function demonstrates how to update an organization's properties,
// specifically focusing on updating the legal name and metadata. It shows
// proper error handling and validation practices.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The organization ID to update. Must be a valid UUID.
//   - newName: The new legal name for the organization.
//   - service: The OrganizationsService instance to use for the API call.
//
// Returns:
//   - *models.Organization: The updated organization if successful.
//   - error: Any error encountered during the update operation.
//
// Example:
//
//	updatedOrg, err := entities.UpdateOrganization(
//	    ctx,
//	    "org-123",
//	    "New Company Name",
//	    sdkEntity.Organizations,
//	)
func UpdateOrganization(
	ctx context.Context,
	orgID, newName string,
	service entities.OrganizationsService,
) (*models.Organization, error) {
	// Create update input
	input := &models.UpdateOrganizationInput{
		LegalName: newName,
		Metadata: map[string]any{
			"industry": "Finance",
			"size":     "Medium",
			"updated":  true,
		},
	}

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid organization update input: %w", err)
	}

	// Update organization
	return service.UpdateOrganization(ctx, orgID, input)
}

// GetOrganization retrieves an organization by its ID.
//
// This function demonstrates how to retrieve an organization's details
// using its unique identifier. It shows proper error handling and
// service interaction.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The organization ID to retrieve. Must be a valid UUID.
//   - service: The OrganizationsService instance to use for the API call.
//
// Returns:
//   - *models.Organization: The retrieved organization if successful.
//   - error: Any error encountered during the retrieval operation.
//
// Example:
//
//	org, err := entities.GetOrganization(
//	    ctx,
//	    "org-123",
//	    sdkEntity.Organizations,
//	)
func GetOrganization(
	ctx context.Context,
	orgID string,
	service entities.OrganizationsService,
) (*models.Organization, error) {
	// Get organization
	return service.GetOrganization(ctx, orgID)
}

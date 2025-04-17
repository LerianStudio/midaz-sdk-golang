package workflows

import (
	"context"
	"fmt"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
)

// CreateSegments creates segments within a ledger
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//
// Returns:
//   - error: Any error encountered during the operation
func CreateSegments(ctx context.Context, client *client.Client, orgID, ledgerID string) error {
	fmt.Println("\n\n🔍 STEP 7: SEGMENT CREATION")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Println("\nCreating segments...")

	// Define segments to create
	segmentsToCreate := []struct {
		Name     string
		Metadata map[string]any
	}{
		{
			Name: "North America Region",
			Metadata: map[string]any{
				"regionCode": "NA",
				"countries":  "USA,Canada,Mexico", // String instead of array to comply with API validation
				"manager":    "John Smith",
			},
		},
		{
			Name: "Europe Region",
			Metadata: map[string]any{
				"regionCode": "EU",
				"countries":  "UK,France,Germany,Italy", // String instead of array to comply with API validation
				"manager":    "Jane Doe",
			},
		},
		{
			Name: "Asia Pacific Region",
			Metadata: map[string]any{
				"regionCode": "APAC",
				"countries":  "Japan,China,Australia,India", // String instead of array to comply with API validation
				"manager":    "David Lee",
			},
		},
	}

	// Create each segment
	for _, segmentInfo := range segmentsToCreate {
		// Create the segment input with metadata
		segmentInput := models.NewCreateSegmentInput(segmentInfo.Name).
			WithMetadata(segmentInfo.Metadata)

		// Attempt to create the segment
		// Note: portfolioID is passed for backward compatibility but not used by the API
		segment, err := client.Entity.Segments.CreateSegment(
			ctx, orgID, ledgerID, segmentInput,
		)

		if err != nil {
			fmt.Printf("❌ Failed to create segment '%s': %s\n", segmentInfo.Name, err.Error())
			return fmt.Errorf("failed to create segment '%s': %w", segmentInfo.Name, err)
		}

		fmt.Printf("✅ Segment created: %s\n", segment.Name)
		fmt.Printf("   ID: %s\n", segment.ID)
		fmt.Printf("   Region: %s\n", segment.Metadata["regionCode"])
		fmt.Printf("   Countries: %s\n", segment.Metadata["countries"])
		fmt.Printf("   Created: %s\n", segment.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	fmt.Println("\n✅ All segments created successfully")
	return nil
}

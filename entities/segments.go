package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/errors"
)

// SegmentsService defines the interface for segment-related operations.
// It provides methods to create, read, update, and delete segments
// within a portfolio, ledger, and organization.
type SegmentsService interface {
	// ListSegments retrieves a paginated list of segments for a ledger with optional filters.
	// The organizationID, ledgerID parameters specify which organization, ledger to query.
	// The opts parameter can be used to specify pagination, sorting, and filtering options.
	// Returns a ListResponse containing the segments and pagination information, or an error if the operation fails.
	ListSegments(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Segment], error)

	// GetSegment retrieves a specific segment by its ID.
	// The organizationID, ledgerID parameters specify which organization, ledger the segment belongs to.
	// The id parameter is the unique identifier of the segment to retrieve.
	// Returns the segment if found, or an error if the operation fails or the segment doesn't exist.
	GetSegment(ctx context.Context, organizationID, ledgerID, id string) (*models.Segment, error)

	// CreateSegment creates a new segment in the specified ledger.
	//
	// Segments allow for further categorization and grouping of accounts or other entities
	// within a portfolio, enabling more detailed reporting and management.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization where the segment will be created.
	//     Must be a valid organization ID.
	//   - ledgerID: The ID of the ledger where the segment will be created.
	//     Must be a valid ledger ID within the specified organization.
	//   - input: The segment details, including required fields:
	//     - Name: The human-readable name of the segment
	//     Optional fields include:
	//     - Status: The initial status (defaults to ACTIVE if not specified)
	//     - Metadata: Additional custom information about the segment
	//
	// Returns:
	//   - *models.Segment: The created segment if successful, containing the segment ID,
	//     name, and other properties.
	//   - error: An error if the operation fails. Possible errors include:
	//     - Invalid input (missing required fields or invalid values)
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Resource not found (invalid organization, ledger, or portfolio ID)
	//     - Network or server errors
	//
	// Example - Creating a basic segment:
	//
	//	// Create a simple segment with just required fields
	//	segment, err := segmentsService.CreateSegment(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    models.NewCreateSegmentInput("North America Region"),
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to create segment: %v", err)
	//	}
	//
	//	// Use the segment
	//	fmt.Printf("Segment created: %s (status: %s)\n",
	//	    segment.ID, segment.Status.Code)
	//
	// Example - Creating a segment with metadata:
	//
	//	// Create a segment with custom metadata
	//	input := models.NewCreateSegmentInput("EMEA Region").
	//	    WithStatus(models.StatusActive).
	//	    WithMetadata(map[string]any{
	//	        "regionCode": "EMEA",
	//	        "countries": []string{"UK", "France", "Germany", "Italy"},
	//	        "headquarters": "London",
	//	        "manager": "John Smith",
	//	    })
	//
	//	segment, err := segmentsService.CreateSegment(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    input,
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to create segment: %v", err)
	//	}
	//
	//	// Use the segment
	//	fmt.Printf("Segment created: %s\n", segment.ID)
	//	fmt.Printf("Name: %s\n", segment.Name)
	CreateSegment(ctx context.Context, organizationID, ledgerID string, input *models.CreateSegmentInput) (*models.Segment, error)

	// UpdateSegment updates an existing segment.
	// The organizationID, ledgerID parameters specify which organization, ledger the segment belongs to.
	// The id parameter is the unique identifier of the segment to update.
	// The input parameter contains the segment details to update, such as name, description, or status.
	// Returns the updated segment, or an error if the operation fails.
	UpdateSegment(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateSegmentInput) (*models.Segment, error)

	// DeleteSegment deletes a segment.
	// The organizationID, ledgerID parameters specify which organization, ledger the segment belongs to.
	// The id parameter is the unique identifier of the segment to delete.
	// Returns an error if the operation fails.
	DeleteSegment(ctx context.Context, organizationID, ledgerID, id string) error

	// GetSegmentsMetricsCount retrieves the count metrics for segments in a ledger.
	// The organizationID and ledgerID parameters specify which organization and ledger to get metrics for.
	// Returns the metrics count if successful, or an error if the operation fails.
	GetSegmentsMetricsCount(ctx context.Context, organizationID, ledgerID string) (*models.MetricsCount, error)
}

// segmentsEntity implements the SegmentsService interface.
// It provides methods for creating, retrieving, updating, and deleting segments.
type segmentsEntity struct {
	HTTPClient *HTTPClient
	baseURLs   map[string]string
}

// NewSegmentsEntity creates a new segments entity.
// It initializes the HTTP client and base URLs for API requests.
func NewSegmentsEntity(client *http.Client, authToken string, baseURLs map[string]string) SegmentsService {
	return &segmentsEntity{
		HTTPClient: NewHTTPClient(client, authToken, nil),
		baseURLs:   baseURLs,
	}
}

// ListSegments lists segments for a ledger with optional filters.
func (e *segmentsEntity) ListSegments(
	ctx context.Context,
	organizationID, ledgerID string,
	opts *models.ListOptions,
) (*models.ListResponse[models.Segment], error) {
	const operation = "ListSegments"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	url := e.buildURL(organizationID, ledgerID, "")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	// Add query parameters if provided
	if opts != nil {
		q := req.URL.Query()

		for key, value := range opts.ToQueryParams() {
			q.Add(key, value)
		}

		req.URL.RawQuery = q.Encode()
	}

	var response models.ListResponse[models.Segment]
	if err := e.HTTPClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetSegment gets a segment by ID.
func (e *segmentsEntity) GetSegment(
	ctx context.Context,
	organizationID, ledgerID, id string,
) (*models.Segment, error) {
	const operation = "GetSegment"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if id == "" {
		return nil, errors.NewMissingParameterError(operation, "id")
	}

	url := e.buildURL(organizationID, ledgerID, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var segment models.Segment
	if err := e.HTTPClient.sendRequest(req, &segment); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &segment, nil
}

// CreateSegment creates a new segment in the specified ledger.
func (e *segmentsEntity) CreateSegment(
	ctx context.Context,
	organizationID, ledgerID string,
	input *models.CreateSegmentInput,
) (*models.Segment, error) {
	const operation = "CreateSegment"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	// portfolioID is no longer required as segments are created directly under ledgers
	// We keep the parameter for backward compatibility but don't validate it

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	url := e.buildURL(organizationID, ledgerID, "")

	body, err := json.Marshal(input)

	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))

	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var segment models.Segment
	if err := e.HTTPClient.sendRequest(req, &segment); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &segment, nil
}

// UpdateSegment updates an existing segment.
func (e *segmentsEntity) UpdateSegment(
	ctx context.Context,
	organizationID, ledgerID, id string,
	input *models.UpdateSegmentInput,
) (*models.Segment, error) {
	const operation = "UpdateSegment"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if id == "" {
		return nil, errors.NewMissingParameterError(operation, "id")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	url := e.buildURL(organizationID, ledgerID, id)

	body, err := json.Marshal(input)

	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))

	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var segment models.Segment
	if err := e.HTTPClient.sendRequest(req, &segment); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &segment, nil
}

// DeleteSegment deletes a segment.
func (e *segmentsEntity) DeleteSegment(
	ctx context.Context,
	organizationID, ledgerID, id string,
) error {
	const operation = "DeleteSegment"

	if organizationID == "" {
		return errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return errors.NewMissingParameterError(operation, "ledgerID")
	}

	if id == "" {
		return errors.NewMissingParameterError(operation, "id")
	}

	url := e.buildURL(organizationID, ledgerID, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)

	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if err := e.HTTPClient.sendRequest(req, nil); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return err
	}

	return nil
}

// GetSegmentsMetricsCount gets the count metrics for segments in a ledger.
func (e *segmentsEntity) GetSegmentsMetricsCount(ctx context.Context, organizationID, ledgerID string) (*models.MetricsCount, error) {
	const operation = "GetSegmentsMetricsCount"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	url := e.buildMetricsURL(organizationID, ledgerID)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var metrics models.MetricsCount
	if err := e.HTTPClient.sendRequest(req, &metrics); err != nil {
		return nil, err
	}

	return &metrics, nil
}

// buildURL builds the URL for segments API calls.
func (e *segmentsEntity) buildURL(organizationID, ledgerID, segmentID string) string {
	baseURL := e.baseURLs["onboarding"]

	// Segments are directly under ledgers in the API, not under portfolios
	if segmentID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/segments", baseURL, organizationID, ledgerID)
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/segments/%s", baseURL, organizationID, ledgerID, segmentID)
}

// buildMetricsURL builds the URL for segments metrics API calls.
func (e *segmentsEntity) buildMetricsURL(organizationID, ledgerID string) string {
	baseURL := e.baseURLs["onboarding"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/segments/metrics/count", baseURL, organizationID, ledgerID)
}

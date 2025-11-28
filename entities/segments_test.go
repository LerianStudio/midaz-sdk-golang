package entities

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test constants for segments tests
const (
	testOrgID     = "org-123"
	testLedgerID  = "ledger-456"
	testSegmentID = "segment-789"
)

// newMockSegmentsHTTPClientAdapter creates a test HTTP client adapter for segments tests
func newMockSegmentsHTTPClientAdapter(mock *MockHTTPClient) *HTTPClient {
	retryOptions := retry.DefaultOptions()
	_ = retry.WithMaxRetries(1)(retryOptions)
	_ = retry.WithInitialDelay(1 * time.Millisecond)(retryOptions)
	_ = retry.WithMaxDelay(10 * time.Millisecond)(retryOptions)
	_ = retry.WithRetryableHTTPCodes(retry.DefaultRetryableHTTPCodes)(retryOptions)

	return &HTTPClient{
		client: &http.Client{
			Transport: &mockTransport{mock: mock},
		},
		retryOptions: retryOptions,
		jsonPool:     performance.NewJSONPool(),
	}
}

func TestNewSegmentsEntity(t *testing.T) {
	tests := []struct {
		name      string
		client    *http.Client
		authToken string
		baseURLs  map[string]string
	}{
		{
			name:      "with custom client",
			client:    &http.Client{Timeout: 30 * time.Second},
			authToken: "test-token-123",
			baseURLs:  map[string]string{"onboarding": "https://api.example.com"},
		},
		{
			name:      "with nil client",
			client:    nil,
			authToken: "another-token",
			baseURLs:  map[string]string{"onboarding": "https://api.test.com"},
		},
		{
			name:      "with empty auth token",
			client:    &http.Client{},
			authToken: "",
			baseURLs:  map[string]string{"onboarding": "https://api.example.com"},
		},
		{
			name:      "with multiple base URLs",
			client:    &http.Client{},
			authToken: "token",
			baseURLs: map[string]string{
				"onboarding":  "https://onboarding.example.com",
				"transaction": "https://transaction.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewSegmentsEntity(tt.client, tt.authToken, tt.baseURLs)
			require.NotNil(t, service)

			entity, ok := service.(*segmentsEntity)
			require.True(t, ok, "expected *segmentsEntity type")
			assert.NotNil(t, entity.HTTPClient)
			assert.Equal(t, tt.baseURLs, entity.baseURLs)
		})
	}
}

func TestSegmentsEntity_buildURL(t *testing.T) {
	entity := &segmentsEntity{
		baseURLs: map[string]string{"onboarding": "https://api.example.com"},
	}

	tests := []struct {
		name        string
		orgID       string
		ledgerID    string
		segmentID   string
		expectedURL string
	}{
		{
			name:        "list segments URL (no segment ID)",
			orgID:       "org-123",
			ledgerID:    "ledger-456",
			segmentID:   "",
			expectedURL: "https://api.example.com/organizations/org-123/ledgers/ledger-456/segments",
		},
		{
			name:        "single segment URL",
			orgID:       "org-abc",
			ledgerID:    "ledger-def",
			segmentID:   "segment-xyz",
			expectedURL: "https://api.example.com/organizations/org-abc/ledgers/ledger-def/segments/segment-xyz",
		},
		{
			name:        "with special characters in IDs",
			orgID:       "org_123",
			ledgerID:    "ledger_456",
			segmentID:   "segment_789",
			expectedURL: "https://api.example.com/organizations/org_123/ledgers/ledger_456/segments/segment_789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := entity.buildURL(tt.orgID, tt.ledgerID, tt.segmentID)
			assert.Equal(t, tt.expectedURL, url)
		})
	}
}

func TestSegmentsEntity_buildMetricsURL(t *testing.T) {
	entity := &segmentsEntity{
		baseURLs: map[string]string{"onboarding": "https://api.example.com"},
	}

	tests := []struct {
		name        string
		orgID       string
		ledgerID    string
		expectedURL string
	}{
		{
			name:        "metrics URL",
			orgID:       "org-123",
			ledgerID:    "ledger-456",
			expectedURL: "https://api.example.com/organizations/org-123/ledgers/ledger-456/segments/metrics/count",
		},
		{
			name:        "metrics URL with different IDs",
			orgID:       "org-abc",
			ledgerID:    "ledger-xyz",
			expectedURL: "https://api.example.com/organizations/org-abc/ledgers/ledger-xyz/segments/metrics/count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := entity.buildMetricsURL(tt.orgID, tt.ledgerID)
			assert.Equal(t, tt.expectedURL, url)
		})
	}
}

func TestSegmentsEntity_ListSegments(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		opts           *models.ListOptions
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		expectedItems  int
		errorContains  string
	}{
		{
			name:     "success with no options",
			orgID:    testOrgID,
			ledgerID: testLedgerID,
			opts:     nil,
			mockResponse: `{
				"items": [
					{"id": "seg-1", "name": "Segment 1", "organizationId": "org-123", "ledgerId": "ledger-456", "status": {"code": "ACTIVE"}},
					{"id": "seg-2", "name": "Segment 2", "organizationId": "org-123", "ledgerId": "ledger-456", "status": {"code": "ACTIVE"}}
				],
				"pagination": {"total": 2, "limit": 10, "offset": 0}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  2,
		},
		{
			name:     "success with pagination options",
			orgID:    testOrgID,
			ledgerID: testLedgerID,
			opts: &models.ListOptions{
				Limit:  5,
				Offset: 10,
			},
			mockResponse: `{
				"items": [
					{"id": "seg-11", "name": "Segment 11", "organizationId": "org-123", "ledgerId": "ledger-456", "status": {"code": "ACTIVE"}}
				],
				"pagination": {"total": 11, "limit": 5, "offset": 10}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  1,
		},
		{
			name:     "success with sorting and filtering",
			orgID:    testOrgID,
			ledgerID: testLedgerID,
			opts: &models.ListOptions{
				Limit:          10,
				OrderBy:        "name",
				OrderDirection: "asc",
				Filters:        map[string]string{"status": "ACTIVE"},
			},
			mockResponse: `{
				"items": [
					{"id": "seg-a", "name": "Alpha Segment", "organizationId": "org-123", "ledgerId": "ledger-456", "status": {"code": "ACTIVE"}},
					{"id": "seg-b", "name": "Beta Segment", "organizationId": "org-123", "ledgerId": "ledger-456", "status": {"code": "ACTIVE"}}
				],
				"pagination": {"total": 2, "limit": 10, "offset": 0}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  2,
		},
		{
			name:          "empty organization ID",
			orgID:         "",
			ledgerID:      testLedgerID,
			expectedError: true,
			errorContains: "organizationID",
		},
		{
			name:          "empty ledger ID",
			orgID:         testOrgID,
			ledgerID:      "",
			expectedError: true,
			errorContains: "ledgerID",
		},
		{
			name:           "server error 500",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:           "unauthorized error 401",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			mockStatusCode: http.StatusUnauthorized,
			mockResponse:   `{"error": "Unauthorized"}`,
			expectedError:  true,
		},
		{
			name:           "forbidden error 403",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			mockStatusCode: http.StatusForbidden,
			mockResponse:   `{"error": "Forbidden"}`,
			expectedError:  true,
		},
		{
			name:          "network error",
			orgID:         testOrgID,
			ledgerID:      testLedgerID,
			mockError:     errors.New("network connection failed"),
			expectedError: true,
		},
		{
			name:     "empty list response",
			orgID:    testOrgID,
			ledgerID: testLedgerID,
			mockResponse: `{
				"items": [],
				"pagination": {"total": 0, "limit": 10, "offset": 0}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}

					assert.Equal(t, http.MethodGet, req.Method)

					statusCode := tt.mockStatusCode
					if statusCode == 0 {
						statusCode = http.StatusOK
					}

					return &http.Response{
						StatusCode: statusCode,
						Body:       io.NopCloser(strings.NewReader(tt.mockResponse)),
					}, nil
				},
			}

			entity := &segmentsEntity{
				HTTPClient: newMockSegmentsHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.ListSegments(context.Background(), tt.orgID, tt.ledgerID, tt.opts)

			if tt.expectedError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Len(t, result.Items, tt.expectedItems)
		})
	}
}

func TestSegmentsEntity_ListSegments_QueryParams(t *testing.T) {
	var capturedURL string

	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			capturedURL = req.URL.String()

			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"items": [],
					"pagination": {"total": 0, "limit": 10, "offset": 0}
				}`)),
			}, nil
		},
	}

	entity := &segmentsEntity{
		HTTPClient: newMockSegmentsHTTPClientAdapter(mockClient),
		baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
	}

	opts := &models.ListOptions{
		Limit:          20,
		Offset:         5,
		OrderBy:        "createdAt",
		OrderDirection: "desc",
		Filters:        map[string]string{"status": "ACTIVE"},
	}

	_, err := entity.ListSegments(context.Background(), testOrgID, testLedgerID, opts)
	require.NoError(t, err)

	assert.Contains(t, capturedURL, "limit=20")
	assert.Contains(t, capturedURL, "offset=5")
	assert.Contains(t, capturedURL, "orderBy=createdAt")
	assert.Contains(t, capturedURL, "status=ACTIVE")
}

func TestSegmentsEntity_GetSegment(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		segmentID      string
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		errorContains  string
		expectedID     string
		expectedName   string
	}{
		{
			name:      "success",
			orgID:     testOrgID,
			ledgerID:  testLedgerID,
			segmentID: testSegmentID,
			mockResponse: `{
				"id": "segment-789",
				"name": "Test Segment",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"status": {"code": "ACTIVE"},
				"metadata": {"key": "value"},
				"createdAt": "2024-01-15T10:30:00Z",
				"updatedAt": "2024-01-15T10:30:00Z"
			}`,
			mockStatusCode: http.StatusOK,
			expectedID:     "segment-789",
			expectedName:   "Test Segment",
		},
		{
			name:          "empty organization ID",
			orgID:         "",
			ledgerID:      testLedgerID,
			segmentID:     testSegmentID,
			expectedError: true,
			errorContains: "organizationID",
		},
		{
			name:          "empty ledger ID",
			orgID:         testOrgID,
			ledgerID:      "",
			segmentID:     testSegmentID,
			expectedError: true,
			errorContains: "ledgerID",
		},
		{
			name:          "empty segment ID",
			orgID:         testOrgID,
			ledgerID:      testLedgerID,
			segmentID:     "",
			expectedError: true,
			errorContains: "id",
		},
		{
			name:           "not found 404",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      "non-existent",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Segment not found"}`,
			expectedError:  true,
		},
		{
			name:           "bad request 400",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      "invalid-id",
			mockStatusCode: http.StatusBadRequest,
			mockResponse:   `{"error": "Invalid segment ID format"}`,
			expectedError:  true,
		},
		{
			name:           "unauthorized 401",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			mockStatusCode: http.StatusUnauthorized,
			mockResponse:   `{"error": "Authentication required"}`,
			expectedError:  true,
		},
		{
			name:           "forbidden 403",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			mockStatusCode: http.StatusForbidden,
			mockResponse:   `{"error": "Access denied"}`,
			expectedError:  true,
		},
		{
			name:           "server error 500",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:          "network error",
			orgID:         testOrgID,
			ledgerID:      testLedgerID,
			segmentID:     testSegmentID,
			mockError:     errors.New("connection refused"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}

					assert.Equal(t, http.MethodGet, req.Method)

					statusCode := tt.mockStatusCode
					if statusCode == 0 {
						statusCode = http.StatusOK
					}

					return &http.Response{
						StatusCode: statusCode,
						Body:       io.NopCloser(strings.NewReader(tt.mockResponse)),
					}, nil
				},
			}

			entity := &segmentsEntity{
				HTTPClient: newMockSegmentsHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.GetSegment(context.Background(), tt.orgID, tt.ledgerID, tt.segmentID)

			if tt.expectedError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedID, result.ID)
			assert.Equal(t, tt.expectedName, result.Name)
		})
	}
}

func TestSegmentsEntity_CreateSegment(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		input          *models.CreateSegmentInput
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		errorContains  string
		expectedID     string
		expectedName   string
	}{
		{
			name:     "success with minimal input",
			orgID:    testOrgID,
			ledgerID: testLedgerID,
			input:    models.NewCreateSegmentInput("New Segment"),
			mockResponse: `{
				"id": "seg-new",
				"name": "New Segment",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"status": {"code": "ACTIVE"},
				"createdAt": "2024-01-15T10:30:00Z",
				"updatedAt": "2024-01-15T10:30:00Z"
			}`,
			mockStatusCode: http.StatusCreated,
			expectedID:     "seg-new",
			expectedName:   "New Segment",
		},
		{
			name:     "success with full input",
			orgID:    testOrgID,
			ledgerID: testLedgerID,
			input: models.NewCreateSegmentInput("Full Segment").
				WithStatus(models.NewStatus("ACTIVE")).
				WithMetadata(map[string]any{"region": "EMEA", "priority": "high"}),
			mockResponse: `{
				"id": "seg-full",
				"name": "Full Segment",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"status": {"code": "ACTIVE"},
				"metadata": {"region": "EMEA", "priority": "high"},
				"createdAt": "2024-01-15T10:30:00Z",
				"updatedAt": "2024-01-15T10:30:00Z"
			}`,
			mockStatusCode: http.StatusCreated,
			expectedID:     "seg-full",
			expectedName:   "Full Segment",
		},
		{
			name:          "empty organization ID",
			orgID:         "",
			ledgerID:      testLedgerID,
			input:         models.NewCreateSegmentInput("Test"),
			expectedError: true,
			errorContains: "organizationID",
		},
		{
			name:          "empty ledger ID",
			orgID:         testOrgID,
			ledgerID:      "",
			input:         models.NewCreateSegmentInput("Test"),
			expectedError: true,
			errorContains: "ledgerID",
		},
		{
			name:          "nil input",
			orgID:         testOrgID,
			ledgerID:      testLedgerID,
			input:         nil,
			expectedError: true,
			errorContains: "input",
		},
		{
			name:           "bad request 400",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			input:          models.NewCreateSegmentInput(""),
			mockStatusCode: http.StatusBadRequest,
			mockResponse:   `{"error": "Name is required"}`,
			expectedError:  true,
		},
		{
			name:           "conflict 409 - duplicate name",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			input:          models.NewCreateSegmentInput("Existing Segment"),
			mockStatusCode: http.StatusConflict,
			mockResponse:   `{"error": "Segment with this name already exists"}`,
			expectedError:  true,
		},
		{
			name:           "unauthorized 401",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			input:          models.NewCreateSegmentInput("Test"),
			mockStatusCode: http.StatusUnauthorized,
			mockResponse:   `{"error": "Authentication required"}`,
			expectedError:  true,
		},
		{
			name:           "forbidden 403",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			input:          models.NewCreateSegmentInput("Test"),
			mockStatusCode: http.StatusForbidden,
			mockResponse:   `{"error": "Insufficient permissions"}`,
			expectedError:  true,
		},
		{
			name:           "server error 500",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			input:          models.NewCreateSegmentInput("Test"),
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:          "network error",
			orgID:         testOrgID,
			ledgerID:      testLedgerID,
			input:         models.NewCreateSegmentInput("Test"),
			mockError:     errors.New("connection timeout"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := createSegmentEntityWithMock(t, tt.mockError, tt.mockStatusCode, tt.mockResponse, tt.input != nil)
			result, err := entity.CreateSegment(context.Background(), tt.orgID, tt.ledgerID, tt.input)
			assertSegmentResult(t, result, err, tt.expectedError, tt.errorContains, tt.expectedID, tt.expectedName)
		})
	}
}

// createSegmentEntityWithMock creates a segment entity with a configured mock HTTP client
func createSegmentEntityWithMock(t *testing.T, mockError error, statusCode int, response string, validateBody bool) *segmentsEntity {
	t.Helper()

	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if mockError != nil {
				return nil, mockError
			}

			assert.Equal(t, http.MethodPost, req.Method)

			if validateBody {
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)

				var inputData map[string]any

				err = json.Unmarshal(body, &inputData)
				require.NoError(t, err)
			}

			code := statusCode
			if code == 0 {
				code = http.StatusCreated
			}

			return &http.Response{
				StatusCode: code,
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		},
	}

	return &segmentsEntity{
		HTTPClient: newMockSegmentsHTTPClientAdapter(mockClient),
		baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
	}
}

// assertSegmentResult validates the result and error from segment operations
func assertSegmentResult(t *testing.T, result *models.Segment, err error, expectedError bool, errorContains, expectedID, expectedName string) {
	t.Helper()

	if expectedError {
		require.Error(t, err)

		if errorContains != "" {
			assert.Contains(t, err.Error(), errorContains)
		}

		return
	}

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, expectedID, result.ID)
	assert.Equal(t, expectedName, result.Name)
}

func TestSegmentsEntity_UpdateSegment(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		segmentID      string
		input          *models.UpdateSegmentInput
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		errorContains  string
		expectedID     string
		expectedName   string
	}{
		{
			name:      "success with name update",
			orgID:     testOrgID,
			ledgerID:  testLedgerID,
			segmentID: testSegmentID,
			input:     models.NewUpdateSegmentInput().WithName("Updated Segment Name"),
			mockResponse: `{
				"id": "segment-789",
				"name": "Updated Segment Name",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"status": {"code": "ACTIVE"},
				"createdAt": "2024-01-15T10:30:00Z",
				"updatedAt": "2024-01-15T11:00:00Z"
			}`,
			mockStatusCode: http.StatusOK,
			expectedID:     "segment-789",
			expectedName:   "Updated Segment Name",
		},
		{
			name:      "success with status update",
			orgID:     testOrgID,
			ledgerID:  testLedgerID,
			segmentID: testSegmentID,
			input:     models.NewUpdateSegmentInput().WithStatus(models.NewStatus("INACTIVE")),
			mockResponse: `{
				"id": "segment-789",
				"name": "Original Segment",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"status": {"code": "INACTIVE"},
				"createdAt": "2024-01-15T10:30:00Z",
				"updatedAt": "2024-01-15T11:00:00Z"
			}`,
			mockStatusCode: http.StatusOK,
			expectedID:     "segment-789",
			expectedName:   "Original Segment",
		},
		{
			name:      "success with full update",
			orgID:     testOrgID,
			ledgerID:  testLedgerID,
			segmentID: testSegmentID,
			input: models.NewUpdateSegmentInput().
				WithName("Fully Updated").
				WithStatus(models.NewStatus("ACTIVE")).
				WithMetadata(map[string]any{"updated": true}),
			mockResponse: `{
				"id": "segment-789",
				"name": "Fully Updated",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"status": {"code": "ACTIVE"},
				"metadata": {"updated": true},
				"createdAt": "2024-01-15T10:30:00Z",
				"updatedAt": "2024-01-15T11:00:00Z"
			}`,
			mockStatusCode: http.StatusOK,
			expectedID:     "segment-789",
			expectedName:   "Fully Updated",
		},
		{
			name:          "empty organization ID",
			orgID:         "",
			ledgerID:      testLedgerID,
			segmentID:     testSegmentID,
			input:         models.NewUpdateSegmentInput().WithName("Test"),
			expectedError: true,
			errorContains: "organizationID",
		},
		{
			name:          "empty ledger ID",
			orgID:         testOrgID,
			ledgerID:      "",
			segmentID:     testSegmentID,
			input:         models.NewUpdateSegmentInput().WithName("Test"),
			expectedError: true,
			errorContains: "ledgerID",
		},
		{
			name:          "empty segment ID",
			orgID:         testOrgID,
			ledgerID:      testLedgerID,
			segmentID:     "",
			input:         models.NewUpdateSegmentInput().WithName("Test"),
			expectedError: true,
			errorContains: "id",
		},
		{
			name:          "nil input",
			orgID:         testOrgID,
			ledgerID:      testLedgerID,
			segmentID:     testSegmentID,
			input:         nil,
			expectedError: true,
			errorContains: "input",
		},
		{
			name:           "not found 404",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      "non-existent",
			input:          models.NewUpdateSegmentInput().WithName("Test"),
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Segment not found"}`,
			expectedError:  true,
		},
		{
			name:           "bad request 400",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			input:          models.NewUpdateSegmentInput().WithName(""),
			mockStatusCode: http.StatusBadRequest,
			mockResponse:   `{"error": "Invalid update data"}`,
			expectedError:  true,
		},
		{
			name:           "unauthorized 401",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			input:          models.NewUpdateSegmentInput().WithName("Test"),
			mockStatusCode: http.StatusUnauthorized,
			mockResponse:   `{"error": "Authentication required"}`,
			expectedError:  true,
		},
		{
			name:           "forbidden 403",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			input:          models.NewUpdateSegmentInput().WithName("Test"),
			mockStatusCode: http.StatusForbidden,
			mockResponse:   `{"error": "Insufficient permissions"}`,
			expectedError:  true,
		},
		{
			name:           "server error 500",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			input:          models.NewUpdateSegmentInput().WithName("Test"),
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:          "network error",
			orgID:         testOrgID,
			ledgerID:      testLedgerID,
			segmentID:     testSegmentID,
			input:         models.NewUpdateSegmentInput().WithName("Test"),
			mockError:     errors.New("connection reset"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}

					assert.Equal(t, http.MethodPatch, req.Method)

					statusCode := tt.mockStatusCode
					if statusCode == 0 {
						statusCode = http.StatusOK
					}

					return &http.Response{
						StatusCode: statusCode,
						Body:       io.NopCloser(strings.NewReader(tt.mockResponse)),
					}, nil
				},
			}

			entity := &segmentsEntity{
				HTTPClient: newMockSegmentsHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.UpdateSegment(context.Background(), tt.orgID, tt.ledgerID, tt.segmentID, tt.input)

			if tt.expectedError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedID, result.ID)
			assert.Equal(t, tt.expectedName, result.Name)
		})
	}
}

func TestSegmentsEntity_DeleteSegment(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		segmentID      string
		mockStatusCode int
		mockResponse   string
		mockError      error
		expectedError  bool
		errorContains  string
	}{
		{
			name:           "success",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			mockStatusCode: http.StatusNoContent,
			mockResponse:   "",
		},
		{
			name:           "success with 200",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			mockStatusCode: http.StatusOK,
			mockResponse:   "",
		},
		{
			name:          "empty organization ID",
			orgID:         "",
			ledgerID:      testLedgerID,
			segmentID:     testSegmentID,
			expectedError: true,
			errorContains: "organizationID",
		},
		{
			name:          "empty ledger ID",
			orgID:         testOrgID,
			ledgerID:      "",
			segmentID:     testSegmentID,
			expectedError: true,
			errorContains: "ledgerID",
		},
		{
			name:          "empty segment ID",
			orgID:         testOrgID,
			ledgerID:      testLedgerID,
			segmentID:     "",
			expectedError: true,
			errorContains: "id",
		},
		{
			name:           "not found 404",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      "non-existent",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Segment not found"}`,
			expectedError:  true,
		},
		{
			name:           "conflict 409 - segment in use",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			mockStatusCode: http.StatusConflict,
			mockResponse:   `{"error": "Cannot delete segment that is in use"}`,
			expectedError:  true,
		},
		{
			name:           "unauthorized 401",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			mockStatusCode: http.StatusUnauthorized,
			mockResponse:   `{"error": "Authentication required"}`,
			expectedError:  true,
		},
		{
			name:           "forbidden 403",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			mockStatusCode: http.StatusForbidden,
			mockResponse:   `{"error": "Insufficient permissions"}`,
			expectedError:  true,
		},
		{
			name:           "server error 500",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			segmentID:      testSegmentID,
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:          "network error",
			orgID:         testOrgID,
			ledgerID:      testLedgerID,
			segmentID:     testSegmentID,
			mockError:     errors.New("network unavailable"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}

					assert.Equal(t, http.MethodDelete, req.Method)

					statusCode := tt.mockStatusCode
					if statusCode == 0 {
						statusCode = http.StatusNoContent
					}

					return &http.Response{
						StatusCode: statusCode,
						Body:       io.NopCloser(strings.NewReader(tt.mockResponse)),
					}, nil
				},
			}

			entity := &segmentsEntity{
				HTTPClient: newMockSegmentsHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			err := entity.DeleteSegment(context.Background(), tt.orgID, tt.ledgerID, tt.segmentID)

			if tt.expectedError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestSegmentsEntity_GetSegmentsMetricsCount(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		mockStatusCode int
		mockResponse   string
		mockError      error
		expectedError  bool
		errorContains  string
	}{
		{
			name:           "success",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			mockStatusCode: http.StatusOK,
			mockResponse:   `{"count": 42}`,
		},
		{
			name:          "empty organization ID",
			orgID:         "",
			ledgerID:      testLedgerID,
			expectedError: true,
			errorContains: "organizationID",
		},
		{
			name:          "empty ledger ID",
			orgID:         testOrgID,
			ledgerID:      "",
			expectedError: true,
			errorContains: "ledgerID",
		},
		{
			name:           "not found 404",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Ledger not found"}`,
			expectedError:  true,
		},
		{
			name:           "unauthorized 401",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			mockStatusCode: http.StatusUnauthorized,
			mockResponse:   `{"error": "Authentication required"}`,
			expectedError:  true,
		},
		{
			name:           "server error 500",
			orgID:          testOrgID,
			ledgerID:       testLedgerID,
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:          "network error",
			orgID:         testOrgID,
			ledgerID:      testLedgerID,
			mockError:     errors.New("dns lookup failed"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}

					assert.Equal(t, http.MethodHead, req.Method)

					statusCode := tt.mockStatusCode
					if statusCode == 0 {
						statusCode = http.StatusOK
					}

					return &http.Response{
						StatusCode: statusCode,
						Body:       io.NopCloser(strings.NewReader(tt.mockResponse)),
					}, nil
				},
			}

			entity := &segmentsEntity{
				HTTPClient: newMockSegmentsHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.GetSegmentsMetricsCount(context.Background(), tt.orgID, tt.ledgerID)

			if tt.expectedError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
		})
	}
}

// newValidationTestEntity creates a segment entity for validation testing
func newValidationTestEntity() *segmentsEntity {
	return &segmentsEntity{
		HTTPClient: newMockSegmentsHTTPClientAdapter(&MockHTTPClient{
			DoFunc: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{}`)),
				}, nil
			},
		}),
		baseURLs: map[string]string{"onboarding": "https://api.example.com"},
	}
}

func TestSegmentsEntity_ValidationEdgeCases_ListSegments(t *testing.T) {
	entity := newValidationTestEntity()
	ctx := context.Background()

	testCases := []struct {
		name          string
		orgID         string
		ledgerID      string
		errorContains string
	}{
		{"empty org", "", "ledger-123", "organizationID"},
		{"empty ledger", "org-123", "", "ledgerID"},
		{"both empty", "", "", "organizationID"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entity.ListSegments(ctx, tc.orgID, tc.ledgerID, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorContains)
		})
	}
}

func TestSegmentsEntity_ValidationEdgeCases_GetSegment(t *testing.T) {
	entity := newValidationTestEntity()
	ctx := context.Background()

	testCases := []struct {
		name          string
		orgID         string
		ledgerID      string
		segmentID     string
		errorContains string
	}{
		{"empty org", "", "ledger-123", "seg-123", "organizationID"},
		{"empty ledger", "org-123", "", "seg-123", "ledgerID"},
		{"empty segment", "org-123", "ledger-123", "", "id"},
		{"all empty", "", "", "", "organizationID"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entity.GetSegment(ctx, tc.orgID, tc.ledgerID, tc.segmentID)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorContains)
		})
	}
}

func TestSegmentsEntity_ValidationEdgeCases_CreateSegment(t *testing.T) {
	entity := newValidationTestEntity()
	ctx := context.Background()

	testCases := []struct {
		name          string
		orgID         string
		ledgerID      string
		input         *models.CreateSegmentInput
		errorContains string
	}{
		{"empty org", "", "ledger-123", models.NewCreateSegmentInput("Test"), "organizationID"},
		{"empty ledger", "org-123", "", models.NewCreateSegmentInput("Test"), "ledgerID"},
		{"nil input", "org-123", "ledger-123", nil, "input"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entity.CreateSegment(ctx, tc.orgID, tc.ledgerID, tc.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorContains)
		})
	}
}

func TestSegmentsEntity_ValidationEdgeCases_UpdateSegment(t *testing.T) {
	entity := newValidationTestEntity()
	ctx := context.Background()

	testCases := []struct {
		name          string
		orgID         string
		ledgerID      string
		segmentID     string
		input         *models.UpdateSegmentInput
		errorContains string
	}{
		{"empty org", "", "ledger-123", "seg-123", models.NewUpdateSegmentInput(), "organizationID"},
		{"empty ledger", "org-123", "", "seg-123", models.NewUpdateSegmentInput(), "ledgerID"},
		{"empty segment", "org-123", "ledger-123", "", models.NewUpdateSegmentInput(), "id"},
		{"nil input", "org-123", "ledger-123", "seg-123", nil, "input"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entity.UpdateSegment(ctx, tc.orgID, tc.ledgerID, tc.segmentID, tc.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorContains)
		})
	}
}

func TestSegmentsEntity_ValidationEdgeCases_DeleteSegment(t *testing.T) {
	entity := newValidationTestEntity()
	ctx := context.Background()

	testCases := []struct {
		name          string
		orgID         string
		ledgerID      string
		segmentID     string
		errorContains string
	}{
		{"empty org", "", "ledger-123", "seg-123", "organizationID"},
		{"empty ledger", "org-123", "", "seg-123", "ledgerID"},
		{"empty segment", "org-123", "ledger-123", "", "id"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := entity.DeleteSegment(ctx, tc.orgID, tc.ledgerID, tc.segmentID)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorContains)
		})
	}
}

func TestSegmentsEntity_ValidationEdgeCases_GetSegmentsMetricsCount(t *testing.T) {
	entity := newValidationTestEntity()
	ctx := context.Background()

	testCases := []struct {
		name          string
		orgID         string
		ledgerID      string
		errorContains string
	}{
		{"empty org", "", "ledger-123", "organizationID"},
		{"empty ledger", "org-123", "", "ledgerID"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entity.GetSegmentsMetricsCount(ctx, tc.orgID, tc.ledgerID)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorContains)
		})
	}
}

func TestSegmentsEntity_IntegrationWithHTTPTestServer(t *testing.T) {
	t.Run("ListSegments with real HTTP server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/organizations/org-123/ledgers/ledger-456/segments")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"items": [
					{"id": "seg-1", "name": "Segment 1", "status": {"code": "ACTIVE"}},
					{"id": "seg-2", "name": "Segment 2", "status": {"code": "INACTIVE"}}
				],
				"pagination": {"total": 2, "limit": 10, "offset": 0}
			}`))
		}))
		defer server.Close()

		entity := NewSegmentsEntity(server.Client(), "test-token", map[string]string{
			"onboarding": server.URL,
		})

		result, err := entity.ListSegments(context.Background(), "org-123", "ledger-456", nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Items, 2)
		assert.Equal(t, "seg-1", result.Items[0].ID)
		assert.Equal(t, "Segment 1", result.Items[0].Name)
	})

	t.Run("GetSegment with real HTTP server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/segments/seg-123")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "seg-123",
				"name": "Test Segment",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"status": {"code": "ACTIVE"},
				"metadata": {"environment": "production"}
			}`))
		}))
		defer server.Close()

		entity := NewSegmentsEntity(server.Client(), "test-token", map[string]string{
			"onboarding": server.URL,
		})

		result, err := entity.GetSegment(context.Background(), "org-123", "ledger-456", "seg-123")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "seg-123", result.ID)
		assert.Equal(t, "Test Segment", result.Name)
		assert.Equal(t, "ACTIVE", result.Status.Code)
	})

	t.Run("CreateSegment with real HTTP server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/organizations/org-123/ledgers/ledger-456/segments")

			// Read and validate request body
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)

			var input map[string]any

			err = json.Unmarshal(body, &input)
			assert.NoError(t, err)
			assert.Equal(t, "New Segment", input["name"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"id": "seg-new",
				"name": "New Segment",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"status": {"code": "ACTIVE"}
			}`))
		}))
		defer server.Close()

		entity := NewSegmentsEntity(server.Client(), "test-token", map[string]string{
			"onboarding": server.URL,
		})

		input := models.NewCreateSegmentInput("New Segment")
		result, err := entity.CreateSegment(context.Background(), "org-123", "ledger-456", input)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "seg-new", result.ID)
		assert.Equal(t, "New Segment", result.Name)
	})

	t.Run("UpdateSegment with real HTTP server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPatch, r.Method)
			assert.Contains(t, r.URL.Path, "/segments/seg-123")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "seg-123",
				"name": "Updated Segment",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"status": {"code": "ACTIVE"}
			}`))
		}))
		defer server.Close()

		entity := NewSegmentsEntity(server.Client(), "test-token", map[string]string{
			"onboarding": server.URL,
		})

		input := models.NewUpdateSegmentInput().WithName("Updated Segment")
		result, err := entity.UpdateSegment(context.Background(), "org-123", "ledger-456", "seg-123", input)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "seg-123", result.ID)
		assert.Equal(t, "Updated Segment", result.Name)
	})

	t.Run("DeleteSegment with real HTTP server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Contains(t, r.URL.Path, "/segments/seg-123")
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		entity := NewSegmentsEntity(server.Client(), "test-token", map[string]string{
			"onboarding": server.URL,
		})

		err := entity.DeleteSegment(context.Background(), "org-123", "ledger-456", "seg-123")
		require.NoError(t, err)
	})

	t.Run("HTTP error responses", func(t *testing.T) {
		errorCodes := []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusNotFound,
			http.StatusInternalServerError,
			http.StatusServiceUnavailable,
		}

		for _, statusCode := range errorCodes {
			t.Run(http.StatusText(statusCode), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(statusCode)
					_, _ = w.Write([]byte(`{"error": "` + http.StatusText(statusCode) + `"}`))
				}))
				defer server.Close()

				entity := NewSegmentsEntity(server.Client(), "test-token", map[string]string{
					"onboarding": server.URL,
				})

				_, err := entity.GetSegment(context.Background(), "org-123", "ledger-456", "seg-123")
				require.Error(t, err)
			})
		}
	})
}

func TestSegmentsEntity_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "seg-1", "name": "Test"}`))
	}))
	defer server.Close()

	entity := NewSegmentsEntity(server.Client(), "test-token", map[string]string{
		"onboarding": server.URL,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := entity.GetSegment(ctx, "org-123", "ledger-456", "seg-123")
	require.Error(t, err)
}

func TestSegmentsEntity_RequestURLConstruction(t *testing.T) {
	var capturedRequests []struct {
		Method string
		URL    string
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequests = append(capturedRequests, struct {
			Method string
			URL    string
		}{
			Method: r.Method,
			URL:    r.URL.String(),
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items": [], "pagination": {"total": 0, "limit": 10, "offset": 0}}`))
	}))
	defer server.Close()

	entity := NewSegmentsEntity(server.Client(), "test-token", map[string]string{
		"onboarding": server.URL,
	})

	ctx := context.Background()

	// Test ListSegments URL
	_, _ = entity.ListSegments(ctx, "org-abc", "ledger-xyz", nil)

	require.Len(t, capturedRequests, 1)
	assert.Equal(t, http.MethodGet, capturedRequests[0].Method)
	assert.Contains(t, capturedRequests[0].URL, "/organizations/org-abc/ledgers/ledger-xyz/segments")

	// Reset and test with options
	capturedRequests = nil
	_, _ = entity.ListSegments(ctx, "org-123", "ledger-456", &models.ListOptions{
		Limit:  25,
		Offset: 50,
	})

	require.Len(t, capturedRequests, 1)
	assert.Contains(t, capturedRequests[0].URL, "limit=25")
	assert.Contains(t, capturedRequests[0].URL, "offset=50")
}

func TestSegmentsEntity_ResponseParsing(t *testing.T) {
	t.Run("segment with all fields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "seg-full",
				"name": "Full Segment",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"status": {
					"code": "ACTIVE",
					"description": "Segment is active and operational"
				},
				"metadata": {
					"stringKey": "stringValue",
					"numberKey": 42,
					"boolKey": true,
					"arrayKey": ["a", "b", "c"],
					"objectKey": {"nested": "value"}
				},
				"createdAt": "2024-01-15T10:30:00Z",
				"updatedAt": "2024-01-15T11:00:00Z"
			}`))
		}))
		defer server.Close()

		entity := NewSegmentsEntity(server.Client(), "test-token", map[string]string{
			"onboarding": server.URL,
		})

		result, err := entity.GetSegment(context.Background(), "org-123", "ledger-456", "seg-full")
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "seg-full", result.ID)
		assert.Equal(t, "Full Segment", result.Name)
		assert.Equal(t, "org-123", result.OrganizationID)
		assert.Equal(t, "ledger-456", result.LedgerID)
		assert.Equal(t, "ACTIVE", result.Status.Code)

		// Verify metadata
		require.NotNil(t, result.Metadata)
		assert.Equal(t, "stringValue", result.Metadata["stringKey"])
		assert.InDelta(t, float64(42), result.Metadata["numberKey"], 0.001)
		assert.Equal(t, true, result.Metadata["boolKey"])
	})

	t.Run("segment with minimal fields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "seg-min",
				"name": "Minimal Segment",
				"status": {"code": "ACTIVE"}
			}`))
		}))
		defer server.Close()

		entity := NewSegmentsEntity(server.Client(), "test-token", map[string]string{
			"onboarding": server.URL,
		})

		result, err := entity.GetSegment(context.Background(), "org-123", "ledger-456", "seg-min")
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "seg-min", result.ID)
		assert.Equal(t, "Minimal Segment", result.Name)
	})

	t.Run("list with pagination info", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"items": [
					{"id": "seg-1", "name": "Segment 1", "status": {"code": "ACTIVE"}},
					{"id": "seg-2", "name": "Segment 2", "status": {"code": "ACTIVE"}}
				],
				"pagination": {
					"total": 100,
					"limit": 2,
					"offset": 10,
					"prevCursor": "prev-cursor-value",
					"nextCursor": "next-cursor-value"
				}
			}`))
		}))
		defer server.Close()

		entity := NewSegmentsEntity(server.Client(), "test-token", map[string]string{
			"onboarding": server.URL,
		})

		result, err := entity.ListSegments(context.Background(), "org-123", "ledger-456", nil)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Len(t, result.Items, 2)
		assert.Equal(t, 100, result.Pagination.Total)
		assert.Equal(t, 2, result.Pagination.Limit)
		assert.Equal(t, 10, result.Pagination.Offset)
	})
}

func TestSegmentsEntity_ConcurrentRequests(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "seg-` + string(rune('0'+requestCount)) + `", "name": "Segment", "status": {"code": "ACTIVE"}}`))
	}))
	defer server.Close()

	entity := NewSegmentsEntity(server.Client(), "test-token", map[string]string{
		"onboarding": server.URL,
	})

	ctx := context.Background()
	done := make(chan struct{}, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			_, err := entity.GetSegment(ctx, "org-123", "ledger-456", "seg-"+string(rune('0'+idx)))
			assert.NoError(t, err)

			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

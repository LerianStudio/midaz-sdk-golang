package entities

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAssetRatesEntity(t *testing.T) {
	tests := []struct {
		name      string
		client    *http.Client
		authToken string
		baseURLs  map[string]string
	}{
		{
			name:      "with custom client",
			client:    &http.Client{Timeout: 30 * time.Second},
			authToken: "test-token",
			baseURLs:  map[string]string{"transaction": "https://api.example.com"},
		},
		{
			name:      "with nil client",
			client:    nil,
			authToken: "another-token",
			baseURLs:  map[string]string{"transaction": "https://api.example.com/v1"},
		},
		{
			name:      "with empty auth token",
			client:    &http.Client{},
			authToken: "",
			baseURLs:  map[string]string{"transaction": "https://localhost:8080"},
		},
		{
			name:      "with multiple base URLs",
			client:    &http.Client{},
			authToken: "token",
			baseURLs: map[string]string{
				"transaction": "https://transaction.api.example.com",
				"onboarding":  "https://onboarding.api.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewAssetRatesEntity(tt.client, tt.authToken, tt.baseURLs)
			assert.NotNil(t, service)

			entity, ok := service.(*assetRatesEntity)
			assert.True(t, ok)
			assert.NotNil(t, entity.httpClient)
			assert.Equal(t, tt.baseURLs, entity.baseURLs)
		})
	}
}

func TestNewAssetRatesEntity_DebugMode(t *testing.T) {
	t.Run("with MIDAZ_DEBUG=true", func(t *testing.T) {
		t.Setenv("MIDAZ_DEBUG", "true")

		service := NewAssetRatesEntity(
			&http.Client{},
			"test-token",
			map[string]string{"transaction": "https://api.example.com"},
		)
		assert.NotNil(t, service)

		entity, ok := service.(*assetRatesEntity)
		assert.True(t, ok)
		assert.NotNil(t, entity.httpClient)
		assert.True(t, entity.httpClient.debug)
	})

	t.Run("with MIDAZ_DEBUG=false", func(t *testing.T) {
		t.Setenv("MIDAZ_DEBUG", "false")

		service := NewAssetRatesEntity(
			&http.Client{},
			"test-token",
			map[string]string{"transaction": "https://api.example.com"},
		)
		assert.NotNil(t, service)

		entity, ok := service.(*assetRatesEntity)
		assert.True(t, ok)
		assert.NotNil(t, entity.httpClient)
		assert.False(t, entity.httpClient.debug)
	})

	t.Run("without MIDAZ_DEBUG env var", func(t *testing.T) {
		service := NewAssetRatesEntity(
			&http.Client{},
			"test-token",
			map[string]string{"transaction": "https://api.example.com"},
		)
		assert.NotNil(t, service)

		entity, ok := service.(*assetRatesEntity)
		assert.True(t, ok)
		assert.NotNil(t, entity.httpClient)
		assert.False(t, entity.httpClient.debug)
	})
}

func TestAssetRatesEntity_buildURL(t *testing.T) {
	entity := &assetRatesEntity{
		baseURLs: map[string]string{"transaction": "https://api.example.com"},
	}

	tests := []struct {
		name           string
		organizationID string
		ledgerID       string
		externalID     string
		expectedURL    string
	}{
		{
			name:           "without external ID",
			organizationID: "org-123",
			ledgerID:       "ledger-456",
			externalID:     "",
			expectedURL:    "https://api.example.com/organizations/org-123/ledgers/ledger-456/asset-rates",
		},
		{
			name:           "with external ID",
			organizationID: "org-789",
			ledgerID:       "ledger-012",
			externalID:     "ext-345",
			expectedURL:    "https://api.example.com/organizations/org-789/ledgers/ledger-012/asset-rates/ext-345",
		},
		{
			name:           "with special characters in IDs",
			organizationID: "org-abc-123",
			ledgerID:       "ledger-xyz-789",
			externalID:     "ext-id-456",
			expectedURL:    "https://api.example.com/organizations/org-abc-123/ledgers/ledger-xyz-789/asset-rates/ext-id-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := entity.buildURL(tt.organizationID, tt.ledgerID, tt.externalID)
			assert.Equal(t, tt.expectedURL, url)
		})
	}
}

func TestAssetRatesEntity_buildFromAssetURL(t *testing.T) {
	entity := &assetRatesEntity{
		baseURLs: map[string]string{"transaction": "https://api.example.com"},
	}

	tests := []struct {
		name           string
		organizationID string
		ledgerID       string
		assetCode      string
		expectedURL    string
	}{
		{
			name:           "with USD asset code",
			organizationID: "org-123",
			ledgerID:       "ledger-456",
			assetCode:      "USD",
			expectedURL:    "https://api.example.com/organizations/org-123/ledgers/ledger-456/asset-rates/from/USD",
		},
		{
			name:           "with BRL asset code",
			organizationID: "org-789",
			ledgerID:       "ledger-012",
			assetCode:      "BRL",
			expectedURL:    "https://api.example.com/organizations/org-789/ledgers/ledger-012/asset-rates/from/BRL",
		},
		{
			name:           "with lowercase asset code",
			organizationID: "org-abc",
			ledgerID:       "ledger-xyz",
			assetCode:      "eur",
			expectedURL:    "https://api.example.com/organizations/org-abc/ledgers/ledger-xyz/asset-rates/from/eur",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := entity.buildFromAssetURL(tt.organizationID, tt.ledgerID, tt.assetCode)
			assert.Equal(t, tt.expectedURL, url)
		})
	}
}

func TestAssetRatesEntity_CreateOrUpdateAssetRate(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		input          *models.CreateAssetRateInput
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		errorContains  string
	}{
		{
			name:     "success with minimal input",
			orgID:    "org-123",
			ledgerID: "ledger-456",
			input:    models.NewCreateAssetRateInput("USD", "BRL", 500),
			mockResponse: `{
				"id": "rate-123",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"externalId": "ext-001",
				"from": "USD",
				"to": "BRL",
				"rate": 5.00,
				"scale": 2,
				"source": "Central Bank",
				"ttl": 3600,
				"createdAt": "2024-01-01T00:00:00Z",
				"updatedAt": "2024-01-01T00:00:00Z"
			}`,
			mockStatusCode: http.StatusOK,
		},
		{
			name:     "success with all fields",
			orgID:    "org-123",
			ledgerID: "ledger-456",
			input: models.NewCreateAssetRateInput("USD", "EUR", 92).
				WithScale(2).
				WithSource("ECB").
				WithTTL(7200).
				WithExternalID("ext-rate-001").
				WithMetadata(map[string]any{"provider": "forex"}),
			mockResponse: `{
				"id": "rate-456",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"externalId": "ext-rate-001",
				"from": "USD",
				"to": "EUR",
				"rate": 0.92,
				"scale": 2,
				"source": "ECB",
				"ttl": 7200,
				"createdAt": "2024-01-01T00:00:00Z",
				"updatedAt": "2024-01-01T00:00:00Z",
				"metadata": {"provider": "forex"}
			}`,
			mockStatusCode: http.StatusOK,
		},
		{
			name:          "empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-456",
			input:         models.NewCreateAssetRateInput("USD", "BRL", 500),
			expectedError: true,
			errorContains: "organizationID",
		},
		{
			name:          "empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			input:         models.NewCreateAssetRateInput("USD", "BRL", 500),
			expectedError: true,
			errorContains: "ledgerID",
		},
		{
			name:          "nil input",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			input:         nil,
			expectedError: true,
			errorContains: "input",
		},
		{
			name:          "invalid input - empty from asset",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			input:         models.NewCreateAssetRateInput("", "BRL", 500),
			expectedError: true,
			errorContains: "from asset code",
		},
		{
			name:          "invalid input - empty to asset",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			input:         models.NewCreateAssetRateInput("USD", "", 500),
			expectedError: true,
			errorContains: "to asset code",
		},
		{
			name:          "invalid input - zero rate",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			input:         models.NewCreateAssetRateInput("USD", "BRL", 0),
			expectedError: true,
			errorContains: "rate must be greater than zero",
		},
		{
			name:          "invalid input - negative rate",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			input:         models.NewCreateAssetRateInput("USD", "BRL", -100),
			expectedError: true,
			errorContains: "rate must be greater than zero",
		},
		{
			name:           "HTTP 400 bad request",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			input:          models.NewCreateAssetRateInput("USD", "BRL", 500),
			mockStatusCode: http.StatusBadRequest,
			mockResponse:   `{"error": "Invalid asset code format"}`,
			expectedError:  true,
		},
		{
			name:           "HTTP 401 unauthorized",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			input:          models.NewCreateAssetRateInput("USD", "BRL", 500),
			mockStatusCode: http.StatusUnauthorized,
			mockResponse:   `{"error": "Invalid or expired token"}`,
			expectedError:  true,
		},
		{
			name:           "HTTP 403 forbidden",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			input:          models.NewCreateAssetRateInput("USD", "BRL", 500),
			mockStatusCode: http.StatusForbidden,
			mockResponse:   `{"error": "Access denied"}`,
			expectedError:  true,
		},
		{
			name:           "HTTP 404 not found",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			input:          models.NewCreateAssetRateInput("USD", "BRL", 500),
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Ledger not found"}`,
			expectedError:  true,
		},
		{
			name:           "HTTP 500 internal server error",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			input:          models.NewCreateAssetRateInput("USD", "BRL", 500),
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client connection error",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			input:         models.NewCreateAssetRateInput("USD", "BRL", 500),
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

					assert.Equal(t, http.MethodPut, req.Method)

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

			entity := &assetRatesEntity{
				httpClient: newHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"transaction": "https://api.example.com"},
			}

			result, err := entity.CreateOrUpdateAssetRate(context.Background(), tt.orgID, tt.ledgerID, tt.input)

			if tt.expectedError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotEmpty(t, result.ID)
			assert.Equal(t, tt.orgID, result.OrganizationID)
			assert.Equal(t, tt.ledgerID, result.LedgerID)
		})
	}
}

func TestAssetRatesEntity_GetAssetRate(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		externalID     string
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		errorContains  string
	}{
		{
			name:       "success",
			orgID:      "org-123",
			ledgerID:   "ledger-456",
			externalID: "ext-789",
			mockResponse: `{
				"id": "rate-123",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"externalId": "ext-789",
				"from": "USD",
				"to": "BRL",
				"rate": 5.00,
				"scale": 2,
				"source": "Central Bank",
				"ttl": 3600,
				"createdAt": "2024-01-01T00:00:00Z",
				"updatedAt": "2024-01-01T00:00:00Z"
			}`,
			mockStatusCode: http.StatusOK,
		},
		{
			name:          "empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-456",
			externalID:    "ext-789",
			expectedError: true,
			errorContains: "organizationID",
		},
		{
			name:          "empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			externalID:    "ext-789",
			expectedError: true,
			errorContains: "ledgerID",
		},
		{
			name:          "empty external ID",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			externalID:    "",
			expectedError: true,
			errorContains: "externalID",
		},
		{
			name:           "HTTP 400 bad request",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			externalID:     "ext-789",
			mockStatusCode: http.StatusBadRequest,
			mockResponse:   `{"error": "Invalid external ID format"}`,
			expectedError:  true,
		},
		{
			name:           "HTTP 401 unauthorized",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			externalID:     "ext-789",
			mockStatusCode: http.StatusUnauthorized,
			mockResponse:   `{"error": "Invalid or expired token"}`,
			expectedError:  true,
		},
		{
			name:           "HTTP 403 forbidden",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			externalID:     "ext-789",
			mockStatusCode: http.StatusForbidden,
			mockResponse:   `{"error": "Access denied"}`,
			expectedError:  true,
		},
		{
			name:           "HTTP 404 not found",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			externalID:     "not-found-id",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Asset rate not found"}`,
			expectedError:  true,
		},
		{
			name:           "HTTP 500 internal server error",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			externalID:     "ext-789",
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client connection error",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			externalID:    "ext-789",
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

			entity := &assetRatesEntity{
				httpClient: newHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"transaction": "https://api.example.com"},
			}

			result, err := entity.GetAssetRate(context.Background(), tt.orgID, tt.ledgerID, tt.externalID)

			if tt.expectedError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotEmpty(t, result.ID)
			assert.Equal(t, tt.externalID, result.ExternalID)
			assert.Equal(t, "USD", result.From)
			assert.Equal(t, "BRL", result.To)
		})
	}
}

func TestAssetRatesEntity_ListAssetRatesByAssetCode(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		assetCode      string
		opts           *models.AssetRateListOptions
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		errorContains  string
		expectedItems  int
		checkRequest   func(t *testing.T, req *http.Request)
	}{
		{
			name:      "success with no options",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			assetCode: "USD",
			opts:      nil,
			mockResponse: `{
				"items": [
					{
						"id": "rate-1",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"from": "USD",
						"to": "BRL",
						"rate": 5.00,
						"createdAt": "2024-01-01T00:00:00Z",
						"updatedAt": "2024-01-01T00:00:00Z"
					},
					{
						"id": "rate-2",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"from": "USD",
						"to": "EUR",
						"rate": 0.92,
						"createdAt": "2024-01-01T00:00:00Z",
						"updatedAt": "2024-01-01T00:00:00Z"
					}
				],
				"limit": 10
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  2,
		},
		{
			name:      "success with all options",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			assetCode: "USD",
			opts: models.NewAssetRateListOptions().
				WithTo("BRL", "EUR").
				WithLimit(5).
				WithDateRange("2024-01-01", "2024-12-31").
				WithSortOrder("desc").
				WithCursor("cursor-abc"),
			mockResponse: `{
				"items": [
					{
						"id": "rate-1",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"from": "USD",
						"to": "BRL",
						"rate": 5.00,
						"createdAt": "2024-01-01T00:00:00Z",
						"updatedAt": "2024-01-01T00:00:00Z"
					}
				],
				"limit": 5,
				"next_cursor": "cursor-xyz"
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  1,
			checkRequest: func(t *testing.T, req *http.Request) {
				t.Helper()

				query := req.URL.Query()
				assert.Equal(t, "BRL,EUR", query.Get("to"))
				assert.Equal(t, "5", query.Get("limit"))
				assert.Equal(t, "2024-01-01", query.Get("start_date"))
				assert.Equal(t, "2024-12-31", query.Get("end_date"))
				assert.Equal(t, "desc", query.Get("sort_order"))
				assert.Equal(t, "cursor-abc", query.Get("cursor"))
			},
		},
		{
			name:      "success with pagination cursor",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			assetCode: "USD",
			opts:      models.NewAssetRateListOptions().WithCursor("next-page-cursor"),
			mockResponse: `{
				"items": [
					{
						"id": "rate-3",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"from": "USD",
						"to": "GBP",
						"rate": 0.79,
						"createdAt": "2024-01-01T00:00:00Z",
						"updatedAt": "2024-01-01T00:00:00Z"
					}
				],
				"limit": 10,
				"prev_cursor": "prev-page-cursor"
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  1,
		},
		{
			name:          "empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-456",
			assetCode:     "USD",
			opts:          nil,
			expectedError: true,
			errorContains: "organizationID",
		},
		{
			name:          "empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			assetCode:     "USD",
			opts:          nil,
			expectedError: true,
			errorContains: "ledgerID",
		},
		{
			name:          "empty asset code",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			assetCode:     "",
			opts:          nil,
			expectedError: true,
			errorContains: "assetCode",
		},
		{
			name:           "HTTP 400 bad request",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			assetCode:      "INVALID",
			opts:           nil,
			mockStatusCode: http.StatusBadRequest,
			mockResponse:   `{"error": "Invalid asset code"}`,
			expectedError:  true,
		},
		{
			name:           "HTTP 401 unauthorized",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			assetCode:      "USD",
			opts:           nil,
			mockStatusCode: http.StatusUnauthorized,
			mockResponse:   `{"error": "Invalid or expired token"}`,
			expectedError:  true,
		},
		{
			name:           "HTTP 403 forbidden",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			assetCode:      "USD",
			opts:           nil,
			mockStatusCode: http.StatusForbidden,
			mockResponse:   `{"error": "Access denied"}`,
			expectedError:  true,
		},
		{
			name:           "HTTP 404 ledger not found",
			orgID:          "org-123",
			ledgerID:       "not-found-ledger",
			assetCode:      "USD",
			opts:           nil,
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Ledger not found"}`,
			expectedError:  true,
		},
		{
			name:           "HTTP 500 internal server error",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			assetCode:      "USD",
			opts:           nil,
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client connection error",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			assetCode:     "USD",
			opts:          nil,
			mockError:     errors.New("connection refused"),
			expectedError: true,
		},
		{
			name:      "empty items response",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			assetCode: "XYZ",
			opts:      nil,
			mockResponse: `{
				"items": [],
				"limit": 10
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runListAssetRatesByAssetCodeTest(t, tt)
		})
	}
}

// runListAssetRatesByAssetCodeTest executes a single test case for ListAssetRatesByAssetCode.
func runListAssetRatesByAssetCodeTest(t *testing.T, tt struct {
	name           string
	orgID          string
	ledgerID       string
	assetCode      string
	opts           *models.AssetRateListOptions
	mockResponse   string
	mockStatusCode int
	mockError      error
	expectedError  bool
	errorContains  string
	expectedItems  int
	checkRequest   func(t *testing.T, req *http.Request)
},
) {
	t.Helper()

	mockClient := createListAssetRatesMockClient(t, tt.mockError, tt.mockStatusCode, tt.mockResponse, tt.checkRequest)
	entity := &assetRatesEntity{
		httpClient: newHTTPClientAdapter(mockClient),
		baseURLs:   map[string]string{"transaction": "https://api.example.com"},
	}

	result, err := entity.ListAssetRatesByAssetCode(context.Background(), tt.orgID, tt.ledgerID, tt.assetCode, tt.opts)

	assertListAssetRatesResult(t, tt.expectedError, tt.errorContains, tt.expectedItems, result, err)
}

// createListAssetRatesMockClient creates a mock HTTP client for list asset rates testing.
func createListAssetRatesMockClient(t *testing.T, mockError error, mockStatusCode int, mockResponse string, checkRequest func(t *testing.T, req *http.Request)) *MockHTTPClient {
	t.Helper()

	return &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if mockError != nil {
				return nil, mockError
			}

			assert.Equal(t, http.MethodGet, req.Method)

			if checkRequest != nil {
				checkRequest(t, req)
			}

			statusCode := mockStatusCode
			if statusCode == 0 {
				statusCode = http.StatusOK
			}

			return &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(strings.NewReader(mockResponse)),
			}, nil
		},
	}
}

// assertListAssetRatesResult asserts the expected result of listing asset rates.
func assertListAssetRatesResult(t *testing.T, expectedError bool, errorContains string, expectedItems int, result *models.AssetRatesResponse, err error) {
	t.Helper()

	if expectedError {
		require.Error(t, err)

		if errorContains != "" {
			assert.Contains(t, err.Error(), errorContains)
		}

		return
	}

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Items, expectedItems)
}

func TestAssetRatesEntity_IntegrationWithHTTPTestServer(t *testing.T) {
	t.Run("CreateOrUpdateAssetRate with real HTTP server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Equal(t, "/organizations/org-123/ledgers/ledger-456/asset-rates", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.NotEmpty(t, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "rate-123",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"from": "USD",
				"to": "BRL",
				"rate": 5.00,
				"createdAt": "2024-01-01T00:00:00Z",
				"updatedAt": "2024-01-01T00:00:00Z"
			}`))
		}))
		defer server.Close()

		entity := NewAssetRatesEntity(
			server.Client(),
			"test-token",
			map[string]string{"transaction": server.URL},
		)

		input := models.NewCreateAssetRateInput("USD", "BRL", 500)
		result, err := entity.CreateOrUpdateAssetRate(context.Background(), "org-123", "ledger-456", input)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "rate-123", result.ID)
		assert.Equal(t, "USD", result.From)
		assert.Equal(t, "BRL", result.To)
	})

	t.Run("GetAssetRate with real HTTP server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/organizations/org-123/ledgers/ledger-456/asset-rates/ext-789", r.URL.Path)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "rate-123",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"externalId": "ext-789",
				"from": "USD",
				"to": "EUR",
				"rate": 0.92,
				"createdAt": "2024-01-01T00:00:00Z",
				"updatedAt": "2024-01-01T00:00:00Z"
			}`))
		}))
		defer server.Close()

		entity := NewAssetRatesEntity(
			server.Client(),
			"test-token",
			map[string]string{"transaction": server.URL},
		)

		result, err := entity.GetAssetRate(context.Background(), "org-123", "ledger-456", "ext-789")

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "rate-123", result.ID)
		assert.Equal(t, "ext-789", result.ExternalID)
		assert.Equal(t, "USD", result.From)
		assert.Equal(t, "EUR", result.To)
	})

	t.Run("ListAssetRatesByAssetCode with real HTTP server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/organizations/org-123/ledgers/ledger-456/asset-rates/from/USD", r.URL.Path)

			query := r.URL.Query()
			assert.Equal(t, "BRL,EUR", query.Get("to"))
			assert.Equal(t, "10", query.Get("limit"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"items": [
					{
						"id": "rate-1",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"from": "USD",
						"to": "BRL",
						"rate": 5.00,
						"createdAt": "2024-01-01T00:00:00Z",
						"updatedAt": "2024-01-01T00:00:00Z"
					},
					{
						"id": "rate-2",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"from": "USD",
						"to": "EUR",
						"rate": 0.92,
						"createdAt": "2024-01-01T00:00:00Z",
						"updatedAt": "2024-01-01T00:00:00Z"
					}
				],
				"limit": 10
			}`))
		}))
		defer server.Close()

		entity := NewAssetRatesEntity(
			server.Client(),
			"test-token",
			map[string]string{"transaction": server.URL},
		)

		opts := models.NewAssetRateListOptions().WithTo("BRL", "EUR").WithLimit(10)
		result, err := entity.ListAssetRatesByAssetCode(context.Background(), "org-123", "ledger-456", "USD", opts)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Items, 2)
		assert.Equal(t, "USD", result.Items[0].From)
		assert.Equal(t, "BRL", result.Items[0].To)
	})
}

func TestAssetRatesEntity_ContextCancellation(t *testing.T) {
	t.Run("CreateOrUpdateAssetRate with cancelled context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		entity := NewAssetRatesEntity(
			server.Client(),
			"test-token",
			map[string]string{"transaction": server.URL},
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		input := models.NewCreateAssetRateInput("USD", "BRL", 500)
		_, err := entity.CreateOrUpdateAssetRate(ctx, "org-123", "ledger-456", input)

		require.Error(t, err)
	})

	t.Run("GetAssetRate with cancelled context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		entity := NewAssetRatesEntity(
			server.Client(),
			"test-token",
			map[string]string{"transaction": server.URL},
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := entity.GetAssetRate(ctx, "org-123", "ledger-456", "ext-789")

		require.Error(t, err)
	})

	t.Run("ListAssetRatesByAssetCode with cancelled context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		entity := NewAssetRatesEntity(
			server.Client(),
			"test-token",
			map[string]string{"transaction": server.URL},
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := entity.ListAssetRatesByAssetCode(ctx, "org-123", "ledger-456", "USD", nil)

		require.Error(t, err)
	})
}

func TestAssetRatesEntity_ValidationEdgeCases(t *testing.T) {
	entity := &assetRatesEntity{
		httpClient: newHTTPClientAdapter(&MockHTTPClient{
			DoFunc: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{}`)),
				}, nil
			},
		}),
		baseURLs: map[string]string{"transaction": "https://api.example.com"},
	}

	t.Run("CreateOrUpdateAssetRate with whitespace-only organization ID", func(t *testing.T) {
		input := models.NewCreateAssetRateInput("USD", "BRL", 500)
		_, err := entity.CreateOrUpdateAssetRate(context.Background(), "   ", "ledger-456", input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "organizationID")
	})

	t.Run("GetAssetRate with whitespace-only external ID", func(t *testing.T) {
		_, err := entity.GetAssetRate(context.Background(), "org-123", "ledger-456", "   ")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "externalID")
	})

	t.Run("ListAssetRatesByAssetCode with whitespace-only asset code", func(t *testing.T) {
		_, err := entity.ListAssetRatesByAssetCode(context.Background(), "org-123", "ledger-456", "   ", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "assetCode")
	})

	t.Run("CreateOrUpdateAssetRate with negative scale", func(t *testing.T) {
		input := models.NewCreateAssetRateInput("USD", "BRL", 500).WithScale(-1)
		_, err := entity.CreateOrUpdateAssetRate(context.Background(), "org-123", "ledger-456", input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scale must be non-negative")
	})
}

func TestAssetRatesEntity_ResponseParsing(t *testing.T) {
	t.Run("CreateOrUpdateAssetRate with complete response fields", func(t *testing.T) {
		mockClient := &MockHTTPClient{
			DoFunc: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`{
						"id": "rate-full",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"externalId": "ext-full",
						"from": "USD",
						"to": "BRL",
						"rate": 5.25,
						"scale": 2,
						"source": "Central Bank",
						"ttl": 3600,
						"createdAt": "2024-01-15T10:30:00Z",
						"updatedAt": "2024-01-15T12:00:00Z",
						"metadata": {"provider": "forex", "region": "LATAM"}
					}`)),
				}, nil
			},
		}

		entity := &assetRatesEntity{
			httpClient: newHTTPClientAdapter(mockClient),
			baseURLs:   map[string]string{"transaction": "https://api.example.com"},
		}

		input := models.NewCreateAssetRateInput("USD", "BRL", 525).WithScale(2)
		result, err := entity.CreateOrUpdateAssetRate(context.Background(), "org-123", "ledger-456", input)

		require.NoError(t, err)
		assert.Equal(t, "rate-full", result.ID)
		assert.Equal(t, "org-123", result.OrganizationID)
		assert.Equal(t, "ledger-456", result.LedgerID)
		assert.Equal(t, "ext-full", result.ExternalID)
		assert.Equal(t, "USD", result.From)
		assert.Equal(t, "BRL", result.To)
		assert.InDelta(t, 5.25, result.Rate, 0.001)
		assert.NotNil(t, result.Scale)
		assert.InDelta(t, float64(2), *result.Scale, 0.001)
		assert.NotNil(t, result.Source)
		assert.Equal(t, "Central Bank", *result.Source)
		assert.Equal(t, 3600, result.TTL)
		assert.NotNil(t, result.Metadata)
		assert.Equal(t, "forex", result.Metadata["provider"])
		assert.Equal(t, "LATAM", result.Metadata["region"])
	})

	t.Run("ListAssetRatesByAssetCode with pagination cursors", func(t *testing.T) {
		mockClient := &MockHTTPClient{
			DoFunc: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`{
						"items": [
							{"id": "rate-1", "from": "USD", "to": "BRL", "rate": 5.00}
						],
						"limit": 10,
						"next_cursor": "cursor-next-123",
						"prev_cursor": "cursor-prev-456"
					}`)),
				}, nil
			},
		}

		entity := &assetRatesEntity{
			httpClient: newHTTPClientAdapter(mockClient),
			baseURLs:   map[string]string{"transaction": "https://api.example.com"},
		}

		result, err := entity.ListAssetRatesByAssetCode(context.Background(), "org-123", "ledger-456", "USD", nil)

		require.NoError(t, err)
		assert.Equal(t, 10, result.Limit)
		assert.NotNil(t, result.NextCursor)
		assert.Equal(t, "cursor-next-123", *result.NextCursor)
		assert.NotNil(t, result.PrevCursor)
		assert.Equal(t, "cursor-prev-456", *result.PrevCursor)
	})

	t.Run("malformed JSON response", func(t *testing.T) {
		mockClient := &MockHTTPClient{
			DoFunc: func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{invalid json}`)),
				}, nil
			},
		}

		entity := &assetRatesEntity{
			httpClient: newHTTPClientAdapter(mockClient),
			baseURLs:   map[string]string{"transaction": "https://api.example.com"},
		}

		input := models.NewCreateAssetRateInput("USD", "BRL", 500)
		_, err := entity.CreateOrUpdateAssetRate(context.Background(), "org-123", "ledger-456", input)

		require.Error(t, err)
	})
}

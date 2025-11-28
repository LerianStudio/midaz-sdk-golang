package entities

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities/mocks"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListAssets tests the ListAssets method with mock service
func TestListAssets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockAssetsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"

	assetsList := &models.ListResponse[models.Asset]{
		Items: []models.Asset{
			{
				ID:             "asset-123",
				Name:           "US Dollar",
				Code:           "USD",
				OrganizationID: orgID,
				LedgerID:       ledgerID,
				Type:           "CURRENCY",
				Status: models.Status{
					Code: "ACTIVE",
				},
			},
			{
				ID:             "asset-456",
				Name:           "Euro",
				Code:           "EUR",
				OrganizationID: orgID,
				LedgerID:       ledgerID,
				Type:           "CURRENCY",
				Status: models.Status{
					Code: "ACTIVE",
				},
			},
		},
		Pagination: models.Pagination{
			Total:  2,
			Limit:  10,
			Offset: 0,
		},
	}

	mockService.EXPECT().
		ListAssets(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(assetsList, nil)

	result, err := mockService.ListAssets(ctx, orgID, ledgerID, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Pagination.Total)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "asset-123", result.Items[0].ID)
	assert.Equal(t, "US Dollar", result.Items[0].Name)
	assert.Equal(t, "USD", result.Items[0].Code)
	assert.Equal(t, "CURRENCY", result.Items[0].Type)
	assert.Equal(t, "ACTIVE", result.Items[0].Status.Code)

	mockService.EXPECT().
		ListAssets(gomock.Any(), "", ledgerID, gomock.Any()).
		Return(nil, errors.New("organization ID is required"))

	_, err = mockService.ListAssets(ctx, "", ledgerID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	mockService.EXPECT().
		ListAssets(gomock.Any(), orgID, "", gomock.Any()).
		Return(nil, errors.New("ledger ID is required"))

	_, err = mockService.ListAssets(ctx, orgID, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")
}

// TestGetAsset tests the GetAsset method with mock service
func TestGetAsset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockAssetsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	assetID := "asset-123"

	asset := &models.Asset{
		ID:             assetID,
		Name:           "US Dollar",
		Code:           "USD",
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Type:           "CURRENCY",
		Status: models.Status{
			Code: "ACTIVE",
		},
	}

	mockService.EXPECT().
		GetAsset(gomock.Any(), orgID, ledgerID, assetID).
		Return(asset, nil)

	result, err := mockService.GetAsset(ctx, orgID, ledgerID, assetID)
	require.NoError(t, err)
	assert.Equal(t, assetID, result.ID)
	assert.Equal(t, "US Dollar", result.Name)
	assert.Equal(t, "USD", result.Code)
	assert.Equal(t, "CURRENCY", result.Type)
	assert.Equal(t, "ACTIVE", result.Status.Code)

	mockService.EXPECT().
		GetAsset(gomock.Any(), "", ledgerID, assetID).
		Return(nil, errors.New("organization ID is required"))

	_, err = mockService.GetAsset(ctx, "", ledgerID, assetID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	mockService.EXPECT().
		GetAsset(gomock.Any(), orgID, "", assetID).
		Return(nil, errors.New("ledger ID is required"))

	_, err = mockService.GetAsset(ctx, orgID, "", assetID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	mockService.EXPECT().
		GetAsset(gomock.Any(), orgID, ledgerID, "").
		Return(nil, errors.New("asset ID is required"))

	_, err = mockService.GetAsset(ctx, orgID, ledgerID, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "asset ID is required")

	mockService.EXPECT().
		GetAsset(gomock.Any(), orgID, ledgerID, "not-found").
		Return(nil, errors.New("Asset not found"))

	_, err = mockService.GetAsset(ctx, orgID, ledgerID, "not-found")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestCreateAssetMock tests the CreateAsset method with mock service
func TestCreateAssetMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockAssetsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	assetID := "asset-new"

	input := models.NewCreateAssetInput("US Dollar", "USD").
		WithType("CURRENCY").
		WithStatus(models.NewStatus("ACTIVE")).
		WithMetadata(map[string]any{"key": "value"})

	asset := &models.Asset{
		ID:             assetID,
		Name:           "US Dollar",
		Code:           "USD",
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Type:           "CURRENCY",
		Status: models.Status{
			Code: "ACTIVE",
		},
		Metadata: map[string]any{"key": "value"},
	}

	mockService.EXPECT().
		CreateAsset(gomock.Any(), orgID, ledgerID, input).
		Return(asset, nil)

	result, err := mockService.CreateAsset(ctx, orgID, ledgerID, input)
	require.NoError(t, err)
	assert.Equal(t, assetID, result.ID)
	assert.Equal(t, "US Dollar", result.Name)
	assert.Equal(t, "USD", result.Code)
	assert.Equal(t, "CURRENCY", result.Type)
	assert.Equal(t, "ACTIVE", result.Status.Code)
	assert.Equal(t, "value", result.Metadata["key"])

	mockService.EXPECT().
		CreateAsset(gomock.Any(), "", ledgerID, input).
		Return(nil, errors.New("organization ID is required"))

	_, err = mockService.CreateAsset(ctx, "", ledgerID, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	mockService.EXPECT().
		CreateAsset(gomock.Any(), orgID, "", input).
		Return(nil, errors.New("ledger ID is required"))

	_, err = mockService.CreateAsset(ctx, orgID, "", input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	mockService.EXPECT().
		CreateAsset(gomock.Any(), orgID, ledgerID, nil).
		Return(nil, errors.New("asset input cannot be nil"))

	_, err = mockService.CreateAsset(ctx, orgID, ledgerID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "asset input cannot be nil")
}

// TestUpdateAssetMock tests the UpdateAsset method with mock service
func TestUpdateAssetMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockAssetsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	assetID := "asset-123"

	input := models.NewUpdateAssetInput().
		WithName("Updated Dollar").
		WithStatus(models.NewStatus("INACTIVE")).
		WithMetadata(map[string]any{"key": "updated"})

	asset := &models.Asset{
		ID:             assetID,
		Name:           "Updated Dollar",
		Code:           "USD",
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Type:           "CURRENCY",
		Status: models.Status{
			Code: "INACTIVE",
		},
		Metadata: map[string]any{"key": "updated"},
	}

	mockService.EXPECT().
		UpdateAsset(gomock.Any(), orgID, ledgerID, assetID, input).
		Return(asset, nil)

	result, err := mockService.UpdateAsset(ctx, orgID, ledgerID, assetID, input)
	require.NoError(t, err)
	assert.Equal(t, assetID, result.ID)
	assert.Equal(t, "Updated Dollar", result.Name)
	assert.Equal(t, "USD", result.Code)
	assert.Equal(t, "CURRENCY", result.Type)
	assert.Equal(t, "INACTIVE", result.Status.Code)
	assert.Equal(t, "updated", result.Metadata["key"])

	mockService.EXPECT().
		UpdateAsset(gomock.Any(), "", ledgerID, assetID, input).
		Return(nil, errors.New("organization ID is required"))

	_, err = mockService.UpdateAsset(ctx, "", ledgerID, assetID, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	mockService.EXPECT().
		UpdateAsset(gomock.Any(), orgID, "", assetID, input).
		Return(nil, errors.New("ledger ID is required"))

	_, err = mockService.UpdateAsset(ctx, orgID, "", assetID, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	mockService.EXPECT().
		UpdateAsset(gomock.Any(), orgID, ledgerID, "", input).
		Return(nil, errors.New("asset ID is required"))

	_, err = mockService.UpdateAsset(ctx, orgID, ledgerID, "", input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "asset ID is required")

	mockService.EXPECT().
		UpdateAsset(gomock.Any(), orgID, ledgerID, assetID, nil).
		Return(nil, errors.New("asset input cannot be nil"))

	_, err = mockService.UpdateAsset(ctx, orgID, ledgerID, assetID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "asset input cannot be nil")

	mockService.EXPECT().
		UpdateAsset(gomock.Any(), orgID, ledgerID, "not-found", input).
		Return(nil, errors.New("Asset not found"))

	_, err = mockService.UpdateAsset(ctx, orgID, ledgerID, "not-found", input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestDeleteAssetMock tests the DeleteAsset method with mock service
func TestDeleteAssetMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockAssetsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	assetID := "asset-123"

	mockService.EXPECT().
		DeleteAsset(gomock.Any(), orgID, ledgerID, assetID).
		Return(nil)

	err := mockService.DeleteAsset(ctx, orgID, ledgerID, assetID)
	require.NoError(t, err)

	mockService.EXPECT().
		DeleteAsset(gomock.Any(), "", ledgerID, assetID).
		Return(errors.New("organization ID is required"))

	err = mockService.DeleteAsset(ctx, "", ledgerID, assetID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	mockService.EXPECT().
		DeleteAsset(gomock.Any(), orgID, "", assetID).
		Return(errors.New("ledger ID is required"))

	err = mockService.DeleteAsset(ctx, orgID, "", assetID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	mockService.EXPECT().
		DeleteAsset(gomock.Any(), orgID, ledgerID, "").
		Return(errors.New("asset ID is required"))

	err = mockService.DeleteAsset(ctx, orgID, ledgerID, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "asset ID is required")

	mockService.EXPECT().
		DeleteAsset(gomock.Any(), orgID, ledgerID, "not-found").
		Return(errors.New("Asset not found"))

	err = mockService.DeleteAsset(ctx, orgID, ledgerID, "not-found")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// newAssetsHTTPClientAdapter creates an HTTPClient adapter for testing
func newAssetsHTTPClientAdapter(mock *MockHTTPClient) *HTTPClient {
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

// TestNewAssetsEntity tests the NewAssetsEntity constructor
func TestNewAssetsEntity(t *testing.T) {
	tests := []struct {
		name      string
		client    *http.Client
		authToken string
		baseURLs  map[string]string
	}{
		{
			name:      "With custom client",
			client:    &http.Client{},
			authToken: "test-token",
			baseURLs:  map[string]string{"onboarding": "https://api.example.com"},
		},
		{
			name:      "With nil client",
			client:    nil,
			authToken: "test-token",
			baseURLs:  map[string]string{"onboarding": "https://api.example.com"},
		},
		{
			name:      "With empty auth token",
			client:    &http.Client{},
			authToken: "",
			baseURLs:  map[string]string{"onboarding": "https://api.example.com"},
		},
		{
			name:      "With multiple base URLs",
			client:    &http.Client{},
			authToken: "test-token",
			baseURLs: map[string]string{
				"onboarding":  "https://api.example.com",
				"transaction": "https://txn.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewAssetsEntity(tt.client, tt.authToken, tt.baseURLs)
			assert.NotNil(t, service)

			entity, ok := service.(*assetsEntity)
			assert.True(t, ok)
			assert.NotNil(t, entity.httpClient)
			assert.Equal(t, tt.baseURLs, entity.baseURLs)
		})
	}
}

// TestAssetsEntity_ListAssets tests the actual ListAssets implementation
func TestAssetsEntity_ListAssets(t *testing.T) {
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
	}{
		{
			name:     "Success with no options",
			orgID:    "org-123",
			ledgerID: "ledger-123",
			opts:     nil,
			mockResponse: `{
				"items": [
					{
						"id": "asset-123",
						"name": "US Dollar",
						"code": "USD",
						"organizationId": "org-123",
						"ledgerId": "ledger-123",
						"type": "CURRENCY",
						"status": {"code": "ACTIVE"}
					},
					{
						"id": "asset-456",
						"name": "Euro",
						"code": "EUR",
						"organizationId": "org-123",
						"ledgerId": "ledger-123",
						"type": "CURRENCY",
						"status": {"code": "ACTIVE"}
					}
				],
				"pagination": {
					"total": 2,
					"limit": 10,
					"offset": 0
				}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  2,
		},
		{
			name:     "Success with options",
			orgID:    "org-123",
			ledgerID: "ledger-123",
			opts: &models.ListOptions{
				Limit:          5,
				Offset:         10,
				OrderBy:        "name",
				OrderDirection: "asc",
				Filters:        map[string]string{"type": "CURRENCY"},
			},
			mockResponse: `{
				"items": [
					{
						"id": "asset-456",
						"name": "Euro",
						"code": "EUR",
						"organizationId": "org-123",
						"ledgerId": "ledger-123",
						"type": "CURRENCY",
						"status": {"code": "ACTIVE"}
					}
				],
				"pagination": {
					"total": 1,
					"limit": 5,
					"offset": 10
				}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  1,
		},
		{
			name:     "Success with empty result",
			orgID:    "org-123",
			ledgerID: "ledger-123",
			opts:     nil,
			mockResponse: `{
				"items": [],
				"pagination": {
					"total": 0,
					"limit": 10,
					"offset": 0
				}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  0,
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-123",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			expectedError: true,
		},
		{
			name:           "API error - Internal server error",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:           "API error - Unauthorized",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			mockStatusCode: http.StatusUnauthorized,
			mockResponse:   `{"error": "Unauthorized"}`,
			expectedError:  true,
		},
		{
			name:           "API error - Forbidden",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			mockStatusCode: http.StatusForbidden,
			mockResponse:   `{"error": "Forbidden"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			mockError:     errors.New("connection error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				DoFunc: func(_ *http.Request) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}

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

			entity := &assetsEntity{
				httpClient: newAssetsHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.ListAssets(context.Background(), tt.orgID, tt.ledgerID, tt.opts)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Len(t, result.Items, tt.expectedItems)
		})
	}
}

// TestAssetsEntity_GetAsset tests the actual GetAsset implementation
func TestAssetsEntity_GetAsset(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		assetID        string
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
	}{
		{
			name:     "Success",
			orgID:    "org-123",
			ledgerID: "ledger-123",
			assetID:  "asset-123",
			mockResponse: `{
				"id": "asset-123",
				"name": "US Dollar",
				"code": "USD",
				"organizationId": "org-123",
				"ledgerId": "ledger-123",
				"type": "CURRENCY",
				"status": {"code": "ACTIVE"}
			}`,
			mockStatusCode: http.StatusOK,
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-123",
			assetID:       "asset-123",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			assetID:       "asset-123",
			expectedError: true,
		},
		{
			name:          "Empty asset ID",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			assetID:       "",
			expectedError: true,
		},
		{
			name:           "Asset not found",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			assetID:        "not-found",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Asset not found"}`,
			expectedError:  true,
		},
		{
			name:           "API error - Internal server error",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			assetID:        "asset-123",
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			assetID:       "asset-123",
			mockError:     errors.New("connection error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				DoFunc: func(_ *http.Request) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}

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

			entity := &assetsEntity{
				httpClient: newAssetsHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.GetAsset(context.Background(), tt.orgID, tt.ledgerID, tt.assetID)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "asset-123", result.ID)
			assert.Equal(t, "US Dollar", result.Name)
			assert.Equal(t, "USD", result.Code)
			assert.Equal(t, "CURRENCY", result.Type)
			assert.Equal(t, "ACTIVE", result.Status.Code)
		})
	}
}

// TestAssetsEntity_CreateAsset tests the actual CreateAsset implementation
func TestAssetsEntity_CreateAsset(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		input          *models.CreateAssetInput
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
	}{
		{
			name:     "Success",
			orgID:    "org-123",
			ledgerID: "ledger-123",
			input:    models.NewCreateAssetInput("US Dollar", "USD").WithType("CURRENCY"),
			mockResponse: `{
				"id": "asset-new",
				"name": "US Dollar",
				"code": "USD",
				"organizationId": "org-123",
				"ledgerId": "ledger-123",
				"type": "CURRENCY",
				"status": {"code": "ACTIVE"}
			}`,
			mockStatusCode: http.StatusCreated,
		},
		{
			name:     "Success with metadata",
			orgID:    "org-123",
			ledgerID: "ledger-123",
			input:    models.NewCreateAssetInput("US Dollar", "USD").WithType("CURRENCY").WithMetadata(map[string]any{"key": "value"}),
			mockResponse: `{
				"id": "asset-new",
				"name": "US Dollar",
				"code": "USD",
				"organizationId": "org-123",
				"ledgerId": "ledger-123",
				"type": "CURRENCY",
				"status": {"code": "ACTIVE"},
				"metadata": {"key": "value"}
			}`,
			mockStatusCode: http.StatusCreated,
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-123",
			input:         models.NewCreateAssetInput("US Dollar", "USD").WithType("CURRENCY"),
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			input:         models.NewCreateAssetInput("US Dollar", "USD").WithType("CURRENCY"),
			expectedError: true,
		},
		{
			name:          "Nil input",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			input:         nil,
			expectedError: true,
		},
		{
			name:           "API error - Bad request",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			input:          models.NewCreateAssetInput("US Dollar", "USD").WithType("CURRENCY"),
			mockStatusCode: http.StatusBadRequest,
			mockResponse:   `{"error": "Invalid input"}`,
			expectedError:  true,
		},
		{
			name:           "API error - Conflict",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			input:          models.NewCreateAssetInput("US Dollar", "USD"),
			mockStatusCode: http.StatusConflict,
			mockResponse:   `{"error": "Asset with code USD already exists"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			input:         models.NewCreateAssetInput("US Dollar", "USD").WithType("CURRENCY"),
			mockError:     errors.New("connection error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				DoFunc: func(_ *http.Request) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}

					statusCode := tt.mockStatusCode
					if statusCode == 0 {
						statusCode = http.StatusCreated
					}

					return &http.Response{
						StatusCode: statusCode,
						Body:       io.NopCloser(strings.NewReader(tt.mockResponse)),
					}, nil
				},
			}

			entity := &assetsEntity{
				httpClient: newAssetsHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.CreateAsset(context.Background(), tt.orgID, tt.ledgerID, tt.input)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "asset-new", result.ID)
		})
	}
}

// TestAssetsEntity_UpdateAsset tests the actual UpdateAsset implementation
func TestAssetsEntity_UpdateAsset(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		assetID        string
		input          *models.UpdateAssetInput
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
	}{
		{
			name:     "Success",
			orgID:    "org-123",
			ledgerID: "ledger-123",
			assetID:  "asset-123",
			input:    models.NewUpdateAssetInput().WithName("Updated Dollar"),
			mockResponse: `{
				"id": "asset-123",
				"name": "Updated Dollar",
				"code": "USD",
				"organizationId": "org-123",
				"ledgerId": "ledger-123",
				"type": "CURRENCY",
				"status": {"code": "ACTIVE"}
			}`,
			mockStatusCode: http.StatusOK,
		},
		{
			name:     "Success with status update",
			orgID:    "org-123",
			ledgerID: "ledger-123",
			assetID:  "asset-123",
			input:    models.NewUpdateAssetInput().WithName("Updated Dollar").WithStatus(models.NewStatus("INACTIVE")),
			mockResponse: `{
				"id": "asset-123",
				"name": "Updated Dollar",
				"code": "USD",
				"organizationId": "org-123",
				"ledgerId": "ledger-123",
				"type": "CURRENCY",
				"status": {"code": "INACTIVE"}
			}`,
			mockStatusCode: http.StatusOK,
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-123",
			assetID:       "asset-123",
			input:         models.NewUpdateAssetInput().WithName("Updated Dollar"),
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			assetID:       "asset-123",
			input:         models.NewUpdateAssetInput().WithName("Updated Dollar"),
			expectedError: true,
		},
		{
			name:          "Empty asset ID",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			assetID:       "",
			input:         models.NewUpdateAssetInput().WithName("Updated Dollar"),
			expectedError: true,
		},
		{
			name:          "Nil input",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			assetID:       "asset-123",
			input:         nil,
			expectedError: true,
		},
		{
			name:           "API error - Not found",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			assetID:        "not-found",
			input:          models.NewUpdateAssetInput().WithName("Updated Dollar"),
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Asset not found"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			assetID:       "asset-123",
			input:         models.NewUpdateAssetInput().WithName("Updated Dollar"),
			mockError:     errors.New("connection error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				DoFunc: func(_ *http.Request) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}

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

			entity := &assetsEntity{
				httpClient: newAssetsHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.UpdateAsset(context.Background(), tt.orgID, tt.ledgerID, tt.assetID, tt.input)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "asset-123", result.ID)
		})
	}
}

// TestAssetsEntity_DeleteAsset tests the actual DeleteAsset implementation
func TestAssetsEntity_DeleteAsset(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		assetID        string
		mockStatusCode int
		mockResponse   string
		mockError      error
		expectedError  bool
	}{
		{
			name:           "Success",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			assetID:        "asset-123",
			mockStatusCode: http.StatusNoContent,
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-123",
			assetID:       "asset-123",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			assetID:       "asset-123",
			expectedError: true,
		},
		{
			name:          "Empty asset ID",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			assetID:       "",
			expectedError: true,
		},
		{
			name:           "API error - Not found",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			assetID:        "not-found",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Asset not found"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			assetID:       "asset-123",
			mockError:     errors.New("connection error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				DoFunc: func(_ *http.Request) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}

					statusCode := tt.mockStatusCode
					if statusCode == 0 {
						statusCode = http.StatusNoContent
					}

					responseBody := tt.mockResponse
					if responseBody == "" {
						responseBody = ""
					}

					return &http.Response{
						StatusCode: statusCode,
						Body:       io.NopCloser(strings.NewReader(responseBody)),
					}, nil
				},
			}

			entity := &assetsEntity{
				httpClient: newAssetsHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			err := entity.DeleteAsset(context.Background(), tt.orgID, tt.ledgerID, tt.assetID)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

// TestAssetsEntity_GetAssetsMetricsCount tests the GetAssetsMetricsCount implementation
func TestAssetsEntity_GetAssetsMetricsCount(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		mockStatusCode int
		mockHeaders    map[string]string
		mockError      error
		expectedError  bool
		expectedCount  int
	}{
		{
			name:           "Success",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			mockStatusCode: http.StatusOK,
			mockHeaders: map[string]string{
				"X-Assets-Count": "42",
			},
			expectedCount: 42,
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-123",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			expectedError: true,
		},
		{
			name:           "API error",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			mockStatusCode: http.StatusInternalServerError,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			mockError:     errors.New("connection error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				DoFunc: func(_ *http.Request) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}

					statusCode := tt.mockStatusCode
					if statusCode == 0 {
						statusCode = http.StatusOK
					}

					header := http.Header{}
					for k, v := range tt.mockHeaders {
						header.Set(k, v)
					}

					return &http.Response{
						StatusCode: statusCode,
						Header:     header,
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				},
			}

			entity := &assetsEntity{
				httpClient: newAssetsHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.GetAssetsMetricsCount(context.Background(), tt.orgID, tt.ledgerID)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
}

// TestAssetsEntity_buildURL tests the buildURL helper function
func TestAssetsEntity_buildURL(t *testing.T) {
	tests := []struct {
		name        string
		orgID       string
		ledgerID    string
		assetID     string
		baseURL     string
		expectedURL string
	}{
		{
			name:        "List assets URL (no asset ID)",
			orgID:       "org-123",
			ledgerID:    "ledger-456",
			assetID:     "",
			baseURL:     "https://api.example.com",
			expectedURL: "https://api.example.com/organizations/org-123/ledgers/ledger-456/assets",
		},
		{
			name:        "Get asset URL (with asset ID)",
			orgID:       "org-123",
			ledgerID:    "ledger-456",
			assetID:     "asset-789",
			baseURL:     "https://api.example.com",
			expectedURL: "https://api.example.com/organizations/org-123/ledgers/ledger-456/assets/asset-789",
		},
		{
			name:        "URL with trailing slash in base",
			orgID:       "org-123",
			ledgerID:    "ledger-456",
			assetID:     "",
			baseURL:     "https://api.example.com/",
			expectedURL: "https://api.example.com/organizations/org-123/ledgers/ledger-456/assets",
		},
		{
			name:        "URL with different base",
			orgID:       "org-abc",
			ledgerID:    "ledger-xyz",
			assetID:     "asset-123",
			baseURL:     "http://localhost:8080",
			expectedURL: "http://localhost:8080/organizations/org-abc/ledgers/ledger-xyz/assets/asset-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &assetsEntity{
				baseURLs: map[string]string{"onboarding": tt.baseURL},
			}

			result := entity.buildURL(tt.orgID, tt.ledgerID, tt.assetID)
			assert.Equal(t, tt.expectedURL, result)
		})
	}
}

// TestAssetsEntity_buildMetricsURL tests the buildMetricsURL helper function
func TestAssetsEntity_buildMetricsURL(t *testing.T) {
	tests := []struct {
		name        string
		orgID       string
		ledgerID    string
		baseURL     string
		expectedURL string
	}{
		{
			name:        "Metrics URL",
			orgID:       "org-123",
			ledgerID:    "ledger-456",
			baseURL:     "https://api.example.com",
			expectedURL: "https://api.example.com/organizations/org-123/ledgers/ledger-456/assets/metrics/count",
		},
		{
			name:        "Metrics URL with different base",
			orgID:       "org-abc",
			ledgerID:    "ledger-xyz",
			baseURL:     "http://localhost:8080",
			expectedURL: "http://localhost:8080/organizations/org-abc/ledgers/ledger-xyz/assets/metrics/count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &assetsEntity{
				baseURLs: map[string]string{"onboarding": tt.baseURL},
			}

			result := entity.buildMetricsURL(tt.orgID, tt.ledgerID)
			assert.Equal(t, tt.expectedURL, result)
		})
	}
}

// TestAssetsEntity_ListAssetsWithHTTPTestServer tests ListAssets with httptest.NewServer
func TestAssetsEntity_ListAssetsWithHTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/organizations/org-123/ledgers/ledger-123/assets")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"items": [
				{
					"id": "asset-123",
					"name": "US Dollar",
					"code": "USD",
					"organizationId": "org-123",
					"ledgerId": "ledger-123",
					"type": "CURRENCY",
					"status": {"code": "ACTIVE"}
				}
			],
			"pagination": {
				"total": 1,
				"limit": 10,
				"offset": 0
			}
		}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewAssetsEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	result, err := service.ListAssets(context.Background(), "org-123", "ledger-123", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, "asset-123", result.Items[0].ID)
	assert.Equal(t, "US Dollar", result.Items[0].Name)
	assert.Equal(t, "USD", result.Items[0].Code)
}

// TestAssetsEntity_GetAssetWithHTTPTestServer tests GetAsset with httptest.NewServer
func TestAssetsEntity_GetAssetWithHTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/organizations/org-123/ledgers/ledger-123/assets/asset-123")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"id": "asset-123",
			"name": "US Dollar",
			"code": "USD",
			"organizationId": "org-123",
			"ledgerId": "ledger-123",
			"type": "CURRENCY",
			"status": {"code": "ACTIVE"}
		}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewAssetsEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	result, err := service.GetAsset(context.Background(), "org-123", "ledger-123", "asset-123")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "asset-123", result.ID)
	assert.Equal(t, "US Dollar", result.Name)
	assert.Equal(t, "USD", result.Code)
	assert.Equal(t, "CURRENCY", result.Type)
}

// TestAssetsEntity_CreateAssetWithHTTPTestServer tests CreateAsset with httptest.NewServer
func TestAssetsEntity_CreateAssetWithHTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/organizations/org-123/ledgers/ledger-123/assets")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte(`{
			"id": "asset-new",
			"name": "US Dollar",
			"code": "USD",
			"organizationId": "org-123",
			"ledgerId": "ledger-123",
			"type": "CURRENCY",
			"status": {"code": "ACTIVE"}
		}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewAssetsEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	input := models.NewCreateAssetInput("US Dollar", "USD").WithType("CURRENCY")
	result, err := service.CreateAsset(context.Background(), "org-123", "ledger-123", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "asset-new", result.ID)
	assert.Equal(t, "US Dollar", result.Name)
	assert.Equal(t, "USD", result.Code)
}

// TestAssetsEntity_UpdateAssetWithHTTPTestServer tests UpdateAsset with httptest.NewServer
func TestAssetsEntity_UpdateAssetWithHTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Contains(t, r.URL.Path, "/organizations/org-123/ledgers/ledger-123/assets/asset-123")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"id": "asset-123",
			"name": "Updated Dollar",
			"code": "USD",
			"organizationId": "org-123",
			"ledgerId": "ledger-123",
			"type": "CURRENCY",
			"status": {"code": "INACTIVE"}
		}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewAssetsEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	input := models.NewUpdateAssetInput().WithName("Updated Dollar").WithStatus(models.NewStatus("INACTIVE"))
	result, err := service.UpdateAsset(context.Background(), "org-123", "ledger-123", "asset-123", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "asset-123", result.ID)
	assert.Equal(t, "Updated Dollar", result.Name)
	assert.Equal(t, "INACTIVE", result.Status.Code)
}

// TestAssetsEntity_DeleteAssetWithHTTPTestServer tests DeleteAsset with httptest.NewServer
func TestAssetsEntity_DeleteAssetWithHTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Contains(t, r.URL.Path, "/organizations/org-123/ledgers/ledger-123/assets/asset-123")

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service := NewAssetsEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	err := service.DeleteAsset(context.Background(), "org-123", "ledger-123", "asset-123")

	require.NoError(t, err)
}

// TestAssetsEntity_HTTPErrorHandling tests HTTP error handling for all methods
func TestAssetsEntity_HTTPErrorHandling(t *testing.T) {
	testCases := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedError bool
	}{
		{
			name:          "Bad Request",
			statusCode:    http.StatusBadRequest,
			responseBody:  `{"error": "Bad request"}`,
			expectedError: true,
		},
		{
			name:          "Unauthorized",
			statusCode:    http.StatusUnauthorized,
			responseBody:  `{"error": "Unauthorized"}`,
			expectedError: true,
		},
		{
			name:          "Forbidden",
			statusCode:    http.StatusForbidden,
			responseBody:  `{"error": "Forbidden"}`,
			expectedError: true,
		},
		{
			name:          "Not Found",
			statusCode:    http.StatusNotFound,
			responseBody:  `{"error": "Not found"}`,
			expectedError: true,
		},
		{
			name:          "Conflict",
			statusCode:    http.StatusConflict,
			responseBody:  `{"error": "Conflict"}`,
			expectedError: true,
		},
		{
			name:          "Internal Server Error",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"error": "Internal server error"}`,
			expectedError: true,
		},
		{
			name:          "Service Unavailable",
			statusCode:    http.StatusServiceUnavailable,
			responseBody:  `{"error": "Service unavailable"}`,
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.responseBody))
			}))
			defer server.Close()

			service := NewAssetsEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

			// Test ListAssets
			_, err := service.ListAssets(context.Background(), "org-123", "ledger-123", nil)
			require.Error(t, err)

			// Test GetAsset
			_, err = service.GetAsset(context.Background(), "org-123", "ledger-123", "asset-123")
			require.Error(t, err)

			// Test CreateAsset
			input := models.NewCreateAssetInput("US Dollar", "USD")
			_, err = service.CreateAsset(context.Background(), "org-123", "ledger-123", input)
			require.Error(t, err)

			// Test UpdateAsset
			updateInput := models.NewUpdateAssetInput().WithName("Updated")
			_, err = service.UpdateAsset(context.Background(), "org-123", "ledger-123", "asset-123", updateInput)
			require.Error(t, err)

			// Test DeleteAsset
			err = service.DeleteAsset(context.Background(), "org-123", "ledger-123", "asset-123")
			require.Error(t, err)
		})
	}
}

// TestAssetsEntity_ContextCancellation tests that context cancellation works
func TestAssetsEntity_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := NewAssetsEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := service.ListAssets(ctx, "org-123", "ledger-123", nil)
	require.Error(t, err)
}

// TestAssetsEntity_QueryParameters tests that query parameters are correctly added
func TestAssetsEntity_QueryParameters(t *testing.T) {
	var receivedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items": [], "pagination": {"total": 0, "limit": 5, "offset": 10}}`))
	}))
	defer server.Close()

	service := NewAssetsEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	opts := &models.ListOptions{
		Limit:          5,
		Offset:         10,
		OrderBy:        "name",
		OrderDirection: "desc",
	}

	_, err := service.ListAssets(context.Background(), "org-123", "ledger-123", opts)
	require.NoError(t, err)

	assert.Contains(t, receivedURL, "limit=5")
	assert.Contains(t, receivedURL, "offset=10")
	assert.Contains(t, receivedURL, "orderBy=name")
	// Note: orderDirection is used instead of sortOrder in this SDK
	assert.Contains(t, receivedURL, "orderDirection=desc")
}

// TestAssetsEntity_RequestHeaders tests that correct headers are sent
func TestAssetsEntity_RequestHeaders(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "asset-123", "name": "US Dollar", "code": "USD", "type": "CURRENCY", "status": {"code": "ACTIVE"}}`))
	}))
	defer server.Close()

	service := NewAssetsEntity(server.Client(), "test-auth-token", map[string]string{"onboarding": server.URL})

	_, err := service.GetAsset(context.Background(), "org-123", "ledger-123", "asset-123")
	require.NoError(t, err)

	// Verify that Authorization header is present (the SDK may use different token formats)
	authHeader := receivedHeaders.Get("Authorization")
	assert.NotEmpty(t, authHeader, "Authorization header should be set")
	assert.Contains(t, authHeader, "test-auth-token")
}

// TestAssetsEntity_CreateAssetRequestBody tests that request body is correctly sent
func TestAssetsEntity_CreateAssetRequestBody(t *testing.T) {
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error

		receivedBody, err = io.ReadAll(r.Body)
		assert.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": "asset-new", "name": "US Dollar", "code": "USD", "type": "CURRENCY", "status": {"code": "ACTIVE"}}`))
	}))
	defer server.Close()

	service := NewAssetsEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	input := models.NewCreateAssetInput("US Dollar", "USD").
		WithType("CURRENCY").
		WithMetadata(map[string]any{"key": "value"})

	_, err := service.CreateAsset(context.Background(), "org-123", "ledger-123", input)
	require.NoError(t, err)

	bodyStr := string(receivedBody)
	assert.Contains(t, bodyStr, `"name":"US Dollar"`)
	assert.Contains(t, bodyStr, `"code":"USD"`)
	assert.Contains(t, bodyStr, `"type":"CURRENCY"`)
	assert.Contains(t, bodyStr, `"metadata"`)
}

// TestAssetsEntity_EmptyResponse tests handling of empty responses
func TestAssetsEntity_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service := NewAssetsEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	err := service.DeleteAsset(context.Background(), "org-123", "ledger-123", "asset-123")
	require.NoError(t, err)
}

// TestAssetsEntity_MalformedJSONResponse tests handling of malformed JSON responses
func TestAssetsEntity_MalformedJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	service := NewAssetsEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	_, err := service.ListAssets(context.Background(), "org-123", "ledger-123", nil)
	require.Error(t, err)

	_, err = service.GetAsset(context.Background(), "org-123", "ledger-123", "asset-123")
	require.Error(t, err)
}

// TestAssetsEntity_LargeResponse tests handling of large responses
func TestAssetsEntity_LargeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		items := make([]string, 100)
		for i := 0; i < 100; i++ {
			items[i] = fmt.Sprintf(`{"id": "asset-%d", "name": "Asset %d", "code": "AST%d", "type": "CURRENCY", "status": {"code": "ACTIVE"}}`, i, i, i)
		}

		response := fmt.Sprintf(`{"items": [%s], "pagination": {"total": 100, "limit": 100, "offset": 0}}`, strings.Join(items, ","))
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	service := NewAssetsEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	result, err := service.ListAssets(context.Background(), "org-123", "ledger-123", nil)
	require.NoError(t, err)
	assert.Len(t, result.Items, 100)
	assert.Equal(t, 100, result.Pagination.Total)
}

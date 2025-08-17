package entities

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities/mocks"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
	"github.com/golang/mock/gomock"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// \1 performs an operation
func TestListAccounts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock accounts service
	mockService := mocks.NewMockAccountsService(ctrl)

	// Test data
	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"

	// Create test account list response
	accountsList := &models.ListResponse[models.Account]{
		Items: []models.Account{
			{
				ID:             "acc-123",
				Name:           "Test Account 1",
				AssetCode:      "USD",
				OrganizationID: orgID,
				LedgerID:       ledgerID,
				Type:           "LIABILITY",
				Status: models.Status{
					Code: "ACTIVE",
				},
			},
			{
				ID:             "acc-456",
				Name:           "Test Account 2",
				AssetCode:      "EUR",
				OrganizationID: orgID,
				LedgerID:       ledgerID,
				Type:           "ASSET",
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

	// Setup expectations
	mockService.EXPECT().
		ListAccounts(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(accountsList, nil)

	// Test with default options
	result, err := mockService.ListAccounts(ctx, orgID, ledgerID, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, result.Pagination.Total)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "acc-123", result.Items[0].ID)
	assert.Equal(t, "Test Account 1", result.Items[0].Name)
	assert.Equal(t, "USD", result.Items[0].AssetCode)
	assert.Equal(t, "LIABILITY", result.Items[0].Type)
	assert.Equal(t, "ACTIVE", result.Items[0].Status.Code)

	// Test validation for empty orgID
	mockService.EXPECT().
		ListAccounts(gomock.Any(), "", ledgerID, gomock.Any()).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.ListAccounts(ctx, "", ledgerID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test validation for empty ledgerID
	mockService.EXPECT().
		ListAccounts(gomock.Any(), orgID, "", gomock.Any()).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.ListAccounts(ctx, orgID, "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")
}

// \1 performs an operation
func TestGetAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock accounts service
	mockService := mocks.NewMockAccountsService(ctrl)

	// Test data
	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	accountID := "acc-123"
	alias := "test-account-1"

	// Create test account
	account := &models.Account{
		ID:             accountID,
		Name:           "Test Account 1",
		AssetCode:      "USD",
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Type:           "LIABILITY",
		Alias:          &alias,
		Status: models.Status{
			Code: "ACTIVE",
		},
	}

	// Setup expectation for successful case
	mockService.EXPECT().
		GetAccount(gomock.Any(), orgID, ledgerID, accountID).
		Return(account, nil)

	// Test getting an account by ID
	result, err := mockService.GetAccount(ctx, orgID, ledgerID, accountID)
	assert.NoError(t, err)
	assert.Equal(t, accountID, result.ID)
	assert.Equal(t, "Test Account 1", result.Name)
	assert.Equal(t, "USD", result.AssetCode)
	assert.Equal(t, "LIABILITY", result.Type)
	assert.Equal(t, "ACTIVE", result.Status.Code)
	assert.NotNil(t, result.Alias)
	assert.Equal(t, alias, *result.Alias)

	// Test with empty organizationID
	mockService.EXPECT().
		GetAccount(gomock.Any(), "", ledgerID, accountID).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.GetAccount(ctx, "", ledgerID, accountID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test with empty ledgerID
	mockService.EXPECT().
		GetAccount(gomock.Any(), orgID, "", accountID).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.GetAccount(ctx, orgID, "", accountID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test with empty accountID
	mockService.EXPECT().
		GetAccount(gomock.Any(), orgID, ledgerID, "").
		Return(nil, fmt.Errorf("account ID is required"))

	_, err = mockService.GetAccount(ctx, orgID, ledgerID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account ID is required")

	// Test with not found
	mockService.EXPECT().
		GetAccount(gomock.Any(), orgID, ledgerID, "not-found").
		Return(nil, fmt.Errorf("Account not found"))

	_, err = mockService.GetAccount(ctx, orgID, ledgerID, "not-found")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// \1 performs an operation
func TestGetAccountByAlias(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock accounts service
	mockService := mocks.NewMockAccountsService(ctrl)

	// Test data
	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	accountID := "acc-123"
	alias := "test-account-1"

	// Create test account
	account := &models.Account{
		ID:             accountID,
		Name:           "Test Account 1",
		AssetCode:      "USD",
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Type:           "LIABILITY",
		Alias:          &alias,
		Status: models.Status{
			Code: "ACTIVE",
		},
	}

	// Setup expectation for successful case
	mockService.EXPECT().
		GetAccountByAlias(gomock.Any(), orgID, ledgerID, alias).
		Return(account, nil)

	// Test getting an account by alias
	result, err := mockService.GetAccountByAlias(ctx, orgID, ledgerID, alias)
	assert.NoError(t, err)
	assert.Equal(t, accountID, result.ID)
	assert.Equal(t, "Test Account 1", result.Name)
	assert.Equal(t, "USD", result.AssetCode)
	assert.Equal(t, "LIABILITY", result.Type)
	assert.Equal(t, "ACTIVE", result.Status.Code)
	assert.NotNil(t, result.Alias)
	assert.Equal(t, alias, *result.Alias)

	// Test with empty organizationID
	mockService.EXPECT().
		GetAccountByAlias(gomock.Any(), "", ledgerID, alias).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.GetAccountByAlias(ctx, "", ledgerID, alias)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test with empty ledgerID
	mockService.EXPECT().
		GetAccountByAlias(gomock.Any(), orgID, "", alias).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.GetAccountByAlias(ctx, orgID, "", alias)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test with empty alias
	mockService.EXPECT().
		GetAccountByAlias(gomock.Any(), orgID, ledgerID, "").
		Return(nil, fmt.Errorf("account alias is required"))

	_, err = mockService.GetAccountByAlias(ctx, orgID, ledgerID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account alias is required")

	// Test with not found
	mockService.EXPECT().
		GetAccountByAlias(gomock.Any(), orgID, ledgerID, "not-found").
		Return(nil, fmt.Errorf("Account not found"))

	_, err = mockService.GetAccountByAlias(ctx, orgID, ledgerID, "not-found")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// \1 performs an operation
func TestCreateAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock accounts service
	mockService := mocks.NewMockAccountsService(ctrl)

	// Test data
	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	accountID := "acc-new"
	alias := "custom-alias"

	// Create test input
	input := models.NewCreateAccountInput("New Account", "USD", "ASSET").
		WithAlias(alias).
		WithStatus(models.NewStatus("ACTIVE")).
		WithMetadata(map[string]any{"key": "value"})

	// Create expected output
	account := &models.Account{
		ID:             accountID,
		Name:           "New Account",
		AssetCode:      "USD",
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Type:           "ASSET",
		Alias:          &alias,
		Status: models.Status{
			Code: "ACTIVE",
		},
		Metadata: map[string]any{"key": "value"},
	}

	// Setup expectation for successful case
	mockService.EXPECT().
		CreateAccount(gomock.Any(), orgID, ledgerID, input).
		Return(account, nil)

	// Test creating a new account
	result, err := mockService.CreateAccount(ctx, orgID, ledgerID, input)
	assert.NoError(t, err)
	assert.Equal(t, accountID, result.ID)
	assert.Equal(t, "New Account", result.Name)
	assert.Equal(t, "USD", result.AssetCode)
	assert.Equal(t, "ASSET", result.Type)
	assert.Equal(t, "ACTIVE", result.Status.Code)
	assert.NotNil(t, result.Alias)
	assert.Equal(t, alias, *result.Alias)
	assert.Equal(t, "value", result.Metadata["key"])

	// Test with empty organizationID
	mockService.EXPECT().
		CreateAccount(gomock.Any(), "", ledgerID, input).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.CreateAccount(ctx, "", ledgerID, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test with empty ledgerID
	mockService.EXPECT().
		CreateAccount(gomock.Any(), orgID, "", input).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.CreateAccount(ctx, orgID, "", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test with nil input
	mockService.EXPECT().
		CreateAccount(gomock.Any(), orgID, ledgerID, nil).
		Return(nil, fmt.Errorf("account input cannot be nil"))

	_, err = mockService.CreateAccount(ctx, orgID, ledgerID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account input cannot be nil")
}

// \1 performs an operation
func TestUpdateAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock accounts service
	mockService := mocks.NewMockAccountsService(ctrl)

	// Test data
	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	accountID := "acc-123"
	alias := "test-account-1"

	// Create test input
	input := models.NewUpdateAccountInput().
		WithName("Updated Account").
		WithStatus(models.NewStatus("INACTIVE")).
		WithMetadata(map[string]any{"key": "updated"})

	// Create expected output
	account := &models.Account{
		ID:             accountID,
		Name:           "Updated Account",
		AssetCode:      "USD",
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Type:           "LIABILITY",
		Alias:          &alias,
		Status: models.Status{
			Code: "INACTIVE",
		},
		Metadata: map[string]any{"key": "updated"},
	}

	// Setup expectation for successful case
	mockService.EXPECT().
		UpdateAccount(gomock.Any(), orgID, ledgerID, accountID, input).
		Return(account, nil)

	// Test updating an account
	result, err := mockService.UpdateAccount(ctx, orgID, ledgerID, accountID, input)
	assert.NoError(t, err)
	assert.Equal(t, accountID, result.ID)
	assert.Equal(t, "Updated Account", result.Name)
	assert.Equal(t, "USD", result.AssetCode)
	assert.Equal(t, "LIABILITY", result.Type)
	assert.Equal(t, "INACTIVE", result.Status.Code)
	assert.Equal(t, "updated", result.Metadata["key"])

	// Test with empty organizationID
	mockService.EXPECT().
		UpdateAccount(gomock.Any(), "", ledgerID, accountID, input).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.UpdateAccount(ctx, "", ledgerID, accountID, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test with empty ledgerID
	mockService.EXPECT().
		UpdateAccount(gomock.Any(), orgID, "", accountID, input).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.UpdateAccount(ctx, orgID, "", accountID, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test with empty accountID
	mockService.EXPECT().
		UpdateAccount(gomock.Any(), orgID, ledgerID, "", input).
		Return(nil, fmt.Errorf("account ID is required"))

	_, err = mockService.UpdateAccount(ctx, orgID, ledgerID, "", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account ID is required")

	// Test with nil input
	mockService.EXPECT().
		UpdateAccount(gomock.Any(), orgID, ledgerID, accountID, nil).
		Return(nil, fmt.Errorf("account input cannot be nil"))

	_, err = mockService.UpdateAccount(ctx, orgID, ledgerID, accountID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account input cannot be nil")

	// Test with not found
	mockService.EXPECT().
		UpdateAccount(gomock.Any(), orgID, ledgerID, "not-found", input).
		Return(nil, fmt.Errorf("Account not found"))

	_, err = mockService.UpdateAccount(ctx, orgID, ledgerID, "not-found", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// \1 performs an operation
func TestDeleteAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock accounts service
	mockService := mocks.NewMockAccountsService(ctrl)

	// Test data
	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	accountID := "acc-123"

	// Setup expectation for successful case
	mockService.EXPECT().
		DeleteAccount(gomock.Any(), orgID, ledgerID, accountID).
		Return(nil)

	// Test deleting an account
	err := mockService.DeleteAccount(ctx, orgID, ledgerID, accountID)
	assert.NoError(t, err)

	// Test with empty organizationID
	mockService.EXPECT().
		DeleteAccount(gomock.Any(), "", ledgerID, accountID).
		Return(fmt.Errorf("organization ID is required"))

	err = mockService.DeleteAccount(ctx, "", ledgerID, accountID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test with empty ledgerID
	mockService.EXPECT().
		DeleteAccount(gomock.Any(), orgID, "", accountID).
		Return(fmt.Errorf("ledger ID is required"))

	err = mockService.DeleteAccount(ctx, orgID, "", accountID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test with empty accountID
	mockService.EXPECT().
		DeleteAccount(gomock.Any(), orgID, ledgerID, "").
		Return(fmt.Errorf("account ID is required"))

	err = mockService.DeleteAccount(ctx, orgID, ledgerID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account ID is required")

	// Test with not found
	mockService.EXPECT().
		DeleteAccount(gomock.Any(), orgID, ledgerID, "not-found").
		Return(fmt.Errorf("Account not found"))

	err = mockService.DeleteAccount(ctx, orgID, ledgerID, "not-found")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// \1 performs an operation
func TestGetBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock accounts service
	mockService := mocks.NewMockAccountsService(ctrl)

	// Test data
	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	accountID := "acc-123"
	accountAlias := "test-account-1"

	// Create test balance
	balance := &models.Balance{
		ID:             "bal-123",
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		AccountID:      accountID,
		Alias:          accountAlias,
		AssetCode:      "USD",
		Available:      decimal.NewFromInt(1000000),
		OnHold:         decimal.NewFromInt(0),
		Version:        1,
		AccountType:    "LIABILITY",
		AllowSending:   true,
		AllowReceiving: true,
	}

	// Setup expectation for successful case
	mockService.EXPECT().
		GetBalance(gomock.Any(), orgID, ledgerID, accountID).
		Return(balance, nil)

	// Test getting an account's balance
	result, err := mockService.GetBalance(ctx, orgID, ledgerID, accountID)
	assert.NoError(t, err)
	assert.Equal(t, "bal-123", result.ID)
	assert.Equal(t, accountID, result.AccountID)
	assert.Equal(t, "USD", result.AssetCode)
	assert.Equal(t, "LIABILITY", result.AccountType)
	assert.Equal(t, decimal.NewFromInt(1000000), result.Available)
	assert.Equal(t, decimal.NewFromInt(0), result.OnHold)
	assert.Equal(t, true, result.AllowSending)
	assert.Equal(t, true, result.AllowReceiving)

	// Test with empty organizationID
	mockService.EXPECT().
		GetBalance(gomock.Any(), "", ledgerID, accountID).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.GetBalance(ctx, "", ledgerID, accountID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test with empty ledgerID
	mockService.EXPECT().
		GetBalance(gomock.Any(), orgID, "", accountID).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.GetBalance(ctx, orgID, "", accountID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test with empty accountID
	mockService.EXPECT().
		GetBalance(gomock.Any(), orgID, ledgerID, "").
		Return(nil, fmt.Errorf("account ID is required"))

	_, err = mockService.GetBalance(ctx, orgID, ledgerID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account ID is required")

	// Test with not found
	mockService.EXPECT().
		GetBalance(gomock.Any(), orgID, ledgerID, "not-found").
		Return(nil, fmt.Errorf("Balance not found"))

	_, err = mockService.GetBalance(ctx, orgID, ledgerID, "not-found")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// MockHTTPClient is a mock implementation of the httpClient
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

func newHTTPClientAdapter(mock *MockHTTPClient) *HTTPClient {
	// Create default retry options for tests
	retryOptions := retry.DefaultOptions()

	// Apply fast retry options for tests to avoid long waits
	_ = retry.WithMaxRetries(1)(retryOptions)                      // Reduced from 3
	_ = retry.WithInitialDelay(1 * time.Millisecond)(retryOptions) // Reduced from default
	_ = retry.WithMaxDelay(10 * time.Millisecond)(retryOptions)    // Reduced from default
	_ = retry.WithRetryableHTTPCodes(retry.DefaultRetryableHTTPCodes)(retryOptions)

	return &HTTPClient{
		client: &http.Client{
			Transport: &mockTransport{mock: mock},
		},
		retryOptions: retryOptions,
		jsonPool:     performance.NewJSONPool(), // Initialize JSONPool to prevent nil pointer dereference
	}
}

type mockTransport struct {
	mock *MockHTTPClient
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.mock.DoFunc(req)
}

func TestNewAccountsEntity(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewAccountsEntity(tt.client, tt.authToken, tt.baseURLs)
			assert.NotNil(t, service)

			// Type assertion to check internal fields
			entity, ok := service.(*accountsEntity)
			assert.True(t, ok)
			assert.NotNil(t, entity.httpClient)
			assert.Equal(t, tt.baseURLs, entity.baseURLs)
		})
	}
}

func TestAccountsEntity_ListAccounts(t *testing.T) {
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
						"id": "acc-123",
						"name": "Test Account 1",
						"assetCode": "USD",
						"organizationId": "org-123",
						"ledgerId": "ledger-123",
						"type": "LIABILITY",
						"status": {"code": "ACTIVE"}
					},
					{
						"id": "acc-456",
						"name": "Test Account 2",
						"assetCode": "EUR",
						"organizationId": "org-123",
						"ledgerId": "ledger-123",
						"type": "ASSET",
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
				Filters:        map[string]string{"type": "ASSET"},
			},
			mockResponse: `{
				"items": [
					{
						"id": "acc-456",
						"name": "Test Account 2",
						"assetCode": "EUR",
						"organizationId": "org-123",
						"ledgerId": "ledger-123",
						"type": "ASSET",
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
			mockResponse:   `{"error": "Internal server error"}`,
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
				DoFunc: func(req *http.Request) (*http.Response, error) {
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

			entity := &accountsEntity{
				httpClient: newHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.ListAccounts(context.Background(), tt.orgID, tt.ledgerID, tt.opts)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Len(t, result.Items, tt.expectedItems)
		})
	}
}

func TestAccountsEntity_GetAccount(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		accountID      string
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
	}{
		{
			name:      "Success",
			orgID:     "org-123",
			ledgerID:  "ledger-123",
			accountID: "acc-123",
			mockResponse: `{
				"id": "acc-123",
				"name": "Test Account",
				"assetCode": "USD",
				"organizationId": "org-123",
				"ledgerId": "ledger-123",
				"type": "LIABILITY",
				"status": {"code": "ACTIVE"},
				"alias": "test-account"
			}`,
			mockStatusCode: http.StatusOK,
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-123",
			accountID:     "acc-123",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			accountID:     "acc-123",
			expectedError: true,
		},
		{
			name:          "Empty account ID",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			accountID:     "",
			expectedError: true,
		},
		{
			name:           "Account not found",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			accountID:      "not-found",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Account not found"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			accountID:     "acc-123",
			mockError:     errors.New("connection error"),
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

			entity := &accountsEntity{
				httpClient: newHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.GetAccount(context.Background(), tt.orgID, tt.ledgerID, tt.accountID)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "acc-123", result.ID)
			assert.Equal(t, "Test Account", result.Name)
			assert.Equal(t, "USD", result.AssetCode)
			assert.Equal(t, "LIABILITY", result.Type)
			assert.Equal(t, "ACTIVE", result.Status.Code)
			assert.NotNil(t, result.Alias)
			assert.Equal(t, "test-account", *result.Alias)
		})
	}
}

func TestAccountsEntity_GetAccountByAlias(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		alias          string
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
	}{
		{
			name:     "Success",
			orgID:    "org-123",
			ledgerID: "ledger-123",
			alias:    "test-account",
			mockResponse: `{
				"items": [
					{
						"id": "acc-123",
						"name": "Test Account",
						"assetCode": "USD",
						"organizationId": "org-123",
						"ledgerId": "ledger-123",
						"type": "LIABILITY",
						"status": {"code": "ACTIVE"},
						"alias": "test-account"
					}
				],
				"pagination": {
					"total": 1,
					"limit": 10,
					"offset": 0
				}
			}`,
			mockStatusCode: http.StatusOK,
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-123",
			alias:         "test-account",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			alias:         "test-account",
			expectedError: true,
		},
		{
			name:          "Empty alias",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			alias:         "",
			expectedError: true,
		},
		{
			name:           "Account not found",
			orgID:          "org-123",
			ledgerID:       "ledger-123",
			alias:          "not-found",
			mockResponse:   `{"items": [], "pagination": {"total": 0, "limit": 10, "offset": 0}}`,
			mockStatusCode: http.StatusOK,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			alias:         "test-account",
			mockError:     errors.New("connection error"),
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

			entity := &accountsEntity{
				httpClient: newHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.GetAccountByAlias(context.Background(), tt.orgID, tt.ledgerID, tt.alias)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "acc-123", result.ID)
			assert.Equal(t, "Test Account", result.Name)
			assert.Equal(t, "USD", result.AssetCode)
			assert.Equal(t, "LIABILITY", result.Type)
			assert.Equal(t, "ACTIVE", result.Status.Code)
			assert.NotNil(t, result.Alias)
			assert.Equal(t, "test-account", *result.Alias)
		})
	}
}

func TestAccountsEntity_CreateAccount(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		input          *models.CreateAccountInput
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
	}{
		{
			name:     "Success",
			orgID:    "org-123",
			ledgerID: "ledger-123",
			input: &models.CreateAccountInput{
				Name:      "New Account",
				Type:      "ASSET",
				AssetCode: "USD",
			},
			mockResponse: `{
				"id": "acc-new",
				"name": "New Account",
				"assetCode": "USD",
				"organizationId": "org-123",
				"ledgerId": "ledger-123",
				"type": "ASSET",
				"status": {"code": "ACTIVE"}
			}`,
			mockStatusCode: http.StatusOK,
		},
		{
			name:     "Empty organization ID",
			orgID:    "",
			ledgerID: "ledger-123",
			input: &models.CreateAccountInput{
				Name:      "New Account",
				Type:      "ASSET",
				AssetCode: "USD",
			},
			expectedError: true,
		},
		{
			name:     "Empty ledger ID",
			orgID:    "org-123",
			ledgerID: "",
			input: &models.CreateAccountInput{
				Name:      "New Account",
				Type:      "ASSET",
				AssetCode: "USD",
			},
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
			name:     "API error",
			orgID:    "org-123",
			ledgerID: "ledger-123",
			input: &models.CreateAccountInput{
				Name:      "New Account",
				Type:      "ASSET",
				AssetCode: "USD",
			},
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:     "HTTP client error",
			orgID:    "org-123",
			ledgerID: "ledger-123",
			input: &models.CreateAccountInput{
				Name:      "New Account",
				Type:      "ASSET",
				AssetCode: "USD",
			},
			mockError:     errors.New("connection error"),
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

			entity := &accountsEntity{
				httpClient: newHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.CreateAccount(context.Background(), tt.orgID, tt.ledgerID, tt.input)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "acc-new", result.ID)
			assert.Equal(t, "New Account", result.Name)
			assert.Equal(t, "USD", result.AssetCode)
			assert.Equal(t, "ASSET", result.Type)
			assert.Equal(t, "ACTIVE", result.Status.Code)
		})
	}
}

func TestAccountsEntity_UpdateAccount(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		accountID      string
		input          *models.UpdateAccountInput
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
	}{
		{
			name:      "Success",
			orgID:     "org-123",
			ledgerID:  "ledger-123",
			accountID: "acc-123",
			input: &models.UpdateAccountInput{
				Name: "Updated Account",
			},
			mockResponse: `{
				"id": "acc-123",
				"name": "Updated Account",
				"assetCode": "USD",
				"organizationId": "org-123",
				"ledgerId": "ledger-123",
				"type": "LIABILITY",
				"status": {"code": "ACTIVE"}
			}`,
			mockStatusCode: http.StatusOK,
		},
		{
			name:      "Empty organization ID",
			orgID:     "",
			ledgerID:  "ledger-123",
			accountID: "acc-123",
			input: &models.UpdateAccountInput{
				Name: "Updated Account",
			},
			expectedError: true,
		},
		{
			name:      "Empty ledger ID",
			orgID:     "org-123",
			ledgerID:  "",
			accountID: "acc-123",
			input: &models.UpdateAccountInput{
				Name: "Updated Account",
			},
			expectedError: true,
		},
		{
			name:      "Empty account ID",
			orgID:     "org-123",
			ledgerID:  "ledger-123",
			accountID: "",
			input: &models.UpdateAccountInput{
				Name: "Updated Account",
			},
			expectedError: true,
		},
		{
			name:          "Nil input",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			accountID:     "acc-123",
			input:         nil,
			expectedError: true,
		},
		{
			name:      "API error",
			orgID:     "org-123",
			ledgerID:  "ledger-123",
			accountID: "acc-123",
			input: &models.UpdateAccountInput{
				Name: "Updated Account",
			},
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:      "HTTP client error",
			orgID:     "org-123",
			ledgerID:  "ledger-123",
			accountID: "acc-123",
			input: &models.UpdateAccountInput{
				Name: "Updated Account",
			},
			mockError:     errors.New("connection error"),
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

			entity := &accountsEntity{
				httpClient: newHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			result, err := entity.UpdateAccount(context.Background(), tt.orgID, tt.ledgerID, tt.accountID, tt.input)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "acc-123", result.ID)
			assert.Equal(t, "Updated Account", result.Name)
			assert.Equal(t, "USD", result.AssetCode)
			assert.Equal(t, "LIABILITY", result.Type)
			assert.Equal(t, "ACTIVE", result.Status.Code)
		})
	}
}

func TestAccountsEntity_DeleteAccount(t *testing.T) {
	tests := []struct {
		name          string
		orgID         string
		ledgerID      string
		accountID     string
		mockError     error
		expectedError bool
	}{
		{
			name:      "Success",
			orgID:     "org-123",
			ledgerID:  "ledger-123",
			accountID: "acc-123",
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-123",
			accountID:     "acc-123",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			accountID:     "acc-123",
			expectedError: true,
		},
		{
			name:          "Empty account ID",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			accountID:     "",
			expectedError: true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-123",
			accountID:     "acc-123",
			mockError:     errors.New("connection error"),
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

					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				},
			}

			entity := &accountsEntity{
				httpClient: newHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"onboarding": "https://api.example.com"},
			}

			err := entity.DeleteAccount(context.Background(), tt.orgID, tt.ledgerID, tt.accountID)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

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
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewBalancesEntity tests the constructor for balances entity
func TestNewBalancesEntity(t *testing.T) {
	tests := []struct {
		name      string
		client    *http.Client
		authToken string
		baseURLs  map[string]string
	}{
		{
			name:      "With custom client",
			client:    &http.Client{Timeout: 30 * time.Second},
			authToken: "test-token",
			baseURLs:  map[string]string{"transaction": "https://api.example.com/v1"},
		},
		{
			name:      "With nil client",
			client:    nil,
			authToken: "test-token",
			baseURLs:  map[string]string{"transaction": "https://api.example.com/v1"},
		},
		{
			name:      "With empty auth token",
			client:    &http.Client{},
			authToken: "",
			baseURLs:  map[string]string{"transaction": "https://api.example.com/v1"},
		},
		{
			name:      "With multiple base URLs",
			client:    &http.Client{},
			authToken: "test-token",
			baseURLs: map[string]string{
				"transaction": "https://transaction.api.example.com/v1",
				"onboarding":  "https://onboarding.api.example.com/v1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewBalancesEntity(tt.client, tt.authToken, tt.baseURLs)
			require.NotNil(t, service)

			entity, ok := service.(*balancesEntity)
			require.True(t, ok, "Expected *balancesEntity type")
			assert.NotNil(t, entity.httpClient)
			assert.Equal(t, tt.baseURLs, entity.baseURLs)
		})
	}
}

// TestBalancesEntity_buildURL tests the URL building helper
func TestBalancesEntity_buildURL(t *testing.T) {
	entity := &balancesEntity{
		baseURLs: map[string]string{"transaction": "https://api.example.com/v1"},
	}

	tests := []struct {
		name      string
		orgID     string
		ledgerID  string
		balanceID string
		expected  string
	}{
		{
			name:      "List balances URL (no balanceID)",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			balanceID: "",
			expected:  "https://api.example.com/v1/organizations/org-123/ledgers/ledger-456/balances",
		},
		{
			name:      "Get specific balance URL",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			balanceID: "bal-789",
			expected:  "https://api.example.com/v1/organizations/org-123/ledgers/ledger-456/balances/bal-789",
		},
		{
			name:      "With UUID-style IDs",
			orgID:     "550e8400-e29b-41d4-a716-446655440000",
			ledgerID:  "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			balanceID: "7ba7b810-9dad-11d1-80b4-00c04fd430c8",
			expected:  "https://api.example.com/v1/organizations/550e8400-e29b-41d4-a716-446655440000/ledgers/6ba7b810-9dad-11d1-80b4-00c04fd430c8/balances/7ba7b810-9dad-11d1-80b4-00c04fd430c8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := entity.buildURL(tt.orgID, tt.ledgerID, tt.balanceID)
			assert.Equal(t, tt.expected, url)
		})
	}
}

// TestBalancesEntity_buildAccountURL tests the account URL builder
func TestBalancesEntity_buildAccountURL(t *testing.T) {
	entity := &balancesEntity{
		baseURLs: map[string]string{"transaction": "https://api.example.com/v1"},
	}

	tests := []struct {
		name      string
		orgID     string
		ledgerID  string
		accountID string
		expected  string
	}{
		{
			name:      "Standard account balances URL",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			accountID: "acc-789",
			expected:  "https://api.example.com/v1/organizations/org-123/ledgers/ledger-456/accounts/acc-789/balances",
		},
		{
			name:      "With UUID-style IDs",
			orgID:     "550e8400-e29b-41d4-a716-446655440000",
			ledgerID:  "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			accountID: "7ba7b810-9dad-11d1-80b4-00c04fd430c8",
			expected:  "https://api.example.com/v1/organizations/550e8400-e29b-41d4-a716-446655440000/ledgers/6ba7b810-9dad-11d1-80b4-00c04fd430c8/accounts/7ba7b810-9dad-11d1-80b4-00c04fd430c8/balances",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := entity.buildAccountURL(tt.orgID, tt.ledgerID, tt.accountID)
			assert.Equal(t, tt.expected, url)
		})
	}
}

// TestBalancesEntity_buildAccountAliasURL tests the account alias URL builder
func TestBalancesEntity_buildAccountAliasURL(t *testing.T) {
	entity := &balancesEntity{
		baseURLs: map[string]string{"transaction": "https://api.example.com/v1"},
	}

	tests := []struct {
		name     string
		orgID    string
		ledgerID string
		alias    string
		expected string
	}{
		{
			name:     "Standard alias URL",
			orgID:    "org-123",
			ledgerID: "ledger-456",
			alias:    "my-account",
			expected: "https://api.example.com/v1/organizations/org-123/ledgers/ledger-456/accounts/alias/my-account/balances",
		},
		{
			name:     "With @-prefixed alias",
			orgID:    "org-123",
			ledgerID: "ledger-456",
			alias:    "@person1",
			expected: "https://api.example.com/v1/organizations/org-123/ledgers/ledger-456/accounts/alias/@person1/balances",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := entity.buildAccountAliasURL(tt.orgID, tt.ledgerID, tt.alias)
			assert.Equal(t, tt.expected, url)
		})
	}
}

// TestBalancesEntity_buildExternalCodeURL tests the external code URL builder
func TestBalancesEntity_buildExternalCodeURL(t *testing.T) {
	entity := &balancesEntity{
		baseURLs: map[string]string{"transaction": "https://api.example.com/v1"},
	}

	tests := []struct {
		name     string
		orgID    string
		ledgerID string
		code     string
		expected string
	}{
		{
			name:     "Standard external code URL",
			orgID:    "org-123",
			ledgerID: "ledger-456",
			code:     "EXT-001",
			expected: "https://api.example.com/v1/organizations/org-123/ledgers/ledger-456/accounts/external/EXT-001/balances",
		},
		{
			name:     "With alphanumeric code",
			orgID:    "org-123",
			ledgerID: "ledger-456",
			code:     "customer123456",
			expected: "https://api.example.com/v1/organizations/org-123/ledgers/ledger-456/accounts/external/customer123456/balances",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := entity.buildExternalCodeURL(tt.orgID, tt.ledgerID, tt.code)
			assert.Equal(t, tt.expected, url)
		})
	}
}

// newBalancesHTTPClientAdapter creates an HTTP client adapter for testing balances
func newBalancesHTTPClientAdapter(mock *MockHTTPClient) *HTTPClient {
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

// createTestBalance creates a test balance for testing
func createTestBalance(id, orgID, ledgerID, accountID, assetCode string) models.Balance {
	now := time.Now()

	return models.Balance{
		ID:             id,
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		AccountID:      accountID,
		Alias:          "@test-account",
		Key:            "primary",
		AssetCode:      assetCode,
		Available:      decimal.NewFromInt(1000000),
		OnHold:         decimal.NewFromInt(50000),
		Version:        1,
		AccountType:    "LIABILITY",
		AllowSending:   true,
		AllowReceiving: true,
		CreatedAt:      now,
		UpdatedAt:      now,
		DeletedAt:      nil,
		Metadata:       map[string]any{"key": "value"},
	}
}

// TestBalancesEntity_ListBalances tests the ListBalances method
func TestBalancesEntity_ListBalances(t *testing.T) {
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
		checkRequest   func(t *testing.T, req *http.Request)
	}{
		{
			name:     "Success with no options",
			orgID:    "org-123",
			ledgerID: "ledger-456",
			opts:     nil,
			mockResponse: `{
				"items": [
					{
						"id": "bal-123",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"alias": "@test",
						"assetCode": "USD",
						"available": "1000000",
						"onHold": "50000",
						"version": 1,
						"accountType": "LIABILITY",
						"allowSending": true,
						"allowReceiving": true
					},
					{
						"id": "bal-456",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-012",
						"alias": "@test2",
						"assetCode": "EUR",
						"available": "2000000",
						"onHold": "0",
						"version": 1,
						"accountType": "ASSET",
						"allowSending": true,
						"allowReceiving": true
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
			name:     "Success with pagination options",
			orgID:    "org-123",
			ledgerID: "ledger-456",
			opts: &models.ListOptions{
				Limit:  5,
				Offset: 10,
			},
			mockResponse: `{
				"items": [
					{
						"id": "bal-123",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"assetCode": "USD",
						"available": "1000000",
						"onHold": "0",
						"version": 1
					}
				],
				"pagination": {
					"total": 11,
					"limit": 5,
					"offset": 10
				}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  1,
			checkRequest: func(t *testing.T, req *http.Request) {
				t.Helper()
				assert.Contains(t, req.URL.RawQuery, "limit=5")
				assert.Contains(t, req.URL.RawQuery, "offset=10")
			},
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-456",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			expectedError: true,
		},
		{
			name:           "API error - internal server error",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
			expectedError:  true,
		},
		{
			name:           "API error - not found",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Ledger not found"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			mockError:     errors.New("connection refused"),
			expectedError: true,
		},
		{
			name:     "Empty list response",
			orgID:    "org-123",
			ledgerID: "ledger-456",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedReq *http.Request

			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					capturedReq = req

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

			entity := &balancesEntity{
				httpClient: newBalancesHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
			}

			result, err := entity.ListBalances(context.Background(), tt.orgID, tt.ledgerID, tt.opts)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Len(t, result.Items, tt.expectedItems)

			if tt.checkRequest != nil {
				tt.checkRequest(t, capturedReq)
			}
		})
	}
}

// TestBalancesEntity_ListAccountBalances tests the ListAccountBalances method
func TestBalancesEntity_ListAccountBalances(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		accountID      string
		opts           *models.ListOptions
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		expectedItems  int
	}{
		{
			name:      "Success with no options",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			accountID: "acc-789",
			opts:      nil,
			mockResponse: `{
				"items": [
					{
						"id": "bal-123",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"assetCode": "USD",
						"available": "1000000",
						"onHold": "0",
						"version": 1
					},
					{
						"id": "bal-456",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"assetCode": "EUR",
						"available": "500000",
						"onHold": "10000",
						"version": 1
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
			name:      "Success with pagination options",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			accountID: "acc-789",
			opts: &models.ListOptions{
				Limit:  5,
				Offset: 0,
			},
			mockResponse: `{
				"items": [
					{
						"id": "bal-123",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"assetCode": "USD",
						"available": "1000000",
						"onHold": "0",
						"version": 1
					}
				],
				"pagination": {
					"total": 1,
					"limit": 5,
					"offset": 0
				}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  1,
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-456",
			accountID:     "acc-789",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			accountID:     "acc-789",
			expectedError: true,
		},
		{
			name:          "Empty account ID",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			accountID:     "",
			expectedError: true,
		},
		{
			name:           "API error - account not found",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			accountID:      "acc-not-found",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Account not found"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			accountID:     "acc-789",
			mockError:     errors.New("connection refused"),
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

			entity := &balancesEntity{
				httpClient: newBalancesHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
			}

			result, err := entity.ListAccountBalances(context.Background(), tt.orgID, tt.ledgerID, tt.accountID, tt.opts)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Len(t, result.Items, tt.expectedItems)
		})
	}
}

// TestBalancesEntity_GetBalance tests the GetBalance method
func TestBalancesEntity_GetBalance(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		balanceID      string
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		expectedID     string
	}{
		{
			name:      "Success",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			balanceID: "bal-789",
			mockResponse: `{
				"id": "bal-789",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"accountId": "acc-012",
				"alias": "@test",
				"key": "primary",
				"assetCode": "USD",
				"available": "1000000",
				"onHold": "50000",
				"version": 1,
				"accountType": "LIABILITY",
				"allowSending": true,
				"allowReceiving": true,
				"metadata": {"key": "value"}
			}`,
			mockStatusCode: http.StatusOK,
			expectedID:     "bal-789",
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-456",
			balanceID:     "bal-789",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			balanceID:     "bal-789",
			expectedError: true,
		},
		{
			name:          "Empty balance ID",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			balanceID:     "",
			expectedError: true,
		},
		{
			name:           "Balance not found",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			balanceID:      "bal-not-found",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Balance not found"}`,
			expectedError:  true,
		},
		{
			name:           "Unauthorized",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			balanceID:      "bal-789",
			mockStatusCode: http.StatusUnauthorized,
			mockResponse:   `{"error": "Unauthorized"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			balanceID:     "bal-789",
			mockError:     errors.New("connection timeout"),
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

			entity := &balancesEntity{
				httpClient: newBalancesHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
			}

			result, err := entity.GetBalance(context.Background(), tt.orgID, tt.ledgerID, tt.balanceID)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedID, result.ID)
			assert.Equal(t, "USD", result.AssetCode)
			assert.Equal(t, "LIABILITY", result.AccountType)
			assert.True(t, result.AllowSending)
			assert.True(t, result.AllowReceiving)
		})
	}
}

// TestBalancesEntity_CreateBalance tests the CreateBalance method
func TestBalancesEntity_CreateBalance(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		accountID      string
		input          *models.CreateBalanceInput
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		expectedID     string
		checkRequest   func(t *testing.T, req *http.Request)
	}{
		{
			name:      "Success",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			accountID: "acc-789",
			input:     models.NewCreateBalanceInput("frozen-funds").WithAllowSending(false).WithAllowReceiving(true),
			mockResponse: `{
				"id": "bal-new",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"accountId": "acc-789",
				"key": "frozen-funds",
				"assetCode": "USD",
				"available": "0",
				"onHold": "0",
				"version": 1,
				"allowSending": false,
				"allowReceiving": true
			}`,
			mockStatusCode: http.StatusOK,
			expectedID:     "bal-new",
			checkRequest: func(t *testing.T, req *http.Request) {
				t.Helper()
				assert.Equal(t, http.MethodPost, req.Method)
				assert.Contains(t, req.URL.Path, "/accounts/acc-789/balances")
			},
		},
		{
			name:      "Success with minimal input",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			accountID: "acc-789",
			input:     models.NewCreateBalanceInput("secondary"),
			mockResponse: `{
				"id": "bal-secondary",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"accountId": "acc-789",
				"key": "secondary",
				"assetCode": "USD",
				"available": "0",
				"onHold": "0",
				"version": 1,
				"allowSending": true,
				"allowReceiving": true
			}`,
			mockStatusCode: http.StatusOK,
			expectedID:     "bal-secondary",
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-456",
			accountID:     "acc-789",
			input:         models.NewCreateBalanceInput("test"),
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			accountID:     "acc-789",
			input:         models.NewCreateBalanceInput("test"),
			expectedError: true,
		},
		{
			name:          "Empty account ID",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			accountID:     "",
			input:         models.NewCreateBalanceInput("test"),
			expectedError: true,
		},
		{
			name:          "Nil input",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			accountID:     "acc-789",
			input:         nil,
			expectedError: true,
		},
		{
			name:          "Invalid input - empty key",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			accountID:     "acc-789",
			input:         &models.CreateBalanceInput{Key: ""},
			expectedError: true,
		},
		{
			name:           "API error - conflict",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			accountID:      "acc-789",
			input:          models.NewCreateBalanceInput("existing"),
			mockStatusCode: http.StatusConflict,
			mockResponse:   `{"error": "Balance with this key already exists"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			accountID:     "acc-789",
			input:         models.NewCreateBalanceInput("test"),
			mockError:     errors.New("connection refused"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedReq *http.Request

			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					capturedReq = req

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

			entity := &balancesEntity{
				httpClient: newBalancesHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
			}

			result, err := entity.CreateBalance(context.Background(), tt.orgID, tt.ledgerID, tt.accountID, tt.input)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedID, result.ID)

			if tt.checkRequest != nil {
				tt.checkRequest(t, capturedReq)
			}
		})
	}
}

// TestBalancesEntity_UpdateBalance tests the UpdateBalance method
func TestBalancesEntity_UpdateBalance(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		balanceID      string
		input          *models.UpdateBalanceInput
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		expectedID     string
		checkRequest   func(t *testing.T, req *http.Request)
	}{
		{
			name:      "Success with metadata",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			balanceID: "bal-789",
			input:     models.NewUpdateBalanceInput().WithMetadata(map[string]any{"updated": "true", "key": "new-value"}),
			mockResponse: `{
				"id": "bal-789",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"accountId": "acc-012",
				"assetCode": "USD",
				"available": "1000000",
				"onHold": "50000",
				"version": 2,
				"metadata": {"updated": "true", "key": "new-value"}
			}`,
			mockStatusCode: http.StatusOK,
			expectedID:     "bal-789",
			checkRequest: func(t *testing.T, req *http.Request) {
				t.Helper()
				assert.Equal(t, http.MethodPatch, req.Method)
			},
		},
		{
			name:      "Success with empty metadata",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			balanceID: "bal-789",
			input:     models.NewUpdateBalanceInput().WithMetadata(map[string]any{}),
			mockResponse: `{
				"id": "bal-789",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"accountId": "acc-012",
				"assetCode": "USD",
				"available": "1000000",
				"onHold": "50000",
				"version": 2,
				"metadata": {}
			}`,
			mockStatusCode: http.StatusOK,
			expectedID:     "bal-789",
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-456",
			balanceID:     "bal-789",
			input:         models.NewUpdateBalanceInput(),
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			balanceID:     "bal-789",
			input:         models.NewUpdateBalanceInput(),
			expectedError: true,
		},
		{
			name:          "Empty balance ID",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			balanceID:     "",
			input:         models.NewUpdateBalanceInput(),
			expectedError: true,
		},
		{
			name:          "Nil input",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			balanceID:     "bal-789",
			input:         nil,
			expectedError: true,
		},
		{
			name:           "Balance not found",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			balanceID:      "bal-not-found",
			input:          models.NewUpdateBalanceInput(),
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Balance not found"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			balanceID:     "bal-789",
			input:         models.NewUpdateBalanceInput(),
			mockError:     errors.New("connection refused"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedReq *http.Request

			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					capturedReq = req

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

			entity := &balancesEntity{
				httpClient: newBalancesHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
			}

			result, err := entity.UpdateBalance(context.Background(), tt.orgID, tt.ledgerID, tt.balanceID, tt.input)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedID, result.ID)

			if tt.checkRequest != nil {
				tt.checkRequest(t, capturedReq)
			}
		})
	}
}

// TestBalancesEntity_DeleteBalance tests the DeleteBalance method
func TestBalancesEntity_DeleteBalance(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		balanceID      string
		mockStatusCode int
		mockResponse   string
		mockError      error
		expectedError  bool
		checkRequest   func(t *testing.T, req *http.Request)
	}{
		{
			name:           "Success",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			balanceID:      "bal-789",
			mockStatusCode: http.StatusOK,
			mockResponse:   "",
			checkRequest: func(t *testing.T, req *http.Request) {
				t.Helper()
				assert.Equal(t, http.MethodDelete, req.Method)
				assert.Contains(t, req.URL.Path, "/balances/bal-789")
			},
		},
		{
			name:           "Success with no content",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			balanceID:      "bal-789",
			mockStatusCode: http.StatusNoContent,
			mockResponse:   "",
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-456",
			balanceID:     "bal-789",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			balanceID:     "bal-789",
			expectedError: true,
		},
		{
			name:          "Empty balance ID",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			balanceID:     "",
			expectedError: true,
		},
		{
			name:           "Balance not found",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			balanceID:      "bal-not-found",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Balance not found"}`,
			expectedError:  true,
		},
		{
			name:           "Forbidden",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			balanceID:      "bal-789",
			mockStatusCode: http.StatusForbidden,
			mockResponse:   `{"error": "Forbidden"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			balanceID:     "bal-789",
			mockError:     errors.New("connection refused"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedReq *http.Request

			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					capturedReq = req

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

			entity := &balancesEntity{
				httpClient: newBalancesHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
			}

			err := entity.DeleteBalance(context.Background(), tt.orgID, tt.ledgerID, tt.balanceID)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.checkRequest != nil {
				tt.checkRequest(t, capturedReq)
			}
		})
	}
}

// TestBalancesEntity_ListBalancesByAccountAlias tests the ListBalancesByAccountAlias method
func TestBalancesEntity_ListBalancesByAccountAlias(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		alias          string
		opts           *models.ListOptions
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		expectedItems  int
		checkRequest   func(t *testing.T, req *http.Request)
	}{
		{
			name:     "Success with no options",
			orgID:    "org-123",
			ledgerID: "ledger-456",
			alias:    "@person1",
			opts:     nil,
			mockResponse: `{
				"items": [
					{
						"id": "bal-123",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"alias": "@person1",
						"assetCode": "USD",
						"available": "1000000",
						"onHold": "50000",
						"version": 1
					}
				],
				"pagination": {
					"total": 1,
					"limit": 10,
					"offset": 0
				}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  1,
			checkRequest: func(t *testing.T, req *http.Request) {
				t.Helper()
				assert.Contains(t, req.URL.Path, "/accounts/alias/@person1/balances")
			},
		},
		{
			name:     "Success with pagination options",
			orgID:    "org-123",
			ledgerID: "ledger-456",
			alias:    "my-account",
			opts: &models.ListOptions{
				Limit:  5,
				Offset: 0,
			},
			mockResponse: `{
				"items": [
					{
						"id": "bal-123",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"alias": "my-account",
						"assetCode": "USD",
						"available": "1000000",
						"onHold": "0",
						"version": 1
					},
					{
						"id": "bal-456",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"alias": "my-account",
						"assetCode": "EUR",
						"available": "500000",
						"onHold": "0",
						"version": 1
					}
				],
				"pagination": {
					"total": 2,
					"limit": 5,
					"offset": 0
				}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  2,
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-456",
			alias:         "@person1",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			alias:         "@person1",
			expectedError: true,
		},
		{
			name:          "Empty alias",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			alias:         "",
			expectedError: true,
		},
		{
			name:           "Account not found by alias",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			alias:          "@unknown",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Account not found"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			alias:         "@person1",
			mockError:     errors.New("connection refused"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedReq *http.Request

			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					capturedReq = req

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

			entity := &balancesEntity{
				httpClient: newBalancesHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
			}

			result, err := entity.ListBalancesByAccountAlias(context.Background(), tt.orgID, tt.ledgerID, tt.alias, tt.opts)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Len(t, result.Items, tt.expectedItems)

			if tt.checkRequest != nil {
				tt.checkRequest(t, capturedReq)
			}
		})
	}
}

// TestBalancesEntity_ListBalancesByExternalCode tests the ListBalancesByExternalCode method
func TestBalancesEntity_ListBalancesByExternalCode(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		code           string
		opts           *models.ListOptions
		mockResponse   string
		mockStatusCode int
		mockError      error
		expectedError  bool
		expectedItems  int
		checkRequest   func(t *testing.T, req *http.Request)
	}{
		{
			name:     "Success with no options",
			orgID:    "org-123",
			ledgerID: "ledger-456",
			code:     "EXT-001",
			opts:     nil,
			mockResponse: `{
				"items": [
					{
						"id": "bal-123",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"assetCode": "USD",
						"available": "1000000",
						"onHold": "50000",
						"version": 1
					}
				],
				"pagination": {
					"total": 1,
					"limit": 10,
					"offset": 0
				}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  1,
			checkRequest: func(t *testing.T, req *http.Request) {
				t.Helper()
				assert.Contains(t, req.URL.Path, "/accounts/external/EXT-001/balances")
			},
		},
		{
			name:     "Success with pagination options",
			orgID:    "org-123",
			ledgerID: "ledger-456",
			code:     "customer123456",
			opts: &models.ListOptions{
				Limit:  10,
				Offset: 0,
			},
			mockResponse: `{
				"items": [
					{
						"id": "bal-123",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"assetCode": "USD",
						"available": "1000000",
						"onHold": "0",
						"version": 1
					},
					{
						"id": "bal-456",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"assetCode": "EUR",
						"available": "500000",
						"onHold": "0",
						"version": 1
					},
					{
						"id": "bal-789",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"assetCode": "GBP",
						"available": "250000",
						"onHold": "0",
						"version": 1
					}
				],
				"pagination": {
					"total": 3,
					"limit": 10,
					"offset": 0
				}
			}`,
			mockStatusCode: http.StatusOK,
			expectedItems:  3,
		},
		{
			name:          "Empty organization ID",
			orgID:         "",
			ledgerID:      "ledger-456",
			code:          "EXT-001",
			expectedError: true,
		},
		{
			name:          "Empty ledger ID",
			orgID:         "org-123",
			ledgerID:      "",
			code:          "EXT-001",
			expectedError: true,
		},
		{
			name:          "Empty external code",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			code:          "",
			expectedError: true,
		},
		{
			name:           "Account not found by external code",
			orgID:          "org-123",
			ledgerID:       "ledger-456",
			code:           "unknown-code",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "Account not found"}`,
			expectedError:  true,
		},
		{
			name:          "HTTP client error",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			code:          "EXT-001",
			mockError:     errors.New("connection refused"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedReq *http.Request

			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					capturedReq = req

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

			entity := &balancesEntity{
				httpClient: newBalancesHTTPClientAdapter(mockClient),
				baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
			}

			result, err := entity.ListBalancesByExternalCode(context.Background(), tt.orgID, tt.ledgerID, tt.code, tt.opts)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Len(t, result.Items, tt.expectedItems)

			if tt.checkRequest != nil {
				tt.checkRequest(t, capturedReq)
			}
		})
	}
}

// TestBalancesEntity_HTTPServerIntegration tests with actual httptest server
func TestBalancesEntity_HTTPServerIntegration(t *testing.T) {
	t.Run("ListBalances with httptest server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/organizations/org-123/ledgers/ledger-456/balances")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"items": [
					{
						"id": "bal-123",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"assetCode": "USD",
						"available": "1000000",
						"onHold": "0",
						"version": 1
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

		entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		result, err := entity.ListBalances(context.Background(), "org-123", "ledger-456", nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Items, 1)
		assert.Equal(t, "bal-123", result.Items[0].ID)
	})

	t.Run("GetBalance with httptest server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/balances/bal-789")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"id": "bal-789",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"accountId": "acc-012",
				"assetCode": "USD",
				"available": "1000000",
				"onHold": "50000",
				"version": 1,
				"allowSending": true,
				"allowReceiving": true
			}`))
			assert.NoError(t, err)
		}))
		defer server.Close()

		entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		result, err := entity.GetBalance(context.Background(), "org-123", "ledger-456", "bal-789")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "bal-789", result.ID)
		assert.Equal(t, "USD", result.AssetCode)
	})

	t.Run("CreateBalance with httptest server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/accounts/acc-789/balances")

			var input models.CreateBalanceInput

			err := json.NewDecoder(r.Body).Decode(&input)
			assert.NoError(t, err)
			assert.Equal(t, "frozen-funds", input.Key)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte(`{
				"id": "bal-new",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"accountId": "acc-789",
				"key": "frozen-funds",
				"assetCode": "USD",
				"available": "0",
				"onHold": "0",
				"version": 1,
				"allowSending": false,
				"allowReceiving": true
			}`))
			assert.NoError(t, err)
		}))
		defer server.Close()

		entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		input := models.NewCreateBalanceInput("frozen-funds").WithAllowSending(false).WithAllowReceiving(true)
		result, err := entity.CreateBalance(context.Background(), "org-123", "ledger-456", "acc-789", input)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "bal-new", result.ID)
		assert.Equal(t, "frozen-funds", result.Key)
	})

	t.Run("UpdateBalance with httptest server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPatch, r.Method)
			assert.Contains(t, r.URL.Path, "/balances/bal-789")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"id": "bal-789",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"accountId": "acc-012",
				"assetCode": "USD",
				"available": "1000000",
				"onHold": "50000",
				"version": 2,
				"metadata": {"updated": "true"}
			}`))
			assert.NoError(t, err)
		}))
		defer server.Close()

		entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		input := models.NewUpdateBalanceInput().WithMetadata(map[string]any{"updated": "true"})
		result, err := entity.UpdateBalance(context.Background(), "org-123", "ledger-456", "bal-789", input)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "bal-789", result.ID)
		assert.Equal(t, int64(2), result.Version)
	})

	t.Run("DeleteBalance with httptest server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Contains(t, r.URL.Path, "/balances/bal-789")

			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		err := entity.DeleteBalance(context.Background(), "org-123", "ledger-456", "bal-789")
		require.NoError(t, err)
	})

	t.Run("ListBalancesByAccountAlias with httptest server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/accounts/alias/@person1/balances")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"items": [
					{
						"id": "bal-123",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"alias": "@person1",
						"assetCode": "USD",
						"available": "1000000",
						"onHold": "0",
						"version": 1
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

		entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		result, err := entity.ListBalancesByAccountAlias(context.Background(), "org-123", "ledger-456", "@person1", nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Items, 1)
		assert.Equal(t, "@person1", result.Items[0].Alias)
	})

	t.Run("ListBalancesByExternalCode with httptest server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/accounts/external/EXT-001/balances")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"items": [
					{
						"id": "bal-123",
						"organizationId": "org-123",
						"ledgerId": "ledger-456",
						"accountId": "acc-789",
						"assetCode": "USD",
						"available": "1000000",
						"onHold": "0",
						"version": 1
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

		entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		result, err := entity.ListBalancesByExternalCode(context.Background(), "org-123", "ledger-456", "EXT-001", nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Items, 1)
	})
}

// TestBalancesEntity_ErrorHandling tests various error scenarios
func TestBalancesEntity_ErrorHandling(t *testing.T) {
	testCases := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedErrMsg string
	}{
		{
			name:           "Bad Request",
			statusCode:     http.StatusBadRequest,
			responseBody:   `{"error": "Invalid request parameters"}`,
			expectedErrMsg: "Invalid request parameters",
		},
		{
			name:           "Unauthorized",
			statusCode:     http.StatusUnauthorized,
			responseBody:   `{"error": "Invalid authentication token"}`,
			expectedErrMsg: "Invalid authentication token",
		},
		{
			name:           "Forbidden",
			statusCode:     http.StatusForbidden,
			responseBody:   `{"error": "Access denied"}`,
			expectedErrMsg: "Access denied",
		},
		{
			name:           "Not Found",
			statusCode:     http.StatusNotFound,
			responseBody:   `{"error": "Resource not found"}`,
			expectedErrMsg: "Resource not found",
		},
		{
			name:           "Internal Server Error",
			statusCode:     http.StatusInternalServerError,
			responseBody:   `{"error": "Internal server error"}`,
			expectedErrMsg: "Internal server error",
		},
		{
			name:           "Service Unavailable",
			statusCode:     http.StatusServiceUnavailable,
			responseBody:   `{"error": "Service temporarily unavailable"}`,
			expectedErrMsg: "Service temporarily unavailable",
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

			entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
				"transaction": server.URL,
			})

			_, err := entity.GetBalance(context.Background(), "org-123", "ledger-456", "bal-789")
			require.Error(t, err)
		})
	}
}

// TestBalancesEntity_ContextCancellation tests context cancellation handling
func TestBalancesEntity_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
		"transaction": server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := entity.ListBalances(ctx, "org-123", "ledger-456", nil)
	require.Error(t, err)
}

// TestBalancesEntity_ContextTimeout tests context timeout handling
func TestBalancesEntity_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
		"transaction": server.URL,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := entity.GetBalance(ctx, "org-123", "ledger-456", "bal-789")
	require.Error(t, err)
}

// TestBalancesEntity_QueryParameterEncoding tests query parameter encoding
func TestBalancesEntity_QueryParameterEncoding(t *testing.T) {
	var capturedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"items": [],
			"pagination": {"total": 0, "limit": 10, "offset": 0}
		}`))
	}))
	defer server.Close()

	entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
		"transaction": server.URL,
	})

	opts := &models.ListOptions{
		Limit:          25,
		Offset:         50,
		OrderBy:        "createdAt",
		OrderDirection: "desc",
	}

	_, err := entity.ListBalances(context.Background(), "org-123", "ledger-456", opts)
	require.NoError(t, err)

	assert.Contains(t, capturedURL, "limit=25")
	assert.Contains(t, capturedURL, "offset=50")
}

// TestBalancesEntity_JSONResponseParsing tests JSON response parsing
func TestBalancesEntity_JSONResponseParsing(t *testing.T) {
	t.Run("Parse balance with all fields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "bal-123",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"accountId": "acc-789",
				"alias": "@test-account",
				"key": "primary",
				"assetCode": "USD",
				"available": "1000000",
				"onHold": "50000",
				"version": 5,
				"accountType": "LIABILITY",
				"allowSending": true,
				"allowReceiving": false,
				"createdAt": "2024-01-15T10:30:00Z",
				"updatedAt": "2024-01-20T14:45:00Z",
				"metadata": {
					"department": "finance",
					"costCenter": "CC001"
				}
			}`))
		}))
		defer server.Close()

		entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		result, err := entity.GetBalance(context.Background(), "org-123", "ledger-456", "bal-123")
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "bal-123", result.ID)
		assert.Equal(t, "org-123", result.OrganizationID)
		assert.Equal(t, "ledger-456", result.LedgerID)
		assert.Equal(t, "acc-789", result.AccountID)
		assert.Equal(t, "@test-account", result.Alias)
		assert.Equal(t, "primary", result.Key)
		assert.Equal(t, "USD", result.AssetCode)
		assert.True(t, result.Available.Equal(decimal.NewFromInt(1000000)))
		assert.True(t, result.OnHold.Equal(decimal.NewFromInt(50000)))
		assert.Equal(t, int64(5), result.Version)
		assert.Equal(t, "LIABILITY", result.AccountType)
		assert.True(t, result.AllowSending)
		assert.False(t, result.AllowReceiving)
		assert.Equal(t, "finance", result.Metadata["department"])
		assert.Equal(t, "CC001", result.Metadata["costCenter"])
	})

	t.Run("Parse balance with minimal fields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "bal-minimal",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"accountId": "acc-789",
				"assetCode": "EUR",
				"available": "0",
				"onHold": "0",
				"version": 1
			}`))
		}))
		defer server.Close()

		entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		result, err := entity.GetBalance(context.Background(), "org-123", "ledger-456", "bal-minimal")
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "bal-minimal", result.ID)
		assert.Equal(t, "EUR", result.AssetCode)
		assert.True(t, result.Available.Equal(decimal.Zero))
		assert.True(t, result.OnHold.Equal(decimal.Zero))
	})
}

// TestBalancesEntity_ListOptionsFilters tests filtering options
func TestBalancesEntity_ListOptionsFilters(t *testing.T) {
	var capturedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"items": [],
			"pagination": {"total": 0, "limit": 10, "offset": 0}
		}`))
	}))
	defer server.Close()

	entity := NewBalancesEntity(server.Client(), "test-token", map[string]string{
		"transaction": server.URL,
	})

	t.Run("With filters", func(t *testing.T) {
		opts := &models.ListOptions{
			Limit:   10,
			Offset:  0,
			Filters: map[string]string{"assetCode": "USD"},
		}

		_, err := entity.ListBalances(context.Background(), "org-123", "ledger-456", opts)
		require.NoError(t, err)
		assert.Contains(t, capturedQuery, "assetCode=USD")
	})
}

// TestCreateBalanceInput_Validation tests validation of CreateBalanceInput
func TestCreateBalanceInput_Validation(t *testing.T) {
	tests := []struct {
		name          string
		input         *models.CreateBalanceInput
		expectedError bool
	}{
		{
			name:          "Valid input with key only",
			input:         models.NewCreateBalanceInput("primary"),
			expectedError: false,
		},
		{
			name:          "Valid input with all fields",
			input:         models.NewCreateBalanceInput("frozen").WithAllowSending(false).WithAllowReceiving(true),
			expectedError: false,
		},
		{
			name:          "Invalid input - empty key",
			input:         &models.CreateBalanceInput{Key: ""},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestUpdateBalanceInput_Validation tests validation of UpdateBalanceInput
func TestUpdateBalanceInput_Validation(t *testing.T) {
	tests := []struct {
		name          string
		input         *models.UpdateBalanceInput
		expectedError bool
	}{
		{
			name:          "Valid input with metadata",
			input:         models.NewUpdateBalanceInput().WithMetadata(map[string]any{"key": "value"}),
			expectedError: false,
		},
		{
			name:          "Valid input with empty metadata",
			input:         models.NewUpdateBalanceInput().WithMetadata(map[string]any{}),
			expectedError: false,
		},
		{
			name:          "Valid input with nil metadata",
			input:         models.NewUpdateBalanceInput(),
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

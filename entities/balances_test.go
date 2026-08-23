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

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_newBalancesEntity tests the constructor for balances entity
func Test_newBalancesEntity(t *testing.T) {
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
			service := newBalancesEntity(tt.client, tt.authToken, tt.baseURLs)
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
	entity := &balancesEntity{serviceEntity: serviceEntity{baseURLs: map[string]string{"transaction": "https://api.example.com/v1"}}}

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
	entity := &balancesEntity{serviceEntity: serviceEntity{baseURLs: map[string]string{"transaction": "https://api.example.com/v1"}}}

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
	entity := &balancesEntity{serviceEntity: serviceEntity{baseURLs: map[string]string{"transaction": "https://api.example.com/v1"}}}

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
	entity := &balancesEntity{serviceEntity: serviceEntity{baseURLs: map[string]string{"transaction": "https://api.example.com/v1"}}}

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

// TestBalancesEntity_ListBalances tests the ListBalances method
func TestBalancesEntity_ListBalances(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		opts           models.BalancesListOpts
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
			opts:     models.BalancesListOpts{},
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
			opts:     models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 5, Page: 3}},
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
				assert.Contains(t, req.URL.RawQuery, "page=3")
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

			entity := &balancesEntity{serviceEntity: serviceEntity{httpClient: newBalancesHTTPClientAdapter(mockClient), baseURLs: map[string]string{"transaction": "https://api.example.com/v1"}}}

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
		opts           models.BalancesListOpts
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
			opts:      models.BalancesListOpts{},
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
			opts:      models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 5, Page: 1}},
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

			entity := &balancesEntity{serviceEntity: serviceEntity{httpClient: newBalancesHTTPClientAdapter(mockClient), baseURLs: map[string]string{"transaction": "https://api.example.com/v1"}}}

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

			entity := &balancesEntity{serviceEntity: serviceEntity{httpClient: newBalancesHTTPClientAdapter(mockClient), baseURLs: map[string]string{"transaction": "https://api.example.com/v1"}}}

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

			entity := &balancesEntity{serviceEntity: serviceEntity{httpClient: newBalancesHTTPClientAdapter(mockClient), baseURLs: map[string]string{"transaction": "https://api.example.com/v1"}}}

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
			name:      "Success with transfer flags",
			orgID:     "org-123",
			ledgerID:  "ledger-456",
			balanceID: "bal-789",
			input:     models.NewUpdateBalanceInput().WithAllowSending(false).WithAllowReceiving(true),
			mockResponse: `{
				"id": "bal-789",
				"organizationId": "org-123",
				"ledgerId": "ledger-456",
				"accountId": "acc-012",
				"assetCode": "USD",
				"available": "1000000",
				"onHold": "50000",
				"version": 2,
				"allowSending": false,
				"allowReceiving": true
			}`,
			mockStatusCode: http.StatusOK,
			expectedID:     "bal-789",
			checkRequest: func(t *testing.T, req *http.Request) {
				t.Helper()
				assert.Equal(t, http.MethodPatch, req.Method)
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				assert.JSONEq(t, `{"allowSending":false,"allowReceiving":true}`, string(body))
				assert.NotContains(t, string(body), "metadata")
			},
		},
		{
			name:          "Metadata update is rejected",
			orgID:         "org-123",
			ledgerID:      "ledger-456",
			balanceID:     "bal-789",
			input:         models.NewUpdateBalanceInput().WithMetadata(map[string]any{"legacy": "ignored"}),
			expectedError: true,
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

			entity := &balancesEntity{serviceEntity: serviceEntity{httpClient: newBalancesHTTPClientAdapter(mockClient), baseURLs: map[string]string{"transaction": "https://api.example.com/v1"}}}

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

func TestBalancesEntity_History_RequestConstruction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.EscapedPath() {
		case "/organizations/org%2F1/ledgers/ledger%2F1/balances/balance%2F1/history":
			assert.Equal(t, "2026-01-02T03:04:05Z", r.URL.Query().Get("date"))

			_, err := w.Write([]byte(`{}`))
			assert.NoError(t, err)
		case "/organizations/org%2F1/ledgers/ledger%2F1/accounts/account%2F1/balances/history":
			assert.Equal(t, "2026-01-02T03:04:05Z", r.URL.Query().Get("date"))

			_, err := w.Write([]byte(`[{}]`))
			assert.NoError(t, err)
		default:
			t.Fatalf("unexpected path: %s", r.URL.EscapedPath())
		}
	}))
	defer server.Close()

	service := newBalancesEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
	history, err := service.GetBalanceHistory(context.Background(), "org/1", "ledger/1", "balance/1", "2026-01-02T03:04:05Z")
	require.NoError(t, err)
	require.NotNil(t, history)

	histories, err := service.GetAccountBalancesHistory(context.Background(), "org/1", "ledger/1", "account/1", "2026-01-02T03:04:05Z")
	require.NoError(t, err)
	require.Len(t, histories, 1)
}

func TestValidateBalanceHistoryDateRequiresExplicitTimezone(t *testing.T) {
	require.NoError(t, validateBalanceHistoryDate("2026-01-02T03:04:05Z"))
	require.NoError(t, validateBalanceHistoryDate("2026-01-02T03:04:05.123456789-03:00"))

	for _, date := range []string{"2026-01-02", "2026-01-02 03:04:05", "2026-01-02T03:04:05"} {
		t.Run(date, func(t *testing.T) {
			err := validateBalanceHistoryDate(date)
			require.Error(t, err)
			require.Contains(t, err.Error(), "RFC3339")
			require.Contains(t, err.Error(), "explicit timezone")
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

			entity := &balancesEntity{serviceEntity: serviceEntity{httpClient: newBalancesHTTPClientAdapter(mockClient), baseURLs: map[string]string{"transaction": "https://api.example.com/v1"}}}

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
		opts           models.BalancesListOpts
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
			opts:     models.BalancesListOpts{},
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
			opts:     models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 5, Page: 1}},
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

			entity := &balancesEntity{serviceEntity: serviceEntity{httpClient: newBalancesHTTPClientAdapter(mockClient), baseURLs: map[string]string{"transaction": "https://api.example.com/v1"}}}

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

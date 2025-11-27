package entities

import (
	"context"
	"encoding/json"
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

// Test constants for operations
const (
	opTestOrgID         = "org-123"
	opTestLedgerID      = "ledger-456"
	opTestAccountID     = "acc-789"
	opTestOperationID   = "op-abc"
	opTestTransactionID = "tx-xyz"
)

// createTestOperation returns a sample operation for testing
func createTestOperation() models.Operation {
	now := time.Now().UTC()
	availableValue := decimal.NewFromInt(10000)
	onHoldValue := decimal.NewFromInt(500)
	amountValue := decimal.NewFromInt(1500)

	return models.Operation{
		ID:              opTestOperationID,
		TransactionID:   opTestTransactionID,
		Description:     "Test operation",
		Type:            "DEBIT",
		AssetCode:       "USD",
		ChartOfAccounts: "1000",
		Amount: models.Amount{
			Value: &amountValue,
		},
		Balance: models.OperationBalance{
			Available: &availableValue,
			OnHold:    &onHoldValue,
		},
		BalanceAfter: models.OperationBalance{
			Available: &availableValue,
			OnHold:    &onHoldValue,
		},
		Status: models.Status{
			Code: "ACTIVE",
		},
		AccountID:      opTestAccountID,
		AccountAlias:   "@test-account",
		BalanceID:      "bal-123",
		OrganizationID: opTestOrgID,
		LedgerID:       opTestLedgerID,
		Route:          "route-1",
		CreatedAt:      now,
		UpdatedAt:      now,
		Metadata:       map[string]any{"key": "value"},
	}
}

// createTestOperationsEntity creates a test operations entity with the given server URL
func createTestOperationsEntity(serverURL string) *operationsEntity {
	retryOpts := retry.DefaultOptions()
	_ = retry.WithMaxRetries(1)(retryOpts)
	_ = retry.WithInitialDelay(1 * time.Millisecond)(retryOpts)
	_ = retry.WithMaxDelay(10 * time.Millisecond)(retryOpts)

	httpClient := &HTTPClient{
		client:       &http.Client{Timeout: 5 * time.Second},
		authToken:    "test-token",
		retryOptions: retryOpts,
		jsonPool:     performance.NewJSONPool(),
	}

	return &operationsEntity{
		HTTPClient: httpClient,
		baseURLs: map[string]string{
			"transaction": serverURL,
			"onboarding":  serverURL,
		},
	}
}

// TestNewOperationsEntity tests the NewOperationsEntity constructor
func TestNewOperationsEntity(t *testing.T) {
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
			authToken: "test-token",
			baseURLs:  map[string]string{"transaction": "https://api.example.com"},
		},
		{
			name:      "with empty auth token",
			client:    &http.Client{},
			authToken: "",
			baseURLs:  map[string]string{"transaction": "https://api.example.com"},
		},
		{
			name:      "with multiple base URLs",
			client:    &http.Client{},
			authToken: "token",
			baseURLs: map[string]string{
				"transaction": "https://transaction.example.com",
				"onboarding":  "https://onboarding.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewOperationsEntity(tt.client, tt.authToken, tt.baseURLs)
			require.NotNil(t, service)

			entity, ok := service.(*operationsEntity)
			require.True(t, ok, "service should be *operationsEntity")
			assert.NotNil(t, entity.HTTPClient)
			assert.Equal(t, tt.baseURLs, entity.baseURLs)
		})
	}
}

// TestOperationsEntity_buildURL tests the buildURL helper function
func TestOperationsEntity_buildURL(t *testing.T) {
	entity := &operationsEntity{
		baseURLs: map[string]string{
			"transaction": "https://api.example.com",
		},
	}

	tests := []struct {
		name        string
		orgID       string
		ledgerID    string
		accountID   string
		operationID string
		expected    string
	}{
		{
			name:        "list operations URL (no operation ID)",
			orgID:       "org-123",
			ledgerID:    "ledger-456",
			accountID:   "acc-789",
			operationID: "",
			expected:    "https://api.example.com/organizations/org-123/ledgers/ledger-456/accounts/acc-789/operations",
		},
		{
			name:        "single operation URL (with operation ID)",
			orgID:       "org-123",
			ledgerID:    "ledger-456",
			accountID:   "acc-789",
			operationID: "op-abc",
			expected:    "https://api.example.com/organizations/org-123/ledgers/ledger-456/accounts/acc-789/operations/op-abc",
		},
		{
			name:        "handles trailing slash in base URL",
			orgID:       "org-123",
			ledgerID:    "ledger-456",
			accountID:   "acc-789",
			operationID: "op-abc",
			expected:    "https://api.example.com/organizations/org-123/ledgers/ledger-456/accounts/acc-789/operations/op-abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "handles trailing slash in base URL" {
				entity.baseURLs["transaction"] = "https://api.example.com/"
			} else {
				entity.baseURLs["transaction"] = "https://api.example.com"
			}

			result := entity.buildURL(tt.orgID, tt.ledgerID, tt.accountID, tt.operationID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestOperationsEntity_ListOperations tests ListOperations method
func TestOperationsEntity_ListOperations(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		accountID      string
		opts           *models.ListOptions
		mockResponse   interface{}
		mockStatusCode int
		expectError    bool
		errorContains  string
		expectedItems  int
	}{
		{
			name:      "success with default options",
			orgID:     opTestOrgID,
			ledgerID:  opTestLedgerID,
			accountID: opTestAccountID,
			opts:      nil,
			mockResponse: models.ListResponse[models.Operation]{
				Items: []models.Operation{createTestOperation()},
				Pagination: models.Pagination{
					Total:  1,
					Limit:  10,
					Offset: 0,
				},
			},
			mockStatusCode: http.StatusOK,
			expectedItems:  1,
		},
		{
			name:      "success with pagination options",
			orgID:     opTestOrgID,
			ledgerID:  opTestLedgerID,
			accountID: opTestAccountID,
			opts: &models.ListOptions{
				Limit:          5,
				Offset:         10,
				OrderBy:        "createdAt",
				OrderDirection: "desc",
			},
			mockResponse: models.ListResponse[models.Operation]{
				Items: []models.Operation{createTestOperation(), createTestOperation()},
				Pagination: models.Pagination{
					Total:  2,
					Limit:  5,
					Offset: 10,
				},
			},
			mockStatusCode: http.StatusOK,
			expectedItems:  2,
		},
		{
			name:      "success with filters",
			orgID:     opTestOrgID,
			ledgerID:  opTestLedgerID,
			accountID: opTestAccountID,
			opts: &models.ListOptions{
				Filters: map[string]string{"type": "DEBIT"},
			},
			mockResponse: models.ListResponse[models.Operation]{
				Items: []models.Operation{createTestOperation()},
				Pagination: models.Pagination{
					Total: 1,
					Limit: 10,
				},
			},
			mockStatusCode: http.StatusOK,
			expectedItems:  1,
		},
		{
			name:          "empty organization ID",
			orgID:         "",
			ledgerID:      opTestLedgerID,
			accountID:     opTestAccountID,
			expectError:   true,
			errorContains: "organizationID",
		},
		{
			name:          "empty ledger ID",
			orgID:         opTestOrgID,
			ledgerID:      "",
			accountID:     opTestAccountID,
			expectError:   true,
			errorContains: "ledgerID",
		},
		{
			name:          "empty account ID",
			orgID:         opTestOrgID,
			ledgerID:      opTestLedgerID,
			accountID:     "",
			expectError:   true,
			errorContains: "accountID",
		},
		{
			name:           "server error 500",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			mockResponse:   map[string]string{"error": "Internal server error"},
			mockStatusCode: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name:           "unauthorized 401",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			mockResponse:   map[string]string{"error": "Unauthorized"},
			mockStatusCode: http.StatusUnauthorized,
			expectError:    true,
		},
		{
			name:           "forbidden 403",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			mockResponse:   map[string]string{"error": "Forbidden"},
			mockStatusCode: http.StatusForbidden,
			expectError:    true,
		},
		{
			name:           "not found 404",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			mockResponse:   map[string]string{"error": "Account not found"},
			mockStatusCode: http.StatusNotFound,
			expectError:    true,
		},
		{
			name:           "bad request 400",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			mockResponse:   map[string]string{"error": "Bad request"},
			mockStatusCode: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:      "empty results",
			orgID:     opTestOrgID,
			ledgerID:  opTestLedgerID,
			accountID: opTestAccountID,
			mockResponse: models.ListResponse[models.Operation]{
				Items: []models.Operation{},
				Pagination: models.Pagination{
					Total: 0,
					Limit: 10,
				},
			},
			mockStatusCode: http.StatusOK,
			expectedItems:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.URL.Path, "/organizations/")
				assert.Contains(t, r.URL.Path, "/ledgers/")
				assert.Contains(t, r.URL.Path, "/accounts/")
				assert.Contains(t, r.URL.Path, "/operations")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatusCode)

				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			entity := createTestOperationsEntity(server.URL)

			result, err := entity.ListOperations(context.Background(), tt.orgID, tt.ledgerID, tt.accountID, tt.opts)

			if tt.expectError {
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

// TestOperationsEntity_ListOperations_QueryParams verifies query parameters are properly set
func TestOperationsEntity_ListOperations_QueryParams(t *testing.T) {
	var capturedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(models.ListResponse[models.Operation]{
			Items:      []models.Operation{},
			Pagination: models.Pagination{Total: 0},
		})
	}))
	defer server.Close()

	entity := createTestOperationsEntity(server.URL)

	opts := &models.ListOptions{
		Limit:          25,
		Offset:         50,
		OrderBy:        "createdAt",
		OrderDirection: "desc",
	}

	_, err := entity.ListOperations(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opts)
	require.NoError(t, err)

	assert.Contains(t, capturedURL, "limit=25")
	assert.Contains(t, capturedURL, "offset=50")
}

// TestOperationsEntity_GetOperation tests GetOperation method
func TestOperationsEntity_GetOperation(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		accountID      string
		operationID    string
		transactionID  []string
		mockResponse   interface{}
		mockStatusCode int
		expectError    bool
		errorContains  string
	}{
		{
			name:           "success without transaction ID",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			operationID:    opTestOperationID,
			transactionID:  nil,
			mockResponse:   createTestOperation(),
			mockStatusCode: http.StatusOK,
		},
		{
			name:           "success with transaction ID",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			operationID:    opTestOperationID,
			transactionID:  []string{opTestTransactionID},
			mockResponse:   createTestOperation(),
			mockStatusCode: http.StatusOK,
		},
		{
			name:          "empty organization ID",
			orgID:         "",
			ledgerID:      opTestLedgerID,
			accountID:     opTestAccountID,
			operationID:   opTestOperationID,
			expectError:   true,
			errorContains: "organizationID",
		},
		{
			name:          "empty ledger ID",
			orgID:         opTestOrgID,
			ledgerID:      "",
			accountID:     opTestAccountID,
			operationID:   opTestOperationID,
			expectError:   true,
			errorContains: "ledgerID",
		},
		{
			name:          "empty account ID",
			orgID:         opTestOrgID,
			ledgerID:      opTestLedgerID,
			accountID:     "",
			operationID:   opTestOperationID,
			expectError:   true,
			errorContains: "accountID",
		},
		{
			name:          "empty operation ID",
			orgID:         opTestOrgID,
			ledgerID:      opTestLedgerID,
			accountID:     opTestAccountID,
			operationID:   "",
			expectError:   true,
			errorContains: "operationID",
		},
		{
			name:           "not found 404",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			operationID:    "nonexistent",
			mockResponse:   map[string]string{"error": "Operation not found"},
			mockStatusCode: http.StatusNotFound,
			expectError:    true,
		},
		{
			name:           "unauthorized 401",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			operationID:    opTestOperationID,
			mockResponse:   map[string]string{"error": "Unauthorized"},
			mockStatusCode: http.StatusUnauthorized,
			expectError:    true,
		},
		{
			name:           "forbidden 403",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			operationID:    opTestOperationID,
			mockResponse:   map[string]string{"error": "Forbidden"},
			mockStatusCode: http.StatusForbidden,
			expectError:    true,
		},
		{
			name:           "server error 500",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			operationID:    opTestOperationID,
			mockResponse:   map[string]string{"error": "Internal server error"},
			mockStatusCode: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name:           "bad request 400",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			operationID:    opTestOperationID,
			mockResponse:   map[string]string{"error": "Bad request"},
			mockStatusCode: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.URL.Path, "/operations/")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatusCode)

				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			entity := createTestOperationsEntity(server.URL)

			var (
				result *models.Operation
				err    error
			)

			if len(tt.transactionID) > 0 {
				result, err = entity.GetOperation(context.Background(), tt.orgID, tt.ledgerID, tt.accountID, tt.operationID, tt.transactionID...)
			} else {
				result, err = entity.GetOperation(context.Background(), tt.orgID, tt.ledgerID, tt.accountID, tt.operationID)
			}

			if tt.expectError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, opTestOperationID, result.ID)
			assert.Equal(t, opTestTransactionID, result.TransactionID)
			assert.Equal(t, "DEBIT", result.Type)
			assert.Equal(t, "USD", result.AssetCode)
			assert.Equal(t, opTestAccountID, result.AccountID)
		})
	}
}

// TestOperationsEntity_GetOperation_ResponseFields tests that all response fields are properly parsed
func TestOperationsEntity_GetOperation_ResponseFields(t *testing.T) {
	testOp := createTestOperation()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(testOp)
	}))
	defer server.Close()

	entity := createTestOperationsEntity(server.URL)

	result, err := entity.GetOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, testOp.ID, result.ID)
	assert.Equal(t, testOp.TransactionID, result.TransactionID)
	assert.Equal(t, testOp.Description, result.Description)
	assert.Equal(t, testOp.Type, result.Type)
	assert.Equal(t, testOp.AssetCode, result.AssetCode)
	assert.Equal(t, testOp.ChartOfAccounts, result.ChartOfAccounts)
	assert.Equal(t, testOp.AccountID, result.AccountID)
	assert.Equal(t, testOp.AccountAlias, result.AccountAlias)
	assert.Equal(t, testOp.BalanceID, result.BalanceID)
	assert.Equal(t, testOp.OrganizationID, result.OrganizationID)
	assert.Equal(t, testOp.LedgerID, result.LedgerID)
	assert.Equal(t, testOp.Route, result.Route)
	assert.Equal(t, testOp.Status.Code, result.Status.Code)
	assert.NotNil(t, result.Metadata)
	assert.Equal(t, "value", result.Metadata["key"])
}

// TestOperationsEntity_UpdateOperation tests UpdateOperation method
func TestOperationsEntity_UpdateOperation(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		accountID      string
		operationID    string
		input          interface{}
		mockResponse   interface{}
		mockStatusCode int
		expectError    bool
		errorContains  string
	}{
		{
			name:        "success with description update",
			orgID:       opTestOrgID,
			ledgerID:    opTestLedgerID,
			accountID:   opTestAccountID,
			operationID: opTestOperationID,
			input: models.UpdateOperationInput{
				Description: "Updated description",
			},
			mockResponse:   createTestOperation(),
			mockStatusCode: http.StatusOK,
		},
		{
			name:        "success with metadata update",
			orgID:       opTestOrgID,
			ledgerID:    opTestLedgerID,
			accountID:   opTestAccountID,
			operationID: opTestOperationID,
			input: models.UpdateOperationInput{
				Metadata: map[string]any{"newKey": "newValue"},
			},
			mockResponse:   createTestOperation(),
			mockStatusCode: http.StatusOK,
		},
		{
			name:        "success with full update",
			orgID:       opTestOrgID,
			ledgerID:    opTestLedgerID,
			accountID:   opTestAccountID,
			operationID: opTestOperationID,
			input: models.UpdateOperationInput{
				Description: "Full update",
				Metadata:    map[string]any{"key1": "value1", "key2": "value2"},
			},
			mockResponse:   createTestOperation(),
			mockStatusCode: http.StatusOK,
		},
		{
			name:          "empty organization ID",
			orgID:         "",
			ledgerID:      opTestLedgerID,
			accountID:     opTestAccountID,
			operationID:   opTestOperationID,
			input:         models.UpdateOperationInput{Description: "test"},
			expectError:   true,
			errorContains: "organizationID",
		},
		{
			name:          "empty ledger ID",
			orgID:         opTestOrgID,
			ledgerID:      "",
			accountID:     opTestAccountID,
			operationID:   opTestOperationID,
			input:         models.UpdateOperationInput{Description: "test"},
			expectError:   true,
			errorContains: "ledgerID",
		},
		{
			name:          "empty account ID",
			orgID:         opTestOrgID,
			ledgerID:      opTestLedgerID,
			accountID:     "",
			operationID:   opTestOperationID,
			input:         models.UpdateOperationInput{Description: "test"},
			expectError:   true,
			errorContains: "accountID",
		},
		{
			name:          "empty operation ID",
			orgID:         opTestOrgID,
			ledgerID:      opTestLedgerID,
			accountID:     opTestAccountID,
			operationID:   "",
			input:         models.UpdateOperationInput{Description: "test"},
			expectError:   true,
			errorContains: "operationID",
		},
		{
			name:          "nil input",
			orgID:         opTestOrgID,
			ledgerID:      opTestLedgerID,
			accountID:     opTestAccountID,
			operationID:   opTestOperationID,
			input:         nil,
			expectError:   true,
			errorContains: "input",
		},
		{
			name:           "not found 404",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			operationID:    "nonexistent",
			input:          models.UpdateOperationInput{Description: "test"},
			mockResponse:   map[string]string{"error": "Operation not found"},
			mockStatusCode: http.StatusNotFound,
			expectError:    true,
		},
		{
			name:           "unauthorized 401",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			operationID:    opTestOperationID,
			input:          models.UpdateOperationInput{Description: "test"},
			mockResponse:   map[string]string{"error": "Unauthorized"},
			mockStatusCode: http.StatusUnauthorized,
			expectError:    true,
		},
		{
			name:           "forbidden 403",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			operationID:    opTestOperationID,
			input:          models.UpdateOperationInput{Description: "test"},
			mockResponse:   map[string]string{"error": "Forbidden"},
			mockStatusCode: http.StatusForbidden,
			expectError:    true,
		},
		{
			name:           "server error 500",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			operationID:    opTestOperationID,
			input:          models.UpdateOperationInput{Description: "test"},
			mockResponse:   map[string]string{"error": "Internal server error"},
			mockStatusCode: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name:           "bad request 400",
			orgID:          opTestOrgID,
			ledgerID:       opTestLedgerID,
			accountID:      opTestAccountID,
			operationID:    opTestOperationID,
			input:          models.UpdateOperationInput{Description: "test"},
			mockResponse:   map[string]string{"error": "Bad request"},
			mockStatusCode: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPatch, r.Method)
				assert.Contains(t, r.URL.Path, "/operations/")

				// Verify request body for non-nil inputs
				if tt.input != nil {
					body, err := io.ReadAll(r.Body)
					assert.NoError(t, err)
					assert.NotEmpty(t, body)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatusCode)

				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			entity := createTestOperationsEntity(server.URL)

			result, err := entity.UpdateOperation(context.Background(), tt.orgID, tt.ledgerID, tt.accountID, tt.operationID, tt.input)

			if tt.expectError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, opTestOperationID, result.ID)
		})
	}
}

// TestOperationsEntity_UpdateOperation_RequestBody verifies request body is properly serialized
func TestOperationsEntity_UpdateOperation_RequestBody(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		json.Unmarshal(body, &capturedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(createTestOperation())
	}))
	defer server.Close()

	entity := createTestOperationsEntity(server.URL)

	input := models.UpdateOperationInput{
		Description: "Test description",
		Metadata:    map[string]any{"testKey": "testValue"},
	}

	_, err := entity.UpdateOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID, input)
	require.NoError(t, err)

	assert.Equal(t, "Test description", capturedBody["description"])
	metadata, ok := capturedBody["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "testValue", metadata["testKey"])
}

// TestOperationsEntity_ContextCancellation tests that context cancellation is respected
func TestOperationsEntity_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(createTestOperation())
	}))
	defer server.Close()

	entity := createTestOperationsEntity(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := entity.GetOperation(ctx, opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID)
	require.Error(t, err)
}

// TestOperationsEntity_ValidationEdgeCases tests validation edge cases using table-driven tests
func TestOperationsEntity_ValidationEdgeCases(t *testing.T) {
	entity := createTestOperationsEntity("http://localhost:8080")

	tests := []struct {
		name          string
		method        string
		orgID         string
		ledgerID      string
		accountID     string
		operationID   string
		input         interface{}
		expectError   bool
		errorContains string
	}{
		// ListOperations validation
		{
			name:          "ListOperations - whitespace org ID",
			method:        "ListOperations",
			orgID:         "",
			ledgerID:      opTestLedgerID,
			accountID:     opTestAccountID,
			expectError:   true,
			errorContains: "organizationID",
		},
		{
			name:          "ListOperations - whitespace ledger ID",
			method:        "ListOperations",
			orgID:         opTestOrgID,
			ledgerID:      "",
			accountID:     opTestAccountID,
			expectError:   true,
			errorContains: "ledgerID",
		},
		{
			name:          "ListOperations - whitespace account ID",
			method:        "ListOperations",
			orgID:         opTestOrgID,
			ledgerID:      opTestLedgerID,
			accountID:     "",
			expectError:   true,
			errorContains: "accountID",
		},
		// GetOperation validation
		{
			name:          "GetOperation - all empty",
			method:        "GetOperation",
			orgID:         "",
			ledgerID:      "",
			accountID:     "",
			operationID:   "",
			expectError:   true,
			errorContains: "organizationID",
		},
		{
			name:          "GetOperation - only org ID",
			method:        "GetOperation",
			orgID:         opTestOrgID,
			ledgerID:      "",
			accountID:     "",
			operationID:   "",
			expectError:   true,
			errorContains: "ledgerID",
		},
		{
			name:          "GetOperation - org and ledger ID",
			method:        "GetOperation",
			orgID:         opTestOrgID,
			ledgerID:      opTestLedgerID,
			accountID:     "",
			operationID:   "",
			expectError:   true,
			errorContains: "accountID",
		},
		{
			name:          "GetOperation - missing operation ID",
			method:        "GetOperation",
			orgID:         opTestOrgID,
			ledgerID:      opTestLedgerID,
			accountID:     opTestAccountID,
			operationID:   "",
			expectError:   true,
			errorContains: "operationID",
		},
		// UpdateOperation validation
		{
			name:          "UpdateOperation - nil input",
			method:        "UpdateOperation",
			orgID:         opTestOrgID,
			ledgerID:      opTestLedgerID,
			accountID:     opTestAccountID,
			operationID:   opTestOperationID,
			input:         nil,
			expectError:   true,
			errorContains: "input",
		},
		{
			name:          "UpdateOperation - missing org ID",
			method:        "UpdateOperation",
			orgID:         "",
			ledgerID:      opTestLedgerID,
			accountID:     opTestAccountID,
			operationID:   opTestOperationID,
			input:         models.UpdateOperationInput{Description: "test"},
			expectError:   true,
			errorContains: "organizationID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error

			ctx := context.Background()

			switch tt.method {
			case "ListOperations":
				_, err = entity.ListOperations(ctx, tt.orgID, tt.ledgerID, tt.accountID, nil)
			case "GetOperation":
				_, err = entity.GetOperation(ctx, tt.orgID, tt.ledgerID, tt.accountID, tt.operationID)
			case "UpdateOperation":
				_, err = entity.UpdateOperation(ctx, tt.orgID, tt.ledgerID, tt.accountID, tt.operationID, tt.input)
			}

			if tt.expectError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestOperationsEntity_HTTPErrorCodes tests handling of various HTTP error codes
func TestOperationsEntity_HTTPErrorCodes(t *testing.T) {
	httpErrorCodes := []struct {
		code        int
		description string
	}{
		{http.StatusBadRequest, "Bad Request"},
		{http.StatusUnauthorized, "Unauthorized"},
		{http.StatusForbidden, "Forbidden"},
		{http.StatusNotFound, "Not Found"},
		{http.StatusConflict, "Conflict"},
		{http.StatusUnprocessableEntity, "Unprocessable Entity"},
		{http.StatusTooManyRequests, "Too Many Requests"},
		{http.StatusInternalServerError, "Internal Server Error"},
		{http.StatusBadGateway, "Bad Gateway"},
		{http.StatusServiceUnavailable, "Service Unavailable"},
		{http.StatusGatewayTimeout, "Gateway Timeout"},
	}

	for _, errorCode := range httpErrorCodes {
		t.Run(errorCode.description, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(errorCode.code)
				json.NewEncoder(w).Encode(map[string]string{
					"error": errorCode.description,
				})
			}))
			defer server.Close()

			entity := createTestOperationsEntity(server.URL)

			// Test ListOperations
			_, err := entity.ListOperations(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, nil)
			require.Error(t, err)

			// Test GetOperation
			_, err = entity.GetOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID)
			require.Error(t, err)

			// Test UpdateOperation
			_, err = entity.UpdateOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID, models.UpdateOperationInput{})
			require.Error(t, err)
		})
	}
}

// TestOperationsEntity_ConcurrentRequests tests concurrent API requests
func TestOperationsEntity_ConcurrentRequests(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(createTestOperation())
	}))
	defer server.Close()

	entity := createTestOperationsEntity(server.URL)

	numRequests := 10
	done := make(chan bool, numRequests)
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			_, err := entity.GetOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID)
			if err != nil {
				errors <- err
			}

			done <- true
		}()
	}

	for i := 0; i < numRequests; i++ {
		<-done
	}

	close(errors)

	for err := range errors {
		t.Errorf("Concurrent request failed: %v", err)
	}
}

// TestOperationsEntity_MalformedJSONResponse tests handling of malformed JSON responses
func TestOperationsEntity_MalformedJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"invalid json`))
	}))
	defer server.Close()

	entity := createTestOperationsEntity(server.URL)

	_, err := entity.GetOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID)
	require.Error(t, err)
}

// TestOperationsEntity_EmptyResponse tests handling of empty responses
func TestOperationsEntity_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	entity := createTestOperationsEntity(server.URL)

	result, err := entity.GetOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID)
	require.NoError(t, err)
	assert.Empty(t, result.ID)
}

// TestOperationsEntity_OperationTypes tests different operation types
func TestOperationsEntity_OperationTypes(t *testing.T) {
	operationTypes := []string{"DEBIT", "CREDIT"}

	for _, opType := range operationTypes {
		t.Run("Operation type "+opType, func(t *testing.T) {
			testOp := createTestOperation()
			testOp.Type = opType

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(testOp)
			}))
			defer server.Close()

			entity := createTestOperationsEntity(server.URL)

			result, err := entity.GetOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID)
			require.NoError(t, err)
			assert.Equal(t, opType, result.Type)
		})
	}
}

// TestOperationsEntity_MetadataHandling tests various metadata scenarios
func TestOperationsEntity_MetadataHandling(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
	}{
		{
			name:     "nil metadata",
			metadata: nil,
		},
		{
			name:     "empty metadata",
			metadata: map[string]any{},
		},
		{
			name:     "string metadata",
			metadata: map[string]any{"key": "value"},
		},
		{
			name:     "numeric metadata",
			metadata: map[string]any{"count": float64(42)},
		},
		{
			name:     "boolean metadata",
			metadata: map[string]any{"active": true},
		},
		{
			name:     "nested metadata",
			metadata: map[string]any{"nested": map[string]any{"inner": "value"}},
		},
		{
			name:     "array metadata",
			metadata: map[string]any{"tags": []interface{}{"tag1", "tag2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testOp := createTestOperation()
			testOp.Metadata = tt.metadata

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(testOp)
			}))
			defer server.Close()

			entity := createTestOperationsEntity(server.URL)

			result, err := entity.GetOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID)
			require.NoError(t, err)

			if tt.metadata == nil {
				assert.Nil(t, result.Metadata)
			} else {
				assert.Len(t, result.Metadata, len(tt.metadata))
			}
		})
	}
}

// TestOperationsEntity_AuthorizationHeader tests that authorization header is set
func TestOperationsEntity_AuthorizationHeader(t *testing.T) {
	var capturedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(createTestOperation())
	}))
	defer server.Close()

	entity := createTestOperationsEntity(server.URL)
	entity.HTTPClient.authToken = "test-bearer-token"

	_, err := entity.GetOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID)
	require.NoError(t, err)

	// The HTTPClient stores the token without the "Bearer " prefix - verify it's being sent
	authHeader := capturedHeaders.Get("Authorization")
	assert.NotEmpty(t, authHeader, "Authorization header should be set")
	assert.Contains(t, authHeader, "test-bearer-token", "Authorization header should contain the token")
}

// TestOperationsEntity_ContentTypeHeader tests that content-type header is set for mutations
func TestOperationsEntity_ContentTypeHeader(t *testing.T) {
	var capturedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(createTestOperation())
	}))
	defer server.Close()

	entity := createTestOperationsEntity(server.URL)

	input := models.UpdateOperationInput{Description: "test"}
	_, err := entity.UpdateOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID, input)
	require.NoError(t, err)

	assert.Equal(t, "application/json", capturedHeaders.Get("Content-Type"))
}

// TestOperationsEntity_UpdateWithMapInput tests UpdateOperation with map input type
func TestOperationsEntity_UpdateWithMapInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(createTestOperation())
	}))
	defer server.Close()

	entity := createTestOperationsEntity(server.URL)

	// Test with map input instead of struct
	input := map[string]interface{}{
		"description": "Updated via map",
		"metadata":    map[string]interface{}{"key": "value"},
	}

	result, err := entity.UpdateOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID, input)
	require.NoError(t, err)
	require.NotNil(t, result)
}

// TestOperationsEntity_ListWithAllFilters tests ListOperations with all filter options
func TestOperationsEntity_ListWithAllFilters(t *testing.T) {
	var capturedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(models.ListResponse[models.Operation]{
			Items:      []models.Operation{},
			Pagination: models.Pagination{Total: 0},
		})
	}))
	defer server.Close()

	entity := createTestOperationsEntity(server.URL)

	opts := &models.ListOptions{
		Limit:          20,
		Offset:         40,
		OrderBy:        "updatedAt",
		OrderDirection: "asc",
		Filters: map[string]string{
			"type":      "CREDIT",
			"assetCode": "EUR",
		},
	}

	_, err := entity.ListOperations(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opts)
	require.NoError(t, err)

	// Verify query parameters
	assert.Contains(t, capturedQuery, "limit=20")
	assert.Contains(t, capturedQuery, "offset=40")
}

// TestMockHTTPClientForOperations tests using the MockHTTPClient pattern
func TestMockHTTPClientForOperations(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(_ *http.Request) (*http.Response, error) {
			testOp := createTestOperation()

			body, err := json.Marshal(testOp)
			if err != nil {
				return nil, err
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(body))),
			}, nil
		},
	}

	retryOpts := retry.DefaultOptions()
	_ = retry.WithMaxRetries(1)(retryOpts)
	_ = retry.WithInitialDelay(1 * time.Millisecond)(retryOpts)

	httpClient := &HTTPClient{
		client: &http.Client{
			Transport: &mockTransport{mock: mockClient},
		},
		authToken:    "test-token",
		retryOptions: retryOpts,
		jsonPool:     performance.NewJSONPool(),
	}

	entity := &operationsEntity{
		HTTPClient: httpClient,
		baseURLs:   map[string]string{"transaction": "http://localhost:8080"},
	}

	result, err := entity.GetOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, opTestOperationID, result.ID)
}

// TestOperationsEntity_URLPathConstruction tests that URL paths are properly constructed
func TestOperationsEntity_URLPathConstruction(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		expectedPath string
	}{
		{
			name:         "ListOperations path",
			method:       "ListOperations",
			expectedPath: "/organizations/org-123/ledgers/ledger-456/accounts/acc-789/operations",
		},
		{
			name:         "GetOperation path",
			method:       "GetOperation",
			expectedPath: "/organizations/org-123/ledgers/ledger-456/accounts/acc-789/operations/op-abc",
		},
		{
			name:         "UpdateOperation path",
			method:       "UpdateOperation",
			expectedPath: "/organizations/org-123/ledgers/ledger-456/accounts/acc-789/operations/op-abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				switch tt.method {
				case "ListOperations":
					json.NewEncoder(w).Encode(models.ListResponse[models.Operation]{
						Items:      []models.Operation{},
						Pagination: models.Pagination{Total: 0},
					})
				default:
					json.NewEncoder(w).Encode(createTestOperation())
				}
			}))
			defer server.Close()

			entity := createTestOperationsEntity(server.URL)

			switch tt.method {
			case "ListOperations":
				entity.ListOperations(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, nil)
			case "GetOperation":
				entity.GetOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID)
			case "UpdateOperation":
				entity.UpdateOperation(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, opTestOperationID, models.UpdateOperationInput{})
			}

			assert.Equal(t, tt.expectedPath, capturedPath)
		})
	}
}

// TestOperationsEntity_SpecialCharactersInIDs tests handling of special characters in IDs
func TestOperationsEntity_SpecialCharactersInIDs(t *testing.T) {
	tests := []struct {
		name        string
		orgID       string
		ledgerID    string
		accountID   string
		operationID string
	}{
		{
			name:        "UUIDs",
			orgID:       "550e8400-e29b-41d4-a716-446655440000",
			ledgerID:    "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			accountID:   "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
			operationID: "6ba7b812-9dad-11d1-80b4-00c04fd430c8",
		},
		{
			name:        "alphanumeric",
			orgID:       "org123abc",
			ledgerID:    "ledger456def",
			accountID:   "acc789ghi",
			operationID: "op012jkl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(createTestOperation())
			}))
			defer server.Close()

			entity := createTestOperationsEntity(server.URL)

			_, err := entity.GetOperation(context.Background(), tt.orgID, tt.ledgerID, tt.accountID, tt.operationID)
			require.NoError(t, err)
		})
	}
}

// TestOperationsEntity_LargeResponseHandling tests handling of large list responses
func TestOperationsEntity_LargeResponseHandling(t *testing.T) {
	// Create a large list of operations
	operations := make([]models.Operation, 100)

	for i := 0; i < 100; i++ {
		op := createTestOperation()
		op.ID = "op-" + string(rune('a'+i%26)) + string(rune('0'+i%10))
		operations[i] = op
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(models.ListResponse[models.Operation]{
			Items: operations,
			Pagination: models.Pagination{
				Total:  100,
				Limit:  100,
				Offset: 0,
			},
		})
	}))
	defer server.Close()

	entity := createTestOperationsEntity(server.URL)

	result, err := entity.ListOperations(context.Background(), opTestOrgID, opTestLedgerID, opTestAccountID, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Items, 100)
}

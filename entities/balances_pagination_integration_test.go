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

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBalancesEntity_ListBalancesByExternalCode(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		ledgerID       string
		code           string
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
			code:     "EXT-001",
			opts:     models.BalancesListOpts{},
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
			opts:     models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 10, Page: 1}},
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

			entity := &balancesEntity{serviceEntity: serviceEntity{httpClient: newBalancesHTTPClientAdapter(mockClient), baseURLs: map[string]string{"transaction": "https://api.example.com/v1"}}}

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

		entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		result, err := entity.ListBalances(context.Background(), "org-123", "ledger-456", models.BalancesListOpts{})
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

		entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
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

		entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
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

		entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		input := models.NewUpdateBalanceInput().WithAllowSending(false)
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

		entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
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

		entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		result, err := entity.ListBalancesByAccountAlias(context.Background(), "org-123", "ledger-456", "@person1", models.BalancesListOpts{})
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

		entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
			"transaction": server.URL,
		})

		result, err := entity.ListBalancesByExternalCode(context.Background(), "org-123", "ledger-456", "EXT-001", models.BalancesListOpts{})
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

			entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
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

	entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
		"transaction": server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := entity.ListBalances(ctx, "org-123", "ledger-456", models.BalancesListOpts{})
	require.Error(t, err)
}

// TestBalancesEntity_ContextTimeout tests context timeout handling
func TestBalancesEntity_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
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

	entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
		"transaction": server.URL,
	})

	opts := models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 25, Page: 3, SortDirection: models.SortDescending}}

	_, err := entity.ListBalances(context.Background(), "org-123", "ledger-456", opts)
	require.NoError(t, err)

	assert.Contains(t, capturedURL, "limit=25")
	assert.Contains(t, capturedURL, "page=3")
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

		entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
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

		entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
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

	entity := newBalancesEntity(server.Client(), "test-token", map[string]string{
		"transaction": server.URL,
	})

	t.Run("With filters", func(t *testing.T) {
		opts := models.BalancesListOpts{
			PageListOpts: models.PageListOpts{Limit: 10},
			Filters:      models.BalancesFilters{AssetCode: "USD"},
		}

		_, err := entity.ListBalances(context.Background(), "org-123", "ledger-456", opts)
		require.NoError(t, err)
		assert.Contains(t, capturedQuery, "asset_code=USD")
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
			name:          "Rejects metadata",
			input:         models.NewUpdateBalanceInput().WithMetadata(map[string]any{"key": "value"}),
			expectedError: true,
		},
		{
			name:          "Rejects empty metadata",
			input:         models.NewUpdateBalanceInput().WithMetadata(map[string]any{}),
			expectedError: true,
		},
		{
			name:          "Empty update is rejected",
			input:         models.NewUpdateBalanceInput(),
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

// twoPageBalancesMock returns a MockHTTPClient that serves two pages of
// balances. Page 1 reports Total=2/Limit=1 (HasMore true via Total/Limit
// math); page 2 reports Total=2/Page=2/Limit=1 (HasMore false). The shared
// pagesRequested slice records every requested ?page= value so callers can
// assert page advancement and early termination.
func twoPageBalancesMock() (*MockHTTPClient, *[]string) {
	pagesRequested := []string{}
	mock := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			page := req.URL.Query().Get("page")
			pagesRequested = append(pagesRequested, page)

			body := `{
				"items": [{"id":"bal-` + page + `","assetCode":"USD","available":"100","onHold":"0","version":1}],
				"pagination": {"total": 2, "limit": 1, "page": ` + page + `}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		},
	}

	return mock, &pagesRequested
}

// TestBalancesEntity_ListBalancesPages_DefaultsAndAdvances covers the three
// happy-path invariants of the iter.Seq2 helpers: opts.Page==0 is normalized
// to 1, subsequent pages are fetched until HasMore==false, and every page is
// yielded.
func TestBalancesEntity_ListBalancesPages_DefaultsAndAdvances(t *testing.T) {
	mock, pagesRequested := twoPageBalancesMock()

	entity := &balancesEntity{serviceEntity: serviceEntity{
		httpClient: newBalancesHTTPClientAdapter(mock),
		baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
	}}

	opts := models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 1}}

	pages := 0

	for page, err := range entity.ListBalancesPages(context.Background(), "org", "ledger", opts) {
		require.NoError(t, err)
		require.NotNil(t, page)
		pages++
	}

	assert.Equal(t, 2, pages, "expected two yielded pages")
	assert.Equal(t, []string{"1", "2"}, *pagesRequested,
		"first request must default Page to 1, second must advance to 2")
}

// TestBalancesEntity_ListBalancesAll_FlattenAcrossPages verifies that
// ListBalancesAll yields every Balance across the page boundary.
func TestBalancesEntity_ListBalancesAll_FlattenAcrossPages(t *testing.T) {
	mock, _ := twoPageBalancesMock()

	entity := &balancesEntity{serviceEntity: serviceEntity{
		httpClient: newBalancesHTTPClientAdapter(mock),
		baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
	}}

	opts := models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 1}}

	items := 0
	for bal, err := range entity.ListBalancesAll(context.Background(), "org", "ledger", opts) {
		require.NoError(t, err)
		require.NotEmpty(t, bal.ID)
		items++
	}

	assert.Equal(t, 2, items, "expected one item per page across two pages")
}

// TestBalancesEntity_ListBalancesPages_EarlyTermination verifies that
// breaking out of the range loop after the first page stops further HTTP
// requests — the iterator must respect a false yield return.
func TestBalancesEntity_ListBalancesPages_EarlyTermination(t *testing.T) {
	mock, pagesRequested := twoPageBalancesMock()

	entity := &balancesEntity{serviceEntity: serviceEntity{
		httpClient: newBalancesHTTPClientAdapter(mock),
		baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
	}}

	opts := models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 1}}

	for page, err := range entity.ListBalancesPages(context.Background(), "org", "ledger", opts) {
		require.NoError(t, err)
		require.NotNil(t, page)
		break // early termination after the first page
	}

	assert.Equal(t, []string{"1"}, *pagesRequested,
		"early break must stop the iterator before requesting page 2")
}

// TestBalancesEntity_ListBalancesPages_ContextCancellation verifies that a
// cancelled context yields ctx.Err() instead of issuing further requests
// AND that the iterator short-circuits before the first HTTP call. The
// pagesRequested spy from twoPageBalancesMock records every wire request,
// so an empty slice proves the transport was never touched.
func TestBalancesEntity_ListBalancesPages_ContextCancellation(t *testing.T) {
	mock, pagesRequested := twoPageBalancesMock()

	entity := &balancesEntity{serviceEntity: serviceEntity{
		httpClient: newBalancesHTTPClientAdapter(mock),
		baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
	}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the first iteration

	opts := models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 1}}

	var observed error

	for _, err := range entity.ListBalancesPages(ctx, "org", "ledger", opts) {
		observed = err
		break
	}

	require.ErrorIs(t, observed, context.Canceled)
	assert.Empty(t, *pagesRequested,
		"a cancelled context must short-circuit before the first HTTP request")
}

// TestBalancesEntity_ListBalancesByAccountAliasPages_AdvancesPages exercises
// the alias variant to confirm the pagination loop behaves identically to
// the ledger-scoped helper.
func TestBalancesEntity_ListBalancesByAccountAliasPages_AdvancesPages(t *testing.T) {
	mock, pagesRequested := twoPageBalancesMock()

	entity := &balancesEntity{serviceEntity: serviceEntity{
		httpClient: newBalancesHTTPClientAdapter(mock),
		baseURLs:   map[string]string{"transaction": "https://api.example.com/v1"},
	}}

	opts := models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 1}}

	pages := 0

	for page, err := range entity.ListBalancesByAccountAliasPages(context.Background(), "org", "ledger", "@alias", opts) {
		require.NoError(t, err)
		require.NotNil(t, page)
		pages++
	}

	assert.Equal(t, 2, pages, "alias helper must traverse both pages")
	assert.Equal(t, []string{"1", "2"}, *pagesRequested)
}

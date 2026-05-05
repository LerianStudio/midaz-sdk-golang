package entities

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionsEntity_CreateTransaction_HTTPRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/organizations/org%2F1/ledgers/ledger%2F1/transactions/json", r.URL.EscapedPath())
		assert.Equal(t, "caller-key-123", r.Header.Get("X-Idempotency"))
		assert.Empty(t, r.Header.Get("X-Midaz-Auto-Idempotency"))

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		send, ok := body["send"].(map[string]any)
		if !assert.True(t, ok) {
			http.Error(w, "missing send", http.StatusBadRequest)
			return
		}

		assert.Equal(t, "100.50", send["value"])

		source, ok := send["source"].(map[string]any)
		if !assert.True(t, ok) {
			http.Error(w, "missing source", http.StatusBadRequest)
			return
		}

		fromItems, ok := source["from"].([]any)
		if !assert.True(t, ok) || !assert.NotEmpty(t, fromItems) {
			http.Error(w, "missing from", http.StatusBadRequest)
			return
		}

		from, ok := fromItems[0].(map[string]any)
		if !assert.True(t, ok) {
			http.Error(w, "invalid from", http.StatusBadRequest)
			return
		}

		assert.Equal(t, "customer:1", from["accountAlias"])
		assert.Equal(t, "route-legacy", from["route"])
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", from["routeId"])

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"id":"tx-1","amount":"100.50","assetCode":"USD"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	routeID := "550e8400-e29b-41d4-a716-446655440000"
	service := NewTransactionsEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
	result, err := service.CreateTransaction(context.Background(), "org/1", "ledger/1", &models.CreateTransactionInput{
		Send: &models.SendInput{
			Asset: "USD",
			Value: "100.50",
			Source: &models.SourceInput{From: []models.FromToInput{{
				Account:      "customer:1",
				AccountAlias: "customer:1",
				Route:        "route-legacy",
				RouteID:      &routeID,
				Amount:       models.AmountInput{Asset: "USD", Value: "100.50"},
			}}},
			Distribute: &models.DistributeInput{To: []models.FromToInput{{
				Account: "merchant:1",
				Amount:  models.AmountInput{Asset: "USD", Value: "100.50"},
			}}},
		},
		IdempotencyKey: "caller-key-123",
	})

	require.NoError(t, err)
	assert.Equal(t, "100.50", result.Amount)
}

func TestTransactionsEntity_ListTransactions_UsesCursorPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.URL.Query().Get("page"))
		assert.Empty(t, r.URL.Query().Get("offset"))
		assert.Equal(t, "cursor-123", r.URL.Query().Get("cursor"))
		assert.Equal(t, "25", r.URL.Query().Get("limit"))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"items":[],"pagination":{"limit":25}}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewTransactionsEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
	_, err := service.ListTransactions(context.Background(), "org-1", "ledger-1", models.NewListOptions().WithPage(3).WithCursor("cursor-123").WithLimit(25))
	require.NoError(t, err)
}

func TestTransactionsEntity_CreateSpecializedTransactions_HTTPWireShape(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		call          func(TransactionsService) (*models.Transaction, error)
		assertPayload func(*testing.T, map[string]any)
	}{
		{
			name: "inflow uses accountAlias in distribute",
			path: "/organizations/org-1/ledgers/ledger-1/transactions/inflow",
			call: func(service TransactionsService) (*models.Transaction, error) {
				return service.CreateInflowTransaction(context.Background(), "org-1", "ledger-1", models.NewCreateInflowInput("USD", 100, &models.DistributeInput{To: []models.FromToInput{{
					Account: "dest-account", Amount: models.AmountInput{Asset: "USD", Value: 100},
				}}}))
			},
			assertPayload: func(t *testing.T, body map[string]any) {
				t.Helper()

				send, ok := body["send"].(map[string]any)
				require.True(t, ok)
				distribute, ok := send["distribute"].(map[string]any)
				require.True(t, ok)
				toItems, ok := distribute["to"].([]any)
				require.True(t, ok)
				require.NotEmpty(t, toItems)
				to, ok := toItems[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "dest-account", to["accountAlias"])
				assert.NotContains(t, to, "account")
			},
		},
		{
			name: "outflow uses accountAlias in source",
			path: "/organizations/org-1/ledgers/ledger-1/transactions/outflow",
			call: func(service TransactionsService) (*models.Transaction, error) {
				return service.CreateOutflowTransaction(context.Background(), "org-1", "ledger-1", models.NewCreateOutflowInput("USD", 100, &models.SourceInput{From: []models.FromToInput{{
					Account: "source-account", Amount: models.AmountInput{Asset: "USD", Value: 100},
				}}}))
			},
			assertPayload: func(t *testing.T, body map[string]any) {
				t.Helper()

				send, ok := body["send"].(map[string]any)
				require.True(t, ok)
				source, ok := send["source"].(map[string]any)
				require.True(t, ok)
				fromItems, ok := source["from"].([]any)
				require.True(t, ok)
				require.NotEmpty(t, fromItems)
				from, ok := fromItems[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "source-account", from["accountAlias"])
				assert.NotContains(t, from, "account")
			},
		},
		{
			name: "annotation uses canonical transaction mapping",
			path: "/organizations/org-1/ledgers/ledger-1/transactions/annotation",
			call: func(service TransactionsService) (*models.Transaction, error) {
				return service.CreateAnnotationTransaction(context.Background(), "org-1", "ledger-1", models.NewCreateAnnotationInput("note", &models.SendInput{
					Asset: "USD", Value: 1,
					Source:     &models.SourceInput{From: []models.FromToInput{{Account: "source", Amount: models.AmountInput{Asset: "USD", Value: 1}}}},
					Distribute: &models.DistributeInput{To: []models.FromToInput{{Account: "dest", Amount: models.AmountInput{Asset: "USD", Value: 1}}}},
				}))
			},
			assertPayload: func(t *testing.T, body map[string]any) {
				t.Helper()

				send, ok := body["send"].(map[string]any)
				require.True(t, ok)
				source, ok := send["source"].(map[string]any)
				require.True(t, ok)
				fromItems, ok := source["from"].([]any)
				require.True(t, ok)
				require.NotEmpty(t, fromItems)
				from, ok := fromItems[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "source", from["accountAlias"])
				assert.NotContains(t, from, "account")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.path, r.URL.EscapedPath())

				var body map[string]any
				if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
					http.Error(w, "invalid request body", http.StatusBadRequest)
					return
				}

				tt.assertPayload(t, body)
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"id":"tx-1","amount":"100","assetCode":"USD"}`))
				assert.NoError(t, err)
			}))
			defer server.Close()

			service := NewTransactionsEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
			result, err := tt.call(service)
			require.NoError(t, err)
			assert.Equal(t, "tx-1", result.ID)
		})
	}
}

func TestTransactionsEntity_CreateTransaction_ParsesOperationFinancialFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"id":"tx-1",
			"amount":"100",
			"assetCode":"USD",
			"operations":[{
				"id":"op-1",
				"type":"debit",
				"assetCode":"USD",
				"amount":{"value":"100"},
				"balance":{"available":"900","onHold":"0"},
				"balanceAfter":{"available":"800","onHold":"0"},
				"status":{"code":"APPROVED"}
			}]
		}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewTransactionsEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
	result, err := service.CreateTransaction(context.Background(), "org-1", "ledger-1", &models.CreateTransactionInput{
		Send: &models.SendInput{
			Asset: "USD", Value: 100,
			Source:     &models.SourceInput{From: []models.FromToInput{{Account: "source", Amount: models.AmountInput{Asset: "USD", Value: 100}}}},
			Distribute: &models.DistributeInput{To: []models.FromToInput{{Account: "dest", Amount: models.AmountInput{Asset: "USD", Value: 100}}}},
		},
	})

	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	require.NotNil(t, result.Operations[0].Amount.Value)
	require.NotNil(t, result.Operations[0].Balance.Available)
	require.NotNil(t, result.Operations[0].BalanceAfter.Available)
	assert.Equal(t, "100", result.Operations[0].Amount.Value.String())
	assert.Equal(t, "900", result.Operations[0].Balance.Available.String())
	assert.Equal(t, "800", result.Operations[0].BalanceAfter.Available.String())
	assert.Equal(t, "APPROVED", result.Operations[0].Status.Code)
}

func TestTransactionsEntity_CreateTransactionWithDSLFile_HTTPMultipart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/organizations/org-1/ledgers/ledger-1/transactions/dsl", r.URL.EscapedPath())

		mediaType := r.Header.Get("Content-Type")
		if !assert.True(t, strings.HasPrefix(mediaType, "multipart/form-data; boundary=")) {
			http.Error(w, "invalid content type", http.StatusBadRequest)
			return
		}

		reader, err := r.MultipartReader()
		if !assert.NoError(t, err) {
			http.Error(w, "invalid multipart", http.StatusBadRequest)
			return
		}

		part, err := reader.NextPart()
		if !assert.NoError(t, err) {
			http.Error(w, "missing multipart part", http.StatusBadRequest)
			return
		}

		defer func() {
			assert.NoError(t, part.Close())
		}()

		assert.Equal(t, "transaction", part.FormName())
		assert.Equal(t, "transaction.dsl", part.FileName())

		payload, err := io.ReadAll(part)
		if !assert.NoError(t, err) {
			http.Error(w, "invalid multipart payload", http.StatusBadRequest)
			return
		}

		assert.Equal(t, "(transaction V1)", string(payload))

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write([]byte(`{"id":"tx-dsl","amount":"1","assetCode":"USD"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewTransactionsEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
	result, err := service.CreateTransactionWithDSLFile(context.Background(), "org-1", "ledger-1", []byte("(transaction V1)"))

	require.NoError(t, err)
	assert.Equal(t, "tx-dsl", result.ID)
}

func TestTransactionsEntity_CancelTransaction_AllowsEmptySuccessBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/organizations/org-1/ledgers/ledger-1/transactions/tx-1/cancel", r.URL.EscapedPath())
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service := NewTransactionsEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
	err := service.CancelTransaction(context.Background(), "org-1", "ledger-1", "tx-1")
	require.NoError(t, err)

	result, err := service.CancelTransactionWithResponse(context.Background(), "org-1", "ledger-1", "tx-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "tx-1", result.ID)
}

func TestTransactionsEntity_CancelTransactionWithResponse_HTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/organizations/org-1/ledgers/ledger-1/transactions/tx-1/cancel", r.URL.EscapedPath())
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"id":"tx-1","amount":"100","assetCode":"USD","status":{"code":"CANCELED"}}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewTransactionsEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
	result, err := service.CancelTransactionWithResponse(context.Background(), "org-1", "ledger-1", "tx-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "tx-1", result.ID)
	assert.Equal(t, "CANCELED", result.Status.Code)
}

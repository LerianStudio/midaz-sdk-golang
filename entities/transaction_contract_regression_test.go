package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionContractDirectConstructors_CopyTransactionBaseURLs(t *testing.T) {
	baseURLs := map[string]string{"transaction": "https://transaction.example.com/"}

	services := []any{
		newTransactionsEntity(nil, baseURLs),
		newOperationsEntity(nil, "token", baseURLs),
		newOperationRoutesEntity(nil, "token", baseURLs),
		newTransactionRoutesEntity(nil, "token", baseURLs),
		newAssetRatesEntity(nil, "token", baseURLs),
	}

	baseURLs["transaction"] = "https://evil.example.com"

	for _, service := range services {
		baseURLReader := service.(interface{ entityHTTPClient() *HTTPClient })
		require.NotNil(t, baseURLReader.entityHTTPClient())
	}

	assert.Equal(t, "https://transaction.example.com", services[0].(*transactionsEntity).baseURLs["transaction"])
	assert.Equal(t, "https://transaction.example.com", services[1].(*operationsEntity).baseURLs["transaction"])
	assert.Equal(t, "https://transaction.example.com", services[2].(*operationRoutesEntity).baseURLs["transaction"])
	assert.Equal(t, "https://transaction.example.com", services[3].(*transactionRoutesEntity).baseURLs["transaction"])
	assert.Equal(t, "https://transaction.example.com", services[4].(*assetRatesEntity).baseURLs["transaction"])
}

func TestTransactionContractRouteServices_HTTPContracts(t *testing.T) {
	var nilContext context.Context

	tests := []struct {
		name       string
		call       func(baseURL string) error
		wantMethod string
		wantPath   string
	}{
		{
			name: "operation route create",
			call: func(baseURL string) error {
				svc := newOperationRoutesEntity(nil, "token", map[string]string{"transaction": baseURL})
				_, err := svc.CreateOperationRoute(nilContext, "org/1", "ledger/1", models.NewCreateOperationRouteInput("Source", "desc", "source"))

				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/organizations/org%2F1/ledgers/ledger%2F1/operation-routes",
		},
		{
			name: "transaction route list (cursor-only)",
			call: func(baseURL string) error {
				svc := newTransactionRoutesEntity(nil, "token", map[string]string{"transaction": baseURL})
				// v3 TransactionRoutesListOpts has NO Page field —
				// compile-time prevention of the v2 silent-drop footgun.
				_, err := svc.ListTransactionRoutes(nilContext, "org/1", "ledger/1", models.TransactionRoutesListOpts{
					CursorListOpts: models.CursorListOpts{Cursor: "next"},
				})

				return err
			},
			wantMethod: http.MethodGet,
			wantPath:   "/organizations/org%2F1/ledgers/ledger%2F1/transaction-routes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen *http.Request

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = r

				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"items":[]}`))
				assert.NoError(t, err)
			}))
			defer server.Close()

			require.NoError(t, tt.call(server.URL+"/"))
			require.NotNil(t, seen)
			assert.Equal(t, tt.wantMethod, seen.Method)
			assert.Equal(t, tt.wantPath, seen.URL.EscapedPath())
			assert.NotContains(t, seen.URL.RawQuery, "page=")
			if tt.wantMethod == http.MethodGet {
				assert.Empty(t, seen.Header.Get("X-Idempotency"))
			}
		})
	}
}

func TestTransactionContractTransactionsCount_WhitelistsContractFilters(t *testing.T) {
	var seen *http.Request

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r

		w.Header().Set(HeaderTotalCount, "42")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	svc := newTransactionsEntity(server.Client(), map[string]string{"transaction": server.URL})
	// v3: cursor/limit/sort don't apply to HEAD /metrics/count.
	// transactionMetricsCountQueryParams emits ONLY status, route, dates.
	count, err := svc.GetTransactionsMetricsCount(context.Background(), "org", "ledger", models.TransactionsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 50, Cursor: "abc"},
		Filters:        models.TransactionsFilters{Route: "cashin", Status: "APPROVED"},
	})
	require.NoError(t, err)
	assert.Equal(t, 42, count.TransactionsCount)
	assert.Equal(t, http.MethodHead, seen.Method)
	assert.Equal(t, "cashin", seen.URL.Query().Get("route"))
	assert.Equal(t, "APPROVED", seen.URL.Query().Get("status"))
	assert.Empty(t, seen.URL.Query().Get("page"))
	assert.Empty(t, seen.URL.Query().Get("limit"))
	assert.Empty(t, seen.URL.Query().Get("cursor"))
	assert.Empty(t, seen.URL.Query().Get("sort_order"))
}

func TestTransactionContractIdempotencyKey_AllowsRetry(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "caller-key", r.Header.Get("X-Idempotency"))

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"error":"temporary"}`))
			assert.NoError(t, err)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"id":"tx-1"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	svc := newTransactionsEntity(server.Client(), map[string]string{"transaction": server.URL}).(*transactionsEntity)
	require.NoError(t, svc.httpClient.WithRetryOptions(retry.WithMaxRetries(1), retry.WithInitialDelay(time.Millisecond), retry.WithMaxDelay(time.Millisecond)))

	input := models.NewCreateTransactionInput("USD", "10").WithSend(&models.SendInput{Asset: "USD", Value: "10", Source: &models.SourceInput{From: []models.FromToInput{{AccountAlias: "@a", Amount: models.AmountInput{Asset: "USD", Value: "10"}}}}, Distribute: &models.DistributeInput{To: []models.FromToInput{{AccountAlias: "@b", Amount: models.AmountInput{Asset: "USD", Value: "10"}}}}})
	input.IdempotencyKey = "caller-key"

	tx, err := svc.CreateTransaction(context.Background(), "org", "ledger", input)
	require.NoError(t, err)
	assert.Equal(t, "tx-1", tx.ID)
	assert.Equal(t, int32(2), calls.Load())
}

func TestTransactionActionMethods_AutoIdempotencyDoesNotMakeActionsRetryable(t *testing.T) {
	tests := []struct {
		name string
		call func(*transactionsEntity) error
	}{
		{
			name: "commit",
			call: func(svc *transactionsEntity) error {
				_, err := svc.CommitTransaction(context.Background(), "org", "ledger", "tx-1")
				return err
			},
		},
		{
			name: "revert",
			call: func(svc *transactionsEntity) error {
				_, err := svc.RevertTransaction(context.Background(), "org", "ledger", "tx-1")
				return err
			},
		},
		{
			name: "cancel",
			call: func(svc *transactionsEntity) error {
				_, err := svc.CancelTransactionWithResponse(context.Background(), "org", "ledger", "tx-1")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				assert.Empty(t, r.Header.Get("X-Idempotency"))
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"error":"temporary"}`))
				assert.NoError(t, err)
			}))
			defer server.Close()

			svc := newTransactionsEntity(server.Client(), map[string]string{"transaction": server.URL}).(*transactionsEntity)
			require.NoError(t, svc.httpClient.WithRetryOptions(retry.WithMaxRetries(1), retry.WithInitialDelay(time.Millisecond), retry.WithMaxDelay(time.Millisecond)))

			err := tt.call(svc)
			require.Error(t, err)
			assert.Equal(t, int32(1), calls.Load())
		})
	}
}

func TestTransactionActionMethods_ExplicitIdempotencyKeyAllowsRetry(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context, *transactionsEntity) (*models.Transaction, error)
	}{
		{
			name: "commit",
			call: func(ctx context.Context, svc *transactionsEntity) (*models.Transaction, error) {
				return svc.CommitTransaction(ctx, "org", "ledger", "tx-1")
			},
		},
		{
			name: "revert",
			call: func(ctx context.Context, svc *transactionsEntity) (*models.Transaction, error) {
				return svc.RevertTransaction(ctx, "org", "ledger", "tx-1")
			},
		},
		{
			name: "cancel",
			call: func(ctx context.Context, svc *transactionsEntity) (*models.Transaction, error) {
				return svc.CancelTransactionWithResponse(ctx, "org", "ledger", "tx-1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "explicit-action-key", r.Header.Get("X-Idempotency"))
				if calls.Add(1) == 1 {
					w.WriteHeader(http.StatusInternalServerError)
					_, err := w.Write([]byte(`{"error":"temporary"}`))
					assert.NoError(t, err)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"id":"tx-1","status":{"code":"APPROVED"}}`))
				assert.NoError(t, err)
			}))
			defer server.Close()

			svc := newTransactionsEntity(server.Client(), map[string]string{"transaction": server.URL}).(*transactionsEntity)
			require.NoError(t, svc.httpClient.WithRetryOptions(retry.WithMaxRetries(1), retry.WithInitialDelay(time.Millisecond), retry.WithMaxDelay(time.Millisecond)))

			tx, err := tt.call(sdkctx.WithIdempotencyKey(context.Background(), "explicit-action-key"), svc)
			require.NoError(t, err)
			require.NotNil(t, tx)
			assert.Equal(t, int32(2), calls.Load())
		})
	}
}

func TestTransactionContractDSLFileValidation_RejectsBeforeNetwork(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	svc := newTransactionsEntity(server.Client(), map[string]string{"transaction": server.URL})

	_, err := svc.CreateTransactionWithDSLFile(context.Background(), "org", "ledger", []byte{0xff, 0xfe})
	require.Error(t, err)
	assert.Equal(t, int32(0), calls.Load())

	_, err = svc.CreateTransactionWithDSLFile(context.Background(), "org", "ledger", make([]byte, maxHTTPRequestBodyBytes+1))
	require.Error(t, err)
	assert.Equal(t, int32(0), calls.Load())
}

func TestTransactionContractAssetRateExternalIDRequiresUUID(t *testing.T) {
	svc := newAssetRatesEntity(nil, "token", map[string]string{"transaction": "https://api.example.com"})

	_, err := svc.CreateOrUpdateAssetRate(context.Background(), "org", "ledger", models.NewCreateAssetRateInput("USD", "BRL", 525).WithExternalID("not-a-uuid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "externalID")

	validID := uuid.NewString()
	require.NoError(t, models.NewCreateAssetRateInput("USD", "BRL", 525).WithExternalID(validID).Validate())
}

func TestTransactionContractUpdatePayloadRejectsTypedNil(t *testing.T) {
	var update *models.UpdateOperationInput

	svc := newOperationsEntity(nil, "token", map[string]string{"transaction": "https://api.example.com"})

	_, err := svc.UpdateTransactionOperation(context.Background(), "org", "ledger", "tx", "op", update)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "input")
}

func TestTransactionContractParseErrorResponse_RedactsTextualEnvelopeFields(t *testing.T) {
	body := []byte(`{"message":"document=12345678900","title":"metadata.secret=value","fields":["external_id=abc"]}`)

	err := (*HTTPClient)(nil).parseErrorResponse(http.StatusBadRequest, body, "req-1")
	require.Error(t, err)

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.NotContains(t, sdkErr.Message, "12345678900")
	assert.NotContains(t, sdkErr.Title, "value")
	require.Len(t, sdkErr.Fields, 1)
	assert.NotContains(t, sdkErr.Fields[0], "abc")
}

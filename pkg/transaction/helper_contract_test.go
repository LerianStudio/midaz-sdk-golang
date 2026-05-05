package transaction

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionHelpers_CreateCanonicalSendPayloads(t *testing.T) {
	tests := []struct {
		name          string
		call          func(context.Context, *entities.Entity) (*models.Transaction, error)
		wantSource    string
		wantDest      string
		wantAsset     string
		wantAmount    string
		wantDesc      string
		wantPending   bool
		wantIdem      string
		wantChartName string
	}{
		{
			name: "transfer with custom options",
			call: func(ctx context.Context, entity *entities.Entity) (*models.Transaction, error) {
				return Transfer(ctx, entity, "org-1", "ledger-1", "cash", "settlement", 12345, 2, "USD", &TransferOptions{
					Description:              "merchant settlement",
					Metadata:                 map[string]any{"case": "transfer"},
					IdempotencyKey:           "idem-transfer",
					Pending:                  true,
					ChartOfAccountsGroupName: "settlement-chart",
				})
			},
			wantSource:    "cash",
			wantDest:      "settlement",
			wantAsset:     "USD",
			wantAmount:    "123.45",
			wantDesc:      "merchant settlement",
			wantPending:   true,
			wantIdem:      "idem-transfer",
			wantChartName: "settlement-chart",
		},
		{
			name: "deposit with explicit external account",
			call: func(ctx context.Context, entity *entities.Entity) (*models.Transaction, error) {
				return Deposit(ctx, entity, "org-1", "ledger-1", "customer", 5000, 2, "USD", &DepositOptions{
					Description:       "card funding",
					IdempotencyKey:    "idem-deposit",
					ExternalAccountID: "@external/card-usd",
				})
			},
			wantSource: "@external/card-usd",
			wantDest:   "customer",
			wantAsset:  "USD",
			wantAmount: "50",
			wantDesc:   "card funding",
			wantIdem:   "idem-deposit",
		},
		{
			name: "withdrawal defaults external account by asset",
			call: func(ctx context.Context, entity *entities.Entity) (*models.Transaction, error) {
				return Withdrawal(ctx, entity, "org-1", "ledger-1", "customer", 750, 2, "BRL", &WithdrawalOptions{
					Description:    "pix out",
					IdempotencyKey: "idem-withdrawal",
				})
			},
			wantSource: "customer",
			wantDest:   "@external/BRL",
			wantAsset:  "BRL",
			wantAmount: "7.50",
			wantDesc:   "pix out",
			wantIdem:   "idem-withdrawal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/v1/organizations/org-1/ledgers/ledger-1/transactions/json", r.URL.EscapedPath())
				assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
				assert.Equal(t, tt.wantIdem, r.Header.Get("X-Idempotency"))

				body := decodeJSONBody(t, r)
				assert.Equal(t, tt.wantDesc, body["description"])

				if tt.wantPending {
					assert.Equal(t, true, body["pending"])
				} else {
					assert.NotContains(t, body, "pending")
				}

				if tt.wantChartName != "" {
					assert.Equal(t, tt.wantChartName, body["chartOfAccountsGroupName"])
				}

				send := requireMap(t, body["send"])
				assert.Equal(t, tt.wantAsset, send["asset"])
				assert.Equal(t, tt.wantAmount, send["value"])

				source := requireMap(t, send["source"])
				from := requireMapSlice(t, source["from"])
				assert.Equal(t, tt.wantSource, from[0]["accountAlias"])

				distribute := requireMap(t, send["distribute"])
				to := requireMapSlice(t, distribute["to"])
				assert.Equal(t, tt.wantDest, to[0]["accountAlias"])

				writeJSON(t, w, map[string]any{"id": "tx-1", "status": map[string]any{"code": "PENDING"}})
			}))
			defer server.Close()

			entity := newTransactionHelperEntity(t, server)
			tx, err := tt.call(context.Background(), entity)
			require.NoError(t, err)
			assert.Equal(t, "tx-1", tx.ID)
		})
	}
}

func TestTransactionHelpers_MultiTransferTemplateAndLifecycle(t *testing.T) {
	requests := make([]map[string]any, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/v1/organizations/org-1/ledgers/ledger-1/transactions/json":
			assert.Equal(t, http.MethodPost, r.Method)
			requests = append(requests, decodeJSONBody(t, r))
			writeJSON(t, w, map[string]any{"id": "tx-created", "status": map[string]any{"code": "PENDING"}})
		case "/v1/organizations/org-1/ledgers/ledger-1/transactions/tx-created/commit":
			assert.Equal(t, http.MethodPost, r.Method)
			writeJSON(t, w, map[string]any{"id": "tx-created", "status": map[string]any{"code": "COMPLETED"}})
		case "/v1/organizations/org-1/ledgers/ledger-1/transactions/tx-created/cancel":
			assert.Equal(t, http.MethodPost, r.Method)
			writeJSON(t, w, map[string]any{"id": "tx-created", "status": map[string]any{"code": "CANCELED"}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	entity := newTransactionHelperEntity(t, server)

	tx, err := MultiAccountTransfer(
		context.Background(),
		entity,
		"org-1",
		"ledger-1",
		map[string]int64{"source-a": 700, "source-b": 300},
		map[string]int64{"dest-a": 600, "dest-b": 400},
		1000,
		2,
		"USD",
		&MultiTransferOptions{Description: "split transfer", IdempotencyKey: "idem-multi"},
	)
	require.NoError(t, err)
	assert.Equal(t, "tx-created", tx.ID)

	template := &Template{
		Description: "templated funding",
		AssetCode:   "USD",
		Scale:       2,
		Metadata:    map[string]any{"template": "funding"},
		BuildSources: func(amount int64) []models.FromToInput {
			return []models.FromToInput{{Account: "template-source", Amount: models.AmountInput{Asset: "USD", Value: formatAmount(amount, 2)}}}
		},
		BuildDestinations: func(amount int64) []models.FromToInput {
			return []models.FromToInput{{Account: "template-dest", Amount: models.AmountInput{Asset: "USD", Value: formatAmount(amount, 2)}}}
		},
	}

	_, err = CreateFromTemplate(context.Background(), entity, "org-1", "ledger-1", template, 1234, map[string]any{"override": "yes"}, "idem-template")
	require.NoError(t, err)
	require.Len(t, requests, 2)

	multiSend := requireMap(t, requests[0]["send"])
	assert.Equal(t, "10", multiSend["value"])
	assert.Len(t, requireMapSlice(t, requireMap(t, multiSend["source"])["from"]), 2)
	assert.Len(t, requireMapSlice(t, requireMap(t, multiSend["distribute"])["to"]), 2)

	templateSend := requireMap(t, requests[1]["send"])
	assert.Equal(t, "12.34", templateSend["value"])
	templateMetadata := requireMap(t, requests[1]["metadata"])
	assert.Equal(t, "funding", templateMetadata["template"])
	assert.Equal(t, "yes", templateMetadata["override"])
	assert.Contains(t, templateMetadata, "timestamp")

	committed, err := CommitPendingTransaction(context.Background(), entity, "org-1", "ledger-1", "tx-created")
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", committed.Status.Code)

	canceled, err := CancelPendingTransaction(context.Background(), entity, "org-1", "ledger-1", "tx-created")
	require.NoError(t, err)
	assert.Equal(t, "CANCELED", canceled.Status.Code)
}

func TestTransactionHelpers_ErrorPaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/organizations/org-1/ledgers/ledger-1/transactions/json", r.URL.EscapedPath())
		http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	entity := newTransactionHelperEntity(t, server)

	_, err := Transfer(context.Background(), entity, "org-1", "ledger-1", "from", "to", 100, 2, "USD", &TransferOptions{IdempotencyKey: "idem"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transfer transaction failed")

	_, err = MultiAccountTransfer(context.Background(), entity, "org-1", "ledger-1", nil, map[string]int64{"dest": 100}, 100, 2, "USD", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source account")

	_, err = MultiAccountTransfer(context.Background(), entity, "org-1", "ledger-1", map[string]int64{"source": -1}, map[string]int64{"dest": 100}, 100, 2, "USD", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")

	_, err = MultiAccountTransfer(context.Background(), entity, "org-1", "ledger-1", map[string]int64{"source": 100}, map[string]int64{"dest": 99}, 100, 2, "USD", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unbalanced")

	_, err = MultiAccountTransfer(context.Background(), entity, "org-1", "ledger-1", map[string]int64{"source": 100}, map[string]int64{"dest": 100}, 101, 2, "USD", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "total amount mismatch")

	_, err = CreateFromTemplate(context.Background(), entity, "org-1", "ledger-1", nil, 100, nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template")

	_, err = CancelPendingTransaction(context.Background(), nil, "org-1", "ledger-1", "tx-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entity is required")

	_, err = CancelPendingTransaction(context.Background(), &entities.Entity{}, "org-1", "ledger-1", "tx-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transactions service")
}

func newTransactionHelperEntity(t *testing.T, server *httptest.Server) *entities.Entity {
	t.Helper()

	entity, err := entities.NewEntity(
		server.Client(),
		"token",
		map[string]string{"onboarding": server.URL, "transaction": server.URL},
		nil,
	)
	require.NoError(t, err)

	return entity
}

func decodeJSONBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()

	var body map[string]any
	require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

	return body
}

func requireMap(t *testing.T, value any) map[string]any {
	t.Helper()

	result, ok := value.(map[string]any)
	require.Truef(t, ok, "expected map[string]any, got %T", value)

	return result
}

func requireMapSlice(t *testing.T, value any) []map[string]any {
	t.Helper()

	values, ok := value.([]any)
	require.Truef(t, ok, "expected []any, got %T", value)

	result := make([]map[string]any, 0, len(values))
	for _, item := range values {
		result = append(result, requireMap(t, item))
	}

	return result
}

func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(body))
}

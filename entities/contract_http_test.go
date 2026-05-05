package entities

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLedgersEntity_HTTPContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers":
			assert.Equal(t, "7", r.URL.Query().Get("limit"))
			assert.Equal(t, "3", r.URL.Query().Get("page"))
			assert.Equal(t, "desc", r.URL.Query().Get("sort_order"))
			assert.Empty(t, r.URL.Query().Get("offset"))
			writeEntityJSON(t, w, map[string]any{"items": []map[string]any{{"id": "ledger/1", "name": "main"}}, "limit": 7, "page": 3})
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1":
			writeEntityJSON(t, w, map[string]any{"id": "ledger/1", "name": "main"})
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers":
			body := decodeEntityJSON(t, r)
			assert.Equal(t, "new ledger", body["name"])
			assert.Contains(t, body, "metadata")
			writeEntityJSON(t, w, map[string]any{"id": "ledger-new", "name": "new ledger"})
		case r.Method == http.MethodPatch && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1":
			body := decodeEntityJSON(t, r)
			assert.Equal(t, "renamed", body["name"])
			assert.NotContains(t, body, "metadata")
			writeEntityJSON(t, w, map[string]any{"id": "ledger/1", "name": "renamed"})
		case r.Method == http.MethodDelete && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodHead && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/metrics/count":
			w.Header().Set(HeaderTotalCount, "42")
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	service := newLedgersEntity(server.Client(), "token", map[string]string{"onboarding": server.URL})
	ctx := context.Background()

	list, err := service.ListLedgers(ctx, "org/1", models.LedgersListOpts{
		PageListOpts: models.PageListOpts{Limit: 7, Page: 3, SortDirection: models.SortDescending},
	})
	require.NoError(t, err)
	assert.Equal(t, "ledger/1", list.Items[0].ID)

	ledger, err := service.GetLedger(ctx, "org/1", "ledger/1")
	require.NoError(t, err)
	assert.Equal(t, "ledger/1", ledger.ID)

	ledger, err = service.CreateLedger(ctx, "org/1", models.NewCreateLedgerInput("new ledger").WithMetadata(map[string]any{"tier": "gold"}))
	require.NoError(t, err)
	assert.Equal(t, "ledger-new", ledger.ID)

	ledger, err = service.UpdateLedger(ctx, "org/1", "ledger/1", models.NewUpdateLedgerInput().WithName("renamed"))
	require.NoError(t, err)
	assert.Equal(t, "renamed", ledger.Name)

	require.NoError(t, service.DeleteLedger(ctx, "org/1", "ledger/1"))

	metrics, err := service.GetLedgersMetricsCount(ctx, "org/1")
	require.NoError(t, err)
	assert.Equal(t, 42, metrics.LedgersCount)
}

func TestOrganizationsAndPortfoliosEntity_HTTPContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1":
			writeEntityJSON(t, w, map[string]any{"id": "org/1", "legalName": "Acme"})
		case r.Method == http.MethodPatch && r.URL.EscapedPath() == "/organizations/org%2F1":
			body := decodeEntityJSON(t, r)
			assert.Equal(t, "Acme Updated", body["legalName"])
			writeEntityJSON(t, w, map[string]any{"id": "org/1", "legalName": "Acme Updated"})
		case r.Method == http.MethodDelete && r.URL.EscapedPath() == "/organizations/org%2F1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodHead && r.URL.EscapedPath() == "/organizations/metrics/count":
			w.Header().Set(HeaderTotalCount, "3")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/portfolios":
			assert.Equal(t, "11", r.URL.Query().Get("limit"))
			writeEntityJSON(t, w, map[string]any{"items": []map[string]any{{"id": "portfolio/1", "name": "Retail"}}})
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/portfolios/portfolio%2F1":
			writeEntityJSON(t, w, map[string]any{"id": "portfolio/1", "name": "Retail"})
		case r.Method == http.MethodPatch && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/portfolios/portfolio%2F1":
			body := decodeEntityJSON(t, r)
			assert.Equal(t, "Retail Updated", body["name"])
			writeEntityJSON(t, w, map[string]any{"id": "portfolio/1", "name": "Retail Updated"})
		case r.Method == http.MethodDelete && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/portfolios/portfolio%2F1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodHead && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/portfolios/metrics/count":
			w.Header().Set(HeaderTotalCount, "9")
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	orgs := newOrganizationsEntity(server.Client(), "token", map[string]string{"onboarding": server.URL})
	ctx := context.Background()

	org, err := orgs.GetOrganization(ctx, "org/1")
	require.NoError(t, err)
	assert.Equal(t, "org/1", org.ID)

	org, err = orgs.UpdateOrganization(ctx, "org/1", (&models.UpdateOrganizationInput{}).WithLegalName("Acme Updated"))
	require.NoError(t, err)
	assert.Equal(t, "Acme Updated", org.LegalName)

	require.NoError(t, orgs.DeleteOrganization(ctx, "org/1"))

	orgMetrics, err := orgs.GetOrganizationsMetricsCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, orgMetrics.OrganizationsCount)

	portfolios := newPortfoliosEntity(server.Client(), "token", map[string]string{"onboarding": server.URL})
	portfolioList, err := portfolios.ListPortfolios(ctx, "org/1", "ledger/1", models.PortfoliosListOpts{
		PageListOpts: models.PageListOpts{Limit: 11},
	})
	require.NoError(t, err)
	assert.Equal(t, "portfolio/1", portfolioList.Items[0].ID)

	portfolio, err := portfolios.GetPortfolio(ctx, "org/1", "ledger/1", "portfolio/1")
	require.NoError(t, err)
	assert.Equal(t, "Retail", portfolio.Name)

	portfolio, err = portfolios.UpdatePortfolio(ctx, "org/1", "ledger/1", "portfolio/1", models.NewUpdatePortfolioInput().WithName("Retail Updated"))
	require.NoError(t, err)
	assert.Equal(t, "Retail Updated", portfolio.Name)

	require.NoError(t, portfolios.DeletePortfolio(ctx, "org/1", "ledger/1", "portfolio/1"))

	portfolioMetrics, err := portfolios.GetPortfoliosMetricsCount(ctx, "org/1", "ledger/1")
	require.NoError(t, err)
	assert.Equal(t, 9, portfolioMetrics.PortfoliosCount)
}

func TestRoutesEntity_HTTPContracts(t *testing.T) {
	transactionRouteID := "28efef2d-50d8-4dc8-b2d7-832874ed32f0"
	operationRouteID := "41eb8891-f5d7-4bf5-9543-8d8f8e3a3e8c"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/operation-routes":
			assert.Equal(t, "5", r.URL.Query().Get("limit"))
			assert.Equal(t, "ACTIVE", r.URL.Query().Get("status"))
			writeEntityJSON(t, w, map[string]any{"items": []map[string]any{{"id": operationRouteID, "title": "source route"}}})
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/operation-routes/"+operationRouteID:
			writeEntityJSON(t, w, map[string]any{"id": operationRouteID, "title": "source route"})
		case r.Method == http.MethodPatch && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/operation-routes/"+operationRouteID:
			body := decodeEntityJSON(t, r)
			assert.Equal(t, "updated operation", body["title"])
			writeEntityJSON(t, w, map[string]any{"id": operationRouteID, "title": "updated operation"})
		case r.Method == http.MethodDelete && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/operation-routes/"+operationRouteID:
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/transaction-routes/"+transactionRouteID:
			writeEntityJSON(t, w, map[string]any{"id": transactionRouteID, "title": "funding"})
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/transaction-routes":
			body := decodeEntityJSON(t, r)
			assert.Equal(t, "funding", body["title"])
			routes := body["operationRoutes"].([]any)
			assert.Equal(t, operationRouteID, routes[0])
			writeEntityJSON(t, w, map[string]any{"id": transactionRouteID, "title": "funding"})
		case r.Method == http.MethodPatch && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/transaction-routes/"+transactionRouteID:
			body := decodeEntityJSON(t, r)
			assert.Equal(t, "funding updated", body["title"])
			writeEntityJSON(t, w, map[string]any{"id": transactionRouteID, "title": "funding updated"})
		case r.Method == http.MethodDelete && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/transaction-routes/"+transactionRouteID:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	ctx := context.Background()
	opRoutes := newOperationRoutesEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
	txRoutes := newTransactionRoutesEntity(server.Client(), "token", map[string]string{"transaction": server.URL})

	opList, err := opRoutes.ListOperationRoutes(ctx, "org/1", "ledger/1", (&models.ListOptions{}).WithLimit(5).WithFilter("status", "ACTIVE"))
	require.NoError(t, err)
	assert.Equal(t, operationRouteID, opList.Items[0].ID.String())

	opRoute, err := opRoutes.GetOperationRoute(ctx, "org/1", "ledger/1", operationRouteID)
	require.NoError(t, err)
	assert.Equal(t, "source route", opRoute.Title)

	opRoute, err = opRoutes.UpdateOperationRoute(ctx, "org/1", "ledger/1", operationRouteID, (&models.UpdateOperationRouteInput{}).WithTitle("updated operation"))
	require.NoError(t, err)
	assert.Equal(t, "updated operation", opRoute.Title)

	require.NoError(t, opRoutes.DeleteOperationRoute(ctx, "org/1", "ledger/1", operationRouteID))

	txRoute, err := txRoutes.GetTransactionRoute(ctx, "org/1", "ledger/1", transactionRouteID)
	require.NoError(t, err)
	assert.Equal(t, "funding", txRoute.Title)

	txRoute, err = txRoutes.CreateTransactionRoute(ctx, "org/1", "ledger/1", models.NewCreateTransactionRouteInput("funding", "desc", []string{operationRouteID}))
	require.NoError(t, err)
	assert.Equal(t, transactionRouteID, txRoute.ID.String())

	txRoute, err = txRoutes.UpdateTransactionRoute(ctx, "org/1", "ledger/1", transactionRouteID, (&models.UpdateTransactionRouteInput{}).WithTitle("funding updated"))
	require.NoError(t, err)
	assert.Equal(t, "funding updated", txRoute.Title)

	require.NoError(t, txRoutes.DeleteTransactionRoute(ctx, "org/1", "ledger/1", transactionRouteID))
}

func TestAccountsEntity_SpecialEndpointContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/accounts/account%2F1/balances":
			assert.Equal(t, "2", r.URL.Query().Get("limit"))
			writeEntityJSON(t, w, map[string]any{"items": []map[string]any{{"id": "balance-1", "accountId": "account/1", "assetCode": "USD"}}})
		case r.Method == http.MethodHead && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/accounts/metrics/count":
			w.Header().Set(HeaderTotalCount, "17")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/accounts/external/USD":
			alias := "@external/USD"
			writeEntityJSON(t, w, map[string]any{"id": "external-usd", "alias": alias, "assetCode": "USD"})
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/accounts/external/USD/balances":
			assert.Equal(t, "2", r.URL.Query().Get("limit"))
			writeEntityJSON(t, w, map[string]any{"items": []map[string]any{{"id": "external-balance", "assetCode": "USD"}}})
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/accounts/alias/customer:alice":
			alias := "customer:alice"
			writeEntityJSON(t, w, map[string]any{"id": "account-alias", "alias": alias})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	service := newAccountsEntity(server.Client(), "token", map[string]string{"onboarding": server.URL, "transaction": server.URL})
	ctx := context.Background()

	balance, err := service.GetBalance(ctx, "org/1", "ledger/1", "account/1")
	require.NoError(t, err)
	assert.Equal(t, "balance-1", balance.ID)

	metrics, err := service.GetAccountsMetricsCount(ctx, "org/1", "ledger/1")
	require.NoError(t, err)
	assert.Equal(t, 17, metrics.AccountsCount)

	account, err := service.GetExternalAccount(ctx, "org/1", "ledger/1", "USD")
	require.NoError(t, err)
	assert.Equal(t, "external-usd", account.ID)

	balance, err = service.GetExternalAccountBalance(ctx, "org/1", "ledger/1", "USD")
	require.NoError(t, err)
	assert.Equal(t, "external-balance", balance.ID)

	account, err = service.GetAccountByAliasPath(ctx, "org/1", "ledger/1", "customer:alice")
	require.NoError(t, err)
	assert.Equal(t, "account-alias", account.ID)
}

func TestTransactionsEntity_JSONDSLAndLifecycleContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/transactions/json":
			assert.Equal(t, "caller-idem", r.Header.Get("X-Idempotency"))
			body := decodeEntityJSON(t, r)
			assert.Equal(t, "wire transfer", body["description"])
			assert.Contains(t, body, "send")
			writeEntityJSON(t, w, entityTransactionResponse("tx-json", "PENDING"))
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/transactions/dsl":
			mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil {
				t.Errorf("parse content type: %v", err)
				http.Error(w, "invalid content type", http.StatusBadRequest)

				return
			}

			assert.Equal(t, "multipart/form-data", mediaType)
			body := readEntityBody(t, r)
			assert.Contains(t, string(body), "send")
			writeEntityJSON(t, w, entityTransactionResponse("tx-dsl", "PENDING"))
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/transactions/tx%2F1":
			writeEntityJSON(t, w, entityTransactionResponse("tx/1", "PENDING"))
		case r.Method == http.MethodPatch && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/transactions/tx%2F1":
			body := decodeEntityJSON(t, r)
			assert.Equal(t, "patched", body["description"])
			writeEntityJSON(t, w, entityTransactionResponse("tx/1", "PENDING"))
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/transactions/tx%2F1/revert":
			writeEntityJSON(t, w, entityTransactionResponse("tx/1", "REVERTED"))
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/transactions/tx%2F1/commit":
			writeEntityJSON(t, w, entityTransactionResponse("tx/1", "COMPLETED"))
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/organizations/org%2F1/ledgers/ledger%2F1/transactions/tx%2F1/cancel":
			writeEntityJSON(t, w, entityTransactionResponse("tx/1", "CANCELED"))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	service := newTransactionsEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
	ctx := context.Background()

	jsonInput := models.NewCreateTransactionInput("USD", "10.00").WithDescription("wire transfer").WithSend(&models.SendInput{
		Asset: "USD",
		Value: "10.00",
		Source: &models.SourceInput{From: []models.FromToInput{{
			Account: "cash", Amount: models.AmountInput{Asset: "USD", Value: "10.00"},
		}}},
		Distribute: &models.DistributeInput{To: []models.FromToInput{{
			Account: "settlement", Amount: models.AmountInput{Asset: "USD", Value: "10.00"},
		}}},
	})
	jsonInput.IdempotencyKey = "caller-idem"
	tx, err := service.CreateTransaction(ctx, "org/1", "ledger/1", jsonInput)
	require.NoError(t, err)
	assert.Equal(t, "tx-json", tx.ID)

	dslInput := &models.TransactionDSLInput{
		ChartOfAccountsGroupName: "TRANSFERS",
		Description:              "dsl transfer",
		Send: &models.DSLSend{
			Asset:      "USD",
			Value:      "10",
			Source:     &models.DSLSource{From: []models.DSLFromTo{{Account: "cash", Amount: &models.DSLAmount{Asset: "USD", Value: "10"}}}},
			Distribute: &models.DSLDistribute{To: []models.DSLFromTo{{Account: "settlement", Amount: &models.DSLAmount{Asset: "USD", Value: "10"}}}},
		},
	}
	tx, err = service.CreateTransactionWithDSL(ctx, "org/1", "ledger/1", dslInput)
	require.NoError(t, err)
	assert.Equal(t, "tx-dsl", tx.ID)

	tx, err = service.GetTransaction(ctx, "org/1", "ledger/1", "tx/1")
	require.NoError(t, err)
	assert.Equal(t, "tx/1", tx.ID)

	tx, err = service.UpdateTransaction(ctx, "org/1", "ledger/1", "tx/1", models.NewUpdateTransactionInput().WithDescription("patched"))
	require.NoError(t, err)
	assert.Equal(t, "tx/1", tx.ID)

	tx, err = service.RevertTransaction(ctx, "org/1", "ledger/1", "tx/1")
	require.NoError(t, err)
	assert.Equal(t, "REVERTED", tx.Status.Code)

	tx, err = service.CommitTransaction(ctx, "org/1", "ledger/1", "tx/1")
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", tx.Status.Code)

	tx, err = service.CancelTransactionWithResponse(ctx, "org/1", "ledger/1", "tx/1")
	require.NoError(t, err)
	assert.Equal(t, "CANCELED", tx.Status.Code)
}

func TestEntityValidationDoesNotHitServer(t *testing.T) {
	var hits atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits.Add(1)
	}))
	defer server.Close()

	ctx := context.Background()
	ledgers := newLedgersEntity(server.Client(), "token", map[string]string{"onboarding": server.URL})
	_, err := ledgers.GetLedger(ctx, "", "ledger-1")
	require.Error(t, err)
	err = ledgers.DeleteLedger(ctx, "org-1", "")
	require.Error(t, err)

	accounts := newAccountsEntity(server.Client(), "token", map[string]string{"onboarding": server.URL, "transaction": server.URL})
	_, err = accounts.GetBalance(ctx, "org-1", "", "account-1")
	require.Error(t, err)
	_, err = accounts.GetExternalAccount(ctx, "org-1", "ledger-1", "")
	require.Error(t, err)

	transactions := newTransactionsEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
	_, err = transactions.GetTransaction(ctx, "org-1", "ledger-1", "")
	require.Error(t, err)
	_, err = transactions.CreateTransactionWithDSL(ctx, "", "ledger-1", &models.TransactionDSLInput{})
	require.Error(t, err)

	assert.Zero(t, hits.Load())
}

func decodeEntityJSON(t *testing.T, r *http.Request) map[string]any {
	t.Helper()

	var body map[string]any
	require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

	return body
}

func readEntityBody(t *testing.T, r *http.Request) []byte {
	t.Helper()

	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)

	return body
}

func writeEntityJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(body))
}

func entityTransactionResponse(id, status string) map[string]any {
	return map[string]any{
		"id":             id,
		"amount":         "10.00",
		"assetCode":      "USD",
		"organizationId": "org/1",
		"ledgerId":       "ledger/1",
		"status":         map[string]any{"code": status, "description": status},
		"metadata":       map[string]any{"source": "contract-test"},
		"operations": []map[string]any{{
			"id":            "op-1",
			"transactionId": id,
			"type":          "source",
			"assetCode":     "USD",
			"amount":        map[string]any{"value": "10.00"},
			"balance":       map[string]any{"available": "90.00", "onHold": "0"},
			"balanceAfter":  map[string]any{"available": "80.00", "onHold": "0"},
			"status":        map[string]any{"code": status},
			"accountAlias":  "cash",
			"metadata":      map[string]any{"op": "1"},
		}},
	}
}

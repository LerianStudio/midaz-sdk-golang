package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/stretchr/testify/require"
)

// TestV2SurfaceWirePaths pins ROUTING for every method on the /v2 ledger
// surface: the HTTP method and the URL path. That is the whole of what it
// guards.
//
// It guards nothing else, on purpose. The fake answers 200 with a path-shaped
// body to every request and the only assertion on the call itself is that it
// returned no error, so request bodies, status handling and decode behaviour are
// all outside this table — they live in the per-facade tests in entities/ and in
// the V2 transaction routing table below.
//
// Routing is worth its own table because the client is built from the PUBLIC
// options (WithBaseURL against a live test server), so one row covers base-URL
// normalization and path construction together. A base-URL-map injection test
// cannot: injecting a base that already carries a version segment makes every
// path assertion pass while the shipped client — whose Ledger base is bare,
// because the generated client versions its own paths — emits something else
// entirely. Epic 1 shipped exactly that bug, invisible to the injected-map tests.
//
// One row per endpoint: the length is the endpoint count, and collapsing rows
// would hide the contract this exists to pin.
func TestV2SurfaceWirePaths(t *testing.T) {
	const (
		orgScope    = "/v2/organizations/org-1"
		ledgerScope = "/v2/organizations/org-1/ledgers/led-1"
		historyDate = "2026-01-02 03:04:05"
	)

	tests := []struct {
		name       string
		wantMethod string
		wantPath   string
		call       func(context.Context, *Client) error
	}{
		// --- organizations -------------------------------------------------
		{"organizations list", http.MethodGet, "/v2/organizations", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Organizations.List(ctx, models.OrganizationsListOpts{})
			return err
		}},
		{"organizations create", http.MethodPost, "/v2/organizations", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Organizations.Create(ctx, &models.CreateOrganizationInput{LegalName: "Acme", LegalDocument: "123"})
			return err
		}},
		{"organizations get", http.MethodGet, orgScope, func(ctx context.Context, c *Client) error {
			_, err := c.V2.Organizations.Get(ctx, "org-1")
			return err
		}},
		{"organizations update", http.MethodPatch, orgScope, func(ctx context.Context, c *Client) error {
			_, err := c.V2.Organizations.Update(ctx, "org-1", &models.UpdateOrganizationInput{LegalName: "Acme 2"})
			return err
		}},
		{"organizations delete", http.MethodDelete, orgScope, func(ctx context.Context, c *Client) error {
			return c.V2.Organizations.Delete(ctx, "org-1")
		}},
		{"organizations count", http.MethodHead, "/v2/organizations/metrics/count", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Organizations.Count(ctx)
			return err
		}},

		// --- ledgers -------------------------------------------------------
		{"ledgers list", http.MethodGet, orgScope + "/ledgers", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Ledgers.List(ctx, "org-1", models.LedgersListOpts{})
			return err
		}},
		{"ledgers create", http.MethodPost, orgScope + "/ledgers", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Ledgers.Create(ctx, "org-1", &models.CreateLedgerInput{Name: "Main"})
			return err
		}},
		{"ledgers get", http.MethodGet, ledgerScope, func(ctx context.Context, c *Client) error {
			_, err := c.V2.Ledgers.Get(ctx, "org-1", "led-1")
			return err
		}},
		{"ledgers update", http.MethodPatch, ledgerScope, func(ctx context.Context, c *Client) error {
			_, err := c.V2.Ledgers.Update(ctx, "org-1", "led-1", &models.UpdateLedgerInput{Name: "Main 2"})
			return err
		}},
		{"ledgers delete", http.MethodDelete, ledgerScope, func(ctx context.Context, c *Client) error {
			return c.V2.Ledgers.Delete(ctx, "org-1", "led-1")
		}},
		{"ledgers count", http.MethodHead, orgScope + "/ledgers/metrics/count", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Ledgers.Count(ctx, "org-1")
			return err
		}},
		{"ledger settings get", http.MethodGet, ledgerScope + "/settings", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Ledgers.GetSettings(ctx, "org-1", "led-1")
			return err
		}},
		{"ledger settings update", http.MethodPatch, ledgerScope + "/settings", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Ledgers.UpdateSettings(ctx, "org-1", "led-1",
				models.NewUpdateLedgerSettingsInput().WithRequireHolder(true))
			return err
		}},

		// --- accounts ------------------------------------------------------
		{"accounts list", http.MethodGet, ledgerScope + "/accounts", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Accounts.List(ctx, "org-1", "led-1", models.AccountsListOpts{})
			return err
		}},
		{"accounts create", http.MethodPost, ledgerScope + "/accounts", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Accounts.Create(ctx, "org-1", "led-1", &models.CreateAccountInput{Name: "Cash", AssetCode: "USD", Type: "deposit"})
			return err
		}},
		{"accounts get", http.MethodGet, ledgerScope + "/accounts/acc-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Accounts.Get(ctx, "org-1", "led-1", "acc-1")
			return err
		}},
		{"accounts get by alias", http.MethodGet, ledgerScope + "/accounts/alias/@cash", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Accounts.GetByAlias(ctx, "org-1", "led-1", "@cash")
			return err
		}},
		{"accounts get by external code", http.MethodGet, ledgerScope + "/accounts/external/USD", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Accounts.GetByExternalCode(ctx, "org-1", "led-1", "USD")
			return err
		}},
		{"accounts update", http.MethodPatch, ledgerScope + "/accounts/acc-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Accounts.Update(ctx, "org-1", "led-1", "acc-1", &models.UpdateAccountInput{Name: "Cash 2"})
			return err
		}},
		{"accounts delete", http.MethodDelete, ledgerScope + "/accounts/acc-1", func(ctx context.Context, c *Client) error {
			return c.V2.Accounts.Delete(ctx, "org-1", "led-1", "acc-1")
		}},
		{"accounts count", http.MethodHead, ledgerScope + "/accounts/metrics/count", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Accounts.Count(ctx, "org-1", "led-1")
			return err
		}},

		// --- account types --------------------------------------------------
		{"account types list", http.MethodGet, ledgerScope + "/account-types", func(ctx context.Context, c *Client) error {
			_, err := c.V2.AccountTypes.List(ctx, "org-1", "led-1", models.AccountTypesListOpts{})
			return err
		}},
		{"account types create", http.MethodPost, ledgerScope + "/account-types", func(ctx context.Context, c *Client) error {
			_, err := c.V2.AccountTypes.Create(ctx, "org-1", "led-1",
				&models.CreateAccountTypeInput{Name: "Deposit", KeyValue: "deposit"})
			return err
		}},
		{"account types get", http.MethodGet, ledgerScope + "/account-types/at-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.AccountTypes.Get(ctx, "org-1", "led-1", "at-1")
			return err
		}},
		{"account types update", http.MethodPatch, ledgerScope + "/account-types/at-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.AccountTypes.Update(ctx, "org-1", "led-1", "at-1", &models.UpdateAccountTypeInput{Name: "Deposit 2"})
			return err
		}},
		{"account types delete", http.MethodDelete, ledgerScope + "/account-types/at-1", func(ctx context.Context, c *Client) error {
			return c.V2.AccountTypes.Delete(ctx, "org-1", "led-1", "at-1")
		}},

		// --- assets ---------------------------------------------------------
		{"assets list", http.MethodGet, ledgerScope + "/assets", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Assets.List(ctx, "org-1", "led-1", models.AssetsListOpts{})
			return err
		}},
		{"assets create", http.MethodPost, ledgerScope + "/assets", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Assets.Create(ctx, "org-1", "led-1", &models.CreateAssetInput{Name: "US Dollar", Type: "currency", Code: "USD"})
			return err
		}},
		{"assets get", http.MethodGet, ledgerScope + "/assets/ast-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Assets.Get(ctx, "org-1", "led-1", "ast-1")
			return err
		}},
		{"assets update", http.MethodPatch, ledgerScope + "/assets/ast-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Assets.Update(ctx, "org-1", "led-1", "ast-1", &models.UpdateAssetInput{Name: "USD"})
			return err
		}},
		{"assets delete", http.MethodDelete, ledgerScope + "/assets/ast-1", func(ctx context.Context, c *Client) error {
			return c.V2.Assets.Delete(ctx, "org-1", "led-1", "ast-1")
		}},
		{"assets count", http.MethodHead, ledgerScope + "/assets/metrics/count", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Assets.Count(ctx, "org-1", "led-1")
			return err
		}},

		// --- balances -------------------------------------------------------
		{"balances list", http.MethodGet, ledgerScope + "/balances", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Balances.ListBalances(ctx, "org-1", "led-1", models.BalancesListOpts{})
			return err
		}},
		{"balances list by account", http.MethodGet, ledgerScope + "/accounts/acc-1/balances", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Balances.ListAccountBalances(ctx, "org-1", "led-1", "acc-1", models.BalancesListOpts{})
			return err
		}},
		{"balances get", http.MethodGet, ledgerScope + "/balances/bal-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Balances.GetBalance(ctx, "org-1", "led-1", "bal-1")
			return err
		}},
		{"balances history", http.MethodGet, ledgerScope + "/balances/bal-1/history", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Balances.GetBalanceHistory(ctx, "org-1", "led-1", "bal-1", historyDate)
			return err
		}},
		{"balances account history", http.MethodGet, ledgerScope + "/accounts/acc-1/balances/history", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Balances.GetAccountBalancesHistory(ctx, "org-1", "led-1", "acc-1", historyDate)
			return err
		}},
		{"balances create", http.MethodPost, ledgerScope + "/accounts/acc-1/balances", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Balances.CreateBalance(ctx, "org-1", "led-1", "acc-1", &models.CreateBalanceInput{Key: "default"})
			return err
		}},
		{"balances update", http.MethodPatch, ledgerScope + "/balances/bal-1", func(ctx context.Context, c *Client) error {
			allow := false
			_, err := c.V2.Balances.UpdateBalance(ctx, "org-1", "led-1", "bal-1", &models.UpdateBalanceInput{AllowSending: &allow})
			return err
		}},
		{"balances delete", http.MethodDelete, ledgerScope + "/balances/bal-1", func(ctx context.Context, c *Client) error {
			return c.V2.Balances.DeleteBalance(ctx, "org-1", "led-1", "bal-1")
		}},
		{"balances by alias", http.MethodGet, ledgerScope + "/accounts/alias/@cash/balances", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Balances.ListBalancesByAccountAlias(ctx, "org-1", "led-1", "@cash")
			return err
		}},
		{"balances by external code", http.MethodGet, ledgerScope + "/accounts/external/USD/balances", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Balances.ListBalancesByExternalCode(ctx, "org-1", "led-1", "USD")
			return err
		}},

		// --- operations -----------------------------------------------------
		{"operations list", http.MethodGet, ledgerScope + "/accounts/acc-1/operations", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Operations.ListOperations(ctx, "org-1", "led-1", "acc-1", models.OperationsListOpts{})
			return err
		}},
		{"operations get", http.MethodGet, ledgerScope + "/accounts/acc-1/operations/op-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Operations.GetOperation(ctx, "org-1", "led-1", "acc-1", "op-1")
			return err
		}},
		{"operations update", http.MethodPatch, ledgerScope + "/transactions/tx-1/operations/op-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Operations.UpdateTransactionOperation(ctx, "org-1", "led-1", "tx-1", "op-1",
				&models.UpdateOperationInput{Description: "updated"})
			return err
		}},

		// --- portfolios -----------------------------------------------------
		{"portfolios list", http.MethodGet, ledgerScope + "/portfolios", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Portfolios.List(ctx, "org-1", "led-1", models.PortfoliosListOpts{})
			return err
		}},
		{"portfolios create", http.MethodPost, ledgerScope + "/portfolios", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Portfolios.Create(ctx, "org-1", "led-1", &models.CreatePortfolioInput{Name: "Retail", EntityID: "ent-1"})
			return err
		}},
		{"portfolios get", http.MethodGet, ledgerScope + "/portfolios/pf-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Portfolios.Get(ctx, "org-1", "led-1", "pf-1")
			return err
		}},
		{"portfolios update", http.MethodPatch, ledgerScope + "/portfolios/pf-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Portfolios.Update(ctx, "org-1", "led-1", "pf-1", &models.UpdatePortfolioInput{Name: "Retail 2"})
			return err
		}},
		{"portfolios delete", http.MethodDelete, ledgerScope + "/portfolios/pf-1", func(ctx context.Context, c *Client) error {
			return c.V2.Portfolios.Delete(ctx, "org-1", "led-1", "pf-1")
		}},
		{"portfolios count", http.MethodHead, ledgerScope + "/portfolios/metrics/count", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Portfolios.Count(ctx, "org-1", "led-1")
			return err
		}},

		// --- segments -------------------------------------------------------
		{"segments list", http.MethodGet, ledgerScope + "/segments", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Segments.List(ctx, "org-1", "led-1", models.SegmentsListOpts{})
			return err
		}},
		{"segments create", http.MethodPost, ledgerScope + "/segments", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Segments.Create(ctx, "org-1", "led-1", &models.CreateSegmentInput{Name: "Premium"})
			return err
		}},
		{"segments get", http.MethodGet, ledgerScope + "/segments/sg-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Segments.Get(ctx, "org-1", "led-1", "sg-1")
			return err
		}},
		{"segments update", http.MethodPatch, ledgerScope + "/segments/sg-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Segments.Update(ctx, "org-1", "led-1", "sg-1", &models.UpdateSegmentInput{Name: "Premium 2"})
			return err
		}},
		{"segments delete", http.MethodDelete, ledgerScope + "/segments/sg-1", func(ctx context.Context, c *Client) error {
			return c.V2.Segments.Delete(ctx, "org-1", "led-1", "sg-1")
		}},
		{"segments count", http.MethodHead, ledgerScope + "/segments/metrics/count", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Segments.Count(ctx, "org-1", "led-1")
			return err
		}},

		// --- routes ---------------------------------------------------------
		{"operation routes list", http.MethodGet, ledgerScope + "/operation-routes", func(ctx context.Context, c *Client) error {
			_, err := c.V2.OperationRoutes.List(ctx, "org-1", "led-1", models.OperationRoutesListOpts{})
			return err
		}},
		{"operation routes create", http.MethodPost, ledgerScope + "/operation-routes", func(ctx context.Context, c *Client) error {
			_, err := c.V2.OperationRoutes.Create(ctx, "org-1", "led-1",
				&models.CreateOperationRouteInput{Title: "Settle", Description: "settlement", OperationType: "source"})
			return err
		}},
		{"operation routes get", http.MethodGet, ledgerScope + "/operation-routes/or-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.OperationRoutes.Get(ctx, "org-1", "led-1", "or-1")
			return err
		}},
		{"operation routes update", http.MethodPatch, ledgerScope + "/operation-routes/or-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.OperationRoutes.Update(ctx, "org-1", "led-1", "or-1", &models.UpdateOperationRouteInput{Title: "Settle 2"})
			return err
		}},
		{"operation routes delete", http.MethodDelete, ledgerScope + "/operation-routes/or-1", func(ctx context.Context, c *Client) error {
			return c.V2.OperationRoutes.Delete(ctx, "org-1", "led-1", "or-1")
		}},
		{"transaction routes list", http.MethodGet, ledgerScope + "/transaction-routes", func(ctx context.Context, c *Client) error {
			_, err := c.V2.TransactionRoutes.List(ctx, "org-1", "led-1", models.TransactionRoutesListOpts{})
			return err
		}},
		{"transaction routes create", http.MethodPost, ledgerScope + "/transaction-routes", func(ctx context.Context, c *Client) error {
			_, err := c.V2.TransactionRoutes.Create(ctx, "org-1", "led-1",
				models.NewCreateTransactionRouteInput("Cash in", "cash in", []string{"11111111-1111-1111-1111-111111111111"}))
			return err
		}},
		{"transaction routes get", http.MethodGet, ledgerScope + "/transaction-routes/tr-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.TransactionRoutes.Get(ctx, "org-1", "led-1", "tr-1")
			return err
		}},
		{"transaction routes update", http.MethodPatch, ledgerScope + "/transaction-routes/tr-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.TransactionRoutes.Update(ctx, "org-1", "led-1", "tr-1", &models.UpdateTransactionRouteInput{Title: "Cash in 2"})
			return err
		}},
		{"transaction routes delete", http.MethodDelete, ledgerScope + "/transaction-routes/tr-1", func(ctx context.Context, c *Client) error {
			return c.V2.TransactionRoutes.Delete(ctx, "org-1", "led-1", "tr-1")
		}},

		// --- transactions (reads + lifecycle) --------------------------------
		{"transactions list", http.MethodGet, ledgerScope + "/transactions", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Transactions.List(ctx, "org-1", "led-1", models.TransactionsListOpts{})
			return err
		}},
		{"transactions get", http.MethodGet, ledgerScope + "/transactions/tx-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Transactions.Get(ctx, "org-1", "led-1", "tx-1")
			return err
		}},
		{"transactions update", http.MethodPatch, ledgerScope + "/transactions/tx-1", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Transactions.Update(ctx, "org-1", "led-1", "tx-1",
				&models.UpdateTransactionV2Input{Description: "corrected"})
			return err
		}},
		{"transactions commit", http.MethodPost, ledgerScope + "/transactions/tx-1/commit", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Transactions.Commit(ctx, "org-1", "led-1", "tx-1")
			return err
		}},
		{"transactions cancel", http.MethodPost, ledgerScope + "/transactions/tx-1/cancel", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Transactions.Cancel(ctx, "org-1", "led-1", "tx-1")
			return err
		}},
		{"transactions revert", http.MethodPost, ledgerScope + "/transactions/tx-1/revert", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Transactions.Revert(ctx, "org-1", "led-1", "tx-1")
			return err
		}},
		{"transactions count", http.MethodHead, ledgerScope + "/transactions/metrics/count", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Transactions.Count(ctx, "org-1", "led-1", models.TransactionsListOpts{})
			return err
		}},

		// --- metadata indexes (unscoped) -------------------------------------
		{"metadata indexes list", http.MethodGet, "/v2/settings/metadata-indexes", func(ctx context.Context, c *Client) error {
			_, err := c.V2.MetadataIndexes.List(ctx, "transaction")
			return err
		}},
		{"metadata indexes create", http.MethodPost, "/v2/settings/metadata-indexes/entities/transaction", func(ctx context.Context, c *Client) error {
			_, err := c.V2.MetadataIndexes.Create(ctx, "transaction", &models.CreateMetadataIndexInput{MetadataKey: "reference"})
			return err
		}},
		{"metadata indexes delete", http.MethodDelete, "/v2/settings/metadata-indexes/entities/transaction/key/reference", func(ctx context.Context, c *Client) error {
			return c.V2.MetadataIndexes.Delete(ctx, "transaction", "reference")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seenMethods, seenPaths []string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seenMethods = append(seenMethods, r.Method)
				seenPaths = append(seenPaths, r.URL.Path)

				writeV2WireResponse(w, r)
			}))
			defer srv.Close()

			c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
			require.NoError(t, err)

			require.NoError(t, tt.call(context.Background(), c))
			require.Equal(t, []string{tt.wantPath}, seenPaths)
			require.Equal(t, []string{tt.wantMethod}, seenMethods)
		})
	}
}

// writeV2WireResponse answers each shape the routing table needs. A count is a
// HEAD reply carrying only the total header; a delete is a bodiless 204;
// everything else gets a body shaped by the path.
func writeV2WireResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		w.Header().Set("X-Total-Count", "7")
		w.WriteHeader(http.StatusOK)

		return
	}

	if r.Method == http.MethodDelete {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(v2WireBody(r.URL.Path)))
}

// v2WireBody picks a body shape from the path: a bare array for the two
// point-in-time reads and the metadata-index list, a paginated envelope for the
// collections, a single object otherwise.
func v2WireBody(path string) string {
	const objectUUID = "44444444-4444-4444-4444-444444444444"

	switch {
	case strings.Contains(path, "/operations/"):
		// An operation's amount is an object, not a scalar — a body that spells
		// it as a string decodes into nothing and would make this table green
		// while the facade returned a zero-valued operation.
		return `{"id":"` + objectUUID + `","assetCode":"USD","amount":{"value":"10"},` +
			`"balance":{"available":"10","onHold":"0"},"balanceAfter":{"available":"20","onHold":"0"}}`
	case strings.HasSuffix(path, "/balances/history"),
		strings.HasSuffix(path, "/metadata-indexes"):
		return `[]`
	case strings.HasSuffix(path, "/balances"),
		strings.HasSuffix(path, "/operations"),
		strings.HasSuffix(path, "/accounts"),
		strings.HasSuffix(path, "/account-types"),
		strings.HasSuffix(path, "/assets"),
		strings.HasSuffix(path, "/ledgers"),
		strings.HasSuffix(path, "/organizations"),
		strings.HasSuffix(path, "/portfolios"),
		strings.HasSuffix(path, "/segments"),
		strings.HasSuffix(path, "/operation-routes"),
		strings.HasSuffix(path, "/transaction-routes"),
		strings.HasSuffix(path, "/transactions"):
		return `{"items":[],"limit":10}`
	default:
		return `{"id":"` + objectUUID + `","assetCode":"USD","available":"10","amount":"10","status":{"code":"APPROVED"}}`
	}
}

// TestV2SurfaceAccountHolderHonoursTheSchemaSplit pins the response split RC2
// introduced on accounts.
//
// The two surfaces answer with DIFFERENT schemas over one SDK type: /v2 answers
// with AccountV2, which carries holderId and holderCheckSkipped, while /v1
// answers with Account, which has neither key. One decoded struct serves both,
// so the guard has to run in both directions — a /v2 read that dropped the
// holder leaves a caller unable to tell whose account they are looking at, and a
// /v1 read that invented one would be worse, because it reads as fact.
func TestV2SurfaceAccountHolderHonoursTheSchemaSplit(t *testing.T) {
	const (
		accountID = "44444444-4444-4444-4444-444444444444"
		holderID  = "55555555-5555-5555-5555-555555555555"
	)

	clientAnswering := func(t *testing.T, body string) *Client {
		t.Helper()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		t.Cleanup(srv.Close)

		c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
		require.NoError(t, err)

		return c
	}

	t.Run("v2 decodes the holder pair", func(t *testing.T) {
		c := clientAnswering(t, `{"id":"`+accountID+`","assetCode":"USD","type":"deposit",`+
			`"holderId":"`+holderID+`","holderCheckSkipped":true}`)

		account, err := c.V2.Accounts.Get(context.Background(), "org-1", "led-1", accountID)
		require.NoError(t, err)
		require.NotNil(t, account.HolderID, "the /v2 AccountV2 schema carries holderId; dropping it loses who owns the account")
		require.Equal(t, holderID, *account.HolderID)
		require.True(t, account.HolderCheckSkipped)
	})

	t.Run("v1 leaves the holder pair zero", func(t *testing.T) {
		c := clientAnswering(t, `{"id":"`+accountID+`","assetCode":"USD","type":"deposit"}`)

		account, err := c.V1.Accounts.Get(context.Background(), "org-1", "led-1", accountID)
		require.NoError(t, err)
		require.Nil(t, account.HolderID, "the /v1 Account schema has no holder keys; anything decoded here would be invented")
		require.False(t, account.HolderCheckSkipped)
	})
}

// TestV2ListsSendTheirPaginationStyle pins that each V2 list advances the way its
// endpoint actually paginates.
//
// A cursor endpoint that receives "page" is not a harmless extra parameter: the
// server drops it, so a caller who set it reads page one believing they read
// page four. The reverse — a page endpoint receiving only a cursor — silently
// re-reads the first page too. Both were live money-path defects on /v1 before
// Epic 2, so the V2 twins get the guard on arrival.
func TestV2ListsSendTheirPaginationStyle(t *testing.T) {
	tests := []struct {
		name     string
		wantKey  string
		absent   string
		wantVal  string
		callList func(context.Context, *Client) error
	}{
		{
			name: "organizations paginate by page", wantKey: "page", wantVal: "3", absent: "cursor",
			callList: func(ctx context.Context, c *Client) error {
				_, err := c.V2.Organizations.List(ctx, models.OrganizationsListOpts{
					PageListOpts: models.PageListOpts{Page: 3, Limit: 5},
				})

				return err
			},
		},
		{
			name: "balances paginate by cursor", wantKey: "cursor", wantVal: "cur-1", absent: "page",
			callList: func(ctx context.Context, c *Client) error {
				_, err := c.V2.Balances.ListBalances(ctx, "org-1", "led-1", models.BalancesListOpts{
					CursorListOpts: models.CursorListOpts{Cursor: "cur-1", Limit: 5},
				})

				return err
			},
		},
		{
			name: "transactions paginate by cursor", wantKey: "cursor", wantVal: "cur-1", absent: "page",
			callList: func(ctx context.Context, c *Client) error {
				_, err := c.V2.Transactions.List(ctx, "org-1", "led-1", models.TransactionsListOpts{
					CursorListOpts: models.CursorListOpts{Cursor: "cur-1", Limit: 5},
				})

				return err
			},
		},
		{
			name: "operation routes paginate by cursor", wantKey: "cursor", wantVal: "cur-1", absent: "page",
			callList: func(ctx context.Context, c *Client) error {
				_, err := c.V2.OperationRoutes.List(ctx, "org-1", "led-1", models.OperationRoutesListOpts{
					CursorListOpts: models.CursorListOpts{Cursor: "cur-1", Limit: 5},
				})

				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen url.Values

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = r.URL.Query()

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"items":[],"limit":5}`))
			}))
			defer srv.Close()

			c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
			require.NoError(t, err)

			require.NoError(t, tt.callList(context.Background(), c))
			require.Equal(t, tt.wantVal, seen.Get(tt.wantKey))
			require.Empty(t, seen.Get(tt.absent),
				"%q has no wire slot on this endpoint; sending it tells the caller a lie about how the list advances", tt.absent)
			require.Equal(t, "5", seen.Get("limit"))
		})
	}
}

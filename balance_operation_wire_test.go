package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
	"github.com/stretchr/testify/require"
)

// TestBalanceAndOperationWirePaths pins ROUTING for every balance and operation
// call: the HTTP method, the URL path, and the query parameters. That is the
// whole of what it guards.
//
// It guards nothing else, on purpose. The fake answers 200 with a
// path-shaped body to every request and the only assertion on the call itself
// is that it returned no error, so request bodies, status handling, and decode
// behavior are all outside this table. Those live in the per-facade tests in
// entities/ and in the decode and idempotency suites; growing this fake into a
// mock server would duplicate them and rot.
//
// Routing is worth its own table because the client is built from the PUBLIC
// options (WithBaseURL against a live test server), so the assertion covers
// base-URL normalization and path construction together. A baseURLs-map
// injection test cannot: injecting a base that already carries "/v1" makes
// every path assertion pass while the shipped client — whose Ledger base is
// bare, because the generated client versions its own paths — emits an
// unversioned path the server routes nowhere. The bug that motivated this test
// was invisible to the injected-map tests for exactly that reason.
//
// One row per endpoint: the length is the endpoint count, and collapsing rows
// would hide the exact routing contract this guard exists to pin.
func TestBalanceAndOperationWirePaths(t *testing.T) {
	const (
		ledgerScope = "/v1/organizations/org-1/ledgers/led-1"
		historyDate = "2026-01-02 03:04:05"
	)

	tests := []struct {
		name       string
		wantMethod string
		wantPath   string
		wantQuery  map[string]string
		call       func(context.Context, *Client) error
	}{
		{
			name:       "list ledger balances",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/balances",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Balances.ListBalances(ctx, "org-1", "led-1", models.BalancesListOpts{})
				return err
			},
		},
		{
			name:       "list account balances",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/accounts/acc-1/balances",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Balances.ListAccountBalances(ctx, "org-1", "led-1", "acc-1", models.BalancesListOpts{})
				return err
			},
		},
		{
			name:       "get one balance",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/balances/bal-1",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Balances.GetBalance(ctx, "org-1", "led-1", "bal-1")
				return err
			},
		},
		{
			name:       "get balance history",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/balances/bal-1/history",
			wantQuery:  map[string]string{"date": historyDate},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Balances.GetBalanceHistory(ctx, "org-1", "led-1", "bal-1", historyDate)
				return err
			},
		},
		{
			name:       "update a balance",
			wantMethod: http.MethodPatch,
			wantPath:   ledgerScope + "/balances/bal-1",
			call: func(ctx context.Context, c *Client) error {
				allow := false
				_, err := c.V1.Balances.UpdateBalance(ctx, "org-1", "led-1", "bal-1",
					&models.UpdateBalanceInput{AllowSending: &allow})
				return err
			},
		},
		{
			name:       "delete a balance",
			wantMethod: http.MethodDelete,
			wantPath:   ledgerScope + "/balances/bal-1",
			call: func(ctx context.Context, c *Client) error {
				return c.V1.Balances.DeleteBalance(ctx, "org-1", "led-1", "bal-1")
			},
		},
		{
			name:       "create an additional account balance",
			wantMethod: http.MethodPost,
			wantPath:   ledgerScope + "/accounts/acc-1/balances",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Balances.CreateBalance(ctx, "org-1", "led-1", "acc-1",
					&models.CreateBalanceInput{Key: "default"})
				return err
			},
		},
		{
			name:       "list balances by account alias",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/accounts/alias/@cash/balances",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Balances.ListBalancesByAccountAlias(ctx, "org-1", "led-1", "@cash")
				return err
			},
		},
		{
			name:       "list balances by external code",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/accounts/external/USD/balances",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Balances.ListBalancesByExternalCode(ctx, "org-1", "led-1", "USD")
				return err
			},
		},
		{
			name:       "get account balances history",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/accounts/acc-1/balances/history",
			wantQuery:  map[string]string{"date": historyDate},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Balances.GetAccountBalancesHistory(ctx, "org-1", "led-1", "acc-1", historyDate)
				return err
			},
		},
		{
			name:       "list account operations",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/accounts/acc-1/operations",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Operations.ListOperations(ctx, "org-1", "led-1", "acc-1", models.OperationsListOpts{})
				return err
			},
		},
		{
			name:       "get one account operation",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/accounts/acc-1/operations/op-1",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Operations.GetOperation(ctx, "org-1", "led-1", "acc-1", "op-1")
				return err
			},
		},
		{
			name:       "update a transaction operation",
			wantMethod: http.MethodPatch,
			wantPath:   ledgerScope + "/transactions/tx-1/operations/op-1",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Operations.UpdateTransactionOperation(ctx, "org-1", "led-1", "tx-1", "op-1",
					&models.UpdateOperationInput{Description: "updated"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				seenMethods []string
				seenPaths   []string
				seenQuery   url.Values
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seenMethods = append(seenMethods, r.Method)
				seenPaths = append(seenPaths, r.URL.Path)
				seenQuery = r.URL.Query()

				// A bodiless 204 carries no Content-Type, matching what the
				// server sends: the generated parser only decodes a body when
				// the content type says JSON.
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusNoContent)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(balanceOperationWireBody(r.URL.Path)))
			}))
			defer srv.Close()

			c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
			require.NoError(t, err)

			require.NoError(t, tt.call(context.Background(), c))
			require.Equal(t, []string{tt.wantPath}, seenPaths)
			require.Equal(t, []string{tt.wantMethod}, seenMethods)

			for key, want := range tt.wantQuery {
				require.Equal(t, want, seenQuery.Get(key), "query param %q", key)
			}
		})
	}
}

// balanceOperationWireBody answers each shape the wire-path table needs.
//
// Every single-object response is decoded by the generated client into typed
// UUID fields before the SDK model sees it, so the ids here have to be real
// UUIDs — which is what a server sends. The list endpoints want a paginated
// envelope instead.
func balanceOperationWireBody(path string) string {
	const (
		balanceUUID = "44444444-4444-4444-4444-444444444444"
		accountUUID = "55555555-5555-5555-5555-555555555555"
	)

	switch {
	case strings.HasSuffix(path, "/balances/history"):
		return `[{"id":"` + balanceUUID + `","accountId":"` + accountUUID + `","assetCode":"USD","available":"10"}]`
	case strings.HasSuffix(path, "/history"):
		return `{"id":"` + balanceUUID + `","accountId":"` + accountUUID + `","assetCode":"USD","available":"10"}`
	case strings.HasSuffix(path, "/balances"), strings.HasSuffix(path, "/operations"):
		return `{"items":[],"limit":10}`
	default:
		return `{"id":"` + balanceUUID + `","accountId":"` + accountUUID + `","assetCode":"USD","available":"10","description":"updated"}`
	}
}

// TestListBalances_DoesNotSendPageOnCursorEndpoint pins the pagination STYLE of
// the ledger-wide balances list. The server paginates it by opaque cursor: its
// handler builds the response envelope from limit/sort_order/start_date/end_date
// plus a cursor, and drops "page" on the floor. Sending "page" therefore buys
// nothing and actively lies to the caller about how the list advances.
func TestListBalances_DoesNotSendPageOnCursorEndpoint(t *testing.T) {
	var seenQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":5}`))
	}))
	defer srv.Close()

	c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
	require.NoError(t, err)

	_, err = c.V1.Balances.ListBalances(context.Background(), "org-1", "led-1", models.BalancesListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 5, Cursor: "cur-1", SortDirection: models.SortDescending},
	})
	require.NoError(t, err)

	require.Equal(t, "5", seenQuery.Get("limit"))
	require.Equal(t, "cur-1", seenQuery.Get("cursor"))
	require.Equal(t, "desc", seenQuery.Get("sort_order"))
	require.Empty(t, seenQuery.Get("page"), "the balances list is cursor-paginated; page has no wire slot")
}

// TestListBalancesAll_AdvancesByCursor is the money-path infinite-loop guard for
// the ledger-wide balances iterator. The endpoint advances by next_cursor, so an
// iterator that increments a page number instead re-requests the FIRST page for
// as long as the server keeps reporting more results — an unbounded loop that
// yields the same balances forever. The request cap turns that into a fast
// failure instead of a hang.
func TestListBalancesAll_AdvancesByCursor(t *testing.T) {
	var seenCursors []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		seenCursors = append(seenCursors, cursor)

		if len(seenCursors) > 4 {
			t.Fatalf("iterator did not terminate: %d requests, cursors=%v", len(seenCursors), seenCursors)
		}

		w.Header().Set("Content-Type", "application/json")

		if cursor == "cur-2" {
			_, _ = w.Write([]byte(`{"items":[{"id":"b-2","assetCode":"USD","available":"20"}],"limit":1}`))
			return
		}

		_, _ = w.Write([]byte(`{"items":[{"id":"b-1","assetCode":"USD","available":"10"}],"limit":1,"next_cursor":"cur-2"}`))
	}))
	defer srv.Close()

	c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
	require.NoError(t, err)

	var ids []string

	for balance, err := range c.V1.Balances.ListBalancesAll(context.Background(), "org-1", "led-1",
		models.BalancesListOpts{CursorListOpts: models.CursorListOpts{Limit: 1}}) {
		require.NoError(t, err)
		ids = append(ids, balance.ID)
	}

	require.Equal(t, []string{"b-1", "b-2"}, ids)
	require.Equal(t, []string{"", "cur-2"}, seenCursors, "the iterator must advance by next_cursor")
}

// TestAccountScopedCursorIterators_AdvanceByCursor is the sibling of
// TestListBalancesAll_AdvancesByCursor for the two account-scoped iterators.
// Same money-path hazard: both endpoints advance by next_cursor, so an iterator
// that incremented a page number instead would re-request the FIRST page for as
// long as the server reported more results — an unbounded loop yielding the same
// balances or operations forever. The request cap turns that into a fast failure
// instead of a hang.
func TestAccountScopedCursorIterators_AdvanceByCursor(t *testing.T) {
	tests := []struct {
		name    string
		collect func(context.Context, *Client) ([]string, error)
	}{
		{"account balances", collectAccountBalanceIDs},
		{"account operations", collectAccountOperationIDs},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seenCursors []string

			srv := httptest.NewServer(twoCursorPageHandler(t, &seenCursors))
			defer srv.Close()

			c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
			require.NoError(t, err)

			ids, err := tt.collect(context.Background(), c)
			require.NoError(t, err)
			require.Equal(t, []string{"x-1", "x-2"}, ids)
			require.Equal(t, []string{"", "cur-2"}, seenCursors, "the iterator must advance by next_cursor")
		})
	}
}

// twoCursorPageHandler serves exactly two cursor pages and fails the test if it
// is asked for more than a handful, so a page-number iterator hits a fast
// failure instead of looping forever.
func twoCursorPageHandler(t *testing.T, seenCursors *[]string) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		*seenCursors = append(*seenCursors, cursor)

		if len(*seenCursors) > 4 {
			t.Fatalf("iterator did not terminate: %d requests, cursors=%v", len(*seenCursors), *seenCursors)
		}

		w.Header().Set("Content-Type", "application/json")

		if cursor == "cur-2" {
			_, _ = w.Write([]byte(`{"items":[{"id":"x-2","assetCode":"USD","available":"20"}],"limit":1}`))
			return
		}

		_, _ = w.Write([]byte(`{"items":[{"id":"x-1","assetCode":"USD","available":"10"}],"limit":1,"next_cursor":"cur-2"}`))
	}
}

func collectAccountBalanceIDs(ctx context.Context, c *Client) ([]string, error) {
	var ids []string

	for balance, err := range c.V1.Balances.ListAccountBalancesAll(ctx, "org-1", "led-1", "acc-1",
		models.BalancesListOpts{CursorListOpts: models.CursorListOpts{Limit: 1}}) {
		if err != nil {
			return ids, err
		}

		ids = append(ids, balance.ID)
	}

	return ids, nil
}

func collectAccountOperationIDs(ctx context.Context, c *Client) ([]string, error) {
	var ids []string

	for op, err := range c.V1.Operations.ListOperationsAll(ctx, "org-1", "led-1", "acc-1",
		models.OperationsListOpts{CursorListOpts: models.CursorListOpts{Limit: 1}}) {
		if err != nil {
			return ids, err
		}

		ids = append(ids, op.ID)
	}

	return ids, nil
}

// TestListOperations_SendsOnlyHonoredFilters pins the account-operations filter
// set to what the server actually applies: operation type, accounting direction,
// and the operation route (by id or code). A filter with no wire slot is worse
// than an absent one — the caller reads a full unfiltered result set as if it
// had been narrowed.
func TestListOperations_SendsOnlyHonoredFilters(t *testing.T) {
	var seenQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":10}`))
	}))
	defer srv.Close()

	c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
	require.NoError(t, err)

	_, err = c.V1.Operations.ListOperations(context.Background(), "org-1", "led-1", "acc-1", models.OperationsListOpts{
		Filters: models.OperationsFilters{
			Type:      "DEBIT",
			Direction: "debit",
			RouteID:   "route-1",
			RouteCode: "RC1",
		},
	})
	require.NoError(t, err)

	for key, want := range map[string]string{
		"type": "DEBIT", "direction": "debit", "route_id": "route-1", "route_code": "RC1",
	} {
		require.Equal(t, want, seenQuery.Get(key), "filter %q must reach the wire", key)
	}
}

// TestBalanceAndOperationBaseURLIsNotDoubleVersioned guards the other direction:
// the Ledger base URL the client actually holds must stay bare, so the "/v1" on
// the wire comes from exactly one place.
func TestBalanceAndOperationBaseURLIsNotDoubleVersioned(t *testing.T) {
	c, err := New(WithConfig(createTestConfig(t)), WithBaseURL("https://ledger.example.com"))
	require.NoError(t, err)

	client := c.GetEntityHTTPClient()
	require.NotNil(t, client)

	urls := c.GetConfig().ServiceURLs
	require.Equal(t, "https://ledger.example.com", urls["onboarding"])
	require.Equal(t, "https://ledger.example.com/v1", urls["tracer"])
}

// TestLedgerURLAloneIsEnoughToConstruct pins the configuration surface down to
// the one URL a caller actually has to supply.
//
// There used to be a second mandatory Ledger key — an internal "transaction"
// routing label populated from the same Ledger URL. Once every accessor moved
// onto the generated plane clients, nothing read it, but construction still
// FAILED when it was absent: an operator handing the SDK a hand-built config
// with only a Ledger URL got an error naming a service that no longer exists as
// a distinct thing. Supplying the Ledger plane is now sufficient.
//
// Both spellings are checked, because they derive the Tracer base differently
// and only one of them touches it: WithLedgerURL addresses the Ledger alone and
// leaves the Tracer on its environment default, while WithBaseURL declares one
// shared origin and fans it out to both planes. Either way the Tracer base ends
// up carrying the "/v1" its spec requires.
func TestLedgerURLAloneIsEnoughToConstruct(t *testing.T) {
	t.Run("WithLedgerURL leaves the tracer on its default", func(t *testing.T) {
		cfg, err := config.NewConfig(
			config.WithAnonymous(),
			config.WithEnvironment(config.EnvironmentLocal),
			config.WithLedgerURL("https://ledger.example.com"),
		)
		require.NoError(t, err, "a Ledger URL on its own must be a valid configuration")
		require.NoError(t, cfg.Validate())

		c, err := New(WithConfig(cfg))
		require.NoError(t, err, "construction must succeed with only a Ledger URL supplied")

		urls := c.GetConfig().ServiceURLs
		require.Equal(t, "https://ledger.example.com", urls[config.ServiceOnboarding])
		require.Equal(t, "http://localhost:4020/v1", urls[config.ServiceTracer],
			"the tracer keeps its own base; WithLedgerURL addresses the Ledger only")
	})

	t.Run("WithBaseURL fans one origin out to both planes", func(t *testing.T) {
		cfg, err := config.NewConfig(
			config.WithAnonymous(),
			config.WithBaseURL("https://midaz.example.com"),
		)
		require.NoError(t, err)
		require.NoError(t, cfg.Validate())

		c, err := New(WithConfig(cfg))
		require.NoError(t, err)

		urls := c.GetConfig().ServiceURLs
		require.Equal(t, "https://midaz.example.com", urls[config.ServiceOnboarding])
		require.Equal(t, "https://midaz.example.com/v1", urls[config.ServiceTracer])
	})
}

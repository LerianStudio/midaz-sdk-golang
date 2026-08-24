package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/stretchr/testify/require"
)

// TestBalanceAndOperationWirePaths pins the method, path and query every balance
// and operation call puts on the wire, driving the REAL pipeline: the client is
// built from the public options (WithBaseURL against a live test server), so the
// assertion covers base-URL normalization AND path construction together.
//
// This is deliberately NOT a baseURLs-map injection test. Injecting a base that
// already carries "/v1" makes every path assertion pass while the shipped client
// — whose Ledger base is bare, because the generated client versions its own
// paths — emits an unversioned path the server routes nowhere. The bug that
// motivated this test was invisible to the injected-map tests for exactly that
// reason.
// The table holds one row per balance/operation endpoint: the length is the
// endpoint count, and collapsing rows would hide the exact wire contract this
// guard exists to pin.
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
				_, err := c.Balances.ListBalances(ctx, "org-1", "led-1", models.BalancesListOpts{})
				return err
			},
		},
		{
			name:       "list account balances",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/accounts/acc-1/balances",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Balances.ListAccountBalances(ctx, "org-1", "led-1", "acc-1", models.BalancesListOpts{})
				return err
			},
		},
		{
			name:       "get one balance",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/balances/bal-1",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Balances.GetBalance(ctx, "org-1", "led-1", "bal-1")
				return err
			},
		},
		{
			name:       "get balance history",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/balances/bal-1/history",
			wantQuery:  map[string]string{"date": historyDate},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Balances.GetBalanceHistory(ctx, "org-1", "led-1", "bal-1", historyDate)
				return err
			},
		},
		{
			name:       "update a balance",
			wantMethod: http.MethodPatch,
			wantPath:   ledgerScope + "/balances/bal-1",
			call: func(ctx context.Context, c *Client) error {
				allow := false
				_, err := c.Balances.UpdateBalance(ctx, "org-1", "led-1", "bal-1",
					&models.UpdateBalanceInput{AllowSending: &allow})
				return err
			},
		},
		{
			name:       "delete a balance",
			wantMethod: http.MethodDelete,
			wantPath:   ledgerScope + "/balances/bal-1",
			call: func(ctx context.Context, c *Client) error {
				return c.Balances.DeleteBalance(ctx, "org-1", "led-1", "bal-1")
			},
		},
		{
			name:       "create an additional account balance",
			wantMethod: http.MethodPost,
			wantPath:   ledgerScope + "/accounts/acc-1/balances",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Balances.CreateBalance(ctx, "org-1", "led-1", "acc-1",
					&models.CreateBalanceInput{Key: "default"})
				return err
			},
		},
		{
			name:       "list balances by account alias",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/accounts/alias/@cash/balances",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Balances.ListBalancesByAccountAlias(ctx, "org-1", "led-1", "@cash", models.BalancesListOpts{})
				return err
			},
		},
		{
			name:       "list balances by external code",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/accounts/external/USD/balances",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Balances.ListBalancesByExternalCode(ctx, "org-1", "led-1", "USD", models.BalancesListOpts{})
				return err
			},
		},
		{
			name:       "get account balances history",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/accounts/acc-1/balances/history",
			wantQuery:  map[string]string{"date": historyDate},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Balances.GetAccountBalancesHistory(ctx, "org-1", "led-1", "acc-1", historyDate)
				return err
			},
		},
		{
			name:       "list account operations",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/accounts/acc-1/operations",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Operations.ListOperations(ctx, "org-1", "led-1", "acc-1", models.OperationsListOpts{})
				return err
			},
		},
		{
			name:       "get one account operation",
			wantMethod: http.MethodGet,
			wantPath:   ledgerScope + "/accounts/acc-1/operations/op-1",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Operations.GetOperation(ctx, "org-1", "led-1", "acc-1", "op-1")
				return err
			},
		},
		{
			name:       "update a transaction operation",
			wantMethod: http.MethodPatch,
			wantPath:   ledgerScope + "/transactions/tx-1/operations/op-1",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Operations.UpdateTransactionOperation(ctx, "org-1", "led-1", "tx-1", "op-1",
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
	case path[len(path)-len("/balances/history"):] == "/balances/history":
		return `[{"id":"` + balanceUUID + `","accountId":"` + accountUUID + `","assetCode":"USD","available":"10"}]`
	case path[len(path)-len("/history"):] == "/history":
		return `{"id":"` + balanceUUID + `","accountId":"` + accountUUID + `","assetCode":"USD","available":"10"}`
	case path[len(path)-len("/balances"):] == "/balances",
		path[len(path)-len("/operations"):] == "/operations":
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

	_, err = c.Balances.ListBalances(context.Background(), "org-1", "led-1", models.BalancesListOpts{
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

	for balance, err := range c.Balances.ListBalancesAll(context.Background(), "org-1", "led-1",
		models.BalancesListOpts{CursorListOpts: models.CursorListOpts{Limit: 1}}) {
		require.NoError(t, err)
		ids = append(ids, balance.ID)
	}

	require.Equal(t, []string{"b-1", "b-2"}, ids)
	require.Equal(t, []string{"", "cur-2"}, seenCursors, "the iterator must advance by next_cursor")
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

	_, err = c.Operations.ListOperations(context.Background(), "org-1", "led-1", "acc-1", models.OperationsListOpts{
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

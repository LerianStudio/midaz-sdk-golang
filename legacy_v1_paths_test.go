package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/stretchr/testify/require"
)

// TestLegacyServices_EmitVersionedPaths pins the wire path the three remaining
// hand-rolled services emit, driving the REAL pipeline: the client is built from
// the public options (WithBaseURL against a live test server), so the assertion
// covers base-URL normalization AND path construction together.
//
// This is deliberately NOT a baseURLs-map injection test. Injecting a base that
// already carries "/v1" makes every path assertion pass while the shipped client
// — whose Ledger base is bare, because the generated client versions its own
// paths — emits an unversioned path the server routes nowhere. The bug that
// motivated this test was invisible to the injected-map tests for exactly that
// reason.
func TestLegacyServices_EmitVersionedPaths(t *testing.T) {
	tests := []struct {
		name     string
		wantPath string
		call     func(context.Context, *Client) error
	}{
		{
			name:     "list ledger balances",
			wantPath: "/v1/organizations/org-1/ledgers/led-1/balances",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Balances.ListBalances(ctx, "org-1", "led-1", models.BalancesListOpts{})
				return err
			},
		},
		{
			name:     "get one balance",
			wantPath: "/v1/organizations/org-1/ledgers/led-1/balances/bal-1",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Balances.GetBalance(ctx, "org-1", "led-1", "bal-1")
				return err
			},
		},
		{
			name:     "list account balances",
			wantPath: "/v1/organizations/org-1/ledgers/led-1/accounts/acc-1/balances",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Balances.ListAccountBalances(ctx, "org-1", "led-1", "acc-1", models.BalancesListOpts{})
				return err
			},
		},
		{
			name:     "list account operations",
			wantPath: "/v1/organizations/org-1/ledgers/led-1/accounts/acc-1/operations",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Operations.ListOperations(ctx, "org-1", "led-1", "acc-1", models.OperationsListOpts{})
				return err
			},
		},
		{
			name:     "get one account operation",
			wantPath: "/v1/organizations/org-1/ledgers/led-1/accounts/acc-1/operations/op-1",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Operations.GetOperation(ctx, "org-1", "led-1", "acc-1", "op-1")
				return err
			},
		},
		{
			name:     "update a transaction operation",
			wantPath: "/v1/organizations/org-1/ledgers/led-1/transactions/tx-1/operations/op-1",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Operations.UpdateTransactionOperation(ctx, "org-1", "led-1", "tx-1", "op-1",
					&models.UpdateOperationInput{Description: "updated"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seenPaths []string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seenPaths = append(seenPaths, r.URL.Path)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"items":[],"page":1,"limit":10}`))
			}))
			defer srv.Close()

			c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
			require.NoError(t, err)

			require.NoError(t, tt.call(context.Background(), c))
			require.Equal(t, []string{tt.wantPath}, seenPaths)
		})
	}
}

// TestLegacyServices_BareBaseURLIsNotDoubleVersioned guards the other direction:
// the Ledger base URL the client actually holds must stay bare, so the "/v1" on
// the wire comes from exactly one place.
func TestLegacyServices_BareBaseURLIsNotDoubleVersioned(t *testing.T) {
	c, err := New(WithConfig(createTestConfig(t)), WithBaseURL("https://ledger.example.com"))
	require.NoError(t, err)

	client := c.GetEntityHTTPClient()
	require.NotNil(t, client)

	urls := c.GetConfig().ServiceURLs
	require.Equal(t, "https://ledger.example.com", urls["onboarding"])
	require.Equal(t, "https://ledger.example.com/v1", urls["tracer"])
}

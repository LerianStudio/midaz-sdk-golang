package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/stretchr/testify/require"
)

// TestInvalidInputIsATypedValidationError pins the classification of a request
// the SDK refuses before it reaches the wire.
//
// Callers branch on sdkerrors.IsValidationError to tell "you sent something
// wrong, fix it and retry" apart from "the ledger is unhappy, do not retry
// blindly". A locally-refused payload that reports false there reads as an
// unclassified failure, and on a write path an unclassified failure is the one
// a caller is most likely to retry — against a request that never left.
//
// The facades hand back whatever the model's own Validate returned, and the
// models do not all return the SDK error type, so the classification depended
// on which model was involved.
func TestInvalidInputIsATypedValidationError(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context, *Client) error
	}{
		{
			name: "create a balance with no key",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Balances.CreateBalance(ctx, "org-1", "led-1", "acc-1", &models.CreateBalanceInput{})
				return err
			},
		},
		{
			name: "update a balance with an empty payload",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Balances.UpdateBalance(ctx, "org-1", "led-1", "bal-1", &models.UpdateBalanceInput{})
				return err
			},
		},
		{
			name: "update an operation with no payload",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Operations.UpdateTransactionOperation(ctx, "org-1", "led-1", "tx-1", "op-1", nil)
				return err
			},
		},
		{
			name: "create an account with no name",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Accounts.Create(ctx, "org-1", "led-1", &models.CreateAccountInput{})
				return err
			},
		},

		// --- money writes: the transaction path ---
		{
			name: "create a transaction with no operations",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Transactions.CreateJSON(ctx, "org-1", "led-1", &models.CreateTransactionInput{})
				return err
			},
		},
		{
			name: "create an inflow with no destination",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Transactions.CreateInflow(ctx, "org-1", "led-1", &models.CreateInflowInput{})
				return err
			},
		},
		{
			name: "create an outflow with no source",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Transactions.CreateOutflow(ctx, "org-1", "led-1", &models.CreateOutflowInput{})
				return err
			},
		},
		{
			name: "update a transaction with an empty payload",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Transactions.UpdateTransaction(ctx, "org-1", "led-1", "tx-1",
					&models.UpdateTransactionInput{})
				return err
			},
		},

		// --- money writes: reservations and billing ---
		{
			name: "reserve with an empty payload",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Reservations.Reserve(ctx, &models.ReserveInput{})
				return err
			},
		},
		{
			name: "create a billing package with an empty payload",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V2.BillingPackages.Create(ctx, "org-1", "led-1",
					&models.CreateBillingPackageInput{})
				return err
			},
		},
		{
			name: "calculate billing with an empty payload",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V2.BillingCalculations.CalculateBilling(ctx, "org-1", "led-1",
					&models.BillingCalculateInput{})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests int

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests++

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
			require.NoError(t, err)

			err = tt.call(context.Background(), c)
			require.Error(t, err)
			require.True(t, sdkerrors.IsValidationError(err),
				"a locally refused payload must classify as a validation failure, got %v", err)
			require.Zero(t, requests, "the request must never leave the SDK")
		})
	}
}

// TestListOptsValidationIsTyped verifies the one exemption the structural check
// in entities/input_validation_structural_test.go grants.
//
// That check demands every model's Validate be routed through validationErr,
// and exempts list options on the grounds that ValidatePageListOpts and
// ValidateCursorListOpts already build the SDK error type, so wrapping them
// would be a no-op. An exemption nobody exercises is just an untested branch
// with a comment on it, and the same shape of assumption is what let the
// original guard sample past 89 methods. So the premise gets its own rows:
// refuse a bad page-based option set and a bad cursor-based one, and read the
// classification back through the public surface.
func TestListOptsValidationIsTyped(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context, *Client) error
	}{
		{
			name: "page-based options with a negative limit",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Accounts.List(ctx, "org-1", "led-1",
					models.AccountsListOpts{PageListOpts: models.PageListOpts{Limit: -1}})
				return err
			},
		},
		{
			name: "page-based options with an unknown sort direction",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Accounts.List(ctx, "org-1", "led-1",
					models.AccountsListOpts{PageListOpts: models.PageListOpts{SortDirection: "sideways"}})
				return err
			},
		},
		{
			name: "cursor-based options with a negative limit",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.V1.Accounts.ListBalances(ctx, "org-1", "led-1", "acc-1",
					models.CursorListOpts{Limit: -1})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests int

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests++

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"items":[],"limit":10}`))
			}))
			defer srv.Close()

			c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
			require.NoError(t, err)

			err = tt.call(context.Background(), c)
			require.Error(t, err)
			require.True(t, sdkerrors.IsValidationError(err),
				"list options are exempt from validationErr because they type themselves; "+
					"if this fails, the exemption is wrong and the wrapping is required, got %v", err)
			require.Zero(t, requests, "the request must never leave the SDK")
		})
	}
}

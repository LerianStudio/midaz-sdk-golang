package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/stretchr/testify/require"
)

// TestEmptyPathID_SilentZeroOnMoneyReads is the money-path regression guard for
// the worst failure this SDK can produce: a read that answers "0" instead of
// failing.
//
// An empty id in a by-id read builds a path with a trailing slash
// (".../balances/"). Midaz runs Fiber with StrictRouting unset, so the router
// trims that slash and the request lands on the COLLECTION route, which answers
// 200 with a paginated envelope. Decoding that envelope into a single Balance
// yields every field at its zero value — Available: 0 — with a nil error. A
// reconciliation client books zero and nothing anywhere says the id was missing.
//
// The fix is a local guard: the request must never leave the SDK.
func TestEmptyPathID_SilentZeroOnMoneyReads(t *testing.T) {
	tests := []struct {
		name      string
		wantParam string
		call      func(context.Context, *Client) error
	}{
		{
			name:      "get a balance with no balance id",
			wantParam: "balanceID",
			call: func(ctx context.Context, c *Client) error {
				balance, err := c.V1.Balances.GetBalance(ctx, "org-1", "led-1", "")
				if err == nil {
					return &silentZeroError{what: "GetBalance", amount: balance.Available.String()}
				}

				return err
			},
		},
		{
			name:      "get an operation with no operation id",
			wantParam: "operationID",
			call: func(ctx context.Context, c *Client) error {
				op, err := c.V1.Operations.GetOperation(ctx, "org-1", "led-1", "acc-1", "")
				if err == nil {
					return &silentZeroError{what: "GetOperation", amount: op.ID}
				}

				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests int

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++

				// Mirror Fiber with StrictRouting unset: ".../balances/" and
				// ".../balances" are the same route.
				path := strings.TrimSuffix(r.URL.Path, "/")

				w.Header().Set("Content-Type", "application/json")

				if strings.HasSuffix(path, "/balances") || strings.HasSuffix(path, "/operations") {
					_, _ = w.Write([]byte(`{"items":[{"id":"b-1","assetCode":"USD","available":"1500.5"}],"limit":10}`))
					return
				}

				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"title":"not found","status":404}`))
			}))
			defer srv.Close()

			c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
			require.NoError(t, err)

			err = tt.call(context.Background(), c)
			require.Error(t, err, "an empty path id must fail")
			require.True(t, sdkerrors.IsValidationError(err),
				"an empty path id is a caller mistake the SDK can name locally, got %v", err)
			require.Contains(t, err.Error(), "missing required parameter: "+tt.wantParam)
			require.Zero(t, requests, "the request must never leave the SDK")
		})
	}
}

// silentZeroError reports the defect shape when a call that should have failed
// locally instead returned a zero-valued money object.
type silentZeroError struct {
	what   string
	amount string
}

func (e *silentZeroError) Error() string {
	return e.what + " returned no error and a zero-valued object (amount/id " + e.amount +
		"): the collection route answered and decoded into a single object"
}

// TestEmptyPathID_RejectedAcrossFacades pins the guard on one representative
// call per facade family. Every service the client exposes formats caller ids
// into a URL path, so every one of them can build a path whose empty segment
// resolves to a different route than the caller asked for. The guard is shared,
// so one row per family is enough to prove the family is wired to it.
//
// Each row names the parameter the error must report, so a row cannot pass on
// some unrelated validation failure that happens to also be a validation error.
func TestEmptyPathID_RejectedAcrossFacades(t *testing.T) {
	tests := []struct {
		name      string
		wantParam string
		call      func(context.Context, *Client) error
	}{
		// --- V1 ledger surface ---
		{"organizations", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V1.Organizations.Get(ctx, "")
			return err
		}},
		{"ledgers", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V1.Ledgers.Get(ctx, "org-1", "")
			return err
		}},
		{"accounts", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V1.Accounts.Get(ctx, "org-1", "led-1", "")
			return err
		}},
		{"account types", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V1.AccountTypes.Get(ctx, "org-1", "led-1", "")
			return err
		}},
		{"assets", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V1.Assets.Get(ctx, "org-1", "led-1", "")
			return err
		}},
		{"asset rates", "externalID", func(ctx context.Context, c *Client) error {
			_, err := c.V1.AssetRates.GetAssetRate(ctx, "org-1", "led-1", "")
			return err
		}},
		{"balances", "balanceID", func(ctx context.Context, c *Client) error {
			_, err := c.V1.Balances.GetBalance(ctx, "org-1", "led-1", "")
			return err
		}},
		{"operations", "operationID", func(ctx context.Context, c *Client) error {
			_, err := c.V1.Operations.GetOperation(ctx, "org-1", "led-1", "acc-1", "")
			return err
		}},
		{"portfolios", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V1.Portfolios.Get(ctx, "org-1", "led-1", "")
			return err
		}},
		{"segments", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V1.Segments.Get(ctx, "org-1", "led-1", "")
			return err
		}},
		{"operation routes", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V1.OperationRoutes.Get(ctx, "org-1", "led-1", "")
			return err
		}},
		{"transaction routes", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V1.TransactionRoutes.Get(ctx, "org-1", "led-1", "")
			return err
		}},
		{"transactions", "transactionID", func(ctx context.Context, c *Client) error {
			_, err := c.V1.Transactions.Get(ctx, "org-1", "led-1", "")
			return err
		}},
		{"metadata indexes", "entityName", func(ctx context.Context, c *Client) error {
			return c.V1.MetadataIndexes.Delete(ctx, "", "idx-1")
		}},

		// --- V2 ledger surface ---
		{"holders", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Holders.Get(ctx, "org-1", "")
			return err
		}},
		{"instruments", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Instruments.Get(ctx, "org-1", "holder-1", "")
			return err
		}},
		{"encryption", "orgID", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Encryption.GetProvisioningStatus(ctx, "")
			return err
		}},
		{"composition", "holderID", func(ctx context.Context, c *Client) error {
			_, err := c.V2.Composition.CreateHolderAccount(ctx, "org-1", "led-1", "",
				&models.CreateHolderAccountInput{Name: "acc", AssetCode: "USD", Type: "deposit"})
			return err
		}},
		{"protection audit", "orgID", func(ctx context.Context, c *Client) error {
			_, err := c.V2.ProtectionAudit.ListAuditEvents(ctx, "", models.AuditEventsListOpts{})
			return err
		}},
		{"billing packages", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V2.BillingPackages.Get(ctx, "org-1", "led-1", "")
			return err
		}},
		{"fee packages", "id", func(ctx context.Context, c *Client) error {
			_, err := c.V2.FeePackages.Get(ctx, "org-1", "led-1", "")
			return err
		}},
		{"fee estimates", "ledgerID", func(ctx context.Context, c *Client) error {
			_, err := c.V2.FeeEstimates.EstimateFee(ctx, "org-1", "", &models.FeeEstimateInput{})
			return err
		}},
		{"billing calculations", "ledgerID", func(ctx context.Context, c *Client) error {
			_, err := c.V2.BillingCalculations.CalculateBilling(ctx, "org-1", "",
				&models.BillingCalculateInput{})
			return err
		}},

		// --- Tracer surface ---
		{"rules", "id", func(ctx context.Context, c *Client) error {
			_, err := c.Rules.Get(ctx, "")
			return err
		}},
		{"limits", "id", func(ctx context.Context, c *Client) error {
			_, err := c.Limits.Get(ctx, "")
			return err
		}},
		// The lifecycle transitions reach the wire through their own shared
		// helper, so they need their own row.
		{"rule lifecycle transition", "id", func(ctx context.Context, c *Client) error {
			_, err := c.Rules.Activate(ctx, "")
			return err
		}},
		{"limit lifecycle transition", "id", func(ctx context.Context, c *Client) error {
			_, err := c.Limits.Activate(ctx, "")
			return err
		}},
		{"reservation transition by transaction", "transactionID", func(ctx context.Context, c *Client) error {
			_, err := c.Reservations.ConfirmByTransaction(ctx, "")
			return err
		}},
		{"validations", "id", func(ctx context.Context, c *Client) error {
			_, err := c.Validations.Get(ctx, "")
			return err
		}},
		{"reservations", "id", func(ctx context.Context, c *Client) error {
			_, err := c.Reservations.Confirm(ctx, "")
			return err
		}},
		{"audit events", "id", func(ctx context.Context, c *Client) error {
			_, err := c.AuditEvents.Get(ctx, "")
			return err
		}},
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
			require.Error(t, err, "an empty path id must fail")
			require.True(t, sdkerrors.IsValidationError(err),
				"an empty path id must be a local validation failure, got %v", err)
			require.Contains(t, err.Error(), "missing required parameter: "+tt.wantParam)
			require.Zero(t, requests, "the request must never leave the SDK")
		})
	}
}

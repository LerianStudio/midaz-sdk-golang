package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/stretchr/testify/require"
)

// TestPointInTimeAccountBalances_OneDateContract pins that the two spellings of
// "what did this account hold at this instant" agree on what a valid instant is.
//
// V1.Accounts.BalancesAtTimestamp and V1.Balances.GetAccountBalancesHistory are
// the same wire call. Only one of them used to check the date: the other omitted
// an empty value from the query entirely, so the server answered with the
// account's CURRENT balances while the caller believed they had asked for a
// moment in the past — a point-in-time read silently answering about now.
func TestPointInTimeAccountBalances_OneDateContract(t *testing.T) {
	readers := map[string]func(context.Context, *Client, string) error{
		"V1.Accounts.BalancesAtTimestamp": func(ctx context.Context, c *Client, date string) error {
			_, err := c.V1.Accounts.BalancesAtTimestamp(ctx, "org-1", "led-1", "acc-1", date)
			return err
		},
		"V1.Balances.GetAccountBalancesHistory": func(ctx context.Context, c *Client, date string) error {
			_, err := c.V1.Balances.GetAccountBalancesHistory(ctx, "org-1", "led-1", "acc-1", date)
			return err
		},
	}

	rejected := map[string]string{
		"no date at all":         "",
		"a day with no instant":  "2026-01-02",
		"a value that is a word": "yesterday",
	}

	for name, read := range readers {
		t.Run(name, func(t *testing.T) {
			for caseName, date := range rejected {
				t.Run("rejects "+caseName, func(t *testing.T) {
					var requests int

					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						requests++

						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`[]`))
					}))
					defer srv.Close()

					c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
					require.NoError(t, err)

					err = read(context.Background(), c, date)
					require.Error(t, err)
					require.True(t, sdkerrors.IsValidationError(err), "got %v", err)
					require.Zero(t, requests, "a point-in-time read with no usable instant must not reach the server")
				})
			}

			t.Run("accepts an instant and sends it", func(t *testing.T) {
				const instant = "2026-01-02 03:04:05"

				var seenDate string

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					seenDate = r.URL.Query().Get("date")

					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`[]`))
				}))
				defer srv.Close()

				c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
				require.NoError(t, err)

				require.NoError(t, read(context.Background(), c, instant))
				require.Equal(t, instant, seenDate)
			})
		})
	}
}

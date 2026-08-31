package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/stretchr/testify/require"
)

// TestDeleteSucceedsOnBodilessJSONContentType is the regression guard for a
// successful delete reported as a failure.
//
// The generated client's DELETE parser unmarshals the response body as an error
// whenever the Content-Type contains "json", regardless of status. Midaz's own
// handlers return a bodiless 204 with NO content type, so the SDK never met the
// combination in a direct deployment — but a gateway, service mesh sidecar or
// CDN in front of the ledger routinely stamps "application/json" onto a
// response it did not author. The moment one does, `json.Unmarshal([]byte{})`
// fails, the parser returns (nil, err), and EVERY delete in the SDK answers
// "unexpected end of JSON input" for a resource the server already removed.
//
// A caller who trusts that error retries the delete, or worse, treats the
// resource as still present. The fix is one shared seam (deleteResource): a 2xx
// with nothing to decode is success, whatever the content type claims.
//
// The table walks one delete per path SHAPE rather than per resource — the seam
// is shared, so the coverage that matters is that no facade kept a private copy
// of the old block. TestNoFacadeDecodesItsOwnDeleteResponse in entities/ proves
// that structurally.
func TestDeleteSucceedsOnBodilessJSONContentType(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context, *Client) error
	}{
		{
			name: "organization (root scope)",
			call: func(ctx context.Context, c *Client) error {
				return c.V1.Organizations.Delete(ctx, "org-1")
			},
		},
		{
			name: "ledger (organization scope)",
			call: func(ctx context.Context, c *Client) error {
				return c.V1.Ledgers.Delete(ctx, "org-1", "led-1")
			},
		},
		{
			name: "balance (ledger scope, money path)",
			call: func(ctx context.Context, c *Client) error {
				return c.V1.Balances.DeleteBalance(ctx, "org-1", "led-1", "bal-1")
			},
		},
		{
			name: "metadata index (unscoped)",
			call: func(ctx context.Context, c *Client) error {
				return c.V1.MetadataIndexes.Delete(ctx, "transaction", "reference")
			},
		},
		{
			name: "holder (v2-only family)",
			call: func(ctx context.Context, c *Client) error {
				return c.V2.Holders.Delete(ctx, "org-1", "hol-1")
			},
		},
		{
			name: "account on the v2 surface",
			call: func(ctx context.Context, c *Client) error {
				return c.V2.Accounts.Delete(ctx, "org-1", "led-1", "acc-1")
			},
		},
		{
			name: "balance on the v2 surface (money path)",
			call: func(ctx context.Context, c *Client) error {
				return c.V2.Balances.DeleteBalance(ctx, "org-1", "led-1", "bal-1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				// The exact shape a proxy produces: the server's bodiless 204,
				// with a content type the server never set.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
			require.NoError(t, err)

			require.NoError(t, tt.call(context.Background(), c),
				"a 204 with an empty body is a successful delete whatever the Content-Type says")
		})
	}
}

// TestDeleteStillReportsServerRefusal is the other half of the contract: making
// an empty 2xx body succeed must not make a REFUSAL succeed. A 409 with the
// ledger's RFC 9457 problem document still has to reach the caller as an error
// carrying the server's detail.
func TestDeleteStillReportsServerRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"title":"Conflict","detail":"ledger still has accounts","code":"0072"}`))
	}))
	defer srv.Close()

	c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
	require.NoError(t, err)

	err = c.V1.Ledgers.Delete(context.Background(), "org-1", "led-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "ledger still has accounts")
}

// TestDeleteReportsEmptyErrorBodyAsItsStatus covers the shape a proxy produces
// on the failure side: an error status with a JSON content type and no body at
// all. The old parser turned that into "unexpected end of JSON input" too,
// hiding the only fact worth reporting — the status the server answered with.
func TestDeleteReportsEmptyErrorBodyAsItsStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
	require.NoError(t, err)

	err = c.V1.Ledgers.Delete(context.Background(), "org-1", "led-1")
	require.Error(t, err)
	require.True(t, sdkerrors.IsNotFoundError(err),
		"an empty error body must still carry the server's status, got %v", err)
}

package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/stretchr/testify/require"
)

// TestDotSegmentPathID_ReopensSilentZero shows that rejecting only emptiness
// leaves the silent-zero read wide open.
//
// The empty-id guard compares the trimmed id against "". A caller who hands the
// SDK "." passes that check, and nothing downstream escapes it: the generated
// client formats the id straight into the path and then resolves the result
// against the base URL, which is where RFC 3986 removes the dot segment. The
// request that leaves is the same ".../balances/" the empty-id guard exists to
// prevent, so the collection route answers, the single-object decoder reads a
// paginated envelope into a zero-valued Balance, and the caller books zero with
// a nil error.
func TestDotSegmentPathID_ReopensSilentZero(t *testing.T) {
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path

		// Mirror Fiber with StrictRouting unset: ".../balances/" and
		// ".../balances" are the same route.
		path := strings.TrimSuffix(r.URL.Path, "/")

		w.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(path, "/balances") {
			_, _ = w.Write([]byte(`{"items":[{"id":"b-1","assetCode":"USD","available":"1500.5"}],"limit":10}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"title":"not found","status":404}`))
	}))
	defer srv.Close()

	c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
	require.NoError(t, err)

	balance, err := c.V1.Balances.GetBalance(context.Background(), "org-1", "led-1", ".")
	require.Error(t, err,
		"a dot-segment balance id must fail locally; it reached %s and decoded to %+v",
		gotPath, balance)
	require.True(t, sdkerrors.IsValidationError(err),
		"a dot-segment path id is a caller mistake the SDK can name locally, got %v", err)
	require.Empty(t, gotPath, "the request must never leave the SDK")
}

// TestDotSegmentPathID_EscalatesDestructiveScope shows the same bypass turning a
// delete into a delete of the PARENT resource.
//
// ".." survives the empty-id guard, and the dot-segment removal that happens
// when the operation path is resolved against the base URL pops one segment off
// the path. A caller deleting a ledger with an id of ".." issues a DELETE
// against the organization; the same shape one level up deletes the whole v1
// collection root.
func TestDotSegmentPathID_EscalatesDestructiveScope(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context, *Client) error
	}{
		{"delete a ledger with a parent-traversal id", func(ctx context.Context, c *Client) error {
			return c.V1.Ledgers.Delete(ctx, "org-1", "..")
		}},
		{"delete an organization with a parent-traversal id", func(ctx context.Context, c *Client) error {
			return c.V1.Organizations.Delete(ctx, "..")
		}},
		{"delete an account with a parent-traversal id", func(ctx context.Context, c *Client) error {
			return c.V1.Accounts.Delete(ctx, "org-1", "led-1", "..")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deleted []string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deleted = append(deleted, r.Method+" "+r.URL.Path)

				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
			require.NoError(t, err)

			err = tt.call(context.Background(), c)
			require.Error(t, err, "a dot-segment path id must fail locally; it issued %v", deleted)
			require.True(t, sdkerrors.IsValidationError(err),
				"a dot-segment path id is a caller mistake the SDK can name locally, got %v", err)
			require.Empty(t, deleted, "the request must never leave the SDK")
		})
	}
}

// TestUnsafePathID_RejectedShapes enumerates the id shapes that must never
// become a URL, and names the one shape that is allowed through so the boundary
// stays pinned from both sides.
func TestUnsafePathID_RejectedShapes(t *testing.T) {
	rejected := []struct {
		name string
		id   string
	}{
		{"a current-directory dot segment", "."},
		{"a parent-directory dot segment", ".."},
		{"a bare separator", "/"},
		{"a relative traversal", "./."},
		{"an embedded separator", "bal-1/../../org-1"},
		{"a backslash separator", `bal-1\..\..`},
		{"a padded dot segment", "  ..  "},
	}

	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			var requests []string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.Path)

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"items":[],"limit":10}`))
			}))
			defer srv.Close()

			c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
			require.NoError(t, err)

			_, err = c.V1.Balances.GetBalance(context.Background(), "org-1", "led-1", tt.id)
			require.Error(t, err, "id %q must be refused locally; it issued %v", tt.id, requests)
			require.True(t, sdkerrors.IsValidationError(err),
				"an unsafe path id must classify as a validation failure, got %v", err)
			require.Contains(t, err.Error(), "path id must not be a dot segment or contain a path separator")
			require.Empty(t, requests, "the request must never leave the SDK")
		})
	}

	// A percent-encoded dot is NOT a dot segment, and the guard deliberately
	// lets it through: it rejects shapes, not strings that merely look like one.
	//
	// It cannot become a dot on the way out. The id is escaped once on its way
	// into the path, so the "%" is itself encoded and "%2e" leaves as "%252e";
	// RFC 3986 dot-segment removal runs on that encoded path and finds nothing
	// to remove. One server-side decode yields the literal text "%2e", not ".".
	// Reaching a dot segment from here needs a server that decodes the path
	// TWICE, which would be a defect on that side of the wire — pinning the
	// exact bytes here makes any future change to the escaping visible as a diff
	// on this line rather than as a silent traversal.
	t.Run("a percent-encoded dot reaches the wire uncollapsed", func(t *testing.T) {
		var gotPath string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.EscapedPath()

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(
				`{"id":"11111111-1111-1111-1111-111111111111","assetCode":"USD","available":"1"}`))
		}))
		defer srv.Close()

		c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
		require.NoError(t, err)

		_, err = c.V1.Balances.GetBalance(context.Background(), "org-1", "led-1", "%2e")
		require.NoError(t, err)
		require.Equal(t, "/v1/organizations/org-1/ledgers/led-1/balances/%252e", gotPath,
			"a percent-encoded dot must stay encoded and stay its own segment")
	})
}

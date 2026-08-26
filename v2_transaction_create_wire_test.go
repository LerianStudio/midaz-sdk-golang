package midaz

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/stretchr/testify/require"
)

// TestV2TransactionCreateRouting pins the four /v2 transaction create actions:
// the HTTP method, the top-level path, the organization and ledger the request
// body carries on every leg, and the idempotency key.
//
// It exists as its own table because these four are the only endpoints on the
// whole surface where the SCOPE does not travel in the URL. Every other call in
// the SDK addresses a ledger by path segment; these address it by naming it on
// each debit and credit leg, and the server refuses a body whose legs disagree.
// So "did the request go to the right ledger" is a question about the BODY here,
// and a routing table that only looked at the path would answer it wrong.
//
// What it pins, precisely:
//
//   - method and path, one row per action, since the action IS the endpoint;
//   - the scope the caller addressed reaching every leg on both sides;
//   - the amount arriving as the exact decimal string it was given;
//   - an X-Idempotency header on all four, because all four are creates and a
//     network retry without one posts a second balance mutation.
//
// What it does NOT pin: response decoding, the /v2 model divergence, and the
// leg-scope refusal path. Those are behavioural, and they live in
// entities/transactions_v2_facade_test.go.
func TestV2TransactionCreateRouting(t *testing.T) {
	const (
		orgID    = "11111111-1111-1111-1111-111111111111"
		ledgerID = "22222222-2222-2222-2222-222222222222"
	)

	tests := []struct {
		name     string
		wantPath string
		call     func(context.Context, *Client, *models.CreateTransactionV2Input) error
	}{
		{
			name: "direct", wantPath: "/v2/transactions/direct",
			call: func(ctx context.Context, c *Client, in *models.CreateTransactionV2Input) error {
				_, err := c.V2.Transactions.CreateDirect(ctx, orgID, ledgerID, in)
				return err
			},
		},
		{
			name: "hold", wantPath: "/v2/transactions/hold",
			call: func(ctx context.Context, c *Client, in *models.CreateTransactionV2Input) error {
				_, err := c.V2.Transactions.CreateHold(ctx, orgID, ledgerID, in)
				return err
			},
		},
		{
			name: "block", wantPath: "/v2/transactions/block",
			call: func(ctx context.Context, c *Client, in *models.CreateTransactionV2Input) error {
				_, err := c.V2.Transactions.CreateBlock(ctx, orgID, ledgerID, in)
				return err
			},
		},
		{
			name: "unblock", wantPath: "/v2/transactions/unblock",
			call: func(ctx context.Context, c *Client, in *models.CreateTransactionV2Input) error {
				_, err := c.V2.Transactions.CreateUnblock(ctx, orgID, ledgerID, in)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				gotMethod, gotPath, gotIdempotency string
				gotBody                            []byte
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				gotIdempotency = r.Header.Get("X-Idempotency")
				gotBody, _ = io.ReadAll(r.Body)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":"33333333-3333-3333-3333-333333333333","assetCode":"USD","amount":"100.25","status":{"code":"APPROVED"}}`))
			}))
			defer srv.Close()

			c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
			require.NoError(t, err)

			// The legs deliberately leave the scope EMPTY: filling it from the
			// addressed pair is the facade's job, and this is what proves it
			// happens on the wire rather than only in a unit test.
			require.NoError(t, tt.call(context.Background(), c, &models.CreateTransactionV2Input{
				Asset:   "USD",
				Amount:  "100.25",
				Debits:  []models.TransactionV2Leg{{Alias: "@src", Amount: "100.25"}},
				Credits: []models.TransactionV2Leg{{Alias: "@dst", Amount: "100.25"}},
			}))

			require.Equal(t, http.MethodPost, gotMethod)
			require.Equal(t, tt.wantPath, gotPath,
				"the v2 creates are top-level; no organization or ledger belongs in this path")
			require.NotEmpty(t, gotIdempotency,
				"a create without an idempotency key lets a network retry post a second balance mutation")

			var wire struct {
				Asset   string `json:"asset"`
				Amount  string `json:"amount"`
				Debits  []map[string]any
				Credits []map[string]any
			}

			require.NoError(t, json.Unmarshal(gotBody, &wire), "body: %s", gotBody)
			require.Equal(t, "USD", wire.Asset)
			require.Equal(t, "100.25", wire.Amount, "the amount must reach the wire as the exact decimal it was given")

			require.Len(t, wire.Debits, 1)
			require.Len(t, wire.Credits, 1)

			for side, legs := range map[string][]map[string]any{"debits": wire.Debits, "credits": wire.Credits} {
				require.Equal(t, orgID, legs[0]["organizationId"],
					"%s leg must carry the addressed organization: /v2 resolves the scope from the body", side)
				require.Equal(t, ledgerID, legs[0]["ledgerId"],
					"%s leg must carry the addressed ledger", side)
				require.Equal(t, "100.25", legs[0]["amount"], "%s leg amount must survive as an exact decimal", side)
			}
		})
	}
}

// TestV2TransactionCreateDoesNotMutateCallerInput is a money-path guard on the
// scope stamping.
//
// The facade fills each leg's organization and ledger from the pair the caller
// addressed. Debits and Credits are SLICES, so stamping in place would write
// through to the caller's own value — and a caller who builds one input and
// posts it to two ledgers would send the second transaction against the first
// ledger, with nothing in either response to say so.
func TestV2TransactionCreateDoesNotMutateCallerInput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"33333333-3333-3333-3333-333333333333","status":{"code":"APPROVED"}}`))
	}))
	defer srv.Close()

	c, err := New(WithConfig(createTestConfig(t)), WithBaseURL(srv.URL))
	require.NoError(t, err)

	input := &models.CreateTransactionV2Input{
		Asset:   "USD",
		Amount:  "10",
		Debits:  []models.TransactionV2Leg{{Alias: "@src", Amount: "10"}},
		Credits: []models.TransactionV2Leg{{Alias: "@dst", Amount: "10"}},
	}

	_, err = c.V2.Transactions.CreateDirect(context.Background(),
		"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", input)
	require.NoError(t, err)

	require.Empty(t, input.Debits[0].OrganizationID,
		"the caller's input must come back untouched, or reusing it addresses the wrong ledger next time")
	require.Empty(t, input.Debits[0].LedgerID)
	require.Empty(t, input.Credits[0].OrganizationID)
	require.Empty(t, input.Credits[0].LedgerID)
}

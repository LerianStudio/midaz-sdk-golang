// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
)

// TestTransactionsFacade_BlockAndUnblock covers the two /v1 transaction actions
// that were generated but had no facade.
//
// Both are money writes. What is worth pinning is that they are creates, not
// state transitions: they take the same body as CreateJSON, settle immediately
// (the ledger forces them non-pending), and must therefore leave with an
// idempotency key like every other create. An action-shaped implementation —
// bodiless, no auto key — would let a network retry post a second block.
func TestTransactionsFacade_BlockAndUnblock(t *testing.T) {
	tests := []struct {
		name   string
		suffix string
		call   func(*transactionsFacade, context.Context) (*models.Transaction, error)
	}{
		{
			name:   "block",
			suffix: "/block",
			call: func(f *transactionsFacade, ctx context.Context) (*models.Transaction, error) {
				return f.CreateBlock(ctx, txOrgID, txLedgerID, sampleTransactionInput())
			},
		},
		{
			name:   "unblock",
			suffix: "/unblock",
			call: func(f *transactionsFacade, ctx context.Context) (*models.Transaction, error) {
				return f.CreateUnblock(ctx, txOrgID, txLedgerID, sampleTransactionInput())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				gotMethod, gotPath, gotIdem string
				gotBody                     []byte
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				gotIdem = r.Header.Get("X-Idempotency")
				gotBody, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(txResponseBody()))
			}))
			defer srv.Close()

			tx, err := tt.call(newTestTransactionsFacade(t, srv), context.Background())
			if err != nil {
				t.Fatalf("create %s: %v", tt.name, err)
			}

			if gotMethod != http.MethodPost || gotPath != txBase()+tt.suffix {
				t.Fatalf("req = %s %s, want POST %s", gotMethod, gotPath, txBase()+tt.suffix)
			}

			if gotIdem == "" {
				t.Fatal("X-Idempotency missing: a block/unblock is a create, so a retry must not post twice")
			}

			assertSendEnvelope(t, gotBody)

			if tx.ID != txID {
				t.Fatalf("tx.ID = %q, want %q", tx.ID, txID)
			}
		})
	}
}

// assertSendEnvelope checks that a create body is the mapper output, same as
// CreateJSON: the action label is applied server-side, never carried in the
// payload, so block and unblock must send exactly what a JSON create sends.
func assertSendEnvelope(t *testing.T, body []byte) {
	t.Helper()

	var wire map[string]any
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("body not a JSON object: %v (%s)", err, body)
	}

	send, ok := wire["send"].(map[string]any)
	if !ok {
		t.Fatalf("send envelope missing: %s", body)
	}

	if send["value"] != "100" {
		t.Fatalf("send.value = %v, want %q: %s", send["value"], "100", body)
	}
}

// TestTransactionsFacade_BlockRejectsBadInputLocally pins the classification of
// a locally refused block payload: a caller must be able to tell "your payload
// is wrong" from "the ledger refused it", because only the second is worth
// retrying and this request never left.
func TestTransactionsFacade_BlockRejectsBadInputLocally(t *testing.T) {
	var reached bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// No send envelope: the input's own Validate refuses it.
	_, err := newTestTransactionsFacade(t, srv).CreateBlock(context.Background(), txOrgID, txLedgerID,
		models.NewCreateTransactionInput("USD", "100"))
	if err == nil {
		t.Fatal("expected a local refusal for a transaction with no send envelope")
	}

	if !sdkerrors.IsValidationError(err) {
		t.Fatalf("err = %v, want a validation error", err)
	}

	if reached {
		t.Fatal("a locally refused payload must not reach the wire")
	}
}

// TestAccountsFacade_GetByExternalCode covers the /v1 external-account lookup,
// generated but previously unexposed.
//
// The external account is the counterparty of every deposit and withdrawal, so
// reaching it matters: without this route a caller cannot resolve the account an
// inflow draws from. The code is the bare asset code and travels as a path
// segment, NOT as the "@external/USD" alias — Midaz prohibits that prefix on a
// client-supplied alias, so the alias route cannot reach this account.
func TestAccountsFacade_GetByExternalCode(t *testing.T) {
	const accountID = "66666666-6666-6666-6666-666666666666"

	var gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + accountID + `","alias":"@external/USD","assetCode":"USD"}`))
	}))
	defer srv.Close()

	facade := newAccountsFacade(newTestLedgerClient(t, srv), true)

	account, err := facade.GetByExternalCode(context.Background(), txOrgID, txLedgerID, "USD")
	if err != nil {
		t.Fatalf("GetByExternalCode: %v", err)
	}

	wantPath := "/v1/organizations/" + txOrgID + "/ledgers/" + txLedgerID + "/accounts/external/USD"
	if gotMethod != http.MethodGet || gotPath != wantPath {
		t.Fatalf("req = %s %s, want GET %s", gotMethod, gotPath, wantPath)
	}

	if account.ID != accountID {
		t.Fatalf("account.ID = %q, want %q", account.ID, accountID)
	}
}

// TestAccountsFacade_GetByExternalCodeGuardsPathIDs pins that the new read is
// covered by the shared path-id guard: an empty asset code would otherwise build
// ".../accounts/external/", which Fiber trims and routes to a different handler.
func TestAccountsFacade_GetByExternalCodeGuardsPathIDs(t *testing.T) {
	var reached bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := newAccountsFacade(newTestLedgerClient(t, srv), true).
		GetByExternalCode(context.Background(), txOrgID, txLedgerID, "  ")
	if err == nil {
		t.Fatal("expected a refusal for a blank asset code")
	}

	if reached {
		t.Fatal("a blank path id must not reach the wire")
	}
}

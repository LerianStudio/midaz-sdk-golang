// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPageListOpts_OverLimit_ValidatesBeforeRequest covers H29: the surviving
// interface-backed page-list methods (Aliases, Balances) must short-circuit on
// Validate() before issuing any HTTP request when opts.Limit > MaxLimit. This
// is the entity-side regression pinning the contract that backs
// ValidatePageListOpts and ValidateCursorListOpts (already covered at the model
// level).
//
// The contract under test:
//   - opts.Limit = MaxLimit + 1 → entity returns "limit exceeds maximum"
//   - The httptest server's request counter stays at 0 (no wire traffic)
//
// The facade-backed resources (accounts, assets, ledgers, portfolios, ...)
// enforce the same pre-flight in their own *_facade_test.go. Cursor entities
// get the same treatment in TestCursorListOpts_OverLimit_ValidatesBeforeRequest
// below.
func TestPageListOpts_OverLimit_ValidatesBeforeRequest(t *testing.T) {
	tests := []struct {
		name string
		// run drives the entity's ListXxx method against the test server.
		// Returns the entity-layer error.
		run func(t *testing.T, baseURL string) error
	}{
		{
			name: "ListAliases",
			run: func(_ *testing.T, baseURL string) error {
				e := newAliasesEntity(http.DefaultClient, map[string]string{"crm": baseURL})
				_, err := e.ListAliases(context.Background(), "org",
					models.AliasesListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
		{
			name: "ListBalances",
			run: func(_ *testing.T, baseURL string) error {
				e := newBalancesEntity(http.DefaultClient, "tok", map[string]string{"transaction": baseURL})
				_, err := e.ListBalances(context.Background(), "org", "ledger",
					models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			err := tt.run(t, server.URL)
			require.Error(t, err, "Limit > MaxLimit must be rejected before sending request")
			assert.Contains(t, err.Error(), "limit exceeds maximum")
			assert.Equal(t, int32(0), hits.Load(), "validation failure must short-circuit before any HTTP request")
		})
	}
}

// TestCursorListOpts_OverLimit_ValidatesBeforeRequest mirrors
// TestPageListOpts_OverLimit_ValidatesBeforeRequest for the surviving
// interface-backed cursor entity (Operations). The facade-backed cursor
// resources (transactions, asset rates, operation/transaction routes) enforce
// the same opts.Validate() pre-flight in their own *_facade_test.go.
//
// This pins the pre-flight short-circuit: opts.Validate() runs before any HTTP
// request, so an over-limit request never reaches the wire.
func TestCursorListOpts_OverLimit_ValidatesBeforeRequest(t *testing.T) {
	tests := []struct {
		name string
		run  func(_ *testing.T, baseURL string) error
	}{
		{
			name: "ListOperations",
			run: func(_ *testing.T, baseURL string) error {
				e := newOperationsEntity(http.DefaultClient, "tok", map[string]string{"transaction": baseURL})
				_, err := e.ListOperations(context.Background(), "org", "ledger", "acc",
					models.OperationsListOpts{CursorListOpts: models.CursorListOpts{Limit: models.MaxLimit + 1}})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			err := tt.run(t, server.URL)
			require.Error(t, err, "Limit > MaxLimit must be rejected before sending request")
			assert.Contains(t, err.Error(), "limit exceeds maximum")
			assert.Equal(t, int32(0), hits.Load(), "validation failure must short-circuit before any HTTP request")
		})
	}
}

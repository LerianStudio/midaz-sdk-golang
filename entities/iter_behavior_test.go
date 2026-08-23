// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Page-based Pages tests (H25).
//
// portfolios already has TestPortfoliosEntity_ListPortfoliosPages_NilContext;
// every other page-based entity goes here. The pattern is constant: serve a
// 2-page response from an httptest server, drive ListXxxPages, assert two
// pages emerged with HasMore() flipping false on page 2.
// ─────────────────────────────────────────────────────────────────────────────

// pageBasedHandler builds an http.Handler that serves a 2-page response of T.
// The first request is page 1 (Total set so HasMore returns true via
// Total>Page*Limit). The second request is page 2 (Total*Limit drained).
// Counts how many requests landed.
//
// Pagination heuristic: server reports Total=3, Limit=2. Page 1 has 2 items,
// Page 2 has 1 item. HasMore() math: page=1 → 1*2 < 3 → true. page=2 →
// 2*2 < 3 → false. Iterator advances correctly.
func pageBasedHandler[T any](t *testing.T, page1 []T, page2 []T) (http.Handler, *atomic.Int32) {
	t.Helper()

	var calls atomic.Int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		page := r.URL.Query().Get("page")

		var (
			items     []T
			pageNum   int
			totalSeen = len(page1) + len(page2)
		)

		switch n {
		case 1:
			assert.Equal(t, "1", page, "first request must be page=1")
			items = page1
			pageNum = 1
		case 2:
			assert.Equal(t, "2", page, "second request must advance to page=2")
			items = page2
			pageNum = 2
		default:
			t.Errorf("unexpected request %d (page=%s) — iterator should have stopped", n, page)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(models.ListResponse[T]{
			Items: items,
			Pagination: models.Pagination{
				Total: totalSeen,
				Limit: 2,
				Page:  pageNum,
			},
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	return handler, &calls
}

func TestAliasesEntity_ListAliasesPages_AdvancesAndStops(t *testing.T) {
	// Aliases is org-scoped (not ledger-scoped) and lives on the CRM service.
	// Use zero-value Alias entries — we only test iteration shape, not field
	// round-trips (the Alias model is large and exhaustive coverage isn't the
	// concern here).
	handler, calls := pageBasedHandler(t,
		[]models.Alias{{}, {}},
		[]models.Alias{{}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newAliasesEntity(server.Client(), map[string]string{"crm": server.URL})
	opts := models.AliasesListOpts{PageListOpts: models.PageListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.Alias]

	for page, err := range entity.ListAliasesPages(context.Background(), "org", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.False(t, pages[1].Pagination.HasMore())
	assert.Equal(t, int32(2), calls.Load())
	// Page 1 yielded 2 items, page 2 yielded 1.
	assert.Len(t, pages[0].Items, 2)
	assert.Len(t, pages[1].Items, 1)
}

func TestBalancesEntity_ListBalancesPages_AdvancesAndStops(t *testing.T) {
	handler, calls := pageBasedHandler(t,
		[]models.Balance{{ID: "b-1"}, {ID: "b-2"}},
		[]models.Balance{{ID: "b-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newBalancesEntity(server.Client(), "tok", map[string]string{"transaction": server.URL})
	opts := models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.Balance]

	for page, err := range entity.ListBalancesPages(context.Background(), "org", "ledger", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.False(t, pages[1].Pagination.HasMore())
	assert.Equal(t, int32(2), calls.Load())
}

// ─────────────────────────────────────────────────────────────────────────────
// Cursor-based advance tests (H24).
//
// 4 entities use cursor pagination (asset_rates is also cursor but is already
// covered by the existing asset_rates_test.go suite). The contract under test:
// page 1 yields next_cursor=X, the iterator MUST issue page 2 with cursor=X
// in the query string, and MUST stop when page 2 returns next_cursor="".
// ─────────────────────────────────────────────────────────────────────────────

// cursorBasedHandler builds an http.Handler simulating a 2-page cursor-paginated
// response. Page 1 yields next_cursor="c-2"; page 2 yields next_cursor="" (end).
// The handler asserts that the second request carries cursor=c-2 in the query
// string — proving the iterator USED the cursor (not silently dropped it).
func cursorBasedHandler[T any](t *testing.T, page1 []T, page2 []T) (http.Handler, *atomic.Int32, *string) {
	t.Helper()

	var (
		calls           atomic.Int32
		secondReqCursor string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		cursor := r.URL.Query().Get("cursor")

		var (
			items []T
			next  string
		)

		switch n {
		case 1:
			assert.Empty(t, cursor, "first request must NOT carry a cursor (initial page)")
			items = page1
			next = "c-2"
		case 2:
			secondReqCursor = cursor
			assert.Equal(t, "c-2", cursor, "second request must carry cursor=c-2 issued by page 1")
			items = page2
			next = "" // signal end-of-stream
		default:
			t.Errorf("unexpected request %d (cursor=%q) — iterator should have stopped", n, cursor)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(models.ListResponse[T]{
			Items: items,
			Pagination: models.Pagination{
				NextCursor: next,
			},
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	return handler, &calls, &secondReqCursor
}

func TestOperationsEntity_ListOperationsPages_CursorAdvances(t *testing.T) {
	handler, calls, secondCursor := cursorBasedHandler(t,
		[]models.Operation{{ID: "op-1"}, {ID: "op-2"}},
		[]models.Operation{{ID: "op-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newOperationsEntity(server.Client(), "tok", map[string]string{"transaction": server.URL})
	opts := models.OperationsListOpts{CursorListOpts: models.CursorListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.Operation]

	for page, err := range entity.ListOperationsPages(context.Background(), "org", "ledger", "acc", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.Equal(t, "c-2", pages[0].Pagination.NextCursor)
	assert.Empty(t, pages[1].Pagination.NextCursor)
	assert.Equal(t, int32(2), calls.Load())
	assert.Equal(t, "c-2", *secondCursor)
}

// runListAllSubtest is the shared body for every TestListXxxAll_DelegatesToPages
// subtest. It serves the given single-page payload from an httptest server,
// hands the server URL to buildSeq so the caller can wire the per-entity
// constructor + Iterator method, drains the resulting Seq2, and forwards
// the collected items to check for per-entity assertions. The check closure
// captures the subtest's *testing.T from outer scope to keep the call site
// terse.
//
// Extracted to keep TestListXxxAll_DelegatesToPages under revive's
// cognitive-complexity budget — the mechanically-uniform subtests against the
// surviving ListXxxAll entry points add up at the function level even though
// each subtest is trivial in isolation.
func runListAllSubtest[T any](
	t *testing.T,
	payload models.ListResponse[T],
	buildSeq func(server *httptest.Server) iter.Seq2[T, error],
	check func(got []T),
) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(payload); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	var got []T
	for item, err := range buildSeq(server) {
		require.NoError(t, err)
		got = append(got, item)
	}

	check(got)
}

// TestListXxxAll_DelegatesToPages covers H30: the All-variant iterators for the
// surviving trio (Aliases, Balances, Operations) are 1-line wrappers over
// `flattenPages(Pages(...))`. Without a single drive-through, every All variant
// sits at 0% coverage despite being ALL the work flattenPages was extracted to
// share. This parametrized test exercises each surviving All iterator over a
// single-page mock response to confirm wiring. The facade-backed resources'
// All variants are covered in their own *_facade_test.go.
//
// The Operations All variant is cursor-based, so the same flattenPages wiring
// is confirmed on a cursor-paginated upstream too.
//
//nolint:revive // cognitive-complexity: the surviving ListXxxAll wrappers are each drive-tested through a t.Run; the runListAllSubtest helper collapses every subtest body to a single call, so the residual complexity is the unavoidable count of entities.
func TestListXxxAll_DelegatesToPages(t *testing.T) {

	t.Run("AliasesAll", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Alias]{Items: []models.Alias{{}}},
			func(s *httptest.Server) iter.Seq2[models.Alias, error] {
				e := newAliasesEntity(s.Client(), map[string]string{"crm": s.URL})
				return e.ListAliasesAll(context.Background(), "org", models.AliasesListOpts{})
			},
			func(got []models.Alias) { assert.Len(t, got, 1) },
		)
	})

	t.Run("BalancesAll", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Balance]{Items: []models.Balance{{ID: "b-1"}}},
			func(s *httptest.Server) iter.Seq2[models.Balance, error] {
				e := newBalancesEntity(s.Client(), "tok", map[string]string{"transaction": s.URL})
				return e.ListBalancesAll(context.Background(), "org", "ledger", models.BalancesListOpts{})
			},
			func(got []models.Balance) { assert.Len(t, got, 1) },
		)
	})

	t.Run("AccountBalancesAll", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Balance]{Items: []models.Balance{{ID: "b-2"}}},
			func(s *httptest.Server) iter.Seq2[models.Balance, error] {
				e := newBalancesEntity(s.Client(), "tok", map[string]string{"transaction": s.URL})
				return e.ListAccountBalancesAll(context.Background(), "org", "ledger", "acc", models.BalancesListOpts{})
			},
			func(got []models.Balance) { assert.Len(t, got, 1) },
		)
	})

	t.Run("BalancesByAccountAliasAll", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Balance]{Items: []models.Balance{{ID: "b-3"}}},
			func(s *httptest.Server) iter.Seq2[models.Balance, error] {
				e := newBalancesEntity(s.Client(), "tok", map[string]string{"transaction": s.URL})
				return e.ListBalancesByAccountAliasAll(context.Background(), "org", "ledger", "@cash", models.BalancesListOpts{})
			},
			func(got []models.Balance) { assert.Len(t, got, 1) },
		)
	})

	t.Run("BalancesByExternalCodeAll", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Balance]{Items: []models.Balance{{ID: "b-4"}}},
			func(s *httptest.Server) iter.Seq2[models.Balance, error] {
				e := newBalancesEntity(s.Client(), "tok", map[string]string{"transaction": s.URL})
				return e.ListBalancesByExternalCodeAll(context.Background(), "org", "ledger", "X-CODE", models.BalancesListOpts{})
			},
			func(got []models.Balance) { assert.Len(t, got, 1) },
		)
	})

	t.Run("OperationsAll (cursor)", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Operation]{Items: []models.Operation{{ID: "op-1"}}},
			func(s *httptest.Server) iter.Seq2[models.Operation, error] {
				e := newOperationsEntity(s.Client(), "tok", map[string]string{"transaction": s.URL})
				return e.ListOperationsAll(context.Background(), "org", "ledger", "acc", models.OperationsListOpts{})
			},
			func(got []models.Operation) { assert.Len(t, got, 1) },
		)
	})

}

// TestBalancesByAccountAliasPages_AdvancesAndStops covers the
// alias-scoped Balances Pages iterator (one of two H30 dead-coverage
// branches in balances.go).
func TestBalancesEntity_ListBalancesByAccountAliasPages_AdvancesAndStops(t *testing.T) {
	handler, calls := pageBasedHandler(t,
		[]models.Balance{{ID: "b-1"}, {ID: "b-2"}},
		[]models.Balance{{ID: "b-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newBalancesEntity(server.Client(), "tok", map[string]string{"transaction": server.URL})
	opts := models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.Balance]
	for page, err := range entity.ListBalancesByAccountAliasPages(context.Background(), "org", "ledger", "@cash", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.Equal(t, int32(2), calls.Load())
}

// TestBalancesByExternalCodePages_AdvancesAndStops covers the
// external-code-scoped Balances Pages iterator.
func TestBalancesEntity_ListBalancesByExternalCodePages_AdvancesAndStops(t *testing.T) {
	handler, calls := pageBasedHandler(t,
		[]models.Balance{{ID: "b-1"}, {ID: "b-2"}},
		[]models.Balance{{ID: "b-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newBalancesEntity(server.Client(), "tok", map[string]string{"transaction": server.URL})
	opts := models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.Balance]
	for page, err := range entity.ListBalancesByExternalCodePages(context.Background(), "org", "ledger", "EXT-1", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.Equal(t, int32(2), calls.Load())
}

// TestAccountBalancesPages_AdvancesAndStops covers the account-scoped
// Balances Pages iterator.
func TestBalancesEntity_ListAccountBalancesPages_AdvancesAndStops(t *testing.T) {
	handler, calls := pageBasedHandler(t,
		[]models.Balance{{ID: "b-1"}, {ID: "b-2"}},
		[]models.Balance{{ID: "b-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newBalancesEntity(server.Client(), "tok", map[string]string{"transaction": server.URL})
	opts := models.BalancesListOpts{PageListOpts: models.PageListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.Balance]
	for page, err := range entity.ListAccountBalancesPages(context.Background(), "org", "ledger", "acc", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.Equal(t, int32(2), calls.Load())
}

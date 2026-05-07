// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Apache-2.0

package entities

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
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

func TestAccountsEntity_ListAccountsPages_AdvancesAndStops(t *testing.T) {
	handler, calls := pageBasedHandler(t,
		[]models.Account{{ID: "a-1"}, {ID: "a-2"}},
		[]models.Account{{ID: "a-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newAccountsEntity(server.Client(), "tok", map[string]string{"onboarding": server.URL})
	opts := models.AccountsListOpts{PageListOpts: models.PageListOpts{Limit: 2}}

	var (
		pages    []*models.ListResponse[models.Account]
		hasMore1 bool
	)

	for page, err := range entity.ListAccountsPages(context.Background(), "org", "ledger", opts) {
		require.NoError(t, err)
		pages = append(pages, page)

		if len(pages) == 1 {
			hasMore1 = page.Pagination.HasMore()
		}
	}

	require.Len(t, pages, 2, "iterator must yield exactly two pages")
	assert.True(t, hasMore1, "page 1 must report HasMore() = true")
	assert.False(t, pages[1].Pagination.HasMore(), "page 2 must report HasMore() = false")
	assert.Equal(t, int32(2), calls.Load(), "exactly two HTTP requests")
	assert.Equal(t, "a-1", pages[0].Items[0].ID)
	assert.Equal(t, "a-3", pages[1].Items[0].ID)
}

func TestAccountTypesEntity_ListAccountTypesPages_AdvancesAndStops(t *testing.T) {
	handler, calls := pageBasedHandler(t,
		[]models.AccountType{{Name: "type-1"}, {Name: "type-2"}},
		[]models.AccountType{{Name: "type-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newAccountTypesEntity(server.Client(), "tok", map[string]string{"onboarding": server.URL})
	opts := models.AccountTypesListOpts{PageListOpts: models.PageListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.AccountType]

	for page, err := range entity.ListAccountTypesPages(context.Background(), "org", "ledger", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.False(t, pages[1].Pagination.HasMore())
	assert.Equal(t, int32(2), calls.Load())
	assert.Equal(t, "type-1", pages[0].Items[0].Name)
	assert.Equal(t, "type-3", pages[1].Items[0].Name)
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

func TestAssetsEntity_ListAssetsPages_AdvancesAndStops(t *testing.T) {
	handler, calls := pageBasedHandler(t,
		[]models.Asset{{ID: "as-1", Code: "USD"}, {ID: "as-2", Code: "EUR"}},
		[]models.Asset{{ID: "as-3", Code: "BRL"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newAssetsEntity(server.Client(), "tok", map[string]string{"onboarding": server.URL})
	opts := models.AssetsListOpts{PageListOpts: models.PageListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.Asset]

	for page, err := range entity.ListAssetsPages(context.Background(), "org", "ledger", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.False(t, pages[1].Pagination.HasMore())
	assert.Equal(t, int32(2), calls.Load())
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

func TestHoldersEntity_ListHoldersPages_AdvancesAndStops(t *testing.T) {
	handler, calls := pageBasedHandler(t,
		[]models.Holder{{}, {}},
		[]models.Holder{{}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newHoldersEntity(server.Client(), "token", map[string]string{"crm": server.URL})
	opts := models.HoldersListOpts{PageListOpts: models.PageListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.Holder]

	for page, err := range entity.ListHoldersPages(context.Background(), "org", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.False(t, pages[1].Pagination.HasMore())
	assert.Equal(t, int32(2), calls.Load())
	assert.Len(t, pages[0].Items, 2)
	assert.Len(t, pages[1].Items, 1)
}

func TestLedgersEntity_ListLedgersPages_AdvancesAndStops(t *testing.T) {
	handler, calls := pageBasedHandler(t,
		[]models.Ledger{{ID: "l-1"}, {ID: "l-2"}},
		[]models.Ledger{{ID: "l-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newLedgersEntity(server.Client(), "token", map[string]string{"onboarding": server.URL})
	opts := models.LedgersListOpts{PageListOpts: models.PageListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.Ledger]

	for page, err := range entity.ListLedgersPages(context.Background(), "org", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.False(t, pages[1].Pagination.HasMore())
	assert.Equal(t, int32(2), calls.Load())
}

func TestOrganizationsEntity_ListOrganizationsPages_AdvancesAndStops(t *testing.T) {
	handler, calls := pageBasedHandler(t,
		[]models.Organization{{ID: "o-1"}, {ID: "o-2"}},
		[]models.Organization{{ID: "o-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newOrganizationsEntity(server.Client(), "tok", map[string]string{"onboarding": server.URL})
	opts := models.OrganizationsListOpts{PageListOpts: models.PageListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.Organization]

	for page, err := range entity.ListOrganizationsPages(context.Background(), opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.False(t, pages[1].Pagination.HasMore())
	assert.Equal(t, int32(2), calls.Load())
}

func TestSegmentsEntity_ListSegmentsPages_AdvancesAndStops(t *testing.T) {
	handler, calls := pageBasedHandler(t,
		[]models.Segment{{ID: "s-1"}, {ID: "s-2"}},
		[]models.Segment{{ID: "s-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newSegmentsEntity(server.Client(), "tok", map[string]string{"onboarding": server.URL})
	opts := models.SegmentsListOpts{PageListOpts: models.PageListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.Segment]

	for page, err := range entity.ListSegmentsPages(context.Background(), "org", "ledger", opts) {
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

func TestTransactionsEntity_ListTransactionsPages_CursorAdvances(t *testing.T) {
	handler, calls, secondCursor := cursorBasedHandler(t,
		[]models.Transaction{{ID: "tx-1"}, {ID: "tx-2"}},
		[]models.Transaction{{ID: "tx-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newTransactionsEntity(server.Client(), map[string]string{"transaction": server.URL})
	opts := models.TransactionsListOpts{CursorListOpts: models.CursorListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.Transaction]

	for page, err := range entity.ListTransactionsPages(context.Background(), "org", "ledger", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2, "iterator must drive both pages then stop")
	assert.Equal(t, "c-2", pages[0].Pagination.NextCursor, "page 1 advertises cursor c-2")
	assert.Empty(t, pages[1].Pagination.NextCursor, "page 2 advertises empty cursor (end)")
	assert.Equal(t, int32(2), calls.Load(), "exactly two HTTP requests")
	assert.Equal(t, "c-2", *secondCursor, "second request body must have actually carried cursor=c-2")
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

func TestOperationRoutesEntity_ListOperationRoutesPages_CursorAdvances(t *testing.T) {
	handler, calls, secondCursor := cursorBasedHandler(t,
		[]models.OperationRoute{{Title: "or-1"}, {Title: "or-2"}},
		[]models.OperationRoute{{Title: "or-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newOperationRoutesEntity(server.Client(), "tok", map[string]string{"transaction": server.URL})
	opts := models.OperationRoutesListOpts{CursorListOpts: models.CursorListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.OperationRoute]

	for page, err := range entity.ListOperationRoutesPages(context.Background(), "org", "ledger", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.Equal(t, "c-2", pages[0].Pagination.NextCursor)
	assert.Empty(t, pages[1].Pagination.NextCursor)
	assert.Equal(t, int32(2), calls.Load())
	assert.Equal(t, "c-2", *secondCursor)
}

func TestTransactionRoutesEntity_ListTransactionRoutesPages_CursorAdvances(t *testing.T) {
	handler, calls, secondCursor := cursorBasedHandler(t,
		[]models.TransactionRoute{{Title: "tr-1"}, {Title: "tr-2"}},
		[]models.TransactionRoute{{Title: "tr-3"}},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	entity := newTransactionRoutesEntity(server.Client(), "tok", map[string]string{"transaction": server.URL})
	opts := models.TransactionRoutesListOpts{CursorListOpts: models.CursorListOpts{Limit: 2}}

	var pages []*models.ListResponse[models.TransactionRoute]

	for page, err := range entity.ListTransactionRoutesPages(context.Background(), "org", "ledger", opts) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 2)
	assert.Equal(t, "c-2", pages[0].Pagination.NextCursor)
	assert.Empty(t, pages[1].Pagination.NextCursor)
	assert.Equal(t, int32(2), calls.Load())
	assert.Equal(t, "c-2", *secondCursor)
}

// ─────────────────────────────────────────────────────────────────────────────
// Mid-iteration ctx.Cancel test (H27).
//
// Every iterator has `if ctx.Err() != nil { yield(nil, ctx.Err()); return }`
// inside its per-page loop. Without coverage, this branch is dead from a
// verification standpoint. We test that:
//   1. A canceled context, observed mid-iteration, stops the iterator cleanly.
//   2. No further page request is issued after cancellation.
//   3. The error surfaced to the consumer is context.Canceled (or wraps it).
//
// We pick portfolios as the representative page-based iterator and
// transactions as the representative cursor-based iterator. Both share the
// same cancellation pattern via requestContext + ctx.Err().
// ─────────────────────────────────────────────────────────────────────────────

func TestPortfoliosEntity_ListPortfoliosPages_StopsOnCtxCancel(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Always claim more pages exist — the only way iteration stops is via
		// the ctx.Err() check, NOT via HasMore.
		_ = json.NewEncoder(w).Encode(models.ListResponse[models.Portfolio]{
			Items: []models.Portfolio{{ID: "p-1"}},
			Pagination: models.Pagination{
				Total: 100,
				Limit: 1,
				Page:  int(calls.Load()),
			},
		})
	}))
	defer server.Close()

	entity := newPortfoliosEntity(server.Client(), "tok", map[string]string{"onboarding": server.URL})
	opts := models.PortfoliosListOpts{PageListOpts: models.PageListOpts{Limit: 1}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // safety net for early-error paths

	var (
		pagesGot int
		gotErr   error
	)

	for page, err := range entity.ListPortfoliosPages(ctx, "org", "ledger", opts) {
		if err != nil {
			gotErr = err
			break
		}

		pagesGot++

		require.NotNil(t, page)
		// Cancel after the first page has been yielded — the next iteration of
		// the closure must observe ctx.Err() and exit before issuing another
		// HTTP request.
		cancel()
	}

	require.ErrorIs(t, gotErr, context.Canceled, "iterator must surface the context cancellation as a context.Canceled error")
	assert.Equal(t, 1, pagesGot, "exactly one page yielded before cancel")
	// The strict invariant: no second request was issued after cancellation.
	// The iterator either issued only the first request, or the second request
	// raced past cancel — but typical happy-path is exactly 1 call.
	assert.LessOrEqual(t, calls.Load(), int32(1), "no further HTTP request after cancel")
}

func TestTransactionsEntity_ListTransactionsPages_StopsOnCtxCancel(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Always issue a NextCursor — the only way the iterator stops is via
		// ctx.Err(), not via empty cursor.
		_ = json.NewEncoder(w).Encode(models.ListResponse[models.Transaction]{
			Items: []models.Transaction{{ID: "tx-1"}},
			Pagination: models.Pagination{
				NextCursor: "always-more",
			},
		})
	}))
	defer server.Close()

	entity := newTransactionsEntity(server.Client(), map[string]string{"transaction": server.URL})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // safety net for early-error paths

	var (
		pagesGot int
		gotErr   error
	)

	for page, err := range entity.ListTransactionsPages(ctx, "org", "ledger", models.TransactionsListOpts{}) {
		if err != nil {
			gotErr = err
			break
		}

		pagesGot++

		require.NotNil(t, page)
		cancel()
	}

	require.ErrorIs(t, gotErr, context.Canceled, "iterator must surface the context cancellation as a context.Canceled error")
	assert.Equal(t, 1, pagesGot)
	assert.LessOrEqual(t, calls.Load(), int32(1))
}

// ─────────────────────────────────────────────────────────────────────────────
// Early-break / Stop semantics test (H28).
//
// The `if !yield(...) { return }` branch in every iterator is untested. When
// the consumer breaks out of the for-range loop, the iterator's yield func
// returns false, and the iterator MUST stop without issuing any further
// HTTP request. Coverage check: only one HTTP call observed by the server.
// ─────────────────────────────────────────────────────────────────────────────

func TestPortfoliosEntity_ListPortfoliosPages_StopsOnConsumerBreak(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models.ListResponse[models.Portfolio]{
			Items: []models.Portfolio{{ID: "p-1"}},
			// Always claim more pages so HasMore() = true; consumer must
			// break to stop iteration.
			Pagination: models.Pagination{
				Total: 100,
				Limit: 1,
				Page:  int(calls.Load()),
			},
		})
	}))
	defer server.Close()

	entity := newPortfoliosEntity(server.Client(), "tok", map[string]string{"onboarding": server.URL})
	opts := models.PortfoliosListOpts{PageListOpts: models.PageListOpts{Limit: 1}}

	pagesGot := 0

	for _, err := range entity.ListPortfoliosPages(context.Background(), "org", "ledger", opts) {
		require.NoError(t, err)
		pagesGot++

		break // exit on first page
	}

	assert.Equal(t, 1, pagesGot, "exactly one page yielded before break")
	assert.Equal(t, int32(1), calls.Load(), "no second HTTP request after consumer break")
}

func TestTransactionsEntity_ListTransactionsPages_StopsOnConsumerBreak(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models.ListResponse[models.Transaction]{
			Items: []models.Transaction{{ID: "tx-1"}},
			// Always advertise a next cursor — consumer must break.
			Pagination: models.Pagination{NextCursor: "always-more"},
		})
	}))
	defer server.Close()

	entity := newTransactionsEntity(server.Client(), map[string]string{"transaction": server.URL})

	pagesGot := 0

	for _, err := range entity.ListTransactionsPages(context.Background(), "org", "ledger", models.TransactionsListOpts{}) {
		require.NoError(t, err)
		pagesGot++

		break
	}

	assert.Equal(t, 1, pagesGot)
	assert.Equal(t, int32(1), calls.Load(), "no second HTTP request after consumer break")
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
// cognitive-complexity budget — 16 mechanically-uniform subtests against
// 16 different ListXxxAll entry points add up fast at the function level
// even though each subtest is trivial in isolation.
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

// TestListXxxAll_DelegatesToPages covers H30: the All-variant iterators
// across every entity are 1-line wrappers over `flattenPages(Pages(...))`.
// Without a single drive-through, every All variant sits at 0% coverage
// despite being ALL the work flattenPages was extracted to share. This
// parametrized test exercises each All iterator over a single-page mock
// response to confirm wiring.
//
// Skipped entities here are covered elsewhere:
//   - AccountsAll: TestAccountsEntity_ListAccountsAll_StopsOnConsumerBreak
//   - AssetRates*All: existing coverage in asset_rates_test.go
//   - PortfoliosAll: implicit via TestAccountsEntity flow (page-based shared)
//
// Cursor-based All variants (operations, transactions, operation_routes,
// transaction_routes) are also exercised here so the same flattenPages
// wiring works on a cursor-paginated upstream.
//
//nolint:revive // cognitive-complexity: 16 ListXxxAll wrappers must each be drive-tested through a t.Run; the runListAllSubtest helper already collapses every subtest body to a single call, so the residual complexity is the unavoidable count of entities.
func TestListXxxAll_DelegatesToPages(t *testing.T) {
	t.Run("AccountTypesAll", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.AccountType]{Items: []models.AccountType{{Name: "type-A"}}},
			func(s *httptest.Server) iter.Seq2[models.AccountType, error] {
				e := newAccountTypesEntity(s.Client(), "tok", map[string]string{"onboarding": s.URL})
				return e.ListAccountTypesAll(context.Background(), "org", "ledger", models.AccountTypesListOpts{})
			},
			func(got []models.AccountType) {
				assert.Len(t, got, 1)
				assert.Equal(t, "type-A", got[0].Name)
			},
		)
	})

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

	t.Run("AssetsAll", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Asset]{Items: []models.Asset{{ID: "a-1", Code: "USD"}}},
			func(s *httptest.Server) iter.Seq2[models.Asset, error] {
				e := newAssetsEntity(s.Client(), "tok", map[string]string{"onboarding": s.URL})
				return e.ListAssetsAll(context.Background(), "org", "ledger", models.AssetsListOpts{})
			},
			func(got []models.Asset) {
				assert.Len(t, got, 1)
				assert.Equal(t, "USD", got[0].Code)
			},
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

	t.Run("HoldersAll", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Holder]{Items: []models.Holder{{}}},
			func(s *httptest.Server) iter.Seq2[models.Holder, error] {
				e := newHoldersEntity(s.Client(), "token", map[string]string{"crm": s.URL})
				return e.ListHoldersAll(context.Background(), "org", models.HoldersListOpts{})
			},
			func(got []models.Holder) { assert.Len(t, got, 1) },
		)
	})

	t.Run("LedgersAll", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Ledger]{Items: []models.Ledger{{ID: "l-1", Name: "Main"}}},
			func(s *httptest.Server) iter.Seq2[models.Ledger, error] {
				e := newLedgersEntity(s.Client(), "token", map[string]string{"onboarding": s.URL})
				return e.ListLedgersAll(context.Background(), "org", models.LedgersListOpts{})
			},
			func(got []models.Ledger) {
				assert.Len(t, got, 1)
				assert.Equal(t, "Main", got[0].Name)
			},
		)
	})

	t.Run("OrganizationsAll", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Organization]{Items: []models.Organization{{ID: "o-1"}}},
			func(s *httptest.Server) iter.Seq2[models.Organization, error] {
				e := newOrganizationsEntity(s.Client(), "tok", map[string]string{"onboarding": s.URL})
				return e.ListOrganizationsAll(context.Background(), models.OrganizationsListOpts{})
			},
			func(got []models.Organization) { assert.Len(t, got, 1) },
		)
	})

	t.Run("PortfoliosAll", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Portfolio]{Items: []models.Portfolio{{ID: "p-1"}}},
			func(s *httptest.Server) iter.Seq2[models.Portfolio, error] {
				e := newPortfoliosEntity(s.Client(), "tok", map[string]string{"onboarding": s.URL})
				return e.ListPortfoliosAll(context.Background(), "org", "ledger", models.PortfoliosListOpts{})
			},
			func(got []models.Portfolio) { assert.Len(t, got, 1) },
		)
	})

	t.Run("SegmentsAll", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Segment]{Items: []models.Segment{{ID: "s-1"}}},
			func(s *httptest.Server) iter.Seq2[models.Segment, error] {
				e := newSegmentsEntity(s.Client(), "tok", map[string]string{"onboarding": s.URL})
				return e.ListSegmentsAll(context.Background(), "org", "ledger", models.SegmentsListOpts{})
			},
			func(got []models.Segment) { assert.Len(t, got, 1) },
		)
	})

	t.Run("TransactionsAll (cursor)", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.Transaction]{Items: []models.Transaction{{ID: "tx-1"}}},
			func(s *httptest.Server) iter.Seq2[models.Transaction, error] {
				e := newTransactionsEntity(s.Client(), map[string]string{"transaction": s.URL})
				return e.ListTransactionsAll(context.Background(), "org", "ledger", models.TransactionsListOpts{})
			},
			func(got []models.Transaction) { assert.Len(t, got, 1) },
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

	t.Run("OperationRoutesAll (cursor)", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.OperationRoute]{Items: []models.OperationRoute{{Title: "or-1"}}},
			func(s *httptest.Server) iter.Seq2[models.OperationRoute, error] {
				e := newOperationRoutesEntity(s.Client(), "tok", map[string]string{"transaction": s.URL})
				return e.ListOperationRoutesAll(context.Background(), "org", "ledger", models.OperationRoutesListOpts{})
			},
			func(got []models.OperationRoute) {
				assert.Len(t, got, 1)
				assert.Equal(t, "or-1", got[0].Title)
			},
		)
	})

	t.Run("TransactionRoutesAll (cursor)", func(t *testing.T) {
		runListAllSubtest(t,
			models.ListResponse[models.TransactionRoute]{Items: []models.TransactionRoute{{Title: "tr-1"}}},
			func(s *httptest.Server) iter.Seq2[models.TransactionRoute, error] {
				e := newTransactionRoutesEntity(s.Client(), "tok", map[string]string{"transaction": s.URL})
				return e.ListTransactionRoutesAll(context.Background(), "org", "ledger", models.TransactionRoutesListOpts{})
			},
			func(got []models.TransactionRoute) { assert.Len(t, got, 1) },
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

// TestListAccountsAll_StopsOnConsumerBreak covers the All variant of an
// iterator (which routes through flattenPages). Returning false from yield
// must propagate up through both layers and stop the underlying Pages
// iterator.
func TestAccountsEntity_ListAccountsAll_StopsOnConsumerBreak(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Each page yields 3 items + claims more pages exist.
		_ = json.NewEncoder(w).Encode(models.ListResponse[models.Account]{
			Items: []models.Account{{ID: "a-1"}, {ID: "a-2"}, {ID: "a-3"}},
			Pagination: models.Pagination{
				Total: 100,
				Limit: 3,
				Page:  int(calls.Load()),
			},
		})
	}))
	defer server.Close()

	entity := newAccountsEntity(server.Client(), "tok", map[string]string{"onboarding": server.URL})

	itemsGot := 0

	for _, err := range entity.ListAccountsAll(context.Background(), "org", "ledger", models.AccountsListOpts{PageListOpts: models.PageListOpts{Limit: 3}}) {
		require.NoError(t, err)
		itemsGot++
		// Stop after first item — Pages must not advance, flattenPages must
		// not consume the rest of page 1's items.
		break
	}

	assert.Equal(t, 1, itemsGot)
	assert.Equal(t, int32(1), calls.Load(), "Pages iterator must stop after consumer breaks out of All")

	// Give the test a moment to detect any rogue background request that
	// might happen if the iterator failed to stop properly. There shouldn't
	// be any — iter.Seq2 is fully synchronous — but assert by reading calls
	// once more after a short delay to catch any post-break request.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), calls.Load())
}

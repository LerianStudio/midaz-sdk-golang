// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// The path ids, uuids and dates every V2 behavioural table in this package
// shares. They are ordinary fixtures with one constraint: the uuids must be
// REAL uuids, because the route and account-type models decode their id into
// uuid.UUID and a fake id fails the unmarshal before the behaviour under test
// is ever reached.
const (
	v2Org     = "11111111-1111-1111-1111-111111111111"
	v2Ledger  = "22222222-2222-2222-2222-222222222222"
	v2Account = "33333333-3333-3333-3333-333333333333"
	v2UUIDA   = "44444444-4444-4444-4444-444444444444"
	v2UUIDB   = "55555555-5555-5555-5555-555555555555"

	v2StartDate = "2025-01-01"
	v2EndDate   = "2025-01-31"
)

// The V2 list surface is bridged, not written: every listXV2Params reads its V1
// twin's output and copies it across field by field, because oapi-codegen emits
// a distinct Go type per operation. A field missed in that copy is the quietest
// defect this SDK can ship — the caller sets a narrowing, the request goes out
// WITHOUT it, and the full result set comes back reading exactly like a narrowed
// one. Nothing errors, nothing is empty, and the caller reconciles against rows
// they asked not to see.
//
// So the property is not "the list works". It is: every narrowing the caller set
// is on the wire, and the page that comes back reaches them decoded. Both halves
// are asserted per row, because a bridge that dropped a field and a decoder that
// dropped the page fail in opposite directions and one row proves neither.
//
// EXHAUSTIVENESS, AND THE TWELVE LINES THIS TABLE CANNOT REACH. The claim above
// is mutation-proven, not asserted: deleting any one of the 102 `Field:
// v1.Field` assignments in v2_params.go reddens this suite — 90 of them do.
// The remaining twelve copy a field that NO opts type can set, so they always
// copy nil and no row can distinguish their absence:
//
//   - Metadata, on all nine bridge functions that carry it. No V1 mapper in
//     this package assigns params.Metadata at all; the ledger's JSON-string
//     metadata predicate has no SDK opts surface. (The transaction list's
//     metadata.<key> predicate is a different query parameter, injected by
//     metadataFilterEditors and pinned in transactions_list_filters_test.go.)
//   - DoingBusinessAs and LegalDocument, on listOrganizationsV2Params.
//     OrganizationsFilters declares neither and listOrganizationsParams never
//     sets them; those two identifiers appear nowhere else in the package.
//   - Cursor, on listAccountTypesV2Params. AccountTypesListOpts embeds
//     PageListOpts, so there is no cursor to copy.
//
// Those twelve stay in the bridge on purpose — the bridge exists so that a
// filter added to a V1 mapper reaches /v2 without anyone wiring it twice, and a
// deleted line is that silent drop arriving later. A THIRTEENTH unreddenable
// line is a finding, not an exemption: re-run the sweep when this table or
// v2_params.go changes, and either pin the line or add it here with its reason.

// v2ListReads is one row per V2 list whose narrowings travel through the
// listXV2Params bridge, with the query the row's opts must produce and the page
// body it must decode. Twelve rows, which is every bridged list EXCEPT the
// transaction list: that one refuses six of its filters and expresses its
// metadata predicate through a request editor, so it is pinned end to end next
// door in transactions_list_filters_test.go rather than duplicated here.
//
// PAGE-based and CURSOR-based families are both here, and the pagination
// assertion differs by row for that reason: a page family that echoed a cursor
// would stop early, and a cursor family that counted pages would loop. The row
// states which signal it expects to survive the decode.
var v2ListReads = []struct {
	name string
	// wantQuery is every narrowing the row's opts set. A key absent from the
	// wire fails the row; the map is exhaustive for what the opts can express,
	// not a sample — every field of the row's filter struct is set above and
	// asserted here, including the ones carried by a request editor rather
	// than by the params struct.
	wantQuery map[string]string
	// page is a realistic single page for this family.
	page string
	// wantIDs are the item ids page must decode into, in order.
	wantIDs []string
	// wantPagination is the page/cursor signal the caller reads to keep walking.
	wantPagination models.Pagination
	list           func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error)
}{
	{
		name: "V2.Organizations.List",
		wantQuery: map[string]string{
			"limit": "5", "page": "2", "sort_order": "asc",
			"start_date": "2025-01-01", "end_date": "2025-01-31",
			"legal_name": "Acme", "status": "ACTIVE", "include_deleted": "true",
		},
		page:           `{"items":[{"id":"org-a","legalName":"Acme"},{"id":"org-b","legalName":"Acme Two"}],"limit":5,"page":2}`,
		wantIDs:        []string{"org-a", "org-b"},
		wantPagination: models.Pagination{Limit: 5, Page: 2, ItemCount: 2},
		list: func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error) {
			t.Helper()

			page, err := newOrganizationsV2Facade(newTestLedgerClient(t, srv), true).
				List(context.Background(), models.OrganizationsListOpts{
					PageListOpts: pageOptsFixture(),
					Filters:      models.OrganizationsFilters{LegalName: "Acme", Status: "ACTIVE", IncludeDeleted: true},
				})

			return idsOfPage(page, err, func(o models.Organization) string { return o.ID })
		},
	},
	{
		name: "V2.Ledgers.List",
		wantQuery: map[string]string{
			"limit": "5", "page": "2", "sort_order": "asc",
			"start_date": "2025-01-01", "end_date": "2025-01-31",
			"name": "Treasury", "status": "ACTIVE", "include_deleted": "true",
		},
		page:           `{"items":[{"id":"led-a","name":"Treasury"}],"limit":5,"page":2}`,
		wantIDs:        []string{"led-a"},
		wantPagination: models.Pagination{Limit: 5, Page: 2, ItemCount: 1},
		list: func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error) {
			t.Helper()

			page, err := newLedgersV2Facade(newTestLedgerClient(t, srv), true).
				List(context.Background(), v2Org, models.LedgersListOpts{
					PageListOpts: pageOptsFixture(),
					Filters:      models.LedgersFilters{Name: "Treasury", Status: "ACTIVE", IncludeDeleted: true},
				})

			return idsOfPage(page, err, func(l models.Ledger) string { return l.ID })
		},
	},
	{
		name: "V2.Accounts.List",
		// Twelve filters, the widest opts on the surface. All twelve are set
		// and all twelve are asserted, which is where "exhaustive, not a
		// sample" earns its keep: six of them reach the wire through a
		// listAccountsV2Params bridge line nothing else in the suite runs.
		wantQuery: map[string]string{
			"limit": "5", "page": "2", "sort_order": "asc",
			"start_date": "2025-01-01", "end_date": "2025-01-31",
			"type": "deposit", "status": "ACTIVE", "asset_code": "USD",
			"alias": "@cash", "include_deleted": "true",
			"portfolio_id": v2UUIDA, "segment_id": v2UUIDB,
			"parent_account_id": v2Account, "entity_id": "ent-9",
			"name": "Cash USD", "blocked": "true", "holder_id": "hld-1",
		},
		page:           `{"items":[{"id":"acc-a","alias":"@cash","assetCode":"USD"}],"limit":5,"page":2}`,
		wantIDs:        []string{"acc-a"},
		wantPagination: models.Pagination{Limit: 5, Page: 2, ItemCount: 1},
		list: func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error) {
			t.Helper()

			page, err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).
				List(context.Background(), v2Org, v2Ledger, models.AccountsListOpts{
					PageListOpts: pageOptsFixture(),
					Filters: models.AccountsFilters{
						Type: "deposit", Status: "ACTIVE", AssetCode: "USD",
						Alias: "@cash", IncludeDeleted: true,
						PortfolioID: v2UUIDA, SegmentID: v2UUIDB,
						ParentAccountID: v2Account, EntityID: "ent-9",
						Name: "Cash USD", Blocked: true, HolderID: "hld-1",
					},
				})

			return idsOfPage(page, err, func(a models.Account) string { return a.ID })
		},
	},
	{
		name: "V2.AccountTypes.List",
		wantQuery: map[string]string{
			"limit": "5", "page": "2", "sort_order": "asc",
			"start_date": "2025-01-01", "end_date": "2025-01-31",
			"key_value": "CASH", "name": "Cash", "include_deleted": "true",
		},
		page:           `{"items":[{"id":"` + v2UUIDA + `","keyValue":"CASH"}],"limit":5,"page":2}`,
		wantIDs:        []string{v2UUIDA},
		wantPagination: models.Pagination{Limit: 5, Page: 2, ItemCount: 1},
		list: func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error) {
			t.Helper()

			page, err := newAccountTypesV2Facade(newTestLedgerClient(t, srv), true).
				List(context.Background(), v2Org, v2Ledger, models.AccountTypesListOpts{
					PageListOpts: pageOptsFixture(),
					Filters:      models.AccountTypesFilters{Name: "Cash", KeyValue: "CASH", IncludeDeleted: true},
				})

			return idsOfPage(page, err, func(at models.AccountType) string { return at.ID.String() })
		},
	},
	{
		name: "V2.Assets.List",
		wantQuery: map[string]string{
			"limit": "5", "page": "2", "sort_order": "asc",
			"start_date": "2025-01-01", "end_date": "2025-01-31",
			"code": "USD", "type": "currency", "status": "ACTIVE",
		},
		page:           `{"items":[{"id":"ast-a","code":"USD"}],"limit":5,"page":2}`,
		wantIDs:        []string{"ast-a"},
		wantPagination: models.Pagination{Limit: 5, Page: 2, ItemCount: 1},
		list: func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error) {
			t.Helper()

			page, err := newAssetsV2Facade(newTestLedgerClient(t, srv), true).
				List(context.Background(), v2Org, v2Ledger, models.AssetsListOpts{
					PageListOpts: pageOptsFixture(),
					Filters:      models.AssetsFilters{Code: "USD", Type: "currency", Status: "ACTIVE"},
				})

			return idsOfPage(page, err, func(a models.Asset) string { return a.ID })
		},
	},
	{
		name: "V2.Portfolios.List",
		wantQuery: map[string]string{
			"limit": "5", "page": "2", "sort_order": "asc",
			"start_date": "2025-01-01", "end_date": "2025-01-31",
			"name": "Alpha", "entity_id": "ent-1", "status": "ACTIVE", "include_deleted": "true",
		},
		page:           `{"items":[{"id":"pf-a","name":"Alpha"}],"limit":5,"page":2}`,
		wantIDs:        []string{"pf-a"},
		wantPagination: models.Pagination{Limit: 5, Page: 2, ItemCount: 1},
		list: func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error) {
			t.Helper()

			page, err := newPortfoliosV2Facade(newTestLedgerClient(t, srv), true).
				List(context.Background(), v2Org, v2Ledger, models.PortfoliosListOpts{
					PageListOpts: pageOptsFixture(),
					Filters: models.PortfoliosFilters{
						Name: "Alpha", EntityID: "ent-1", Status: "ACTIVE", IncludeDeleted: true,
					},
				})

			return idsOfPage(page, err, func(p models.Portfolio) string { return p.ID })
		},
	},
	{
		name: "V2.Segments.List",
		wantQuery: map[string]string{
			"limit": "5", "page": "2", "sort_order": "asc",
			"start_date": "2025-01-01", "end_date": "2025-01-31",
			"name": "North", "status": "ACTIVE", "include_deleted": "true",
		},
		page:           `{"items":[{"id":"sg-a","name":"North"}],"limit":5,"page":2}`,
		wantIDs:        []string{"sg-a"},
		wantPagination: models.Pagination{Limit: 5, Page: 2, ItemCount: 1},
		list: func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error) {
			t.Helper()

			page, err := newSegmentsV2Facade(newTestLedgerClient(t, srv), true).
				List(context.Background(), v2Org, v2Ledger, models.SegmentsListOpts{
					PageListOpts: pageOptsFixture(),
					Filters:      models.SegmentsFilters{Name: "North", Status: "ACTIVE", IncludeDeleted: true},
				})

			return idsOfPage(page, err, func(s models.Segment) string { return s.ID })
		},
	},
	{
		name: "V2.OperationRoutes.List",
		wantQuery: map[string]string{
			"limit": "5", "cursor": "cur-in", "sort_order": "asc",
			"start_date": "2025-01-01", "end_date": "2025-01-31",
			"name": "Cashin", "status": "ACTIVE", "operation_type": "source",
		},
		page:           `{"items":[{"id":"` + v2UUIDA + `","title":"Cashin"}],"limit":5,"next_cursor":"cur-out"}`,
		wantIDs:        []string{v2UUIDA},
		wantPagination: models.Pagination{Limit: 5, NextCursor: "cur-out", ItemCount: 1},
		list: func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error) {
			t.Helper()

			page, err := newOperationRoutesV2Facade(newTestLedgerClient(t, srv), true).
				List(context.Background(), v2Org, v2Ledger, models.OperationRoutesListOpts{
					CursorListOpts: cursorOptsFixture(),
					Filters: models.OperationRoutesFilters{
						Name: "Cashin", Status: "ACTIVE", OperationType: "source",
					},
				})

			return idsOfPage(page, err, func(r models.OperationRoute) string { return r.ID.String() })
		},
	},
	{
		name: "V2.TransactionRoutes.List",
		wantQuery: map[string]string{
			"limit": "5", "cursor": "cur-in", "sort_order": "asc",
			"start_date": "2025-01-01", "end_date": "2025-01-31",
			"name": "Settlement", "status": "ACTIVE", "operation_route_id": v2UUIDB,
		},
		page:           `{"items":[{"id":"` + v2UUIDA + `","title":"Settlement"}],"limit":5,"next_cursor":"cur-out"}`,
		wantIDs:        []string{v2UUIDA},
		wantPagination: models.Pagination{Limit: 5, NextCursor: "cur-out", ItemCount: 1},
		list: func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error) {
			t.Helper()

			page, err := newTransactionRoutesV2Facade(newTestLedgerClient(t, srv), true).
				List(context.Background(), v2Org, v2Ledger, models.TransactionRoutesListOpts{
					CursorListOpts: cursorOptsFixture(),
					Filters: models.TransactionRoutesFilters{
						Name: "Settlement", Status: "ACTIVE", OperationRouteID: v2UUIDB,
					},
				})

			return idsOfPage(page, err, func(r models.TransactionRoute) string { return r.ID.String() })
		},
	},
	{
		name: "V2.Balances.ListBalances",
		wantQuery: map[string]string{
			"limit": "5", "cursor": "cur-in", "sort_order": "asc",
			"start_date": "2025-01-01", "end_date": "2025-01-31",
		},
		page:           `{"items":[{"id":"bal-a","assetCode":"USD","available":"10.5"}],"limit":5,"next_cursor":"cur-out"}`,
		wantIDs:        []string{"bal-a"},
		wantPagination: models.Pagination{Limit: 5, NextCursor: "cur-out", ItemCount: 1},
		list: func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error) {
			t.Helper()

			page, err := newBalancesV2Facade(newTestLedgerClient(t, srv), true).
				ListBalances(context.Background(), v2Org, v2Ledger, models.BalancesListOpts{
					CursorListOpts: cursorOptsFixture(),
				})

			return idsOfPage(page, err, func(b models.Balance) string { return b.ID })
		},
	},
	{
		name: "V2.Balances.ListAccountBalances",
		wantQuery: map[string]string{
			"limit": "5", "cursor": "cur-in", "sort_order": "asc",
			"start_date": "2025-01-01", "end_date": "2025-01-31",
		},
		page:           `{"items":[{"id":"bal-b","assetCode":"USD","available":"7"}],"limit":5,"next_cursor":"cur-out"}`,
		wantIDs:        []string{"bal-b"},
		wantPagination: models.Pagination{Limit: 5, NextCursor: "cur-out", ItemCount: 1},
		list: func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error) {
			t.Helper()

			page, err := newBalancesV2Facade(newTestLedgerClient(t, srv), true).
				ListAccountBalances(context.Background(), v2Org, v2Ledger, v2Account, models.BalancesListOpts{
					CursorListOpts: cursorOptsFixture(),
				})

			return idsOfPage(page, err, func(b models.Balance) string { return b.ID })
		},
	},
	{
		name: "V2.Operations.ListOperations",
		wantQuery: map[string]string{
			"limit": "5", "cursor": "cur-in", "sort_order": "asc",
			"start_date": "2025-01-01", "end_date": "2025-01-31",
			"type": "DEBIT", "direction": "debit",
			"route_id": v2UUIDB, "route_code": "CASHOUT",
		},
		page:           `{"items":[{"id":"op-a","amount":{"value":"3"}}],"limit":5,"next_cursor":"cur-out"}`,
		wantIDs:        []string{"op-a"},
		wantPagination: models.Pagination{Limit: 5, NextCursor: "cur-out", ItemCount: 1},
		list: func(t *testing.T, srv *httptest.Server) ([]string, models.Pagination, error) {
			t.Helper()

			page, err := newOperationsV2Facade(newTestLedgerClient(t, srv), true).
				ListOperations(context.Background(), v2Org, v2Ledger, v2Account, models.OperationsListOpts{
					CursorListOpts: cursorOptsFixture(),
					Filters: models.AccountOperationsFilters{
						Type: "DEBIT", Direction: "debit",
						RouteID: v2UUIDB, RouteCode: "CASHOUT",
					},
				})

			return idsOfPage(page, err, func(o models.Operation) string { return o.ID })
		},
	},
}

// TestV2ListCarriesEveryNarrowingToTheWire is the bridge guard.
//
// Each row sets every narrowing its opts can express and asserts the query that
// left. A field the listXV2Params copy forgot is silently absent here, which is
// the whole point: on a live server that absence is invisible, because an
// unnarrowed result set looks exactly like a narrowed one to the caller.
func TestV2ListCarriesEveryNarrowingToTheWire(t *testing.T) {
	for _, read := range v2ListReads {
		t.Run(read.name, func(t *testing.T) {
			var query url.Values

			srv := queryCapturingServer(t, &query, read.page)

			if _, _, err := read.list(t, srv); err != nil {
				t.Fatalf("List: %v", err)
			}

			assertNarrowingsOnWire(t, query, read.wantQuery)
		})
	}
}

// TestV2ListDecodesThePageItReceives is the other half. A bridge that carried
// every filter perfectly and a decoder that dropped the items would both pass
// the query assertion above; a caller reads the items and the cursor, not the
// request.
func TestV2ListDecodesThePageItReceives(t *testing.T) {
	for _, read := range v2ListReads {
		t.Run(read.name, func(t *testing.T) {
			var query url.Values

			srv := queryCapturingServer(t, &query, read.page)

			ids, pagination, err := read.list(t, srv)
			if err != nil {
				t.Fatalf("List: %v", err)
			}

			assertIDs(t, ids, read.wantIDs)

			if pagination != read.wantPagination {
				t.Fatalf("pagination = %+v, want %+v — the caller cannot walk past this page",
					pagination, read.wantPagination)
			}
		})
	}
}

// TestV2ListSendsNoNarrowingWhenTheCallerSetNone is the negative half of the
// bridge guard: a mapper that emitted a zero value instead of omitting the
// field narrows a result set the caller asked to see whole. "limit=0" is the
// expensive spelling — a server that honours it returns nothing.
func TestV2ListSendsNoNarrowingWhenTheCallerSetNone(t *testing.T) {
	var query url.Values

	srv := queryCapturingServer(t, &query, `{"items":[],"limit":10}`)

	if _, err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).
		List(context.Background(), v2Org, v2Ledger, models.AccountsListOpts{}); err != nil {
		t.Fatalf("List: %v", err)
	}

	omitted := []string{
		"limit", "page", "sort_order", "start_date", "end_date",
		"status", "type", "alias", "asset_code", "portfolio_id", "segment_id",
		"parent_account_id", "entity_id", "name", "blocked", "holder_id",
		"include_deleted",
	}

	for _, key := range omitted {
		if got, ok := query[key]; ok {
			t.Fatalf("%s = %v, want the parameter omitted entirely when unset", key, got)
		}
	}
}

// The narrowing every row sets. They are constants rather than per-row values
// so that each row's wantQuery stays a LITERAL expectation: a table that
// computed the expected query from the same fixture it sent would pass no
// matter what the bridge did with it.
const (
	v2ListLimit  = 5
	v2ListPage   = 2
	v2ListCursor = "cur-in"
)

// pageOptsFixture is the page-based narrowing every page row sets.
func pageOptsFixture() models.PageListOpts {
	return models.PageListOpts{
		Limit:         v2ListLimit,
		Page:          v2ListPage,
		SortDirection: models.SortAscending,
		StartDate:     v2StartDate,
		EndDate:       v2EndDate,
	}
}

// cursorOptsFixture is pageOptsFixture for the cursor families.
func cursorOptsFixture() models.CursorListOpts {
	return models.CursorListOpts{
		Limit:         v2ListLimit,
		Cursor:        v2ListCursor,
		SortDirection: models.SortAscending,
		StartDate:     v2StartDate,
		EndDate:       v2EndDate,
	}
}

// queryCapturingServer records the query of every request and answers each one
// with body.
func queryCapturingServer(t *testing.T, query *url.Values, body string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*query = r.URL.Query()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(srv.Close)

	return srv
}

// assertNarrowingsOnWire checks every expected narrowing against the query that left.
func assertNarrowingsOnWire(t *testing.T, got url.Values, want map[string]string) {
	t.Helper()

	for key, value := range want {
		if got.Get(key) != value {
			t.Fatalf("%s = %q, want %q — a narrowing the caller set never reached the server",
				key, got.Get(key), value)
		}
	}
}

// assertIDs compares the decoded item ids with the ids the fixture page carries.
func assertIDs(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("decoded %d items, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("item %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// idsOfPage drains a decoded page into the ids the assertions compare, guarding
// the nil dereference a failed read would otherwise cause in the extractor.
func idsOfPage[T any](page *models.ListResponse[T], err error, id func(T) string) ([]string, models.Pagination, error) {
	if err != nil {
		return nil, models.Pagination{}, err
	}

	if page == nil {
		return nil, models.Pagination{}, nil
	}

	ids := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, id(item))
	}

	return ids, page.Pagination, nil
}

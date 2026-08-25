// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// The V2 reads in this file are the ones a routing table cannot vouch for: each
// answers with a shape of its own — a settings object, a point-in-time balance,
// a bare array, a count that travels in a HEADER rather than a body — so the
// decode is per-endpoint code and a sibling row proves nothing about it.
//
// The shared failure is the same one this epic has now fixed on four surfaces:
// a read that returns a zero-valued model with a nil error. Nothing distinguishes
// "the account holds nothing" from "the SDK dropped the answer", and on a
// reconciliation path the two lead to opposite actions.

// TestV2LookupReadsDecodeTheResourceTheyName covers the alias and external-code
// lookups, which reach an account by a path segment the caller controls rather
// than by id.
//
// The external-code lookup matters beyond its coverage: it reaches the ledger's
// EXTERNAL account, the counterparty every deposit is drawn from. A zero-valued
// answer there names no account at all, and the deposit is built against it.
func TestV2LookupReadsDecodeTheResourceTheyName(t *testing.T) {
	reads := []struct {
		name     string
		body     string
		wantID   string
		wantMore map[string]string
		read     func(t *testing.T, srv *httptest.Server) (*models.Account, error)
	}{
		{
			name:     "V2.Accounts.GetByAlias",
			body:     `{"id":"acc-alias","alias":"@cash","assetCode":"USD","type":"deposit"}`,
			wantID:   "acc-alias",
			wantMore: map[string]string{"alias": "@cash", "assetCode": "USD"},
			read: func(t *testing.T, srv *httptest.Server) (*models.Account, error) {
				t.Helper()

				return newAccountsV2Facade(newTestLedgerClient(t, srv), true).
					GetByAlias(context.Background(), v2Org, v2Ledger, "@cash")
			},
		},
		{
			name:     "V2.Accounts.GetByExternalCode",
			body:     `{"id":"acc-external","alias":"@external/USD","assetCode":"USD","type":"external"}`,
			wantID:   "acc-external",
			wantMore: map[string]string{"alias": "@external/USD", "assetCode": "USD"},
			read: func(t *testing.T, srv *httptest.Server) (*models.Account, error) {
				t.Helper()

				return newAccountsV2Facade(newTestLedgerClient(t, srv), true).
					GetByExternalCode(context.Background(), v2Org, v2Ledger, "USD")
			},
		},
	}

	for _, read := range reads {
		t.Run(read.name, func(t *testing.T) {
			account, err := read.read(t, emptyBodyServer(t, http.StatusOK, read.body))
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			if account.ID != read.wantID {
				t.Fatalf("id = %q, want %q", account.ID, read.wantID)
			}

			got := map[string]string{"alias": derefString(account.Alias), "assetCode": account.AssetCode}
			for field, want := range read.wantMore {
				if got[field] != want {
					t.Fatalf("%s = %q, want %q", field, got[field], want)
				}
			}
		})
	}
}

// TestV2LedgerSettingsDecodeEveryGate is the decode guard for a read whose whole
// payload is booleans.
//
// Booleans are the worst thing to lose in a decode: a dropped field is
// indistinguishable from a deliberate `false`, and every gate here reads
// "permitted" when it is lost. A caller checking whether this ledger allows a
// fee skip gets "yes" from a decode that returned nothing at all — so the
// fixture sets each flag to the value that is NOT the zero value.
func TestV2LedgerSettingsDecodeEveryGate(t *testing.T) {
	const body = `{
		"accounting":{"requireHolder":true,"validateAccountType":true,"validateRoutes":true},
		"overrides":{"allowFeeSkip":true,"allowHolderSkip":true,"allowTracerSkip":true}
	}`

	settings, err := newLedgersV2Facade(newTestLedgerClient(t, emptyBodyServer(t, http.StatusOK, body)), true).
		GetSettings(context.Background(), v2Org, v2UUIDA)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}

	gates := map[string]bool{
		"accounting.requireHolder":       settings.Accounting.RequireHolder,
		"accounting.validateAccountType": settings.Accounting.ValidateAccountType,
		"accounting.validateRoutes":      settings.Accounting.ValidateRoutes,
		"overrides.allowFeeSkip":         settings.Overrides.AllowFeeSkip,
		"overrides.allowHolderSkip":      settings.Overrides.AllowHolderSkip,
		"overrides.allowTracerSkip":      settings.Overrides.AllowTracerSkip,
	}

	for gate, enabled := range gates {
		if !enabled {
			t.Fatalf("%s = false, want true — a dropped gate reads as one the ledger never set", gate)
		}
	}
}

// TestV2BalanceHistoryKeepsMoneyExact is the money-path decode guard for the
// point-in-time reads.
//
// A balance history is what a reconciliation run compares against. Rounding
// "1500.00000001" through a float reports a discrepancy that does not exist,
// and rounding it the other way hides one that does.
func TestV2BalanceHistoryKeepsMoneyExact(t *testing.T) {
	const (
		available = "1500.00000001"
		onHold    = "0.00000002"
	)

	entry := `{"id":"bh-1","accountId":"` + v2Account + `","assetCode":"USD","available":"` +
		available + `","onHold":"` + onHold + `","version":7}`

	t.Run("single balance history", func(t *testing.T) {
		history, err := newBalancesV2Facade(newTestLedgerClient(t, emptyBodyServer(t, http.StatusOK, entry)), true).
			GetBalanceHistory(context.Background(), v2Org, v2Ledger, v2UUIDA, balanceInstant)
		if err != nil {
			t.Fatalf("GetBalanceHistory: %v", err)
		}

		assertHistoryExact(t, *history, available, onHold)
	})

	t.Run("account balance history", func(t *testing.T) {
		entries, err := newBalancesV2Facade(newTestLedgerClient(t, emptyBodyServer(t, http.StatusOK, "["+entry+"]")), true).
			GetAccountBalancesHistory(context.Background(), v2Org, v2Ledger, v2Account, balanceInstant)
		if err != nil {
			t.Fatalf("GetAccountBalancesHistory: %v", err)
		}

		if len(entries) != 1 {
			t.Fatalf("decoded %d entries, want 1", len(entries))
		}

		assertHistoryExact(t, entries[0], available, onHold)
	})
}

// assertHistoryExact checks one history entry decoded its money without loss.
func assertHistoryExact(t *testing.T, entry models.BalanceHistory, available, onHold string) {
	t.Helper()

	if entry.Available.String() != available {
		t.Fatalf("available = %s, want the exact decimal %s", entry.Available, available)
	}

	if entry.OnHold.String() != onHold {
		t.Fatalf("onHold = %s, want the exact decimal %s", entry.OnHold, onHold)
	}

	if entry.Version != 7 {
		t.Fatalf("version = %d, want 7 — the optimistic-lock version a writer needs", entry.Version)
	}
}

// TestV2BalanceLookupListsDecodeTheirItems covers the two balance lists reached
// by alias and external code. They answer with a paginated envelope like the
// other lists but take no opts, so nothing else in this package exercises them.
func TestV2BalanceLookupListsDecodeTheirItems(t *testing.T) {
	const body = `{"items":[{"id":"bal-x","assetCode":"USD","available":"42.5","key":"default"}],"limit":10}`

	lists := []struct {
		name string
		list func(t *testing.T, srv *httptest.Server) (*models.ListResponse[models.Balance], error)
	}{
		{
			name: "V2.Balances.ListBalancesByAccountAlias",
			list: func(t *testing.T, srv *httptest.Server) (*models.ListResponse[models.Balance], error) {
				t.Helper()

				return newBalancesV2Facade(newTestLedgerClient(t, srv), true).
					ListBalancesByAccountAlias(context.Background(), v2Org, v2Ledger, "@cash")
			},
		},
		{
			name: "V2.Balances.ListBalancesByExternalCode",
			list: func(t *testing.T, srv *httptest.Server) (*models.ListResponse[models.Balance], error) {
				t.Helper()

				return newBalancesV2Facade(newTestLedgerClient(t, srv), true).
					ListBalancesByExternalCode(context.Background(), v2Org, v2Ledger, "USD")
			},
		},
	}

	for _, list := range lists {
		t.Run(list.name, func(t *testing.T) {
			page, err := list.list(t, emptyBodyServer(t, http.StatusOK, body))
			if err != nil {
				t.Fatalf("list: %v", err)
			}

			if len(page.Items) != 1 {
				t.Fatalf("decoded %d balances, want 1", len(page.Items))
			}

			if got := page.Items[0].Available.String(); got != "42.5" {
				t.Fatalf("available = %s, want the exact decimal 42.5", got)
			}

			if page.Items[0].ID != "bal-x" {
				t.Fatalf("id = %q, want bal-x", page.Items[0].ID)
			}
		})
	}
}

// TestV2MetadataIndexListDecodesEachIndex pins the bare-array read's positive
// half. The empty-body table already proves it accepts "null" and preserves a
// gateway status; neither says the index fields arrive.
func TestV2MetadataIndexListDecodesEachIndex(t *testing.T) {
	const body = `[{"indexName":"idx_customer_id","entityName":"account","metadataKey":"customer_id","unique":true}]`

	indexes, err := newMetadataIndexesV2Facade(newTestLedgerClient(t, emptyBodyServer(t, http.StatusOK, body)), true).
		List(context.Background(), "account")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(indexes) != 1 {
		t.Fatalf("decoded %d indexes, want 1", len(indexes))
	}

	if indexes[0].MetadataKey != "customer_id" || !indexes[0].Unique {
		t.Fatalf("index = %+v, want the customer_id key marked unique", indexes[0])
	}
}

// v2Counts is one row per V2 count. Each reaches its own generated HEAD
// operation, so the six are six behaviours rather than one.
var v2Counts = []struct {
	name  string
	count func(t *testing.T, srv *httptest.Server) (int, error)
}{
	{"V2.Organizations.Count", func(t *testing.T, srv *httptest.Server) (int, error) {
		t.Helper()

		return newOrganizationsV2Facade(newTestLedgerClient(t, srv), true).Count(context.Background())
	}},
	{"V2.Ledgers.Count", func(t *testing.T, srv *httptest.Server) (int, error) {
		t.Helper()

		return newLedgersV2Facade(newTestLedgerClient(t, srv), true).Count(context.Background(), v2Org)
	}},
	{"V2.Accounts.Count", func(t *testing.T, srv *httptest.Server) (int, error) {
		t.Helper()

		return newAccountsV2Facade(newTestLedgerClient(t, srv), true).Count(context.Background(), v2Org, v2Ledger)
	}},
	{"V2.Assets.Count", func(t *testing.T, srv *httptest.Server) (int, error) {
		t.Helper()

		return newAssetsV2Facade(newTestLedgerClient(t, srv), true).Count(context.Background(), v2Org, v2Ledger)
	}},
	{"V2.Portfolios.Count", func(t *testing.T, srv *httptest.Server) (int, error) {
		t.Helper()

		return newPortfoliosV2Facade(newTestLedgerClient(t, srv), true).Count(context.Background(), v2Org, v2Ledger)
	}},
	{"V2.Segments.Count", func(t *testing.T, srv *httptest.Server) (int, error) {
		t.Helper()

		return newSegmentsV2Facade(newTestLedgerClient(t, srv), true).Count(context.Background(), v2Org, v2Ledger)
	}},
}

// TestV2CountReadsTheTotalFromTheHeader pins where a V2 count actually comes
// from: an X-Total-Count header on a HEAD reply, not a body.
//
// The endpoint is HEAD, so there is nothing else to read. A count that looked in
// the body would find nothing and, without the header check below, report zero.
func TestV2CountReadsTheTotalFromTheHeader(t *testing.T) {
	for _, count := range v2Counts {
		t.Run(count.name, func(t *testing.T) {
			got, err := count.count(t, totalCountServer(t, "42", http.StatusOK))
			if err != nil {
				t.Fatalf("Count: %v", err)
			}

			if got != 42 {
				t.Fatalf("count = %d, want 42", got)
			}
		})
	}
}

// TestV2CountRefusesAnUnreadableTotal is the half that keeps the count honest.
//
// A count is used to decide whether a page walk is finished and whether a
// migration moved everything. A silent zero from a missing or malformed header
// reads as "there is nothing here" — the same answer as a genuinely empty
// ledger, and the reason a caller stops looking.
func TestV2CountRefusesAnUnreadableTotal(t *testing.T) {
	headers := []struct {
		name  string
		value string
	}{
		{name: "header absent", value: ""},
		{name: "header blank", value: "   "},
		{name: "header not a number", value: "many"},
		{name: "header negative", value: "-1"},
	}

	for _, count := range v2Counts {
		t.Run(count.name, func(t *testing.T) {
			for _, header := range headers {
				t.Run(header.name, func(t *testing.T) {
					got, err := count.count(t, totalCountServer(t, header.value, http.StatusOK))
					if err == nil {
						t.Fatalf("an unreadable total must not read as %d", got)
					}
				})
			}
		})
	}
}

// totalCountServer answers every request with status and, when value is not
// empty, an X-Total-Count header carrying it.
func totalCountServer(t *testing.T, value string, status int) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if value != "" {
			w.Header().Set(HeaderTotalCount, value)
		}

		w.WriteHeader(status)
	}))

	t.Cleanup(srv.Close)

	return srv
}

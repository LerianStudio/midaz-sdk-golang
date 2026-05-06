package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountsListOpts_ToQueryParams_TypedFilters(t *testing.T) {
	tests := []struct {
		name string
		opts AccountsListOpts
		want map[string]string
	}{
		{
			name: "non-trivial account filters include true booleans",
			opts: AccountsListOpts{
				PageListOpts: PageListOpts{
					Limit:         50,
					Page:          3,
					SortDirection: SortAscending,
					StartDate:     "2026-01-01",
					EndDate:       "2026-01-31",
				},
				Filters: AccountsFilters{
					Type:            "deposit",
					Status:          "ACTIVE",
					AssetCode:       "BRL",
					HolderID:        "holder-123",
					PortfolioID:     "portfolio-123",
					SegmentID:       "segment-123",
					Alias:           "main-cash",
					ParentAccountID: "parent-123",
					Name:            "Cash",
					EntityID:        "entity-123",
					IncludeDeleted:  true,
					Blocked:         true,
				},
			},
			want: map[string]string{
				"limit":             "50",
				"page":              "3",
				"sort_order":        "asc",
				"start_date":        "2026-01-01",
				"end_date":          "2026-01-31",
				"type":              "deposit",
				"status":            "ACTIVE",
				"asset_code":        "BRL",
				"holder_id":         "holder-123",
				"portfolio_id":      "portfolio-123",
				"segment_id":        "segment-123",
				"alias":             "main-cash",
				"parent_account_id": "parent-123",
				"name":              "Cash",
				"entity_id":         "entity-123",
				"include_deleted":   "true",
				"blocked":           "true",
			},
		},
		{
			name: "false boolean filters are omitted",
			opts: AccountsListOpts{
				Filters: AccountsFilters{IncludeDeleted: false, Blocked: false},
			},
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.opts.Validate())
			assert.Equal(t, tt.want, tt.opts.ToQueryParams())
		})
	}
}

func TestPortfoliosListOpts_ToQueryParams_TypedFilters(t *testing.T) {
	tests := []struct {
		name string
		opts PortfoliosListOpts
		want map[string]string
	}{
		{
			name: "portfolio filters include typed fields and shared page params",
			opts: PortfoliosListOpts{
				PageListOpts: PageListOpts{Limit: 20, Page: 2, SortDirection: SortDescending},
				Filters: PortfoliosFilters{
					Name:           "Treasury",
					EntityID:       "entity-123",
					Status:         "ACTIVE",
					IncludeDeleted: true,
				},
			},
			want: map[string]string{
				"limit":           "20",
				"page":            "2",
				"sort_order":      "desc",
				"name":            "Treasury",
				"entity_id":       "entity-123",
				"status":          "ACTIVE",
				"include_deleted": "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.opts.Validate())
			assert.Equal(t, tt.want, tt.opts.ToQueryParams())
		})
	}
}

func TestPageQueryParams_EmitsOnlyServerSupportedKeys(t *testing.T) {
	opts := PageListOpts{
		Limit:         10,
		Page:          2,
		SortDirection: SortDescending,
	}

	params := PageQueryParams(opts)

	assert.Equal(t, map[string]string{
		"limit":      "10",
		"page":       "2",
		"sort_order": "desc",
	}, params)
}

// TestValidatePageListOpts_DateRange covers M22: SDK-side validation of
// the StartDate/EndDate pair. Malformed dates must be rejected with a
// typed validation error before any HTTP request is issued, and an
// inverted range (start > end) must be flagged as well.
func TestValidatePageListOpts_DateRange(t *testing.T) {
	tests := []struct {
		name    string
		opts    PageListOpts
		wantErr string
	}{
		{name: "no dates is valid", opts: PageListOpts{}},
		{name: "start only is valid", opts: PageListOpts{StartDate: "2026-01-01"}},
		{name: "end only is valid", opts: PageListOpts{EndDate: "2026-12-31"}},
		{name: "valid range", opts: PageListOpts{StartDate: "2026-01-01", EndDate: "2026-12-31"}},
		{name: "equal range is valid", opts: PageListOpts{StartDate: "2026-06-15", EndDate: "2026-06-15"}},
		{name: "malformed start date", opts: PageListOpts{StartDate: "2026-13-50"}, wantErr: "start date must be YYYY-MM-DD"},
		{name: "malformed end date", opts: PageListOpts{EndDate: "not-a-date"}, wantErr: "end date must be YYYY-MM-DD"},
		{name: "RFC3339 is rejected (must be YYYY-MM-DD)", opts: PageListOpts{StartDate: "2026-01-01T00:00:00Z"}, wantErr: "start date must be YYYY-MM-DD"},
		{name: "inverted range", opts: PageListOpts{StartDate: "2026-12-31", EndDate: "2026-01-01"}, wantErr: "start date must be on or before end date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePageListOpts("test", tt.opts)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestValidateCursorListOpts_DateRange mirrors TestValidatePageListOpts_DateRange
// for the cursor-paginated shape — same rules, parallel coverage so a future
// drift between the two validators surfaces as a test failure.
func TestValidateCursorListOpts_DateRange(t *testing.T) {
	tests := []struct {
		name    string
		opts    CursorListOpts
		wantErr string
	}{
		{name: "no dates is valid", opts: CursorListOpts{}},
		{name: "valid range", opts: CursorListOpts{StartDate: "2026-01-01", EndDate: "2026-12-31"}},
		{name: "malformed start date", opts: CursorListOpts{StartDate: "2026-13-50"}, wantErr: "start date must be YYYY-MM-DD"},
		{name: "malformed end date", opts: CursorListOpts{EndDate: "tomorrow"}, wantErr: "end date must be YYYY-MM-DD"},
		{name: "inverted range", opts: CursorListOpts{StartDate: "2026-12-31", EndDate: "2026-01-01"}, wantErr: "start date must be on or before end date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCursorListOpts("test", tt.opts)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestAssetRatesListOpts_EmbedsCursorListOpts is the M20 shape-consistency
// regression. AssetRatesListOpts must embed CursorListOpts (matching the
// other 4 cursor-paginated opts shapes — Transactions, Operations,
// OperationRoutes, TransactionRoutes) rather than duplicate the cursor
// fields at the top level. Asserts both compile-time field promotion and
// validate-time date-range enforcement.
func TestAssetRatesListOpts_EmbedsCursorListOpts(t *testing.T) {
	t.Run("CursorListOpts is reachable via field promotion", func(t *testing.T) {
		opts := AssetRatesListOpts{
			CursorListOpts: CursorListOpts{
				Limit:         50,
				Cursor:        "abc",
				SortDirection: SortDescending,
				StartDate:     "2026-01-01",
				EndDate:       "2026-12-31",
			},
			Filters: AssetRatesFilters{To: []string{"BRL"}},
		}

		// Promoted-field access compiles and round-trips.
		assert.Equal(t, 50, opts.Limit)
		assert.Equal(t, "abc", opts.Cursor)
		assert.Equal(t, SortDescending, opts.SortDirection)
		assert.Equal(t, "2026-01-01", opts.StartDate)
		assert.Equal(t, "2026-12-31", opts.EndDate)
	})

	t.Run("Validate rejects malformed dates", func(t *testing.T) {
		opts := AssetRatesListOpts{
			CursorListOpts: CursorListOpts{StartDate: "2026-13-50"},
		}
		require.ErrorContains(t, opts.Validate(), "start date must be YYYY-MM-DD")
	})

	t.Run("Validate rejects inverted date range", func(t *testing.T) {
		opts := AssetRatesListOpts{
			CursorListOpts: CursorListOpts{StartDate: "2026-12-31", EndDate: "2026-01-01"},
		}
		require.ErrorContains(t, opts.Validate(), "start date must be on or before end date")
	})

	t.Run("dates emit on the wire as start_date / end_date", func(t *testing.T) {
		opts := AssetRatesListOpts{
			CursorListOpts: CursorListOpts{StartDate: "2026-01-01", EndDate: "2026-12-31"},
		}
		params := opts.ToQueryParams()
		assert.Equal(t, "2026-01-01", params["start_date"])
		assert.Equal(t, "2026-12-31", params["end_date"])
	})
}

// TestValidatePageListOpts_LimitPageSort covers the H23 finding: the
// non-date branches of ValidatePageListOpts (limit caps, negative page,
// invalid sort direction) must return typed validation errors before any
// HTTP request is issued. M22 added the date-range branch; this fills in
// the remaining branches for the central validator that backs 11 typed
// page-based ListOpts.
func TestValidatePageListOpts_LimitPageSort(t *testing.T) {
	tests := []struct {
		name    string
		opts    PageListOpts
		wantErr string
	}{
		{name: "zero values are valid", opts: PageListOpts{}},
		{name: "limit at max is valid", opts: PageListOpts{Limit: MaxLimit}},
		{name: "limit one over max", opts: PageListOpts{Limit: MaxLimit + 1}, wantErr: "limit exceeds maximum"},
		{name: "limit way over max", opts: PageListOpts{Limit: 100000}, wantErr: "limit exceeds maximum"},
		{name: "negative limit", opts: PageListOpts{Limit: -1}, wantErr: "limit must be non-negative"},
		{name: "negative page", opts: PageListOpts{Page: -1}, wantErr: "page must be non-negative"},
		{name: "page=0 is valid (server treats as 1)", opts: PageListOpts{Page: 0}},
		{name: "asc sort is valid", opts: PageListOpts{SortDirection: SortAscending}},
		{name: "desc sort is valid", opts: PageListOpts{SortDirection: SortDescending}},
		{name: "empty sort is valid", opts: PageListOpts{SortDirection: ""}},
		{name: "ASC uppercase is rejected", opts: PageListOpts{SortDirection: "ASC"}, wantErr: "sort direction must be empty"},
		{name: "garbage sort direction", opts: PageListOpts{SortDirection: "weird"}, wantErr: "sort direction must be empty"},
		{name: "combined valid case", opts: PageListOpts{Limit: 50, Page: 3, SortDirection: SortDescending, StartDate: "2026-01-01", EndDate: "2026-12-31"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePageListOpts("test", tt.opts)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestValidateCursorListOpts_LimitCursorSort mirrors
// TestValidatePageListOpts_LimitPageSort for the cursor variant. Cursor
// opts has no Page field; it has a Cursor field instead, which is a free
// string with no SDK-side preconditions (server-issued opaque tokens).
// What we DO need to assert: empty cursor is valid (means "first page")
// and the limit/sort branches behave identically to the page validator.
func TestValidateCursorListOpts_LimitCursorSort(t *testing.T) {
	tests := []struct {
		name    string
		opts    CursorListOpts
		wantErr string
	}{
		{name: "zero values are valid", opts: CursorListOpts{}},
		{name: "empty cursor is valid", opts: CursorListOpts{Cursor: ""}},
		{name: "non-empty cursor is valid", opts: CursorListOpts{Cursor: "abc-token-123"}},
		{name: "limit at max is valid", opts: CursorListOpts{Limit: MaxLimit}},
		{name: "limit one over max", opts: CursorListOpts{Limit: MaxLimit + 1}, wantErr: "limit exceeds maximum"},
		{name: "negative limit", opts: CursorListOpts{Limit: -1}, wantErr: "limit must be non-negative"},
		{name: "asc sort is valid", opts: CursorListOpts{SortDirection: SortAscending}},
		{name: "desc sort is valid", opts: CursorListOpts{SortDirection: SortDescending}},
		{name: "garbage sort direction", opts: CursorListOpts{SortDirection: "weird"}, wantErr: "sort direction must be empty"},
		{name: "combined valid case", opts: CursorListOpts{Limit: 50, Cursor: "c1", SortDirection: SortDescending, StartDate: "2026-01-01", EndDate: "2026-12-31"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCursorListOpts("test", tt.opts)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestCursorQueryParams_EmitsOnlyServerSupportedKeys mirrors
// TestPageQueryParams_EmitsOnlyServerSupportedKeys for the cursor opts.
// Pinning the wire encoding so a future shape change surfaces here
// instead of as a server 4xx that nobody reads.
func TestCursorQueryParams_EmitsOnlyServerSupportedKeys(t *testing.T) {
	opts := CursorListOpts{
		Limit:         50,
		Cursor:        "next-token",
		SortDirection: SortAscending,
	}

	params := CursorQueryParams(opts)

	assert.Equal(t, map[string]string{
		"limit":      "50",
		"cursor":     "next-token",
		"sort_order": "asc",
	}, params)

	// Negative space: page and offset must NEVER appear on a cursor opts
	// — that was the v2 footgun audit finding 5.5.
	_, hasPage := params["page"]
	assert.False(t, hasPage, "cursor opts must not emit page= on the wire")

	_, hasOffset := params["offset"]
	assert.False(t, hasOffset, "cursor opts must not emit offset= on the wire")
}

// TestPageListOpts_TypedShape_AllPageBased covers H26: every typed
// page-based ListOpts must construct cleanly, validate cleanly when
// Limit is at the cap, and reject Limit=MaxLimit+1. This is parametrized
// across all 11 page-based opts so a regression in any of them surfaces
// here. It also doubles as the H29 limit-clamping coverage for the 10
// page-based entities not previously tested for over-limit behavior.
//
// The contract under test is: validators ERROR on Limit > MaxLimit (no
// silent clamp). If this changes to clamp-and-continue, this test will
// need to be inverted. That's deliberate — the surfacing matters.
func TestPageListOpts_TypedShape_AllPageBased(t *testing.T) {
	type validator interface {
		Validate() error
		ToQueryParams() map[string]string
	}

	tests := []struct {
		name    string
		atMax   validator
		overMax validator
	}{
		{
			name:    "AccountTypesListOpts",
			atMax:   AccountTypesListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}},
			overMax: AccountTypesListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "AccountsListOpts",
			atMax:   AccountsListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}},
			overMax: AccountsListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "AliasesListOpts",
			atMax:   AliasesListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}},
			overMax: AliasesListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "AssetsListOpts",
			atMax:   AssetsListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}},
			overMax: AssetsListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "BalancesListOpts",
			atMax:   BalancesListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}},
			overMax: BalancesListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "HoldersListOpts",
			atMax:   HoldersListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}},
			overMax: HoldersListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "LedgersListOpts",
			atMax:   LedgersListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}},
			overMax: LedgersListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "OrganizationsListOpts",
			atMax:   OrganizationsListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}},
			overMax: OrganizationsListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "PortfoliosListOpts",
			atMax:   PortfoliosListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}},
			overMax: PortfoliosListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "SegmentsListOpts",
			atMax:   SegmentsListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}},
			overMax: SegmentsListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.atMax.Validate(), "Limit=MaxLimit must be accepted")
			assert.Equal(t, "100", tt.atMax.ToQueryParams()["limit"], "Limit=MaxLimit must round-trip on the wire")

			err := tt.overMax.Validate()
			require.Error(t, err, "Limit=MaxLimit+1 must be rejected")
			assert.Contains(t, err.Error(), "limit exceeds maximum")
		})
	}
}

// TestCursorListOpts_TypedShape_AllCursorBased mirrors
// TestPageListOpts_TypedShape_AllPageBased for the 5 cursor-based opts.
// AssetRatesListOpts is already covered by an existing test in this
// file; including it here ensures the shape-consistency regression
// stays intact.
func TestCursorListOpts_TypedShape_AllCursorBased(t *testing.T) {
	type validator interface {
		Validate() error
		ToQueryParams() map[string]string
	}

	tests := []struct {
		name    string
		atMax   validator
		overMax validator
	}{
		{
			name:    "AssetRatesListOpts",
			atMax:   AssetRatesListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit}},
			overMax: AssetRatesListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "OperationsListOpts",
			atMax:   OperationsListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit}},
			overMax: OperationsListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "OperationRoutesListOpts",
			atMax:   OperationRoutesListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit}},
			overMax: OperationRoutesListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "TransactionsListOpts",
			atMax:   TransactionsListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit}},
			overMax: TransactionsListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit + 1}},
		},
		{
			name:    "TransactionRoutesListOpts",
			atMax:   TransactionRoutesListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit}},
			overMax: TransactionRoutesListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit + 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.atMax.Validate(), "Limit=MaxLimit must be accepted")
			assert.Equal(t, "100", tt.atMax.ToQueryParams()["limit"], "Limit=MaxLimit must round-trip on the wire")
			// Cursor opts must NEVER emit page on the wire, regardless of opts.
			_, hasPage := tt.atMax.ToQueryParams()["page"]
			assert.False(t, hasPage, "cursor-based opts must not emit page=")

			err := tt.overMax.Validate()
			require.Error(t, err, "Limit=MaxLimit+1 must be rejected")
			assert.Contains(t, err.Error(), "limit exceeds maximum")
		})
	}
}

// TestTypedListOpts_ToQueryParams_FilterEncoding pins the wire encoding
// of every typed ListOpts' Filter sub-struct. Each entity's filters are
// per-endpoint (replaces v2's mega-struct ListOptions per audit finding
// 5.12), so a regression in a single Filter→param mapping surfaces here.
//
// Boolean false filters MUST be omitted (existing pattern in
// TestAccountsListOpts_ToQueryParams_TypedFilters); empty strings MUST
// be omitted; populated fields MUST round-trip exactly.
func TestTypedListOpts_ToQueryParams_FilterEncoding(t *testing.T) {
	t.Run("AccountTypesListOpts filters", func(t *testing.T) {
		opts := AccountTypesListOpts{
			PageListOpts: PageListOpts{Limit: 25},
			Filters: AccountTypesFilters{
				Name:           "Deposit",
				KeyValue:       "deposit",
				IncludeDeleted: true,
			},
		}
		require.NoError(t, opts.Validate())
		params := opts.ToQueryParams()
		assert.Equal(t, "25", params["limit"])
		assert.Equal(t, "Deposit", params["name"])
		assert.Equal(t, "deposit", params["key_value"])
		assert.Equal(t, "true", params["include_deleted"])
	})

	t.Run("AliasesListOpts filters", func(t *testing.T) {
		opts := AliasesListOpts{
			Filters: AliasesFilters{
				HolderID:  "h-123",
				AccountID: "a-456",
			},
		}
		require.NoError(t, opts.Validate())
		params := opts.ToQueryParams()
		assert.Equal(t, "h-123", params["holder_id"])
		assert.Equal(t, "a-456", params["account_id"])
	})

	t.Run("AssetsListOpts filters", func(t *testing.T) {
		opts := AssetsListOpts{
			PageListOpts: PageListOpts{Limit: 30},
			Filters: AssetsFilters{
				Code:   "USD",
				Type:   "currency",
				Status: "ACTIVE",
			},
		}
		require.NoError(t, opts.Validate())
		params := opts.ToQueryParams()
		assert.Equal(t, "30", params["limit"])
		assert.Equal(t, "USD", params["code"])
		assert.Equal(t, "currency", params["type"])
		assert.Equal(t, "ACTIVE", params["status"])
	})

	t.Run("BalancesListOpts filters", func(t *testing.T) {
		opts := BalancesListOpts{
			Filters: BalancesFilters{
				AssetCode: "BRL",
				AccountID: "acc-1",
				Status:    "ACTIVE",
			},
		}
		require.NoError(t, opts.Validate())
		params := opts.ToQueryParams()
		assert.Equal(t, "BRL", params["asset_code"])
		assert.Equal(t, "acc-1", params["account_id"])
		assert.Equal(t, "ACTIVE", params["status"])
	})

	t.Run("HoldersListOpts filters", func(t *testing.T) {
		opts := HoldersListOpts{
			Filters: HoldersFilters{
				Name:           "Alice",
				Document:       "123456789",
				Status:         "ACTIVE",
				ExternalID:     "ext-1",
				IncludeDeleted: true,
			},
		}
		require.NoError(t, opts.Validate())
		params := opts.ToQueryParams()
		assert.Equal(t, "Alice", params["name"])
		assert.Equal(t, "123456789", params["document"])
		assert.Equal(t, "ACTIVE", params["status"])
		assert.Equal(t, "ext-1", params["external_id"])
		assert.Equal(t, "true", params["include_deleted"])
	})

	t.Run("LedgersListOpts filters", func(t *testing.T) {
		opts := LedgersListOpts{
			Filters: LedgersFilters{
				Name:           "Main Ledger",
				Status:         "ACTIVE",
				IncludeDeleted: true,
			},
		}
		require.NoError(t, opts.Validate())
		params := opts.ToQueryParams()
		assert.Equal(t, "Main Ledger", params["name"])
		assert.Equal(t, "ACTIVE", params["status"])
		assert.Equal(t, "true", params["include_deleted"])
	})

	t.Run("OrganizationsListOpts filters", func(t *testing.T) {
		opts := OrganizationsListOpts{
			Filters: OrganizationsFilters{
				LegalName:      "Lerian Studio",
				Status:         "ACTIVE",
				IncludeDeleted: true,
			},
		}
		require.NoError(t, opts.Validate())
		params := opts.ToQueryParams()
		assert.Equal(t, "Lerian Studio", params["legal_name"])
		assert.Equal(t, "ACTIVE", params["status"])
		assert.Equal(t, "true", params["include_deleted"])
	})

	t.Run("SegmentsListOpts filters", func(t *testing.T) {
		opts := SegmentsListOpts{
			Filters: SegmentsFilters{
				Name:           "VIP",
				Status:         "ACTIVE",
				IncludeDeleted: true,
			},
		}
		require.NoError(t, opts.Validate())
		params := opts.ToQueryParams()
		assert.Equal(t, "VIP", params["name"])
		assert.Equal(t, "ACTIVE", params["status"])
		assert.Equal(t, "true", params["include_deleted"])
	})

	t.Run("OperationsListOpts filters", func(t *testing.T) {
		opts := OperationsListOpts{
			CursorListOpts: CursorListOpts{Limit: 20, Cursor: "c1"},
			Filters: OperationsFilters{
				Type:      "DEBIT",
				AssetCode: "USD",
				Status:    "ACTIVE",
			},
		}
		require.NoError(t, opts.Validate())
		params := opts.ToQueryParams()
		assert.Equal(t, "20", params["limit"])
		assert.Equal(t, "c1", params["cursor"])
		assert.Equal(t, "DEBIT", params["type"])
		assert.Equal(t, "USD", params["asset_code"])
		assert.Equal(t, "ACTIVE", params["status"])
	})

	t.Run("OperationRoutesListOpts filters", func(t *testing.T) {
		opts := OperationRoutesListOpts{
			CursorListOpts: CursorListOpts{Limit: 20},
			Filters: OperationRoutesFilters{
				Name:          "cashin",
				Status:        "ACTIVE",
				OperationType: "DEBIT",
			},
		}
		require.NoError(t, opts.Validate())
		params := opts.ToQueryParams()
		assert.Equal(t, "20", params["limit"])
		assert.Equal(t, "cashin", params["name"])
		assert.Equal(t, "ACTIVE", params["status"])
		assert.Equal(t, "DEBIT", params["operation_type"])
	})

	t.Run("TransactionsListOpts filters", func(t *testing.T) {
		opts := TransactionsListOpts{
			CursorListOpts: CursorListOpts{Limit: 50},
			Filters: TransactionsFilters{
				AssetCode:          "EUR",
				Status:             "COMPLETED",
				Reference:          "ref-9",
				DestinationAccount: "dst",
				SourceAccount:      "src",
				Route:              "cashout",
			},
		}
		require.NoError(t, opts.Validate())
		params := opts.ToQueryParams()
		assert.Equal(t, "EUR", params["asset_code"])
		assert.Equal(t, "COMPLETED", params["status"])
		assert.Equal(t, "ref-9", params["reference"])
		assert.Equal(t, "dst", params["destination_account"])
		assert.Equal(t, "src", params["source_account"])
		assert.Equal(t, "cashout", params["route"])
	})

	t.Run("TransactionRoutesListOpts filters", func(t *testing.T) {
		opts := TransactionRoutesListOpts{
			CursorListOpts: CursorListOpts{Limit: 25},
			Filters: TransactionRoutesFilters{
				Name:             "transfer",
				Status:           "ACTIVE",
				OperationRouteID: "or-1",
			},
		}
		require.NoError(t, opts.Validate())
		params := opts.ToQueryParams()
		assert.Equal(t, "25", params["limit"])
		assert.Equal(t, "transfer", params["name"])
		assert.Equal(t, "ACTIVE", params["status"])
		assert.Equal(t, "or-1", params["operation_route_id"])
	})
}

// TestUpdateInputMarshalJSON_NilPointersReturnNull is the M31 nil-safety
// smoke test for the four Update*Input types whose MarshalJSON receivers
// were migrated from value to pointer (account, alias, holder,
// operation_route). Marshaling a nil pointer must round-trip as JSON
// "null" instead of panicking.
func TestUpdateInputMarshalJSON_NilPointersReturnNull(t *testing.T) {
	t.Run("UpdateAliasInput nil pointer", func(t *testing.T) {
		var input *UpdateAliasInput

		got, err := json.Marshal(input)
		require.NoError(t, err)
		assert.Equal(t, "null", string(got))
	})

	t.Run("UpdateHolderInput nil pointer", func(t *testing.T) {
		var input *UpdateHolderInput

		got, err := json.Marshal(input)
		require.NoError(t, err)
		assert.Equal(t, "null", string(got))
	})

	t.Run("UpdateOperationRouteInput nil pointer", func(t *testing.T) {
		var input *UpdateOperationRouteInput

		got, err := json.Marshal(input)
		require.NoError(t, err)
		assert.Equal(t, "null", string(got))
	})

	// UpdateAccountInput nil pointer is already covered by
	// TestUpdateAccountInputMarshalJSONNilPointer in account_test.go.
}

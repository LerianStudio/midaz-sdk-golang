package models

import (
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
					OrderBy:       "createdAt",
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

func TestPageListOpts_OrderByRetainedButNotEmitted(t *testing.T) {
	opts := PageListOpts{
		Limit:         10,
		Page:          2,
		OrderBy:       "createdAt",
		SortDirection: SortDescending,
	}

	params := PageQueryParams(opts)

	assert.Equal(t, "createdAt", opts.OrderBy)
	assert.Equal(t, map[string]string{
		"limit":      "10",
		"page":       "2",
		"sort_order": "desc",
	}, params)
	assert.NotContains(t, params, QueryParamOrderBy)
}

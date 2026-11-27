// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
)

type accountPlanListOpts interface {
	Validate() error
	ToQueryParams() map[string]string
}

func TestAccountPlanListOpts_ToQueryParams(t *testing.T) {
	disabled := false

	tests := []struct {
		name string
		opts accountPlanListOpts
		want map[string]string
	}{
		{"plans without options", AccountPlansListOpts{}, map[string]string{}},
		{
			"plans with limit, page and enable false",
			AccountPlansListOpts{PageListOpts: PageListOpts{Limit: 5, Page: 2}, Filters: AccountPlansFilters{Enable: &disabled}},
			map[string]string{"limit": "5", "page": "2", "enable": "false"},
		},
		{"versions without options", AccountPlanVersionsListOpts{}, map[string]string{}},
		{
			"versions with limit and page",
			AccountPlanVersionsListOpts{PageListOpts: PageListOpts{Limit: 10, Page: 3}},
			map[string]string{"limit": "10", "page": "3"},
		},
		{"plan accounts without options", AccountPlanAccountsListOpts{}, map[string]string{}},
		{
			"plan accounts sorted descending",
			AccountPlanAccountsListOpts{PageListOpts: PageListOpts{Limit: 20, Page: 1, SortDirection: SortDescending}},
			map[string]string{"limit": "20", "page": "1", "sort_order": "desc"},
		},
		{"usage without period", AccountPlanUsageOpts{}, map[string]string{}},
		{"usage with period", AccountPlanUsageOpts{Period: "2026-10"}, map[string]string{"period": "2026-10"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.opts.ToQueryParams())
		})
	}
}

func TestAccountPlanListOpts_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    accountPlanListOpts
		wantErr string
	}{
		{"plans zero value", AccountPlansListOpts{}, ""},
		{"plans limit at max", AccountPlansListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}}, ""},
		{"plans limit over max", AccountPlansListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}}, "limit exceeds maximum"},
		{"plans negative limit", AccountPlansListOpts{PageListOpts: PageListOpts{Limit: -1}}, "limit must be non-negative"},
		{"plans negative page", AccountPlansListOpts{PageListOpts: PageListOpts{Page: -1}}, "page must be non-negative"},
		{"versions limit at max", AccountPlanVersionsListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}}, ""},
		{"versions limit over max", AccountPlanVersionsListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}}, "limit exceeds maximum"},
		{"versions negative page", AccountPlanVersionsListOpts{PageListOpts: PageListOpts{Page: -1}}, "page must be non-negative"},
		{"plan accounts limit at max", AccountPlanAccountsListOpts{PageListOpts: PageListOpts{Limit: MaxLimit}}, ""},
		{"plan accounts limit over max", AccountPlanAccountsListOpts{PageListOpts: PageListOpts{Limit: MaxLimit + 1}}, "limit exceeds maximum"},
		{"plan accounts negative page", AccountPlanAccountsListOpts{PageListOpts: PageListOpts{Page: -1}}, "page must be non-negative"},
		{"plan accounts unknown sort", AccountPlanAccountsListOpts{PageListOpts: PageListOpts{SortDirection: "up"}}, "sort direction must be empty"},
		{"usage empty period", AccountPlanUsageOpts{}, ""},
		{"usage valid period", AccountPlanUsageOpts{Period: "2026-10"}, ""},
		{"usage one-digit month", AccountPlanUsageOpts{Period: "2026-1"}, "period must be YYYY-MM"},
		{"usage month out of range", AccountPlanUsageOpts{Period: "2026-13"}, "period must be YYYY-MM"},
		{"usage full date", AccountPlanUsageOpts{Period: "2026-10-01"}, "period must be YYYY-MM"},
		{"usage garbage", AccountPlanUsageOpts{Period: "xx"}, "period must be YYYY-MM"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.True(t, sdkerrors.IsValidationError(err), "want a validation error, got %v", err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

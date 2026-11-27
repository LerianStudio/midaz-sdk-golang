// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"fmt"
	"strconv"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
)

// accountPlanUsagePeriodFormat is the YYYY-MM format of a usage period.
const accountPlanUsagePeriodFormat = "2006-01"

// AccountPlansListOpts is the typed options struct for listing account plans
// and their iterators.
//
// Account plans are PAGE-paginated (Page + Limit). The response total is the
// number of items on the page, not across pages, so iteration stops on the
// first page shorter than the limit rather than on Pagination.HasMore().
//
// AccountPlansListOpts is a value type. Concurrent-safe by construction — the
// entity layer never mutates a caller's opts.
type AccountPlansListOpts struct {
	PageListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters AccountPlansFilters
}

// AccountPlansFilters is the typed filter set for the account plans listing.
type AccountPlansFilters struct {
	// Enable narrows by the enabled flag. Nil means no narrowing.
	Enable *bool
}

// Validate enforces SDK-side preconditions on AccountPlansListOpts.
func (o AccountPlansListOpts) Validate() error {
	return ValidatePageListOpts("AccountPlansListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an AccountPlansListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it. Treat as an
// internal contract; not part of the user-facing API.
func (o AccountPlansListOpts) ToQueryParams() map[string]string {
	params := PageQueryParams(o.PageListOpts)

	if o.Filters.Enable != nil {
		params["enable"] = strconv.FormatBool(*o.Filters.Enable)
	}

	return params
}

// AccountPlanVersionsListOpts is the typed options struct for listing the
// versions of an account plan and their iterators. Versions are
// PAGE-paginated, and the response total is the number of items on the page.
//
// AccountPlanVersionsListOpts is a value type. Concurrent-safe by construction.
type AccountPlanVersionsListOpts struct {
	PageListOpts
}

// Validate enforces SDK-side preconditions on AccountPlanVersionsListOpts.
func (o AccountPlanVersionsListOpts) Validate() error {
	return ValidatePageListOpts("AccountPlanVersionsListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an AccountPlanVersionsListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it. Treat as an
// internal contract; not part of the user-facing API.
func (o AccountPlanVersionsListOpts) ToQueryParams() map[string]string {
	return PageQueryParams(o.PageListOpts)
}

// AccountPlanAccountsListOpts is the typed options struct for listing the
// account assignments of an account plan and their iterators. The listing is
// PAGE-paginated and carries no total. SortDirection orders by assignment
// start and is sent as sort_order.
//
// AccountPlanAccountsListOpts is a value type. Concurrent-safe by construction.
type AccountPlanAccountsListOpts struct {
	PageListOpts
}

// Validate enforces SDK-side preconditions on AccountPlanAccountsListOpts.
func (o AccountPlanAccountsListOpts) Validate() error {
	return ValidatePageListOpts("AccountPlanAccountsListOpts.Validate", o.PageListOpts)
}

// ToQueryParams renders an AccountPlanAccountsListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it. Treat as an
// internal contract; not part of the user-facing API.
func (o AccountPlanAccountsListOpts) ToQueryParams() map[string]string {
	return PageQueryParams(o.PageListOpts)
}

// AccountPlanUsageOpts selects the period of an account plan usage read.
//
// AccountPlanUsageOpts is a value type. Concurrent-safe by construction.
type AccountPlanUsageOpts struct {
	// Period is the period to read, in the YYYY-MM format and the plan
	// timezone. Empty reads the current period; the server refuses a future one.
	Period string
}

// Validate refuses a Period that is not in the YYYY-MM format.
func (o AccountPlanUsageOpts) Validate() error {
	if o.Period == "" {
		return nil
	}

	if _, err := time.Parse(accountPlanUsagePeriodFormat, o.Period); err != nil {
		return errors.NewValidationError("AccountPlanUsageOpts.Validate",
			"period must be YYYY-MM",
			fmt.Errorf("got %q", o.Period))
	}

	return nil
}

// ToQueryParams renders an AccountPlanUsageOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it. Treat as an
// internal contract; not part of the user-facing API.
func (o AccountPlanUsageOpts) ToQueryParams() map[string]string {
	params := make(map[string]string)

	if o.Period != "" {
		params["period"] = o.Period
	}

	return params
}

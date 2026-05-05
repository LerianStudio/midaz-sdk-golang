package entities

import (
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
)

// cursorListQueryParams renders pagination/filtering options for endpoints
// that ONLY accept cursor-based pagination (currently: operations,
// transactions, operation_routes, transaction_routes). Page-based
// pagination fields (Page, Offset) on the input are ignored — they make
// no semantic sense on these endpoints.
//
// v3 contract: callers should run ListOptions.Validate() before invoking
// list methods. Track 5 of the v3-dx plan redesigns pagination into
// per-service typed ListOpts, which makes invalid combinations
// uncompilable rather than silently dropped.
//
// Intentionally separate from ListOptions.ToQueryParams (which is for
// page-based endpoints): the two contracts are incompatible.
func cursorListQueryParams(opts *models.ListOptions) map[string]string {
	params := map[string]string{}
	if opts == nil {
		return params
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = models.DefaultLimit
	} else if limit > models.MaxLimit {
		limit = models.MaxLimit
	}

	params[models.QueryParamLimit] = fmt.Sprintf("%d", limit)

	if opts.Cursor != "" {
		params[models.QueryParamCursor] = opts.Cursor
	}

	if opts.StartDate != "" {
		params[models.QueryParamStartDate] = opts.StartDate
	}

	if opts.EndDate != "" {
		params[models.QueryParamEndDate] = opts.EndDate
	}

	params[models.QueryParamOrderDirection] = cursorOrderDirection(opts.OrderDirection)

	for key, value := range opts.Filters {
		if value != "" {
			params[key] = value
		}
	}

	for key, value := range opts.AdditionalParams {
		if value == "" || isReservedCursorQueryParam(key) {
			continue
		}

		params[key] = value
	}

	return params
}

func cursorOrderDirection(direction string) string {
	switch direction {
	case "":
		return models.DefaultSortDirection
	case string(models.SortAscending), string(models.SortDescending):
		return direction
	default:
		return models.DefaultSortDirection
	}
}

func isReservedCursorQueryParam(key string) bool {
	switch key {
	case models.QueryParamLimit,
		models.QueryParamPage,
		models.QueryParamOffset,
		models.QueryParamCursor,
		models.QueryParamOrderBy,
		models.QueryParamOrderDirection,
		models.QueryParamStartDate,
		models.QueryParamEndDate:
		return true
	default:
		return false
	}
}

package entities

import (
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
)

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
		params["start_date"] = opts.StartDate
	}

	if opts.EndDate != "" {
		params["end_date"] = opts.EndDate
	}

	if opts.OrderDirection != "" {
		params[models.QueryParamOrderDirection] = opts.OrderDirection
	} else {
		params[models.QueryParamOrderDirection] = models.DefaultSortDirection
	}

	for key, value := range opts.Filters {
		if value != "" {
			params[key] = value
		}
	}

	for key, value := range opts.AdditionalParams {
		if value != "" {
			params[key] = value
		}
	}

	return params
}

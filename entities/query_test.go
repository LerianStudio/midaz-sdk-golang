package entities

import (
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/stretchr/testify/require"
)

func TestCursorListQueryParamsClampsInvalidOrderDirection(t *testing.T) {
	params := cursorListQueryParams(&models.ListOptions{OrderDirection: "sideways"})

	require.Equal(t, models.DefaultSortDirection, params[models.QueryParamOrderDirection])
}

func TestCursorListQueryParamsAllowsKnownOrderDirections(t *testing.T) {
	for _, direction := range []string{string(models.SortAscending), string(models.SortDescending)} {
		params := cursorListQueryParams(&models.ListOptions{OrderDirection: direction})
		require.Equal(t, direction, params[models.QueryParamOrderDirection])
	}
}

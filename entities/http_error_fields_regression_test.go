package entities

import (
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPErrorFieldsParseErrorResponseAcceptsMidazFieldsObject(t *testing.T) {
	body := []byte(`{
		"code":"ERR_INVALID_INPUT",
		"title":"Bad Request",
		"message":"validation failed",
		"entityType":"Account",
		"fields":{"type":"must not be external"}
	}`)

	err := (*HTTPClient)(nil).parseErrorResponse(400, body, "req-123")
	require.Error(t, err)

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Equal(t, "ERR_INVALID_INPUT", sdkErr.APICode)
	assert.Equal(t, "Bad Request", sdkErr.Title)
	assert.Equal(t, "Account", sdkErr.EntityType)
	assert.Contains(t, sdkErr.Fields, "type")
	assert.Contains(t, sdkErr.Details, "fields")
}

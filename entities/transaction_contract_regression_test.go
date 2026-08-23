package entities

import (
	"context"
	"net/http"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionContractUpdatePayloadRejectsTypedNil(t *testing.T) {
	var update *models.UpdateOperationInput

	svc := newOperationsEntity(nil, "token", map[string]string{"transaction": "https://api.example.com"})

	_, err := svc.UpdateTransactionOperation(context.Background(), "org", "ledger", "tx", "op", update)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "input")
}

func TestTransactionContractParseErrorResponse_RedactsTextualEnvelopeFields(t *testing.T) {
	body := []byte(`{"message":"document=12345678900","title":"metadata.secret=value","fields":["external_id=abc"]}`)

	err := (*HTTPClient)(nil).parseErrorResponse(http.StatusBadRequest, body, "req-1")
	require.Error(t, err)

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.NotContains(t, sdkErr.Message, "12345678900")
	assert.NotContains(t, sdkErr.Title, "value")
	require.Len(t, sdkErr.Fields, 1)
	assert.NotContains(t, sdkErr.Fields[0], "abc")
}

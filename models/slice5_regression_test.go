package models

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlice5RouteAndAssetRateValidators_NilSafe(t *testing.T) {
	var (
		createOperationRoute   *CreateOperationRouteInput
		updateOperationRoute   *UpdateOperationRouteInput
		createTransactionRoute *CreateTransactionRouteInput
		updateTransactionRoute *UpdateTransactionRouteInput
		createAssetRate        *CreateAssetRateInput
	)

	require.Error(t, createOperationRoute.Validate())
	require.Error(t, updateOperationRoute.Validate())
	require.Error(t, createTransactionRoute.Validate())
	require.Error(t, updateTransactionRoute.Validate())
	require.Error(t, createAssetRate.Validate())

	require.Nil(t, createOperationRoute.WithMetadata(map[string]any{"k": "v"}))
	require.Nil(t, updateOperationRoute.WithTitle("title"))
	require.Nil(t, createTransactionRoute.WithMetadata(map[string]any{"k": "v"}))
	require.Nil(t, updateTransactionRoute.WithMetadata(map[string]any{"k": "v"}))
	require.Nil(t, createAssetRate.WithScale(2))
}

func TestSlice5RouteValidators_ContractLimitsAndMetadata(t *testing.T) {
	const (
		expectedRouteTitleLength       = 255
		expectedRouteDescriptionLength = 250
	)

	assert.Equal(t, expectedRouteTitleLength, maxRouteTitleLength)
	assert.Equal(t, expectedRouteDescriptionLength, maxRouteDescriptionLength)

	operationRoute := NewCreateOperationRouteInput(strings.Repeat("a", expectedRouteTitleLength+1), "desc", "source")
	require.ErrorContains(t, operationRoute.Validate(), "title")

	operationRoute = NewCreateOperationRouteInput("title", "desc", "debit")
	require.ErrorContains(t, operationRoute.Validate(), "operationType")

	operationRoute = NewCreateOperationRouteInput("title", "desc", "source").WithMetadata(map[string]any{strings.Repeat("m", 101): "value"})
	require.ErrorContains(t, operationRoute.Validate(), "metadata")

	operationRoute = NewCreateOperationRouteInput(strings.Repeat("\u00e9", expectedRouteTitleLength), "desc", "source")
	require.NoError(t, operationRoute.Validate())

	operationRoute = NewCreateOperationRouteInput(strings.Repeat("\u00e9", expectedRouteTitleLength+1), "desc", "source")
	require.ErrorContains(t, operationRoute.Validate(), "title")

	txRoute := NewCreateTransactionRouteInput("title", strings.Repeat("d", expectedRouteDescriptionLength+1), []string{uuid.NewString()})
	require.ErrorContains(t, txRoute.Validate(), "description")

	txRoute = NewCreateTransactionRouteInput("title", "desc", []string{"not-a-uuid"})
	require.ErrorContains(t, txRoute.Validate(), "operationRoutes")
}

func TestSlice5UpdateOperationRouteInput_AccountingEntriesRawNullIsChange(t *testing.T) {
	input := &UpdateOperationRouteInput{}
	input.AccountingEntriesRaw = json.RawMessage("null")

	require.NoError(t, input.Validate())

	data, err := json.Marshal(input)
	require.NoError(t, err)
	assert.JSONEq(t, `{"accountingEntries":null}`, string(data))
}

func TestSlice5AssetRateValidation(t *testing.T) {
	input := NewCreateAssetRateInput("USD", "BRL", 525).WithExternalID("not-a-uuid")
	require.ErrorContains(t, input.Validate(), "externalID")

	valid := NewCreateAssetRateInput("USD", "BRL", 525).
		WithExternalID(uuid.NewString()).
		WithMetadata(map[string]any{"provider": "central-bank"})
	require.NoError(t, valid.Validate())
}

// Note: the v2 AssetRateListOptions query-join coverage that was here
// has moved to TestAssetRatesListOpts_ToQueryParams in
// entities/asset_rates_test.go alongside the typed AssetRatesListOpts
// shape that replaced it (Track 5 Batch 5C).

func TestSlice5AnnotationRequiresSend(t *testing.T) {
	require.NoError(t, NewCreateAnnotationInput("note").Validate())
	require.NoError(t, NewCreateAnnotationInput("note", &SendInput{Asset: "USD", Value: "10", Source: &SourceInput{From: []FromToInput{{AccountAlias: "@a", Amount: AmountInput{Asset: "USD", Value: "10"}}}}, Distribute: &DistributeInput{To: []FromToInput{{AccountAlias: "@b", Amount: AmountInput{Asset: "USD", Value: "10"}}}}}).Validate())
}

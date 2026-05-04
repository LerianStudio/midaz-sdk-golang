package models

import (
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
	require.Nil(t, WithUpdateTransactionRouteMetadata(updateTransactionRoute, map[string]any{"k": "v"}))
	require.Nil(t, createAssetRate.WithScale(2))
}

func TestSlice5RouteValidators_ContractLimitsAndMetadata(t *testing.T) {
	operationRoute := NewCreateOperationRouteInput(strings.Repeat("a", maxRouteTitleLength+1), "desc", "source")
	require.ErrorContains(t, operationRoute.Validate(), "title")

	operationRoute = NewCreateOperationRouteInput("title", "desc", "debit")
	require.ErrorContains(t, operationRoute.Validate(), "operationType")

	operationRoute = NewCreateOperationRouteInput("title", "desc", "source").WithMetadata(map[string]any{strings.Repeat("m", 101): "value"})
	require.ErrorContains(t, operationRoute.Validate(), "metadata")

	operationRoute = NewCreateOperationRouteInput(strings.Repeat("\u00e9", maxRouteTitleLength), "desc", "source")
	require.NoError(t, operationRoute.Validate())

	operationRoute = NewCreateOperationRouteInput(strings.Repeat("\u00e9", maxRouteTitleLength+1), "desc", "source")
	require.ErrorContains(t, operationRoute.Validate(), "title")

	txRoute := NewCreateTransactionRouteInput("title", strings.Repeat("d", maxRouteDescriptionLength+1), []string{uuid.NewString()})
	require.ErrorContains(t, txRoute.Validate(), "description")

	txRoute = NewCreateTransactionRouteInput("title", "desc", []string{"not-a-uuid"})
	require.ErrorContains(t, txRoute.Validate(), "operationRoutes")
}

func TestSlice5AssetRateValidationAndQueryJoin(t *testing.T) {
	input := NewCreateAssetRateInput("USD", "BRL", 525).WithExternalID("not-a-uuid")
	require.ErrorContains(t, input.Validate(), "externalID")

	valid := NewCreateAssetRateInput("USD", "BRL", 525).
		WithExternalID(uuid.NewString()).
		WithMetadata(map[string]any{"provider": "central-bank"})
	require.NoError(t, valid.Validate())

	params := NewAssetRateListOptions().WithTo("BRL", "EUR", "USD").ToQueryParams()
	assert.Equal(t, "BRL,EUR,USD", params["to"])
}

func TestSlice5AnnotationRequiresSend(t *testing.T) {
	require.ErrorContains(t, NewCreateAnnotationInput("note").Validate(), "send is required")
	require.NoError(t, NewCreateAnnotationInput("note", &SendInput{Asset: "USD", Value: "10", Source: &SourceInput{From: []FromToInput{{AccountAlias: "@a", Amount: AmountInput{Asset: "USD", Value: "10"}}}}, Distribute: &DistributeInput{To: []FromToInput{{AccountAlias: "@b", Amount: AmountInput{Asset: "USD", Value: "10"}}}}}).Validate())
}

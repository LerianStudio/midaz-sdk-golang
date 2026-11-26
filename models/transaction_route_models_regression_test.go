package models

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionRouteModelsRouteAndAssetRateValidators_NilSafe(t *testing.T) {
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

func TestTransactionRouteModelsRouteValidators_ContractLimitsAndMetadata(t *testing.T) {
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

func TestTransactionRouteModelsUpdateOperationRouteInput_AccountingEntriesRawNullIsChange(t *testing.T) {
	input := &UpdateOperationRouteInput{}
	input.AccountingEntriesRaw = json.RawMessage("null")

	require.NoError(t, input.Validate())

	data, err := json.Marshal(input)
	require.NoError(t, err)
	assert.JSONEq(t, `{"accountingEntries":null}`, string(data))
}

// previousActionsAccountingEntriesJSON is the accountingEntries wire body for
// the six actions that predate block, unblock and crossLedger, with distinct
// rubric codes per action and side.
const previousActionsAccountingEntriesJSON = `{"direct":{"debit":{"code":"1001","description":"direct debit"},"credit":{"code":"2001","description":"direct credit"}},` +
	`"hold":{"debit":{"code":"1002","description":"hold debit"},"credit":{"code":"2002","description":"hold credit"}},` +
	`"commit":{"debit":{"code":"1005","description":"commit debit"},"credit":{"code":"2005","description":"commit credit"}},` +
	`"cancel":{"debit":{"code":"1006","description":"cancel debit"},"credit":{"code":"2006","description":"cancel credit"}},` +
	`"revert":{"debit":{"code":"1007","description":"revert debit"},"credit":{"code":"2007","description":"revert credit"}},` +
	`"overdraft":{"debit":{"code":"1008","description":"overdraft debit"},"credit":{"code":"2008","description":"overdraft credit"}}}`

func testAccountingEntry(debitCode, debitDescription, creditCode, creditDescription string) *AccountingEntry {
	return &AccountingEntry{
		Debit:  &AccountingRubric{Code: debitCode, Description: debitDescription},
		Credit: &AccountingRubric{Code: creditCode, Description: creditDescription},
	}
}

func previousActionsAccountingEntries() *AccountingEntries {
	return &AccountingEntries{
		Direct:    testAccountingEntry("1001", "direct debit", "2001", "direct credit"),
		Hold:      testAccountingEntry("1002", "hold debit", "2002", "hold credit"),
		Commit:    testAccountingEntry("1005", "commit debit", "2005", "commit credit"),
		Cancel:    testAccountingEntry("1006", "cancel debit", "2006", "cancel credit"),
		Revert:    testAccountingEntry("1007", "revert debit", "2007", "revert credit"),
		Overdraft: testAccountingEntry("1008", "overdraft debit", "2008", "overdraft credit"),
	}
}

func TestTransactionRouteModelsOperationRouteInputs_BodyWithoutNewActionsUnchanged(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name: "create without new actions",
			input: &CreateOperationRouteInput{
				Title:             "probe A",
				Description:       "probe route",
				Code:              "PRB-A",
				OperationType:     "bidirectional",
				AccountingEntries: previousActionsAccountingEntries(),
			},
			expected: `{"title":"probe A","description":"probe route","code":"PRB-A","operationType":"bidirectional","accountingEntries":` +
				previousActionsAccountingEntriesJSON + `,"metadata":null}`,
		},
		{
			name: "update without new actions",
			input: &UpdateOperationRouteInput{
				Title:             "probe A updated",
				AccountingEntries: previousActionsAccountingEntries(),
			},
			expected: `{"accountingEntries":` + previousActionsAccountingEntriesJSON + `,"title":"probe A updated"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, string(data))
			assert.NotContains(t, string(data), `"block"`)
			assert.NotContains(t, string(data), `"unblock"`)
			assert.NotContains(t, string(data), `"crossLedger"`)
		})
	}
}

func TestTransactionRouteModelsAssetRateValidation(t *testing.T) {
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

func TestTransactionRouteModelsAnnotationRequiresSend(t *testing.T) {
	require.NoError(t, NewCreateAnnotationInput("note").Validate())
	require.NoError(t, NewCreateAnnotationInput("note", &SendInput{Asset: "USD", Value: "10", Source: &SourceInput{From: []FromToInput{{AccountAlias: "@a", Amount: AmountInput{Asset: "USD", Value: "10"}}}}, Distribute: &DistributeInput{To: []FromToInput{{AccountAlias: "@b", Amount: AmountInput{Asset: "USD", Value: "10"}}}}}).Validate())
}

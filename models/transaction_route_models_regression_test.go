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

func TestTransactionRouteModelsOperationRouteInputs_SerializeNewActions(t *testing.T) {
	blockEntry := testAccountingEntry("1003", "block debit", "2003", "block credit")
	unblockEntry := testAccountingEntry("1004", "unblock debit", "2004", "unblock credit")
	crossLedgerEntry := testAccountingEntry("9001", "in", "9002", "out")

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name: "create with direct, block and unblock",
			input: &CreateOperationRouteInput{
				Title:         "probe A",
				OperationType: "bidirectional",
				AccountingEntries: &AccountingEntries{
					Direct:  testAccountingEntry("1001", "direct debit", "2001", "direct credit"),
					Block:   blockEntry,
					Unblock: unblockEntry,
				},
			},
			expected: `{"title":"probe A","operationType":"bidirectional","accountingEntries":{` +
				`"direct":{"debit":{"code":"1001","description":"direct debit"},"credit":{"code":"2001","description":"direct credit"}},` +
				`"block":{"debit":{"code":"1003","description":"block debit"},"credit":{"code":"2003","description":"block credit"}},` +
				`"unblock":{"debit":{"code":"1004","description":"unblock debit"},"credit":{"code":"2004","description":"unblock credit"}}},` +
				`"metadata":null}`,
		},
		{
			name: "create bridge route with crossLedger only",
			input: &CreateOperationRouteInput{
				Title:             "probe B",
				OperationType:     "bidirectional",
				AccountingEntries: &AccountingEntries{CrossLedger: crossLedgerEntry},
			},
			expected: `{"title":"probe B","operationType":"bidirectional","accountingEntries":{` +
				`"crossLedger":{"debit":{"code":"9001","description":"in"},"credit":{"code":"9002","description":"out"}}},` +
				`"metadata":null}`,
		},
		{
			name: "update with block and unblock",
			input: &UpdateOperationRouteInput{
				AccountingEntries: &AccountingEntries{Block: blockEntry, Unblock: unblockEntry},
			},
			expected: `{"accountingEntries":{` +
				`"block":{"debit":{"code":"1003","description":"block debit"},"credit":{"code":"2003","description":"block credit"}},` +
				`"unblock":{"debit":{"code":"1004","description":"unblock debit"},"credit":{"code":"2004","description":"unblock credit"}}}}`,
		},
		{
			name: "update with crossLedger",
			input: &UpdateOperationRouteInput{
				AccountingEntries: &AccountingEntries{CrossLedger: crossLedgerEntry},
			},
			expected: `{"accountingEntries":{` +
				`"crossLedger":{"debit":{"code":"9001","description":"in"},"credit":{"code":"9002","description":"out"}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			require.NoError(t, err)

			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

func TestTransactionRouteModelsUpdateOperationRouteInput_AccountingEntriesRawKeepsPrecedence(t *testing.T) {
	const raw = `{"direct":{"debit":{"code":"1001","description":"direct debit"},"credit":{"code":"2001","description":"direct credit"}}}`

	input := &UpdateOperationRouteInput{
		AccountingEntries: &AccountingEntries{
			Block: testAccountingEntry("1003", "block debit", "2003", "block credit"),
		},
		AccountingEntriesRaw: json.RawMessage(raw),
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	assert.JSONEq(t, `{"accountingEntries":`+raw+`}`, string(data))
	assert.NotContains(t, string(data), `"block"`)
}

// eightActionsOperationRouteJSON is the Midaz response for a bidirectional
// route with every action except crossLedger.
const eightActionsOperationRouteJSON = `{"id":"01a2187d-81c1-7f26-b162-b41fc292c4b5","organizationId":"01a2187d-174a-79e1-9a6f-e7ff20a4a7c4","ledgerId":"01a2187d-1768-7280-8222-3adfe0d315fb","title":"probe A v2","code":"PRB-A-v2","operationType":"bidirectional",` +
	`"accountingEntries":{"direct":{"debit":{"code":"1001","description":"direct debit"},"credit":{"code":"2001","description":"direct credit"}},"hold":{"debit":{"code":"1002","description":"hold debit"},"credit":{"code":"2002","description":"hold credit"}},"commit":{"debit":{"code":"1005","description":"commit debit"},"credit":{"code":"2005","description":"commit credit"}},"cancel":{"debit":{"code":"1006","description":"cancel debit"},"credit":{"code":"2006","description":"cancel credit"}},"revert":{"debit":{"code":"1007","description":"revert debit"},"credit":{"code":"2007","description":"revert credit"}},"overdraft":{"debit":{"code":"1008","description":"overdraft debit"},"credit":{"code":"2008","description":"overdraft credit"}},"block":{"debit":{"code":"1003","description":"block debit"},"credit":{"code":"2003","description":"block credit"}},"unblock":{"debit":{"code":"1004","description":"unblock debit"},"credit":{"code":"2004","description":"unblock credit"}}},` +
	`"createdAt":"2026-11-26T15:33:08.51566Z","updatedAt":"2026-11-26T15:33:08.51566Z","deletedAt":null}`

// bridgeOperationRouteJSON is the Midaz response for a bridge route that
// carries only the crossLedger action.
const bridgeOperationRouteJSON = `{"id":"01a2187d-3523-7a1b-90f9-d0c41673c018","organizationId":"01a2187d-174a-79e1-9a6f-e7ff20a4a7c4","ledgerId":"01a2187d-1768-7280-8222-3adfe0d315fb","title":"probe B v2","code":"PRB-B-v2","operationType":"bidirectional","accountingEntries":{"crossLedger":{"debit":{"code":"9001","description":"in"},"credit":{"code":"9002","description":"out"}}},"createdAt":"2026-11-26T15:33:08.51566Z","updatedAt":"2026-11-26T15:33:08.51566Z","deletedAt":null}`

func accountingEntriesByAction(entries *AccountingEntries) map[string]*AccountingEntry {
	return map[string]*AccountingEntry{
		"direct":      entries.Direct,
		"hold":        entries.Hold,
		"commit":      entries.Commit,
		"cancel":      entries.Cancel,
		"revert":      entries.Revert,
		"overdraft":   entries.Overdraft,
		"block":       entries.Block,
		"unblock":     entries.Unblock,
		"crossLedger": entries.CrossLedger,
	}
}

func TestTransactionRouteModelsOperationRoute_DecodesServerActions(t *testing.T) {
	type rubrics struct {
		debitCode, debitDescription, creditCode, creditDescription string
	}

	tests := []struct {
		name     string
		body     string
		id       string
		title    string
		code     string
		expected map[string]*rubrics
	}{
		{
			name:  "eight actions route",
			body:  eightActionsOperationRouteJSON,
			id:    "01a2187d-81c1-7f26-b162-b41fc292c4b5",
			title: "probe A v2",
			code:  "PRB-A-v2",
			expected: map[string]*rubrics{
				"direct":      {"1001", "direct debit", "2001", "direct credit"},
				"hold":        {"1002", "hold debit", "2002", "hold credit"},
				"commit":      {"1005", "commit debit", "2005", "commit credit"},
				"cancel":      {"1006", "cancel debit", "2006", "cancel credit"},
				"revert":      {"1007", "revert debit", "2007", "revert credit"},
				"overdraft":   {"1008", "overdraft debit", "2008", "overdraft credit"},
				"block":       {"1003", "block debit", "2003", "block credit"},
				"unblock":     {"1004", "unblock debit", "2004", "unblock credit"},
				"crossLedger": nil,
			},
		},
		{
			name:  "bridge route",
			body:  bridgeOperationRouteJSON,
			id:    "01a2187d-3523-7a1b-90f9-d0c41673c018",
			title: "probe B v2",
			code:  "PRB-B-v2",
			expected: map[string]*rubrics{
				"direct":      nil,
				"hold":        nil,
				"commit":      nil,
				"cancel":      nil,
				"revert":      nil,
				"overdraft":   nil,
				"block":       nil,
				"unblock":     nil,
				"crossLedger": {"9001", "in", "9002", "out"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var route OperationRoute
			require.NoError(t, json.Unmarshal([]byte(tt.body), &route))

			assert.Equal(t, tt.id, route.ID.String())
			assert.Equal(t, tt.title, route.Title)
			assert.Equal(t, tt.code, route.Code)
			assert.Equal(t, "bidirectional", route.OperationType)
			require.NotNil(t, route.AccountingEntries)

			actual := accountingEntriesByAction(route.AccountingEntries)
			require.Len(t, tt.expected, len(actual))

			for action, want := range tt.expected {
				got := actual[action]
				if want == nil {
					assert.Nil(t, got, action)
					continue
				}

				require.NotNil(t, got, action)
				require.NotNil(t, got.Debit, action)
				require.NotNil(t, got.Credit, action)
				assert.Equal(t, want.debitCode, got.Debit.Code, action)
				assert.Equal(t, want.debitDescription, got.Debit.Description, action)
				assert.Equal(t, want.creditCode, got.Credit.Code, action)
				assert.Equal(t, want.creditDescription, got.Credit.Description, action)
			}
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

// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation"
)

// The literals below are response bodies captured from a local Midaz 4.6.0
// ledger, copied in compact form with the server's key order.

// capturedPlanWithRounding is the 201 body of a create carrying rounding
// (scale 2, halfUp) and two transaction fees, one received and one sent.
const capturedPlanWithRounding = `{"id":"01a21da6-a6fa-7044-9058-c1f644284b5a","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"plano-1795793790","enable":true,"timezone":"America/Sao_Paulo","currentVersion":1,"createdAt":"2026-11-27T15:36:30.714Z","updatedAt":"2026-11-27T15:36:30.714Z","version":1,"effectiveFrom":"2026-11-27T15:36:30.714Z","fees":{"pixIn":{"feeLabel":"Pix recebido","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":2,"period":"monthly","deferrable":false},"pixOut":{"feeLabel":"Pix enviado","trigger":{"type":"transaction","schemes":["PIX"],"direction":"sent"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}},"rounding":{"scale":2,"mode":"halfUp"}}`

// capturedPlanZeroScale is the 200 body of an update that set the description
// and a rounding of scale 0, mode floor, creating version 2.
const capturedPlanZeroScale = `{"id":"01a21da6-a6fa-7044-9058-c1f644284b5a","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"plano-1795793790","description":"v2","enable":true,"timezone":"America/Sao_Paulo","currentVersion":2,"createdAt":"2026-11-27T15:36:30.714Z","updatedAt":"2026-11-27T15:36:30.749Z","version":2,"effectiveFrom":"2026-11-27T15:36:30.749Z","fees":{"pixIn":{"feeLabel":"Pix recebido","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":2,"period":"monthly","deferrable":false},"pixOut":{"feeLabel":"Pix enviado","trigger":{"type":"transaction","schemes":["PIX"],"direction":"sent"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}},"rounding":{"scale":0,"mode":"floor"}}`

// capturedPlanWithoutRounding is the 201 body of a create with enable false and
// no rounding: the response carries no rounding key.
const capturedPlanWithoutRounding = `{"id":"01a21dac-8ae1-714f-93cd-3aeaf14c3850","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"D-1795794176","enable":false,"timezone":"America/Sao_Paulo","currentVersion":1,"createdAt":"2026-11-27T15:42:56.737Z","updatedAt":"2026-11-27T15:42:56.737Z","version":1,"effectiveFrom":"2026-11-27T15:42:56.737Z","fees":{"a":{"feeLabel":"f","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.00"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false},"b":{"feeLabel":"f","trigger":{"type":"transaction","schemes":["PIX"],"direction":"sent"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.00"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}}}`

// capturedPlanDeferrableAndService is the 201 body of a create with a
// deferrable sent fee matching metadata and a service fee.
const capturedPlanDeferrableAndService = `{"id":"01a21dcf-9e7c-7f17-8f14-063306f63db8","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"DF-1795796475","description":"deferrable e servico","enable":true,"timezone":"America/Sao_Paulo","currentVersion":1,"createdAt":"2026-11-27T16:21:15.516Z","updatedAt":"2026-11-27T16:21:15.516Z","version":1,"effectiveFrom":"2026-11-27T16:21:15.516Z","fees":{"statement":{"feeLabel":"Extrato","trigger":{"type":"service","serviceCode":"STATEMENT"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"2.50"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false},"tedOut":{"feeLabel":"TED enviada","trigger":{"type":"transaction","schemes":["TED"],"direction":"sent","metadata":{"channel":"app"}},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"2.50"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":1,"period":"monthly","deferrable":true}}}`

// capturedPlanList is the 200 body of a plan listing with limit=1 and
// enable=false.
const capturedPlanList = `{"items":[{"id":"01a21dac-8ae1-714f-93cd-3aeaf14c3850","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"D-1795794176","enable":false,"timezone":"America/Sao_Paulo","currentVersion":1,"createdAt":"2026-11-27T15:42:56.737Z","updatedAt":"2026-11-27T15:42:56.737Z"}],"page":1,"limit":1,"total":1}`

// capturedVersionList is the 200 body of a version listing of a plan with two
// versions: scale 2 halfUp, then scale 0 floor.
const capturedVersionList = `{"items":[{"accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","version":1,"effectiveFrom":"2026-11-27T15:36:30.714Z","fees":{"pixIn":{"feeLabel":"Pix recebido","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":2,"period":"monthly","deferrable":false},"pixOut":{"feeLabel":"Pix enviado","trigger":{"type":"transaction","schemes":["PIX"],"direction":"sent"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}},"rounding":{"scale":2,"mode":"halfUp"},"createdAt":"2026-11-27T15:36:30.714Z"},{"accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","version":2,"effectiveFrom":"2026-11-27T15:36:30.749Z","fees":{"pixIn":{"feeLabel":"Pix recebido","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":2,"period":"monthly","deferrable":false},"pixOut":{"feeLabel":"Pix enviado","trigger":{"type":"transaction","schemes":["PIX"],"direction":"sent"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}},"rounding":{"scale":0,"mode":"floor"},"createdAt":"2026-11-27T15:36:30.749Z"}],"page":1,"limit":10,"total":2}`

// capturedAssignment is the 200 body of an assignment.
const capturedAssignment = `{"id":"01a21da6-a748-796a-8591-5ba05e7212ea","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","startedAt":"2026-11-27T15:36:30.792581Z","endedAt":null}`

// capturedAssignmentsActive is the 200 body of an assignment read with an
// active assignment and two ended ones.
const capturedAssignmentsActive = `{"active":{"id":"01a21dac-8b76-7978-b2af-ac3107bf3c46","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21dac-8ac6-7491-8690-6685df647f59","startedAt":"2026-11-27T15:42:56.886594Z","endedAt":null},"history":[{"id":"01a21dac-8b76-7978-b2af-ac3107bf3c46","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21dac-8ac6-7491-8690-6685df647f59","startedAt":"2026-11-27T15:42:56.886594Z","endedAt":null},{"id":"01a21dac-8b63-77e1-a2e4-a419e5a79ded","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21dac-8a87-70fa-a7c8-f02fd0563314","startedAt":"2026-11-27T15:42:56.86749Z","endedAt":"2026-11-27T15:42:56.886594Z"},{"id":"01a21da6-a748-796a-8591-5ba05e7212ea","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","startedAt":"2026-11-27T15:36:30.792581Z","endedAt":"2026-11-27T15:36:30.832799Z"}]}`

// capturedAssignmentsNoActive is the 200 body of an assignment read after the
// account was unassigned: active is null and every history entry has ended.
const capturedAssignmentsNoActive = `{"active":null,"history":[{"id":"01a21dac-8b76-7978-b2af-ac3107bf3c46","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21dac-8ac6-7491-8690-6685df647f59","startedAt":"2026-11-27T15:42:56.886594Z","endedAt":"2026-11-27T15:42:56.916827Z"},{"id":"01a21dac-8b63-77e1-a2e4-a419e5a79ded","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21dac-8a87-70fa-a7c8-f02fd0563314","startedAt":"2026-11-27T15:42:56.86749Z","endedAt":"2026-11-27T15:42:56.886594Z"},{"id":"01a21da6-a748-796a-8591-5ba05e7212ea","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","startedAt":"2026-11-27T15:36:30.792581Z","endedAt":"2026-11-27T15:36:30.832799Z"}]}`

// capturedPlanAccounts is the 200 body of a listing of the accounts of a plan;
// it carries no total.
const capturedPlanAccounts = `{"items":[{"id":"01a21da6-a748-796a-8591-5ba05e7212ea","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","startedAt":"2026-11-27T15:36:30.792581Z","endedAt":null}],"limit":10,"page":1}`

// capturedUsage is the 200 body of a usage read of the current period.
const capturedUsage = `{"accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","accountPlanVersion":2,"period":{"label":"2026-11","start":"2026-11-01T03:00:00Z","end":"2026-12-01T03:00:00Z"},"items":[{"feeKey":"pixIn","trigger":"fee:pixIn:*","period":{"label":"2026-11","start":"2026-11-01T03:00:00Z","end":"2026-12-01T03:00:00Z"},"freeQuota":2,"used":0,"remaining":2,"charged":0},{"feeKey":"pixOut","trigger":"fee:pixOut:*","period":{"label":"2026-11","start":"2026-11-01T03:00:00Z","end":"2026-12-01T03:00:00Z"},"freeQuota":0,"used":0,"remaining":0,"charged":0}]}`

func decodeCaptured[T any](t *testing.T, body string) T {
	t.Helper()

	var out T
	require.NoError(t, json.Unmarshal([]byte(body), &out))

	return out
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339Nano, value)
	require.NoError(t, err)

	return parsed
}

func TestAccountPlanView_DecodesCapturedCreateWithRounding(t *testing.T) {
	view := decodeCaptured[AccountPlanView](t, capturedPlanWithRounding)

	assert.Equal(t, "01a21da6-a6fa-7044-9058-c1f644284b5a", view.ID)
	assert.Equal(t, "01a21da6-a657-7c9a-a4e5-1d488031102e", view.OrganizationID)
	assert.Equal(t, "01a21da6-a670-7033-ae97-a38bd670f3b8", view.LedgerID)
	assert.Equal(t, "plano-1795793790", view.Label)
	assert.Nil(t, view.Description)
	assert.True(t, view.Enable)
	assert.Equal(t, "America/Sao_Paulo", view.Timezone)
	assert.Equal(t, 1, view.CurrentVersion)
	assert.Equal(t, 1, view.Version)
	assert.Equal(t, mustTime(t, "2026-11-27T15:36:30.714Z"), view.CreatedAt)
	assert.Equal(t, mustTime(t, "2026-11-27T15:36:30.714Z"), view.EffectiveFrom)
	assert.Nil(t, view.DeletedAt)
	require.NotNil(t, view.Rounding)
	assert.Equal(t, FeeRounding{Scale: 2, Mode: FeeRoundingModeHalfUp}, *view.Rounding)

	require.Len(t, view.Fees, 2)
	assert.Equal(t, AccountPlanFee{
		FeeLabel: "Pix recebido",
		Trigger: AccountPlanTrigger{
			Type:      AccountPlanTriggerTypeTransaction,
			Direction: AccountPlanTriggerDirectionReceived,
			Schemes:   []string{"PIX"},
		},
		CalculationModel: FeeCalculationModel{
			ApplicationRule: "flatFee",
			Calculations:    []Calculation{{Type: "flat", Value: "1.239"}},
		},
		ReferenceAmount: "originalAmount",
		CreditAccount:   "@fees-1795793790",
		FreeQuota:       2,
		Period:          "monthly",
	}, view.Fees["pixIn"])
	assert.Equal(t, AccountPlanTriggerDirectionSent, view.Fees["pixOut"].Trigger.Direction)
	assert.Equal(t, 0, view.Fees["pixOut"].FreeQuota)
}

func TestAccountPlanView_DecodesCapturedZeroScaleRounding(t *testing.T) {
	view := decodeCaptured[AccountPlanView](t, capturedPlanZeroScale)

	require.NotNil(t, view.Description)
	assert.Equal(t, "v2", *view.Description)
	assert.Equal(t, 2, view.Version)
	assert.Equal(t, 2, view.CurrentVersion)
	require.NotNil(t, view.Rounding, "a scale-zero rounding is present, not absent")
	assert.Equal(t, FeeRounding{Scale: 0, Mode: FeeRoundingModeFloor}, *view.Rounding)
}

func TestAccountPlanView_DecodesCapturedPlanWithoutRounding(t *testing.T) {
	view := decodeCaptured[AccountPlanView](t, capturedPlanWithoutRounding)

	assert.Nil(t, view.Rounding, "a missing rounding key is absent, not a zero value")
	assert.False(t, view.Enable)
	assert.Equal(t, "D-1795794176", view.Label)
	assert.Len(t, view.Fees, 2)
}

func TestAccountPlanView_DecodesCapturedDeferrableAndServiceFees(t *testing.T) {
	view := decodeCaptured[AccountPlanView](t, capturedPlanDeferrableAndService)

	ted := view.Fees["tedOut"]
	assert.True(t, ted.Deferrable)
	assert.Equal(t, 1, ted.FreeQuota)
	assert.Equal(t, AccountPlanTrigger{
		Type:      AccountPlanTriggerTypeTransaction,
		Direction: AccountPlanTriggerDirectionSent,
		Schemes:   []string{"TED"},
		Metadata:  map[string]string{"channel": "app"},
	}, ted.Trigger)
	assert.Nil(t, ted.Accounting)

	statement := view.Fees["statement"]
	assert.False(t, statement.Deferrable)
	assert.Equal(t, AccountPlanTrigger{
		Type:        AccountPlanTriggerTypeService,
		ServiceCode: "STATEMENT",
	}, statement.Trigger)
}

func TestAccountPlanAccounting_Decodes(t *testing.T) {
	// No capture carries accounting: the local ledger does not validate routes.
	// The keys come from the FeeAccountPlanAccounting contract schema.
	fee := decodeCaptured[AccountPlanFee](t, `{"accounting":{"transactionRoute":"tr","operationRouteFrom":"from","operationRouteTo":"to"}}`)

	require.NotNil(t, fee.Accounting)
	assert.Equal(t, AccountPlanAccounting{
		TransactionRoute:   "tr",
		OperationRouteFrom: "from",
		OperationRouteTo:   "to",
	}, *fee.Accounting)
}

func TestAccountPlan_DecodesCapturedList(t *testing.T) {
	list := decodeCaptured[ListResponse[AccountPlan]](t, capturedPlanList)

	require.Len(t, list.Items, 1)
	assert.Equal(t, AccountPlan{
		ID:             "01a21dac-8ae1-714f-93cd-3aeaf14c3850",
		OrganizationID: "01a21da6-a657-7c9a-a4e5-1d488031102e",
		LedgerID:       "01a21da6-a670-7033-ae97-a38bd670f3b8",
		Label:          "D-1795794176",
		Enable:         false,
		Timezone:       "America/Sao_Paulo",
		CurrentVersion: 1,
		CreatedAt:      mustTime(t, "2026-11-27T15:42:56.737Z"),
		UpdatedAt:      mustTime(t, "2026-11-27T15:42:56.737Z"),
	}, list.Items[0])
	assert.Equal(t, 1, list.Pagination.Page)
	assert.Equal(t, 1, list.Pagination.Limit)
	assert.Equal(t, 1, list.Pagination.Total)
}

func TestAccountPlanVersion_DecodesCapturedList(t *testing.T) {
	list := decodeCaptured[ListResponse[AccountPlanVersion]](t, capturedVersionList)

	require.Len(t, list.Items, 2)

	first, second := list.Items[0], list.Items[1]
	assert.Equal(t, "01a21da6-a6fa-7044-9058-c1f644284b5a", first.AccountPlanID)
	assert.Equal(t, 1, first.Version)
	assert.Equal(t, mustTime(t, "2026-11-27T15:36:30.714Z"), first.EffectiveFrom)
	assert.Equal(t, mustTime(t, "2026-11-27T15:36:30.714Z"), first.CreatedAt)
	require.NotNil(t, first.Rounding)
	assert.Equal(t, FeeRounding{Scale: 2, Mode: FeeRoundingModeHalfUp}, *first.Rounding)
	assert.Len(t, first.Fees, 2)

	assert.Equal(t, 2, second.Version)
	require.NotNil(t, second.Rounding)
	assert.Equal(t, FeeRounding{Scale: 0, Mode: FeeRoundingModeFloor}, *second.Rounding)
	assert.Equal(t, 2, list.Pagination.Total)
}

func TestAccountPlanAssignment_DecodesCaptured(t *testing.T) {
	assignment := decodeCaptured[AccountPlanAssignment](t, capturedAssignment)

	assert.Equal(t, AccountPlanAssignment{
		ID:             "01a21da6-a748-796a-8591-5ba05e7212ea",
		OrganizationID: "01a21da6-a657-7c9a-a4e5-1d488031102e",
		LedgerID:       "01a21da6-a670-7033-ae97-a38bd670f3b8",
		AccountID:      "01a21da6-a6ce-781f-b7d2-ad9e3b66a550",
		AccountAlias:   "@cust-1795793790",
		AccountPlanID:  "01a21da6-a6fa-7044-9058-c1f644284b5a",
		StartedAt:      mustTime(t, "2026-11-27T15:36:30.792581Z"),
	}, assignment)
}

func TestAccountPlanAssignments_DecodesCapturedActive(t *testing.T) {
	assignments := decodeCaptured[AccountPlanAssignments](t, capturedAssignmentsActive)

	require.NotNil(t, assignments.Active)
	assert.Equal(t, "01a21dac-8ac6-7491-8690-6685df647f59", assignments.Active.AccountPlanID)
	assert.Nil(t, assignments.Active.EndedAt)

	require.Len(t, assignments.History, 3)
	assert.Nil(t, assignments.History[0].EndedAt)
	require.NotNil(t, assignments.History[1].EndedAt)
	assert.Equal(t, mustTime(t, "2026-11-27T15:42:56.886594Z"), *assignments.History[1].EndedAt)
}

func TestAccountPlanAssignments_DecodesCapturedWithoutActive(t *testing.T) {
	assignments := decodeCaptured[AccountPlanAssignments](t, capturedAssignmentsNoActive)

	assert.Nil(t, assignments.Active)
	require.Len(t, assignments.History, 3)

	for i, entry := range assignments.History {
		assert.NotNil(t, entry.EndedAt, "history[%d] has ended", i)
	}

	assert.Equal(t, mustTime(t, "2026-11-27T15:42:56.916827Z"), *assignments.History[0].EndedAt)
}

func TestAccountPlanAssignment_DecodesCapturedPlanAccounts(t *testing.T) {
	list := decodeCaptured[ListResponse[AccountPlanAssignment]](t, capturedPlanAccounts)

	require.Len(t, list.Items, 1)
	assert.Equal(t, "@cust-1795793790", list.Items[0].AccountAlias)
	assert.Nil(t, list.Items[0].EndedAt)
	assert.Equal(t, 10, list.Pagination.Limit)
	assert.Equal(t, 1, list.Pagination.Page)
	assert.Equal(t, 0, list.Pagination.Total, "the plan accounts listing carries no total")
}

func TestAccountPlanUsage_DecodesCaptured(t *testing.T) {
	usage := decodeCaptured[AccountPlanUsage](t, capturedUsage)

	period := QuotaUsagePeriod{
		Label: "2026-11",
		Start: mustTime(t, "2026-11-01T03:00:00Z"),
		End:   mustTime(t, "2026-12-01T03:00:00Z"),
	}

	assert.Equal(t, "01a21da6-a6ce-781f-b7d2-ad9e3b66a550", usage.AccountID)
	assert.Equal(t, "01a21da6-a6fa-7044-9058-c1f644284b5a", usage.AccountPlanID)
	assert.Equal(t, 2, usage.AccountPlanVersion)
	assert.Equal(t, period, usage.Period)
	assert.Equal(t, []AccountPlanFeeUsage{
		{FeeKey: "pixIn", Trigger: "fee:pixIn:*", Period: period, FreeQuota: 2, Remaining: 2},
		{FeeKey: "pixOut", Trigger: "fee:pixOut:*", Period: period},
	}, usage.Items)
}

// accountPlanFieldErrors returns the fields named by a Validate error, which
// must be a *validation.FieldErrors.
func accountPlanFieldErrors(t *testing.T, err error) []string {
	t.Helper()

	var fieldErrs *validation.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)

	fields := make([]string, 0, fieldErrs.Len())
	for _, fe := range fieldErrs.Errs() {
		fields = append(fields, fe.Field)
	}

	return fields
}

func validAccountPlanFeeInput() AccountPlanFeeInput {
	return AccountPlanFeeInput{
		FeeLabel: "Pix enviado",
		Trigger: AccountPlanTriggerInput{
			Type:      AccountPlanTriggerTypeTransaction,
			Direction: AccountPlanTriggerDirectionSent,
			Schemes:   []string{"PIX"},
		},
		CalculationModel: FeeCalculationModel{
			ApplicationRule: "flatFee",
			Calculations:    []Calculation{{Type: "flat", Value: "1.239"}},
		},
		CreditAccount: "@fees",
	}
}

func TestCreateAccountPlanInput_Validate(t *testing.T) {
	withFee := func(mutate func(fee *AccountPlanFeeInput)) *CreateAccountPlanInput {
		fee := validAccountPlanFeeInput()
		mutate(&fee)

		return NewCreateAccountPlanInput("plano", true, map[string]AccountPlanFeeInput{"pixOut": fee})
	}
	valid := func() *CreateAccountPlanInput { return withFee(func(*AccountPlanFeeInput) {}) }

	tests := []struct {
		name      string
		input     *CreateAccountPlanInput
		wantField string
	}{
		{"blank label", NewCreateAccountPlanInput("  ", true, valid().Fees), "label"},
		{"nil fees", NewCreateAccountPlanInput("plano", true, nil), "fees"},
		{"empty fees", NewCreateAccountPlanInput("plano", true, map[string]AccountPlanFeeInput{}), "fees"},
		{"missing enable", &CreateAccountPlanInput{Label: "plano", Fees: valid().Fees}, "enable"},
		{"rounding scale below 0", valid().WithRounding(FeeRounding{Scale: -1, Mode: FeeRoundingModeFloor}), "rounding.scale"},
		{"rounding scale above 18", valid().WithRounding(FeeRounding{Scale: 19, Mode: FeeRoundingModeHalfUp}), "rounding.scale"},
		{"rounding unknown mode", valid().WithRounding(FeeRounding{Scale: 2, Mode: "ceil"}), "rounding.mode"},
		{"rounding missing mode", valid().WithRounding(FeeRounding{Scale: 2}), "rounding.mode"},
		{"fee key upper case first", NewCreateAccountPlanInput("plano", true, map[string]AccountPlanFeeInput{"PixOut": validAccountPlanFeeInput()}), "fees[PixOut]"},
		{"fee key digit first", NewCreateAccountPlanInput("plano", true, map[string]AccountPlanFeeInput{"1pix": validAccountPlanFeeInput()}), "fees[1pix]"},
		{"fee key underscore", NewCreateAccountPlanInput("plano", true, map[string]AccountPlanFeeInput{"pix_out": validAccountPlanFeeInput()}), "fees[pix_out]"},
		{"fee key empty", NewCreateAccountPlanInput("plano", true, map[string]AccountPlanFeeInput{"": validAccountPlanFeeInput()}), "fees[]"},
		{"fee without label", withFee(func(f *AccountPlanFeeInput) { f.FeeLabel = " " }), "fees[pixOut].feeLabel"},
		{"fee without credit account", withFee(func(f *AccountPlanFeeInput) { f.CreditAccount = "" }), "fees[pixOut].creditAccount"},
		{"fee without calculation model", withFee(func(f *AccountPlanFeeInput) { f.CalculationModel = FeeCalculationModel{} }), "fees[pixOut].calculationModel"},
		{"fee with negative free quota", withFee(func(f *AccountPlanFeeInput) { f.FreeQuota = -1 }), "fees[pixOut].freeQuota"},
		{"trigger without type", withFee(func(f *AccountPlanFeeInput) { f.Trigger.Type = "" }), "fees[pixOut].trigger.type"},
		{"trigger unknown type", withFee(func(f *AccountPlanFeeInput) { f.Trigger.Type = "batch" }), "fees[pixOut].trigger.type"},
		{"trigger unknown direction", withFee(func(f *AccountPlanFeeInput) { f.Trigger.Direction = "both" }), "fees[pixOut].trigger.direction"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			require.Error(t, err)
			assert.Equal(t, []string{tt.wantField}, accountPlanFieldErrors(t, err))
		})
	}
}

func TestCreateAccountPlanInput_ValidateRejectsNil(t *testing.T) {
	var input *CreateAccountPlanInput

	require.EqualError(t, input.Validate(), "input cannot be nil")
}

func TestCreateAccountPlanInput_ValidateAccepts(t *testing.T) {
	service := AccountPlanFeeInput{
		FeeLabel:         "Extrato",
		Trigger:          AccountPlanTriggerInput{Type: AccountPlanTriggerTypeService, ServiceCode: "STATEMENT"},
		CalculationModel: FeeCalculationModel{ApplicationRule: "flatFee", Calculations: []Calculation{{Type: "flat", Value: "2.50"}}},
		CreditAccount:    "@fees",
	}
	received := validAccountPlanFeeInput()
	received.Trigger.Direction = AccountPlanTriggerDirectionReceived

	tests := []struct {
		name  string
		input *CreateAccountPlanInput
	}{
		{"enable false", NewCreateAccountPlanInput("plano", false, map[string]AccountPlanFeeInput{"pixOut": validAccountPlanFeeInput()})},
		{"rounding scale 0", NewCreateAccountPlanInput("plano", true, map[string]AccountPlanFeeInput{"pixOut": validAccountPlanFeeInput()}).WithRounding(FeeRounding{Scale: 0, Mode: FeeRoundingModeFloor})},
		{"rounding scale 18", NewCreateAccountPlanInput("plano", true, map[string]AccountPlanFeeInput{"pixOut": validAccountPlanFeeInput()}).WithRounding(FeeRounding{Scale: 18, Mode: FeeRoundingModeHalfUp})},
		{"received and service fees", NewCreateAccountPlanInput("plano", true, map[string]AccountPlanFeeInput{"pixIn": received, "statement": service, "tedOut2": validAccountPlanFeeInput()})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, tt.input.Validate())
		})
	}
}

func TestCreateAccountPlanInput_BuildersOnNil(t *testing.T) {
	var input *CreateAccountPlanInput

	assert.Nil(t, input.WithDescription("d"))
	assert.Nil(t, input.WithTimezone("UTC"))
	assert.Nil(t, input.WithRounding(FeeRounding{Scale: 2, Mode: FeeRoundingModeFloor}))
}

func TestCreateAccountPlanInput_JSONSendsEnableFalseAndOmitsUnset(t *testing.T) {
	input := NewCreateAccountPlanInput("plano", false, map[string]AccountPlanFeeInput{"pixOut": validAccountPlanFeeInput()})

	body, err := json.Marshal(input)
	require.NoError(t, err)

	assert.Equal(t, `{"label":"plano","enable":false,"fees":{"pixOut":{"feeLabel":"Pix enviado","trigger":{"type":"transaction","direction":"sent","schemes":["PIX"]},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"creditAccount":"@fees"}}}`, string(body))
}

func TestCreateAccountPlanInput_JSONMatchesCapturedCreateWithRounding(t *testing.T) {
	calculation := FeeCalculationModel{ApplicationRule: "flatFee", Calculations: []Calculation{{Type: "flat", Value: "1.239"}}}
	input := NewCreateAccountPlanInput("plano-1795793790", true, map[string]AccountPlanFeeInput{
		"pixIn": {
			FeeLabel:         "Pix recebido",
			Trigger:          AccountPlanTriggerInput{Type: AccountPlanTriggerTypeTransaction, Direction: AccountPlanTriggerDirectionReceived, Schemes: []string{"PIX"}},
			CalculationModel: calculation,
			CreditAccount:    "@fees-1795793790",
			FreeQuota:        2,
		},
		"pixOut": {
			FeeLabel:         "Pix enviado",
			Trigger:          AccountPlanTriggerInput{Type: AccountPlanTriggerTypeTransaction, Direction: AccountPlanTriggerDirectionSent, Schemes: []string{"PIX"}},
			CalculationModel: calculation,
			CreditAccount:    "@fees-1795793790",
		},
	}).WithRounding(FeeRounding{Scale: 2, Mode: FeeRoundingModeHalfUp})
	require.NoError(t, input.Validate())

	body, err := json.Marshal(input)
	require.NoError(t, err)

	// Request body the server accepted with 201.
	assert.JSONEq(t, `{"label":"plano-1795793790","enable":true,"rounding":{"scale":2,"mode":"halfUp"},"fees":{"pixIn":{"feeLabel":"Pix recebido","trigger":{"type":"transaction","direction":"received","schemes":["PIX"]},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"creditAccount":"@fees-1795793790","freeQuota":2},"pixOut":{"feeLabel":"Pix enviado","trigger":{"type":"transaction","direction":"sent","schemes":["PIX"]},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"creditAccount":"@fees-1795793790"}}}`, string(body))
}

func TestCreateAccountPlanInput_JSONMatchesCapturedDeferrableAndService(t *testing.T) {
	calculation := FeeCalculationModel{ApplicationRule: "flatFee", Calculations: []Calculation{{Type: "flat", Value: "2.50"}}}
	input := NewCreateAccountPlanInput("DF-1795796475", true, map[string]AccountPlanFeeInput{
		"tedOut": {
			FeeLabel: "TED enviada",
			Trigger: AccountPlanTriggerInput{
				Type:      AccountPlanTriggerTypeTransaction,
				Direction: AccountPlanTriggerDirectionSent,
				Schemes:   []string{"TED"},
				Metadata:  map[string]string{"channel": "app"},
			},
			CalculationModel: calculation,
			CreditAccount:    "@fees-1795793790",
			FreeQuota:        1,
			Deferrable:       true,
		},
		"statement": {
			FeeLabel:         "Extrato",
			Trigger:          AccountPlanTriggerInput{Type: AccountPlanTriggerTypeService, ServiceCode: "STATEMENT"},
			CalculationModel: calculation,
			CreditAccount:    "@fees-1795793790",
		},
	}).WithDescription("deferrable e servico").WithTimezone("America/Sao_Paulo")
	require.NoError(t, input.Validate())

	body, err := json.Marshal(input)
	require.NoError(t, err)

	// Request body the server accepted with 201.
	assert.JSONEq(t, `{"label":"DF-1795796475","description":"deferrable e servico","enable":true,"timezone":"America/Sao_Paulo","fees":{"tedOut":{"feeLabel":"TED enviada","trigger":{"type":"transaction","direction":"sent","schemes":["TED"],"metadata":{"channel":"app"}},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"2.50"}]},"creditAccount":"@fees-1795793790","freeQuota":1,"deferrable":true},"statement":{"feeLabel":"Extrato","trigger":{"type":"service","serviceCode":"STATEMENT"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"2.50"}]},"creditAccount":"@fees-1795793790"}}}`, string(body))
}

func TestAccountPlanFeeInput_JSONSendsAccounting(t *testing.T) {
	fee := validAccountPlanFeeInput()
	fee.Accounting = &AccountPlanAccountingInput{TransactionRoute: "tr", OperationRouteFrom: "from", OperationRouteTo: "to"}

	body, err := json.Marshal(fee)
	require.NoError(t, err)

	assert.Equal(t, `{"feeLabel":"Pix enviado","trigger":{"type":"transaction","direction":"sent","schemes":["PIX"]},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"creditAccount":"@fees","accounting":{"transactionRoute":"tr","operationRouteFrom":"from","operationRouteTo":"to"}}`, string(body))
}

func marshalUpdate(t *testing.T, input *UpdateAccountPlanInput) string {
	t.Helper()

	require.NoError(t, input.Validate())

	body, err := json.Marshal(input)
	require.NoError(t, err)

	return string(body)
}

func TestUpdateAccountPlanInput_JSON(t *testing.T) {
	tests := []struct {
		name  string
		input *UpdateAccountPlanInput
		want  string
	}{
		{
			// Captured: description-only PATCH, 200 with the same version.
			name:  "registry edit sends only description",
			input: NewUpdateAccountPlanInput().WithDescription("x"),
			want:  `{"description":"x"}`,
		},
		{
			// Captured: fee removal, 200 with the fee gone.
			name:  "removed fee is sent as null",
			input: NewUpdateAccountPlanInput().WithoutFee("b"),
			want:  `{"fees":{"b":null}}`,
		},
		{
			// Captured: rounding replacement, 200 with version 2.
			name:  "rounding is sent whole",
			input: NewUpdateAccountPlanInput().WithRounding(FeeRounding{Scale: 2, Mode: FeeRoundingModeFloor}),
			want:  `{"rounding":{"scale":2,"mode":"floor"}}`,
		},
		{
			// Captured: description with a scale-zero rounding, 200 with version 2.
			name:  "scale zero rounding is sent",
			input: NewUpdateAccountPlanInput().WithDescription("v2").WithRounding(FeeRounding{Scale: 0, Mode: FeeRoundingModeFloor}),
			want:  `{"description":"v2","rounding":{"scale":0,"mode":"floor"}}`,
		},
		{
			// Captured: retroactive effectiveFrom, refused by the server with 422/0553.
			name:  "effectiveFrom with rounding",
			input: NewUpdateAccountPlanInput().WithEffectiveFrom(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)).WithRounding(FeeRounding{Scale: 2, Mode: FeeRoundingModeHalfUp}),
			want:  `{"effectiveFrom":"2020-01-01T00:00:00Z","rounding":{"scale":2,"mode":"halfUp"}}`,
		},
		{
			name:  "fee patch sends only the key and the fields set",
			input: NewUpdateAccountPlanInput().WithFeePatch("pixIn", AccountPlanFeePatch{FreeQuota: ptr(5), Deferrable: ptr(false)}),
			want:  `{"fees":{"pixIn":{"freeQuota":5,"deferrable":false}}}`,
		},
		{
			name:  "enable false is sent",
			input: NewUpdateAccountPlanInput().WithEnable(false),
			want:  `{"enable":false}`,
		},
		{
			name: "every field in a stable order",
			input: NewUpdateAccountPlanInput().
				WithoutFee("zeta").
				WithFeePatch("alpha", AccountPlanFeePatch{
					FeeLabel:         ptr("Alpha"),
					Trigger:          &AccountPlanTriggerInput{Type: AccountPlanTriggerTypeTransaction, Direction: AccountPlanTriggerDirectionReceived},
					CalculationModel: &FeeCalculationModel{ApplicationRule: "flatFee", Calculations: []Calculation{{Type: "flat", Value: "1.00"}}},
					ReferenceAmount:  ptr("originalAmount"),
					CreditAccount:    ptr("@fees"),
					FreeQuota:        ptr(0),
					Period:           ptr("monthly"),
					Deferrable:       ptr(true),
					Accounting:       &AccountPlanAccountingInput{TransactionRoute: "tr"},
				}).
				WithTimezone("UTC").
				WithLabel("novo").
				WithEnable(true).
				WithDescription("").
				WithRounding(FeeRounding{Scale: 18, Mode: FeeRoundingModeHalfUp}).
				WithEffectiveFrom(time.Date(2027, 1, 1, 3, 0, 0, 0, time.UTC)),
			want: `{"description":"","effectiveFrom":"2027-01-01T03:00:00Z","enable":true,"fees":{"alpha":{"feeLabel":"Alpha","trigger":{"type":"transaction","direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.00"}]},"referenceAmount":"originalAmount","creditAccount":"@fees","freeQuota":0,"period":"monthly","deferrable":true,"accounting":{"transactionRoute":"tr"}},"zeta":null},"label":"novo","rounding":{"scale":18,"mode":"halfUp"},"timezone":"UTC"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, marshalUpdate(t, tt.input))
		})
	}
}

func TestUpdateAccountPlanInput_JSONIsDeterministic(t *testing.T) {
	input := NewUpdateAccountPlanInput()
	for _, key := range []string{"pixOut", "boleto", "tedIn", "aaa", "zzz", "pixIn"} {
		input.WithFeePatch(key, AccountPlanFeePatch{FreeQuota: ptr(1)})
	}

	first := marshalUpdate(t, input)
	for range 20 {
		assert.Equal(t, first, marshalUpdate(t, input))
	}

	assert.Equal(t, `{"fees":{"aaa":{"freeQuota":1},"boleto":{"freeQuota":1},"pixIn":{"freeQuota":1},"pixOut":{"freeQuota":1},"tedIn":{"freeQuota":1},"zzz":{"freeQuota":1}}}`, first)
}

func TestUpdateAccountPlanInput_LastFeeCallWins(t *testing.T) {
	patchThenRemove := NewUpdateAccountPlanInput().
		WithFeePatch("pixIn", AccountPlanFeePatch{FreeQuota: ptr(3)}).
		WithoutFee("pixIn")
	assert.Equal(t, `{"fees":{"pixIn":null}}`, marshalUpdate(t, patchThenRemove))

	removeThenPatch := NewUpdateAccountPlanInput().
		WithoutFee("pixIn").
		WithFeePatch("pixIn", AccountPlanFeePatch{FreeQuota: ptr(3)})
	assert.Equal(t, `{"fees":{"pixIn":{"freeQuota":3}}}`, marshalUpdate(t, removeThenPatch))
}

func TestUpdateAccountPlanInput_FeePatchIsCopied(t *testing.T) {
	patch := AccountPlanFeePatch{FreeQuota: ptr(3)}
	input := NewUpdateAccountPlanInput().WithFeePatch("pixIn", patch)

	patch.FreeQuota = ptr(9)

	assert.Equal(t, `{"fees":{"pixIn":{"freeQuota":3}}}`, marshalUpdate(t, input))
}

func TestUpdateAccountPlanInput_ValidateRejectsNilAndEmpty(t *testing.T) {
	var nilInput *UpdateAccountPlanInput
	require.EqualError(t, nilInput.Validate(), "input cannot be nil")

	require.EqualError(t, NewUpdateAccountPlanInput().Validate(), "empty update payload not allowed")
	require.EqualError(t, (&UpdateAccountPlanInput{Fees: map[string]*AccountPlanFeePatch{}}).Validate(), "empty update payload not allowed")
}

func TestUpdateAccountPlanInput_ValidateFields(t *testing.T) {
	tests := []struct {
		name      string
		input     *UpdateAccountPlanInput
		wantField string
	}{
		{"rounding scale below 0", NewUpdateAccountPlanInput().WithRounding(FeeRounding{Scale: -1, Mode: FeeRoundingModeFloor}), "rounding.scale"},
		{"rounding scale above 18", NewUpdateAccountPlanInput().WithRounding(FeeRounding{Scale: 19, Mode: FeeRoundingModeFloor}), "rounding.scale"},
		{"rounding unknown mode", NewUpdateAccountPlanInput().WithRounding(FeeRounding{Scale: 2, Mode: "ceil"}), "rounding.mode"},
		{"rounding missing mode", NewUpdateAccountPlanInput().WithRounding(FeeRounding{Scale: 2}), "rounding.mode"},
		{"fee key format on patch", NewUpdateAccountPlanInput().WithFeePatch("Pix", AccountPlanFeePatch{FreeQuota: ptr(1)}), "fees[Pix]"},
		{"fee key format on removal", NewUpdateAccountPlanInput().WithoutFee("pix-in"), "fees[pix-in]"},
		{"empty fee patch", NewUpdateAccountPlanInput().WithFeePatch("pixIn", AccountPlanFeePatch{}), "fees[pixIn]"},
		{"negative free quota", NewUpdateAccountPlanInput().WithFeePatch("pixIn", AccountPlanFeePatch{FreeQuota: ptr(-1)}), "fees[pixIn].freeQuota"},
		{"trigger unknown type", NewUpdateAccountPlanInput().WithFeePatch("pixIn", AccountPlanFeePatch{Trigger: &AccountPlanTriggerInput{Type: "batch"}}), "fees[pixIn].trigger.type"},
		{"trigger unknown direction", NewUpdateAccountPlanInput().WithFeePatch("pixIn", AccountPlanFeePatch{Trigger: &AccountPlanTriggerInput{Type: AccountPlanTriggerTypeTransaction, Direction: "both"}}), "fees[pixIn].trigger.direction"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			require.Error(t, err)
			assert.Equal(t, []string{tt.wantField}, accountPlanFieldErrors(t, err))
		})
	}
}

func TestUpdateAccountPlanInput_BuildersOnNil(t *testing.T) {
	var input *UpdateAccountPlanInput

	assert.Nil(t, input.WithLabel("l"))
	assert.Nil(t, input.WithDescription("d"))
	assert.Nil(t, input.WithEnable(true))
	assert.Nil(t, input.WithTimezone("UTC"))
	assert.Nil(t, input.WithRounding(FeeRounding{Scale: 2, Mode: FeeRoundingModeFloor}))
	assert.Nil(t, input.WithEffectiveFrom(time.Now().UTC()))
	assert.Nil(t, input.WithFeePatch("pixIn", AccountPlanFeePatch{FreeQuota: ptr(1)}))
	assert.Nil(t, input.WithoutFee("pixIn"))

	body, err := input.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, "null", string(body))
}

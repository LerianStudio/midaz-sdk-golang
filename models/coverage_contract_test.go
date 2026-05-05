package models

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBalanceSettingsAndInputsContracts(t *testing.T) {
	require.NoError(t, (*BalanceSettings)(nil).Validate())
	assert.Equal(t, BalanceScopeTransactional, NewDefaultBalanceSettings().BalanceScope)

	limit := "100.50"
	tests := []struct {
		name    string
		setting *BalanceSettings
		wantErr string
	}{
		{name: "valid transactional default", setting: &BalanceSettings{BalanceScope: BalanceScopeTransactional}},
		{name: "valid internal overdraft", setting: &BalanceSettings{BalanceScope: BalanceScopeInternal, AllowOverdraft: true, OverdraftLimitEnabled: true, OverdraftLimit: &limit}},
		{name: "invalid scope", setting: &BalanceSettings{BalanceScope: "external"}, wantErr: "balanceScope"},
		{name: "limit forbidden when disabled", setting: &BalanceSettings{OverdraftLimit: &limit}, wantErr: "omitted"},
		{name: "limit enabled requires overdraft allowed", setting: &BalanceSettings{OverdraftLimitEnabled: true, OverdraftLimit: &limit}, wantErr: "allowOverdraft"},
		{name: "limit required when enabled", setting: &BalanceSettings{AllowOverdraft: true, OverdraftLimitEnabled: true}, wantErr: "required"},
		{name: "limit positive decimal", setting: &BalanceSettings{AllowOverdraft: true, OverdraftLimitEnabled: true, OverdraftLimit: ptrString("0")}, wantErr: "positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setting.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}

	require.Error(t, (*UpdateBalanceInput)(nil).Validate())
	require.Error(t, NewUpdateBalanceInput().Validate())
	require.Error(t, NewUpdateBalanceInput().WithMetadata(map[string]any{"legacy": true}).Validate())
	require.Error(t, NewUpdateBalanceInput().WithSettings(&BalanceSettings{BalanceScope: "bad"}).Validate())
	require.NoError(t, NewUpdateBalanceInput().WithAllowSending(false).WithAllowReceiving(true).WithSettings(&BalanceSettings{BalanceScope: BalanceScopeInternal}).Validate())

	updateJSON, err := json.Marshal(NewUpdateBalanceInput().WithAllowSending(false).WithMetadata(map[string]any{"not": "sent"}))
	require.NoError(t, err)
	assert.JSONEq(t, `{"allowSending":false}`, string(updateJSON))

	require.Error(t, (*CreateBalanceInput)(nil).Validate())
	require.Error(t, NewCreateBalanceInput("").Validate())
	require.Error(t, NewCreateBalanceInput("blocked").WithSettings(&BalanceSettings{BalanceScope: "bad"}).Validate())

	create := NewCreateBalanceInput("available").WithAllowSending(true).WithAllowReceiving(false).WithDirection("credit").WithSettings(&BalanceSettings{BalanceScope: BalanceScopeTransactional})
	require.NoError(t, create.Validate())
	createJSON, err := json.Marshal(create)
	require.NoError(t, err)
	assert.JSONEq(t, `{"key":"available","allowSending":true,"allowReceiving":false,"direction":"credit","settings":{"balanceScope":"transactional","allowOverdraft":false,"overdraftLimitEnabled":false}}`, string(createJSON))

	accountsJSON, err := json.Marshal(Accounts{})
	require.NoError(t, err)
	assert.Contains(t, string(accountsJSON), `"items":[]`)

	listJSON, err := json.Marshal(ListAccountResponse{})
	require.NoError(t, err)
	assert.Contains(t, string(listJSON), `"items":[]`)
}

func TestMetadataIndexContracts(t *testing.T) {
	invalid := []*CreateMetadataIndexInput{
		nil,
		NewCreateMetadataIndexInput(""),
		NewCreateMetadataIndexInput(strings.Repeat("a", 101)),
		NewCreateMetadataIndexInput("1bad"),
		NewCreateMetadataIndexInput("bad-key"),
	}

	for _, input := range invalid {
		require.Error(t, input.Validate())
	}

	input := NewCreateMetadataIndexInput("customer_id").WithUnique(true).WithSparse(false)
	require.NoError(t, input.Validate())
	assert.True(t, input.Unique)
	require.NotNil(t, input.Sparse)
	assert.False(t, *input.Sparse)

	assert.Nil(t, (*CreateMetadataIndexInput)(nil).WithUnique(true))
	assert.Nil(t, (*CreateMetadataIndexInput)(nil).WithSparse(true))

	for _, entity := range []string{"organization", "ledger", "segment", "account", "portfolio", "asset", "account_type", "transaction", "operation", "operation_route", "transaction_route"} {
		assert.True(t, IsValidMetadataIndexEntity(entity), entity)
	}

	assert.False(t, IsValidMetadataIndexEntity("holder"))
}

func TestQueueConversionCopiesRawMessages(t *testing.T) {
	organizationID := uuid.New()
	ledgerID := uuid.New()
	auditID := uuid.New()
	accountID := uuid.New()
	queueDataID := uuid.New()
	raw := json.RawMessage(`{"kind":"pending"}`)

	queue := (&Queue{OrganizationID: organizationID, LedgerID: ledgerID, AuditID: auditID, AccountID: accountID}).AddQueueData(queueDataID, raw)
	require.NotNil(t, queue)

	raw[2] = 'X'

	assert.JSONEq(t, `{"kind":"pending"}`, string(queue.QueueData[0].Value))
	assert.Nil(t, (*Queue)(nil).AddQueueData(queueDataID, raw))

	backend := queue.ToMmodelQueue()
	backend.QueueData[0].Value[2] = 'Y'
	assert.JSONEq(t, `{"kind":"pending"}`, string(queue.QueueData[0].Value))

	converted := FromMmodelQueue(mmodel.Queue{
		OrganizationID: organizationID,
		LedgerID:       ledgerID,
		AuditID:        auditID,
		AccountID:      accountID,
		QueueData: []mmodel.QueueData{{
			ID:    queueDataID,
			Value: json.RawMessage(`{"from":"backend"}`),
		}},
	})
	assert.Equal(t, organizationID, converted.OrganizationID)
	assert.JSONEq(t, `{"from":"backend"}`, string(converted.QueueData[0].Value))
	assert.Empty(t, (*Queue)(nil).ToMmodelQueue().QueueData)
}

func TestRouteAndAccountTypeInputContracts(t *testing.T) {
	accountType := NewCreateAccountTypeInput("Cash", "CASH").WithDescription("Liquid cash").WithMetadata(map[string]any{"class": "asset"})
	require.NoError(t, accountType.Validate())
	accountTypeJSON, err := json.Marshal(accountType)
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"Cash","description":"Liquid cash","keyValue":"CASH","metadata":{"class":"asset"}}`, string(accountTypeJSON))

	require.Error(t, NewCreateAccountTypeInput("", "CASH").Validate())
	require.Error(t, NewCreateAccountTypeInput("Cash", "").Validate())
	require.Error(t, NewCreateAccountTypeInput(strings.Repeat("a", 101), "CASH").Validate())
	assert.Nil(t, (*CreateAccountTypeInput)(nil).WithDescription("x"))
	assert.Nil(t, (*CreateAccountTypeInput)(nil).WithMetadata(map[string]any{"x": "y"}))

	updateAccountType := NewUpdateAccountTypeInput().WithName("Cash Updated").WithDescription("Updated").WithMetadata(map[string]any{"v": 2})
	require.NoError(t, updateAccountType.Validate())
	updateAccountTypeJSON, err := json.Marshal(updateAccountType)
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"Cash Updated","description":"Updated","metadata":{"v":2}}`, string(updateAccountTypeJSON))
	require.Error(t, NewUpdateAccountTypeInput().Validate())
	assert.Nil(t, (*UpdateAccountTypeInput)(nil).WithName("x"))
	assert.Nil(t, (*UpdateAccountTypeInput)(nil).WithDescription("x"))
	assert.Nil(t, (*UpdateAccountTypeInput)(nil).WithMetadata(map[string]any{"x": "y"}))

	operationRoute := NewCreateOperationRouteInput("Source", "Funding source", "source").WithAccountTypes([]string{"external"}).WithMetadata(map[string]any{"flow": "funding"})
	require.NoError(t, operationRoute.Validate())
	assert.Nil(t, (*CreateOperationRouteInput)(nil).WithAccountAlias("alias"))
	assert.Nil(t, (*CreateOperationRouteInput)(nil).WithAccountTypes([]string{"cash"}))
	assert.Nil(t, (*CreateOperationRouteInput)(nil).WithMetadata(map[string]any{"x": "y"}))
	require.Error(t, NewCreateOperationRouteInput("", "desc", "source").Validate())
	require.Error(t, NewCreateOperationRouteInput("Title", "desc", "debit").Validate())

	updateOperationRoute := (&UpdateOperationRouteInput{}).WithTitle("Destination").WithDescription("dest route").WithAccountTypes([]string{"settlement"}).WithMetadata(map[string]any{"flow": "settlement"})
	require.NoError(t, updateOperationRoute.Validate())
	assert.Nil(t, (*UpdateOperationRouteInput)(nil).WithTitle("x"))
	assert.Nil(t, (*UpdateOperationRouteInput)(nil).WithDescription("x"))
	assert.Nil(t, (*UpdateOperationRouteInput)(nil).WithAccountTypes([]string{"x"}))
	assert.Nil(t, (*UpdateOperationRouteInput)(nil).WithMetadata(map[string]any{"x": "y"}))
	require.Error(t, (&UpdateOperationRouteInput{}).Validate())

	operationRouteID := uuid.NewString()
	txRoute := NewCreateTransactionRouteInput("Funding", "Funding route", []string{operationRouteID}).WithMetadata(map[string]any{"flow": "funding"})
	require.NoError(t, txRoute.Validate())
	require.Error(t, NewCreateTransactionRouteInput("Funding", "Funding route", []string{"not-a-uuid"}).Validate())
	require.Error(t, NewCreateTransactionRouteInput("Funding", "Funding route", nil).Validate())
	assert.Nil(t, (*CreateTransactionRouteInput)(nil).WithMetadata(map[string]any{"x": "y"}))

	updateTxRoute := NewUpdateTransactionRouteInput().
		WithTitle("Funding Updated").
		WithDescription("desc").
		WithMetadata(map[string]any{"v": 1})
	require.NoError(t, updateTxRoute.Validate())
	require.Error(t, NewUpdateTransactionRouteInput().Validate())
	assert.Nil(t, (*UpdateTransactionRouteInput)(nil).WithTitle("x"))
	assert.Nil(t, (*UpdateTransactionRouteInput)(nil).WithDescription("x"))
	assert.Nil(t, (*UpdateTransactionRouteInput)(nil).WithMetadata(map[string]any{"x": "y"}))
}

func TestCoreResourceInputValidationAndMarshalContracts(t *testing.T) {
	ledger := NewCreateLedgerInput("Main").WithStatus(NewStatus(StatusActive)).WithMetadata(map[string]any{"department": "ops"})
	require.NoError(t, ledger.Validate())
	ledgerJSON, err := json.Marshal(ledger)
	require.NoError(t, err)
	assert.Contains(t, string(ledgerJSON), `"name":"Main"`)
	assert.Nil(t, (*CreateLedgerInput)(nil).WithStatus(NewStatus(StatusActive)))
	assert.Nil(t, (*CreateLedgerInput)(nil).WithMetadata(map[string]any{"x": "y"}))
	require.Error(t, NewCreateLedgerInput("").Validate())
	require.Error(t, NewCreateLedgerInput(strings.Repeat("a", 257)).Validate())

	ledgerUpdate := NewUpdateLedgerInput().WithName("Main Updated").WithStatus(NewStatus(StatusInactive)).WithMetadata(map[string]any{"department": "finance"})
	require.NoError(t, ledgerUpdate.Validate())
	ledgerUpdateJSON, err := json.Marshal(ledgerUpdate)
	require.NoError(t, err)
	assert.Contains(t, string(ledgerUpdateJSON), `"name":"Main Updated"`)
	require.Error(t, NewUpdateLedgerInput().Validate())

	portfolio := NewCreatePortfolioInput("entity-1", "Retail").WithStatus(NewStatus(StatusActive)).WithEntityID("entity-2").WithMetadata(map[string]any{"segment": "retail"})
	require.NoError(t, portfolio.Validate())
	portfolioJSON, err := json.Marshal(portfolio)
	require.NoError(t, err)
	assert.Contains(t, string(portfolioJSON), `"entityId":"entity-2"`)
	require.Error(t, NewCreatePortfolioInput("entity", "").Validate())

	portfolioUpdate := NewUpdatePortfolioInput().WithName("Retail Updated").WithEntityID("entity-3").WithStatus(NewStatus(StatusInactive)).WithMetadata(map[string]any{"segment": "private"})
	require.NoError(t, portfolioUpdate.Validate())
	portfolioUpdateJSON, err := json.Marshal(portfolioUpdate)
	require.NoError(t, err)
	assert.Contains(t, string(portfolioUpdateJSON), `"name":"Retail Updated"`)
	require.Error(t, NewUpdatePortfolioInput().Validate())

	org := NewCreateOrganizationInput("Acme", "123456789").WithDoingBusinessAs("Acme Pay").WithStatus(NewStatus(StatusActive)).WithAddress(Address{Line1: "Avenida Paulista", City: "Sao Paulo", State: "SP", ZipCode: "01310-100", Country: "BR"}).WithMetadata(map[string]any{"industry": "fintech"})
	require.NoError(t, org.Validate())
	assert.NotNil(t, org.ToMmodelCreateOrganizationInput())
	orgJSON, err := json.Marshal(org)
	require.NoError(t, err)
	assert.Contains(t, string(orgJSON), `"legalName":"Acme"`)
	require.Error(t, NewCreateOrganizationInput("", "doc").Validate())
	require.Error(t, NewCreateOrganizationInput("Acme", "").Validate())

	orgUpdate := (&UpdateOrganizationInput{}).WithLegalName("Acme Updated").WithDoingBusinessAs("Acme Bank").WithAddress(Address{Line1: "Rua A", City: "Rio", State: "RJ", ZipCode: "20000-000", Country: "BR"}).WithStatus(NewStatus(StatusInactive)).WithMetadata(map[string]any{"industry": "banking"})
	require.NoError(t, orgUpdate.Validate())
	assert.NotNil(t, orgUpdate.ToMmodelUpdateOrganizationInput())
	orgUpdateJSON, err := json.Marshal(orgUpdate)
	require.NoError(t, err)
	assert.Contains(t, string(orgUpdateJSON), `"legalName":"Acme Updated"`)
	require.Error(t, (&UpdateOrganizationInput{}).Validate())
}

func ptrString(value string) *string {
	return &value
}

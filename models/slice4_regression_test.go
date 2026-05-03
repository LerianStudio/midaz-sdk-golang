package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlice4UpdateInputs_OmitUnsetPatchFields(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{name: "organization legal name only", input: NewUpdateOrganizationInput().WithLegalName("Lerian"), want: `{"legalName":"Lerian"}`},
		{name: "ledger name only", input: NewUpdateLedgerInput().WithName("Main"), want: `{"name":"Main"}`},
		{name: "asset name only", input: NewUpdateAssetInput().WithName("USD"), want: `{"name":"USD"}`},
		{name: "account type name only", input: NewUpdateAccountTypeInput().WithName("Deposit"), want: `{"name":"Deposit"}`},
		{name: "portfolio name only", input: NewUpdatePortfolioInput().WithName("Retail"), want: `{"name":"Retail"}`},
		{name: "segment name only", input: NewUpdateSegmentInput().WithName("BR"), want: `{"name":"BR"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.input)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(body))
			assert.NotContains(t, string(body), "metadata")
			assert.NotContains(t, string(body), "status")
		})
	}
}

func TestSlice4ModelNilSafety(t *testing.T) {
	var (
		createOrg         *CreateOrganizationInput
		updateOrg         *UpdateOrganizationInput
		createLedger      *CreateLedgerInput
		updateLedger      *UpdateLedgerInput
		createAccount     *CreateAccountInput
		updateAccount     *UpdateAccountInput
		createAccountType *CreateAccountTypeInput
		updateAccountType *UpdateAccountTypeInput
		createAsset       *CreateAssetInput
		updateAsset       *UpdateAssetInput
		createPortfolio   *CreatePortfolioInput
		updatePortfolio   *UpdatePortfolioInput
		createSegment     *CreateSegmentInput
		updateSegment     *UpdateSegmentInput
	)

	validators := []func() error{
		createOrg.Validate,
		updateOrg.Validate,
		createLedger.Validate,
		updateLedger.Validate,
		createAccount.Validate,
		updateAccount.Validate,
		createAccountType.Validate,
		updateAccountType.Validate,
		createAsset.Validate,
		updateAsset.Validate,
		createPortfolio.Validate,
		updatePortfolio.Validate,
		createSegment.Validate,
		updateSegment.Validate,
	}

	for _, validate := range validators {
		require.Error(t, validate())
	}

	require.Nil(t, createLedger.WithStatus(NewStatus("ACTIVE")))
	require.Nil(t, updateLedger.WithName("Main"))
	require.Nil(t, createAccount.WithAlias("alias"))
	require.Nil(t, updateAccount.WithBlocked(true))
	require.Nil(t, createAccountType.WithDescription("desc"))
	require.Nil(t, updateAccountType.WithMetadata(map[string]any{"k": "v"}))
	require.Nil(t, createAsset.WithType("currency"))
	require.Nil(t, updateAsset.WithName("USD"))
	require.Nil(t, createPortfolio.WithEntityID("entity"))
	require.Nil(t, updatePortfolio.WithEntityID("entity"))
	require.Nil(t, createSegment.WithMetadata(map[string]any{"k": "v"}))
	require.Nil(t, updateSegment.WithName("BR"))
}

func TestSlice4AssetAndPortfolioContracts(t *testing.T) {
	asset := NewCreateAssetInputWithType("US Dollar", "USD", "currency")
	require.NoError(t, asset.Validate())

	legacyAsset := NewCreateAssetInput("US Dollar", "USD")
	require.ErrorContains(t, legacyAsset.Validate(), "type is required")

	portfolio := NewCreatePortfolioInput("", "Retail")
	require.NoError(t, portfolio.Validate())

	body, err := json.Marshal(portfolio)
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"Retail"}`, string(body))
}

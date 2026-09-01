// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleFeeEstimateSend() *SendInput {
	return &SendInput{
		Asset: "USD",
		Value: "100",
		Source: &SourceInput{From: []FromToInput{
			{AccountAlias: "@src", Amount: AmountInput{Asset: "USD", Value: "100"}},
		}},
		Distribute: &DistributeInput{To: []FromToInput{
			{AccountAlias: "@dst", Amount: AmountInput{Asset: "USD", Value: "100"}},
		}},
	}
}

func TestNewFeeEstimateInput_ValidateAccumulatesFieldKeys(t *testing.T) {
	tests := []struct {
		name    string
		input   *FeeEstimateInput
		wantErr string
	}{
		{
			name: "valid with optionals",
			input: NewFeeEstimateInput("pkg-1", sampleFeeEstimateSend()).
				WithChartOfAccountsGroupName("FUNDING").
				WithDescription("d").
				WithCode("c").
				WithPending(true).
				WithMetadata(map[string]any{"k": "v"}).
				WithRouteID("route-1").
				WithTransactionDate("2026-01-01T00:00:00Z"),
			wantErr: "",
		},
		{name: "missing packageId", input: NewFeeEstimateInput("", sampleFeeEstimateSend()), wantErr: "packageId"},
		{name: "missing send", input: NewFeeEstimateInput("pkg-1", nil), wantErr: "transaction.send"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestNewFeeEstimateInput_SetsOptionalTransactionFields(t *testing.T) {
	in := NewFeeEstimateInput("pkg-1", sampleFeeEstimateSend()).
		WithChartOfAccountsGroupName("FUNDING").
		WithDescription("desc").
		WithCode("code").
		WithPending(true).
		WithRouteID("route-1").
		WithTransactionDate("2026-01-01T00:00:00Z").
		WithMetadata(map[string]any{"k": "v"})

	assert.Equal(t, "pkg-1", in.PackageID)
	assert.Equal(t, "FUNDING", in.Transaction.ChartOfAccountsGroupName)
	assert.Equal(t, "desc", in.Transaction.Description)
	assert.Equal(t, "code", in.Transaction.Code)
	assert.True(t, in.Transaction.Pending)
	require.NotNil(t, in.Transaction.RouteID)
	assert.Equal(t, "route-1", *in.Transaction.RouteID)
	assert.Equal(t, "2026-01-01T00:00:00Z", in.Transaction.TransactionDate)
	assert.Equal(t, "v", in.Transaction.Metadata["k"])
	assert.NotNil(t, in.Transaction.Send)
}

func TestNewBillingCalculateInput_ValidateAccumulatesFieldKeys(t *testing.T) {
	tests := []struct {
		name    string
		input   *BillingCalculateInput
		wantErr string
	}{
		{name: "valid with type", input: NewBillingCalculateInput("2026-01").WithType("volume"), wantErr: ""},
		{name: "valid without type", input: NewBillingCalculateInput("2026-01"), wantErr: ""},
		{name: "missing period", input: NewBillingCalculateInput(""), wantErr: "period"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestNewBillingCalculateInput_SetsFields(t *testing.T) {
	in := NewBillingCalculateInput("2026-W13").WithType("maintenance")

	assert.Equal(t, "2026-W13", in.Period)
	assert.Equal(t, "maintenance", in.Type)
}

func TestNewCreateHolderAccountInput_ValidateAccumulatesFieldKeys(t *testing.T) {
	tests := []struct {
		name    string
		input   *CreateHolderAccountInput
		wantErr string
	}{
		{name: "valid", input: NewCreateHolderAccountInput("USD", "deposit").WithName("Customer"), wantErr: ""},
		{name: "missing assetCode", input: NewCreateHolderAccountInput("", "deposit"), wantErr: "assetCode"},
		{name: "missing type", input: NewCreateHolderAccountInput("USD", ""), wantErr: "type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestNewCreateHolderAccountInput_SetsOptionalFields(t *testing.T) {
	in := NewCreateHolderAccountInput("USD", "deposit").
		WithName("Customer").
		WithParentAccountID("parent-1").
		WithEntityID("entity-1").
		WithPortfolioID("portfolio-1").
		WithSegmentID("segment-1").
		WithAlias("@customer").
		WithBlocked(true).
		WithMetadata(map[string]any{"k": "v"}).
		WithSkip(&AccountSkip{Holder: true}).
		WithBankingDetails(&BankingDetails{}).
		WithRegulatoryFields(&RegulatoryFields{}).
		WithRelatedParties([]*RelatedParty{{}})

	assert.Equal(t, "USD", in.AssetCode)
	assert.Equal(t, "deposit", in.Type)
	assert.Equal(t, "Customer", in.Name)
	require.NotNil(t, in.ParentAccountID)
	assert.Equal(t, "parent-1", *in.ParentAccountID)
	require.NotNil(t, in.EntityID)
	assert.Equal(t, "entity-1", *in.EntityID)
	require.NotNil(t, in.PortfolioID)
	assert.Equal(t, "portfolio-1", *in.PortfolioID)
	require.NotNil(t, in.SegmentID)
	assert.Equal(t, "segment-1", *in.SegmentID)
	require.NotNil(t, in.Alias)
	assert.Equal(t, "@customer", *in.Alias)
	require.NotNil(t, in.Blocked)
	assert.True(t, *in.Blocked)
	assert.Equal(t, "v", in.Metadata["k"])
	require.NotNil(t, in.Skip)
	assert.True(t, in.Skip.Holder)
	assert.NotNil(t, in.BankingDetails)
	assert.NotNil(t, in.RegulatoryFields)
	assert.Len(t, in.RelatedParties, 1)
}

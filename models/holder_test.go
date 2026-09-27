package models

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The field names are Midaz's /v2 holder contract; a rename on either side breaks this test.
func TestHolderFinancialFiguresWireNames(t *testing.T) {
	income := decimal.RequireFromString("12500.50")
	revenue := decimal.RequireFromString("4800000")
	assets := decimal.RequireFromString("0")

	input := NewCreateHolderInput(HolderTypeLegalPerson, "Acme SA", "12345678000199").
		WithNaturalPerson(&NaturalPerson{MonthlyGrossIncome: &MonetaryAmount{Value: &income, Currency: "BRL", ReferenceDate: "2026-06-30"}}).
		WithLegalPerson(&LegalPerson{
			AnnualGrossRevenue: &MonetaryAmount{Value: &revenue, Currency: "BRL", ReferenceDate: "2025-12-31"},
			TotalAssets:        &MonetaryAmount{Value: &assets, Currency: "USD", ReferenceDate: "2025-12-31"},
		})
	require.NoError(t, input.Validate())

	data, err := json.Marshal(input)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"type":"LEGAL_PERSON","name":"Acme SA","document":"12345678000199",
		"naturalPerson":{"monthlyGrossIncome":{"value":"12500.5","currency":"BRL","referenceDate":"2026-06-30"}},
		"legalPerson":{
			"annualGrossRevenue":{"value":"4800000","currency":"BRL","referenceDate":"2025-12-31"},
			"totalAssets":{"value":"0","currency":"USD","referenceDate":"2025-12-31"}
		}
	}`, string(data))

	var holder Holder
	require.NoError(t, json.Unmarshal(data, &holder))
	require.NotNil(t, holder.NaturalPerson.MonthlyGrossIncome)
	assert.True(t, income.Equal(*holder.NaturalPerson.MonthlyGrossIncome.Value))
	assert.True(t, revenue.Equal(*holder.LegalPerson.AnnualGrossRevenue.Value))
	assert.Equal(t, "USD", holder.LegalPerson.TotalAssets.Currency)
	assert.Equal(t, "2025-12-31", holder.LegalPerson.TotalAssets.ReferenceDate)
}

func TestHolderFinancialFiguresValidation(t *testing.T) {
	negative := decimal.RequireFromString("-0.01")
	ten := decimal.RequireFromString("10")

	tests := []struct {
		name      string
		figure    *MonetaryAmount
		wantField string
	}{
		{name: "well formed", figure: &MonetaryAmount{Value: &ten, Currency: "BRL", ReferenceDate: "2026-06-30"}},
		{name: "missing value", figure: &MonetaryAmount{Currency: "BRL", ReferenceDate: "2026-06-30"}, wantField: "legalPerson.totalAssets.value"},
		{name: "negative value", figure: &MonetaryAmount{Value: &negative, Currency: "BRL", ReferenceDate: "2026-06-30"}, wantField: "legalPerson.totalAssets.value"},
		{name: "lowercase currency", figure: &MonetaryAmount{Value: &ten, Currency: "brl", ReferenceDate: "2026-06-30"}, wantField: "legalPerson.totalAssets.currency"},
		{name: "missing currency", figure: &MonetaryAmount{Value: &ten, ReferenceDate: "2026-06-30"}, wantField: "legalPerson.totalAssets.currency"},
		{name: "impossible date", figure: &MonetaryAmount{Value: &ten, Currency: "BRL", ReferenceDate: "2026-02-30"}, wantField: "legalPerson.totalAssets.referenceDate"},
		{name: "RFC3339 date", figure: &MonetaryAmount{Value: &ten, Currency: "BRL", ReferenceDate: "2026-06-30T00:00:00Z"}, wantField: "legalPerson.totalAssets.referenceDate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			create := NewCreateHolderInput(HolderTypeLegalPerson, "Acme SA", "12345678000199").
				WithLegalPerson(&LegalPerson{TotalAssets: tt.figure})
			update := NewUpdateHolderInput().WithLegalPerson(&LegalPerson{TotalAssets: tt.figure})

			for _, err := range []error{create.Validate(), update.Validate()} {
				if tt.wantField == "" {
					require.NoError(t, err)
					continue
				}

				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantField)
			}
		})
	}
}

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

	person := NewCreateHolderInput(HolderTypeNaturalPerson, "Jane Doe", "12345678900").
		WithNaturalPerson(&NaturalPerson{MonthlyGrossIncome: &MonetaryAmount{Value: &income, Currency: "BRL", ReferenceDate: "2026-06-30"}})
	company := NewCreateHolderInput(HolderTypeLegalPerson, "Acme SA", "12345678000199").
		WithLegalPerson(&LegalPerson{
			AnnualGrossRevenue: &MonetaryAmount{Value: &revenue, Currency: "BRL", ReferenceDate: "2025-12-31"},
			TotalAssets:        &MonetaryAmount{Value: &assets, Currency: "USD", ReferenceDate: "2025-12-31"},
		})

	for _, tt := range []struct {
		input *CreateHolderInput
		want  string
	}{
		{input: person, want: `{
			"type":"NATURAL_PERSON","name":"Jane Doe","document":"12345678900",
			"naturalPerson":{"monthlyGrossIncome":{"value":"12500.5","currency":"BRL","referenceDate":"2026-06-30"}}
		}`},
		{input: company, want: `{
			"type":"LEGAL_PERSON","name":"Acme SA","document":"12345678000199",
			"legalPerson":{
				"annualGrossRevenue":{"value":"4800000","currency":"BRL","referenceDate":"2025-12-31"},
				"totalAssets":{"value":"0","currency":"USD","referenceDate":"2025-12-31"}
			}
		}`},
	} {
		require.NoError(t, tt.input.Validate())

		data, err := json.Marshal(tt.input)
		require.NoError(t, err)
		assert.JSONEq(t, tt.want, string(data))

		var holder Holder
		require.NoError(t, json.Unmarshal(data, &holder))

		sent, err := json.Marshal([]any{tt.input.NaturalPerson, tt.input.LegalPerson})
		require.NoError(t, err)
		read, err := json.Marshal([]any{holder.NaturalPerson, holder.LegalPerson})
		require.NoError(t, err)
		assert.JSONEq(t, string(sent), string(read), "a Holder read back must carry the same figures")
	}
}

func TestHolderFinancialFiguresValidation(t *testing.T) {
	dec := decimal.RequireFromString

	tests := []struct {
		name      string
		figure    MonetaryAmount
		wantField string
	}{
		{name: "well formed", figure: MonetaryAmount{Value: ptr(dec("10")), Currency: "BRL", ReferenceDate: "2026-06-30"}},
		{name: "largest bounded value", figure: MonetaryAmount{Value: ptr(dec("99999999999999999999.9999999999")), Currency: "BRL", ReferenceDate: "2026-06-30"}},
		{name: "missing value", figure: MonetaryAmount{Currency: "BRL", ReferenceDate: "2026-06-30"}, wantField: ".value"},
		{name: "negative value", figure: MonetaryAmount{Value: ptr(dec("-0.01")), Currency: "BRL", ReferenceDate: "2026-06-30"}, wantField: ".value"},
		{name: "21 integer digits", figure: MonetaryAmount{Value: ptr(decimal.New(1, 20)), Currency: "BRL", ReferenceDate: "2026-06-30"}, wantField: ".value"},
		{name: "11 fraction digits", figure: MonetaryAmount{Value: ptr(dec("1.12345678901")), Currency: "BRL", ReferenceDate: "2026-06-30"}, wantField: ".value"},
		{name: "huge exponent", figure: MonetaryAmount{Value: ptr(decimal.New(1, 1000000)), Currency: "BRL", ReferenceDate: "2026-06-30"}, wantField: ".value"},
		{name: "lowercase currency", figure: MonetaryAmount{Value: ptr(dec("10")), Currency: "brl", ReferenceDate: "2026-06-30"}, wantField: ".currency"},
		{name: "currency not in ISO 4217", figure: MonetaryAmount{Value: ptr(dec("10")), Currency: "ZZZ", ReferenceDate: "2026-06-30"}, wantField: ".currency"},
		{name: "missing currency", figure: MonetaryAmount{Value: ptr(dec("10")), ReferenceDate: "2026-06-30"}, wantField: ".currency"},
		{name: "impossible date", figure: MonetaryAmount{Value: ptr(dec("10")), Currency: "BRL", ReferenceDate: "2026-02-30"}, wantField: ".referenceDate"},
		{name: "RFC3339 date", figure: MonetaryAmount{Value: ptr(dec("10")), Currency: "BRL", ReferenceDate: "2026-06-30T00:00:00Z"}, wantField: ".referenceDate"},
	}

	paths := map[string]func(*MonetaryAmount) (*CreateHolderInput, *UpdateHolderInput){
		"naturalPerson.monthlyGrossIncome": func(f *MonetaryAmount) (*CreateHolderInput, *UpdateHolderInput) {
			np := &NaturalPerson{MonthlyGrossIncome: f}
			return NewCreateHolderInput(HolderTypeNaturalPerson, "Jane Doe", "12345678900").WithNaturalPerson(np), NewUpdateHolderInput().WithNaturalPerson(np)
		},
		"legalPerson.annualGrossRevenue": func(f *MonetaryAmount) (*CreateHolderInput, *UpdateHolderInput) {
			lp := &LegalPerson{AnnualGrossRevenue: f}
			return NewCreateHolderInput(HolderTypeLegalPerson, "Acme SA", "12345678000199").WithLegalPerson(lp), NewUpdateHolderInput().WithLegalPerson(lp)
		},
		"legalPerson.totalAssets": func(f *MonetaryAmount) (*CreateHolderInput, *UpdateHolderInput) {
			lp := &LegalPerson{TotalAssets: f}
			return NewCreateHolderInput(HolderTypeLegalPerson, "Acme SA", "12345678000199").WithLegalPerson(lp), NewUpdateHolderInput().WithLegalPerson(lp)
		},
	}

	for path, inputs := range paths {
		for _, tt := range tests {
			t.Run(path+"/"+tt.name, func(t *testing.T) {
				figure := tt.figure
				create, update := inputs(&figure)

				for _, err := range []error{create.Validate(), update.Validate()} {
					if tt.wantField == "" {
						require.NoError(t, err)
						continue
					}

					require.Error(t, err)
					assert.Contains(t, err.Error(), path+tt.wantField)
				}
			})
		}
	}
}

func TestUpdateHolderInputRemovesOneFigure(t *testing.T) {
	input := NewUpdateHolderInput().
		WithNaturalPerson(&NaturalPerson{MotherName: ptr("Maria Doe")}).
		WithNullFields("naturalPerson.monthlyGrossIncome", "legalPerson.totalAssets")
	require.NoError(t, input.Validate())

	data, err := json.Marshal(input)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"naturalPerson":{"motherName":"Maria Doe","monthlyGrossIncome":null},
		"legalPerson":{"totalAssets":null}
	}`, string(data))

	assets := decimal.RequireFromString("10")
	conflict := NewUpdateHolderInput().
		WithLegalPerson(&LegalPerson{TotalAssets: &MonetaryAmount{Value: &assets, Currency: "BRL", ReferenceDate: "2026-06-30"}}).
		WithNullField("legalPerson.totalAssets")
	require.ErrorContains(t, conflict.Validate(), "cannot be set and cleared")

	_, err = json.Marshal(conflict)
	require.ErrorContains(t, err, "cannot be set and cleared")
}

func ptr[T any](value T) *T {
	return &value
}

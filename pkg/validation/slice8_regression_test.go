package validation

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type nilAccountDSL struct{}

func (nilAccountDSL) GetAsset() string                           { return "USD" }
func (nilAccountDSL) GetValue() float64                          { return 1 }
func (nilAccountDSL) GetSourceAccounts() []AccountReference      { return []AccountReference{nil} }
func (nilAccountDSL) GetDestinationAccounts() []AccountReference { return []AccountReference{nil} }
func (nilAccountDSL) GetMetadata() map[string]any                { return nil }

func TestSlice8ValidationContracts(t *testing.T) {
	t.Run("send based transaction payload is accepted", func(t *testing.T) {
		input := map[string]any{
			"send": map[string]any{
				"asset": "BRL",
				"value": "1000",
				"source": map[string]any{
					"from": []any{map[string]any{
						"accountAlias": "@person1",
						"amount":       map[string]any{"asset": "BRL", "value": "1000"},
					}},
				},
				"distribute": map[string]any{
					"to": []any{map[string]any{
						"accountAlias": "@person2",
						"amount":       map[string]any{"asset": "BRL", "value": "1000"},
					}},
				},
			},
		}

		summary := ValidateCreateTransactionInput(input)
		require.True(t, summary.Valid, summary.GetErrorSummary())
		require.False(t, EnhancedValidateTransactionInput(input).HasErrors())
	})

	t.Run("public alias and asset validators match Midaz contract", func(t *testing.T) {
		require.NoError(t, ValidateAccountAlias("@treasury_checking"))
		require.NoError(t, ValidateAccountAlias(strings.Repeat("a", 100)))
		require.NoError(t, ValidateAssetCode("CUSTOMASSET"))
		require.NoError(t, ValidateAccountType("custom_liability"))
		require.Error(t, ValidateAccountType("external"))
	})

	t.Run("metadata allows Midaz numeric and key limits", func(t *testing.T) {
		metadata := map[string]any{
			strings.Repeat("k", 100): int64(123),
			"path.with.dot":          []any{"ok", int64(1), true},
		}

		require.NoError(t, ValidateMetadata(metadata))
		require.False(t, EnhancedValidateMetadata(metadata).HasErrors())
	})

	t.Run("address line accepts 256 characters", func(t *testing.T) {
		line := strings.Repeat("a", 256)
		require.NoError(t, ValidateAddress(&Address{
			Line1:   line,
			ZipCode: "12345",
			City:    "Sao Paulo",
			State:   "SP",
			Country: "BR",
		}))
	})

	t.Run("non finite values are rejected", func(t *testing.T) {
		require.False(t, ValidateCreateTransactionInput(map[string]any{
			"asset_code": "USD",
			"amount":     math.NaN(),
			"scale":      2,
			"operations": []map[string]any{},
		}).Valid)

		require.Error(t, ValidateMetadata(map[string]any{"nan": math.NaN()}))
	})

	t.Run("nil helpers do not panic", func(t *testing.T) {
		var validator *Validator
		require.Error(t, validator.ValidateMetadata(map[string]any{"a": "b"}))
		require.Error(t, validator.ValidateAddress(&Address{}))
		require.Error(t, ValidateTransactionDSL(nilAccountDSL{}))
		require.True(t, EnhancedValidateTransactionDSL(nilAccountDSL{}).HasErrors())

		summary := Summary{Valid: true}
		summary.AddError(nil)
		require.True(t, summary.Valid)

		fieldErrors := NewFieldErrors()
		fieldErrors.AddError(nil)
		require.False(t, fieldErrors.HasErrors())
		require.Nil(t, WrapError("field", nil, nil))
		require.NotNil(t, WrapError("field", nil, errors.New("boom")))
	})
}

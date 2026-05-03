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
		// Aliases support an optional leading '@' and accept the documented
		// punctuation set. The cap is now 50 characters; longer values are
		// rejected at the SDK boundary instead of being forwarded to the
		// backend.
		require.NoError(t, ValidateAccountAlias("@treasury_checking"))
		require.NoError(t, ValidateAccountAlias(strings.Repeat("a", 50)))
		require.Error(t, ValidateAccountAlias(strings.Repeat("a", 100)))

		// Asset codes must be 3-4 uppercase letters (ISO-4217-ish).
		require.NoError(t, ValidateAssetCode("USD"))
		require.NoError(t, ValidateAssetCode("USDT"))
		require.Error(t, ValidateAssetCode("CUSTOMASSET"))

		// Account type is a strict allowlist; "custom_liability" is NOT on
		// it, but "liability" is.
		require.NoError(t, ValidateAccountType("liability"))
		require.Error(t, ValidateAccountType("custom_liability"))
		require.Error(t, ValidateAccountType("external"))
	})

	t.Run("metadata allows Midaz numeric and key limits", func(t *testing.T) {
		// Restored Mongo-key safety: '.' and '$' are reserved at the storage
		// layer and must be rejected at the SDK boundary. The previous
		// permissive shape allowed dotted keys to flow through and confuse
		// downstream path-aware indexes.
		metadata := map[string]any{
			strings.Repeat("k", 100): int64(123),
			"safe_array_key":         []any{"ok", int64(1), true},
		}

		require.NoError(t, ValidateMetadata(metadata))
		require.False(t, EnhancedValidateMetadata(metadata).HasErrors())

		// And confirm the dotted-key rejection round-trips.
		require.Error(t, ValidateMetadata(map[string]any{"path.with.dot": "x"}))
		require.Error(t, ValidateMetadata(map[string]any{"$reserved": "x"}))
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

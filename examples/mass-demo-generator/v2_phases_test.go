package main

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// TestV2ProofAmountsAgreeAcrossScales locks the one piece of arithmetic in the
// V2 transaction proof that can be wrong without anything failing loudly.
//
// The proof SENDS amounts formatted by formatAmountByScale (a decimal string
// built from minor units) and ASSERTS balances built by shifting a
// decimal.Decimal by the same scale. Those are two independent conversions of
// the same number. If they ever disagree, the proof does not error — it asserts
// the WRONG expected balance, which is worse than no assertion at all: a real
// money-path defect could pass, or a healthy ledger could fail the run.
//
// Every number here comes from v2ProofAmountsFor, the same call the proof makes.
// An earlier version of this test re-declared the quantities locally, which made
// it prove only that the test agreed with itself — a wrong production
// expectation stayed green.
//
// Scale 0 is included because it takes a different branch in
// formatAmountByScale (no fractional part at all).
func TestV2ProofAmountsAgreeAcrossScales(t *testing.T) {
	for _, scale := range []int{0, 2, 6} {
		amounts := v2ProofAmountsFor(pow10(scale))

		cases := []struct {
			name  string
			minor int64
		}{
			{"fundSource", amounts.fundSource},
			{"fundDest", amounts.fundDest},
			{"transfer", amounts.transfer},
			{"held", amounts.held},
			{"canceled", amounts.canceled},
			{"expectedSource", amounts.expectedSource},
			{"expectedDest", amounts.expectedDest},
		}

		for _, tc := range cases {
			sent := formatAmountByScale(tc.minor, int64(scale))
			asserted := decimal.NewFromInt(tc.minor).Shift(int32(-scale))

			parsed, err := decimal.NewFromString(sent)
			require.NoErrorf(t, err, "scale %d: %s formatted as %q is not a decimal", scale, tc.name, sent)
			require.Truef(t, parsed.Equal(asserted),
				"scale %d: %s is sent as %q but asserted as %q", scale, tc.name, sent, asserted.String())
		}

		// Double-entry over the whole cycle: what leaves the external account
		// is exactly what the two internal accounts end up holding.
		injected := decimal.NewFromInt(amounts.fundSource + amounts.fundDest).Shift(int32(-scale))
		settled := decimal.NewFromInt(amounts.expectedSource).Shift(int32(-scale)).
			Add(decimal.NewFromInt(amounts.expectedDest).Shift(int32(-scale)))
		require.Truef(t, injected.Equal(settled),
			"scale %d: funded %s but the demo accounts end up holding %s", scale, injected, settled)
	}
}

// TestV2ProofCanceledHoldNetsToZero pins the meaning of the released-hold leg:
// its value must not appear in either expected balance.
//
// The leg exists to prove the cancel path returns held value to the source. If
// an expectation ever absorbed it — expectedSource lowered by the canceled
// amount, say — the run would pass whether the cancel released the value or kept
// it, which is the whole reason the leg was added.
func TestV2ProofCanceledHoldNetsToZero(t *testing.T) {
	for _, scale := range []int{0, 2, 6} {
		unit := pow10(scale)
		amounts := v2ProofAmountsFor(unit)

		require.Positivef(t, amounts.canceled, "scale %d: the released hold must move a real amount", scale)

		// Same expectations as a cycle without the canceled leg at all.
		require.Equalf(t, amounts.expectedSource, amounts.fundSource-amounts.transfer-amounts.held,
			"scale %d: the canceled hold must leave the source expectation untouched", scale)
		require.Equalf(t, amounts.expectedDest, amounts.fundDest+amounts.transfer+amounts.held,
			"scale %d: the canceled hold must leave the destination expectation untouched", scale)
	}
}

// TestDefaultAssetBalancesPrefersTheDefaultKey pins the balance the proof
// asserts on. A keyed balance beside the default holds value the demo never
// moved, and matching on asset code alone made the verdict depend on the order
// the server listed them in.
func TestDefaultAssetBalancesPrefersTheDefaultKey(t *testing.T) {
	balance := func(asset, key string) models.Balance {
		return models.Balance{AssetCode: asset, Key: key}
	}

	t.Run("default beside a keyed balance", func(t *testing.T) {
		items := []models.Balance{
			balance("USD", "asset-freeze"),
			balance("USD", "default"),
			balance("EUR", "default"),
		}

		got := defaultAssetBalances(items, "USD")
		require.Len(t, got, 1)
		require.Equal(t, "default", got[0].Key)
	})

	t.Run("empty key reads as the default", func(t *testing.T) {
		got := defaultAssetBalances([]models.Balance{balance("USD", "")}, "USD")
		require.Len(t, got, 1)
	})

	t.Run("two keyed balances stay ambiguous", func(t *testing.T) {
		items := []models.Balance{balance("USD", "asset-freeze"), balance("USD", "escrow")}
		require.Len(t, defaultAssetBalances(items, "USD"), 2)
	})

	t.Run("no balance for the asset", func(t *testing.T) {
		require.Empty(t, defaultAssetBalances([]models.Balance{balance("EUR", "default")}, "USD"))
	})
}

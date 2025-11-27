package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFiatCurrencyTemplates(t *testing.T) {
	assets := FiatCurrencyTemplates()

	require.Len(t, assets, 4)

	t.Run("US Dollar", func(t *testing.T) {
		usd := assets[0]
		assert.Equal(t, "US Dollar", usd.Name)
		assert.Equal(t, "currency", usd.Type)
		assert.Equal(t, "USD", usd.Code)
		assert.Equal(t, 2, usd.Scale)
		assert.Equal(t, "$", usd.Metadata["symbol"])
		assert.Equal(t, "USD", usd.Metadata["iso"])
	})

	t.Run("Euro", func(t *testing.T) {
		eur := assets[1]
		assert.Equal(t, "Euro", eur.Name)
		assert.Equal(t, "currency", eur.Type)
		assert.Equal(t, "EUR", eur.Code)
		assert.Equal(t, 2, eur.Scale)
		assert.Equal(t, "\u20ac", eur.Metadata["symbol"])
		assert.Equal(t, "EUR", eur.Metadata["iso"])
	})

	t.Run("Brazilian Real", func(t *testing.T) {
		brl := assets[2]
		assert.Equal(t, "Brazilian Real", brl.Name)
		assert.Equal(t, "currency", brl.Type)
		assert.Equal(t, "BRL", brl.Code)
		assert.Equal(t, 2, brl.Scale)
		assert.Equal(t, "R$", brl.Metadata["symbol"])
		assert.Equal(t, "BRL", brl.Metadata["iso"])
	})

	t.Run("Japanese Yen", func(t *testing.T) {
		jpy := assets[3]
		assert.Equal(t, "Japanese Yen", jpy.Name)
		assert.Equal(t, "currency", jpy.Type)
		assert.Equal(t, "JPY", jpy.Code)
		assert.Equal(t, 0, jpy.Scale) // JPY has no decimals
		assert.Equal(t, "\u00a5", jpy.Metadata["symbol"])
		assert.Equal(t, "JPY", jpy.Metadata["iso"])
	})
}

func TestCryptoAssetTemplates(t *testing.T) {
	assets := CryptoAssetTemplates()

	require.Len(t, assets, 3)

	t.Run("Bitcoin", func(t *testing.T) {
		btc := assets[0]
		assert.Equal(t, "Bitcoin", btc.Name)
		assert.Equal(t, "crypto", btc.Type)
		assert.Equal(t, "BTC", btc.Code)
		assert.Equal(t, 8, btc.Scale) // BTC has 8 decimal places (satoshis)
		assert.Equal(t, "\u20bf", btc.Metadata["symbol"])
	})

	t.Run("Ether", func(t *testing.T) {
		eth := assets[1]
		assert.Equal(t, "Ether", eth.Name)
		assert.Equal(t, "crypto", eth.Type)
		assert.Equal(t, "ETH", eth.Code)
		assert.Equal(t, 18, eth.Scale) // ETH has 18 decimal places (wei)
		assert.Equal(t, "\u039e", eth.Metadata["symbol"])
	})

	t.Run("Tether USD", func(t *testing.T) {
		usdt := assets[2]
		assert.Equal(t, "Tether USD", usdt.Name)
		assert.Equal(t, "crypto", usdt.Type)
		assert.Equal(t, "USDT", usdt.Code)
		assert.Equal(t, 6, usdt.Scale)
		assert.Equal(t, "\u20ae", usdt.Metadata["symbol"])
	})
}

func TestLoyaltyTemplates(t *testing.T) {
	assets := LoyaltyTemplates()

	require.Len(t, assets, 2)

	t.Run("Loyalty Points", func(t *testing.T) {
		points := assets[0]
		assert.Equal(t, "Loyalty Points", points.Name)
		assert.Equal(t, "others", points.Type)
		assert.Equal(t, "POINTS", points.Code)
		assert.Equal(t, 0, points.Scale) // Points are whole numbers
		assert.Equal(t, "loyalty", points.Metadata["category"])
	})

	t.Run("Airline Miles", func(t *testing.T) {
		miles := assets[1]
		assert.Equal(t, "Airline Miles", miles.Name)
		assert.Equal(t, "others", miles.Type)
		assert.Equal(t, "MILES", miles.Code)
		assert.Equal(t, 0, miles.Scale) // Miles are whole numbers
		assert.Equal(t, "loyalty", miles.Metadata["category"])
	})
}

func TestStoreCreditTemplates(t *testing.T) {
	assets := StoreCreditTemplates()

	require.Len(t, assets, 1)

	credit := assets[0]
	assert.Equal(t, "Store Credit", credit.Name)
	assert.Equal(t, "others", credit.Type)
	assert.Equal(t, "CREDIT", credit.Code)
	assert.Equal(t, 2, credit.Scale) // Store credit has 2 decimal places like currency
	assert.Equal(t, "store_credit", credit.Metadata["category"])
}

func TestAllAssetTemplates(t *testing.T) {
	assets := AllAssetTemplates()

	// Count expected assets:
	// FiatCurrencyTemplates: 4
	// CryptoAssetTemplates: 3
	// LoyaltyTemplates: 2
	// StoreCreditTemplates: 1
	// Total: 10
	expectedCount := 10
	require.Len(t, assets, expectedCount)

	t.Run("all assets have required fields", func(t *testing.T) {
		for i, asset := range assets {
			assert.NotEmpty(t, asset.Name, "asset %d should have name", i)
			assert.NotEmpty(t, asset.Type, "asset %d should have type", i)
			assert.NotEmpty(t, asset.Code, "asset %d should have code", i)
			assert.GreaterOrEqual(t, asset.Scale, 0, "asset %d scale should be >= 0", i)
		}
	})

	t.Run("all assets have valid types", func(t *testing.T) {
		validTypes := map[string]bool{
			"currency": true,
			"crypto":   true,
			"others":   true,
		}

		for i, asset := range assets {
			assert.True(t, validTypes[asset.Type], "asset %d has invalid type: %s", i, asset.Type)
		}
	})

	t.Run("all assets have non-nil metadata", func(t *testing.T) {
		for i, asset := range assets {
			assert.NotNil(t, asset.Metadata, "asset %d should have non-nil metadata", i)
		}
	})

	t.Run("unique asset codes", func(t *testing.T) {
		codes := make(map[string]bool)
		for _, asset := range assets {
			assert.False(t, codes[asset.Code], "duplicate asset code found: %s", asset.Code)
			codes[asset.Code] = true
		}
	})

	t.Run("valid scales", func(t *testing.T) {
		for i, asset := range assets {
			assert.GreaterOrEqual(t, asset.Scale, 0, "asset %d scale should be >= 0", i)
			assert.LessOrEqual(t, asset.Scale, 18, "asset %d scale should be <= 18", i)
		}
	})
}

func TestAssetTemplateStruct(t *testing.T) {
	asset := AssetTemplate{
		Name:     "Test Asset",
		Type:     "currency",
		Code:     "TST",
		Scale:    4,
		Metadata: map[string]any{"key": "value"},
	}

	assert.Equal(t, "Test Asset", asset.Name)
	assert.Equal(t, "currency", asset.Type)
	assert.Equal(t, "TST", asset.Code)
	assert.Equal(t, 4, asset.Scale)
	assert.Equal(t, "value", asset.Metadata["key"])
}

func TestAssetTemplateWithNilMetadata(t *testing.T) {
	asset := AssetTemplate{
		Name:     "Test Asset",
		Type:     "currency",
		Code:     "TST",
		Scale:    2,
		Metadata: nil,
	}

	assert.Nil(t, asset.Metadata)
}

func TestAssetTemplateWithEmptyMetadata(t *testing.T) {
	asset := AssetTemplate{
		Name:     "Test Asset",
		Type:     "currency",
		Code:     "TST",
		Scale:    2,
		Metadata: map[string]any{},
	}

	assert.NotNil(t, asset.Metadata)
	assert.Empty(t, asset.Metadata)
}

func TestAssetCategoryCounts(t *testing.T) {
	assets := AllAssetTemplates()

	typeCounts := make(map[string]int)
	for _, asset := range assets {
		typeCounts[asset.Type]++
	}

	assert.Equal(t, 4, typeCounts["currency"], "should have 4 currency assets")
	assert.Equal(t, 3, typeCounts["crypto"], "should have 3 crypto assets")
	assert.Equal(t, 3, typeCounts["others"], "should have 3 others assets")
}

func TestCurrencyScales(t *testing.T) {
	// Test that fiat currencies have appropriate scales
	fiatAssets := FiatCurrencyTemplates()

	for _, asset := range fiatAssets {
		if asset.Code == "JPY" {
			assert.Equal(t, 0, asset.Scale, "JPY should have scale 0")
		} else {
			assert.Equal(t, 2, asset.Scale, "%s should have scale 2", asset.Code)
		}
	}
}

func TestCryptoScales(t *testing.T) {
	// Test that crypto assets have appropriate scales
	cryptoAssets := CryptoAssetTemplates()

	scaleMap := map[string]int{
		"BTC":  8,
		"ETH":  18,
		"USDT": 6,
	}

	for _, asset := range cryptoAssets {
		expected, ok := scaleMap[asset.Code]
		assert.True(t, ok, "unexpected crypto asset: %s", asset.Code)
		assert.Equal(t, expected, asset.Scale, "%s should have scale %d", asset.Code, expected)
	}
}

func TestAssetCodesAreUppercase(t *testing.T) {
	assets := AllAssetTemplates()

	for _, asset := range assets {
		for _, c := range asset.Code {
			if c >= 'a' && c <= 'z' {
				t.Errorf("asset code %s contains lowercase characters", asset.Code)
				break
			}
		}
	}
}

func TestFiatCurrenciesHaveISOMetadata(t *testing.T) {
	fiatAssets := FiatCurrencyTemplates()

	for _, asset := range fiatAssets {
		iso, ok := asset.Metadata["iso"]
		assert.True(t, ok, "%s should have iso metadata", asset.Code)
		assert.Equal(t, asset.Code, iso, "%s iso metadata should match code", asset.Code)
	}
}

func TestAllAssetsHaveSymbolMetadata(t *testing.T) {
	// Check that fiat and crypto assets have symbol metadata
	fiatAssets := FiatCurrencyTemplates()
	cryptoAssets := CryptoAssetTemplates()

	for _, asset := range fiatAssets {
		_, ok := asset.Metadata["symbol"]
		assert.True(t, ok, "%s should have symbol metadata", asset.Code)
	}

	for _, asset := range cryptoAssets {
		_, ok := asset.Metadata["symbol"]
		assert.True(t, ok, "%s should have symbol metadata", asset.Code)
	}
}

func TestLoyaltyAssetsHaveCategoryMetadata(t *testing.T) {
	loyaltyAssets := LoyaltyTemplates()
	storeCreditAssets := StoreCreditTemplates()

	for _, asset := range loyaltyAssets {
		category, ok := asset.Metadata["category"]
		assert.True(t, ok, "%s should have category metadata", asset.Code)
		assert.Equal(t, "loyalty", category, "%s category should be loyalty", asset.Code)
	}

	for _, asset := range storeCreditAssets {
		category, ok := asset.Metadata["category"]
		assert.True(t, ok, "%s should have category metadata", asset.Code)
		assert.Equal(t, "store_credit", category, "%s category should be store_credit", asset.Code)
	}
}

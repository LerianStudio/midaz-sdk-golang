package data

// Asset templates and catalogs used by generators.

// FiatCurrencyTemplates returns common fiat currency templates with proper scales.
func FiatCurrencyTemplates() []AssetTemplate {
    return []AssetTemplate{
        {Name: "US Dollar", Type: "currency", Code: "USD", Scale: 2, Metadata: map[string]any{"symbol": "$", "iso": "USD"}},
        {Name: "Euro", Type: "currency", Code: "EUR", Scale: 2, Metadata: map[string]any{"symbol": "€", "iso": "EUR"}},
        {Name: "Brazilian Real", Type: "currency", Code: "BRL", Scale: 2, Metadata: map[string]any{"symbol": "R$", "iso": "BRL"}},
        {Name: "Japanese Yen", Type: "currency", Code: "JPY", Scale: 0, Metadata: map[string]any{"symbol": "¥", "iso": "JPY"}},
    }
}

// CryptoAssetTemplates returns a minimal set of crypto assets with realistic precision.
func CryptoAssetTemplates() []AssetTemplate {
    return []AssetTemplate{
        {Name: "Bitcoin", Type: "crypto", Code: "BTC", Scale: 8, Metadata: map[string]any{"symbol": "₿"}},
        {Name: "Ether", Type: "crypto", Code: "ETH", Scale: 18, Metadata: map[string]any{"symbol": "Ξ"}},
        {Name: "Tether USD", Type: "crypto", Code: "USDT", Scale: 6, Metadata: map[string]any{"symbol": "₮"}},
    }
}

// LoyaltyTemplates returns point/mile like assets classified as 'others'.
func LoyaltyTemplates() []AssetTemplate {
    return []AssetTemplate{
        {Name: "Loyalty Points", Type: "others", Code: "POINTS", Scale: 0, Metadata: map[string]any{"category": "loyalty"}},
        {Name: "Airline Miles", Type: "others", Code: "MILES", Scale: 0, Metadata: map[string]any{"category": "loyalty"}},
    }
}

// StoreCreditTemplates returns store credit like assets as 'others'.
func StoreCreditTemplates() []AssetTemplate {
    return []AssetTemplate{
        {Name: "Store Credit", Type: "others", Code: "CREDIT", Scale: 2, Metadata: map[string]any{"category": "store_credit"}},
    }
}

// AllAssetTemplates aggregates all predefined assets.
func AllAssetTemplates() []AssetTemplate {
    out := []AssetTemplate{}
    groups := [][]AssetTemplate{FiatCurrencyTemplates(), CryptoAssetTemplates(), LoyaltyTemplates(), StoreCreditTemplates()}
    for _, g := range groups {
        out = append(out, g...)
    }
    return out
}


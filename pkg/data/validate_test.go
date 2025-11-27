package data

import (
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/stretchr/testify/assert"
)

func TestValidateDSLAlias(t *testing.T) {
	t.Run("valid aliases", func(t *testing.T) {
		validAliases := []string{
			"@merchant",
			"@customer",
			"@platform_fee",
			"@merchant-main",
			"@external/bank/123",
			"merchant",
			"customer_account",
			"account-123",
			"a",
			"A",
			"test123",
			"@test/path/to/account",
			"@a-b_c/d-e_f",
		}

		for _, alias := range validAliases {
			t.Run(alias, func(t *testing.T) {
				err := ValidateDSLAlias(alias)
				assert.NoError(t, err)
			})
		}
	})

	t.Run("invalid aliases", func(t *testing.T) {
		invalidAliases := []struct {
			name  string
			alias string
		}{
			{"empty string", ""},
			{"space in alias", "alias with space"},
			{"newline in alias", "alias\nwith\nnewline"},
			{"tab in alias", "alias\twith\ttab"},
			{"special char exclamation", "@alias!"},
			{"special char at", "alias@test"},
			{"special char semicolon", "@alias;"},
			{"special char backtick", "@alias`"},
			{"special char dollar", "@alias$"},
			{"special char braces", "@alias{}"},
			{"special char brackets", "@alias[]"},
			{"special char parens", "@alias()"},
			{"special char less than", "@alias<"},
			{"special char greater than", "@alias>"},
			{"special char ampersand", "@alias&"},
			{"special char pipe", "@alias|"},
			{"special char quote", "@alias\""},
			{"special char single quote", "@alias'"},
			{"special char backslash", "@alias\\"},
			{"special char percent", "@alias%"},
			{"special char hash", "@alias#"},
			{"special char caret", "@alias^"},
			{"special char asterisk", "@alias*"},
			{"special char plus", "@alias+"},
			{"special char equals", "@alias="},
		}

		for _, tt := range invalidAliases {
			t.Run(tt.name, func(t *testing.T) {
				err := ValidateDSLAlias(tt.alias)
				assert.Error(t, err)
			})
		}
	})

	t.Run("empty alias returns error", func(t *testing.T) {
		err := ValidateDSLAlias("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("invalid format returns error with pattern", func(t *testing.T) {
		err := ValidateDSLAlias("invalid alias!")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid alias format")
		assert.Contains(t, err.Error(), "must match pattern")
	})
}

func TestValidateOrgTemplate(t *testing.T) {
	t.Run("valid org template", func(t *testing.T) {
		org := OrgTemplate{
			LegalName: "Test Company Inc.",
			TradeName: "TestCo",
			TaxID:     "12-3456789",
			Address:   models.NewAddress("123 Main St", "12345", "Test City", "TS", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Metadata:  map[string]any{"key": "value"},
		}

		err := ValidateOrgTemplate(org)
		assert.NoError(t, err)
	})

	t.Run("valid org template without metadata", func(t *testing.T) {
		org := OrgTemplate{
			LegalName: "Test Company Inc.",
			TradeName: "TestCo",
			TaxID:     "12-3456789",
			Address:   models.NewAddress("123 Main St", "12345", "Test City", "TS", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Metadata:  nil,
		}

		err := ValidateOrgTemplate(org)
		assert.NoError(t, err)
	})

	t.Run("missing legal name", func(t *testing.T) {
		org := OrgTemplate{
			LegalName: "",
			TradeName: "TestCo",
			Address:   models.NewAddress("123 Main St", "12345", "Test City", "TS", "US"),
		}

		err := ValidateOrgTemplate(org)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "legal name is required")
	})

	t.Run("incomplete address - missing city", func(t *testing.T) {
		org := OrgTemplate{
			LegalName: "Test Company Inc.",
			TradeName: "TestCo",
			Address:   models.NewAddress("123 Main St", "12345", "", "TS", "US"),
		}

		err := ValidateOrgTemplate(org)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "address is incomplete")
	})

	t.Run("incomplete address - missing country", func(t *testing.T) {
		org := OrgTemplate{
			LegalName: "Test Company Inc.",
			TradeName: "TestCo",
			Address:   models.NewAddress("123 Main St", "12345", "Test City", "TS", ""),
		}

		err := ValidateOrgTemplate(org)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "address is incomplete")
	})

	t.Run("incomplete address - missing line1", func(t *testing.T) {
		org := OrgTemplate{
			LegalName: "Test Company Inc.",
			TradeName: "TestCo",
			Address:   models.NewAddress("", "12345", "Test City", "TS", "US"),
		}

		err := ValidateOrgTemplate(org)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "address is incomplete")
	})

	t.Run("invalid metadata - key too long", func(t *testing.T) {
		longKey := strings.Repeat("a", 101) // Key > 100 chars
		org := OrgTemplate{
			LegalName: "Test Company Inc.",
			Address:   models.NewAddress("123 Main St", "12345", "Test City", "TS", "US"),
			Metadata:  map[string]any{longKey: "value"},
		}

		err := ValidateOrgTemplate(org)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid metadata")
	})
}

func TestValidateAssetTemplate(t *testing.T) {
	t.Run("valid asset template - currency", func(t *testing.T) {
		asset := AssetTemplate{
			Name:     "US Dollar",
			Type:     "currency",
			Code:     "USD",
			Scale:    2,
			Metadata: map[string]any{"symbol": "$"},
		}

		err := ValidateAssetTemplate(asset)
		assert.NoError(t, err)
	})

	t.Run("valid asset template - crypto", func(t *testing.T) {
		asset := AssetTemplate{
			Name:     "Bitcoin",
			Type:     "crypto",
			Code:     "BTC",
			Scale:    8,
			Metadata: map[string]any{"symbol": "\u20bf"},
		}

		err := ValidateAssetTemplate(asset)
		assert.NoError(t, err)
	})

	t.Run("valid asset template - others", func(t *testing.T) {
		asset := AssetTemplate{
			Name:     "Loyalty Points",
			Type:     "others",
			Code:     "POINTS",
			Scale:    0,
			Metadata: nil,
		}

		err := ValidateAssetTemplate(asset)
		assert.NoError(t, err)
	})

	t.Run("missing name", func(t *testing.T) {
		asset := AssetTemplate{
			Name:  "",
			Type:  "currency",
			Code:  "USD",
			Scale: 2,
		}

		err := ValidateAssetTemplate(asset)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "asset name is required")
	})

	t.Run("invalid type", func(t *testing.T) {
		asset := AssetTemplate{
			Name:  "Test Asset",
			Type:  "invalid_type",
			Code:  "TST",
			Scale: 2,
		}

		err := ValidateAssetTemplate(asset)
		assert.Error(t, err)
	})

	t.Run("scale too low", func(t *testing.T) {
		asset := AssetTemplate{
			Name:  "Test Asset",
			Type:  "currency",
			Code:  "TST",
			Scale: -1,
		}

		err := ValidateAssetTemplate(asset)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid scale")
	})

	t.Run("scale too high", func(t *testing.T) {
		asset := AssetTemplate{
			Name:  "Test Asset",
			Type:  "currency",
			Code:  "TST",
			Scale: 19,
		}

		err := ValidateAssetTemplate(asset)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid scale")
	})

	t.Run("scale at boundaries", func(t *testing.T) {
		// Scale 0 should be valid
		asset0 := AssetTemplate{
			Name:  "Test Asset",
			Type:  "currency",
			Code:  "JPY",
			Scale: 0,
		}
		err := ValidateAssetTemplate(asset0)
		assert.NoError(t, err)

		// Scale 18 should be valid
		asset18 := AssetTemplate{
			Name:  "Test Asset",
			Type:  "crypto",
			Code:  "ETH",
			Scale: 18,
		}
		err = ValidateAssetTemplate(asset18)
		assert.NoError(t, err)
	})

	t.Run("invalid metadata - key too long", func(t *testing.T) {
		longKey := strings.Repeat("a", 101)
		asset := AssetTemplate{
			Name:     "Test Asset",
			Type:     "currency",
			Code:     "USD",
			Scale:    2,
			Metadata: map[string]any{longKey: "value"},
		}

		err := ValidateAssetTemplate(asset)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid metadata")
	})
}

func TestValidateAccountTemplate(t *testing.T) {
	t.Run("valid account template", func(t *testing.T) {
		acc := AccountTemplate{
			Name:     "Test Account",
			Type:     "deposit",
			Status:   models.NewStatus(models.StatusActive),
			Alias:    StrPtr("test_alias"),
			Metadata: map[string]any{"key": "value"},
		}

		err := ValidateAccountTemplate(acc)
		assert.NoError(t, err)
	})

	t.Run("valid account template without optional fields", func(t *testing.T) {
		acc := AccountTemplate{
			Name:   "Test Account",
			Type:   "deposit",
			Status: models.NewStatus(models.StatusActive),
		}

		err := ValidateAccountTemplate(acc)
		assert.NoError(t, err)
	})

	t.Run("missing name", func(t *testing.T) {
		acc := AccountTemplate{
			Name: "",
			Type: "deposit",
		}

		err := ValidateAccountTemplate(acc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "account name is required")
	})

	t.Run("alias starting with @external/", func(t *testing.T) {
		acc := AccountTemplate{
			Name:  "Test Account",
			Type:  "deposit",
			Alias: StrPtr("@external/bank/123"),
		}

		err := ValidateAccountTemplate(acc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must not start with '@external/'")
	})

	t.Run("nil alias is valid", func(t *testing.T) {
		acc := AccountTemplate{
			Name:  "Test Account",
			Type:  "deposit",
			Alias: nil,
		}

		err := ValidateAccountTemplate(acc)
		assert.NoError(t, err)
	})

	t.Run("empty alias is valid", func(t *testing.T) {
		acc := AccountTemplate{
			Name:  "Test Account",
			Type:  "deposit",
			Alias: StrPtr(""),
		}

		err := ValidateAccountTemplate(acc)
		assert.NoError(t, err)
	})

	t.Run("invalid metadata - key too long", func(t *testing.T) {
		longKey := strings.Repeat("a", 101)
		acc := AccountTemplate{
			Name:     "Test Account",
			Type:     "deposit",
			Metadata: map[string]any{longKey: "value"},
		}

		err := ValidateAccountTemplate(acc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid metadata")
	})
}

func TestValidateTransactionPattern(t *testing.T) {
	t.Run("valid transaction pattern", func(t *testing.T) {
		pattern := TransactionPattern{
			ChartOfAccountsGroupName: "payment",
			Description:              "Test payment",
			DSLTemplate:              "send [USD 100] (source = @a)",
			RequiresCommit:           false,
			IdempotencyKey:           "idem-123",
			ExternalID:               "ext-456",
			Metadata:                 map[string]any{"pattern": "test"},
		}

		err := ValidateTransactionPattern(pattern)
		assert.NoError(t, err)
	})

	t.Run("valid transaction pattern without metadata", func(t *testing.T) {
		pattern := TransactionPattern{
			ChartOfAccountsGroupName: "payment",
			DSLTemplate:              "send [USD 100] (source = @a)",
			IdempotencyKey:           "idem-123",
		}

		err := ValidateTransactionPattern(pattern)
		assert.NoError(t, err)
	})

	t.Run("missing DSL template", func(t *testing.T) {
		pattern := TransactionPattern{
			ChartOfAccountsGroupName: "payment",
			DSLTemplate:              "",
			IdempotencyKey:           "idem-123",
		}

		err := ValidateTransactionPattern(pattern)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dsl template is required")
	})

	t.Run("missing chart of accounts group name", func(t *testing.T) {
		pattern := TransactionPattern{
			ChartOfAccountsGroupName: "",
			DSLTemplate:              "send [USD 100] (source = @a)",
			IdempotencyKey:           "idem-123",
		}

		err := ValidateTransactionPattern(pattern)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "chart of accounts group name is required")
	})

	t.Run("missing idempotency key", func(t *testing.T) {
		pattern := TransactionPattern{
			ChartOfAccountsGroupName: "payment",
			DSLTemplate:              "send [USD 100] (source = @a)",
			IdempotencyKey:           "",
		}

		err := ValidateTransactionPattern(pattern)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "idempotency key is required")
	})

	t.Run("invalid metadata - key too long", func(t *testing.T) {
		longKey := strings.Repeat("a", 101)
		pattern := TransactionPattern{
			ChartOfAccountsGroupName: "payment",
			DSLTemplate:              "send [USD 100] (source = @a)",
			IdempotencyKey:           "idem-123",
			Metadata:                 map[string]any{longKey: "value"},
		}

		err := ValidateTransactionPattern(pattern)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid metadata")
	})
}

func TestValidateDSLAliasPattern(t *testing.T) {
	// Test the regex pattern directly
	tests := []struct {
		alias   string
		matches bool
	}{
		{"@merchant", true},
		{"@merchant_main", true},
		{"@merchant-main", true},
		{"@path/to/account", true},
		{"merchant", true},
		{"MERCHANT", true},
		{"merchant123", true},
		{"123merchant", true},
		{"@123", true},
		{"", false},
		{"@", false}, // @ alone is invalid - regex requires at least one char after optional @
		{"@merchant account", false},
		{"@merchant\n", false},
		{"@merchant!", false},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			if tt.alias == "" {
				err := ValidateDSLAlias(tt.alias)
				assert.Error(t, err)
				return
			}

			err := ValidateDSLAlias(tt.alias)
			if tt.matches {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestValidateOrgTemplateEdgeCases(t *testing.T) {
	t.Run("whitespace only legal name", func(t *testing.T) {
		org := OrgTemplate{
			LegalName: "   ",
			Address:   models.NewAddress("123 Main St", "12345", "City", "ST", "US"),
		}

		err := ValidateOrgTemplate(org)
		// Whitespace-only should pass since we only check for empty
		assert.NoError(t, err)
	})

	t.Run("very long legal name", func(t *testing.T) {
		org := OrgTemplate{
			LegalName: strings.Repeat("a", 1000),
			Address:   models.NewAddress("123 Main St", "12345", "City", "ST", "US"),
		}

		err := ValidateOrgTemplate(org)
		// Long names should pass - no length limit in validation
		assert.NoError(t, err)
	})
}

func TestValidateAssetTemplateEdgeCases(t *testing.T) {
	t.Run("whitespace only name", func(t *testing.T) {
		asset := AssetTemplate{
			Name:  "   ",
			Type:  "currency",
			Code:  "USD",
			Scale: 2,
		}

		err := ValidateAssetTemplate(asset)
		// Whitespace-only should pass since we only check for empty
		assert.NoError(t, err)
	})

	t.Run("valid asset types", func(t *testing.T) {
		validTypes := []string{"currency", "crypto", "commodity", "others"}

		for _, assetType := range validTypes {
			asset := AssetTemplate{
				Name:  "Test",
				Type:  assetType,
				Code:  "TST",
				Scale: 2,
			}

			err := ValidateAssetTemplate(asset)
			assert.NoError(t, err, "type %s should be valid", assetType)
		}
	})
}

func TestValidateAccountTemplateEdgeCases(t *testing.T) {
	t.Run("alias with @external case sensitivity", func(t *testing.T) {
		// Test that @external/ check is case sensitive
		acc := AccountTemplate{
			Name:  "Test Account",
			Type:  "deposit",
			Alias: StrPtr("@External/bank/123"),
		}

		// The validation uses IsValidExternalAccountID which checks prefix
		// IsValidExternalAccountID returns true only for lowercase @external/
		// so @External/ should NOT be flagged as external
		err := ValidateAccountTemplate(acc)
		// @External (uppercase) is NOT treated as external, so it should pass
		assert.NoError(t, err)
	})

	t.Run("alias with lowercase @external fails", func(t *testing.T) {
		acc := AccountTemplate{
			Name:  "Test Account",
			Type:  "deposit",
			Alias: StrPtr("@external/bank/123"),
		}

		err := ValidateAccountTemplate(acc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must not start with '@external/'")
	})
}

func TestValidateTransactionPatternEdgeCases(t *testing.T) {
	t.Run("whitespace only DSL template", func(t *testing.T) {
		pattern := TransactionPattern{
			ChartOfAccountsGroupName: "payment",
			DSLTemplate:              "   ",
			IdempotencyKey:           "idem-123",
		}

		err := ValidateTransactionPattern(pattern)
		// Whitespace-only should pass since we only check for empty
		assert.NoError(t, err)
	})

	t.Run("very long DSL template", func(t *testing.T) {
		pattern := TransactionPattern{
			ChartOfAccountsGroupName: "payment",
			DSLTemplate:              strings.Repeat("send [USD 100]", 1000),
			IdempotencyKey:           "idem-123",
		}

		err := ValidateTransactionPattern(pattern)
		// Long DSL should pass - no length limit in validation
		assert.NoError(t, err)
	})
}

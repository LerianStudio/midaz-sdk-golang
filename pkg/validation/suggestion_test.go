package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCommonSuggestions(t *testing.T) {
	tests := []struct {
		name         string
		field        string
		value        any
		suggType     SuggestionType
		wantContains []string
		wantMinLen   int
	}{
		// Format suggestions
		{
			name:         "Format - asset code",
			field:        "asset_code",
			value:        "us",
			suggType:     Format,
			wantContains: []string{"3-4 uppercase letters", "USD", "EUR"},
			wantMinLen:   1,
		},
		{
			name:         "Format - currency field",
			field:        "currency",
			value:        "invalid",
			suggType:     Format,
			wantContains: []string{"3-4 uppercase letters"},
			wantMinLen:   1,
		},
		{
			name:         "Format - account alias",
			field:        "account_alias",
			value:        "invalid@alias",
			suggType:     Format,
			wantContains: []string{"alphanumeric", "underscore", "hyphen"},
			wantMinLen:   2,
		},
		{
			name:         "Format - code field (matches asset code)",
			field:        "code",
			value:        "bad",
			suggType:     Format,
			wantContains: []string{"uppercase", "letters"},
			wantMinLen:   1,
		},
		{
			name:         "Format - date field",
			field:        "start_date",
			value:        "invalid",
			suggType:     Format,
			wantContains: []string{"ISO 8601", "YYYY-MM-DD"},
			wantMinLen:   1,
		},
		{
			name:         "Format - email field",
			field:        "email",
			value:        "notanemail",
			suggType:     Format,
			wantContains: []string{"user@example.com", "@"},
			wantMinLen:   1,
		},
		{
			name:         "Format - UUID field",
			field:        "transaction_id",
			value:        "not-a-uuid",
			suggType:     Format,
			wantContains: []string{"UUID", "123e4567"},
			wantMinLen:   1,
		},
		{
			name:         "Format - metadata field",
			field:        "metadata",
			value:        nil,
			suggType:     Format,
			wantContains: []string{"keys", "values", "4KB"},
			wantMinLen:   1,
		},
		{
			name:         "Format - chart of accounts",
			field:        "chart_of_accounts",
			value:        "bad@chart",
			suggType:     Format,
			wantContains: []string{"alphanumeric", "1-100"},
			wantMinLen:   1,
		},
		{
			name:         "Format - external account",
			field:        "external_account",
			value:        "badformat",
			suggType:     Format,
			wantContains: []string{"@external/", "XXX"},
			wantMinLen:   1,
		},
		{
			name:         "Format - country field",
			field:        "country",
			value:        "USA",
			suggType:     Format,
			wantContains: []string{"ISO 3166", "2 letters", "US"},
			wantMinLen:   1,
		},
		{
			name:         "Format - generic field",
			field:        "some_random_field",
			value:        "value",
			suggType:     Format,
			wantContains: []string{"documentation", "format"},
			wantMinLen:   1,
		},

		// Required suggestions
		{
			name:         "Required - any field",
			field:        "name",
			value:        nil,
			suggType:     Required,
			wantContains: []string{"required", "cannot be empty"},
			wantMinLen:   2,
		},

		// Range suggestions
		{
			name:         "Range - amount field",
			field:        "amount",
			value:        -100,
			suggType:     Range,
			wantContains: []string{"greater than zero", "scale"},
			wantMinLen:   1,
		},
		{
			name:         "Range - scale field",
			field:        "scale",
			value:        20,
			suggType:     Range,
			wantContains: []string{"0 and 18", "scale 2"},
			wantMinLen:   2,
		},
		{
			name:         "Range - name field",
			field:        "name",
			value:        "too long...",
			suggType:     Range,
			wantContains: []string{"length", "maximum"},
			wantMinLen:   1,
		},
		{
			name:         "Range - generic field",
			field:        "custom_field",
			value:        999,
			suggType:     Range,
			wantContains: []string{"outside", "range"},
			wantMinLen:   1,
		},

		// Enumeration suggestions
		{
			name:         "Enumeration - asset type",
			field:        "asset_type",
			value:        "invalid",
			suggType:     Enumeration,
			wantContains: []string{"crypto", "currency", "commodity", "others"},
			wantMinLen:   2,
		},
		{
			name:         "Enumeration - account type",
			field:        "account_type",
			value:        "invalid",
			suggType:     Enumeration,
			wantContains: []string{"deposit", "savings", "loans"},
			wantMinLen:   2,
		},
		{
			name:         "Enumeration - operation type",
			field:        "operation_type",
			value:        "invalid",
			suggType:     Enumeration,
			wantContains: []string{"DEBIT", "CREDIT"},
			wantMinLen:   2,
		},
		{
			name:         "Enumeration - transaction status",
			field:        "transaction_status",
			value:        "invalid",
			suggType:     Enumeration,
			wantContains: []string{"PENDING", "COMPLETED", "FAILED"},
			wantMinLen:   2,
		},
		{
			name:         "Enumeration - generic field",
			field:        "custom_enum",
			value:        "invalid",
			suggType:     Enumeration,
			wantContains: []string{"allowed values", "case-sensitive"},
			wantMinLen:   1,
		},

		// Consistency suggestions
		{
			name:         "Consistency - asset code",
			field:        "asset_code",
			value:        "USD",
			suggType:     Consistency,
			wantContains: []string{"consistent", "external accounts"},
			wantMinLen:   2,
		},
		{
			name:         "Consistency - balance field",
			field:        "balance",
			value:        1000,
			suggType:     Consistency,
			wantContains: []string{"balance", "debit", "credit"},
			wantMinLen:   2,
		},
		{
			name:         "Consistency - date field",
			field:        "start_date",
			value:        "2023-01-01",
			suggType:     Consistency,
			wantContains: []string{"before", "ISO 8601"},
			wantMinLen:   2,
		},
		{
			name:         "Consistency - generic field",
			field:        "custom_field",
			value:        "value",
			suggType:     Consistency,
			wantContains: []string{"consistency", "consistent"},
			wantMinLen:   1,
		},

		// Structure suggestions
		{
			name:         "Structure - operations field",
			field:        "operations",
			value:        nil,
			suggType:     Structure,
			wantContains: []string{"source", "destination", "account"},
			wantMinLen:   2,
		},
		{
			name:         "Structure - send field",
			field:        "send",
			value:        nil,
			suggType:     Structure,
			wantContains: []string{"source", "destination"},
			wantMinLen:   2,
		},
		{
			name:         "Structure - metadata field",
			field:        "metadata",
			value:        nil,
			suggType:     Structure,
			wantContains: []string{"JSON", "keys", "values"},
			wantMinLen:   2,
		},
		{
			name:         "Structure - address field",
			field:        "address",
			value:        nil,
			suggType:     Structure,
			wantContains: []string{"line1", "zipCode", "city", "country"},
			wantMinLen:   2,
		},
		{
			name:         "Structure - generic field",
			field:        "custom_field",
			value:        nil,
			suggType:     Structure,
			wantContains: []string{"structure", "subfields"},
			wantMinLen:   1,
		},

		// Default/unknown type
		{
			name:         "Unknown suggestion type",
			field:        "field",
			value:        "value",
			suggType:     SuggestionType("unknown"),
			wantContains: []string{"documentation"},
			wantMinLen:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := GetCommonSuggestions(tt.field, tt.value, tt.suggType)

			assert.GreaterOrEqual(t, len(suggestions), tt.wantMinLen,
				"Expected at least %d suggestions, got %d", tt.wantMinLen, len(suggestions))

			for _, wantPart := range tt.wantContains {
				found := false

				for _, sugg := range suggestions {
					if containsIgnoreCase(sugg, wantPart) {
						found = true
						break
					}
				}

				assert.True(t, found,
					"Expected suggestions to contain '%s', got: %v", wantPart, suggestions)
			}
		})
	}
}

func TestSuggestionTypeConstants(t *testing.T) {
	t.Run("Format constant is defined", func(t *testing.T) {
		assert.Equal(t, Format, SuggestionType("format"))
	})

	t.Run("Required constant is defined", func(t *testing.T) {
		assert.Equal(t, Required, SuggestionType("required"))
	})

	t.Run("Range constant is defined", func(t *testing.T) {
		assert.Equal(t, Range, SuggestionType("range"))
	})

	t.Run("Enumeration constant is defined", func(t *testing.T) {
		assert.Equal(t, Enumeration, SuggestionType("enumeration"))
	})

	t.Run("Consistency constant is defined", func(t *testing.T) {
		assert.Equal(t, Consistency, SuggestionType("consistency"))
	})

	t.Run("Structure constant is defined", func(t *testing.T) {
		assert.Equal(t, Structure, SuggestionType("structure"))
	})
}

func TestGetExampleValue(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		expected string
	}{
		{
			name:     "Asset code field",
			field:    "asset_code",
			expected: "USD",
		},
		{
			name:     "Currency field",
			field:    "currency",
			expected: "USD",
		},
		{
			name:     "Account alias field",
			field:    "account_alias",
			expected: "customer_savings",
		},
		{
			name:     "Alias field",
			field:    "alias",
			expected: "customer_savings",
		},
		{
			name:     "Code field matches asset code pattern",
			field:    "code",
			expected: "USD",
		},
		{
			name:     "Amount field",
			field:    "amount",
			expected: "10000 (for $100.00 with scale 2)",
		},
		{
			name:     "Value field",
			field:    "value",
			expected: "10000 (for $100.00 with scale 2)",
		},
		{
			name:     "Asset type field",
			field:    "asset_type",
			expected: "currency",
		},
		{
			name:     "Account type field",
			field:    "account_type",
			expected: "savings",
		},
		{
			name:     "External account field",
			field:    "external_account",
			expected: "@external/USD",
		},
		{
			name:     "Country field with underscore",
			field:    "billing_country",
			expected: "US",
		},
		{
			name:     "Just country field",
			field:    "country",
			expected: "US",
		},
		{
			name:     "Unknown field",
			field:    "unknown_field",
			expected: "See documentation for examples",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetExampleValue(tt.field)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		pattern string
		want    bool
	}{
		{
			name:    "Exact match",
			field:   "email",
			pattern: "email",
			want:    true,
		},
		{
			name:    "Pattern with OR",
			field:   "amount",
			pattern: "amount|value",
			want:    true,
		},
		{
			name:    "Pattern with wildcard",
			field:   "asset_code",
			pattern: "asset.*code",
			want:    true,
		},
		{
			name:    "No match",
			field:   "name",
			pattern: "email|phone",
			want:    false,
		},
		{
			name:    "Case sensitive pattern",
			field:   "EMAIL",
			pattern: "email",
			want:    false,
		},
		{
			name:    "Partial match",
			field:   "transaction_code",
			pattern: "code",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := match(tt.field, tt.pattern)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestGetFormatSuggestions(t *testing.T) {
	tests := []struct {
		name       string
		field      string
		value      any
		wantMinLen int
	}{
		{
			name:       "Asset code suggestions",
			field:      "asset_code",
			value:      "bad",
			wantMinLen: 2,
		},
		{
			name:       "Email suggestions",
			field:      "email",
			value:      "notanemail",
			wantMinLen: 2,
		},
		{
			name:       "UUID suggestions",
			field:      "id",
			value:      "not-uuid",
			wantMinLen: 2,
		},
		{
			name:       "Generic suggestions",
			field:      "unknown",
			value:      "value",
			wantMinLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := getFormatSuggestions(tt.field, tt.value)
			assert.GreaterOrEqual(t, len(suggestions), tt.wantMinLen)
		})
	}
}

func TestGetRequiredSuggestions(t *testing.T) {
	t.Run("Returns required field suggestions", func(t *testing.T) {
		suggestions := getRequiredSuggestions("field_name")

		assert.GreaterOrEqual(t, len(suggestions), 2)

		hasRequired := false
		hasFieldName := false

		for _, s := range suggestions {
			if containsIgnoreCase(s, "required") {
				hasRequired = true
			}

			if containsIgnoreCase(s, "field_name") {
				hasFieldName = true
			}
		}

		assert.True(t, hasRequired, "Should mention 'required'")
		assert.True(t, hasFieldName, "Should mention field name")
	})
}

func TestGetRangeSuggestions(t *testing.T) {
	t.Run("Amount field returns amount suggestions", func(t *testing.T) {
		suggestions := getRangeSuggestions("amount", -100)
		assert.GreaterOrEqual(t, len(suggestions), 2)
	})

	t.Run("Scale field returns scale suggestions", func(t *testing.T) {
		suggestions := getRangeSuggestions("scale", 20)
		hasZeroTo18 := false

		for _, s := range suggestions {
			if containsIgnoreCase(s, "0 and 18") || containsIgnoreCase(s, "0-18") {
				hasZeroTo18 = true
				break
			}
		}

		assert.True(t, hasZeroTo18, "Should mention scale range")
	})
}

func TestGetEnumerationSuggestions(t *testing.T) {
	t.Run("Asset type returns valid options", func(t *testing.T) {
		suggestions := getEnumerationSuggestions("asset_type", "invalid")

		found := false

		for _, s := range suggestions {
			if containsIgnoreCase(s, "crypto") && containsIgnoreCase(s, "currency") {
				found = true
				break
			}
		}

		assert.True(t, found, "Should list valid asset types")
	})

	t.Run("Account type returns valid options", func(t *testing.T) {
		suggestions := getEnumerationSuggestions("account_type", "invalid")

		found := false

		for _, s := range suggestions {
			if containsIgnoreCase(s, "deposit") && containsIgnoreCase(s, "savings") {
				found = true
				break
			}
		}

		assert.True(t, found, "Should list valid account types")
	})
}

func TestGetConsistencySuggestions(t *testing.T) {
	t.Run("Asset code returns consistency suggestions", func(t *testing.T) {
		suggestions := getConsistencySuggestions("asset_code")
		assert.GreaterOrEqual(t, len(suggestions), 2)
	})

	t.Run("Balance returns balance suggestions", func(t *testing.T) {
		suggestions := getConsistencySuggestions("balance")

		found := false

		for _, s := range suggestions {
			if containsIgnoreCase(s, "debit") || containsIgnoreCase(s, "credit") {
				found = true
				break
			}
		}

		assert.True(t, found, "Should mention debit/credit")
	})
}

func TestGetStructureSuggestions(t *testing.T) {
	t.Run("Operations returns structure suggestions", func(t *testing.T) {
		suggestions := getStructureSuggestions("operations")
		assert.GreaterOrEqual(t, len(suggestions), 2)
	})

	t.Run("Address returns address suggestions", func(t *testing.T) {
		suggestions := getStructureSuggestions("address")

		hasLine1 := false
		hasCountry := false

		for _, s := range suggestions {
			if containsIgnoreCase(s, "line1") {
				hasLine1 = true
			}

			if containsIgnoreCase(s, "country") {
				hasCountry = true
			}
		}

		assert.True(t, hasLine1, "Should mention line1")
		assert.True(t, hasCountry, "Should mention country")
	})

	t.Run("Metadata returns metadata structure suggestions", func(t *testing.T) {
		suggestions := getStructureSuggestions("metadata")
		assert.GreaterOrEqual(t, len(suggestions), 2)
	})
}

// Helper function to check if a string contains a substring (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > 0 && len(substr) > 0 &&
				(s[0] == substr[0] || s[0]+32 == substr[0] || s[0] == substr[0]+32) &&
				containsIgnoreCase(s[1:], substr[1:]) ||
			containsIgnoreCase(s[1:], substr))
}

package validation

import (
	"testing"
	"time"
)

func TestEnhancedValidateAssetCode(t *testing.T) {
	tests := []struct {
		name      string
		assetCode string
		wantErr   bool
	}{
		{"Valid asset code", "USD", false},
		{"Valid 4-letter asset code", "USDT", false},
		{"Empty asset code", "", true},
		{"Invalid asset code - lowercase", "usd", true},
		{"Invalid asset code - too short", "US", true},
		{"Invalid asset code - too long", "USDOL", true},
		{"Invalid asset code - with number", "USD1", true},
		{"Invalid asset code - with symbol", "US$", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnhancedValidateAssetCode(tt.assetCode)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnhancedValidateAssetCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				// Check that error has suggestions
				if len(err.Suggestions) == 0 {
					t.Errorf("EnhancedValidateAssetCode() error should include suggestions")
				}

				// Check that constraint is set
				if err.Constraint == "" {
					t.Errorf("EnhancedValidateAssetCode() error should include constraint")
				}
			}
		})
	}
}

func TestEnhancedValidateAccountAlias(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		wantErr bool
	}{
		{"Valid alias", "savings_account", false},
		{"Valid alias with hyphen", "savings-account", false},
		{"Valid alias with numbers", "account123", false},
		{"Empty alias", "", true},
		{"Too long alias", "this_is_a_very_long_alias_that_exceeds_the_maximum_allowed_length_for_an_account_alias_in_the_system", true},
		{"Invalid characters", "savings@account", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnhancedValidateAccountAlias(tt.alias)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnhancedValidateAccountAlias() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && len(err.Suggestions) == 0 {
				t.Errorf("EnhancedValidateAccountAlias() error should include suggestions")
			}
		})
	}
}

//nolint:revive // cognitive-complexity: table-driven test pattern
func TestEnhancedValidateDateRange(t *testing.T) {
	now := time.Now()
	past := now.AddDate(0, -1, 0)  // One month ago
	future := now.AddDate(0, 1, 0) // One month in the future

	tests := []struct {
		name       string
		start      time.Time
		end        time.Time
		wantErr    bool
		errorCount int
	}{
		{"Valid date range", past, now, false, 0},
		{"Valid date range - same day", now, now, false, 0},
		{"Invalid - start after end", future, now, true, 1},
		{"Invalid - empty start", time.Time{}, now, true, 1},
		{"Invalid - empty end", now, time.Time{}, true, 1},
		{"Invalid - both empty", time.Time{}, time.Time{}, true, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := EnhancedValidateDateRange(tt.start, tt.end, "startDate", "endDate")
			if (errors.HasErrors()) != tt.wantErr {
				t.Errorf("EnhancedValidateDateRange() errors = %v, wantErr %v", errors, tt.wantErr)
				return
			}

			if tt.wantErr && len(errors.Errors) != tt.errorCount {
				t.Errorf("EnhancedValidateDateRange() error count = %d, want %d", len(errors.Errors), tt.errorCount)
			}

			if errors.HasErrors() {
				for _, err := range errors.Errors {
					if len(err.Suggestions) == 0 {
						t.Errorf("EnhancedValidateDateRange() error should include suggestions: %v", err)
					}
				}
			}
		})
	}
}

func TestEnhancedValidateExternalAccount(t *testing.T) {
	tests := []struct {
		name    string
		account string
		wantErr bool
	}{
		{"Valid external account", "@external/USD", false},
		{"Valid external account - 4 letters", "@external/USDT", false},
		{"Empty account", "", true},
		{"Missing @ prefix", "external/USD", true},
		{"Invalid format", "@externalUSD", true},
		{"Invalid asset code", "@external/US", true},
		{"Invalid asset code with number", "@external/USD1", true},
		{"Invalid asset code with lowercase", "@external/usd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnhancedValidateExternalAccount(tt.account)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnhancedValidateExternalAccount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && len(err.Suggestions) == 0 {
				t.Errorf("EnhancedValidateExternalAccount() error should include suggestions")
			}
		})
	}
}

func TestEnhancedValidateExternalAccountWithTransactionAsset(t *testing.T) {
	tests := []struct {
		name             string
		account          string
		transactionAsset string
		wantErr          bool
	}{
		{"Valid with matching asset", "@external/USD", "USD", false},
		{"Invalid with mismatched asset", "@external/USD", "EUR", true},
		{"Invalid external account format", "@externalUSD", "USD", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnhancedValidateExternalAccountWithTransactionAsset(tt.account, tt.transactionAsset)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnhancedValidateExternalAccountWithTransactionAsset() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && len(err.Suggestions) == 0 {
				t.Errorf("EnhancedValidateExternalAccountWithTransactionAsset() error should include suggestions")
			}
		})
	}
}

func TestEnhancedValidateMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		wantErr  bool
	}{
		{
			"Valid metadata",
			map[string]any{
				"reference":   "INV-123",
				"customer_id": 12345,
				"amount":      100.50,
				"approved":    true,
			},
			false,
		},
		{
			"Empty metadata",
			map[string]any{},
			false,
		},
		{
			"Nil metadata",
			nil,
			false,
		},
		{
			"Invalid - key too long",
			map[string]any{
				"this_is_a_very_long_key_that_exceeds_the_maximum_allowed_length_for_metadata_keys_in_the_system_and_should_cause_a_validation_error": "value",
			},
			true,
		},
		{
			"Invalid - string value too long",
			map[string]any{
				"key": string(make([]byte, 300)),
			},
			true,
		},
		{
			"Invalid - unsupported value type",
			map[string]any{
				"key": []string{"not", "supported"},
			},
			true,
		},
		{
			"Invalid - number out of range",
			map[string]any{
				"key": 10000000000, // 10 billion
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := EnhancedValidateMetadata(tt.metadata)
			if (errors.HasErrors()) != tt.wantErr {
				t.Errorf("EnhancedValidateMetadata() errors = %v, wantErr %v", errors, tt.wantErr)
				return
			}

			if errors.HasErrors() {
				for _, err := range errors.Errors {
					if len(err.Suggestions) == 0 {
						t.Errorf("EnhancedValidateMetadata() error should include suggestions: %v", err)
					}
				}
			}
		})
	}
}

func TestEnhancedValidateAddress(t *testing.T) {
	validAddress := &Address{
		Line1:   "123 Main St",
		ZipCode: "12345",
		City:    "New York",
		State:   "NY",
		Country: "US",
	}

	line2 := "Apt 4B"
	addressWithLine2 := &Address{
		Line1:   "123 Main St",
		Line2:   &line2,
		ZipCode: "12345",
		City:    "New York",
		State:   "NY",
		Country: "US",
	}

	tests := []struct {
		name    string
		address *Address
		wantErr bool
	}{
		{"Valid address", validAddress, false},
		{"Valid address with Line2", addressWithLine2, false},
		{"Nil address", nil, true},
		{"Missing Line1", &Address{ZipCode: "12345", City: "New York", State: "NY", Country: "US"}, true},
		{"Missing ZipCode", &Address{Line1: "123 Main St", City: "New York", State: "NY", Country: "US"}, true},
		{"Missing City", &Address{Line1: "123 Main St", ZipCode: "12345", State: "NY", Country: "US"}, true},
		{"Missing State", &Address{Line1: "123 Main St", ZipCode: "12345", City: "New York", Country: "US"}, true},
		{"Missing Country", &Address{Line1: "123 Main St", ZipCode: "12345", City: "New York", State: "NY"}, true},
		{"Invalid Country Code", &Address{Line1: "123 Main St", ZipCode: "12345", City: "New York", State: "NY", Country: "USA"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := EnhancedValidateAddress(tt.address, "address")
			if (errors.HasErrors()) != tt.wantErr {
				t.Errorf("EnhancedValidateAddress() errors = %v, wantErr %v", errors, tt.wantErr)
				return
			}

			if errors.HasErrors() {
				for _, err := range errors.Errors {
					if len(err.Suggestions) == 0 {
						t.Errorf("EnhancedValidateAddress() error should include suggestions: %v", err)
					}
				}
			}
		})
	}
}

// Mock implementation of TransactionDSLValidator for testing
type mockTransactionDSLValidator struct {
	asset               string
	value               float64
	sourceAccounts      []AccountReference
	destinationAccounts []AccountReference
	metadata            map[string]any
}

func (m *mockTransactionDSLValidator) GetAsset() string {
	return m.asset
}

func (m *mockTransactionDSLValidator) GetValue() float64 {
	return m.value
}

func (m *mockTransactionDSLValidator) GetSourceAccounts() []AccountReference {
	return m.sourceAccounts
}

func (m *mockTransactionDSLValidator) GetDestinationAccounts() []AccountReference {
	return m.destinationAccounts
}

func (m *mockTransactionDSLValidator) GetMetadata() map[string]any {
	return m.metadata
}

// Mock implementation of AccountReference for testing
type mockAccountReference struct {
	account string
}

func (m mockAccountReference) GetAccount() string {
	return m.account
}

func TestEnhancedValidateTransactionDSL(t *testing.T) {
	validInput := &mockTransactionDSLValidator{
		asset: "USD",
		value: 100.0,
		sourceAccounts: []AccountReference{
			mockAccountReference{account: "account1"},
		},
		destinationAccounts: []AccountReference{
			mockAccountReference{account: "account2"},
		},
		metadata: map[string]any{
			"reference": "INV-123",
		},
	}

	tests := []struct {
		name    string
		input   TransactionDSLValidator
		wantErr bool
	}{
		{"Valid input", validInput, false},
		{"Nil input", nil, true},
		{
			"Invalid asset code",
			&mockTransactionDSLValidator{
				asset:               "US", // Too short
				value:               100.0,
				sourceAccounts:      []AccountReference{mockAccountReference{account: "account1"}},
				destinationAccounts: []AccountReference{mockAccountReference{account: "account2"}},
			},
			true,
		},
		{
			"Invalid amount",
			&mockTransactionDSLValidator{
				asset:               "USD",
				value:               0.0, // Zero amount
				sourceAccounts:      []AccountReference{mockAccountReference{account: "account1"}},
				destinationAccounts: []AccountReference{mockAccountReference{account: "account2"}},
			},
			true,
		},
		{
			"No source accounts",
			&mockTransactionDSLValidator{
				asset:               "USD",
				value:               100.0,
				sourceAccounts:      []AccountReference{},
				destinationAccounts: []AccountReference{mockAccountReference{account: "account2"}},
			},
			true,
		},
		{
			"No destination accounts",
			&mockTransactionDSLValidator{
				asset:               "USD",
				value:               100.0,
				sourceAccounts:      []AccountReference{mockAccountReference{account: "account1"}},
				destinationAccounts: []AccountReference{},
			},
			true,
		},
		{
			"External account with mismatched asset",
			&mockTransactionDSLValidator{
				asset:               "USD",
				value:               100.0,
				sourceAccounts:      []AccountReference{mockAccountReference{account: "@external/EUR"}},
				destinationAccounts: []AccountReference{mockAccountReference{account: "account2"}},
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := EnhancedValidateTransactionDSL(tt.input)
			if (errors != nil && errors.HasErrors()) != tt.wantErr {
				t.Errorf("EnhancedValidateTransactionDSL() errors = %v, wantErr %v", errors, tt.wantErr)
				return
			}

			if errors != nil && errors.HasErrors() {
				for _, err := range errors.Errors {
					if len(err.Suggestions) == 0 {
						t.Errorf("EnhancedValidateTransactionDSL() error should include suggestions: %v", err)
					}
				}
			}
		})
	}
}

func TestEnhancedValidateTransactionInput(t *testing.T) {
	validInput := map[string]any{
		"asset_code": "USD",
		"amount":     float64(1000),
		"scale":      2,
		"operations": []map[string]any{
			{
				"type":       "DEBIT",
				"account_id": "account1",
				"amount":     float64(1000),
			},
			{
				"type":       "CREDIT",
				"account_id": "account2",
				"amount":     float64(1000),
			},
		},
	}

	tests := []struct {
		name    string
		input   map[string]any
		wantErr bool
	}{
		{"Valid input", validInput, false},
		{"Nil input", nil, true},
		{
			"Missing asset code",
			map[string]any{
				"amount": float64(1000),
				"scale":  2,
				"operations": []map[string]any{
					{
						"type":       "DEBIT",
						"account_id": "account1",
						"amount":     float64(1000),
					},
					{
						"type":       "CREDIT",
						"account_id": "account2",
						"amount":     float64(1000),
					},
				},
			},
			true,
		},
		{
			"Missing operations",
			map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
			},
			true,
		},
		{
			"Unbalanced operations",
			map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{
						"type":       "DEBIT",
						"account_id": "account1",
						"amount":     float64(1000),
					},
					{
						"type":       "CREDIT",
						"account_id": "account2",
						"amount":     float64(500), // Only half the debit amount
					},
				},
			},
			true,
		},
		{
			"Missing operation type",
			map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{
						"account_id": "account1", // Missing type
						"amount":     float64(1000),
					},
					{
						"type":       "CREDIT",
						"account_id": "account2",
						"amount":     float64(1000),
					},
				},
			},
			true,
		},
		{
			"Invalid operation amount",
			map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{
						"type":       "DEBIT",
						"account_id": "account1",
						"amount":     float64(0), // Zero amount
					},
					{
						"type":       "CREDIT",
						"account_id": "account2",
						"amount":     float64(1000),
					},
				},
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := EnhancedValidateTransactionInput(tt.input)
			if (errors.HasErrors()) != tt.wantErr {
				t.Errorf("EnhancedValidateTransactionInput() errors = %v, wantErr %v", errors, tt.wantErr)
				return
			}

			if errors.HasErrors() {
				// Check that errors have suggestions
				for _, err := range errors.Errors {
					if len(err.Suggestions) == 0 {
						t.Errorf("EnhancedValidateTransactionInput() error should include suggestions: %v", err)
					}
				}
			}
		})
	}
}

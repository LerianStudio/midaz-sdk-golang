package core_test

import (
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAssetCode(t *testing.T) {
	tests := []struct {
		name      string
		assetCode string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "Valid 3-letter code",
			assetCode: "USD",
			wantErr:   false,
		},
		{
			name:      "Valid 4-letter code",
			assetCode: "USDT",
			wantErr:   false,
		},
		{
			name:      "Empty code",
			assetCode: "",
			wantErr:   true,
			errMsg:    "asset code is required",
		},
		{
			name:      "Lowercase code",
			assetCode: "usd",
			wantErr:   true,
			errMsg:    "invalid asset code format: usd (must be 3-4 uppercase letters)",
		},
		{
			name:      "Too long code",
			assetCode: "USDOL",
			wantErr:   true,
			errMsg:    "invalid asset code format: USDOL (must be 3-4 uppercase letters)",
		},
		{
			name:      "With numbers",
			assetCode: "US1",
			wantErr:   true,
			errMsg:    "invalid asset code format: US1 (must be 3-4 uppercase letters)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateAssetCode(tt.assetCode)
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateAccountAlias(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Valid simple alias",
			alias:   "savings",
			wantErr: false,
		},
		{
			name:    "Valid with underscore",
			alias:   "savings_account",
			wantErr: false,
		},
		{
			name:    "Valid with hyphen",
			alias:   "savings-account",
			wantErr: false,
		},
		{
			name:    "Valid with numbers",
			alias:   "savings123",
			wantErr: false,
		},
		{
			name:    "Empty alias",
			alias:   "",
			wantErr: true,
			errMsg:  "account alias cannot be empty",
		},
		{
			name:    "With special characters",
			alias:   "savings@account",
			wantErr: true,
			errMsg:  "invalid account alias format: savings@account (must be alphanumeric with optional underscores and hyphens, max 50 chars)",
		},
		{
			name:    "Too long alias",
			alias:   "a123456789012345678901234567890123456789012345678901", // 51 chars
			wantErr: true,
			errMsg:  "invalid account alias format: a123456789012345678901234567890123456789012345678901 (must be alphanumeric with optional underscores and hyphens, max 50 chars)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateAccountAlias(tt.alias)
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateTransactionCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Valid simple code",
			code:    "TX123",
			wantErr: false,
		},
		{
			name:    "Valid with underscore",
			code:    "TX_123",
			wantErr: false,
		},
		{
			name:    "Valid with hyphen",
			code:    "TX-123",
			wantErr: false,
		},
		{
			name:    "Empty code",
			code:    "",
			wantErr: true,
			errMsg:  "transaction code cannot be empty",
		},
		{
			name:    "With special characters",
			code:    "TX@123",
			wantErr: true,
			errMsg:  "invalid transaction code format: TX@123 (must be alphanumeric with optional underscores and hyphens, max 100 chars)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateTransactionCode(tt.code)
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "Nil metadata",
			metadata: nil,
			wantErr:  false,
		},
		{
			name:     "Empty metadata",
			metadata: map[string]any{},
			wantErr:  false,
		},
		{
			name: "Valid metadata with string",
			metadata: map[string]any{
				"reference": "inv123",
			},
			wantErr: false,
		},
		{
			name: "Valid metadata with number",
			metadata: map[string]any{
				"amount": 100.50,
			},
			wantErr: false,
		},
		{
			name: "Valid metadata with boolean",
			metadata: map[string]any{
				"is_paid": true,
			},
			wantErr: false,
		},
		{
			name: "Valid metadata with nil",
			metadata: map[string]any{
				"optional_field": nil,
			},
			wantErr: false,
		},
		{
			name: "Valid metadata with nested map",
			metadata: map[string]any{
				"customer": map[string]any{
					"id":   "cust123",
					"name": "John Doe",
				},
			},
			wantErr: false,
		},
		{
			name: "Valid metadata with array",
			metadata: map[string]any{
				"items": []any{"item1", "item2", "item3"},
			},
			wantErr: false,
		},
		{
			name: "Empty key",
			metadata: map[string]any{
				"": "value",
			},
			wantErr: true,
			errMsg:  "metadata keys cannot be empty",
		},
		{
			name: "Invalid value type",
			metadata: map[string]any{
				"complex": complex(1, 2),
			},
			wantErr: true,
			errMsg:  "invalid metadata value type for key 'complex': complex128 (must be string, number, boolean, or nil)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateMetadata(tt.metadata)
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateDateRange(t *testing.T) {
	tests := []struct {
		name    string
		start   time.Time
		end     time.Time
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Valid date range",
			start:   time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			end:     time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "Same start and end date",
			start:   time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			end:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "Start date after end date",
			start:   time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			end:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr: true,
			errMsg:  "start date (2023-12-31T00:00:00Z) cannot be after end date (2023-01-01T00:00:00Z)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateDateRange(tt.start, tt.end)
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateAddress(t *testing.T) {
	line2Value := "Apt 4B"
	longLine := string(make([]byte, 101))
	longZip := string(make([]byte, 21))
	longCity := string(make([]byte, 101))
	longState := string(make([]byte, 101))

	tests := []struct {
		name    string
		address *core.Address
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid address",
			address: &core.Address{
				Line1:   "123 Main St",
				Line2:   nil,
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr: false,
		},
		{
			name: "Valid address with line2",
			address: &core.Address{
				Line1:   "123 Main St",
				Line2:   &line2Value,
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr: false,
		},
		{
			name:    "Nil address",
			address: nil,
			wantErr: true,
			errMsg:  "address cannot be nil",
		},
		{
			name: "Empty line1",
			address: &core.Address{
				Line1:   "",
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr: true,
			errMsg:  "address line 1 is required",
		},
		{
			name: "Line1 too long",
			address: &core.Address{
				Line1:   longLine,
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr: true,
			errMsg:  "address line 1 must be at most 100 characters",
		},
		{
			name: "Line2 too long",
			address: &core.Address{
				Line1:   "123 Main St",
				Line2:   &longLine,
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr: true,
			errMsg:  "address line 2 must be at most 100 characters",
		},
		{
			name: "Empty zip code",
			address: &core.Address{
				Line1:   "123 Main St",
				ZipCode: "",
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr: true,
			errMsg:  "zip code is required",
		},
		{
			name: "Zip code too long",
			address: &core.Address{
				Line1:   "123 Main St",
				ZipCode: longZip,
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr: true,
			errMsg:  "zip code must be at most 20 characters",
		},
		{
			name: "Empty city",
			address: &core.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    "",
				State:   "NY",
				Country: "US",
			},
			wantErr: true,
			errMsg:  "city is required",
		},
		{
			name: "City too long",
			address: &core.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    longCity,
				State:   "NY",
				Country: "US",
			},
			wantErr: true,
			errMsg:  "city must be at most 100 characters",
		},
		{
			name: "Empty state",
			address: &core.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    "New York",
				State:   "",
				Country: "US",
			},
			wantErr: true,
			errMsg:  "state is required",
		},
		{
			name: "State too long",
			address: &core.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    "New York",
				State:   longState,
				Country: "US",
			},
			wantErr: true,
			errMsg:  "state must be at most 100 characters",
		},
		{
			name: "Empty country",
			address: &core.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "",
			},
			wantErr: true,
			errMsg:  "country is required",
		},
		{
			name: "Invalid country code",
			address: &core.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "XX",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateAddress(tt.address)
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidationConfig(t *testing.T) {
	t.Run("DefaultValidationConfig returns valid defaults", func(t *testing.T) {
		config := core.DefaultValidationConfig()
		assert.NotNil(t, config)
		assert.Equal(t, 4096, config.MaxMetadataSize)
		assert.Equal(t, 256, config.MaxStringLength)
		assert.Equal(t, 100, config.MaxAddressLineLength)
		assert.Equal(t, 20, config.MaxZipCodeLength)
		assert.Equal(t, 100, config.MaxCityLength)
		assert.Equal(t, 100, config.MaxStateLength)
		assert.False(t, config.StrictMode)
	})

	t.Run("NewValidationConfig with options", func(t *testing.T) {
		config, err := core.NewValidationConfig(
			core.WithMaxMetadataSize(8192),
			core.WithMaxStringLength(512),
			core.WithMaxAddressLineLength(200),
			core.WithMaxZipCodeLength(30),
			core.WithMaxCityLength(150),
			core.WithMaxStateLength(150),
			core.WithStrictMode(true),
		)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, 8192, config.MaxMetadataSize)
		assert.Equal(t, 512, config.MaxStringLength)
		assert.Equal(t, 200, config.MaxAddressLineLength)
		assert.Equal(t, 30, config.MaxZipCodeLength)
		assert.Equal(t, 150, config.MaxCityLength)
		assert.Equal(t, 150, config.MaxStateLength)
		assert.True(t, config.StrictMode)
	})

	t.Run("WithMaxMetadataSize error for zero", func(t *testing.T) {
		_, err := core.NewValidationConfig(core.WithMaxMetadataSize(0))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max metadata size must be positive")
	})

	t.Run("WithMaxMetadataSize error for negative", func(t *testing.T) {
		_, err := core.NewValidationConfig(core.WithMaxMetadataSize(-1))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max metadata size must be positive")
	})

	t.Run("WithMaxStringLength error for zero", func(t *testing.T) {
		_, err := core.NewValidationConfig(core.WithMaxStringLength(0))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max string length must be positive")
	})

	t.Run("WithMaxStringLength error for negative", func(t *testing.T) {
		_, err := core.NewValidationConfig(core.WithMaxStringLength(-1))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max string length must be positive")
	})

	t.Run("WithMaxAddressLineLength error for zero", func(t *testing.T) {
		_, err := core.NewValidationConfig(core.WithMaxAddressLineLength(0))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max address line length must be positive")
	})

	t.Run("WithMaxAddressLineLength error for negative", func(t *testing.T) {
		_, err := core.NewValidationConfig(core.WithMaxAddressLineLength(-1))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max address line length must be positive")
	})

	t.Run("WithMaxZipCodeLength error for zero", func(t *testing.T) {
		_, err := core.NewValidationConfig(core.WithMaxZipCodeLength(0))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max zip code length must be positive")
	})

	t.Run("WithMaxZipCodeLength error for negative", func(t *testing.T) {
		_, err := core.NewValidationConfig(core.WithMaxZipCodeLength(-1))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max zip code length must be positive")
	})

	t.Run("WithMaxCityLength error for zero", func(t *testing.T) {
		_, err := core.NewValidationConfig(core.WithMaxCityLength(0))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max city length must be positive")
	})

	t.Run("WithMaxCityLength error for negative", func(t *testing.T) {
		_, err := core.NewValidationConfig(core.WithMaxCityLength(-1))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max city length must be positive")
	})

	t.Run("WithMaxStateLength error for zero", func(t *testing.T) {
		_, err := core.NewValidationConfig(core.WithMaxStateLength(0))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max state length must be positive")
	})

	t.Run("WithMaxStateLength error for negative", func(t *testing.T) {
		_, err := core.NewValidationConfig(core.WithMaxStateLength(-1))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max state length must be positive")
	})

	t.Run("WithStrictMode true", func(t *testing.T) {
		config, err := core.NewValidationConfig(core.WithStrictMode(true))
		require.NoError(t, err)
		assert.True(t, config.StrictMode)
	})

	t.Run("WithStrictMode false", func(t *testing.T) {
		config, err := core.NewValidationConfig(core.WithStrictMode(false))
		require.NoError(t, err)
		assert.False(t, config.StrictMode)
	})
}

func TestValidateAccountType(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		wantErr     bool
		errContains string
	}{
		{
			name:        "Valid deposit type",
			accountType: "deposit",
			wantErr:     false,
		},
		{
			name:        "Valid savings type",
			accountType: "savings",
			wantErr:     false,
		},
		{
			name:        "Valid loans type",
			accountType: "loans",
			wantErr:     false,
		},
		{
			name:        "Valid marketplace type",
			accountType: "marketplace",
			wantErr:     false,
		},
		{
			name:        "Valid creditCard type",
			accountType: "creditCard",
			wantErr:     false,
		},
		{
			name:        "Empty account type",
			accountType: "",
			wantErr:     true,
			errContains: "account type is required",
		},
		{
			name:        "Invalid account type",
			accountType: "invalid_type",
			wantErr:     true,
			errContains: "invalid account type",
		},
		{
			name:        "Invalid account type - uppercase",
			accountType: "DEPOSIT",
			wantErr:     true,
			errContains: "invalid account type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateAccountType(tt.accountType)
			if tt.wantErr {
				require.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateAssetType(t *testing.T) {
	tests := []struct {
		name        string
		assetType   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "Valid crypto type",
			assetType: "crypto",
			wantErr:   false,
		},
		{
			name:      "Valid currency type",
			assetType: "currency",
			wantErr:   false,
		},
		{
			name:      "Valid commodity type",
			assetType: "commodity",
			wantErr:   false,
		},
		{
			name:      "Valid others type",
			assetType: "others",
			wantErr:   false,
		},
		{
			name:      "Valid uppercase CRYPTO",
			assetType: "CRYPTO",
			wantErr:   false,
		},
		{
			name:      "Valid mixed case Crypto",
			assetType: "Crypto",
			wantErr:   false,
		},
		{
			name:        "Empty asset type",
			assetType:   "",
			wantErr:     true,
			errContains: "asset type is required",
		},
		{
			name:        "Invalid asset type",
			assetType:   "invalid_type",
			wantErr:     true,
			errContains: "invalid asset type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateAssetType(tt.assetType)
			if tt.wantErr {
				require.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateCurrencyCode(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "Valid USD",
			code:    "USD",
			wantErr: false,
		},
		{
			name:    "Valid EUR",
			code:    "EUR",
			wantErr: false,
		},
		{
			name:    "Valid JPY",
			code:    "JPY",
			wantErr: false,
		},
		{
			name:    "Valid BRL",
			code:    "BRL",
			wantErr: false,
		},
		{
			name:        "Empty currency code",
			code:        "",
			wantErr:     true,
			errContains: "currency code cannot be empty",
		},
		{
			name:        "Invalid currency code",
			code:        "XXX",
			wantErr:     true,
			errContains: "invalid currency code",
		},
		{
			name:        "Too short currency code",
			code:        "US",
			wantErr:     true,
			errContains: "invalid currency code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateCurrencyCode(tt.code)
			if tt.wantErr {
				require.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateCountryCode(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "Valid US",
			code:    "US",
			wantErr: false,
		},
		{
			name:    "Valid BR",
			code:    "BR",
			wantErr: false,
		},
		{
			name:    "Valid GB",
			code:    "GB",
			wantErr: false,
		},
		{
			name:    "Valid JP",
			code:    "JP",
			wantErr: false,
		},
		{
			name:        "Empty country code",
			code:        "",
			wantErr:     true,
			errContains: "country code cannot be empty",
		},
		{
			name:        "Invalid country code - three letters",
			code:        "USA",
			wantErr:     true,
			errContains: "invalid country code",
		},
		{
			name:        "Invalid country code - lowercase",
			code:        "us",
			wantErr:     true,
			errContains: "invalid country code",
		},
		{
			name:        "Invalid country code - unknown",
			code:        "XX",
			wantErr:     true,
			errContains: "invalid country code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateCountryCode(tt.code)
			if tt.wantErr {
				require.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMetadataWithNestedStructures(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		wantErr  bool
		errMsg   string
	}{
		{
			name: "Valid nested map",
			metadata: map[string]any{
				"customer": map[string]any{
					"id":   "cust123",
					"name": "John Doe",
					"age":  30,
				},
			},
			wantErr: false,
		},
		{
			name: "Valid array",
			metadata: map[string]any{
				"items": []any{"item1", "item2", "item3"},
			},
			wantErr: false,
		},
		{
			name: "Valid array with mixed types",
			metadata: map[string]any{
				"mixed": []any{"string", 123, 45.67, true, nil},
			},
			wantErr: false,
		},
		{
			name: "Valid deeply nested map",
			metadata: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": "deep value",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Valid array with nested map",
			metadata: map[string]any{
				"items": []any{
					map[string]any{
						"id":   1,
						"name": "item1",
					},
					map[string]any{
						"id":   2,
						"name": "item2",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Valid metadata with int32",
			metadata: map[string]any{
				"count": int32(100),
			},
			wantErr: false,
		},
		{
			name: "Valid metadata with int64",
			metadata: map[string]any{
				"count": int64(100),
			},
			wantErr: false,
		},
		{
			name: "Valid metadata with float32",
			metadata: map[string]any{
				"rate": float32(3.14),
			},
			wantErr: false,
		},
		{
			name: "Invalid nested map with empty key",
			metadata: map[string]any{
				"customer": map[string]any{
					"": "invalid key",
				},
			},
			wantErr: true,
			errMsg:  "metadata keys cannot be empty",
		},
		{
			name: "Invalid array with unsupported type",
			metadata: map[string]any{
				"items": []any{
					complex(1, 2),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateMetadata(tt.metadata)
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateTransactionCodeBoundaryValues(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{
			name:    "Single character code",
			code:    "a",
			wantErr: false,
		},
		{
			name:    "Exactly 100 characters",
			code:    "a234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890",
			wantErr: false,
		},
		{
			name:    "101 characters - exceeds limit",
			code:    "a2345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901",
			wantErr: true,
		},
		{
			name:    "Only numbers",
			code:    "1234567890",
			wantErr: false,
		},
		{
			name:    "Only underscores and hyphens",
			code:    "_-_-_-_",
			wantErr: false,
		},
		{
			name:    "With spaces - invalid",
			code:    "tx 123",
			wantErr: true,
		},
		{
			name:    "With dot - invalid",
			code:    "tx.123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateTransactionCode(tt.code)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateAccountAliasBoundaryValues(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		wantErr bool
	}{
		{
			name:    "Single character alias",
			alias:   "a",
			wantErr: false,
		},
		{
			name:    "Exactly 50 characters",
			alias:   "a2345678901234567890123456789012345678901234567890",
			wantErr: false,
		},
		{
			name:    "51 characters - exceeds limit",
			alias:   "a23456789012345678901234567890123456789012345678901",
			wantErr: true,
		},
		{
			name:    "Only numbers",
			alias:   "1234567890",
			wantErr: false,
		},
		{
			name:    "Only underscores and hyphens",
			alias:   "_-_-_",
			wantErr: false,
		},
		{
			name:    "Mixed alphanumeric",
			alias:   "Account_123-test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateAccountAlias(tt.alias)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateDateRangeEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		start   time.Time
		end     time.Time
		wantErr bool
	}{
		{
			name:    "Start one nanosecond before end",
			start:   time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			end:     time.Date(2023, 1, 1, 0, 0, 0, 1, time.UTC),
			wantErr: false,
		},
		{
			name:    "Start one nanosecond after end",
			start:   time.Date(2023, 1, 1, 0, 0, 0, 1, time.UTC),
			end:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr: true,
		},
		{
			name:    "Very old date",
			start:   time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
			end:     time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "Future dates",
			start:   time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
			end:     time.Date(2200, 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "Different timezones - effectively same moment",
			start:   time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			end:     time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateDateRange(tt.start, tt.end)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRegexPatterns(t *testing.T) {
	t.Run("ExternalAccountPattern", func(t *testing.T) {
		validCases := []string{
			"@external/USD",
			"@external/EUR",
			"@external/BTC",
			"@external/USDT",
		}
		for _, tc := range validCases {
			assert.True(t, core.ExternalAccountPattern.MatchString(tc), "Expected %s to match", tc)
		}

		invalidCases := []string{
			"@external/us",
			"@external/USDOL",
			"external/USD",
			"@ext/USD",
			"@external/123",
			"@external/",
		}
		for _, tc := range invalidCases {
			assert.False(t, core.ExternalAccountPattern.MatchString(tc), "Expected %s to not match", tc)
		}
	})

	t.Run("AccountAliasPattern", func(t *testing.T) {
		validCases := []string{
			"savings",
			"SAVINGS",
			"savings_account",
			"savings-account",
			"savings123",
			"a",
			"a2345678901234567890123456789012345678901234567890",
		}
		for _, tc := range validCases {
			assert.True(t, core.AccountAliasPattern.MatchString(tc), "Expected %s to match", tc)
		}

		invalidCases := []string{
			"",
			"savings account",
			"savings@account",
			"savings.account",
			"a23456789012345678901234567890123456789012345678901",
		}
		for _, tc := range invalidCases {
			assert.False(t, core.AccountAliasPattern.MatchString(tc), "Expected %s to not match", tc)
		}
	})

	t.Run("AssetCodePattern", func(t *testing.T) {
		validCases := []string{
			"USD",
			"EUR",
			"BTC",
			"USDT",
			"ETH",
		}
		for _, tc := range validCases {
			assert.True(t, core.AssetCodePattern.MatchString(tc), "Expected %s to match", tc)
		}

		invalidCases := []string{
			"",
			"us",
			"USDOL",
			"US",
			"USD1",
			"123",
		}
		for _, tc := range invalidCases {
			assert.False(t, core.AssetCodePattern.MatchString(tc), "Expected %s to not match", tc)
		}
	})

	t.Run("TransactionCodePattern", func(t *testing.T) {
		validCases := []string{
			"TX123",
			"tx-123",
			"TX_123",
			"a",
			"a234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890",
		}
		for _, tc := range validCases {
			assert.True(t, core.TransactionCodePattern.MatchString(tc), "Expected %s to match", tc)
		}

		invalidCases := []string{
			"",
			"TX 123",
			"TX@123",
			"a2345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901",
		}
		for _, tc := range invalidCases {
			assert.False(t, core.TransactionCodePattern.MatchString(tc), "Expected %s to not match", tc)
		}
	})
}

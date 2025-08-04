package core_test

import (
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation/core"
	"github.com/stretchr/testify/assert"
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
				assert.Error(t, err)

				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
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
				assert.Error(t, err)

				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
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
				assert.Error(t, err)

				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
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
				assert.Error(t, err)

				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
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
				assert.Error(t, err)

				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAddress(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateAddress(tt.address)
			if tt.wantErr {
				assert.Error(t, err)

				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

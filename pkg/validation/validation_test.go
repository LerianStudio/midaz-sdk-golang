package validation_test

import (
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation/core"
	"github.com/stretchr/testify/assert"
)

func TestValidateTransactionDSL(t *testing.T) {
	testCases := []struct {
		name          string
		input         validation.TransactionDSLValidator
		expectedError bool
		errorContains string
	}{
		{
			name:          "Nil input",
			input:         nil,
			expectedError: true,
			errorContains: "cannot be nil",
		},
		{
			name: "Nil send object",
			input: &models.TransactionDSLInput{
				Send: &models.DSLSend{},
			},
			expectedError: true,
			errorContains: "asset code is required",
		},
		{
			name: "Empty asset code",
			input: &models.TransactionDSLInput{
				Send: &models.DSLSend{},
			},
			expectedError: true,
			errorContains: "asset code is required",
		},
		{
			name: "Invalid asset code",
			input: &models.TransactionDSLInput{
				Send: &models.DSLSend{
					Asset: "us",
				},
			},
			expectedError: true,
			errorContains: "invalid asset code format",
		},
		{
			name: "Zero amount",
			input: &models.TransactionDSLInput{
				Send: &models.DSLSend{
					Asset: "USD",
					Value: "",
				},
			},
			expectedError: true,
			errorContains: "transaction amount must be greater than zero",
		},
		{
			name: "Negative amount",
			input: &models.TransactionDSLInput{
				Send: &models.DSLSend{
					Asset: "USD",
					Value: "-1.00",
				},
			},
			expectedError: true,
			errorContains: "transaction amount must be greater than zero",
		},
		{
			name: "No source accounts",
			input: &models.TransactionDSLInput{
				Send: &models.DSLSend{
					Asset:  "USD",
					Value:  "1.00",
					Source: &models.DSLSource{},
				},
			},
			expectedError: true,
			errorContains: "at least one source account is required",
		},
		{
			name: "Invalid source account format",
			input: &models.TransactionDSLInput{
				Send: &models.DSLSend{
					Asset: "USD",
					Value: "1.00",
					Source: &models.DSLSource{
						From: []models.DSLFromTo{
							{Account: "@external/INV@LID"},
						},
					},
					Distribute: &models.DSLDistribute{
						To: []models.DSLFromTo{
							{Account: "account2"},
						},
					},
				},
			},
			expectedError: true,
			errorContains: "invalid external account format",
		},
		{
			name: "No destination accounts",
			input: &models.TransactionDSLInput{
				Send: &models.DSLSend{
					Asset: "USD",
					Value: "1.00",
					Source: &models.DSLSource{
						From: []models.DSLFromTo{
							{Account: "account1"},
						},
					},
					Distribute: &models.DSLDistribute{},
				},
			},
			expectedError: true,
			errorContains: "at least one destination account is required",
		},
		{
			name: "Invalid destination account format",
			input: &models.TransactionDSLInput{
				Send: &models.DSLSend{
					Asset: "USD",
					Value: "1.00",
					Source: &models.DSLSource{
						From: []models.DSLFromTo{
							{Account: "account1"},
						},
					},
					Distribute: &models.DSLDistribute{
						To: []models.DSLFromTo{
							{Account: "@external/INV@LID"},
						},
					},
				},
			},
			expectedError: true,
			errorContains: "invalid external account format",
		},
		{
			name: "Asset mismatch in external account",
			input: &models.TransactionDSLInput{
				Send: &models.DSLSend{
					Asset: "USD",
					Value: "1.00",
					Source: &models.DSLSource{
						From: []models.DSLFromTo{
							{Account: "account1"},
						},
					},
					Distribute: &models.DSLDistribute{
						To: []models.DSLFromTo{
							{Account: "@external/EUR"},
						},
					},
				},
			},
			expectedError: true,
			errorContains: "external account asset (EUR) must match transaction asset (USD)",
		},
		{
			name: "Invalid metadata",
			input: &models.TransactionDSLInput{
				Send: &models.DSLSend{
					Asset: "USD",
					Value: "1.00",
					Source: &models.DSLSource{
						From: []models.DSLFromTo{
							{Account: "account1"},
						},
					},
					Distribute: &models.DSLDistribute{
						To: []models.DSLFromTo{
							{Account: "account2"},
						},
					},
				},
				Metadata: map[string]any{
					"key": []string{"invalid type"},
				},
			},
			expectedError: true,
			errorContains: "invalid metadata",
		},
		{
			name: "Valid input",
			input: &models.TransactionDSLInput{
				Send: &models.DSLSend{
					Asset: "USD",
					Value: "1.00",
					Source: &models.DSLSource{
						From: []models.DSLFromTo{
							{Account: "account1"},
						},
					},
					Distribute: &models.DSLDistribute{
						To: []models.DSLFromTo{
							{Account: "account2"},
						},
					},
				},
				Metadata: map[string]any{
					"reference": "TX-123456",
					"amount":    100.50,
					"approved":  true,
				},
			},
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.ValidateTransactionDSL(tc.input)
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAssetCode(t *testing.T) {
	testCases := []struct {
		name          string
		assetCode     string
		expectedError bool
		errorContains string
	}{
		{
			name:          "Empty asset code",
			assetCode:     "",
			expectedError: true,
			errorContains: "asset code cannot be empty",
		},
		{
			name:          "Too short asset code",
			assetCode:     "US",
			expectedError: true,
			errorContains: "invalid asset code format",
		},
		{
			name:          "Too long asset code",
			assetCode:     "USDOL",
			expectedError: true,
			errorContains: "invalid asset code format",
		},
		{
			name:          "Lowercase asset code",
			assetCode:     "usd",
			expectedError: true,
			errorContains: "invalid asset code format",
		},
		{
			name:          "Mixed case asset code",
			assetCode:     "Usd",
			expectedError: true,
			errorContains: "invalid asset code format",
		},
		{
			name:          "Non-alphabetic asset code",
			assetCode:     "US1",
			expectedError: true,
			errorContains: "invalid asset code format",
		},
		{
			name:          "Valid 3-letter asset code",
			assetCode:     "USD",
			expectedError: false,
		},
		{
			name:          "Valid 4-letter asset code",
			assetCode:     "USDT",
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.ValidateAssetCode(tc.assetCode)
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAccountAlias(t *testing.T) {
	testCases := []struct {
		name          string
		alias         string
		expectedError bool
		errorContains string
	}{
		{
			name:          "Empty alias",
			alias:         "",
			expectedError: true,
			errorContains: "account alias cannot be empty",
		},
		{
			name:          "Valid alphanumeric alias",
			alias:         "savings123",
			expectedError: false,
		},
		{
			name:          "Valid alias with underscore",
			alias:         "savings_account",
			expectedError: false,
		},
		{
			name:          "Valid alias with hyphen",
			alias:         "savings-account",
			expectedError: false,
		},
		{
			name:          "Valid alias with mixed case",
			alias:         "SavingsAccount",
			expectedError: false,
		},
		{
			name:          "Invalid alias with space",
			alias:         "savings account",
			expectedError: true,
			errorContains: "invalid account alias format",
		},
		{
			name:          "Invalid alias with special character",
			alias:         "savings@account",
			expectedError: true,
			errorContains: "invalid account alias format",
		},
		{
			name:          "Too long alias",
			alias:         "this_is_a_very_long_account_alias_that_exceeds_the_maximum_allowed_length_of_fifty_characters_which_is_not_allowed",
			expectedError: true,
			errorContains: "invalid account alias format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.ValidateAccountAlias(tc.alias)
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTransactionCode(t *testing.T) {
	testCases := []struct {
		name          string
		code          string
		expectedError bool
		errorContains string
	}{
		{
			name:          "Empty code",
			code:          "",
			expectedError: true,
			errorContains: "transaction code cannot be empty",
		},
		{
			name:          "Valid alphanumeric code",
			code:          "TX123456",
			expectedError: false,
		},
		{
			name:          "Valid code with underscore",
			code:          "TX_123456",
			expectedError: false,
		},
		{
			name:          "Valid code with hyphen",
			code:          "TX-123456",
			expectedError: false,
		},
		{
			name:          "Valid code with mixed case",
			code:          "Tx123456",
			expectedError: false,
		},
		{
			name:          "Invalid code with space",
			code:          "TX 123456",
			expectedError: true,
			errorContains: "invalid transaction code format",
		},
		{
			name:          "Invalid code with special character",
			code:          "TX@123456",
			expectedError: true,
			errorContains: "invalid transaction code format",
		},
		{
			name:          "Too long code",
			code:          "TX_123456_THIS_IS_A_VERY_LONG_TRANSACTION_CODE_THAT_EXCEEDS_THE_MAXIMUM_ALLOWED_LENGTH",
			expectedError: true,
			errorContains: "invalid transaction code format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.ValidateTransactionCode(tc.code)
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMetadata(t *testing.T) {
	testCases := []struct {
		name          string
		metadata      map[string]any
		expectedError bool
		errorContains string
	}{
		{
			name:          "Nil metadata",
			metadata:      nil,
			expectedError: false,
		},
		{
			name:          "Empty metadata",
			metadata:      map[string]any{},
			expectedError: false,
		},
		{
			name: "Valid metadata with string",
			metadata: map[string]any{
				"reference": "TX-123456",
			},
			expectedError: false,
		},
		{
			name: "Valid metadata with boolean",
			metadata: map[string]any{
				"approved": true,
			},
			expectedError: false,
		},
		{
			name: "Valid metadata with integer",
			metadata: map[string]any{
				"customer_id": 12345,
			},
			expectedError: false,
		},
		{
			name: "Valid metadata with float",
			metadata: map[string]any{
				"amount": 100.50,
			},
			expectedError: false,
		},
		{
			name: "Valid metadata with nil value",
			metadata: map[string]any{
				"optional_field": nil,
			},
			expectedError: false,
		},
		{
			name: "Valid metadata with multiple types",
			metadata: map[string]any{
				"reference":   "TX-123456",
				"amount":      100.50,
				"customer_id": 12345,
				"approved":    true,
				"notes":       nil,
			},
			expectedError: false,
		},
		{
			name: "Invalid metadata with empty key",
			metadata: map[string]any{
				"": "value",
			},
			expectedError: true,
			errorContains: "metadata key cannot be empty",
		},
		{
			name: "Invalid metadata with too long key",
			metadata: map[string]any{
				"this_is_a_very_long_metadata_key_that_exceeds_the_maximum_allowed_length_of_sixty_four_characters": "value",
			},
			expectedError: true,
			errorContains: "exceeds maximum length of 64 characters",
		},
		{
			name: "Invalid metadata with unsupported type (slice)",
			metadata: map[string]any{
				"tags": []string{"tag1", "tag2"},
			},
			expectedError: true,
			errorContains: "unsupported type",
		},
		{
			name: "Invalid metadata with unsupported type (map)",
			metadata: map[string]any{
				"nested": map[string]string{"key": "value"},
			},
			expectedError: true,
			errorContains: "unsupported type",
		},
		{
			name: "Invalid metadata with string too long",
			metadata: map[string]any{
				"description": string(make([]byte, 300)),
			},
			expectedError: true,
			errorContains: "exceeds maximum length",
		},
		{
			name: "Invalid metadata with integer out of range",
			metadata: map[string]any{
				"big_number": 10000000000,
			},
			expectedError: true,
			errorContains: "outside allowed range",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.ValidateMetadata(tc.metadata)
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDateRange(t *testing.T) {
	testCases := []struct {
		name          string
		start         time.Time
		end           time.Time
		expectedError bool
		errorContains string
	}{
		{
			name:          "Valid date range",
			start:         time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			end:           time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			expectedError: false,
		},
		{
			name:          "Same start and end date",
			start:         time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			end:           time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedError: false,
		},
		{
			name:          "Start date after end date",
			start:         time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			end:           time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedError: true,
			errorContains: "cannot be after end date",
		},
		{
			name:          "Zero start date",
			start:         time.Time{},
			end:           time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			expectedError: true,
			errorContains: "start date cannot be empty",
		},
		{
			name:          "Zero end date",
			start:         time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			end:           time.Time{},
			expectedError: true,
			errorContains: "end date cannot be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.ValidateDateRange(tc.start, tc.end)
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatorWithOptions(t *testing.T) {
	// Create validator with custom options
	validator, err := validation.NewValidator(
		core.WithMaxStringLength(10),
		core.WithMaxMetadataSize(100),
		core.WithStrictMode(true),
	)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	// Test metadata validation with custom string length limit
	t.Run("Metadata validation with custom string length", func(t *testing.T) {
		// This should pass with default validator
		metadata1 := map[string]any{
			"key": "12345678901234567890", // 20 chars
		}
		if err := validation.ValidateMetadata(metadata1); err != nil {
			t.Errorf("Default validator should accept 20-char string: %v", err)
		}

		// This should fail with custom validator (10 char limit)
		if err := validator.ValidateMetadata(metadata1); err == nil {
			t.Error("Custom validator should reject string longer than 10 chars")
		}

		// This should pass with custom validator
		metadata2 := map[string]any{
			"key": "1234567890", // 10 chars
		}
		if err := validator.ValidateMetadata(metadata2); err != nil {
			t.Errorf("Custom validator should accept 10-char string: %v", err)
		}
	})

	// Test metadata size limit
	t.Run("Metadata validation with custom size limit", func(t *testing.T) {
		// Create large metadata that exceeds custom 100 byte limit
		largeMetadata := map[string]any{
			"key1": "value1-very-long-string-to-test-size-limits-exceed-100-bytes-easily",
			"key2": "value2-another-long-string-to-make-sure-we-go-over-the-limit-set-to-100-bytes",
		}

		// Should fail with custom validator (100 byte limit)
		if err := validator.ValidateMetadata(largeMetadata); err == nil {
			t.Error("Custom validator should reject metadata larger than 100 bytes")
		}

		// Should pass with default validator (4096 byte limit)
		if err := validation.ValidateMetadata(largeMetadata); err != nil {
			t.Errorf("Default validator should accept metadata under 4KB: %v", err)
		}
	})

	// Test address validation with custom length limits
	t.Run("Address validation with custom line length", func(t *testing.T) {
		// Create a custom validator with small address line length
		smallLinesValidator, err := validation.NewValidator(
			core.WithMaxAddressLineLength(20),
		)
		if err != nil {
			t.Fatalf("Failed to create smallLinesValidator: %v", err)
		}

		// Create address with long lines
		longLineAddress := &validation.Address{
			Line1:   "1234567890123456789012345678901234567890", // 40 chars
			ZipCode: "12345",
			City:    "Test City",
			State:   "Test State",
			Country: "US",
		}

		// Should fail with custom validator (20 char limit)
		if err := smallLinesValidator.ValidateAddress(longLineAddress); err == nil {
			t.Error("Custom validator should reject address line longer than 20 chars")
		}

		// Should pass with default validator (100 char default limit)
		if err := validation.ValidateAddress(longLineAddress); err != nil {
			t.Errorf("Default validator should accept address lines under 100 chars: %v", err)
		}
	})
}

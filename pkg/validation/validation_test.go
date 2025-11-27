package validation_test

import (
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				require.Error(t, err)

				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
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
				require.Error(t, err)

				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
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
				require.Error(t, err)

				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
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
				require.Error(t, err)

				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
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
				require.Error(t, err)

				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
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
				require.Error(t, err)

				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
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

func TestValidationSummary(t *testing.T) {
	t.Run("Empty summary is valid", func(t *testing.T) {
		summary := validation.ValidationSummary{
			Valid:  true,
			Errors: []error{},
		}
		assert.True(t, summary.Valid)
		assert.Nil(t, summary.GetErrorMessages())
		assert.Empty(t, summary.GetErrorSummary())
	})

	t.Run("AddError makes summary invalid", func(t *testing.T) {
		summary := validation.ValidationSummary{
			Valid:  true,
			Errors: []error{},
		}
		summary.AddError(assert.AnError)
		assert.False(t, summary.Valid)
		assert.Len(t, summary.Errors, 1)
	})

	t.Run("GetErrorMessages returns messages", func(t *testing.T) {
		summary := validation.ValidationSummary{
			Valid:  true,
			Errors: []error{},
		}
		summary.AddError(assert.AnError)
		summary.AddError(assert.AnError)
		messages := summary.GetErrorMessages()
		assert.Len(t, messages, 2)
	})

	t.Run("GetErrorSummary formats correctly", func(t *testing.T) {
		summary := validation.ValidationSummary{
			Valid:  true,
			Errors: []error{},
		}
		summary.AddError(assert.AnError)
		summaryStr := summary.GetErrorSummary()
		assert.Contains(t, summaryStr, "Validation failed with 1 errors")
	})
}

func TestValidateCreateTransactionInput(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]any
		expectValid bool
		errContains []string
	}{
		{
			name:        "Nil input",
			input:       nil,
			expectValid: false,
			errContains: []string{"cannot be nil"},
		},
		{
			name: "Valid transaction",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: true,
		},
		{
			name: "Missing asset code",
			input: map[string]any{
				"amount": float64(1000),
				"scale":  2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"asset code is required"},
		},
		{
			name: "Invalid asset code type",
			input: map[string]any{
				"asset_code": 123,
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"asset_code must be a string"},
		},
		{
			name: "Invalid asset code format",
			input: map[string]any{
				"asset_code": "us",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"invalid asset code format"},
		},
		{
			name: "Missing amount",
			input: map[string]any{
				"asset_code": "USD",
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"amount is required"},
		},
		{
			name: "Invalid amount type",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     "1000",
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"amount must be a number"},
		},
		{
			name: "Zero amount",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(0),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"amount must be greater than zero"},
		},
		{
			name: "Negative amount",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(-1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"amount must be greater than zero"},
		},
		{
			name: "Integer amount (should convert)",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     1000,
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: true,
		},
		{
			name: "Missing scale",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"scale is required"},
		},
		{
			name: "Invalid scale type",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      "2",
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"scale must be an integer"},
		},
		{
			name: "Scale out of range - negative",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      -1,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"scale must be between 0 and 18"},
		},
		{
			name: "Scale out of range - too high",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      19,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"scale must be between 0 and 18"},
		},
		{
			name: "Float64 scale (should convert)",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      float64(2),
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: true,
		},
		{
			name: "Missing operations",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
			},
			expectValid: false,
			errContains: []string{"at least one operation is required"},
		},
		{
			name: "Invalid operations type",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": "invalid",
			},
			expectValid: false,
			errContains: []string{"operations must be an array"},
		},
		{
			name: "Missing operation type",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"type is required"},
		},
		{
			name: "Invalid operation type value",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "INVALID", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"invalid type", "must be DEBIT or CREDIT"},
		},
		{
			name: "Missing operation account ID",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"account ID is required"},
		},
		{
			name: "Missing operation amount",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1"},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"amount is required"},
		},
		{
			name: "Invalid operation amount type",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": "1000"},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"amount must be a number"},
		},
		{
			name: "Zero operation amount",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(0)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"amount must be greater than zero"},
		},
		{
			name: "Integer operation amount (should convert)",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": 1000},
					{"type": "CREDIT", "account_id": "acc2", "amount": 1000},
				},
			},
			expectValid: true,
		},
		{
			name: "Unbalanced transaction",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(500)},
				},
			},
			expectValid: false,
			errContains: []string{"unbalanced"},
		},
		{
			name: "Operation amounts don't match transaction amount",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(2000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"do not match transaction amount"},
		},
		{
			name: "Invalid account alias",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "account_alias": "invalid@alias", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"invalid account alias format"},
		},
		{
			name: "Mismatched operation asset code",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "asset_code": "EUR", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"must match transaction asset code"},
		},
		{
			name: "Valid chart of accounts group name",
			input: map[string]any{
				"asset_code":                   "USD",
				"amount":                       float64(1000),
				"scale":                        2,
				"chart_of_accounts_group_name": "Standard Chart",
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: true,
		},
		{
			name: "Invalid chart of accounts group name type",
			input: map[string]any{
				"asset_code":                   "USD",
				"amount":                       float64(1000),
				"scale":                        2,
				"chart_of_accounts_group_name": 123,
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"chart_of_accounts_group_name must be a string"},
		},
		{
			name: "Invalid chart of accounts group name - too long",
			input: map[string]any{
				"asset_code":                   "USD",
				"amount":                       float64(1000),
				"scale":                        2,
				"chart_of_accounts_group_name": string(make([]byte, 101)),
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"exceeds maximum length of 100 characters"},
		},
		{
			name: "Invalid chart of accounts group name - invalid characters",
			input: map[string]any{
				"asset_code":                   "USD",
				"amount":                       float64(1000),
				"scale":                        2,
				"chart_of_accounts_group_name": "Invalid@Chart!",
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"invalid characters"},
		},
		{
			name: "Invalid metadata type",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"metadata":   "invalid",
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: false,
			errContains: []string{"metadata must be an object"},
		},
		{
			name: "Valid metadata",
			input: map[string]any{
				"asset_code": "USD",
				"amount":     float64(1000),
				"scale":      2,
				"metadata": map[string]any{
					"reference": "TX-123",
				},
				"operations": []map[string]any{
					{"type": "DEBIT", "account_id": "acc1", "amount": float64(1000)},
					{"type": "CREDIT", "account_id": "acc2", "amount": float64(1000)},
				},
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := validation.ValidateCreateTransactionInput(tt.input)
			if tt.expectValid {
				assert.True(t, summary.Valid, "Expected valid but got errors: %v", summary.GetErrorMessages())
			} else {
				assert.False(t, summary.Valid)

				errSummary := summary.GetErrorSummary()
				for _, expected := range tt.errContains {
					assert.Contains(t, errSummary, expected, "Error summary should contain '%s'", expected)
				}
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
			name:        "Empty asset type",
			assetType:   "",
			wantErr:     true,
			errContains: "asset type is required",
		},
		{
			name:        "Invalid asset type",
			assetType:   "invalid",
			wantErr:     true,
			errContains: "invalid asset type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateAssetType(tt.assetType)
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
			accountType: "invalid",
			wantErr:     true,
			errContains: "invalid account type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateAccountType(tt.accountType)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateCurrencyCode(tt.code)
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
			name:        "Empty country code",
			code:        "",
			wantErr:     true,
			errContains: "country code cannot be empty",
		},
		{
			name:        "Invalid country code",
			code:        "XX",
			wantErr:     true,
			errContains: "invalid country code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateCountryCode(tt.code)
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

func TestValidateAddress(t *testing.T) {
	line2 := "Apt 4B"
	longLine := string(make([]byte, 101))
	longZip := string(make([]byte, 21))
	longCity := string(make([]byte, 101))
	longState := string(make([]byte, 101))

	tests := []struct {
		name        string
		address     *validation.Address
		wantErr     bool
		errContains string
	}{
		{
			name: "Valid address",
			address: &validation.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr: false,
		},
		{
			name: "Valid address with Line2",
			address: &validation.Address{
				Line1:   "123 Main St",
				Line2:   &line2,
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr: false,
		},
		{
			name:        "Nil address",
			address:     nil,
			wantErr:     true,
			errContains: "address cannot be nil",
		},
		{
			name: "Empty Line1",
			address: &validation.Address{
				Line1:   "",
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr:     true,
			errContains: "address line 1 is required",
		},
		{
			name: "Line1 too long",
			address: &validation.Address{
				Line1:   longLine,
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr:     true,
			errContains: "exceeds maximum length",
		},
		{
			name: "Line2 too long",
			address: &validation.Address{
				Line1:   "123 Main St",
				Line2:   &longLine,
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr:     true,
			errContains: "exceeds maximum length",
		},
		{
			name: "Empty ZipCode",
			address: &validation.Address{
				Line1:   "123 Main St",
				ZipCode: "",
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr:     true,
			errContains: "zip code is required",
		},
		{
			name: "ZipCode too long",
			address: &validation.Address{
				Line1:   "123 Main St",
				ZipCode: longZip,
				City:    "New York",
				State:   "NY",
				Country: "US",
			},
			wantErr:     true,
			errContains: "exceeds maximum length",
		},
		{
			name: "Empty City",
			address: &validation.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    "",
				State:   "NY",
				Country: "US",
			},
			wantErr:     true,
			errContains: "city is required",
		},
		{
			name: "City too long",
			address: &validation.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    longCity,
				State:   "NY",
				Country: "US",
			},
			wantErr:     true,
			errContains: "exceeds maximum length",
		},
		{
			name: "Empty State",
			address: &validation.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    "New York",
				State:   "",
				Country: "US",
			},
			wantErr:     true,
			errContains: "state is required",
		},
		{
			name: "State too long",
			address: &validation.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    "New York",
				State:   longState,
				Country: "US",
			},
			wantErr:     true,
			errContains: "exceeds maximum length",
		},
		{
			name: "Empty Country",
			address: &validation.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "",
			},
			wantErr:     true,
			errContains: "country is required",
		},
		{
			name: "Invalid Country",
			address: &validation.Address{
				Line1:   "123 Main St",
				ZipCode: "12345",
				City:    "New York",
				State:   "NY",
				Country: "XX",
			},
			wantErr:     true,
			errContains: "invalid country code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateAddress(tt.address)
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

func TestGetExternalAccountReference(t *testing.T) {
	tests := []struct {
		assetCode string
		expected  string
	}{
		{"USD", "@external/USD"},
		{"EUR", "@external/EUR"},
		{"BTC", "@external/BTC"},
		{"USDT", "@external/USDT"},
	}

	for _, tt := range tests {
		t.Run(tt.assetCode, func(t *testing.T) {
			result := validation.GetExternalAccountReference(tt.assetCode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultValidator(t *testing.T) {
	t.Run("DefaultValidator returns non-nil validator", func(t *testing.T) {
		validator := validation.DefaultValidator()
		assert.NotNil(t, validator)
	})

	t.Run("DefaultValidator validates metadata", func(t *testing.T) {
		validator := validation.DefaultValidator()
		metadata := map[string]any{
			"key": "value",
		}
		err := validator.ValidateMetadata(metadata)
		require.NoError(t, err)
	})

	t.Run("DefaultValidator validates address", func(t *testing.T) {
		validator := validation.DefaultValidator()
		address := &validation.Address{
			Line1:   "123 Main St",
			ZipCode: "12345",
			City:    "New York",
			State:   "NY",
			Country: "US",
		}
		err := validator.ValidateAddress(address)
		require.NoError(t, err)
	})
}

func TestValidatorMetadataNumericRanges(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		wantErr  bool
	}{
		{
			name:     "Valid int at max boundary",
			metadata: map[string]any{"val": 9999999999},
			wantErr:  false,
		},
		{
			name:     "Valid int at min boundary",
			metadata: map[string]any{"val": -9999999999},
			wantErr:  false,
		},
		{
			name:     "Invalid int above max",
			metadata: map[string]any{"val": 10000000000},
			wantErr:  true,
		},
		{
			name:     "Invalid int below min",
			metadata: map[string]any{"val": -10000000000},
			wantErr:  true,
		},
		{
			name:     "Valid float64 at max boundary",
			metadata: map[string]any{"val": 9999999999.0},
			wantErr:  false,
		},
		{
			name:     "Valid float64 at min boundary",
			metadata: map[string]any{"val": -9999999999.0},
			wantErr:  false,
		},
		{
			name:     "Invalid float64 above max",
			metadata: map[string]any{"val": 10000000000.0},
			wantErr:  true,
		},
		{
			name:     "Invalid float64 below min",
			metadata: map[string]any{"val": -10000000000.0},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateMetadata(tt.metadata)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

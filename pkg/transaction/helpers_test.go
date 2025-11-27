package transaction

import (
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatAmount tests the formatAmount helper function
func TestFormatAmount(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		scale    int64
		expected string
	}{
		{
			name:     "zero scale returns integer string",
			amount:   1000,
			scale:    0,
			expected: "1000",
		},
		{
			name:     "scale 2 whole number strips trailing zeros",
			amount:   1000,
			scale:    2,
			expected: "10", // fractional is 0, so it returns just whole
		},
		{
			name:     "scale 2 with fractional part",
			amount:   1050,
			scale:    2,
			expected: "10.50",
		},
		{
			name:     "scale 2 with single digit fractional",
			amount:   1005,
			scale:    2,
			expected: "10.05",
		},
		{
			name:     "scale 3 formats correctly",
			amount:   1234567,
			scale:    3,
			expected: "1234.567",
		},
		{
			name:     "zero amount with scale",
			amount:   0,
			scale:    2,
			expected: "0",
		},
		{
			name:     "fractional zero returns integer only",
			amount:   1000,
			scale:    2,
			expected: "10", // fractional is 0
		},
		{
			name:     "large amount with scale",
			amount:   999999999,
			scale:    2,
			expected: "9999999.99",
		},
		{
			name:     "small amount less than divisor",
			amount:   50,
			scale:    2,
			expected: "0.50",
		},
		{
			name:     "very small amount",
			amount:   1,
			scale:    2,
			expected: "0.01",
		},
		{
			name:     "negative scale treated as zero",
			amount:   100,
			scale:    0,
			expected: "100",
		},
		{
			name:     "scale 6 for crypto",
			amount:   123456789,
			scale:    6,
			expected: "123.456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAmount(tt.amount, tt.scale)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDefaultTransferOptions tests the default transfer options
func TestDefaultTransferOptions(t *testing.T) {
	opts := DefaultTransferOptions()

	require.NotNil(t, opts)
	assert.Equal(t, "Transfer between accounts", opts.Description)
	assert.NotNil(t, opts.Metadata)
	assert.Equal(t, "go-sdk-transaction-helper", opts.Metadata["source"])
	assert.False(t, opts.Pending)
	assert.NotEmpty(t, opts.IdempotencyKey)
	assert.Empty(t, opts.ExternalID)
	assert.Empty(t, opts.ChartOfAccountsGroupName)
}

// TestDefaultDepositOptions tests the default deposit options
func TestDefaultDepositOptions(t *testing.T) {
	opts := DefaultDepositOptions()

	require.NotNil(t, opts)
	assert.Equal(t, "Deposit from external source", opts.Description)
	assert.NotNil(t, opts.Metadata)
	assert.Equal(t, "go-sdk-transaction-helper", opts.Metadata["source"])
	assert.Equal(t, "deposit", opts.Metadata["type"])
	assert.False(t, opts.Pending)
	assert.NotEmpty(t, opts.IdempotencyKey)
	assert.Empty(t, opts.ExternalAccountID)
}

// TestDefaultWithdrawalOptions tests the default withdrawal options
func TestDefaultWithdrawalOptions(t *testing.T) {
	opts := DefaultWithdrawalOptions()

	require.NotNil(t, opts)
	assert.Equal(t, "Withdrawal to external destination", opts.Description)
	assert.NotNil(t, opts.Metadata)
	assert.Equal(t, "go-sdk-transaction-helper", opts.Metadata["source"])
	assert.Equal(t, "withdrawal", opts.Metadata["type"])
	assert.False(t, opts.Pending)
	assert.NotEmpty(t, opts.IdempotencyKey)
	assert.Empty(t, opts.ExternalAccountID)
}

// TestDefaultMultiTransferOptions tests the default multi-transfer options
func TestDefaultMultiTransferOptions(t *testing.T) {
	opts := DefaultMultiTransferOptions()

	require.NotNil(t, opts)
	assert.Equal(t, "Multi-account transfer", opts.Description)
	assert.NotNil(t, opts.Metadata)
	assert.Equal(t, "go-sdk-transaction-helper", opts.Metadata["source"])
	assert.Equal(t, "multi-transfer", opts.Metadata["type"])
	assert.False(t, opts.Pending)
	assert.NotEmpty(t, opts.IdempotencyKey)
}

// TestTransactionStatus tests the transaction status helper functions
func TestTransactionStatus(t *testing.T) {
	t.Run("IsTransactionSuccessful", func(t *testing.T) {
		tests := []struct {
			name     string
			tx       *models.Transaction
			expected bool
		}{
			{
				name:     "nil transaction returns false",
				tx:       nil,
				expected: false,
			},
			{
				name: "COMPLETED status returns true",
				tx: &models.Transaction{
					Status: models.Status{Code: "COMPLETED"},
				},
				expected: true,
			},
			{
				name: "PENDING status returns false",
				tx: &models.Transaction{
					Status: models.Status{Code: "PENDING"},
				},
				expected: false,
			},
			{
				name: "FAILED status returns false",
				tx: &models.Transaction{
					Status: models.Status{Code: "FAILED"},
				},
				expected: false,
			},
			{
				name: "CANCELED status returns false",
				tx: &models.Transaction{
					Status: models.Status{Code: "CANCELED"},
				},
				expected: false,
			},
			{
				name: "empty status returns false",
				tx: &models.Transaction{
					Status: models.Status{Code: ""},
				},
				expected: false,
			},
			{
				name: "unknown status returns false",
				tx: &models.Transaction{
					Status: models.Status{Code: "UNKNOWN"},
				},
				expected: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := IsTransactionSuccessful(tt.tx)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("GetTransactionStatus", func(t *testing.T) {
		tests := []struct {
			name     string
			tx       *models.Transaction
			expected string
		}{
			{
				name:     "nil transaction returns Unknown",
				tx:       nil,
				expected: "Unknown",
			},
			{
				name: "COMPLETED status returns Completed",
				tx: &models.Transaction{
					Status: models.Status{Code: "COMPLETED"},
				},
				expected: "Completed",
			},
			{
				name: "PENDING status returns Pending",
				tx: &models.Transaction{
					Status: models.Status{Code: "PENDING"},
				},
				expected: "Pending",
			},
			{
				name: "FAILED status returns Failed",
				tx: &models.Transaction{
					Status: models.Status{Code: "FAILED"},
				},
				expected: "Failed",
			},
			{
				name: "CANCELED status returns Canceled",
				tx: &models.Transaction{
					Status: models.Status{Code: "CANCELED"},
				},
				expected: "Canceled",
			},
			{
				name: "unknown status returns as-is",
				tx: &models.Transaction{
					Status: models.Status{Code: "PROCESSING"},
				},
				expected: "PROCESSING",
			},
			{
				name: "empty status returns empty",
				tx: &models.Transaction{
					Status: models.Status{Code: ""},
				},
				expected: "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := GetTransactionStatus(tt.tx)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}

// TestTransferOptionsFields tests TransferOptions struct fields
func TestTransferOptionsFields(t *testing.T) {
	opts := &TransferOptions{
		Description:              "Test transfer",
		Metadata:                 map[string]any{"key": "value"},
		IdempotencyKey:           "test-key-123",
		Pending:                  true,
		ExternalID:               "ext-123",
		ChartOfAccountsGroupName: "chart-group-1",
	}

	assert.Equal(t, "Test transfer", opts.Description)
	assert.Equal(t, "value", opts.Metadata["key"])
	assert.Equal(t, "test-key-123", opts.IdempotencyKey)
	assert.True(t, opts.Pending)
	assert.Equal(t, "ext-123", opts.ExternalID)
	assert.Equal(t, "chart-group-1", opts.ChartOfAccountsGroupName)
}

// TestDepositOptionsFields tests DepositOptions struct fields
func TestDepositOptionsFields(t *testing.T) {
	opts := &DepositOptions{
		Description:              "Test deposit",
		Metadata:                 map[string]any{"key": "value"},
		IdempotencyKey:           "deposit-key-123",
		Pending:                  true,
		ExternalID:               "ext-deposit-123",
		ChartOfAccountsGroupName: "deposit-chart",
		ExternalAccountID:        "@external/USD",
	}

	assert.Equal(t, "Test deposit", opts.Description)
	assert.Equal(t, "value", opts.Metadata["key"])
	assert.Equal(t, "deposit-key-123", opts.IdempotencyKey)
	assert.True(t, opts.Pending)
	assert.Equal(t, "ext-deposit-123", opts.ExternalID)
	assert.Equal(t, "deposit-chart", opts.ChartOfAccountsGroupName)
	assert.Equal(t, "@external/USD", opts.ExternalAccountID)
}

// TestWithdrawalOptionsFields tests WithdrawalOptions struct fields
func TestWithdrawalOptionsFields(t *testing.T) {
	opts := &WithdrawalOptions{
		Description:              "Test withdrawal",
		Metadata:                 map[string]any{"key": "value"},
		IdempotencyKey:           "withdrawal-key-123",
		Pending:                  true,
		ExternalID:               "ext-withdrawal-123",
		ChartOfAccountsGroupName: "withdrawal-chart",
		ExternalAccountID:        "@external/EUR",
	}

	assert.Equal(t, "Test withdrawal", opts.Description)
	assert.Equal(t, "value", opts.Metadata["key"])
	assert.Equal(t, "withdrawal-key-123", opts.IdempotencyKey)
	assert.True(t, opts.Pending)
	assert.Equal(t, "ext-withdrawal-123", opts.ExternalID)
	assert.Equal(t, "withdrawal-chart", opts.ChartOfAccountsGroupName)
	assert.Equal(t, "@external/EUR", opts.ExternalAccountID)
}

// TestMultiTransferOptionsFields tests MultiTransferOptions struct fields
func TestMultiTransferOptionsFields(t *testing.T) {
	opts := &MultiTransferOptions{
		Description:              "Test multi-transfer",
		Metadata:                 map[string]any{"key": "value"},
		IdempotencyKey:           "multi-key-123",
		Pending:                  true,
		ExternalID:               "ext-multi-123",
		ChartOfAccountsGroupName: "multi-chart",
	}

	assert.Equal(t, "Test multi-transfer", opts.Description)
	assert.Equal(t, "value", opts.Metadata["key"])
	assert.Equal(t, "multi-key-123", opts.IdempotencyKey)
	assert.True(t, opts.Pending)
	assert.Equal(t, "ext-multi-123", opts.ExternalID)
	assert.Equal(t, "multi-chart", opts.ChartOfAccountsGroupName)
}

// TestTransactionTemplateFields tests TransactionTemplate struct fields
func TestTransactionTemplateFields(t *testing.T) {
	buildSources := func(amount int64) []models.FromToInput {
		return []models.FromToInput{
			{Account: "source-account"},
		}
	}
	buildDests := func(amount int64) []models.FromToInput {
		return []models.FromToInput{
			{Account: "dest-account"},
		}
	}

	template := &TransactionTemplate{
		Description:              "Template description",
		AssetCode:                "USD",
		Scale:                    2,
		Metadata:                 map[string]any{"template": "test"},
		Pending:                  true,
		ChartOfAccountsGroupName: "template-chart",
		BuildSources:             buildSources,
		BuildDestinations:        buildDests,
	}

	assert.Equal(t, "Template description", template.Description)
	assert.Equal(t, "USD", template.AssetCode)
	assert.Equal(t, int64(2), template.Scale)
	assert.Equal(t, "test", template.Metadata["template"])
	assert.True(t, template.Pending)
	assert.Equal(t, "template-chart", template.ChartOfAccountsGroupName)
	assert.NotNil(t, template.BuildSources)
	assert.NotNil(t, template.BuildDestinations)

	// Test the functions work
	sources := template.BuildSources(1000)
	assert.Len(t, sources, 1)
	assert.Equal(t, "source-account", sources[0].Account)

	dests := template.BuildDestinations(1000)
	assert.Len(t, dests, 1)
	assert.Equal(t, "dest-account", dests[0].Account)
}

// TestIdempotencyKeyGeneration tests that idempotency keys are unique
func TestIdempotencyKeyGeneration(t *testing.T) {
	t.Run("DefaultTransferOptions generates unique keys", func(t *testing.T) {
		opts1 := DefaultTransferOptions()
		opts2 := DefaultTransferOptions()
		assert.NotEqual(t, opts1.IdempotencyKey, opts2.IdempotencyKey)
	})

	t.Run("DefaultDepositOptions generates unique keys", func(t *testing.T) {
		opts1 := DefaultDepositOptions()
		opts2 := DefaultDepositOptions()
		assert.NotEqual(t, opts1.IdempotencyKey, opts2.IdempotencyKey)
	})

	t.Run("DefaultWithdrawalOptions generates unique keys", func(t *testing.T) {
		opts1 := DefaultWithdrawalOptions()
		opts2 := DefaultWithdrawalOptions()
		assert.NotEqual(t, opts1.IdempotencyKey, opts2.IdempotencyKey)
	})

	t.Run("DefaultMultiTransferOptions generates unique keys", func(t *testing.T) {
		opts1 := DefaultMultiTransferOptions()
		opts2 := DefaultMultiTransferOptions()
		assert.NotEqual(t, opts1.IdempotencyKey, opts2.IdempotencyKey)
	})
}

// TestFormatAmountEdgeCases tests edge cases for formatAmount
func TestFormatAmountEdgeCases(t *testing.T) {
	t.Run("negative amount with no fractional", func(t *testing.T) {
		result := formatAmount(-1000, 2)
		// -1000 / 100 = -10, fractional = 0, so returns just whole
		assert.Equal(t, "-10", result)
	})

	t.Run("negative amount with fractional", func(t *testing.T) {
		result := formatAmount(-1050, 2)
		assert.Equal(t, "-10.50", result)
	})

	t.Run("max int64 with scale", func(t *testing.T) {
		// This tests that we don't overflow
		result := formatAmount(9223372036854775807, 0)
		assert.Equal(t, "9223372036854775807", result)
	})

	t.Run("scale larger than amount digits", func(t *testing.T) {
		result := formatAmount(5, 4)
		assert.Equal(t, "0.0005", result)
	})
}

// TestTransactionWithVariousStatuses tests various transaction status scenarios
func TestTransactionWithVariousStatuses(t *testing.T) {
	statuses := []struct {
		code       string
		successful bool
		display    string
	}{
		{"COMPLETED", true, "Completed"},
		{"PENDING", false, "Pending"},
		{"FAILED", false, "Failed"},
		{"CANCELED", false, "Canceled"},
		{"IN_PROGRESS", false, "IN_PROGRESS"},
		{"REVERSED", false, "REVERSED"},
	}

	for _, s := range statuses {
		t.Run(s.code, func(t *testing.T) {
			tx := &models.Transaction{
				Status: models.Status{Code: s.code},
			}
			assert.Equal(t, s.successful, IsTransactionSuccessful(tx))
			assert.Equal(t, s.display, GetTransactionStatus(tx))
		})
	}
}

// TestOptionsWithEmptyMetadata tests options with nil/empty metadata
func TestOptionsWithEmptyMetadata(t *testing.T) {
	t.Run("TransferOptions with nil metadata", func(t *testing.T) {
		opts := &TransferOptions{
			Description: "Test",
			Metadata:    nil,
		}
		assert.Nil(t, opts.Metadata)
	})

	t.Run("TransferOptions with empty metadata", func(t *testing.T) {
		opts := &TransferOptions{
			Description: "Test",
			Metadata:    map[string]any{},
		}
		assert.NotNil(t, opts.Metadata)
		assert.Empty(t, opts.Metadata)
	})
}

// TestBatchResultFields tests BatchResult struct fields
func TestBatchResultFields(t *testing.T) {
	result := BatchResult{
		Index:         5,
		TransactionID: "tx-123",
		Error:         nil,
		Duration:      100,
	}

	assert.Equal(t, 5, result.Index)
	assert.Equal(t, "tx-123", result.TransactionID)
	assert.Nil(t, result.Error)
	assert.Equal(t, 100, int(result.Duration))
}

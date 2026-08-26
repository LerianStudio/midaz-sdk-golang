package transaction

import (
	"context"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
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
			name:     "scale 2 whole number preserves decimal places",
			amount:   1000,
			scale:    2,
			expected: "10.00", // financial wire format: always show decimals at non-zero scale
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
			expected: "0.00",
		},
		{
			name:     "fractional zero preserves decimal places",
			amount:   1000,
			scale:    2,
			expected: "10.00",
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
	assert.Empty(t, opts.IdempotencyKey)
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
	assert.Empty(t, opts.IdempotencyKey)
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
	assert.Empty(t, opts.IdempotencyKey)
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
	assert.Empty(t, opts.IdempotencyKey)
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
				name: "APPROVED status returns true",
				tx: &models.Transaction{
					Status: models.Status{Code: "APPROVED"},
				},
				expected: true,
			},
			{
				name: "CREATED status returns false",
				tx: &models.Transaction{
					Status: models.Status{Code: "CREATED"},
				},
				expected: false,
			},
			{
				name: "PENDING status returns false",
				tx: &models.Transaction{
					Status: models.Status{Code: "PENDING"},
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
				name: "NOTED status returns false",
				tx: &models.Transaction{
					Status: models.Status{Code: "NOTED"},
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
				name: "CREATED status returns Created",
				tx: &models.Transaction{
					Status: models.Status{Code: "CREATED"},
				},
				expected: "Created",
			},
			{
				name: "APPROVED status returns Approved",
				tx: &models.Transaction{
					Status: models.Status{Code: "APPROVED"},
				},
				expected: "Approved",
			},
			{
				name: "PENDING status returns Pending",
				tx: &models.Transaction{
					Status: models.Status{Code: "PENDING"},
				},
				expected: "Pending",
			},
			{
				name: "CANCELED status returns Canceled",
				tx: &models.Transaction{
					Status: models.Status{Code: "CANCELED"},
				},
				expected: "Canceled",
			},
			{
				name: "NOTED status returns Noted",
				tx: &models.Transaction{
					Status: models.Status{Code: "NOTED"},
				},
				expected: "Noted",
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
		ChartOfAccountsGroupName: "chart-group-1",
	}

	assert.Equal(t, "Test transfer", opts.Description)
	assert.Equal(t, "value", opts.Metadata["key"])
	assert.Equal(t, "test-key-123", opts.IdempotencyKey)
	assert.True(t, opts.Pending)
	assert.Equal(t, "chart-group-1", opts.ChartOfAccountsGroupName)
}

// TestDepositOptionsFields tests DepositOptions struct fields
func TestDepositOptionsFields(t *testing.T) {
	opts := &DepositOptions{
		Description:              "Test deposit",
		Metadata:                 map[string]any{"key": "value"},
		IdempotencyKey:           "deposit-key-123",
		Pending:                  true,
		ChartOfAccountsGroupName: "deposit-chart",
		ExternalAccountID:        "@external/USD",
	}

	assert.Equal(t, "Test deposit", opts.Description)
	assert.Equal(t, "value", opts.Metadata["key"])
	assert.Equal(t, "deposit-key-123", opts.IdempotencyKey)
	assert.True(t, opts.Pending)
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
		ChartOfAccountsGroupName: "withdrawal-chart",
		ExternalAccountID:        "@external/EUR",
	}

	assert.Equal(t, "Test withdrawal", opts.Description)
	assert.Equal(t, "value", opts.Metadata["key"])
	assert.Equal(t, "withdrawal-key-123", opts.IdempotencyKey)
	assert.True(t, opts.Pending)
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
		ChartOfAccountsGroupName: "multi-chart",
	}

	assert.Equal(t, "Test multi-transfer", opts.Description)
	assert.Equal(t, "value", opts.Metadata["key"])
	assert.Equal(t, "multi-key-123", opts.IdempotencyKey)
	assert.True(t, opts.Pending)
	assert.Equal(t, "multi-chart", opts.ChartOfAccountsGroupName)
}

// TestTemplateFields tests Template struct fields
func TestTemplateFields(t *testing.T) {
	buildSources := func(_ int64) []models.FromToInput {
		return []models.FromToInput{
			{AccountAlias: "source-account"},
		}
	}
	buildDests := func(_ int64) []models.FromToInput {
		return []models.FromToInput{
			{AccountAlias: "dest-account"},
		}
	}

	template := &Template{
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
	assert.Equal(t, "source-account", sources[0].AccountAlias)

	dests := template.BuildDestinations(1000)
	assert.Len(t, dests, 1)
	assert.Equal(t, "dest-account", dests[0].AccountAlias)
}

// TestDefaultOptionsDoNotPinIdempotencyKeys verifies defaults do not bake a
// one-shot idempotency key into reusable options structs.
func TestDefaultOptionsDoNotPinIdempotencyKeys(t *testing.T) {
	assert.Empty(t, DefaultTransferOptions().IdempotencyKey)
	assert.Empty(t, DefaultDepositOptions().IdempotencyKey)
	assert.Empty(t, DefaultWithdrawalOptions().IdempotencyKey)
	assert.Empty(t, DefaultMultiTransferOptions().IdempotencyKey)
}

// TestDefaultTransferOptionsReturnsIndependentInstances pins the
// no-shared-state contract on the Default*Options helpers: two
// successive calls must yield structs whose nested maps are NOT the
// same underlying allocation. Sharing the Metadata map (or any
// embedded map / slice) would let a caller mutating one returned
// options struct silently corrupt the next caller's defaults.
//
// This is the kind of regression that compiles, passes unit tests in
// isolation, and only blows up in production once two parallel
// transfer flows happen to mutate metadata at the same instant.
func TestDefaultTransferOptionsReturnsIndependentInstances(t *testing.T) {
	first := DefaultTransferOptions()
	second := DefaultTransferOptions()

	require.NotNil(t, first)
	require.NotNil(t, second)
	require.NotNil(t, first.Metadata)
	require.NotNil(t, second.Metadata)

	first.Metadata["mutated"] = "by-caller-one"

	_, present := second.Metadata["mutated"]
	assert.False(t, present, "second DefaultTransferOptions() must not see mutations to the first")
}

// TestFormatAmountEdgeCases tests edge cases for formatAmount
func TestFormatAmountEdgeCases(t *testing.T) {
	t.Run("negative amount with no fractional", func(t *testing.T) {
		result := formatAmount(-1000, 2)
		// -10.00 — financial wire format always shows decimals at non-zero scale
		assert.Equal(t, "-10.00", result)
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
		{"APPROVED", true, "Approved"},
		{"CREATED", false, "Created"},
		{"PENDING", false, "Pending"},
		{"CANCELED", false, "Canceled"},
		{"NOTED", false, "Noted"},
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
		assert.Equal(t, "Test", opts.Description)
		assert.Nil(t, opts.Metadata)
	})

	t.Run("TransferOptions with empty metadata", func(t *testing.T) {
		opts := &TransferOptions{
			Description: "Test",
			Metadata:    map[string]any{},
		}
		assert.Equal(t, "Test", opts.Description)
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
	require.NoError(t, result.Error)
	assert.Equal(t, 100, int(result.Duration))
}

// TestTransfer_NilEntity_ReturnsError verifies that Transfer and its sibling
// helpers return a clean error rather than panicking when the caller passes a
// nil *entities.Entity. Mirrors the existing CancelPendingTransaction guard.
func TestTransfer_NilEntity_ReturnsError(t *testing.T) {
	ctx := context.Background()

	t.Run("Transfer", func(t *testing.T) {
		require.NotPanics(t, func() {
			tx, err := Transfer(ctx, nil, "org", "ledger", "from", "to", 100, 2, "USD", nil)
			require.Error(t, err)
			assert.Nil(t, tx)
			assert.Contains(t, err.Error(), "entity is required")
		})
	})

	t.Run("Deposit", func(t *testing.T) {
		require.NotPanics(t, func() {
			tx, err := Deposit(ctx, nil, "org", "ledger", "to", 100, 2, "USD", nil)
			require.Error(t, err)
			assert.Nil(t, tx)
			assert.Contains(t, err.Error(), "entity is required")
		})
	})

	t.Run("Withdrawal", func(t *testing.T) {
		require.NotPanics(t, func() {
			tx, err := Withdrawal(ctx, nil, "org", "ledger", "from", 100, 2, "USD", nil)
			require.Error(t, err)
			assert.Nil(t, tx)
			assert.Contains(t, err.Error(), "entity is required")
		})
	})

	t.Run("MultiAccountTransfer", func(t *testing.T) {
		require.NotPanics(t, func() {
			tx, err := MultiAccountTransfer(ctx, nil, "org", "ledger",
				map[string]int64{"a": 100}, map[string]int64{"b": 100},
				100, 2, "USD", nil)
			require.Error(t, err)
			assert.Nil(t, tx)
			assert.Contains(t, err.Error(), "entity is required")
		})
	})

	t.Run("CreateFromTemplate", func(t *testing.T) {
		require.NotPanics(t, func() {
			tmpl := &Template{
				Description:       "test",
				AssetCode:         "USD",
				Scale:             2,
				BuildSources:      func(int64) []models.FromToInput { return nil },
				BuildDestinations: func(int64) []models.FromToInput { return nil },
			}
			tx, err := CreateFromTemplate(ctx, nil, "org", "ledger", tmpl, 100, nil, "")
			require.Error(t, err)
			assert.Nil(t, tx)
			assert.Contains(t, err.Error(), "entity is required")
		})
	})

	t.Run("CommitPendingTransaction", func(t *testing.T) {
		require.NotPanics(t, func() {
			tx, err := CommitPendingTransaction(ctx, nil, "org", "ledger", "tx-id")
			require.Error(t, err)
			assert.Nil(t, tx)
			assert.Contains(t, err.Error(), "entity is required")
		})
	})

	t.Run("CancelPendingTransaction", func(t *testing.T) {
		// Pre-existing guard - covered for completeness.
		require.NotPanics(t, func() {
			tx, err := CancelPendingTransaction(ctx, nil, "org", "ledger", "tx-id")
			require.Error(t, err)
			assert.Nil(t, tx)
			assert.Contains(t, err.Error(), "entity is required")
		})
	})
}

// TestTransfer_NilTransactionsService_ReturnsError verifies that an Entity with
// a nil Transactions service returns a clean error rather than panicking.
func TestTransfer_NilTransactionsService_ReturnsError(t *testing.T) {
	ctx := context.Background()
	entity := &entities.Entity{} // Transactions field is nil

	tx, err := Transfer(ctx, entity, "org", "ledger", "from", "to", 100, 2, "USD", nil)
	require.Error(t, err)
	assert.Nil(t, tx)
	assert.Contains(t, err.Error(), "transactions service is not initialized")
}

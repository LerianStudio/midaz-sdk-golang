package format_test

import (
	"errors"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/format"
	"github.com/stretchr/testify/assert"
)

func TestFormatAmount(t *testing.T) {
	testCases := []struct {
		name     string
		amount   int64
		scale    int
		expected string
	}{
		{
			name:     "Positive integer",
			amount:   100,
			scale:    0,
			expected: "100",
		},
		{
			name:     "Positive decimal",
			amount:   12345,
			scale:    2,
			expected: "123.45",
		},
		{
			name:     "Negative decimal",
			amount:   -5075,
			scale:    2,
			expected: "-50.75",
		},
		{
			name:     "Zero amount",
			amount:   0,
			scale:    2,
			expected: "0.00",
		},
		{
			name:     "Small decimal",
			amount:   5,
			scale:    2,
			expected: "0.05",
		},
		{
			name:     "Very small decimal",
			amount:   1,
			scale:    5,
			expected: "0.00001",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := format.FormatAmount(tc.amount, tc.scale)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Helper for string pointers
func str(s string) *string {
	return &s
}

func TestFormatTransaction(t *testing.T) {
	// Test nil transaction
	summary := format.FormatTransaction(nil)
	assert.Equal(t, "Invalid transaction: nil", summary)

	// Test minimal transaction
	tx := &models.Transaction{
		ID:        "tx-123",
		Amount:    "100.00",
		AssetCode: "USD",
		Status:    models.Status{Code: "COMPLETED"},
	}
	summary = format.FormatTransaction(tx)
	assert.Equal(t, "Transaction: 100.00 USD (Completed)", summary)

	// Test deposit transaction
	depositTx := &models.Transaction{
		ID:        "tx-deposit",
		Amount:    "250.00",
		AssetCode: "USD",
		Status:    models.Status{Code: "COMPLETED"},
		Operations: []models.Operation{
			{
				Type:         "CREDIT",
				AccountID:    "acc-ext",
				AccountAlias: str("@external/USD"),
			},
			{
				Type:         "DEBIT",
				AccountID:    "acc-target",
				AccountAlias: str("customer-account"),
			},
		},
	}
	summary = format.FormatTransaction(depositTx)
	assert.Equal(t, "Deposit: 250.00 USD from customer-account (Completed)", summary)

	// Test withdrawal transaction
	withdrawalTx := &models.Transaction{
		ID:        "tx-withdrawal",
		Amount:    "50.00",
		AssetCode: "USD",
		Status:    models.Status{Code: "PENDING"},
		Operations: []models.Operation{
			{
				Type:         "DEBIT",
				AccountID:    "acc-source",
				AccountAlias: str("savings-account"),
			},
			{
				Type:         "CREDIT",
				AccountID:    "acc-ext",
				AccountAlias: str("@external/USD"),
			},
		},
	}
	summary = format.FormatTransaction(withdrawalTx)
	assert.Equal(t, "Withdrawal: 50.00 USD from savings-account (Pending)", summary)

	// Test transfer transaction
	transferTx := &models.Transaction{
		ID:        "tx-transfer",
		Amount:    "15.00",
		AssetCode: "USD",
		Status:    models.Status{Code: "COMPLETED"},
		Operations: []models.Operation{
			{
				Type:         "DEBIT",
				AccountID:    "acc-source",
				AccountAlias: str("checking"),
			},
			{
				Type:         "CREDIT",
				AccountID:    "acc-target",
				AccountAlias: str("savings"),
			},
		},
	}
	summary = format.FormatTransaction(transferTx)
	assert.Equal(t, "Transfer: 15.00 USD from checking to savings (Completed)", summary)

	// Test transaction with multiple sources/destinations
	multiTx := &models.Transaction{
		ID:        "tx-multi",
		Amount:    "20.00",
		AssetCode: "USD",
		Status:    models.Status{Code: "COMPLETED"},
		Operations: []models.Operation{
			{
				Type:         "DEBIT",
				AccountID:    "acc-source1",
				AccountAlias: str("checking1"),
			},
			{
				Type:         "DEBIT",
				AccountID:    "acc-source2",
				AccountAlias: str("checking2"),
			},
			{
				Type:         "CREDIT",
				AccountID:    "acc-target1",
				AccountAlias: str("savings1"),
			},
			{
				Type:         "CREDIT",
				AccountID:    "acc-target2",
				AccountAlias: str("savings2"),
			},
		},
	}
	summary = format.FormatTransaction(multiTx)
	assert.Equal(t, "Transfer: 20.00 USD from multiple accounts (2) to multiple accounts (2) (Completed)", summary)
}

func TestFormatDate(t *testing.T) {
	now := time.Date(2023, 5, 15, 10, 30, 0, 0, time.UTC)

	formatted := format.FormatDate(now)
	assert.Equal(t, "2023-05-15", formatted)

	zeroTime := time.Time{}
	formatted = format.FormatDate(zeroTime)
	assert.Equal(t, "N/A", formatted)
}

func TestFormatDateTimeWithOptions(t *testing.T) {
	now := time.Date(2023, 5, 15, 10, 30, 45, 0, time.UTC)

	// Test with default options
	formatted, err := format.FormatDateTimeWithOptions(now)
	assert.NoError(t, err)
	assert.Equal(t, "2023-05-15 10:30:45", formatted)

	// Test with custom format
	formatted, err = format.FormatDateTimeWithOptions(now, format.WithFormat("Monday, Jan 2 2006"))
	assert.NoError(t, err)
	assert.Equal(t, "Monday, May 15 2023", formatted)

	// Test with default value
	zeroTime := time.Time{}
	formatted, err = format.FormatDateTimeWithOptions(zeroTime, format.WithDefaultValue("NOT AVAILABLE"))
	assert.NoError(t, err)
	assert.Equal(t, "NOT AVAILABLE", formatted)

	// Test with no default on zero
	formatted, err = format.FormatDateTimeWithOptions(zeroTime, format.WithDefaultOnZero(false))
	assert.NoError(t, err)
	assert.Equal(t, "0001-01-01 00:00:00", formatted)

	// Test with UTC conversion
	nonUTCLocation, err := time.LoadLocation("America/New_York")
	assert.NoError(t, err)
	nonUTCTime := time.Date(2023, 5, 15, 10, 30, 45, 0, nonUTCLocation)
	formatted, err = format.FormatDateTimeWithOptions(nonUTCTime, format.WithUTC(true))
	assert.NoError(t, err)

	// The UTC time should be different from the non-UTC time
	nonUTCFormatted := nonUTCTime.Format("2006-01-02 15:04:05")
	utcFormatted := nonUTCTime.UTC().Format("2006-01-02 15:04:05")
	assert.Equal(t, utcFormatted, formatted)
	assert.NotEqual(t, nonUTCFormatted, formatted)

	// Test with error
	_, err = format.FormatDateTimeWithOptions(now, func(o *format.DateTimeOptions) error {
		return errors.New("test error")
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test error")
}

func TestFormatTime(t *testing.T) {
	now := time.Date(2023, 5, 15, 10, 30, 45, 0, time.UTC)

	formatted := format.FormatTime(now)
	assert.Equal(t, "10:30:45", formatted)

	zeroTime := time.Time{}
	formatted = format.FormatTime(zeroTime)
	assert.Equal(t, "N/A", formatted)
}

func TestFormatDateTime(t *testing.T) {
	now := time.Date(2023, 5, 15, 10, 30, 45, 0, time.UTC)

	formatted := format.FormatDateTime(now)
	assert.Equal(t, "2023-05-15 10:30:45", formatted)

	zeroTime := time.Time{}
	formatted = format.FormatDateTime(zeroTime)
	assert.Equal(t, "N/A", formatted)
}

func TestFormatDuration(t *testing.T) {
	testCases := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "Microseconds",
			duration: 500 * time.Microsecond,
			expected: "500μs",
		},
		{
			name:     "Milliseconds",
			duration: 500 * time.Millisecond,
			expected: "500ms",
		},
		{
			name:     "Seconds",
			duration: 5 * time.Second,
			expected: "5.00s",
		},
		{
			name:     "Minutes and seconds",
			duration: 5*time.Minute + 30*time.Second,
			expected: "5m 30s",
		},
		{
			name:     "Hours and minutes",
			duration: 2*time.Hour + 45*time.Minute,
			expected: "2h 45m",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := format.FormatDuration(tc.duration)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatDurationWithOptions(t *testing.T) {
	duration := 2*time.Hour + 45*time.Minute + 30*time.Second

	// Test with default options
	result, err := format.FormatDurationWithOptions(duration)
	assert.NoError(t, err)
	assert.Equal(t, "2h 45m", result)

	// Test with long units
	result, err = format.FormatDurationWithOptions(duration, format.WithShortUnits(false))
	assert.NoError(t, err)
	assert.Equal(t, "2 hours 45 minutes", result)

	// Test with max components = 3
	result, err = format.FormatDurationWithOptions(duration, format.WithMaxComponents(3))
	assert.NoError(t, err)
	assert.Equal(t, "2h 45m 30s", result)

	// Test with max components = 1
	result, err = format.FormatDurationWithOptions(duration, format.WithMaxComponents(1))
	assert.NoError(t, err)
	assert.Equal(t, "2h", result)

	// Test with different precision for seconds
	result, err = format.FormatDurationWithOptions(5*time.Second+500*time.Millisecond, format.WithPrecision(3))
	assert.NoError(t, err)
	assert.Equal(t, "5.500s", result)

	// Test with error
	_, err = format.FormatDurationWithOptions(duration, func(o *format.DurationOptions) error {
		return errors.New("test error")
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test error")
}

func TestFormatISODate(t *testing.T) {
	now := time.Date(2023, 5, 15, 10, 30, 0, 0, time.UTC)

	formatted := format.FormatISODate(now)
	assert.Equal(t, "2023-05-15", formatted)

	zeroTime := time.Time{}
	formatted = format.FormatISODate(zeroTime)
	assert.Equal(t, "N/A", formatted)
}

func TestFormatISODateTime(t *testing.T) {
	now := time.Date(2023, 5, 15, 10, 30, 45, 0, time.UTC)

	formatted := format.FormatISODateTime(now)
	assert.Equal(t, "2023-05-15T10:30:45Z", formatted)

	zeroTime := time.Time{}
	formatted = format.FormatISODateTime(zeroTime)
	assert.Equal(t, "N/A", formatted)
}

func TestFormatAmountWithOptions(t *testing.T) {
	// Test with default options
	result, err := format.FormatAmountWithOptions(12345, 2)
	assert.NoError(t, err)
	assert.Equal(t, "123.45", result)

	// Test with thousands separator
	result, err = format.FormatAmountWithOptions(1234567, 2,
		format.WithThousandsSeparator(","))
	assert.NoError(t, err)
	assert.Equal(t, "12,345.67", result)

	// Test with custom decimal separator
	result, err = format.FormatAmountWithOptions(12345, 2,
		format.WithDecimalSeparator(","))
	assert.NoError(t, err)
	assert.Equal(t, "123,45", result)

	// Test with both separators
	result, err = format.FormatAmountWithOptions(1234567, 2,
		format.WithThousandsSeparator("."),
		format.WithDecimalSeparator(","))
	assert.NoError(t, err)
	assert.Equal(t, "12.345,67", result)

	// Test without scale
	result, err = format.FormatAmountWithOptions(12345, 0)
	assert.NoError(t, err)
	assert.Equal(t, "12345", result)

	// Test with negative amount
	result, err = format.FormatAmountWithOptions(-12345, 2)
	assert.NoError(t, err)
	assert.Equal(t, "-123.45", result)

	// Test with error
	_, err = format.FormatAmountWithOptions(12345, 2,
		func(o *format.AmountOptions) error {
			return errors.New("test error")
		})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test error")

	// Test with invalid decimal separator
	_, err = format.FormatAmountWithOptions(12345, 2,
		format.WithDecimalSeparator(""))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decimal separator cannot be empty")
}

func TestFormatTransactionWithOptions(t *testing.T) {
	tx := &models.Transaction{
		ID:        "tx-123",
		Amount:    "100.00",
		AssetCode: "USD",
		Status:    models.Status{Code: "COMPLETED"},
		CreatedAt: time.Date(2023, 5, 15, 10, 30, 45, 0, time.UTC),
		Operations: []models.Operation{
			{
				Type:         "DEBIT",
				AccountID:    "acc-source",
				AccountAlias: str("checking"),
			},
			{
				Type:         "CREDIT",
				AccountID:    "acc-target",
				AccountAlias: str("savings"),
			},
		},
	}

	// Test with default options
	result, err := format.FormatTransactionWithOptions(tx)
	assert.NoError(t, err)
	assert.Equal(t, "Transfer: 100.00 USD from checking to savings (Completed)", result)

	// Test with include ID
	result, err = format.FormatTransactionWithOptions(tx, format.WithTransactionID(true))
	assert.NoError(t, err)
	assert.Equal(t, "tx-123 - Transfer: 100.00 USD from checking to savings (Completed)", result)

	// Test with include timestamp
	result, err = format.FormatTransactionWithOptions(tx, format.WithTransactionTimestamp(true))
	assert.NoError(t, err)
	assert.Equal(t, "2023-05-15 10:30:45 - Transfer: 100.00 USD from checking to savings (Completed)", result)

	// Test with custom status mapping
	result, err = format.FormatTransactionWithOptions(tx,
		format.WithCustomStatusMapping(map[string]string{
			"COMPLETED": "Success",
			"PENDING":   "In Progress",
		}))
	assert.NoError(t, err)
	assert.Equal(t, "Transfer: 100.00 USD from checking to savings (Success)", result)

	// Test with error
	_, err = format.FormatTransactionWithOptions(tx,
		func(o *format.TransactionOptions) error {
			return errors.New("test error")
		})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test error")

	// Test nil transaction
	result, err = format.FormatTransactionWithOptions(nil)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid transaction: nil", result)
}

package format_test

import (
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatISO(t *testing.T) {
	// Create a fixed time for testing
	testTime := time.Date(2025, 4, 2, 15, 4, 5, 123456789, time.UTC)
	zeroTime := time.Time{}

	testCases := []struct {
		name      string
		formatter *format.FormatISO
		time      time.Time
		expected  string
	}{
		{
			name:      "Default format with normal time",
			formatter: format.DefaultISOFormat(),
			time:      testTime,
			expected:  "2025-04-02T15:04:05Z",
		},
		{
			name:      "Default format with zero time",
			formatter: format.DefaultISOFormat(),
			time:      zeroTime,
			expected:  "N/A",
		},
		{
			name:      "Date only with normal time",
			formatter: format.DateOnly(),
			time:      testTime,
			expected:  "2025-04-02",
		},
		{
			name:      "Date only with zero time",
			formatter: format.DateOnly(),
			time:      zeroTime,
			expected:  "N/A",
		},
		{
			name:      "DateTime with millis with normal time",
			formatter: format.DateTimeWithMillis(),
			time:      testTime,
			expected:  "2025-04-02T15:04:05.123456789Z",
		},
		{
			name:      "Custom formatter (no NA on zero)",
			formatter: &format.FormatISO{IncludeTime: true, NAOnZero: false},
			time:      zeroTime,
			expected:  "0001-01-01T00:00:00Z",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.formatter.Format(tc.time)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNewFormatISO(t *testing.T) {
	// Test with default options
	formatter, err := format.NewFormatISO()
	require.NoError(t, err)
	assert.True(t, formatter.IncludeTime)
	assert.False(t, formatter.IncludeMilliseconds)
	assert.True(t, formatter.NAOnZero)

	// Test with custom options
	formatter, err = format.NewFormatISO(
		format.WithIncludeTime(false),
		format.WithNAOnZero(false),
	)
	require.NoError(t, err)
	assert.False(t, formatter.IncludeTime)
	assert.False(t, formatter.IncludeMilliseconds)
	assert.False(t, formatter.NAOnZero)

	// Test milliseconds automatically enables time
	formatter, err = format.NewFormatISO(
		format.WithIncludeTime(false),
		format.WithIncludeMilliseconds(true),
	)
	require.NoError(t, err)
	assert.True(t, formatter.IncludeTime, "Time should be automatically enabled when milliseconds are enabled")
	assert.True(t, formatter.IncludeMilliseconds)

	// Test explicit disable of time is overridden by milliseconds
	formatter, err = format.NewFormatISO(
		format.WithIncludeMilliseconds(true),
		format.WithIncludeTime(false),
	)
	require.NoError(t, err)
	assert.True(t, formatter.IncludeTime, "Time should remain enabled despite attempt to disable it")
	assert.True(t, formatter.IncludeMilliseconds)

	// Test correct ordering works
	formatter, err = format.NewFormatISO(
		format.WithIncludeTime(true),
		format.WithIncludeMilliseconds(true),
	)
	require.NoError(t, err)
	assert.True(t, formatter.IncludeTime)
	assert.True(t, formatter.IncludeMilliseconds)
}

func TestParseISO(t *testing.T) {
	expected := time.Date(2025, 4, 2, 15, 4, 5, 0, time.UTC)
	expectedDate := time.Date(2025, 4, 2, 0, 0, 0, 0, time.UTC)
	expectedWithMillis := time.Date(2025, 4, 2, 15, 4, 5, 123456789, time.UTC)

	testCases := []struct {
		name     string
		input    string
		expected time.Time
		hasError bool
	}{
		{
			name:     "RFC3339 format",
			input:    "2025-04-02T15:04:05Z",
			expected: expected,
			hasError: false,
		},
		{
			name:     "RFC3339Nano format",
			input:    "2025-04-02T15:04:05.123456789Z",
			expected: expectedWithMillis,
			hasError: false,
		},
		{
			name:     "Date only format",
			input:    "2025-04-02",
			expected: expectedDate,
			hasError: false,
		},
		{
			name:     "Invalid format",
			input:    "not-a-date",
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := format.ParseISO(tc.input)

			if tc.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

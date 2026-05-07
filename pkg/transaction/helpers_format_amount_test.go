package transaction

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFormatAmount_PrecisionBoundaries verifies that formatAmount uses integer
// math only and never loses precision through float64 conversion. Float64 has
// ~15-17 significant decimal digits; fixed-point amounts at scale 8 (BTC sat)
// or scale 18 (wei) silently round past the 2^53 mantissa boundary when
// converted via float64. For double-entry accounting this is a third-rail
// violation — every sat and every wei must round-trip exactly.
//
// Regression for: prior implementation used
//
//	fmt.Sprintf("%.*f", scale, float64(amount)/float64(divisor))
//
// which lost precision above 2^53.
func TestFormatAmount_PrecisionBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		scale    int64
		expected string
	}{
		// --- Float64 mantissa boundary regression (2^53 + 1 = 9007199254740993) ---
		{
			name:     "regression: 2^53+1 at scale 2 must not round via float64",
			amount:   9007199254740993, // 2^53 + 1 — first int64 not exactly representable as float64
			scale:    2,
			expected: "90071992547409.93",
		},
		{
			name:     "regression: 2^53+1 at scale 0 returns raw integer",
			amount:   9007199254740993,
			scale:    0,
			expected: "9007199254740993",
		},

		// --- Wei precision (scale 18, ETH) ---
		{
			name:     "wei precision: 1.234567890123456789 ETH = 1234567890123456789 wei",
			amount:   1234567890123456789,
			scale:    18,
			expected: "1.234567890123456789",
		},
		{
			name:     "wei precision: 1 wei",
			amount:   1,
			scale:    18,
			expected: "0.000000000000000001",
		},
		{
			name:     "wei precision: max int64 at scale 18",
			amount:   9223372036854775807, // math.MaxInt64
			scale:    18,
			expected: "9.223372036854775807",
		},

		// --- BTC sat precision (scale 8) ---
		{
			name:     "BTC sat precision: 90000000.00000001 BTC",
			amount:   9000000000000001, // 9.0e15 + 1 sat — past float64 safe range
			scale:    8,
			expected: "90000000.00000001",
		},
		{
			name:     "BTC sat precision: 1 sat",
			amount:   1,
			scale:    8,
			expected: "0.00000001",
		},
		{
			name:     "BTC sat precision: 21M coins (max supply) + 1 sat",
			amount:   2100000000000001, // 21M BTC + 1 sat
			scale:    8,
			expected: "21000000.00000001",
		},

		// --- Sign handling ---
		{
			name:     "negative wei: -1 wei",
			amount:   -1,
			scale:    18,
			expected: "-0.000000000000000001",
		},
		{
			name:     "negative whole + frac at scale 8",
			amount:   -9000000000000001,
			scale:    8,
			expected: "-90000000.00000001",
		},
		{
			name:     "negative scale 2 with fractional",
			amount:   -1234,
			scale:    2,
			expected: "-12.34",
		},

		// --- scale = 0 edge ---
		{
			name:     "scale 0 returns raw integer",
			amount:   1234567890,
			scale:    0,
			expected: "1234567890",
		},
		{
			name:     "scale 0 with negative",
			amount:   -42,
			scale:    0,
			expected: "-42",
		},

		// --- Out-of-range scale: graceful raw int return ---
		{
			name:     "negative scale returns raw integer",
			amount:   1000,
			scale:    -1,
			expected: "1000",
		},
		{
			name:     "scale 19 returns raw integer (would overflow int64 divisor)",
			amount:   1000,
			scale:    19,
			expected: "1000",
		},
		{
			name:     "scale 100 returns raw integer (was previously +Inf via math.Pow10)",
			amount:   1000,
			scale:    100,
			expected: "1000",
		},

		// --- math.MinInt64 — negation overflow guard ---
		{
			name:     "math.MinInt64 at scale 0",
			amount:   -1 << 63,
			scale:    0,
			expected: "-9223372036854775808",
		},
		{
			name:     "math.MinInt64 at scale 18",
			amount:   -1 << 63,
			scale:    18,
			expected: "-9.223372036854775808",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAmount(tt.amount, tt.scale)
			assert.Equal(t, tt.expected, got,
				"formatAmount(%d, %d): wire format must be exact — no float64 rounding",
				tt.amount, tt.scale)
		})
	}
}

// TestFormatAmount_RoundTripParseable ensures the produced wire format is a
// valid decimal string accepted by the standard library. This guards against
// formatting bugs that would slip past arithmetic correctness checks.
func TestFormatAmount_RoundTripParseable(t *testing.T) {
	cases := []struct {
		amount int64
		scale  int64
	}{
		{1234567890123456789, 18},
		{9000000000000001, 8},
		{9007199254740993, 2},
		{-1, 18},
		{-1 << 63, 18},
	}
	for _, c := range cases {
		out := formatAmount(c.amount, c.scale)
		_, err := strconv.ParseFloat(out, 64)
		// ParseFloat doesn't preserve precision, but it must parse syntactically.
		assert.NoErrorf(t, err, "formatAmount(%d, %d) = %q is not a valid decimal", c.amount, c.scale, out)
	}
}

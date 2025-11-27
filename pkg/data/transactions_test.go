package data

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// helper to sum percentages from a DSL string
func sumPercents(t *testing.T, dsl string) int {
	t.Helper()
	re := regexp.MustCompile(`(?m)^\s*(\d+)% to `)
	matches := re.FindAllStringSubmatch(dsl, -1)
	total := 0
	for _, m := range matches {
		v, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("failed to parse percent: %v", err)
		}
		total += v
	}
	return total
}

// TestPaymentPattern tests the PaymentPattern function
func TestPaymentPattern(t *testing.T) {
	t.Run("basic payment pattern", func(t *testing.T) {
		p := PaymentPattern("USD", 10000, "idem-123", "ext-456")

		assert.Equal(t, "payment", p.ChartOfAccountsGroupName)
		assert.Equal(t, "Customer payment to merchant with platform fee", p.Description)
		assert.Equal(t, false, p.RequiresCommit)
		assert.Equal(t, "idem-123", p.IdempotencyKey)
		assert.Equal(t, "ext-456", p.ExternalID)
		assert.Equal(t, "payment", p.Metadata["pattern"])

		// Verify DSL template
		assert.Contains(t, p.DSLTemplate, "send [USD 10000]")
		assert.Contains(t, p.DSLTemplate, "source = @customer")
		assert.Contains(t, p.DSLTemplate, "distribute [USD 10000]")
		assert.Contains(t, p.DSLTemplate, "97% to @merchant_main")
		assert.Contains(t, p.DSLTemplate, "3% to @platform_fee")
	})

	t.Run("payment with different amounts", func(t *testing.T) {
		tests := []struct {
			asset  string
			amount int
		}{
			{"USD", 100},
			{"EUR", 5000},
			{"BRL", 999999},
			{"JPY", 0},
		}

		for _, tt := range tests {
			p := PaymentPattern(tt.asset, tt.amount, "idem", "ext")
			assert.Contains(t, p.DSLTemplate, tt.asset)
			assert.Contains(t, p.DSLTemplate, strconv.Itoa(tt.amount))
		}
	})
}

// TestRefundPattern tests the RefundPattern function
func TestRefundPattern(t *testing.T) {
	t.Run("basic refund pattern", func(t *testing.T) {
		p := RefundPattern("USD", 5000, "idem-refund-123", "ext-refund-456")

		assert.Equal(t, "refund", p.ChartOfAccountsGroupName)
		assert.Equal(t, "Merchant refund to customer", p.Description)
		assert.Equal(t, false, p.RequiresCommit)
		assert.Equal(t, "idem-refund-123", p.IdempotencyKey)
		assert.Equal(t, "ext-refund-456", p.ExternalID)
		assert.Equal(t, "refund", p.Metadata["pattern"])

		// Verify DSL template
		assert.Contains(t, p.DSLTemplate, "send [USD 5000]")
		assert.Contains(t, p.DSLTemplate, "source = @merchant")
		assert.Contains(t, p.DSLTemplate, "distribute [USD 5000]")
		assert.Contains(t, p.DSLTemplate, "100% to @customer")
	})

	t.Run("refund with various currencies", func(t *testing.T) {
		currencies := []string{"USD", "EUR", "BRL", "GBP", "BTC"}
		for _, curr := range currencies {
			p := RefundPattern(curr, 1000, "idem", "ext")
			assert.Contains(t, p.DSLTemplate, curr)
		}
	})
}

// TestTransferPattern tests the TransferPattern function
func TestTransferPattern(t *testing.T) {
	t.Run("valid transfer pattern", func(t *testing.T) {
		p := TransferPattern("USD", 2500, "@source_account", "@dest_account", "idem-transfer", "ext-transfer")

		assert.Equal(t, "transfer", p.ChartOfAccountsGroupName)
		assert.Equal(t, "Internal transfer", p.Description)
		assert.Equal(t, false, p.RequiresCommit)
		assert.Equal(t, "idem-transfer", p.IdempotencyKey)
		assert.Equal(t, "ext-transfer", p.ExternalID)
		assert.Equal(t, "transfer", p.Metadata["pattern"])

		// Verify DSL template
		assert.Contains(t, p.DSLTemplate, "send [USD 2500]")
		assert.Contains(t, p.DSLTemplate, "source = @source_account")
		assert.Contains(t, p.DSLTemplate, "100% to @dest_account")
	})

	t.Run("transfer with invalid source alias", func(t *testing.T) {
		p := TransferPattern("USD", 1000, "invalid alias!", "@dest", "idem", "ext")

		assert.Equal(t, "", p.DSLTemplate)
		assert.Contains(t, p.Description, "Invalid source alias")
		assert.Contains(t, p.Metadata["error"].(string), "invalid alias format")
	})

	t.Run("transfer with invalid destination alias", func(t *testing.T) {
		p := TransferPattern("USD", 1000, "@source", "invalid dest!", "idem", "ext")

		assert.Equal(t, "", p.DSLTemplate)
		assert.Contains(t, p.Description, "Invalid destination alias")
		assert.Contains(t, p.Metadata["error"].(string), "invalid alias format")
	})

	t.Run("transfer with empty source alias", func(t *testing.T) {
		p := TransferPattern("USD", 1000, "", "@dest", "idem", "ext")

		assert.Equal(t, "", p.DSLTemplate)
		assert.Contains(t, p.Description, "Invalid source alias")
	})

	t.Run("transfer with empty destination alias", func(t *testing.T) {
		p := TransferPattern("USD", 1000, "@source", "", "idem", "ext")

		assert.Equal(t, "", p.DSLTemplate)
		assert.Contains(t, p.Description, "Invalid destination alias")
	})

	t.Run("transfer with aliases without @ prefix", func(t *testing.T) {
		p := TransferPattern("USD", 1000, "source_account", "dest_account", "idem", "ext")

		assert.NotEmpty(t, p.DSLTemplate)
		assert.Contains(t, p.DSLTemplate, "source = source_account")
		assert.Contains(t, p.DSLTemplate, "100% to dest_account")
	})
}

// TestFeeCollectionPattern tests the FeeCollectionPattern function
func TestFeeCollectionPattern(t *testing.T) {
	t.Run("basic fee collection", func(t *testing.T) {
		p := FeeCollectionPattern("USD", 1000, 5, "idem-fee", "ext-fee")

		assert.Equal(t, "fee_collection", p.ChartOfAccountsGroupName)
		assert.Equal(t, "Payment with platform fee percentage", p.Description)
		assert.Equal(t, false, p.RequiresCommit)
		assert.Equal(t, "idem-fee", p.IdempotencyKey)
		assert.Equal(t, "ext-fee", p.ExternalID)
		assert.Equal(t, "fee_collection", p.Metadata["pattern"])
		assert.Equal(t, 5, p.Metadata["fee_percent"])

		// Verify DSL template
		assert.Contains(t, p.DSLTemplate, "send [USD 1000]")
		assert.Contains(t, p.DSLTemplate, "95% to @merchant")
		assert.Contains(t, p.DSLTemplate, "5% to @platform-fee")
	})

	t.Run("fee percent edge cases", func(t *testing.T) {
		tests := []struct {
			name         string
			inputPercent int
			expectKeep   int
			expectFee    int
		}{
			{"zero fee", 0, 100, 0},
			{"full fee", 100, 0, 100},
			{"negative fee clamped to zero", -10, 100, 0},
			{"over 100 clamped to 100", 150, 0, 100},
			{"normal fee", 25, 75, 25},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				p := FeeCollectionPattern("USD", 1000, tt.inputPercent, "idem", "ext")

				assert.Contains(t, p.DSLTemplate, strconv.Itoa(tt.expectKeep)+"% to @merchant")
				assert.Contains(t, p.DSLTemplate, strconv.Itoa(tt.expectFee)+"% to @platform-fee")
			})
		}
	})
}

// TestCurrencyExchangePattern tests the CurrencyExchangePattern function
func TestCurrencyExchangePattern(t *testing.T) {
	t.Run("basic currency exchange", func(t *testing.T) {
		p := CurrencyExchangePattern("USD", "EUR", 10000, "idem-fx", "ext-fx")

		assert.Equal(t, "fx", p.ChartOfAccountsGroupName)
		assert.Equal(t, "Customer FX exchange", p.Description)
		assert.Equal(t, false, p.RequiresCommit)
		assert.Equal(t, "idem-fx", p.IdempotencyKey)
		assert.Equal(t, "ext-fx", p.ExternalID)
		assert.Equal(t, "fx", p.Metadata["pattern"])

		// Verify DSL template contains both currencies
		assert.Contains(t, p.DSLTemplate, "send [USD 10000]")
		assert.Contains(t, p.DSLTemplate, "distribute [EUR 10000]")
		assert.Contains(t, p.DSLTemplate, "source = @customer")
		assert.Contains(t, p.DSLTemplate, "100% to @customer")
	})

	t.Run("various currency pairs", func(t *testing.T) {
		pairs := [][2]string{
			{"USD", "EUR"},
			{"BRL", "USD"},
			{"GBP", "JPY"},
			{"BTC", "USD"},
		}

		for _, pair := range pairs {
			p := CurrencyExchangePattern(pair[0], pair[1], 1000, "idem", "ext")
			assert.Contains(t, p.DSLTemplate, pair[0])
			assert.Contains(t, p.DSLTemplate, pair[1])
		}
	})
}

// TestNormalizePercentages tests the normalizePercentages helper function
func TestNormalizePercentages(t *testing.T) {
	t.Run("empty destinations", func(t *testing.T) {
		aliases, normalized := normalizePercentages(map[string]int{})
		assert.Empty(t, aliases)
		assert.Nil(t, normalized)
	})

	t.Run("single destination", func(t *testing.T) {
		dest := map[string]int{"@merchant": 50}
		aliases, normalized := normalizePercentages(dest)

		assert.Equal(t, []string{"@merchant"}, aliases)
		assert.Equal(t, []int{100}, normalized)
	})

	t.Run("equal distribution", func(t *testing.T) {
		dest := map[string]int{
			"@a": 33,
			"@b": 33,
			"@c": 33,
		}
		aliases, normalized := normalizePercentages(dest)

		assert.Len(t, aliases, 3)
		total := 0
		for _, n := range normalized {
			total += n
		}
		assert.Equal(t, 100, total)
	})

	t.Run("clamping negative values", func(t *testing.T) {
		dest := map[string]int{
			"@a": -50,
			"@b": 100,
		}
		aliases, normalized := normalizePercentages(dest)

		assert.Len(t, aliases, 2)
		total := 0
		for _, n := range normalized {
			total += n
		}
		assert.Equal(t, 100, total)
	})

	t.Run("clamping values over 100", func(t *testing.T) {
		dest := map[string]int{
			"@a": 200,
			"@b": 50,
		}
		_, normalized := normalizePercentages(dest)

		total := 0
		for _, n := range normalized {
			total += n
		}
		assert.Equal(t, 100, total)
	})

	t.Run("all zeros assigns first alias", func(t *testing.T) {
		dest := map[string]int{
			"@a": 0,
			"@b": 0,
			"@c": 0,
		}
		aliases, normalized := normalizePercentages(dest)

		// First alias alphabetically should get 100%
		assert.Equal(t, "@a", aliases[0])
		total := 0
		for _, n := range normalized {
			total += n
		}
		assert.Equal(t, 100, total)
	})

	t.Run("deterministic ordering", func(t *testing.T) {
		dest := map[string]int{
			"@z": 25,
			"@a": 25,
			"@m": 25,
		}

		// Run multiple times to ensure deterministic ordering
		for i := 0; i < 10; i++ {
			aliases, _ := normalizePercentages(dest)
			assert.Equal(t, []string{"@a", "@m", "@z"}, aliases, "iteration %d", i)
		}
	})
}

// TestBatchSettlementPattern tests the BatchSettlementPattern function
func TestBatchSettlementPattern(t *testing.T) {
	t.Run("basic batch settlement", func(t *testing.T) {
		dest := map[string]int{
			"@merchant_a": 50,
			"@merchant_b": 30,
			"@platform":   20,
		}
		p := BatchSettlementPattern("USD", 100000, dest, "idem-batch", "ext-batch")

		assert.Equal(t, "batch_settlement", p.ChartOfAccountsGroupName)
		assert.Equal(t, "Batch settlement to multiple parties", p.Description)
		assert.Equal(t, false, p.RequiresCommit)
		assert.Equal(t, "idem-batch", p.IdempotencyKey)
		assert.Equal(t, "ext-batch", p.ExternalID)
		assert.Equal(t, "batch_settlement", p.Metadata["pattern"])

		// Verify DSL template
		assert.Contains(t, p.DSLTemplate, "send [USD 100000]")
		assert.Contains(t, p.DSLTemplate, "source = @settlement_pool")

		// Verify percentages sum to 100
		got := sumPercents(t, p.DSLTemplate)
		assert.Equal(t, 100, got)
	})

	t.Run("invalid alias in destinations", func(t *testing.T) {
		dest := map[string]int{
			"@valid":   50,
			"invalid!": 50,
		}
		p := BatchSettlementPattern("USD", 1000, dest, "idem", "ext")

		assert.Equal(t, "", p.DSLTemplate)
		assert.Contains(t, p.Description, "Invalid destination alias")
		assert.Contains(t, p.Metadata["error"].(string), "invalid alias format")
	})

	t.Run("normalizes to 100", func(t *testing.T) {
		dest := map[string]int{
			"@merchant_main": 50,
			"@platform_fee":  30,
		}
		p := BatchSettlementPattern("USD", 1000, dest, "idemp", "ext")
		got := sumPercents(t, p.DSLTemplate)
		assert.Equal(t, 100, got)
	})

	t.Run("all zeros assigns first alias", func(t *testing.T) {
		dest := map[string]int{
			"@b": 0,
			"@a": 0,
		}
		p := BatchSettlementPattern("USD", 500, dest, "idemp", "ext")
		got := sumPercents(t, p.DSLTemplate)
		assert.Equal(t, 100, got)
	})

	t.Run("clamps and normalizes", func(t *testing.T) {
		dest := map[string]int{
			"@x": -10,
			"@y": 250,
			"@z": 10,
		}
		p := BatchSettlementPattern("EUR", 123, dest, "idemp", "ext")
		got := sumPercents(t, p.DSLTemplate)
		assert.Equal(t, 100, got)
	})
}

// TestSubscriptionPattern tests the SubscriptionPattern function
func TestSubscriptionPattern(t *testing.T) {
	t.Run("basic subscription", func(t *testing.T) {
		p := SubscriptionPattern("USD", 9999, "idem-sub", "ext-sub")

		assert.Equal(t, "subscription", p.ChartOfAccountsGroupName)
		assert.Equal(t, "Recurring subscription payment", p.Description)
		assert.Equal(t, false, p.RequiresCommit)
		assert.Equal(t, "idem-sub", p.IdempotencyKey)
		assert.Equal(t, "ext-sub", p.ExternalID)
		assert.Equal(t, "subscription", p.Metadata["pattern"])
		assert.Equal(t, true, p.Metadata["recurring"])

		// Verify DSL template
		assert.Contains(t, p.DSLTemplate, "send [USD 9999]")
		assert.Contains(t, p.DSLTemplate, "source = @customer")
		assert.Contains(t, p.DSLTemplate, "100% to @merchant_main")
	})

	t.Run("subscription with different currencies", func(t *testing.T) {
		currencies := []string{"USD", "EUR", "GBP", "BRL"}
		for _, curr := range currencies {
			p := SubscriptionPattern(curr, 1999, "idem", "ext")
			assert.Contains(t, p.DSLTemplate, curr)
		}
	})
}

// TestSplitPaymentPattern tests the SplitPaymentPattern function
func TestSplitPaymentPattern(t *testing.T) {
	t.Run("basic split payment", func(t *testing.T) {
		dest := map[string]int{
			"@merchant_a": 70,
			"@merchant_b": 30,
		}
		p := SplitPaymentPattern("USD", 50000, dest, "idem-split", "ext-split")

		assert.Equal(t, "split_payment", p.ChartOfAccountsGroupName)
		assert.Equal(t, "Customer payment split among multiple recipients", p.Description)
		assert.Equal(t, false, p.RequiresCommit)
		assert.Equal(t, "idem-split", p.IdempotencyKey)
		assert.Equal(t, "ext-split", p.ExternalID)
		assert.Equal(t, "split_payment", p.Metadata["pattern"])

		// Verify DSL template
		assert.Contains(t, p.DSLTemplate, "send [USD 50000]")
		assert.Contains(t, p.DSLTemplate, "source = @customer")

		// Verify percentages sum to 100
		got := sumPercents(t, p.DSLTemplate)
		assert.Equal(t, 100, got)
	})

	t.Run("invalid alias in destinations", func(t *testing.T) {
		dest := map[string]int{
			"@valid":   50,
			"invalid!": 50,
		}
		p := SplitPaymentPattern("USD", 1000, dest, "idem", "ext")

		assert.Equal(t, "", p.DSLTemplate)
		assert.Contains(t, p.Description, "Invalid destination alias")
		assert.Contains(t, p.Metadata["error"].(string), "invalid alias format")
	})

	t.Run("normalizes to 100", func(t *testing.T) {
		dest := map[string]int{
			"@merchant_main": 50,
			"@platform_fee":  30,
		}
		p := SplitPaymentPattern("USD", 1000, dest, "idemp", "ext")
		if !strings.Contains(p.DSLTemplate, "distribute [USD 1000]") {
			t.Fatalf("unexpected DSL: %s", p.DSLTemplate)
		}
		got := sumPercents(t, p.DSLTemplate)
		assert.Equal(t, 100, got)
	})

	t.Run("all zeros assigns first alias", func(t *testing.T) {
		dest := map[string]int{
			"@b": 0,
			"@a": 0,
		}
		p := SplitPaymentPattern("USD", 500, dest, "idemp", "ext")
		got := sumPercents(t, p.DSLTemplate)
		assert.Equal(t, 100, got)
	})

	t.Run("clamps and normalizes", func(t *testing.T) {
		dest := map[string]int{
			"@x": -10,
			"@y": 250,
			"@z": 10,
		}
		p := SplitPaymentPattern("EUR", 123, dest, "idemp", "ext")
		got := sumPercents(t, p.DSLTemplate)
		assert.Equal(t, 100, got)
	})

	t.Run("many destinations", func(t *testing.T) {
		dest := map[string]int{
			"@a": 10,
			"@b": 10,
			"@c": 10,
			"@d": 10,
			"@e": 10,
			"@f": 10,
			"@g": 10,
			"@h": 10,
			"@i": 10,
			"@j": 10,
		}
		p := SplitPaymentPattern("USD", 100000, dest, "idem", "ext")
		got := sumPercents(t, p.DSLTemplate)
		assert.Equal(t, 100, got)
	})
}

// TestTransactionPatternStructFromTransactions tests the TransactionPattern struct
func TestTransactionPatternStructFromTransactions(t *testing.T) {
	p := TransactionPattern{
		ChartOfAccountsGroupName: "test_group",
		Description:              "Test description",
		DSLTemplate:              "send [USD 100] (source = @a)",
		RequiresCommit:           true,
		IdempotencyKey:           "test-idem-key",
		ExternalID:               "test-ext-id",
		Metadata:                 map[string]any{"key": "value"},
	}

	assert.Equal(t, "test_group", p.ChartOfAccountsGroupName)
	assert.Equal(t, "Test description", p.Description)
	assert.Equal(t, "send [USD 100] (source = @a)", p.DSLTemplate)
	assert.True(t, p.RequiresCommit)
	assert.Equal(t, "test-idem-key", p.IdempotencyKey)
	assert.Equal(t, "test-ext-id", p.ExternalID)
	assert.Equal(t, "value", p.Metadata["key"])
}

// TestDSLTemplateInjectionPrevention tests that aliases are properly validated
func TestDSLTemplateInjectionPrevention(t *testing.T) {
	maliciousAliases := []string{
		"@alias\nmalicious_code",
		"@alias; drop table",
		"@alias`command`",
		"@alias$(command)",
		"@alias{{template}}",
		"@alias<script>",
		"alias with spaces",
		"alias\twith\ttabs",
		"",
	}

	for _, malicious := range maliciousAliases {
		t.Run("transfer_source_"+malicious, func(t *testing.T) {
			p := TransferPattern("USD", 1000, malicious, "@dest", "idem", "ext")
			assert.Empty(t, p.DSLTemplate)
		})

		t.Run("transfer_dest_"+malicious, func(t *testing.T) {
			p := TransferPattern("USD", 1000, "@source", malicious, "idem", "ext")
			assert.Empty(t, p.DSLTemplate)
		})
	}
}

// BenchmarkNormalizePercentages benchmarks the normalization function
func BenchmarkNormalizePercentages(b *testing.B) {
	dest := map[string]int{
		"@a": 25,
		"@b": 25,
		"@c": 25,
		"@d": 25,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		normalizePercentages(dest)
	}
}

// BenchmarkSplitPaymentPattern benchmarks split payment generation
func BenchmarkSplitPaymentPattern(b *testing.B) {
	dest := map[string]int{
		"@merchant_a": 50,
		"@merchant_b": 30,
		"@platform":   20,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SplitPaymentPattern("USD", 10000, dest, "idem", "ext")
	}
}

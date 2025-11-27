package data

import (
	"fmt"
	"math"
	"sort"
)

// PaymentPattern returns a TransactionPattern for a simple payment flow.
// Placeholder aliases: @customer, @merchant, @platform-fee.
func PaymentPattern(asset string, amount int, idempotencyKey, externalID string) TransactionPattern {
	dsl := fmt.Sprintf(`
send [%s %d] (
  source = @customer
)
distribute [%s %d] (
  destination = {
    97%% to @merchant_main
    3%% to @platform_fee
  }
)
`, asset, amount, asset, amount)

	return TransactionPattern{
		ChartOfAccountsGroupName: "payment",
		Description:              "Customer payment to merchant with platform fee",
		DSLTemplate:              dsl,
		RequiresCommit:           false,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               externalID,
		Metadata:                 map[string]any{"pattern": "payment"},
	}
}

// RefundPattern reverses a prior payment.
func RefundPattern(asset string, amount int, idempotencyKey, externalID string) TransactionPattern {
	dsl := fmt.Sprintf(`
send [%s %d] (
  source = @merchant
)
distribute [%s %d] (
  destination = {
    100%% to @customer
  }
)
`, asset, amount, asset, amount)

	return TransactionPattern{
		ChartOfAccountsGroupName: "refund",
		Description:              "Merchant refund to customer",
		DSLTemplate:              dsl,
		RequiresCommit:           false,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               externalID,
		Metadata:                 map[string]any{"pattern": "refund"},
	}
}

// TransferPattern moves funds between two aliases.
// Aliases are validated to prevent DSL template injection.
// Returns an empty DSLTemplate if alias validation fails.
func TransferPattern(asset string, amount int, sourceAlias, destAlias, idempotencyKey, externalID string) TransactionPattern {
	// Validate aliases to prevent DSL injection
	if err := ValidateDSLAlias(sourceAlias); err != nil {
		return TransactionPattern{
			ChartOfAccountsGroupName: "transfer",
			Description:              fmt.Sprintf("Invalid source alias: %v", err),
			DSLTemplate:              "",
			IdempotencyKey:           idempotencyKey,
			ExternalID:               externalID,
			Metadata:                 map[string]any{"pattern": "transfer", "error": err.Error()},
		}
	}

	if err := ValidateDSLAlias(destAlias); err != nil {
		return TransactionPattern{
			ChartOfAccountsGroupName: "transfer",
			Description:              fmt.Sprintf("Invalid destination alias: %v", err),
			DSLTemplate:              "",
			IdempotencyKey:           idempotencyKey,
			ExternalID:               externalID,
			Metadata:                 map[string]any{"pattern": "transfer", "error": err.Error()},
		}
	}

	dsl := fmt.Sprintf(`
send [%s %d] (
  source = %s
)
distribute [%s %d] (
  destination = {
    100%% to %s
  }
)
`, asset, amount, sourceAlias, asset, amount, destAlias)

	return TransactionPattern{
		ChartOfAccountsGroupName: "transfer",
		Description:              "Internal transfer",
		DSLTemplate:              dsl,
		RequiresCommit:           false,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               externalID,
		Metadata:                 map[string]any{"pattern": "transfer"},
	}
}

// FeeCollectionPattern collects a percentage-based fee.
func FeeCollectionPattern(asset string, amount int, feePercent int, idempotencyKey, externalID string) TransactionPattern {
	if feePercent < 0 {
		feePercent = 0
	}

	if feePercent > 100 {
		feePercent = 100
	}

	keep := 100 - feePercent
	dsl := fmt.Sprintf(`
send [%s %d] (
  source = @customer
)
distribute [%s %d] (
  destination = {
    %d%% to @merchant
    %d%% to @platform-fee
  }
)
`, asset, amount, asset, amount, keep, feePercent)

	return TransactionPattern{
		ChartOfAccountsGroupName: "fee_collection",
		Description:              "Payment with platform fee percentage",
		DSLTemplate:              dsl,
		RequiresCommit:           false,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               externalID,
		Metadata:                 map[string]any{"pattern": "fee_collection", "fee_percent": feePercent},
	}
}

// CurrencyExchangePattern performs a basic FX exchange using rate configs (applied later).
func CurrencyExchangePattern(srcAsset string, destAsset string, amount int, idempotencyKey, externalID string) TransactionPattern {
	dsl := fmt.Sprintf(`
send [%s %d] (
  source = @customer
)
// In later phases, rate application will be configured via AssetRateService
distribute [%s %d] (
  destination = {
    100%% to @customer
  }
)
`, srcAsset, amount, destAsset, amount)

	return TransactionPattern{
		ChartOfAccountsGroupName: "fx",
		Description:              "Customer FX exchange",
		DSLTemplate:              dsl,
		RequiresCommit:           false,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               externalID,
		Metadata:                 map[string]any{"pattern": "fx"},
	}
}

// normalizePercentages takes a map of alias -> percentage share and returns sorted aliases
// with their normalized percentages that sum to exactly 100%.
// Handles clamping (0-100), zero-sum fallback, and rounding error correction.
func normalizePercentages(destinations map[string]int) ([]string, []int) {
	// Build a deterministic list of aliases for stable output
	aliases := make([]string, 0, len(destinations))
	for alias := range destinations {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	if len(aliases) == 0 {
		return aliases, nil
	}

	// Clamp and collect initial percentages
	clamped := make([]int, len(aliases))
	sum := 0

	for i, alias := range aliases {
		pct := destinations[alias]
		if pct < 0 {
			pct = 0
		}

		if pct > 100 {
			pct = 100
		}

		clamped[i] = pct
		sum += pct
	}

	// Normalize to ensure the total equals exactly 100%
	normalized := make([]int, len(aliases))

	if sum == 0 {
		// If all inputs are zero or invalid, assign 100% to the first alias
		normalized[0] = 100
	} else {
		// Proportional normalization with rounding error correction
		fractional := make([]struct {
			idx  int
			frac float64
		}, len(aliases))
		total := 0

		for i := range aliases {
			raw := (float64(clamped[i]) * 100.0) / float64(sum)
			floor := int(math.Floor(raw))
			normalized[i] = floor
			fractional[i] = struct {
				idx  int
				frac float64
			}{idx: i, frac: raw - float64(floor)}
			total += floor
		}

		// Distribute remaining percentages to highest fractional parts
		shortfall := 100 - total
		if shortfall > 0 {
			sort.Slice(fractional, func(i, j int) bool { return fractional[i].frac > fractional[j].frac })

			for k := 0; k < shortfall && k < len(fractional); k++ {
				normalized[fractional[k].idx]++
			}
		}
	}

	return aliases, normalized
}

// BatchSettlementPattern distributes to multiple destinations with shares.
// Destination aliases are validated to prevent DSL template injection.
// Returns an empty DSLTemplate if any alias validation fails.
func BatchSettlementPattern(asset string, amount int, destinations map[string]int, idempotencyKey, externalID string) TransactionPattern {
	// Validate all destination aliases before processing
	for alias := range destinations {
		if err := ValidateDSLAlias(alias); err != nil {
			return TransactionPattern{
				ChartOfAccountsGroupName: "batch_settlement",
				Description:              fmt.Sprintf("Invalid destination alias: %v", err),
				DSLTemplate:              "",
				IdempotencyKey:           idempotencyKey,
				ExternalID:               externalID,
				Metadata:                 map[string]any{"pattern": "batch_settlement", "error": err.Error()},
			}
		}
	}

	aliases, normalized := normalizePercentages(destinations)

	// Build DSL shares block
	shares := ""

	for i, alias := range aliases {
		if normalized[i] <= 0 {
			continue
		}

		shares += fmt.Sprintf("    %d%% to %s\n", normalized[i], alias)
	}

	dsl := fmt.Sprintf(`
send [%s %d] (
  source = @settlement_pool
)
distribute [%s %d] (
  destination = {
%s  }
)
`, asset, amount, asset, amount, shares)

	return TransactionPattern{
		ChartOfAccountsGroupName: "batch_settlement",
		Description:              "Batch settlement to multiple parties",
		DSLTemplate:              dsl,
		RequiresCommit:           false,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               externalID,
		Metadata:                 map[string]any{"pattern": "batch_settlement"},
	}
}

// SubscriptionPattern represents a recurring charge from a customer to a merchant.
func SubscriptionPattern(asset string, amount int, idempotencyKey, externalID string) TransactionPattern {
	dsl := fmt.Sprintf(`
send [%s %d] (
  source = @customer
)
distribute [%s %d] (
  destination = {
    100%% to @merchant_main
  }
)
`, asset, amount, asset, amount)

	return TransactionPattern{
		ChartOfAccountsGroupName: "subscription",
		Description:              "Recurring subscription payment",
		DSLTemplate:              dsl,
		RequiresCommit:           false,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               externalID,
		Metadata:                 map[string]any{"pattern": "subscription", "recurring": true},
	}
}

// SplitPaymentPattern splits a payment from a customer to multiple recipients using percentages.
// Destination aliases are validated to prevent DSL template injection.
// Returns an empty DSLTemplate if any alias validation fails.
func SplitPaymentPattern(asset string, amount int, destinations map[string]int, idempotencyKey, externalID string) TransactionPattern {
	// Validate all destination aliases before processing
	for alias := range destinations {
		if err := ValidateDSLAlias(alias); err != nil {
			return TransactionPattern{
				ChartOfAccountsGroupName: "split_payment",
				Description:              fmt.Sprintf("Invalid destination alias: %v", err),
				DSLTemplate:              "",
				IdempotencyKey:           idempotencyKey,
				ExternalID:               externalID,
				Metadata:                 map[string]any{"pattern": "split_payment", "error": err.Error()},
			}
		}
	}

	aliases, normalized := normalizePercentages(destinations)

	// Build DSL shares block
	shares := ""

	for i, alias := range aliases {
		if normalized[i] <= 0 {
			continue
		}

		shares += fmt.Sprintf("    %d%% to %s\n", normalized[i], alias)
	}

	dsl := fmt.Sprintf(`
send [%s %d] (
  source = @customer
)
distribute [%s %d] (
  destination = {
%s  }
)
`, asset, amount, asset, amount, shares)

	return TransactionPattern{
		ChartOfAccountsGroupName: "split_payment",
		Description:              "Customer payment split among multiple recipients",
		DSLTemplate:              dsl,
		RequiresCommit:           false,
		IdempotencyKey:           idempotencyKey,
		ExternalID:               externalID,
		Metadata:                 map[string]any{"pattern": "split_payment"},
	}
}

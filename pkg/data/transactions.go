package data

import (
    "fmt"
    "log"
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
func TransferPattern(asset string, amount int, sourceAlias, destAlias, idempotencyKey, externalID string) TransactionPattern {
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

// BatchSettlementPattern distributes to multiple destinations with shares.
func BatchSettlementPattern(asset string, amount int, destinations map[string]int, idempotencyKey, externalID string) TransactionPattern {
	// destinations is a map of alias -> percentage share
	shares := ""
	for alias, pct := range destinations {
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		shares += fmt.Sprintf("    %d%% to %s\n", pct, alias)
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
func SplitPaymentPattern(asset string, amount int, destinations map[string]int, idempotencyKey, externalID string) TransactionPattern {
    shares := ""
    totalPct := 0
    for alias, pct := range destinations {
        if pct < 0 {
            pct = 0
        }
        if pct > 100 {
            pct = 100
        }
        totalPct += pct
        shares += fmt.Sprintf("    %d%% to %s\n", pct, alias)
    }

    if totalPct != 100 {
        log.Printf("Warning: split percentages sum to %d%%, not 100%%", totalPct)
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

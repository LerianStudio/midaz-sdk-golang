package data

import (
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
)

// Account type key constants define standard account type identifiers.
// These match the constants in pkg/generator for consistency.
const (
	AccountTypeKeyChecking   = "CHECKING"
	AccountTypeKeySavings    = "SAVINGS"
	AccountTypeKeyCreditCard = "CREDIT_CARD"
	AccountTypeKeyExpense    = "EXPENSE"
	AccountTypeKeyRevenue    = "REVENUE"
	AccountTypeKeyLiability  = "LIABILITY"
	AccountTypeKeyEquity     = "EQUITY"
)

// CustomerAccounts returns templates representing customer-owned accounts.
// Types align to validator recognized values (deposit/savings/creditCard/marketplace).
func CustomerAccounts() []AccountTemplate {
	return []AccountTemplate{
		{
			Name:           "Customer Deposits",
			Type:           "deposit",
			Status:         models.NewStatus(models.StatusActive),
			AccountTypeKey: StrPtr(AccountTypeKeyChecking),
			Metadata: map[string]any{
				"role":       "customer",
				"risk_level": "low",
			},
		},
		{
			Name:           "Customer Savings",
			Type:           "savings",
			Status:         models.NewStatus(models.StatusActive),
			AccountTypeKey: StrPtr(AccountTypeKeySavings),
			Metadata: map[string]any{
				"role":    "customer",
				"purpose": "savings",
			},
		},
		{
			Name:           "Primary Customer",
			Type:           "deposit",
			Status:         models.NewStatus(models.StatusActive),
			Alias:          StrPtr("customer"),
			AccountTypeKey: StrPtr(AccountTypeKeyChecking),
			Metadata: map[string]any{
				"role":    "customer",
				"primary": true,
			},
		},
	}
}

// MerchantAccounts returns merchant accounts for settlement routing.
func MerchantAccounts() []AccountTemplate {
	alias := "merchant_main"

	return []AccountTemplate{
		{
			Name:           "Merchant Settlement",
			Type:           "marketplace",
			Status:         models.NewStatus(models.StatusActive),
			Alias:          &alias,
			AccountTypeKey: StrPtr(AccountTypeKeyChecking),
			Metadata: map[string]any{
				"role":     "merchant",
				"category": "settlement",
			},
		},
	}
}

// FeeAccounts returns templates for fee collection.
func FeeAccounts() []AccountTemplate {
	alias := "platform_fee"

	return []AccountTemplate{
		{
			Name:           "Platform Fees",
			Type:           "deposit",
			Status:         models.NewStatus(models.StatusActive),
			Alias:          &alias,
			AccountTypeKey: StrPtr(AccountTypeKeyChecking),
			Metadata: map[string]any{
				"role":     "internal",
				"category": "fees",
			},
		},
	}
}

// SettlementAccounts returns templates for internal settlement.
func SettlementAccounts() []AccountTemplate {
	alias := "settlement_pool"

	return []AccountTemplate{
		{
			Name:           "Settlement Pool",
			Type:           "deposit",
			Status:         models.NewStatus(models.StatusActive),
			Alias:          &alias,
			AccountTypeKey: StrPtr(AccountTypeKeyChecking),
			Metadata: map[string]any{
				"role":     "internal",
				"category": "settlement",
			},
		},
	}
}

// EscrowAccounts returns templates for holding funds.
func EscrowAccounts() []AccountTemplate {
	alias := "escrow_hold"

	return []AccountTemplate{
		{
			Name:           "Escrow Hold",
			Type:           "deposit",
			Status:         models.NewStatus(models.StatusActive),
			Alias:          &alias,
			AccountTypeKey: StrPtr(AccountTypeKeyChecking),
			Metadata: map[string]any{
				"role":     "internal",
				"category": "escrow",
			},
		},
	}
}

// RevenueAccounts returns templates for revenue categorization (mapped later to account types).
func RevenueAccounts() []AccountTemplate {
	alias := "revenue_main"

	return []AccountTemplate{
		{
			Name:           "Revenue Main",
			Type:           "revenue",
			Status:         models.NewStatus(models.StatusActive),
			Alias:          &alias,
			AccountTypeKey: StrPtr(AccountTypeKeyRevenue),
			Metadata: map[string]any{
				"role":     "internal",
				"category": "revenue",
			},
		},
	}
}

// ExpenseAccounts returns templates for expense tracking (mapped later to account types).
func ExpenseAccounts() []AccountTemplate {
	alias := "expense_main"

	return []AccountTemplate{
		{
			Name:           "Expense Main",
			Type:           "expense",
			Status:         models.NewStatus(models.StatusActive),
			Alias:          &alias,
			AccountTypeKey: StrPtr(AccountTypeKeyExpense),
			Metadata: map[string]any{
				"role":     "internal",
				"category": "expense",
			},
		},
	}
}

// LiabilityAccounts returns templates representing liability-class accounts.
func LiabilityAccounts() []AccountTemplate {
	apAlias := "ap_main"
	loanAlias := "loan_payable"

	return []AccountTemplate{
		{
			Name:           "Accounts Payable",
			Type:           "liability",
			Status:         models.NewStatus(models.StatusActive),
			Alias:          &apAlias,
			AccountTypeKey: StrPtr(AccountTypeKeyLiability),
			Metadata: map[string]any{
				"role":     "internal",
				"category": "payables",
			},
		},
		{
			Name:           "Loans Payable",
			Type:           "liability",
			Status:         models.NewStatus(models.StatusActive),
			Alias:          &loanAlias,
			AccountTypeKey: StrPtr(AccountTypeKeyLiability),
			Metadata: map[string]any{
				"role":     "internal",
				"category": "loans",
			},
		},
	}
}

// EquityAccounts returns templates representing equity-class accounts.
func EquityAccounts() []AccountTemplate {
	alias := "owners_equity"

	return []AccountTemplate{
		{
			Name:           "Owner's Equity",
			Type:           "equity",
			Status:         models.NewStatus(models.StatusActive),
			Alias:          &alias,
			AccountTypeKey: StrPtr(AccountTypeKeyEquity),
			Metadata: map[string]any{
				"role":     "internal",
				"category": "equity",
			},
		},
	}
}

// AllAccountTemplates aggregates representative account templates.
func AllAccountTemplates() []AccountTemplate {
	out := []AccountTemplate{}
	groups := [][]AccountTemplate{
		CustomerAccounts(),
		MerchantAccounts(),
		FeeAccounts(),
		SettlementAccounts(),
		EscrowAccounts(),
		RevenueAccounts(),
		ExpenseAccounts(),
		LiabilityAccounts(),
		EquityAccounts(),
	}

	for _, g := range groups {
		out = append(out, g...)
	}

	return out
}

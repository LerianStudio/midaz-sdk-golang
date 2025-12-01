package data

import (
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomerAccounts(t *testing.T) {
	accounts := CustomerAccounts()

	require.Len(t, accounts, 3)

	t.Run("deposits account", func(t *testing.T) {
		acc := accounts[0]
		assert.Equal(t, "Customer Deposits", acc.Name)
		assert.Equal(t, "deposit", acc.Type)
		assert.Equal(t, models.StatusActive, acc.Status.Code)
		assert.NotNil(t, acc.AccountTypeKey)
		assert.Equal(t, AccountTypeKeyChecking, *acc.AccountTypeKey)
		assert.Equal(t, "customer", acc.Metadata["role"])
		assert.Equal(t, "low", acc.Metadata["risk_level"])
	})

	t.Run("savings account", func(t *testing.T) {
		acc := accounts[1]
		assert.Equal(t, "Customer Savings", acc.Name)
		assert.Equal(t, "savings", acc.Type)
		assert.Equal(t, models.StatusActive, acc.Status.Code)
		assert.NotNil(t, acc.AccountTypeKey)
		assert.Equal(t, AccountTypeKeySavings, *acc.AccountTypeKey)
		assert.Equal(t, "customer", acc.Metadata["role"])
		assert.Equal(t, "savings", acc.Metadata["purpose"])
	})

	t.Run("primary customer account with alias", func(t *testing.T) {
		acc := accounts[2]
		assert.Equal(t, "Primary Customer", acc.Name)
		assert.Equal(t, "deposit", acc.Type)
		require.NotNil(t, acc.Alias)
		assert.Equal(t, "customer", *acc.Alias)
		assert.Equal(t, true, acc.Metadata["primary"])
	})
}

func TestMerchantAccounts(t *testing.T) {
	accounts := MerchantAccounts()

	require.Len(t, accounts, 1)

	acc := accounts[0]
	assert.Equal(t, "Merchant Settlement", acc.Name)
	assert.Equal(t, "marketplace", acc.Type)
	assert.Equal(t, models.StatusActive, acc.Status.Code)
	require.NotNil(t, acc.Alias)
	assert.Equal(t, "merchant_main", *acc.Alias)
	assert.NotNil(t, acc.AccountTypeKey)
	assert.Equal(t, AccountTypeKeyChecking, *acc.AccountTypeKey)
	assert.Equal(t, "merchant", acc.Metadata["role"])
	assert.Equal(t, "settlement", acc.Metadata["category"])
}

func TestFeeAccounts(t *testing.T) {
	accounts := FeeAccounts()

	require.Len(t, accounts, 1)

	acc := accounts[0]
	assert.Equal(t, "Platform Fees", acc.Name)
	assert.Equal(t, "deposit", acc.Type)
	assert.Equal(t, models.StatusActive, acc.Status.Code)
	require.NotNil(t, acc.Alias)
	assert.Equal(t, "platform_fee", *acc.Alias)
	assert.NotNil(t, acc.AccountTypeKey)
	assert.Equal(t, AccountTypeKeyChecking, *acc.AccountTypeKey)
	assert.Equal(t, "internal", acc.Metadata["role"])
	assert.Equal(t, "fees", acc.Metadata["category"])
}

func TestSettlementAccounts(t *testing.T) {
	accounts := SettlementAccounts()

	require.Len(t, accounts, 1)

	acc := accounts[0]
	assert.Equal(t, "Settlement Pool", acc.Name)
	assert.Equal(t, "deposit", acc.Type)
	assert.Equal(t, models.StatusActive, acc.Status.Code)
	require.NotNil(t, acc.Alias)
	assert.Equal(t, "settlement_pool", *acc.Alias)
	assert.NotNil(t, acc.AccountTypeKey)
	assert.Equal(t, AccountTypeKeyChecking, *acc.AccountTypeKey)
	assert.Equal(t, "internal", acc.Metadata["role"])
	assert.Equal(t, "settlement", acc.Metadata["category"])
}

func TestEscrowAccounts(t *testing.T) {
	accounts := EscrowAccounts()

	require.Len(t, accounts, 1)

	acc := accounts[0]
	assert.Equal(t, "Escrow Hold", acc.Name)
	assert.Equal(t, "deposit", acc.Type)
	assert.Equal(t, models.StatusActive, acc.Status.Code)
	require.NotNil(t, acc.Alias)
	assert.Equal(t, "escrow_hold", *acc.Alias)
	assert.NotNil(t, acc.AccountTypeKey)
	assert.Equal(t, AccountTypeKeyChecking, *acc.AccountTypeKey)
	assert.Equal(t, "internal", acc.Metadata["role"])
	assert.Equal(t, "escrow", acc.Metadata["category"])
}

func TestRevenueAccounts(t *testing.T) {
	accounts := RevenueAccounts()

	require.Len(t, accounts, 1)

	acc := accounts[0]
	assert.Equal(t, "Revenue Main", acc.Name)
	assert.Equal(t, "revenue", acc.Type)
	assert.Equal(t, models.StatusActive, acc.Status.Code)
	require.NotNil(t, acc.Alias)
	assert.Equal(t, "revenue_main", *acc.Alias)
	assert.NotNil(t, acc.AccountTypeKey)
	assert.Equal(t, AccountTypeKeyRevenue, *acc.AccountTypeKey)
	assert.Equal(t, "internal", acc.Metadata["role"])
	assert.Equal(t, "revenue", acc.Metadata["category"])
}

func TestExpenseAccounts(t *testing.T) {
	accounts := ExpenseAccounts()

	require.Len(t, accounts, 1)

	acc := accounts[0]
	assert.Equal(t, "Expense Main", acc.Name)
	assert.Equal(t, "expense", acc.Type)
	assert.Equal(t, models.StatusActive, acc.Status.Code)
	require.NotNil(t, acc.Alias)
	assert.Equal(t, "expense_main", *acc.Alias)
	assert.NotNil(t, acc.AccountTypeKey)
	assert.Equal(t, AccountTypeKeyExpense, *acc.AccountTypeKey)
	assert.Equal(t, "internal", acc.Metadata["role"])
	assert.Equal(t, "expense", acc.Metadata["category"])
}

func TestLiabilityAccounts(t *testing.T) {
	accounts := LiabilityAccounts()

	require.Len(t, accounts, 2)

	t.Run("accounts payable", func(t *testing.T) {
		acc := accounts[0]
		assert.Equal(t, "Accounts Payable", acc.Name)
		assert.Equal(t, "liability", acc.Type)
		assert.Equal(t, models.StatusActive, acc.Status.Code)
		require.NotNil(t, acc.Alias)
		assert.Equal(t, "ap_main", *acc.Alias)
		assert.NotNil(t, acc.AccountTypeKey)
		assert.Equal(t, AccountTypeKeyLiability, *acc.AccountTypeKey)
		assert.Equal(t, "internal", acc.Metadata["role"])
		assert.Equal(t, "payables", acc.Metadata["category"])
	})

	t.Run("loans payable", func(t *testing.T) {
		acc := accounts[1]
		assert.Equal(t, "Loans Payable", acc.Name)
		assert.Equal(t, "liability", acc.Type)
		assert.Equal(t, models.StatusActive, acc.Status.Code)
		require.NotNil(t, acc.Alias)
		assert.Equal(t, "loan_payable", *acc.Alias)
		assert.NotNil(t, acc.AccountTypeKey)
		assert.Equal(t, AccountTypeKeyLiability, *acc.AccountTypeKey)
		assert.Equal(t, "internal", acc.Metadata["role"])
		assert.Equal(t, "loans", acc.Metadata["category"])
	})
}

func TestEquityAccounts(t *testing.T) {
	accounts := EquityAccounts()

	require.Len(t, accounts, 1)

	acc := accounts[0]
	assert.Equal(t, "Owner's Equity", acc.Name)
	assert.Equal(t, "equity", acc.Type)
	assert.Equal(t, models.StatusActive, acc.Status.Code)
	require.NotNil(t, acc.Alias)
	assert.Equal(t, "owners_equity", *acc.Alias)
	assert.NotNil(t, acc.AccountTypeKey)
	assert.Equal(t, AccountTypeKeyEquity, *acc.AccountTypeKey)
	assert.Equal(t, "internal", acc.Metadata["role"])
	assert.Equal(t, "equity", acc.Metadata["category"])
}

func TestAllAccountTemplates(t *testing.T) {
	accounts := AllAccountTemplates()

	// Count expected accounts:
	// CustomerAccounts: 3
	// MerchantAccounts: 1
	// FeeAccounts: 1
	// SettlementAccounts: 1
	// EscrowAccounts: 1
	// RevenueAccounts: 1
	// ExpenseAccounts: 1
	// LiabilityAccounts: 2
	// EquityAccounts: 1
	// Total: 12
	expectedCount := 12
	require.Len(t, accounts, expectedCount)

	// Verify all accounts have required fields
	for i, acc := range accounts {
		assert.NotEmpty(t, acc.Name, "account %d should have a name", i)
		assert.NotEmpty(t, acc.Type, "account %d should have a type", i)
		assert.NotNil(t, acc.Metadata, "account %d should have metadata", i)
	}

	// Verify account types are valid
	validTypes := map[string]bool{
		"deposit":     true,
		"savings":     true,
		"marketplace": true,
		"revenue":     true,
		"expense":     true,
		"liability":   true,
		"equity":      true,
	}

	for i, acc := range accounts {
		assert.True(t, validTypes[acc.Type], "account %d has invalid type: %s", i, acc.Type)
	}
}

func TestAccountTypeKeyConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"checking", AccountTypeKeyChecking, "CHECKING"},
		{"savings", AccountTypeKeySavings, "SAVINGS"},
		{"credit card", AccountTypeKeyCreditCard, "CREDIT_CARD"},
		{"expense", AccountTypeKeyExpense, "EXPENSE"},
		{"revenue", AccountTypeKeyRevenue, "REVENUE"},
		{"liability", AccountTypeKeyLiability, "LIABILITY"},
		{"equity", AccountTypeKeyEquity, "EQUITY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

func TestAccountTemplateMetadataNotNil(t *testing.T) {
	allAccounts := AllAccountTemplates()

	for i, acc := range allAccounts {
		assert.NotNil(t, acc.Metadata, "account %d (%s) should have non-nil metadata", i, acc.Name)
	}
}

func TestAccountTemplatesHaveUniqueAliases(t *testing.T) {
	allAccounts := AllAccountTemplates()

	aliases := make(map[string]bool)

	for _, acc := range allAccounts {
		if acc.Alias != nil && *acc.Alias != "" {
			assert.False(t, aliases[*acc.Alias], "duplicate alias found: %s", *acc.Alias)
			aliases[*acc.Alias] = true
		}
	}
}

func TestAccountTemplatesAllActive(t *testing.T) {
	allAccounts := AllAccountTemplates()

	for i, acc := range allAccounts {
		assert.Equal(t, models.StatusActive, acc.Status.Code, "account %d (%s) should be active", i, acc.Name)
	}
}

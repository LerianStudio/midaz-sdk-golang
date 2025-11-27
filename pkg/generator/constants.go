package generator

// Account type keys define the standard account type identifiers used across the SDK.
// These constants ensure consistency and prevent magic string usage.
const (
	// AccountTypeKeyChecking represents a checking/deposit account type.
	AccountTypeKeyChecking = "CHECKING"

	// AccountTypeKeySavings represents a savings account type.
	AccountTypeKeySavings = "SAVINGS"

	// AccountTypeKeyCreditCard represents a credit card account type.
	AccountTypeKeyCreditCard = "CREDIT_CARD"

	// AccountTypeKeyExpense represents an expense account type.
	AccountTypeKeyExpense = "EXPENSE"

	// AccountTypeKeyRevenue represents a revenue account type.
	AccountTypeKeyRevenue = "REVENUE"

	// AccountTypeKeyLiability represents a liability account type.
	AccountTypeKeyLiability = "LIABILITY"

	// AccountTypeKeyEquity represents an equity account type.
	AccountTypeKeyEquity = "EQUITY"
)

// maxWorkers defines the upper limit for concurrent workers to prevent
// resource exhaustion in high-concurrency scenarios.
const maxWorkers = 100

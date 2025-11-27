package integrity

import (
	"context"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// -----------------------------------------------------------------------------
// Test Fixtures and Helpers
// -----------------------------------------------------------------------------

func ptr[T any](v T) *T {
	return &v
}

func createTestBalance(accountID, assetCode string, available, onHold int64) models.Balance {
	return models.Balance{
		ID:        "balance-" + accountID,
		AccountID: accountID,
		AssetCode: assetCode,
		Available: decimal.NewFromInt(available),
		OnHold:    decimal.NewFromInt(onHold),
	}
}

func createTestAccount(id string, alias *string) *models.Account {
	return &models.Account{
		ID:    id,
		Alias: alias,
	}
}

// mockObservabilityProvider implements observability.Provider for testing
type mockObservabilityProvider struct {
	enabled bool
	logger  *mockLogger
}

func newMockObservabilityProvider(enabled bool) *mockObservabilityProvider {
	return &mockObservabilityProvider{
		enabled: enabled,
		logger:  &mockLogger{},
	}
}

func (m *mockObservabilityProvider) Tracer() trace.Tracer {
	return noop.NewTracerProvider().Tracer("")
}

func (m *mockObservabilityProvider) Meter() metric.Meter {
	return nil
}

func (m *mockObservabilityProvider) Logger() observability.Logger {
	return m.logger
}

func (m *mockObservabilityProvider) Shutdown(ctx context.Context) error {
	return nil
}

func (m *mockObservabilityProvider) IsEnabled() bool {
	return m.enabled
}

// mockLogger implements observability.Logger for testing
type mockLogger struct {
	debugCalls []string
	infoCalls  []string
	warnCalls  []string
	errorCalls []string
}

func (l *mockLogger) Debug(args ...any)                                      { l.debugCalls = append(l.debugCalls, "debug") }
func (l *mockLogger) Debugf(format string, args ...any)                      { l.debugCalls = append(l.debugCalls, format) }
func (l *mockLogger) Info(args ...any)                                       { l.infoCalls = append(l.infoCalls, "info") }
func (l *mockLogger) Infof(format string, args ...any)                       { l.infoCalls = append(l.infoCalls, format) }
func (l *mockLogger) Warn(args ...any)                                       { l.warnCalls = append(l.warnCalls, "warn") }
func (l *mockLogger) Warnf(format string, args ...any)                       { l.warnCalls = append(l.warnCalls, format) }
func (l *mockLogger) Error(args ...any)                                      { l.errorCalls = append(l.errorCalls, "error") }
func (l *mockLogger) Errorf(format string, args ...any)                      { l.errorCalls = append(l.errorCalls, format) }
func (l *mockLogger) Fatal(args ...any)                                      {}
func (l *mockLogger) Fatalf(format string, args ...any)                      {}
func (l *mockLogger) With(fields map[string]any) observability.Logger        { return l }
func (l *mockLogger) WithContext(ctx trace.SpanContext) observability.Logger { return l }
func (l *mockLogger) WithSpan(span trace.Span) observability.Logger          { return l }

// -----------------------------------------------------------------------------
// Mock Services - Complete implementations of entities interfaces
// -----------------------------------------------------------------------------

// testBalancesService implements entities.BalancesService for testing
type testBalancesService struct {
	listBalancesFn               func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error)
	listAccountBalancesFn        func(ctx context.Context, orgID, ledgerID, accountID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error)
	getBalanceFn                 func(ctx context.Context, orgID, ledgerID, balanceID string) (*models.Balance, error)
	updateBalanceFn              func(ctx context.Context, orgID, ledgerID, balanceID string, input *models.UpdateBalanceInput) (*models.Balance, error)
	deleteBalanceFn              func(ctx context.Context, orgID, ledgerID, balanceID string) error
	createBalanceFn              func(ctx context.Context, orgID, ledgerID, accountID string, input *models.CreateBalanceInput) (*models.Balance, error)
	listBalancesByAccountAliasFn func(ctx context.Context, orgID, ledgerID, alias string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error)
	listBalancesByExternalCodeFn func(ctx context.Context, orgID, ledgerID, code string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error)
}

func (s *testBalancesService) ListBalances(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
	if s.listBalancesFn != nil {
		return s.listBalancesFn(ctx, orgID, ledgerID, opts)
	}
	return nil, nil
}

func (s *testBalancesService) ListAccountBalances(ctx context.Context, orgID, ledgerID, accountID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
	if s.listAccountBalancesFn != nil {
		return s.listAccountBalancesFn(ctx, orgID, ledgerID, accountID, opts)
	}
	return nil, nil
}

func (s *testBalancesService) GetBalance(ctx context.Context, orgID, ledgerID, balanceID string) (*models.Balance, error) {
	if s.getBalanceFn != nil {
		return s.getBalanceFn(ctx, orgID, ledgerID, balanceID)
	}
	return nil, nil
}

func (s *testBalancesService) UpdateBalance(ctx context.Context, orgID, ledgerID, balanceID string, input *models.UpdateBalanceInput) (*models.Balance, error) {
	if s.updateBalanceFn != nil {
		return s.updateBalanceFn(ctx, orgID, ledgerID, balanceID, input)
	}
	return nil, nil
}

func (s *testBalancesService) DeleteBalance(ctx context.Context, orgID, ledgerID, balanceID string) error {
	if s.deleteBalanceFn != nil {
		return s.deleteBalanceFn(ctx, orgID, ledgerID, balanceID)
	}
	return nil
}

func (s *testBalancesService) CreateBalance(ctx context.Context, orgID, ledgerID, accountID string, input *models.CreateBalanceInput) (*models.Balance, error) {
	if s.createBalanceFn != nil {
		return s.createBalanceFn(ctx, orgID, ledgerID, accountID, input)
	}
	return nil, nil
}

func (s *testBalancesService) ListBalancesByAccountAlias(ctx context.Context, orgID, ledgerID, alias string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
	if s.listBalancesByAccountAliasFn != nil {
		return s.listBalancesByAccountAliasFn(ctx, orgID, ledgerID, alias, opts)
	}
	return nil, nil
}

func (s *testBalancesService) ListBalancesByExternalCode(ctx context.Context, orgID, ledgerID, code string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
	if s.listBalancesByExternalCodeFn != nil {
		return s.listBalancesByExternalCodeFn(ctx, orgID, ledgerID, code, opts)
	}
	return nil, nil
}

// testAccountsService implements entities.AccountsService for testing
type testAccountsService struct {
	listAccountsFn              func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Account], error)
	getAccountFn                func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error)
	getAccountByAliasFn         func(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error)
	createAccountFn             func(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountInput) (*models.Account, error)
	updateAccountFn             func(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateAccountInput) (*models.Account, error)
	deleteAccountFn             func(ctx context.Context, orgID, ledgerID, id string) error
	getBalanceFn                func(ctx context.Context, orgID, ledgerID, accountID string) (*models.Balance, error)
	getAccountsMetricsCountFn   func(ctx context.Context, orgID, ledgerID string) (*models.MetricsCount, error)
	getExternalAccountFn        func(ctx context.Context, orgID, ledgerID, assetCode string) (*models.Account, error)
	getExternalAccountBalanceFn func(ctx context.Context, orgID, ledgerID, assetCode string) (*models.Balance, error)
	getAccountByAliasPathFn     func(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error)
}

func (s *testAccountsService) ListAccounts(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Account], error) {
	if s.listAccountsFn != nil {
		return s.listAccountsFn(ctx, orgID, ledgerID, opts)
	}
	return nil, nil
}

func (s *testAccountsService) GetAccount(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
	if s.getAccountFn != nil {
		return s.getAccountFn(ctx, orgID, ledgerID, id)
	}
	return nil, nil
}

func (s *testAccountsService) GetAccountByAlias(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error) {
	if s.getAccountByAliasFn != nil {
		return s.getAccountByAliasFn(ctx, orgID, ledgerID, alias)
	}
	return nil, nil
}

func (s *testAccountsService) CreateAccount(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountInput) (*models.Account, error) {
	if s.createAccountFn != nil {
		return s.createAccountFn(ctx, orgID, ledgerID, input)
	}
	return nil, nil
}

func (s *testAccountsService) UpdateAccount(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateAccountInput) (*models.Account, error) {
	if s.updateAccountFn != nil {
		return s.updateAccountFn(ctx, orgID, ledgerID, id, input)
	}
	return nil, nil
}

func (s *testAccountsService) DeleteAccount(ctx context.Context, orgID, ledgerID, id string) error {
	if s.deleteAccountFn != nil {
		return s.deleteAccountFn(ctx, orgID, ledgerID, id)
	}
	return nil
}

func (s *testAccountsService) GetBalance(ctx context.Context, orgID, ledgerID, accountID string) (*models.Balance, error) {
	if s.getBalanceFn != nil {
		return s.getBalanceFn(ctx, orgID, ledgerID, accountID)
	}
	return nil, nil
}

func (s *testAccountsService) GetAccountsMetricsCount(ctx context.Context, orgID, ledgerID string) (*models.MetricsCount, error) {
	if s.getAccountsMetricsCountFn != nil {
		return s.getAccountsMetricsCountFn(ctx, orgID, ledgerID)
	}
	return nil, nil
}

func (s *testAccountsService) GetExternalAccount(ctx context.Context, orgID, ledgerID, assetCode string) (*models.Account, error) {
	if s.getExternalAccountFn != nil {
		return s.getExternalAccountFn(ctx, orgID, ledgerID, assetCode)
	}
	return nil, nil
}

func (s *testAccountsService) GetExternalAccountBalance(ctx context.Context, orgID, ledgerID, assetCode string) (*models.Balance, error) {
	if s.getExternalAccountBalanceFn != nil {
		return s.getExternalAccountBalanceFn(ctx, orgID, ledgerID, assetCode)
	}
	return nil, nil
}

func (s *testAccountsService) GetAccountByAliasPath(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error) {
	if s.getAccountByAliasPathFn != nil {
		return s.getAccountByAliasPathFn(ctx, orgID, ledgerID, alias)
	}
	return nil, nil
}

// Test error variables
var (
	errNetworkError    = errorf("network error")
	errAccountNotFound = errorf("account not found")
)

func errorf(format string, args ...any) error {
	return &testError{msg: format}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// -----------------------------------------------------------------------------
// NewChecker Tests
// -----------------------------------------------------------------------------

func TestNewChecker(t *testing.T) {
	entity := &entities.Entity{}
	checker := NewChecker(entity)

	assert.NotNil(t, checker)
	assert.Equal(t, entity, checker.e)
	assert.Equal(t, time.Duration(0), checker.sleepBetweenAccountLookups)
	assert.Nil(t, checker.obs)
}

func TestNewChecker_NilEntity(t *testing.T) {
	checker := NewChecker(nil)

	assert.NotNil(t, checker)
	assert.Nil(t, checker.e)
}

// -----------------------------------------------------------------------------
// WithAccountLookupDelay Tests
// -----------------------------------------------------------------------------

func TestCheckerWithAccountLookupDelay(t *testing.T) {
	tests := []struct {
		name     string
		delay    time.Duration
		expected time.Duration
	}{
		{
			name:     "valid delay",
			delay:    2 * time.Second,
			expected: 2 * time.Second,
		},
		{
			name:     "negative delay clamped to zero",
			delay:    -1 * time.Second,
			expected: 0,
		},
		{
			name:     "excessive delay clamped to max",
			delay:    10 * time.Second,
			expected: maxAccountLookupDelay,
		},
		{
			name:     "zero delay",
			delay:    0,
			expected: 0,
		},
		{
			name:     "max allowed delay",
			delay:    maxAccountLookupDelay,
			expected: maxAccountLookupDelay,
		},
		{
			name:     "just below max",
			delay:    maxAccountLookupDelay - time.Millisecond,
			expected: maxAccountLookupDelay - time.Millisecond,
		},
		{
			name:     "just above max",
			delay:    maxAccountLookupDelay + time.Millisecond,
			expected: maxAccountLookupDelay,
		},
		{
			name:     "very large negative",
			delay:    -100 * time.Second,
			expected: 0,
		},
		{
			name:     "very large positive",
			delay:    100 * time.Second,
			expected: maxAccountLookupDelay,
		},
		{
			name:     "milliseconds",
			delay:    500 * time.Millisecond,
			expected: 500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &entities.Entity{}
			checker := NewChecker(entity)

			result := checker.WithAccountLookupDelay(tt.delay)

			// Should return the same checker instance for chaining
			assert.Equal(t, checker, result)
			assert.Equal(t, tt.expected, checker.sleepBetweenAccountLookups)
		})
	}
}

// -----------------------------------------------------------------------------
// WithObservability Tests
// -----------------------------------------------------------------------------

func TestCheckerWithObservability(t *testing.T) {
	entity := &entities.Entity{}
	checker := NewChecker(entity)
	obs := newMockObservabilityProvider(true)

	result := checker.WithObservability(obs)

	assert.Equal(t, checker, result)
	assert.NotNil(t, checker.obs)
}

func TestCheckerWithObservability_Nil(t *testing.T) {
	entity := &entities.Entity{}
	checker := NewChecker(entity)

	result := checker.WithObservability(nil)

	assert.Equal(t, checker, result)
	assert.Nil(t, checker.obs)
}

// -----------------------------------------------------------------------------
// GenerateLedgerReport Tests - Entity Initialization
// -----------------------------------------------------------------------------

func TestGenerateLedgerReport_NilEntity(t *testing.T) {
	checker := &Checker{e: nil}

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	assert.Nil(t, report)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "entities not initialized")
}

func TestGenerateLedgerReport_NilBalancesService(t *testing.T) {
	checker := &Checker{
		e: &entities.Entity{
			Accounts: nil,
			Balances: nil,
		},
	}

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	assert.Nil(t, report)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "entities not initialized")
}

func TestGenerateLedgerReport_NilAccountsService(t *testing.T) {
	mockBalances := &testBalancesService{}

	checker := &Checker{
		e: &entities.Entity{
			Accounts: nil,
			Balances: mockBalances,
		},
	}

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	assert.Nil(t, report)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "entities not initialized")
}

// -----------------------------------------------------------------------------
// GenerateLedgerReport Tests - Successful Scenarios
// -----------------------------------------------------------------------------

func TestGenerateLedgerReport_EmptyLedger(t *testing.T) {
	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      []models.Balance{},
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}
	mockAccounts := &testAccountsService{}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "ledger-1", report.LedgerID)
	assert.Empty(t, report.TotalsByAsset)
}

func TestGenerateLedgerReport_SingleBalance(t *testing.T) {
	balance := createTestBalance("account-1", "USD", 1000, 100)

	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      []models.Balance{balance},
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			return createTestAccount("account-1", ptr("@user/account1")), nil
		},
	}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "ledger-1", report.LedgerID)
	assert.Len(t, report.TotalsByAsset, 1)

	usdTotals := report.TotalsByAsset["USD"]
	require.NotNil(t, usdTotals)
	assert.Equal(t, "USD", usdTotals.Asset)
	assert.Equal(t, 1, usdTotals.Accounts)
	assert.True(t, usdTotals.TotalAvailable.Equal(decimal.NewFromInt(1000)))
	assert.True(t, usdTotals.TotalOnHold.Equal(decimal.NewFromInt(100)))
	assert.True(t, usdTotals.InternalNetTotal.Equal(decimal.NewFromInt(1100))) // 1000 + 100
	assert.Empty(t, usdTotals.Overdrawn)
}

func TestGenerateLedgerReport_MultipleBalancesSameAsset(t *testing.T) {
	balances := []models.Balance{
		createTestBalance("account-1", "USD", 1000, 100),
		createTestBalance("account-2", "USD", 2000, 200),
		createTestBalance("account-3", "USD", 3000, 300),
	}

	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      balances,
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}

	accountAliases := map[string]string{
		"account-1": "@user/account1",
		"account-2": "@user/account2",
		"account-3": "@user/account3",
	}
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			alias := accountAliases[id]
			return createTestAccount(id, ptr(alias)), nil
		},
	}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)

	usdTotals := report.TotalsByAsset["USD"]
	require.NotNil(t, usdTotals)
	assert.Equal(t, 3, usdTotals.Accounts)
	assert.True(t, usdTotals.TotalAvailable.Equal(decimal.NewFromInt(6000))) // 1000+2000+3000
	assert.True(t, usdTotals.TotalOnHold.Equal(decimal.NewFromInt(600)))     // 100+200+300
	assert.True(t, usdTotals.InternalNetTotal.Equal(decimal.NewFromInt(6600)))
}

func TestGenerateLedgerReport_MultipleAssets(t *testing.T) {
	balances := []models.Balance{
		createTestBalance("account-1", "USD", 1000, 100),
		createTestBalance("account-2", "EUR", 2000, 200),
		createTestBalance("account-3", "BTC", 3000, 300),
	}

	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      balances,
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			return createTestAccount("account", ptr("@user/account")), nil
		},
	}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Len(t, report.TotalsByAsset, 3)
	assert.Contains(t, report.TotalsByAsset, "USD")
	assert.Contains(t, report.TotalsByAsset, "EUR")
	assert.Contains(t, report.TotalsByAsset, "BTC")
}

func TestGenerateLedgerReport_ExternalAccount(t *testing.T) {
	balances := []models.Balance{
		createTestBalance("account-1", "USD", 1000, 100),
		createTestBalance("external-1", "USD", -1000, -100), // External account (debit side)
	}

	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      balances,
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}

	accountAliases := map[string]string{
		"account-1":  "@user/internal",
		"external-1": "@external/bank",
	}
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			alias := accountAliases[id]
			return createTestAccount(id, ptr(alias)), nil
		},
	}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)

	usdTotals := report.TotalsByAsset["USD"]
	require.NotNil(t, usdTotals)
	assert.Equal(t, 2, usdTotals.Accounts)
	// External account should NOT be included in internal net total
	assert.True(t, usdTotals.InternalNetTotal.Equal(decimal.NewFromInt(1100))) // Only internal account
	// External account has negative balance, should be in overdrawn
	assert.Len(t, usdTotals.Overdrawn, 1)
	assert.Contains(t, usdTotals.Overdrawn, "@external/bank")
}

func TestGenerateLedgerReport_OverdrawnAccount(t *testing.T) {
	balances := []models.Balance{
		createTestBalance("account-1", "USD", -500, 0), // Overdrawn
		createTestBalance("account-2", "USD", 1000, 100),
	}

	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      balances,
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}

	accountAliases := map[string]string{
		"account-1": "@user/overdrawn",
		"account-2": "@user/healthy",
	}
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			alias := accountAliases[id]
			return createTestAccount(id, ptr(alias)), nil
		},
	}

	obs := newMockObservabilityProvider(true)
	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	}).WithObservability(obs)

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)

	usdTotals := report.TotalsByAsset["USD"]
	assert.Len(t, usdTotals.Overdrawn, 1)
	assert.Contains(t, usdTotals.Overdrawn, "@user/overdrawn")

	// Verify warning was logged
	assert.NotEmpty(t, obs.logger.warnCalls)
}

func TestGenerateLedgerReport_AccountWithNoAlias(t *testing.T) {
	balances := []models.Balance{
		createTestBalance("account-1", "USD", -500, 0), // Overdrawn, no alias
	}

	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      balances,
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			return createTestAccount("account-1", nil), nil
		},
	}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)

	usdTotals := report.TotalsByAsset["USD"]
	// When no alias, should use account ID
	assert.Contains(t, usdTotals.Overdrawn, "account-1")
}

func TestGenerateLedgerReport_AccountWithEmptyAlias(t *testing.T) {
	balances := []models.Balance{
		createTestBalance("account-1", "USD", -500, 0),
	}

	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      balances,
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			return createTestAccount("account-1", ptr("")), nil
		},
	}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)

	usdTotals := report.TotalsByAsset["USD"]
	// When alias is empty string, should use account ID
	assert.Contains(t, usdTotals.Overdrawn, "account-1")
}

// -----------------------------------------------------------------------------
// GenerateLedgerReport Tests - Pagination
// -----------------------------------------------------------------------------

func TestGenerateLedgerReport_Pagination(t *testing.T) {
	callCount := 0
	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			callCount++
			if callCount == 1 {
				return &models.ListResponse[models.Balance]{
					Items: []models.Balance{
						createTestBalance("account-1", "USD", 1000, 100),
					},
					Pagination: models.Pagination{NextCursor: "cursor-1"},
				}, nil
			}
			return &models.ListResponse[models.Balance]{
				Items: []models.Balance{
					createTestBalance("account-2", "USD", 2000, 200),
				},
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			return createTestAccount("account", ptr("@user/account")), nil
		},
	}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)

	usdTotals := report.TotalsByAsset["USD"]
	assert.Equal(t, 2, usdTotals.Accounts)
	assert.True(t, usdTotals.TotalAvailable.Equal(decimal.NewFromInt(3000)))
}

// -----------------------------------------------------------------------------
// GenerateLedgerReport Tests - Error Scenarios
// -----------------------------------------------------------------------------

func TestGenerateLedgerReport_ListBalancesError(t *testing.T) {
	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return nil, errNetworkError
		},
	}
	mockAccounts := &testAccountsService{}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	assert.Nil(t, report)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}

func TestGenerateLedgerReport_GetAccountError(t *testing.T) {
	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      []models.Balance{createTestBalance("account-1", "USD", 1000, 100)},
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			return nil, errAccountNotFound
		},
	}

	obs := newMockObservabilityProvider(true)
	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	}).WithObservability(obs)

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	assert.Nil(t, report)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account-1")

	// Verify error was logged
	assert.NotEmpty(t, obs.logger.errorCalls)
}

// -----------------------------------------------------------------------------
// GenerateLedgerReport Tests - Context Cancellation
// -----------------------------------------------------------------------------

func TestGenerateLedgerReport_ContextCancellation(t *testing.T) {
	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      []models.Balance{createTestBalance("account-1", "USD", 1000, 100)},
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}
	mockAccounts := &testAccountsService{}

	// Setup account lookup delay
	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	}).WithAccountLookupDelay(5 * time.Second) // Long delay

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report, err := checker.GenerateLedgerReport(ctx, "org-1", "ledger-1")

	assert.Nil(t, report)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestGenerateLedgerReport_ContextTimeout(t *testing.T) {
	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      []models.Balance{createTestBalance("account-1", "USD", 1000, 100)},
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}
	mockAccounts := &testAccountsService{}

	// Setup account lookup delay longer than context timeout
	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	}).WithAccountLookupDelay(5 * time.Second)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	report, err := checker.GenerateLedgerReport(ctx, "org-1", "ledger-1")

	assert.Nil(t, report)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

// -----------------------------------------------------------------------------
// GenerateLedgerReport Tests - Account Caching
// -----------------------------------------------------------------------------

func TestGenerateLedgerReport_AccountCaching(t *testing.T) {
	// Same account appears multiple times in balances
	balances := []models.Balance{
		createTestBalance("account-1", "USD", 1000, 100),
		createTestBalance("account-1", "EUR", 2000, 200), // Same account, different asset
	}

	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      balances,
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}

	// GetAccount should only be called once due to caching
	getAccountCallCount := 0
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			getAccountCallCount++
			return createTestAccount("account-1", ptr("@user/account1")), nil
		},
	}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Len(t, report.TotalsByAsset, 2)
	assert.Equal(t, 1, getAccountCallCount) // Should only be called once due to caching
}

// -----------------------------------------------------------------------------
// Report.ToSummaryMap Tests
// -----------------------------------------------------------------------------

func TestReportToSummaryMap_Empty(t *testing.T) {
	report := &Report{
		LedgerID:      "ledger-1",
		TotalsByAsset: map[string]*BalanceTotals{},
	}

	summary := report.ToSummaryMap()

	assert.NotNil(t, summary)
	assert.Empty(t, summary)
}

func TestReportToSummaryMap_SingleAsset(t *testing.T) {
	report := &Report{
		LedgerID: "ledger-1",
		TotalsByAsset: map[string]*BalanceTotals{
			"USD": {
				Asset:            "USD",
				Accounts:         5,
				TotalAvailable:   decimal.NewFromInt(10000),
				TotalOnHold:      decimal.NewFromInt(1000),
				InternalNetTotal: decimal.Zero, // Balanced
				Overdrawn:        []string{},
			},
		},
	}

	summary := report.ToSummaryMap()

	assert.Len(t, summary, 1)
	usd := summary["USD"]
	assert.Equal(t, 5, usd["accounts"])
	assert.Equal(t, "10000", usd["totalAvailable"])
	assert.Equal(t, "1000", usd["totalOnHold"])
	assert.Equal(t, "0", usd["internalNetTotal"])
	assert.Equal(t, true, usd["doubleEntryBalanced"])
	assert.Empty(t, usd["overdrawnAccounts"])
}

func TestReportToSummaryMap_MultipleAssets(t *testing.T) {
	report := &Report{
		LedgerID: "ledger-1",
		TotalsByAsset: map[string]*BalanceTotals{
			"USD": {
				Asset:            "USD",
				Accounts:         3,
				TotalAvailable:   decimal.NewFromInt(5000),
				TotalOnHold:      decimal.NewFromInt(500),
				InternalNetTotal: decimal.NewFromInt(100), // Not balanced
				Overdrawn:        []string{"account-1"},
			},
			"EUR": {
				Asset:            "EUR",
				Accounts:         2,
				TotalAvailable:   decimal.NewFromInt(2000),
				TotalOnHold:      decimal.NewFromInt(200),
				InternalNetTotal: decimal.Zero, // Balanced
				Overdrawn:        []string{},
			},
		},
	}

	summary := report.ToSummaryMap()

	assert.Len(t, summary, 2)

	// USD assertions
	usd := summary["USD"]
	assert.Equal(t, 3, usd["accounts"])
	assert.Equal(t, false, usd["doubleEntryBalanced"]) // Non-zero internal net
	assert.Equal(t, []string{"account-1"}, usd["overdrawnAccounts"])

	// EUR assertions
	eur := summary["EUR"]
	assert.Equal(t, 2, eur["accounts"])
	assert.Equal(t, true, eur["doubleEntryBalanced"])
	assert.Empty(t, eur["overdrawnAccounts"])
}

func TestReportToSummaryMap_DecimalPrecision(t *testing.T) {
	report := &Report{
		LedgerID: "ledger-1",
		TotalsByAsset: map[string]*BalanceTotals{
			"BTC": {
				Asset:            "BTC",
				Accounts:         1,
				TotalAvailable:   decimal.NewFromFloat(0.12345678),
				TotalOnHold:      decimal.NewFromFloat(0.00000001),
				InternalNetTotal: decimal.NewFromFloat(0.12345679),
				Overdrawn:        nil,
			},
		},
	}

	summary := report.ToSummaryMap()

	btc := summary["BTC"]
	assert.Equal(t, "0.12345678", btc["totalAvailable"])
	assert.Equal(t, "0.00000001", btc["totalOnHold"])
	assert.Equal(t, "0.12345679", btc["internalNetTotal"])
}

func TestReportToSummaryMap_NilOverdrawn(t *testing.T) {
	report := &Report{
		LedgerID: "ledger-1",
		TotalsByAsset: map[string]*BalanceTotals{
			"USD": {
				Asset:            "USD",
				Accounts:         1,
				TotalAvailable:   decimal.NewFromInt(1000),
				TotalOnHold:      decimal.NewFromInt(0),
				InternalNetTotal: decimal.Zero,
				Overdrawn:        nil, // nil slice
			},
		},
	}

	summary := report.ToSummaryMap()

	usd := summary["USD"]
	assert.Nil(t, usd["overdrawnAccounts"])
}

// -----------------------------------------------------------------------------
// BalanceTotals Type Tests
// -----------------------------------------------------------------------------

func TestBalanceTotalsType(t *testing.T) {
	totals := &BalanceTotals{
		Asset:            "USD",
		Accounts:         5,
		TotalAvailable:   decimal.NewFromInt(1000),
		TotalOnHold:      decimal.NewFromInt(100),
		InternalNetTotal: decimal.NewFromInt(900),
		Overdrawn:        []string{"account-1", "account-2"},
	}

	assert.Equal(t, "USD", totals.Asset)
	assert.Equal(t, 5, totals.Accounts)
	assert.True(t, totals.TotalAvailable.Equal(decimal.NewFromInt(1000)))
	assert.True(t, totals.TotalOnHold.Equal(decimal.NewFromInt(100)))
	assert.True(t, totals.InternalNetTotal.Equal(decimal.NewFromInt(900)))
	assert.Len(t, totals.Overdrawn, 2)
	assert.Contains(t, totals.Overdrawn, "account-1")
	assert.Contains(t, totals.Overdrawn, "account-2")
}

func TestBalanceTotalsDecimalOperations(t *testing.T) {
	totals := &BalanceTotals{
		TotalAvailable:   decimal.NewFromFloat(0.12345678),
		TotalOnHold:      decimal.NewFromFloat(0.00000001),
		InternalNetTotal: decimal.NewFromFloat(0.12345679),
	}

	assert.True(t, totals.TotalAvailable.Equal(decimal.NewFromFloat(0.12345678)))
	assert.True(t, totals.TotalOnHold.Equal(decimal.NewFromFloat(0.00000001)))

	sum := totals.TotalAvailable.Add(totals.TotalOnHold)
	assert.True(t, sum.Equal(totals.InternalNetTotal))
}

func TestBalanceTotalsWithNegativeValues(t *testing.T) {
	totals := &BalanceTotals{
		Asset:            "USD",
		Accounts:         2,
		TotalAvailable:   decimal.NewFromInt(-500),
		TotalOnHold:      decimal.NewFromInt(100),
		InternalNetTotal: decimal.NewFromInt(-400),
		Overdrawn:        []string{"account-1", "account-2"},
	}

	assert.Equal(t, "USD", totals.Asset)
	assert.Equal(t, 2, totals.Accounts)
	assert.True(t, totals.TotalAvailable.IsNegative())
	assert.True(t, totals.TotalOnHold.Equal(decimal.NewFromInt(100)))
	assert.True(t, totals.InternalNetTotal.IsNegative())
	assert.Len(t, totals.Overdrawn, 2)
}

// -----------------------------------------------------------------------------
// Report Type Tests
// -----------------------------------------------------------------------------

func TestReportType(t *testing.T) {
	totals := map[string]*BalanceTotals{
		"USD": {
			Asset:            "USD",
			Accounts:         3,
			TotalAvailable:   decimal.NewFromInt(1500),
			TotalOnHold:      decimal.NewFromInt(150),
			InternalNetTotal: decimal.NewFromInt(1350),
			Overdrawn:        []string{},
		},
		"EUR": {
			Asset:            "EUR",
			Accounts:         2,
			TotalAvailable:   decimal.NewFromInt(800),
			TotalOnHold:      decimal.NewFromInt(80),
			InternalNetTotal: decimal.NewFromInt(720),
			Overdrawn:        []string{"account-3"},
		},
	}

	report := &Report{
		LedgerID:      "ledger-123",
		TotalsByAsset: totals,
	}

	assert.Equal(t, "ledger-123", report.LedgerID)
	assert.Len(t, report.TotalsByAsset, 2)
	assert.Contains(t, report.TotalsByAsset, "USD")
	assert.Contains(t, report.TotalsByAsset, "EUR")

	usdTotals := report.TotalsByAsset["USD"]
	assert.Equal(t, "USD", usdTotals.Asset)
	assert.Equal(t, 3, usdTotals.Accounts)

	eurTotals := report.TotalsByAsset["EUR"]
	assert.Equal(t, "EUR", eurTotals.Asset)
	assert.Equal(t, 2, eurTotals.Accounts)
	assert.Len(t, eurTotals.Overdrawn, 1)
}

func TestReportWithMultipleAssets(t *testing.T) {
	assets := []string{"USD", "EUR", "BTC", "POINTS"}
	report := &Report{
		LedgerID:      "multi-asset-ledger",
		TotalsByAsset: make(map[string]*BalanceTotals),
	}

	for i, asset := range assets {
		report.TotalsByAsset[asset] = &BalanceTotals{
			Asset:            asset,
			Accounts:         i + 1,
			TotalAvailable:   decimal.NewFromInt(int64((i + 1) * 1000)),
			TotalOnHold:      decimal.NewFromInt(int64((i + 1) * 100)),
			InternalNetTotal: decimal.NewFromInt(int64((i + 1) * 1100)),
			Overdrawn:        []string{},
		}
	}

	assert.Equal(t, "multi-asset-ledger", report.LedgerID)
	assert.Len(t, report.TotalsByAsset, len(assets))

	for i, asset := range assets {
		totals, exists := report.TotalsByAsset[asset]
		assert.True(t, exists, "Asset %s should exist in report", asset)
		assert.Equal(t, asset, totals.Asset)
		assert.Equal(t, i+1, totals.Accounts)
		assert.True(t, totals.TotalAvailable.Equal(decimal.NewFromInt(int64((i+1)*1000))))
	}
}

// -----------------------------------------------------------------------------
// Method Chaining Tests
// -----------------------------------------------------------------------------

func TestCheckerChaining(t *testing.T) {
	entity := &entities.Entity{}
	obs := newMockObservabilityProvider(true)

	checker := NewChecker(entity).
		WithAccountLookupDelay(1 * time.Second).
		WithObservability(obs).
		WithAccountLookupDelay(2 * time.Second)

	assert.NotNil(t, checker)
	assert.Equal(t, 2*time.Second, checker.sleepBetweenAccountLookups)
	assert.NotNil(t, checker.obs)
}

func TestCheckerChainingReturnsSameInstance(t *testing.T) {
	entity := &entities.Entity{}
	checker := NewChecker(entity)

	result1 := checker.WithAccountLookupDelay(1 * time.Second)
	result2 := result1.WithAccountLookupDelay(2 * time.Second)

	assert.Equal(t, checker, result1)
	assert.Equal(t, checker, result2)
	assert.Equal(t, result1, result2)
	assert.Equal(t, 2*time.Second, checker.sleepBetweenAccountLookups)
}

// -----------------------------------------------------------------------------
// Constant Tests
// -----------------------------------------------------------------------------

func TestMaxAccountLookupDelay(t *testing.T) {
	expectedMax := 5 * time.Second
	assert.Equal(t, expectedMax, maxAccountLookupDelay)
}

// -----------------------------------------------------------------------------
// waitForThrottling Tests
// -----------------------------------------------------------------------------

func TestWaitForThrottling_NoDelay(t *testing.T) {
	checker := &Checker{sleepBetweenAccountLookups: 0}

	start := time.Now()
	err := checker.waitForThrottling(context.Background())
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, elapsed, 10*time.Millisecond) // Should be instant
}

func TestWaitForThrottling_WithDelay(t *testing.T) {
	checker := &Checker{sleepBetweenAccountLookups: 50 * time.Millisecond}

	start := time.Now()
	err := checker.waitForThrottling(context.Background())
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
	assert.Less(t, elapsed, 100*time.Millisecond) // Allow some tolerance
}

func TestWaitForThrottling_ContextCancelled(t *testing.T) {
	checker := &Checker{sleepBetweenAccountLookups: 5 * time.Second}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := checker.waitForThrottling(ctx)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestWaitForThrottling_ContextCancelledDuringWait(t *testing.T) {
	checker := &Checker{sleepBetweenAccountLookups: 5 * time.Second}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := checker.waitForThrottling(ctx)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Less(t, elapsed, 100*time.Millisecond) // Should not wait full 5 seconds
}

// -----------------------------------------------------------------------------
// getOrCreateBalanceTotals Tests
// -----------------------------------------------------------------------------

func TestGetOrCreateBalanceTotals_New(t *testing.T) {
	checker := &Checker{}
	totals := map[string]*BalanceTotals{}

	result := checker.getOrCreateBalanceTotals(totals, "USD")

	assert.NotNil(t, result)
	assert.Equal(t, "USD", result.Asset)
	assert.True(t, result.TotalAvailable.Equal(decimal.Zero))
	assert.True(t, result.TotalOnHold.Equal(decimal.Zero))
	assert.True(t, result.InternalNetTotal.Equal(decimal.Zero))
	assert.Len(t, totals, 1)
}

func TestGetOrCreateBalanceTotals_Existing(t *testing.T) {
	checker := &Checker{}
	existing := &BalanceTotals{
		Asset:          "USD",
		Accounts:       5,
		TotalAvailable: decimal.NewFromInt(1000),
	}
	totals := map[string]*BalanceTotals{"USD": existing}

	result := checker.getOrCreateBalanceTotals(totals, "USD")

	assert.Equal(t, existing, result)
	assert.Equal(t, 5, result.Accounts)
	assert.True(t, result.TotalAvailable.Equal(decimal.NewFromInt(1000)))
	assert.Len(t, totals, 1)
}

// -----------------------------------------------------------------------------
// updateBalanceTotals Tests
// -----------------------------------------------------------------------------

func TestUpdateBalanceTotals(t *testing.T) {
	checker := &Checker{}
	totals := &BalanceTotals{
		Asset:          "USD",
		Accounts:       0,
		TotalAvailable: decimal.Zero,
		TotalOnHold:    decimal.Zero,
	}
	balance := models.Balance{
		Available: decimal.NewFromInt(1000),
		OnHold:    decimal.NewFromInt(100),
	}

	checker.updateBalanceTotals(totals, balance)

	assert.Equal(t, 1, totals.Accounts)
	assert.True(t, totals.TotalAvailable.Equal(decimal.NewFromInt(1000)))
	assert.True(t, totals.TotalOnHold.Equal(decimal.NewFromInt(100)))
}

func TestUpdateBalanceTotals_Accumulation(t *testing.T) {
	checker := &Checker{}
	totals := &BalanceTotals{
		Asset:          "USD",
		Accounts:       2,
		TotalAvailable: decimal.NewFromInt(2000),
		TotalOnHold:    decimal.NewFromInt(200),
	}
	balance := models.Balance{
		Available: decimal.NewFromInt(1000),
		OnHold:    decimal.NewFromInt(100),
	}

	checker.updateBalanceTotals(totals, balance)

	assert.Equal(t, 3, totals.Accounts)
	assert.True(t, totals.TotalAvailable.Equal(decimal.NewFromInt(3000)))
	assert.True(t, totals.TotalOnHold.Equal(decimal.NewFromInt(300)))
}

// -----------------------------------------------------------------------------
// updateInternalNetTotal Tests
// -----------------------------------------------------------------------------

func TestUpdateInternalNetTotal_InternalAccount(t *testing.T) {
	checker := &Checker{}
	totals := &BalanceTotals{InternalNetTotal: decimal.Zero}
	balance := models.Balance{
		Available: decimal.NewFromInt(1000),
		OnHold:    decimal.NewFromInt(100),
	}

	checker.updateInternalNetTotal(totals, balance, "@user/internal")

	assert.True(t, totals.InternalNetTotal.Equal(decimal.NewFromInt(1100)))
}

func TestUpdateInternalNetTotal_ExternalAccount(t *testing.T) {
	checker := &Checker{}
	totals := &BalanceTotals{InternalNetTotal: decimal.NewFromInt(1000)}
	balance := models.Balance{
		Available: decimal.NewFromInt(500),
		OnHold:    decimal.NewFromInt(50),
	}

	checker.updateInternalNetTotal(totals, balance, "@external/bank")

	// External accounts should NOT be included
	assert.True(t, totals.InternalNetTotal.Equal(decimal.NewFromInt(1000)))
}

func TestUpdateInternalNetTotal_EmptyAlias(t *testing.T) {
	checker := &Checker{}
	totals := &BalanceTotals{InternalNetTotal: decimal.Zero}
	balance := models.Balance{
		Available: decimal.NewFromInt(1000),
		OnHold:    decimal.NewFromInt(100),
	}

	checker.updateInternalNetTotal(totals, balance, "")

	// Empty alias should be treated as internal
	assert.True(t, totals.InternalNetTotal.Equal(decimal.NewFromInt(1100)))
}

func TestUpdateInternalNetTotal_ExternalPrefix(t *testing.T) {
	tests := []struct {
		alias    string
		expected bool // true if should be included (internal)
	}{
		{"@external/bank", false},
		{"@external/", false},
		{"@external", true}, // Does not have the slash
		{"@internal/account", true},
		{"external/account", true}, // Missing @
		{"@user/account", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			checker := &Checker{}
			totals := &BalanceTotals{InternalNetTotal: decimal.Zero}
			balance := models.Balance{
				Available: decimal.NewFromInt(100),
				OnHold:    decimal.NewFromInt(10),
			}

			checker.updateInternalNetTotal(totals, balance, tt.alias)

			if tt.expected {
				assert.True(t, totals.InternalNetTotal.Equal(decimal.NewFromInt(110)))
			} else {
				assert.True(t, totals.InternalNetTotal.Equal(decimal.Zero))
			}
		})
	}
}

// -----------------------------------------------------------------------------
// checkForOverdraft Tests
// -----------------------------------------------------------------------------

func TestCheckForOverdraft_NotOverdrawn(t *testing.T) {
	checker := &Checker{}
	totals := &BalanceTotals{Overdrawn: []string{}}
	balance := models.Balance{
		AccountID: "account-1",
		AssetCode: "USD",
		Available: decimal.NewFromInt(1000),
	}

	checker.checkForOverdraft(totals, balance, "@user/account")

	assert.Empty(t, totals.Overdrawn)
}

func TestCheckForOverdraft_Overdrawn_WithAlias(t *testing.T) {
	checker := &Checker{}
	totals := &BalanceTotals{Overdrawn: []string{}}
	balance := models.Balance{
		AccountID: "account-1",
		AssetCode: "USD",
		Available: decimal.NewFromInt(-500),
	}

	checker.checkForOverdraft(totals, balance, "@user/overdrawn")

	assert.Len(t, totals.Overdrawn, 1)
	assert.Contains(t, totals.Overdrawn, "@user/overdrawn")
}

func TestCheckForOverdraft_Overdrawn_NoAlias(t *testing.T) {
	checker := &Checker{}
	totals := &BalanceTotals{Overdrawn: []string{}}
	balance := models.Balance{
		AccountID: "account-123",
		AssetCode: "USD",
		Available: decimal.NewFromInt(-500),
	}

	checker.checkForOverdraft(totals, balance, "")

	assert.Len(t, totals.Overdrawn, 1)
	assert.Contains(t, totals.Overdrawn, "account-123")
}

func TestCheckForOverdraft_ZeroBalance(t *testing.T) {
	checker := &Checker{}
	totals := &BalanceTotals{Overdrawn: []string{}}
	balance := models.Balance{
		AccountID: "account-1",
		AssetCode: "USD",
		Available: decimal.Zero,
	}

	checker.checkForOverdraft(totals, balance, "@user/account")

	assert.Empty(t, totals.Overdrawn)
}

func TestCheckForOverdraft_WithObservability(t *testing.T) {
	obs := newMockObservabilityProvider(true)
	checker := &Checker{obs: obs}
	totals := &BalanceTotals{Overdrawn: []string{}}
	balance := models.Balance{
		AccountID: "account-1",
		AssetCode: "USD",
		Available: decimal.NewFromInt(-500),
	}

	checker.checkForOverdraft(totals, balance, "@user/overdrawn")

	assert.NotEmpty(t, obs.logger.warnCalls)
}

// -----------------------------------------------------------------------------
// Logging Tests
// -----------------------------------------------------------------------------

func TestLogDebug_Enabled(t *testing.T) {
	obs := newMockObservabilityProvider(true)
	checker := &Checker{obs: obs}

	checker.logDebug("test message %s", "arg")

	assert.NotEmpty(t, obs.logger.debugCalls)
}

func TestLogDebug_Disabled(t *testing.T) {
	obs := newMockObservabilityProvider(false)
	checker := &Checker{obs: obs}

	checker.logDebug("test message")

	assert.Empty(t, obs.logger.debugCalls)
}

func TestLogDebug_NilObservability(t *testing.T) {
	checker := &Checker{obs: nil}

	// Should not panic
	checker.logDebug("test message")
}

func TestLogInfo_Enabled(t *testing.T) {
	obs := newMockObservabilityProvider(true)
	checker := &Checker{obs: obs}

	checker.logInfo("test message %s", "arg")

	assert.NotEmpty(t, obs.logger.infoCalls)
}

func TestLogWarn_Enabled(t *testing.T) {
	obs := newMockObservabilityProvider(true)
	checker := &Checker{obs: obs}

	checker.logWarn("test message %s", "arg")

	assert.NotEmpty(t, obs.logger.warnCalls)
}

func TestLogError_Enabled(t *testing.T) {
	obs := newMockObservabilityProvider(true)
	checker := &Checker{obs: obs}

	checker.logError("test message %s", "arg")

	assert.NotEmpty(t, obs.logger.errorCalls)
}

// -----------------------------------------------------------------------------
// Edge Cases and Boundary Tests
// -----------------------------------------------------------------------------

func TestGenerateLedgerReport_LargeNumberOfBalances(t *testing.T) {
	// Create 100 balances
	balances := make([]models.Balance, 100)
	for i := 0; i < 100; i++ {
		balances[i] = createTestBalance(
			"account-"+string(rune('0'+i%10)),
			"USD",
			int64(i*100),
			int64(i*10),
		)
	}

	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      balances,
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}

	// Accounts are cached, so only 10 unique accounts
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			return createTestAccount(id, ptr("@user/"+id)), nil
		},
	}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 100, report.TotalsByAsset["USD"].Accounts)
}

func TestGenerateLedgerReport_VerySmallDecimalBalances(t *testing.T) {
	balance := models.Balance{
		ID:        "balance-1",
		AccountID: "account-1",
		AssetCode: "BTC",
		Available: decimal.NewFromFloat(0.00000001), // 1 satoshi
		OnHold:    decimal.NewFromFloat(0.00000000000001),
	}

	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      []models.Balance{balance},
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			return createTestAccount("account-1", ptr("@user/btc")), nil
		},
	}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)

	btcTotals := report.TotalsByAsset["BTC"]
	assert.True(t, btcTotals.TotalAvailable.Equal(decimal.NewFromFloat(0.00000001)))
}

func TestGenerateLedgerReport_VeryLargeDecimalBalances(t *testing.T) {
	// Very large balance (quadrillions)
	largeValue := decimal.NewFromInt(1).Shift(18) // 10^18
	balance := models.Balance{
		ID:        "balance-1",
		AccountID: "account-1",
		AssetCode: "POINTS",
		Available: largeValue,
		OnHold:    decimal.Zero,
	}

	mockBalances := &testBalancesService{
		listBalancesFn: func(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error) {
			return &models.ListResponse[models.Balance]{
				Items:      []models.Balance{balance},
				Pagination: models.Pagination{NextCursor: ""},
			}, nil
		},
	}
	mockAccounts := &testAccountsService{
		getAccountFn: func(ctx context.Context, orgID, ledgerID, id string) (*models.Account, error) {
			return createTestAccount("account-1", ptr("@user/points")), nil
		},
	}

	checker := NewChecker(&entities.Entity{
		Accounts: mockAccounts,
		Balances: mockBalances,
	})

	report, err := checker.GenerateLedgerReport(context.Background(), "org-1", "ledger-1")

	require.NoError(t, err)
	require.NotNil(t, report)

	pointsTotals := report.TotalsByAsset["POINTS"]
	assert.True(t, pointsTotals.TotalAvailable.Equal(largeValue))
}

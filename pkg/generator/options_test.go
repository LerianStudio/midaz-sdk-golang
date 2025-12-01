package generator

import (
	"context"
	"runtime"
	"testing"

	conc "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
	"github.com/stretchr/testify/assert"
)

func TestWithWorkers(t *testing.T) {
	tests := []struct {
		name     string
		workers  int
		expected int
	}{
		{
			name:     "Positive workers",
			workers:  10,
			expected: 10,
		},
		{
			name:     "Zero workers returns original context",
			workers:  0,
			expected: 0,
		},
		{
			name:     "Negative workers returns original context",
			workers:  -5,
			expected: 0,
		},
		{
			name:     "Large worker count",
			workers:  50,
			expected: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := WithWorkers(ctx, tt.workers)

			if tt.expected == 0 {
				// Context should be unchanged when workers <= 0
				val := result.Value(contextKeyWorkers{})
				assert.Nil(t, val)
			} else {
				val := result.Value(contextKeyWorkers{})
				assert.Equal(t, tt.expected, val)
			}
		})
	}
}

func TestGetWorkers(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() context.Context
		check    func(t *testing.T, result int)
	}{
		{
			name: "Workers from context",
			setupCtx: func() context.Context {
				return WithWorkers(context.Background(), 16)
			},
			check: func(t *testing.T, result int) {
				t.Helper()
				assert.Equal(t, 16, result)
			},
		},
		{
			name: "Workers capped at maxWorkers",
			setupCtx: func() context.Context {
				return WithWorkers(context.Background(), 200)
			},
			check: func(t *testing.T, result int) {
				t.Helper()
				assert.Equal(t, maxWorkers, result)
			},
		},
		{
			name: "Default workers when not in context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			check: func(t *testing.T, result int) {
				t.Helper()

				expectedDefault := runtime.NumCPU() * 2
				if expectedDefault < 4 {
					expectedDefault = 4
				}

				if expectedDefault > maxWorkers {
					expectedDefault = maxWorkers
				}

				assert.Equal(t, expectedDefault, result)
			},
		},
		{
			name: "Wrong type in context returns default",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), contextKeyWorkers{}, "not-an-int")
			},
			check: func(t *testing.T, result int) {
				t.Helper()

				expectedDefault := runtime.NumCPU() * 2
				if expectedDefault < 4 {
					expectedDefault = 4
				}

				if expectedDefault > maxWorkers {
					expectedDefault = maxWorkers
				}

				assert.Equal(t, expectedDefault, result)
			},
		},
		{
			name: "Zero value in context returns default",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), contextKeyWorkers{}, 0)
			},
			check: func(t *testing.T, result int) {
				t.Helper()

				expectedDefault := runtime.NumCPU() * 2
				if expectedDefault < 4 {
					expectedDefault = 4
				}

				if expectedDefault > maxWorkers {
					expectedDefault = maxWorkers
				}

				assert.Equal(t, expectedDefault, result)
			},
		},
		{
			name: "Negative value in context returns default",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), contextKeyWorkers{}, -5)
			},
			check: func(t *testing.T, result int) {
				t.Helper()

				expectedDefault := runtime.NumCPU() * 2
				if expectedDefault < 4 {
					expectedDefault = 4
				}

				if expectedDefault > maxWorkers {
					expectedDefault = maxWorkers
				}

				assert.Equal(t, expectedDefault, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			result := getWorkers(ctx)
			tt.check(t, result)
		})
	}
}

func TestWithCircuitBreaker(t *testing.T) {
	tests := []struct {
		name     string
		cb       *conc.CircuitBreaker
		expectCB bool
	}{
		{
			name:     "Valid circuit breaker",
			cb:       conc.NewCircuitBreaker(5, 2, 1000),
			expectCB: true,
		},
		{
			name:     "Nil circuit breaker returns original context",
			cb:       nil,
			expectCB: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := WithCircuitBreaker(ctx, tt.cb)

			if !tt.expectCB {
				// Context should be unchanged when cb is nil
				val := result.Value(contextKeyCircuitBreaker{})
				assert.Nil(t, val)
			} else {
				val := result.Value(contextKeyCircuitBreaker{})
				assert.NotNil(t, val)
				assert.IsType(t, &conc.CircuitBreaker{}, val)
			}
		})
	}
}

func TestGetCircuitBreaker(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() context.Context
		expectCB bool
	}{
		{
			name: "Circuit breaker present",
			setupCtx: func() context.Context {
				cb := conc.NewCircuitBreaker(5, 2, 1000)
				return WithCircuitBreaker(context.Background(), cb)
			},
			expectCB: true,
		},
		{
			name: "No circuit breaker in context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			expectCB: false,
		},
		{
			name: "Wrong type in context",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), contextKeyCircuitBreaker{}, "not-a-cb")
			},
			expectCB: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			result := getCircuitBreaker(ctx)

			if tt.expectCB {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestWithOrgLocale(t *testing.T) {
	tests := []struct {
		name           string
		locale         string
		expectedLocale string
		sameContext    bool
	}{
		{
			name:           "BR locale",
			locale:         "br",
			expectedLocale: "br",
			sameContext:    false,
		},
		{
			name:           "US locale",
			locale:         "us",
			expectedLocale: "us",
			sameContext:    false,
		},
		{
			name:           "Empty locale returns original context",
			locale:         "",
			expectedLocale: "",
			sameContext:    true,
		},
		{
			name:           "Custom locale",
			locale:         "eu",
			expectedLocale: "eu",
			sameContext:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := WithOrgLocale(ctx, tt.locale)

			if tt.sameContext {
				// Context should be unchanged when locale is empty
				val := result.Value(contextKeyOrgLocale{})
				assert.Nil(t, val)
			} else {
				val := result.Value(contextKeyOrgLocale{})
				assert.Equal(t, tt.expectedLocale, val)
			}
		})
	}
}

func TestGetOrgLocale(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() context.Context
		expected string
	}{
		{
			name: "BR locale from context",
			setupCtx: func() context.Context {
				return WithOrgLocale(context.Background(), "br")
			},
			expected: "br",
		},
		{
			name: "Default US locale when not in context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			expected: "us",
		},
		{
			name: "Wrong type in context returns default",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), contextKeyOrgLocale{}, 12345)
			},
			expected: "us",
		},
		{
			name: "Empty string in context returns default",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), contextKeyOrgLocale{}, "")
			},
			expected: "us",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			result := getOrgLocale(ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWithOrgID(t *testing.T) {
	tests := []struct {
		name     string
		orgID    string
		expected string
	}{
		{
			name:     "Valid org ID",
			orgID:    "org-123",
			expected: "org-123",
		},
		{
			name:     "Empty org ID",
			orgID:    "",
			expected: "",
		},
		{
			name:     "UUID org ID",
			orgID:    "550e8400-e29b-41d4-a716-446655440000",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := WithOrgID(ctx, tt.orgID)

			val := result.Value(contextKeyOrgID{})
			assert.Equal(t, tt.expected, val)
		})
	}
}

func TestWithLedgerID(t *testing.T) {
	tests := []struct {
		name     string
		ledgerID string
		expected string
	}{
		{
			name:     "Valid ledger ID",
			ledgerID: "ledger-123",
			expected: "ledger-123",
		},
		{
			name:     "Empty ledger ID",
			ledgerID: "",
			expected: "",
		},
		{
			name:     "UUID ledger ID",
			ledgerID: "550e8400-e29b-41d4-a716-446655440000",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := WithLedgerID(ctx, tt.ledgerID)

			val := result.Value(contextKeyLedgerID{})
			assert.Equal(t, tt.expected, val)
		})
	}
}

func TestContextChaining(t *testing.T) {
	t.Run("Multiple context values can be chained", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithWorkers(ctx, 8)
		ctx = WithOrgLocale(ctx, "br")
		ctx = WithOrgID(ctx, "org-123")
		ctx = WithLedgerID(ctx, "ledger-456")
		cb := conc.NewCircuitBreaker(5, 2, 1000)
		ctx = WithCircuitBreaker(ctx, cb)

		assert.Equal(t, 8, getWorkers(ctx))
		assert.Equal(t, "br", getOrgLocale(ctx))
		assert.Equal(t, "org-123", ctx.Value(contextKeyOrgID{}))
		assert.Equal(t, "ledger-456", ctx.Value(contextKeyLedgerID{}))
		assert.NotNil(t, getCircuitBreaker(ctx))
	})
}

func TestMaxWorkersConstant(t *testing.T) {
	t.Run("maxWorkers is set to 100", func(t *testing.T) {
		assert.Equal(t, 100, maxWorkers)
	})
}

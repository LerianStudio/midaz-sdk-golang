package generator

import (
    "context"
)

// executeWithCircuitBreaker wraps a function with a circuit breaker if one is present in context.
func executeWithCircuitBreaker(ctx context.Context, fn func() error) error {
    if cb := getCircuitBreaker(ctx); cb != nil {
        return cb.Execute(ctx, fn)
    }
    return fn()
}


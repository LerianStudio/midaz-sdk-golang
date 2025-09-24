package generator

import (
    "context"
    "runtime"
    conc "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
)

// context keys
type contextKeyWorkers struct{}
type contextKeyCircuitBreaker struct{}

// WithWorkers stores a preferred worker count in context for batch generation.
func WithWorkers(ctx context.Context, workers int) context.Context {
    if workers <= 0 {
        return ctx
    }
    return context.WithValue(ctx, contextKeyWorkers{}, workers)
}

func getWorkers(ctx context.Context) int {
    if v := ctx.Value(contextKeyWorkers{}); v != nil {
        if n, ok := v.(int); ok && n > 0 {
            return n
        }
    }
    // default heuristic: 2x CPU cores, min 4
    n := runtime.NumCPU() * 2
    if n < 4 {
        n = 4
    }
    return n
}

// WithCircuitBreaker stores a circuit breaker in context for generator calls.
func WithCircuitBreaker(ctx context.Context, cb *conc.CircuitBreaker) context.Context {
    if cb == nil {
        return ctx
    }
    return context.WithValue(ctx, contextKeyCircuitBreaker{}, cb)
}

func getCircuitBreaker(ctx context.Context) *conc.CircuitBreaker {
    v := ctx.Value(contextKeyCircuitBreaker{})
    if v == nil {
        return nil
    }
    if cb, ok := v.(*conc.CircuitBreaker); ok {
        return cb
    }
    return nil
}

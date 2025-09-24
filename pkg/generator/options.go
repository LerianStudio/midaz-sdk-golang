package generator

import (
    "context"
    "runtime"
)

// context keys
type contextKeyWorkers struct{}

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


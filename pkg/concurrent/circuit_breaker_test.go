package concurrent

import (
    "context"
    "errors"
    "testing"
    "time"
)

func TestCircuitBreakerTransitions(t *testing.T) {
    cb := NewCircuitBreaker(2, 1, 100*time.Millisecond)

    // Two consecutive failures should open the circuit
    var executed int
    failFn := func() error { executed++; return errors.New("boom") }
    _ = cb.Execute(context.Background(), failFn)
    _ = cb.Execute(context.Background(), failFn)

    if executed != 2 {
        t.Fatalf("expected 2 executions before open, got %d", executed)
    }

    // Third attempt should be blocked
    err := cb.Execute(context.Background(), failFn)
    if !errors.Is(err, ErrCircuitOpen) {
        t.Fatalf("expected ErrCircuitOpen, got %v", err)
    }

    // Wait for half-open transition
    time.Sleep(120 * time.Millisecond)

    // Successful probe should close the circuit (successThreshold=1)
    successFn := func() error { executed++; return nil }
    if err := cb.Execute(context.Background(), successFn); err != nil {
        t.Fatalf("expected success in half-open, got %v", err)
    }

    // Verify subsequent calls are allowed (closed state)
    if err := cb.Execute(context.Background(), successFn); err != nil {
        t.Fatalf("expected success in closed state, got %v", err)
    }
}


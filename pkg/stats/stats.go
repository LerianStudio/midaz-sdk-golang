package stats

import (
    "sync/atomic"
    "time"
)

// Counter tracks successes over time and provides a basic TPS view.
// This is intentionally lightweight for Phase 1; later phases can fold in
// richer observability/performance integrations when generating entities.
type Counter struct {
    start        time.Time
    successCount int64
}

// NewCounter initializes a new Counter instance.
func NewCounter() *Counter {
    return &Counter{start: time.Now()}
}

// RecordSuccess increments the success counter.
func (c *Counter) RecordSuccess() {
    atomic.AddInt64(&c.successCount, 1)
}

// SuccessCount returns the number of successes recorded.
func (c *Counter) SuccessCount() int64 {
    return atomic.LoadInt64(&c.successCount)
}

// Elapsed returns the elapsed time since the counter started.
func (c *Counter) Elapsed() time.Duration {
    return time.Since(c.start)
}

// TPS returns a simple successes-per-second rate since start.
func (c *Counter) TPS() float64 {
    elapsed := time.Since(c.start).Seconds()
    if elapsed <= 0 {
        return 0
    }
    return float64(c.SuccessCount()) / elapsed
}


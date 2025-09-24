package concurrent

import (
    "context"
    "errors"
    "log"
    "sync"
    "time"
)

var ErrCircuitOpen = errors.New("circuit breaker open")

type state int

const (
    closed state = iota
    open
    halfOpen
)

// CircuitBreaker is a simple circuit breaker implementation to protect
// downstream services from cascading failures.
type CircuitBreaker struct {
    mu               sync.Mutex
    st               state
    failureCount     int
    successCount     int
    lastFailureTime  time.Time
    failureThreshold int
    successThreshold int
    openTimeout      time.Duration
    name             string
    logger           *log.Logger
}

func NewCircuitBreaker(failureThreshold, successThreshold int, openTimeout time.Duration) *CircuitBreaker {
    if failureThreshold <= 0 {
        failureThreshold = 5
    }
    if successThreshold <= 0 {
        successThreshold = 2
    }
    if openTimeout <= 0 {
        openTimeout = 5 * time.Second
    }
    return &CircuitBreaker{
        st:               closed,
        failureThreshold: failureThreshold,
        successThreshold: successThreshold,
        openTimeout:      openTimeout,
    }
}

// NewCircuitBreakerNamed creates a circuit breaker with a label for observability.
func NewCircuitBreakerNamed(name string, failureThreshold, successThreshold int, openTimeout time.Duration) *CircuitBreaker {
    cb := NewCircuitBreaker(failureThreshold, successThreshold, openTimeout)
    cb.name = name
    return cb
}

// WithLogger attaches a logger for state transition messages.
func (cb *CircuitBreaker) WithLogger(l *log.Logger) *CircuitBreaker {
    cb.logger = l
    return cb
}

// Execute runs fn under circuit breaker control.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
    if !cb.allow() {
        return ErrCircuitOpen
    }

    err := fn()
    cb.after(err)
    return err
}

func (cb *CircuitBreaker) allow() bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    switch cb.st {
    case closed:
        return true
    case open:
        if time.Since(cb.lastFailureTime) >= cb.openTimeout {
            cb.st = halfOpen
            cb.successCount = 0
            return true
        }
        return false
    case halfOpen:
        // Allow limited probes, treat as single probe here
        return true
    default:
        return true
    }
}

func (cb *CircuitBreaker) after(err error) {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    if err == nil {
        switch cb.st {
        case halfOpen:
            cb.successCount++
            if cb.successCount >= cb.successThreshold {
                cb.reset()
            }
        case closed:
            // success in closed resets failures
            cb.failureCount = 0
        }
        return
    }

    // On error
    switch cb.st {
    case closed:
        cb.failureCount++
        if cb.failureCount >= cb.failureThreshold {
            cb.open()
        }
    case halfOpen:
        // Any failure during half-open sends back to open
        cb.open()
    }
}

func (cb *CircuitBreaker) open() {
    cb.st = open
    cb.lastFailureTime = time.Now()
    if cb.logger != nil && cb.name != "" {
        cb.logger.Printf("circuit '%s' opened after %d failures", cb.name, cb.failureCount)
    }
}

func (cb *CircuitBreaker) reset() {
    cb.st = closed
    cb.failureCount = 0
    cb.successCount = 0
    if cb.logger != nil && cb.name != "" {
        cb.logger.Printf("circuit '%s' closed after recovery", cb.name)
    }
}

package concurrent

import (
	"context"
	"errors"
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

// CBLogger is a minimal interface for circuit breaker logging.
// Compatible with observability.Logger and standard log.Logger via Printf.
type CBLogger interface {
	Printf(format string, v ...any)
}

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
	logger           CBLogger
}

// Default circuit breaker configuration values.
const (
	DefaultFailureThreshold = 5
	DefaultSuccessThreshold = 2
	DefaultOpenTimeout      = 5 * time.Second
)

func NewCircuitBreaker(failureThreshold, successThreshold int, openTimeout time.Duration) *CircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = DefaultFailureThreshold
	}

	if successThreshold <= 0 {
		successThreshold = DefaultSuccessThreshold
	}

	if openTimeout <= 0 {
		openTimeout = DefaultOpenTimeout
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
// Accepts any logger implementing Printf(format string, v ...any).
func (cb *CircuitBreaker) WithLogger(l CBLogger) *CircuitBreaker {
	cb.logger = l
	return cb
}

// Execute runs fn under circuit breaker control.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if !cb.canProceed() {
		return ErrCircuitOpen
	}

	err := fn()
	cb.after(err)

	return err
}

// canProceed determines if the circuit breaker will allow the operation to proceed.
// Returns true if circuit is closed or half-open (allowing probe), false if circuit is open.
func (cb *CircuitBreaker) canProceed() bool {
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

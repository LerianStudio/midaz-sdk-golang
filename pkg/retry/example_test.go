package retry_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/retry"
)

// ExampleWithMaxRetries shows the simplest customization: cap the
// retry count. The default is 3; reduce or increase to match your
// SLA bounds. Combined with WithErrorPredicate to control what counts
// as retryable for arbitrary errors.
func ExampleWithMaxRetries() {
	transient := errors.New("transient")

	opts := []retry.Option{
		retry.WithMaxRetries(5),
		retry.WithInitialDelay(time.Microsecond),
		retry.WithErrorPredicate(func(err error) bool {
			return errors.Is(err, transient)
		}),
	}

	attempts := 0
	err := retry.Do(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return transient
		}
		return nil
	}, opts...)

	fmt.Println(err)
	fmt.Println(attempts)
	// Output:
	// <nil>
	// 3
}

// ExampleWithBackoffFactor demonstrates customizing exponential
// backoff. The default factor is 2.0 (each delay doubles); a higher
// factor backs off more aggressively.
func ExampleWithBackoffFactor() {
	opts := []retry.Option{
		retry.WithMaxRetries(2),
		retry.WithInitialDelay(time.Millisecond),
		retry.WithBackoffFactor(3.0),
	}

	_ = retry.Do(context.Background(), func() error {
		return errors.New("always fails")
	}, opts...)

	fmt.Println("retry policy applied with 3x backoff factor")
	// Output: retry policy applied with 3x backoff factor
}

// ExampleWithErrorPredicate shows a custom retry predicate. Use this
// when the default classification (network errors, 5xx, 408, 425, 429)
// doesn't match your downstream service's idempotency / safety contract.
func ExampleWithErrorPredicate() {
	tooBusy := errors.New("downstream too busy")

	opts := []retry.Option{
		retry.WithMaxRetries(1),
		retry.WithInitialDelay(time.Microsecond),
		retry.WithErrorPredicate(func(err error) bool {
			return errors.Is(err, tooBusy)
		}),
	}

	attempts := 0
	_ = retry.Do(context.Background(), func() error {
		attempts++
		return tooBusy
	}, opts...)

	fmt.Println(attempts)
	// Output: 2
}

package retry

import (
	"context"
	"errors"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetryOptionsAreIsolatedAndValidated(t *testing.T) {
	t.Run("default slices are isolated", func(t *testing.T) {
		first := DefaultOptions()
		first.RetryableErrors[0] = "mutated"
		first.RetryableHTTPCodes[0] = 599

		second := DefaultOptions()
		require.NotEqual(t, "mutated", second.RetryableErrors[0])
		require.NotEqual(t, 599, second.RetryableHTTPCodes[0])

		httpFirst := DefaultHTTPOptions()
		httpFirst.RetryableNetworkErrors[0] = "mutated"
		httpFirst.RetryableHTTPCodes[0] = 599

		httpSecond := DefaultHTTPOptions()
		require.NotEqual(t, "mutated", httpSecond.RetryableNetworkErrors[0])
		require.NotEqual(t, 599, httpSecond.RetryableHTTPCodes[0])
	})

	t.Run("option setters copy caller slices", func(t *testing.T) {
		errorsList := []string{"temporary"}
		codes := []int{http.StatusServiceUnavailable}

		require.NoError(t, Do(context.Background(), func() error { return nil }, WithRetryableErrors(errorsList), WithRetryableHTTPCodes(codes)))
		errorsList[0] = "mutated"
		codes[0] = 599

		opts := DefaultOptions()
		require.NoError(t, WithRetryableErrors([]string{"temporary"})(opts))
		require.NoError(t, WithRetryableHTTPCodes([]int{http.StatusServiceUnavailable})(opts))
		require.Equal(t, "temporary", opts.RetryableErrors[0])
		require.Equal(t, http.StatusServiceUnavailable, opts.RetryableHTTPCodes[0])
	})

	t.Run("context options cannot bypass validation", func(t *testing.T) {
		ctx := WithOptionsContext(context.Background(), &Options{
			MaxRetries:    -1,
			InitialDelay:  time.Millisecond,
			MaxDelay:      time.Second,
			BackoffFactor: 2,
			JitterFactor:  0.1,
		})

		err := DoWithContext(ctx, func() error { return errors.New("timeout") })
		require.ErrorContains(t, err, "maxRetries")

		httpCtx := WithHTTPOptionsContext(context.Background(), &HTTPOptions{
			MaxRetries:             1,
			InitialDelay:           time.Second,
			MaxDelay:               time.Millisecond,
			BackoffFactor:          2,
			RetryableHTTPCodes:     []int{http.StatusServiceUnavailable},
			RetryableNetworkErrors: []string{"timeout"},
			JitterFactor:           0.1,
		})

		_, err = DoHTTPRequestWithContext(httpCtx, nil, mustRequest(t))
		require.ErrorContains(t, err, "maxDelay")
	})

	t.Run("non finite retry factors are rejected", func(t *testing.T) {
		require.Error(t, WithBackoffFactor(math.NaN())(DefaultOptions()))
		require.Error(t, WithJitterFactor(math.Inf(1))(DefaultOptions()))
		require.Error(t, WithHTTPBackoffFactor(math.NaN())(DefaultHTTPOptions()))
		require.Error(t, WithHTTPJitterFactor(math.Inf(1))(DefaultHTTPOptions()))
	})
}

func mustRequest(t *testing.T) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	return req
}

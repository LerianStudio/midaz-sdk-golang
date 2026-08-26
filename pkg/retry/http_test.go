package retry_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nilContext() context.Context {
	return nil
}

func TestDoHTTPRequest_Success(t *testing.T) {
	// Create a test server that succeeds on the first try
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"success"}`))
	}))
	defer server.Close()

	// Create a request
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	// Execute the request with retries
	resp, err := retry.DoHTTPRequest(context.Background(), http.DefaultClient, req)

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	assert.NotNil(t, resp.Body)
	assert.Contains(t, string(resp.Body), "success")
	assert.Equal(t, 0, resp.Attempt) // First attempt should succeed
}

func TestDoHTTPRequest_ServerError(t *testing.T) {
	// Create attempt counter
	attempts := 0

	// Create a test server that fails with 500 twice, then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal server error"}`))

			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"success after retry"}`))
	}))
	defer server.Close()

	// Create a request
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	// Execute the request with retries
	resp, err := retry.DoHTTPRequest(
		context.Background(),
		http.DefaultClient,
		req,
		retry.WithHTTPRetryAllServerErrors(true),
		retry.WithHTTPInitialDelay(10*time.Millisecond), // Speed up the test
	)

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	assert.Contains(t, string(resp.Body), "success after retry")
	assert.Equal(t, 2, resp.Attempt) // Should succeed on the third attempt (index 2)
	assert.Equal(t, 3, attempts)     // Server should have been called 3 times
}

func TestDoHTTPRequest_ClientError(t *testing.T) {
	// Create a test server that returns a client error (400)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	// Create a request
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	// Execute the request with retries
	resp, err := retry.DoHTTPRequest(context.Background(), http.DefaultClient, req)

	// Assertions
	require.Error(t, err) // Should return an error for client errors
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.Response.StatusCode)
	assert.Contains(t, string(resp.Body), "bad request")
}

func TestDoHTTPRequest_RetryOn429(t *testing.T) {
	// Create attempt counter
	attempts := 0

	// Create a test server that returns 429 twice, then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))

			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"success after rate limit"}`))
	}))
	defer server.Close()

	// Create a request
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	// Execute the request with retries
	resp, err := retry.DoHTTPRequest(
		context.Background(),
		http.DefaultClient,
		req,
		retry.WithHTTPInitialDelay(10*time.Millisecond), // Speed up the test
	)

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	assert.Contains(t, string(resp.Body), "success after rate limit")
	assert.Equal(t, 2, resp.Attempt) // Should succeed on the third attempt (index 2)
	assert.Equal(t, 3, attempts)     // Server should have been called 3 times
}

func TestDoHTTPRequest_NetworkError(t *testing.T) {
	// Create a server that immediately closes the connection
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("This should not be called as we're using a non-existent URL")
	}))
	server.Close() // Close immediately to cause a connection error

	// Use the already-closed server's URL to ensure a connection error
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	// Execute the request with minimal retries to speed up the test
	resp, err := retry.DoHTTPRequest(
		context.Background(),
		http.DefaultClient,
		req,
		retry.WithHTTPMaxRetries(1),
		retry.WithHTTPInitialDelay(10*time.Millisecond),
	)

	// Assertions
	require.Error(t, err)
	assert.NotNil(t, resp)
	assert.Nil(t, resp.Response) // There should be no response
	require.Error(t, resp.Error) // Error should be captured in the response
}

func TestHTTPRetryExportedAPIs_NilInputsDoNotPanic(t *testing.T) {
	tests := []struct {
		name string
		run  func() (*retry.HTTPResponse, error)
	}{
		{
			name: "DoHTTPRequest with nil context returns error",
			run: func() (*retry.HTTPResponse, error) {
				req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
				require.NoError(t, err)

				return retry.DoHTTPRequest(nilContext(), http.DefaultClient, req)
			},
		},
		{
			name: "DoHTTPRequest with nil request returns error",
			run: func() (*retry.HTTPResponse, error) {
				return retry.DoHTTPRequest(context.Background(), http.DefaultClient, nil)
			},
		},
		{
			name: "DoHTTPRequest with nil option returns error",
			run: func() (*retry.HTTPResponse, error) {
				req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
				require.NoError(t, err)

				return retry.DoHTTPRequest(context.Background(), http.DefaultClient, req, nil)
			},
		},
		{
			name: "DoHTTPRequestWithContext with nil context returns error",
			run: func() (*retry.HTTPResponse, error) {
				req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
				require.NoError(t, err)

				return retry.DoHTTPRequestWithContext(nilContext(), http.DefaultClient, req)
			},
		},
		{
			name: "DoHTTP with nil context returns error",
			run: func() (*retry.HTTPResponse, error) {
				return retry.DoHTTP(nilContext(), http.DefaultClient, http.MethodGet, "https://example.com", nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("expected error instead of panic, got panic: %v", recovered)
				}
			}()

			_, err := tt.run()
			require.Error(t, err)
		})
	}
}

func TestHTTPRetryContextOptions_NilInputsReturnDefaults(t *testing.T) {
	t.Run("WithHTTPOptionsContext nil parent uses background", func(t *testing.T) {
		ctx := retry.WithHTTPOptionsContext(nilContext(), &retry.HTTPOptions{MaxRetries: 9})
		if ctx == nil {
			t.Fatal("expected context, got nil")
		}

		options := retry.GetHTTPOptionsFromContext(ctx)
		assert.Equal(t, 9, options.MaxRetries)
	})

	t.Run("GetHTTPOptionsFromContext nil context returns defaults", func(t *testing.T) {
		options := retry.GetHTTPOptionsFromContext(nilContext())
		require.NotNil(t, options)
		assert.Equal(t, retry.DefaultHTTPOptions().MaxRetries, options.MaxRetries)
	})

	t.Run("nil context-stored options return defaults", func(t *testing.T) {
		ctx := retry.WithHTTPOptionsContext(context.Background(), nil)
		options := retry.GetHTTPOptionsFromContext(ctx)
		require.NotNil(t, options)
		assert.Equal(t, retry.DefaultHTTPOptions().MaxRetries, options.MaxRetries)
	})
}

func TestDoHTTPRequest_UnsafePOSTWithoutIdempotencyDoesNotRetry(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL, nil)
	require.NoError(t, err)

	resp, err := retry.DoHTTPRequest(
		context.Background(),
		http.DefaultClient,
		req,
		retry.WithHTTPMaxRetries(3),
		retry.WithHTTPInitialDelay(time.Millisecond),
	)

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusInternalServerError, resp.Response.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestDoHTTPRequest_UnsafePOSTWithLegacyIdempotencyKeyDoesNotRetry(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Idempotency-Key", "legacy-key")

	resp, err := retry.DoHTTPRequest(
		context.Background(),
		http.DefaultClient,
		req,
		retry.WithHTTPMaxRetries(3),
		retry.WithHTTPInitialDelay(time.Millisecond),
	)

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusInternalServerError, resp.Response.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestDoHTTPRequest_UnsafePOSTWithIdempotencyAndReplayableBodyRetriesIdenticalBody(t *testing.T) {
	var (
		attempts    int32
		bodies      []string
		handlerErrs = make(chan error, 2)
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			handlerErrs <- err

			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		bodies = append(bodies, string(body))

		if atomic.LoadInt32(&attempts) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	body := []byte(`{"amount":100}`)
	req, err := http.NewRequest(http.MethodPost, server.URL, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("X-Idempotency", "idem-123")
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}

	resp, err := retry.DoHTTPRequest(
		context.Background(),
		http.DefaultClient,
		req,
		retry.WithHTTPMaxRetries(1),
		retry.WithHTTPInitialDelay(time.Millisecond),
	)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assertNoHandlerError(t, handlerErrs)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	assert.Equal(t, int32(2), atomic.LoadInt32(&attempts))
	require.Len(t, bodies, 2)
	assert.Equal(t, string(body), bodies[0])
	assert.Equal(t, string(body), bodies[1])
}

func TestDoHTTPRequest_BodyWithoutGetBodyDoesNotRetryWithEmptyBody(t *testing.T) {
	var (
		attempts     int32
		receivedBody string
		handlerErrs  = make(chan error, 1)
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			handlerErrs <- err

			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		receivedBody = string(body)

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL, io.NopCloser(strings.NewReader(`{"amount":100}`)))
	require.NoError(t, err)
	req.Header.Set("X-Idempotency", "idem-123")
	req.GetBody = nil

	resp, err := retry.DoHTTPRequest(
		context.Background(),
		http.DefaultClient,
		req,
		retry.WithHTTPMaxRetries(1),
		retry.WithHTTPInitialDelay(time.Millisecond),
	)

	require.Error(t, err)
	require.NotNil(t, resp)
	assertNoHandlerError(t, handlerErrs)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
	assert.Equal(t, `{"amount":100}`, receivedBody)
}

func TestDoHTTPRequest_RetryContextCancellationStopsActualRequestAttempt(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		<-r.Context().Done()
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		_, err := retry.DoHTTPRequest(ctx, http.DefaultClient, req, retry.WithHTTPMaxRetries(0))
		errCh <- err
	}()

	<-started
	cancel()
	close(release)

	select {
	case err := <-errCh:
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("expected request attempt to stop after retry context cancellation")
	}
}

func TestDoHTTPRequest_PreRetryHookRunsOnlyForRealRetries(t *testing.T) {
	var (
		attempts  int32
		hookCalls int32
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	resp, err := retry.DoHTTPRequest(
		context.Background(),
		http.DefaultClient,
		req,
		retry.WithHTTPMaxRetries(3),
		retry.WithHTTPInitialDelay(time.Millisecond),
		retry.WithHTTPPreRetryHook(func(_ context.Context, _ *http.Request, resp *retry.HTTPResponse) error {
			require.NotNil(t, resp)
			require.NotNil(t, resp.Response)
			assert.Equal(t, http.StatusInternalServerError, resp.Response.StatusCode)
			atomic.AddInt32(&hookCalls, 1)

			return nil
		}),
	)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	assert.Equal(t, int32(2), atomic.LoadInt32(&attempts))
	assert.Equal(t, int32(1), atomic.LoadInt32(&hookCalls))
}

func assertNoHandlerError(t *testing.T, errs <-chan error) {
	t.Helper()

	select {
	case err := <-errs:
		require.NoError(t, err)
	default:
	}
}

func TestDoHTTPRequest_ContextCancellation(t *testing.T) {
	// Create a test server that intentionally delays
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Sleep long enough to cause timeout
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"success"}`))
	}))
	defer server.Close()

	// Create a request
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	// Use a custom client with a very short timeout (10ms)
	client := &http.Client{
		Timeout: 10 * time.Millisecond,
	}

	// Execute the request with retries
	ctx := context.Background()
	resp, err := retry.DoHTTPRequest(
		ctx,
		client,
		req,
		retry.WithHTTPMaxRetries(1),
		retry.WithHTTPInitialDelay(5*time.Millisecond),
		retry.WithHTTPRetryableNetworkErrors([]string{"context deadline exceeded", "deadline exceeded"}),
	)

	// Should get a timeout error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
	assert.NotNil(t, resp)
}

func TestDoHTTP_SimpleAPI(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check method and path
		if r.Method == http.MethodPost && r.URL.Path == "/test" {
			// Check body
			body, _ := io.ReadAll(r.Body)
			if string(body) == `{"test":"body"}` {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"message":"success"}`))

				return
			}
		}

		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	// Use the simpler DoHTTP API
	body := strings.NewReader(`{"test":"body"}`)
	resp, err := retry.DoHTTP(context.Background(), http.DefaultClient, "POST", server.URL+"/test", body)

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	assert.Contains(t, string(resp.Body), "success")
}

// Test HTTP options via context
func TestHTTPOptionsContext(t *testing.T) {
	baseCtx := context.Background()
	options := retry.DefaultHTTPOptions()
	options.MaxRetries = 7
	options.InitialDelay = 250 * time.Millisecond
	options.RetryAllServerErrors = false

	// Add options to context
	ctx := retry.WithHTTPOptionsContext(baseCtx, options)

	// Get options from context
	retrievedOptions := retry.GetHTTPOptionsFromContext(ctx)

	// Check that options match
	assert.Equal(t, options.MaxRetries, retrievedOptions.MaxRetries)
	assert.Equal(t, options.InitialDelay, retrievedOptions.InitialDelay)
	assert.Equal(t, options.RetryAllServerErrors, retrievedOptions.RetryAllServerErrors)
}

func TestDoHTTPRequestWithContext(t *testing.T) {
	// Create a test server that succeeds on the first try
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"success"}`))
	}))
	defer server.Close()

	// Create a request
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	// Create context with options
	options := retry.DefaultHTTPOptions()
	options.MaxRetries = 2
	ctx := retry.WithHTTPOptionsContext(context.Background(), options)

	// Execute the request with retries from context
	resp, err := retry.DoHTTPRequestWithContext(ctx, http.DefaultClient, req)

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	assert.NotNil(t, resp.Body)
	assert.Contains(t, string(resp.Body), "success")
}

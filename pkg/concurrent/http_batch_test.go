package concurrent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPBatchProcessor_ExecuteBatch(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requests []concurrent.HTTPBatchRequest
		err := json.NewDecoder(r.Body).Decode(&requests)
		require.NoError(t, err)

		// Create responses
		responses := make([]concurrent.HTTPBatchResponse, len(requests))
		for i, req := range requests {
			statusCode := 200
			var responseBody any
			var errorMsg string

			// Handle requests based on path
			if req.Path == "/error" {
				statusCode = 400
				errorMsg = "Test error"
			} else if req.Path == "/data" {
				responseBody = map[string]any{
					"message": "Test data",
					"id":      req.ID,
				}
			}

			// Convert response body to JSON
			var jsonBody json.RawMessage
			if responseBody != nil {
				data, err := json.Marshal(responseBody)
				require.NoError(t, err)
				jsonBody = data
			}

			// Create response
			responses[i] = concurrent.HTTPBatchResponse{
				ID:         req.ID,
				StatusCode: statusCode,
				Body:       jsonBody,
				Error:      errorMsg,
			}
		}

		// Return responses
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer server.Close()

	// Create test requests
	requests := []concurrent.HTTPBatchRequest{
		{
			Method: "GET",
			Path:   "/data",
			ID:     "req_1",
		},
		{
			Method: "GET",
			Path:   "/data",
			ID:     "req_2",
		},
	}

	// With ContinueOnError=false but no errors, should succeed
	processor := concurrent.NewHTTPBatchProcessor(
		server.Client(),
		server.URL,
		concurrent.WithBatchContinueOnError(false),
	)
	ctx := context.Background()
	result, err := processor.ExecuteBatch(ctx, requests)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, len(result.Responses))

	// Add a request that returns an error
	requests = append(requests, concurrent.HTTPBatchRequest{
		Method: "GET",
		Path:   "/error",
		ID:     "req_3",
	})

	// Reset processor with ContinueOnError=true
	processor = concurrent.NewHTTPBatchProcessor(
		server.Client(),
		server.URL,
		concurrent.WithBatchContinueOnError(true),
	)

	// Execute batch
	result, err = processor.ExecuteBatch(ctx, requests)

	// Should succeed but with an error in the result
	assert.NoError(t, err)
	assert.Equal(t, 3, len(result.Responses))
	assert.Equal(t, 200, result.Responses[0].StatusCode)
	assert.Equal(t, 200, result.Responses[1].StatusCode)
	assert.Equal(t, 400, result.Responses[2].StatusCode)
	assert.Equal(t, "Test error", result.Responses[2].Error)

	// Test ParseResponse
	var response map[string]any
	err = processor.ParseResponse(result, "req_1", &response)
	assert.NoError(t, err)
	assert.Equal(t, "Test data", response["message"])
	assert.Equal(t, "req_1", response["id"])

	// Test ParseResponse with error
	err = processor.ParseResponse(result, "req_3", &response)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Test error")

	// Test custom worker pool options
	result, err = processor.ExecuteBatchWithPoolOptions(ctx, requests,
		concurrent.WithWorkers(3),
		concurrent.WithUnorderedResults(),
	)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(result.Responses))
}

// TestHTTPBatchProcessor_ExecuteLargeBatch tests the batching functionality with a large number of requests
func TestHTTPBatchProcessor_ExecuteLargeBatch(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requests []concurrent.HTTPBatchRequest
		err := json.NewDecoder(r.Body).Decode(&requests)
		require.NoError(t, err)

		// Create responses
		responses := make([]concurrent.HTTPBatchResponse, len(requests))
		for i, req := range requests {
			responses[i] = concurrent.HTTPBatchResponse{
				ID:         req.ID,
				StatusCode: 200,
				Body:       []byte(`{"success": true}`),
			}
		}

		// Return responses
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer server.Close()

	// Create batch processor with small MaxBatchSize to force multiple batches
	processor := concurrent.NewHTTPBatchProcessor(
		server.Client(),
		server.URL,
		concurrent.WithMaxBatchSize(5),
	)

	// Create many test requests
	var requests []concurrent.HTTPBatchRequest
	for i := 0; i < 23; i++ {
		requests = append(requests, concurrent.HTTPBatchRequest{
			Method: "GET",
			Path:   "/data",
			ID:     fmt.Sprintf("req_%d", i),
		})
	}

	// Execute batch
	ctx := context.Background()
	result, err := processor.ExecuteBatch(ctx, requests)

	// Should succeed
	assert.NoError(t, err)
	assert.Equal(t, 23, len(result.Responses))

	// All responses should have status 200
	for _, response := range result.Responses {
		assert.Equal(t, 200, response.StatusCode)
	}
}

// TestHTTPBatchProcessor_Retry tests the retry functionality
func TestHTTPBatchProcessor_Retry(t *testing.T) {
	// Create a counter for retries
	attemptCount := 0

	// Create a test server that fails the first two times
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requests []concurrent.HTTPBatchRequest
		err := json.NewDecoder(r.Body).Decode(&requests)
		require.NoError(t, err)

		attemptCount++

		if attemptCount <= 2 {
			// Fail the first two attempts
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "internal server error"}`))
			return
		}

		// Succeed on the third attempt
		// Create responses
		responses := make([]concurrent.HTTPBatchResponse, len(requests))
		for i, req := range requests {
			responses[i] = concurrent.HTTPBatchResponse{
				ID:         req.ID,
				StatusCode: 200,
				Body:       []byte(`{"success": true}`),
			}
		}

		// Return responses
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer server.Close()

	// Create batch processor with retry
	processor := concurrent.NewHTTPBatchProcessor(
		server.Client(),
		server.URL,
		concurrent.WithBatchRetryCount(3),
		concurrent.WithBatchRetryBackoff(10*time.Millisecond),
	)

	// Create test requests
	requests := []concurrent.HTTPBatchRequest{
		{
			Method: "GET",
			Path:   "/data",
			ID:     "req_1",
		},
	}

	// Execute batch
	ctx := context.Background()
	result, err := processor.ExecuteBatch(ctx, requests)

	// Should succeed after retries
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Responses))
	assert.Equal(t, 3, attemptCount) // Should have made 3 attempts
}

// TestHTTPBatchProcessor_CustomJSONMarshaler tests using a custom JSON marshaler
func TestHTTPBatchProcessor_CustomJSONMarshaler(t *testing.T) {
	// Create a simple custom JSON marshaler
	customMarshaler := &CustomJSONMarshaler{}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a simple response
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"req_1","statusCode":200,"body":{"test":"value"}}]`))
	}))
	defer server.Close()

	// Create batch processor
	processor := concurrent.NewHTTPBatchProcessor(server.Client(), server.URL)
	processor.SetJSONMarshaler(customMarshaler)

	// Create test requests
	requests := []concurrent.HTTPBatchRequest{
		{
			Method: "GET",
			Path:   "/data",
			ID:     "req_1",
		},
	}

	// Execute batch
	ctx := context.Background()
	result, err := processor.ExecuteBatch(ctx, requests)

	// Should succeed
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result.Responses))

	// Parse response
	var response map[string]any
	err = processor.ParseResponse(result, "req_1", &response)
	assert.NoError(t, err)
	assert.Equal(t, "value", response["test"])
}

// CustomJSONMarshaler is a simple implementation of JSONMarshaler for testing
type CustomJSONMarshaler struct{}

func (m *CustomJSONMarshaler) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (m *CustomJSONMarshaler) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// TestHTTPBatchProcessor_EmptyBatch tests executing an empty batch
func TestHTTPBatchProcessor_EmptyBatch(t *testing.T) {
	processor := concurrent.NewHTTPBatchProcessor(nil, "http://example.com")

	result, err := processor.ExecuteBatch(context.Background(), []concurrent.HTTPBatchRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Responses)
}

// TestHTTPBatchProcessor_NilClient tests creating processor with nil client
func TestHTTPBatchProcessor_NilClient(t *testing.T) {
	processor := concurrent.NewHTTPBatchProcessor(nil, "http://example.com")
	assert.NotNil(t, processor)
}

// TestHTTPBatchProcessor_DefaultHeaders tests setting default headers
func TestHTTPBatchProcessor_DefaultHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers were set
		assert.Equal(t, "test-value", r.Header.Get("X-Test-Header"))
		assert.Equal(t, "another-value", r.Header.Get("X-Another-Header"))

		// Return empty batch response
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"req_1","statusCode":200}]`))
	}))
	defer server.Close()

	processor := concurrent.NewHTTPBatchProcessor(server.Client(), server.URL)
	processor.SetDefaultHeader("X-Test-Header", "test-value")
	processor.SetDefaultHeaders(map[string]string{
		"X-Another-Header": "another-value",
	})

	requests := []concurrent.HTTPBatchRequest{
		{Method: "GET", Path: "/test", ID: "req_1"},
	}

	result, err := processor.ExecuteBatch(context.Background(), requests)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestHTTPBatchProcessor_ParseResponse_NilResult tests parsing nil result
func TestHTTPBatchProcessor_ParseResponse_NilResult(t *testing.T) {
	processor := concurrent.NewHTTPBatchProcessor(nil, "http://example.com")

	var target map[string]any
	err := processor.ParseResponse(nil, "req_1", &target)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch result is nil")
}

// TestHTTPBatchProcessor_ParseResponse_NotFound tests parsing non-existent request
func TestHTTPBatchProcessor_ParseResponse_NotFound(t *testing.T) {
	processor := concurrent.NewHTTPBatchProcessor(nil, "http://example.com")

	result := &concurrent.HTTPBatchResult{
		Responses: []concurrent.HTTPBatchResponse{
			{ID: "req_1", StatusCode: 200},
		},
	}

	var target map[string]any
	err := processor.ParseResponse(result, "req_nonexistent", &target)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no response found")
}

// TestHTTPBatchProcessor_ParseResponse_StatusError tests parsing response with status error
func TestHTTPBatchProcessor_ParseResponse_StatusError(t *testing.T) {
	processor := concurrent.NewHTTPBatchProcessor(nil, "http://example.com")

	result := &concurrent.HTTPBatchResult{
		Responses: []concurrent.HTTPBatchResponse{
			{ID: "req_1", StatusCode: 500},
		},
	}

	var target map[string]any
	err := processor.ParseResponse(result, "req_1", &target)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed with status 500")
}

// TestHTTPBatchOptions tests the functional options
func TestHTTPBatchOptions(t *testing.T) {
	t.Run("DefaultOptions", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()

		assert.Equal(t, 60*time.Second, opts.Timeout)
		assert.Equal(t, 100, opts.MaxBatchSize)
		assert.Equal(t, 3, opts.RetryCount)
		assert.Equal(t, 500*time.Millisecond, opts.RetryBackoff)
		assert.False(t, opts.ContinueOnError)
		assert.Equal(t, 5, opts.Workers)
	})

	t.Run("WithBatchTimeout_Valid", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithBatchTimeout(30 * time.Second)(opts)

		assert.NoError(t, err)
		assert.Equal(t, 30*time.Second, opts.Timeout)
	})

	t.Run("WithBatchTimeout_Invalid", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithBatchTimeout(0)(opts)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout must be positive")
	})

	t.Run("WithBatchTimeout_Negative", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithBatchTimeout(-1 * time.Second)(opts)

		assert.Error(t, err)
	})

	t.Run("WithMaxBatchSize_Valid", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithMaxBatchSize(50)(opts)

		assert.NoError(t, err)
		assert.Equal(t, 50, opts.MaxBatchSize)
	})

	t.Run("WithMaxBatchSize_Invalid", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithMaxBatchSize(0)(opts)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "maxBatchSize must be positive")
	})

	t.Run("WithMaxBatchSize_Negative", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithMaxBatchSize(-10)(opts)

		assert.Error(t, err)
	})

	t.Run("WithBatchRetryCount_Valid", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithBatchRetryCount(5)(opts)

		assert.NoError(t, err)
		assert.Equal(t, 5, opts.RetryCount)
	})

	t.Run("WithBatchRetryCount_Zero", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithBatchRetryCount(0)(opts)

		assert.NoError(t, err)
		assert.Equal(t, 0, opts.RetryCount)
	})

	t.Run("WithBatchRetryCount_Negative", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithBatchRetryCount(-1)(opts)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "retryCount must be non-negative")
	})

	t.Run("WithBatchRetryBackoff_Valid", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithBatchRetryBackoff(100 * time.Millisecond)(opts)

		assert.NoError(t, err)
		assert.Equal(t, 100*time.Millisecond, opts.RetryBackoff)
	})

	t.Run("WithBatchRetryBackoff_Invalid", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithBatchRetryBackoff(0)(opts)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "retryBackoff must be positive")
	})

	t.Run("WithBatchContinueOnError", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithBatchContinueOnError(true)(opts)

		assert.NoError(t, err)
		assert.True(t, opts.ContinueOnError)
	})

	t.Run("WithBatchWorkers_Valid", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithBatchWorkers(10)(opts)

		assert.NoError(t, err)
		assert.Equal(t, 10, opts.Workers)
	})

	t.Run("WithBatchWorkers_Invalid", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithBatchWorkers(0)(opts)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workers must be positive")
	})
}

// TestHTTPBatchProcessor_PresetOptions tests preset option configurations
func TestHTTPBatchProcessor_PresetOptions(t *testing.T) {
	t.Run("WithHighThroughputBatch", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithHighThroughputBatch()(opts)

		assert.NoError(t, err)
		assert.Equal(t, 200, opts.MaxBatchSize)
		assert.Equal(t, 5, opts.RetryCount)
		assert.Equal(t, 10, opts.Workers)
		assert.Equal(t, 120*time.Second, opts.Timeout)
	})

	t.Run("WithLowLatencyBatch", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithLowLatencyBatch()(opts)

		assert.NoError(t, err)
		assert.Equal(t, 25, opts.MaxBatchSize)
		assert.Equal(t, 8, opts.Workers)
		assert.Equal(t, 30*time.Second, opts.Timeout)
		assert.Equal(t, 100*time.Millisecond, opts.RetryBackoff)
	})

	t.Run("WithHighReliabilityBatch", func(t *testing.T) {
		opts := concurrent.DefaultHTTPBatchOptions()
		err := concurrent.WithHighReliabilityBatch()(opts)

		assert.NoError(t, err)
		assert.Equal(t, 7, opts.RetryCount)
		assert.Equal(t, 750*time.Millisecond, opts.RetryBackoff)
		assert.True(t, opts.ContinueOnError)
		assert.Equal(t, 180*time.Second, opts.Timeout)
	})
}

// TestHTTPBatchProcessor_ContextCancellation tests context cancellation
func TestHTTPBatchProcessor_ContextCancellation(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"req_1","statusCode":200}]`))
	}))
	defer server.Close()

	processor := concurrent.NewHTTPBatchProcessor(
		server.Client(),
		server.URL,
		concurrent.WithBatchTimeout(100*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	requests := []concurrent.HTTPBatchRequest{
		{Method: "GET", Path: "/slow", ID: "req_1"},
	}

	_, err := processor.ExecuteBatch(ctx, requests)
	assert.Error(t, err)
}

// TestHTTPBatchProcessor_ServerError tests handling server errors
func TestHTTPBatchProcessor_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	processor := concurrent.NewHTTPBatchProcessor(
		server.Client(),
		server.URL,
		concurrent.WithBatchRetryCount(0), // No retries
	)

	requests := []concurrent.HTTPBatchRequest{
		{Method: "GET", Path: "/error", ID: "req_1"},
	}

	_, err := processor.ExecuteBatch(context.Background(), requests)
	assert.Error(t, err)
}

// TestHTTPBatchProcessor_ClientError tests handling client errors (no retry)
func TestHTTPBatchProcessor_ClientError(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	processor := concurrent.NewHTTPBatchProcessor(
		server.Client(),
		server.URL,
		concurrent.WithBatchRetryCount(3),
	)

	requests := []concurrent.HTTPBatchRequest{
		{Method: "GET", Path: "/bad", ID: "req_1"},
	}

	_, err := processor.ExecuteBatch(context.Background(), requests)
	assert.Error(t, err)
	// Client errors (4xx) should not be retried
	assert.Equal(t, 1, attemptCount)
}

// TestHTTPBatchProcessor_AutoGeneratedIDs tests auto-generation of request IDs
func TestHTTPBatchProcessor_AutoGeneratedIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requests []concurrent.HTTPBatchRequest
		err := json.NewDecoder(r.Body).Decode(&requests)
		require.NoError(t, err)

		// Verify IDs were auto-generated
		for i, req := range requests {
			assert.Equal(t, fmt.Sprintf("req_%d", i), req.ID)
		}

		// Return responses
		responses := make([]concurrent.HTTPBatchResponse, len(requests))
		for i, req := range requests {
			responses[i] = concurrent.HTTPBatchResponse{
				ID:         req.ID,
				StatusCode: 200,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer server.Close()

	processor := concurrent.NewHTTPBatchProcessor(server.Client(), server.URL)

	// Create requests without IDs
	requests := []concurrent.HTTPBatchRequest{
		{Method: "GET", Path: "/test1"},
		{Method: "GET", Path: "/test2"},
		{Method: "GET", Path: "/test3"},
	}

	result, err := processor.ExecuteBatch(context.Background(), requests)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(result.Responses))
}

// TestHTTPBatchProcessor_ContinueOnError tests continue on error behavior
func TestHTTPBatchProcessor_ContinueOnError(t *testing.T) {
	t.Run("ContinueOnError_True", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			responses := []concurrent.HTTPBatchResponse{
				{ID: "req_1", StatusCode: 200},
				{ID: "req_2", StatusCode: 500, Error: "server error"},
				{ID: "req_3", StatusCode: 200},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(responses)
		}))
		defer server.Close()

		processor := concurrent.NewHTTPBatchProcessor(
			server.Client(),
			server.URL,
			concurrent.WithBatchContinueOnError(true),
		)

		requests := []concurrent.HTTPBatchRequest{
			{Method: "GET", Path: "/test1", ID: "req_1"},
			{Method: "GET", Path: "/test2", ID: "req_2"},
			{Method: "GET", Path: "/test3", ID: "req_3"},
		}

		result, err := processor.ExecuteBatch(context.Background(), requests)
		// Should not return error when ContinueOnError is true
		assert.NoError(t, err)
		assert.Equal(t, 3, len(result.Responses))
	})

	t.Run("ContinueOnError_False", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			responses := []concurrent.HTTPBatchResponse{
				{ID: "req_1", StatusCode: 200},
				{ID: "req_2", StatusCode: 500, Error: "server error"},
				{ID: "req_3", StatusCode: 200},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(responses)
		}))
		defer server.Close()

		processor := concurrent.NewHTTPBatchProcessor(
			server.Client(),
			server.URL,
			concurrent.WithBatchContinueOnError(false),
		)

		requests := []concurrent.HTTPBatchRequest{
			{Method: "GET", Path: "/test1", ID: "req_1"},
			{Method: "GET", Path: "/test2", ID: "req_2"},
			{Method: "GET", Path: "/test3", ID: "req_3"},
		}

		result, err := processor.ExecuteBatch(context.Background(), requests)
		// Should return error when ContinueOnError is false
		assert.Error(t, err)
		assert.NotNil(t, result)
	})
}

// TestDefaultJSONMarshaler tests the default JSON marshaler
func TestDefaultJSONMarshaler(t *testing.T) {
	marshaler := &concurrent.DefaultJSONMarshaler{}

	t.Run("Marshal", func(t *testing.T) {
		data := map[string]string{"key": "value"}
		result, err := marshaler.Marshal(data)

		assert.NoError(t, err)
		assert.Contains(t, string(result), "key")
		assert.Contains(t, string(result), "value")
	})

	t.Run("Unmarshal", func(t *testing.T) {
		jsonData := []byte(`{"key": "value"}`)
		var result map[string]string

		err := marshaler.Unmarshal(jsonData, &result)

		assert.NoError(t, err)
		assert.Equal(t, "value", result["key"])
	})

	t.Run("UnmarshalInvalid", func(t *testing.T) {
		jsonData := []byte(`invalid json`)
		var result map[string]string

		err := marshaler.Unmarshal(jsonData, &result)

		assert.Error(t, err)
	})
}

// TestHTTPBatchRequest tests the HTTPBatchRequest struct
func TestHTTPBatchRequest(t *testing.T) {
	t.Run("FullRequest", func(t *testing.T) {
		req := concurrent.HTTPBatchRequest{
			Method: "POST",
			Path:   "/api/v1/resource",
			Headers: map[string]string{
				"X-Custom-Header": "custom-value",
			},
			Body: map[string]any{
				"name": "test",
			},
			ID: "req_123",
		}

		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, "/api/v1/resource", req.Path)
		assert.Equal(t, "custom-value", req.Headers["X-Custom-Header"])
		assert.Equal(t, "test", req.Body.(map[string]any)["name"])
		assert.Equal(t, "req_123", req.ID)
	})
}

// TestHTTPBatchResponse tests the HTTPBatchResponse struct
func TestHTTPBatchResponse(t *testing.T) {
	t.Run("SuccessResponse", func(t *testing.T) {
		resp := concurrent.HTTPBatchResponse{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       json.RawMessage(`{"result": "success"}`),
			ID:         "req_1",
		}

		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Headers["Content-Type"])
		assert.JSONEq(t, `{"result": "success"}`, string(resp.Body))
		assert.Equal(t, "req_1", resp.ID)
		assert.Empty(t, resp.Error)
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		resp := concurrent.HTTPBatchResponse{
			StatusCode: 500,
			Error:      "Internal server error",
			ID:         "req_1",
		}

		assert.Equal(t, 500, resp.StatusCode)
		assert.Equal(t, "Internal server error", resp.Error)
		assert.Equal(t, "req_1", resp.ID)
	})
}

// TestHTTPBatchResult tests the HTTPBatchResult struct
func TestHTTPBatchResult(t *testing.T) {
	t.Run("WithResponses", func(t *testing.T) {
		result := &concurrent.HTTPBatchResult{
			Responses: []concurrent.HTTPBatchResponse{
				{ID: "req_1", StatusCode: 200},
				{ID: "req_2", StatusCode: 201},
			},
		}

		assert.Nil(t, result.Error)
		assert.Len(t, result.Responses, 2)
	})

	t.Run("WithError", func(t *testing.T) {
		expectedErr := fmt.Errorf("batch failed")
		result := &concurrent.HTTPBatchResult{
			Responses: []concurrent.HTTPBatchResponse{},
			Error:     expectedErr,
		}

		assert.Empty(t, result.Responses)
		assert.Equal(t, expectedErr, result.Error)
	})
}

// TestHTTPBatchProcessor_InvalidJSON tests handling of invalid JSON responses
func TestHTTPBatchProcessor_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`invalid json response`))
	}))
	defer server.Close()

	processor := concurrent.NewHTTPBatchProcessor(
		server.Client(),
		server.URL,
		concurrent.WithBatchRetryCount(0),
	)

	requests := []concurrent.HTTPBatchRequest{
		{Method: "GET", Path: "/test", ID: "req_1"},
	}

	_, err := processor.ExecuteBatch(context.Background(), requests)
	assert.Error(t, err)
}

// TestHTTPBatchProcessor_WithExistingTimeout tests behavior with existing context timeout
func TestHTTPBatchProcessor_WithExistingTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responses := []concurrent.HTTPBatchResponse{
			{ID: "req_1", StatusCode: 200},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer server.Close()

	processor := concurrent.NewHTTPBatchProcessor(
		server.Client(),
		server.URL,
		concurrent.WithBatchTimeout(60*time.Second), // Processor timeout
	)

	// Create context with its own deadline
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	requests := []concurrent.HTTPBatchRequest{
		{Method: "GET", Path: "/test", ID: "req_1"},
	}

	result, err := processor.ExecuteBatch(ctx, requests)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestHTTPBatchProcessor_EmptyBatchWithPoolOptions tests empty batch with pool options
func TestHTTPBatchProcessor_EmptyBatchWithPoolOptions(t *testing.T) {
	processor := concurrent.NewHTTPBatchProcessor(nil, "http://example.com")

	result, err := processor.ExecuteBatchWithPoolOptions(
		context.Background(),
		[]concurrent.HTTPBatchRequest{},
		concurrent.WithWorkers(5),
	)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Responses)
}

// TestHTTPBatchProcessorWithRetry tests the HTTPBatchProcessorWithRetry
func TestHTTPBatchProcessorWithRetry(t *testing.T) {
	t.Run("NewHTTPBatchProcessorWithRetry_Success", func(t *testing.T) {
		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(nil, "http://example.com")

		assert.NoError(t, err)
		assert.NotNil(t, processor)
	})

	t.Run("NewHTTPBatchProcessorWithRetry_WithOptions", func(t *testing.T) {
		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(
			nil,
			"http://example.com",
			concurrent.WithBatchRetryCount(5),
			concurrent.WithBatchRetryBackoff(100*time.Millisecond),
		)

		assert.NoError(t, err)
		assert.NotNil(t, processor)
	})

	t.Run("SetJSONMarshaler", func(t *testing.T) {
		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(nil, "http://example.com")
		require.NoError(t, err)

		marshaler := &CustomJSONMarshaler{}
		processor.SetJSONMarshaler(marshaler)

		// No panic means success
	})

	t.Run("SetDefaultHeader", func(t *testing.T) {
		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(nil, "http://example.com")
		require.NoError(t, err)

		processor.SetDefaultHeader("X-Custom-Header", "value")

		// No panic means success
	})

	t.Run("SetDefaultHeaders", func(t *testing.T) {
		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(nil, "http://example.com")
		require.NoError(t, err)

		processor.SetDefaultHeaders(map[string]string{
			"X-Header-1": "value1",
			"X-Header-2": "value2",
		})

		// No panic means success
	})

	t.Run("ExecuteBatch_Empty", func(t *testing.T) {
		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(nil, "http://example.com")
		require.NoError(t, err)

		result, err := processor.ExecuteBatch(context.Background(), []concurrent.HTTPBatchRequest{})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Responses)
	})

	t.Run("ExecuteBatch_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var requests []concurrent.HTTPBatchRequest
			err := json.NewDecoder(r.Body).Decode(&requests)
			require.NoError(t, err)

			responses := make([]concurrent.HTTPBatchResponse, len(requests))
			for i, req := range requests {
				responses[i] = concurrent.HTTPBatchResponse{
					ID:         req.ID,
					StatusCode: 200,
					Body:       []byte(`{"success": true}`),
				}
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(responses)
		}))
		defer server.Close()

		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(server.Client(), server.URL)
		require.NoError(t, err)

		requests := []concurrent.HTTPBatchRequest{
			{Method: "GET", Path: "/test1", ID: "req_1"},
			{Method: "GET", Path: "/test2", ID: "req_2"},
		}

		result, err := processor.ExecuteBatch(context.Background(), requests)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Responses, 2)
	})

	t.Run("ParseResponse_NilResult", func(t *testing.T) {
		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(nil, "http://example.com")
		require.NoError(t, err)

		var target map[string]any
		err = processor.ParseResponse(nil, "req_1", &target)

		assert.Error(t, err)
	})

	t.Run("ParseResponse_Success", func(t *testing.T) {
		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(nil, "http://example.com")
		require.NoError(t, err)

		result := &concurrent.HTTPBatchResult{
			Responses: []concurrent.HTTPBatchResponse{
				{
					ID:         "req_1",
					StatusCode: 200,
					Body:       json.RawMessage(`{"key": "value"}`),
				},
			},
		}

		var target map[string]any
		err = processor.ParseResponse(result, "req_1", &target)

		assert.NoError(t, err)
		assert.Equal(t, "value", target["key"])
	})

	t.Run("ParseResponse_NotFound", func(t *testing.T) {
		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(nil, "http://example.com")
		require.NoError(t, err)

		result := &concurrent.HTTPBatchResult{
			Responses: []concurrent.HTTPBatchResponse{
				{ID: "req_1", StatusCode: 200},
			},
		}

		var target map[string]any
		err = processor.ParseResponse(result, "req_nonexistent", &target)

		assert.Error(t, err)
	})

	t.Run("ParseResponse_WithError", func(t *testing.T) {
		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(nil, "http://example.com")
		require.NoError(t, err)

		result := &concurrent.HTTPBatchResult{
			Responses: []concurrent.HTTPBatchResponse{
				{ID: "req_1", StatusCode: 500, Error: "server error"},
			},
		}

		var target map[string]any
		err = processor.ParseResponse(result, "req_1", &target)

		assert.Error(t, err)
	})

	t.Run("ParseResponse_StatusError", func(t *testing.T) {
		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(nil, "http://example.com")
		require.NoError(t, err)

		result := &concurrent.HTTPBatchResult{
			Responses: []concurrent.HTTPBatchResponse{
				{ID: "req_1", StatusCode: 400},
			},
		}

		var target map[string]any
		err = processor.ParseResponse(result, "req_1", &target)

		assert.Error(t, err)
	})

	t.Run("ExecuteBatch_LargeBatch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var requests []concurrent.HTTPBatchRequest
			err := json.NewDecoder(r.Body).Decode(&requests)
			require.NoError(t, err)

			responses := make([]concurrent.HTTPBatchResponse, len(requests))
			for i, req := range requests {
				responses[i] = concurrent.HTTPBatchResponse{
					ID:         req.ID,
					StatusCode: 200,
				}
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(responses)
		}))
		defer server.Close()

		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(
			server.Client(),
			server.URL,
			concurrent.WithMaxBatchSize(5),
		)
		require.NoError(t, err)

		// Create more requests than MaxBatchSize
		var requests []concurrent.HTTPBatchRequest
		for i := 0; i < 15; i++ {
			requests = append(requests, concurrent.HTTPBatchRequest{
				Method: "GET",
				Path:   fmt.Sprintf("/test%d", i),
				ID:     fmt.Sprintf("req_%d", i),
			})
		}

		result, err := processor.ExecuteBatch(context.Background(), requests)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Responses, 15)
	})

	t.Run("ExecuteBatch_AutoGeneratedIDs", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var requests []concurrent.HTTPBatchRequest
			err := json.NewDecoder(r.Body).Decode(&requests)
			require.NoError(t, err)

			// Verify IDs were auto-generated
			for i, req := range requests {
				assert.Equal(t, fmt.Sprintf("req_%d", i), req.ID)
			}

			responses := make([]concurrent.HTTPBatchResponse, len(requests))
			for i, req := range requests {
				responses[i] = concurrent.HTTPBatchResponse{
					ID:         req.ID,
					StatusCode: 200,
				}
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(responses)
		}))
		defer server.Close()

		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(server.Client(), server.URL)
		require.NoError(t, err)

		// Create requests without IDs
		requests := []concurrent.HTTPBatchRequest{
			{Method: "GET", Path: "/test1"},
			{Method: "GET", Path: "/test2"},
		}

		result, err := processor.ExecuteBatch(context.Background(), requests)

		assert.NoError(t, err)
		assert.Len(t, result.Responses, 2)
	})

	t.Run("ExecuteBatch_WithContextTimeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var requests []concurrent.HTTPBatchRequest
			_ = json.NewDecoder(r.Body).Decode(&requests)

			responses := make([]concurrent.HTTPBatchResponse, len(requests))
			for i, req := range requests {
				responses[i] = concurrent.HTTPBatchResponse{ID: req.ID, StatusCode: 200}
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(responses)
		}))
		defer server.Close()

		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(server.Client(), server.URL)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		requests := []concurrent.HTTPBatchRequest{
			{Method: "GET", Path: "/test", ID: "req_1"},
		}

		result, err := processor.ExecuteBatch(ctx, requests)

		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("ExecuteBatch_ContinueOnError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			responses := []concurrent.HTTPBatchResponse{
				{ID: "req_1", StatusCode: 200},
				{ID: "req_2", StatusCode: 500, Error: "server error"},
				{ID: "req_3", StatusCode: 200},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(responses)
		}))
		defer server.Close()

		processor, err := concurrent.NewHTTPBatchProcessorWithRetry(
			server.Client(),
			server.URL,
			concurrent.WithBatchContinueOnError(true),
		)
		require.NoError(t, err)

		requests := []concurrent.HTTPBatchRequest{
			{Method: "GET", Path: "/test1", ID: "req_1"},
			{Method: "GET", Path: "/test2", ID: "req_2"},
			{Method: "GET", Path: "/test3", ID: "req_3"},
		}

		result, err := processor.ExecuteBatch(context.Background(), requests)

		assert.NoError(t, err)
		assert.Len(t, result.Responses, 3)
	})
}

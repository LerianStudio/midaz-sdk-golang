package concurrent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/pkg/concurrent"
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
		json.NewEncoder(w).Encode(responses)
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
		json.NewEncoder(w).Encode(responses)
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
			w.Write([]byte(`{"error": "internal server error"}`))
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
		json.NewEncoder(w).Encode(responses)
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
		w.Write([]byte(`[{"id":"req_1","statusCode":200,"body":{"test":"value"}}]`))
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

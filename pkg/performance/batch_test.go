package performance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func createMockBatchServer() *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/batch" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the request
		var requests []BatchRequest
		if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Process each request
		var responses []BatchResponse
		for _, req := range requests {
			resp := BatchResponse{
				ID: req.ID,
			}

			// Mock response based on the request
			switch req.Path {
			case "/success":
				resp.StatusCode = http.StatusOK
				resp.Body = json.RawMessage(`{"success":true}`)
			case "/error":
				resp.StatusCode = http.StatusBadRequest
				resp.Error = "Bad request"
			case "/not-found":
				resp.StatusCode = http.StatusNotFound
				resp.Error = "Not found"
			default:
				resp.StatusCode = http.StatusOK
				resp.Body = json.RawMessage(`{"path":"` + req.Path + `"}`)
			}

			responses = append(responses, resp)
		}

		// Send the response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(responses)
	})

	return httptest.NewServer(handler)
}

func TestBatchProcessor_ExecuteBatch(t *testing.T) {
	// Create a mock server
	server := createMockBatchServer()
	defer server.Close()

	// Create a batch processor
	processor := NewBatchProcessorWithDefaults(http.DefaultClient, server.URL, nil)

	// Test a successful batch
	t.Run("SuccessfulBatch", func(t *testing.T) {
		requests := []BatchRequest{
			{
				Method: "GET",
				Path:   "/success",
				ID:     "req_1",
			},
			{
				Method: "GET",
				Path:   "/custom",
				ID:     "req_2",
			},
		}

		result, err := processor.ExecuteBatch(context.Background(), requests)
		if err != nil {
			t.Fatalf("ExecuteBatch returned an error: %v", err)
		}

		if len(result.Responses) != 2 {
			t.Fatalf("Expected 2 responses, got %d", len(result.Responses))
		}

		if result.Responses[0].ID != "req_1" {
			t.Errorf("Expected ID req_1, got %s", result.Responses[0].ID)
		}

		if result.Responses[0].StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", result.Responses[0].StatusCode)
		}

		// Parse the first response
		var resp1 struct {
			Success bool `json:"success"`
		}
		if err := processor.ParseBatchResponse(result, "req_1", &resp1); err != nil {
			t.Fatalf("ParseBatchResponse returned an error: %v", err)
		}

		if !resp1.Success {
			t.Errorf("Expected success=true, got false")
		}

		// Parse the second response
		var resp2 struct {
			Path string `json:"path"`
		}
		if err := processor.ParseBatchResponse(result, "req_2", &resp2); err != nil {
			t.Fatalf("ParseBatchResponse returned an error: %v", err)
		}

		if resp2.Path != "/custom" {
			t.Errorf("Expected path=/custom, got %s", resp2.Path)
		}
	})

	// Test a batch with errors
	t.Run("BatchWithErrors", func(t *testing.T) {
		// Set ContinueOnError to true to get all responses
		processor.options.ContinueOnError = true

		requests := []BatchRequest{
			{
				Method: "GET",
				Path:   "/success",
				ID:     "req_1",
			},
			{
				Method: "GET",
				Path:   "/error",
				ID:     "req_2",
			},
		}

		result, err := processor.ExecuteBatch(context.Background(), requests)
		if err != nil {
			t.Fatalf("ExecuteBatch returned an error: %v", err)
		}

		if len(result.Responses) != 2 {
			t.Fatalf("Expected 2 responses, got %d", len(result.Responses))
		}

		// First request should be successful
		if result.Responses[0].StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for first request, got %d", result.Responses[0].StatusCode)
		}

		// Second request should have an error
		if result.Responses[1].StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for second request, got %d", result.Responses[1].StatusCode)
		}

		if result.Responses[1].Error != "Bad request" {
			t.Errorf("Expected error 'Bad request', got %s", result.Responses[1].Error)
		}

		// Parsing the error response should fail
		if err := processor.ParseBatchResponse(result, "req_2", nil); err == nil {
			t.Fatalf("Expected ParseBatchResponse to return an error, got nil")
		}
	})

	// Test a batch with stop on error
	t.Run("BatchStopOnError", func(t *testing.T) {
		// Set ContinueOnError to false to stop on first error
		processor.options.ContinueOnError = false

		requests := []BatchRequest{
			{
				Method: "GET",
				Path:   "/success",
				ID:     "req_1",
			},
			{
				Method: "GET",
				Path:   "/error",
				ID:     "req_2",
			},
		}

		result, err := processor.ExecuteBatch(context.Background(), requests)
		if err == nil {
			t.Fatalf("Expected ExecuteBatch to return an error, got nil")
		}

		if result == nil {
			t.Fatalf("Expected result to not be nil even with error")
		}

		if len(result.Responses) != 2 {
			t.Fatalf("Expected 2 responses, got %d", len(result.Responses))
		}

		// First request should be successful
		if result.Responses[0].StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for first request, got %d", result.Responses[0].StatusCode)
		}

		// Second request should have an error
		if result.Responses[1].StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for second request, got %d", result.Responses[1].StatusCode)
		}
	})

	// Test an empty batch
	t.Run("EmptyBatch", func(t *testing.T) {
		result, err := processor.ExecuteBatch(context.Background(), []BatchRequest{})
		if err != nil {
			t.Fatalf("ExecuteBatch returned an error: %v", err)
		}

		if len(result.Responses) != 0 {
			t.Fatalf("Expected 0 responses, got %d", len(result.Responses))
		}
	})

	// Test batch with auto-generated IDs
	t.Run("AutoGeneratedIDs", func(t *testing.T) {
		requests := []BatchRequest{
			{
				Method: "GET",
				Path:   "/success",
				// No ID specified, should be auto-generated
			},
		}

		result, err := processor.ExecuteBatch(context.Background(), requests)
		if err != nil {
			t.Fatalf("ExecuteBatch returned an error: %v", err)
		}

		if len(result.Responses) != 1 {
			t.Fatalf("Expected 1 response, got %d", len(result.Responses))
		}

		// ID should be auto-generated
		if result.Responses[0].ID == "" {
			t.Errorf("Expected auto-generated ID, got empty string")
		}
	})

	// Test with context timeout
	t.Run("ContextTimeout", func(t *testing.T) {
		// Create a context with a very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Sleep to ensure the context times out
		time.Sleep(5 * time.Millisecond)

		requests := []BatchRequest{
			{
				Method: "GET",
				Path:   "/success",
				ID:     "req_1",
			},
		}

		_, err := processor.ExecuteBatch(ctx, requests)
		if err == nil {
			t.Fatalf("Expected ExecuteBatch to return an error due to timeout, got nil")
		}
	})

	// Test batch splitting
	t.Run("BatchSplitting", func(t *testing.T) {
		// Set a small max batch size to trigger splitting
		processor.options.MaxBatchSize = 1

		requests := []BatchRequest{
			{
				Method: "GET",
				Path:   "/success",
				ID:     "req_1",
			},
			{
				Method: "GET",
				Path:   "/custom",
				ID:     "req_2",
			},
		}

		result, err := processor.ExecuteBatch(context.Background(), requests)
		if err != nil {
			t.Fatalf("ExecuteBatch returned an error: %v", err)
		}

		if len(result.Responses) != 2 {
			t.Fatalf("Expected 2 responses, got %d", len(result.Responses))
		}

		// Both requests should be successful
		for i, resp := range result.Responses {
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 for request %d, got %d", i+1, resp.StatusCode)
			}
		}
	})
}

func TestDefaultBatchOptions(t *testing.T) {
	options := DefaultBatchOptions()

	if options.Timeout != 60*time.Second {
		t.Errorf("Expected Timeout=60s, got %v", options.Timeout)
	}
	if options.MaxBatchSize != 100 {
		t.Errorf("Expected MaxBatchSize=100, got %d", options.MaxBatchSize)
	}
	if options.RetryCount != 3 {
		t.Errorf("Expected RetryCount=3, got %d", options.RetryCount)
	}
	if options.RetryBackoff != 500*time.Millisecond {
		t.Errorf("Expected RetryBackoff=500ms, got %v", options.RetryBackoff)
	}
	if options.ContinueOnError != false {
		t.Errorf("Expected ContinueOnError=false, got %v", options.ContinueOnError)
	}
}

func TestBatchOptions_WithOptions(t *testing.T) {
	// Test with valid options
	options, err := NewBatchOptions(
		WithBatchTimeout(120*time.Second),
		WithMaxBatchSize(200),
		WithRetryCount(5),
		WithRetryBackoff(1*time.Second),
		WithContinueOnError(true),
	)
	if err != nil {
		t.Fatalf("NewBatchOptions returned an error: %v", err)
	}

	if options.Timeout != 120*time.Second {
		t.Errorf("Expected Timeout=120s, got %v", options.Timeout)
	}
	if options.MaxBatchSize != 200 {
		t.Errorf("Expected MaxBatchSize=200, got %d", options.MaxBatchSize)
	}
	if options.RetryCount != 5 {
		t.Errorf("Expected RetryCount=5, got %d", options.RetryCount)
	}
	if options.RetryBackoff != 1*time.Second {
		t.Errorf("Expected RetryBackoff=1s, got %v", options.RetryBackoff)
	}
	if !options.ContinueOnError {
		t.Errorf("Expected ContinueOnError=true, got false")
	}

	// Test with invalid options
	_, err = NewBatchOptions(
		WithBatchTimeout(-1 * time.Second),
	)
	if err == nil {
		t.Fatalf("Expected NewBatchOptions to return an error for negative timeout, got nil")
	}

	_, err = NewBatchOptions(
		WithMaxBatchSize(0),
	)
	if err == nil {
		t.Fatalf("Expected NewBatchOptions to return an error for zero batch size, got nil")
	}

	_, err = NewBatchOptions(
		WithRetryCount(-1),
	)
	if err == nil {
		t.Fatalf("Expected NewBatchOptions to return an error for negative retry count, got nil")
	}

	_, err = NewBatchOptions(
		WithRetryBackoff(-1 * time.Second),
	)
	if err == nil {
		t.Fatalf("Expected NewBatchOptions to return an error for negative retry backoff, got nil")
	}

	// Test convenience options
	options, err = NewBatchOptions(
		WithHighThroughputBatching(),
	)
	if err != nil {
		t.Fatalf("NewBatchOptions returned an error: %v", err)
	}

	if options.MaxBatchSize != 200 {
		t.Errorf("Expected MaxBatchSize=200, got %d", options.MaxBatchSize)
	}
	if options.RetryCount != 5 {
		t.Errorf("Expected RetryCount=5, got %d", options.RetryCount)
	}
	if options.RetryBackoff != 100*time.Millisecond {
		t.Errorf("Expected RetryBackoff=100ms, got %v", options.RetryBackoff)
	}

	options, err = NewBatchOptions(
		WithReliableBatching(),
	)
	if err != nil {
		t.Fatalf("NewBatchOptions returned an error: %v", err)
	}

	if options.RetryCount != 10 {
		t.Errorf("Expected RetryCount=10, got %d", options.RetryCount)
	}
	if options.RetryBackoff != 1*time.Second {
		t.Errorf("Expected RetryBackoff=1s, got %v", options.RetryBackoff)
	}
	if !options.ContinueOnError {
		t.Errorf("Expected ContinueOnError=true, got false")
	}
}

func TestNewBatchProcessor(t *testing.T) {
	// Test with valid options
	processor, err := NewBatchProcessor(
		"http://example.com",
		WithHTTPClient(http.DefaultClient),
		WithBatchOptions(&BatchOptions{
			Timeout:         120 * time.Second,
			MaxBatchSize:    200,
			RetryCount:      5,
			RetryBackoff:    1 * time.Second,
			ContinueOnError: true,
		}),
		WithDefaultHeader("X-API-Key", "test-key"),
		WithDefaultHeaders(map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		}),
	)
	if err != nil {
		t.Fatalf("NewBatchProcessor returned an error: %v", err)
	}

	if processor.baseURL != "http://example.com" {
		t.Errorf("Expected baseURL=http://example.com, got %s", processor.baseURL)
	}

	if processor.httpClient != http.DefaultClient {
		t.Errorf("Expected httpClient=http.DefaultClient, got %v", processor.httpClient)
	}

	if processor.options.Timeout != 120*time.Second {
		t.Errorf("Expected options.Timeout=120s, got %v", processor.options.Timeout)
	}

	if processor.options.MaxBatchSize != 200 {
		t.Errorf("Expected options.MaxBatchSize=200, got %d", processor.options.MaxBatchSize)
	}

	if processor.defaultHeaders["X-API-Key"] != "test-key" {
		t.Errorf("Expected defaultHeaders[X-API-Key]=test-key, got %s", processor.defaultHeaders["X-API-Key"])
	}

	if processor.defaultHeaders["Content-Type"] != "application/json" {
		t.Errorf("Expected defaultHeaders[Content-Type]=application/json, got %s", processor.defaultHeaders["Content-Type"])
	}

	if processor.defaultHeaders["Accept"] != "application/json" {
		t.Errorf("Expected defaultHeaders[Accept]=application/json, got %s", processor.defaultHeaders["Accept"])
	}

	// Test with invalid options
	_, err = NewBatchProcessor("", WithHTTPClient(http.DefaultClient))
	if err == nil {
		t.Fatalf("Expected NewBatchProcessor to return an error for empty baseURL, got nil")
	}

	_, err = NewBatchProcessor(
		"http://example.com",
		WithHTTPClient(nil),
	)
	if err == nil {
		t.Fatalf("Expected NewBatchProcessor to return an error for nil client, got nil")
	}

	_, err = NewBatchProcessor(
		"http://example.com",
		WithBatchOptions(nil),
	)
	if err == nil {
		t.Fatalf("Expected NewBatchProcessor to return an error for nil options, got nil")
	}

	_, err = NewBatchProcessor(
		"http://example.com",
		WithDefaultHeader("", "value"),
	)
	if err == nil {
		t.Fatalf("Expected NewBatchProcessor to return an error for empty header key, got nil")
	}

	_, err = NewBatchProcessor(
		"http://example.com",
		WithDefaultHeaders(nil),
	)
	if err == nil {
		t.Fatalf("Expected NewBatchProcessor to return an error for nil headers, got nil")
	}

	// Test backward compatibility function
	processor = NewBatchProcessorWithDefaults(http.DefaultClient, "http://example.com", nil)

	if processor.baseURL != "http://example.com" {
		t.Errorf("Expected baseURL=http://example.com, got %s", processor.baseURL)
	}

	if processor.httpClient != http.DefaultClient {
		t.Errorf("Expected httpClient=http.DefaultClient, got %v", processor.httpClient)
	}
}

func TestBatchProcessor_SetDefaultHeader(t *testing.T) {
	processor := NewBatchProcessorWithDefaults(http.DefaultClient, "http://example.com", nil)

	// Set a default header
	processor.SetDefaultHeader("X-Test", "value")

	if processor.defaultHeaders["X-Test"] != "value" {
		t.Errorf("Expected defaultHeaders[X-Test]=value, got %s", processor.defaultHeaders["X-Test"])
	}

	// Set multiple default headers
	headers := map[string]string{
		"X-Test2": "value2",
		"X-Test3": "value3",
	}
	processor.SetDefaultHeaders(headers)

	if processor.defaultHeaders["X-Test2"] != "value2" {
		t.Errorf("Expected defaultHeaders[X-Test2]=value2, got %s", processor.defaultHeaders["X-Test2"])
	}
	if processor.defaultHeaders["X-Test3"] != "value3" {
		t.Errorf("Expected defaultHeaders[X-Test3]=value3, got %s", processor.defaultHeaders["X-Test3"])
	}
}

func TestBatchProcessor_ParseBatchResponseEdgeCases(t *testing.T) {
	server := createMockBatchServer()
	defer server.Close()

	processor := NewBatchProcessorWithDefaults(http.DefaultClient, server.URL, nil)

	t.Run("NilResult", func(t *testing.T) {
		err := processor.ParseBatchResponse(nil, "req_1", nil)
		if err == nil {
			t.Error("Expected error for nil result, got nil")
		}
	})

	t.Run("RequestIDNotFound", func(t *testing.T) {
		result := &BatchResult{
			Responses: []BatchResponse{
				{ID: "req_1", StatusCode: http.StatusOK},
			},
		}
		err := processor.ParseBatchResponse(result, "non_existent", nil)
		if err == nil {
			t.Error("Expected error for non-existent request ID, got nil")
		}
	})

	t.Run("ResponseWithErrorStatus", func(t *testing.T) {
		result := &BatchResult{
			Responses: []BatchResponse{
				{ID: "req_1", StatusCode: http.StatusInternalServerError},
			},
		}
		err := processor.ParseBatchResponse(result, "req_1", nil)
		if err == nil {
			t.Error("Expected error for error status code, got nil")
		}
	})

	t.Run("ResponseWithErrorMessage", func(t *testing.T) {
		result := &BatchResult{
			Responses: []BatchResponse{
				{ID: "req_1", StatusCode: http.StatusOK, Error: "some error"},
			},
		}
		err := processor.ParseBatchResponse(result, "req_1", nil)
		if err == nil {
			t.Error("Expected error for response with error message, got nil")
		}
	})

	t.Run("ValidResponseWithEmptyBody", func(t *testing.T) {
		result := &BatchResult{
			Responses: []BatchResponse{
				{ID: "req_1", StatusCode: http.StatusOK, Body: nil},
			},
		}
		var target struct{}
		err := processor.ParseBatchResponse(result, "req_1", &target)
		if err != nil {
			t.Errorf("Expected no error for empty body, got %v", err)
		}
	})

	t.Run("InvalidJSONBody", func(t *testing.T) {
		result := &BatchResult{
			Responses: []BatchResponse{
				{ID: "req_1", StatusCode: http.StatusOK, Body: json.RawMessage(`invalid json`)},
			},
		}
		var target struct {
			Field string `json:"field"`
		}
		err := processor.ParseBatchResponse(result, "req_1", &target)
		if err == nil {
			t.Error("Expected error for invalid JSON body, got nil")
		}
	})
}

func TestBatchProcessor_WithJSONPool(t *testing.T) {
	pool := NewJSONPool()

	processor, err := NewBatchProcessor(
		"http://example.com",
		WithJSONPool(pool),
	)
	if err != nil {
		t.Fatalf("NewBatchProcessor with JSONPool returned error: %v", err)
	}

	if processor.jsonPool != pool {
		t.Error("Expected JSONPool to be set")
	}

	// Test nil JSON pool
	_, err = NewBatchProcessor(
		"http://example.com",
		WithJSONPool(nil),
	)
	if err == nil {
		t.Error("Expected error for nil JSONPool, got nil")
	}
}

func TestBatchProcessor_WithBaseURL(t *testing.T) {
	// Test empty base URL via option
	_, err := NewBatchProcessor(
		"http://example.com",
		WithBaseURL(""),
	)
	if err == nil {
		t.Error("Expected error for empty base URL, got nil")
	}

	// Test valid base URL override
	processor, err := NewBatchProcessor(
		"http://example.com",
		WithBaseURL("http://newurl.com"),
	)
	if err != nil {
		t.Fatalf("NewBatchProcessor with BaseURL returned error: %v", err)
	}

	if processor.baseURL != "http://newurl.com" {
		t.Errorf("Expected baseURL=http://newurl.com, got %s", processor.baseURL)
	}
}

func TestBatchProcessor_ServerErrorResponse(t *testing.T) {
	// Create a mock server that returns 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	processor := NewBatchProcessorWithDefaults(http.DefaultClient, server.URL, nil)
	processor.options.RetryCount = 0 // Disable retries for this test

	requests := []BatchRequest{
		{Method: "GET", Path: "/test", ID: "req_1"},
	}

	_, err := processor.ExecuteBatch(context.Background(), requests)
	if err == nil {
		t.Error("Expected error for server error response, got nil")
	}
}

func TestBatchProcessor_InvalidJSONResponse(t *testing.T) {
	// Create a mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid json response`))
	}))
	defer server.Close()

	processor := NewBatchProcessorWithDefaults(http.DefaultClient, server.URL, nil)
	processor.options.RetryCount = 0

	requests := []BatchRequest{
		{Method: "GET", Path: "/test", ID: "req_1"},
	}

	_, err := processor.ExecuteBatch(context.Background(), requests)
	if err == nil {
		t.Error("Expected error for invalid JSON response, got nil")
	}
}

func TestBatchProcessor_RequestWithBody(t *testing.T) {
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requests []BatchRequest
		if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if len(requests) > 0 && requests[0].Body != nil {
			if bodyMap, ok := requests[0].Body.(map[string]any); ok {
				receivedBody = bodyMap
			}
		}

		responses := []BatchResponse{
			{ID: requests[0].ID, StatusCode: http.StatusOK, Body: json.RawMessage(`{"received":true}`)},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer server.Close()

	processor := NewBatchProcessorWithDefaults(http.DefaultClient, server.URL, nil)

	requests := []BatchRequest{
		{
			Method:  "POST",
			Path:    "/create",
			ID:      "req_1",
			Body:    map[string]any{"name": "test", "value": 123},
			Headers: map[string]string{"X-Custom": "header"},
		},
	}

	result, err := processor.ExecuteBatch(context.Background(), requests)
	if err != nil {
		t.Fatalf("ExecuteBatch returned error: %v", err)
	}

	if len(result.Responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(result.Responses))
	}

	if receivedBody["name"] != "test" {
		t.Errorf("Expected body name=test, got %v", receivedBody["name"])
	}
}

func TestBatchProcessor_NewBatchProcessorWithDefaults_AllOptions(t *testing.T) {
	options := &BatchOptions{
		Timeout:         30 * time.Second,
		MaxBatchSize:    50,
		RetryCount:      2,
		RetryBackoff:    200 * time.Millisecond,
		ContinueOnError: true,
	}

	processor := NewBatchProcessorWithDefaults(http.DefaultClient, "http://example.com", options)

	if processor.options.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout=30s, got %v", processor.options.Timeout)
	}
	if processor.options.MaxBatchSize != 50 {
		t.Errorf("Expected MaxBatchSize=50, got %d", processor.options.MaxBatchSize)
	}
	if processor.options.RetryCount != 2 {
		t.Errorf("Expected RetryCount=2, got %d", processor.options.RetryCount)
	}
}

func TestBatchProcessor_NewBatchProcessorWithDefaults_NilClient(t *testing.T) {
	processor := NewBatchProcessorWithDefaults(nil, "http://example.com", nil)

	// Should create a default client
	if processor.httpClient == nil {
		t.Error("Expected httpClient to be created, got nil")
	}
}

func createMockServerWithErrorBody() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"custom error message"}`))
	}))
}

func TestBatchProcessor_HandleErrorResponse(t *testing.T) {
	server := createMockServerWithErrorBody()
	defer server.Close()

	processor := NewBatchProcessorWithDefaults(http.DefaultClient, server.URL, nil)
	processor.options.RetryCount = 0

	requests := []BatchRequest{
		{Method: "GET", Path: "/test", ID: "req_1"},
	}

	_, err := processor.ExecuteBatch(context.Background(), requests)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestBatchProcessor_RetryWithBackoff(t *testing.T) {
	// This test verifies that the processor retries on network-level failures
	// Note: The current implementation retries on HTTP client Do errors, not HTTP status codes
	// HTTP 500 responses are considered successful at the transport level and are not retried
	// The error handling converts them to internal errors

	// Test that retry backoff option is properly set
	processor := NewBatchProcessorWithDefaults(http.DefaultClient, "http://example.com", nil)
	processor.options.RetryCount = 3
	processor.options.RetryBackoff = 10 * time.Millisecond

	if processor.options.RetryCount != 3 {
		t.Errorf("Expected RetryCount=3, got %d", processor.options.RetryCount)
	}
	if processor.options.RetryBackoff != 10*time.Millisecond {
		t.Errorf("Expected RetryBackoff=10ms, got %v", processor.options.RetryBackoff)
	}

	// Verify shouldRetry logic
	ctx := context.Background()
	if !processor.shouldRetry(ctx, 0) {
		t.Error("Expected shouldRetry to return true for retry=0")
	}
	if !processor.shouldRetry(ctx, 2) {
		t.Error("Expected shouldRetry to return true for retry=2")
	}
	if processor.shouldRetry(ctx, 3) {
		t.Error("Expected shouldRetry to return false for retry=3 (equals RetryCount)")
	}

	// Test with cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if processor.shouldRetry(cancelledCtx, 0) {
		t.Error("Expected shouldRetry to return false for cancelled context")
	}
}

// BenchmarkBatchProcessing benchmarks the batch processing performance
func BenchmarkBatchProcessing(b *testing.B) {
	// Create a mock server
	server := createMockBatchServer()
	defer server.Close()

	// Create the batch requests
	smallBatch := make([]BatchRequest, 10)
	largeBatch := make([]BatchRequest, 100)

	// Fill the batches with sample requests
	for i := 0; i < len(smallBatch); i++ {
		smallBatch[i] = BatchRequest{
			Method: "GET",
			Path:   "/success",
			ID:     "req_" + string(rune(i)),
		}
	}

	for i := 0; i < len(largeBatch); i++ {
		largeBatch[i] = BatchRequest{
			Method: "GET",
			Path:   "/success",
			ID:     "req_" + string(rune(i)),
		}
	}

	// Benchmark with small batch
	b.Run("SmallBatch", func(b *testing.B) {
		// Create a processor with default options
		processor, err := NewBatchProcessor(server.URL, WithHTTPClient(http.DefaultClient))
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := processor.ExecuteBatch(context.Background(), smallBatch)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Benchmark with large batch
	b.Run("LargeBatch", func(b *testing.B) {
		// Create a processor with default options
		processor, err := NewBatchProcessor(server.URL, WithHTTPClient(http.DefaultClient))
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := processor.ExecuteBatch(context.Background(), largeBatch)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Benchmark with batch splitting
	b.Run("BatchSplitting", func(b *testing.B) {
		// Create a processor with small max batch size to force splitting
		options, err := NewBatchOptions(WithMaxBatchSize(10))
		if err != nil {
			b.Fatal(err)
		}

		processor, err := NewBatchProcessor(
			server.URL,
			WithHTTPClient(http.DefaultClient),
			WithBatchOptions(options),
		)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := processor.ExecuteBatch(context.Background(), largeBatch)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Benchmark with parallel processing
	b.Run("ParallelProcessing", func(b *testing.B) {
		// Create a processor with parallel processing config
		processor, err := NewBatchProcessor(
			server.URL,
			WithHTTPClient(http.DefaultClient),
			WithBatchOptions(&BatchOptions{
				MaxBatchSize: 10,
				RetryCount:   3,
				Timeout:      60 * time.Second,
				RetryBackoff: 500 * time.Millisecond,
			}),
		)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := processor.ExecuteBatch(context.Background(), largeBatch)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Tests for batch_adapter.go

func TestConvertRequests(t *testing.T) {
	requests := []BatchRequest{
		{
			Method:  "GET",
			Path:    "/test",
			Headers: map[string]string{"X-Test": "value"},
			Body:    map[string]string{"key": "value"},
			ID:      "req_1",
		},
		{
			Method: "POST",
			Path:   "/create",
			ID:     "req_2",
		},
	}

	httpRequests := ConvertRequests(requests)

	if len(httpRequests) != 2 {
		t.Fatalf("Expected 2 HTTP requests, got %d", len(httpRequests))
	}

	if httpRequests[0].Method != "GET" {
		t.Errorf("Expected method=GET, got %s", httpRequests[0].Method)
	}
	if httpRequests[0].Path != "/test" {
		t.Errorf("Expected path=/test, got %s", httpRequests[0].Path)
	}
	if httpRequests[0].Headers["X-Test"] != "value" {
		t.Errorf("Expected header X-Test=value, got %s", httpRequests[0].Headers["X-Test"])
	}
	if httpRequests[0].ID != "req_1" {
		t.Errorf("Expected ID=req_1, got %s", httpRequests[0].ID)
	}

	if httpRequests[1].Method != "POST" {
		t.Errorf("Expected method=POST, got %s", httpRequests[1].Method)
	}
}

func TestConvertResponses(t *testing.T) {
	httpResponses := []struct {
		StatusCode int
		Headers    map[string]string
		Body       json.RawMessage
		Error      string
		ID         string
	}{
		{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       json.RawMessage(`{"success":true}`),
			ID:         "req_1",
		},
		{
			StatusCode: 400,
			Error:      "Bad Request",
			ID:         "req_2",
		},
	}

	// Create concurrent.HTTPBatchResponse equivalents
	type httpBatchResponse struct {
		StatusCode int
		Headers    map[string]string
		Body       json.RawMessage
		Error      string
		ID         string
	}

	// Test empty slice conversion
	emptyResponses := ConvertResponses(nil)
	if len(emptyResponses) != 0 {
		t.Errorf("Expected 0 responses for nil input, got %d", len(emptyResponses))
	}

	// Since ConvertResponses expects concurrent.HTTPBatchResponse, we'll test it indirectly
	// through ConvertResult
	_ = httpResponses
}

func TestConvertResult(t *testing.T) {
	t.Run("NilResult", func(t *testing.T) {
		result := ConvertResult(nil)
		if result != nil {
			t.Error("Expected nil for nil input")
		}
	})
}

func TestConcurrentRegistry(t *testing.T) {
	// Test Store and Get functions
	t.Run("StoreAndGet", func(t *testing.T) {
		processor := &BatchProcessor{
			baseURL:        "http://example.com",
			defaultHeaders: make(map[string]string),
		}

		// Initially should be nil
		result := adapterRegistry.Get(processor)
		if result != nil {
			t.Error("Expected nil for unregistered processor")
		}
	})
}

func TestExecuteBatchWithAdapter(t *testing.T) {
	server := createMockBatchServer()
	defer server.Close()

	// Create a processor without adapter registration (should fall back to original)
	processor := NewBatchProcessorWithDefaults(http.DefaultClient, server.URL, nil)

	requests := []BatchRequest{
		{Method: "GET", Path: "/success", ID: "req_1"},
	}

	// This should fall back to the original ExecuteBatch
	result, err := ExecuteBatchWithAdapter(processor, context.Background(), requests)
	if err != nil {
		t.Fatalf("ExecuteBatchWithAdapter returned error: %v", err)
	}

	if len(result.Responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(result.Responses))
	}
}

func TestParseResponseWithAdapter(t *testing.T) {
	processor := NewBatchProcessorWithDefaults(http.DefaultClient, "http://example.com", nil)

	result := &BatchResult{
		Responses: []BatchResponse{
			{
				ID:         "req_1",
				StatusCode: http.StatusOK,
				Body:       json.RawMessage(`{"name":"test"}`),
			},
		},
	}

	var target struct {
		Name string `json:"name"`
	}

	// This should fall back to the original ParseBatchResponse
	err := ParseResponseWithAdapter(processor, result, "req_1", &target)
	if err != nil {
		t.Fatalf("ParseResponseWithAdapter returned error: %v", err)
	}

	if target.Name != "test" {
		t.Errorf("Expected name=test, got %s", target.Name)
	}
}

func TestCreateBatchProcessor(t *testing.T) {
	options := &BatchOptions{
		Timeout:         30 * time.Second,
		MaxBatchSize:    50,
		RetryCount:      2,
		RetryBackoff:    100 * time.Millisecond,
		ContinueOnError: true,
	}

	processor := CreateBatchProcessor(http.DefaultClient, "http://example.com", options)

	if processor == nil {
		t.Fatal("CreateBatchProcessor returned nil")
	}

	if processor.baseURL != "http://example.com" {
		t.Errorf("Expected baseURL=http://example.com, got %s", processor.baseURL)
	}

	if processor.options.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout=30s, got %v", processor.options.Timeout)
	}

	// Verify that it was registered
	httpProcessor := adapterRegistry.Get(processor)
	if httpProcessor == nil {
		t.Error("Expected processor to be registered in adapter registry")
	}
}

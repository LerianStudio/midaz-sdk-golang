package performance

import (
	"context"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
)

// Adapter Functions

// NOTE: These functions adapt the concurrent.HTTPBatchProcessor to the existing
// performance.BatchProcessor interface, allowing existing code to use the new
// implementation without changes.

// CreateBatchProcessor creates a BatchProcessor that uses the concurrent.HTTPBatchProcessor internally.
// This adapter function allows existing code to use the new implementation.
func CreateBatchProcessor(client *http.Client, baseURL string, options *BatchOptions) *BatchProcessor {
	// Create the HTTP batch processor with functional options
	httpBatchProcessor := concurrent.NewHTTPBatchProcessor(
		client,
		baseURL,
		concurrent.WithBatchTimeout(options.Timeout),
		concurrent.WithMaxBatchSize(options.MaxBatchSize),
		concurrent.WithBatchRetryCount(options.RetryCount),
		concurrent.WithBatchRetryBackoff(options.RetryBackoff),
		concurrent.WithBatchContinueOnError(options.ContinueOnError),
		concurrent.WithBatchWorkers(5), // Default value
	)

	// Create and initialize the batch processor
	processor := &BatchProcessor{
		httpClient:     client,
		baseURL:        baseURL,
		defaultHeaders: make(map[string]string),
		options:        options,
	}

	// Store the HTTP batch processor in the adapter data
	adapterRegistry.Store(processor, httpBatchProcessor)

	return processor
}

// adapterRegistry is a map of BatchProcessor instances to their underlying HTTPBatchProcessor
var adapterRegistry = &concurrentRegistry{
	processors: make(map[*BatchProcessor]*concurrent.HTTPBatchProcessor),
}

// concurrentRegistry is a registry of BatchProcessor instances and their underlying HTTPBatchProcessor
type concurrentRegistry struct {
	processors map[*BatchProcessor]*concurrent.HTTPBatchProcessor
}

// Store adds a mapping from a BatchProcessor to its underlying HTTPBatchProcessor
func (r *concurrentRegistry) Store(processor *BatchProcessor, httpProcessor *concurrent.HTTPBatchProcessor) {
	r.processors[processor] = httpProcessor
}

// Get returns the HTTPBatchProcessor for a given BatchProcessor
func (r *concurrentRegistry) Get(processor *BatchProcessor) *concurrent.HTTPBatchProcessor {
	return r.processors[processor]
}

// ConvertRequests converts BatchRequest to HTTPBatchRequest
func ConvertRequests(requests []BatchRequest) []concurrent.HTTPBatchRequest {
	httpRequests := make([]concurrent.HTTPBatchRequest, len(requests))
	for i, req := range requests {
		httpRequests[i] = concurrent.HTTPBatchRequest{
			Method:  req.Method,
			Path:    req.Path,
			Headers: req.Headers,
			Body:    req.Body,
			ID:      req.ID,
		}
	}
	return httpRequests
}

// ConvertResponses converts HTTPBatchResponse to BatchResponse
func ConvertResponses(httpResponses []concurrent.HTTPBatchResponse) []BatchResponse {
	responses := make([]BatchResponse, len(httpResponses))
	for i, resp := range httpResponses {
		responses[i] = BatchResponse{
			StatusCode: resp.StatusCode,
			Headers:    resp.Headers,
			Body:       resp.Body,
			Error:      resp.Error,
			ID:         resp.ID,
		}
	}
	return responses
}

// ConvertResult converts HTTPBatchResult to BatchResult
func ConvertResult(httpResult *concurrent.HTTPBatchResult) *BatchResult {
	if httpResult == nil {
		return nil
	}

	return &BatchResult{
		Responses: ConvertResponses(httpResult.Responses),
		Error:     httpResult.Error,
	}
}

// Adapter Methods

// ExecuteBatchWithAdapter executes a batch of requests using the HTTPBatchProcessor
func ExecuteBatchWithAdapter(processor *BatchProcessor, ctx context.Context, requests []BatchRequest) (*BatchResult, error) {
	// Get the HTTP batch processor
	httpBatchProcessor := adapterRegistry.Get(processor)
	if httpBatchProcessor == nil {
		// Fall back to original implementation if adapter not found
		return processor.ExecuteBatch(ctx, requests)
	}

	// Convert requests to HTTP batch requests
	httpRequests := ConvertRequests(requests)

	// Execute the batch
	httpResult, err := httpBatchProcessor.ExecuteBatch(ctx, httpRequests)
	if err != nil {
		return nil, err
	}

	// Convert the result
	return ConvertResult(httpResult), nil
}

// ParseResponseWithAdapter parses a batch response using the HTTPBatchProcessor
func ParseResponseWithAdapter(processor *BatchProcessor, result *BatchResult, requestID string, target any) error {
	// Get the HTTP batch processor
	httpBatchProcessor := adapterRegistry.Get(processor)
	if httpBatchProcessor == nil {
		// Fall back to original implementation if adapter not found
		return processor.ParseBatchResponse(result, requestID, target)
	}

	// Convert the result to HTTP batch result
	httpResult := &concurrent.HTTPBatchResult{
		Responses: make([]concurrent.HTTPBatchResponse, len(result.Responses)),
		Error:     result.Error,
	}

	for i, resp := range result.Responses {
		httpResult.Responses[i] = concurrent.HTTPBatchResponse{
			StatusCode: resp.StatusCode,
			Headers:    resp.Headers,
			Body:       resp.Body,
			Error:      resp.Error,
			ID:         resp.ID,
		}
	}

	// Parse the response
	return httpBatchProcessor.ParseResponse(httpResult, requestID, target)
}

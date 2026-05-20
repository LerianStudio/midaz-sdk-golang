package concurrent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/concurrent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPBatchProcessor_DuplicatePerItemIdempotencyAcrossChunksFailsBeforeSend(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	processor := concurrent.NewHTTPBatchProcessor(
		server.Client(),
		server.URL,
		concurrent.WithMaxBatchSize(1),
		concurrent.WithBatchRetryCount(1),
	)

	_, err := processor.ExecuteBatch(context.Background(), duplicateUnsafeBatchRequests())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reuses X-Idempotency")
	assert.Equal(t, int32(0), calls.Load())
}

func TestHTTPBatchProcessor_DuplicatePerItemIdempotencyAcrossPoolChunksFailsBeforeSend(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	processor := concurrent.NewHTTPBatchProcessor(
		server.Client(),
		server.URL,
		concurrent.WithMaxBatchSize(1),
		concurrent.WithBatchRetryCount(1),
	)

	_, err := processor.ExecuteBatchWithPoolOptions(context.Background(), duplicateUnsafeBatchRequests(), concurrent.WithWorkers(2))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reuses X-Idempotency")
	assert.Equal(t, int32(0), calls.Load())
}

func TestHTTPBatchProcessorWithRetry_DuplicatePerItemIdempotencyAcrossChunksFailsBeforeSend(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	processor, err := concurrent.NewHTTPBatchProcessorWithRetry(
		server.Client(),
		server.URL,
		concurrent.WithMaxBatchSize(1),
		concurrent.WithBatchRetryCount(1),
	)
	require.NoError(t, err)

	_, err = processor.ExecuteBatch(context.Background(), duplicateUnsafeBatchRequests())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reuses X-Idempotency")
	assert.Equal(t, int32(0), calls.Load())
}

func duplicateUnsafeBatchRequests() []concurrent.HTTPBatchRequest {
	return []concurrent.HTTPBatchRequest{
		{Method: http.MethodPost, Path: "/transactions", ID: "a", Headers: map[string]string{"X-Idempotency": "same-key"}},
		{Method: http.MethodPost, Path: "/transactions", ID: "b", Headers: map[string]string{"X-Idempotency": "same-key"}},
	}
}

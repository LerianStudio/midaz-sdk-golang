// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package concurrent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/concurrent"
	"github.com/stretchr/testify/require"
)

// TestHTTPBatchProcessor_SetDefaultHeaderConcurrentWithExecute pins the
// concurrent-write contract on HTTPBatchProcessor.defaultHeaders:
// multiple goroutines calling SetDefaultHeader while another goroutine
// runs ExecuteBatch must not race on the underlying map. The processor
// guards the map with a sync.RWMutex; this test exists so a future
// refactor that drops the mutex (or replaces it with a raw map write)
// surfaces under `go test -race`.
//
// Run with `go test -race ./pkg/concurrent/...` to exercise the
// detector.
func TestHTTPBatchProcessor_SetDefaultHeaderConcurrentWithExecute(t *testing.T) {
	server := newBatchEchoServer(t)
	defer server.Close()

	processor := concurrent.NewHTTPBatchProcessor(server.Client(), server.URL)

	requests := []concurrent.HTTPBatchRequest{
		{ID: "r1", Method: http.MethodGet, Path: "/echo"},
	}

	const writers = 32

	const writesPerGoroutine = 64

	const executors = 8

	const executesPerGoroutine = 16

	var wg sync.WaitGroup

	wg.Add(writers + executors)

	// Writers: spam SetDefaultHeader from N goroutines.
	for w := 0; w < writers; w++ {
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < writesPerGoroutine; i++ {
				processor.SetDefaultHeader(
					fmt.Sprintf("X-Worker-%d-%d", workerID, i),
					fmt.Sprintf("value-%d-%d", workerID, i),
				)
			}
		}(w)
	}

	// Executors: run ExecuteBatch concurrently. Any failure here is
	// reported back through t.Error so the race detector + the explicit
	// error both surface to the test runner.
	for e := 0; e < executors; e++ {
		go func() {
			defer wg.Done()

			ctx := context.Background()
			for i := 0; i < executesPerGoroutine; i++ {
				if _, err := processor.ExecuteBatch(ctx, requests); err != nil {
					t.Errorf("ExecuteBatch failed under concurrent SetDefaultHeader: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()
}

// TestHTTPBatchProcessorWithRetry_SetDefaultHeaderConcurrentWithExecute
// mirrors the previous test for the *WithRetry adapter, which carries
// its own defaultHeaders map + mutex.
func TestHTTPBatchProcessorWithRetry_SetDefaultHeaderConcurrentWithExecute(t *testing.T) {
	server := newBatchEchoServer(t)
	defer server.Close()

	processor, err := concurrent.NewHTTPBatchProcessorWithRetry(server.Client(), server.URL)
	require.NoError(t, err)

	requests := []concurrent.HTTPBatchRequest{
		{ID: "r1", Method: http.MethodGet, Path: "/echo"},
	}

	const writers = 32

	const writesPerGoroutine = 64

	const executors = 8

	const executesPerGoroutine = 16

	var wg sync.WaitGroup

	wg.Add(writers + executors)

	for w := 0; w < writers; w++ {
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < writesPerGoroutine; i++ {
				processor.SetDefaultHeader(
					fmt.Sprintf("X-Worker-%d-%d", workerID, i),
					fmt.Sprintf("value-%d-%d", workerID, i),
				)
			}
		}(w)
	}

	for e := 0; e < executors; e++ {
		go func() {
			defer wg.Done()

			ctx := context.Background()
			for i := 0; i < executesPerGoroutine; i++ {
				if _, err := processor.ExecuteBatch(ctx, requests); err != nil {
					t.Errorf("ExecuteBatch failed under concurrent SetDefaultHeader: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()
}

// newBatchEchoServer stands up a minimal /batch endpoint that returns a
// 200 response for every request in the batch. The handler is
// concurrency-safe (it only reads request headers and writes a static
// body) so it never bottlenecks the regression test.
func newBatchEchoServer(t *testing.T) *httptest.Server {
	t.Helper()

	var hits atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)

		var requests []concurrent.HTTPBatchRequest
		if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		responses := make([]concurrent.HTTPBatchResponse, len(requests))
		for i, req := range requests {
			responses[i] = concurrent.HTTPBatchResponse{
				ID:         req.ID,
				StatusCode: http.StatusOK,
				Body:       json.RawMessage(`{"ok":true}`),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
	}))

	return srv
}

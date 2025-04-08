package concurrent

import (
	"context"
	"testing"
	"time"
)

// BenchmarkWorkerPool benchmarks the worker pool with different worker counts
func BenchmarkWorkerPool(b *testing.B) {
	// Create test items
	const numItems = 1000
	items := make([]int, numItems)
	for i := range items {
		items[i] = i
	}

	// Simple worker function that does minimal work
	workFn := func(ctx context.Context, item int) (int, error) {
		// Simulate a small amount of work
		time.Sleep(50 * time.Microsecond)
		return item * 2, nil
	}

	// Benchmarks with different worker counts
	benchmarkCases := []struct {
		name       string
		numWorkers int
	}{
		{"Workers1", 1},
		{"Workers2", 2},
		{"Workers4", 4},
		{"Workers8", 8},
		{"Workers16", 16},
		{"Workers32", 32},
		{"Workers64", 64},
		{"Workers128", 128},
	}

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				WorkerPool(
					context.Background(),
					items,
					workFn,
					WithWorkers(bc.numWorkers),
					WithBufferSize(numItems),
				)
			}
		})
	}
}

// BenchmarkBatchProcessing benchmarks batch processing with different batch sizes
func BenchmarkBatchProcessing(b *testing.B) {
	// Create test items
	const numItems = 1000
	items := make([]int, numItems)
	for i := range items {
		items[i] = i
	}

	// Simple batch function that does minimal work
	batchFn := func(ctx context.Context, batch []int) ([]int, error) {
		// Simulate a small amount of work per batch
		time.Sleep(100 * time.Microsecond)
		result := make([]int, len(batch))
		for i, item := range batch {
			result[i] = item * 2
		}
		return result, nil
	}

	// Benchmarks with different batch sizes
	benchmarkCases := []struct {
		name      string
		batchSize int
	}{
		{"BatchSize1", 1},
		{"BatchSize10", 10},
		{"BatchSize25", 25},
		{"BatchSize50", 50},
		{"BatchSize100", 100},
		{"BatchSize250", 250},
		{"BatchSize500", 500},
		{"BatchSize1000", 1000},
	}

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Batch(
					context.Background(),
					items,
					bc.batchSize,
					batchFn,
					WithWorkers(8), // Fixed worker count
				)
			}
		})
	}
}

// BenchmarkForEach benchmarks the forEach function with different worker counts
func BenchmarkForEach(b *testing.B) {
	// Create test items
	const numItems = 1000
	items := make([]int, numItems)
	for i := range items {
		items[i] = i
	}

	// Simple forEach function that does minimal work
	forEachFn := func(ctx context.Context, item int) error {
		// Simulate a small amount of work
		time.Sleep(50 * time.Microsecond)
		return nil
	}

	// Benchmarks with different worker counts
	benchmarkCases := []struct {
		name       string
		numWorkers int
	}{
		{"Workers1", 1},
		{"Workers4", 4},
		{"Workers16", 16},
		{"Workers64", 64},
	}

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ForEach(
					context.Background(),
					items,
					forEachFn,
					WithWorkers(bc.numWorkers),
					WithBufferSize(numItems),
				)
			}
		})
	}
}

// BenchmarkRateLimiter benchmarks the rate limiter with different rates
func BenchmarkRateLimiter(b *testing.B) {
	benchmarkCases := []struct {
		name      string
		opsPerSec int
		burst     int
	}{
		{"Rate10Burst1", 10, 1},
		{"Rate100Burst10", 100, 10},
		{"Rate1000Burst100", 1000, 100},
		{"Rate10000Burst1000", 10000, 1000},
	}

	const numOps = 100

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				func() {
					rl := NewRateLimiter(bc.opsPerSec, bc.burst)
					defer rl.Stop()

					ctx := context.Background()
					for j := 0; j < numOps; j++ {
						rl.Wait(ctx)
					}
				}()
			}
		})
	}
}

package performance_test

import (
	"bytes"
	"fmt"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/performance"
)

// ExampleGetOptimalBatchSize captures a surprising behavior: this helper does
// NOT clamp at maxBatchSize. It walks down from maxBatchSize searching for a
// divisor of totalCount so that every batch is the same size. When totalCount
// has no divisor in (1, maxBatchSize] — for example, a prime — the helper
// can return a value far smaller than expected. Always use it for "make
// progress reporting tidy", never as an upper bound on batch size.
func ExampleGetOptimalBatchSize() {
	// 10000 is divisible by 200 → returns 200 (50 batches of 200).
	fmt.Println(performance.GetOptimalBatchSize(10000, 200))

	// 9900 is not divisible by 200, but 9900 / 198 = 50 exactly → 198.
	fmt.Println(performance.GetOptimalBatchSize(9900, 200))

	// 9997 = 13 × 769. The largest divisor ≤ 200 is 13, so the function
	// returns 13 — 769 batches of 13. Surprising if you assumed ≈ 200.
	fmt.Println(performance.GetOptimalBatchSize(9997, 200))

	// totalCount ≤ maxBatchSize short-circuits to totalCount.
	fmt.Println(performance.GetOptimalBatchSize(50, 200))
	// Output:
	// 200
	// 198
	// 13
	// 50
}

// ExampleMarshal documents the trailing-newline quirk of the pooled JSON
// marshaller. Internally it delegates to *json.Encoder, which always appends
// "\n" — unlike encoding/json.Marshal which does not. Comparing the output
// byte-for-byte to a string literal, or hashing it as a signature payload,
// will silently break if you swap encoding/json.Marshal for this function
// without trimming the newline.
func ExampleMarshal() {
	b, err := performance.Marshal(map[string]int{"a": 1})
	if err != nil {
		fmt.Println("err:", err)
		return
	}
	// %q makes the trailing newline visible.
	fmt.Printf("%q\n", string(b))
	fmt.Println("trailing newline:", bytes.HasSuffix(b, []byte("\n")))
	// Output:
	// "{\"a\":1}\n"
	// trailing newline: true
}

// ExampleNewBatchOptions shows the option that matters when ExecuteBatch
// receives more requests than MaxBatchSize: the request set is split into
// chunks and dispatched concurrently. Without WithBatchWorkers, the default
// is 10; a misconfigured zero/negative value falls back to 10 inside
// executeBatches. A 100k-request payload at MaxBatchSize=100 would otherwise
// invite a 1000-goroutine fan-out — Workers bounds it.
func ExampleNewBatchOptions() {
	opts, err := performance.NewBatchOptions(
		performance.WithMaxBatchSize(100),
		performance.WithBatchWorkers(4),
		performance.WithBatchTimeout(30*time.Second),
		performance.WithRetryCount(2),
		performance.WithContinueOnError(true),
	)
	if err != nil {
		fmt.Println("err:", err)
		return
	}
	// In real code: pass opts via WithBatchOptions to NewBatchProcessor and
	// call processor.ExecuteBatch(ctx, requests).
	fmt.Println("max-batch:", opts.MaxBatchSize)
	fmt.Println("workers:", opts.Workers)
	fmt.Println("continue-on-error:", opts.ContinueOnError)
	// Output:
	// max-batch: 100
	// workers: 4
	// continue-on-error: true
}

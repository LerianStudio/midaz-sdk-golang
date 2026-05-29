package concurrent_test

import (
	"context"
	"fmt"
	"sort"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/concurrent"
)

// ExampleWorkerPool demonstrates parallel processing of a slice of items with
// bounded concurrency. The pool returns one Result per input item, preserving
// input order by default; pair WithUnorderedResults with a sort step (as
// below) when you only need the values.
func ExampleWorkerPool() {
	ctx := context.Background()

	accountIDs := []string{"acc-1", "acc-2", "acc-3", "acc-4"}

	results := concurrent.WorkerPool(ctx, accountIDs,
		func(_ context.Context, id string) (string, error) {
			// In real code: client.Entity.Accounts.Get(ctx, orgID, ledgerID, id).
			return "balance:" + id, nil
		},
		concurrent.WithWorkers(2),
		concurrent.WithBufferSize(8),
	)

	values := make([]string, 0, len(results))
	for _, r := range results {
		if r.Error != nil {
			continue
		}
		values = append(values, r.Value)
	}
	sort.Strings(values)
	for _, v := range values {
		fmt.Println(v)
	}
	// Output:
	// balance:acc-1
	// balance:acc-2
	// balance:acc-3
	// balance:acc-4
}

// ExampleBatch shows how to fan out a large slice into fixed-size chunks
// processed by a bulk API. Each input item is reported back individually in
// the result slice, with errors propagated per item if the batch call fails.
func ExampleBatch() {
	ctx := context.Background()

	transactionIDs := []int{1, 2, 3, 4, 5}

	results := concurrent.Batch(ctx, transactionIDs, 2,
		func(_ context.Context, batch []int) ([]string, error) {
			// In real code: client.Entity.Transactions.BulkCreate(ctx, batch).
			out := make([]string, len(batch))
			for i, id := range batch {
				out[i] = fmt.Sprintf("tx-%d:ok", id)
			}
			return out, nil
		},
		concurrent.WithWorkers(2),
	)

	values := make([]string, 0, len(results))
	for _, r := range results {
		if r.Error != nil {
			continue
		}
		values = append(values, r.Value)
	}
	sort.Strings(values)
	for _, v := range values {
		fmt.Println(v)
	}
	// Output:
	// tx-1:ok
	// tx-2:ok
	// tx-3:ok
	// tx-4:ok
	// tx-5:ok
}

// ExampleNewRateLimiter demonstrates the standalone RateLimiter, which is
// useful when a single shared budget must be enforced across many goroutines
// that are not co-managed by a single WorkerPool. Always defer Stop() so the
// background ticker goroutine exits.
func ExampleNewRateLimiter() {
	rl := concurrent.NewRateLimiter(1000, 50)
	defer rl.Stop()

	ctx := context.Background()

	if err := rl.Wait(ctx); err != nil {
		fmt.Println("canceled:", err)
		return
	}
	// In real code: issue the rate-limited call here.
	fmt.Println("token acquired")
	// Output: token acquired
}

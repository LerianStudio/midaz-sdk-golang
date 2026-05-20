package concurrent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWorkerPoolPanicReturnsPerItemError(t *testing.T) {
	done := make(chan []Result[int, int], 1)

	go func() {
		done <- WorkerPool(context.Background(), []int{1, 2}, func(_ context.Context, item int) (int, error) {
			if item == 2 {
				panic("boom")
			}

			return item * 10, nil
		}, WithWorkers(2))
	}()

	select {
	case results := <-done:
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].Error != nil || results[0].Value != 10 || results[0].Index != 0 {
			t.Fatalf("unexpected first result: %+v", results[0])
		}
		if results[1].Error == nil || !strings.Contains(results[1].Error.Error(), "worker panic at index 1") {
			t.Fatalf("expected panic error for second result, got %+v", results[1])
		}
		if results[1].Item != 2 || results[1].Index != 1 {
			t.Fatalf("panic result lost item/index identity: %+v", results[1])
		}
	case <-time.After(time.Second):
		t.Fatal("worker panic path hung")
	}
}

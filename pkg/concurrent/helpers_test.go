package concurrent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchAccountsInParallel(t *testing.T) {
	t.Run("SuccessfulFetch", func(t *testing.T) {
		accountIDs := []string{"acc_1", "acc_2", "acc_3"}

		fetchFn := func(_ context.Context, accountID string) (*models.Account, error) {
			return &models.Account{
				ID:   accountID,
				Name: "Account " + accountID,
			}, nil
		}

		result, err := FetchAccountsInParallel(context.Background(), fetchFn, accountIDs)

		require.NoError(t, err)
		assert.Len(t, result, 3)

		for _, id := range accountIDs {
			assert.Contains(t, result, id)
			assert.Equal(t, id, result[id].ID)
		}
	})

	t.Run("WithError", func(t *testing.T) {
		accountIDs := []string{"acc_1", "acc_2", "acc_3"}
		expectedErr := errors.New("fetch error")

		fetchFn := func(_ context.Context, accountID string) (*models.Account, error) {
			if accountID == "acc_2" {
				return nil, expectedErr
			}

			return &models.Account{ID: accountID}, nil
		}

		result, err := FetchAccountsInParallel(context.Background(), fetchFn, accountIDs)

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		// Some accounts may have been fetched before error
		assert.NotNil(t, result)
	})

	t.Run("EmptyAccountIDs", func(t *testing.T) {
		fetchFn := func(_ context.Context, accountID string) (*models.Account, error) {
			return &models.Account{ID: accountID}, nil
		}

		result, err := FetchAccountsInParallel(context.Background(), fetchFn, []string{})

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("WithOptions", func(t *testing.T) {
		accountIDs := []string{"acc_1", "acc_2"}

		fetchFn := func(_ context.Context, accountID string) (*models.Account, error) {
			return &models.Account{ID: accountID}, nil
		}

		result, err := FetchAccountsInParallel(
			context.Background(),
			fetchFn,
			accountIDs,
			WithWorkers(10),
			WithUnorderedResults(),
		)

		require.NoError(t, err)
		assert.Len(t, result, 2)
	})
}

func TestBatchCreateAccounts(t *testing.T) {
	t.Run("SuccessfulBatchCreate", func(t *testing.T) {
		accounts := []*models.Account{
			{Name: "Account 1"},
			{Name: "Account 2"},
			{Name: "Account 3"},
			{Name: "Account 4"},
			{Name: "Account 5"},
		}

		createBatchFn := func(_ context.Context, batch []*models.Account) ([]*models.Account, error) {
			result := make([]*models.Account, len(batch))
			for i, acc := range batch {
				result[i] = &models.Account{
					ID:   "created_" + acc.Name,
					Name: acc.Name,
				}
			}

			return result, nil
		}

		result, err := BatchCreateAccounts(
			context.Background(),
			createBatchFn,
			accounts,
			2, // Batch size of 2
		)

		require.NoError(t, err)
		assert.Len(t, result, 5)
	})

	t.Run("WithError", func(t *testing.T) {
		accounts := []*models.Account{
			{Name: "Account 1"},
			{Name: "Account 2"},
		}
		expectedErr := errors.New("create error")

		createBatchFn := func(_ context.Context, _ []*models.Account) ([]*models.Account, error) {
			return nil, expectedErr
		}

		result, err := BatchCreateAccounts(
			context.Background(),
			createBatchFn,
			accounts,
			2,
		)

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.NotNil(t, result)
	})

	t.Run("EmptyAccounts", func(t *testing.T) {
		createBatchFn := func(_ context.Context, batch []*models.Account) ([]*models.Account, error) {
			return batch, nil
		}

		result, err := BatchCreateAccounts(
			context.Background(),
			createBatchFn,
			[]*models.Account{},
			10,
		)

		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestProcessTransactionsInParallel(t *testing.T) {
	t.Run("SuccessfulProcess", func(t *testing.T) {
		transactions := []*models.Transaction{
			{ID: "TX001", Description: "Transaction 1"},
			{ID: "TX002", Description: "Transaction 2"},
			{ID: "TX003", Description: "Transaction 3"},
		}

		processFn := func(_ context.Context, tx *models.Transaction) (*models.Transaction, error) {
			// Simulate processing
			return &models.Transaction{
				ID:          tx.ID,
				Description: "Processed: " + tx.ID,
			}, nil
		}

		result, errs := ProcessTransactionsInParallel(
			context.Background(),
			processFn,
			transactions,
		)

		assert.Len(t, result, 3)
		assert.Len(t, errs, 3)

		for _, err := range errs {
			require.NoError(t, err)
		}
	})

	t.Run("WithErrors", func(t *testing.T) {
		transactions := []*models.Transaction{
			{ID: "TX001"},
			{ID: "TX002"},
			{ID: "TX003"},
		}
		expectedErr := errors.New("process error")

		processFn := func(_ context.Context, tx *models.Transaction) (*models.Transaction, error) {
			if tx.ID == "TX002" {
				return nil, expectedErr
			}

			return tx, nil
		}

		result, errs := ProcessTransactionsInParallel(
			context.Background(),
			processFn,
			transactions,
		)

		assert.Len(t, result, 3)
		assert.Len(t, errs, 3)

		// Check that at least one error exists
		hasError := false

		for _, err := range errs {
			if err != nil {
				hasError = true
				break
			}
		}

		assert.True(t, hasError)
	})

	t.Run("EmptyTransactions", func(t *testing.T) {
		processFn := func(_ context.Context, tx *models.Transaction) (*models.Transaction, error) {
			return tx, nil
		}

		result, errs := ProcessTransactionsInParallel(
			context.Background(),
			processFn,
			[]*models.Transaction{},
		)

		assert.Empty(t, result)
		assert.Empty(t, errs)
	})

	t.Run("WithOptions", func(t *testing.T) {
		transactions := []*models.Transaction{
			{ID: "TX001"},
			{ID: "TX002"},
		}

		processFn := func(_ context.Context, tx *models.Transaction) (*models.Transaction, error) {
			return tx, nil
		}

		result, errs := ProcessTransactionsInParallel(
			context.Background(),
			processFn,
			transactions,
			WithWorkers(5),
			WithBufferSize(10),
		)

		assert.Len(t, result, 2)
		assert.Len(t, errs, 2)
	})
}

func TestBulkFetchResourceMap(t *testing.T) {
	t.Run("SuccessfulFetch", func(t *testing.T) {
		resourceIDs := []string{"res_1", "res_2", "res_3"}

		fetchFn := func(_ context.Context, id string) (string, error) {
			return "value_" + id, nil
		}

		result, err := BulkFetchResourceMap(
			context.Background(),
			fetchFn,
			resourceIDs,
		)

		require.NoError(t, err)
		assert.Len(t, result, 3)
		assert.Equal(t, "value_res_1", result["res_1"])
		assert.Equal(t, "value_res_2", result["res_2"])
		assert.Equal(t, "value_res_3", result["res_3"])
	})

	t.Run("WithIntKeys", func(t *testing.T) {
		resourceIDs := []int{1, 2, 3}

		fetchFn := func(_ context.Context, _ int) (string, error) {
			return "value_for_int", nil
		}

		result, err := BulkFetchResourceMap(
			context.Background(),
			fetchFn,
			resourceIDs,
		)

		require.NoError(t, err)
		assert.Len(t, result, 3)
	})

	t.Run("WithError", func(t *testing.T) {
		resourceIDs := []string{"res_1", "res_2"}
		expectedErr := errors.New("fetch error")

		fetchFn := func(_ context.Context, id string) (string, error) {
			if id == "res_2" {
				return "", expectedErr
			}

			return "value_" + id, nil
		}

		result, err := BulkFetchResourceMap(
			context.Background(),
			fetchFn,
			resourceIDs,
		)

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.NotNil(t, result)
	})

	t.Run("EmptyResourceIDs", func(t *testing.T) {
		fetchFn := func(_ context.Context, id string) (string, error) {
			return "value_" + id, nil
		}

		result, err := BulkFetchResourceMap(
			context.Background(),
			fetchFn,
			[]string{},
		)

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("WithOptions", func(t *testing.T) {
		resourceIDs := []string{"res_1", "res_2"}

		fetchFn := func(_ context.Context, id string) (string, error) {
			return "value_" + id, nil
		}

		result, err := BulkFetchResourceMap(
			context.Background(),
			fetchFn,
			resourceIDs,
			WithWorkers(3),
			WithUnorderedResults(),
		)

		require.NoError(t, err)
		assert.Len(t, result, 2)
	})
}

//nolint:revive // cognitive-complexity: comprehensive test with many sub-tests
func TestRunConcurrentOperations(t *testing.T) {
	t.Run("AllSuccessful", func(t *testing.T) {
		var counter int32

		operations := []func(context.Context) error{
			func(_ context.Context) error {
				atomic.AddInt32(&counter, 1)
				return nil
			},
			func(_ context.Context) error {
				atomic.AddInt32(&counter, 1)
				return nil
			},
			func(_ context.Context) error {
				atomic.AddInt32(&counter, 1)
				return nil
			},
		}

		errs := RunConcurrentOperations(context.Background(), operations)

		assert.Len(t, errs, 3)

		for _, err := range errs {
			require.NoError(t, err)
		}

		assert.Equal(t, int32(3), atomic.LoadInt32(&counter))
	})

	t.Run("WithErrors", func(t *testing.T) {
		expectedErr := errors.New("operation error")

		operations := []func(context.Context) error{
			func(_ context.Context) error {
				return nil
			},
			func(_ context.Context) error {
				return expectedErr
			},
			func(_ context.Context) error {
				return nil
			},
		}

		errs := RunConcurrentOperations(context.Background(), operations)

		assert.Len(t, errs, 3)
		require.NoError(t, errs[0])
		assert.Equal(t, expectedErr, errs[1])
		require.NoError(t, errs[2])
	})

	t.Run("EmptyOperations", func(t *testing.T) {
		errs := RunConcurrentOperations(context.Background(), []func(context.Context) error{})

		assert.Empty(t, errs)
	})

	t.Run("WithContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		var (
			started int32
			mu      sync.Mutex
		)

		completed := make([]bool, 3)

		operations := []func(context.Context) error{
			func(ctx context.Context) error {
				atomic.AddInt32(&started, 1)

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(100 * time.Millisecond):
					mu.Lock()

					completed[0] = true

					mu.Unlock()

					return nil
				}
			},
			func(ctx context.Context) error {
				atomic.AddInt32(&started, 1)

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(100 * time.Millisecond):
					mu.Lock()

					completed[1] = true

					mu.Unlock()

					return nil
				}
			},
			func(ctx context.Context) error {
				atomic.AddInt32(&started, 1)

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(100 * time.Millisecond):
					mu.Lock()

					completed[2] = true

					mu.Unlock()

					return nil
				}
			},
		}

		// Cancel after a short delay
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		errs := RunConcurrentOperations(ctx, operations)

		assert.Len(t, errs, 3)
		// All operations should have started
		assert.Equal(t, int32(3), atomic.LoadInt32(&started))
		// At least one should have context cancellation error
		hasContextErr := false

		for _, err := range errs {
			if errors.Is(err, context.Canceled) {
				hasContextErr = true
				break
			}
		}

		assert.True(t, hasContextErr)
	})

	t.Run("SingleOperation", func(t *testing.T) {
		var executed bool

		operations := []func(context.Context) error{
			func(_ context.Context) error {
				executed = true
				return nil
			},
		}

		errs := RunConcurrentOperations(context.Background(), operations)

		assert.Len(t, errs, 1)
		require.NoError(t, errs[0])
		assert.True(t, executed)
	})

	t.Run("AllFailing", func(t *testing.T) {
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")
		err3 := errors.New("error 3")

		operations := []func(context.Context) error{
			func(_ context.Context) error { return err1 },
			func(_ context.Context) error { return err2 },
			func(_ context.Context) error { return err3 },
		}

		errs := RunConcurrentOperations(context.Background(), operations)

		assert.Len(t, errs, 3)
		assert.Equal(t, err1, errs[0])
		assert.Equal(t, err2, errs[1])
		assert.Equal(t, err3, errs[2])
	})

	t.Run("ConcurrentExecution", func(t *testing.T) {
		// Verify operations run concurrently
		var (
			maxConcurrent int32
			current       int32
		)

		operations := make([]func(context.Context) error, 10)
		for i := 0; i < 10; i++ {
			operations[i] = func(_ context.Context) error {
				c := atomic.AddInt32(&current, 1)
				// Track max concurrent
				for {
					currentMax := atomic.LoadInt32(&maxConcurrent)
					if c <= currentMax || atomic.CompareAndSwapInt32(&maxConcurrent, currentMax, c) {
						break
					}
				}

				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&current, -1)

				return nil
			}
		}

		errs := RunConcurrentOperations(context.Background(), operations)

		assert.Len(t, errs, 10)
		// Should have had multiple operations running concurrently
		assert.Greater(t, atomic.LoadInt32(&maxConcurrent), int32(1))
	})
}

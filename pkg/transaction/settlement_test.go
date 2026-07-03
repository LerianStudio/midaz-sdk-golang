// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package transaction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeBalanceReader is a hand mock of balanceReader whose per-call response is
// driven by fn(callNumber). It counts calls so tests can assert the poller does
// not read again after a match.
type fakeBalanceReader struct {
	calls int
	fn    func(call int) (*models.ListResponse[models.Balance], error)
}

func (f *fakeBalanceReader) ListAccountBalances(_ context.Context, _, _, _ string, _ models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
	f.calls++
	return f.fn(f.calls)
}

func balancePage(b ...models.Balance) *models.ListResponse[models.Balance] {
	return &models.ListResponse[models.Balance]{Items: b}
}

// settledAtV2 treats a balance as settled once its Version has advanced to >= 2.
func settledAtV2(b models.Balance) bool { return b.Version >= 2 }

func TestWaitForSettlement_PollsUntilSettled(t *testing.T) {
	r := &fakeBalanceReader{fn: func(call int) (*models.ListResponse[models.Balance], error) {
		if call < 3 {
			return balancePage(models.Balance{ID: "b-1", Version: 1}), nil
		}
		return balancePage(models.Balance{ID: "b-settled", Version: 2, AssetCode: "USD"}), nil
	}}

	got, err := WaitForSettlement(context.Background(), r, "org", "ledger", "acc", settledAtV2,
		WithPollInterval(time.Millisecond), WithMaxInterval(2*time.Millisecond), WithTimeout(time.Second))

	require.NoError(t, err)
	assert.Equal(t, "b-settled", got.ID, "must return the balance that satisfied the predicate")
	assert.Equal(t, int64(2), got.Version)
	assert.Equal(t, "USD", got.AssetCode)
	assert.Equal(t, 3, r.calls, "should stop reading once the predicate matched")
}

func TestWaitForSettlement_ReturnsMatchingBalanceAmongMany(t *testing.T) {
	// Money-path regression guard: a page where Items[0] is NOT settled and
	// Items[1] IS. REDs on a regression to `return page.Items[0]`.
	r := &fakeBalanceReader{fn: func(int) (*models.ListResponse[models.Balance], error) {
		return balancePage(
			models.Balance{ID: "b-unsettled", Version: 1},
			models.Balance{ID: "b-settled", Version: 2},
		), nil
	}}

	got, err := WaitForSettlement(context.Background(), r, "org", "ledger", "acc", settledAtV2,
		WithPollInterval(time.Millisecond), WithTimeout(time.Second))

	require.NoError(t, err)
	assert.Equal(t, "b-settled", got.ID, "must return the balance that satisfied the predicate, not Items[0]")
}

func TestWaitForSettlement_NormalizesDeadlineDuringRead(t *testing.T) {
	// Deadline already elapsed → the read returns a wrapped DeadlineExceeded.
	// The read-error path must normalize it to ErrSettlementTimeout, matching
	// the sleep-select path. REDs on the raw-return.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	r := &fakeBalanceReader{fn: func(int) (*models.ListResponse[models.Balance], error) {
		return nil, context.DeadlineExceeded
	}}

	_, err := WaitForSettlement(ctx, r, "org", "ledger", "acc", settledAtV2,
		WithPollInterval(time.Millisecond), WithTimeout(time.Second))

	require.ErrorIs(t, err, ErrSettlementTimeout)
}

func TestWaitForSettlement_NilPredicateReturnsError(t *testing.T) {
	r := &fakeBalanceReader{fn: func(int) (*models.ListResponse[models.Balance], error) {
		return balancePage(), nil
	}}

	_, err := WaitForSettlement(context.Background(), r, "org", "ledger", "acc", nil)

	require.ErrorContains(t, err, "predicate must not be nil")
	assert.Equal(t, 0, r.calls, "nil predicate must be rejected before any read")
}

func TestWaitForSettlement_TimesOutWhenNeverSettled(t *testing.T) {
	r := &fakeBalanceReader{fn: func(int) (*models.ListResponse[models.Balance], error) {
		return balancePage(models.Balance{ID: "b-1", Version: 1}), nil
	}}

	_, err := WaitForSettlement(context.Background(), r, "org", "ledger", "acc", settledAtV2,
		WithPollInterval(time.Millisecond), WithTimeout(20*time.Millisecond))

	require.ErrorIs(t, err, ErrSettlementTimeout)
}

func TestWaitForSettlement_ReturnsContextErrorOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the first backoff

	r := &fakeBalanceReader{fn: func(int) (*models.ListResponse[models.Balance], error) {
		return balancePage(models.Balance{ID: "b-1", Version: 1}), nil
	}}

	// Long timeout so ErrSettlementTimeout cannot win the race — the cancel must.
	_, err := WaitForSettlement(ctx, r, "org", "ledger", "acc", settledAtV2,
		WithPollInterval(time.Millisecond), WithTimeout(time.Second))

	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, ErrSettlementTimeout)
}

func TestWaitForSettlement_NoExtraReadAfterImmediateMatch(t *testing.T) {
	r := &fakeBalanceReader{fn: func(int) (*models.ListResponse[models.Balance], error) {
		return balancePage(models.Balance{ID: "b-1", Version: 2}), nil
	}}

	got, err := WaitForSettlement(context.Background(), r, "org", "ledger", "acc", settledAtV2,
		WithPollInterval(time.Millisecond), WithTimeout(time.Second))

	require.NoError(t, err)
	assert.Equal(t, int64(2), got.Version)
	assert.Equal(t, 1, r.calls, "a first-read match must not trigger any further read")
}

func TestWaitForSettlement_PropagatesReadError(t *testing.T) {
	wantErr := errors.New("transport boom")
	r := &fakeBalanceReader{fn: func(int) (*models.ListResponse[models.Balance], error) {
		return nil, wantErr
	}}

	_, err := WaitForSettlement(context.Background(), r, "org", "ledger", "acc", settledAtV2,
		WithPollInterval(time.Millisecond), WithTimeout(time.Second))

	require.ErrorIs(t, err, wantErr)
}

// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package transaction

import (
	"context"
	"errors"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
)

// balanceReader is the narrow slice of the Balances accessor WaitForSettlement
// needs. client.Balances satisfies it structurally, so callers pass c.Balances
// with no adapter.
type balanceReader interface {
	ListAccountBalances(ctx context.Context, orgID, ledgerID, accountID string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error)
}

// ErrSettlementTimeout is returned by WaitForSettlement when its deadline
// elapses before the settled predicate matched any of the account's balances.
var ErrSettlementTimeout = errors.New("wait for settlement: timed out before balance settled")

// SettlementOption tunes WaitForSettlement's polling behavior.
type SettlementOption func(*settlementConfig)

type settlementConfig struct {
	pollInterval time.Duration
	maxInterval  time.Duration
	timeout      time.Duration
}

// WithPollInterval sets the initial delay between balance reads (default 500ms).
// The delay grows exponentially up to WithMaxInterval.
func WithPollInterval(d time.Duration) SettlementOption {
	return func(c *settlementConfig) { c.pollInterval = d }
}

// WithMaxInterval caps the exponential backoff between reads (default 5s).
func WithMaxInterval(d time.Duration) SettlementOption {
	return func(c *settlementConfig) { c.maxInterval = d }
}

// WithTimeout sets the overall deadline before WaitForSettlement gives up with
// ErrSettlementTimeout (default 30s). The caller's ctx cancellation still wins.
func WithTimeout(d time.Duration) SettlementOption {
	return func(c *settlementConfig) { c.timeout = d }
}

// WaitForSettlement polls an account's balances until the caller-supplied
// settled predicate matches one, then returns that Balance.
//
// It exists because an accepted transaction (HTTP 201) is NOT settled: 201
// means the create was recorded, not that the ledger balance reflects it. This
// waits on the BALANCE EFFECT, not the transaction status. The settled
// predicate is caller-supplied — the SDK does not hardcode a settlement model
// the server does not dictate (e.g. wait on Version advancing, on Available
// crossing a threshold, or on OnHold releasing).
//
// It reads via ListAccountBalances with bounded-exponential backoff between
// polls (WithPollInterval start, doubling up to WithMaxInterval) until:
//   - the predicate matches one of the returned balances → returns that Balance;
//   - the caller's ctx is cancelled → returns ctx.Err();
//   - the WithTimeout deadline elapses → returns ErrSettlementTimeout.
//
// A read error from ListAccountBalances is returned immediately (not retried —
// transport-level retry is the SDK client's job, not this poller's).
//
// ponytail: polls one ListAccountBalances page (default opts); for an account
// whose settling balance sits beyond the first page, pass a predicate that
// matches within it or widen the page via the accessor. No pagination here
// until a real multi-page account needs it.
func WaitForSettlement(
	ctx context.Context,
	r balanceReader,
	orgID, ledgerID, accountID string,
	settled func(models.Balance) bool,
	opts ...SettlementOption,
) (models.Balance, error) {
	cfg := settlementConfig{
		pollInterval: 500 * time.Millisecond,
		maxInterval:  5 * time.Second,
		timeout:      30 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	interval := cfg.pollInterval

	for {
		page, err := r.ListAccountBalances(ctx, orgID, ledgerID, accountID, models.BalancesListOpts{})
		if err != nil {
			return models.Balance{}, err
		}

		if page != nil {
			for _, b := range page.Items {
				if settled(b) {
					return b, nil
				}
			}
		}

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return models.Balance{}, ErrSettlementTimeout
			}

			return models.Balance{}, ctx.Err()
		case <-time.After(interval):
		}

		if interval *= 2; interval > cfg.maxInterval {
			interval = cfg.maxInterval
		}
	}
}

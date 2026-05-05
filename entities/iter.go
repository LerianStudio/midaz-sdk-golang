// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Apache-2.0

package entities

import "iter"

// Collect drains up to maxItems items from a paginated iter.Seq2[T, error]
// into a slice and short-circuits on the first error.
//
// # Why this helper exists
//
// Go 1.26 made iter.Seq2 the stdlib idiom for paginated streams. The
// SDK exposes ListAll(...) iter.Seq2[T, error] on every entity for
// item-level iteration; Collect provides a one-line bridge for callers
// who need a slice and a known cap.
//
// The maxItems argument is a hard cap, not a target. The function
// returns at the moment one of three things happens:
//   - len(out) == maxItems  → bounded success
//   - the iterator yields an error → returns (partial, err)
//   - the iterator stops yielding   → bounded success with len(out) ≤ maxItems
//
// # When to use Collect vs. CollectAll vs. raw iter.Seq2
//
//   - Use Collect when the result set is unbounded server-side (transactions
//     across an organization, operations on a hot account) and you need a
//     hard ceiling to keep memory bounded.
//   - Use CollectAll only when you know the result set is genuinely small
//     (e.g. a list of asset codes for a specific ledger) and the cost of
//     accidentally draining a much larger set would be acceptable.
//   - Use raw iter.Seq2 when the caller streams items into a downstream
//     pipeline (database write, message bus emit) without ever materializing
//     the full slice.
//
// # Error semantics
//
// On error, the returned slice contains every successfully-yielded item
// up to that point. Callers can use partial results for diagnostics or
// retry-from-cursor patterns. The error wraps any context cancellation
// signaled by the underlying transport.
//
// # Example
//
//	accounts, err := entities.Collect(
//	    client.Accounts.ListAll(ctx, orgID, ledgerID, opts),
//	    1000,
//	)
//	if err != nil {
//	    return fmt.Errorf("failed to collect accounts: %w", err)
//	}
//
// Collect is generic over T so the same helper works for every entity
// the SDK exposes.
//
// A maxItems value ≤ 0 returns an empty slice without consuming the
// iterator. Pass [math.MaxInt] (or [CollectAll]) when no cap is desired.
func Collect[T any](seq iter.Seq2[T, error], maxItems int) ([]T, error) {
	if maxItems <= 0 {
		return nil, nil
	}

	out := make([]T, 0, minCap(maxItems, 64))

	for item, err := range seq {
		if err != nil {
			return out, err
		}

		out = append(out, item)
		if len(out) >= maxItems {
			break
		}
	}

	return out, nil
}

// CollectAll drains every item from a paginated iter.Seq2[T, error]
// into a slice. It short-circuits on the first error and returns
// (partial, err).
//
// # Caution
//
// CollectAll places no upper bound on result-set size. For unbounded
// endpoints (transactions, operations, balances on a high-volume
// ledger) this will load the entire history into memory and is
// almost certainly the wrong choice. Prefer [Collect] with an
// explicit cap, or stream the iter.Seq2 directly.
//
// CollectAll is appropriate for endpoints where the result set is
// known to be small by domain construction — for example a list of
// asset types defined on a specific ledger.
//
// # Example
//
//	assetTypes, err := entities.CollectAll(
//	    client.AccountTypes.ListAll(ctx, orgID, ledgerID, opts),
//	)
//	if err != nil {
//	    return fmt.Errorf("failed to collect asset types: %w", err)
//	}
func CollectAll[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var out []T

	for item, err := range seq {
		if err != nil {
			return out, err
		}

		out = append(out, item)
	}

	return out, nil
}

// minCap returns the smaller of a and b, with both clamped to ≥ 0.
// Used to bound the initial capacity of a Collect-allocated slice so
// that an unbounded maxItems argument does not force a huge upfront
// allocation.
func minCap(a, b int) int {
	if a < 0 {
		a = 0
	}

	if b < 0 {
		b = 0
	}

	if a < b {
		return a
	}

	return b
}

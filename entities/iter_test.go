// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"errors"
	"iter"
	"math"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pair is a single (item, error) tick of a synthetic iter.Seq2.
type pair struct {
	v   int
	err error
}

// seqOf builds a synthetic iter.Seq2[int, error] from a fixed slice of
// (item, error) pairs. Used to exercise Collect/CollectAll without
// hitting any HTTP transport.
func seqOf(pairs []pair) iter.Seq2[int, error] {
	return func(yield func(int, error) bool) {
		for _, p := range pairs {
			if !yield(p.v, p.err) {
				return
			}

			if p.err != nil {
				return
			}
		}
	}
}

// assertCollectResult is a single-purpose assertion helper used by
// TestCollect and TestCollectAll. Extracting the assertion keeps the
// table-driven test bodies under the project lint cognitive-complexity
// budget (max 20) without sacrificing test coverage breadth.
func assertCollectResult(t *testing.T, got []int, err error, want []int, wantErr string) {
	t.Helper()

	if wantErr == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	} else if err == nil || err.Error() != wantErr {
		t.Fatalf("expected error %q, got %v", wantErr, err)
	}

	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d (got=%v)", len(got), len(want), got)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestCollect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []pair
		maxItems int
		want     []int
		wantErr  string
	}{
		{
			name:     "drains under cap",
			input:    []pair{{v: 1}, {v: 2}, {v: 3}},
			maxItems: 10,
			want:     []int{1, 2, 3},
		},
		{
			name:     "stops at cap",
			input:    []pair{{v: 1}, {v: 2}, {v: 3}, {v: 4}, {v: 5}},
			maxItems: 3,
			want:     []int{1, 2, 3},
		},
		{
			name:     "stops on error with partial results",
			input:    []pair{{v: 1}, {v: 2}, {err: errors.New("transport blew up")}},
			maxItems: 100,
			want:     []int{1, 2},
			wantErr:  "transport blew up",
		},
		{
			name:     "zero cap returns empty",
			input:    []pair{{v: 1}},
			maxItems: 0,
			want:     nil,
		},
		{
			name:     "negative cap returns empty",
			input:    []pair{{v: 1}},
			maxItems: -5,
			want:     nil,
		},
		{
			name:     "MaxInt cap drains everything",
			input:    []pair{{v: 7}, {v: 8}, {v: 9}},
			maxItems: math.MaxInt,
			want:     []int{7, 8, 9},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Collect(seqOf(tt.input), tt.maxItems)
			assertCollectResult(t, got, err, tt.want, tt.wantErr)
		})
	}
}

func TestCollectNilSequence(t *testing.T) {
	t.Parallel()

	items, err := Collect[int](nil, 10)
	require.NoError(t, err)
	assert.Empty(t, items)

	allItems, err := CollectAll[int](nil)
	require.NoError(t, err)
	assert.Empty(t, allItems)
}

func TestCollectAll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []pair
		want    []int
		wantErr string
	}{
		{
			name:  "drains everything",
			input: []pair{{v: 10}, {v: 20}, {v: 30}},
			want:  []int{10, 20, 30},
		},
		{
			name:  "empty sequence",
			input: nil,
			want:  nil,
		},
		{
			name:    "stops on error with partial results",
			input:   []pair{{v: 1}, {err: errors.New("page 2 failed")}},
			want:    []int{1},
			wantErr: "page 2 failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CollectAll(seqOf(tt.input))
			assertCollectResult(t, got, err, tt.want, tt.wantErr)
		})
	}
}

func TestMinCap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b, want int
	}{
		{a: 5, b: 10, want: 5},
		{a: 10, b: 5, want: 5},
		{a: 0, b: 100, want: 0},
		{a: -5, b: 10, want: 0},  // clamped
		{a: 100, b: -1, want: 0}, // clamped
		{a: -1, b: -1, want: 0},  // both clamped
	}

	for _, tt := range tests {
		got := minCap(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("minCap(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// pageOrErr models a single tick of a synthetic page-iter feeding flattenPages.
// Either Page is non-nil (success), or Err is non-nil (transport failure), or
// both are nil (the contract for "skip this page" — flattenPages must not
// yield anything for a nil page and must continue to the next entry).
type pageOrErr struct {
	Page *models.ListResponse[int]
	Err  error
}

// pagesSeqOf builds an iter.Seq2[*models.ListResponse[int], error] from a
// fixed slice of page-or-error ticks. Lets us drive flattenPages directly
// without spinning up an HTTP server, isolating the page→item flatten logic
// from any transport concerns.
func pagesSeqOf(ticks []pageOrErr) iter.Seq2[*models.ListResponse[int], error] {
	return func(yield func(*models.ListResponse[int], error) bool) {
		for _, t := range ticks {
			if !yield(t.Page, t.Err) {
				return
			}
			// On error, the upstream iter.Seq2 contract is "stop yielding".
			if t.Err != nil {
				return
			}
		}
	}
}

// TestFlattenPages_HappyPathMultiPage covers the dominant path: every page
// yields, every item flows through, no error, no early termination.
func TestFlattenPages_HappyPathMultiPage(t *testing.T) {
	t.Parallel()

	ticks := []pageOrErr{
		{Page: &models.ListResponse[int]{Items: []int{1, 2, 3}}},
		{Page: &models.ListResponse[int]{Items: []int{4, 5}}},
		{Page: &models.ListResponse[int]{Items: []int{6}}},
	}

	var got []int

	for item, err := range flattenPages(pagesSeqOf(ticks)) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got = append(got, item)
	}

	want := []int{1, 2, 3, 4, 5, 6}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d (got=%v)", len(got), len(want), got)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%d, want %d", i, got[i], want[i])
		}
	}
}

// TestFlattenPages_NilPageIsSkipped covers the nil-page branch in
// flattenPages. A nil *ListResponse must not produce a yield (it would
// panic on page.Items dereference) — flattenPages must just skip it
// and move to the next page. This is the regression for the source
// iterator yielding a nil page on a transient empty response.
func TestFlattenPages_NilPageIsSkipped(t *testing.T) {
	t.Parallel()

	ticks := []pageOrErr{
		{Page: &models.ListResponse[int]{Items: []int{1}}},
		{Page: nil}, // must be skipped
		{Page: &models.ListResponse[int]{Items: []int{2, 3}}},
	}

	var got []int

	for item, err := range flattenPages(pagesSeqOf(ticks)) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got = append(got, item)
	}

	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d (got=%v)", len(got), len(want), got)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%d, want %d", i, got[i], want[i])
		}
	}
}

// TestFlattenPages_ErrorPropagates covers the error branch in
// flattenPages. When the upstream pages iterator yields (nil, err), the
// flattened iterator must surface (zero, err) to the caller and stop —
// not panic, not produce a phantom item.
func TestFlattenPages_ErrorPropagates(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("transport blew up")

	ticks := []pageOrErr{
		{Page: &models.ListResponse[int]{Items: []int{1, 2}}},
		{Err: transportErr},
		// Anything after the error must be unreachable.
		{Page: &models.ListResponse[int]{Items: []int{99}}},
	}

	var (
		got     []int
		gotErr  error
		yielded int
	)

	for item, err := range flattenPages(pagesSeqOf(ticks)) {
		yielded++

		if err != nil {
			gotErr = err

			// On error contract: T is the zero value.
			if item != 0 {
				t.Fatalf("expected zero item on error, got %d", item)
			}

			break
		}

		got = append(got, item)
	}

	if gotErr == nil {
		t.Fatal("expected error to propagate")
	}

	if !errors.Is(gotErr, transportErr) && gotErr.Error() != transportErr.Error() {
		t.Fatalf("expected transport error, got %v", gotErr)
	}

	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("expected partial results [1 2] before error, got %v", got)
	}

	// We yielded two items + one error tick = 3 yields. Anything beyond
	// is the post-error page, which must not have been reached.
	if yielded != 3 {
		t.Fatalf("expected 3 yields (2 items + 1 error), got %d", yielded)
	}
}

// TestFlattenPages_EarlyYieldFalseStops covers the `if !yield(item, nil)
// { return }` branch in flattenPages. When the consumer returns false
// from the yield closure (e.g. a `break` inside `for ... range`), the
// flattenPages loop must stop — no further item from the current page,
// no further page consumption.
func TestFlattenPages_EarlyYieldFalseStops(t *testing.T) {
	t.Parallel()

	// Track whether the second page is ever pulled. If early termination
	// works, we should stop before the source iterator advances past
	// page 1's first item.
	var page2Touched bool

	ticks := iter.Seq2[*models.ListResponse[int], error](func(yield func(*models.ListResponse[int], error) bool) {
		if !yield(&models.ListResponse[int]{Items: []int{1, 2, 3}}, nil) {
			return
		}

		page2Touched = true
		yield(&models.ListResponse[int]{Items: []int{4, 5}}, nil)
	})

	var got []int

	for item, err := range flattenPages(ticks) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got = append(got, item)

		// Stop after first item.
		break
	}

	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("expected [1] before break, got %v", got)
	}

	if page2Touched {
		t.Fatal("page 2 must not have been requested after caller broke out of loop")
	}
}

// TestFlattenPages_EmptyPagesProduceNothing covers a defensive edge:
// pages with empty Items must not surface phantom items. flattenPages
// just ranges over Items (empty range = no yield), so this should pass
// trivially — but pinning it as a test guards against accidental
// "always yield once per page" regressions.
func TestFlattenPages_EmptyPagesProduceNothing(t *testing.T) {
	t.Parallel()

	ticks := []pageOrErr{
		{Page: &models.ListResponse[int]{Items: []int{}}},
		{Page: &models.ListResponse[int]{Items: nil}},
		{Page: &models.ListResponse[int]{Items: []int{42}}},
	}

	var got []int

	for item, err := range flattenPages(pagesSeqOf(ticks)) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got = append(got, item)
	}

	if len(got) != 1 || got[0] != 42 {
		t.Fatalf("expected single item 42 after two empty pages, got %v", got)
	}
}

// TestFlattenPages_ZeroPagesProduceNothing covers the absolute edge:
// an upstream that never yields anything. flattenPages must not panic
// and must not produce any items.
func TestFlattenPages_ZeroPagesProduceNothing(t *testing.T) {
	t.Parallel()

	empty := iter.Seq2[*models.ListResponse[int], error](func(_ func(*models.ListResponse[int], error) bool) {})

	count := 0

	for range flattenPages(empty) {
		count++
	}

	if count != 0 {
		t.Fatalf("expected zero items from empty pages iter, got %d", count)
	}
}

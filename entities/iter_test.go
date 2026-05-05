// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Apache-2.0

package entities

import (
	"errors"
	"iter"
	"math"
	"testing"
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

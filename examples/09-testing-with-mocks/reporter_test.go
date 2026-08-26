package main

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// mockAccountSource is a hand-written test double for the consumer-defined
// accountSource interface. The idiomatic pattern is a tiny local mock
// over your own narrow interface — no generated mocks, no SDK test deps.
type mockAccountSource struct {
	allFn        func(ctx context.Context, orgID, ledgerID string, opts models.AccountsListOpts) iter.Seq2[models.Account, error]
	getByAliasFn func(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error)
}

func (m *mockAccountSource) All(ctx context.Context, orgID, ledgerID string, opts models.AccountsListOpts) iter.Seq2[models.Account, error] {
	return m.allFn(ctx, orgID, ledgerID, opts)
}

func (m *mockAccountSource) GetByAlias(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error) {
	return m.getByAliasFn(ctx, orgID, ledgerID, alias)
}

// TestCountAccounts_Success demonstrates the canonical mock-based test:
// stub the interface method, return canned data, assert the result.
func TestCountAccounts_Success(t *testing.T) {
	// Build a synthetic iter.Seq2 yielding three accounts. In real tests
	// you'd often build this from a slice via a helper.
	accounts := []models.Account{
		{ID: "acc_1", Name: "Account A"},
		{ID: "acc_2", Name: "Account B"},
		{ID: "acc_3", Name: "Account C"},
	}
	stream := func(yield func(models.Account, error) bool) {
		for _, a := range accounts {
			if !yield(a, nil) {
				return
			}
		}
	}

	svc := &mockAccountSource{
		allFn: func(_ context.Context, _, _ string, _ models.AccountsListOpts) iter.Seq2[models.Account, error] {
			return stream
		},
	}

	r := NewAccountReporter(svc)
	got, err := r.CountAccounts(context.Background(), "org_test", "ledger_test")
	if err != nil {
		t.Fatalf("CountAccounts: %v", err)
	}
	if got != 3 {
		t.Errorf("CountAccounts = %d, want 3", got)
	}
}

// TestCountAccounts_StreamError shows error propagation: the SDK's
// iter.Seq2 yields a non-nil error mid-stream and our reporter wraps
// it. The test asserts the wrap is correct.
func TestCountAccounts_StreamError(t *testing.T) {
	wantErr := errors.New("transport blew up")
	stream := func(yield func(models.Account, error) bool) {
		yield(models.Account{ID: "acc_1"}, nil)
		yield(models.Account{}, wantErr)
	}

	svc := &mockAccountSource{
		allFn: func(_ context.Context, _, _ string, _ models.AccountsListOpts) iter.Seq2[models.Account, error] {
			return stream
		},
	}

	r := NewAccountReporter(svc)
	count, err := r.CountAccounts(context.Background(), "org_test", "ledger_test")

	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("CountAccounts err = %v, want wrap of %v", err, wantErr)
	}
	if count != 1 {
		t.Errorf("CountAccounts pre-error count = %d, want 1", count)
	}
}

// TestFindByAlias_Success demonstrates testing a method that calls the
// SDK once with specific arguments and returns a typed model.
func TestFindByAlias_Success(t *testing.T) {
	alias := "treasury"
	expected := &models.Account{
		ID:    "acc_42",
		Name:  "Treasury",
		Alias: &alias,
	}

	svc := &mockAccountSource{
		getByAliasFn: func(_ context.Context, orgID, ledgerID, gotAlias string) (*models.Account, error) {
			if orgID != "org_test" || ledgerID != "ledger_test" || gotAlias != "treasury" {
				t.Errorf("GetByAlias args = %q/%q/%q", orgID, ledgerID, gotAlias)
			}
			return expected, nil
		},
	}

	r := NewAccountReporter(svc)
	got, err := r.FindByAlias(context.Background(), "org_test", "ledger_test", "treasury")
	if err != nil {
		t.Fatalf("FindByAlias: %v", err)
	}
	if got.ID != expected.ID {
		t.Errorf("FindByAlias ID = %q, want %q", got.ID, expected.ID)
	}
}

// TestFindByAlias_NotFound demonstrates testing the SDK error path:
// the mock returns an error, our reporter wraps it with context, and
// the wrap preserves the original via errors.Is.
func TestFindByAlias_NotFound(t *testing.T) {
	wantErr := errors.New("not found")

	svc := &mockAccountSource{
		getByAliasFn: func(_ context.Context, _, _, _ string) (*models.Account, error) {
			return nil, wantErr
		},
	}

	r := NewAccountReporter(svc)
	got, err := r.FindByAlias(context.Background(), "org_test", "ledger_test", "missing")
	if got != nil {
		t.Errorf("FindByAlias returned non-nil account on error: %+v", got)
	}
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("FindByAlias err = %v, want wrap of %v", err, wantErr)
	}
}

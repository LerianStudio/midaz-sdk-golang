package main

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/entities/mocks"
	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"go.uber.org/mock/gomock"
)

// TestCountAccounts_Success demonstrates the canonical mock-based test:
// expect a method call, return canned data, assert the result.
func TestCountAccounts_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockAccountsService(ctrl)

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

	mockSvc.EXPECT().
		ListAccountsAll(
			gomock.Any(),
			"org_test",
			"ledger_test",
			gomock.Any(),
		).
		Return(iter.Seq2[models.Account, error](stream))

	r := NewAccountReporter(mockSvc)
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
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockAccountsService(ctrl)

	wantErr := errors.New("transport blew up")
	stream := func(yield func(models.Account, error) bool) {
		yield(models.Account{ID: "acc_1"}, nil)
		yield(models.Account{}, wantErr)
	}

	mockSvc.EXPECT().
		ListAccountsAll(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(iter.Seq2[models.Account, error](stream))

	r := NewAccountReporter(mockSvc)
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
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockAccountsService(ctrl)

	alias := "treasury"
	expected := &models.Account{
		ID:    "acc_42",
		Name:  "Treasury",
		Alias: &alias,
	}

	mockSvc.EXPECT().
		GetAccountByAlias(
			gomock.Any(),
			"org_test",
			"ledger_test",
			"treasury",
		).
		Return(expected, nil)

	r := NewAccountReporter(mockSvc)
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
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockAccountsService(ctrl)

	wantErr := errors.New("not found")
	mockSvc.EXPECT().
		GetAccountByAlias(gomock.Any(), gomock.Any(), gomock.Any(), "missing").
		Return(nil, wantErr)

	r := NewAccountReporter(mockSvc)
	got, err := r.FindByAlias(context.Background(), "org_test", "ledger_test", "missing")
	if got != nil {
		t.Errorf("FindByAlias returned non-nil account on error: %+v", got)
	}
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("FindByAlias err = %v, want wrap of %v", err, wantErr)
	}
}

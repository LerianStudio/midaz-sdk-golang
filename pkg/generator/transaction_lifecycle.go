package generator

import (
	"context"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
)

// lifecycle implements TransactionLifecycle using the entities API with retry and observability.
type lifecycle struct {
	e   *entities.Entity
	obs observability.Provider
}

// NewTransactionLifecycle creates a TransactionLifecycle implementation.
func NewTransactionLifecycle(e *entities.Entity, obs observability.Provider) TransactionLifecycle {
	return &lifecycle{e: e, obs: obs}
}

// CreatePending creates a transaction marked as pending, respecting idempotency and retries.
func (l *lifecycle) CreatePending(ctx context.Context, input *models.CreateTransactionInput) (*models.Transaction, error) {
	if l.e == nil || l.e.Transactions == nil {
		return nil, fmt.Errorf("entity transactions service not initialized")
	}

	if input == nil {
		return nil, fmt.Errorf("transaction input is required")
	}

	// Ensure Pending flag is set
	input.Pending = true

	// orgID and ledgerID are required; pass them via context to keep interface minimal
	orgID, _ := ctx.Value(contextKeyOrgID{}).(string)
	ledgerID, _ := ctx.Value(contextKeyLedgerID{}).(string)

	if orgID == "" || ledgerID == "" {
		return nil, fmt.Errorf("organization and ledger IDs are required in context")
	}

	var out *models.Transaction

	err := observability.WithSpan(ctx, l.obs, "Lifecycle.CreatePending", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				tx, err := l.e.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
				if err != nil {
					return err
				}

				out = tx

				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// Commit commits a pending transaction using the dedicated API endpoint.
func (l *lifecycle) Commit(ctx context.Context, txID string) error {
	if l.e == nil || l.e.Transactions == nil {
		return fmt.Errorf("entity transactions service not initialized")
	}

	if txID == "" {
		return fmt.Errorf("transaction ID is required")
	}

	orgID, _ := ctx.Value(contextKeyOrgID{}).(string)
	ledgerID, _ := ctx.Value(contextKeyLedgerID{}).(string)

	if orgID == "" || ledgerID == "" {
		return fmt.Errorf("organization and ledger IDs are required in context")
	}

	return observability.WithSpan(ctx, l.obs, "Lifecycle.Commit", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				_, err := l.e.Transactions.CommitTransaction(ctx, orgID, ledgerID, txID)
				return err
			})
		})
	})
}

// Revert reverts a committed transaction.
func (l *lifecycle) Revert(ctx context.Context, txID string) error {
	if l.e == nil || l.e.Transactions == nil {
		return fmt.Errorf("entity transactions service not initialized")
	}

	if txID == "" {
		return fmt.Errorf("transaction ID is required")
	}

	orgID, _ := ctx.Value(contextKeyOrgID{}).(string)
	ledgerID, _ := ctx.Value(contextKeyLedgerID{}).(string)

	if orgID == "" || ledgerID == "" {
		return fmt.Errorf("organization and ledger IDs are required in context")
	}

	return observability.WithSpan(ctx, l.obs, "Lifecycle.Revert", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				_, err := l.e.Transactions.RevertTransaction(ctx, orgID, ledgerID, txID)
				return err
			})
		})
	})
}

// HandleInsufficientFunds inspects errors and classifies insufficient balance cases.
// It returns nil for non-insufficient-balance errors, or the original error for insufficient funds
// so that callers can decide to apply compensating transactions.
func (l *lifecycle) HandleInsufficientFunds(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	if sdkerrors.IsInsufficientBalanceError(err) {
		return err
	}

	return nil
}

// contextKeyLedgerID is a private context key used to store and retrieve
// the ledger ID for transaction lifecycle operations.
type contextKeyLedgerID struct{}

// WithLedgerID returns a derived context that carries the ledger ID for lifecycle operations.
// The ledger ID is used by CreatePending, Commit, and Revert methods to identify which
// ledger the transaction belongs to.
//
// Usage:
//
//	ctx := generator.WithOrgID(ctx, orgID)
//	ctx = generator.WithLedgerID(ctx, ledgerID)
//	tx, err := lifecycle.CreatePending(ctx, input)
func WithLedgerID(ctx context.Context, ledgerID string) context.Context {
	return context.WithValue(ctx, contextKeyLedgerID{}, ledgerID)
}

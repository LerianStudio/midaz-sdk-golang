package generator

import (
	"context"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	conc "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/concurrent"
	"github.com/stretchr/testify/require"
)

func TestGeneratorContractContextHelpersNilSafe(t *testing.T) {
	nilCtx := nilContext()
	require.NotNil(t, WithWorkers(nilCtx, 2))
	require.NotZero(t, getWorkers(nilCtx))
	require.Nil(t, getCircuitBreaker(nilCtx))
	require.NotNil(t, WithCircuitBreaker(nilCtx, conc.NewCircuitBreaker(1, 1, 0)))
	require.Equal(t, "us", getOrgLocale(nilCtx))
	require.NotNil(t, WithOrgLocale(nilCtx, "br"))
	require.NotNil(t, WithOrgID(nilCtx, "org"))
	require.NotNil(t, WithLedgerID(nilCtx, "ledger"))
}

func nilContext() context.Context { return nil }

func TestGeneratorContractRouteGeneratorsRejectNilInputs(t *testing.T) {
	opGen := &operationRouteGenerator{operationRoutes: &mockOperationRoutesService{}}
	_, err := opGen.Generate(context.Background(), "org", "ledger", nil)
	require.Error(t, err)

	txGen := &transactionRouteGenerator{transactionRoutes: &mockTransactionRoutesService{}}
	_, err = txGen.GenerateDefaults(context.Background(), "org", "ledger", nil)
	require.NoError(t, err)

	_, err = txGen.GenerateDefaults(context.Background(), "org", "ledger", []*models.OperationRoute{nil})
	require.Error(t, err)
}

package workflows

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandleTransactionRoutesFailsFastByDefault(t *testing.T) {
	t.Setenv("MIDAZ_DEMO_ALLOW_MOCK_TRANSACTION_ROUTES", "")

	paymentRoute, refundRoute, err := handleTransactionRoutes(context.Background(), nil, "00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002", nil, nil)

	require.Error(t, err)
	require.Nil(t, paymentRoute)
	require.Nil(t, refundRoute)
	require.Contains(t, err.Error(), "transaction routes API not available")
}

func TestHandleTransactionRoutesAllowsMockFallbackWhenExplicitlyEnabled(t *testing.T) {
	t.Setenv("MIDAZ_DEMO_ALLOW_MOCK_TRANSACTION_ROUTES", "true")

	paymentRoute, refundRoute, err := handleTransactionRoutes(context.Background(), nil, "00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002", nil, nil)

	require.NoError(t, err)
	require.NotNil(t, paymentRoute)
	require.NotNil(t, refundRoute)
}

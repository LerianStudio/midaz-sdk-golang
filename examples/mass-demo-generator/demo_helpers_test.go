package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/transaction"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDefaultDemoConfigPrecedence(t *testing.T) {
	t.Setenv("DEMO_ORGS", "9")

	timeout, orgs, ledgers, accounts, tx, concurrency, batch := 120, 3, 1, 2, 4, 5, 6
	cfg := defaultDemoConfig(&timeout, &orgs, &ledgers, &accounts, &tx, &concurrency, &batch, "us", map[string]bool{"orgs": true})
	require.Equal(t, 3, cfg.orgsVal)

	cfg = defaultDemoConfig(&timeout, &orgs, &ledgers, &accounts, &tx, &concurrency, &batch, "us", nil)
	require.Equal(t, 9, cfg.orgsVal)
}

func TestValidateDemoConfigRejectsUnsafeValues(t *testing.T) {
	cfg := demoConfig{timeoutSecVal: 120, orgsVal: 1, ledgersPerOrgVal: 1, accountsPerLedgerVal: 1, txPerAccountVal: 1, assetsCountVal: 1, batchSizeVal: 1, orgLocaleVal: "us"}
	require.NoError(t, validateDemoConfig(cfg))

	cfg.txPerAccountVal = -1
	require.ErrorContains(t, validateDemoConfig(cfg), "tx")
}

// TestValidateDemoConfigRejectsAnUnrunnableV2Phase pins the refusal that keeps
// the money-path proof from being skipped in silence.
//
// runV2Phases is driven by the ledger contexts the demo flow builds, so
// DEMO_RUN_V2=true with no flow, no organizations or no ledgers used to exit 0
// having proven nothing — the one outcome worse than a failing proof, because
// nothing distinguishes it from a passing one.
func TestValidateDemoConfigRejectsAnUnrunnableV2Phase(t *testing.T) {
	base := demoConfig{
		timeoutSecVal: 120, orgsVal: 1, ledgersPerOrgVal: 1, accountsPerLedgerVal: 1,
		txPerAccountVal: 1, assetsCountVal: 1, batchSizeVal: 1, orgLocaleVal: "us",
		doDemoVal: true, runV2Val: true,
	}
	require.NoError(t, validateDemoConfig(base))

	noFlow := base
	noFlow.doDemoVal = false
	require.ErrorContains(t, validateDemoConfig(noFlow), "DEMO_RUN_FLOW")

	noOrgs := base
	noOrgs.orgsVal = 0
	require.ErrorContains(t, validateDemoConfig(noOrgs), "DEMO_ORGS")

	noLedgers := base
	noLedgers.ledgersPerOrgVal = 0
	require.ErrorContains(t, validateDemoConfig(noLedgers), "DEMO_LEDGERS_PER_ORG")

	// The same zero counts are fine when the phase is off — the refusal is about
	// asking for a proof that cannot run, not about the counts themselves.
	off := noFlow
	off.runV2Val = false
	require.NoError(t, validateDemoConfig(off))
}

func TestSaveEntitiesIDsUsesOwnerOnlyPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entities.json")

	err := saveEntitiesIDs(path, transaction.ReportEntityIDs{OrganizationIDs: []string{"org-1"}})
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestTryLoadDemoFileDefaultsRequiresMassDemoBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "default.yaml")
	require.NoError(t, os.WriteFile(path, []byte("timeout: 10\n"), 0o600))

	defaults, ok := tryLoadDemoFileDefaults(path)

	require.False(t, ok)
	require.Equal(t, demoFileDefaults{}, defaults)
}

func TestTryLoadDemoFileDefaultsAcceptsMassDemoBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "default.yaml")
	require.NoError(t, os.WriteFile(path, []byte("mass_demo:\n  timeout: 30\n"), 0o600))

	defaults, ok := tryLoadDemoFileDefaults(path)

	require.True(t, ok)
	require.NotNil(t, defaults.Timeout)
	require.Equal(t, 30, *defaults.Timeout)
}

func TestSelectDemoRoutesReturnsRequiredIDs(t *testing.T) {
	sourceID := uuid.New()
	destinationID := uuid.New()
	transactionID := uuid.New()

	routes, err := selectDemoRoutes(
		[]*models.OperationRoute{
			{ID: sourceID, Title: "Source: External (any)"},
			{ID: destinationID, Title: "Destination: Customer (CHECKING)"},
		},
		[]*models.TransactionRoute{{ID: transactionID, Title: "External Funding Flow"}},
	)

	require.NoError(t, err)
	require.Equal(t, sourceID.String(), routes.sourceRouteID)
	require.Equal(t, destinationID.String(), routes.destinationRouteID)
	require.Equal(t, transactionID.String(), routes.transactionRouteID)
}

func TestSelectDemoRoutesRequiresDefaultRoutes(t *testing.T) {
	_, err := selectDemoRoutes(
		[]*models.OperationRoute{{ID: uuid.New(), Title: "Source: External (any)"}},
		[]*models.TransactionRoute{},
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "operation route Destination: Customer (CHECKING)")
	require.Contains(t, err.Error(), "transaction route External Funding Flow")
}

func TestCreateSDKConfigRequiresExplicitAnonymousModeWhenAuthDisabled(t *testing.T) {
	t.Setenv("PLUGIN_AUTH_ENABLED", "false")

	_, err := createSDKConfig()

	require.Error(t, err)
	require.Contains(t, err.Error(), "DEMO_AUTH_MODE=anonymous-local")
}

func TestCreateSDKConfigAllowsExplicitAnonymousLocalMode(t *testing.T) {
	t.Setenv("PLUGIN_AUTH_ENABLED", "false")
	t.Setenv("DEMO_AUTH_MODE", "anonymous-local")

	cfg, err := createSDKConfig()

	require.NoError(t, err)
	require.True(t, cfg.Anonymous)
}

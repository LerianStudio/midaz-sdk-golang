package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/transaction"
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

func TestSaveEntitiesIDsUsesOwnerOnlyPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entities.json")

	err := saveEntitiesIDs(path, transaction.ReportEntityIDs{OrganizationIDs: []string{"org-1"}})
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

package config

import (
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccessManagerRequiresExplicitTarget(t *testing.T) {
	accessManager := auth.AccessManager{
		Address:      "https://auth.example.com",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}

	t.Run("default local target is not enough", func(t *testing.T) {
		_, err := NewConfig(WithAccessManager(accessManager))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "explicit environment or service URL is required")
	})

	t.Run("explicit environment counts", func(t *testing.T) {
		_, err := NewConfig(WithEnvironment(EnvironmentProduction), WithAccessManager(accessManager))
		require.NoError(t, err)
	})

	t.Run("explicit base URL counts", func(t *testing.T) {
		_, err := NewConfig(WithBaseURL("https://api.example.com"), WithAccessManager(accessManager))
		require.NoError(t, err)
	})

	t.Run("explicit ledger URL counts", func(t *testing.T) {
		_, err := NewConfig(WithLedgerURL("https://api.example.com/v1"), WithAccessManager(accessManager))
		require.NoError(t, err)
	})

	t.Run("explicit crm URL counts", func(t *testing.T) {
		_, err := NewConfig(WithCRMURL("https://api.example.com/crm"), WithAccessManager(accessManager))
		require.NoError(t, err)
	})

	t.Run("anonymous mode is exempt", func(t *testing.T) {
		_, err := NewConfig(WithAnonymous())
		require.NoError(t, err)
	})
}

func TestAccessManagerTargetFromEnvironmentCountsAsExplicit(t *testing.T) {
	t.Setenv("PLUGIN_AUTH_ENABLED", "true")
	t.Setenv("PLUGIN_AUTH_ADDRESS", "https://auth.example.com")
	t.Setenv("MIDAZ_CLIENT_ID", "client-id")
	t.Setenv("MIDAZ_CLIENT_SECRET", "client-secret")
	t.Setenv("MIDAZ_ENVIRONMENT", "production")

	_, err := NewConfig(FromEnvironment())
	require.NoError(t, err)
}

func TestAccessManagerFromEnvironmentRequiresExplicitTarget(t *testing.T) {
	t.Setenv("PLUGIN_AUTH_ENABLED", "true")
	t.Setenv("PLUGIN_AUTH_ADDRESS", "https://auth.example.com")
	t.Setenv("MIDAZ_CLIENT_ID", "client-id")
	t.Setenv("MIDAZ_CLIENT_SECRET", "client-secret")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "explicit environment or service URL is required")
}

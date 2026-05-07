package midaz

import (
	"net/http"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
	"github.com/stretchr/testify/require"
)

// createTestConfig creates a test config with sensible defaults.
// It uses t.Setenv for automatic cleanup and t.Fatalf on config errors.
func createTestConfig(t *testing.T) *config.Config {
	t.Helper()

	t.Setenv("MIDAZ_SKIP_AUTH_CHECK", "true")

	cfg, err := config.NewConfig(
		config.WithAnonymous(),
		config.WithEnvironment(config.EnvironmentLocal),
	)
	if err != nil {
		t.Fatalf("createTestConfig: %v", err)
	}

	return cfg
}

func TestNewClient(t *testing.T) {
	// Test creating a new client with a test config
	client, err := New(WithConfig(createTestConfig(t)))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Check that default config was created
	if client.config == nil {
		t.Fatal("Expected config to be set, got nil")
	}

	// Check that context was set
	if client.ctx == nil {
		t.Fatal("Expected context to be set, got nil")
	}

	// Test creating a client with options
	customHTTPClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Create a base config
	testCfg := createTestConfig(t)

	client, err = New(
		WithConfig(testCfg),
		WithHTTPClient(customHTTPClient),
		WithOnboardingURL("https://test.example.com/onboarding"),
		WithTransactionURL("https://test.example.com/transaction"),
		WithCRMURL("https://test.example.com/crm"),
		WithTimeout(30*time.Second),
		WithDebug(true),
		WithEnvironment(config.EnvironmentDevelopment),
	)
	if err != nil {
		t.Fatalf("Failed to create client with options: %v", err)
	}

	// Check that all options were applied
	if client.config.AccessManager.Enabled {
		t.Errorf("Expected AccessManager.Enabled to be false, got true")
	}

	if client.config.HTTPClient != customHTTPClient {
		t.Error("Expected HTTP client to be set to custom client")
	}

	if client.config.Environment != config.EnvironmentDevelopment {
		t.Errorf("Expected environment to be 'development', got '%s'", client.config.Environment)
	}

	if !client.config.Debug {
		t.Error("Expected debug to be true")
	}

	if got := client.config.ServiceURLs[config.ServiceCRM]; got != "https://test.example.com/crm" {
		t.Errorf("Expected CRM URL to be applied, got %q", got)
	}

	require.NotNil(t, client.Entity)
	require.NotNil(t, client.Holders)
	require.NotNil(t, client.Aliases)
	require.NotNil(t, client.MetadataIndexes)

	// Test creating a client with a complete config
	cfg, err := config.NewConfig(
		config.WithAnonymous(),
		config.WithEnvironment(config.EnvironmentProduction),
	)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	client, err = New(WithConfig(cfg))
	if err != nil {
		t.Fatalf("Failed to create client with config: %v", err)
	}

	if client.config.Environment != config.EnvironmentProduction {
		t.Errorf("Expected environment to be 'production', got '%s'", client.config.Environment)
	}
}

func TestEntityAlwaysInitialized(t *testing.T) {
	// v3: Entity surface is always initialized; no opt-in required.
	c, err := New(WithConfig(createTestConfig(t)))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	require.NotNil(t, c.Entity, "v3 must always initialize Entity")
	require.NotNil(t, c.Accounts)
	require.NotNil(t, c.Transactions)
	require.NotNil(t, c.Organizations)
}

func TestGetConfig(t *testing.T) {
	client, err := New(WithConfig(createTestConfig(t)))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	cfg := client.GetConfig()
	if cfg == nil {
		t.Fatal("Expected config to be returned, got nil")
	}
}

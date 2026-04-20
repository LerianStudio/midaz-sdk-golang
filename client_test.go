package client

import (
	"net/http"
	"testing"
	"time"

	auth "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
)

// createTestConfig creates a test config with sensible defaults.
// It uses t.Setenv for automatic cleanup and t.Fatalf on config errors.
func createTestConfig(t *testing.T) *config.Config {
	t.Helper()

	t.Setenv("MIDAZ_SKIP_AUTH_CHECK", "true")

	cfg, err := config.NewConfig(
		config.WithAccessManager(auth.AccessManager{Enabled: false, Address: ""}),
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
		WithOnboardingURL("http://test.example.com/onboarding"),
		WithTransactionURL("http://test.example.com/transaction"),
		WithTimeout(30*time.Second),
		WithDebug(true),
		WithEnvironment(config.EnvironmentDevelopment),
		UseEntity(),
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

	if !client.useEntity {
		t.Error("Expected useEntity to be true")
	}

	// Test creating a client with a complete config
	cfg, err := config.NewConfig(
		config.WithAccessManager(auth.AccessManager{Enabled: false, Address: ""}),
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

func TestUseAllAPIs(t *testing.T) {
	client, err := New(UseAllAPIs(), WithConfig(createTestConfig(t)))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if !client.useEntity {
		t.Error("Expected useEntity to be true")
	}
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

// TestClientWithTenantID verifies that the WithTenantID client option correctly
// sets the tenant ID on the Client struct, which is later propagated to the
// Entity layer during setupEntity.
func TestClientWithTenantID(t *testing.T) {
	// Create client with tenant ID option
	c, err := New(
		WithConfig(createTestConfig(t)),
		WithTenantID("test-tenant"),
	)
	if err != nil {
		t.Fatalf("Failed to create client with tenant ID: %v", err)
	}

	// Verify the tenant ID is stored on the client
	if c.tenantID != "test-tenant" {
		t.Errorf("Expected tenantID to be 'test-tenant', got '%s'", c.tenantID)
	}
}

// TestClientWithTenantIDEmpty verifies that an empty tenant ID is accepted
// (it simply won't be propagated as a header later).
func TestClientWithTenantIDEmpty(t *testing.T) {
	c, err := New(
		WithConfig(createTestConfig(t)),
		WithTenantID(""),
	)
	if err != nil {
		t.Fatalf("Failed to create client with empty tenant ID: %v", err)
	}

	if c.tenantID != "" {
		t.Errorf("Expected tenantID to be empty, got '%s'", c.tenantID)
	}
}

// TestClientWithTenantIDPropagatedToEntity verifies that when UseEntityAPI is
// enabled, the client-level tenant ID is propagated to the Entity layer's
// underlying HTTPClient via entities.WithDefaultTenantID in setupEntity.
func TestClientWithTenantIDPropagatedToEntity(t *testing.T) {
	c, err := New(
		WithConfig(createTestConfig(t)),
		WithTenantID("propagated-tenant"),
		UseEntityAPI(),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify entity was created
	if c.Entity == nil {
		t.Fatal("Expected Entity to be set")
	}

	// Verify the client-level tenant ID is stored correctly
	if c.tenantID != "propagated-tenant" {
		t.Errorf("Expected client tenantID to be 'propagated-tenant', got '%s'", c.tenantID)
	}

	// Verify the effective tenant ID was wired into the Entity's HTTPClient
	entityHTTPClient := c.Entity.GetEntityHTTPClient()
	if entityHTTPClient == nil {
		t.Fatal("Expected Entity HTTP client to be set")
	}

	if got := entityHTTPClient.GetTenantID(); got != "propagated-tenant" {
		t.Errorf("Expected Entity HTTP client tenantID to be 'propagated-tenant', got '%s'", got)
	}
}

// TestClientWithTenantIDFromConfig verifies that the tenant ID from config
// is used when no client-level tenant is set, and that the Entity layer
// receives the config-level tenant.
func TestClientWithTenantIDFromConfig(t *testing.T) {
	cfg := createTestConfig(t)
	cfg.TenantID = "config-tenant"

	c, err := New(
		WithConfig(cfg),
		UseEntityAPI(),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Client-level tenantID should be empty since we didn't use WithTenantID
	if c.tenantID != "" {
		t.Errorf("Expected client tenantID to be empty, got '%s'", c.tenantID)
	}

	// Config-level tenant ID should be set
	if c.config.TenantID != "config-tenant" {
		t.Errorf("Expected config TenantID to be 'config-tenant', got '%s'", c.config.TenantID)
	}

	// Entity HTTP client should receive the config-level tenant
	entityHTTPClient := c.Entity.GetEntityHTTPClient()
	if entityHTTPClient == nil {
		t.Fatal("Expected Entity HTTP client to be set")
	}

	if got := entityHTTPClient.GetTenantID(); got != "config-tenant" {
		t.Errorf("Expected Entity HTTP client tenantID to be 'config-tenant', got '%s'", got)
	}
}

// TestClientWithTenantIDEmptyOverrideDoesNotClearConfig verifies that
// WithTenantID("") explicitly clears the tenant (does not fall back to config).
func TestClientWithTenantIDEmptyOverrideDoesNotClearConfig(t *testing.T) {
	cfg := createTestConfig(t)
	cfg.TenantID = "config-tenant"

	c, err := New(
		WithConfig(cfg),
		WithTenantID(""),
		UseEntityAPI(),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Client explicitly set tenant to empty — should NOT fall back to config
	entityHTTPClient := c.Entity.GetEntityHTTPClient()
	if entityHTTPClient == nil {
		t.Fatal("Expected Entity HTTP client to be set")
	}

	if got := entityHTTPClient.GetTenantID(); got != "" {
		t.Errorf("Expected Entity HTTP client tenantID to be empty (explicitly cleared), got '%s'", got)
	}
}

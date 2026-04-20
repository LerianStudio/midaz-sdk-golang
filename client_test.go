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

// tenantTestCase describes a single tenant ID precedence scenario.
type tenantTestCase struct {
	name            string
	clientTenantID  string // value for WithTenantID
	setClientTenant bool   // true = apply WithTenantID option
	configTenantID  string // if non-empty, set on config before New()
	useEntityAPI    bool
	wantClientTID   string
	wantConfigTID   string // only checked when checkConfigTID is true
	checkConfigTID  bool
	wantEntityTID   string // only checked when useEntityAPI is true
}

// buildTenantTestClient creates a Client from a tenantTestCase.
func buildTenantTestClient(t *testing.T, tt tenantTestCase) *Client {
	t.Helper()

	cfg := createTestConfig(t)
	if tt.configTenantID != "" {
		cfg.TenantID = tt.configTenantID
	}

	opts := []Option{WithConfig(cfg)}
	if tt.setClientTenant {
		opts = append(opts, WithTenantID(tt.clientTenantID))
	}

	if tt.useEntityAPI {
		opts = append(opts, UseEntityAPI())
	}

	c, err := New(opts...)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return c
}

// assertEntityTenantID verifies the Entity layer received the expected tenant.
func assertEntityTenantID(t *testing.T, c *Client, wantTID string) {
	t.Helper()

	if c.Entity == nil {
		t.Fatal("Expected Entity to be set")
	}

	entityHTTPClient := c.Entity.GetEntityHTTPClient()
	if entityHTTPClient == nil {
		t.Fatal("Expected Entity HTTP client to be set")
	}

	if got := entityHTTPClient.GetTenantID(); got != wantTID {
		t.Errorf("Expected Entity HTTP client tenantID %q, got %q", wantTID, got)
	}
}

// TestClientTenantOptions is a table-driven test covering all tenant ID
// precedence permutations: client-level WithTenantID, config-level TenantID,
// whitespace normalization, explicit clearing, and Entity layer propagation.
func TestClientTenantOptions(t *testing.T) {
	tests := []tenantTestCase{
		{
			name:            "basic tenant set",
			clientTenantID:  "test-tenant",
			setClientTenant: true,
			wantClientTID:   "test-tenant",
		},
		{
			name:            "empty tenant accepted",
			clientTenantID:  "",
			setClientTenant: true,
			wantClientTID:   "",
		},
		{
			name:            "propagated to entity",
			clientTenantID:  "propagated-tenant",
			setClientTenant: true,
			useEntityAPI:    true,
			wantClientTID:   "propagated-tenant",
			wantEntityTID:   "propagated-tenant",
		},
		{
			name:           "config fallback when no client tenant",
			configTenantID: "config-tenant",
			useEntityAPI:   true,
			wantClientTID:  "",
			checkConfigTID: true,
			wantConfigTID:  "config-tenant",
			wantEntityTID:  "config-tenant",
		},
		{
			name:            "empty override clears config tenant",
			clientTenantID:  "",
			setClientTenant: true,
			configTenantID:  "config-tenant",
			useEntityAPI:    true,
			wantClientTID:   "",
			wantEntityTID:   "",
		},
		{
			name:            "whitespace trimmed",
			clientTenantID:  "  tenant-a  ",
			setClientTenant: true,
			useEntityAPI:    true,
			wantClientTID:   "tenant-a",
			wantEntityTID:   "tenant-a",
		},
		{
			name:            "whitespace-only becomes empty",
			clientTenantID:  "   ",
			setClientTenant: true,
			useEntityAPI:    true,
			wantClientTID:   "",
			wantEntityTID:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := buildTenantTestClient(t, tt)

			if c.tenantID != tt.wantClientTID {
				t.Errorf("Expected client tenantID %q, got %q", tt.wantClientTID, c.tenantID)
			}

			if tt.checkConfigTID && c.config.TenantID != tt.wantConfigTID {
				t.Errorf("Expected config TenantID %q, got %q", tt.wantConfigTID, c.config.TenantID)
			}

			if tt.useEntityAPI {
				assertEntityTenantID(t, c, tt.wantEntityTID)
			}
		})
	}
}

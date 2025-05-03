package client

import (
	"net/http"
	"os"
	"testing"
	"time"

	auth "github.com/LerianStudio/midaz-sdk-golang/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/config"
)

// createTestConfig creates a test config with sensible defaults
func createTestConfig() *config.Config {
	// Set environment variable to skip auth check in tests
	os.Setenv("MIDAZ_SKIP_AUTH_CHECK", "true")

	cfg, _ := config.NewConfig(
		config.WithPluginAccessManager(auth.PluginAccessManager{Enabled: false, Address: ""}),
		config.WithEnvironment(config.EnvironmentLocal),
	)
	return cfg
}

func TestNewClient(t *testing.T) {
	// Test creating a new client with a test config
	client, err := New(WithConfig(createTestConfig()))
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
	testCfg := createTestConfig()

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
	if client.config.PluginAccessManager.Enabled != false {
		t.Errorf("Expected PluginAccessManager.Enabled to be false, got true")
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
		config.WithPluginAccessManager(auth.PluginAccessManager{Enabled: false, Address: ""}),
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
	client, err := New(UseAllAPIs(), WithConfig(createTestConfig()))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if !client.useEntity {
		t.Error("Expected useEntity to be true")
	}
}

func TestGetConfig(t *testing.T) {
	client, err := New(WithConfig(createTestConfig()))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	cfg := client.GetConfig()
	if cfg == nil {
		t.Fatal("Expected config to be returned, got nil")
	}
}

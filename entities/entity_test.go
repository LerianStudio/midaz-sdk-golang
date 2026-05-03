package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMainFunction serves as an entry point for all tests in the entities package
func TestMainFunction(_ *testing.T) {
	// This is an empty test function that ensures the package has at least one test
	// so that the testing package will execute all test files in the package.
}

func TestNewWithServiceURLs_DefaultsMissingCRMURLToOnboarding(t *testing.T) {
	t.Setenv("MIDAZ_CRM_URL", "")

	entity, err := NewWithServiceURLs(map[string]string{
		"onboarding":  "https://api.example.com/onboarding",
		"transaction": "https://api.example.com/transaction",
	})

	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/onboarding/v1", entity.baseURLs["crm"])
}

func TestNormalizeBaseURLs_DoesNotUseEnvironmentFallbackForCRM(t *testing.T) {
	t.Setenv("MIDAZ_CRM_URL", "https://api.example.com/crm")

	baseURLs, err := normalizeBaseURLs(map[string]string{
		"transaction": "https://api.example.com/transaction",
	})

	require.NoError(t, err)
	require.Empty(t, baseURLs["crm"])
}

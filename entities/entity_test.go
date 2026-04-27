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

func TestNewWithServiceURLs_RequiresCRMURL(t *testing.T) {
	t.Setenv("MIDAZ_CRM_URL", "")

	entity, err := NewWithServiceURLs(map[string]string{
		"onboarding":  "https://api.example.com/onboarding",
		"transaction": "https://api.example.com/transaction",
	})

	require.ErrorContains(t, err, "missing crm URL")
	require.Nil(t, entity)
}

func TestNewWithServiceURLs_UsesCRMURLFromEnvironment(t *testing.T) {
	t.Setenv("MIDAZ_CRM_URL", "https://api.example.com/crm")

	entity, err := NewWithServiceURLs(map[string]string{
		"onboarding":  "https://api.example.com/onboarding",
		"transaction": "https://api.example.com/transaction",
	})

	require.NoError(t, err)
	require.NotNil(t, entity)
	require.Equal(t, "https://api.example.com/crm", entity.baseURLs["crm"])
}

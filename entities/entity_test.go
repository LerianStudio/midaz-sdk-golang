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

func TestNewWithServiceURLs_AllowsMissingCRMURL(t *testing.T) {
	entity, err := NewWithServiceURLs(map[string]string{
		"onboarding":  "https://api.example.com/onboarding",
		"transaction": "https://api.example.com/transaction",
	})

	require.NoError(t, err)
	require.NotNil(t, entity)
	require.NotContains(t, entity.baseURLs, "crm")
}

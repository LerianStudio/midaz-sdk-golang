package data

import (
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTechCompanies(t *testing.T) {
	orgs := TechCompanies()

	require.Len(t, orgs, 2)

	t.Run("alpha cloud systems", func(t *testing.T) {
		org := orgs[0]
		assert.Equal(t, "Alpha Cloud Systems Inc.", org.LegalName)
		assert.Equal(t, "AlphaCloud", org.TradeName)
		assert.Equal(t, "00-0000001", org.TaxID)
		assert.Equal(t, models.StatusActive, org.Status.Code)
		assert.Equal(t, "technology", org.Industry)
		assert.Equal(t, "medium", org.Size)

		// Verify address
		assert.Equal(t, "100 Market St", org.Address.Line1)
		assert.Equal(t, "94103", org.Address.ZipCode)
		assert.Equal(t, "San Francisco", org.Address.City)
		assert.Equal(t, "CA", org.Address.State)
		assert.Equal(t, "US", org.Address.Country)

		// Verify metadata
		assert.Equal(t, "saas,cloud", org.Metadata["tags"])
		assert.Equal(t, "SaaS", org.Metadata["category"])
		assert.Equal(t, "tech_demo_1", org.Metadata["custom_field"])
	})

	t.Run("beta marketplace", func(t *testing.T) {
		org := orgs[1]
		assert.Equal(t, "Beta Marketplace LLC", org.LegalName)
		assert.Equal(t, "BetaMarket", org.TradeName)
		assert.Equal(t, "00-0000002", org.TaxID)
		assert.Equal(t, models.StatusActive, org.Status.Code)
		assert.Equal(t, "technology", org.Industry)
		assert.Equal(t, "large", org.Size)

		// Verify address
		assert.Equal(t, "500 7th Ave", org.Address.Line1)
		assert.Equal(t, "10018", org.Address.ZipCode)
		assert.Equal(t, "New York", org.Address.City)
		assert.Equal(t, "NY", org.Address.State)
		assert.Equal(t, "US", org.Address.Country)

		// Verify metadata
		assert.Equal(t, "marketplace,platform", org.Metadata["tags"])
		assert.Equal(t, "Marketplace", org.Metadata["category"])
	})
}

func TestECommerceBusinesses(t *testing.T) {
	orgs := ECommerceBusinesses()

	require.Len(t, orgs, 2)

	t.Run("gamma retail online", func(t *testing.T) {
		org := orgs[0]
		assert.Equal(t, "Gamma Retail Online Ltd.", org.LegalName)
		assert.Equal(t, "GammaShop", org.TradeName)
		assert.Equal(t, "00-0000010", org.TaxID)
		assert.Equal(t, models.StatusActive, org.Status.Code)
		assert.Equal(t, "ecommerce", org.Industry)
		assert.Equal(t, "medium", org.Size)

		// Verify address
		assert.Equal(t, "200 Commerce Blvd", org.Address.Line1)
		assert.Equal(t, "Chicago", org.Address.City)
		assert.Equal(t, "IL", org.Address.State)

		// Verify metadata
		assert.Equal(t, "b2c", org.Metadata["tags"])
		assert.Equal(t, "Retail", org.Metadata["category"])
	})

	t.Run("delta b2b wholesale", func(t *testing.T) {
		org := orgs[1]
		assert.Equal(t, "Delta B2B Wholesale Corp.", org.LegalName)
		assert.Equal(t, "DeltaWholesale", org.TradeName)
		assert.Equal(t, "00-0000011", org.TaxID)
		assert.Equal(t, models.StatusActive, org.Status.Code)
		assert.Equal(t, "ecommerce", org.Industry)
		assert.Equal(t, "large", org.Size)

		// Verify address
		assert.Equal(t, "50 Industrial Rd", org.Address.Line1)
		assert.Equal(t, "Hoboken", org.Address.City)
		assert.Equal(t, "NJ", org.Address.State)

		// Verify metadata
		assert.Equal(t, "b2b,wholesale", org.Metadata["tags"])
		assert.Equal(t, "Wholesale", org.Metadata["category"])
	})
}

func TestFinancialInstitutions(t *testing.T) {
	orgs := FinancialInstitutions()

	require.Len(t, orgs, 2)

	t.Run("epsilon bank", func(t *testing.T) {
		org := orgs[0]
		assert.Equal(t, "Epsilon Bank N.A.", org.LegalName)
		assert.Equal(t, "EpsilonBank", org.TradeName)
		assert.Equal(t, "00-0000020", org.TaxID)
		assert.Equal(t, models.StatusActive, org.Status.Code)
		assert.Equal(t, "financial", org.Industry)
		assert.Equal(t, "enterprise", org.Size)

		// Verify address
		assert.Equal(t, "1 Finance Way", org.Address.Line1)
		assert.Equal(t, "10005", org.Address.ZipCode)
		assert.Equal(t, "New York", org.Address.City)
		assert.Equal(t, "NY", org.Address.State)

		// Verify metadata
		assert.Equal(t, "bank,retail", org.Metadata["tags"])
		assert.Equal(t, "Banking", org.Metadata["category"])
	})

	t.Run("zeta payments", func(t *testing.T) {
		org := orgs[1]
		assert.Equal(t, "Zeta Payments Co.", org.LegalName)
		assert.Equal(t, "ZetaPay", org.TradeName)
		assert.Equal(t, "00-0000021", org.TaxID)
		assert.Equal(t, models.StatusActive, org.Status.Code)
		assert.Equal(t, "financial", org.Industry)
		assert.Equal(t, "medium", org.Size)

		// Verify address
		assert.Equal(t, "1200 Gateway Dr", org.Address.Line1)
		assert.Equal(t, "San Francisco", org.Address.City)
		assert.Equal(t, "CA", org.Address.State)

		// Verify metadata
		assert.Equal(t, "payments,processor", org.Metadata["tags"])
		assert.Equal(t, "Payments", org.Metadata["category"])
	})
}

func TestHealthcareOrganizations(t *testing.T) {
	orgs := HealthcareOrganizations()

	require.Len(t, orgs, 2)

	t.Run("eta health systems", func(t *testing.T) {
		org := orgs[0]
		assert.Equal(t, "Eta Health Systems", org.LegalName)
		assert.Equal(t, "EtaHealth", org.TradeName)
		assert.Equal(t, "00-0000030", org.TaxID)
		assert.Equal(t, models.StatusActive, org.Status.Code)
		assert.Equal(t, "healthcare", org.Industry)
		assert.Equal(t, "large", org.Size)

		// Verify address
		assert.Equal(t, "700 Wellness Ave", org.Address.Line1)
		assert.Equal(t, "90012", org.Address.ZipCode)
		assert.Equal(t, "Los Angeles", org.Address.City)
		assert.Equal(t, "CA", org.Address.State)

		// Verify metadata
		assert.Equal(t, "hospital,clinic", org.Metadata["tags"])
		assert.Equal(t, "Healthcare", org.Metadata["category"])
	})

	t.Run("theta health insurance", func(t *testing.T) {
		org := orgs[1]
		assert.Equal(t, "Theta Health Insurance Group", org.LegalName)
		assert.Equal(t, "ThetaInsure", org.TradeName)
		assert.Equal(t, "00-0000031", org.TaxID)
		assert.Equal(t, models.StatusActive, org.Status.Code)
		assert.Equal(t, "healthcare", org.Industry)
		assert.Equal(t, "enterprise", org.Size)

		// Verify address
		assert.Equal(t, "44 Insurance Pl", org.Address.Line1)
		assert.Equal(t, "Atlanta", org.Address.City)
		assert.Equal(t, "GA", org.Address.State)

		// Verify metadata
		assert.Equal(t, "insurance", org.Metadata["tags"])
		assert.Equal(t, "Insurance", org.Metadata["category"])
	})
}

func TestRetailChains(t *testing.T) {
	orgs := RetailChains()

	require.Len(t, orgs, 2)

	t.Run("iota retail group", func(t *testing.T) {
		org := orgs[0]
		assert.Equal(t, "Iota Retail Group", org.LegalName)
		assert.Equal(t, "IotaStores", org.TradeName)
		assert.Equal(t, "00-0000040", org.TaxID)
		assert.Equal(t, models.StatusActive, org.Status.Code)
		assert.Equal(t, "retail", org.Industry)
		assert.Equal(t, "large", org.Size)

		// Verify address
		assert.Equal(t, "11 High St", org.Address.Line1)
		assert.Equal(t, "02108", org.Address.ZipCode)
		assert.Equal(t, "Boston", org.Address.City)
		assert.Equal(t, "MA", org.Address.State)

		// Verify metadata
		assert.Equal(t, "retail,omnichannel", org.Metadata["tags"])
		assert.Equal(t, "Retail", org.Metadata["category"])
	})

	t.Run("kappa online retailers", func(t *testing.T) {
		org := orgs[1]
		assert.Equal(t, "Kappa Online Retailers", org.LegalName)
		assert.Equal(t, "KappaShop", org.TradeName)
		assert.Equal(t, "00-0000041", org.TaxID)
		assert.Equal(t, models.StatusActive, org.Status.Code)
		assert.Equal(t, "retail", org.Industry)
		assert.Equal(t, "medium", org.Size)

		// Verify address
		assert.Equal(t, "88 Liberty Rd", org.Address.Line1)
		assert.Equal(t, "Austin", org.Address.City)
		assert.Equal(t, "TX", org.Address.State)

		// Verify metadata
		assert.Equal(t, "ecommerce", org.Metadata["tags"])
		assert.Equal(t, "Retail", org.Metadata["category"])
	})
}

func TestDefaultOrganizations(t *testing.T) {
	orgs := DefaultOrganizations()

	// Count expected organizations:
	// TechCompanies: 2
	// ECommerceBusinesses: 2
	// FinancialInstitutions: 2
	// HealthcareOrganizations: 2
	// RetailChains: 2
	// Total: 10
	expectedCount := 10
	require.Len(t, orgs, expectedCount)

	t.Run("all organizations have required fields", func(t *testing.T) {
		for i, org := range orgs {
			assert.NotEmpty(t, org.LegalName, "org %d should have legal name", i)
			assert.NotEmpty(t, org.TradeName, "org %d should have trade name", i)
			assert.NotEmpty(t, org.TaxID, "org %d should have tax ID", i)
			assert.NotEmpty(t, org.Industry, "org %d should have industry", i)
			assert.NotEmpty(t, org.Size, "org %d should have size", i)
		}
	})

	t.Run("all organizations have valid addresses", func(t *testing.T) {
		for i, org := range orgs {
			assert.NotEmpty(t, org.Address.Line1, "org %d should have address line1", i)
			assert.NotEmpty(t, org.Address.City, "org %d should have city", i)
			assert.NotEmpty(t, org.Address.State, "org %d should have state", i)
			assert.NotEmpty(t, org.Address.Country, "org %d should have country", i)
			assert.NotEmpty(t, org.Address.ZipCode, "org %d should have zip code", i)
		}
	})

	t.Run("all organizations have non-nil metadata", func(t *testing.T) {
		for i, org := range orgs {
			assert.NotNil(t, org.Metadata, "org %d should have non-nil metadata", i)
		}
	})

	t.Run("all organizations are active", func(t *testing.T) {
		for i, org := range orgs {
			assert.Equal(t, models.StatusActive, org.Status.Code, "org %d should be active", i)
		}
	})

	t.Run("unique tax IDs", func(t *testing.T) {
		taxIDs := make(map[string]bool)
		for _, org := range orgs {
			assert.False(t, taxIDs[org.TaxID], "duplicate tax ID found: %s", org.TaxID)
			taxIDs[org.TaxID] = true
		}
	})

	t.Run("valid industries", func(t *testing.T) {
		validIndustries := map[string]bool{
			"technology": true,
			"ecommerce":  true,
			"financial":  true,
			"healthcare": true,
			"retail":     true,
		}

		for i, org := range orgs {
			assert.True(t, validIndustries[org.Industry], "org %d has invalid industry: %s", i, org.Industry)
		}
	})

	t.Run("valid sizes", func(t *testing.T) {
		validSizes := map[string]bool{
			"small":      true,
			"medium":     true,
			"large":      true,
			"enterprise": true,
		}

		for i, org := range orgs {
			assert.True(t, validSizes[org.Size], "org %d has invalid size: %s", i, org.Size)
		}
	})
}

func TestOrgTemplateMetadataFallback(t *testing.T) {
	// Test that DefaultOrganizations ensures metadata is not nil
	orgs := DefaultOrganizations()

	for i, org := range orgs {
		assert.NotNil(t, org.Metadata, "org %d should have non-nil metadata", i)
	}
}

func TestOrgTemplateStruct(t *testing.T) {
	// Test the OrgTemplate struct directly
	org := OrgTemplate{
		LegalName: "Test Corp",
		TradeName: "TestCo",
		TaxID:     "12-3456789",
		Address:   models.NewAddress("123 Test St", "12345", "TestCity", "TS", "US"),
		Status:    models.NewStatus(models.StatusActive),
		Industry:  "technology",
		Size:      "medium",
		Metadata:  map[string]any{"key": "value"},
	}

	assert.Equal(t, "Test Corp", org.LegalName)
	assert.Equal(t, "TestCo", org.TradeName)
	assert.Equal(t, "12-3456789", org.TaxID)
	assert.Equal(t, "123 Test St", org.Address.Line1)
	assert.Equal(t, "12345", org.Address.ZipCode)
	assert.Equal(t, "TestCity", org.Address.City)
	assert.Equal(t, "TS", org.Address.State)
	assert.Equal(t, "US", org.Address.Country)
	assert.Equal(t, models.StatusActive, org.Status.Code)
	assert.Equal(t, "technology", org.Industry)
	assert.Equal(t, "medium", org.Size)
	assert.Equal(t, "value", org.Metadata["key"])
}

func TestOrgTemplateWithNilMetadata(t *testing.T) {
	org := OrgTemplate{
		LegalName: "Test Corp",
		TradeName: "TestCo",
		TaxID:     "12-3456789",
		Address:   models.NewAddress("123 Test St", "12345", "TestCity", "TS", "US"),
		Status:    models.NewStatus(models.StatusActive),
		Metadata:  nil,
	}

	assert.Equal(t, "Test Corp", org.LegalName)
	assert.Equal(t, "TestCo", org.TradeName)
	assert.Equal(t, "12-3456789", org.TaxID)
	assert.NotNil(t, org.Address)
	assert.NotNil(t, org.Status)
	assert.Nil(t, org.Metadata)
}

func TestOrgTemplateWithEmptyMetadata(t *testing.T) {
	org := OrgTemplate{
		LegalName: "Test Corp",
		TradeName: "TestCo",
		TaxID:     "12-3456789",
		Address:   models.NewAddress("123 Test St", "12345", "TestCity", "TS", "US"),
		Status:    models.NewStatus(models.StatusActive),
		Metadata:  map[string]any{},
	}

	assert.Equal(t, "Test Corp", org.LegalName)
	assert.Equal(t, "TestCo", org.TradeName)
	assert.Equal(t, "12-3456789", org.TaxID)
	assert.NotNil(t, org.Address)
	assert.NotNil(t, org.Status)
	assert.NotNil(t, org.Metadata)
	assert.Empty(t, org.Metadata)
}

func TestOrganizationCategoriesCoverage(t *testing.T) {
	// Ensure we have organizations from all industries
	orgs := DefaultOrganizations()

	industries := make(map[string]int)
	for _, org := range orgs {
		industries[org.Industry]++
	}

	// Each industry should have exactly 2 organizations
	expectedIndustries := []string{"technology", "ecommerce", "financial", "healthcare", "retail"}
	for _, industry := range expectedIndustries {
		assert.Equal(t, 2, industries[industry], "industry %s should have 2 organizations", industry)
	}
}

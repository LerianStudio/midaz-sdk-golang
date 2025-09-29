package data

import (
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
)

// TechCompanies returns sample OrgTemplate definitions for technology companies.
func TechCompanies() []OrgTemplate {
	return []OrgTemplate{
		{
			LegalName: "Alpha Cloud Systems Inc.",
			TradeName: "AlphaCloud",
			TaxID:     "00-0000001",
			Address:   models.NewAddress("100 Market St", "94103", "San Francisco", "CA", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Industry:  "technology",
			Size:      "medium",
			Metadata: map[string]any{
				"tags":         "saas,cloud",
				"category":     "SaaS",
				"custom_field": "tech_demo_1",
			},
		},
		{
			LegalName: "Beta Marketplace LLC",
			TradeName: "BetaMarket",
			TaxID:     "00-0000002",
			Address:   models.NewAddress("500 7th Ave", "10018", "New York", "NY", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Industry:  "technology",
			Size:      "large",
			Metadata: map[string]any{
				"tags":     "marketplace,platform",
				"category": "Marketplace",
			},
		},
	}
}

// ECommerceBusinesses returns sample e-commerce organization templates.
func ECommerceBusinesses() []OrgTemplate {
	return []OrgTemplate{
		{
			LegalName: "Gamma Retail Online Ltd.",
			TradeName: "GammaShop",
			TaxID:     "00-0000010",
			Address:   models.NewAddress("200 Commerce Blvd", "60607", "Chicago", "IL", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Industry:  "ecommerce",
			Size:      "medium",
			Metadata: map[string]any{
				"tags":     "b2c",
				"category": "Retail",
			},
		},
		{
			LegalName: "Delta B2B Wholesale Corp.",
			TradeName: "DeltaWholesale",
			TaxID:     "00-0000011",
			Address:   models.NewAddress("50 Industrial Rd", "07030", "Hoboken", "NJ", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Industry:  "ecommerce",
			Size:      "large",
			Metadata: map[string]any{
				"tags":     "b2b,wholesale",
				"category": "Wholesale",
			},
		},
	}
}

// FinancialInstitutions returns sample financial organization templates.
func FinancialInstitutions() []OrgTemplate {
	return []OrgTemplate{
		{
			LegalName: "Epsilon Bank N.A.",
			TradeName: "EpsilonBank",
			TaxID:     "00-0000020",
			Address:   models.NewAddress("1 Finance Way", "10005", "New York", "NY", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Industry:  "financial",
			Size:      "enterprise",
			Metadata: map[string]any{
				"tags":     "bank,retail",
				"category": "Banking",
			},
		},
		{
			LegalName: "Zeta Payments Co.",
			TradeName: "ZetaPay",
			TaxID:     "00-0000021",
			Address:   models.NewAddress("1200 Gateway Dr", "94105", "San Francisco", "CA", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Industry:  "financial",
			Size:      "medium",
			Metadata: map[string]any{
				"tags":     "payments,processor",
				"category": "Payments",
			},
		},
	}
}

// HealthcareOrganizations returns sample healthcare organization templates.
func HealthcareOrganizations() []OrgTemplate {
	return []OrgTemplate{
		{
			LegalName: "Eta Health Systems",
			TradeName: "EtaHealth",
			TaxID:     "00-0000030",
			Address:   models.NewAddress("700 Wellness Ave", "90012", "Los Angeles", "CA", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Industry:  "healthcare",
			Size:      "large",
			Metadata: map[string]any{
				"tags":     "hospital,clinic",
				"category": "Healthcare",
			},
		},
		{
			LegalName: "Theta Health Insurance Group",
			TradeName: "ThetaInsure",
			TaxID:     "00-0000031",
			Address:   models.NewAddress("44 Insurance Pl", "30303", "Atlanta", "GA", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Industry:  "healthcare",
			Size:      "enterprise",
			Metadata: map[string]any{
				"tags":     "insurance",
				"category": "Insurance",
			},
		},
	}
}

// RetailChains returns sample retail chain organization templates.
func RetailChains() []OrgTemplate {
	return []OrgTemplate{
		{
			LegalName: "Iota Retail Group",
			TradeName: "IotaStores",
			TaxID:     "00-0000040",
			Address:   models.NewAddress("11 High St", "02108", "Boston", "MA", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Industry:  "retail",
			Size:      "large",
			Metadata: map[string]any{
				"tags":     "retail,omnichannel",
				"category": "Retail",
			},
		},
		{
			LegalName: "Kappa Online Retailers",
			TradeName: "KappaShop",
			TaxID:     "00-0000041",
			Address:   models.NewAddress("88 Liberty Rd", "73301", "Austin", "TX", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Industry:  "retail",
			Size:      "medium",
			Metadata: map[string]any{
				"tags":     "ecommerce",
				"category": "Retail",
			},
		},
	}
}

// DefaultOrganizations aggregates samples across industries.
func DefaultOrganizations() []OrgTemplate {
	out := []OrgTemplate{}
	groups := [][]OrgTemplate{TechCompanies(), ECommerceBusinesses(), FinancialInstitutions(), HealthcareOrganizations(), RetailChains()}

	for _, g := range groups {
		out = append(out, g...)
	}

	// Ensure metadata remains within limits via Validate (length checks handled separately)
	for i := range out {
		if out[i].Metadata == nil {
			out[i].Metadata = map[string]any{"template": fmt.Sprintf("org_%d", i)}
		}
	}

	return out
}

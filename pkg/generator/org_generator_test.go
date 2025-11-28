package generator

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: convert numeric string to slice of ints, ignoring non-digits
func digitsOf(s string) []int {
	out := make([]int, 0, len(s))

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			out = append(out, int(c-'0'))
		}
	}

	return out
}

func TestGenerateCNPJ_CheckDigitsValid(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	// weights used by algorithm
	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}

	for i := 0; i < 100; i++ {
		cnpj := generateCNPJ(r, false)
		ds := digitsOf(cnpj)

		if len(ds) != 14 {
			t.Fatalf("generated CNPJ should have 14 digits, got %d (%s)", len(ds), cnpj)
		}

		d1 := cnpjCheckDigit(ds[:12], w1)
		d2 := cnpjCheckDigit(ds[:13], w2)

		if ds[12] != d1 || ds[13] != d2 {
			t.Fatalf("invalid check digits for %s: got %d%d expected %d%d", cnpj, ds[12], ds[13], d1, d2)
		}
	}
}

func TestGenerateCNPJ_FormattedPattern(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	cnpj := generateCNPJ(r, true)

	if len(cnpj) != 18 {
		t.Fatalf("formatted CNPJ should have length 18, got %d (%s)", len(cnpj), cnpj)
	}

	if cnpj[2] != '.' || cnpj[6] != '.' || cnpj[10] != '/' || cnpj[15] != '-' {
		t.Fatalf("formatted CNPJ has wrong punctuation: %s", cnpj)
	}

	// verify digits still valid
	ds := digitsOf(cnpj)
	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	d1 := cnpjCheckDigit(ds[:12], w1)
	d2 := cnpjCheckDigit(ds[:13], w2)

	if ds[12] != d1 || ds[13] != d2 {
		t.Fatalf("invalid check digits for %s: got %d%d expected %d%d", cnpj, ds[12], ds[13], d1, d2)
	}
}

func TestGenerateEIN(t *testing.T) {
	tests := []struct {
		name string
		seed int64
	}{
		{"seed 1", 1},
		{"seed 42", 42},
		{"seed 12345", 12345},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := rand.New(rand.NewSource(tt.seed))
			ein := generateEIN(r)

			assert.Len(t, ein, 10)
			assert.Equal(t, '-', rune(ein[2]))

			prefix := ein[0:2]
			suffix := ein[3:10]

			for _, c := range prefix {
				assert.True(t, c >= '0' && c <= '9', "prefix must be numeric")
			}

			for _, c := range suffix {
				assert.True(t, c >= '0' && c <= '9', "suffix must be numeric")
			}
		})
	}
}

func TestGenerateEIN_Format(t *testing.T) {
	r := rand.New(rand.NewSource(100))

	for i := 0; i < 50; i++ {
		ein := generateEIN(r)
		assert.Regexp(t, `^\d{2}-\d{7}$`, ein)
	}
}

func TestCnpjCheckDigit(t *testing.T) {
	// Use a valid CNPJ to test: 11.222.333/0001-81
	// Base: 11222333000181
	// First 12 digits: 112223330001
	// First check digit: 8
	// Full 13 digits: 1122233300018
	// Second check digit: 1
	t.Run("Valid CNPJ check digits", func(t *testing.T) {
		// Test with the digits of CNPJ 11.222.333/0001-81
		// First 12 digits of 11222333000181 is 112223330001
		nums := []int{1, 1, 2, 2, 2, 3, 3, 3, 0, 0, 0, 1}
		w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
		d1 := cnpjCheckDigit(nums, w1)
		assert.Equal(t, 8, d1)

		// First 13 digits (with first check digit)
		nums13 := []int{1, 1, 2, 2, 2, 3, 3, 3, 0, 0, 0, 1, 8}
		w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
		d2 := cnpjCheckDigit(nums13, w2)
		assert.Equal(t, 1, d2)
	})

	t.Run("All zeros returns 0", func(t *testing.T) {
		nums := []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		weights := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
		result := cnpjCheckDigit(nums, weights)
		assert.Equal(t, 0, result)
	})
}

func TestCnpjCheckDigit_Consistency(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5, 6, 7, 8, 0, 0, 0, 1}
	weights := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}

	result1 := cnpjCheckDigit(nums, weights)
	result2 := cnpjCheckDigit(nums, weights)
	assert.Equal(t, result1, result2)
}

func TestGenerateCNPJ_BranchDigits(t *testing.T) {
	r := rand.New(rand.NewSource(999))

	for i := 0; i < 20; i++ {
		cnpj := generateCNPJ(r, false)
		ds := digitsOf(cnpj)
		assert.Equal(t, 0, ds[8])
		assert.Equal(t, 0, ds[9])
		assert.Equal(t, 0, ds[10])
		assert.Equal(t, 1, ds[11])
	}
}

func TestNewOrganizationGenerator(t *testing.T) {
	t.Run("Create with nil entity", func(t *testing.T) {
		gen := NewOrganizationGenerator(nil, nil)
		assert.NotNil(t, gen)
	})

	t.Run("Create with entity", func(t *testing.T) {
		e := &entities.Entity{}
		gen := NewOrganizationGenerator(e, nil)
		assert.NotNil(t, gen)
	})
}

func TestOrgGenerator_Generate_NilEntity(t *testing.T) {
	gen := NewOrganizationGenerator(nil, nil)
	template := data.OrgTemplate{
		LegalName: "Test Corp",
	}

	_, err := gen.Generate(context.Background(), template)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestOrgGenerator_Generate_NilOrganizationsService(t *testing.T) {
	e := &entities.Entity{}
	gen := NewOrganizationGenerator(e, nil)
	template := data.OrgTemplate{
		LegalName: "Test Corp",
	}

	_, err := gen.Generate(context.Background(), template)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestOrgGenerator_GenerateBatch_ZeroCount(t *testing.T) {
	gen := NewOrganizationGenerator(nil, nil)

	results, err := gen.GenerateBatch(context.Background(), 0)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestOrgGenerator_GenerateBatch_NegativeCount(t *testing.T) {
	gen := NewOrganizationGenerator(nil, nil)

	results, err := gen.GenerateBatch(context.Background(), -5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestOrgGenerator_GenerateBatch_NilEntity(t *testing.T) {
	gen := NewOrganizationGenerator(nil, nil)

	results, err := gen.GenerateBatch(context.Background(), 3)
	require.Error(t, err)
	assert.Empty(t, results)
}

func TestOrgGenerator_GenerateBatch_WithLocale(t *testing.T) {
	tests := []struct {
		name   string
		locale string
	}{
		{"BR locale", "br"},
		{"US locale", "us"},
		{"Default locale", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.locale != "" {
				ctx = WithOrgLocale(ctx, tt.locale)
			}

			locale := getOrgLocale(ctx)
			if tt.locale == "" {
				assert.Equal(t, "us", locale)
			} else {
				assert.Equal(t, tt.locale, locale)
			}
		})
	}
}

type mockOrganizationsService struct {
	createFunc func(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error)
}

func (m *mockOrganizationsService) CreateOrganization(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, input)
	}

	return &models.Organization{ID: "org-123"}, nil
}

func (*mockOrganizationsService) GetOrganization(_ context.Context, _ string) (*models.Organization, error) {
	return nil, errors.New("mock: GetOrganization not implemented")
}

func (*mockOrganizationsService) ListOrganizations(_ context.Context, _ *models.ListOptions) (*models.ListResponse[models.Organization], error) {
	return nil, errors.New("mock: ListOrganizations not implemented")
}

func (*mockOrganizationsService) UpdateOrganization(_ context.Context, _ string, _ *models.UpdateOrganizationInput) (*models.Organization, error) {
	return nil, errors.New("mock: UpdateOrganization not implemented")
}

func (*mockOrganizationsService) DeleteOrganization(_ context.Context, _ string) error {
	return nil
}

func (*mockOrganizationsService) GetOrganizationsMetricsCount(_ context.Context) (*models.MetricsCount, error) {
	return nil, errors.New("mock: GetOrganizationsMetricsCount not implemented")
}

func TestOrgGenerator_Generate_Success(t *testing.T) {
	mockSvc := &mockOrganizationsService{
		createFunc: func(_ context.Context, input *models.CreateOrganizationInput) (*models.Organization, error) {
			return &models.Organization{
				ID:        "org-success",
				LegalName: input.LegalName,
			}, nil
		},
	}

	e := &entities.Entity{
		Organizations: mockSvc,
	}

	gen := NewOrganizationGenerator(e, nil)
	template := data.OrgTemplate{
		LegalName: "Test Corporation",
		TradeName: "TestCo",
		TaxID:     "12-3456789",
		Status:    models.NewStatus(models.StatusActive),
		Address:   models.NewAddress("123 Main St", "12345", "Test City", "TS", "US"),
		Metadata: map[string]any{
			"key": "value",
		},
	}

	result, err := gen.Generate(context.Background(), template)
	require.NoError(t, err)
	assert.Equal(t, "org-success", result.ID)
	assert.Equal(t, "Test Corporation", result.LegalName)
}

func TestOrgGenerator_Generate_Error(t *testing.T) {
	mockSvc := &mockOrganizationsService{
		createFunc: func(_ context.Context, _ *models.CreateOrganizationInput) (*models.Organization, error) {
			return nil, errors.New("API error")
		},
	}

	e := &entities.Entity{
		Organizations: mockSvc,
	}

	gen := NewOrganizationGenerator(e, nil)
	template := data.OrgTemplate{
		LegalName: "Test Corporation",
	}

	result, err := gen.Generate(context.Background(), template)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "API error")
}

func TestOrgGenerator_GenerateBatch_WithWorkers(t *testing.T) {
	ctx := context.Background()
	ctx = WithWorkers(ctx, 4)

	workers := getWorkers(ctx)
	assert.Equal(t, 4, workers)
}

func TestGenerateCNPJ_Deterministic(t *testing.T) {
	r1 := rand.New(rand.NewSource(12345))
	r2 := rand.New(rand.NewSource(12345))

	cnpj1 := generateCNPJ(r1, true)
	cnpj2 := generateCNPJ(r2, true)

	assert.Equal(t, cnpj1, cnpj2)
}

func TestGenerateEIN_Deterministic(t *testing.T) {
	r1 := rand.New(rand.NewSource(54321))
	r2 := rand.New(rand.NewSource(54321))

	ein1 := generateEIN(r1)
	ein2 := generateEIN(r2)

	assert.Equal(t, ein1, ein2)
}

func TestGenerateCNPJ_Unformatted(t *testing.T) {
	r := rand.New(rand.NewSource(777))
	cnpj := generateCNPJ(r, false)

	assert.Len(t, cnpj, 14)

	for _, c := range cnpj {
		assert.True(t, c >= '0' && c <= '9', "unformatted CNPJ should only contain digits")
	}
}

func TestOrgTemplate_ValidStatus(t *testing.T) {
	statuses := []string{
		models.StatusActive,
		models.StatusInactive,
	}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			template := data.OrgTemplate{
				LegalName: "Test Corp",
				Status:    models.NewStatus(status),
			}
			assert.Equal(t, "Test Corp", template.LegalName)
			assert.NotNil(t, template.Status)
			assert.Equal(t, status, template.Status.Code)
		})
	}
}

func TestOrgTemplate_Metadata(t *testing.T) {
	t.Run("Nil metadata", func(t *testing.T) {
		template := data.OrgTemplate{
			LegalName: "Test Corp",
			Metadata:  nil,
		}
		assert.Equal(t, "Test Corp", template.LegalName)
		assert.Nil(t, template.Metadata)
	})

	t.Run("With metadata", func(t *testing.T) {
		template := data.OrgTemplate{
			LegalName: "Test Corp",
			Metadata: map[string]any{
				"industry": "technology",
				"size":     "enterprise",
			},
		}
		assert.Equal(t, "Test Corp", template.LegalName)
		assert.NotNil(t, template.Metadata)
		assert.Equal(t, "technology", template.Metadata["industry"])
		assert.Equal(t, "enterprise", template.Metadata["size"])
	})
}

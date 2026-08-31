package generator

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/concurrent"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/data"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/stats"
	fake "github.com/brianvoe/gofakeit/v7"
)

// organizationsAPI is the narrow slice of the organizations facade this
// generator needs (Epic 5.3 consumer-side interface).
type organizationsAPI interface {
	Create(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error)
}

type orgGenerator struct {
	orgs organizationsAPI
	obs  observability.Provider
	mc   *observability.MetricsCollector
}

// NewOrganizationGenerator creates a new OrganizationGenerator backed by entities API.
func NewOrganizationGenerator(e *entities.Entity, obs observability.Provider) OrganizationGenerator {
	var mc *observability.MetricsCollector

	if obs != nil && obs.IsEnabled() {
		if c, err := observability.NewMetricsCollector(obs); err == nil {
			mc = c
		}
	}

	g := &orgGenerator{obs: obs, mc: mc}
	if e != nil && e.V1.Organizations != nil {
		g.orgs = e.V1.Organizations
	}

	return g
}

// Generate creates a single organization from the provided template.
func (g *orgGenerator) Generate(ctx context.Context, template data.OrgTemplate) (*models.Organization, error) {
	ctx = normalizeContext(ctx)

	if g.orgs == nil {
		return nil, errors.New("entity organizations service not initialized")
	}

	input := models.NewCreateOrganizationInput(template.LegalName, template.TaxID).
		WithDoingBusinessAs(template.TradeName).
		WithLegalDocument(template.TaxID).
		WithStatus(template.Status).
		WithAddress(template.Address).
		WithMetadata(template.Metadata)

	var out *models.Organization

	err := observability.WithSpan(ctx, g.obs, "GenerateOrganization", func(ctx context.Context) error {
		// Respect retry + circuit breaker options from context if present
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				org, err := g.orgs.Create(ctx, input)
				if err != nil {
					return err
				}

				out = org

				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}

	if out == nil {
		return nil, errNilGenerated("organization")
	}

	return out, nil
}

// GenerateBatch creates multiple organizations concurrently.
func (g *orgGenerator) GenerateBatch(ctx context.Context, count int) ([]*models.Organization, error) {
	ctx = normalizeContext(ctx)

	if count <= 0 {
		return []*models.Organization{}, nil
	}

	// Prepare work items (templates)
	items := make([]int, count)
	for i := 0; i < count; i++ {
		items[i] = i
	}

	// Metrics timer for the whole batch
	var timer *observability.Timer
	if g.mc != nil {
		timer = g.mc.NewTimer(ctx, "organizations.batch.create", "organizations")
	}

	counter := stats.NewCounter()

	// Decide workers and buffering
	workers := getWorkers(ctx)

	buf := workers * 2
	results := concurrent.WorkerPool(ctx, items, func(ctx context.Context, i int) (*models.Organization, error) {
		// Seed per worker to diversify
		seed := time.Now().UnixNano() + int64(i)
		// #nosec G404 - non-cryptographic PRNG used only for demo data variety.
		r := rand.New(rand.NewSource(seed))
		// gofakeit.Seed returns an error only for documentation; the underlying
		// implementation always returns nil. Safe to ignore.
		_ = fake.Seed(seed) //nolint:errcheck // gofakeit.Seed always returns nil

		// Random company and DBA
		legal := fake.Company()

		trade := strings.ReplaceAll(strings.ToLower(legal), " ", "")
		if len(trade) > 16 {
			trade = trade[:16]
		}

		// EIN or CNPJ depending on locale
		taxID := taxDocumentFor(r, getOrgLocale(ctx), true)

		// Address
		addr := fake.Address()
		address := models.NewAddress(addr.Address, addr.Zip, addr.City, addr.State, addr.Country)

		// Industry and size
		industries := []string{"technology", "ecommerce", "financial", "healthcare", "retail"}
		sizes := []string{"small", "medium", "large", "enterprise"}

		t := data.OrgTemplate{
			LegalName: legal,
			TradeName: trade,
			TaxID:     taxID,
			Address:   address,
			Status:    models.NewStatus(models.StatusActive),
			Metadata: map[string]any{
				"source":     "generator",
				"created_at": time.Now().Format(time.RFC3339),
			},
			Industry: industries[r.Intn(len(industries))],
			Size:     sizes[r.Intn(len(sizes))],
		}

		org, err := g.Generate(ctx, t)
		if err == nil {
			counter.RecordSuccess()
		}

		return org, err
	}, concurrent.WithWorkers(workers), concurrent.WithBufferSize(buf))

	// Collect results and check errors
	out := make([]*models.Organization, 0, count)

	var errs []error

	for _, r := range results {
		if r.Error != nil {
			errs = append(errs, r.Error)
			continue
		}

		if r.Value == nil {
			errs = append(errs, errNilGenerated("organization"))
			continue
		}

		out = append(out, r.Value)
	}

	if timer != nil {
		timer.StopBatch(len(out))
	}

	// Log batch TPS
	if g.obs != nil && g.obs.IsEnabled() && g.obs.Logger() != nil {
		g.obs.Logger().Infof("organizations: created=%d tps=%.2f", counter.SuccessCount(), counter.TPS())
	}

	if len(errs) > 0 {
		return out, errorsJoin(errs...)
	}

	return out, nil
}

// taxDocumentFor returns a company tax document for the locale: a Brazilian
// CNPJ for "br", a US EIN otherwise. formatted applies to the CNPJ only — an
// EIN has one spelling.
func taxDocumentFor(r *rand.Rand, locale string, formatted bool) string {
	if strings.EqualFold(locale, "br") {
		return generateCNPJ(r, formatted)
	}

	return generateEIN(r)
}

// GenerateTaxDocument returns a company tax document appropriate to the locale:
// a Brazilian CNPJ with valid check digits for "br", a US EIN otherwise. When
// formatted is true the CNPJ carries its punctuation (NN.NNN.NNN/NNNN-NN);
// false yields the bare 14 digits, which is the spelling most CRM surfaces
// store.
//
// The locale match is CASE-INSENSITIVE: "BR", "Br" and "br" all select the
// CNPJ. A case-sensitive comparison used to hand "BR" a US EIN, which is a
// Brazilian organization carrying a US federal document — demo data that lies
// about its own shape, from a caller that spelled its locale in caps.
//
// It seeds from the auto-seeded global source rather than from the clock, so
// two calls in the same nanosecond still yield different documents. That is the
// whole reason for the indirection: a clock seed collides in a tight loop, and
// a demo that writes five holders with one document is not a demo of five
// holders. Non-cryptographic — for demo and fixture data only.
func GenerateTaxDocument(locale string, formatted bool) string {
	// #nosec G404 - non-cryptographic PRNG used only for demo data variety.
	r := rand.New(rand.NewSource(rand.Int63()))

	return taxDocumentFor(r, locale, formatted)
}

// generateEIN returns a US EIN in NN-NNNNNNN format.
func generateEIN(r *rand.Rand) string {
	return fmt.Sprintf("%02d-%07d", r.Intn(100), r.Intn(10_000_000))
}

// generateCNPJ generates a Brazilian CNPJ with valid check digits.
// When formatted is true, returns in the format NN.NNN.NNN/NNNN-NN.
func generateCNPJ(r *rand.Rand, formatted bool) string {
	// Base 12 digits
	base := make([]int, 12)
	for i := 0; i < 12; i++ {
		base[i] = r.Intn(10)
	}
	// Commonly the branch (positions 9-12) is 0001
	base[8], base[9], base[10], base[11] = 0, 0, 0, 1

	// First check digit
	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	d1 := cnpjCheckDigit(base, w1)

	// Second check digit - create temporary slice for calculation
	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	baseWithD1 := make([]int, 0, 13)
	baseWithD1 = append(baseWithD1, base...)
	baseWithD1 = append(baseWithD1, d1)
	d2 := cnpjCheckDigit(baseWithD1, w2)

	// Build final digits slice
	digits := make([]int, 0, 14)
	digits = append(digits, base...)
	digits = append(digits, d1, d2)

	if !formatted {
		// Plain 14 digits
		out := make([]byte, 0, 14)
		for _, d := range digits {
			out = append(out, decimalDigitByte(d))
		}

		return string(out)
	}
	// Format NN.NNN.NNN/NNNN-NN
	return fmt.Sprintf("%d%d.%d%d%d.%d%d%d/%d%d%d%d-%d%d",
		digits[0], digits[1], digits[2], digits[3], digits[4], digits[5], digits[6], digits[7],
		digits[8], digits[9], digits[10], digits[11], digits[12], digits[13],
	)
}

func cnpjCheckDigit(nums []int, weights []int) int {
	sum := 0

	for i := 0; i < len(weights); i++ {
		sum += nums[i] * weights[i]
	}

	mod := sum % 11

	if mod < 2 {
		return 0
	}

	return 11 - mod
}

func decimalDigitByte(d int) byte {
	if d < 0 || d > 9 {
		return '0'
	}

	return byte('0' + d)
}

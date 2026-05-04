package generator

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
)

type assetGenerator struct {
	e   *entities.Entity
	obs observability.Provider
}

const generatedAssetRateMaxScale = 18

// NewAssetGenerator creates a new AssetGenerator backed by entities API.
func NewAssetGenerator(e *entities.Entity, obs observability.Provider) AssetGenerator {
	return &assetGenerator{e: e, obs: obs}
}

// Generate creates an asset from the provided template.
func (g *assetGenerator) Generate(ctx context.Context, ledgerID string, template data.AssetTemplate) (*models.Asset, error) {
	ctx = normalizeContext(ctx)

	if g.e == nil || g.e.Assets == nil {
		return nil, errors.New("entity assets service not initialized")
	}
	// We require orgID to create assets; since Assets API needs orgID and ledgerID,
	// we cannot derive orgID from ledgerID here, so expect callers to embed org information in ctx.
	// To keep the interface stable, we attempt to extract orgID from context key if provided.
	// Type assertion ok value is intentionally ignored - empty string check handles both
	// cases (missing key and wrong type)
	orgID, _ := ctx.Value(contextKeyOrgID{}).(string) //nolint:errcheck // ok check unnecessary, empty string validated below
	if orgID == "" {
		return nil, errors.New("organization id missing in context for asset creation")
	}

	input := models.NewCreateAssetInputWithType(template.Name, template.Code, template.Type).
		WithStatus(models.NewStatus(models.StatusActive)).
		WithMetadata(mergeMetadata(template.Metadata, map[string]any{"scale": template.Scale}))

	var out *models.Asset

	err := observability.WithSpan(ctx, g.obs, "GenerateAsset", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				asset, err := g.e.Assets.CreateAsset(ctx, orgID, ledgerID, input)
				if err != nil {
					return err
				}

				out = asset

				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}

	if out == nil {
		return nil, errNilGenerated("asset")
	}

	return out, nil
}

// GenerateWithRates creates an asset with rate management (not implemented in current SDK version).
func (g *assetGenerator) GenerateWithRates(ctx context.Context, ledgerID, baseAsset string) error {
	baseAsset = strings.ToUpper(strings.TrimSpace(baseAsset))
	if baseAsset == "" {
		return errors.New("base asset is required")
	}

	rates, err := defaultAssetRatesFrom(baseAsset)
	if err != nil {
		return err
	}

	return g.updateRatesFrom(ctx, ledgerID, baseAsset, rates)
}

// UpdateRates creates or updates asset rates using the organization ID stored in context via WithOrgID.
func (g *assetGenerator) UpdateRates(ctx context.Context, ledgerID string, rates map[string]float64) error {
	return g.updateRatesFrom(ctx, ledgerID, "USD", rates)
}

func defaultAssetRatesFrom(baseAsset string) (map[string]float64, error) {
	usdRates := map[string]float64{"USD": 1, "EUR": 0.92, "BRL": 5.25}

	baseRate, ok := usdRates[baseAsset]
	if !ok {
		return nil, fmt.Errorf("unsupported base asset %q", baseAsset)
	}

	rates := make(map[string]float64, len(usdRates)-1)
	for asset, usdRate := range usdRates {
		if asset == baseAsset {
			continue
		}

		rates[asset] = usdRate / baseRate
	}

	return rates, nil
}

func scaledAssetRate(rate float64) (rateValue int, scale int, err error) {
	text := strconv.FormatFloat(rate, 'f', -1, 64)

	if dot := strings.IndexByte(text, '.'); dot >= 0 {
		scale = len(text) - dot - 1
	}

	if scale > generatedAssetRateMaxScale {
		scale = generatedAssetRateMaxScale
	}

	scaled := math.Round(rate * math.Pow10(scale))

	maxInt := int(^uint(0) >> 1)
	if scaled > float64(maxInt) {
		return 0, 0, fmt.Errorf("asset rate %v exceeds integer range at scale %d", rate, scale)
	}

	return int(scaled), scale, nil
}

func (g *assetGenerator) updateRatesFrom(ctx context.Context, ledgerID, fromAsset string, rates map[string]float64) error {
	ctx = normalizeContext(ctx)

	if g.e == nil || g.e.AssetRates == nil {
		return errors.New("entity asset rates service not initialized")
	}

	orgID, _ := ctx.Value(contextKeyOrgID{}).(string) //nolint:errcheck // Empty string is validated below.
	if orgID == "" || ledgerID == "" || fromAsset == "" {
		return errors.New("organization and ledger IDs are required for asset rate updates")
	}

	var errs []error

	for toAsset, rate := range rates {
		if toAsset == "" || math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 {
			errs = append(errs, fmt.Errorf("invalid asset rate for %s", toAsset))
			continue
		}

		scaledRate, scale, err := scaledAssetRate(rate)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		input := models.NewCreateAssetRateInput(fromAsset, toAsset, scaledRate).
			WithScale(scale).
			WithSource("mass-demo-generator")
		if _, err := g.e.AssetRates.CreateOrUpdateAssetRate(ctx, orgID, ledgerID, input); err != nil {
			errs = append(errs, err)
		}
	}

	return errorsJoin(errs...)
}

// contextKeyOrgID is a private key to extract orgID from context for asset creation.
type contextKeyOrgID struct{}

// WithOrgID returns a derived context that carries the organization ID.
func WithOrgID(ctx context.Context, orgID string) context.Context {
	ctx = normalizeContext(ctx)

	return context.WithValue(ctx, contextKeyOrgID{}, orgID)
}

func mergeMetadata(a map[string]any, b map[string]any) map[string]any {
	if a == nil && b == nil {
		return nil
	}

	out := map[string]any{}

	for k, v := range a {
		out[k] = v
	}

	for k, v := range b {
		out[k] = v
	}

	return out
}

package generator

import (
    "context"
    "fmt"

    "github.com/LerianStudio/midaz-sdk-golang/v2/entities"
    "github.com/LerianStudio/midaz-sdk-golang/v2/models"
    data "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
)

type assetGenerator struct {
    e   *entities.Entity
    obs observability.Provider
}

// NewAssetGenerator creates a new AssetGenerator backed by entities API.
func NewAssetGenerator(e *entities.Entity, obs observability.Provider) AssetGenerator {
    return &assetGenerator{e: e, obs: obs}
}

func (g *assetGenerator) Generate(ctx context.Context, ledgerID string, template data.AssetTemplate) (*models.Asset, error) {
    if g.e == nil || g.e.Assets == nil {
        return nil, fmt.Errorf("entity assets service not initialized")
    }
    // We require orgID to create assets; since Assets API needs orgID and ledgerID,
    // we cannot derive orgID from ledgerID here. For Phase 3, expect caller to embed org in ctx.
    // To keep the interface stable, we attempt to extract orgID from context key if provided.
    orgID, _ := ctx.Value(contextKeyOrgID{}).(string)
    if orgID == "" {
        return nil, fmt.Errorf("organization id missing in context for asset creation")
    }

    input := models.NewCreateAssetInput(template.Name, template.Code).
        WithType(template.Type).
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
    return out, nil
}

func (g *assetGenerator) GenerateWithRates(ctx context.Context, ledgerID string, baseAsset string) error {
    // Rate management is not exposed in current SDK; defer to a future phase.
    return fmt.Errorf("asset rate management not implemented in this SDK version")
}

func (g *assetGenerator) UpdateRates(ctx context.Context, ledgerID string, rates map[string]float64) error {
    // Rate management is not exposed in current SDK; defer to a future phase.
    return fmt.Errorf("asset rate management not implemented in this SDK version")
}

// contextKeyOrgID is a private key to extract orgID from context for asset creation.
type contextKeyOrgID struct{}

// WithOrgID returns a derived context that carries the organization ID.
func WithOrgID(ctx context.Context, orgID string) context.Context {
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

package client

import (
	"context"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type typedNilProvider struct{}

func (*typedNilProvider) Tracer() trace.Tracer           { return nil }
func (*typedNilProvider) Meter() metric.Meter            { return nil }
func (*typedNilProvider) Logger() observability.Logger   { return nil }
func (*typedNilProvider) IsEnabled() bool                { return true }
func (*typedNilProvider) Shutdown(context.Context) error { return nil }

func TestSlice8ClientHelpers(t *testing.T) {
	t.Run("typed nil observability provider returns error", func(t *testing.T) {
		var provider *typedNilProvider

		_, err := New(WithObservabilityProvider(provider))
		require.Error(t, err)
	})

	t.Run("WithConfig clones caller-owned config", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.ServiceURLs[config.ServiceOnboarding] = "https://original.example.com/v1"

		c, err := New(WithConfig(cfg))
		require.NoError(t, err)

		cfg.ServiceURLs[config.ServiceOnboarding] = "https://mutated.example.com/v1"
		require.Equal(t, "https://original.example.com/v1", c.config.ServiceURLs[config.ServiceOnboarding])

		returned := c.GetConfig()
		returned.ServiceURLs[config.ServiceOnboarding] = "https://returned.example.com/v1"
		require.Equal(t, "https://original.example.com/v1", c.config.ServiceURLs[config.ServiceOnboarding])
	})

	t.Run("nil shutdown is safe", func(t *testing.T) {
		var c *Client
		require.NoError(t, c.Shutdown(context.Background()))
	})
}

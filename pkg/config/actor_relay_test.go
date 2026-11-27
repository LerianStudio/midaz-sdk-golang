package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// actorRelayFakeToken is an obviously fake partner token.
const actorRelayFakeToken = "fake-actor-token"

func fakeActorSource(context.Context) (string, bool) { return actorRelayFakeToken, true }

// TestWithActorRelay covers the option, its getter and its survival across
// Clone. Subtest names carrying a spec scenario name trace back to
// specs/ledger-actor-relay/spec.md.
func TestWithActorRelay(t *testing.T) {
	t.Run("option sets the source read by the getter", func(t *testing.T) {
		cfg, err := NewConfig(WithAnonymous(), WithActorRelay(fakeActorSource))
		require.NoError(t, err)

		source := cfg.GetActorTokenSource()
		require.NotNil(t, source)

		token, ok := source(context.Background())
		assert.True(t, ok)
		assert.Equal(t, actorRelayFakeToken, token)
	})

	t.Run("relay is off by default", func(t *testing.T) {
		cfg, err := NewConfig(WithAnonymous())
		require.NoError(t, err)

		assert.Nil(t, cfg.GetActorTokenSource())
	})

	t.Run("fonte-nula-recusada-na-construcao", func(t *testing.T) {
		cfg, err := NewConfig(WithAnonymous(), WithActorRelay(nil))
		require.Error(t, err)
		assert.Nil(t, cfg, "no config must be built from a nil source")
		assert.Contains(t, err.Error(), "actor token source cannot be nil")
	})

	t.Run("option on a nil config returns an error", func(t *testing.T) {
		require.Error(t, WithActorRelay(fakeActorSource)(nil))
	})

	t.Run("getter on a nil config returns nil", func(t *testing.T) {
		var cfg *Config

		assert.Nil(t, cfg.GetActorTokenSource())
	})

	t.Run("clone keeps the source", func(t *testing.T) {
		cfg, err := NewConfig(WithAnonymous(), WithActorRelay(fakeActorSource))
		require.NoError(t, err)

		source := cfg.Clone().GetActorTokenSource()
		require.NotNil(t, source)

		token, ok := source(context.Background())
		assert.True(t, ok)
		assert.Equal(t, actorRelayFakeToken, token)
	})
}

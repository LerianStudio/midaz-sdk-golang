package config

import (
	"net/http"
	"testing"
	"time"

	auth "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager"
	"github.com/stretchr/testify/require"
)

func TestSlice8ConfigStrictnessAndCopying(t *testing.T) {
	t.Run("environment integers reject malformed suffixes", func(t *testing.T) {
		for _, value := range []string{"1.5", "1a", "123abc", "10s"} {
			_, err := parseEnvInt(value)
			require.Error(t, err, value)
		}

		got, err := parseEnvInt(" 42 ")
		require.NoError(t, err)
		require.Equal(t, 42, got)
	})

	t.Run("public options reject nil config", func(t *testing.T) {
		options := []Option{
			WithDebug(true),
			WithIdempotency(true),
			WithTenantID("tenant"),
			WithMaxRetries(1),
			WithRetryWaitMin(time.Millisecond),
			WithRetryWaitMax(time.Second),
		}

		for _, option := range options {
			require.Error(t, option(nil))
		}
	})

	t.Run("clone copies service URL map", func(t *testing.T) {
		cfg, err := NewConfig(WithAccessManager(auth.AccessManager{}))
		require.NoError(t, err)

		cloned := cfg.Clone()
		require.NotSame(t, cfg, cloned)

		cloned.ServiceURLs[ServiceOnboarding] = "https://changed.example.com/v1"
		require.NotEqual(t, cfg.ServiceURLs[ServiceOnboarding], cloned.ServiceURLs[ServiceOnboarding])
	})

	t.Run("default client rejects redirect userinfo", func(t *testing.T) {
		client := NewDefaultHTTPClient(time.Second)
		req, err := http.NewRequest(http.MethodGet, "https://user:pass@example.com", nil)
		require.NoError(t, err)
		require.Error(t, client.CheckRedirect(req, nil))
	})
}

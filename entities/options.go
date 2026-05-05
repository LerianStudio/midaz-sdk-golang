package entities

import (
	"errors"
	"net/http"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
)

// Option is a function that configures an Entity.
type Option func(*Entity) error

// WithDebug returns an Option that enables or disables debug mode for the Entity.
func WithDebug(debug bool) Option {
	return func(e *Entity) error {
		if e == nil || e.httpClient == nil {
			return errors.New("entity HTTP client cannot be nil")
		}

		e.httpClient.setDebugLocked(debug)

		return nil
	}
}

// WithUserAgent returns an Option that sets the user agent for the Entity.
func WithUserAgent(userAgent string) Option {
	return func(e *Entity) error {
		if e == nil || e.httpClient == nil {
			return errors.New("entity HTTP client cannot be nil")
		}

		e.httpClient.setUserAgentLocked(userAgent)

		return nil
	}
}

// WithObservability returns an Option that sets the observability provider for the Entity.
func WithObservability(provider observability.Provider) Option {
	return func(e *Entity) error {
		if e == nil || e.httpClient == nil {
			return errors.New("entity HTTP client cannot be nil")
		}

		if provider == nil {
			return nil // No-op if the provider is nil
		}

		// Reconfigure the HTTP client through a single locked setter so the
		// provider and its derived metrics collector flip atomically.
		var metrics *observability.MetricsCollector

		if provider.IsEnabled() {
			collector, err := observability.NewMetricsCollector(provider)
			if err != nil {
				return err
			}

			metrics = collector
		}

		e.httpClient.setObservabilityLocked(provider, metrics)
		e.observability = provider

		return nil
	}
}

// WithHTTPClient returns an Option that sets the HTTP client for the Entity.
// The tenant ID configured on the entity is preserved across the replacement.
func WithHTTPClient(client *http.Client) Option {
	return func(e *Entity) error {
		if e == nil {
			return errors.New("entity cannot be nil")
		}

		if client == nil {
			return errors.New("HTTP client cannot be nil")
		}

		if e.httpClient == nil {
			e.httpClient = NewHTTPClient(client, "", e.observability)
			e.initServices()

			return nil
		}

		savedConfig := e.httpClient.cloneConfiguration()

		// Create a new HTTP client with the same auth token and observability
		e.httpClient = NewHTTPClient(client, e.httpClient.authToken, e.observability)
		e.httpClient.applyConfigurationSnapshot(savedConfig)

		// Re-initialize services with the new HTTP client
		e.initServices()

		return nil
	}
}

// WithDefaultTenantID returns an Option that sets the default tenant ID for all
// requests made through this Entity. Per-request tenant IDs set via
// WithTenantID(ctx, tenantID) take precedence over this default.
// If tenantID is empty, the option is a no-op.
func WithDefaultTenantID(tenantID string) Option {
	return func(e *Entity) error {
		if e == nil || e.httpClient == nil {
			return errors.New("entity HTTP client cannot be nil")
		}

		tenantID = strings.TrimSpace(tenantID)
		if tenantID == "" {
			return nil
		}

		e.httpClient.setTenantIDLocked(tenantID)

		return nil
	}
}

// WithPluginAuth returns an Option that configures plugin-based authentication.
// This is a wrapper around auth.WithAccessManager to make it compatible with entities.Option.
func WithPluginAuth(pluginAuth auth.AccessManager) Option {
	return func(e *Entity) error {
		// Call the auth.WithAccessManager function with the entity
		return auth.WithAccessManager(pluginAuth)(e)
	}
}

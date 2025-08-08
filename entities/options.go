package entities

import (
	"context"
	"fmt"
	"net/http"

	auth "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
)

// Option is a function that configures an Entity.
type Option func(*Entity) error

// WithDebug returns an Option that enables or disables debug mode for the Entity.
func WithDebug(debug bool) Option {
	return func(e *Entity) error {
		fmt.Printf("[SDK] Setting debug mode to: %v\n", debug)
		e.httpClient.debug = debug

		return nil
	}
}

// WithUserAgent returns an Option that sets the user agent for the Entity.
func WithUserAgent(userAgent string) Option {
	return func(e *Entity) error {
		e.httpClient.userAgent = userAgent

		return nil
	}
}

// WithObservability returns an Option that sets the observability provider for the Entity.
func WithObservability(provider observability.Provider) Option {
	return func(e *Entity) error {
		if provider == nil {
			return nil // No-op if the provider is nil
		}

		// Set the provider on the entity
		e.observability = provider

		// Set the provider on the HTTP client
		e.httpClient.observability = provider

		// Create metrics collector if needed
		if provider.IsEnabled() {
			var err error
			e.httpClient.metrics, err = observability.NewMetricsCollector(provider)

			if err != nil {
				return err
			}
		}

		return nil
	}
}

// WithContext returns an Option that sets the context for the Entity.
func WithContext(ctx context.Context) Option {
	return func(e *Entity) error {
		if ctx == nil {
			return fmt.Errorf("context cannot be nil")
		}

		// Set the context in the HTTP client if it has a context field
		// Note: This assumes the HTTP client has a context field, which may need to be added
		// e.httpClient.ctx = ctx

		return nil
	}
}

// WithHTTPClient returns an Option that sets the HTTP client for the Entity.
func WithHTTPClient(client *http.Client) Option {
	return func(e *Entity) error {
		if client == nil {
			return fmt.Errorf("HTTP client cannot be nil")
		}

		// Create a new HTTP client with the same auth token and observability
		e.httpClient = NewHTTPClient(client, e.httpClient.authToken, e.observability)

		// Re-initialize services with the new HTTP client
		e.initServices()

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

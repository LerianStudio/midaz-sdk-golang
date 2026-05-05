package entities

import (
	"errors"

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

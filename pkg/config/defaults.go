package config

import (
	"errors"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/version"
)

// DefaultConfig creates a new Config with default values.
// Unlike NewConfig, this doesn't validate required fields, making it suitable for initialization
// before applying options.
//
// Returns:
//   - *Config: A new configuration with default values
func DefaultConfig() *Config {
	// Create a config with default values
	config := &Config{
		Environment:       EnvironmentLocal,
		ServiceURLs:       make(map[ServiceType]string),
		Timeout:           DefaultTimeout * time.Second,
		UserAgent:         version.UserAgent(),
		MaxRetries:        DefaultMaxRetries,
		RetryWaitMin:      DefaultMinRetryWait,
		RetryWaitMax:      DefaultRetryWaitMax,
		EnableIdempotency: DefaultEnableIdempotency,
		ExposeErrorBody:   DefaultExposeErrorBody,
	}

	// Apply default URLs based on environment.
	// Error is safely ignored because DefaultConfig always uses EnvironmentLocal
	// which is a valid, known environment that will never return an error.
	_ = setDefaultServiceURLs(config) //nolint:errcheck // EnvironmentLocal is hardcoded above and always valid

	// Create HTTP client
	config.HTTPClient = NewDefaultHTTPClient(config.Timeout)
	config.httpClientOwned = true

	return config
}

// WithMaxRetries sets the maximum number of retries for HTTP requests.
//
// Parameters:
//   - maxRetries: The maximum number of retry attempts
//
// Returns:
//   - Option: A function that sets the max retries on a Config
func WithMaxRetries(maxRetries int) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if maxRetries < 0 {
			return errors.New("max retries cannot be negative")
		}

		c.MaxRetries = maxRetries

		return nil
	}
}

// WithRetryWaitMin sets the minimum wait time between retries.
//
// The Option rejects values that would invert the retry-wait pair:
// applying WithRetryWaitMin with a value greater than the current
// RetryWaitMax returns an error rather than silently producing a
// nonsensical (min > max) configuration.
//
// Parameters:
//   - waitTime: The minimum wait time between retries
//
// Returns:
//   - Option: A function that sets the minimum retry wait time on a Config
func WithRetryWaitMin(waitTime time.Duration) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if waitTime <= 0 {
			return errors.New("minimum wait time must be greater than 0")
		}

		if c.RetryWaitMax > 0 && waitTime > c.RetryWaitMax {
			return errors.New("minimum wait time must be less than or equal to maximum wait time")
		}

		c.RetryWaitMin = waitTime

		return nil
	}
}

// WithRetryWaitMax sets the maximum wait time between retries.
//
// Parameters:
//   - waitTime: The maximum wait time between retries
//
// Returns:
//   - Option: A function that sets the maximum retry wait time on a Config
func WithRetryWaitMax(waitTime time.Duration) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if waitTime <= 0 {
			return errors.New("maximum wait time must be greater than 0")
		}

		if waitTime < c.RetryWaitMin {
			return errors.New("maximum wait time must be greater than or equal to minimum wait time")
		}

		c.RetryWaitMax = waitTime

		return nil
	}
}

// NewLocalConfig creates a Config for local development.
// This is a convenience function that combines:
//   - Environment defaults pinned to EnvironmentLocal
//   - Full env-var loading via FromEnvironment (PLUGIN_AUTH_*, MIDAZ_*, proxy)
//   - Caller-supplied options applied last so they override env values
//
// Parameters:
//   - options: Additional options to apply after the local + env-driven baseline
//
// Returns:
//   - *Config: A configuration for local development
//   - error: An error if configuration fails
//
// Behavior change vs v2: v2 only loaded PLUGIN_AUTH_* env vars in this path.
// v3 routes every env-driven knob through FromEnvironment so there is exactly
// one place that reads the environment.
//
// Auth posture: NewLocalConfig is for local development — it pre-applies
// [WithAnonymous] so the config validates without requiring auth credentials.
// Callers that DO want auth on a local config can override by appending
// [WithAccessManager] in the options list (last-applied wins, and
// WithAccessManager clears Anonymous).
func NewLocalConfig(options ...Option) (*Config, error) {
	localOptions := append(
		[]Option{
			WithEnvironment(EnvironmentLocal),
			WithAnonymous(),
			FromEnvironment(),
		},
		options...,
	)

	return NewConfig(localOptions...)
}

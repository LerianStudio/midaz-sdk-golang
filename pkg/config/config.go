// Package config provides configuration management for the Midaz SDK.
//
// This package centralizes all configuration options for the SDK, including:
// - API endpoints and authentication
// - HTTP client settings like timeouts and retries
// - Feature flags and behavior controls
//
// It uses the functional options pattern for flexible, type-safe configuration.
package config

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/internal/reflectutil"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/security"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/version"
)

// ServiceType represents a type of service in the Midaz API ecosystem.
type ServiceType string

// Service types constants define the available Midaz services.
const (
	// ServiceOnboarding is the internal routing label for the onboarding subset
	// of Ledger endpoints. It shares its base URL with [ServiceTransaction];
	// both are populated from [WithLedgerURL].
	ServiceOnboarding ServiceType = "onboarding"

	// ServiceTransaction is the internal routing label for the transaction
	// subset of Ledger endpoints. See [ServiceOnboarding] for the
	// shared-base-URL note.
	ServiceTransaction ServiceType = "transaction"

	// ServiceCRM represents the CRM service.
	ServiceCRM ServiceType = "crm"
)

// Environment represents a deployment environment for the Midaz API.
type Environment string

// Environment constants define the available Midaz environments.
const (
	// EnvironmentLocal represents a local development environment.
	EnvironmentLocal Environment = "local"

	// EnvironmentDevelopment represents a development/staging environment.
	EnvironmentDevelopment Environment = "development"

	// EnvironmentProduction represents the production environment.
	EnvironmentProduction Environment = "production"
)

// Default configuration values
const (
	// Default timeout for HTTP requests in seconds
	DefaultTimeout = 60

	// Default URLs for each environment
	DefaultLocalLedgerBaseURL       = "http://localhost:3002"
	DefaultLocalCRMBaseURL          = "http://localhost:4003"
	DefaultDevelopmentLedgerBaseURL = "https://api.dev.midaz.io"
	DefaultProductionLedgerBaseURL  = "https://api.midaz.io"
	DefaultLedgerAPIVersionPath     = "/v1"

	// Default retry configuration
	DefaultMaxRetries   = 3
	DefaultMinRetryWait = 1 * time.Second
	DefaultRetryWaitMax = 30 * time.Second

	// Default feature flags
	DefaultEnableIdempotency = true
	DefaultExposeErrorBody   = false
)

// Config holds the configuration for the Midaz SDK.
// It centralizes all settings needed to interact with the Midaz API.
type Config struct {
	// AccessManager configuration for authentication
	AccessManager auth.AccessManager

	// Environment specifies which Midaz environment to connect to.
	// This affects the default URLs used if not explicitly overridden.
	Environment Environment

	// ServiceURLs maps service types to their base URLs.
	// These take precedence over Environment-based URLs.
	ServiceURLs map[ServiceType]string

	// HTTPClient is the HTTP client to use for requests.
	// If nil, a default client will be created with the configured timeout.
	HTTPClient *http.Client

	// Timeout is the timeout for HTTP requests.
	Timeout time.Duration

	// UserAgent is the user agent string sent in HTTP requests.
	UserAgent string

	// Retry configuration for failed requests.
	// Retries are off when MaxRetries == 0; there is no separate enable flag.
	// Use [github.com/LerianStudio/midaz-sdk-golang/v3.WithoutRetries] (canonical
	// off-switch) or [WithMaxRetries](0) to disable.
	MaxRetries   int
	RetryWaitMin time.Duration
	RetryWaitMax time.Duration

	// Debug enables verbose logging of requests and responses.
	Debug bool

	// ObservabilityProvider for tracing, metrics, and logging.
	ObservabilityProvider observability.Provider

	// EnableIdempotency enables automatic generation of idempotency keys.
	EnableIdempotency bool

	// ExposeErrorBody controls whether upstream 4xx/5xx response bodies are
	// attached to SDK errors. The attached body is raw and only truncated.
	ExposeErrorBody bool

	baseURLSet      bool
	ledgerURLSet    bool
	crmURLSet       bool
	environmentSet  bool
	httpClientOwned bool

	// skipAuthCheck bypasses auth validation for package-internal tests only.
	// It is deliberately not populated from environment variables.
	skipAuthCheck bool

	// Anonymous is the explicit acknowledgment that the client is being
	// constructed without any authentication source. Programmatic callers set
	// this via [github.com/LerianStudio/midaz-sdk-golang/v3.WithAnonymous] (the
	// midaz package re-export) to prove that omitting AccessManager was
	// intentional — typically for local development against an unsecured
	// midaz-onboarding/midaz-transaction stack, or for tests. v3 rejects
	// construction with no auth source AND no Anonymous=true via validateConfig.
	Anonymous bool

	// AllowInsecureHTTP opts the configured Ledger and CRM service URLs out
	// of the SDK's "http:// only for localhost" gate. Default is false
	// (strict). Set this BEFORE applying [WithLedgerURL], [WithCRMURL], or
	// [WithBaseURL] — those option setters validate their input via
	// [parseURL], which honors the flag value at the time it runs.
	//
	// Intended for Kubernetes cluster-internal services reached over the
	// cluster mesh (e.g. http://midaz-ledger.midaz-mt.svc.cluster.local:3000)
	// where TLS is terminated by the service mesh, and for development or
	// test deployments behind a controlled network boundary. Production
	// deployments over the public internet must leave this off.
	//
	// This flag is independent of [auth.AccessManager.AllowInsecureHTTP],
	// which gates the Access Manager (auth) URL. The two are decoupled so a
	// client may run HTTPS against the Access Manager while reaching the
	// Ledger over an in-cluster HTTP service, or vice versa.
	AllowInsecureHTTP bool
}

// Option is a function that configures a Config.
// It's the core of the functional options pattern used throughout the SDK.
type Option func(*Config) error

// WithEnvironment sets the environment for the Config.
// Two-layer surface: this is the internal/test-layer Option that operates on
// [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithEnvironment] is what most callers
// should use; it composes with [github.com/LerianStudio/midaz-sdk-golang/v3.New]
// directly.
//
// This determines the default URLs used for services if not explicitly overridden.
//
// Parameters:
//   - env: The environment to use (EnvironmentLocal, EnvironmentDevelopment, EnvironmentProduction)
//
// Returns:
//   - Option: A function that sets the environment on a Config
func WithEnvironment(env Environment) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		c.Environment = env
		c.environmentSet = true
		if !c.baseURLSet {
			if err := setDefaultServiceURLs(c); err != nil {
				return err
			}
		}

		return nil
	}
}

// WithLedgerURL sets the base URL for the Ledger API. The Ledger service
// serves both onboarding and transaction endpoints under the same plane, so
// a single URL is the canonical configuration shape.
//
// Two-layer surface: this is the internal/test-layer Option that operates on
// [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithLedgerURL] is what most callers
// should use; it composes with [github.com/LerianStudio/midaz-sdk-golang/v3.New]
// directly.
//
// This overrides any URL derived from the Environment setting.
//
// Parameters:
//   - ledgerURL: The base URL for the Ledger API
//
// Returns:
//   - Option: A function that sets the Ledger URL on a Config
//   - May return an error if the URL is invalid
func WithLedgerURL(ledgerURL string) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if err := parseURLWithInsecureHTTP(ledgerURL, c.AllowInsecureHTTP); err != nil {
			return fmt.Errorf("invalid ledger URL: %w", err)
		}

		if c.ServiceURLs == nil {
			c.ServiceURLs = make(map[ServiceType]string)
		}

		trimmed := strings.TrimRight(ledgerURL, "/")
		c.ServiceURLs[ServiceOnboarding] = trimmed
		c.ServiceURLs[ServiceTransaction] = trimmed
		c.ledgerURLSet = true

		return nil
	}
}

// WithCRMURL sets the base URL for the CRM API.
// Two-layer surface: this is the internal/test-layer Option that operates on
// [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithCRMURL] is what most callers
// should use; it composes with [github.com/LerianStudio/midaz-sdk-golang/v3.New]
// directly.
func WithCRMURL(crmURL string) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if err := parseURLWithInsecureHTTP(crmURL, c.AllowInsecureHTTP); err != nil {
			return fmt.Errorf("invalid crm URL: %w", err)
		}

		if c.ServiceURLs == nil {
			c.ServiceURLs = make(map[ServiceType]string)
		}

		c.ServiceURLs[ServiceCRM] = strings.TrimRight(crmURL, "/")
		c.crmURLSet = true

		return nil
	}
}

// WithBaseURL sets a common base URL that will be used for all services.
// Two-layer surface: this is the internal/test-layer Option that operates on
// [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithBaseURL] is what most callers
// should use; it composes with [github.com/LerianStudio/midaz-sdk-golang/v3.New]
// directly.
//
// Service-specific ports and paths will be automatically added.
// This is useful for connecting to custom deployments.
//
// Parameters:
//   - baseURL: The base URL (e.g., "http://example.com")
//
// Returns:
//   - Option: A function that sets all service URLs derived from the base URL
//   - May return an error if the URL is invalid
func WithBaseURL(baseURL string) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		// Validate the base URL
		if err := parseURLWithInsecureHTTP(baseURL, c.AllowInsecureHTTP); err != nil {
			return fmt.Errorf("invalid base URL: %w", err)
		}

		// Remove trailing slash if present
		baseURL = strings.TrimSuffix(baseURL, "/")

		// Initialize the map if needed
		if c.ServiceURLs == nil {
			c.ServiceURLs = make(map[ServiceType]string)
		}

		ledgerURL, err := buildLedgerServiceURL(baseURL)
		if err != nil {
			return fmt.Errorf("invalid ledger base URL: %w", err)
		}

		crmURL, err := buildCRMServiceURL(baseURL)
		if err != nil {
			return fmt.Errorf("invalid crm base URL: %w", err)
		}

		if !c.ledgerURLSet {
			c.ServiceURLs[ServiceOnboarding] = ledgerURL
			c.ServiceURLs[ServiceTransaction] = ledgerURL
		}

		if !c.crmURLSet {
			c.ServiceURLs[ServiceCRM] = crmURL
		}

		c.baseURLSet = true

		return nil
	}
}

// WithHTTPClient sets a custom HTTP client for the Config.
// Two-layer surface: this is the internal/test-layer Option that operates on
// [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithHTTPClient] is what most callers
// should use; it composes with [github.com/LerianStudio/midaz-sdk-golang/v3.New]
// directly.
//
// This allows for advanced customization of the HTTP client behavior.
//
// Parameters:
//   - client: The HTTP client to use
//
// Returns:
//   - Option: A function that sets the HTTP client on a Config
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if client == nil {
			return errors.New("HTTP client cannot be nil")
		}

		c.HTTPClient = security.EnsureRedirectPolicy(client)
		c.httpClientOwned = false

		return nil
	}
}

// WithTimeout sets the timeout duration for HTTP requests.
// Two-layer surface: this is the internal/test-layer Option that operates on
// [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithTimeout] is what most callers
// should use; it composes with [github.com/LerianStudio/midaz-sdk-golang/v3.New]
// directly.
//
// Parameters:
//   - timeout: The timeout duration
//
// Returns:
//   - Option: A function that sets the timeout on a Config
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if timeout <= 0 {
			return errors.New("timeout must be greater than 0")
		}

		c.Timeout = timeout
		if c.HTTPClient != nil && c.httpClientOwned {
			c.HTTPClient.Timeout = timeout
		}

		return nil
	}
}

// WithUserAgent sets the user agent for HTTP requests.
// Two-layer surface: this is the internal/test-layer Option that operates on
// [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithUserAgent] is what most callers
// should use; it composes with [github.com/LerianStudio/midaz-sdk-golang/v3.New]
// directly.
//
// Parameters:
//   - userAgent: The user agent string
//
// Returns:
//   - Option: A function that sets the user agent on a Config
func WithUserAgent(userAgent string) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if userAgent == "" {
			return errors.New("user agent cannot be empty")
		}

		c.UserAgent = userAgent

		return nil
	}
}

// WithDebug enables or disables debug mode.
// Two-layer surface: this is the internal/test-layer Option that operates on
// [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithDebug] is what most callers
// should use; it composes with [github.com/LerianStudio/midaz-sdk-golang/v3.New]
// directly.
//
// In debug mode, the SDK logs detailed information about requests and responses.
//
// Parameters:
//   - enabled: Whether to enable debug mode
//
// Returns:
//   - Option: A function that sets the debug flag on a Config
func WithDebug(enabled bool) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		c.Debug = enabled

		return nil
	}
}

// WithErrorBodyExposure enables or disables raw upstream error response body
// exposure on SDK errors. When enabled, upstream 4xx/5xx response bodies are
// attached without redaction and only truncated.
func WithErrorBodyExposure(enabled bool) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		c.ExposeErrorBody = enabled

		return nil
	}
}

// WithObservabilityProvider sets the observability provider.
// Two-layer surface: this is the internal/test-layer Option that operates on
// [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithObservabilityProvider] is what most callers
// should use; it composes with [github.com/LerianStudio/midaz-sdk-golang/v3.New]
// directly.
//
// Parameters:
//   - provider: The observability provider to use
//
// Returns:
//   - Option: A function that sets the observability provider on a Config
func WithObservabilityProvider(provider observability.Provider) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if reflectutil.IsTypedNil(provider) {
			return errors.New("observability provider cannot be nil")
		}

		c.ObservabilityProvider = provider

		return nil
	}
}

// WithIdempotency enables or disables automatic idempotency key generation.
// Two-layer surface: this is the internal/test-layer Option that operates on
// [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithIdempotency] is what most
// callers should use; it composes with
// [github.com/LerianStudio/midaz-sdk-golang/v3.New] directly.
//
// Parameters:
//   - enabled: Whether to enable idempotency key generation
//
// Returns:
//   - Option: A function that sets the idempotency flag on a Config
func WithIdempotency(enabled bool) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		c.EnableIdempotency = enabled

		return nil
	}
}

// WithAccessManager sets the plugin-based authentication configuration.
// Two-layer surface: this is the internal/test-layer Option that operates on
// [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithAccessManager] is what most callers
// should use; it composes with [github.com/LerianStudio/midaz-sdk-golang/v3.New]
// directly.
//
// The Enabled field of the supplied AccessManager is OVERRIDDEN to true —
// the act of calling WithAccessManager is the user's signal that they want
// auth enabled. Callers MUST populate Address, ClientID, and ClientSecret;
// validation will reject an Enabled-but-Address-less config at New() time.
//
// To construct a client deliberately without an authentication source — for
// example, against an unsecured local stack — use [WithAnonymous] instead of
// passing a zero-value AccessManager.
//
// WithAccessManager preserves a previously-applied
// [WithAllowInsecureAccessManagerHTTP] opt-in. To disable that opt-in after
// setting Access Manager credentials, apply WithAllowInsecureAccessManagerHTTP(false)
// last.
//
// Parameters:
//   - accessManager: The plugin authentication configuration. Address,
//     ClientID, and ClientSecret are all required.
//
// Returns:
//   - Option: A function that sets the plugin authentication on a Config
func WithAccessManager(accessManager auth.AccessManager) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		// Auto-enable: callers don't need to set Enabled themselves. The
		// existence of the WithAccessManager call is the opt-in.
		allowInsecureHTTP := accessManager.AllowInsecureHTTP || c.AccessManager.AllowInsecureHTTP
		accessManager.Enabled = true
		accessManager.AllowInsecureHTTP = allowInsecureHTTP
		c.AccessManager = accessManager
		// Anonymous and AccessManager are mutually exclusive by definition;
		// the last-applied option wins. Clearing Anonymous here keeps the
		// final config consistent with the user's most recent intent.
		c.Anonymous = false

		return nil
	}
}

// WithAnonymous explicitly opts the client out of authentication. Use this
// for local development against an unsecured midaz stack, or for tests that
// don't exercise auth-protected endpoints.
//
// Without WithAnonymous AND without WithAccessManager, [Config.Validate]
// returns an error of the form
//
//	"no auth source configured; use WithAccessManager or WithAnonymous"
//
// This converts the v2 silent-localhost footgun (where a client without
// credentials happily issued unauthenticated requests and got 401s on the
// first real call) into an explicit construction-time choice.
//
// WithAnonymous and WithAccessManager are mutually exclusive — the last
// Two-layer surface: this is the internal/test-layer Option that operates on
// [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithAnonymous] is what most callers
// should use; it composes with [github.com/LerianStudio/midaz-sdk-golang/v3.New]
// directly.
//
// option applied wins. Calling WithAnonymous after WithAccessManager
// disables the previously-set Access Manager configuration.
//
// Returns:
//   - Option: A function that flags the Config as deliberately auth-less.
func WithAnonymous() Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		c.Anonymous = true
		// Disable any previously-applied AccessManager so validateConfig
		// correctly recognizes the Anonymous opt-out as the active path.
		// Other AccessManager fields (Address, ClientID, ClientSecret) are
		// preserved — env-driven loaders may have populated them and tests
		// often introspect the captured values; only Enabled controls the
		// auth-source semantics.
		c.AccessManager.Enabled = false

		return nil
	}
}

// WithAllowInsecureAccessManagerHTTP opts the client into accepting plain
// http:// Access Manager URLs even for non-loopback hosts. The default is
// strict (HTTPS or loopback only) because the credentials posted to the
// Access Manager token endpoint are the equivalent of long-lived passwords
// and must not cross a plaintext link.
//
// The flag exists for the canonical in-cluster Kubernetes pattern where the
// Access Manager is reached via a ClusterIP Service DNS name
// (e.g. http://plugin-access-manager-auth.midaz-plugins.svc.cluster.local:4000)
// and the transport security is provided by the service mesh or trusted
// network segment.
//
// SECURITY: this disables a deliberate transport-security gate. Production
// deployments must leave this off. Setting it true causes a Warn-level log
// line at client construction so the override is auditable.
//
// Two-layer surface: this is the internal/test-layer Option that operates
// on [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithAllowInsecureAccessManagerHTTP]
// is what most callers should use; it composes with
// [github.com/LerianStudio/midaz-sdk-golang/v3.New] directly.
//
// Parameters:
//   - allow: Whether to permit plain http:// for non-loopback hosts.
//
// Returns:
//   - Option: A function that wires the flag onto AccessManager.
func WithAllowInsecureAccessManagerHTTP(allow bool) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		c.AccessManager.AllowInsecureHTTP = allow

		return nil
	}
}

// WithAllowInsecureHTTP opts the configured Ledger and CRM service URLs
// out of the SDK's "http:// only for localhost" gate. DEFAULT IS FALSE
// (strict).
//
// The canonical use case is a Kubernetes cluster-internal service reached
// via cluster DNS (e.g. http://midaz-ledger.midaz-mt.svc.cluster.local:3000)
// where TLS is terminated by the service mesh, so the link from this SDK
// to the upstream is plaintext but inside a trusted network boundary.
// A secondary use case is dev/test deployments behind a controlled LAN
// where issuing a TLS certificate would be operationally heavy.
//
// SECURITY: this disables a deliberate transport-security gate. Production
// deployments over the public internet must leave this off. The flag
// lifts only the http-non-localhost guard — the scheme allowlist
// (http/https), userinfo rejection, and missing-host rejection remain
// active.
//
// ORDERING NOTE: this option mutates a flag read by [WithLedgerURL],
// [WithCRMURL], and [WithBaseURL] at the moment those options run.
// Apply WithAllowInsecureHTTP BEFORE the URL setters in your option
// chain:
//
//	cfg, err := config.NewConfig(
//	    config.WithAllowInsecureHTTP(true),
//	    config.WithLedgerURL("http://midaz-ledger.midaz-mt.svc.cluster.local:3000"),
//	    config.WithAccessManager(am),
//	)
//
// When the URLs come from [FromEnvironment], the helper loads
// MIDAZ_ALLOW_INSECURE_HTTP before processing MIDAZ_LEDGER_URL /
// MIDAZ_CRM_URL / MIDAZ_BASE_URL so the ordering is automatic.
//
// This flag is independent of [WithAllowInsecureAccessManagerHTTP], which
// gates the Access Manager (auth) endpoint. Set both when both the
// Access Manager and the Ledger live behind the cluster mesh.
//
// Two-layer surface: this is the internal/test-layer Option that operates
// on [Config]. The user-facing wrapper at
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithAllowInsecureHTTP]
// is what most callers should use; it composes with
// [github.com/LerianStudio/midaz-sdk-golang/v3.New] directly.
//
// Parameters:
//   - allow: Whether to permit plain http:// for non-loopback Ledger/CRM hosts.
//
// Returns:
//   - Option: A function that wires the flag onto a Config.
func WithAllowInsecureHTTP(allow bool) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		c.AllowInsecureHTTP = allow

		return nil
	}
}

// FromEnvironment loads configuration from environment variables.
// This allows for configuration without code changes.
//
// Environment variables:
//   - MIDAZ_ENVIRONMENT: The environment to use (local, development, production)
//   - PLUGIN_AUTH_ENABLED: Enable access manager authentication (parsed via [strconv.ParseBool])
//   - PLUGIN_AUTH_ADDRESS: The address of the access manager service
//   - MIDAZ_CLIENT_ID: The client ID for authentication
//   - MIDAZ_CLIENT_SECRET: The client secret for authentication
//   - MIDAZ_LEDGER_URL: The URL for the Ledger API (serves both onboarding and transaction endpoints)
//   - MIDAZ_CRM_URL: The URL for the CRM API
//   - MIDAZ_BASE_URL: The base URL for all services
//   - MIDAZ_TIMEOUT: The timeout in seconds for HTTP requests
//   - MIDAZ_DEBUG: Enable debug mode (parsed via [strconv.ParseBool])
//   - MIDAZ_MAX_RETRIES: Maximum number of retries
//   - MIDAZ_IDEMPOTENCY: Enable idempotency (parsed via [strconv.ParseBool])
//   - MIDAZ_ERROR_EXPOSE_BODY: Attach raw upstream 4xx/5xx response bodies
//     to SDK errors (parsed via [strconv.ParseBool]); default is false
//   - MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP: Permit plain http:// Access
//     Manager URLs for non-loopback hosts (parsed via [strconv.ParseBool]).
//     Production deployments must leave this unset or false; the flag
//     exists for the in-cluster Kubernetes Service pattern.
//   - MIDAZ_ALLOW_INSECURE_HTTP: Permit plain http:// Ledger/CRM service
//     URLs for non-loopback hosts (parsed via [strconv.ParseBool]). Loaded
//     before the URL env vars so MIDAZ_LEDGER_URL / MIDAZ_CRM_URL /
//     MIDAZ_BASE_URL pointing at cluster-internal services are accepted.
//     Production deployments over the public internet must leave this
//     unset or false; the flag exists for the in-cluster Kubernetes
//     Service pattern and for dev/test deployments behind a controlled
//     network boundary.
//
// Boolean variables accept the canonical [strconv.ParseBool] forms only:
// "1", "t", "T", "TRUE", "true", "True", "0", "f", "F", "FALSE", "false",
// and "False". Any other value (including "yes", "no", "on", "off") returns
// an error rather than silently defaulting — the previous behavior treated
// every non-"true" value as false, which silently flipped MIDAZ_IDEMPOTENCY=yes
// off when callers expected it on.
//
// Returns:
//   - Option: A function that sets configuration from environment variables
func FromEnvironment() Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if err := configureEnvironment(c); err != nil {
			return err
		}

		if err := configureAccessManager(c); err != nil {
			return err
		}

		// configureInsecureHTTP MUST run before configureURLs so the
		// in-cluster cluster.local Ledger/CRM URLs that drove the flag's
		// existence in the first place are accepted by parseURL.
		if err := configureInsecureHTTP(c); err != nil {
			return err
		}

		if err := configureURLs(c); err != nil {
			return err
		}

		if err := configureTimeoutAndRetries(c); err != nil {
			return err
		}

		return configureOptionalSettings(c)
	}
}

// configureEnvironment sets the environment from environment variables
func configureEnvironment(c *Config) error {
	env := os.Getenv("MIDAZ_ENVIRONMENT")
	if env == "" {
		return nil
	}

	switch Environment(env) {
	case EnvironmentLocal:
		return WithEnvironment(EnvironmentLocal)(c)
	case EnvironmentDevelopment:
		return WithEnvironment(EnvironmentDevelopment)(c)
	case EnvironmentProduction:
		return WithEnvironment(EnvironmentProduction)(c)
	default:
		return fmt.Errorf("invalid environment: %s", env)
	}
}

// configureAccessManager sets up access manager configuration from environment.
//
// Programmatic AccessManager fields populated before FromEnvironment runs are
// preserved when the corresponding env var is empty. The previous behavior
// unconditionally overwrote Address/ClientID/ClientSecret with os.Getenv's
// empty-string return whenever PLUGIN_AUTH_ENABLED was set, which silently
// wiped credentials configured via WithAccessManager when callers chained
// FromEnvironment afterwards (e.g. NewLocalConfig).
func configureAccessManager(c *Config) error {
	enable := os.Getenv("PLUGIN_AUTH_ENABLED")
	var enabled bool
	if enable != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(enable))
		if err != nil {
			return fmt.Errorf("invalid PLUGIN_AUTH_ENABLED value %q: %w", enable, err)
		}

		enabled = parsed
	}

	if insecure := os.Getenv("MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP"); insecure != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(insecure))
		if err != nil {
			return fmt.Errorf("invalid MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP value %q: %w", insecure, err)
		}

		c.AccessManager.AllowInsecureHTTP = parsed
	}

	if enable == "" {
		return nil
	}

	if address := os.Getenv("PLUGIN_AUTH_ADDRESS"); address != "" {
		c.AccessManager.Address = address
	}

	if clientID := os.Getenv("MIDAZ_CLIENT_ID"); clientID != "" {
		c.AccessManager.ClientID = clientID
	}

	if clientSecret := os.Getenv("MIDAZ_CLIENT_SECRET"); clientSecret != "" {
		c.AccessManager.ClientSecret = clientSecret
	}

	c.AccessManager.Enabled = enabled
	if enabled {
		c.Anonymous = false
	}

	return nil
}

// configureInsecureHTTP loads MIDAZ_ALLOW_INSECURE_HTTP and applies it to
// the Config before any URL setter runs. Unset env var leaves the
// programmatically-configured value (typically false) untouched.
func configureInsecureHTTP(c *Config) error {
	raw := os.Getenv("MIDAZ_ALLOW_INSECURE_HTTP")
	if raw == "" {
		return nil
	}

	parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid MIDAZ_ALLOW_INSECURE_HTTP value %q: %w", raw, err)
	}

	c.AllowInsecureHTTP = parsed

	return nil
}

// configureURLs sets up URL configuration from environment variables
func configureURLs(c *Config) error {
	// URLs take precedence in this order: specific URL > base URL > environment default
	if baseURL := os.Getenv("MIDAZ_BASE_URL"); baseURL != "" {
		if err := WithBaseURL(baseURL)(c); err != nil {
			return err
		}
	}

	return configureSpecificURLs(c)
}

// configureSpecificURLs sets specific service URLs that override base URL
func configureSpecificURLs(c *Config) error {
	if ledgerURL := os.Getenv("MIDAZ_LEDGER_URL"); ledgerURL != "" {
		if err := WithLedgerURL(ledgerURL)(c); err != nil {
			return err
		}
	}

	if crmURL := os.Getenv("MIDAZ_CRM_URL"); crmURL != "" {
		if err := WithCRMURL(crmURL)(c); err != nil {
			return err
		}
	}

	return nil
}

// configureTimeoutAndRetries sets timeout and retry configuration from environment
func configureTimeoutAndRetries(c *Config) error {
	if err := configureTimeout(c); err != nil {
		return err
	}

	return configureRetries(c)
}

// configureTimeout sets timeout from environment variable
func configureTimeout(c *Config) error {
	timeout := os.Getenv("MIDAZ_TIMEOUT")
	if timeout == "" {
		return nil
	}

	seconds, err := parseEnvInt(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	return WithTimeout(time.Duration(seconds) * time.Second)(c)
}

// configureRetries sets max retries from environment variable
func configureRetries(c *Config) error {
	retries := os.Getenv("MIDAZ_MAX_RETRIES")
	if retries == "" {
		return nil
	}

	maxRetries, err := parseEnvInt(retries)
	if err != nil {
		return fmt.Errorf("invalid max retries: %w", err)
	}

	return WithMaxRetries(maxRetries)(c)
}

// configureOptionalSettings sets optional boolean settings from environment.
//
// Boolean env vars are parsed strictly via [strconv.ParseBool] so callers get
// a clear error for typos like MIDAZ_IDEMPOTENCY=yes. The previous behavior
// silently treated every non-"true" value as false, which is the worst kind
// of silent default — the user reads the doc, types a reasonable value, and
// the SDK quietly does the opposite.
func configureOptionalSettings(c *Config) error {
	if debug := os.Getenv("MIDAZ_DEBUG"); debug != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(debug))
		if err != nil {
			return fmt.Errorf("invalid MIDAZ_DEBUG value %q: %w", debug, err)
		}

		c.Debug = parsed
	}

	if idempotency := os.Getenv("MIDAZ_IDEMPOTENCY"); idempotency != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(idempotency))
		if err != nil {
			return fmt.Errorf("invalid MIDAZ_IDEMPOTENCY value %q: %w", idempotency, err)
		}

		c.EnableIdempotency = parsed
	}

	if exposeBody := os.Getenv("MIDAZ_ERROR_EXPOSE_BODY"); exposeBody != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(exposeBody))
		if err != nil {
			return fmt.Errorf("invalid MIDAZ_ERROR_EXPOSE_BODY value %q: %w", exposeBody, err)
		}

		c.ExposeErrorBody = parsed
	}

	if skip := os.Getenv("MIDAZ_SKIP_AUTH_CHECK"); skip != "" {
		return errors.New("MIDAZ_SKIP_AUTH_CHECK is not supported by FromEnvironment")
	}

	return nil
}

// parseEnvInt parses an integer from an environment variable.
func parseEnvInt(value string) (int, error) {
	result, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}

	return result, nil
}

// NewConfig creates a new Config with default values.
// The resulting Config will have sensible defaults for all settings.
// Use options to customize any aspect of the configuration.
//
// Parameters:
//   - options: Zero or more Option functions to customize the Config
//
// Returns:
//   - *Config: A new configuration with the provided options applied
//   - error: An error if any option validation fails
func NewConfig(options ...Option) (*Config, error) {
	// Create a config with default values
	config := &Config{
		AccessManager:     auth.AccessManager{},
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

	// Apply default URLs based on environment
	if err := setDefaultServiceURLs(config); err != nil {
		return nil, err
	}

	// Apply provided options
	for _, option := range options {
		if option == nil {
			return nil, errors.New("option cannot be nil")
		}

		if err := option(config); err != nil {
			return nil, err
		}
	}

	// Create HTTP client if not provided
	if config.HTTPClient == nil {
		config.HTTPClient = NewDefaultHTTPClient(config.Timeout)
		config.httpClientOwned = true
	}

	// Validate required fields
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// setDefaultServiceURLs sets default URLs based on the environment.
func setDefaultServiceURLs(config *Config) error {
	ensureServiceURLMap(config)

	serviceURLs, err := defaultServiceURLsForEnvironment(config.Environment)
	if err != nil {
		return err
	}

	applyDefaultServiceURLs(config, serviceURLs)

	return nil
}

func ensureServiceURLMap(config *Config) {
	if config.ServiceURLs == nil {
		config.ServiceURLs = make(map[ServiceType]string)
	}
}

type defaultServiceURLs struct {
	ledgerURL string
	crmURL    string
}

func defaultServiceURLsForEnvironment(environment Environment) (defaultServiceURLs, error) {
	switch environment {
	case EnvironmentLocal:
		return defaultLocalServiceURLs()
	case EnvironmentDevelopment:
		return defaultLedgerBackedServiceURLs(DefaultDevelopmentLedgerBaseURL)
	case EnvironmentProduction:
		return defaultLedgerBackedServiceURLs(DefaultProductionLedgerBaseURL)
	default:
		return defaultServiceURLs{}, fmt.Errorf("unknown environment: %s", environment)
	}
}

func defaultLocalServiceURLs() (defaultServiceURLs, error) {
	ledgerURL, err := buildLedgerServiceURL(DefaultLocalLedgerBaseURL)
	if err != nil {
		return defaultServiceURLs{}, err
	}

	crmURL, err := buildCRMServiceURL(DefaultLocalCRMBaseURL)
	if err != nil {
		return defaultServiceURLs{}, err
	}

	return defaultServiceURLs{ledgerURL: ledgerURL, crmURL: crmURL}, nil
}

func defaultLedgerBackedServiceURLs(baseURL string) (defaultServiceURLs, error) {
	ledgerURL, err := buildLedgerServiceURL(baseURL)
	if err != nil {
		return defaultServiceURLs{}, err
	}

	return defaultServiceURLs{ledgerURL: ledgerURL, crmURL: ledgerURL}, nil
}

func applyDefaultServiceURLs(config *Config, serviceURLs defaultServiceURLs) {
	if !config.ledgerURLSet {
		config.ServiceURLs[ServiceOnboarding] = serviceURLs.ledgerURL
		config.ServiceURLs[ServiceTransaction] = serviceURLs.ledgerURL
	}

	if !config.crmURLSet {
		config.ServiceURLs[ServiceCRM] = serviceURLs.crmURL
	}
}

// Validate reports whether the Config has all required fields populated and
// is internally consistent.
//
// midaz.New() calls Validate() automatically after applying all options. Most
// callers will never invoke Validate() directly; it is exported so advanced
// users (e.g. config-loading helpers, test fixtures) can re-validate after
// mutating a Config they constructed via DefaultConfig().
//
// Returns nil on success or an error describing the first problem encountered.
// Use [Config.ValidateAll] for an accumulated multi-problem view (planned for
// v3 Track 8).
//
// Validation rules:
//   - ServiceURLs[ServiceOnboarding] and ServiceURLs[ServiceTransaction] must
//     both be set — these are populated from LedgerURL (the single user-facing
//     knob) and used internally to route onboarding vs transaction endpoints.
//   - Exactly one auth source must be configured: either WithAccessManager
//     (enables AccessManager and requires Address) or WithAnonymous (explicit
//     auth-less mode). Construction without either fails.
//   - If AccessManager.Enabled, AccessManager.Address must be set.
//   - If AccessManager.Enabled, callers must explicitly select an environment
//     or service URL target instead of relying on the SDK's default local URLs.
//   - AccessManager.Address must use https:// (or a loopback host with
//     http://). [WithAllowInsecureAccessManagerHTTP] opts into accepting
//     plain http:// for non-loopback hosts (in-cluster k8s service DNS).
func (c *Config) Validate() error {
	return validateConfig(c)
}

// validateConfig ensures that the Config has all required fields by
// dispatching to topical helpers. Each helper owns one concern (service
// URLs, retry knobs, auth sources) so this entry point stays a thin
// sequential gate.
func validateConfig(config *Config) error {
	if err := validateServiceURLs(config); err != nil {
		return err
	}

	if err := validateRetrySettings(config); err != nil {
		return err
	}

	return validateAuthSettings(config)
}

// validateServiceURLs enforces that the Ledger service URL is configured.
// The onboarding and transaction internal routes both resolve to LedgerURL,
// so both map entries must be populated for the entity layer to function.
// It also refuses the AllowInsecureHTTP opt-in in the production
// environment, mirroring the Access Manager equivalent — the flag is for
// in-cluster or controlled-network deployments, never the public internet.
func validateServiceURLs(config *Config) error {
	if onboardingURL, ok := config.ServiceURLs[ServiceOnboarding]; !ok || strings.TrimSpace(onboardingURL) == "" {
		return errors.New("ledger URL is required")
	}

	if transactionURL, ok := config.ServiceURLs[ServiceTransaction]; !ok || strings.TrimSpace(transactionURL) == "" {
		return errors.New("ledger URL is required")
	}

	if config.Environment == EnvironmentProduction && config.AllowInsecureHTTP {
		return errors.New("insecure HTTP is not allowed in production")
	}

	return nil
}

// validateRetrySettings enforces the min ≤ max invariant on the retry
// wait pair. WithRetryWaitMin/WithRetryWaitMax already catch inversions at
// option-application time, but a caller that mutates the fields directly
// on a Config they own (e.g. via DefaultConfig) can still land in min > max.
// Refuse construction here rather than producing a config the retry layer
// would have to handle defensively.
func validateRetrySettings(config *Config) error {
	if config.RetryWaitMin > 0 && config.RetryWaitMax > 0 && config.RetryWaitMin > config.RetryWaitMax {
		return errors.New("minimum wait time must be less than or equal to maximum wait time")
	}

	return nil
}

// validateAuthSettings enforces the auth-source contract: exactly one of
// AccessManager or Anonymous must be configured, and when AccessManager is
// the chosen source its required fields (address, client id, client secret)
// must all be present.
//
// The package-internal skipAuthCheck escape hatch bypasses the gate for tests.
// This closes v2's silent-localhost footgun where construction succeeded
// with no credentials and every real request returned 401.
func validateAuthSettings(config *Config) error {
	if !config.AccessManager.Enabled && !config.Anonymous && !config.skipAuthCheck {
		return errors.New("no auth source configured; use WithAccessManager or WithAnonymous")
	}

	if config.AccessManager.Enabled && config.Anonymous && !config.skipAuthCheck {
		return errors.New("exactly one auth source must be configured: Access Manager or Anonymous")
	}

	if !config.AccessManager.Enabled || config.skipAuthCheck {
		return nil
	}

	if strings.TrimSpace(config.AccessManager.Address) == "" {
		return errors.New("plugin auth address is required")
	}
	// Strict scheme enforcement: plain http:// is rejected for non-loopback
	// hosts unless [WithAllowInsecureAccessManagerHTTP] opted in. The flag
	// exists for in-cluster Kubernetes service DNS; production deployments
	// must keep it off.
	if config.Environment == EnvironmentProduction && config.AccessManager.AllowInsecureHTTP {
		return errors.New("plugin auth insecure HTTP is not allowed in production")
	}

	if err := auth.ValidateAccessManagerAddressWithInsecure(
		config.AccessManager.Address,
		config.AccessManager.AllowInsecureHTTP,
	); err != nil {
		return fmt.Errorf("invalid plugin auth address: %w", err)
	}

	if strings.TrimSpace(config.AccessManager.ClientID) == "" {
		return errors.New("plugin auth client id is required")
	}

	return validateAccessManagerClientSettings(config)
}

func validateAccessManagerClientSettings(config *Config) error {
	if strings.TrimSpace(config.AccessManager.ClientSecret) == "" {
		return errors.New("plugin auth client secret is required")
	}

	if !config.hasExplicitTarget() {
		return errors.New("explicit environment or service URL is required when Access Manager is enabled")
	}

	return nil
}

func (c *Config) hasExplicitTarget() bool {
	if c == nil {
		return false
	}

	return c.environmentSet || c.baseURLSet || c.ledgerURLSet || c.crmURLSet
}

// GetBaseURLs converts ServiceURLs to the map format expected by the entity layer.
func (c *Config) GetBaseURLs() map[string]string {
	result := make(map[string]string)
	for service, serviceURL := range c.ServiceURLs {
		result[string(service)] = serviceURL
	}

	return result
}

// GetHTTPClient returns the HTTP client to use for requests.
func (c *Config) GetHTTPClient() *http.Client {
	return c.HTTPClient
}

// GetPluginAuth returns the plugin authentication configuration.
func (c *Config) GetPluginAuth() auth.AccessManager {
	// Return a copy of the plugin auth configuration
	return auth.AccessManager{
		Address:           c.AccessManager.Address,
		ClientID:          c.AccessManager.ClientID,
		ClientSecret:      c.AccessManager.ClientSecret,
		Enabled:           c.AccessManager.Enabled,
		AllowInsecureHTTP: c.AccessManager.AllowInsecureHTTP,
	}
}

// GetObservabilityProvider returns the observability provider.
func (c *Config) GetObservabilityProvider() observability.Provider {
	return c.ObservabilityProvider
}

// GetAllowInsecureHTTP returns the data-plane (Ledger / CRM) insecure HTTP
// opt-in flag. The entities layer reads this to gate the runtime
// [security.ValidateOutboundRequest] check the same way the config-time
// [parseURL] gate is relaxed.
func (c *Config) GetAllowInsecureHTTP() bool {
	if c == nil {
		return false
	}

	return c.AllowInsecureHTTP
}

// Clone returns an independent copy of the configuration.
//
// The clone is safe to mutate without affecting the receiver. In particular,
// when c.HTTPClient is non-nil and the SDK owns it, the clone receives a
// freshly allocated http.Client value so that scalar mutations via WithTimeout
// on the clone (which only writes when httpClientOwned == true) do not race
// with the original config's client. The underlying Transport is intentionally
// shared because http.Transport is goroutine-safe by design and is expensive
// to rebuild; consumers needing an isolated transport should rebuild it
// explicitly with WithHTTPClient.
//
// For caller-supplied (non-owned) clients we keep the original pointer so the
// surrounding "do not mutate caller's client" contract is preserved.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	cloned := *c
	if c.ServiceURLs != nil {
		cloned.ServiceURLs = make(map[ServiceType]string, len(c.ServiceURLs))
		for service, serviceURL := range c.ServiceURLs {
			cloned.ServiceURLs[service] = serviceURL
		}
	}

	if c.HTTPClient != nil && c.httpClientOwned {
		clientCopy := *c.HTTPClient
		cloned.HTTPClient = &clientCopy
		cloned.httpClientOwned = true
	}

	return &cloned
}

// parseURL validates that a URL is properly formatted using the SDK's
// default strict mode (HTTP permitted only for localhost targets). The
// scheme and userinfo checks are always enforced. To allow plain HTTP for
// a non-localhost target (typically an in-cluster Kubernetes Service),
// use [parseURLWithInsecureHTTP] with allowInsecureHTTP=true.
func parseURL(rawURL string) error {
	return parseURLWithInsecureHTTP(rawURL, false)
}

// parseURLWithInsecureHTTP validates that a URL is properly formatted.
//
// Scheme allowlist (http/https), missing-scheme/host rejection, and the
// userinfo block are always enforced. The "http:// only for localhost"
// gate is the single rule honored by allowInsecureHTTP=true.
func parseURLWithInsecureHTTP(rawURL string, allowInsecureHTTP bool) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	// Require scheme and host
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errors.New("URL must include scheme and host")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("URL scheme must be http or https")
	}

	if parsedURL.User != nil {
		return errors.New("URL must not include user information")
	}

	if parsedURL.Scheme == "http" && !allowInsecureHTTP && !isLocalhost(parsedURL.Host) {
		return fmt.Errorf("insecure HTTP is only allowed for localhost targets: %s", parsedURL.Host)
	}

	return nil
}

// NewDefaultHTTPClient returns an SDK-owned HTTP client with a conservative
// pooled transport.
//
// The transport pins TLS to a minimum of TLS 1.2 ([tls.VersionTLS12]). Go's
// runtime default already lands here (Go 1.18+ enforces TLS 1.2 client-side),
// but pinning it explicitly insulates the SDK from any future runtime change
// that would lower the floor — and makes the floor visible to anyone reading
// the code instead of buried in the standard library defaults.
func NewDefaultHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: security.ValidateRedirect,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			MaxConnsPerHost:       100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}
}

func buildLedgerServiceURL(baseURL string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSuffix(baseURL, "/"))
	if err != nil {
		return "", err
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", errors.New("URL must include scheme and host")
	}

	if isLocalhost(parsedURL.Host) && parsedURL.Port() == "" {
		parsedURL.Host = withPort(parsedURL.Hostname(), "3002")
	}

	return ensureAPIVersionPath(parsedURL), nil
}

func buildCRMServiceURL(baseURL string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSuffix(baseURL, "/"))
	if err != nil {
		return "", err
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", errors.New("URL must include scheme and host")
	}

	if isLocalhost(parsedURL.Host) && parsedURL.Port() == "" {
		parsedURL.Host = withPort(parsedURL.Hostname(), "4003")
	}

	return ensureAPIVersionPath(parsedURL), nil
}

func ensureAPIVersionPath(parsedURL *url.URL) string {
	cleanPath := strings.TrimSuffix(parsedURL.Path, "/")
	if cleanPath == "" {
		parsedURL.Path = DefaultLedgerAPIVersionPath
		return parsedURL.String()
	}

	if cleanPath == DefaultLedgerAPIVersionPath {
		parsedURL.Path = cleanPath
		return parsedURL.String()
	}

	parsedURL.Path = cleanPath + DefaultLedgerAPIVersionPath

	return parsedURL.String()
}

func withPort(hostname, port string) string {
	if strings.Contains(hostname, ":") {
		return "[" + hostname + "]:" + port
	}

	return hostname + ":" + port
}

// isLocalhost checks if the host is a localhost address (for development use).
func isLocalhost(host string) bool {
	hostname := host
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		hostname = splitHost
	}

	hostname = strings.Trim(hostname, "[]")
	if hostname == "localhost" {
		return true
	}
	// RFC 6761 §6.3: ".localhost" is a reserved special-use TLD that resolvers
	// must treat as loopback. Used by Docker Compose aliases and dev tooling.
	if strings.HasSuffix(hostname, ".localhost") {
		return true
	}

	ip := net.ParseIP(hostname)

	return ip != nil && ip.IsLoopback()
}

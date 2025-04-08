package performance

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

// Default values for connection pooling.
const (
	// DefaultMaxIdleConns is the default maximum number of idle connections.
	DefaultMaxIdleConns = 100

	// DefaultMaxIdleConnsPerHost is the default maximum number of idle connections per host.
	DefaultMaxIdleConnsPerHost = 10

	// DefaultMaxConnsPerHost is the default maximum number of connections per host.
	DefaultMaxConnsPerHost = 100

	// DefaultIdleConnTimeout is the default maximum amount of time an idle connection will remain idle.
	DefaultIdleConnTimeout = 90 * time.Second

	// DefaultTLSHandshakeTimeout is the default maximum amount of time to wait for a TLS handshake.
	DefaultTLSHandshakeTimeout = 10 * time.Second

	// DefaultResponseHeaderTimeout is the default amount of time to wait for a server's response headers.
	DefaultResponseHeaderTimeout = 30 * time.Second

	// DefaultExpectContinueTimeout is the default amount of time to wait for a 100 Continue response.
	DefaultExpectContinueTimeout = 1 * time.Second

	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 60 * time.Second

	// DefaultKeepAlive is the default keep-alive period.
	DefaultKeepAlive = 30 * time.Second
)

// TransportConfig contains configuration options for HTTP transport.
type TransportConfig struct {
	// MaxIdleConns controls the maximum number of idle (keep-alive) connections across all hosts.
	// Zero means no limit.
	MaxIdleConns int

	// MaxIdleConnsPerHost controls the maximum number of idle (keep-alive) connections to keep per-host.
	// If zero, DefaultMaxIdleConnsPerHost is used.
	MaxIdleConnsPerHost int

	// MaxConnsPerHost limits the total number of connections per host.
	// Zero means no limit.
	MaxConnsPerHost int

	// IdleConnTimeout is the maximum amount of time an idle (keep-alive) connection will remain
	// idle before closing itself.
	// Zero means no limit.
	IdleConnTimeout time.Duration

	// TLSHandshakeTimeout specifies the maximum amount of time waiting to
	// wait for a TLS handshake. Zero means no timeout.
	TLSHandshakeTimeout time.Duration

	// ResponseHeaderTimeout, if non-zero, specifies the amount of time to wait
	// for a server's response headers after fully writing the request
	// (including its body, if any). This time does not include the time to
	// read the response body.
	ResponseHeaderTimeout time.Duration

	// ExpectContinueTimeout, if non-zero, specifies the amount of time to
	// wait for a server's first response headers after fully writing the
	// request headers if the request has an "Expect: 100-continue" header.
	// Zero means no timeout.
	// This is only used for Transport and not for Client timeouts.
	ExpectContinueTimeout time.Duration

	// DisableKeepAlives, if true, disables HTTP keep-alives and will only
	// use the connection to the server for a single HTTP request.
	DisableKeepAlives bool

	// DisableCompression, if true, prevents the Transport from requesting
	// compression with an "Accept-Encoding: gzip" request header.
	DisableCompression bool

	// DialTimeout is the maximum amount of time a dial will wait for a
	// connect to complete.
	DialTimeout time.Duration

	// KeepAlive specifies the keep-alive period for an active network
	// connection. If zero, keep-alives are enabled if supported
	// by the protocol and operating system.
	KeepAlive time.Duration
}

// TransportOption defines a function that configures a TransportConfig
type TransportOption func(*TransportConfig) error

// WithMaxIdleConns sets the maximum number of idle connections across all hosts
func WithMaxIdleConns(n int) TransportOption {
	return func(c *TransportConfig) error {
		if n < 0 {
			return fmt.Errorf("max idle connections must be non-negative, got %d", n)
		}
		c.MaxIdleConns = n
		return nil
	}
}

// WithTransportMaxIdleConnsPerHost sets the maximum number of idle connections to keep per-host
func WithTransportMaxIdleConnsPerHost(n int) TransportOption {
	return func(c *TransportConfig) error {
		if n < 0 {
			return fmt.Errorf("max idle connections per host must be non-negative, got %d", n)
		}
		c.MaxIdleConnsPerHost = n
		return nil
	}
}

// WithMaxConnsPerHost sets the maximum number of connections per host
func WithMaxConnsPerHost(n int) TransportOption {
	return func(c *TransportConfig) error {
		if n < 0 {
			return fmt.Errorf("max connections per host must be non-negative, got %d", n)
		}
		c.MaxConnsPerHost = n
		return nil
	}
}

// WithIdleConnTimeout sets the maximum amount of time an idle connection will remain idle
func WithIdleConnTimeout(d time.Duration) TransportOption {
	return func(c *TransportConfig) error {
		if d < 0 {
			return fmt.Errorf("idle connection timeout must be non-negative, got %v", d)
		}
		c.IdleConnTimeout = d
		return nil
	}
}

// WithTLSHandshakeTimeout sets the maximum amount of time to wait for a TLS handshake
func WithTLSHandshakeTimeout(d time.Duration) TransportOption {
	return func(c *TransportConfig) error {
		if d < 0 {
			return fmt.Errorf("TLS handshake timeout must be non-negative, got %v", d)
		}
		c.TLSHandshakeTimeout = d
		return nil
	}
}

// WithResponseHeaderTimeout sets the amount of time to wait for response headers
func WithResponseHeaderTimeout(d time.Duration) TransportOption {
	return func(c *TransportConfig) error {
		if d < 0 {
			return fmt.Errorf("response header timeout must be non-negative, got %v", d)
		}
		c.ResponseHeaderTimeout = d
		return nil
	}
}

// WithExpectContinueTimeout sets the amount of time to wait for a 100 Continue response
func WithExpectContinueTimeout(d time.Duration) TransportOption {
	return func(c *TransportConfig) error {
		if d < 0 {
			return fmt.Errorf("expect continue timeout must be non-negative, got %v", d)
		}
		c.ExpectContinueTimeout = d
		return nil
	}
}

// WithDisableKeepAlives sets whether to disable HTTP keep-alives
func WithDisableKeepAlives(disable bool) TransportOption {
	return func(c *TransportConfig) error {
		c.DisableKeepAlives = disable
		return nil
	}
}

// WithDisableCompression sets whether to disable compression
func WithDisableCompression(disable bool) TransportOption {
	return func(c *TransportConfig) error {
		c.DisableCompression = disable
		return nil
	}
}

// WithDialTimeout sets the maximum amount of time a dial will wait for a connect to complete
func WithDialTimeout(d time.Duration) TransportOption {
	return func(c *TransportConfig) error {
		if d < 0 {
			return fmt.Errorf("dial timeout must be non-negative, got %v", d)
		}
		c.DialTimeout = d
		return nil
	}
}

// WithKeepAlive sets the keep-alive period for an active network connection
func WithKeepAlive(d time.Duration) TransportOption {
	return func(c *TransportConfig) error {
		if d < 0 {
			return fmt.Errorf("keep-alive period must be non-negative, got %v", d)
		}
		c.KeepAlive = d
		return nil
	}
}

// WithHighThroughput configures the transport for high throughput operations
func WithHighThroughput() TransportOption {
	return func(c *TransportConfig) error {
		c.MaxIdleConns = 200
		c.MaxIdleConnsPerHost = 50
		c.MaxConnsPerHost = 200
		c.IdleConnTimeout = 180 * time.Second
		c.KeepAlive = 60 * time.Second
		return nil
	}
}

// WithLowLatency configures the transport for low latency operations
func WithLowLatency() TransportOption {
	return func(c *TransportConfig) error {
		c.TLSHandshakeTimeout = 5 * time.Second
		c.ResponseHeaderTimeout = 15 * time.Second
		c.ExpectContinueTimeout = 500 * time.Millisecond
		c.DialTimeout = 30 * time.Second
		return nil
	}
}

// DefaultTransportConfig returns a TransportConfig with default values.
func DefaultTransportConfig() *TransportConfig {
	return &TransportConfig{
		MaxIdleConns:          DefaultMaxIdleConns,
		MaxIdleConnsPerHost:   DefaultMaxIdleConnsPerHost,
		MaxConnsPerHost:       DefaultMaxConnsPerHost,
		IdleConnTimeout:       DefaultIdleConnTimeout,
		TLSHandshakeTimeout:   DefaultTLSHandshakeTimeout,
		ResponseHeaderTimeout: DefaultResponseHeaderTimeout,
		ExpectContinueTimeout: DefaultExpectContinueTimeout,
		DisableKeepAlives:     false,
		DisableCompression:    false,
		DialTimeout:           DefaultTimeout,
		KeepAlive:             DefaultKeepAlive,
	}
}

// NewTransportConfig creates a new TransportConfig with the given options.
func NewTransportConfig(opts ...TransportOption) (*TransportConfig, error) {
	// Start with default config
	config := DefaultTransportConfig()

	// Apply all provided options
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, fmt.Errorf("failed to apply transport option: %w", err)
		}
	}

	return config, nil
}

// NewTransport creates a new http.Transport with the given options.
func NewTransport(opts ...TransportOption) (*http.Transport, error) {
	config, err := NewTransportConfig(opts...)
	if err != nil {
		return nil, err
	}

	return newTransportWithConfig(config), nil
}

// newTransportWithConfig creates a new http.Transport with the given config.
func newTransportWithConfig(config *TransportConfig) *http.Transport {
	if config == nil {
		config = DefaultTransportConfig()
	}

	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		DisableKeepAlives:     config.DisableKeepAlives,
		DisableCompression:    config.DisableCompression,
		ForceAttemptHTTP2:     true,
	}
}

// HTTPClientOption defines a function that configures an http.Client
type HTTPClientOption func(*http.Client) error

// WithTimeout sets the timeout for the HTTP client
func WithTimeout(d time.Duration) HTTPClientOption {
	return func(c *http.Client) error {
		if d < 0 {
			return fmt.Errorf("timeout must be non-negative, got %v", d)
		}
		c.Timeout = d
		return nil
	}
}

// WithTransport sets the transport for the HTTP client
func WithTransport(transport http.RoundTripper) HTTPClientOption {
	return func(c *http.Client) error {
		c.Transport = transport
		return nil
	}
}

// NewClient creates a new http.Client with optimal settings for high-performance API usage.
func NewClient(opts ...HTTPClientOption) (*http.Client, error) {
	// Create transport with default config
	transport, err := NewTransport()
	if err != nil {
		return nil, err
	}

	// Create client with default settings
	client := &http.Client{
		Transport: transport,
		Timeout:   DefaultTimeout,
	}

	// Apply all provided options
	for _, opt := range opts {
		if err := opt(client); err != nil {
			return nil, fmt.Errorf("failed to apply HTTP client option: %w", err)
		}
	}

	return client, nil
}

// OptimizeHTTPClient modifies an existing http.Client with optimal connection pooling settings.
func OptimizeHTTPClient(client *http.Client, opts ...TransportOption) (*http.Client, error) {
	if client == nil {
		return NewClient()
	}

	// Create config with provided options
	config, err := NewTransportConfig(opts...)
	if err != nil {
		return nil, err
	}

	// If the client has a custom transport, optimize it
	if transport, ok := client.Transport.(*http.Transport); ok {
		// Apply our optimized settings while preserving any custom settings
		// Only set values that are not already set
		if transport.MaxIdleConns == 0 {
			transport.MaxIdleConns = config.MaxIdleConns
		}
		if transport.MaxIdleConnsPerHost == 0 {
			transport.MaxIdleConnsPerHost = config.MaxIdleConnsPerHost
		}
		if transport.MaxConnsPerHost == 0 {
			transport.MaxConnsPerHost = config.MaxConnsPerHost
		}
		if transport.IdleConnTimeout == 0 {
			transport.IdleConnTimeout = config.IdleConnTimeout
		}
		if transport.TLSHandshakeTimeout == 0 {
			transport.TLSHandshakeTimeout = config.TLSHandshakeTimeout
		}
		if transport.ResponseHeaderTimeout == 0 {
			transport.ResponseHeaderTimeout = config.ResponseHeaderTimeout
		}
		if transport.ExpectContinueTimeout == 0 {
			transport.ExpectContinueTimeout = config.ExpectContinueTimeout
		}
		if transport.ForceAttemptHTTP2 == false {
			transport.ForceAttemptHTTP2 = true
		}
	} else if client.Transport == nil {
		// If client has no transport, create one
		client.Transport = newTransportWithConfig(config)
	}

	// Set timeout if not set
	if client.Timeout == 0 {
		client.Timeout = DefaultTimeout
	}

	return client, nil
}

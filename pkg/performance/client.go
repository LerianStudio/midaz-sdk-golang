package performance

import (
	"net"
	"net/http"
	"time"
)

// HTTPClientOptions holds optional configuration for the HTTP client
type HTTPClientOptions struct {
	// MaxIdleConnsPerHost is the maximum number of idle connections to keep per host
	MaxIdleConnsPerHost int

	// IdleConnTimeout is the maximum amount of time an idle connection will be kept in the pool
	IdleConnTimeout time.Duration

	// TLSHandshakeTimeout is the maximum amount of time waiting for a TLS handshake
	TLSHandshakeTimeout time.Duration

	// DisableCompression disables compression of request bodies
	DisableCompression bool

	// DisableKeepAlives disables HTTP keep-alives and will only use the connection for a single HTTP request
	DisableKeepAlives bool
}

// DefaultHTTPClientOptions returns default options for HTTP client optimization
func DefaultHTTPClientOptions() HTTPClientOptions {
	return HTTPClientOptions{
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   false,
	}
}

// OptimizeClient configures the provided http.Client for optimal performance.
// If the client is nil, a new client is created.
// If options is nil, default options are used.
//
// Deprecated: use OptimizeHTTPClient for new code. OptimizeClient delegates to
// OptimizeHTTPClient while preserving the legacy HTTPClientOptions behavior.
func OptimizeClient(client *http.Client, options *HTTPClientOptions) *http.Client {
	// Use default options if none provided
	opts := DefaultHTTPClientOptions()
	if options != nil {
		opts = *options
	}

	optimized, err := OptimizeHTTPClient(
		client,
		WithTransportMaxIdleConnsPerHost(opts.MaxIdleConnsPerHost),
		WithIdleConnTimeout(opts.IdleConnTimeout),
		WithTLSHandshakeTimeout(opts.TLSHandshakeTimeout),
		WithDisableCompression(opts.DisableCompression),
		WithDisableKeepAlives(opts.DisableKeepAlives),
	)
	if err != nil {
		return client
	}

	if transport, ok := optimized.Transport.(*http.Transport); ok {
		transport.MaxIdleConnsPerHost = opts.MaxIdleConnsPerHost
		transport.DisableCompression = opts.DisableCompression
		transport.DisableKeepAlives = opts.DisableKeepAlives

		if transport.DialContext == nil {
			dialer := &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			transport.DialContext = dialer.DialContext
		}
	}

	return optimized
}

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
func OptimizeClient(client *http.Client, options *HTTPClientOptions) *http.Client {
	// Use default options if none provided
	opts := DefaultHTTPClientOptions()
	if options != nil {
		opts = *options
	}

	// Create a new client if none provided
	if client == nil {
		client = &http.Client{}
	}

	// Create a transport if client doesn't have one
	transport := client.Transport
	if transport == nil {
		transport = &http.Transport{}
	}

	// Type assert to *http.Transport to modify settings
	if t, ok := transport.(*http.Transport); ok {
		// Configure connection pooling and timeouts
		t.MaxIdleConnsPerHost = opts.MaxIdleConnsPerHost

		// Only set these if they're not already set
		if t.IdleConnTimeout == 0 {
			t.IdleConnTimeout = opts.IdleConnTimeout
		}
		if t.TLSHandshakeTimeout == 0 {
			t.TLSHandshakeTimeout = opts.TLSHandshakeTimeout
		}

		// Set optional flags
		t.DisableCompression = opts.DisableCompression
		t.DisableKeepAlives = opts.DisableKeepAlives

		// Configure DNS cache
		if t.DialContext == nil {
			dialer := &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			t.DialContext = dialer.DialContext
		}

		// Update the client transport
		client.Transport = t
	}

	return client
}

package performance

import (
	"fmt"
	"net"
	"net/http"
	"os"
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
		// OptimizeHTTPClient previously failed silently here, which made
		// "I configured pooling but my requests still feel slow" reports
		// impossible to debug. Surface the error to stderr so it shows up
		// in the consumer's logs while preserving the no-fail return
		// contract (we still hand back the original client).
		fmt.Fprintf(os.Stderr, "[Midaz SDK Performance] OptimizeHTTPClient failed: %v\n", err)

		return client
	}

	if transport, ok := optimized.Transport.(*http.Transport); ok {
		// MaxIdleConnsPerHost / DisableCompression / DisableKeepAlives are
		// already applied by OptimizeHTTPClient via the With* options
		// above. We used to write them again here, which was redundant
		// and made it look like the function was the source of truth for
		// those fields.
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

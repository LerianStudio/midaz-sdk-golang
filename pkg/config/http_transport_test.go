package config

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// namedTransport is a round tripper that only records which base it wraps.
type namedTransport struct {
	name string
	base http.RoundTripper
}

func (n *namedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return n.base.RoundTrip(req)
}

func wrapNamed(name string) func(http.RoundTripper) http.RoundTripper {
	return func(base http.RoundTripper) http.RoundTripper {
		return &namedTransport{name: name, base: base}
	}
}

// TestWithHTTPTransport_WrapsTheDefaultClientCreatedAfterOptions covers NewConfig,
// which creates its default client only after every option ran: the wrappers must
// still land on it, in option order, over the SDK's own pooled transport, and the
// client must keep the configured timeout.
func TestWithHTTPTransport_WrapsTheDefaultClientCreatedAfterOptions(t *testing.T) {
	cfg, err := NewConfig(
		WithHTTPTransport(wrapNamed("inner")),
		WithHTTPTransport(wrapNamed("outer")),
		WithTimeout(9*time.Second),
		WithAnonymous(),
	)
	require.NoError(t, err)

	outer, ok := cfg.HTTPClient.Transport.(*namedTransport)
	require.True(t, ok, "transport must be the wrapper, got %T", cfg.HTTPClient.Transport)
	assert.Equal(t, "outer", outer.name)

	inner, ok := outer.base.(*namedTransport)
	require.True(t, ok, "outer must wrap inner, got %T", outer.base)
	assert.Equal(t, "inner", inner.name)

	sdkTransport, ok := inner.base.(*http.Transport)
	require.True(t, ok, "innermost base must be the SDK pooled transport, got %T", inner.base)
	assert.Equal(t, 30*time.Second, sdkTransport.ResponseHeaderTimeout)

	assert.Equal(t, 9*time.Second, cfg.HTTPClient.Timeout)
	assert.True(t, cfg.httpClientOwned, "the default client stays SDK-owned")
}

func TestWithHTTPTransport_UnsetClientTransportFallsBackToDefaultTransport(t *testing.T) {
	cfg, err := NewConfig(
		WithHTTPClient(&http.Client{}),
		WithHTTPTransport(wrapNamed("only")),
		WithAnonymous(),
	)
	require.NoError(t, err)

	got, ok := cfg.HTTPClient.Transport.(*namedTransport)
	require.True(t, ok)
	assert.Same(t, http.DefaultTransport, got.base)
}

func TestWithHTTPTransport_Rejections(t *testing.T) {
	_, err := NewConfig(WithHTTPTransport(nil), WithAnonymous())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP transport wrapper cannot be nil")

	nilReturning := func(http.RoundTripper) http.RoundTripper { return nil }

	_, err = NewConfig(WithHTTPTransport(nilReturning), WithAnonymous())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "returned a nil round tripper")

	_, err = NewConfig(WithHTTPTransport(nilReturning), WithHTTPClient(&http.Client{}), WithAnonymous())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "returned a nil round tripper")

	require.Error(t, WithHTTPTransport(wrapNamed("x"))(nil))
}

// TestWithHTTPTransport_DoesNotLeakIntoAConfigSharingTheClient covers Clone, which
// keeps a caller-supplied client by pointer: wrapping the clone's transport must
// not wrap the original configuration's client too.
func TestWithHTTPTransport_DoesNotLeakIntoAConfigSharingTheClient(t *testing.T) {
	callerTransport := &namedTransport{name: "caller", base: http.DefaultTransport}

	original, err := NewConfig(WithHTTPClient(&http.Client{Transport: callerTransport}), WithAnonymous())
	require.NoError(t, err)

	clone := original.Clone()
	require.Same(t, original.HTTPClient, clone.HTTPClient, "precondition: Clone shares a caller-supplied client")

	require.NoError(t, WithHTTPTransport(wrapNamed("clone-only"))(clone))

	assert.Same(t, callerTransport, original.HTTPClient.Transport, "the original config must keep its transport")

	wrapped, ok := clone.HTTPClient.Transport.(*namedTransport)
	require.True(t, ok)
	assert.Equal(t, "clone-only", wrapped.name)
	assert.Same(t, callerTransport, wrapped.base)
}

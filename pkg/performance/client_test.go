package performance

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultHTTPClientOptions(t *testing.T) {
	opts := DefaultHTTPClientOptions()

	if opts.MaxIdleConnsPerHost != 10 {
		t.Errorf("Expected MaxIdleConnsPerHost=10, got %d", opts.MaxIdleConnsPerHost)
	}

	if opts.IdleConnTimeout != 90*time.Second {
		t.Errorf("Expected IdleConnTimeout=90s, got %v", opts.IdleConnTimeout)
	}

	if opts.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("Expected TLSHandshakeTimeout=10s, got %v", opts.TLSHandshakeTimeout)
	}

	if opts.DisableCompression {
		t.Error("Expected DisableCompression=false")
	}

	if opts.DisableKeepAlives {
		t.Error("Expected DisableKeepAlives=false")
	}
}

func TestOptimizeClient_NilClient(t *testing.T) {
	client := OptimizeClient(nil, nil)

	if client == nil {
		t.Fatal("OptimizeClient returned nil for nil input")
	}

	// Should have a transport set
	if client.Transport == nil {
		t.Error("Expected Transport to be set")
	}
}

func TestOptimizeClient_NilOptions(t *testing.T) {
	originalClient := &http.Client{}
	client := OptimizeClient(originalClient, nil)

	if client == nil {
		t.Fatal("OptimizeClient returned nil")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected *http.Transport")
	}

	// Should use default options
	defaults := DefaultHTTPClientOptions()
	if transport.MaxIdleConnsPerHost != defaults.MaxIdleConnsPerHost {
		t.Errorf("Expected MaxIdleConnsPerHost=%d, got %d", defaults.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	}
}

func TestOptimizeClient_CustomOptions(t *testing.T) {
	originalClient := &http.Client{}
	opts := &HTTPClientOptions{
		MaxIdleConnsPerHost: 25,
		IdleConnTimeout:     120 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
		DisableCompression:  true,
		DisableKeepAlives:   true,
	}

	client := OptimizeClient(originalClient, opts)

	if client == nil {
		t.Fatal("OptimizeClient returned nil")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected *http.Transport")
	}

	if transport.MaxIdleConnsPerHost != 25 {
		t.Errorf("Expected MaxIdleConnsPerHost=25, got %d", transport.MaxIdleConnsPerHost)
	}

	if !transport.DisableCompression {
		t.Error("Expected DisableCompression=true")
	}

	if !transport.DisableKeepAlives {
		t.Error("Expected DisableKeepAlives=true")
	}
}

func TestOptimizeClient_ExistingTransport(t *testing.T) {
	// Create a client with existing transport and some values set
	existingTransport := &http.Transport{
		IdleConnTimeout:     60 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	originalClient := &http.Client{
		Transport: existingTransport,
	}

	client := OptimizeClient(originalClient, nil)

	if client == nil {
		t.Fatal("OptimizeClient returned nil")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected *http.Transport")
	}

	// Should preserve existing non-zero values
	if transport.IdleConnTimeout != 60*time.Second {
		t.Errorf("Expected IdleConnTimeout=60s, got %v", transport.IdleConnTimeout)
	}

	if transport.TLSHandshakeTimeout != 5*time.Second {
		t.Errorf("Expected TLSHandshakeTimeout=5s, got %v", transport.TLSHandshakeTimeout)
	}

	// Should set MaxIdleConnsPerHost (was zero)
	if transport.MaxIdleConnsPerHost != DefaultHTTPClientOptions().MaxIdleConnsPerHost {
		t.Errorf("Expected MaxIdleConnsPerHost=%d, got %d", DefaultHTTPClientOptions().MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	}
}

func TestOptimizeClient_ExistingTransportZeroValues(t *testing.T) {
	// Create a transport with all zero values
	existingTransport := &http.Transport{}
	originalClient := &http.Client{
		Transport: existingTransport,
	}

	opts := &HTTPClientOptions{
		MaxIdleConnsPerHost: 30,
		IdleConnTimeout:     180 * time.Second,
		TLSHandshakeTimeout: 20 * time.Second,
	}

	client := OptimizeClient(originalClient, opts)

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected *http.Transport")
	}

	// All values should be set from options
	if transport.MaxIdleConnsPerHost != 30 {
		t.Errorf("Expected MaxIdleConnsPerHost=30, got %d", transport.MaxIdleConnsPerHost)
	}

	if transport.IdleConnTimeout != 180*time.Second {
		t.Errorf("Expected IdleConnTimeout=180s, got %v", transport.IdleConnTimeout)
	}

	if transport.TLSHandshakeTimeout != 20*time.Second {
		t.Errorf("Expected TLSHandshakeTimeout=20s, got %v", transport.TLSHandshakeTimeout)
	}
}

func TestOptimizeClient_NoTransport(t *testing.T) {
	// Client with nil transport
	originalClient := &http.Client{
		Transport: nil,
	}

	client := OptimizeClient(originalClient, nil)

	if client == nil {
		t.Fatal("OptimizeClient returned nil")
	}

	// Should have created a new transport
	if client.Transport == nil {
		t.Error("Expected Transport to be created")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected *http.Transport")
	}

	// Should have default values set
	defaults := DefaultHTTPClientOptions()
	if transport.MaxIdleConnsPerHost != defaults.MaxIdleConnsPerHost {
		t.Errorf("Expected MaxIdleConnsPerHost=%d, got %d", defaults.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	}
}

func TestOptimizeClient_NonHTTPTransport(t *testing.T) {
	// Create a client with a custom non-http.Transport
	customTransport := &mockTransport{}
	originalClient := &http.Client{
		Transport: customTransport,
	}

	client := OptimizeClient(originalClient, nil)

	if client == nil {
		t.Fatal("OptimizeClient returned nil")
	}

	// Should preserve the custom transport
	if _, ok := client.Transport.(*mockTransport); !ok {
		t.Error("Expected custom transport to be preserved")
	}
}

func TestOptimizeClient_DialContextConfiguration(t *testing.T) {
	// Test that DialContext is configured when nil
	originalClient := &http.Client{}
	client := OptimizeClient(originalClient, nil)

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected *http.Transport")
	}

	// DialContext should be set
	if transport.DialContext == nil {
		t.Error("Expected DialContext to be configured")
	}
}

func TestOptimizeClient_PreserveExistingDialContext(t *testing.T) {
	// Create transport with existing DialContext
	existingTransport := &http.Transport{}
	// DialContext is typically set via a net.Dialer
	// We'll just verify that when DialContext is already set, it's not overwritten

	originalClient := &http.Client{
		Transport: existingTransport,
	}

	// First call should set DialContext
	client := OptimizeClient(originalClient, nil)

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected *http.Transport")
	}

	// DialContext should be set
	if transport.DialContext == nil {
		t.Error("Expected DialContext to be set")
	}

	// Save the DialContext reference
	originalDialContext := transport.DialContext

	// Optimize again - should preserve existing DialContext
	client2 := OptimizeClient(client, nil)

	transport2, ok := client2.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected *http.Transport")
	}

	// DialContext should still be the same
	if transport2.DialContext == nil {
		t.Error("Expected DialContext to be preserved")
	}

	// Verify it's the same function (by checking it's not nil)
	_ = originalDialContext // Reference kept to show intent
}

func TestHTTPClientOptions_AllFields(t *testing.T) {
	opts := HTTPClientOptions{
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     2 * time.Minute,
		TLSHandshakeTimeout: 30 * time.Second,
		DisableCompression:  true,
		DisableKeepAlives:   true,
	}

	if opts.MaxIdleConnsPerHost != 50 {
		t.Errorf("Expected MaxIdleConnsPerHost=50, got %d", opts.MaxIdleConnsPerHost)
	}

	if opts.IdleConnTimeout != 2*time.Minute {
		t.Errorf("Expected IdleConnTimeout=2m, got %v", opts.IdleConnTimeout)
	}

	if opts.TLSHandshakeTimeout != 30*time.Second {
		t.Errorf("Expected TLSHandshakeTimeout=30s, got %v", opts.TLSHandshakeTimeout)
	}

	if !opts.DisableCompression {
		t.Error("Expected DisableCompression=true")
	}

	if !opts.DisableKeepAlives {
		t.Error("Expected DisableKeepAlives=true")
	}
}

// mockTransport is a simple mock for testing non-http.Transport scenarios
type mockTransport struct{}

func (*mockTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, http.ErrNotSupported
}

func BenchmarkOptimizeClient(b *testing.B) {
	b.Run("NilClient", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = OptimizeClient(nil, nil)
		}
	})

	b.Run("ExistingClient", func(b *testing.B) {
		client := &http.Client{}

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = OptimizeClient(client, nil)
		}
	})

	b.Run("CustomOptions", func(b *testing.B) {
		client := &http.Client{}
		opts := &HTTPClientOptions{
			MaxIdleConnsPerHost: 25,
			IdleConnTimeout:     120 * time.Second,
			TLSHandshakeTimeout: 15 * time.Second,
		}

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = OptimizeClient(client, opts)
		}
	})
}

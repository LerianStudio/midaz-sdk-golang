package performance

import (
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestNewTransport(t *testing.T) {
	// Test with default options
	transport, err := NewTransport()
	if err != nil {
		t.Fatalf("NewTransport() returned an error: %v", err)
	}
	if transport == nil {
		t.Fatal("NewTransport() returned nil transport")
	}

	// Verify default values are set
	if transport.MaxIdleConns != DefaultMaxIdleConns {
		t.Errorf("Expected MaxIdleConns=%d, got %d", DefaultMaxIdleConns, transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != DefaultMaxIdleConnsPerHost {
		t.Errorf("Expected MaxIdleConnsPerHost=%d, got %d", DefaultMaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	}
	if transport.MaxConnsPerHost != DefaultMaxConnsPerHost {
		t.Errorf("Expected MaxConnsPerHost=%d, got %d", DefaultMaxConnsPerHost, transport.MaxConnsPerHost)
	}
	if transport.IdleConnTimeout != DefaultIdleConnTimeout {
		t.Errorf("Expected IdleConnTimeout=%v, got %v", DefaultIdleConnTimeout, transport.IdleConnTimeout)
	}
	if transport.TLSHandshakeTimeout != DefaultTLSHandshakeTimeout {
		t.Errorf("Expected TLSHandshakeTimeout=%v, got %v", DefaultTLSHandshakeTimeout, transport.TLSHandshakeTimeout)
	}
	if transport.DisableKeepAlives != false {
		t.Errorf("Expected DisableKeepAlives=false, got %v", transport.DisableKeepAlives)
	}
	if transport.DisableCompression != false {
		t.Errorf("Expected DisableCompression=false, got %v", transport.DisableCompression)
	}

	// Test with custom options
	customTransport, err := NewTransport(
		WithMaxIdleConns(200),
		WithTransportMaxIdleConnsPerHost(20),
		WithMaxConnsPerHost(200),
		WithIdleConnTimeout(2*time.Minute),
		WithDisableKeepAlives(true),
		WithDisableCompression(true),
	)
	if err != nil {
		t.Fatalf("NewTransport() with options returned an error: %v", err)
	}

	if customTransport.MaxIdleConns != 200 {
		t.Errorf("Expected MaxIdleConns=200, got %d", customTransport.MaxIdleConns)
	}
	if customTransport.MaxIdleConnsPerHost != 20 {
		t.Errorf("Expected MaxIdleConnsPerHost=20, got %d", customTransport.MaxIdleConnsPerHost)
	}
	if customTransport.MaxConnsPerHost != 200 {
		t.Errorf("Expected MaxConnsPerHost=200, got %d", customTransport.MaxConnsPerHost)
	}
	if customTransport.IdleConnTimeout != 2*time.Minute {
		t.Errorf("Expected IdleConnTimeout=2m, got %v", customTransport.IdleConnTimeout)
	}
	if !customTransport.DisableKeepAlives {
		t.Errorf("Expected DisableKeepAlives=true, got false")
	}
	if !customTransport.DisableCompression {
		t.Errorf("Expected DisableCompression=true, got false")
	}

	// Test with invalid options
	_, err = NewTransport(
		WithMaxIdleConns(-1),
	)
	if err == nil {
		t.Fatalf("Expected NewTransport with negative MaxIdleConns to return an error, got nil")
	}
}

func TestNewClient(t *testing.T) {
	// Test with default options
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() returned an error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	// Verify default timeout
	if client.Timeout != DefaultTimeout {
		t.Errorf("Expected Timeout=%v, got %v", DefaultTimeout, client.Timeout)
	}

	// Verify transport is set
	if client.Transport == nil {
		t.Fatal("Client.Transport is nil")
	}

	// Test with custom timeout
	customClient, err := NewClient(
		WithTimeout(30 * time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient() with timeout returned an error: %v", err)
	}

	// Verify timeout is set
	if customClient.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout=30s, got %v", customClient.Timeout)
	}

	// Test with custom transport
	transport, err := NewTransport(
		WithMaxIdleConns(200),
		WithTransportMaxIdleConnsPerHost(20),
		WithMaxConnsPerHost(200),
	)
	if err != nil {
		t.Fatalf("NewTransport() returned an error: %v", err)
	}

	customClient, err = NewClient(
		WithTransport(transport),
	)
	if err != nil {
		t.Fatalf("NewClient() with transport returned an error: %v", err)
	}

	// Verify transport is the one we provided
	if customClient.Transport != transport {
		t.Errorf("Expected Transport=%v, got %v", transport, customClient.Transport)
	}

	// Test with invalid options
	_, err = NewClient(
		WithTimeout(-1 * time.Second),
	)
	if err == nil {
		t.Fatalf("Expected NewClient with negative timeout to return an error, got nil")
	}
}

func TestOptimizeClient(t *testing.T) {
	// Test with nil client
	client, err := OptimizeHTTPClient(nil)
	if err != nil {
		t.Fatalf("OptimizeHTTPClient(nil) returned an error: %v", err)
	}
	if client == nil {
		t.Fatal("OptimizeHTTPClient(nil) returned nil")
	}

	// Test with client that has no transport
	emptyClient := &http.Client{}
	optimizedClient, err := OptimizeHTTPClient(emptyClient)
	if err != nil {
		t.Fatalf("OptimizeHTTPClient(emptyClient) returned an error: %v", err)
	}

	// Check that the transport is set
	if optimizedClient.Transport == nil {
		t.Error("Expected Transport to be set, got nil")
	}

	// Test with client that has custom transport
	customTransport := &http.Transport{
		MaxIdleConns: 50,
		// Leave other fields at zero value for testing
	}
	customClient := &http.Client{
		Transport: customTransport,
		Timeout:   30 * time.Second,
	}

	optimized, err := OptimizeHTTPClient(customClient,
		WithTransportMaxIdleConnsPerHost(20),
	)
	if err != nil {
		t.Fatalf("OptimizeHTTPClient(customClient) returned an error: %v", err)
	}

	optimizedTransport, ok := optimized.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("optimized.Transport is not *http.Transport: %T", optimized.Transport)
	}

	// Should preserve the custom value
	if optimizedTransport.MaxIdleConns != 50 {
		t.Errorf("Expected MaxIdleConns=50, got %d", optimizedTransport.MaxIdleConns)
	}

	// Should set the custom value via options
	if optimizedTransport.MaxIdleConnsPerHost != 20 {
		t.Errorf("Expected MaxIdleConnsPerHost=%d, got %d", 20, optimizedTransport.MaxIdleConnsPerHost)
	}

	// Should preserve the client timeout
	if optimized.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout=30s, got %v", optimized.Timeout)
	}

	// Test with invalid options
	_, err = OptimizeHTTPClient(customClient, WithMaxIdleConns(-1))
	if err == nil {
		t.Fatalf("Expected OptimizeHTTPClient with negative MaxIdleConns to return an error, got nil")
	}
}

func TestDefaultTransportConfig(t *testing.T) {
	config := DefaultTransportConfig()

	expected := &TransportConfig{
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

	if !reflect.DeepEqual(config, expected) {
		t.Errorf("DefaultTransportConfig() returned unexpected result.\nGot: %+v\nExpected: %+v", config, expected)
	}
}

// BenchmarkHTTPPooling benchmarks the HTTP connection pooling
func BenchmarkHTTPPooling(b *testing.B) {
	// Skip the test for now due to issues with the test server
	b.Skip("HTTP pooling benchmark temporarily disabled")
}

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

func TestTransportOptions_Validation(t *testing.T) {
	t.Run("WithTransportMaxIdleConnsPerHost_Negative", func(t *testing.T) {
		_, err := NewTransport(WithTransportMaxIdleConnsPerHost(-1))
		if err == nil {
			t.Error("Expected error for negative MaxIdleConnsPerHost, got nil")
		}
	})

	t.Run("WithMaxConnsPerHost_Negative", func(t *testing.T) {
		_, err := NewTransport(WithMaxConnsPerHost(-1))
		if err == nil {
			t.Error("Expected error for negative MaxConnsPerHost, got nil")
		}
	})

	t.Run("WithIdleConnTimeout_Negative", func(t *testing.T) {
		_, err := NewTransport(WithIdleConnTimeout(-1 * time.Second))
		if err == nil {
			t.Error("Expected error for negative IdleConnTimeout, got nil")
		}
	})

	t.Run("WithTLSHandshakeTimeout_Valid", func(t *testing.T) {
		transport, err := NewTransport(WithTLSHandshakeTimeout(15 * time.Second))
		if err != nil {
			t.Fatalf("NewTransport returned error: %v", err)
		}
		if transport.TLSHandshakeTimeout != 15*time.Second {
			t.Errorf("Expected TLSHandshakeTimeout=15s, got %v", transport.TLSHandshakeTimeout)
		}
	})

	t.Run("WithTLSHandshakeTimeout_Negative", func(t *testing.T) {
		_, err := NewTransport(WithTLSHandshakeTimeout(-1 * time.Second))
		if err == nil {
			t.Error("Expected error for negative TLSHandshakeTimeout, got nil")
		}
	})

	t.Run("WithResponseHeaderTimeout_Valid", func(t *testing.T) {
		transport, err := NewTransport(WithResponseHeaderTimeout(20 * time.Second))
		if err != nil {
			t.Fatalf("NewTransport returned error: %v", err)
		}
		if transport.ResponseHeaderTimeout != 20*time.Second {
			t.Errorf("Expected ResponseHeaderTimeout=20s, got %v", transport.ResponseHeaderTimeout)
		}
	})

	t.Run("WithResponseHeaderTimeout_Negative", func(t *testing.T) {
		_, err := NewTransport(WithResponseHeaderTimeout(-1 * time.Second))
		if err == nil {
			t.Error("Expected error for negative ResponseHeaderTimeout, got nil")
		}
	})

	t.Run("WithExpectContinueTimeout_Valid", func(t *testing.T) {
		transport, err := NewTransport(WithExpectContinueTimeout(2 * time.Second))
		if err != nil {
			t.Fatalf("NewTransport returned error: %v", err)
		}
		if transport.ExpectContinueTimeout != 2*time.Second {
			t.Errorf("Expected ExpectContinueTimeout=2s, got %v", transport.ExpectContinueTimeout)
		}
	})

	t.Run("WithExpectContinueTimeout_Negative", func(t *testing.T) {
		_, err := NewTransport(WithExpectContinueTimeout(-1 * time.Second))
		if err == nil {
			t.Error("Expected error for negative ExpectContinueTimeout, got nil")
		}
	})

	t.Run("WithDialTimeout_Valid", func(t *testing.T) {
		transport, err := NewTransport(WithDialTimeout(45 * time.Second))
		if err != nil {
			t.Fatalf("NewTransport returned error: %v", err)
		}
		// DialTimeout is set via Dialer, not directly on transport
		// Just verify no error occurred
		if transport == nil {
			t.Error("Expected transport, got nil")
		}
	})

	t.Run("WithDialTimeout_Negative", func(t *testing.T) {
		_, err := NewTransport(WithDialTimeout(-1 * time.Second))
		if err == nil {
			t.Error("Expected error for negative DialTimeout, got nil")
		}
	})

	t.Run("WithKeepAlive_Valid", func(t *testing.T) {
		transport, err := NewTransport(WithKeepAlive(60 * time.Second))
		if err != nil {
			t.Fatalf("NewTransport returned error: %v", err)
		}
		if transport == nil {
			t.Error("Expected transport, got nil")
		}
	})

	t.Run("WithKeepAlive_Negative", func(t *testing.T) {
		_, err := NewTransport(WithKeepAlive(-1 * time.Second))
		if err == nil {
			t.Error("Expected error for negative KeepAlive, got nil")
		}
	})
}

func TestTransportPresets(t *testing.T) {
	t.Run("WithHighThroughput", func(t *testing.T) {
		transport, err := NewTransport(WithHighThroughput())
		if err != nil {
			t.Fatalf("NewTransport returned error: %v", err)
		}

		if transport.MaxIdleConns != 200 {
			t.Errorf("Expected MaxIdleConns=200, got %d", transport.MaxIdleConns)
		}
		if transport.MaxIdleConnsPerHost != 50 {
			t.Errorf("Expected MaxIdleConnsPerHost=50, got %d", transport.MaxIdleConnsPerHost)
		}
		if transport.MaxConnsPerHost != 200 {
			t.Errorf("Expected MaxConnsPerHost=200, got %d", transport.MaxConnsPerHost)
		}
		if transport.IdleConnTimeout != 180*time.Second {
			t.Errorf("Expected IdleConnTimeout=180s, got %v", transport.IdleConnTimeout)
		}
	})

	t.Run("WithLowLatency", func(t *testing.T) {
		transport, err := NewTransport(WithLowLatency())
		if err != nil {
			t.Fatalf("NewTransport returned error: %v", err)
		}

		if transport.TLSHandshakeTimeout != 5*time.Second {
			t.Errorf("Expected TLSHandshakeTimeout=5s, got %v", transport.TLSHandshakeTimeout)
		}
		if transport.ResponseHeaderTimeout != 15*time.Second {
			t.Errorf("Expected ResponseHeaderTimeout=15s, got %v", transport.ResponseHeaderTimeout)
		}
		if transport.ExpectContinueTimeout != 500*time.Millisecond {
			t.Errorf("Expected ExpectContinueTimeout=500ms, got %v", transport.ExpectContinueTimeout)
		}
	})
}

func TestNewTransportConfig(t *testing.T) {
	t.Run("DefaultValues", func(t *testing.T) {
		config, err := NewTransportConfig()
		if err != nil {
			t.Fatalf("NewTransportConfig returned error: %v", err)
		}

		if config.MaxIdleConns != DefaultMaxIdleConns {
			t.Errorf("Expected MaxIdleConns=%d, got %d", DefaultMaxIdleConns, config.MaxIdleConns)
		}
		if config.MaxIdleConnsPerHost != DefaultMaxIdleConnsPerHost {
			t.Errorf("Expected MaxIdleConnsPerHost=%d, got %d", DefaultMaxIdleConnsPerHost, config.MaxIdleConnsPerHost)
		}
	})

	t.Run("WithMultipleOptions", func(t *testing.T) {
		config, err := NewTransportConfig(
			WithMaxIdleConns(150),
			WithTransportMaxIdleConnsPerHost(25),
			WithMaxConnsPerHost(150),
			WithIdleConnTimeout(120*time.Second),
			WithDisableKeepAlives(true),
			WithDisableCompression(true),
		)
		if err != nil {
			t.Fatalf("NewTransportConfig returned error: %v", err)
		}

		if config.MaxIdleConns != 150 {
			t.Errorf("Expected MaxIdleConns=150, got %d", config.MaxIdleConns)
		}
		if config.MaxIdleConnsPerHost != 25 {
			t.Errorf("Expected MaxIdleConnsPerHost=25, got %d", config.MaxIdleConnsPerHost)
		}
		if config.MaxConnsPerHost != 150 {
			t.Errorf("Expected MaxConnsPerHost=150, got %d", config.MaxConnsPerHost)
		}
		if config.IdleConnTimeout != 120*time.Second {
			t.Errorf("Expected IdleConnTimeout=120s, got %v", config.IdleConnTimeout)
		}
		if !config.DisableKeepAlives {
			t.Error("Expected DisableKeepAlives=true")
		}
		if !config.DisableCompression {
			t.Error("Expected DisableCompression=true")
		}
	})

	t.Run("WithInvalidOption", func(t *testing.T) {
		_, err := NewTransportConfig(WithMaxIdleConns(-1))
		if err == nil {
			t.Error("Expected error for invalid option, got nil")
		}
	})
}

func TestNewTransportWithConfig_NilConfig(t *testing.T) {
	// Call newTransportWithConfig with nil to test default handling
	transport := newTransportWithConfig(nil)
	if transport == nil {
		t.Error("Expected transport, got nil")
	}

	// Should use default values
	if transport.MaxIdleConns != DefaultMaxIdleConns {
		t.Errorf("Expected MaxIdleConns=%d, got %d", DefaultMaxIdleConns, transport.MaxIdleConns)
	}
}

func TestOptimizeHTTPClient_CustomTransport(t *testing.T) {
	t.Run("CustomTransportWithNonHTTPTransport", func(t *testing.T) {
		// Create a custom round tripper that is not *http.Transport
		customClient := &http.Client{
			Transport: &customRoundTripper{},
		}

		optimized, err := OptimizeHTTPClient(customClient)
		if err != nil {
			t.Fatalf("OptimizeHTTPClient returned error: %v", err)
		}

		// Should not modify non-http.Transport
		if _, ok := optimized.Transport.(*customRoundTripper); !ok {
			t.Error("Expected custom transport to be preserved")
		}
	})

	t.Run("TransportWithZeroTimeout", func(t *testing.T) {
		client := &http.Client{
			Transport: &http.Transport{},
			Timeout:   0, // Zero timeout
		}

		optimized, err := OptimizeHTTPClient(client)
		if err != nil {
			t.Fatalf("OptimizeHTTPClient returned error: %v", err)
		}

		// Should set default timeout
		if optimized.Timeout != DefaultTimeout {
			t.Errorf("Expected Timeout=%v, got %v", DefaultTimeout, optimized.Timeout)
		}
	})

	t.Run("TransportWithExistingValues", func(t *testing.T) {
		client := &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:          50,
				MaxIdleConnsPerHost:   5,
				MaxConnsPerHost:       50,
				IdleConnTimeout:       60 * time.Second,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 15 * time.Second,
				ExpectContinueTimeout: 500 * time.Millisecond,
			},
			Timeout: 30 * time.Second,
		}

		optimized, err := OptimizeHTTPClient(client)
		if err != nil {
			t.Fatalf("OptimizeHTTPClient returned error: %v", err)
		}

		transport, ok := optimized.Transport.(*http.Transport)
		if !ok {
			t.Fatal("Expected *http.Transport")
		}

		// Should preserve existing non-zero values
		if transport.MaxIdleConns != 50 {
			t.Errorf("Expected MaxIdleConns=50, got %d", transport.MaxIdleConns)
		}
		if transport.MaxIdleConnsPerHost != 5 {
			t.Errorf("Expected MaxIdleConnsPerHost=5, got %d", transport.MaxIdleConnsPerHost)
		}
		if optimized.Timeout != 30*time.Second {
			t.Errorf("Expected Timeout=30s, got %v", optimized.Timeout)
		}
	})
}

// customRoundTripper is a mock RoundTripper for testing
type customRoundTripper struct{}

func (c *customRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, nil
}

func TestHTTPClientOptions(t *testing.T) {
	t.Run("WithTimeoutValid", func(t *testing.T) {
		client, err := NewClient(WithTimeout(45 * time.Second))
		if err != nil {
			t.Fatalf("NewClient returned error: %v", err)
		}
		if client.Timeout != 45*time.Second {
			t.Errorf("Expected Timeout=45s, got %v", client.Timeout)
		}
	})

	t.Run("WithTimeoutZero", func(t *testing.T) {
		client, err := NewClient(WithTimeout(0))
		if err != nil {
			t.Fatalf("NewClient returned error: %v", err)
		}
		if client.Timeout != 0 {
			t.Errorf("Expected Timeout=0, got %v", client.Timeout)
		}
	})

	t.Run("WithNilTransport", func(t *testing.T) {
		client, err := NewClient(WithTransport(nil))
		if err != nil {
			t.Fatalf("NewClient returned error: %v", err)
		}
		// With nil transport, it should remain nil
		if client.Transport != nil {
			t.Errorf("Expected nil transport")
		}
	})
}

func TestNewClient_AllOptions(t *testing.T) {
	transport, err := NewTransport(
		WithMaxIdleConns(100),
		WithHighThroughput(),
	)
	if err != nil {
		t.Fatalf("NewTransport returned error: %v", err)
	}

	client, err := NewClient(
		WithTimeout(90*time.Second),
		WithTransport(transport),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	if client.Timeout != 90*time.Second {
		t.Errorf("Expected Timeout=90s, got %v", client.Timeout)
	}
	if client.Transport != transport {
		t.Error("Expected custom transport to be set")
	}
}

// BenchmarkHTTPPooling benchmarks the HTTP connection pooling
func BenchmarkHTTPPooling(b *testing.B) {
	// Skip the test for now due to issues with the test server
	b.Skip("HTTP pooling benchmark temporarily disabled")
}

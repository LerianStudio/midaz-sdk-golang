package performance

import (
	"testing"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.BatchSize != 50 {
		t.Errorf("Expected BatchSize=50, got %d", opts.BatchSize)
	}
	if !opts.EnableHTTPPooling {
		t.Error("Expected EnableHTTPPooling=true")
	}
	if opts.MaxIdleConnsPerHost != 10 {
		t.Errorf("Expected MaxIdleConnsPerHost=10, got %d", opts.MaxIdleConnsPerHost)
	}
	if !opts.UseJSONIterator {
		t.Error("Expected UseJSONIterator=true")
	}
}

func TestNewOptions(t *testing.T) {
	t.Run("DefaultValues", func(t *testing.T) {
		opts, err := NewOptions()
		if err != nil {
			t.Fatalf("NewOptions returned error: %v", err)
		}

		defaultOpts := DefaultOptions()
		if opts.BatchSize != defaultOpts.BatchSize {
			t.Errorf("Expected BatchSize=%d, got %d", defaultOpts.BatchSize, opts.BatchSize)
		}
	})

	t.Run("WithBatchSize_Valid", func(t *testing.T) {
		opts, err := NewOptions(WithBatchSize(100))
		if err != nil {
			t.Fatalf("NewOptions returned error: %v", err)
		}

		if opts.BatchSize != 100 {
			t.Errorf("Expected BatchSize=100, got %d", opts.BatchSize)
		}
	})

	t.Run("WithBatchSize_Invalid", func(t *testing.T) {
		_, err := NewOptions(WithBatchSize(0))
		if err == nil {
			t.Error("Expected error for zero BatchSize, got nil")
		}

		_, err = NewOptions(WithBatchSize(-1))
		if err == nil {
			t.Error("Expected error for negative BatchSize, got nil")
		}
	})

	t.Run("WithHTTPPooling", func(t *testing.T) {
		opts, err := NewOptions(WithHTTPPooling(false))
		if err != nil {
			t.Fatalf("NewOptions returned error: %v", err)
		}

		if opts.EnableHTTPPooling {
			t.Error("Expected EnableHTTPPooling=false")
		}

		opts, err = NewOptions(WithHTTPPooling(true))
		if err != nil {
			t.Fatalf("NewOptions returned error: %v", err)
		}

		if !opts.EnableHTTPPooling {
			t.Error("Expected EnableHTTPPooling=true")
		}
	})

	t.Run("WithMaxIdleConnsPerHost_Valid", func(t *testing.T) {
		opts, err := NewOptions(WithMaxIdleConnsPerHost(20))
		if err != nil {
			t.Fatalf("NewOptions returned error: %v", err)
		}

		if opts.MaxIdleConnsPerHost != 20 {
			t.Errorf("Expected MaxIdleConnsPerHost=20, got %d", opts.MaxIdleConnsPerHost)
		}
	})

	t.Run("WithMaxIdleConnsPerHost_Invalid", func(t *testing.T) {
		_, err := NewOptions(WithMaxIdleConnsPerHost(-1))
		if err == nil {
			t.Error("Expected error for negative MaxIdleConnsPerHost, got nil")
		}
	})

	t.Run("WithMaxIdleConnsPerHost_Zero", func(t *testing.T) {
		opts, err := NewOptions(WithMaxIdleConnsPerHost(0))
		if err != nil {
			t.Fatalf("NewOptions returned error: %v", err)
		}

		if opts.MaxIdleConnsPerHost != 0 {
			t.Errorf("Expected MaxIdleConnsPerHost=0, got %d", opts.MaxIdleConnsPerHost)
		}
	})

	t.Run("WithJSONIterator", func(t *testing.T) {
		opts, err := NewOptions(WithJSONIterator(false))
		if err != nil {
			t.Fatalf("NewOptions returned error: %v", err)
		}

		if opts.UseJSONIterator {
			t.Error("Expected UseJSONIterator=false")
		}

		opts, err = NewOptions(WithJSONIterator(true))
		if err != nil {
			t.Fatalf("NewOptions returned error: %v", err)
		}

		if !opts.UseJSONIterator {
			t.Error("Expected UseJSONIterator=true")
		}
	})

	t.Run("MultipleOptions", func(t *testing.T) {
		opts, err := NewOptions(
			WithBatchSize(200),
			WithHTTPPooling(false),
			WithMaxIdleConnsPerHost(25),
			WithJSONIterator(false),
		)
		if err != nil {
			t.Fatalf("NewOptions returned error: %v", err)
		}

		if opts.BatchSize != 200 {
			t.Errorf("Expected BatchSize=200, got %d", opts.BatchSize)
		}
		if opts.EnableHTTPPooling {
			t.Error("Expected EnableHTTPPooling=false")
		}
		if opts.MaxIdleConnsPerHost != 25 {
			t.Errorf("Expected MaxIdleConnsPerHost=25, got %d", opts.MaxIdleConnsPerHost)
		}
		if opts.UseJSONIterator {
			t.Error("Expected UseJSONIterator=false")
		}
	})
}

func TestApplyGlobalPerformanceOptions(t *testing.T) {
	// Save original options
	originalOptions := GetGlobalOptions()
	defer ApplyGlobalPerformanceOptions(originalOptions)

	t.Run("ApplyFullOptions", func(t *testing.T) {
		opts := Options{
			BatchSize:           100,
			EnableHTTPPooling:   false,
			MaxIdleConnsPerHost: 20,
			UseJSONIterator:     false,
		}

		ApplyGlobalPerformanceOptions(opts)

		result := GetGlobalOptions()
		if result.BatchSize != 100 {
			t.Errorf("Expected BatchSize=100, got %d", result.BatchSize)
		}
		if result.EnableHTTPPooling {
			t.Error("Expected EnableHTTPPooling=false")
		}
		if result.MaxIdleConnsPerHost != 20 {
			t.Errorf("Expected MaxIdleConnsPerHost=20, got %d", result.MaxIdleConnsPerHost)
		}
		if result.UseJSONIterator {
			t.Error("Expected UseJSONIterator=false")
		}
	})

	t.Run("ApplyPartialOptions", func(t *testing.T) {
		// First set known values
		ApplyGlobalPerformanceOptions(Options{
			BatchSize:           50,
			EnableHTTPPooling:   true,
			MaxIdleConnsPerHost: 10,
			UseJSONIterator:     true,
		})

		// Apply partial options (BatchSize=0 should not change)
		ApplyGlobalPerformanceOptions(Options{
			BatchSize:           0, // Should not change
			EnableHTTPPooling:   false,
			MaxIdleConnsPerHost: 0, // Should not change
		})

		result := GetGlobalOptions()
		if result.BatchSize != 50 {
			t.Errorf("Expected BatchSize=50, got %d", result.BatchSize)
		}
		if result.EnableHTTPPooling {
			t.Error("Expected EnableHTTPPooling=false")
		}
		if result.MaxIdleConnsPerHost != 10 {
			t.Errorf("Expected MaxIdleConnsPerHost=10, got %d", result.MaxIdleConnsPerHost)
		}
	})
}

func TestApplyGlobalOptions(t *testing.T) {
	// Save original options
	originalOptions := GetGlobalOptions()
	defer ApplyGlobalPerformanceOptions(originalOptions)

	t.Run("ValidOptions", func(t *testing.T) {
		err := ApplyGlobalOptions(
			WithBatchSize(75),
			WithHTTPPooling(false),
			WithMaxIdleConnsPerHost(15),
		)
		if err != nil {
			t.Fatalf("ApplyGlobalOptions returned error: %v", err)
		}

		result := GetGlobalOptions()
		if result.BatchSize != 75 {
			t.Errorf("Expected BatchSize=75, got %d", result.BatchSize)
		}
		if result.EnableHTTPPooling {
			t.Error("Expected EnableHTTPPooling=false")
		}
		if result.MaxIdleConnsPerHost != 15 {
			t.Errorf("Expected MaxIdleConnsPerHost=15, got %d", result.MaxIdleConnsPerHost)
		}
	})

	t.Run("InvalidOptions", func(t *testing.T) {
		err := ApplyGlobalOptions(WithBatchSize(-1))
		if err == nil {
			t.Error("Expected error for invalid BatchSize, got nil")
		}
	})
}

func TestApplyBatchingOptions(t *testing.T) {
	// Save original options
	originalOptions := GetGlobalOptions()
	defer ApplyGlobalPerformanceOptions(originalOptions)

	t.Run("ApplyBatchSize", func(t *testing.T) {
		ApplyBatchingOptions(Options{BatchSize: 150})

		result := GetBatchSize()
		if result != 150 {
			t.Errorf("Expected BatchSize=150, got %d", result)
		}
	})

	t.Run("ZeroBatchSizeNotApplied", func(t *testing.T) {
		// First set a known value
		ApplyBatchingOptions(Options{BatchSize: 200})

		// Try to apply zero (should not change)
		ApplyBatchingOptions(Options{BatchSize: 0})

		result := GetBatchSize()
		if result != 200 {
			t.Errorf("Expected BatchSize=200, got %d", result)
		}
	})
}

func TestGetGlobalOptions(t *testing.T) {
	// Just ensure it returns a copy of the global options
	opts := GetGlobalOptions()

	// Modify the returned copy
	opts.BatchSize = 9999

	// Verify global options are unchanged
	globalOpts := GetGlobalOptions()
	if globalOpts.BatchSize == 9999 {
		t.Error("GetGlobalOptions should return a copy, not a reference")
	}
}

func TestGetBatchSize(t *testing.T) {
	// Save original options
	originalOptions := GetGlobalOptions()
	defer ApplyGlobalPerformanceOptions(originalOptions)

	ApplyGlobalPerformanceOptions(Options{BatchSize: 123})

	result := GetBatchSize()
	if result != 123 {
		t.Errorf("Expected BatchSize=123, got %d", result)
	}
}

func TestGetOptimalBatchSize(t *testing.T) {
	// Save original options
	originalOptions := GetGlobalOptions()
	defer ApplyGlobalPerformanceOptions(originalOptions)

	t.Run("ExactDivisor", func(t *testing.T) {
		// 100 divides 1000 evenly
		result := GetOptimalBatchSize(1000, 100)
		if result != 100 {
			t.Errorf("Expected 100, got %d", result)
		}
	})

	t.Run("FindDivisor", func(t *testing.T) {
		// For 100 total with max 30, should find divisor
		// 100 % 25 == 0, 100 % 20 == 0, 100 % 10 == 0, etc.
		result := GetOptimalBatchSize(100, 30)
		// Should be 25 or lower divisor
		if result > 30 {
			t.Errorf("Expected result <= 30, got %d", result)
		}
		if 100%result != 0 {
			t.Errorf("Expected divisor of 100, got %d", result)
		}
	})

	t.Run("NoDivisor", func(t *testing.T) {
		// Prime number total with small max
		// 97 is prime, so no divisor between 2 and 30
		result := GetOptimalBatchSize(97, 30)
		if result != 30 {
			t.Errorf("Expected 30 (max), got %d", result)
		}
	})

	t.Run("TotalLessThanMax", func(t *testing.T) {
		result := GetOptimalBatchSize(50, 100)
		if result != 50 {
			t.Errorf("Expected 50, got %d", result)
		}
	})

	t.Run("ZeroMaxBatchSize", func(t *testing.T) {
		ApplyGlobalPerformanceOptions(Options{BatchSize: 75})

		result := GetOptimalBatchSize(1000, 0)
		if result != 75 {
			t.Errorf("Expected global batch size 75, got %d", result)
		}
	})

	t.Run("NegativeMaxBatchSize", func(t *testing.T) {
		ApplyGlobalPerformanceOptions(Options{BatchSize: 80})

		result := GetOptimalBatchSize(1000, -1)
		if result != 80 {
			t.Errorf("Expected global batch size 80, got %d", result)
		}
	})

	t.Run("ExactMatch", func(t *testing.T) {
		// When total equals max
		result := GetOptimalBatchSize(100, 100)
		if result != 100 {
			t.Errorf("Expected 100, got %d", result)
		}
	})

	t.Run("LargeTotalSmallMax", func(t *testing.T) {
		// 10000 with max 200
		result := GetOptimalBatchSize(10000, 200)
		if result > 200 {
			t.Errorf("Expected result <= 200, got %d", result)
		}
		if 10000%result != 0 {
			t.Errorf("Expected divisor of 10000, got %d", result)
		}
	})
}

func TestOptionsStruct(t *testing.T) {
	// Test direct struct initialization
	opts := Options{
		BatchSize:           100,
		EnableHTTPPooling:   true,
		MaxIdleConnsPerHost: 20,
		UseJSONIterator:     true,
	}

	if opts.BatchSize != 100 {
		t.Errorf("Expected BatchSize=100, got %d", opts.BatchSize)
	}
	if !opts.EnableHTTPPooling {
		t.Error("Expected EnableHTTPPooling=true")
	}
	if opts.MaxIdleConnsPerHost != 20 {
		t.Errorf("Expected MaxIdleConnsPerHost=20, got %d", opts.MaxIdleConnsPerHost)
	}
	if !opts.UseJSONIterator {
		t.Error("Expected UseJSONIterator=true")
	}
}

func BenchmarkGetOptimalBatchSize(b *testing.B) {
	b.Run("SmallTotal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			GetOptimalBatchSize(100, 50)
		}
	})

	b.Run("LargeTotal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			GetOptimalBatchSize(1000000, 1000)
		}
	})

	b.Run("PrimeTotal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			GetOptimalBatchSize(104729, 500) // Prime number
		}
	})
}

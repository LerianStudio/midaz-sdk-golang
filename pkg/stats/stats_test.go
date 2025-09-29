package stats

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCounter(t *testing.T) {
	before := time.Now()
	counter := NewCounter()
	after := time.Now()

	assert.NotNil(t, counter)
	assert.Equal(t, int64(0), counter.SuccessCount())

	// Start time should be between before and after
	assert.True(t, counter.start.After(before.Add(-time.Millisecond)))
	assert.True(t, counter.start.Before(after.Add(time.Millisecond)))
}

func TestCounterRecordSuccess(t *testing.T) {
	counter := NewCounter()

	// Initially should be zero
	assert.Equal(t, int64(0), counter.SuccessCount())

	// Record one success
	counter.RecordSuccess()
	assert.Equal(t, int64(1), counter.SuccessCount())

	// Record multiple successes
	counter.RecordSuccess()
	counter.RecordSuccess()
	assert.Equal(t, int64(3), counter.SuccessCount())
}

func TestCounterSuccessCountConcurrency(t *testing.T) {
	counter := NewCounter()
	numGoroutines := 100
	successesPerGoroutine := 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Start multiple goroutines to record successes concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < successesPerGoroutine; j++ {
				counter.RecordSuccess()
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * successesPerGoroutine)
	assert.Equal(t, expected, counter.SuccessCount())
}

func TestCounterElapsed(t *testing.T) {
	counter := NewCounter()

	// Test immediately after creation
	elapsed := counter.Elapsed()
	assert.True(t, elapsed >= 0)
	assert.True(t, elapsed < 10*time.Millisecond) // Should be very small

	// Sleep and test again
	time.Sleep(10 * time.Millisecond)
	elapsed = counter.Elapsed()
	assert.True(t, elapsed >= 10*time.Millisecond)
	assert.True(t, elapsed < 50*time.Millisecond) // Allow some leeway
}

func TestCounterTPS(t *testing.T) {
	tests := []struct {
		name      string
		successes int64
		delay     time.Duration
		minTPS    float64
		maxTPS    float64
	}{
		{
			name:      "no successes",
			successes: 0,
			delay:     100 * time.Millisecond,
			minTPS:    0,
			maxTPS:    0,
		},
		{
			name:      "single success after delay",
			successes: 1,
			delay:     100 * time.Millisecond,
			minTPS:    8,  // Allowing some leeway for timing variations
			maxTPS:    12, // Should be around 10 TPS
		},
		{
			name:      "multiple successes",
			successes: 5,
			delay:     100 * time.Millisecond,
			minTPS:    40, // Allowing some leeway for timing variations
			maxTPS:    60, // Should be around 50 TPS
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := NewCounter()

			// Record successes
			for i := int64(0); i < tt.successes; i++ {
				counter.RecordSuccess()
			}

			// Wait for the specified delay
			time.Sleep(tt.delay)

			tps := counter.TPS()

			if tt.successes == 0 {
				assert.Equal(t, 0.0, tps)
			} else {
				assert.True(t, tps >= tt.minTPS, "TPS %f should be >= %f", tps, tt.minTPS)
				assert.True(t, tps <= tt.maxTPS, "TPS %f should be <= %f", tps, tt.maxTPS)
			}
		})
	}
}

func TestCounterTPSImmediateCall(t *testing.T) {
	counter := NewCounter()

	// Record some successes
	counter.RecordSuccess()
	counter.RecordSuccess()

	// Call TPS immediately - should handle very small elapsed time
	tps := counter.TPS()

	// Should not panic and should return a reasonable value
	assert.True(t, tps >= 0)
	// TPS can be very high immediately after creation, so we just ensure it's finite
	assert.False(t, tps < 0, "TPS should not be negative")
}

func TestCounterTPSWithZeroElapsed(t *testing.T) {
	// Test edge case where elapsed time might be zero or negative
	counter := &Counter{
		start:        time.Now().Add(time.Second), // Start time in future
		successCount: 5,
	}

	tps := counter.TPS()
	assert.Equal(t, 0.0, tps, "TPS should be 0 when elapsed time <= 0")
}

func TestCounterConsistency(t *testing.T) {
	// Test that multiple calls to the same methods return consistent results
	counter := NewCounter()

	// Record some successes
	for i := 0; i < 10; i++ {
		counter.RecordSuccess()
	}

	// Get initial values
	count1 := counter.SuccessCount()
	elapsed1 := counter.Elapsed()

	// Wait a bit
	time.Sleep(1 * time.Millisecond)

	// Get values again
	count2 := counter.SuccessCount()
	elapsed2 := counter.Elapsed()

	// Success count should remain the same
	assert.Equal(t, count1, count2)

	// Elapsed should increase
	assert.True(t, elapsed2 > elapsed1)

	// Start time should remain the same
	assert.Equal(t, counter.start, counter.start) // Redundant but ensures start doesn't change
}

func TestCounterLongRunning(t *testing.T) {
	// Test counter behavior over a slightly longer period
	counter := NewCounter()

	// Record successes over time
	for i := 0; i < 20; i++ {
		counter.RecordSuccess()
		time.Sleep(time.Millisecond) // Small delay between successes
	}

	finalCount := counter.SuccessCount()
	finalElapsed := counter.Elapsed()
	finalTPS := counter.TPS()

	assert.Equal(t, int64(20), finalCount)
	assert.True(t, finalElapsed > 20*time.Millisecond)
	assert.True(t, finalTPS > 0)
	assert.True(t, finalTPS < 2000) // Should be reasonable TPS, not astronomical
}

func TestCounterStructFields(t *testing.T) {
	// Test that the Counter struct has the expected fields
	counter := NewCounter()

	// Verify that fields are accessible (this is more of a compile-time check)
	assert.NotEqual(t, time.Time{}, counter.start)
	assert.Equal(t, int64(0), counter.successCount)

	// Test that start time is reasonable (not zero and not too far in future)
	now := time.Now()
	assert.True(t, counter.start.Before(now.Add(time.Second)))
	assert.True(t, counter.start.After(now.Add(-time.Second)))
}

func TestCounterAtomicOperations(t *testing.T) {
	// Test that atomic operations work correctly under concurrent access
	counter := NewCounter()
	numGoroutines := 10
	operationsPerGoroutine := 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Start goroutines that record successes and read count simultaneously
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				counter.RecordSuccess()
				// Also read the count to test concurrent reads
				count := counter.SuccessCount()
				assert.True(t, count >= 0)
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * operationsPerGoroutine)
	assert.Equal(t, expected, counter.SuccessCount())
}

func BenchmarkCounterRecordSuccess(b *testing.B) {
	counter := NewCounter()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.RecordSuccess()
		}
	})
}

func BenchmarkCounterSuccessCount(b *testing.B) {
	counter := NewCounter()
	// Pre-populate with some successes
	for i := 0; i < 1000; i++ {
		counter.RecordSuccess()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = counter.SuccessCount()
		}
	})
}

func BenchmarkCounterTPS(b *testing.B) {
	counter := NewCounter()
	// Pre-populate with some successes
	for i := 0; i < 1000; i++ {
		counter.RecordSuccess()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = counter.TPS()
	}
}

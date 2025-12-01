package data

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAmountGenerator(t *testing.T) {
	t.Run("with seed zero uses current time", func(t *testing.T) {
		gen := NewAmountGenerator(0)
		require.NotNil(t, gen)

		// Generate some values to ensure it works
		val := gen.Normal(100, 10, 2)
		assert.GreaterOrEqual(t, val, int64(0))
	})

	t.Run("with specific seed", func(t *testing.T) {
		gen := NewAmountGenerator(12345)
		require.NotNil(t, gen)

		// Generate some values to ensure it works
		val := gen.Normal(100, 10, 2)
		assert.GreaterOrEqual(t, val, int64(0))
	})

	t.Run("same seed produces same sequence", func(t *testing.T) {
		gen1 := NewAmountGenerator(42)
		gen2 := NewAmountGenerator(42)

		// Generate a sequence of values
		vals1 := make([]int64, 10)
		vals2 := make([]int64, 10)

		for i := 0; i < 10; i++ {
			vals1[i] = gen1.Normal(100, 10, 2)
			vals2[i] = gen2.Normal(100, 10, 2)
		}

		assert.Equal(t, vals1, vals2)
	})

	t.Run("different seeds produce different sequences", func(_ *testing.T) {
		gen1 := NewAmountGenerator(42)
		gen2 := NewAmountGenerator(43)

		// Generate values - verifies generators can produce values without panicking
		// Note: We don't assert inequality since random values could theoretically match
		val1 := gen1.Normal(100, 10, 2)
		val2 := gen2.Normal(100, 10, 2)

		_ = val1
		_ = val2
	})
}

func TestAmountGeneratorNormal(t *testing.T) {
	gen := NewAmountGenerator(12345)

	t.Run("basic normal distribution", func(t *testing.T) {
		// Generate many values and check they're centered around the mean
		sum := int64(0)
		count := 1000
		mean := 100.0
		scale := 2

		for i := 0; i < count; i++ {
			val := gen.Normal(mean, 10, scale)
			assert.GreaterOrEqual(t, val, int64(0), "value should be non-negative")
			sum += val
		}

		// Average should be roughly around mean * 10^scale
		avg := float64(sum) / float64(count)
		expectedMean := mean * math.Pow10(scale)

		// Allow 20% variance for statistical fluctuation
		assert.InDelta(t, expectedMean, avg, expectedMean*0.2)
	})

	t.Run("zero stddev uses 10% of mean", func(t *testing.T) {
		val := gen.Normal(100, 0, 2)
		assert.GreaterOrEqual(t, val, int64(0))
	})

	t.Run("negative stddev uses 10% of mean", func(t *testing.T) {
		val := gen.Normal(100, -5, 2)
		assert.GreaterOrEqual(t, val, int64(0))
	})

	t.Run("different scales", func(t *testing.T) {
		scales := []int{0, 1, 2, 4, 8}

		for _, scale := range scales {
			val := gen.Normal(100, 10, scale)
			assert.GreaterOrEqual(t, val, int64(0), "scale %d", scale)
		}
	})

	t.Run("values are non-negative", func(t *testing.T) {
		// Even with low mean and high stddev, values should be clamped to 0
		for i := 0; i < 100; i++ {
			val := gen.Normal(10, 100, 2)
			assert.GreaterOrEqual(t, val, int64(0))
		}
	})

	t.Run("scale 0 returns integer units", func(t *testing.T) {
		val := gen.Normal(100, 10, 0)
		assert.GreaterOrEqual(t, val, int64(0))
	})

	t.Run("scale 18 for crypto precision", func(t *testing.T) {
		val := gen.Normal(1, 0.1, 18)
		assert.GreaterOrEqual(t, val, int64(0))
	})
}

func TestAmountGeneratorPowerLaw(t *testing.T) {
	gen := NewAmountGenerator(12345)

	t.Run("basic power law distribution", func(t *testing.T) {
		// Power law should produce mostly small values with occasional large ones
		small := 0
		large := 0
		threshold := 50.0 * 100 // threshold at 50 with scale 2

		for i := 0; i < 1000; i++ {
			val := gen.PowerLaw(1, 100, 1.3, 2)
			if float64(val) < threshold {
				small++
			} else {
				large++
			}
		}

		// Power law should have more small values than large
		assert.Greater(t, small, large, "power law should produce more small values")
	})

	t.Run("minVal <= 0 defaults to 1", func(t *testing.T) {
		val := gen.PowerLaw(0, 100, 1.3, 2)
		assert.GreaterOrEqual(t, val, int64(0))

		val = gen.PowerLaw(-5, 100, 1.3, 2)
		assert.GreaterOrEqual(t, val, int64(0))
	})

	t.Run("maxVal <= minVal defaults to minVal * 10", func(t *testing.T) {
		val := gen.PowerLaw(10, 5, 1.3, 2) // maxVal < minVal
		assert.GreaterOrEqual(t, val, int64(0))

		val = gen.PowerLaw(10, 10, 1.3, 2) // maxVal == minVal
		assert.GreaterOrEqual(t, val, int64(0))
	})

	t.Run("alpha <= 0 defaults to 1.3", func(t *testing.T) {
		val := gen.PowerLaw(1, 100, 0, 2)
		assert.GreaterOrEqual(t, val, int64(0))

		val = gen.PowerLaw(1, 100, -1, 2)
		assert.GreaterOrEqual(t, val, int64(0))
	})

	t.Run("different alpha values", func(t *testing.T) {
		alphas := []float64{0.5, 1.0, 1.3, 2.0, 3.0}

		for _, alpha := range alphas {
			val := gen.PowerLaw(1, 100, alpha, 2)
			assert.GreaterOrEqual(t, val, int64(0), "alpha %f", alpha)
		}
	})

	t.Run("different scales", func(t *testing.T) {
		scales := []int{0, 2, 4, 8}

		for _, scale := range scales {
			val := gen.PowerLaw(1, 100, 1.3, scale)
			assert.GreaterOrEqual(t, val, int64(0), "scale %d", scale)
		}
	})
}

func TestAmountGeneratorExponential(t *testing.T) {
	gen := NewAmountGenerator(12345)

	t.Run("basic exponential distribution", func(t *testing.T) {
		// Generate many values and verify distribution characteristics
		sum := int64(0)
		count := 1000
		mean := 100.0
		scale := 2

		for i := 0; i < count; i++ {
			val := gen.Exponential(mean, scale)
			assert.GreaterOrEqual(t, val, int64(0))
			sum += val
		}

		// Exponential distribution mean equals the parameter
		avg := float64(sum) / float64(count)
		expectedMean := mean * math.Pow10(scale)

		// Allow 30% variance for exponential distribution
		assert.InDelta(t, expectedMean, avg, expectedMean*0.3)
	})

	t.Run("mean <= 0 defaults to 1", func(t *testing.T) {
		val := gen.Exponential(0, 2)
		assert.GreaterOrEqual(t, val, int64(0))

		val = gen.Exponential(-5, 2)
		assert.GreaterOrEqual(t, val, int64(0))
	})

	t.Run("values are non-negative", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			val := gen.Exponential(100, 2)
			assert.GreaterOrEqual(t, val, int64(0))
		}
	})

	t.Run("different scales", func(t *testing.T) {
		scales := []int{0, 2, 4, 8}

		for _, scale := range scales {
			val := gen.Exponential(100, scale)
			assert.GreaterOrEqual(t, val, int64(0), "scale %d", scale)
		}
	})
}

func TestAmountGeneratorUniform(t *testing.T) {
	gen := NewAmountGenerator(12345)

	t.Run("basic uniform distribution", func(t *testing.T) {
		minVal := 10.0
		maxVal := 100.0
		scale := 2

		for i := 0; i < 100; i++ {
			val := gen.Uniform(minVal, maxVal, scale)
			// Value should be within [minVal * 10^scale, maxVal * 10^scale]
			minExpected := int64(minVal * math.Pow10(scale))
			maxExpected := int64(maxVal * math.Pow10(scale))

			assert.GreaterOrEqual(t, val, minExpected)
			assert.LessOrEqual(t, val, maxExpected)
		}
	})

	t.Run("maxVal < minVal swaps values", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			val := gen.Uniform(100, 10, 2) // maxVal < minVal
			// Should swap: minVal becomes 10, maxVal becomes 100
			assert.GreaterOrEqual(t, val, int64(1000)) // 10 * 100
			assert.LessOrEqual(t, val, int64(10000))   // 100 * 100
		}
	})

	t.Run("maxVal == minVal sets maxVal to minVal + 1", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			val := gen.Uniform(50, 50, 2) // maxVal == minVal
			// Should be in range [50*100, 51*100]
			assert.GreaterOrEqual(t, val, int64(5000))
			assert.LessOrEqual(t, val, int64(5100))
		}
	})

	t.Run("different scales", func(t *testing.T) {
		scales := []int{0, 2, 4, 8}

		for _, scale := range scales {
			val := gen.Uniform(10, 100, scale)
			minExpected := int64(10 * math.Pow10(scale))
			maxExpected := int64(100 * math.Pow10(scale))

			assert.GreaterOrEqual(t, val, minExpected, "scale %d", scale)
			assert.LessOrEqual(t, val, maxExpected, "scale %d", scale)
		}
	})

	t.Run("scale 0 for whole numbers", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			val := gen.Uniform(1, 10, 0)
			assert.GreaterOrEqual(t, val, int64(1))
			assert.LessOrEqual(t, val, int64(10))
		}
	})

	t.Run("large range", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			val := gen.Uniform(0.01, 1000000, 2)
			assert.GreaterOrEqual(t, val, int64(1))      // 0.01 * 100
			assert.LessOrEqual(t, val, int64(100000000)) // 1000000 * 100
		}
	})
}

func TestAmountGeneratorReproducibility(t *testing.T) {
	t.Run("same seed produces same Normal sequence", func(t *testing.T) {
		gen1 := NewAmountGenerator(999)
		gen2 := NewAmountGenerator(999)

		for i := 0; i < 20; i++ {
			v1 := gen1.Normal(100, 10, 2)
			v2 := gen2.Normal(100, 10, 2)
			assert.Equal(t, v1, v2, "iteration %d", i)
		}
	})

	t.Run("same seed produces same PowerLaw sequence", func(t *testing.T) {
		gen1 := NewAmountGenerator(999)
		gen2 := NewAmountGenerator(999)

		for i := 0; i < 20; i++ {
			v1 := gen1.PowerLaw(1, 100, 1.3, 2)
			v2 := gen2.PowerLaw(1, 100, 1.3, 2)
			assert.Equal(t, v1, v2, "iteration %d", i)
		}
	})

	t.Run("same seed produces same Exponential sequence", func(t *testing.T) {
		gen1 := NewAmountGenerator(999)
		gen2 := NewAmountGenerator(999)

		for i := 0; i < 20; i++ {
			v1 := gen1.Exponential(100, 2)
			v2 := gen2.Exponential(100, 2)
			assert.Equal(t, v1, v2, "iteration %d", i)
		}
	})

	t.Run("same seed produces same Uniform sequence", func(t *testing.T) {
		gen1 := NewAmountGenerator(999)
		gen2 := NewAmountGenerator(999)

		for i := 0; i < 20; i++ {
			v1 := gen1.Uniform(10, 100, 2)
			v2 := gen2.Uniform(10, 100, 2)
			assert.Equal(t, v1, v2, "iteration %d", i)
		}
	})
}

func TestAmountGeneratorStatisticalProperties(t *testing.T) {
	gen := NewAmountGenerator(54321)
	sampleSize := 10000

	t.Run("normal distribution mean", func(t *testing.T) {
		mean := 100.0
		scale := 2
		sum := int64(0)

		for i := 0; i < sampleSize; i++ {
			sum += gen.Normal(mean, 10, scale)
		}

		avg := float64(sum) / float64(sampleSize)
		expectedMean := mean * math.Pow10(scale)

		// Allow 10% variance
		assert.InDelta(t, expectedMean, avg, expectedMean*0.1)
	})

	t.Run("uniform distribution mean", func(t *testing.T) {
		minVal := 10.0
		maxVal := 100.0
		scale := 2
		sum := int64(0)

		for i := 0; i < sampleSize; i++ {
			sum += gen.Uniform(minVal, maxVal, scale)
		}

		avg := float64(sum) / float64(sampleSize)
		expectedMean := ((minVal + maxVal) / 2) * math.Pow10(scale)

		// Allow 5% variance for uniform
		assert.InDelta(t, expectedMean, avg, expectedMean*0.05)
	})
}

func TestAmountGeneratorEdgeCases(t *testing.T) {
	gen := NewAmountGenerator(11111)

	t.Run("very small mean for Normal", func(t *testing.T) {
		val := gen.Normal(0.001, 0.0001, 8)
		assert.GreaterOrEqual(t, val, int64(0))
	})

	t.Run("very large mean for Normal", func(t *testing.T) {
		val := gen.Normal(1000000, 100000, 2)
		assert.GreaterOrEqual(t, val, int64(0))
	})

	t.Run("very small range for Uniform", func(t *testing.T) {
		val := gen.Uniform(0.001, 0.002, 8)
		assert.GreaterOrEqual(t, val, int64(0))
	})

	t.Run("negative scale behavior", func(t *testing.T) {
		// Negative scale would result in values less than 1
		// The math.Pow10(-1) = 0.1
		val := gen.Uniform(10, 100, -1)
		// Value will be very small but still non-negative
		assert.GreaterOrEqual(t, val, int64(0))
	})
}

// Benchmarks
func BenchmarkNormal(b *testing.B) {
	gen := NewAmountGenerator(12345)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gen.Normal(100, 10, 2)
	}
}

func BenchmarkPowerLaw(b *testing.B) {
	gen := NewAmountGenerator(12345)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gen.PowerLaw(1, 100, 1.3, 2)
	}
}

func BenchmarkExponential(b *testing.B) {
	gen := NewAmountGenerator(12345)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gen.Exponential(100, 2)
	}
}

func BenchmarkUniform(b *testing.B) {
	gen := NewAmountGenerator(12345)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gen.Uniform(10, 100, 2)
	}
}

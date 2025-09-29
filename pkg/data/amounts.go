package data

import (
	"math"
	"math/rand"
	"time"
)

// AmountGenerator provides helpers to generate realistic transaction amounts
// while respecting asset scale/precision.
type AmountGenerator struct {
	r *rand.Rand
}

// NewAmountGenerator creates a generator seeded for reproducibility.
func NewAmountGenerator(seed int64) *AmountGenerator {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	// #nosec G404 - non-cryptographic PRNG is intentional here to generate
	// realistic but reproducible demo amounts. No security-sensitive use.
	return &AmountGenerator{r: rand.New(rand.NewSource(seed))}
}

// Normal generates an amount following a truncated normal distribution centered at mean
// with the given standard deviation. Returns the integer minor units (e.g., cents).
func (g *AmountGenerator) Normal(mean, stddev float64, scale int) int64 {
	if stddev <= 0 {
		stddev = mean * 0.1
	}

	val := g.r.NormFloat64()*stddev + mean
	if val < 0 {
		val = 0
	}

	factor := math.Pow10(scale)

	return int64(math.Round(val * factor))
}

// PowerLaw generates amounts where small values are common and large values are rare
// which is often seen in e-commerce transactions.
func (g *AmountGenerator) PowerLaw(minVal, maxVal float64, alpha float64, scale int) int64 {
	if minVal <= 0 {
		minVal = 1
	}

	if maxVal <= minVal {
		maxVal = minVal * 10
	}

	if alpha <= 0 {
		alpha = 1.3
	}

	u := 1 - g.r.Float64() // (0,1]
	// Use alpha in the power law formula: x = min * (1 - u * (1 - (min/max)^alpha))^(-1/alpha)
	x := minVal * math.Pow(1-u*(1-math.Pow(minVal/maxVal, alpha)), -1/alpha)
	factor := math.Pow10(scale)

	return int64(math.Round(x * factor))
}

// Exponential generates amounts following an exponential distribution with the given mean.
// Returns minor units based on the provided scale.
func (g *AmountGenerator) Exponential(mean float64, scale int) int64 {
	if mean <= 0 {
		mean = 1
	}

	val := g.r.ExpFloat64() * mean
	if val < 0 {
		val = 0
	}

	factor := math.Pow10(scale)

	return int64(math.Round(val * factor))
}

// Uniform generates amounts uniformly in [minVal, maxVal].
// Returns minor units based on the provided scale.
func (g *AmountGenerator) Uniform(minVal, maxVal float64, scale int) int64 {
	if maxVal < minVal {
		minVal, maxVal = maxVal, minVal
	}

	if maxVal == minVal {
		maxVal = minVal + 1
	}

	val := minVal + g.r.Float64()*(maxVal-minVal)
	factor := math.Pow10(scale)

	return int64(math.Round(val * factor))
}

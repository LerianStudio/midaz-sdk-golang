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
func (g *AmountGenerator) PowerLaw(min, max float64, alpha float64, scale int) int64 {
	if min <= 0 {
		min = 1
	}
	if max <= min {
		max = min * 10
	}
	if alpha <= 0 {
		alpha = 1.3
	}
	u := 1 - g.r.Float64() // (0,1]
	x := min * math.Pow((max/min), u)
	factor := math.Pow10(scale)
	return int64(math.Round(x * factor))
}

package generator

import (
	"math/rand"
	"testing"
)

// helper: convert numeric string to slice of ints, ignoring non-digits
func digitsOf(s string) []int {
	out := make([]int, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			out = append(out, int(c-'0'))
		}
	}
	return out
}

func TestGenerateCNPJ_CheckDigitsValid(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	// weights used by algorithm
	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}

	for i := 0; i < 100; i++ {
		cnpj := generateCNPJ(r, false)
		ds := digitsOf(cnpj)
		if len(ds) != 14 {
			t.Fatalf("generated CNPJ should have 14 digits, got %d (%s)", len(ds), cnpj)
		}
		d1 := cnpjCheckDigit(ds[:12], w1)
		d2 := cnpjCheckDigit(ds[:13], w2)
		if ds[12] != d1 || ds[13] != d2 {
			t.Fatalf("invalid check digits for %s: got %d%d expected %d%d", cnpj, ds[12], ds[13], d1, d2)
		}
	}
}

func TestGenerateCNPJ_FormattedPattern(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	cnpj := generateCNPJ(r, true)
	if len(cnpj) != 18 {
		t.Fatalf("formatted CNPJ should have length 18, got %d (%s)", len(cnpj), cnpj)
	}
	if cnpj[2] != '.' || cnpj[6] != '.' || cnpj[10] != '/' || cnpj[15] != '-' {
		t.Fatalf("formatted CNPJ has wrong punctuation: %s", cnpj)
	}
	// verify digits still valid
	ds := digitsOf(cnpj)
	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	d1 := cnpjCheckDigit(ds[:12], w1)
	d2 := cnpjCheckDigit(ds[:13], w2)
	if ds[12] != d1 || ds[13] != d2 {
		t.Fatalf("invalid check digits for %s: got %d%d expected %d%d", cnpj, ds[12], ds[13], d1, d2)
	}
}

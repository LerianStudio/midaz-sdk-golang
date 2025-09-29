package data

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// helper to sum percentages from a DSL string
func sumPercents(t *testing.T, dsl string) int {
	t.Helper()
	re := regexp.MustCompile(`(?m)^\s*(\d+)% to `)
	matches := re.FindAllStringSubmatch(dsl, -1)
	total := 0
	for _, m := range matches {
		v, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("failed to parse percent: %v", err)
		}
		total += v
	}
	return total
}

func TestSplitPaymentPattern_NormalizesTo100(t *testing.T) {
	dest := map[string]int{
		"@merchant_main": 50,
		"@platform_fee":  30,
	}
	p := SplitPaymentPattern("USD", 1000, dest, "idemp", "ext")
	if !strings.Contains(p.DSLTemplate, "distribute [USD 1000]") {
		t.Fatalf("unexpected DSL: %s", p.DSLTemplate)
	}
	got := sumPercents(t, p.DSLTemplate)
	if got != 100 {
		t.Fatalf("expected total 100%%, got %d%%", got)
	}
}

func TestSplitPaymentPattern_AllZeroAssignsFirstAlias(t *testing.T) {
	dest := map[string]int{
		"@b": 0,
		"@a": 0,
	}
	p := SplitPaymentPattern("USD", 500, dest, "idemp", "ext")
	got := sumPercents(t, p.DSLTemplate)
	if got != 100 {
		t.Fatalf("expected total 100%%, got %d%%", got)
	}
}

func TestSplitPaymentPattern_ClampsAndNormalizes(t *testing.T) {
	dest := map[string]int{
		"@x": -10, // clamps to 0
		"@y": 250, // clamps to 100
		"@z": 10,
	}
	p := SplitPaymentPattern("EUR", 123, dest, "idemp", "ext")
	got := sumPercents(t, p.DSLTemplate)
	if got != 100 {
		t.Fatalf("expected total 100%%, got %d%%", got)
	}
}

func TestBatchSettlementPattern_NormalizesTo100(t *testing.T) {
	dest := map[string]int{
		"@one":   25,
		"@two":   25,
		"@three": 25,
	}
	p := BatchSettlementPattern("USD", 1000, dest, "idemp", "ext")
	got := sumPercents(t, p.DSLTemplate)
	if got != 100 {
		t.Fatalf("expected total 100%%, got %d%%", got)
	}
}

func TestBatchSettlementPattern_AllZeroAssignsFirstAlias(t *testing.T) {
	dest := map[string]int{
		"@b": 0,
		"@a": 0,
	}
	p := BatchSettlementPattern("USD", 500, dest, "idemp", "ext")
	got := sumPercents(t, p.DSLTemplate)
	if got != 100 {
		t.Fatalf("expected total 100%%, got %d%%", got)
	}
}

func TestBatchSettlementPattern_ClampsAndNormalizes(t *testing.T) {
	dest := map[string]int{
		"@x": -10, // clamps to 0
		"@y": 250, // clamps to 100
		"@z": 10,
	}
	p := BatchSettlementPattern("EUR", 123, dest, "idemp", "ext")
	got := sumPercents(t, p.DSLTemplate)
	if got != 100 {
		t.Fatalf("expected total 100%%, got %d%%", got)
	}
}

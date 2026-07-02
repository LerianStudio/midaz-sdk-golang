package generator

import (
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/data"
)

// TestDSLTemplateToInput asserts the DSL-template→CreateTransactionInput
// conversion preserves the template intent: asset/amount, the single source at
// the full amount, and each destination as a percentage Share.
func TestDSLTemplateToInput(t *testing.T) {
	p := data.PaymentPattern("USD", 100, "idem-1", "ext-1")

	in, err := dslTemplateToInput(p.DSLTemplate)
	if err != nil {
		t.Fatalf("dslTemplateToInput: %v", err)
	}

	if in.AssetCode != "USD" || in.Amount != "100" {
		t.Fatalf("asset/amount = %q/%q, want USD/100", in.AssetCode, in.Amount)
	}

	if in.Send == nil || in.Send.Source == nil || len(in.Send.Source.From) != 1 {
		t.Fatalf("send/source = %+v, want 1 source", in.Send)
	}

	src := in.Send.Source.From[0]
	if src.Account != "@customer" || src.Amount.Value != "100" {
		t.Fatalf("source = %+v, want @customer/100", src)
	}

	to := in.Send.Distribute.To
	if len(to) != 2 {
		t.Fatalf("destinations = %d, want 2", len(to))
	}

	got := map[string]int64{}
	for _, d := range to {
		if d.Share == nil {
			t.Fatalf("destination %q has no Share", d.Account)
		}
		got[d.Account] = d.Share.Percentage
	}

	if got["@merchant_main"] != 97 || got["@platform_fee"] != 3 {
		t.Fatalf("shares = %v, want @merchant_main:97 @platform_fee:3", got)
	}
}

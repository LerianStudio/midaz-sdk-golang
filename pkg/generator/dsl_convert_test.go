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

// TestDSLTemplateToInput_ValidatesAndWireOmitsShareLegAmount is the money-path
// guard the generator's mocked CreateJSON tests hide: a real percentage pattern
// must (1) pass client-side CreateTransactionInput.Validate() so CreateJSON
// actually sends it, and (2) serialize each share destination WITHOUT an
// "amount" key. The Midaz /transactions/json contract rejects a distribute
// entry that carries more than one of amount/share/remaining, so an empty
// amount{} beside share is a server error (and, if silently zeroed, an
// unbalanced double-entry). The source leg must still carry the full fixed
// amount.
func TestDSLTemplateToInput_ValidatesAndWireOmitsShareLegAmount(t *testing.T) {
	p := data.PaymentPattern("USD", 100, "idem-1", "ext-1")

	in, err := dslTemplateToInput(p.DSLTemplate)
	if err != nil {
		t.Fatalf("dslTemplateToInput: %v", err)
	}

	if err := in.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil (share-only legs must be valid)", err)
	}

	wire := in.ToLibTransaction()
	send, ok := wire["send"].(map[string]any)
	if !ok {
		t.Fatalf("wire send = %T, want map", wire["send"])
	}

	// Source leg keeps the full fixed amount.
	src := send["source"].(map[string]any)["from"].([]map[string]any)
	if len(src) != 1 {
		t.Fatalf("source.from = %d entries, want 1", len(src))
	}

	srcAmount, ok := src[0]["amount"].(map[string]any)
	if !ok || srcAmount["value"] != "100" || srcAmount["asset"] != "USD" {
		t.Fatalf("source amount = %v, want {asset:USD value:100}", src[0]["amount"])
	}

	// Every share destination must carry "share" and omit "amount".
	to := send["distribute"].(map[string]any)["to"].([]map[string]any)
	if len(to) != 2 {
		t.Fatalf("distribute.to = %d entries, want 2", len(to))
	}

	for _, e := range to {
		if _, hasShare := e["share"]; !hasShare {
			t.Fatalf("distribute entry %v missing share", e["accountAlias"])
		}

		if amount, hasAmount := e["amount"]; hasAmount {
			t.Fatalf("distribute entry %v carries amount=%v; share legs must omit amount", e["accountAlias"], amount)
		}
	}
}

// TestDSLTemplateToInput_RejectsMultiAsset guards the FX money path: a template
// whose distribute clause names a different asset than its send clause (e.g.
// CurrencyExchangePattern's send [USD n] / distribute [EUR n]) must be rejected.
// The converter only ever read the send asset, so it silently produced a
// balanced USD->USD self-transfer and dropped EUR entirely — a "conversion"
// that moved nothing yet returned 201.
func TestDSLTemplateToInput_RejectsMultiAsset(t *testing.T) {
	p := data.CurrencyExchangePattern("USD", "EUR", 100, "idem-fx", "ext-fx")

	_, err := dslTemplateToInput(p.DSLTemplate)
	if err == nil {
		t.Fatal("dslTemplateToInput accepted a multi-asset template; want rejection")
	}

	want := `dsl: multi-asset templates are not supported (send asset "USD" != distribute asset "EUR")`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

// TestDSLTemplateToInput_SingleAssetWithMatchingDistributeAsset is the
// regression guard for the new multi-asset check's happy path: a template whose
// distribute clause repeats the send asset must still convert exactly as before.
func TestDSLTemplateToInput_SingleAssetWithMatchingDistributeAsset(t *testing.T) {
	p := data.FeeCollectionPattern("BRL", 100, 3, "idem-fee", "ext-fee")

	in, err := dslTemplateToInput(p.DSLTemplate)
	if err != nil {
		t.Fatalf("dslTemplateToInput: %v", err)
	}

	if in.AssetCode != "BRL" || in.Send == nil || in.Send.Asset != "BRL" {
		t.Fatalf("assetCode/send = %q/%+v, want BRL", in.AssetCode, in.Send)
	}
}

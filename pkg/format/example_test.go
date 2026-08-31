package format_test

import (
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/format"
)

// ExampleAmount converts an integer amount in minor units to its decimal string
// form. The Midaz ledger stores monetary values as int64 minor units paired
// with a scale (number of decimal places). Display always goes through Amount;
// dividing by 10^scale in calling code rounds at the boundary and silently
// loses precision for currencies like BTC (scale 8) or USDT (scale 6).
func ExampleAmount() {
	// 12345 with scale 2 represents 123.45 (e.g. $123.45 stored as cents).
	fmt.Println(format.Amount(12345, 2))

	// Negative amounts keep the sign.
	fmt.Println(format.Amount(-12345, 2))

	// Scale 0 is treated as a plain integer (no decimal point appended).
	fmt.Println(format.Amount(12345, 0))

	// When the integer part is smaller than the scale, Amount pads with a
	// leading zero — 5 cents stays "0.05", never ".05".
	fmt.Println(format.Amount(5, 2))
	// Output:
	// 123.45
	// -123.45
	// 12345
	// 0.05
}

// ExampleAmountWithOptions demonstrates locale-aware separators. The same
// int64+scale pair renders differently for pt-BR ("1.234.567,89") and en-US
// ("1,234,567.89"). Do not swap the strings in the wrong order: passing "."
// to WithThousandsSeparator and "," to WithDecimalSeparator (or vice-versa)
// silently produces output that parses fine but means a different number.
func ExampleAmountWithOptions() {
	// pt-BR locale: thousands "." and decimal ",".
	br, _ := format.AmountWithOptions(123456789, 2,
		format.WithThousandsSeparator("."),
		format.WithDecimalSeparator(","),
	)
	fmt.Println(br)

	// en-US locale: thousands "," and decimal "." (which is the default).
	us, _ := format.AmountWithOptions(123456789, 2,
		format.WithThousandsSeparator(","),
	)
	fmt.Println(us)
	// Output:
	// 1.234.567,89
	// 1,234,567.89
}

// ExampleCurrency is the canonical one-liner for rendering an operation or
// transaction amount alongside its asset code. The scale here is per-asset
// (USD: 2, BTC: 8, JPY: 0) and must match the value the ledger stored —
// passing the wrong scale shifts the decimal point.
func ExampleCurrency() {
	fmt.Println(format.Currency(12345, 2, "USD"))
	fmt.Println(format.Currency(100000000, 8, "BTC"))
	fmt.Println(format.Currency(5000, 0, "JPY"))
	// Output:
	// 123.45 USD
	// 1.00000000 BTC
	// 5000 JPY
}

// ExampleTransaction shows the heuristic the formatter uses to label a
// transaction. The label is derived from the operation accounts, not from any
// type field: any operation whose AccountAlias starts with "@external/" is
// treated as the outside world. External + internal => Deposit/Withdrawal;
// all internal => Transfer. Callers that bypass the "@external/" convention
// will get "Transaction" or the wrong label.
func ExampleTransaction() {
	transfer := &models.Transaction{
		Amount:    "100.00",
		AssetCode: "USD",
		Status:    models.Status{Code: "APPROVED"},
		Operations: []models.Operation{
			{Type: "DEBIT", AccountAlias: "savings"},
			{Type: "CREDIT", AccountAlias: "checking"},
		},
	}
	fmt.Println(format.Transaction(transfer))

	deposit := &models.Transaction{
		Amount:    "50.00",
		AssetCode: "USD",
		Status:    models.Status{Code: "APPROVED"},
		Operations: []models.Operation{
			{Type: "DEBIT", AccountAlias: "@external/USD"},
			{Type: "CREDIT", AccountAlias: "wallet"},
		},
	}
	fmt.Println(format.Transaction(deposit))
	// Output:
	// Transfer: 100.00 USD from savings to checking (Approved)
	// Deposit: 50.00 USD to wallet (Approved)
}

// ExampleParseISO handles the three ISO timestamp shapes the Midaz API may
// return: date-only ("2026-05-28"), RFC3339 ("…T15:04:05Z"), and RFC3339Nano
// (with fractional seconds). Use ParseISO whenever a payload field is typed
// as string and the schema documents it as ISO 8601 — calling time.Parse
// directly with one fixed layout will reject the other two forms.
func ExampleParseISO() {
	for _, s := range []string{
		"2026-05-28",
		"2026-05-28T15:04:05Z",
		"2026-05-28T15:04:05.123Z",
	} {
		t, err := format.ParseISO(s)
		if err != nil {
			fmt.Println("err:", err)
			continue
		}
		fmt.Println(t.UTC().Format("2006-01-02T15:04:05.000Z"))
	}
	// Output:
	// 2026-05-28T00:00:00.000Z
	// 2026-05-28T15:04:05.000Z
	// 2026-05-28T15:04:05.123Z
}

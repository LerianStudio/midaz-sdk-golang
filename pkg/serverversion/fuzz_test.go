package serverversion_test

import (
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/serverversion"
)

func FuzzParse(f *testing.F) {
	for _, tt := range parseCases {
		f.Add([]byte(tt.body))
	}

	f.Fuzz(func(t *testing.T, body []byte) {
		v := serverversion.Parse(body)
		if !v.Known && serverversion.ResolveFeeMode(v) != serverversion.FeeModeLegacy {
			t.Fatalf("unknown version %+v resolved to %q, want legacy", v, serverversion.ResolveFeeMode(v))
		}
	})
}

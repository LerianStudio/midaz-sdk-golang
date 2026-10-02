package serverversion_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/serverversion"
)

// TestParse covers the /version bodies observed in midaz v3.x, v4.0.x and
// v4.1+, plus the malformed shapes the parser must fail safe on.
func TestParse(t *testing.T) {
	tests := []struct {
		name string
		body string
		want serverversion.ServerVersion
		mode serverversion.FeeMode
	}{
		{
			name: "v4.1 buildinfo v1",
			body: `{"schemaVersion":"v1","service":"midaz-ledger","version":"4.1.3","revision":"b6506518f","buildTime":"2026-09-30T00:00:00Z","modified":false,"goVersion":"go1.27.0"}`,
			want: serverversion.ServerVersion{Raw: "4.1.3", Major: 4, Minor: 1, Patch: 3, Known: true, Source: serverversion.SourceBuildInfoV1},
			mode: serverversion.FeeModeNative,
		},
		{
			name: "v4.2 beta keeps the prerelease and stays native",
			body: `{"schemaVersion":"v1","service":"midaz-ledger","version":"4.2.0-beta.5","revision":"b6506518f","buildTime":"2026-09-30T00:00:00Z","modified":false,"goVersion":"go1.27.0"}`,
			want: serverversion.ServerVersion{Raw: "4.2.0-beta.5", Major: 4, Minor: 2, Prerelease: "beta.5", Known: true, Source: serverversion.SourceBuildInfoV1},
			mode: serverversion.FeeModeNative,
		},
		{
			name: "buildinfo v1 dev placeholder",
			body: `{"schemaVersion":"v1","service":"midaz-ledger","version":"dev"}`,
			want: serverversion.ServerVersion{Raw: "dev", Source: serverversion.SourceBuildInfoV1},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "v4.0.x legacy shape",
			body: `{"version":"v4.0.7","requestDate":"2026-09-30T12:00:00Z","commit":"abc","buildTime":"2026-09-30T00:00:00Z","dirty":false}`,
			want: serverversion.ServerVersion{Raw: "v4.0.7", Major: 4, Minor: 0, Patch: 7, Known: true, Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeNative,
		},
		{
			name: "v4.0.x without VERSION serves 0.0.0",
			body: `{"version":"0.0.0","requestDate":"2026-09-30T12:00:00Z","commit":"abc","buildTime":"2026-09-30T00:00:00Z","dirty":false}`,
			want: serverversion.ServerVersion{Raw: "0.0.0", Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "v3 legacy shape",
			body: `{"version":"v3.8.0","requestDate":"2026-09-30T12:00:00Z"}`,
			want: serverversion.ServerVersion{Raw: "v3.8.0", Major: 3, Minor: 8, Patch: 0, Known: true, Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "dev placeholder",
			body: `{"version":"dev","requestDate":"2026-09-30T12:00:00Z"}`,
			want: serverversion.ServerVersion{Raw: "dev", Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "unknown schemaVersion fails safe to unavailable",
			body: `{"schemaVersion":"v2","service":"midaz-ledger","version":"5.0.0"}`,
			want: serverversion.ServerVersion{Source: serverversion.SourceUnavailable},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "empty version",
			body: `{"version":""}`,
			want: serverversion.ServerVersion{Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "major only",
			body: `{"version":"4"}`,
			want: serverversion.ServerVersion{Raw: "4", Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "major and minor only",
			body: `{"version":"4.1"}`,
			want: serverversion.ServerVersion{Raw: "4.1", Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "non-numeric patch",
			body: `{"version":"4.1.x"}`,
			want: serverversion.ServerVersion{Raw: "4.1.x", Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "negative major",
			body: `{"version":"-1.0.0"}`,
			want: serverversion.ServerVersion{Raw: "-1.0.0", Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "empty prerelease",
			body: `{"version":"4.1.3-"}`,
			want: serverversion.ServerVersion{Raw: "4.1.3-", Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "empty build metadata",
			body: `{"version":"4.1.3+"}`,
			want: serverversion.ServerVersion{Raw: "4.1.3+", Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "build metadata is discarded",
			body: `{"version":"4.1.3+build.7"}`,
			want: serverversion.ServerVersion{Raw: "4.1.3+build.7", Major: 4, Minor: 1, Patch: 3, Known: true, Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeNative,
		},
		{
			name: "surrounding spaces are trimmed for parsing only",
			body: `{"version":" v4.1.3 "}`,
			want: serverversion.ServerVersion{Raw: " v4.1.3 ", Major: 4, Minor: 1, Patch: 3, Known: true, Source: serverversion.SourceLegacy},
			mode: serverversion.FeeModeNative,
		},
		{
			name: "non-JSON body",
			body: `404 page not found`,
			want: serverversion.ServerVersion{Source: serverversion.SourceUnavailable},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "empty object",
			body: `{}`,
			want: serverversion.ServerVersion{Source: serverversion.SourceUnavailable},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "null",
			body: `null`,
			want: serverversion.ServerVersion{Source: serverversion.SourceUnavailable},
			mode: serverversion.FeeModeLegacy,
		},
		{
			name: "array",
			body: `[]`,
			want: serverversion.ServerVersion{Source: serverversion.SourceUnavailable},
			mode: serverversion.FeeModeLegacy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serverversion.Parse([]byte(tt.body))

			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.mode, serverversion.ResolveFeeMode(got))
		})
	}
}

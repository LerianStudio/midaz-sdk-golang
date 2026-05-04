package version

import "testing"

func TestUserAgent(t *testing.T) {
	want := SDKName + "/" + Version
	if got := UserAgent(); got != want {
		t.Fatalf("UserAgent() = %q, want %q", got, want)
	}
}

func TestVersionConstantsAreSet(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "version", value: Version},
		{name: "sdk name", value: SDKName},
		{name: "sdk language", value: SDKLanguage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == "" {
				t.Fatalf("%s must not be empty", tt.name)
			}
		})
	}
}

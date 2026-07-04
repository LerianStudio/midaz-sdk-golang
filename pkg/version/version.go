// Package version provides version information for the Midaz SDK.
// This is the single source of truth for the SDK version.
package version

// Version constants for the Midaz SDK.
// These should be updated when releasing new versions.
const (
	// Version is the current version of the SDK. The release pipeline
	// (semantic-release prepare step) rewrites this string on each release; it
	// is the single source of truth for both version.Version and midaz.Version.
	Version = "4.2.0-beta.2"

	// SDKName is the name identifier for the SDK.
	SDKName = "midaz-go-sdk"

	// SDKLanguage identifies the programming language of this SDK.
	SDKLanguage = "go"
)

// UserAgent returns a formatted user agent string for HTTP requests.
func UserAgent() string {
	return SDKName + "/" + Version
}

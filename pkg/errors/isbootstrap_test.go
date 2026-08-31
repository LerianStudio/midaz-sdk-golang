package errors_test

import (
	stderrors "errors"
	"net/http"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
)

func TestIsBootstrapErrorMatchesEveryBootstrapCategory(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"config", sdkerrors.NewConfigurationError("midaz.New", "missing", nil), true},
		{"auth", sdkerrors.NewAuthenticationError("midaz.New", "401", nil), true},
		{"authz", sdkerrors.NewAuthorizationError("midaz.New", "403", nil), true},
		{"rate", sdkerrors.NewRateLimitError("midaz.New", "429", nil), true},
		{"net", sdkerrors.NewNetworkError("midaz.New", stderrors.New("dial tcp")), true},
		{"internal", sdkerrors.NewInternalError("midaz.New", stderrors.New("500")), true},
		{"validation (not bootstrap)", sdkerrors.NewValidationError("op", "bad", nil), false},
		{"notfound (not bootstrap)", sdkerrors.NewNotFoundError("op", "x", "y", nil), false},
		{"raw error (not bootstrap)", stderrors.New("oops"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sdkerrors.IsBootstrapError(tc.err); got != tc.want {
				t.Errorf("IsBootstrapError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsBootstrapErrorRejectsRuntimeHTTPFailures(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"runtime 401", sdkerrors.ErrorFromHTTPResponse(http.StatusUnauthorized, "req-401", "unauthorized", "", "", "")},
		{"runtime 429", sdkerrors.ErrorFromHTTPResponse(http.StatusTooManyRequests, "req-429", "rate limited", "", "", "")},
		{"runtime 500", sdkerrors.ErrorFromHTTPResponse(http.StatusInternalServerError, "req-500", "internal", "", "", "")},
		{"runtime 503", sdkerrors.ErrorFromHTTPResponse(http.StatusServiceUnavailable, "req-503", "unavailable", "", "", "")},
		{"non-New network", sdkerrors.NewNetworkError("entities.GetAccount", stderrors.New("dial tcp"))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if sdkerrors.IsBootstrapError(tc.err) {
				t.Fatalf("runtime error must not be classified as bootstrap: %v", tc.err)
			}
		})
	}
}

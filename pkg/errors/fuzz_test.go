package errors_test

import (
	"context"
	stderrors "errors"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

func FuzzRedactSensitiveString(f *testing.F) {
	for _, seed := range []string{
		"access_token=secret",
		"Authorization: Bearer abc.def.ghi",
		"safe message",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, value string) {
		_ = t
		_ = sdkerrors.RedactSensitiveString(value)
	})
}

func FuzzClassifyTransportError(f *testing.F) {
	for _, seed := range []string{"connection refused", "timeout", "deadline exceeded", "totally unrelated"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, message string) {
		_ = t
		_ = sdkerrors.ClassifyTransportError("fuzz", stderrors.New(message))
		_ = sdkerrors.ClassifyTransportError("fuzz", context.Canceled)
	})
}

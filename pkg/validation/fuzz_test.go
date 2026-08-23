package validation_test

import (
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/validation/core"
)

func FuzzValidateDateRange(f *testing.F) {
	now := time.Now().Unix()
	f.Add(now-86400, now)
	f.Add(now, now-86400)
	f.Add(int64(0), int64(0))

	f.Fuzz(func(t *testing.T, startUnix, endUnix int64) {
		_ = t
		start := time.Unix(startUnix, 0).UTC()
		end := time.Unix(endUnix, 0).UTC()
		_ = validation.ValidateDateRange(start, end)
		_ = core.ValidateDateRange(start, end)
	})
}

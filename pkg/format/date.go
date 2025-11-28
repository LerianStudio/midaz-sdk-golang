package format

import (
	"time"
)

// Date formatting constants
const (
	// DateFormat is the standard date format (YYYY-MM-DD)
	DateFormat = "2006-01-02"

	// DateTimeFormat is the standard date-time format (YYYY-MM-DD HH:MM:SS)
	DateTimeFormat = "2006-01-02 15:04:05"

	// TimeFormat is the standard time format (HH:MM:SS)
	TimeFormat = "15:04:05"
)

// ISO formats dates and times in ISO 8601 format with various options
// for controlling the level of detail and formatting options.
type ISO struct {
	// IncludeTime determines whether to include the time component
	IncludeTime bool

	// IncludeMilliseconds determines whether to include milliseconds
	IncludeMilliseconds bool

	// NAOnZero returns "N/A" for zero time values instead of formatting them
	NAOnZero bool
}

// ISOOption defines a function that configures an ISO formatter instance
type ISOOption func(*ISO) error

// WithIncludeTime configures whether to include the time component.
// Note that if milliseconds are already enabled, time cannot be disabled.
func WithIncludeTime(include bool) ISOOption {
	return func(f *ISO) error {
		// Can't disable time if milliseconds are enabled
		if !include && f.IncludeMilliseconds {
			// Keep time enabled when milliseconds are enabled
			return nil
		}

		f.IncludeTime = include

		return nil
	}
}

// WithIncludeMilliseconds configures whether to include milliseconds
// If milliseconds are included, time will automatically be included as well.
func WithIncludeMilliseconds(include bool) ISOOption {
	return func(f *ISO) error {
		if include {
			// Automatically enable time when milliseconds are enabled
			f.IncludeTime = true
		}

		f.IncludeMilliseconds = include

		return nil
	}
}

// WithNAOnZero configures whether to return "N/A" for zero time values
func WithNAOnZero(enabled bool) ISOOption {
	return func(f *ISO) error {
		f.NAOnZero = enabled
		return nil
	}
}

// NewISO creates a new ISO formatter instance with the given options
func NewISO(opts ...ISOOption) (*ISO, error) {
	// Start with default options
	formatter := &ISO{
		IncludeTime:         true,
		IncludeMilliseconds: false,
		NAOnZero:            true,
	}

	// Apply all provided options
	for _, opt := range opts {
		if err := opt(formatter); err != nil {
			return nil, err
		}
	}

	return formatter, nil
}

// DefaultISOFormat returns the default ISO format configuration
// For backward compatibility, this returns an ISO formatter with default settings
func DefaultISOFormat() *ISO {
	// NewISO with no options always succeeds - error check is safe to ignore
	// as the default options have no validation that can fail
	formatter, _ := NewISO() //nolint:errcheck // default options never fail
	return formatter
}

// DateOnly returns an ISO formatter configured for date-only output
// For backward compatibility, this returns an ISO formatter with date-only settings
func DateOnly() *ISO {
	// WithIncludeTime option always returns nil error
	formatter, _ := NewISO(WithIncludeTime(false)) //nolint:errcheck // option never fails
	return formatter
}

// DateTimeWithMillis returns an ISO formatter configured for date-time with milliseconds
// For backward compatibility, this returns an ISO formatter with date-time with milliseconds settings
func DateTimeWithMillis() *ISO {
	// WithIncludeMilliseconds option always returns nil error
	formatter, _ := NewISO(WithIncludeMilliseconds(true)) //nolint:errcheck // option never fails
	return formatter
}

// Format formats the time according to the formatter's configuration
func (f *ISO) Format(t time.Time) string {
	if t.IsZero() && f.NAOnZero {
		return "N/A"
	}

	if !f.IncludeTime {
		return t.Format(DateFormat)
	}

	if f.IncludeMilliseconds {
		return t.Format(time.RFC3339Nano)
	}

	return t.Format(time.RFC3339)
}

// ParseISO parses a string in ISO format (YYYY-MM-DD or YYYY-MM-DDThh:mm:ssZ)
// and returns a time.Time value.
func ParseISO(s string) (time.Time, error) {
	// First try RFC3339 format (with time)
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}

	// Then try RFC3339Nano format
	t, err = time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return t, nil
	}

	// Finally try date-only format
	return time.Parse(DateFormat, s)
}

// Backward compatibility aliases - deprecated, use the shorter names instead

// FormatISO is deprecated, use ISO instead.
type FormatISO = ISO //nolint:revive // backward compatibility alias

// FormatISOOption is deprecated, use ISOOption instead.
type FormatISOOption = ISOOption //nolint:revive // backward compatibility alias

// NewFormatISO is deprecated, use NewISO instead.
var NewFormatISO = NewISO

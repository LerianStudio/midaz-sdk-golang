package validation

import (
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation/adapters"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation/core"
)

// ValidatorProvider returns a ValidatorProvider option for the SDK client.
// This option configures the validation system to use a specific validator implementation.
//
// Usage:
//
//	// Use the default lib-commons based validator (default behavior)
//	client, err := client.New(validation.WithValidatorProvider(nil))
//
//	// Use a custom validator
//	client, err := client.New(validation.WithValidatorProvider(myCustomValidator))
//
// This option can be used with client.New() to configure validation behavior:
//
//	import (
//	    "github.com/LerianStudio/midaz-sdk-golang/pkg/validation"
//	    "github.com/LerianStudio/midaz-sdk-golang/pkg/validation/adapters"
//	)
//
//	// Create a client with a custom validator
//	client, err := client.New(
//	    validation.WithValidatorProvider(adapters.NewLibCommonsValidator()),
//	    // ... other options
//	)
func WithValidatorProvider(provider core.ValidatorProvider) func(interface{}) error {
	return func(i interface{}) error {
		if configurable, ok := i.(interface{ SetValidatorProvider(core.ValidatorProvider) }); ok {
			// If no provider specified, use default lib-commons validator
			if provider == nil {
				provider = adapters.NewLibCommonsValidator()
			}

			configurable.SetValidatorProvider(provider)
		}
		return nil
	}
}

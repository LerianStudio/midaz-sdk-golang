# Midaz Go SDK Examples

This guide provides brief examples of common operations with the Midaz Go SDK.

## Client Initialization

```go
import (
	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/config"
)

// Create a client with default options
c, err := client.New(
	client.WithPluginAccessManager("your-auth-token"),
	client.WithEnvironment(config.EnvironmentProduction),
	client.UseAllAPIs(),
)
if err != nil {
	// Handle error
}
```

## Working with Organizations

```go
// Create an organization
org, err := c.Entity.Organizations.CreateOrganization(ctx, &models.CreateOrganizationInput{
	LegalName:       "Example Organization",
	LegalDocument:   "123456789",
	DoingBusinessAs: "Example",
})

// Get an organization
org, err := c.Entity.Organizations.GetOrganization(ctx, "org-id")

// List organizations
orgs, err := c.Entity.Organizations.ListOrganizations(ctx, nil)
```

## Using Pagination

```go
// Create a paginator for accounts
paginator := c.Entity.Accounts.GetAccountPaginator(ctx, "org-id", "ledger-id", &models.ListOptions{
	Limit: 10,
})

// Iterate through all pages
for paginator.HasNext() {
	accounts, err := paginator.Next()
	if err != nil {
		// Handle error
	}
	
	for _, account := range accounts.Items {
		// Process each account
	}
}
```

For more detailed examples, refer to the `examples` directory in the SDK repository.

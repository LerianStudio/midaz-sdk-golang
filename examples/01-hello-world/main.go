// Package main is the smallest possible Midaz SDK demo: build a client,
// list organizations, print them. No retries, no observability, no auth
// (assumes a local Midaz stack with auth disabled). 25 lines of body.
//
// Usage:
//
//	go run ./examples/01-hello-world
//
// For a production-shaped client (Access Manager, retries, logging),
// see examples/02-auth and examples/configuration.
package main

import (
	"context"
	"fmt"
	"log"

	midaz "github.com/LerianStudio/midaz-sdk-golang/v5"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
)

func main() {
	c, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
		midaz.WithAnonymous(),
	)
	if err != nil {
		log.Fatalf("midaz.New: %v", err)
	}
	defer func() {
		if err := c.Shutdown(context.Background()); err != nil {
			log.Printf("client shutdown: %v", err)
		}
	}()

	page, err := c.Organizations.List(context.Background(), models.OrganizationsListOpts{
		PageListOpts: models.PageListOpts{Limit: 5},
	})
	if err != nil {
		log.Fatalf("ListOrganizations: %v", err)
	}
	for _, org := range page.Items {
		fmt.Printf("- %s (%s)\n", org.LegalName, org.ID)
	}
}

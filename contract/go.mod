// Nested module: isolates the Midaz server dependency (and its transitive
// graph) from the SDK's published go.mod. See drift_test.go for rationale.
module github.com/LerianStudio/midaz-sdk-golang/v3/contract

go 1.26.3

require (
	github.com/LerianStudio/midaz-sdk-golang/v3 v3.0.0
	github.com/LerianStudio/midaz/v3 v3.7.5
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/LerianStudio/lib-observability v1.0.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/LerianStudio/midaz-sdk-golang/v3 => ../

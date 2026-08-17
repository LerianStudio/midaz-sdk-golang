package models

import (
	"encoding/json"
	"errors"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation"
)

// CreateInflowInput represents input for creating an inflow transaction.
// Inflow transactions have no source; funds flow into destination accounts.
type CreateInflowInput struct {
	// ChartOfAccountsGroupName for accounting purposes
	ChartOfAccountsGroupName string `json:"chartOfAccountsGroupName,omitempty"`

	// Description provides a human-readable explanation
	Description string `json:"description,omitempty"`

	// Code is a human-readable reference label for display and reporting.
	// It is not a query handle — see CreateTransactionInput.Code.
	Code string `json:"code,omitempty"`

	// Metadata contains custom key-value data
	Metadata map[string]any `json:"metadata,omitempty"`

	// Route is the transaction route identifier
	Route string `json:"route,omitempty"`

	// RouteID is the UUID transaction route identifier.
	RouteID string `json:"routeId,omitempty"`

	// TransactionDate is the effective date/time for the transaction.
	TransactionDate string `json:"transactionDate,omitempty"`

	// Send contains the asset, value, and distribution details
	Send *SendInflowInput `json:"send"`
}

// SendInflowInput represents the send details for an inflow transaction.
type SendInflowInput struct {
	// Asset is the asset code being transferred
	Asset string `json:"asset"`

	// Value is the exact decimal amount being transferred.
	Value any `json:"value"`

	// Distribute contains the destination accounts
	Distribute *DistributeInput `json:"distribute"`
}

// NewCreateInflowInput creates a new CreateInflowInput with the required fields.
func NewCreateInflowInput(asset string, value any, distribute *DistributeInput) *CreateInflowInput {
	return &CreateInflowInput{
		Send: &SendInflowInput{
			Asset:      asset,
			Value:      decimalStringFromAny(value),
			Distribute: distribute,
		},
	}
}

// WithDescription sets the description.
func (input *CreateInflowInput) WithDescription(description string) *CreateInflowInput {
	if input == nil {
		return nil
	}

	input.Description = description

	return input
}

// WithCode sets the code.
func (input *CreateInflowInput) WithCode(code string) *CreateInflowInput {
	if input == nil {
		return nil
	}

	input.Code = code

	return input
}

// WithMetadata sets the metadata.
func (input *CreateInflowInput) WithMetadata(metadata map[string]any) *CreateInflowInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithChartOfAccountsGroupName sets the chart of accounts group name.
func (input *CreateInflowInput) WithChartOfAccountsGroupName(name string) *CreateInflowInput {
	if input == nil {
		return nil
	}

	input.ChartOfAccountsGroupName = name

	return input
}

// WithRoute sets the route.
func (input *CreateInflowInput) WithRoute(route string) *CreateInflowInput {
	if input == nil {
		return nil
	}

	input.Route = route

	return input
}

// WithRouteID sets the route UUID.
func (input *CreateInflowInput) WithRouteID(routeID string) *CreateInflowInput {
	if input == nil {
		return nil
	}

	input.RouteID = routeID

	return input
}

// WithTransactionDate sets the transaction effective date/time.
func (input *CreateInflowInput) WithTransactionDate(transactionDate string) *CreateInflowInput {
	if input == nil {
		return nil
	}

	input.TransactionDate = transactionDate

	return input
}

// Validate checks that the CreateInflowInput meets all validation requirements.
func (input *CreateInflowInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.Send == nil {
		return errors.New("send is required")
	}

	var errs validation.FieldErrors

	appendTransactionCreateCommon(&errs, input.Description, input.Code, input.Metadata,
		input.Route, input.RouteID, input.TransactionDate, false)

	if input.Send.Asset == "" {
		errs.Append("asset", "is required")
	}

	if err := validatePositiveDecimalString(input.Send.Value, "value"); err != nil {
		errs.Append("value", err.Error())
	}

	switch {
	case input.Send.Distribute == nil || len(input.Send.Distribute.To) == 0:
		errs.Append("distribute.to", "is required")
	default:
		if err := input.Send.Distribute.Validate(); err != nil {
			errs.Append("distribute", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// ToMap converts a CreateInflowInput to a map for API requests.
func (input *CreateInflowInput) ToMap() map[string]any {
	if input == nil {
		return nil
	}

	tx := transactionCommonMap(input.ChartOfAccountsGroupName, input.Description, input.Code, input.Metadata, input.Route, input.RouteID, input.TransactionDate, false)
	if input.Send != nil {
		send := map[string]any{
			"asset": input.Send.Asset,
			"value": decimalStringFromAny(input.Send.Value),
		}

		if input.Send.Distribute != nil {
			send["distribute"] = input.Send.Distribute.ToMap()
		}

		tx["send"] = send
	}

	return tx
}

// MarshalJSON emits the /transactions/inflow request body (ToMap), keeping
// json.Marshal(input) identical to what the SDK puts on the wire.
func (input CreateInflowInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(input.ToMap())
}

// CreateOutflowInput represents input for creating an outflow transaction.
// Outflow transactions have no destination - funds flow out of the system (e.g., withdrawals, payouts).
type CreateOutflowInput struct {
	// ChartOfAccountsGroupName for accounting purposes
	ChartOfAccountsGroupName string `json:"chartOfAccountsGroupName,omitempty"`

	// Description provides a human-readable explanation
	Description string `json:"description,omitempty"`

	// Code is a human-readable reference label for display and reporting.
	// It is not a query handle — see CreateTransactionInput.Code.
	Code string `json:"code,omitempty"`

	// Metadata contains custom key-value data
	Metadata map[string]any `json:"metadata,omitempty"`

	// Route is the transaction route identifier
	Route string `json:"route,omitempty"`

	// RouteID is the UUID transaction route identifier.
	RouteID string `json:"routeId,omitempty"`

	// Pending indicates whether the transaction should be created in a pending state.
	Pending bool `json:"pending,omitempty"`

	// TransactionDate is the effective date/time for the transaction.
	TransactionDate string `json:"transactionDate,omitempty"`

	// Send contains the asset, value, and source details
	Send *SendOutflowInput `json:"send"`
}

// SendOutflowInput represents the send details for an outflow transaction.
type SendOutflowInput struct {
	// Asset is the asset code being transferred
	Asset string `json:"asset"`

	// Value is the exact decimal amount being transferred.
	Value any `json:"value"`

	// Source contains the source accounts
	Source *SourceInput `json:"source"`
}

// NewCreateOutflowInput creates a new CreateOutflowInput with the required fields.
func NewCreateOutflowInput(asset string, value any, source *SourceInput) *CreateOutflowInput {
	return &CreateOutflowInput{
		Send: &SendOutflowInput{
			Asset:  asset,
			Value:  decimalStringFromAny(value),
			Source: source,
		},
	}
}

// WithDescription sets the description.
func (input *CreateOutflowInput) WithDescription(description string) *CreateOutflowInput {
	if input == nil {
		return nil
	}

	input.Description = description

	return input
}

// WithCode sets the code.
func (input *CreateOutflowInput) WithCode(code string) *CreateOutflowInput {
	if input == nil {
		return nil
	}

	input.Code = code

	return input
}

// WithMetadata sets the metadata.
func (input *CreateOutflowInput) WithMetadata(metadata map[string]any) *CreateOutflowInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithChartOfAccountsGroupName sets the chart of accounts group name.
func (input *CreateOutflowInput) WithChartOfAccountsGroupName(name string) *CreateOutflowInput {
	if input == nil {
		return nil
	}

	input.ChartOfAccountsGroupName = name

	return input
}

// WithRoute sets the route.
func (input *CreateOutflowInput) WithRoute(route string) *CreateOutflowInput {
	if input == nil {
		return nil
	}

	input.Route = route

	return input
}

// WithRouteID sets the route UUID.
func (input *CreateOutflowInput) WithRouteID(routeID string) *CreateOutflowInput {
	if input == nil {
		return nil
	}

	input.RouteID = routeID

	return input
}

// WithTransactionDate sets the transaction effective date/time.
func (input *CreateOutflowInput) WithTransactionDate(transactionDate string) *CreateOutflowInput {
	if input == nil {
		return nil
	}

	input.TransactionDate = transactionDate

	return input
}

// Validate checks that the CreateOutflowInput meets all validation requirements.
func (input *CreateOutflowInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if input.Send == nil {
		return errors.New("send is required")
	}

	var errs validation.FieldErrors

	appendTransactionCreateCommon(&errs, input.Description, input.Code, input.Metadata,
		input.Route, input.RouteID, input.TransactionDate, input.Pending)

	if input.Send.Asset == "" {
		errs.Append("asset", "is required")
	}

	if err := validatePositiveDecimalString(input.Send.Value, "value"); err != nil {
		errs.Append("value", err.Error())
	}

	switch {
	case input.Send.Source == nil || len(input.Send.Source.From) == 0:
		errs.Append("source.from", "is required")
	default:
		if err := input.Send.Source.Validate(); err != nil {
			errs.Append("source", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// ToMap converts a CreateOutflowInput to a map for API requests.
func (input *CreateOutflowInput) ToMap() map[string]any {
	if input == nil {
		return nil
	}

	tx := transactionCommonMap(input.ChartOfAccountsGroupName, input.Description, input.Code, input.Metadata, input.Route, input.RouteID, input.TransactionDate, input.Pending)
	if input.Send != nil {
		send := map[string]any{
			"asset": input.Send.Asset,
			"value": decimalStringFromAny(input.Send.Value),
		}

		if input.Send.Source != nil {
			send["source"] = input.Send.Source.ToMap()
		}

		tx["send"] = send
	}

	return tx
}

// MarshalJSON emits the /transactions/outflow request body (ToMap), keeping
// json.Marshal(input) identical to what the SDK puts on the wire.
func (input CreateOutflowInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(input.ToMap())
}

// CreateAnnotationInput is the payload for creating an annotation transaction.
type CreateAnnotationInput struct {
	ChartOfAccountsGroupName string         `json:"chartOfAccountsGroupName,omitempty"`
	Description              string         `json:"description,omitempty"`
	Pending                  bool           `json:"pending,omitempty"`
	Code                     string         `json:"code,omitempty"`
	Route                    string         `json:"route,omitempty"`
	RouteID                  string         `json:"routeId,omitempty"`
	TransactionDate          string         `json:"transactionDate,omitempty"`
	Metadata                 map[string]any `json:"metadata,omitempty"`
	Send                     *SendInput     `json:"send,omitempty"`
}

// NewCreateAnnotationInput creates a new CreateAnnotationInput.
func NewCreateAnnotationInput(description string, send ...*SendInput) *CreateAnnotationInput {
	var sendInput *SendInput
	if len(send) > 0 {
		sendInput = send[0]
	}

	return &CreateAnnotationInput{
		Description: description,
		Send:        sendInput,
	}
}

// Validate checks that the CreateAnnotationInput meets all validation requirements.
func (input *CreateAnnotationInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	appendTransactionCreateCommon(&errs, input.Description, input.Code, input.Metadata,
		input.Route, input.RouteID, input.TransactionDate, input.Pending)

	if input.Send != nil {
		if err := input.Send.Validate(); err != nil {
			errs.Append("send", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// ToLibTransaction converts a CreateAnnotationInput to the backend transaction payload.
func (input *CreateAnnotationInput) ToLibTransaction() map[string]any {
	if input == nil {
		return nil
	}

	tx := transactionCommonMap(input.ChartOfAccountsGroupName, input.Description, input.Code, input.Metadata, input.Route, input.RouteID, input.TransactionDate, input.Pending)
	if input.Send != nil {
		tx["send"] = input.Send.ToMap()
	}

	return tx
}

// MarshalJSON emits the /transactions/annotation request body
// (ToLibTransaction), keeping json.Marshal(input) identical to the wire.
func (input CreateAnnotationInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(input.ToLibTransaction())
}

// WithCode sets the annotation transaction code.
func (input *CreateAnnotationInput) WithCode(code string) *CreateAnnotationInput {
	if input == nil {
		return nil
	}

	input.Code = code

	return input
}

// WithMetadata sets annotation transaction metadata.
func (input *CreateAnnotationInput) WithMetadata(metadata map[string]any) *CreateAnnotationInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// WithCode sets the transaction code.
func (input *CreateTransactionInput) WithCode(code string) *CreateTransactionInput {
	if input == nil {
		return nil
	}

	input.Code = code

	return input
}

// WithPending sets the pending flag.
func (input *CreateOutflowInput) WithPending(pending bool) *CreateOutflowInput {
	if input == nil {
		return nil
	}

	input.Pending = pending

	return input
}

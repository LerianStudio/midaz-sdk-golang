// Package errors provides the canonical error system for the Midaz Go SDK.
//
// Every error returned by this SDK that originates from SDK code is a
// *[Error]. Errors that bubble up from the standard library (context
// cancellations, network failures, JSON parse errors) are classified and
// wrapped into a *[Error] at the transport boundary so callers always
// observe a uniform shape.
//
// # The canonical Error type
//
// A single struct, [Error], carries everything callers need to react:
//
//   - [Category] — broad classification (validation, not_found, network, ...)
//   - [Code]     — specific machine-readable code (insufficient_balance, ...)
//   - Operation  — SDK call site (e.g. "accounts.Create")
//   - Resource   — entity type involved (e.g. "account")
//   - ResourceID — concrete ID, populated when the call site has it
//   - Fields     — server-reported field list for validation errors
//   - StatusCode + RequestID + APICode + Title + EntityType + Details — server context
//   - Err        — underlying cause, walkable via [errors.Unwrap]
//
// # Classification taxonomy
//
// The SDK uses 11 categories. Each error has exactly one [Category]
// and at least one [Code]:
//
//	┌────────────────────┬───────────────────────────────┬──────────────┐
//	│ Category           │ Typical Codes                 │ Retryable?   │
//	├────────────────────┼───────────────────────────────┼──────────────┤
//	│ validation         │ validation_error              │ no           │
//	│                    │ asset_mismatch                │              │
//	│                    │ account_eligibility_error     │              │
//	│ not_found          │ not_found                     │ no           │
//	│ conflict           │ already_exists                │ no           │
//	│                    │ idempotency_error             │              │
//	│ auth               │ authentication_error          │ refresh+yes  │
//	│                    │ permission_error              │ no           │
//	│ limit_exceeded     │ rate_limit_exceeded           │ yes (backoff)│
//	│ timeout            │ timeout                       │ yes          │
//	│ cancellation       │ cancelled                     │ no           │
//	│ network            │ network_error                 │ yes          │
//	│ unprocessable      │ unprocessable_error           │ depends      │
//	│                    │ insufficient_balance          │              │
//	│ internal           │ internal_error                │ yes          │
//	│ configuration      │ configuration_error           │ no (fatal)   │
//	└────────────────────┴───────────────────────────────┴──────────────┘
//
// # Recognising an error
//
// Three idioms cover every use case:
//
// 1. Match by category (broad — any not-found, regardless of specific code):
//
//	if errors.Is(err, errors.ErrNotFound) {
//	    // any not_found, e.g. account not found OR ledger not found
//	}
//
// 2. Match by code (specific — only this exact failure):
//
//	if errors.Is(err, errors.ErrInsufficientBalance) {
//	    // unprocessable + insufficient_balance specifically
//	}
//
// 3. Extract the typed error for full context (request ID, status code, fields):
//
//	var sdkErr *errors.Error
//	if errors.As(err, &sdkErr) {
//	    log.Printf("op=%s status=%d req=%s api=%s",
//	        sdkErr.Operation,
//	        sdkErr.StatusCode,
//	        sdkErr.RequestID,
//	        sdkErr.APICode)
//	}
//
// 4. Walk client-side validation field errors:
//
//	var fe *validation.FieldErrors
//	if errors.As(err, &fe) {
//	    for _, item := range fe.Errs() {
//	        log.Printf("field=%s message=%s", item.Field, item.Message)
//	    }
//	}
//
// # Predicate functions
//
// For each category there is an Is*Error predicate that combines the
// type assertion and category check into one call:
//
//	errors.IsNotFoundError(err)        // any not_found
//	errors.IsAuthError(err)            // any authentication or authorization failure
//	errors.IsValidationError(err)      // any client-side or server-side validation failure
//	errors.IsRateLimitError(err)       // server is throttling
//	errors.IsTimeoutError(err)         // request deadline exceeded
//	errors.IsCancellationError(err)    // context cancelled by caller
//	errors.IsNetworkError(err)         // DNS, conn-refused, TLS, broken pipe — pre-response
//	errors.IsConflictError(err)        // 409 — already exists or idempotency conflict
//	errors.IsUnprocessableError(err)   // 422 — domain rule violation
//	errors.IsInternalError(err)        // 5xx
//	errors.IsConfigurationError(err)   // SDK setup mistake (eager, from client.New)
//
// The predicates are nil-safe: passing a nil error returns false.
//
// # Authoring SDK errors (contributors)
//
// Every public service method follows this shape:
//
//	const operation = "accounts.Create"
//
//	if input == nil {
//	    return nil, errors.NewMissingParameterError(operation, "input")
//	}
//	if err := input.Validate(); err != nil {
//	    return nil, errors.NewValidationError(operation, "invalid input", err)
//	}
//
//	// Transport call. The transport layer populates Operation,
//	// Resource, ResourceID, StatusCode, RequestID, APICode automatically.
//	acc, err := s.transport.Do(ctx, ...)
//	if err != nil {
//	    return nil, err  // already a fully-populated *Error
//	}
//	return acc, nil
//
// # Sensitive data redaction
//
// All [Error.Error] output passes through a redactor that strips Bearer
// tokens, idempotency keys, document numbers, and other sensitive
// substrings. See [RedactSensitiveString] for the exposed primitive
// applications can use to redact arbitrary log lines.
//
// # See also
//
//   - [Error]                      — the canonical error type
//   - [NewValidationError]         — construct a validation error
//   - [NewNotFoundError]           — construct a not-found error
//   - [NewNetworkError]            — wrap a transport-layer failure
//   - [NewConfigurationError]      — eager SDK setup error
//   - [ErrorFromHTTPResponse]      — server-response → *Error
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation.FieldErrors]
//     — multi-field validation accumulator
package errors

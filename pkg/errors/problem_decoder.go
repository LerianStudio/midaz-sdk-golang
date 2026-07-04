// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package errors

import (
	"encoding/json"
	"fmt"
)

// problemEnvelope mirrors the unified RFC 9457 (application/problem+json)
// error body emitted by both server planes (ledger and tracer). It is a
// local shape on purpose: the decoder speaks the wire format directly so
// pkg/errors never imports the generated internal/gen* types, keeping the
// generated surface out of the public SDK error path. All fields are
// optional pointers on the wire; the decoder nil-guards every one.
type problemEnvelope struct {
	Code     *string               `json:"code"`
	Detail   *string               `json:"detail"`
	Errors   *[]problemErrorDetail `json:"errors"`
	Instance *string               `json:"instance"`
	Status   *int64                `json:"status"`
	Title    *string               `json:"title"`
	Type     *string               `json:"type"`
}

// problemErrorDetail mirrors a single RFC 9457 field error.
type problemErrorDetail struct {
	Location *string `json:"location"`
	Message  *string `json:"message"`
	Value    *any    `json:"value"`
}

// DecodeProblemJSON decodes a unified RFC 9457 problem+json error body into
// an *Error. It accepts the raw response bytes (the wire format the two
// server planes share) and the transport-observed HTTP status/request ID.
//
// Mapping: Code→APICode, Detail→Message, Title→Title, Status→StatusCode
// (envelope status preferred, transport status as fallback), and
// Errors[]→field-errors (Location into Fields, {location,message,value}
// per entry into Details["errors"]).
//
// Retryability is not decided here: it is a property of the resulting
// *Error, keyed by the shared status→category adapter and the Code-suffix
// override (see applyAPICodeMapping). The returned error is always an
// *Error; a missing, empty, or non-JSON body degrades to a status-only
// error so the transport layer still sees uniform, correctly-classified
// shape.
func DecodeProblemJSON(httpStatus int, body []byte, requestID string) error {
	if len(body) == 0 {
		return ErrorFromHTTPResponse(httpStatus, requestID, "Empty response from server", "", "", "")
	}

	var env problemEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		message := fmt.Sprintf("API returned non-JSON error response with status code %d and body length %d", httpStatus, len(body))

		return ErrorFromHTTPResponse(httpStatus, requestID, message, "", "", "")
	}

	statusCode := httpStatus
	if env.Status != nil && *env.Status > 0 {
		statusCode = int(*env.Status)
	}

	message := deref(env.Detail)
	if message == "" {
		message = fmt.Sprintf("API error with status code %d", statusCode)
	}

	fields, details := decodeProblemFieldErrors(env.Errors)

	return ErrorFromHTTPResponseWithDetails(
		statusCode,
		requestID,
		message,
		deref(env.Code),
		"", // entityType: not part of the RFC 9457 envelope
		"", // resourceID: not part of the RFC 9457 envelope
		deref(env.Title),
		fields,
		details,
	)
}

// decodeProblemFieldErrors flattens RFC 9457 Errors[] into the SDK's
// field-errors: locations become Fields, and the full per-entry shape is
// preserved under Details["errors"] for programmatic consumers. Redaction
// is applied downstream by ErrorFromHTTPResponseWithDetails.
func decodeProblemFieldErrors(errs *[]problemErrorDetail) (fields []string, details map[string]any) {
	if errs == nil || len(*errs) == 0 {
		return nil, nil
	}

	entries := make([]any, 0, len(*errs))

	for _, e := range *errs {
		if loc := deref(e.Location); loc != "" {
			fields = append(fields, loc)
		}

		entry := map[string]any{}
		if loc := deref(e.Location); loc != "" {
			entry["location"] = loc
		}
		if msg := deref(e.Message); msg != "" {
			entry["message"] = msg
		}
		if e.Value != nil {
			entry["value"] = *e.Value
		}

		entries = append(entries, entry)
	}

	return fields, map[string]any{"errors": entries}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

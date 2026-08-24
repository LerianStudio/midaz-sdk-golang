// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package errors

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

// problemEnvelope mirrors the unified RFC 9457 (application/problem+json)
// error body emitted by both server planes (ledger and tracer). It is a
// local shape on purpose: the decoder speaks the wire format directly so
// pkg/errors never imports the generated internal/gen* types, keeping the
// generated surface out of the public SDK error path. All fields are
// optional pointers on the wire; the decoder nil-guards every one.
// The envelope also carries the two members that exist ONLY on the legacy /v1
// shape (LegacyError: application/json with code/message/title/entityType/fields).
// Both server surfaces are alive — /v1 is deprecated, not gone — so one decoder
// reads both: an /v1 error body has no "detail" and no "errors", and its message
// and per-field detail live under "message" and "fields" instead.
type problemEnvelope struct {
	Code     *string               `json:"code"`
	Detail   *string               `json:"detail"`
	Errors   *[]problemErrorDetail `json:"errors"`
	Instance *string               `json:"instance"`
	Status   *int64                `json:"status"`
	Title    *string               `json:"title"`
	Type     *string               `json:"type"`

	// Message is the /v1 counterpart of Detail.
	Message *string `json:"message"`

	// EntityType is the /v1 domain entity the error concerns; the RFC 9457
	// envelope has no equivalent member.
	EntityType *string `json:"entityType"`

	// Fields is the /v1 counterpart of Errors: per-field violation detail keyed by
	// field name. Values are not always strings (an unexpected field carries the
	// offending value), hence map[string]any.
	Fields *map[string]any `json:"fields"`
}

// problemErrorDetail mirrors a single RFC 9457 field error.
type problemErrorDetail struct {
	Location *string `json:"location"`
	Message  *string `json:"message"`
	Value    *any    `json:"value"`
}

// DecodeProblemJSON decodes a server error body into an *Error. It accepts the
// raw response bytes and the transport-observed HTTP status/request ID, and reads
// BOTH live wire formats: the RFC 9457 problem+json envelope (/v2 and the Tracer
// plane) and the legacy /v1 shape (application/json with
// code/message/title/entityType/fields).
//
// Mapping: Code→APICode, Detail→Message, Title→Title, Status→StatusCode
// (envelope status preferred, transport status as fallback), and
// Errors[]→field-errors (Location into Fields, {location,message,value}
// per entry into Details["errors"]).
//
// When the RFC 9457 members are absent the legacy members take over:
// Message→Message and Fields→field-errors (keys into Fields, the whole map into
// Details["fields"]), plus EntityType. Without this the whole /v1 surface reported
// nothing but "API error with status code N" and dropped every field violation.
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

	// "detail" is the RFC 9457 member; "message" is its /v1 counterpart. Prefer
	// the RFC one so a body carrying both (a server mid-migration) reads as /v2.
	message := deref(env.Detail)
	if message == "" {
		message = deref(env.Message)
	}

	if message == "" {
		message = fmt.Sprintf("API error with status code %d", statusCode)
	}

	fields, details := decodeProblemFieldErrors(env.Errors)
	if fields == nil && details == nil {
		fields, details = decodeLegacyFieldErrors(env.Fields)
	}

	return ErrorFromHTTPResponseWithDetails(
		statusCode,
		requestID,
		message,
		deref(env.Code),
		deref(env.EntityType), // /v1 only; absent from the RFC 9457 envelope
		"",                    // resourceID: carried by neither envelope
		deref(env.Title),
		fields,
		details,
	)
}

// decodeLegacyFieldErrors flattens the /v1 "fields" object into the SDK's
// field-errors: the keys become Fields (the same slot RFC 9457 locations land in,
// so consumers read one shape), and the full map is preserved under
// Details["fields"] because a value is not always a string — for an unexpected
// field it is the offending value. Keys are sorted so Fields is deterministic.
// Redaction is applied downstream by ErrorFromHTTPResponseWithDetails.
func decodeLegacyFieldErrors(legacy *map[string]any) (fields []string, details map[string]any) {
	if legacy == nil || len(*legacy) == 0 {
		return nil, nil
	}

	fields = slices.Sorted(maps.Keys(*legacy))

	return fields, map[string]any{"fields": *legacy}
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

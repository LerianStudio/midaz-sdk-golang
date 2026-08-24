// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"iter"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// The response helpers in this file read a generated operation's RAW result —
// the (*http.Response, error) pair — instead of its generated Parse*Resp
// wrapper.
//
// That parser is not a neutral convenience. It picks what to unmarshal from the
// response Content-Type and the single status the OAS declares, and it returns
// (nil, err) whenever that guess does not fit the bytes that actually arrived.
// Three shapes make it guess wrong, and each turns a perfectly readable response
// into an SDK-internal fault:
//
//   - A bodiless 204 carrying "Content-Type: application/json". Midaz's own
//     delete handlers emit no content type, but any gateway or proxy in front of
//     one may add it, and then EVERY delete in the SDK reports "unexpected end of
//     JSON input" — a successful delete, reported as a failure.
//   - An error reply with an empty body (a HEAD count, a proxy-generated 502):
//     the real status is lost behind the same unmarshal failure.
//   - A success body whose id is not a UUID. The generated models parse ids into
//     openapi_types.UUID, so the unmarshal fails and a DECODE problem surfaces as
//     errors.NewInternalError — the "the SDK is broken" class — rather than as a
//     decode error naming the status the server sent.
//
// Every facade already decodes the body into a models.* type of its own, so the
// generated parser contributes nothing but those failure modes. readCount
// (transactions_facade.go) bypassed it for exactly this reason on the count
// endpoints; these helpers do the same for the delete, single-object, list and
// bare-array shapes, which is every remaining response shape the ledger serves.

// deleteResource maps a DELETE response to success or a unified error.
//
// A 2xx is success regardless of what the response carries: a delete has nothing
// to return, so an empty body, a body, and a content type that disagrees with
// either are all the same outcome. Only a non-2xx is decoded, through the shared
// RFC 9457 mapper, which degrades an empty body to a status-only error.
func deleteResource(operation string, resp *http.Response, err error) error {
	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	httpResp, body, err := readRawResponse(resp, err)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if !isSuccess(httpResp.StatusCode) {
		return errors.DecodeProblemJSON(httpResp.StatusCode, body, requestIDOf(httpResp))
	}

	return nil
}

// isEmptyBody reports whether a 2xx body carries no resource: an empty body,
// the JSON literal "null", or the empty object "{}".
//
// All three unmarshal into a ZERO-VALUED T with a nil error — json.Unmarshal on
// "null" is a documented no-op and "{}" sets nothing — so without this predicate
// a caller who branches on err != nil reads a settled transfer with id "" and
// status "" as a success. See decodeOne, which refuses all three centrally.
//
// "{}" is refused alongside the bodiless shapes because no single-object
// response this SDK reads is legitimately empty: every resource carries at least
// an id, and the one id-less single-object read — a ledger's settings — declares
// accounting, tracer and overrides all REQUIRED in the ledger contract
// (components/ledger/api/openapi.huma.yaml, schema LedgerSettings), so "{}" is
// not a valid body there either.
func isEmptyBody(body []byte) bool {
	trimmed := bytes.TrimSpace(body)

	return len(trimmed) == 0 ||
		bytes.Equal(trimmed, []byte("null")) ||
		bytes.Equal(trimmed, []byte("{}"))
}

// readOne maps a single-object response into the public model.
func readOne[T any](operation string, resp *http.Response, err error) (*T, error) {
	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	httpResp, body, err := readRawResponse(resp, err)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[T](operation, httpResp.StatusCode, body, httpResp)
}

// guardListBody applies the two refusals EVERY list read makes, whichever
// envelope it goes on to decode.
//
// A non-2xx decodes the unified RFC 9457 envelope. A 2xx carrying no page is
// refused: "null" and "{}" both decode into a zero-valued envelope with a nil
// error, which reads as an EMPTY PAGE with no next cursor, so a caller iterating
// a ledger's transactions stops on the first dropped body and concludes the
// ledger is empty. The class is a RESPONSE DECODE error, not an internal one —
// the server answered and the read cannot be trusted, which is a different fact
// from "the SDK is broken".
//
// No legitimate list body is any of these shapes: every list envelope the ledger
// declares (Pagination, TransactionV2ListBody) marks items and limit REQUIRED,
// so an empty page is {"items":[],"limit":N} and never {} or null. The tracer
// plane's named-field envelopes ({"rules":[...],"nextCursor":""}) are the same:
// the field is always present. The bare-ARRAY reads go through readSlice, not
// here, and that distinction matters — see the note there.
func guardListBody(operation string, status int, body []byte, httpResp *http.Response) error {
	if !isSuccess(status) {
		return errors.DecodeProblemJSON(status, body, requestIDOf(httpResp))
	}

	if isEmptyBody(body) {
		return errors.NewResponseDecodeError(operation, status, errEmptySuccessBody)
	}

	return nil
}

// readList maps a paginated envelope into models.ListResponse[T].
//
// This is the single decode behind EVERY list on both surfaces. It reads the raw
// response rather than the generated *WithResponse parser for the reason the
// file header gives, and the list surface proves it: that parser unmarshals the
// body itself whenever the content type says json, so a 200 whose body a gateway
// dropped fails INSIDE it and surfaces as an internal error — "the SDK is
// broken" — before any guard here runs. Reading raw puts every 2xx shape in
// front of guardListBody.
//
// The four tracer-plane lists decode a differently-named envelope of their own
// ({"rules":[...],"nextCursor":""}) and call guardListBody directly, which is the
// same pair of refusals without the shared envelope type.
func readList[T any](operation string, resp *http.Response, err error) (*models.ListResponse[T], error) {
	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	httpResp, body, err := readRawResponse(resp, err)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if err := guardListBody(operation, httpResp.StatusCode, body, httpResp); err != nil {
		return nil, err
	}

	var page models.ListResponse[T]
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, errors.NewResponseDecodeError(operation, httpResp.StatusCode, err)
	}

	if page.Items == nil {
		page.Items = []T{}
	}

	return &page, nil
}

// readSlice maps a bare JSON array response (the point-in-time balance reads and
// the metadata-index list answer with one, not with a paginated envelope).
//
// Deliberately WITHOUT the empty-body refusal decodeOne and readList apply, and
// the exception is narrow rather than an oversight. Those two read objects,
// where "null" cannot be a real answer. This reads an ARRAY, and Go's
// encoding/json marshals a nil slice as the literal "null" — so a handler
// returning an empty result set emits exactly the shape the guard would refuse,
// and adding it here would turn "no balances at that instant" into an error. An
// empty body still fails, in the unmarshal, as it always did.
func readSlice[T any](operation string, resp *http.Response, err error) ([]T, error) {
	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	httpResp, body, err := readRawResponse(resp, err)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if !isSuccess(httpResp.StatusCode) {
		return nil, errors.DecodeProblemJSON(httpResp.StatusCode, body, requestIDOf(httpResp))
	}

	var out []T
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, errors.NewResponseDecodeError(operation, httpResp.StatusCode, err)
	}

	return out, nil
}

// pageSeq drives a PAGE-numbered list, handing each page to the caller and
// advancing the page number while the response reports more results.
//
// page returns a pointer to the opts' page field, which is how one helper drives
// a dozen differently-typed option structs without any of them implementing an
// interface. Callers pass a two-line closure: func(o *X) *int { return &o.Page }.
func pageSeq[O, T any](
	ctx context.Context,
	opts O,
	page func(*O) *int,
	fetch func(O) (*models.ListResponse[T], error),
) iter.Seq2[*models.ListResponse[T], error] {
	return func(yield func(*models.ListResponse[T], error) bool) {
		current := opts
		if *page(&current) == 0 {
			*page(&current) = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			result, err := fetch(current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(result, nil) {
				return
			}

			if !result.Pagination.HasMore() {
				return
			}

			*page(&current)++
		}
	}
}

// cursorSeq drives a CURSOR-paginated list, echoing each response's next_cursor
// into the following request.
//
// The stop condition is an EMPTY next_cursor, deliberately not
// Pagination.HasMore(): HasMore's page-based branch can report true on a full
// terminal page that carries a page field but no cursor, which would reset the
// cursor to "" and re-request the first page forever. That is not hypothetical —
// it is the unbounded balance-iterator loop Epic 2 found and fixed.
//
// cursor returns a pointer to the opts' cursor field, for the same reason
// pageSeq takes a pointer to the page field.
func cursorSeq[O, T any](
	ctx context.Context,
	opts O,
	cursor func(*O) *string,
	fetch func(O) (*models.ListResponse[T], error),
) iter.Seq2[*models.ListResponse[T], error] {
	return func(yield func(*models.ListResponse[T], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			result, err := fetch(current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(result, nil) {
				return
			}

			if result.Pagination.NextCursor == "" {
				return
			}

			*cursor(&current) = result.Pagination.NextCursor
		}
	}
}

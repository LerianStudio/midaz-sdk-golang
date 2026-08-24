// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
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

// readOne maps a single-object response into the public model.
func readOne[T any](operation string, resp *http.Response, err error) (*T, error) {
	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	httpResp, body, err := readRawResponse(resp, err)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	return decodeOne[T](operation, httpResp.StatusCode, body, httpResp)
}

// readList maps a paginated envelope into models.ListResponse[T].
func readList[T any](operation string, resp *http.Response, err error) (*models.ListResponse[T], error) {
	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	httpResp, body, err := readRawResponse(resp, err)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if !isSuccess(httpResp.StatusCode) {
		return nil, errors.DecodeProblemJSON(httpResp.StatusCode, body, requestIDOf(httpResp))
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

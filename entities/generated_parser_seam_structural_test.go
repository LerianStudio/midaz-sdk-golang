// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNoFacadeCallsAGeneratedParser is the structural invariant behind every
// response-reading fix this SDK has made: the facade layer reads the RAW
// (*http.Response, error) pair and never the generated *WithResponse parser.
//
// # Why the parser is banned rather than merely discouraged
//
// oapi-codegen's Parse*Resp picks what to unmarshal from the response
// Content-Type and the single status the OAS declares, and it returns (nil,
// err) whenever that guess does not fit the bytes that arrived. It runs BEFORE
// any facade logic, so the facade can only report the parser's failure — and
// the parser's failure is always the wrong fact:
//
//   - A gateway 403 or 404 carrying an EMPTY body fails inside the unmarshal,
//     so "you are not allowed" and "it is not there" both arrive as
//     internal_error/500. The status the caller needs to choose a next action is
//     destroyed.
//   - An unreadable 2xx becomes internal_error ("the SDK is broken") instead of
//     a response-decode error ("the server answered and the answer is
//     unreadable") — which on a write is the difference between retrying and
//     reconciling.
//   - A 2xx whose id is not a UUID fails the same way: the generated models
//     parse ids into openapi_types.UUID, so a server-side id-format change
//     reports as an SDK-internal fault.
//
// See the file header of facade_responses.go for the same three, and readOne /
// readList / readSlice / deleteResource for the helpers that replace it.
//
// # Why this scan exists at all, and in this shape
//
// The retrofit that emptied this surface was enumerated FOUR times by hand
// across Epic 3, and each count missed a site: the v1 lists, the tracer
// envelopes, then a fifth readSlice caller that a mutation found still on the
// banned parser with the whole suite green. Counting is not enumerating. This
// scan is the enumeration, and it is mechanical: a site added tomorrow is
// covered when it lands rather than when someone remembers it.
//
// It matches on the OPERATION, never on the syntax around it. Every earlier
// scan in this package that asked about the shape of a call — the receiver's
// field name, whether the selector was the thing being called, whether the
// declaration was a function at all — was walked past by a compiling facade
// someone wrote to prove the point. So this one inspects EVERY selector in the
// whole file, package-level declarations included, and asks one question: does
// this name a generated operation's parser spelling?
//
// # The parser has TWO spellings, and banning one banned nothing
//
// The first version of this scan refused only the *WithResponse METHOD. But
// oapi-codegen exports the same parser as a free function beside it —
// ParseGetSegmentByIDResp, 197 of them in genledger and 28 in gentracer — and
// GetSegmentByIDWithResponse is nothing more than the raw call followed by that
// function. A facade making the raw call and handing the response to
// ParseGetSegmentByIDResp reproduced the banned behaviour byte for byte and this
// scan reported zero offenders; it was written as a compiling facade and
// confirmed to pass before the match was widened. Both spellings are now
// refused, through the shared isParserSpelling, so the delete-seam scan that
// makes the same distinction moves with it.
//
// # Scope, and the tripwire under it
//
// This scan reads THIS directory. That is sound only while entities/ is the
// module's sole importer of the generated clients — a helper elsewhere taking a
// *genledger.ClientWithResponses would call the parser with this scan green.
// The premise is asserted rather than assumed, by
// TestEntitiesIsTheSoleImporterOfTheGeneratedClients below.
//
// # The sibling scan depends on this one
//
// The path-id scan (TestEveryPathParameterOperationIsGuarded) resolves path
// argument positions from the raw *Client signatures, so a call reached through
// any other spelling has no resolvable positions there. It reports such a call
// rather than crediting it, but keeping those spellings out of the package
// altogether is THIS scan's job, and that is the direction the dependency runs.
//
// There is no exemption list, because no site needs one. A site that genuinely
// cannot route through a shared helper belongs here, named, with the reason it
// cannot — never skipped silently, and never by loosening the match.
//
// Known ceiling, shared with the two sibling scans: reflection is out of AST
// reach. A client method resolved by name at runtime is invisible to a scan of
// this kind. Nothing in this package does that, and this scan does not pretend
// to cover it.
func TestNoFacadeCallsAGeneratedParser(t *testing.T) {
	fset := token.NewFileSet()

	ops := generatedOperations(t, fset)
	require.NotEmpty(t, ops, "found no generated operations; the scan is broken, not the code")

	var offenders []string

	rawOps := map[string]bool{}

	for path, file := range parseGoFiles(t, fset, ".") {
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			op, ok := generatedOperation(sel.Sel.Name)
			if !ok || !ops[op] {
				return true
			}

			if !isParserSpelling(sel.Sel.Name) {
				rawOps[op] = true
				return true
			}

			offenders = append(offenders, filepath.Base(path)+":"+
				strconv.Itoa(fset.Position(sel.Pos()).Line)+" names "+sel.Sel.Name)

			return true
		})
	}

	sort.Strings(offenders)

	t.Logf("%d generated operations reached through the raw call; %d parser call sites remain",
		len(rawOps), len(offenders))

	require.Empty(t, offenders,
		"the facade layer must read the raw response through readOne/readList/readSlice/deleteResource; "+
			"the generated parser loses the server's status on an empty body and misreports an "+
			"unreadable one as an SDK fault:\n  %s",
		strings.Join(offenders, "\n  "))

	// A floor on the POSITIVE half, so a scan that stops matching selectors
	// cannot read as success: the emptiness above is only meaningful while the
	// same matching still finds the raw calls, and an assertion that nothing
	// matched passes trivially once the matching itself breaks.
	//
	// It runs AFTER the emptiness check on purpose. A facade reverted to the
	// parser lowers this count too, and letting the floor fire first would
	// report a regression in a facade as "the scan went blind" — the wrong fact,
	// and one that sends the reader to the wrong file.
	require.GreaterOrEqual(t, len(rawOps), rawOperationFloor,
		"expected at least %d generated operations reached through the raw call; found %d, "+
			"so the scan stopped seeing them", rawOperationFloor, len(rawOps))
}

// rawOperationFloor is the number of distinct generated operations the facade
// layer reaches through the raw call today, across both planes.
//
// 181 before this retrofit, 225 after — the 45 parser sites carried 44
// operations the raw half had not already reached. The one already there is
// UpdateOperation, which /v1 spells twice: V1.Transactions reached it raw and
// V1.Operations reached it through the parser, so the same endpoint answered
// differently depending on which spelling the caller used. Losing a facade
// lowers this count; a scan
// that stops matching selectors drops it to zero, which is the case this floor
// exists to catch, because an emptiness assertion passes trivially once the
// matching breaks.
const rawOperationFloor = 225

// TestEntitiesIsTheSoleImporterOfTheGeneratedClients pins the premise the three
// structural scans in this package rest on.
//
// Each of them reads "." — this directory — and that scope is sound only while
// entities/ is the module's ONLY importer of internal/genledger and
// internal/gentracer. A helper in another package taking a
// *genledger.ClientWithResponses can call the banned parser, forward an
// unguarded id into a URL path and re-open the delete seam with all three scans
// green: not because they are weak, but because they are not looking there. That
// was written as a compiling helper and confirmed to pass all three.
//
// Widening every scan to the whole module would be the thorough fix, and it buys
// nothing today, because there is one importer. This is the cheaper half and the
// honest one: the day a second importer appears, this test says so instead of
// the scans going quietly narrow. Relaxing this assertion is therefore never the
// fix — widening the scans is.
func TestEntitiesIsTheSoleImporterOfTheGeneratedClients(t *testing.T) {
	require.Equal(t, []string{"entities"}, generatedClientImporters(t, ".."),
		"the structural scans in this package only read entities/; a second importer of the generated "+
			"clients needs those scans widened to reach it, not this assertion relaxed")
}

// generatedClientImporters walks the module and returns the directories, relative
// to root, holding a Go file that imports either generated client.
//
// Imports are PARSED rather than grepped. The text "internal/genledger" appears
// in doc comments, plan records, mapping docs and scripts all over this repo,
// and none of those is an importer.
func generatedClientImporters(t *testing.T, root string) []string {
	t.Helper()

	dirs := map[string]bool{}

	walk := func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir():
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor") {
				return filepath.SkipDir
			}

			return nil
		case !strings.HasSuffix(path, ".go") || !importsAGeneratedClient(t, path):
			return nil
		}

		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}

		dirs[rel] = true

		return nil
	}

	require.NoError(t, filepath.WalkDir(root, walk))

	return sortedKeys(dirs)
}

// importsAGeneratedClient reports whether one Go file imports either generated
// client package.
func importsAGeneratedClient(t *testing.T, path string) bool {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	require.NoError(t, err)

	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		require.NoError(t, err)

		if strings.HasSuffix(imported, "/internal/genledger") ||
			strings.HasSuffix(imported, "/internal/gentracer") {
			return true
		}
	}

	return false
}

// generatedOperations reads both generated clients and returns every operation
// they expose, keyed by name.
//
// Derived from the request builders — oapi-codegen emits exactly one
// New<Op>Request per operation — for the same reason the sibling scans do it:
// the set of names that count as an operation is then the generated code's
// answer, not a list someone maintains.
func generatedOperations(t *testing.T, fset *token.FileSet) map[string]bool {
	t.Helper()

	ops := map[string]bool{}

	for _, dir := range []string{"../internal/genledger", "../internal/gentracer"} {
		for _, file := range parseGoFiles(t, fset, dir) {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Recv != nil {
					continue
				}

				if op, ok := requestBuilderOperation(fn.Name.Name); ok {
					ops[op] = true
				}
			}
		}
	}

	return ops
}

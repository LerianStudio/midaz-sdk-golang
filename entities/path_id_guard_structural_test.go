package entities

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// transitionHelpers are the package-local helpers that take a generated client
// method as a function VALUE and call requirePathIDs on the caller's behalf. A
// facade method that delegates to one of these never names the generated
// operation in a call of its own, so the structural check credits the
// delegation instead.
//
// The list is deliberately literal rather than derived from a call graph. A
// call graph would credit any local helper that happens to guard somewhere,
// which turns "this method is guarded" into "something this method touches
// guards something". Adding a fourth helper here should be a decision someone
// makes on purpose.
var transitionHelpers = map[string]bool{
	"ruleTransition":        true,
	"limitTransition":       true,
	"reservationTransition": true,
}

// TestEveryPathParameterOperationIsGuarded is the structural invariant behind
// the empty/dot-segment path id guard.
//
// The table-driven guard tests sample one call per facade family, which proves
// the family is wired up but says nothing about the other methods in it:
// deleting the guard from a method no row happens to name leaves the suite
// green. This test closes that gap by checking the property directly, on every
// method at once.
//
// The rule it enforces: if a facade function calls a generated client operation
// whose request builder formats a value into the URL PATH, then EVERY value it
// forwards into that path must be one the function handed to requirePathIDs.
//
// # Why identity, and not the presence of the call
//
// An earlier version asked only whether requirePathIDs appeared somewhere in the
// function. That accepted a call naming NO pairs at all — requirePathIDs(op)
// returns nil on an empty list — so a facade could satisfy the scan with a guard
// that checked nothing, and every by-id read behind it would be back to the
// silent-zero and scope-escalation defects two fix rounds were spent closing.
// There were no live instances; it was a hole in the proof rather than a defect
// in the product, which is exactly the kind of hole that stops being theoretical
// the first time someone edits a facade to make a test stop complaining.
//
// The check now compares expressions. The path ARGUMENT POSITIONS come from the
// generated code — the request builder says which of its parameters it styles
// into the path, and the client method's signature says where those parameters
// sit in the argument list — so a facade forwarding orgID into a path segment
// must have named orgID to the guard. Guarding a different variable, or none,
// fails.
//
// Every input is read out of the source at test time rather than from a
// checked-in list, so a newly generated operation or a newly written facade
// method is covered the moment it lands, without anyone remembering to add it.
func TestEveryPathParameterOperationIsGuarded(t *testing.T) {
	fset := token.NewFileSet()

	pathOps := operationsWithPathParameters(t, fset)
	require.NotEmpty(t, pathOps, "found no generated operations with path parameters; the scan is broken, not the code")

	scan := pathGuardScan{
		fset:         fset,
		pathOps:      pathOps,
		pathArgs:     pathArgumentNames(t, fset),
		methodParams: clientMethodParameters(t, fset),
	}

	var unguarded []string

	guarded := 0
	helperGuards := map[string]bool{}

	for name, file := range parseGoFiles(t, fset, ".") {
		for _, decl := range file.Decls {
			problem, credited := scan.classify(name, decl, helperGuards)
			if problem != "" {
				unguarded = append(unguarded, problem)
			}

			guarded += credited
		}
	}

	// A transition helper takes the generated method as a PARAMETER, so its own
	// body never names the operation and the loop above cannot see it. Its
	// callers are credited purely on the promise that it guards, so that promise
	// has to be checked here or deleting the guard from the helper silently
	// unguards every method that delegates to it.
	for name := range transitionHelpers {
		require.True(t, helperGuards[name],
			"%s is credited as a path-id guard for the facade methods that delegate to it, so it must "+
				"hand every id it forwards to the generated call to requirePathIDs; found=%v, guards=%v",
			name, helperGuards[name], helperGuards)
	}

	sort.Strings(unguarded)

	require.Empty(t, unguarded,
		"every facade function that formats a caller value into a URL path must reject bad path ids "+
			"locally, via requirePathIDs or a transition helper:\n  %s",
		strings.Join(unguarded, "\n  "))

	t.Logf("guard verified on %d facade functions across %d generated operations with path parameters",
		guarded, len(pathOps))

	// A floor, so the scan silently matching nothing cannot read as success.
	require.Greater(t, guarded, 100,
		"expected the guard on more than 100 facade functions; found %d, so the scan stopped seeing them", guarded)
}

// pathGuardScan carries the three derived inputs the per-declaration check
// needs, so the check is one method with one argument rather than a loop body
// nested inside the test function. Flattening it that way is also what keeps the
// test under the cognitive-complexity limit, which the COLD linter has caught in
// this package once per fix round.
type pathGuardScan struct {
	fset         *token.FileSet
	pathOps      map[string]bool
	pathArgs     map[string]map[string]bool
	methodParams map[string][]string
}

// classify judges one top-level declaration, returning the offence to report (or
// "") and 1 when the declaration is a facade function whose path ids are
// verifiably guarded.
//
// It also records a transition helper's own verdict into helperGuards, because
// the helper is the one shape whose guard cannot be judged from the call sites
// that depend on it.
func (s pathGuardScan) classify(path string, decl ast.Decl, helperGuards map[string]bool) (string, int) {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok || fn.Body == nil {
		return s.packageScopeBinding(path, decl), 0
	}

	if transitionHelpers[fn.Name.Name] {
		helperGuards[fn.Name.Name] = helperGuardsWhatItForwards(fn)
	}

	ops := pathOperationsNamedBy(fn.Body, s.pathOps)
	if len(ops) == 0 {
		return "", 0
	}

	site := " (declared at " + filepath.Base(path) + ":" +
		strconv.Itoa(s.fset.Position(fn.Pos()).Line) + ")"

	if len(directPathCalls(fn, s.pathOps)) > 0 {
		missing := unguardedPathArguments(fn, s.pathOps, s.pathArgs, s.methodParams)
		if len(missing) > 0 {
			return funcLabel(fn) + " forwards " + strings.Join(missing, ", ") +
				" into a URL path without handing it to requirePathIDs" + site, 0
		}

		return "", 1
	}

	// The operation is NAMED but not called: the function hands it to a
	// transition helper as a function value, so there are no call arguments here
	// to compare. The helper's own guard is checked by the caller.
	if guardsPathIDs(fn) {
		return "", 1
	}

	return funcLabel(fn) + " reaches " + strings.Join(ops, ", ") + site, 0
}

// packageScopeBinding reports a path-parameter operation bound OUTSIDE any
// function body.
//
// A package-level var, const or type that binds one leaves every caller naming a
// bare identifier, which no scan of function bodies can attribute back to the
// operation — so the guard requirement evaporates silently. See
// packageScopeEscape (delete_seam_structural_test.go) for the two shapes this
// was proven against and for the reflection ceiling both scans share.
func (s pathGuardScan) packageScopeBinding(path string, decl ast.Decl) string {
	ops := pathOperationsNamedBy(decl, s.pathOps)
	if len(ops) == 0 {
		return ""
	}

	return "the package-level declaration in " + filepath.Base(path) + ":" +
		strconv.Itoa(s.fset.Position(decl.Pos()).Line) + " binds " + strings.Join(ops, ", ") +
		", which no caller can then be checked for a guard"
}

// directPathCalls returns the calls in fn that INVOKE a generated
// path-parameter operation, as opposed to merely naming one.
//
// The distinction is what decides which check applies: an invoked operation has
// argument expressions to compare against the guard, while an operation handed
// to a transition helper as a function value has none.
func directPathCalls(fn *ast.FuncDecl, pathOps map[string]bool) []*ast.CallExpr {
	var calls []*ast.CallExpr

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if op, ok := generatedOperation(sel.Sel.Name); ok && pathOps[op] {
			calls = append(calls, call)
		}

		return true
	})

	return calls
}

// unguardedPathArguments returns the expressions fn forwards into a URL path
// that it did not hand to requirePathIDs.
//
// This is the identity check. For each generated operation the function calls,
// it takes the argument positions the generated client formats into the path and
// compares those expressions, as source text, against the values the guard
// received. An empty result means every path segment the caller can influence
// was checked before the request was built.
func unguardedPathArguments(
	fn *ast.FuncDecl,
	pathOps map[string]bool,
	pathArgs map[string]map[string]bool,
	methodParams map[string][]string,
) []string {
	checked := guardedValues(fn)

	var missing []string

	for _, call := range directPathCalls(fn, pathOps) {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		op, _ := generatedOperation(sel.Sel.Name)

		for _, arg := range pathArgumentsOf(call, sel.Sel.Name, op, pathArgs, methodParams) {
			if !checked[arg] {
				missing = append(missing, arg+" (into "+op+")")
			}
		}
	}

	sort.Strings(missing)

	return missing
}

// pathArgumentsOf returns the argument expressions of one call that the
// generated client styles into the URL path.
//
// method is the exact spelling at the call site (GetAssetByID,
// CreateAssetWithBody, ...) because the argument LIST differs between an
// operation's spellings, while op is the canonical name the request builder —
// and therefore the set of path parameter names — belongs to.
func pathArgumentsOf(
	call *ast.CallExpr,
	method, op string,
	pathArgs map[string]map[string]bool,
	methodParams map[string][]string,
) []string {
	names, wanted := methodParams[method], pathArgs[op]
	if len(names) == 0 || len(wanted) == 0 {
		return nil
	}

	var args []string

	for i, name := range names {
		if !wanted[name] || i >= len(call.Args) {
			continue
		}

		args = append(args, types.ExprString(call.Args[i]))
	}

	return args
}

// guardedValues returns the VALUE half of every requirePathIDs pair in fn, as
// source text. requirePathIDs(operation, "orgID", orgID, "id", id) contributes
// orgID and id — the names are labels for the error message and prove nothing
// about what was checked.
func guardedValues(fn *ast.FuncDecl) map[string]bool {
	values := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if ident, ok := call.Fun.(*ast.Ident); !ok || ident.Name != "requirePathIDs" {
			return true
		}

		// args[0] is the operation label; the pairs start at 1, values at 2.
		for i := 2; i < len(call.Args); i += 2 {
			values[types.ExprString(call.Args[i])] = true
		}

		return true
	})

	return values
}

// helperGuardsWhatItForwards reports whether a transition helper hands every id
// it forwards to the generated method to requirePathIDs.
//
// A transition helper receives the generated method as a function-typed
// parameter and calls it as a plain identifier, so the path positions cannot be
// resolved from the generated signatures the way a direct call's can. What can
// be checked is that nothing reaches that call unguarded: every plain identifier
// after the context must appear among the guarded values.
//
// Checking only that requirePathIDs is CALLED would leave the helpers open to
// the vacuous guard this scan exists to reject — and the helpers are the more
// expensive place to leave it open, because thirty-odd facade methods are
// credited on their promise rather than on a guard of their own.
func helperGuardsWhatItForwards(fn *ast.FuncDecl) bool {
	callParam := functionTypedParameter(fn)
	if callParam == "" {
		return false
	}

	checked := guardedValues(fn)
	guarded := true

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if ident, ok := call.Fun.(*ast.Ident); !ok || ident.Name != callParam {
			return true
		}

		// Skip the context at position 0; a variadic editor call is not an
		// identifier and drops out with everything else that is not one.
		for _, arg := range call.Args[1:] {
			if ident, ok := arg.(*ast.Ident); ok && !checked[ident.Name] {
				guarded = false
			}
		}

		return true
	})

	return guarded
}

// functionTypedParameter returns the name of fn's function-typed parameter — the
// generated method a transition helper is handed — or "" when it has none.
func functionTypedParameter(fn *ast.FuncDecl) string {
	if fn.Type.Params == nil {
		return ""
	}

	for _, field := range fn.Type.Params.List {
		if _, ok := field.Type.(*ast.FuncType); !ok || len(field.Names) == 0 {
			continue
		}

		return field.Names[0].Name
	}

	return ""
}

// pathArgumentNames reads the generated request builders and returns, per
// operation, the names of the builder parameters styled into the URL path.
//
// The identifier is taken rather than the wire name beside it: oapi-codegen
// writes StyleParamWithLocation("simple", false, "organization_id",
// runtime.ParamLocationPath, organizationId), and it is organizationId — the Go
// parameter — that lines up with the client method's signature.
func pathArgumentNames(t *testing.T, fset *token.FileSet) map[string]map[string]bool {
	t.Helper()

	args := map[string]map[string]bool{}

	for _, dir := range []string{"../internal/genledger", "../internal/gentracer"} {
		for _, file := range parseGoFiles(t, fset, dir) {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Recv != nil {
					continue
				}

				op, ok := requestBuilderOperation(fn.Name.Name)
				if !ok {
					continue
				}

				if names := styledPathParameters(fn); len(names) > 0 {
					args[op] = names
				}
			}
		}
	}

	return args
}

// styledPathParameters returns the identifiers one request builder styles into
// the URL path.
func styledPathParameters(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 5 {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "StyleParamWithLocation" {
			return true
		}

		location, ok := call.Args[3].(*ast.SelectorExpr)
		if !ok || location.Sel.Name != "ParamLocationPath" {
			return true
		}

		if ident, ok := call.Args[4].(*ast.Ident); ok {
			names[ident.Name] = true
		}

		return true
	})

	return names
}

// clientMethodParameters returns the ordered parameter names of every generated
// *Client method, keyed by the exact method name.
//
// Exact rather than canonical: GetAssetByID and CreateAssetWithBody belong to
// different operations' argument lists, and a params struct or a content type
// shifts everything after it, so the positions have to come from the spelling
// the facade actually called.
func clientMethodParameters(t *testing.T, fset *token.FileSet) map[string][]string {
	t.Helper()

	params := map[string][]string{}

	for _, dir := range []string{"../internal/genledger", "../internal/gentracer"} {
		for _, file := range parseGoFiles(t, fset, dir) {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || fn.Type.Params == nil {
					continue
				}

				if !isGeneratedClientReceiver(fn.Recv) {
					continue
				}

				params[fn.Name.Name] = parameterNames(fn.Type.Params)
			}
		}
	}

	return params
}

// isGeneratedClientReceiver reports whether the method hangs off *Client, the
// generated raw client. ClientWithResponses methods are the parser spellings and
// no facade may call them (see TestNoFacadeCallsAGeneratedParser).
func isGeneratedClientReceiver(recv *ast.FieldList) bool {
	if len(recv.List) == 0 {
		return false
	}

	typ := recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}

	ident, ok := typ.(*ast.Ident)

	return ok && ident.Name == "Client"
}

// parameterNames flattens a signature into one name per argument position, so a
// grouped "organizationId, ledgerId string" still yields two positions.
func parameterNames(params *ast.FieldList) []string {
	var names []string

	for _, field := range params.List {
		if len(field.Names) == 0 {
			names = append(names, "")
			continue
		}

		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}

	return names
}

// operationsWithPathParameters reads the generated clients and returns the
// operations whose request builder formats at least one value into the URL
// path. oapi-codegen marks exactly those with runtime.ParamLocationPath.
func operationsWithPathParameters(t *testing.T, fset *token.FileSet) map[string]bool {
	t.Helper()

	ops := map[string]bool{}

	for _, dir := range []string{"../internal/genledger", "../internal/gentracer"} {
		for _, file := range parseGoFiles(t, fset, dir) {
			collectPathOperations(file, ops)
		}
	}

	return ops
}

// collectPathOperations records every request builder in one generated file
// that formats a value into the URL path.
func collectPathOperations(file *ast.File, ops map[string]bool) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Recv != nil {
			continue
		}

		op, ok := requestBuilderOperation(fn.Name.Name)
		if !ok {
			continue
		}

		if formatsAPathParameter(fn) {
			ops[op] = true
		}
	}
}

// parseGoFiles parses the non-test Go files in dir, keyed by path.
//
// This is what go/parser's ParseDir used to do before it was deprecated, minus
// the package grouping these scans never needed: both callers want a flat list
// of files, and dropping the package level takes a loop out of each of them.
func parseGoFiles(t *testing.T, fset *token.FileSet, dir string) map[string]*ast.File {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	files := map[string]*ast.File{}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		path := filepath.Join(dir, name)

		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)

		files[path] = file
	}

	require.NotEmpty(t, files, "no non-test Go files found in %s; the scan is broken, not the code", dir)

	return files
}

// requestBuilderOperation maps a generated request-builder name back to its
// operation: NewGetBalanceByIDRequest and NewCreateAccountRequestWithBody both
// belong to the operation between the "New" and "Request".
func requestBuilderOperation(name string) (string, bool) {
	if !strings.HasPrefix(name, "New") {
		return "", false
	}

	trimmed := strings.TrimSuffix(name, "WithBody")
	if !strings.HasSuffix(trimmed, "Request") {
		return "", false
	}

	op := strings.TrimSuffix(strings.TrimPrefix(trimmed, "New"), "Request")

	return op, op != ""
}

// formatsAPathParameter reports whether a generated request builder styles a
// value into the URL path.
func formatsAPathParameter(fn *ast.FuncDecl) bool {
	found := false

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "ParamLocationPath" {
			return true
		}

		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "runtime" {
			found = true
			return false
		}

		return true
	})

	return found
}

// pathOperationsNamedBy returns the path-parameter operations a node names,
// whether it CALLS the generated method or passes it as a function value to a
// transition helper. Both reach the same request builder, so both count.
//
// It takes any ast.Node — a function body, or a whole package-level declaration
// — so the same matching covers both scopes.
func pathOperationsNamedBy(node ast.Node, pathOps map[string]bool) []string {
	seen := map[string]bool{}

	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if op, ok := generatedOperation(sel.Sel.Name); ok && pathOps[op] {
			seen[op] = true
		}

		return true
	})

	ops := make([]string, 0, len(seen))
	for op := range seen {
		ops = append(ops, op)
	}

	sort.Strings(ops)

	return ops
}

// generatedOperation strips the suffixes oapi-codegen appends to the four
// spellings it emits per operation: Op, OpWithBody, OpWithResponse and
// OpWithBodyWithResponse.
func generatedOperation(name string) (string, bool) {
	op := strings.TrimSuffix(name, "WithResponse")
	op = strings.TrimSuffix(op, "WithBody")

	return op, op != ""
}

// guardsPathIDs reports whether a facade function refuses bad path ids before
// the request leaves — directly, or by delegating to a transition helper.
func guardsPathIDs(fn *ast.FuncDecl) bool {
	return callsAny(fn, func(name string) bool {
		return name == "requirePathIDs" || transitionHelpers[name]
	})
}

// callsAny reports whether the function calls a package-local function whose
// name satisfies match.
func callsAny(fn *ast.FuncDecl, match func(string) bool) bool {
	found := false

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		fun := call.Fun

		// A generic helper reads as reservationTransition[T].
		if idx, ok := fun.(*ast.IndexExpr); ok {
			fun = idx.X
		}

		ident, ok := fun.(*ast.Ident)
		if !ok {
			return true
		}

		if match(ident.Name) {
			found = true
			return false
		}

		return true
	})

	return found
}

// funcLabel names a function the way a reader would: Type.Method, or the bare
// name for a package-level function.
func funcLabel(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}

	recv := fn.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}

	if ident, ok := recv.(*ast.Ident); ok {
		return ident.Name + "." + fn.Name.Name
	}

	return fn.Name.Name
}

package entities

import (
	"go/ast"
	"go/parser"
	"go/token"
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
// The rule it enforces: if a facade function names a generated client operation
// whose request builder formats a value into the URL PATH, that function must
// refuse bad path ids locally — by calling requirePathIDs itself, or by handing
// the operation to a transition helper that does.
//
// Both halves are read out of the source at test time rather than from a
// checked-in list, so a newly generated operation or a newly written facade
// method is covered the moment it lands, without anyone remembering to add it.
func TestEveryPathParameterOperationIsGuarded(t *testing.T) {
	pathOps := operationsWithPathParameters(t)
	require.NotEmpty(t, pathOps, "found no generated operations with path parameters; the scan is broken, not the code")

	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	require.NoError(t, err)

	var unguarded []string

	guarded := 0
	helperGuards := map[string]bool{}

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}

			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}

				if transitionHelpers[fn.Name.Name] {
					helperGuards[fn.Name.Name] = callsRequirePathIDs(fn)
				}

				ops := pathOperationsNamedBy(fn, pathOps)
				if len(ops) == 0 {
					continue
				}

				if guardsPathIDs(fn) {
					guarded++
					continue
				}

				unguarded = append(unguarded, funcLabel(fn)+" reaches "+strings.Join(ops, ", ")+
					" (declared at "+filepath.Base(name)+":"+
					strconv.Itoa(fset.Position(fn.Pos()).Line)+")")
			}
		}
	}

	// A transition helper takes the generated method as a PARAMETER, so its own
	// body never names the operation and the loop above cannot see it. Its
	// callers are credited purely on the promise that it guards, so that promise
	// has to be checked here or deleting the guard from the helper silently
	// unguards every method that delegates to it.
	for name := range transitionHelpers {
		require.True(t, helperGuards[name],
			"%s is credited as a path-id guard for the facade methods that delegate to it, "+
				"so it must call requirePathIDs; found=%v, guards=%v",
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

// operationsWithPathParameters reads the generated clients and returns the
// operations whose request builder formats at least one value into the URL
// path. oapi-codegen marks exactly those with runtime.ParamLocationPath.
func operationsWithPathParameters(t *testing.T) map[string]bool {
	t.Helper()

	ops := map[string]bool{}

	for _, dir := range []string{"../internal/genledger", "../internal/gentracer"} {
		fset := token.NewFileSet()

		pkgs, err := parser.ParseDir(fset, dir, nil, 0)
		require.NoError(t, err)

		for _, pkg := range pkgs {
			for _, file := range pkg.Files {
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
		}
	}

	return ops
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

// pathOperationsNamedBy returns the path-parameter operations a facade function
// names, whether it CALLS the generated method or passes it as a function value
// to a transition helper. Both reach the same request builder, so both count.
func pathOperationsNamedBy(fn *ast.FuncDecl, pathOps map[string]bool) []string {
	seen := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
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

// callsRequirePathIDs reports the DIRECT guard only. Transition helpers are
// checked with this rather than with guardsPathIDs, which would let a helper
// vouch for itself.
func callsRequirePathIDs(fn *ast.FuncDecl) bool {
	return callsAny(fn, func(name string) bool { return name == "requirePathIDs" })
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

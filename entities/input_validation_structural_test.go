package entities

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEveryInputValidationIsTyped is the structural invariant behind the
// "locally refused input is a validation failure" contract.
//
// Callers branch on sdkerrors.IsValidationError to tell "you sent something
// wrong, fix it and retry" apart from "the ledger is unhappy, do not retry
// blindly". The models do not all return the SDK error type from Validate, so a
// facade that hands the model's error straight back leaks an unclassified
// failure — and on a write path an unclassified failure is the one a caller is
// most likely to retry, against a request that never left. validationErr is
// what normalises that, and it has to be on every input path or the
// classification is a coin flip per model.
//
// The behavioural table in input_validation_typing_test.go proves the
// classification is right on the paths it names. This proves no path is
// missing, which a table of examples cannot.
func TestEveryInputValidationIsTyped(t *testing.T) {
	fset := token.NewFileSet()

	var unwrapped []string

	wrapped, exempt := 0, 0

	for name, file := range parseGoFiles(t, fset, ".") {
		typedByValidationErr := wrappedValidateCalls(file)

		// The walk is per FUNCTION rather than per file, so the exemption below can
		// be checked against the enclosing signature instead of against a name.
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			listOpts := listOptsParams(fn)

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := validateCallReceiver(n)
				if !ok {
					return true
				}

				switch {
				case typedByValidationErr[n.Pos()]:
					wrapped++
				case isListOptsReceiver(sel, listOpts):
					// List options are validated by ValidatePageListOpts /
					// ValidateCursorListOpts, which build the SDK error type
					// themselves, so wrapping would be a no-op. The exemption is
					// not taken on trust: TestListOptsValidationIsTyped exercises
					// it through the public pipeline.
					exempt++
				default:
					unwrapped = append(unwrapped,
						filepath.Base(name)+":"+strconv.Itoa(fset.Position(n.Pos()).Line)+
							" — "+exprText(sel)+".Validate() is returned without validationErr")
				}

				return true
			})
		}
	}

	sort.Strings(unwrapped)

	require.Empty(t, unwrapped,
		"every locally refused input must classify as a validation failure, so each model's "+
			"Validate must be routed through validationErr:\n  %s",
		strings.Join(unwrapped, "\n  "))

	t.Logf("input validation typed on %d call sites, %d list-opts sites exempt", wrapped, exempt)

	// Floors, so a scan that silently stops matching cannot read as success.
	require.Greater(t, wrapped, 40, "expected validationErr on more than 40 input sites, found %d", wrapped)
	require.Greater(t, exempt, 20, "expected more than 20 list-opts sites, found %d", exempt)
}

// validateCallReceiver reports whether a node is a `X.Validate()` call and, if
// so, returns the X it was called on.
func validateCallReceiver(n ast.Node) (ast.Expr, bool) {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return nil, false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Validate" {
		return nil, false
	}

	return sel.X, true
}

// wrappedValidateCalls returns the positions of the Validate calls that appear
// as an argument to validationErr.
func wrappedValidateCalls(file *ast.File) map[token.Pos]bool {
	wrapped := map[token.Pos]bool{}

	ast.Inspect(file, func(n ast.Node) bool {
		outer, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if ident, ok := outer.Fun.(*ast.Ident); !ok || ident.Name != "validationErr" {
			return true
		}

		for _, arg := range outer.Args {
			inner, ok := arg.(*ast.CallExpr)
			if !ok {
				continue
			}

			if sel, ok := inner.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Validate" {
				wrapped[inner.Pos()] = true
			}
		}

		return true
	})

	return wrapped
}

// isListOptsReceiver reports whether a Validate call is on one of the enclosing
// function's list-options parameters.
//
// The exemption is bound to the DECLARED TYPE, not to the identifier text. An
// earlier version exempted anything spelled "opts", which read one direction of
// the hazard and left the other open: a facade naming a request-BODY input
// "opts" was exempted too, and its model's Validate then escaped the
// validationErr requirement with this scan still green. That is the same
// shape-versus-fact hole the sibling scans in this package are written to avoid
// — a check that asks how something is spelled rather than what it is.
func isListOptsReceiver(x ast.Expr, listOpts map[string]bool) bool {
	ident, ok := x.(*ast.Ident)

	return ok && listOpts[ident.Name]
}

// listOptsParams returns the names of a function's parameters whose declared
// type is a list-options type.
//
// The type is matched by its "ListOpts" suffix, which is a naming rule the
// models package holds without exception (AccountsListOpts, TransactionsListOpts,
// InstrumentsListOpts, ...) and which the models' own list-opts test pins. A
// suffix on the TYPE is a different claim from a suffix on the variable: the
// caller cannot rename a type at the call site.
func listOptsParams(fn *ast.FuncDecl) map[string]bool {
	if fn.Type.Params == nil {
		return nil
	}

	names := map[string]bool{}

	for _, field := range fn.Type.Params.List {
		if !isListOptsType(field.Type) {
			continue
		}

		for _, name := range field.Names {
			names[name.Name] = true
		}
	}

	return names
}

// isListOptsType reports whether a parameter type names a list-options type,
// through a qualified name (models.AccountsListOpts) or a bare one.
func isListOptsType(x ast.Expr) bool {
	switch t := x.(type) {
	case *ast.SelectorExpr:
		return strings.HasSuffix(t.Sel.Name, "ListOpts")
	case *ast.Ident:
		return strings.HasSuffix(t.Name, "ListOpts")
	default:
		return false
	}
}

// exprText renders the receiver of a Validate call for the failure message.
func exprText(x ast.Expr) string {
	switch e := x.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprText(e.X) + "." + e.Sel.Name
	default:
		return "input"
	}
}

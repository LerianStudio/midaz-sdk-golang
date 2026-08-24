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

	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	require.NoError(t, err)

	var unwrapped []string

	wrapped, exempt := 0, 0

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}

			typedByValidationErr := wrappedValidateCalls(file)

			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Validate" {
					return true
				}

				switch {
				case typedByValidationErr[call.Pos()]:
					wrapped++
				case isListOptsReceiver(sel.X):
					// List options are validated by ValidatePageListOpts /
					// ValidateCursorListOpts, which build the SDK error type
					// themselves, so wrapping would be a no-op. The exemption is
					// not taken on trust: TestListOptsValidationIsTyped exercises
					// it through the public pipeline.
					exempt++
				default:
					unwrapped = append(unwrapped,
						filepath.Base(name)+":"+strconv.Itoa(fset.Position(call.Pos()).Line)+
							" — "+exprText(sel.X)+".Validate() is returned without validationErr")
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

// isListOptsReceiver reports whether a Validate call is on the list-options
// parameter. Every facade spells that parameter "opts"; a differently named
// receiver is treated as an input and must be wrapped.
func isListOptsReceiver(x ast.Expr) bool {
	ident, ok := x.(*ast.Ident)

	return ok && ident.Name == "opts"
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

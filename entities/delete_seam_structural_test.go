package entities

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEveryDeleteRoutesThroughTheSharedSeam is the structural invariant behind
// the "a 2xx delete with an empty body is a success" fix.
//
// The defect it guards is not per-facade. The generated DELETE parser decodes
// the body as an error whenever the Content-Type contains "json", so a bodiless
// 204 stamped with a JSON content type by any proxy makes a SUCCESSFUL delete
// report "unexpected end of JSON input" — identically, in every facade. Fixing
// it once only stays fixed if no facade re-opens the private copy.
//
// The rule: a facade function that names a generated Delete* operation must hand
// the response to deleteResource. Reaching for the generated *WithResponse
// spelling is refused outright, because that spelling IS the broken parser.
//
// Derived from the source at test time rather than from a checked-in list, so a
// delete added tomorrow is covered when it lands rather than when someone
// remembers it.
func TestEveryDeleteRoutesThroughTheSharedSeam(t *testing.T) {
	fset := token.NewFileSet()

	var offenders []string

	seamed := 0

	for name, file := range parseGoFiles(t, fset, ".") {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			parsed, raw := deleteOperationsNamedBy(fn)
			if len(parsed) == 0 && len(raw) == 0 {
				continue
			}

			site := filepath.Base(name) + ":" + strconv.Itoa(fset.Position(fn.Pos()).Line)

			switch {
			case len(parsed) > 0:
				offenders = append(offenders, funcLabel(fn)+" ("+site+") calls the generated parser "+
					strings.Join(parsed, ", ")+"; a bodiless 204 with a JSON content type fails there")
			case !callsDeleteResource(fn):
				offenders = append(offenders, funcLabel(fn)+" ("+site+") reaches "+
					strings.Join(raw, ", ")+" without deleteResource")
			default:
				seamed++
			}
		}
	}

	sort.Strings(offenders)

	require.Empty(t, offenders,
		"every delete must decide success on the status alone, through deleteResource:\n  %s",
		strings.Join(offenders, "\n  "))

	// A floor, so a scan that stops matching cannot read as success. The tracer's
	// two deletes route through their own readRawResponse pair and are counted
	// here too — they never used the generated parser.
	require.Greater(t, seamed, 14,
		"expected the shared delete seam on more than 14 facade functions; found %d", seamed)
}

// deleteOperationsNamedBy splits the generated delete operations a function
// names into the ones reached through the generated parser (*WithResponse) and
// the ones reached raw. Only the ledger plane is scanned by name here; the
// tracer's deletes are matched by the same Delete prefix.
func deleteOperationsNamedBy(fn *ast.FuncDecl) (parsed, raw []string) {
	seenParsed := map[string]bool{}
	seenRaw := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !strings.HasPrefix(sel.Sel.Name, "Delete") {
			return true
		}

		// f.ledger.X / f.tracer.X only — never a facade calling its own Delete.
		inner, ok := sel.X.(*ast.SelectorExpr)
		if !ok || (inner.Sel.Name != "ledger" && inner.Sel.Name != "tracer") {
			return true
		}

		if strings.HasSuffix(sel.Sel.Name, "WithResponse") {
			seenParsed[sel.Sel.Name] = true
		} else {
			seenRaw[sel.Sel.Name] = true
		}

		return true
	})

	return sortedKeys(seenParsed), sortedKeys(seenRaw)
}

// callsDeleteResource reports whether the function hands its response to the
// shared delete seam. The tracer facades predate it and use the readRawResponse
// pair directly, which decides success on the status the same way.
func callsDeleteResource(fn *ast.FuncDecl) bool {
	return callsAny(fn, func(name string) bool {
		return name == "deleteResource" || name == "readRawResponse"
	})
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

// readFileForScan reads a source file for the scans that need its raw text.
func readFileForScan(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(data)
}
